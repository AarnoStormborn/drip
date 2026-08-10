package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbletea"

	"drip/internal/db"
	"drip/internal/model"
)

func seedTestDB(t *testing.T) *App {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	ctx := context.Background()
	subs := []model.Subscription{
		{Service: "Netflix", Amount: 64900, Cycle: "monthly", Rail: "card_emandate", Issuer: "HDFC", CardLast4: "3456", NextPaymentDate: "2026-08-14", Status: "active"},
		{Service: "YouTube Premium", Amount: 14900, Cycle: "monthly", Rail: "upi_autopay", MandateID: "MND-1", UPIApp: "gpay", NextPaymentDate: "2099-01-01", Status: "active"},
		{Service: "Notion Plus", Amount: 35900, Cycle: "monthly", Status: "paused"},
	}
	for i := range subs {
		if _, err := db.CreateSubscription(ctx, conn, &subs[i]); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	return New(conn)
}

func TestRenderAllTabs(t *testing.T) {
	a := seedTestDB(t)
	a.Init() // triggers refresh

	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	want := map[Tab][]string{
		TabDashboard:     {"Active subscriptions", "Monthly spend", "Netflix", "₹649.00"},
		TabSubscriptions: {"SERVICE", "Netflix", "YouTube Premium", "Notion Plus", "Monthly", "paused"},
		TabImport:        {"MONTHLY ROUTINE", "drip import", "Recurring detection"},
		TabReconcile:     {"Reconcile", "milestone 7"},
	}
	for tab, needles := range want {
		a.tab = tab
		a.refresh()
		v := a.View()
		for _, n := range needles {
			if !strings.Contains(v, n) {
				t.Errorf("tab %d: View() missing %q\n---\n%s", tab, n, v)
			}
		}
	}
}

func TestRenderEmptyDB(t *testing.T) {
	conn, err := db.Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	a := New(conn)
	a.Init()
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := a.View()
	for _, n := range []string{"No active subscriptions yet", "Dashboard", "Subscriptions", "Import", "Reconcile"} {
		if !strings.Contains(v, n) {
			t.Errorf("empty state missing %q\n---\n%s", n, v)
		}
	}
}
