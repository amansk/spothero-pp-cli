package cli

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"spothero-pp-cli/internal/chromecookies"
	"spothero-pp-cli/internal/cliutil/testenv"
	"spothero-pp-cli/internal/config"

	_ "modernc.org/sqlite"
)

// encryptLinuxCookie builds a Chrome basic-store value: v10, AES-CBC, host hash.
func encryptLinuxCookie(t *testing.T, key []byte, host, value string) []byte {
	t.Helper()
	sum := sha256.Sum256([]byte(host))
	plain := append(sum[:], []byte(value)...)
	pad := aes.BlockSize - len(plain)%aes.BlockSize
	plain = append(plain, []byte(strings.Repeat(string(rune(pad)), pad))...)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, []byte(strings.Repeat(" ", aes.BlockSize))).CryptBlocks(out, plain)
	return append([]byte("v10"), out...)
}

func writePeanutsCookieDB(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	key, err := chromecookies.Key("peanuts", 1)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE meta(key TEXT, value TEXT)`,
		`INSERT INTO meta VALUES('version','24')`,
		`CREATE TABLE cookies(host_key TEXT, name TEXT, value TEXT, encrypted_value BLOB)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	insert := func(host, name, value string) {
		t.Helper()
		if _, err := db.Exec(`INSERT INTO cookies VALUES(?,?,'',?)`, host, name, encryptLinuxCookie(t, key, host, value)); err != nil {
			t.Fatal(err)
		}
	}
	insert(".spothero.com", "AUTH_TOKEN", "fixture")
	insert("spothero.com", "csrftoken", "fixture-csrf")
}

// hideKeyringAndSQLite leaves secret-tool failing and sqlite3 off PATH, which
// is the box where 0.2.0 fell through to pycookiecheat.
func hideKeyringAndSQLite(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "secret-tool")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestAuthLoginChromeLinuxWithoutKeyringOrSQLite(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux chrome profile layout")
	}
	layouts := []struct {
		name  string
		place func(t *testing.T, home string)
		want  string
	}{
		{
			name: "default cookies database",
			place: func(t *testing.T, home string) {
				writePeanutsCookieDB(t, filepath.Join(home, ".config", "google-chrome", "Default", "Cookies"))
			},
			want: "Chrome/Default",
		},
		{
			name: "network cookies only",
			place: func(t *testing.T, home string) {
				writePeanutsCookieDB(t, filepath.Join(home, ".config", "google-chrome", "Default", "Network", "Cookies"))
			},
			want: "Chrome/Default",
		},
		{
			name: "default is a symlink",
			place: func(t *testing.T, home string) {
				real := filepath.Join(home, "real-profile")
				writePeanutsCookieDB(t, filepath.Join(real, "Cookies"))
				linkParent := filepath.Join(home, ".config", "google-chrome")
				if err := os.MkdirAll(linkParent, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(real, filepath.Join(linkParent, "Default")); err != nil {
					t.Fatal(err)
				}
			},
			want: "Chrome/Default",
		},
		{
			name: "signed in under google-chrome-headless",
			place: func(t *testing.T, home string) {
				// An empty stable directory is what made discovery return
				// "no Chrome profile found" and skip the profile that has cookies.
				if err := os.MkdirAll(filepath.Join(home, ".config", "google-chrome"), 0o700); err != nil {
					t.Fatal(err)
				}
				writePeanutsCookieDB(t, filepath.Join(home, ".config", "google-chrome-headless", "Default", "Cookies"))
			},
			want: "Chrome Headless/Default",
		},
	}
	for _, layout := range layouts {
		t.Run(layout.name, func(t *testing.T) {
			testenv.Isolate(t)
			hideKeyringAndSQLite(t)
			home, err := os.UserHomeDir()
			if err != nil {
				t.Fatal(err)
			}
			layout.place(t, home)
			srv := fakeAccountAPI(t, true)
			t.Setenv("SPOTHERO_BASE_URL", srv.URL)
			out, code := runCLI(t, "auth", "login", "--chrome")
			if code != 0 {
				t.Fatalf("exit = %d: %s", code, out)
			}
			if !strings.Contains(out, "Session saved") {
				t.Fatalf("session was not saved: %s", out)
			}
			if !strings.Contains(out, layout.want) {
				t.Fatalf("profile = want %q in: %s", layout.want, out)
			}
			if strings.Contains(out, "AUTH_TOKEN=") || strings.Contains(out, "Native Chrome read did not") {
				t.Fatalf("login leaked or skipped the native read: %s", out)
			}
			statusOut, statusCode := runCLI(t, "auth", "status", "--json")
			if statusCode != 0 {
				t.Fatalf("status exit = %d: %s", statusCode, statusOut)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(statusOut), &got); err != nil {
				t.Fatalf("status json: %v: %s", err, statusOut)
			}
			if got["authenticated"] != true {
				t.Fatalf("authenticated = %v: %s", got["authenticated"], statusOut)
			}
			cfg, err := config.Load("")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(cfg.CookieCredential(), "AUTH_TOKEN=fixture") {
				t.Fatal("saved credential is not the Chrome session cookie")
			}
		})
	}
}

func TestAuthLoginChromeLinuxWrongKeyringUsesPeanuts(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux chrome profile layout")
	}
	testenv.Isolate(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "secret-tool")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' 'not-the-chrome-password'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	writePeanutsCookieDB(t, filepath.Join(home, ".config", "google-chrome", "Default", "Cookies"))
	srv := fakeAccountAPI(t, true)
	t.Setenv("SPOTHERO_BASE_URL", srv.URL)
	out, code := runCLI(t, "auth", "login", "--chrome")
	if code != 0 {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.Contains(out, "Session saved") || strings.Contains(out, "AUTH_TOKEN=") {
		t.Fatalf("login output: %s", out)
	}
}

func TestDiscoverChromeProfileWhenListingIsDenied(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux directory mode")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory read permission")
	}
	testenv.Isolate(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(home, ".config", "google-chrome")
	writePeanutsCookieDB(t, filepath.Join(dataDir, "Default", "Cookies"))
	if err := os.Chmod(dataDir, 0o111); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dataDir, 0o700) })
	profiles, err := discoverChromeProfilesNative(t.Context(), "spothero.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].Dir != "Default" {
		t.Fatalf("profiles = %+v", profiles)
	}
	db, ok := chromecookies.LocateDB(profileCookieDir(profiles[0]))
	if !ok {
		t.Fatal("cookie database not found through an unreadable listing")
	}
	key, err := chromecookies.Key("peanuts", 1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := chromecookies.Read(t.Context(), db, "spothero.com", key)
	if err != nil {
		t.Fatal(err)
	}
	if got["AUTH_TOKEN"] != "fixture" {
		t.Fatal("did not decrypt the session cookie")
	}
}

func TestAuthLoginCookiesFileAcceptsV018Session(t *testing.T) {
	testenv.Isolate(t)
	srv := fakeAccountAPI(t, true)
	t.Setenv("SPOTHERO_BASE_URL", srv.URL)
	// Map shape written by v0.1.8 SaveSession. A junk value must not fail the
	// header check that used to report "'=' not found".
	body := `{"cookies":{"AUTH_TOKEN":"fixture","appSession":"fixture-session","broken":"has;no-equals-after"},"source":"chrome","updated_at":"2026-10-11T00:00:00Z"}`
	out, code := runCLI(t, "auth", "login", "--cookies-file", writeCookieFile(t, body))
	if code != 0 {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if strings.Contains(out, "malformed") || strings.Contains(out, "AUTH_TOKEN=") {
		t.Fatalf("import failed or leaked the cookie: %s", out)
	}
	array := `{"cookies":[{"name":"AUTH_TOKEN","value":"fixture","domain":".spothero.com","path":"/"}],"source":"chrome","updated_at":"2026-10-11T00:00:00Z"}`
	out, code = runCLI(t, "auth", "login", "--cookies-file", writeCookieFile(t, array))
	if code != 0 {
		t.Fatalf("array session exit = %d: %s", code, out)
	}
}
