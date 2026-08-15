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
	"drip/internal/mandate"
	"drip/internal/model"
	"drip/internal/tui"
)

// version is overridden at build time via -ldflags "-X main.version=…"
// (see Makefile and .goreleaser.yml).
var version = "dev"

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
		case "mandates":
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "drip: usage: drip mandates import <file> | list")
				os.Exit(2)
			}
			if args[1] == "list" {
				runMandatesList(*dbPath)
			} else if args[1] == "import" {
				if len(args) < 3 {
					fmt.Fprintln(os.Stderr, "drip: usage: drip mandates import <file>")
					os.Exit(2)
				}
				runMandatesImport(args[2], *dbPath)
			} else {
				fmt.Fprintf(os.Stderr, "drip: unknown mandates subcommand %q — use import or list\n", args[1])
				os.Exit(2)
			}
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

// runMandatesImport parses a GPay mandate-list text file into the mandates table.
func runMandatesImport(file, dbPath string) {
	content, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "drip:", err)
		os.Exit(1)
	}
	mandates, errs := mandate.ParseFile(string(content))
	if len(mandates) == 0 && len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "drip: no mandates parsed from", file)
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "  ✗", e)
		}
		os.Exit(1)
	}

	sqldb, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "drip:", err)
		os.Exit(1)
	}
	defer sqldb.Close()

	ctx := context.Background()
	for i := range mandates {
		if _, err := db.CreateMandate(ctx, sqldb, &mandates[i]); err != nil {
			fmt.Fprintln(os.Stderr, "drip:", err)
			os.Exit(1)
		}
	}
	fmt.Printf("✓ %d mandates imported\n", len(mandates))
	for _, e := range errs {
		fmt.Printf("  ✗ %s\n", e)
	}
}

// runMandatesList prints the stored mandates.
func runMandatesList(dbPath string) {
	sqldb, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "drip:", err)
		os.Exit(1)
	}
	defer sqldb.Close()

	mandates, err := db.ListMandates(context.Background(), sqldb, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "drip:", err)
		os.Exit(1)
	}
	if len(mandates) == 0 {
		fmt.Println("no mandates stored — copy your GPay UPI Autopay list into a file and run:")
		fmt.Println("  drip mandates import <file>")
		return
	}
	for _, m := range mandates {
		next := m.NextDebitDate
		if next == "" {
			next = "—"
		}
		fmt.Printf("%-24s %10s  %-11s  next %-12s  %s\n",
			m.Service, formatRupeesCLI(m.Amount), m.Cycle, next, m.Status)
	}
}

func formatRupeesCLI(paise int64) string {
	return fmt.Sprintf("₹%d.%02d", paise/100, paise%100)
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
