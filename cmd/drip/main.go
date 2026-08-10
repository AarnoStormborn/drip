// Command drip is the entry point: a Bubble Tea TUI for tracking paid
// subscriptions (India: UPI AutoPay + credit cards).
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"drip/internal/db"
	"drip/internal/detect"
	"drip/internal/import"
	"drip/internal/model"
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
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "drip: usage: drip import <dir> — parse statement files in <dir>")
				os.Exit(2)
			}
			runImport(args[1], *dbPath)
			return
		case "detect":
			runDetect(*dbPath)
			return
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

// runImport parses every supported statement file in dir into the DB, runs
// recurring detection on the fresh records, and prints a per-file summary.
func runImport(dir, dbPath string) {
	sqldb, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "drip:", err)
		os.Exit(1)
	}
	defer sqldb.Close()

	ctx := context.Background()
	results, err := importer.ImportDir(ctx, sqldb, dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "drip:", err)
		os.Exit(1)
	}
	if len(results) == 0 {
		fmt.Printf("no statement files (.pdf/.csv/.xlsx) found in %s\n", dir)
		return
	}

	var totalImported, totalDup int64
	for _, r := range results {
		if r.Err != nil {
			fmt.Printf("✗ %-24s %s\n", r.File, r.Err)
			continue
		}
		dr, cr := countSides(r.Records)
		fmt.Printf("✓ %-24s %s: %d records (%d debit / %d credit)", r.File, r.Source, len(r.Records), dr, cr)
		if r.Imported > 0 {
			fmt.Printf(" → %d inserted", r.Imported)
		}
		if r.Duplicates > 0 {
			fmt.Printf(", %d duplicates skipped", r.Duplicates)
		}
		fmt.Println()
		totalImported += r.Imported
		totalDup += r.Duplicates
	}
	fmt.Printf("\n%d new records, %d duplicates skipped\n", totalImported, totalDup)

	if totalImported > 0 {
		printDetection(runDetectOn(sqldb))
	}
}

// runDetect re-scans the whole ledger and reports.
func runDetect(dbPath string) {
	sqldb, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "drip:", err)
		os.Exit(1)
	}
	defer sqldb.Close()
	printDetection(runDetectOn(sqldb))
}

func runDetectOn(sqldb *sql.DB) detect.Result {
	res, err := detect.Run(context.Background(), sqldb)
	if err != nil {
		fmt.Fprintln(os.Stderr, "drip:", err)
		os.Exit(1)
	}
	return res
}

func printDetection(res detect.Result) {
	fmt.Printf("detect: %d recurring subscriptions (%d created, %d refreshed), %d single-charge candidates\n",
		res.Total, res.Created, res.Updated, res.Candidates)
}

// countSides tallies debit (negative) vs credit (positive) records.
func countSides(recs []model.StatementRecord) (dr, cr int) {
	for _, r := range recs {
		if r.Amount < 0 {
			dr++
		} else {
			cr++
		}
	}
	return dr, cr
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
