# Drip

A terminal-based subscription tracker. It keeps a ledger of everything you're subscribed to — what you pay, how much, and when it's due — so you never lose track of your recurring spend.

Built with Go, [Bubble Tea](https://github.com/charmbracelet/bubbletea), and SQLite (pure Go via `modernc.org/sqlite`, no CGO).

## Usage

### Install

```sh
# Homebrew (macOS)
brew install aarnostormborn/tap/drip

# or via the Go toolchain
go install github.com/AarnoStormborn/drip/cmd/drip@latest

# or build locally
make build && sudo cp bin/drip /usr/local/bin/
```

### Run

```sh
make build        # → bin/drip
make run          # start the TUI
make test         # run tests

drip import <dir>            # parse statement files in <dir> into the database
drip detect                  # re-scan the ledger for recurring subscriptions
drip mandates import <file>  # load your GPay UPI Autopay list (one per line)
drip mandates list           # show stored mandates
```

### Keys

| Key | Action |
|---|---|
| `1`–`4` | Switch tab (Dashboard / Subscriptions / Import / Reconcile) |
| `tab` / `shift+tab` | Cycle tabs |
| `r` | Refresh from database |
| `q` / `ctrl+c` | Quit |

In the Subscriptions tab:

| Key | Action |
|---|---|
| `↑`/`↓` | Select subscription |
| `enter` | Open detail (price history, descriptors) |
| `e` | Edit — fields: ↑/↓ move, ←/→ cycle, `ctrl+s` save, `esc` cancel |
| `a` | Add a subscription manually (same form) |
| `d` | Delete (confirm with `y`) |

### Database

The database defaults to `~/.drip/drip.db` (override with `-db <path>` or `DRIP_DB`). It holds personal financial data and is never committed; `.gitignore` covers `*.db`, `/data/`, and `/inbox/`.
