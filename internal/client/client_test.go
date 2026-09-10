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
	_, err := c.Checkout(client.CheckoutRequest{
		Currency: "usd",
		Email:    "a@b.com",
		UseSpotHeroCredit: false,
		Cards:             []client.CheckoutCard{{CardExternalID: "test-card-uuid"}},
		Items: []client.CheckoutItem{{
			ItemType: "rental", Price: 100, RateID: "1", QuoteToken: "qt", QuoteMAC: "1",
			ItemContext: client.CheckoutItemContext{Facility: 1, Starts: "2026-09-15T09:00:00-07:00", Ends: "2026-09-15T17:00:00-07:00"},
		}},
	})
	if err == nil {
		t.Fatal("expected dry-run error")
	}
}

func TestListReservationsLiveShape(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/reservations/" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		_, _ = w.Write(envelope(map[string]any{
			"results": []map[string]any{{
				"rental_id":      132887395,
				"display_id":     "132887395",
				"price":          1908,
				"status":         "success",
				"is_cancellable": true,
				"facility":       map[string]any{"title": "Garage"},
			}},
		}))
	})
	list, err := c.ListReservations()
	if err != nil {
		t.Fatal(err)
	}
	if list[0].ID != "132887395" || list[0].PriceCents != 1908 {
		t.Fatalf("%+v", list[0])
	}
}

func TestProbeSessionAuthUsesPageSize(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page_size") != "1" {
			t.Fatalf("page_size=%q", r.URL.Query().Get("page_size"))
		}
		_, _ = w.Write(envelope(map[string]any{"results": []any{}}))
	})
	if err := c.ProbeSessionAuth(); err != nil {
		t.Fatal(err)
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
				"page_info": map[string]any{"setup": map[string]any{"city": map[string]any{"slug": "chicago"}}},
			}))
		case "/search/transient":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results":[{"distance":{"walking_meters":100},"rates":[{"quote":{"total_price":{"value":1200}}}],"availability":{"available":true},"facility":{"common":{"id":"99","title":"Lot","status":"on_sales_allowed","addresses":[]}}}]}`))
		default:
			t.Fatalf("unexpected %s", r.URL.Path)
		}
	})
	c.CraigBaseURL = c.BaseURL
	res, err := c.Search(client.SearchQuery{
		Address: "Chicago, IL",
		Starts:  "2026-09-11T09:30",
		Ends:    "2026-09-11T12:30",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Params.Latitude != 41.88 || len(res.Results) != 1 {
		t.Fatalf("params=%+v results=%d", res.Params, len(res.Results))
	}
}
