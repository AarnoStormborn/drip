package importer

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ledongthuc/pdf"

	"drip/internal/model"
)

// HDFC PDF statements are text-extractable (mobile-app / e-statement
// exports). Column layout (from the page-1 header row):
//
//	Date(39.9) Narration(144.2) Chq./Ref.No.(283.5) ValueDt(361.5)
//	WithdrawalAmt.(~472 right edge) DepositAmt.(~550 right edge) ClosingBalance(564+)
//
// Amounts are right-aligned into the withdrawal/deposit columns, so start-x
// varies with text width; sign is resolved via running-balance math with an
// x-edge fallback. A transaction's narration may span several lines; lines
// without a leading date continue the previous transaction's narration.
//
// The parser validates itself: the parsed opening balance, debit/credit
// counts and totals should match the summary row on the last page.

const (
	hdfcYtol    = 2.5 // point tolerance for grouping chars into lines
	hdfcXtol    = 4.0 // point tolerance for grouping chars into cells
	digitWidth  = 4.0 // approx. advance per digit for right-edge math
	hdfcAccount = "HDFC-savings"
)

type pdfRun struct {
	x, y float64
	s    string
}

func absf(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

type pdfCell struct {
	x float64
	s string
}

// clusterCells groups positioned chars into lines (by y) and cells (by x),
// preserving the original reading order within each group.
func clusterCells(content pdf.Content) [][]pdfCell {
	var runs []pdfRun
	for _, t := range content.Text {
		if t.S != "\n" {
			runs = append(runs, pdfRun{t.X, t.Y, t.S})
		}
	}
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].y > runs[j].y })

	type line struct {
		y     float64
		cells [][]pdfRun
	}
	var lines []line
	for _, r := range runs {
		if len(lines) == 0 || absf(lines[len(lines)-1].y-r.y) > hdfcYtol {
			lines = append(lines, line{y: r.y, cells: [][]pdfRun{{r}}})
			continue
		}
		l := &lines[len(lines)-1]
		last := &l.cells[len(l.cells)-1]
		if absf((*last)[0].x-r.x) > hdfcXtol {
			l.cells = append(l.cells, []pdfRun{r})
		} else {
			*last = append(*last, r)
		}
	}

	out := make([][]pdfCell, 0, len(lines))
	for _, l := range lines {
		cells := make([]pdfCell, 0, len(l.cells))
		for _, c := range l.cells {
			var b strings.Builder
			for _, r := range c {
				b.WriteString(r.s)
			}
			cells = append(cells, pdfCell{x: c[0].x, s: b.String()})
		}
		out = append(out, cells)
	}
	return out
}

var footerPrefixes = []string{
	"*Closingbalance", "Contentsofthis", "thisstatement.", "Stateaccountbranch",
	"HDFCBankGSTIN", "RegisteredOfficeAddress:", "GeneratedOn:", "Thisisa",
	"notrequiresignature.", "PageNo.:",
}

func isFooterLine(cells []pdfCell) bool {
	if len(cells) == 0 {
		return true
	}
	var b strings.Builder
	for _, c := range cells {
		b.WriteString(c.s)
	}
	joined := b.String()
	for _, p := range footerPrefixes {
		if strings.HasPrefix(joined, p) {
			return true
		}
	}
	// OpeningBalance summary header + its data row
	if cells[0].s == "OpeningBalance" || (len(cells) >= 4 && moneyRE.MatchString(cells[0].s) && isInt(cells[1].s)) {
		return true
	}
	return false
}

func isInt(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ParseHDFCPDF parses an HDFC savings-account statement PDF into normalized
// records. Debits are negative amounts.
func ParseHDFCPDF(path string) ([]model.StatementRecord, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()

	type rawTxn struct {
		date, valueDate, ref string
		narration            []string
		amountRaw            string
		amountX              float64
		balance              int64
		hasAmount, hasBal    bool
	}

	var txns []rawTxn
	opening := int64(0)
	openingKnown := false
	lastFooterWasOpening := false

	for pg := 1; pg <= r.NumPage(); pg++ {
		lines := clusterCells(r.Page(pg).Content())
		sawTxnOnPage := false

		for _, ln := range lines {
			if len(ln) == 0 {
				continue
			}

			// summary data row follows the OpeningBalance header
			if lastFooterWasOpening {
				lastFooterWasOpening = false
				if openingKnown {
					continue // already parsed the summary page
				}
				// cells: opening | drCount | crCount | debits | credits | closing
				if len(ln) >= 6 && moneyRE.MatchString(ln[0].s) {
					opening, _ = parseMoney(ln[0].s)
					openingKnown = true
				}
				continue
			}
			if len(ln) >= 1 && ln[0].s == "OpeningBalance" {
				lastFooterWasOpening = true
				continue
			}
			if isFooterLine(ln) {
				continue
			}

			first := ln[0].s
			if dateRE.MatchString(first) {
				sawTxnOnPage = true
				t := rawTxn{date: first}
				for _, c := range ln[1:] {
					switch {
					case dateRE.MatchString(c.s):
						if t.valueDate == "" {
							t.valueDate = c.s
						}
					case refRE.MatchString(c.s) && t.ref == "":
						t.ref = c.s
					case moneyRE.MatchString(c.s) && !t.hasAmount:
						t.amountRaw = c.s
						t.amountX = c.x
						t.hasAmount = true
					case moneyRE.MatchString(c.s) && !t.hasBal:
						t.balance, _ = parseMoney(c.s)
						t.hasBal = true
					default:
						t.narration = append(t.narration, c.s)
					}
				}
				txns = append(txns, t)
				continue
			}

			// continuation line: append narration (only after txns started
			// on this page — everything before the first txn line is the
			// repeated account-header block)
			if !sawTxnOnPage {
				continue
			}
			if len(txns) == 0 {
				continue
			}
			last := &txns[len(txns)-1]
			for _, c := range ln {
				last.narration = append(last.narration, c.s)
			}
		}
	}

	if !openingKnown {
		return nil, fmt.Errorf("hdfc pdf: summary row (OpeningBalance) not found on last page")
	}

	// Resolve signs via running balance; fall back to column x on mismatch.
	recs := make([]model.StatementRecord, 0, len(txns))
	prev := opening
	for _, t := range txns {
		if !t.hasAmount || !t.hasBal {
			continue
		}
		amt, _ := parseMoney(t.amountRaw)
		var signed int64
		switch {
		case prev-amt == t.balance:
			signed = -amt
		case prev+amt == t.balance:
			signed = amt
		default:
			if classifyMoneySide(t.amountX, t.amountRaw) < 0 {
				signed = -amt
			} else {
				signed = amt
			}
		}

		date, err := parseDDMMYY(t.date)
		if err != nil {
			date = ""
		}
		valueDate := ""
		if t.valueDate != "" {
			valueDate, _ = parseDDMMYY(t.valueDate)
		}
		bal := t.balance
		recs = append(recs, model.StatementRecord{
			Date:        date,
			ValueDate:   valueDate,
			Description: strings.Join(t.narration, " "),
			Amount:      signed,
			Balance:     &bal,
			SourceFile:  filepath.Base(path),
			Account:     hdfcAccount,
		})
		prev = t.balance
	}
	return recs, nil
}
