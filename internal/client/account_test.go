package client

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed testdata/users_me_live_shape.json
var usersMeFixtureAccount []byte

//go:embed testdata/users_vehicles_live_shape.json
var usersVehiclesFixtureAccount []byte

func TestParseUserAccountDefaultCard(t *testing.T) {
	acct, err := parseUserAccount(usersMeFixtureAccount)
	if err != nil {
		t.Fatal(err)
	}
	if acct.ID != 42 || acct.Email != "user@example.com" || acct.DefaultCardID != 123 {
		t.Fatalf("%+v", acct)
	}
}

func TestParseVehiclesDefault(t *testing.T) {
	vehicles, err := parseVehicles(usersVehiclesFixtureAccount)
	if err != nil {
		t.Fatal(err)
	}
	v, ok := pickDefaultVehicle(vehicles)
	if !ok || v.ID != 37062538 || v.LicensePlate != "9XCV666" {
		t.Fatalf("%+v ok=%v", v, ok)
	}
}

func TestParseUserAccountPaymentMethodsFallback(t *testing.T) {
	raw := json.RawMessage(`{"id":1,"email":"a@b.com","payment_methods":[{"id":99,"is_default":true}]}`)
	acct, err := parseUserAccount(raw)
	if err != nil {
		t.Fatal(err)
	}
	if acct.DefaultCardID != 99 {
		t.Fatalf("card=%d", acct.DefaultCardID)
	}
}
