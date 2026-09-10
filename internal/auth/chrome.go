package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ImportChromeCookies extracts spothero.com cookies from a Chrome profile.
// Requires a local Chrome install; returns an error when unavailable.
func ImportChromeCookies() (*Session, error) {
	profileDir, err := defaultChromeProfile()
	if err != nil {
		return nil, err
	}
	cookiesDB := filepath.Join(profileDir, "Cookies")
	if _, err := os.Stat(cookiesDB); err != nil {
		return nil, fmt.Errorf("chrome cookies database not found at %s: sign in at spothero.com first", cookiesDB)
	}
	// Use python3 + browser_cookie3 or sqlite3 fallback via a small helper script.
	if sess, err := importViaPython(cookiesDB); err == nil {
		return sess, nil
	}
	return importViaSQLiteCopy(cookiesDB)
}

func defaultChromeProfile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "Default"), nil
	case "linux":
		return filepath.Join(home, ".config", "google-chrome", "Default"), nil
	case "windows":
		return filepath.Join(home, "AppData", "Local", "Google", "Chrome", "User Data", "Default"), nil
	default:
		return "", fmt.Errorf("unsupported OS for --chrome: %s", runtime.GOOS)
	}
}

func importViaPython(cookiesDB string) (*Session, error) {
	script := `
import sys, json
try:
    import browser_cookie3
except ImportError:
    sys.exit(2)
jar = browser_cookie3.chrome(cookie_file=sys.argv[1])
out = {}
for c in jar:
    if "spothero" in c.domain:
        out[c.name] = c.value
print(json.dumps(out))
`
	cmd := exec.Command("python3", "-c", script, cookiesDB)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("chrome cookie import (install browser_cookie3: pip install browser_cookie3): %w", err)
	}
	var cookies map[string]string
	if err := json.Unmarshal(out, &cookies); err != nil {
		return nil, err
	}
	if len(cookies) == 0 {
		return nil, fmt.Errorf("no spothero.com cookies in chrome; sign in at https://spothero.com first")
	}
	return &Session{
		Cookies:   cookies,
		Source:    "chrome",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func importViaSQLiteCopy(cookiesDB string) (*Session, error) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		return nil, fmt.Errorf("chrome import needs python3+browser_cookie3 or sqlite3; sign in at spothero.com and use auth login --cookies-file")
	}
	tmp := filepath.Join(os.TempDir(), "spothero-chrome-cookies.db")
	copyCmd := exec.Command("cp", cookiesDB, tmp)
	if err := copyCmd.Run(); err != nil {
		return nil, fmt.Errorf("copy chrome cookies db: %w", err)
	}
	defer os.Remove(tmp)
	query := `SELECT name, value FROM cookies WHERE host_key LIKE '%spothero%'`
	cmd := exec.Command("sqlite3", tmp, query)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("read chrome cookies: %w", err)
	}
	cookies := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 {
			cookies[parts[0]] = parts[1]
		}
	}
	if len(cookies) == 0 {
		return nil, fmt.Errorf("no spothero.com cookies in chrome; sign in at https://spothero.com first")
	}
	return &Session{
		Cookies:   cookies,
		Source:    "chrome",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}
