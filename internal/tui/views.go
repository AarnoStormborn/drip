package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	wService = 26
	wCycle   = 12
	wAmount  = 14
	wNext    = 14
	wStatus  = 12
)

func (a *App) renderImport() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("THE MONTHLY ROUTINE (~10–15 MIN)"))
	b.WriteString("\n\n")
	steps := []string{
		"1. Download bank statements — HDFC / ICICI savings in CSV (mobile app / netbanking).",
		"2. Copy the GPay UPI Autopay mandate list — Google Pay → Profile → UPI Autopay",
		"   (NPCI OC-223 makes this list complete across UPI apps).",
		"3. Download credit-card statement PDFs — HDFC & ICICI card portals.",
		"4. Optional: GPay app → transaction history → “Get statement” PDF (UPI cross-check).",
		"5. Run  drip import <folder>  — parser + recurring detection.",
	}
	for _, s := range steps {
		b.WriteString(mutedStyle.Render(s))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		accentStyle.Render("Status:"),
		"  • Import engine — HDFC PDF ✓ (CSV/XLSX parsers pending samples)",
		"  • Recurring detection ✓ — auto-creates subscriptions from debits",
		"  • Card PDF unlock + parse (HDFC/ICICI)              — milestone 4",
		"  • GPay statement PDF + mandate list                  — milestone 5",
	)))

	if len(a.imports) > 0 {
		b.WriteString("\n\n")
		b.WriteString(headerStyle.Render("RECENT IMPORTS"))
		b.WriteString("\n")
		for _, im := range a.imports {
			when := strings.TrimSuffix(im.ImportedAt, ":00")
			fmt.Fprintf(&b, "  %-24s %-12s %-8s %s\n",
				truncate(im.File, 24), im.Source, fmt.Sprint(im.Matched)+" rec", when)
		}
	}
	return b.String()
}
