package client

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// GetMe returns the authenticated consumer account (preferred over GET /user/).
func (c *Client) GetMe() (UserAccount, error) {
	raw, err := c.doJSONData(http.MethodGet, PathUsersMe, nil)
	if err != nil {
		return UserAccount{}, err
	}
	return parseUserAccount(raw)
}

// ResolveEmail prefers /users/me/, falling back to /user/.
func (c *Client) ResolveEmail(preferred string) (string, error) {
	if preferred != "" {
		return preferred, nil
	}
	if me, err := c.GetMe(); err == nil && me.Email != "" {
		return me.Email, nil
	}
	user, err := c.GetUser()
	if err != nil {
		return "", err
	}
	if user.Email == "" {
		return "", fmt.Errorf("account email unavailable")
	}
	return user.Email, nil
}

func parseUserAccount(raw json.RawMessage) (UserAccount, error) {
	var wire struct {
		ID             FlexInt `json:"id"`
		Email          string  `json:"email"`
		FirstName      string  `json:"first_name"`
		LastName       string  `json:"last_name"`
		DefaultCardID  FlexInt `json:"default_card_id"`
		PaymentMethods []struct {
			ID             FlexInt `json:"id"`
			CardID         FlexInt `json:"card_id"`
			CardExternalID string  `json:"card_external_id"`
			IsDefault      bool    `json:"is_default"`
		} `json:"payment_methods"`
		Cards []creditCardWire `json:"cards"`
		CreditCards []creditCardWire `json:"credit_cards"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return UserAccount{}, err
	}
	out := UserAccount{
		ID:        int(wire.ID),
		Email:     wire.Email,
		FirstName: wire.FirstName,
		LastName:  wire.LastName,
	}
	if wire.DefaultCardID > 0 {
		out.DefaultCardID = int(wire.DefaultCardID)
	}
	cards := append(append([]creditCardWire{}, wire.CreditCards...), wire.Cards...)
	for _, pm := range wire.PaymentMethods {
		cards = append(cards, creditCardWire{
			ID:             pm.ID,
			CardID:         pm.CardID,
			CardExternalID: pm.CardExternalID,
			IsDefault:      pm.IsDefault,
		})
	}
	out.CreditCards = normalizeCreditCards(cards)
	if ext, id, ok := pickDefaultCardExternal(out.CreditCards, out.DefaultCardID); ok {
		out.DefaultCardExternalID = ext
		if out.DefaultCardID == 0 {
			out.DefaultCardID = id
		}
	}
	return out, nil
}

type creditCardWire struct {
	ID             FlexInt `json:"id"`
	CardID         FlexInt `json:"card_id"`
	CardExternalID string  `json:"card_external_id"`
	IsDefault      bool    `json:"is_default"`
}

func normalizeCreditCards(wires []creditCardWire) []CreditCard {
	out := make([]CreditCard, 0, len(wires))
	for _, w := range wires {
		cardID := int(w.CardID)
		if cardID == 0 {
			cardID = int(w.ID)
		}
		ext := w.CardExternalID
		if cardID == 0 && ext == "" {
			continue
		}
		out = append(out, CreditCard{
			CardID:         cardID,
			CardExternalID: ext,
			IsDefault:      w.IsDefault,
		})
	}
	return out
}

func pickDefaultCardExternal(cards []CreditCard, preferredID int) (externalID string, cardID int, ok bool) {
	if preferredID > 0 {
		for _, c := range cards {
			if c.CardID == preferredID && c.CardExternalID != "" {
				return c.CardExternalID, c.CardID, true
			}
		}
	}
	for _, c := range cards {
		if c.IsDefault && c.CardExternalID != "" {
			return c.CardExternalID, c.CardID, true
		}
	}
	for _, c := range cards {
		if c.CardExternalID != "" {
			return c.CardExternalID, c.CardID, true
		}
	}
	return "", 0, false
}

func resolveCardExternalID(me UserAccount, cardExternalID string, cardID int) (string, error) {
	if cardExternalID != "" {
		return cardExternalID, nil
	}
	lookupID := cardID
	if lookupID == 0 {
		lookupID = me.DefaultCardID
	}
	if ext, _, ok := pickDefaultCardExternal(me.CreditCards, lookupID); ok {
		return ext, nil
	}
	if lookupID > 0 {
		return "", fmt.Errorf("no card_external_id for card_id %d; pass --card-external-id", lookupID)
	}
	return "", fmt.Errorf("no default payment card; pass --card-external-id")
}

// ListVehicles returns saved vehicles for a user id.
func (c *Client) ListVehicles(userID int) ([]VehicleProfile, error) {
	path := fmt.Sprintf(PathUserVehicles, userID)
	raw, err := c.doJSONData(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return parseVehicles(raw)
}

func parseVehicles(raw json.RawMessage) ([]VehicleProfile, error) {
	var list []VehicleProfile
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		return list, nil
	}
	var wrapped struct {
		Results []vehicleWire `json:"results"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, err
	}
	out := make([]VehicleProfile, 0, len(wrapped.Results))
	for _, v := range wrapped.Results {
		out = append(out, v.normalize())
	}
	return out, nil
}

type vehicleWire struct {
	ID                FlexInt `json:"id"`
	LicensePlate      string  `json:"license_plate"`
	LicensePlateState string  `json:"license_plate_state"`
	IsDefault         bool    `json:"is_default"`
}

func (v vehicleWire) normalize() VehicleProfile {
	return VehicleProfile{
		ID:                int(v.ID),
		LicensePlate:      v.LicensePlate,
		LicensePlateState: v.LicensePlateState,
		IsDefault:         v.IsDefault,
	}
}

func pickDefaultVehicle(vehicles []VehicleProfile) (VehicleProfile, bool) {
	for _, v := range vehicles {
		if v.IsDefault {
			return v, true
		}
	}
	if len(vehicles) == 1 {
		return vehicles[0], true
	}
	if len(vehicles) > 0 {
		return vehicles[0], true
	}
	return VehicleProfile{}, false
}
