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

	"github.com/amansk/spothero-pp-cli/internal/exitcode"
)

// BulkTransientSearchRequest is POST /v2/search/bulk/transient body (live confirmed).
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

type craigSearchResponse struct {
	Results []json.RawMessage `json:"results"`
	Next    string            `json:"@next"`
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

func parseCraigSearchResult(raw json.RawMessage) (SearchSpot, error) {
	var wire struct {
		Distance struct {
			WalkingMeters int     `json:"walking_meters"`
			LinearMeters  float64 `json:"linear_meters"`
		} `json:"distance"`
		AveragePrice moneyWire `json:"average_price"`
		Facility     struct {
			Common struct {
				ID        FlexID `json:"id"`
				Title     string `json:"title"`
				Slug      string `json:"slug"`
				Status    string `json:"status"`
				Addresses []craigAddress `json:"addresses"`
			} `json:"common"`
		} `json:"facility"`
		BulkRates []struct {
			Rates []struct {
				Quote struct {
					TotalPrice moneyWire `json:"total_price"`
				} `json:"quote"`
			} `json:"rates"`
		} `json:"bulk_rates"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return SearchSpot{}, fmt.Errorf("craig result: %w", err)
	}
	id, _ := strconv.Atoi(wire.Facility.Common.ID.String())
	priceCents := int(wire.AveragePrice.Value)
	if priceCents == 0 {
		for _, br := range wire.BulkRates {
			for _, rate := range br.Rates {
				if v := int(rate.Quote.TotalPrice.Value); v > 0 {
					priceCents = v
					break
				}
			}
		}
	}
	walkM := wire.Distance.WalkingMeters
	if walkM == 0 && wire.Distance.LinearMeters > 0 {
		walkM = int(wire.Distance.LinearMeters)
	}
	return SearchSpot{
		FacilityID:     id,
		Title:          wire.Facility.Common.Title,
		Slug:           wire.Facility.Common.Slug,
		Address:        formatFacilityAddress(wire.Facility.Common.Addresses),
		DistanceMeters: walkM,
		PriceCents:     priceCents,
		Price:          formatUSD(priceCents),
		Available:      wire.Facility.Common.Status == "on_sales_allowed",
		Status:         wire.Facility.Common.Status,
	}, nil
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
