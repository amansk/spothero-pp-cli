package client

import (
	"fmt"

	"github.com/amansk/spothero-pp-cli/internal/exitcode"
)

// BuildCheckout assembles a consumer checkout body from a fresh Craig quote and account data.
func (c *Client) BuildCheckout(in BookPlaceInput) (CheckoutRequest, RateQuote, error) {
	quote, err := c.GetFacilityRates(FacilityRateQuery{
		FacilityID: in.FacilityID,
		Starts:     in.Starts,
		Ends:       in.Ends,
		CitySlug:   in.CitySlug,
	})
	if err != nil {
		return CheckoutRequest{}, RateQuote{}, err
	}
	if len(quote.Rates) == 0 {
		return CheckoutRequest{}, RateQuote{}, exitcode.NotFoundf("no rates for facility %d", in.FacilityID)
	}
	chosen := quote.Rates[0]
	if in.SelectRateID != "" {
		for _, r := range quote.Rates {
			if r.RateID == in.SelectRateID || r.QuoteToken == in.SelectRateID {
				chosen = r
				break
			}
		}
	}
	if !chosen.Available {
		return CheckoutRequest{}, RateQuote{}, exitcode.NotFoundf("facility %d not available for selected window", in.FacilityID)
	}

	me, err := c.GetMe()
	if err != nil {
		return CheckoutRequest{}, RateQuote{}, exitcode.Usagef("account profile required for checkout: %v", err)
	}
	email, err := c.ResolveEmail(in.Email)
	if err != nil {
		return CheckoutRequest{}, RateQuote{}, exitcode.Usagef("--email required when account profile unavailable: %v", err)
	}

	cardExternalID, err := resolveCardExternalID(me, in.CardExternalID, in.CardID)
	if err != nil {
		return CheckoutRequest{}, RateQuote{}, exitcode.Usagef("%v", err)
	}

	ctx := CheckoutItemContext{
		Facility: in.FacilityID,
		Starts:   chosen.ContextStarts,
		Ends:     chosen.ContextEnds,
	}
	if ctx.Starts == "" || ctx.Ends == "" {
		params := SearchParams{CitySlug: in.CitySlug}
		startLocal, err := formatCheckoutInstant(in.Starts, params)
		if err != nil {
			return CheckoutRequest{}, RateQuote{}, exitcode.Usagef("starts: %v", err)
		}
		endLocal, err := formatCheckoutInstant(in.Ends, params)
		if err != nil {
			return CheckoutRequest{}, RateQuote{}, exitcode.Usagef("ends: %v", err)
		}
		ctx.Starts = startLocal
		ctx.Ends = endLocal
	}

	if in.VehicleProfileID > 0 {
		ctx.VehicleProfileID = in.VehicleProfileID
	} else if in.LicensePlateStr != "" {
		ctx.LicensePlateStr = in.LicensePlateStr
		ctx.LicensePlateState = in.LicensePlateState
	} else if vehicles, verr := c.ListVehicles(me.ID); verr == nil {
		if vehicle, ok := pickDefaultVehicle(vehicles); ok && vehicle.ID > 0 {
			ctx.VehicleProfileID = vehicle.ID
		}
	}
	if chosen.LicensePlateRequired && ctx.VehicleProfileID == 0 && ctx.LicensePlateStr == "" {
		return CheckoutRequest{}, RateQuote{}, exitcode.Usagef("license plate required; add a saved vehicle or pass --vehicle-profile-id / --license-plate")
	}

	if chosen.RateID == "" {
		return CheckoutRequest{}, RateQuote{}, exitcode.APIf("quote missing rate_id")
	}
	if chosen.QuoteToken == "" {
		return CheckoutRequest{}, RateQuote{}, exitcode.APIf("quote missing quote_token")
	}
	quoteMAC := chosen.QuoteMAC
	if quoteMAC == "" {
		quoteMAC = chosen.RateID
	}

	req := CheckoutRequest{
		Currency:          "usd",
		Email:             email,
		UseSpotHeroCredit: false,
		Cards:             []CheckoutCard{{CardExternalID: cardExternalID}},
		Items: []CheckoutItem{{
			ItemType:    "rental",
			Price:       chosen.PriceCents,
			RateID:      chosen.RateID,
			QuoteToken:  chosen.QuoteToken,
			QuoteMAC:    quoteMAC,
			ItemContext: ctx,
		}},
	}
	return req, chosen, nil
}

// ApplyBookPreviewFromRate copies quote fields into preview output.
func ApplyBookPreviewFromRate(p *BookPreview, rate RateQuote) {
	p.RateID = rate.RateID
	p.QuoteToken = rate.QuoteToken
	p.QuoteMAC = rate.QuoteMAC
	p.PriceCents = rate.PriceCents
	p.Price = rate.Price
	p.LicensePlateRequired = rate.LicensePlateRequired
}

func validateCheckoutItem(item CheckoutItem) error {
	if item.ItemType != "rental" {
		return fmt.Errorf("item_type must be rental")
	}
	if item.Price <= 0 || item.RateID == "" || item.QuoteToken == "" || item.QuoteMAC == "" {
		return fmt.Errorf("incomplete checkout item")
	}
	if item.ItemContext.Facility == 0 || item.ItemContext.Starts == "" || item.ItemContext.Ends == "" {
		return fmt.Errorf("incomplete item_context")
	}
	return nil
}
