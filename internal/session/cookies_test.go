package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"spothero-pp-cli/internal/session"
)

func TestParseCookieInput(t *testing.T) {
	s, err := session.ParseCookieInput("sessionid=abc; csrftoken=xyz", "test")
	if err != nil {
		t.Fatal(err)
	}
	if s.CookieHeader() != "sessionid=abc; csrftoken=xyz" {
		t.Fatalf("header=%q", s.CookieHeader())
	}
}

func TestParseSessionFileV018Map(t *testing.T) {
	raw := []byte(`{"cookies":{"AUTH_TOKEN":"fixture","appSession":"fixture-session"},"source":"chrome","updated_at":"2026-10-11T00:00:00Z"}`)
	s, err := session.ParseSessionFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if s.Cookies["AUTH_TOKEN"] != "fixture" || s.Source != "chrome" {
		t.Fatalf("source=%q names=%d", s.Source, len(s.Cookies))
	}
	header := session.SafeHeader(s.Cookies)
	if !strings.Contains(header, "AUTH_TOKEN=fixture") {
		t.Fatalf("header missing session cookie: %q", header)
	}
}

func TestSafeHeaderDropsUnparseableCookie(t *testing.T) {
	header := session.SafeHeader(map[string]string{
		"AUTH_TOKEN": "fixture",
		"broken":     "has;no-equals-after",
	})
	if header != "AUTH_TOKEN=fixture" {
		t.Fatalf("header=%q", header)
	}
}

func TestParsePlaywrightStorageState(t *testing.T) {
	raw := `{"cookies":[{"name":"sessionid","value":"abc","domain":".spothero.com"}]}`
	s, err := session.ParseCookiesFile([]byte(raw), "test")
	if err != nil {
		t.Fatal(err)
	}
	if s.Cookies["sessionid"] != "abc" {
		t.Fatalf("cookies=%v", s.Cookies)
	}
}

func TestSaveSessionPermissions(t *testing.T) {
	dir := t.TempDir()
	s := &session.Session{Cookies: map[string]string{"a": "b"}, Source: "test"}
	if err := session.SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "cookies.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm=%o", info.Mode().Perm())
	}
}

func TestSaveSessionTightensExistingPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookies.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &session.Session{Cookies: map[string]string{"a": "b"}, Source: "test"}
	if err := session.SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm=%o", info.Mode().Perm())
	}
}
