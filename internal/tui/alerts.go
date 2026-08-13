package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// alert is a single dismissible notification. prio 1 = highest.
type alert struct {
	key  string // stable identity; dismiss marks this key for the session
	text string
	prio int
}

var (
	alertP1 = lipgloss.NewStyle().Bold(true).Foreground(dangerColor)
	alertP2 = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	alertP3 = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// computeAlerts derives actionable notifications from the current data.
// Priorities: overdue > due-soon > onboarding/import info.
func (a *App) computeAlerts() []alert {
	var out []alert

	if n := len(a.dash.Overdue); n > 0 {
		out = append(out, alert{
			key:  fmt.Sprintf("overdue:%d", n),
			text: fmt.Sprintf("⚠ %d subscription%s overdue — press 2 to review", n, plural(n)),
			prio: 1,
		})
	}
	if n := len(a.rec.MissingSubs); n > 0 {
		out = append(out, alert{
			key:  fmt.Sprintf("missing-sub:%d", n),
			text: fmt.Sprintf("⚠ %d mandate%s have no matching subscription — press 4 to review", n, plural(n)),
			prio: 1,
		})
	}
	if n := len(a.dash.DueSoon); n > 0 {
		out = append(out, alert{
			key:  fmt.Sprintf("duesoon:%d", n),
			text: fmt.Sprintf("• %d due within 7 days", n),
			prio: 2,
		})
	}
	if a.stmtCount == 0 && len(a.subs) == 0 {
		out = append(out, alert{
			key:  "welcome",
			text: "No statement data yet — drop a statement in inbox/ and run: drip import",
			prio: 3,
		})
	} else if a.stmtCount > 0 && len(a.subs) == 0 {
		out = append(out, alert{
			key:  "nodetect",
			text: "Statements loaded, no subscriptions yet — run: drip detect",
			prio: 3,
		})
	}
	if len(a.imports) > 0 {
		im := a.imports[0]
		out = append(out, alert{
			key:  "import:" + im.File + im.ImportedAt,
			text: fmt.Sprintf("Last import: %s (%d records)", im.File, im.Matched),
			prio: 4,
		})
	}
	return out
}

// refreshAlerts picks the highest-priority undismissed alert, if any.
func (a *App) refreshAlerts() {
	a.alertKey, a.alertText = "", ""
	for _, al := range a.computeAlerts() {
		if a.dismissed[al.key] {
			continue
		}
		a.alertKey, a.alertText = al.key, al.text
		return
	}
}

// dismissAlert hides the current alert (per session) and shows the next one.
func (a *App) dismissAlert() {
	if a.alertKey == "" {
		return
	}
	a.dismissed[a.alertKey] = true
	a.refreshAlerts()
}

// renderAlert draws the notification strip between header and content.
func (a *App) renderAlert() string {
	if a.alertText == "" {
		return ""
	}
	style := alertP3
	switch {
	case len(a.dash.Overdue) > 0:
		style = alertP1
	case len(a.dash.DueSoon) > 0:
		style = alertP2
	}
	// approximate right-side dismissal hint
	pad := a.w - lipgloss.Width(a.alertText) - len("  · x dismiss")
	if pad < 2 {
		pad = 2
	}
	return style.Render(a.alertText) + strings.Repeat(" ", pad) + dimStyle.Render("· x dismiss")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
