# spothero-pp-cli

Agent-native [Printing Press](https://github.com/mvanhorn/cli-printing-press) CLI for **SpotHero consumer accounts**. Search parking, list reservations, and **hard-gated** book/cancel — using the same cookie-session APIs as [spothero.com](https://spothero.com), not the partner `X-API-Key` API.

**Author:** [Amandeep Khurana](https://github.com/amansk) (@amansk) · **License:** Apache-2.0

## Install

```bash
go install github.com/amansk/spothero-pp-cli/cmd/spothero-pp-cli@latest
```

Or build from source:

```bash
git clone https://github.com/amansk/spothero-pp-cli
cd spothero-pp-cli
go build -o spothero-pp-cli ./cmd/spothero-pp-cli
```

## Quick start

1. Sign in at [spothero.com](https://spothero.com) in Chrome.
2. Import session cookies:

```bash
spothero-pp-cli auth login --chrome
# or: spothero-pp-cli auth login --cookies-file ./storage-state.json
```

3. Verify setup:

```bash
spothero-pp-cli doctor --json
spothero-pp-cli auth status
spothero-pp-cli account cards --json
```

Saved cards come from `GET /users/{id}/credit-cards/` (live `/users/me/` does not embed them). Output is last4 + ids only — never full PAN or session secrets.

4. Search (address **or** lat/lng). Uses `GET api.spothero.com/v2/search/transient` (live HAR path), not dead `/api/v1/facilities/`:

```bash
spothero-pp-cli search --address "500 Howard St, San Francisco, CA" \
  --starts 2026-09-15T09:00 --ends 2026-09-15T17:00 --json

spothero-pp-cli search --lat 37.788 --lng -122.396 \
  --starts 2026-09-15T09:00 --ends 2026-09-15T17:00 --json
```

Naive `--starts`/`--ends` without timezone are interpreted in the search city's local time (from geocode). Use RFC3339 with `Z` or offset to override.

5. Reservations:

```bash
spothero-pp-cli reservations list --json
spothero-pp-cli reservations get <reservation-id> --json
```

## Booking (hard-gated)

Preview never charges:

```bash
spothero-pp-cli book preview --facility-id 12345 \
  --starts 2026-09-11T09:30 --ends 2026-09-11T12:30 \
  --city-slug chicago --json
```

Uses Craig `GET /v2/search/transient/{facilityId}` (no charge). Pass `--city-slug` from search output for naive local datetimes. Closed or outside-hours windows surface `unavailable_reasons` (e.g. `Outside Hours`) instead of failing later at checkout with a missing `quote_token`.

Live booking requires **all three** gates (exact confirm string). The CLI refreshes the Craig quote, resolves email/phone from `/users/me/`, default card from `/users/{id}/credit-cards/` when needed, vehicle from `/users/{id}/vehicles/`, and POSTs the consumer-checkout body to `/checkout/` (nested `payment.cards[{ card_external_id }]`, `total_price`, `item_context` with `phone_number`/`quote_token`/`rental_source_title`, currency `usd`).

```bash
spothero-pp-cli book place --facility-id 12345 \
  --starts 2026-09-11T09:30 --ends 2026-09-11T12:30 \
  --city-slug chicago \
  --enable-live-booking --owner-approved \
  --confirm "PLACE SPOTHERO BOOKING" --json
```

Inspect the checkout payload without posting:

```bash
spothero-pp-cli book place ... --dry-run --json
```

Overrides: `--card-external-id`, `--card-id`, `--phone`, `--vehicle-profile-id`, `--license-plate`, `--license-plate-state`, `--email`.

## Cancellation (token-gated)

Confirm tokens are stored under your config dir (`confirm-tokens.json`) so preview and confirm can run in separate processes.

```bash
spothero-pp-cli cancel preview <reservation-id> --json
# note confirm_token in output

spothero-pp-cli cancel <reservation-id> --yes --confirm <token> --json
```

Cancel/refund uses `POST /reservations/{id}/refund/` (not `/cancellation/`).

## Global flags

| Flag | Description |
|------|-------------|
| `--json` | Machine-readable JSON output |
| `--agent` | `--json --no-color --no-input` (does **not** imply `--yes`) |
| `--dry-run` | Skip mutating POSTs where supported |
| `--home` | Override config dir (`$SPOTHERO_PP_HOME` or `~/.config/spothero-pp-cli`) |

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage / validation |
| 3 | Not found |
| 4 | Auth |
| 5 | API error |
| 7 | Rate limit / transient |

## Auth details

- Cookies saved to `~/.config/spothero-pp-cli/cookies.json` with mode **0600**.
- Override with `SPOTHERO_COOKIES` (raw Cookie header).
- **`auth status` and `doctor` never print secret values.**

## Development

```bash
go test ./...
go vet ./...
```

See [PLAN.md](./PLAN.md) for consumer API endpoint notes (verified vs unverified).

## Publishing

Intended for eventual `/printing-press-publish` into [mvanhorn/printing-press-library](https://github.com/mvanhorn/printing-press-library) under `library/commerce/spothero/`.
