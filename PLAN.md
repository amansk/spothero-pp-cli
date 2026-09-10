# SpotHero consumer API plan

Hand-maintained reverse-engineering notes for **spothero-pp-cli**.

> **Not the partner API.** Partner `X-API-Key` on `api.spothero.com/v2` is B2B-only. Consumer web uses the same host without API keys.

## Search inventory (live HAR confirmed Sep 2026)

**Dead path:** `GET spothero.com/api/v1/facilities/` — always empty `results` (with/without cookies). Do not use.

**Live path:** `GET https://api.spothero.com/v2/search/transient`

Query params (required):

| Param | Example |
|-------|---------|
| `lat`, `lon` | From `search-params` geocode |
| `starts`, `ends` | UTC ISO (`2026-09-15T16:00:00Z` = 9am PT) |
| `sort_by` | `relevance` |
| `include_walking_distance` | `true` |
| `show_unavailable` | `false` |
| `initial_search` | `true` |
| `action` | `LIST_SEARCH` |
| `session_id`, `search_id`, `action_id`, `fingerprint` | Fresh UUIDs each request |
| `max_distance_meters` | e.g. `1609` |
| `page_size` | e.g. `25` |

Headers: `Accept`, `Origin: https://spothero.com`, `Referer: https://spothero.com/search`, `x-spothero-spa: consumer-web`, `SpotHero-Version: 2019-08-13`. Cookie optional.

Response: `{ results[], @next, tracking, display }`. Price from `rates[0].quote.total_price.value` (cents, incl. fees). Availability from `availability.available` + `facility.common.status`.

**Fallback:** `POST /v2/search/bulk/transient?lat=&lon=` with JSON `periods[]` if GET fails.

**Geocode helper:** `GET spothero.com/api/v1/search-params/` (address → lat/lon, city slug for timezone).

## Session API — `https://spothero.com/api/v1`

| Endpoint | Notes |
|----------|-------|
| `/reservations/` | Cookie auth. `rental_id` int, `price` int cents, `is_cancellable`, nested `facility.title`. |
| `/checkout/` | Gated booking POST. |
| `/user/` | Often 401 with valid reservation cookies — not used for doctor auth. |

## Timezone

Naive `--starts`/`--ends` → local time in search city IANA zone (from `page_info.setup.city.slug`). RFC3339 with `Z`/offset overrides.

## Safety gates

- **Book:** `--enable-live-booking --owner-approved --confirm "PLACE SPOTHERO BOOKING"`
- **Cancel:** `cancel preview` → `cancel --yes --confirm <token>`
