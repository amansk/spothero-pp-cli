# spothero-pp-cli skill

Use this CLI to search SpotHero consumer parking, list reservations, and (with explicit gates) book or cancel.

## Auth first

```bash
spothero-pp-cli auth login --chrome
spothero-pp-cli doctor --json --live
```

Never log or echo cookie values. `auth status --json` is safe.

## Read commands (no confirmation)

```bash
spothero-pp-cli search --address "..." --starts ... --ends ... --json
spothero-pp-cli reservations list --json
spothero-pp-cli reservations get <id> --json
spothero-pp-cli book preview --facility-id <id> --starts ... --ends ... --json
spothero-pp-cli cancel preview <id> --json
```

## Write commands (require owner approval)

**Book** — all three required:

```bash
spothero-pp-cli book place ... \
  --enable-live-booking --owner-approved \
  --confirm "PLACE SPOTHERO BOOKING"
```

**Cancel** — preview first, then:

```bash
spothero-pp-cli cancel <id> --yes --confirm <token-from-preview>
```

## Agent mode

Pass `--agent` for JSON + non-interactive output. **`--agent` does not bypass book/cancel gates.**

## Errors

Check exit code: 4 = re-auth, 5 = API, 7 = retry later.
