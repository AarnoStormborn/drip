// Package tui is the Bubble Tea application: a tabbed shell over the Drip
// database (Dashboard / Subscriptions / Import / Reconcile).
package tui

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"drip/internal/db"
	"drip/internal/model"
)

// Tab is the active top-level view.
type Tab int

const (
	TabDashboard Tab = iota
	TabSubscriptions
	TabImport
	TabReconcile
)

var tabNames = [...]string{"Dashboard", "Subscriptions", "Import", "Reconcile"}

// App is the root Bubble Tea model.
type App struct {
	sqldb *sql.DB
	tab   Tab
	w, h  int
	err   error

	dash    DashboardData
	subs    []model.Subscription // all subscriptions (any status)
	imports []model.ImportAudit  // recent import runs

	// Subscriptions tab interaction state
	subsMode   subsMode
	subsSel    int
	selHistory []model.PriceHistory
	form       *editForm
}

// New builds the root model. The caller owns closing sqldb.
func New(sqldb *sql.DB) *App {
	return &App{sqldb: sqldb}
}

func (a *App) Init() tea.Cmd {
	a.refresh()
	return nil
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.w, a.h = m.Width, m.Height
		if a.form != nil {
			a.form.Update(m)
		}
	case tea.KeyMsg:
		// Edit form consumes all keys (except ctrl+c quits) so typing works.
		if a.tab == TabSubscriptions && a.subsMode == subsForm && a.form != nil {
			switch m.String() {
			case "ctrl+c":
				return a, tea.Quit
			case "esc":
				a.form.Cancelled = true
			case "ctrl+s":
				a.saveForm()
			default:
				a.form.Update(m)
			}
			if a.form != nil && a.form.Cancelled {
				a.form = nil
				a.subsMode = subsDetail
			}
			return a, nil
		}

		switch m.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "1":
			a.tab = TabDashboard
			a.subsMode = subsList
			a.refresh()
		case "2":
			a.tab = TabSubscriptions
			a.refresh()
		case "3":
			a.tab = TabImport
			a.subsMode = subsList
		case "4":
			a.tab = TabReconcile
			a.subsMode = subsList
		case "tab":
			a.tab = Tab((int(a.tab) + 1) % len(tabNames))
			a.subsMode = subsList
			a.refresh()
		case "shift+tab":
			a.tab = Tab((int(a.tab) + len(tabNames) - 1) % len(tabNames))
			a.subsMode = subsList
			a.refresh()
		case "r":
			a.refresh()
		default:
			if a.tab == TabSubscriptions {
				a.handleSubsKey(m)
			}
		}
	}
	return a, nil
}

// saveForm persists the edited subscription (called from the form on ctrl+s).
func (a *App) saveForm() {
	s, err := db.GetSubscription(context.Background(), a.sqldb, a.form.targetID)
	if err != nil {
		a.form.Err = fmt.Sprintf("load subscription: %v", err)
		return
	}
	updated, err := a.form.Build(s)
	if err != nil {
		a.form.Err = err.Error()
		return
	}
	if err := db.UpdateSubscription(context.Background(), a.sqldb, &updated); err != nil {
		a.form.Err = err.Error()
		return
	}
	a.form = nil
	a.subsMode = subsDetail
	a.refresh()
	a.openDetail(a.subsSel)
}

// refresh reloads dashboard data and the subscription list from SQLite.
// Data volumes are small (a personal ledger), so synchronous reads are fine.
func (a *App) refresh() {
	ctx := context.Background()

	active, err := db.ListSubscriptions(ctx, a.sqldb, "active")
	if err != nil {
		a.err = err
		return
	}
	subs, err := db.ListSubscriptions(ctx, a.sqldb, "")
	if err != nil {
		a.err = err
		return
	}
	a.err = nil
	a.subs = subs
	a.dash = buildDashboard(active)

	imports, err := db.ListImports(ctx, a.sqldb, 5)
	if err != nil {
		a.err = err
		return
	}
	a.imports = imports
}

func (a *App) View() string {
	header := lipgloss.JoinHorizontal(lipgloss.Center,
		titleStyle.Render("Drip — subscription tracker"),
		"   ",
		a.renderTabs(),
	)

	var body string
	switch a.tab {
	case TabDashboard:
		body = a.renderDashboard()
	case TabSubscriptions:
		body = a.renderSubscriptions()
	case TabImport:
		body = a.renderImport()
	case TabReconcile:
		body = a.renderReconcile()
	}
	if a.err != nil {
		body = errorStyle.Render("⚠ "+a.err.Error()) + "\n\n" + body
	}

	content := lipgloss.JoinVertical(lipgloss.Left, header, contentStyle.Render(body))
	footer := footerStyle.Render(
		"1 Dashboard · 2 Subscriptions · 3 Import · 4 Reconcile · tab/⇧tab switch · r refresh · q quit")

	frame := lipgloss.JoinVertical(lipgloss.Left, content, footer)
	if a.h > 0 {
		frame = lipgloss.NewStyle().Height(a.h).Render(frame)
	}
	return frame
}

func (a *App) renderTabs() string {
	var cells []string
	for i, name := range tabNames {
		if Tab(i) == a.tab {
			cells = append(cells, activeTabStyle.Render(name))
		} else {
			cells = append(cells, tabStyle.Render(name))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cells...)
}

// ── column helpers for the subscriptions table ────────────────────────────
// Note: lipgloss's JoinHorizontal trims trailing cell padding, so we pad
// cells as plain text first and apply styles afterwards.

func statusCellPadded(status string) string {
	label := fmt.Sprintf("%-*s", wStatus, StatusLabel(status))
	switch status {
	case "active":
		return greenStyle.Render(label)
	case "paused":
		return yellowStyle.Render(label)
	case "cancelled":
		return dimStyle.Render(label)
	default:
		return label
	}
}
