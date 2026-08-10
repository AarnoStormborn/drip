# Drip 🩸

A TUI subscription tracker for India — UPI AutoPay (Google Pay) + credit cards (HDFC/ICICI).

Built with Go, [Bubble Tea](https://github.com/charmbracelet/bubbletea), and SQLite (pure Go via `modernc.org/sqlite`, no CGO).

> Status: **milestone 1 (scaffold)** — see `docs/plan.md` for the roadmap and `docs/research.md` for the data-acquisition research.

## Build & run

```sh
make build        # → bin/drip
make run          # TUI
make test         # unit tests
```

The database defaults to `~/.drip/drip.db` (override with `-db` or `DRIP_DB`).
**Never commit it** — `.gitignore` covers `*.db`, `/data/`, and `/inbox/`.

## TUI keys

| Key | Action |
|---|---|
| `1`–`4` | Switch tab (Dashboard / Subscriptions / Import / Reconcile) |
| `tab` / `shift+tab` | Cycle tabs |
| `r` | Refresh from database |
| `q` / `ctrl+c` | Quit |

## Planned CLI

```sh
drip import <folder>   # parse statements, detect subscriptions (milestone 2)
```

## Roadmap (see docs/plan.md)

1. ✅ Scaffold — module, TUI shell, SQLite schema
2. ⏳ Import engine — HDFC/ICICI savings CSV → normalized records
3. ⏳ Detection — recurring heuristics → subscription upserts
4. ⏳ Card PDFs — unlock + extract HDFC/ICICI card statements
5. ⏳ GPay — statement PDF + mandate list ingestion
6. ⏳ TUI polish — dashboard/detail/forms, due-date reminders
7. ⏳ Reconcile view + backfill
