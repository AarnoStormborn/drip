// Command drip is the entry point: a Bubble Tea TUI for tracking paid
// subscriptions (India: UPI AutoPay + credit cards).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"drip/internal/db"
	"drip/internal/tui"
)

const version = "0.1.0"

func main() {
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "drip v%s — TUI subscription tracker (UPI AutoPay + cards)\n\n", version)
		fmt.Fprintf(out, "Usage: drip [flags] [command]\n\nCommands:\n  import <dir>   (planned, milestone 2) parse statements and detect subscriptions\n\nFlags:\n")
		flag.PrintDefaults()
	}
	dbPath := flag.String("db", defaultDBPath(), "SQLite database path (env DRIP_DB overrides the default)")
	showVer := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVer {
		fmt.Printf("drip v%s\n", version)
		return
	}

	if args := flag.Args(); len(args) > 0 {
		switch args[0] {
		case "import":
			fmt.Fprintln(os.Stderr, "drip: `import` arrives in milestone 2 (import engine + parsers). Use the TUI to browse for now.")
			os.Exit(2)
		default:
			fmt.Fprintf(os.Stderr, "drip: unknown command %q — see `drip -h`\n", args[0])
			os.Exit(2)
		}
	}

	sqldb, err := db.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "drip:", err)
		os.Exit(1)
	}
	defer sqldb.Close()

	p := tea.NewProgram(tui.New(sqldb), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "drip:", err)
		os.Exit(1)
	}
}

// defaultDBPath keeps the database outside the repo (it holds personal
// financial data). Prefers $DRIP_DB, then ~/.drip/drip.db.
func defaultDBPath() string {
	if p := os.Getenv("DRIP_DB"); p != "" {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".drip", "drip.db")
	}
	return "drip.db"
}
