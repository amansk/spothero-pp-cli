# SpotHero consumer API plan

Hand-maintained reverse-engineering notes for **spothero-pp-cli**. This is the spec source referenced by `.printing-press.json`.

> **Not the partner API.** Do not use `api.spothero.com/v2` with `X-API-Key` for personal driver reservations. That path is B2B/partner-only.

## Verified (Sep 2026 probes)

| Endpoint | Method | Auth | Notes |
|----------|--------|------|-------|
| `https://spothero.com/api/v1/search-params/` | GET | No | Normalizes address → lat/lng or accepts coordinates + `starts`/`ends`. Returns sort/distance defaults. |
| `https://spothero.com/api/v1/facilities/` | GET | No* | Paginated `results[]`. Query params mirror `search-params` output. *May return empty without full session in some cases. |
| `https://spothero.com/api/v1/facilities/{id}/rates/` | GET | Unknown | Referenced in consumer web bundle (`/js/sh-*.js`). |
| `https://spothero.com/api/v1/user/` | GET | Cookie | 401 `not_authenticated` without session. |
| `https://spothero.com/api/v1/reservations/` | GET, POST | Cookie | List/create reservations. |
| `https://spothero.com/api/v1/checkout/` | POST | Cookie | Requires `items`, `payment`, `currency`, `email`. |

Response envelope (consistent):

```json
{"meta": {}, "data": { ... }, "notifications": []}
```

Errors appear under `data.errors[]` with `code`, `messages`, optional `field_name`.

## Unverified / non-binding guesses

These are inferred from web-app strings or common REST patterns. **Treat as placeholders** until confirmed with a live session:

| Guess | Purpose |
|-------|---------|
| `GET /api/v1/reservations/{id}/` | Fetch one reservation |
| `POST /api/v1/reservations/{id}/cancellation/` | Cancel reservation |
| `POST /api/v1/search/bulk/transient/` | Alternate search path seen in bundle strings (404 as REST path during probes) |

Follow-up once cookies are available: capture HAR from spothero.com search → checkout → cancel and update this table.

## CLI mapping

| Command | API |
|---------|-----|
| `search` | `search-params` → `facilities` |
| `reservations list/get` | `reservations` |
| `book preview` | `facilities/{id}/rates` (read-only) |
| `book place` | `checkout` (gated) |
| `cancel preview/get` | `reservations/{id}` |
| `cancel` | `reservations/{id}/cancellation/` (gated) |

## Auth

- Domain: `spothero.com`
- Storage: `~/.config/spothero-pp-cli/cookies.json` (0600)
- Override: `SPOTHERO_COOKIES` raw Cookie header
- Import: Chrome (`--chrome`) or Playwright storage-state JSON (`--cookies-file`)

## Safety gates

- **Book:** `--enable-live-booking --owner-approved --confirm "PLACE SPOTHERO BOOKING"`
- **Cancel:** `cancel preview <id>` then `cancel <id> --yes --confirm <token>`
