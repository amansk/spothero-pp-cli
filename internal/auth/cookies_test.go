package auth_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amansk/spothero-pp-cli/internal/auth"
)

func TestParseCookieInput(t *testing.T) {
	s, err := auth.ParseCookieInput("sessionid=abc; csrftoken=xyz", "test")
	if err != nil {
		t.Fatal(err)
	}
	if s.CookieHeader() != "sessionid=abc; csrftoken=xyz" {
		t.Fatalf("header=%q", s.CookieHeader())
	}
}

func TestParsePlaywrightStorageState(t *testing.T) {
	raw := `{"cookies":[{"name":"sessionid","value":"abc","domain":".spothero.com"}]}`
	s, err := auth.ParseCookiesFile([]byte(raw), "test")
	if err != nil {
		t.Fatal(err)
	}
	if s.Cookies["sessionid"] != "abc" {
		t.Fatalf("cookies=%v", s.Cookies)
	}
}

func TestSaveSessionPermissions(t *testing.T) {
	dir := t.TempDir()
	s := &auth.Session{Cookies: map[string]string{"a": "b"}, Source: "test"}
	if err := auth.SaveSession(dir, s); err != nil {
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
	s := &auth.Session{Cookies: map[string]string{"a": "b"}, Source: "test"}
	if err := auth.SaveSession(dir, s); err != nil {
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

func TestStatusNeverIncludesSecrets(t *testing.T) {
	s, _ := auth.ParseCookieInput("secret=VALUE", "test")
	st := s.Status()
	for _, v := range st {
		if str, ok := v.(string); ok && str == "VALUE" {
			t.Fatal("status leaked secret")
		}
	}
}
