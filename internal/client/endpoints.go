package client

// Consumer API base (verified via unauthenticated probes, Sep 2026).
// NOT the partner B2B API at api.spothero.com/v2.
const DefaultBaseURL = "https://spothero.com/api/v1"

// Endpoint paths on the consumer session API.
const (
	PathSearchParams  = "/search-params/"
	PathFacilities    = "/facilities/"
	PathFacilityRates = "/facilities/%d/rates/"
	PathUser          = "/user/"
	PathReservations  = "/reservations/"
	PathReservation   = "/reservations/%s/"
	PathCheckout      = "/checkout/"
	// PathReservationCancel is inferred from web-app behavior; live shape may differ.
	PathReservationCancel = "/reservations/%s/cancellation/"
)
