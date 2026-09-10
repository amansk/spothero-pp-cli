package client_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amansk/spothero-pp-cli/internal/auth"
	"github.com/amansk/spothero-pp-cli/internal/client"
	"github.com/amansk/spothero-pp-cli/internal/exitcode"
)

func envelope(data any) []byte {
	b, _ := json.Marshal(client.Envelope{Data: data})
	return b
}

func testClient(t *testing.T, handler http.HandlerFunc) *client.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	sess := &auth.Session{RawCookieHeader: "sessionid=abc"}
	c := client.New(sess)
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	return c
}

func TestSearchParams(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search-params/" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		_, _ = w.Write(envelope(map[string]any{
			"latitude": 41.88, "longitude": -87.62,
			"starts": "2026-09-11T09:30", "ends": "2026-09-11T12:30",
			"sort": "distance", "sort_order": "asc", "distance_lt": 1609.0,
		}))
	})
	params, err := c.GetSearchParams(client.SearchQuery{
		Latitude: 41.88, Longitude: -87.62,
		Starts: "2026-09-11T09:30", Ends: "2026-09-11T12:30",
	})
	if err != nil {
		t.Fatal(err)
	}
	if params.Latitude != 41.88 {
		t.Fatalf("lat=%v", params.Latitude)
	}
}

func TestListFacilities(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/facilities/" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		_, _ = w.Write(envelope(map[string]any{
			"results": []map[string]any{{
				"id": 123, "title": "Garage A", "price": "$12.00", "distance": 100,
			}},
		}))
	})
	list, err := c.ListFacilities(client.SearchParams{
		Latitude: 41.88, Longitude: -87.62,
		Starts: "2026-09-11T09:30", Ends: "2026-09-11T12:30",
		Sort: "distance", SortOrder: "asc", DistanceLT: 1609,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != 123 {
		t.Fatalf("list=%+v", list)
	}
}

func TestNotAuthenticated(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(envelope(map[string]any{
			"errors": []map[string]any{{
				"code": "not_authenticated", "messages": []string{"Authentication credentials were not provided."},
			}},
		}))
	})
	_, err := c.GetUser()
	if err == nil {
		t.Fatal("expected error")
	}
	var ex *exitcode.Error
	if !exitcode.As(err, &ex) || ex.Code != exitcode.Auth {
		t.Fatalf("code=%v err=%v", ex, err)
	}
}

func TestCheckoutDryRun(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not hit network in dry-run")
	})
	c.DryRun = true
	_, err := c.Checkout(client.CheckoutRequest{Currency: "USD", Email: "a@b.com"})
	if err == nil {
		t.Fatal("expected dry-run error")
	}
}

func TestSearchIntegration(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search-params/":
			_, _ = w.Write(envelope(map[string]any{
				"latitude": 41.88, "longitude": -87.62,
				"starts": "2026-09-11T09:30", "ends": "2026-09-11T12:30",
				"sort": "distance", "sort_order": "asc", "distance_lt": 1609.0,
			}))
		case "/facilities/":
			_, _ = w.Write(envelope(map[string]any{"results": []any{}}))
		default:
			t.Fatalf("unexpected %s", r.URL.Path)
		}
	})
	res, err := c.Search(client.SearchQuery{
		Address: "Chicago, IL",
		Starts:  "2026-09-11T09:30",
		Ends:    "2026-09-11T12:30",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Params.Latitude != 41.88 {
		t.Fatalf("params=%+v", res.Params)
	}
}
