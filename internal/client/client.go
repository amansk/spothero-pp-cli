package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/amansk/spothero-pp-cli/internal/auth"
	"github.com/amansk/spothero-pp-cli/internal/exitcode"
)

// Client calls SpotHero consumer web session APIs.
type Client struct {
	BaseURL    string
	HTTP       *http.Client
	Session    *auth.Session
	UserAgent  string
	DryRun     bool
}

func New(session *auth.Session) *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		Session: session,
		UserAgent: "spothero-pp-cli/0.1.1 (+https://github.com/amansk/spothero-pp-cli)",
	}
}

func (c *Client) url(path string) string {
	return strings.TrimRight(c.BaseURL, "/") + path
}

func (c *Client) newRequest(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, c.url(path), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Origin", "https://spothero.com")
	req.Header.Set("Referer", "https://spothero.com/search")
	if c.Session != nil {
		if ch := c.Session.CookieHeader(); ch != "" {
			req.Header.Set("Cookie", ch)
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
	req, err := c.newRequest(method, path, rdr)
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
	req, err := c.newRequest(method, path, rdr)
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

// Search resolves search parameters then lists facilities.
func (c *Client) Search(q SearchQuery) (*SearchResult, error) {
	params, err := c.GetSearchParams(q)
	if err != nil {
		return nil, err
	}
	facilities, err := c.ListFacilities(params)
	if err != nil {
		return nil, err
	}
	result := &SearchResult{Params: params, Facilities: facilities}
	if len(facilities) == 0 {
		result.EmptyHint = searchEmptyHint
	}
	return result, nil
}

const searchEmptyHint = "GET /api/v1/facilities/ returned zero results. The spothero.com web app uses a separate inventory API (api.spothero.com/v2/search/transient with session/search UUIDs) not yet wired in this CLI — see PLAN.md. Query params forwarded from search-params may still be incomplete pending HAR capture."

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
					ID int `json:"id"`
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
	return out, nil
}

func (c *Client) ListFacilities(p SearchParams) ([]Facility, error) {
	v := url.Values{}
	v.Set("latitude", fmt.Sprintf("%f", p.Latitude))
	v.Set("longitude", fmt.Sprintf("%f", p.Longitude))
	v.Set("starts", p.Starts)
	v.Set("ends", p.Ends)
	v.Set("sort", p.Sort)
	v.Set("sort_order", p.SortOrder)
	if p.DistanceLT > 0 {
		v.Set("distance_lt", fmt.Sprintf("%f", p.DistanceLT))
	}
	v.Set("distance_gt", fmt.Sprintf("%f", p.DistanceGT))
	if p.IdealSearchDistance > 0 {
		v.Set("ideal_search_distance", fmt.Sprintf("%f", p.IdealSearchDistance))
	}
	if p.GooglePlaceID != "" {
		v.Set("place_id", p.GooglePlaceID)
	}
	if p.CityID > 0 {
		v.Set("city_id", strconv.Itoa(p.CityID))
	}
	v.Set("monthly", strconv.FormatBool(p.Monthly))
	v.Set("airport", strconv.FormatBool(p.Airport))
	path := PathFacilities + "?" + v.Encode()
	var resp FacilitiesResponse
	if err := c.doJSON(http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Results, nil
}

func (c *Client) GetFacilityRates(facilityID int, starts, ends string) ([]RateQuote, error) {
	path := fmt.Sprintf(PathFacilityRates, facilityID) + "?" + url.Values{
		"starts": {starts},
		"ends":   {ends},
	}.Encode()
	var raw map[string]any
	if err := c.doJSON(http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	// Normalize flexible API shapes into RateQuote slice.
	b, _ := json.Marshal(raw)
	var list []RateQuote
	if err := json.Unmarshal(b, &list); err == nil {
		return list, nil
	}
	var wrapped struct {
		Results []RateQuote `json:"results"`
	}
	if err := json.Unmarshal(b, &wrapped); err == nil && len(wrapped.Results) > 0 {
		return wrapped.Results, nil
	}
	return nil, nil
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
	var out map[string]any
	if err := c.doJSON(http.MethodPost, PathCheckout, req, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CancelReservation(id string) (map[string]any, error) {
	path := fmt.Sprintf(PathReservationCancel, url.PathEscape(id))
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
