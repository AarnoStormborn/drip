package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"drip/internal/db"
	"drip/internal/model"
)

// subsMode is the interaction state of the Subscriptions tab.
type subsMode int

const (
	subsList subsMode = iota
	subsDetail
	subsForm
	subsConfirm
)

var (
	selStyle  = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	dueToday  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	infoLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	hintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
)

// handleSubsKey routes keys while on the Subscriptions tab (list/detail/confirm).
func (a *App) handleSubsKey(m tea.KeyMsg) bool {
	switch a.subsMode {
	case subsList:
		switch m.String() {
		case "up", "k":
			if a.subsSel > 0 {
				a.subsSel--
			}
		case "down", "j":
			if a.subsSel < len(a.subs)-1 {
				a.subsSel++
			}
		case "enter", "l", "right":
			if len(a.subs) > 0 {
				a.openDetail(a.subsSel)
			}
		case "e":
			if len(a.subs) > 0 {
				a.openForm(a.subs[a.subsSel])
			}
		case "d":
			if len(a.subs) > 0 {
				a.subsMode = subsConfirm
			}
		default:
			return false
		}
	case subsDetail:
		switch m.String() {
		case "esc", "b", "backspace", "h", "left":
			a.subsMode = subsList
		case "e":
			a.openForm(a.subs[a.subsSel])
		case "d":
			a.subsMode = subsConfirm
		default:
			return false
		}
	case subsConfirm:
		switch m.String() {
		case "y", "enter":
			a.deleteSelected()
		case "n", "esc":
			a.subsMode = subsDetail
		default:
			return false
		}
	}
	return true
}

func (a *App) openDetail(idx int) {
	a.subsSel = idx
	a.subsMode = subsDetail
	s := a.subs[idx]
	hist, err := db.ListPriceHistory(context.Background(), a.sqldb, s.ID)
	if err != nil {
		a.err = err
	} else {
		a.err = nil
	}
	a.selHistory = hist
}

func (a *App) openForm(s model.Subscription) {
	a.form = newEditForm(s)
	a.subsMode = subsForm
}

func (a *App) deleteSelected() {
	s := a.subs[a.subsSel]
	if err := db.DeleteSubscription(context.Background(), a.sqldb, s.ID); err != nil {
		a.err = err
	} else {
		a.err = nil
	}
	a.subsMode = subsList
	a.refresh()
	if a.subsSel >= len(a.subs) {
		a.subsSel = len(a.subs) - 1
	}
	if a.subsSel < 0 {
		a.subsSel = 0
	}
}

// renderSubscriptions draws the tab according to its interaction mode.
func (a *App) renderSubscriptions() string {
	switch a.subsMode {
	case subsDetail:
		if a.subsSel >= len(a.subs) {
			a.subsSel = 0
			a.subsMode = subsList
			return a.renderSubscriptions()
		}
		return a.renderDetail(a.subs[a.subsSel])
	case subsForm:
		if a.form == nil {
			a.subsMode = subsDetail
			return a.renderSubscriptions()
		}
		return a.form.View()
	case subsConfirm:
		return a.renderConfirm()
	default:
		return a.renderSubsList()
	}
}

func (a *App) renderSubsList() string {
	if len(a.subs) == 0 {
		return cardStyle.Render(
			"Nothing here yet.\n\n" +
				"Run `drip import <dir>` to parse statements and auto-create subscriptions,\n" +
				"or `drip detect` to re-scan the ledger.")
	}

	rows := []string{fmt.Sprintf("%-*s %-*s %*s %-*s %s",
		wService, "SERVICE",
		wCycle, "CYCLE",
		wAmount, "AMOUNT",
		wNext, "NEXT DUE",
		headerStyle.Render(fmt.Sprintf("%-*s", wStatus, "STATUS")),
	)}

	today := time.Now().Format("2006-01-02")
	for i, s := range a.subs {
		sel := i == a.subsSel
		next := FormatDate(s.NextPaymentDate)
		marker := " "
		if s.NextPaymentDate == today {
			marker = "⚠"
			next = dueToday.Render(next)
		}
		row := fmt.Sprintf("%s%-*s %-*s %*s %-*s %s",
			marker,
			wService-1, truncate(s.Service, wService-1),
			wCycle, CycleLabel(s.Cycle),
			wAmount, FormatINR(s.Amount),
			wNext, next,
			statusCellPadded(s.Status),
		)
		if sel {
			row = selStyle.Render(row)
		}
		rows = append(rows, row)
	}

	body := strings.Join(rows, "\n")
	if a.err != nil {
		body += "\n\n" + errorStyle.Render("⚠ "+a.err.Error())
	}
	return body + "\n\n" + hintStyle.Render("↑/↓ select · enter detail · e edit · d delete")
}

func (a *App) renderConfirm() string {
	if a.subsSel >= len(a.subs) {
		a.subsMode = subsList
		return a.renderSubscriptions()
	}
	s := a.subs[a.subsSel]
	return cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		yellowStyle.Render("Delete "+s.Service+"?"),
		"",
		mutedStyle.Render("This removes the subscription and its price history."),
		"",
		greenStyle.Render("y")+mutedStyle.Render("es delete  ·  ")+dimStyle.Render("n")+mutedStyle.Render("o / esc cancel"),
	))
}

func (a *App) renderDetail(s model.Subscription) string {
	var b strings.Builder
	b.WriteString(headerStyle.Render(s.Service))
	b.WriteString("  " + statusLabelStyled(s.Status))
	b.WriteString("\n\n")

	pairs := [][2]string{
		{"Amount", FormatINR(s.Amount) + " / " + CycleLabel(s.Cycle)},
		{"Category", orDash(s.Category)},
		{"Rail", orDash(s.Rail)},
		{"Issuer / card", joinNonEmpty(orDash(s.Issuer), orDash(s.CardLast4))},
		{"Mandate ID", orDash(s.MandateID)},
		{"Next payment", orDash(FormatDate(s.NextPaymentDate))},
		{"Last payment", orDash(FormatDate(s.LastPaymentDate))},
		{"First seen", orDash(FormatDate(s.FirstSeen))},
		{"Created", orDash(s.CreatedAt)},
	}
	for _, p := range pairs {
		fmt.Fprintf(&b, "  %-16s %s\n", infoLabel.Render(p[0]), p[1])
	}
	if s.Notes != "" {
		b.WriteString("\n" + mutedStyle.Render("Notes: "+s.Notes) + "\n")
	}

	if len(a.selHistory) > 0 {
		b.WriteString("\n" + headerStyle.Render("PRICE HISTORY"))
		b.WriteString("\n")
		for _, h := range a.selHistory {
			fmt.Fprintf(&b, "  %s  %s\n", FormatINR(h.Amount), orDash(h.ChangedAt))
		}
	}

	// descriptors
	var descs []string
	if json.Unmarshal([]byte(s.Descriptors), &descs) == nil && len(descs) > 0 {
		b.WriteString("\n" + headerStyle.Render("DESCRIPTORS"))
		b.WriteString("\n")
		for _, d := range descs {
			b.WriteString("  " + dimStyle.Render(d) + "\n")
		}
	}

	b.WriteString("\n" + hintStyle.Render("e edit · d delete · esc back"))
	return b.String()
}

func statusLabelStyled(status string) string {
	switch status {
	case "active":
		return greenStyle.Render(StatusLabel(status))
	case "paused":
		return yellowStyle.Render(StatusLabel(status))
	case "cancelled":
		return dimStyle.Render(StatusLabel(status))
	default:
		return status
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func joinNonEmpty(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" && p != "—" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return "—"
	}
	return strings.Join(out, " ")
}
