package client

// Consumer session API (reservations, checkout, geocode helper).
const DefaultBaseURL = "https://spothero.com/api/v1"

// CraigAPIBaseURL is the consumer web inventory API (craigSearch client in spothero.com JS).
// Uses cookie session optional; does NOT use partner X-API-Key.
const CraigAPIBaseURL = "https://api.spothero.com/v2"

// Endpoint paths on the consumer session API.
const (
	PathSearchParams  = "/search-params/"
	PathFacilityRates = "/facilities/%d/rates/"
	PathUser          = "/user/"
	PathReservations  = "/reservations/"
	PathReservation   = "/reservations/%s/"
	PathCheckout      = "/checkout/"
	// PathReservationCancel is inferred from web-app behavior; live shape may differ.
	PathReservationCancel = "/reservations/%s/cancellation/"
)

// Craig consumer search paths (live confirmed Sep 2026).
const (
	PathCraigBulkTransientSearch = "/search/bulk/transient"
)
