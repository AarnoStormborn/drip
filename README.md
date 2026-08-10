# Drip 🩸

A terminal-based subscription tracker for India — UPI AutoPay (Google Pay) + credit cards (HDFC/ICICI).

Built with Go, [Bubble Tea](https://github.com/charmbracelet/bubbletea), and SQLite (pure Go via `modernc.org/sqlite`, no CGO).

## Usage

```sh
make build        # → bin/drip
make run          # start the TUI
make test         # run tests
```

### Keys

| Key | Action |
|---|---|
| `1`–`4` | Switch tab (Dashboard / Subscriptions / Import / Reconcile) |
| `tab` / `shift+tab` | Cycle tabs |
| `r` | Refresh from database |
| `q` / `ctrl+c` | Quit |

### Database

The database defaults to `~/.drip/drip.db` (override with `-db <path>` or `DRIP_DB`). It holds personal financial data and is never committed; `.gitignore` covers `*.db`, `/data/`, and `/inbox/`.
