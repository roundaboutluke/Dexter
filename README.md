# Dexter

Dexter is a Discord/Telegram alarm system for Pokémon GO data — wild spawns, raids, quests, invasions, lures, gyms, nests, weather, max battles, and more — driven by scanner webhooks (e.g. **Golbat**) and backed by MySQL/MariaDB.

Originally a Golang port of **PoracleJS**, Dexter has since grown well beyond parity with significant new features and improvements.

📖 **Full documentation: [dexterWiki](https://roundaboutluke.github.io/dexterWiki/)**

## Key Features

- **Full slash command support** — slash-first UX with autocomplete, guided flows, and localization
- **Scheduling** — active hours with quest digest on return
- **Raid RSVP** — updates existing raid posts with RSVP details
- **Max Battle tracking** — dedicated alerts for max battle encounters
- **Changed Pokémon alerts** — detect when a spawn changes mid-lifespan
- **Quest AR / No-AR** — distinguish and target AR vs No-AR in tracking and digest
- **PvP link helpers** — DTS helpers for PvP site URLs (`{{pvpSlug nameEng}}`)
- **Campfire links** — integrated where relevant
- **Auto data refresh** — game data downloaded on first run, refreshed every 6 hours
- **PoracleJS compatible** — reuse your existing config, DTS templates, and database

## Quick Start

```bash
git clone <repo-url>
cd dexter
go build -o dexter ./cmd/dexter
./dexter
```

On first startup Dexter downloads required game data and creates `config/local.json` from defaults. Edit it with your tokens, DB credentials, and webhook settings.

For full installation instructions (PM2, systemd, Docker), configuration reference, and command documentation, see the **[dexterWiki](https://roundaboutluke.github.io/dexterWiki/)**.

## Migrating from PoracleJS

Dexter is designed to be as close to a drop-in replacement as possible. In many deployments you can reuse your existing PoracleJS config, DTS templates, and database. See the [migration guide](https://roundaboutluke.github.io/dexterWiki/migration/) for details.

Dexter keeps **PoracleJS-compatible version numbering** to maintain compatibility with ecosystem tooling (notably ReactMap).

## Prerequisites

- Go toolchain matching `go.mod`
- MySQL/MariaDB
- A webhook provider (commonly Golbat)
- Discord bot token and/or Telegram bot token

## Related Projects

- PoracleJS (original): https://github.com/KartulUdus/PoracleJS

## Special Thanks

- The Unown# team: https://github.com/UnownHash
