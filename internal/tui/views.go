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

func (a *App) renderSubscriptions() string {
	if len(a.subs) == 0 {
		return cardStyle.Render(
			"Nothing here yet.\n\n" +
				"Once the import pipeline lands (milestone 2), `drip import <folder>` will parse your\n" +
				"statements and auto-create subscriptions. You'll also be able to add one manually.")
	}

	rows := []string{fmt.Sprintf("%-*s %-*s %*s %-*s %s",
		wService, "SERVICE",
		wCycle, "CYCLE",
		wAmount, "AMOUNT",
		wNext, "NEXT DUE",
		headerStyle.Render(fmt.Sprintf("%-*s", wStatus, "STATUS")),
	)}
	for _, s := range a.subs {
		rows = append(rows, fmt.Sprintf("%-*s %-*s %*s %-*s %s",
			wService, truncate(s.Service, wService),
			wCycle, CycleLabel(s.Cycle),
			wAmount, FormatINR(s.Amount),
			wNext, FormatDate(s.NextPaymentDate),
			statusCellPadded(s.Status),
		))
	}
	return strings.Join(rows, "\n")
}

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
		"  • Recurring detection → auto-created subscriptions  — milestone 3",
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

func (a *App) renderReconcile() string {
	return cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		accentStyle.Render("Reconcile — planned (milestone 7)"),
		"",
		mutedStyle.Render(
			"Cross-checks your GPay UPI Autopay mandate list against the ledger:",
			"  • mandates with no matching subscription → add prompt",
			"  • subscriptions missing from the mandate list → flag (maybe cancelled at source)",
			"  • amount / next-due mismatches → flag for review",
			"",
			"The GPay UPI Autopay screen is the authoritative “active subscriptions” source",
			"per NPCI OC-223 — this view turns that list into actionable diffs.",
		),
	))
}
