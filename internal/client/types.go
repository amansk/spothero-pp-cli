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
	SearchString  string  `json:"search_string"`
	SearchURL     string  `json:"search_url"`
}

// Facility is a parking location search result.
type Facility struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Address     string  `json:"address"`
	Distance    float64 `json:"distance"`
	PriceCents  int     `json:"price_cents"`
	Price       string  `json:"price"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Available   bool    `json:"available"`
	Slug        string  `json:"slug"`
}

// FacilitiesResponse is GET /facilities/ data payload.
type FacilitiesResponse struct {
	Results []Facility `json:"results"`
}

// Reservation is a consumer parking reservation.
type Reservation struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	FacilityID      int    `json:"facility_id"`
	FacilityTitle   string `json:"facility_title"`
	Starts          string `json:"starts"`
	Ends            string `json:"ends"`
	ConfirmationCode string `json:"confirmation_code"`
	PriceCents      int    `json:"price_cents"`
	Price           string `json:"price"`
	Barcode         string `json:"barcode"`
	Cancellable     bool   `json:"cancellable"`
}

// ReservationsResponse is GET /reservations/ data payload.
type ReservationsResponse struct {
	Results []Reservation `json:"results"`
}

// UserProfile is GET /user/ data payload.
type UserProfile struct {
	ID        int    `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
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
	FacilityID  int    `json:"facility_id"`
	Starts      string `json:"starts"`
	Ends        string `json:"ends"`
	PriceCents  int    `json:"price_cents"`
	Price       string `json:"price"`
	RateID      string `json:"rate_id,omitempty"`
	Email       string `json:"email,omitempty"`
	DryRun      bool   `json:"dry_run"`
	Message     string `json:"message"`
}

// CancelPreview is local preview before cancellation.
type CancelPreview struct {
	ReservationID string `json:"reservation_id"`
	Status        string `json:"status"`
	Refundable    bool   `json:"refundable"`
	ConfirmToken  string `json:"confirm_token"`
	Message       string `json:"message"`
}
