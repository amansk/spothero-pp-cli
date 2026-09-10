package client

// Envelope is the standard SpotHero consumer API response wrapper.
type Envelope struct {
	Meta          map[string]any `json:"meta"`
	Data          any            `json:"data"`
	Notifications []any          `json:"notifications"`
}

// APIError represents a SpotHero API error item.
type APIError struct {
	Code       string   `json:"code"`
	Messages   []string `json:"messages"`
	FieldName  string   `json:"field_name,omitempty"`
}

// SearchParams holds normalized search parameters from GET /search-params/.
type SearchParams struct {
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Starts        string  `json:"starts"`
	Ends          string  `json:"ends"`
	StartsLocal   string  `json:"starts_local"`
	EndsLocal     string  `json:"ends_local"`
	Sort          string  `json:"sort"`
	SortOrder     string  `json:"sort_order"`
	DistanceLT    float64 `json:"distance_lt"`
	DistanceGT    float64 `json:"distance_gt"`
	Monthly       bool    `json:"monthly"`
	Airport       bool    `json:"airport"`
	SearchString          string  `json:"search_string"`
	SearchURL             string  `json:"search_url"`
	IdealSearchDistance   float64 `json:"ideal_search_distance"`
	GooglePlaceID       string  `json:"-"`
	CityID              int     `json:"-"`
	CitySlug            string  `json:"city_slug,omitempty"`
}

// SearchSpot is one Craig bulk transient search hit mapped for CLI output.
type SearchSpot struct {
	FacilityID     int    `json:"facility_id"`
	Title          string `json:"title"`
	Address        string `json:"address,omitempty"`
	Slug           string `json:"slug,omitempty"`
	DistanceMeters int    `json:"distance_meters"`
	PriceCents     int    `json:"price_cents"`
	Price          string `json:"price"`
	Available      bool   `json:"available"`
	Status         string `json:"status,omitempty"`
}

// SearchResult combines geocode params and Craig inventory results.
type SearchResult struct {
	Params       SearchParams   `json:"params"`
	PeriodsUTC   []SearchPeriod `json:"periods_utc"`
	TimezoneNote string         `json:"timezone_note,omitempty"`
	Results      []SearchSpot   `json:"results"`
	Count        int            `json:"count"`
	NextURL      string         `json:"next_url,omitempty"`
}

// UserProfile is GET /user/ data payload.
type UserProfile struct {
	ID        int    `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// FacilityRateQuery is input for Craig GET /search/transient/{facilityId}.
type FacilityRateQuery struct {
	FacilityID int
	Starts     string
	Ends       string
	CitySlug   string // optional: naive --starts/--ends timezone (from search params)
}

// FacilityRateResult is Craig facility quote mapped for book preview.
type FacilityRateResult struct {
	Rates        []RateQuote    `json:"rates"`
	PeriodsUTC   []SearchPeriod `json:"periods_utc"`
	TimezoneNote string         `json:"timezone_note,omitempty"`
	Title        string         `json:"title,omitempty"`
}

// RateQuote is a facility rate for a time window.
type RateQuote struct {
	FacilityID int    `json:"facility_id"`
	Starts     string `json:"starts"`
	Ends       string `json:"ends"`
	PriceCents int    `json:"price_cents"`
	Price      string `json:"price"`
	Available  bool   `json:"available"`
	RateID     string `json:"rate_id"`
}

// CheckoutRequest is POST /checkout/ body (verified required fields).
type CheckoutRequest struct {
	Items    []CheckoutItem    `json:"items"`
	Payment  CheckoutPayment   `json:"payment"`
	Currency string            `json:"currency"`
	Email    string            `json:"email"`
}

type CheckoutItem struct {
	FacilityID int    `json:"facility_id"`
	Starts     string `json:"starts"`
	Ends       string `json:"ends"`
	RateID     string `json:"rate_id,omitempty"`
}

type CheckoutPayment struct {
	MethodID string `json:"method_id,omitempty"`
	Token    string `json:"token,omitempty"`
}

// BookPreview is local preview output before a live booking.
type BookPreview struct {
	FacilityID   int            `json:"facility_id"`
	Title        string         `json:"title,omitempty"`
	Starts       string         `json:"starts"`
	Ends         string         `json:"ends"`
	PeriodsUTC   []SearchPeriod `json:"periods_utc,omitempty"`
	TimezoneNote string         `json:"timezone_note,omitempty"`
	PriceCents   int            `json:"price_cents"`
	Price        string         `json:"price"`
	RateID       string         `json:"rate_id,omitempty"`
	Email        string         `json:"email,omitempty"`
	DryRun       bool           `json:"dry_run"`
	Message      string         `json:"message"`
}

// CancelPreview is local preview before cancellation.
type CancelPreview struct {
	ReservationID string `json:"reservation_id"`
	Status        string `json:"status"`
	Refundable    bool   `json:"refundable"`
	ConfirmToken  string `json:"confirm_token"`
	Message       string `json:"message"`
}
