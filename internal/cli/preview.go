package cli

import (
	"fmt"

	"github.com/amansk/spothero-pp-cli/internal/auth"
)

func issueCancelToken(home, reservationID string) (string, error) {
	return auth.IssueCancelToken(home, reservationID)
}

func consumeCancelToken(home, token, reservationID string) error {
	if err := auth.ConsumeCancelToken(home, token, reservationID); err != nil {
		return fmt.Errorf("%v", err)
	}
	return nil
}
