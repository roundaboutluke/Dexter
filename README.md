# Dexter

Dexter is a Discord/Telegram alarm system for Pokémon GO data (wild spawns, raids/eggs, quests, invasions, lures, gyms, nests, weather, and more) driven by scanner webhooks (e.g. **Golbat**) and backed by MySQL/MariaDB.

Originally a Golang port of **PoracleJS**, Dexter has since grown well beyond parity with significant new features and improvements.

## Migrating from PoracleJS

Dexter is designed to be as close to a "drop-in" as possible. In many deployments you can **reuse your existing PoracleJS config, DTS templates, and even your existing database**.

That said, always take a backup first and do a test boot: differences in migrations/schema/history between environments can require minor manual adjustments.

## Versioning

Dexter keeps **PoracleJS-compatible version numbering** to maintain compatibility with ecosystem tooling (notably ReactMap).

## What's Different vs PoracleJS

In addition to parity with the core tracking/matching logic, Dexter adds:

- **Full scheduling** (active/quiet hours) and a **quest digest** when returning to active hours.
- **Full `/command`** support (slash-first UX) and **Campfire links** where relevant.
- **Raid post updates with RSVP details** (updates the existing post instead of separate RSVP posts).
- **Safe auto-reload of all data** every 6 hours.
- **Max Battle** tracking/alerts.
- **Spawn "changed Pokémon" alerts** for encounter updates where the Pokémon changes mid-lifespan.
- **Quest AR / No-AR support for tracking** (quests were previously parsed, but you couldn't *distinguish/target* AR vs No-AR in tracking), including digest output for both when applicable.
- **PvP link helpers** for Pokémon names that require special URL formatting (e.g. Mr. Mime → `Mr_Mime` / `mr_mime`).

## Prerequisites

- Go toolchain matching `go.mod` (currently `go 1.25.5`).
- MySQL/MariaDB for Dexter's database.
- A webhook provider (commonly Golbat) configured to POST events to Dexter.
- Discord bot token (and/or Telegram bot token) depending on platform.

## Quick Start (Local / Dev)

```bash
git clone <repo-url>
cd dexter

# Build
go build -o dexter ./cmd/dexter

# Run (first run copies config defaults into config/)
./dexter
```

On first startup, Dexter automatically downloads the required game data files into `util/` (monsters, moves, items, grunts, quests, types, translations, and locale data). This requires internet access. If the download fails, the app will exit with a clear error message.

On subsequent startups, the app refreshes data in the background — if the download fails it continues with the existing files. Data is also refreshed automatically every 6 hours while running.

If you need to regenerate data manually (e.g. for CI/CD or offline development), you can run:

```bash
go run ./cmd/dexter-generate
```

After first run, edit `config/local.json` (created from `config/default.json`) with your tokens, DB credentials, webhook secret, etc.

Notes:

- The local runtime config files (`config/local.json`, `config/dts.json`, `config/partials.json`) are intentionally ignored by git.
- Generated masterdata in `util/` is also ignored by git (automatically downloaded on first run).

## Configuration

Configuration lives under `config/` and follows the same "default + local overrides" pattern as PoracleJS.

- `config/default.json` is the checked-in baseline.
- `config/local.json` is created on first start and should contain your real secrets/overrides.
- Optional: set `NODE_CONFIG_DIR` to point Dexter at a different config directory.

Templates (DTS):

- `config/dts.json` is created on first start from `config/defaults/dts.json`.
- Edit `config/dts.json` to change alert formatting, embeds, maps, etc.
- If you are reusing PoracleJS DTS templates verbatim, you may need **minor formatting tweaks**: Dexter uses a different Handlebars engine and whitespace/newline handling around block helpers can differ slightly.

PvP links (DTS helper):

- Use `{{pvpSlug nameEng}}` for PvPIVs-style URLs (e.g. `Mr_Mime`).
- Use `{{lowercase (pvpSlug nameEng)}}` for PvPoke-style URLs (e.g. `mr_mime`).

Map frontend links (DTS):

- Dexter exposes optional "map URLs" into DTS. For example:
  - `reactMapUrl` (set `general.reactMapURL`)
  - `diademUrl` (set `general.diademURL` to your Diadem base URL; do not include `/map`)

Geocoding providers:

- `geocoding.provider` can be `none`, `nominatim`, `pelias`, or `google`.
- `nominatim`: set `geocoding.providerURL` to your Nominatim base URL.
- `pelias`: set `geocoding.providerURL` to your Pelias API base URL (e.g. `http://localhost:4000`). If your Pelias endpoint requires an API key, set `geocoding.providerKey` (sent as `api_key` query param).
- Pelias advanced options:
  - `geocoding.peliasLayers`: CSV list of layers to request (e.g. `venue,address,street`).
  - `geocoding.peliasPreferredLayer`: if set, Dexter will prefer the first result whose `properties.layer` matches this value.
  - `geocoding.peliasResultSize`: number of results to request from Pelias (helps ensure the preferred layer exists in the returned set).
  - `geocoding.peliasBoundaryCountry`: optional country filter (e.g. `GB`).

Pelias note: Pelias commonly returns much more detailed labels/POI names than Nominatim, so you may want to be more deliberate about how you construct `{{addr}}`.

Reverse geocoding cache:

- Dexter caches geocoding results both in-memory and on disk at `.cache/geocoderCache.json`.
- If you change geocoding provider settings (e.g. switching Nominatim → Pelias, changing Pelias layers, etc), stop Dexter and remove `.cache/geocoderCache.json` before restarting to ensure results reflect the new settings.

Recommended `addressFormat` examples:

- Nominatim-style (simple):
  - `{{streetNumber}} {{{streetName}}}`
- Pelias-style (prefer POI name, then number+street, then street, else Unknown):
  - `{{#if shop}}{{{shop}}}, {{/if}}{{#if streetName}}{{#if streetNumber}}{{streetNumber}} {{/if}}{{{streetName}}}{{else}}Unknown{{/if}}`

Pelias debugging examples:

```bash
# Reverse geocode: show candidate matches with distance/layer
curl -s 'http://localhost:4000/v1/reverse?point.lat=51.878058&point.lon=-0.508682&layers=venue,address,street&boundary.country=GB&size=5' \
  | jq '.features[].properties | {layer,name,label,distance,street,housenumber,locality}'
```

## Building for Production

```bash
go build -trimpath -ldflags "-s -w" -o dexter ./cmd/dexter
```

Run it under your process manager of choice (systemd, pm2, etc.).

## Tileservercache Templates

If you use `tileservercache` for static maps, reference templates are included in `tileservercache_templates/` (mirrors the PoracleJS set, including `poracle-maxbattle.json`).

## Optional: Timezone DB (Per-Location Timezones)

Dexter can optionally use a timezone lookup database for converting times based on alert location (useful if you scan outside your server's local timezone).

- Default lookup base path: `internal/tz/timezone` (various file extensions may exist).
- Config keys:
  - `general.timezoneDbPath` (base path, without extension recommended)
  - `general.timezoneDbType` (default `boltdb`)
  - `general.timezoneDbEncoding` (default `msgpack`)
  - `general.timezoneDbSnappy` (default `true`)

This DB is ignored by git (by design). If you don't provide it, Dexter will fall back to the server timezone.

## Related Projects

- PoracleJS (original): https://github.com/KartulUdus/PoracleJS

## Special Thanks

- The Unown# team: https://github.com/UnownHash
