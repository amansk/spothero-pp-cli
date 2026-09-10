package client

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/amansk/spothero-pp-cli/internal/exitcode"
)

// BulkTransientSearchRequest is POST /v2/search/bulk/transient body (fallback).
type BulkTransientSearchRequest struct {
	Periods                []SearchPeriod `json:"periods"`
	Oversize               bool           `json:"oversize"`
	ShowUnavailable        bool           `json:"show_unavailable"`
	SortBy                 string         `json:"sort_by"`
	IncludeWalkingDistance bool           `json:"include_walking_distance"`
	MaxDistanceMeters      int            `json:"max_distance_meters"`
	PageSize               int            `json:"page_size"`
}

func (c *Client) craigURL(path string) string {
	base := c.CraigBaseURL
	if base == "" {
		base = CraigAPIBaseURL
	}
	return strings.TrimRight(base, "/") + path
}

func (c *Client) newCraigRequest(method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	u := c.craigURL(path)
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Origin", "https://spothero.com")
	req.Header.Set("Referer", "https://spothero.com/search")
	req.Header.Set("x-spothero-spa", "consumer-web")
	req.Header.Set("SpotHero-Version", "2019-08-13")
	if c.Session != nil {
		if ch := c.Session.CookieHeader(); ch != "" {
			req.Header.Set("Cookie", ch)
		}
	}
	return req, nil
}

func (c *Client) doCraigJSON(method, path string, query url.Values, body any, out any) error {
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
	req, err := c.newCraigRequest(method, path, query, rdr)
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
	if resp.StatusCode >= 400 {
		var errBody struct {
			Description string `json:"description"`
		}
		_ = json.Unmarshal(raw, &errBody)
		msg := errBody.Description
		if msg == "" {
			msg = truncate(string(raw), 200)
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return exitcode.Authf("%s", msg)
		}
		return exitcode.APIf("HTTP %d: %s", resp.StatusCode, msg)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return exitcode.APIf("decode craig response: %v", err)
	}
	return nil
}

type craigSearchResponse struct {
	Results []json.RawMessage `json:"results"`
	Next    string            `json:"@next"`
}

func newCraigTrackingIDs() (sessionID, searchID, actionID, fingerprint string) {
	return newRandomUUID(), newRandomUUID(), newRandomUUID(), newRandomUUID()
}

func newRandomUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// searchTransientGET is the live HAR path for consumer web inventory search.
func (c *Client) searchTransientGET(lat, lon float64, startsUTC, endsUTC string, maxDist, pageSize int) (craigSearchResponse, error) {
	sessionID, searchID, actionID, fingerprint := newCraigTrackingIDs()
	q := url.Values{}
	q.Set("lat", fmt.Sprintf("%f", lat))
	q.Set("lon", fmt.Sprintf("%f", lon))
	q.Set("starts", startsUTC)
	q.Set("ends", endsUTC)
	q.Set("sort_by", "relevance")
	q.Set("include_walking_distance", "true")
	q.Set("show_unavailable", "false")
	q.Set("initial_search", "true")
	q.Set("action", "LIST_SEARCH")
	q.Set("session_id", sessionID)
	q.Set("search_id", searchID)
	q.Set("action_id", actionID)
	q.Set("fingerprint", fingerprint)
	q.Set("max_distance_meters", strconv.Itoa(maxDist))
	q.Set("page_size", strconv.Itoa(pageSize))
	var resp craigSearchResponse
	if err := c.doCraigJSON(http.MethodGet, PathCraigTransientSearch, q, nil, &resp); err != nil {
		return craigSearchResponse{}, err
	}
	return resp, nil
}

func (c *Client) searchBulkTransient(lat, lon float64, req BulkTransientSearchRequest) (craigSearchResponse, error) {
	q := url.Values{}
	q.Set("lat", fmt.Sprintf("%f", lat))
	q.Set("lon", fmt.Sprintf("%f", lon))
	var resp craigSearchResponse
	if err := c.doCraigJSON(http.MethodPost, PathCraigBulkTransientSearch, q, req, &resp); err != nil {
		return craigSearchResponse{}, err
	}
	return resp, nil
}

// searchInventory prefers GET /search/transient (live HAR); falls back to bulk POST.
func (c *Client) searchInventory(lat, lon float64, startsUTC, endsUTC string, maxDist, pageSize int) (craigSearchResponse, error) {
	resp, err := c.searchTransientGET(lat, lon, startsUTC, endsUTC, maxDist, pageSize)
	if err == nil {
		return resp, nil
	}
	periods := []SearchPeriod{{Starts: startsUTC, Ends: endsUTC}}
	req := BulkTransientSearchRequest{
		Periods:                periods,
		Oversize:               false,
		ShowUnavailable:        false,
		SortBy:                 "relevance",
		IncludeWalkingDistance: true,
		MaxDistanceMeters:      maxDist,
		PageSize:               pageSize,
	}
	return c.searchBulkTransient(lat, lon, req)
}

func parseCraigSearchResults(items []json.RawMessage) ([]SearchSpot, error) {
	out := make([]SearchSpot, 0, len(items))
	for _, raw := range items {
		spot, err := parseCraigSearchResult(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, spot)
	}
	return out, nil
}

type rateQuoteWire struct {
	ID    FlexID `json:"id"`
	Quote struct {
		Meta struct {
			QuoteToken string `json:"quote_token"`
		} `json:"meta"`
		TotalPrice      moneyWire `json:"total_price"`
		AdvertisedPrice moneyWire `json:"advertised_price"`
	} `json:"quote"`
}

func parseCraigSearchResult(raw json.RawMessage) (SearchSpot, error) {
	var wire struct {
		Distance struct {
			WalkingMeters int     `json:"walking_meters"`
			LinearMeters  float64 `json:"linear_meters"`
		} `json:"distance"`
		AveragePrice moneyWire `json:"average_price"`
		Availability struct {
			Available bool `json:"available"`
		} `json:"availability"`
		Rates     []rateQuoteWire `json:"rates"`
		BulkRates []struct {
			Rates []rateQuoteWire `json:"rates"`
		} `json:"bulk_rates"`
		Facility struct {
			Common struct {
				ID        FlexID         `json:"id"`
				Title     string         `json:"title"`
				Slug      string         `json:"slug"`
				Status    string         `json:"status"`
				Addresses []craigAddress `json:"addresses"`
			} `json:"common"`
		} `json:"facility"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return SearchSpot{}, fmt.Errorf("craig result: %w", err)
	}
	id, _ := strconv.Atoi(wire.Facility.Common.ID.String())
	priceCents := pickPriceCents(wire.AveragePrice, wire.Rates, wire.BulkRates)
	walkM := wire.Distance.WalkingMeters
	if walkM == 0 && wire.Distance.LinearMeters > 0 {
		walkM = int(wire.Distance.LinearMeters)
	}
	available := wire.Availability.Available
	if !available && wire.Facility.Common.Status == "on_sales_allowed" && len(wire.Rates) > 0 {
		available = true
	}
	if wire.Facility.Common.Status != "" && wire.Facility.Common.Status != "on_sales_allowed" {
		available = false
	}
	return SearchSpot{
		FacilityID:     id,
		Title:          wire.Facility.Common.Title,
		Slug:           wire.Facility.Common.Slug,
		Address:        formatFacilityAddress(wire.Facility.Common.Addresses),
		DistanceMeters: walkM,
		PriceCents:     priceCents,
		Price:          formatUSD(priceCents),
		Available:      available,
		Status:         wire.Facility.Common.Status,
	}, nil
}

func pickPriceCents(avg moneyWire, rates []rateQuoteWire, bulk []struct {
	Rates []rateQuoteWire `json:"rates"`
}) int {
	if v := int(avg.Value); v > 0 {
		return v
	}
	if len(rates) > 0 {
		if v := int(rates[0].Quote.TotalPrice.Value); v > 0 {
			return v
		}
		if v := int(rates[0].Quote.AdvertisedPrice.Value); v > 0 {
			return v
		}
	}
	for _, br := range bulk {
		for _, rate := range br.Rates {
			if v := int(rate.Quote.TotalPrice.Value); v > 0 {
				return v
			}
			if v := int(rate.Quote.AdvertisedPrice.Value); v > 0 {
				return v
			}
		}
	}
	return 0
}

type moneyWire struct {
	CurrencyCode string  `json:"currency_code"`
	Value        FlexInt `json:"value"`
}

type craigAddress struct {
	StreetAddress string   `json:"street_address"`
	City          string   `json:"city"`
	State         string   `json:"state"`
	PostalCode    string   `json:"postal_code"`
	Types         []string `json:"types"`
}

type craigFacilityResponse struct {
	Result json.RawMessage `json:"result"`
}

// searchTransientFacilityGET is GET /v2/search/transient/{facilityId} (live book preview path).
func (c *Client) searchTransientFacilityGET(facilityID int, startsUTC, endsUTC string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("starts", startsUTC)
	q.Set("ends", endsUTC)
	path := fmt.Sprintf(PathCraigTransientFacility, facilityID)
	var resp craigFacilityResponse
	if err := c.doCraigJSON(http.MethodGet, path, q, nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Result) == 0 {
		return nil, exitcode.NotFoundf("no quote for facility %d", facilityID)
	}
	return resp.Result, nil
}

func parseCraigFacilityRates(raw json.RawMessage, facilityID int, starts, ends string) ([]RateQuote, string, error) {
	var wire struct {
		Availability struct {
			Available bool `json:"available"`
		} `json:"availability"`
		Rates    []rateQuoteWire `json:"rates"`
		Facility struct {
			Common struct {
				Title  string `json:"title"`
				Status string `json:"status"`
			} `json:"common"`
		} `json:"facility"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, "", fmt.Errorf("craig facility result: %w", err)
	}
	available := wire.Availability.Available
	if !available && wire.Facility.Common.Status == "on_sales_allowed" && len(wire.Rates) > 0 {
		available = true
	}
	if wire.Facility.Common.Status != "" && wire.Facility.Common.Status != "on_sales_allowed" {
		available = false
	}
	out := make([]RateQuote, 0, len(wire.Rates))
	for _, rate := range wire.Rates {
		cents := int(rate.Quote.TotalPrice.Value)
		if cents <= 0 {
			cents = int(rate.Quote.AdvertisedPrice.Value)
		}
		rateID := rate.Quote.Meta.QuoteToken
		if rateID == "" {
			rateID = rate.ID.String()
		}
		out = append(out, RateQuote{
			FacilityID: facilityID,
			Starts:     starts,
			Ends:       ends,
			PriceCents: cents,
			Price:      formatUSD(cents),
			Available:  available,
			RateID:     rateID,
		})
	}
	return out, wire.Facility.Common.Title, nil
}

func formatFacilityAddress(addrs []craigAddress) string {
	for _, pref := range []string{"default_vehicle_entrance", "search", "physical"} {
		for _, a := range addrs {
			for _, t := range a.Types {
				if t == pref && a.StreetAddress != "" {
					return strings.TrimSpace(fmt.Sprintf("%s, %s, %s %s", a.StreetAddress, a.City, a.State, a.PostalCode))
				}
			}
		}
	}
	if len(addrs) > 0 && addrs[0].StreetAddress != "" {
		a := addrs[0]
		return strings.TrimSpace(fmt.Sprintf("%s, %s, %s %s", a.StreetAddress, a.City, a.State, a.PostalCode))
	}
	return ""
}
