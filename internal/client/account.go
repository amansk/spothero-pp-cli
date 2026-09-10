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
		ID        FlexInt `json:"id"`
		Email     string  `json:"email"`
		FirstName string  `json:"first_name"`
		LastName  string  `json:"last_name"`
		DefaultCardID FlexInt `json:"default_card_id"`
		PaymentMethods []struct {
			ID        FlexInt `json:"id"`
			CardID    FlexInt `json:"card_id"`
			IsDefault bool    `json:"is_default"`
		} `json:"payment_methods"`
		Cards []struct {
			CardID    FlexInt `json:"card_id"`
			IsDefault bool    `json:"is_default"`
		} `json:"cards"`
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
	if out.DefaultCardID == 0 {
		for _, pm := range wire.PaymentMethods {
			if pm.IsDefault {
				if pm.CardID > 0 {
					out.DefaultCardID = int(pm.CardID)
				} else if pm.ID > 0 {
					out.DefaultCardID = int(pm.ID)
				}
				break
			}
		}
	}
	if out.DefaultCardID == 0 {
		for _, card := range wire.Cards {
			if card.IsDefault && card.CardID > 0 {
				out.DefaultCardID = int(card.CardID)
				break
			}
		}
	}
	if out.DefaultCardID == 0 && len(wire.Cards) == 1 && wire.Cards[0].CardID > 0 {
		out.DefaultCardID = int(wire.Cards[0].CardID)
	}
	return out, nil
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
