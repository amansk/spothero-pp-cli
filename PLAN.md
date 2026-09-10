# SpotHero consumer API plan

Hand-maintained reverse-engineering notes for **spothero-pp-cli**. This is the spec source referenced by `.printing-press.json`.

> **Not the partner API.** Do not use `api.spothero.com/v2` with `X-API-Key` for personal driver reservations. That path is B2B/partner-only.

## Verified (Sep 2026 probes + live smoke)

| Endpoint | Method | Auth | Notes |
|----------|--------|------|-------|
| `https://spothero.com/api/v1/search-params/` | GET | No | Normalizes address → lat/lng or accepts coordinates + `starts`/`ends`. Returns sort/distance defaults, `ideal_search_distance`, `google_place.place_id`, `page_info.setup.city.id`. |
| `https://spothero.com/api/v1/facilities/` | GET | No* | Returns `{results:[]} / meta.count=0` for SF/Chicago windows where the website shows inventory. Forwards params from `search-params` (`ideal_search_distance`, `place_id`, `city_id`, etc.) — **still empty** in live smoke. |
| `https://spothero.com/api/v1/facilities/{id}/rates/` | GET | Unknown | Referenced in consumer web bundle (`/js/sh-*.js`). |
| `https://spothero.com/api/v1/reservations/` | GET | Cookie | **Live confirmed.** `meta.count` matches account. Reservation id is `rental_id` (int). `price` is int cents. `is_cancellable` bool. Facility title nested under `facility.title`. |
| `https://spothero.com/api/v1/checkout/` | POST | Cookie | Requires `items`, `payment`, `currency`, `email`. |

### Reservations wire shape (live)

```json
{
  "rental_id": 132887395,
  "display_id": "132887395",
  "qrcode_uuid": "...",
  "price": 1908,
  "facility_id": 107496,
  "starts": "2026-09-10T18:00:00-07:00",
  "ends": "2026-09-10T22:00:00-07:00",
  "status": "success",
  "is_cancellable": true,
  "facility": { "id": 107496, "title": "..." }
}
```

CLI maps `rental_id` → public `id` string. There is **no** top-level string `id` field in live payloads.

Response envelope (consistent):

```json
{"meta": {}, "data": { ... }, "notifications": []}
```

Errors appear under `data.errors[]` with `code`, `messages`, optional `field_name`.

## `/user/` — wrong auth probe for cookie sessions (live smoke)

`GET /api/v1/user/` returns **401 `not_authenticated`** even when the same cookie session successfully lists 70+ reservations. **Non-binding guess:** `/user/` may require a different token/cookie subset or is legacy for this auth path.

**CLI behavior:** `doctor --live` treats `GET /reservations/?page_size=1` as the session proof. `/user/` is informational only and does not fail doctor.

## Search inventory gap (HAR pending)

The spothero.com **website** loads inventory via a separate client (`craigSearch` in `/js/sh-*.js`), base URL **`https://api.spothero.com/v2`**, e.g.:

- `GET search/transient?lat=&lon=&starts=&ends=&page_size=&include_walking_distance=true&...`
- Requires `session_id`, `search_id`, `action_id` for some flows (deduced from JS, not fully verified without HAR)

This is **not** the partner `X-API-Key` integration — it is the consumer web inventory API. Unauthenticated probes return results for some queries, but wiring it properly needs a captured HAR with session UUIDs.

**CLI today:** `search` → `search-params` → `GET /api/v1/facilities/` (forwards deduced query params). Empty results include an explicit hint pointing here. **TODO:** capture HAR and either complete `/facilities/` params or add a v2 `search/transient` path with session UUIDs.

## Unverified / non-binding guesses

| Guess | Purpose |
|-------|---------|
| `GET /api/v1/reservations/{rental_id}/` | Fetch one reservation |
| `POST /api/v1/reservations/{id}/cancellation/` | Cancel reservation |
| `POST api.spothero.com/v2/search/bulk/transient` | Bulk search in web bundle (POST body; not wired) |

## CLI mapping

| Command | API |
|---------|-----|
| `search` | `search-params` → `facilities` (inventory gap — see above) |
| `reservations list/get` | `reservations` |
| `book preview` | `facilities/{id}/rates` (read-only) |
| `book place` | `checkout` (gated) |
| `cancel preview/get` | `reservations/{rental_id}` |
| `cancel` | `reservations/{id}/cancellation/` (gated) |
| `doctor --live` | `search-params` ping + `reservations?page_size=1` auth proof |

## Auth

- Domain: `spothero.com`
- Storage: `~/.config/spothero-pp-cli/cookies.json` (0600)
- Override: `SPOTHERO_COOKIES` raw Cookie header
- Import: Chrome (`--chrome`) or Playwright storage-state JSON (`--cookies-file`)

## Safety gates

- **Book:** `--enable-live-booking --owner-approved --confirm "PLACE SPOTHERO BOOKING"`
- **Cancel:** `cancel preview <id>` then `cancel <id> --yes --confirm <token>`
