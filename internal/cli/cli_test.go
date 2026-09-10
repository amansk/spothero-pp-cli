package cli_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amansk/spothero-pp-cli/internal/auth"
	"github.com/amansk/spothero-pp-cli/internal/cli"
	"github.com/amansk/spothero-pp-cli/internal/client"
)

func mockHTTP(t *testing.T) *client.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search-params/":
			_, _ = w.Write([]byte(`{"data":{"latitude":41.88,"longitude":-87.62,"starts":"2026-09-11T09:30","ends":"2026-09-11T12:30","sort":"distance","sort_order":"asc","distance_lt":1609,"page_info":{"setup":{"city":{"slug":"chicago"}}}}}`))
		case "/search/transient":
			_, _ = w.Write([]byte(`{"results":[{"distance":{"walking_meters":50},"rates":[{"quote":{"total_price":{"value":500}}}],"availability":{"available":true},"facility":{"common":{"id":"1","title":"Lot","status":"on_sales_allowed","addresses":[{"street_address":"1 Main","city":"Chicago","state":"IL","postal_code":"60601","types":["search"]}]}}}]}`))
		case "/user/":
			_, _ = w.Write([]byte(`{"data":{"id":1,"email":"user@example.com"}}`))
		case "/users/me/":
			_, _ = w.Write([]byte(`{"data":{"id":42,"email":"user@example.com","default_card_id":46111484,"credit_cards":[{"card_id":46111484,"card_external_id":"REDACTED-CARD-UUID","is_default":true}]}}`))
		case "/users/42/vehicles/":
			_, _ = w.Write([]byte(`{"data":{"results":[{"id":37062538,"license_plate":"9XCV666","license_plate_state":"CA","is_default":true}]}}`))
		case "/reservations/":
			_, _ = w.Write([]byte(`{"data":{"results":[{"rental_id":132887395,"display_id":"132887395","status":"success","price":1908,"is_cancellable":true,"starts":"2026-09-11T09:30","facility":{"title":"Lot"}}]}}`))
		case "/reservations/132887395/":
			_, _ = w.Write([]byte(`{"data":{"rental_id":132887395,"status":"success","is_cancellable":true}}`))
		case "/reservations/r1/":
			_, _ = w.Write([]byte(`{"data":{"rental_id":1,"display_id":"r1","status":"upcoming","is_cancellable":true}}`))
		case "/search/transient/6698":
			_, _ = w.Write([]byte(`{"result":{"availability":{"available":true},"rates":[{"quote":{"meta":{"quote_token":"tok-1","quote_mac":"113821"},"order":[{"rate_id":"113821","starts":"2026-09-15T09:00:00-07:00","ends":"2026-09-15T17:00:00-07:00","total_price":{"value":2968}}],"total_price":{"value":2968}}}],"facility":{"common":{"id":"6698","title":"Lot 6698","status":"on_sales_allowed"}}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	c := client.New(&auth.Session{RawCookieHeader: "s=1; csrftoken=csrf"})
	c.BaseURL = srv.URL
	c.CraigBaseURL = srv.URL
	c.HTTP = srv.Client()
	return c
}

func runCLIWithHome(t *testing.T, home string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	root := cli.NewRootForTest(&cli.Options{
		HTTP: mockHTTP(t),
		Home: home,
	})
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	code := 0
	if err := root.Execute(); err != nil {
		code = cli.ExitCodeForTest(err)
	}
	return code, stdout.String(), stderr.String()
}

func runCLI(t *testing.T, args ...string) (int, string, string) {
	return runCLIWithHome(t, t.TempDir(), args...)
}

func TestVersion(t *testing.T) {
	code, out, _ := runCLI(t, "--version")
	if code != 0 || out == "" {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestSearchJSON(t *testing.T) {
	code, out, errOut := runCLI(t, "--json", "search", "--lat", "41.88", "--lng", "-87.62", "--starts", "2026-09-11T09:30", "--ends", "2026-09-11T12:30")
	if code != 0 {
		t.Fatalf("code=%d err=%q out=%q", code, errOut, out)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
}

func TestBookPreviewJSON(t *testing.T) {
	code, out, errOut := runCLI(t, "--json", "book", "preview",
		"--facility-id", "6698",
		"--starts", "2026-09-15T09:00",
		"--ends", "2026-09-15T17:00",
		"--city-slug", "san-francisco",
	)
	if code != 0 {
		t.Fatalf("code=%d err=%q out=%q", code, errOut, out)
	}
	var preview struct {
		FacilityID int    `json:"facility_id"`
		Title      string `json:"title"`
		PriceCents int    `json:"price_cents"`
		RateID     string `json:"rate_id"`
		QuoteToken string `json:"quote_token"`
		DryRun     bool   `json:"dry_run"`
	}
	if err := json.Unmarshal([]byte(out), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.FacilityID != 6698 || preview.PriceCents != 2968 || preview.RateID != "113821" || preview.QuoteToken != "tok-1" || !preview.DryRun {
		t.Fatalf("%+v", preview)
	}
	if preview.Title != "Lot 6698" {
		t.Fatalf("title=%q", preview.Title)
	}
}

func TestBookPlaceDryRunCheckoutBody(t *testing.T) {
	code, out, errOut := runCLI(t, "--json", "--dry-run", "book", "place",
		"--facility-id", "6698",
		"--starts", "2026-09-15T09:00",
		"--ends", "2026-09-15T17:00",
		"--city-slug", "san-francisco",
		"--enable-live-booking", "--owner-approved",
		"--confirm", "PLACE SPOTHERO BOOKING",
	)
	if code != 0 {
		t.Fatalf("code=%d err=%q out=%q", code, errOut, out)
	}
	var payload struct {
		DryRun  bool                   `json:"dry_run"`
		Request map[string]any         `json:"request"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.DryRun || payload.Request["currency"] != "usd" {
		t.Fatalf("%+v", payload)
	}
	items := payload.Request["items"].([]any)
	item := items[0].(map[string]any)
	if item["rate_id"] != "113821" || item["quote_token"] != "tok-1" || item["quote_mac"] != "113821" {
		t.Fatalf("item=%v", item)
	}
	ctx := item["item_context"].(map[string]any)
	if ctx["facility"].(float64) != 6698 || ctx["vehicle_profile_id"].(float64) != 37062538 {
		t.Fatalf("context=%v", ctx)
	}
	if payload.Request["use_spothero_credit"] != false {
		t.Fatalf("use_spothero_credit=%v", payload.Request["use_spothero_credit"])
	}
	cards := payload.Request["cards"].([]any)
	if cards[0].(map[string]any)["card_external_id"] != "REDACTED-CARD-UUID" {
		t.Fatalf("cards=%v", payload.Request["cards"])
	}
}

func TestBookPlaceRefusesWithoutGates(t *testing.T) {
	code, _, errOut := runCLI(t, "book", "place", "--facility-id", "1", "--starts", "2026-09-11T09:30", "--ends", "2026-09-11T12:30")
	if code != 2 {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
}

func TestBookPlaceRequiresExactConfirm(t *testing.T) {
	code, _, _ := runCLI(t, "book", "place", "--facility-id", "1", "--starts", "2026-09-11T09:30", "--ends", "2026-09-11T12:30",
		"--enable-live-booking", "--owner-approved", "--confirm", "WRONG")
	if code != 2 {
		t.Fatalf("code=%d", code)
	}
}

func TestCancelRequiresToken(t *testing.T) {
	code, _, _ := runCLI(t, "cancel", "132887395")
	if code != 2 {
		t.Fatalf("code=%d", code)
	}
}

func TestCancelPreviewConfirmAcrossProcesses(t *testing.T) {
	home := t.TempDir()
	code, out, errOut := runCLIWithHome(t, home, "--json", "cancel", "preview", "132887395")
	if code != 0 {
		t.Fatalf("preview code=%d err=%q out=%q", code, errOut, out)
	}
	var preview struct {
		ConfirmToken string `json:"confirm_token"`
	}
	if err := json.Unmarshal([]byte(out), &preview); err != nil {
		t.Fatal(err)
	}
	code, _, errOut = runCLIWithHome(t, home, "--json", "--dry-run", "cancel", "132887395", "--yes", "--confirm", preview.ConfirmToken)
	if code != 0 {
		t.Fatalf("cancel code=%d err=%q", code, errOut)
	}
	code, _, _ = runCLIWithHome(t, home, "--json", "--dry-run", "cancel", "132887395", "--yes", "--confirm", preview.ConfirmToken)
	if code != 2 {
		t.Fatalf("expected single-use token failure, code=%d", code)
	}
}

func TestReservationsListLiveShapeJSON(t *testing.T) {
	code, out, errOut := runCLI(t, "--json", "reservations", "list")
	if code != 0 {
		t.Fatalf("code=%d err=%q out=%q", code, errOut, out)
	}
	var payload struct {
		Reservations []struct {
			ID         string `json:"id"`
			PriceCents int    `json:"price_cents"`
		} `json:"reservations"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Reservations) != 1 || payload.Reservations[0].ID != "132887395" {
		t.Fatalf("%+v", payload.Reservations)
	}
	if payload.Reservations[0].PriceCents != 1908 {
		t.Fatalf("price_cents=%d", payload.Reservations[0].PriceCents)
	}
}

func TestDoctor(t *testing.T) {
	home := t.TempDir()
	root := cli.NewRootForTest(&cli.Options{HTTP: mockHTTP(t), Home: home})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"doctor", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorLivePassesWhenUser401ButReservationsOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search-params/":
			_, _ = w.Write([]byte(`{"data":{"latitude":41.88,"longitude":-87.62,"starts":"2026-09-11T09:30","ends":"2026-09-11T12:30","sort":"distance","sort_order":"asc","distance_lt":1609,"page_info":{"setup":{"city":{"slug":"chicago"}}}}}`))
		case "/reservations/":
			_, _ = w.Write([]byte(`{"meta":{"count":1},"data":{"results":[{"rental_id":1,"price":100,"status":"success"}]}}`))
		case "/user/":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"data":{"errors":[{"code":"not_authenticated","messages":["nope"]}]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	home := t.TempDir()
	if err := auth.SaveSession(home, &auth.Session{RawCookieHeader: "s=1", Source: "test"}); err != nil {
		t.Fatal(err)
	}
	c := client.New(&auth.Session{RawCookieHeader: "s=1"})
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	var out bytes.Buffer
	root := cli.NewRootForTest(&cli.Options{HTTP: c, Home: home})
	root.SetOut(&out)
	root.SetArgs([]string{"doctor", "--live", "--json", "--home", home})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var report struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("doctor should pass: %s", out.String())
	}
}
