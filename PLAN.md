# SpotHero consumer API plan

Hand-maintained reverse-engineering notes for **spothero-pp-cli**.

> **Not the partner API.** Partner `X-API-Key` on `api.spothero.com/v2` is B2B-only. Consumer web uses cookie session on `spothero.com/api/v1` and Craig inventory on `api.spothero.com/v2` (no API key).

## Search inventory (live HAR confirmed Sep 2026)

**Dead path:** `GET spothero.com/api/v1/facilities/` — always empty `results`. Do not use.

**Live path:** `GET https://api.spothero.com/v2/search/transient`

Query params: `lat`, `lon`, UTC `starts`/`ends`, `action=LIST_SEARCH`, tracking UUIDs, etc.

Headers: `x-spothero-spa: consumer-web`, `SpotHero-Version: 2019-08-13`, Origin/Referer.

**Geocode helper:** `GET spothero.com/api/v1/search-params/` (city slug for timezone).

## Book preview (live verified Sep 2026)

**Live path:** `GET https://api.spothero.com/v2/search/transient/{facilityId}?starts=<UTC>&ends=<UTC>`

Response: `{ tracking, result }` with `rates[].quote`:

| Field | Checkout use |
|-------|----------------|
| `quote.order[0].rate_id` | Item `rate_id` (numeric string) |
| `quote.meta.quote_token` | Item `quote_token` (JWT-like) |
| `quote.meta.quote_mac` | Item `quote_mac` (often same as rate_id) |
| `quote.order[0].starts/ends` | `item_context.starts/ends` (RFC3339 with offset) |
| `quote.total_price.value` | Item `price` (cents) |
| `facility.requirements.license_plate` | Requires vehicle or ad-hoc plate in `item_context` |

**Dead path:** `GET /api/v1/facilities/{id}/rates/` (404 HTML).

## Checkout / book place (live consumer-checkout shape)

**Path:** `POST https://spothero.com/api/v1/checkout/`

**Headers (mutating):** session cookies, `X-CSRFToken` (from `csrftoken` cookie), `SpotHero-Version: 2025-04-28`, `Origin`, `Referer: https://spothero.com/checkout`.

**Body shape:**

```json
{
  "currency": "usd",
  "email": "user@example.com",
  "use_spothero_credit": false,
  "cards": [{ "card_external_id": "REDACTED-CARD-UUID" }],
  "items": [{
    "item_type": "rental",
    "price": 2968,
    "rate_id": "113821",
    "quote_token": "…",
    "quote_mac": "113821",
    "item_context": {
      "facility": 6698,
      "starts": "2026-09-15T09:00:00-07:00",
      "ends": "2026-09-15T17:00:00-07:00",
      "vehicle_profile_id": 37062538
    }
  }]
}
```

**Account discovery (reservation-auth authoritative):**

| Endpoint | Use |
|----------|-----|
| `GET /users/me/` | Email, `credit_cards[]` with `card_external_id` + `card_id`, default card |
| `GET /users/{id}/vehicles/` | Default saved vehicle → `vehicle_profile_id` |
| `GET /user/` | Fallback email only; often 401 while reservations work |

**License plate:** when required, saved vehicles use `item_context.vehicle_profile_id`; ad-hoc uses `license_plate_str` + optional `license_plate_state`.

**Safety gates (unchanged):** `--enable-live-booking --owner-approved --confirm "PLACE SPOTHERO BOOKING"`

## Cancel / refund (live verified Sep 2026)

**Dead path:** `POST /api/v1/reservations/{id}/cancellation/` (404 live).

**Live path:** `POST https://spothero.com/api/v1/reservations/{id}/refund/` with empty body → `{ totalRefunded, status: cancelled/refund }`.

**Confirm tokens:** persisted to `{config}/confirm-tokens.json` (0600) with 15m TTL so `cancel preview` → separate-process `cancel --confirm` works.

## Session API — `https://spothero.com/api/v1`

| Endpoint | Notes |
|----------|-------|
| `/reservations/` | Cookie auth. `rental_id` int, `price` int cents, `is_cancellable`. |
| `/checkout/` | Consumer checkout POST (shape above). |
| `/users/me/` | Preferred profile for checkout assembly. |
| `/user/` | Often 401 — not used alone for doctor auth. |

## Timezone

Naive `--starts`/`--ends` → local time via `--city-slug` or search city slug. RFC3339 with `Z`/offset overrides. Craig queries use UTC; checkout `item_context` uses offset times from quote order or local formatting.

## Safety gates

- **Book:** `--enable-live-booking --owner-approved --confirm "PLACE SPOTHERO BOOKING"`
- **Cancel:** `cancel preview` → `cancel --yes --confirm <token>` (token on disk, single-use)
