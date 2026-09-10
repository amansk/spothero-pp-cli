package client

// Consumer session API (reservations, checkout, geocode helper).
const DefaultBaseURL = "https://spothero.com/api/v1"

// CraigAPIBaseURL is the consumer web inventory API (craigSearch client in spothero.com JS).
// Uses cookie session optional; does NOT use partner X-API-Key.
const CraigAPIBaseURL = "https://api.spothero.com/v2"

// Endpoint paths on the consumer session API.
const (
	PathSearchParams = "/search-params/"
	// PathFacilityRates is legacy ratesSearch on spothero.com (404/dead); book preview uses Craig instead.
	PathFacilityRates = "/facilities/%d/rates/"
	PathUser          = "/user/"
	PathUsersMe       = "/users/me/"
	PathUserVehicles  = "/users/%d/vehicles/"
	PathReservations  = "/reservations/"
	PathReservation   = "/reservations/%s/"
	PathCheckout      = "/checkout/"
	// PathReservationRefund is live consumer cancel/refund (POST empty body).
	PathReservationRefund = "/reservations/%s/refund/"
)

// ConsumerCheckoutVersion is SpotHero-Version on checkout mutating requests (live HAR).
const ConsumerCheckoutVersion = "2025-04-28"

// Craig consumer search paths (live confirmed Sep 2026 HAR).
const (
	PathCraigTransientSearch     = "/search/transient"
	PathCraigBulkTransientSearch = "/search/bulk/transient"
	// PathCraigTransientFacility is GET quote/rates for one facility (book preview).
	PathCraigTransientFacility = "/search/transient/%d"
)
