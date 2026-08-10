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
	MonthlySpend int64 // paise
	DueSoon      []model.Subscription
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
		if !next.Before(today) && !next.After(week) {
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
		"Due within 7 days      "+accentStyle.Render(strconv.Itoa(len(d.DueSoon))),
	))
	b.WriteString(stats)
	b.WriteString("\n\n")

	if d.ActiveCount == 0 {
		b.WriteString(mutedStyle.Render(
			"No active subscriptions yet.\n" +
				"Drop your statement files in the inbox and run `drip import <folder>` (arriving in milestone 2),\n" +
				"or press 3 (Import) for the monthly routine."))
		return b.String()
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
