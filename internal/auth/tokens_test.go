package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCancelTokenPersistsAcrossProcesses(t *testing.T) {
	home := t.TempDir()
	token, err := IssueCancelToken(home, "132887395")
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("empty token")
	}
	// Simulate a new process reading the same config dir.
	if err := ConsumeCancelToken(home, token, "132887395"); err != nil {
		t.Fatal(err)
	}
	if err := ConsumeCancelToken(home, token, "132887395"); err == nil {
		t.Fatal("expected single-use failure")
	}
	path := filepath.Join(home, "confirm-tokens.json")
	if _, err := os.ReadFile(path); err != nil {
		t.Fatalf("token file missing: %v", err)
	}
}
