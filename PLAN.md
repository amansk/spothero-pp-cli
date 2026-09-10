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

## Book preview (live verified Sep 2026)

**Dead path:** `GET spothero.com/api/v1/facilities/{id}/rates/` — 404 HTML in live probe. Website also calls this as `ratesSearch` with `include=transient` but it failed without SPA headers.

**Live path:** `GET https://api.spothero.com/v2/search/transient/{facilityId}?starts=<UTC>&ends=<UTC>`

Same Craig headers as search (`x-spothero-spa`, `SpotHero-Version`, Origin/Referer). Cookie optional.

Response: `{ tracking, result }` — singular **`result`**, not `results`. Shape matches one list-search hit: `rates[]`, `availability`, `facility.common`, `options`.

Price from `rates[0].quote.total_price.value` (cents). Rate token for checkout: `rates[0].quote.meta.quote_token` (mapped to CLI `rate_id`).

CLI: `book preview --facility-id <id> --starts ... --ends ... [--city-slug <slug>]`. Naive datetimes use `--city-slug` (from search `params.city_slug`) for local TZ; RFC3339 with `Z`/offset overrides.

## Checkout / book place (HAR-unverified)

**Gated:** `book place` requires `--enable-live-booking --owner-approved --confirm "PLACE SPOTHERO BOOKING"`.

**Inferred path:** `POST spothero.com/api/v1/checkout/` with items `{ facility_id, starts, ends, rate_id }`, payment, currency, email.

**Still unverified in live HAR:**

- Exact checkout body shape (payment method/token fields, quote MAC binding)
- Whether `rate_id` must be `quote_token` + additional `quote_mac` from preview
- Success response / reservation ID mapping
- Error codes for sold-out or expired quote

`book place` uses existing checkout client when all gates pass; do not run live without owner approval. Use `--dry-run` to inspect payload only.

## Session API — `https://spothero.com/api/v1`

| Endpoint | Notes |
|----------|-------|
| `/reservations/` | Cookie auth. `rental_id` int, `price` int cents, `is_cancellable`, nested `facility.title`. |
| `/checkout/` | Gated booking POST (HAR-unverified details above). |
| `/user/` | Often 401 with valid reservation cookies — not used for doctor auth. |

## Timezone

Naive `--starts`/`--ends` → local time in search city IANA zone (from `page_info.setup.city.slug` or `--city-slug`). RFC3339 with `Z`/offset overrides.

## Safety gates

- **Book:** `--enable-live-booking --owner-approved --confirm "PLACE SPOTHERO BOOKING"`
- **Cancel:** `cancel preview` → `cancel --yes --confirm <token>`
