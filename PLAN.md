# SpotHero consumer API plan

Hand-maintained reverse-engineering notes for **spothero-pp-cli**. This is the spec source referenced by `.printing-press.json`.

> **Not the partner API.** Do not use `api.spothero.com/v2` with `X-API-Key` for personal driver reservations. That path is B2B/partner-only.

## Verified (Sep 2026 probes + live smoke)

### Session API — `https://spothero.com/api/v1`

| Endpoint | Method | Auth | Notes |
|----------|--------|------|-------|
| `/search-params/` | GET | No | Geocode address → lat/lng; normalize `starts`/`ends`; city slug for timezone. |
| `/reservations/` | GET | Cookie | Live confirmed. `rental_id` (int) is the reservation id. `price` int cents. `is_cancellable`. `facility.title` nested. |
| `/checkout/` | POST | Cookie | Booking (CLI hard-gated). |
| `/facilities/` | GET | — | **Dead for inventory** — always empty `results` in live smoke (with/without cookies). Do not use for search. |

### Craig consumer inventory — `https://api.spothero.com/v2`

Used by spothero.com `craigSearch` JS client. **No partner `X-API-Key`** in live probes (cookie optional).

| Endpoint | Method | Notes |
|----------|--------|-------|
| `POST /search/bulk/transient?lat=&lon=` | POST | **Live search path.** JSON body with `periods[]` (UTC ISO), `page_size`, `max_distance_meters`, etc. Returns `results[]`, `@next`. |

Required headers (live):

- `Content-Type: application/json`
- `Accept: application/json`
- `Origin: https://spothero.com`
- `Referer: https://spothero.com/search`
- `x-spothero-spa: consumer-web`
- `SpotHero-Version: 2019-08-13`

Example body (500 Howard SF, Tue 9am–5pm PT → UTC):

```json
{
  "periods": [{"starts": "2026-09-15T16:00:00Z", "ends": "2026-09-16T00:00:00Z"}],
  "oversize": false,
  "show_unavailable": false,
  "sort_by": "relevance",
  "include_walking_distance": true,
  "max_distance_meters": 1609,
  "page_size": 25
}
```

Result mapping (CLI): `facility.common.id/title/addresses`, `distance.walking_meters`, `average_price.value` (cents), `facility.common.status` (`on_sales_allowed` → available).

### `/user/` — wrong auth probe (live smoke)

`GET /api/v1/user/` returns **401** with cookies that successfully list reservations. `doctor --live` uses `GET /reservations/?page_size=1` instead.

## CLI mapping

| Command | API |
|---------|-----|
| `search` | `search-params` → `POST api.spothero.com/v2/search/bulk/transient` |
| `reservations list/get` | `GET /reservations/` |
| `book preview` | `GET /facilities/{id}/rates/` (read-only; may need follow-up) |
| `book place` | `POST /checkout/` (gated) |
| `cancel preview/get` | `GET /reservations/{rental_id}` |
| `cancel` | `POST /reservations/{id}/cancellation/` (gated, unverified path) |
| `doctor --live` | `search-params` ping + `reservations?page_size=1` |

## Timezone handling

- `--starts` / `--ends` **with** `Z` or numeric offset → parsed as RFC3339 UTC.
- **Without** offset → interpreted in IANA timezone from search city slug (`page_info.setup.city.slug`), e.g. `san-francisco` → `America/Los_Angeles`. Unknown city → UTC with `timezone_note` in JSON output.

## Auth

- Domain: `spothero.com` cookies forwarded to both v1 session and Craig v2 search.
- Storage: `~/.config/spothero-pp-cli/cookies.json` (0600)
- Override: `SPOTHERO_COOKIES`

## Safety gates

- **Book:** `--enable-live-booking --owner-approved --confirm "PLACE SPOTHERO BOOKING"`
- **Cancel:** `cancel preview <id>` then `cancel <id> --yes --confirm <token>`

## Unverified / non-binding

| Guess | Notes |
|-------|-------|
| `session_id` / `search_id` on Craig search | Present in web JS for some flows; bulk transient works without them in probes |
| `GET /reservations/{rental_id}/` | Single reservation fetch |
| `POST /reservations/{id}/cancellation/` | Cancel path |
