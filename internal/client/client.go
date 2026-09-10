package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/amansk/spothero-pp-cli/internal/auth"
	"github.com/amansk/spothero-pp-cli/internal/exitcode"
)

// Client calls SpotHero consumer web session APIs.
type Client struct {
	BaseURL      string
	CraigBaseURL string
	HTTP         *http.Client
	Session      *auth.Session
	UserAgent    string
	DryRun       bool
}

func New(session *auth.Session) *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		Session: session,
		UserAgent: "spothero-pp-cli/0.1.8 (+https://github.com/amansk/spothero-pp-cli)",
	}
}

func (c *Client) url(path string) string {
	return strings.TrimRight(c.BaseURL, "/") + path
}

func (c *Client) newRequest(method, path string, body io.Reader) *requestOptions {
	return &requestOptions{
		client: c,
		method: method,
		path:   path,
		body:   body,
	}
}

type requestOptions struct {
	client      *Client
	method      string
	path        string
	body        io.Reader
	referer     string
	spotVersion string
	mutating    bool
}

func (o *requestOptions) withCheckoutHeaders() *requestOptions {
	o.referer = "https://spothero.com/checkout"
	o.spotVersion = ConsumerCheckoutVersion
	o.mutating = true
	return o
}

func (o *requestOptions) build() (*http.Request, error) {
	req, err := http.NewRequest(o.method, o.client.url(o.path), o.body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", o.client.UserAgent)
	req.Header.Set("Origin", "https://spothero.com")
	referer := o.referer
	if referer == "" {
		referer = "https://spothero.com/search"
	}
	req.Header.Set("Referer", referer)
	if o.spotVersion != "" {
		req.Header.Set("SpotHero-Version", o.spotVersion)
	}
	if o.client.Session != nil {
		if ch := o.client.Session.CookieHeader(); ch != "" {
			req.Header.Set("Cookie", ch)
		}
		if o.mutating {
			if csrf := o.client.Session.CSRFToken(); csrf != "" {
				req.Header.Set("X-CSRFToken", csrf)
			}
		}
	}
	return req, nil
}

func (c *Client) doJSON(method, path string, body any, out any) error {
	if c.DryRun && method != http.MethodGet && method != http.MethodHead {
		return exitcode.Usagef("dry-run: would %s %s", method, path)
	}
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	opts := c.newRequest(method, path, rdr)
	if method != http.MethodGet && method != http.MethodHead {
		opts.mutating = true
	}
	req, err := opts.build()
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return exitcode.Transientf("request failed: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return exitcode.Transientf("rate limited (HTTP 429)")
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		if resp.StatusCode >= 400 {
			return exitcode.APIf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
		}
		return exitcode.APIf("decode response: %v", err)
	}
	if err := checkEnvelope(resp.StatusCode, env); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	b, err := json.Marshal(env.Data)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func (c *Client) doJSONData(method, path string, body any) (json.RawMessage, error) {
	if c.DryRun && method != http.MethodGet && method != http.MethodHead {
		return nil, exitcode.Usagef("dry-run: would %s %s", method, path)
	}
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}
	opts := c.newRequest(method, path, rdr)
	if method != http.MethodGet && method != http.MethodHead {
		opts.mutating = true
	}
	req, err := opts.build()
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, exitcode.Transientf("request failed: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, exitcode.Transientf("rate limited (HTTP 429)")
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		if resp.StatusCode >= 400 {
			return nil, exitcode.APIf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
		}
		return nil, exitcode.APIf("decode response: %v", err)
	}
	if err := checkEnvelope(resp.StatusCode, env); err != nil {
		return nil, err
	}
	b, err := json.Marshal(env.Data)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func checkEnvelope(status int, env Envelope) error {
	errs := extractErrors(env.Data)
	if len(errs) > 0 {
		msg := strings.Join(errs, "; ")
		code := strings.ToLower(firstErrorCode(env.Data))
		switch {
		case code == "not_authenticated" || status == http.StatusUnauthorized:
			return exitcode.Authf("%s", msg)
		case code == "not_found" || status == http.StatusNotFound:
			return exitcode.NotFoundf("%s", msg)
		default:
			return exitcode.APIf("%s", msg)
		}
	}
	if status >= 400 {
		return exitcode.APIf("HTTP %d", status)
	}
	return nil
}

func extractErrors(data any) []string {
	m, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := m["errors"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, item := range arr {
		em, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if msgs, ok := em["messages"].([]any); ok {
			for _, m := range msgs {
				out = append(out, fmt.Sprint(m))
			}
		}
	}
	return out
}

func firstErrorCode(data any) string {
	m, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	raw, ok := m["errors"]
	if !ok {
		return ""
	}
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		return ""
	}
	em, ok := arr[0].(map[string]any)
	if !ok {
		return ""
	}
	return fmt.Sprint(em["code"])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Search geocodes via search-params then queries Craig bulk transient inventory.
func (c *Client) Search(q SearchQuery) (*SearchResult, error) {
	params, err := c.GetSearchParams(q)
	if err != nil {
		return nil, err
	}
	periods, loc, err := periodsToUTC(q.Starts, q.Ends, params)
	if err != nil {
		return nil, exitcode.Usagef("%v", err)
	}
	maxDist := int(params.DistanceLT)
	if maxDist <= 0 {
		maxDist = 1609
	}
	startsUTC := periods[0].Starts
	endsUTC := periods[0].Ends
	resp, err := c.searchInventory(params.Latitude, params.Longitude, startsUTC, endsUTC, maxDist, 25)
	if err != nil {
		return nil, err
	}
	spots, err := parseCraigSearchResults(resp.Results)
	if err != nil {
		return nil, err
	}
	note := ""
	if loc != nil && loc != time.UTC {
		note = fmt.Sprintf("Naive --starts/--ends interpreted in %s (from search city). Pass RFC3339 with offset or Z to override.", loc.String())
	} else if !hasExplicitOffset(q.Starts) {
		note = "Naive --starts/--ends interpreted as UTC (city timezone unknown). Pass RFC3339 with offset for local wall times."
	}
	return &SearchResult{
		Params:       params,
		PeriodsUTC:   periods,
		TimezoneNote: note,
		Results:      spots,
		Count:        len(spots),
		NextURL:      resp.Next,
	}, nil
}

func hasExplicitOffset(raw string) bool {
	raw = strings.TrimSpace(raw)
	if strings.HasSuffix(strings.ToUpper(raw), "Z") {
		return true
	}
	return strings.Contains(raw, "+") || strings.Count(raw, "-") > 2
}

// SearchQuery input for search.
type SearchQuery struct {
	Address   string
	Latitude  float64
	Longitude float64
	Starts    string
	Ends      string
}

func (c *Client) GetSearchParams(q SearchQuery) (SearchParams, error) {
	v := url.Values{}
	if q.Address != "" {
		v.Set("search_string", q.Address)
	}
	if q.Latitude != 0 || q.Longitude != 0 {
		v.Set("latitude", fmt.Sprintf("%f", q.Latitude))
		v.Set("longitude", fmt.Sprintf("%f", q.Longitude))
	}
	v.Set("starts", q.Starts)
	v.Set("ends", q.Ends)
	path := PathSearchParams + "?" + v.Encode()
	raw, err := c.doJSONData(http.MethodGet, path, nil)
	if err != nil {
		return SearchParams{}, err
	}
	var wire struct {
		SearchParams
		GooglePlace struct {
			PlaceID string `json:"place_id"`
		} `json:"google_place"`
		PageInfo struct {
			Setup struct {
				City struct {
					ID   int    `json:"id"`
					Slug string `json:"slug"`
				} `json:"city"`
			} `json:"setup"`
		} `json:"page_info"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return SearchParams{}, exitcode.APIf("decode search-params: %v", err)
	}
	out := wire.SearchParams
	out.GooglePlaceID = wire.GooglePlace.PlaceID
	out.CityID = wire.PageInfo.Setup.City.ID
	out.CitySlug = wire.PageInfo.Setup.City.Slug
	return out, nil
}

// GetFacilityRates queries Craig GET /v2/search/transient/{facilityId} for quote/rates.
func (c *Client) GetFacilityRates(q FacilityRateQuery) (FacilityRateResult, error) {
	params := SearchParams{CitySlug: q.CitySlug}
	periods, loc, err := periodsToUTC(q.Starts, q.Ends, params)
	if err != nil {
		return FacilityRateResult{}, exitcode.Usagef("%v", err)
	}
	startsUTC := periods[0].Starts
	endsUTC := periods[0].Ends
	fq, err := c.searchTransientFacilityGET(q.FacilityID, startsUTC, endsUTC)
	if err != nil {
		return FacilityRateResult{}, err
	}
	rates, title, err := parseCraigFacilityRates(fq.Result, q.FacilityID, q.Starts, q.Ends)
	if err != nil {
		return FacilityRateResult{}, exitcode.APIf("%v", err)
	}
	note := timezoneNoteForSearch(q.Starts, loc)
	return FacilityRateResult{
		Rates:        rates,
		PeriodsUTC:   periods,
		TimezoneNote: note,
		Title:        title,
		Tracking:     fq.Tracking,
	}, nil
}

func timezoneNoteForSearch(starts string, loc *time.Location) string {
	if hasExplicitOffset(starts) {
		return ""
	}
	if loc != nil && loc != time.UTC {
		return fmt.Sprintf("Naive --starts/--ends interpreted in %s (from --city-slug or search). Pass RFC3339 with offset or Z to override.", loc.String())
	}
	return "Naive --starts/--ends interpreted as UTC (city timezone unknown). Pass --city-slug from search output or RFC3339 with offset for local wall times."
}

func (c *Client) GetUser() (UserProfile, error) {
	var out UserProfile
	if err := c.doJSON(http.MethodGet, PathUser, nil, &out); err != nil {
		return UserProfile{}, err
	}
	return out, nil
}

func (c *Client) ListReservations() ([]Reservation, error) {
	return c.listReservations("")
}

// ProbeSessionAuth verifies cookie session using a minimal reservations read.
// Live smoke: GET /user/ may 401 even when reservations work — do not use /user/ alone.
func (c *Client) ProbeSessionAuth() error {
	_, err := c.listReservations("1")
	return err
}

func (c *Client) listReservations(pageSize string) ([]Reservation, error) {
	path := PathReservations
	if pageSize != "" {
		path += "?page_size=" + url.QueryEscape(pageSize)
	}
	raw, err := c.doJSONData(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return parseReservationsPayload(raw)
}

func (c *Client) GetReservation(id string) (Reservation, error) {
	path := fmt.Sprintf(PathReservation, url.PathEscape(id))
	raw, err := c.doJSONData(http.MethodGet, path, nil)
	if err != nil {
		return Reservation{}, err
	}
	list, err := parseReservationsPayload(raw)
	if err != nil {
		return Reservation{}, err
	}
	if len(list) == 1 {
		return list[0], nil
	}
	if len(list) > 1 {
		for _, r := range list {
			if r.ID == id || r.DisplayID == id || r.QRCodeUUID == id {
				return r, nil
			}
		}
		return list[0], nil
	}
	var wire reservationWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return Reservation{}, exitcode.APIf("decode reservation: %v", err)
	}
	out := normalizeReservation(wire)
	if out.ID == "" {
		return Reservation{}, exitcode.NotFoundf("reservation %s not found", id)
	}
	return out, nil
}

func (c *Client) Checkout(req CheckoutRequest) (map[string]any, error) {
	if len(req.Items) == 0 {
		return nil, exitcode.Usagef("checkout requires at least one item")
	}
	if err := validateCheckoutItem(req.Items[0]); err != nil {
		return nil, exitcode.Usagef("invalid checkout item: %v", err)
	}
	if c.DryRun {
		return nil, exitcode.Usagef("dry-run: would POST %s", PathCheckout)
	}
	var rdr io.Reader
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	rdr = bytes.NewReader(b)
	reqHTTP, err := c.newRequest(http.MethodPost, PathCheckout, rdr).withCheckoutHeaders().build()
	if err != nil {
		return nil, err
	}
	reqHTTP.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(reqHTTP)
	if err != nil {
		return nil, exitcode.Transientf("request failed: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, exitcode.Transientf("rate limited (HTTP 429)")
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		if resp.StatusCode >= 400 {
			return nil, exitcode.APIf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
		}
		return nil, exitcode.APIf("decode response: %v", err)
	}
	if err := checkEnvelope(resp.StatusCode, env); err != nil {
		return nil, err
	}
	out := map[string]any{}
	if env.Data != nil {
		if m, ok := env.Data.(map[string]any); ok {
			out = m
		}
	}
	return out, nil
}

func (c *Client) RefundReservation(id string) (map[string]any, error) {
	path := fmt.Sprintf(PathReservationRefund, url.PathEscape(id))
	var out map[string]any
	if err := c.doJSON(http.MethodPost, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Ping checks API reachability without auth.
func (c *Client) Ping() error {
	_, err := c.GetSearchParams(SearchQuery{
		Latitude: 41.88,
		Longitude: -87.62,
		Starts:    time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04"),
		Ends:      time.Now().Add(27 * time.Hour).Format("2006-01-02T15:04"),
	})
	return err
}
