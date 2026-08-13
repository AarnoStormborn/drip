package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"drip/internal/db"
	"drip/internal/detect"
	"drip/internal/model"
)

// mismatch pairs a mandate with the subscription it matched, when their
// amounts disagree.
type mismatch struct {
	Mandate model.Mandate
	Sub     model.Subscription
}

// ReconcileResult is the diff between the GPay mandate list and the ledger.
type ReconcileResult struct {
	Mandates       []model.Mandate      // all stored mandates
	Matched        []model.Mandate      // mandates with a matching subscription
	MissingSubs    []model.Mandate      // mandates with no ledger match
	NotInMandates  []model.Subscription // active upi_autopay subs with no mandate
	AmountMismatch []mismatch           // matched but amounts disagree
}

// computeReconcile diffs active mandates against active subscriptions.
// Matching: normalized service name equality, falling back to descriptor
// containment (e.g. mandate "AXIO" → sub "Paytm" via "upiautopayaxiopaytm…").
func computeReconcile(subs []model.Subscription, mandates []model.Mandate) ReconcileResult {
	var res ReconcileResult
	res.Mandates = mandates

	var activeSubs []model.Subscription
	for _, s := range subs {
		if s.Status == "active" {
			activeSubs = append(activeSubs, s)
		}
	}

	matchedSubs := map[int64]bool{}
	for _, m := range mandates {
		if m.Status != "active" {
			continue
		}
		sub, ok := matchMandate(m, activeSubs)
		if !ok {
			res.MissingSubs = append(res.MissingSubs, m)
			continue
		}
		matchedSubs[sub.ID] = true
		res.Matched = append(res.Matched, m)
		if sub.Amount != 0 && m.Amount != 0 && sub.Amount != m.Amount {
			res.AmountMismatch = append(res.AmountMismatch, mismatch{Mandate: m, Sub: sub})
		}
	}

	for _, s := range activeSubs {
		if s.Rail == "upi_autopay" && !matchedSubs[s.ID] {
			res.NotInMandates = append(res.NotInMandates, s)
		}
	}
	return res
}

func matchMandate(m model.Mandate, subs []model.Subscription) (model.Subscription, bool) {
	key := detect.NormalizeDescriptor(m.Service)
	if key == "" {
		return model.Subscription{}, false
	}
	for _, s := range subs {
		if detect.NormalizeDescriptor(s.Service) == key {
			return s, true
		}
		if len(key) >= 4 {
			var descs []string
			if json.Unmarshal([]byte(s.Descriptors), &descs) == nil {
				for _, d := range descs {
					if strings.Contains(d, key) {
						return s, true
					}
				}
			}
		}
	}
	return model.Subscription{}, false
}

// ── TUI state for the Reconcile tab ───────────────────────────────────────

type recMode int

const (
	recList recMode = iota
	recConfirm
	recForm
)

// handleRecKey routes keys on the Reconcile tab.
func (a *App) handleRecKey(m tea.KeyMsg) bool {
	switch a.recMode {
	case recList:
		switch m.String() {
		case "up", "k":
			if a.recSel > 0 {
				a.recSel--
			}
		case "down", "j":
			if a.recSel < len(a.mandates)-1 {
				a.recSel++
			}
		case "a":
			a.mForm = newMandateForm()
			a.recMode = recForm
		case "d":
			if len(a.mandates) > 0 {
				a.recMode = recConfirm
			}
		case "p":
			if len(a.mandates) > 0 {
				a.promoteMandate(a.mandates[a.recSel])
			}
		default:
			return false
		}
	case recConfirm:
		switch m.String() {
		case "y", "enter":
			a.deleteMandate()
		case "n", "esc":
			a.recMode = recList
		default:
			return false
		}
	case recForm:
		switch m.String() {
		case "esc":
			a.mForm = nil
			a.recMode = recList
		case "ctrl+s":
			a.saveMandate()
		default:
			if a.mForm != nil {
				a.mForm.Update(m)
			}
		}
	}
	return true
}

func (a *App) deleteMandate() {
	m := a.mandates[a.recSel]
	if err := db.DeleteMandate(context.Background(), a.sqldb, m.ID); err != nil {
		a.err = err
	} else {
		a.err = nil
	}
	a.recMode = recList
	a.refresh()
	if a.recSel >= len(a.mandates) {
		a.recSel = len(a.mandates) - 1
	}
	if a.recSel < 0 {
		a.recSel = 0
	}
}

// promoteMandate creates a subscription from a mandate (rail upi_autopay),
// so reconciliation then finds a match.
func (a *App) promoteMandate(m model.Mandate) {
	_, err := db.CreateSubscription(context.Background(), a.sqldb, &model.Subscription{
		Service:         m.Service,
		Amount:          m.Amount,
		Currency:        "INR",
		Cycle:           m.Cycle,
		Rail:            "upi_autopay",
		UPIApp:          m.UPIApp,
		NextPaymentDate: m.NextDebitDate,
		LastPaymentDate: "",
		Status:          "active",
		FirstSeen:       m.NextDebitDate,
	})
	if err != nil {
		a.err = err
		return
	}
	a.err = nil
	a.refresh()
}

func (a *App) saveMandate() {
	m, err := a.mForm.Build()
	if err != nil {
		a.mForm.Err = err.Error()
		return
	}
	if _, err := db.CreateMandate(context.Background(), a.sqldb, &m); err != nil {
		a.mForm.Err = err.Error()
		return
	}
	a.mForm = nil
	a.recMode = recList
	a.refresh()
}

// renderReconcile draws the Reconcile tab.
func (a *App) renderReconcile() string {
	switch a.recMode {
	case recConfirm:
		return a.renderMandateConfirm()
	case recForm:
		if a.mForm == nil {
			a.recMode = recList
			return a.renderReconcile()
		}
		return a.mForm.View()
	default:
		return a.renderReconcileList()
	}
}

func (a *App) renderReconcileList() string {
	if len(a.mandates) == 0 {
		return cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			accentStyle.Render("No mandates yet — GPay UPI Autopay list not imported."),
			"",
			mutedStyle.Render(
				"Your mandate list is the authoritative \"active subscriptions\" record (NPCI OC-223).",
				"To import it:",
				"  • Google Pay → Profile → UPI Autopay → copy the list into a text file, then",
				"  • run:  drip mandates import <file>",
				"  • or press a to add mandates one by one in the TUI.",
			),
		))
	}

	rec := a.rec
	var b strings.Builder

	// summary line
	fmt.Fprintf(&b, " %-24s %s\n", "Mandates stored", fmt.Sprint(len(rec.Mandates)))
	fmt.Fprintf(&b, " %-24s %s\n", "Matched to a subscription", greenStyle.Render(fmt.Sprint(len(rec.Matched))))
	if len(rec.MissingSubs) > 0 {
		fmt.Fprintf(&b, " %-24s %s\n", "Missing subscription", overdueStyle.Render(fmt.Sprint(len(rec.MissingSubs))))
	} else {
		fmt.Fprintf(&b, " %-24s %s\n", "Missing subscription", greenStyle.Render("0"))
	}
	if len(rec.NotInMandates) > 0 {
		fmt.Fprintf(&b, " %-24s %s\n", "Not in mandate list", yellowStyle.Render(fmt.Sprint(len(rec.NotInMandates))))
	} else {
		fmt.Fprintf(&b, " %-24s %s\n", "Not in mandate list", greenStyle.Render("0"))
	}
	if len(rec.AmountMismatch) > 0 {
		fmt.Fprintf(&b, " %-24s %s\n", "Amount mismatch", yellowStyle.Render(fmt.Sprint(len(rec.AmountMismatch))))
	} else {
		fmt.Fprintf(&b, " %-24s %s\n", "Amount mismatch", greenStyle.Render("0"))
	}
	b.WriteString("\n")

	if len(rec.MissingSubs) > 0 {
		b.WriteString(overdueStyle.Render("MISSING SUBSCRIPTION — mandate has no ledger match"))
		b.WriteString("\n")
		for i, m := range rec.MissingSubs {
			fmt.Fprintf(&b, "  %s %-26s %s  %s\n", selMark(i == a.recSel), truncate(m.Service, 26),
				FormatINR(m.Amount), orDash(FormatDate(m.NextDebitDate)))
		}
		b.WriteString("\n")
	}
	if len(rec.AmountMismatch) > 0 {
		b.WriteString(yellowStyle.Render("AMOUNT MISMATCH — mandate vs subscription"))
		b.WriteString("\n")
		for _, mm := range rec.AmountMismatch {
			fmt.Fprintf(&b, "  %-26s mandate %s vs ledger %s\n", truncate(mm.Mandate.Service, 26),
				FormatINR(mm.Mandate.Amount), FormatINR(mm.Sub.Amount))
		}
		b.WriteString("\n")
	}
	if len(rec.NotInMandates) > 0 {
		b.WriteString(yellowStyle.Render("ACTIVE UPI AUTOPAY SUB — not in mandate list"))
		b.WriteString("\n")
		for _, s := range rec.NotInMandates {
			fmt.Fprintf(&b, "  %-26s %s  %s\n", truncate(s.Service, 26), FormatINR(s.Amount), FormatDate(s.NextPaymentDate))
		}
		b.WriteString("\n")
	}

	// mandate roster with match status
	b.WriteString(headerStyle.Render("MANDATES"))
	b.WriteString("\n")
	matchedIDs := map[int64]bool{}
	for _, m := range rec.Matched {
		matchedIDs[m.ID] = true
	}
	for i, m := range a.mandates {
		if m.Status != "active" {
			continue
		}
		status := dimStyle.Render("no match")
		if matchedIDs[m.ID] {
			status = greenStyle.Render("matched")
		}
		fmt.Fprintf(&b, "  %s %-26s %s  %s\n", selMark(i == a.recSel), truncate(m.Service, 26),
			FormatINR(m.Amount), status)
	}

	b.WriteString("\n" + hintStyle.Render("↑/↓ select · a add · p promote → subscription · d delete · esc"))
	return b.String()
}

func (a *App) renderMandateConfirm() string {
	if a.recSel >= len(a.mandates) {
		a.recMode = recList
		return a.renderReconcile()
	}
	m := a.mandates[a.recSel]
	return cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		yellowStyle.Render("Delete mandate "+m.Service+"?"),
		"",
		mutedStyle.Render("The mandate is removed from the list. The subscription (if any) stays."),
		"",
		greenStyle.Render("y")+mutedStyle.Render("es delete  ·  ")+dimStyle.Render("n")+mutedStyle.Render("o / esc cancel"),
	))
}

func selMark(sel bool) string {
	if sel {
		return "▸"
	}
	return " "
}
