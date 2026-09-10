package client

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

//go:embed testdata/search_bulk_transient_live_shape.json
var craigSearchFixture []byte

func TestParseCraigSearchFixture(t *testing.T) {
	var resp craigSearchResponse
	if err := json.Unmarshal(craigSearchFixture, &resp); err != nil {
		t.Fatal(err)
	}
	spots, err := parseCraigSearchResults(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	if len(spots) != 1 {
		t.Fatalf("len=%d", len(spots))
	}
	s := spots[0]
	if s.FacilityID != 86395 || s.Title != "524 Howard St. - Lot" {
		t.Fatalf("%+v", s)
	}
	if s.PriceCents != 4000 || s.Price != "$40.00" {
		t.Fatalf("price=%+v", s)
	}
	if !s.Available || s.DistanceMeters != 63 {
		t.Fatalf("avail/dist=%v %d", s.Available, s.DistanceMeters)
	}
	if !strings.Contains(s.Address, "524 Howard Street") {
		t.Fatalf("address=%q", s.Address)
	}
}

func TestSearchUsesCraigBulkTransient(t *testing.T) {
	var gotPath, gotLat, gotLon string
	var body BulkTransientSearchRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search-params/" {
			_, _ = w.Write([]byte(`{"meta":{},"data":{"latitude":37.788,"longitude":-122.396,"starts":"2026-09-15T09:00","ends":"2026-09-15T17:00","distance_lt":1609,"sort":"distance","sort_order":"asc","page_info":{"setup":{"city":{"slug":"san-francisco"}}}}}`))
			return
		}
		if r.URL.Path == "/search/bulk/transient" {
			gotPath = r.URL.Path
			gotLat = r.URL.Query().Get("lat")
			gotLon = r.URL.Query().Get("lon")
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(craigSearchFixture)
			return
		}
		t.Fatalf("unexpected %s", r.URL.Path)
	}))
	t.Cleanup(srv.Close)

	c := New(nil)
	c.BaseURL = srv.URL
	c.CraigBaseURL = srv.URL
	c.HTTP = srv.Client()

	res, err := c.Search(SearchQuery{
		Latitude: 37.788,
		Longitude: -122.396,
		Starts:   "2026-09-15T09:00",
		Ends:     "2026-09-15T17:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/search/bulk/transient" || gotLat == "" || gotLon == "" {
		t.Fatalf("path=%q lat=%q lon=%q", gotPath, gotLat, gotLon)
	}
	if len(body.Periods) != 1 || body.Periods[0].Starts == "" {
		t.Fatalf("periods=%+v", body.Periods)
	}
	if len(res.Results) != 1 || res.Results[0].FacilityID != 86395 {
		t.Fatalf("%+v", res)
	}
	if res.NextURL == "" {
		t.Fatal("expected next url")
	}
}
