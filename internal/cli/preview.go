package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// previewStore holds short-lived cancel confirmation tokens in-process.
var previewStore = struct {
	sync.Mutex
	tokens map[string]previewEntry
}{
	tokens: map[string]previewEntry{},
}

type previewEntry struct {
	ReservationID string
	Expires       time.Time
}

func issueCancelToken(reservationID string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	previewStore.Lock()
	defer previewStore.Unlock()
	previewStore.tokens[token] = previewEntry{
		ReservationID: reservationID,
		Expires:       time.Now().Add(15 * time.Minute),
	}
	return token
}

func consumeCancelToken(token, reservationID string) error {
	previewStore.Lock()
	defer previewStore.Unlock()
	entry, ok := previewStore.tokens[token]
	if !ok {
		return fmt.Errorf("invalid or expired confirm token; run cancel preview again")
	}
	if time.Now().After(entry.Expires) {
		delete(previewStore.tokens, token)
		return fmt.Errorf("confirm token expired; run cancel preview again")
	}
	if entry.ReservationID != reservationID {
		return fmt.Errorf("confirm token does not match reservation %s", reservationID)
	}
	delete(previewStore.tokens, token)
	return nil
}
