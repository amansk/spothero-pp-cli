package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const confirmTokenTTL = 15 * time.Minute

type confirmTokenFile struct {
	Cancel map[string]confirmTokenEntry `json:"cancel"`
}

type confirmTokenEntry struct {
	ReservationID string `json:"reservation_id"`
	Expires       string `json:"expires"`
}

var tokenFileMu sync.Mutex

func confirmTokensPath(home string) string {
	return filepath.Join(home, "confirm-tokens.json")
}

func loadConfirmTokens(home string) (confirmTokenFile, error) {
	path := confirmTokensPath(home)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return confirmTokenFile{Cancel: map[string]confirmTokenEntry{}}, nil
		}
		return confirmTokenFile{}, err
	}
	var f confirmTokenFile
	if err := json.Unmarshal(b, &f); err != nil {
		return confirmTokenFile{}, fmt.Errorf("parse confirm tokens: %w", err)
	}
	if f.Cancel == nil {
		f.Cancel = map[string]confirmTokenEntry{}
	}
	return f, nil
}

func saveConfirmTokens(home string, f confirmTokenFile) error {
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	if f.Cancel == nil {
		f.Cancel = map[string]confirmTokenEntry{}
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(confirmTokensPath(home), b, 0o600)
}

func pruneExpiredCancel(f *confirmTokenFile) {
	now := time.Now()
	for token, entry := range f.Cancel {
		exp, err := time.Parse(time.RFC3339, entry.Expires)
		if err != nil || now.After(exp) {
			delete(f.Cancel, token)
		}
	}
}

// IssueCancelToken stores a single-use cancel confirmation token on disk.
func IssueCancelToken(home, reservationID string) (string, error) {
	if home == "" {
		return "", fmt.Errorf("config home required for cancel tokens")
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	tokenFileMu.Lock()
	defer tokenFileMu.Unlock()
	f, err := loadConfirmTokens(home)
	if err != nil {
		return "", err
	}
	pruneExpiredCancel(&f)
	f.Cancel[token] = confirmTokenEntry{
		ReservationID: reservationID,
		Expires:       time.Now().Add(confirmTokenTTL).UTC().Format(time.RFC3339),
	}
	if err := saveConfirmTokens(home, f); err != nil {
		return "", err
	}
	return token, nil
}

// ConsumeCancelToken validates and deletes a cancel confirmation token.
func ConsumeCancelToken(home, token, reservationID string) error {
	if home == "" {
		return fmt.Errorf("config home required for cancel tokens")
	}
	tokenFileMu.Lock()
	defer tokenFileMu.Unlock()
	f, err := loadConfirmTokens(home)
	if err != nil {
		return err
	}
	pruneExpiredCancel(&f)
	entry, ok := f.Cancel[token]
	if !ok {
		return fmt.Errorf("invalid or expired confirm token; run cancel preview again")
	}
	exp, err := time.Parse(time.RFC3339, entry.Expires)
	if err != nil || time.Now().After(exp) {
		delete(f.Cancel, token)
		_ = saveConfirmTokens(home, f)
		return fmt.Errorf("confirm token expired; run cancel preview again")
	}
	if entry.ReservationID != reservationID {
		return fmt.Errorf("confirm token does not match reservation %s", reservationID)
	}
	delete(f.Cancel, token)
	return saveConfirmTokens(home, f)
}
