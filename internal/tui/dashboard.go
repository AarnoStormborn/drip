package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"drip/internal/model"
)

// DashboardData is the computed state shown on the Dashboard tab.
type DashboardData struct {
	ActiveCount  int
	MonthlySpend int64                // paise
	Overdue      []model.Subscription // next payment date in the past
	DueSoon      []model.Subscription // next payment within the next 7 days
}

func buildDashboard(active []model.Subscription) DashboardData {
	var d DashboardData
	d.ActiveCount = len(active)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	week := today.AddDate(0, 0, 7)

	for _, s := range active {
		d.MonthlySpend += MonthlyAmount(s)
		next, err := time.Parse("2006-01-02", s.NextPaymentDate)
		if err != nil {
			continue
		}
		switch {
		case next.Before(today):
			d.Overdue = append(d.Overdue, s)
		case !next.After(week):
			d.DueSoon = append(d.DueSoon, s)
		}
	}
	return d
}

func (a *App) renderDashboard() string {
	d := a.dash
	var b strings.Builder

	stats := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		"Active subscriptions   "+accentStyle.Render(strconv.Itoa(d.ActiveCount)),
		"Monthly spend          "+accentStyle.Render(FormatINR(d.MonthlySpend)),
		"Overdue                "+overdueCount(d.Overdue),
		"Due within 7 days      "+accentStyle.Render(strconv.Itoa(len(d.DueSoon))),
	))
	b.WriteString(stats)
	b.WriteString("\n\n")

	if d.ActiveCount == 0 {
		b.WriteString(mutedStyle.Render(
			"No active subscriptions yet.\n" +
				"Drop your statement files in the inbox and run `drip import <folder>`,\n" +
				"or press 3 (Import) for the monthly routine."))
		return b.String()
	}

	if len(d.Overdue) > 0 {
		b.WriteString(overdueStyle.Render("OVERDUE — payment date has passed"))
		b.WriteString("\n")
		for _, s := range d.Overdue {
			fmt.Fprintf(&b, "  ⚠ %-28s %s   due %s\n",
				truncate(s.Service, 28), accentStyle.Render(FormatINR(s.Amount)), dueToday.Render(FormatDate(s.NextPaymentDate)))
		}
		b.WriteString("\n")
	}

	b.WriteString(headerStyle.Render("NEXT 7 DAYS"))
	b.WriteString("\n")
	if len(d.DueSoon) == 0 {
		b.WriteString("  " + greenStyle.Render("Nothing due — enjoy the calm."))
	} else {
		for _, s := range d.DueSoon {
			fmt.Fprintf(&b, "  %-28s %s   due %s\n",
				truncate(s.Service, 28), accentStyle.Render(FormatINR(s.Amount)), FormatDate(s.NextPaymentDate))
		}
	}
	return b.String()
}

func overdueCount(overdue []model.Subscription) string {
	n := len(overdue)
	if n == 0 {
		return greenStyle.Render("0")
	}
	return overdueStyle.Render(strconv.Itoa(n))
}
