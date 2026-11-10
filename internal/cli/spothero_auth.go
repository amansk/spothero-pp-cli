package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"spothero-pp-cli/internal/chromecookies"
	"spothero-pp-cli/internal/config"
	"spothero-pp-cli/internal/consumer"
	"spothero-pp-cli/internal/exitcode"
	"spothero-pp-cli/internal/session"
)

// sessionCookie is the one cookie the API authenticates. Proven live on 2026-11-07:
// AUTH_TOKEN alone returned 200 from /reservations/ and /users/me/, while appSession,
// sh-sso and csrftoken alone returned 401, and the same value as an Authorization
// header with no cookie also returned 401. So it must travel as a cookie.
const sessionCookie = "AUTH_TOKEN"

// consumerClient builds the hand-written client from the generated config, so env
// overrides, the credentials file and the base URL all have one source of truth.
func consumerClient(flags *rootFlags) (*consumer.Client, *config.Config, error) {
	// Building the generated client first applies its config and profile gates, so hand
	// commands refuse in the same situations as generated ones.
	if _, err := flags.newClient(); err != nil {
		return nil, nil, err
	}
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, nil, configErr(err)
	}
	var sess *session.Session
	if raw := strings.TrimSpace(cfg.CookieCredential()); raw != "" {
		if sess, err = session.ParseCookieInput(raw, cfg.AuthSource); err != nil {
			return nil, nil, authErr(err)
		}
	}
	c := consumer.New(sess)
	c.BaseURL = cfg.BaseURL
	c.CraigBaseURL = inventoryBaseURL()
	c.DryRun = flags.dryRun
	return c, cfg, nil
}

// mapConsumerErr keeps the documented exit codes when errors cross from the client
// into generated code.
func mapConsumerErr(err error) error {
	if err == nil {
		return nil
	}
	var ex *exitcode.Error
	if exitcode.As(err, &ex) {
		switch ex.Code {
		case exitcode.Usage:
			return usageErr(err)
		case exitcode.NotFound:
			return notFoundErr(err)
		case exitcode.Auth:
			return authErr(err)
		case exitcode.Transient:
			return rateLimitErr(err)
		default:
			return apiErr(err)
		}
	}
	return apiErr(err)
}

// verifyCookieHeader asks the API whether a session works. /users/me/ is used
// because /user/ returns 401 even for a good session.
func verifyCookieHeader(ctx context.Context, flags *rootFlags, header string) error {
	sess, err := session.ParseCookieInput(header, "verify")
	if err != nil {
		return exitcode.Authf("%v", err)
	}
	if sess.CookieValue(sessionCookie) == "" {
		return exitcode.Authf("no non-empty %s cookie in this session", sessionCookie)
	}
	c := consumer.New(sess)
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return err
	}
	c.BaseURL = cfg.BaseURL
	// Bounded so cancellation and a short --timeout also end this check.
	ctx, cancel := boundCtx(ctx, flags)
	defer cancel()
	c.Ctx = ctx
	me, err := c.GetMe()
	if err != nil {
		return err
	}
	// A 200 with no account is not an accepted session; a proxy or captive portal can
	// return one.
	if me.ID == 0 {
		return exitcode.Authf("the API returned no account for this session")
	}
	return nil
}

// verifySessionForLogin turns a failed check into a refusal with next steps.
func verifySessionForLogin(ctx context.Context, flags *rootFlags, header string) error {
	err := verifyCookieHeader(ctx, flags, header)
	if err == nil {
		return nil
	}
	var ex *exitcode.Error
	if exitcode.As(err, &ex) && ex.Code == exitcode.Transient {
		return rateLimitErr(fmt.Errorf("could not reach the API to verify the session, so nothing was saved: %v", err))
	}
	return authErr(fmt.Errorf("the API rejected this session, so nothing was saved (%v). Sign in at https://spothero.com in Chrome, wait a minute, and run auth login --chrome again", err))
}

// nativeChromeSession reads each Chrome profile that holds the session cookie and
// returns the first one the API accepts. Testing against the API rather than
// prompting means multi-profile users get the right account with no question, and
// agents with no input still work.
func nativeChromeSession(ctx context.Context, w io.Writer, flags *rootFlags, domain, profileFlag string) (string, error) {
	profiles, err := discoverChromeProfilesNative(ctx, domain)
	if err != nil {
		return "", err
	}
	if profileFlag != "" {
		profiles = filterChromeProfilesByName(profiles, profileFlag)
		if len(profiles) == 0 {
			return "", fmt.Errorf("Chrome profile %q not found", profileFlag)
		}
	}
	if len(profiles) == 0 {
		return "", fmt.Errorf("no Chrome profile found")
	}
	keys, err := chromecookies.CandidateKeys(ctx)
	if err != nil {
		return "", err
	}
	var lastErr error = chromecookies.ErrNoCookies
	for _, p := range profiles {
		db, ok := chromecookies.LocateDB(profileCookieDir(p))
		if !ok {
			continue
		}
		for _, key := range keys {
			read, readErr := chromecookies.Read(ctx, db, domain, key)
			if readErr != nil {
				lastErr = readErr
				continue
			}
			if read[sessionCookie] == "" {
				lastErr = fmt.Errorf("profile %s has no %s cookie; sign in at https://spothero.com", p.profileLocation(), sessionCookie)
				continue
			}
			parts := []string{sessionCookie + "=" + read[sessionCookie]}
			if v := read["csrftoken"]; v != "" {
				parts = append(parts, "csrftoken="+v)
			}
			header := strings.Join(parts, "; ")
			// A keyring password that does not decrypt this profile must not
			// hide the basic-store key. The API is what decides the key worked.
			if err := verifyCookieHeader(ctx, flags, header); err != nil {
				lastErr = fmt.Errorf("profile %s: %v", p.profileLocation(), err)
				continue
			}
			fmt.Fprintf(w, "Read the session from Chrome profile %q.\n", p.profileLocation())
			return header, nil
		}
	}
	return "", lastErr
}

func profileCookieDir(p chromeProfile) string {
	if p.Dir == "" || p.Dir == "." {
		return p.DataDir
	}
	return filepath.Join(p.DataDir, p.Dir)
}

func filterChromeProfilesByName(profiles []chromeProfile, name string) []chromeProfile {
	want := strings.ToLower(name)
	var matched []chromeProfile
	for _, p := range profiles {
		if strings.ToLower(p.DisplayName) == want || strings.ToLower(p.Dir) == want || strings.ToLower(p.profileLocation()) == want {
			matched = append(matched, p)
		}
	}
	return matched
}

// discoverChromeProfilesNative lists profiles without the sqlite3 binary the
// generated discovery needs. Linux Chrome often keeps the database at
// Default/Cookies or Default/Network/Cookies, including when Default is a
// symlink or the user-data dir is google-chrome-headless. Directory listing
// alone misses those, which is how login reported "no Chrome profile found"
// and fell through to pycookiecheat.
func discoverChromeProfilesNative(_ context.Context, _ string) ([]chromeProfile, error) {
	dirs, err := nativeChromeDataDirs()
	if err != nil {
		return nil, err
	}
	type ranked struct {
		profile chromeProfile
		hasDB   bool
	}
	var found []ranked
	seen := map[string]bool{}
	for _, ch := range dirs {
		for _, name := range nativeProfileDirNames(ch.DataDir) {
			key := ch.DataDir + "\x00" + name
			if seen[key] {
				continue
			}
			seen[key] = true
			dir := ch.DataDir
			if name != "." {
				dir = filepath.Join(ch.DataDir, name)
			}
			display := readProfileDisplayName(filepath.Join(dir, "Preferences"))
			if display == "" {
				display = name
			}
			_, hasDB := chromecookies.LocateDB(dir)
			found = append(found, ranked{
				profile: chromeProfile{Channel: ch.Channel, DataDir: ch.DataDir, Dir: name, DisplayName: display},
				hasDB:   hasDB,
			})
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].hasDB != found[j].hasDB {
			return found[i].hasDB
		}
		if ri, rj := channelRank(found[i].profile.Channel), channelRank(found[j].profile.Channel); ri != rj {
			return ri < rj
		}
		return profileDirRank(found[i].profile.Dir) < profileDirRank(found[j].profile.Dir)
	})
	out := make([]chromeProfile, 0, len(found))
	for _, item := range found {
		if item.hasDB {
			out = append(out, item.profile)
		}
	}
	// A channel with no cookie database still counts as "a profile" only when
	// nothing anywhere has a database, so the caller can say so. Preferring
	// empty stubs ahead of a later channel that has cookies was the regression.
	if len(out) == 0 {
		for _, item := range found {
			out = append(out, item.profile)
		}
	}
	return out, nil
}

func profileDirRank(dir string) int {
	switch {
	case dir == "Default":
		return 0
	case strings.HasPrefix(dir, "Profile "):
		return 1
	case dir == ".":
		return 3
	default:
		return 2
	}
}

// nativeChromeDataDirs is the generated channel list plus the Linux layouts
// Chrome actually uses when there is no desktop session.
func nativeChromeDataDirs() ([]chromeChannelDir, error) {
	if runtime.GOOS != "linux" {
		return chromeChannelDirs()
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	bases := []string{filepath.Join(home, ".config")}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		bases = append([]string{xdg}, bases...)
	}
	type candidate struct {
		channel string
		dir     string
	}
	var candidates []candidate
	for _, base := range bases {
		candidates = append(candidates,
			candidate{chromeChannelStable, filepath.Join(base, "google-chrome")},
			candidate{"Chrome Headless", filepath.Join(base, "google-chrome-headless")},
			candidate{chromeChannelBeta, filepath.Join(base, "google-chrome-beta")},
			candidate{chromeChannelDev, filepath.Join(base, "google-chrome-unstable")},
			candidate{"Chromium", filepath.Join(base, "chromium")},
			candidate{"Chromium Headless", filepath.Join(base, "chromium-headless")},
		)
	}
	candidates = append(candidates,
		candidate{"Chromium", filepath.Join(home, "snap", "chromium", "common", "chromium")},
		candidate{chromeChannelStable, filepath.Join(home, ".var", "app", "com.google.Chrome", "config", "google-chrome")},
	)
	seen := map[string]bool{}
	var existing []chromeChannelDir
	for _, c := range candidates {
		dir := filepath.Clean(c.dir)
		if seen[dir] {
			continue
		}
		info, statErr := os.Stat(dir)
		if statErr != nil || !info.IsDir() {
			continue
		}
		seen[dir] = true
		existing = append(existing, chromeChannelDir{Channel: c.channel, DataDir: dir})
	}
	if len(existing) == 0 {
		return nil, fmt.Errorf("no Chrome user data directory found")
	}
	return existing, nil
}

// nativeProfileDirNames returns profile directory names under a Chrome user-data
// directory. It stats Default directly so a symlink or an unreadable listing
// still resolves the path v0.1.8 read, and it trusts Local State for names that
// are not "Default" or "Profile N".
func nativeProfileDirNames(dataDir string) []string {
	seen := map[string]bool{}
	var names []string
	add := func(name string) {
		if !validProfileDirName(name) || seen[name] {
			return
		}
		dir := dataDir
		if name != "." {
			dir = filepath.Join(dataDir, name)
		}
		info, err := os.Stat(dir) // follows a symlink Default, which ReadDir.IsDir does not
		if err != nil || !info.IsDir() {
			return
		}
		known := name == "." || name == "Default" || strings.HasPrefix(name, "Profile ")
		if !known {
			if _, ok := chromecookies.LocateDB(dir); !ok {
				return
			}
		}
		seen[name] = true
		names = append(names, name)
	}
	add("Default")
	for _, name := range profileNamesFromLocalState(filepath.Join(dataDir, "Local State")) {
		add(name)
	}
	entries, err := os.ReadDir(dataDir)
	if err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if name == "Default" || strings.HasPrefix(name, "Profile ") {
				add(name)
				continue
			}
			full := filepath.Join(dataDir, name)
			info, statErr := os.Stat(full)
			if statErr != nil || !info.IsDir() {
				continue
			}
			if _, ok := chromecookies.LocateDB(full); ok {
				add(name)
			}
		}
	}
	if _, ok := chromecookies.LocateDB(dataDir); ok {
		add(".")
	}
	return names
}

func validProfileDirName(name string) bool {
	if name == "." {
		return true
	}
	if name == "" || name == ".." || strings.ContainsAny(name, `/\`) {
		return false
	}
	return true
}

func profileNamesFromLocalState(path string) []string {
	data, err := os.ReadFile(path) // #nosec G304 -- Chrome's own Local State next to the profile being read
	if err != nil {
		return nil
	}
	var state struct {
		Profile struct {
			InfoCache map[string]json.RawMessage `json:"info_cache"`
			LastUsed  string                     `json:"last_used"`
		} `json:"profile"`
	}
	if json.Unmarshal(data, &state) != nil {
		return nil
	}
	names := make([]string, 0, len(state.Profile.InfoCache)+1)
	if state.Profile.LastUsed != "" {
		names = append(names, state.Profile.LastUsed)
	}
	for name := range state.Profile.InfoCache {
		names = append(names, name)
	}
	return names
}

func safeCookieHeader(cookies map[string]string) string {
	return session.SafeHeader(cookies)
}

// cookieHeaderFromSessionFile accepts the v0.1.8 cookies.json map and a
// storage-state document. The generated importer only understood a cookies
// array; a map fell through to net/http and failed with "'=' not found".
func cookieHeaderFromSessionFile(data []byte) (string, bool, error) {
	if !strings.HasPrefix(strings.TrimSpace(string(data)), "{") {
		return "", false, nil
	}
	stored, err := session.ParseSessionFile(data)
	if err != nil || stored == nil || len(stored.Cookies) == 0 {
		return "", false, nil
	}
	header := session.SafeHeader(stored.Cookies)
	if header == "" {
		return "", true, fmt.Errorf("cookies file has no usable spothero.com cookies")
	}
	return header, true, nil
}

// jwtExpiry reads exp from the session JWT for display. It is not a security check,
// so the signature is not verified, and the API remains the authority.
func jwtExpiry(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(raw, &claims) != nil || claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0).UTC(), true
}

// runVerifiedAuthStatus reports authenticated only when the API accepts the session.
func runVerifiedAuthStatus(cmd *cobra.Command, flags *rootFlags) error {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return configErr(err)
	}
	report := map[string]any{"authenticated": false, "domain": "spothero.com", "config": cfg.Path}
	emit := func(human string) error {
		if flags.asJSON {
			return flags.printJSON(cmd, report)
		}
		fmt.Fprintln(cmd.OutOrStdout(), human)
		for _, k := range []string{"source", "expires_at", "reason", "hint"} {
			if v, ok := report[k]; ok && v != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %v\n", k, v)
			}
		}
		return nil
	}
	if refusals := cfg.CredentialRefusalSummaries(); len(refusals) > 0 {
		report["reason"] = strings.Join(refusals, "; ")
		_ = emit(red("Credentials present but refused"))
		return authErr(cfg.CredentialRefusalError())
	}
	header := strings.TrimSpace(cfg.CookieCredential())
	if header == "" {
		report["reason"] = "no session saved"
		report["hint"] = "spothero-pp-cli auth login --chrome"
		_ = emit(red("Not authenticated"))
		return authErr(errors.New("no credentials configured"))
	}
	report["source"] = cfg.AuthSource
	if sess, perr := session.ParseCookieInput(header, ""); perr == nil {
		if exp, ok := jwtExpiry(sess.CookieValue(sessionCookie)); ok {
			report["expires_at"] = exp.Format(time.RFC3339)
		}
	}
	if flags.dryRun {
		report["reason"] = "dry run: session not checked with the API"
		return emit("Not checked (dry run)")
	}
	if err := verifyCookieHeader(cmd.Context(), flags, header); err != nil {
		var ex *exitcode.Error
		if exitcode.As(err, &ex) && ex.Code == exitcode.Transient {
			report["reason"] = "could not reach the API to check the session"
			_ = emit(red("Not verified"))
			return rateLimitErr(err)
		}
		report["reason"] = err.Error()
		report["hint"] = "spothero-pp-cli auth login --chrome"
		_ = emit(red("Not authenticated: the API rejected the saved session"))
		return authErr(err)
	}
	report["authenticated"] = true
	return emit(green("Authenticated") + " (accepted by the API)")
}
