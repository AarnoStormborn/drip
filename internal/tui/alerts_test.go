package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"

	"github.com/AarnoStormborn/drip/internal/db"
	"github.com/AarnoStormborn/drip/internal/model"
)

func TestAlertsOverdueAndDismiss(t *testing.T) {
	conn, err := db.Open(t.TempDir() + "/alerts.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()
	ctx := context.Background()

	past := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	if _, err := db.CreateSubscription(ctx, conn, &model.Subscription{
		Service: "Lapsed", Amount: 10000, Cycle: "monthly", Status: "active", NextPaymentDate: past,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	a := New(conn)
	a.Init()
	a.Update(tea.WindowSizeMsg{Width: 110, Height: 40})

	if a.alertText == "" {
		t.Fatal("expected an alert (overdue sub)")
	}
	if !strings.Contains(a.alertText, "overdue") {
		t.Errorf("alert = %q, want overdue mention", a.alertText)
	}
	if a.alertKey == "" {
		t.Fatal("alert key empty")
	}

	// dismiss → no more alerts
	a.Update(keyRune('x'))
	if a.alertText != "" {
		t.Errorf("alert after dismiss = %q, want empty", a.alertText)
	}

	// the overdue alert is keyed by count; a second overdue sub changes the
	// key and re-surfaces it
	if _, err := db.CreateSubscription(ctx, conn, &model.Subscription{
		Service: "Lapsed2", Amount: 5000, Cycle: "monthly", Status: "active", NextPaymentDate: past,
	}); err != nil {
		t.Fatalf("seed 2: %v", err)
	}
	a.refresh()
	if !strings.Contains(a.alertText, "2 subscription") {
		t.Errorf("alert after second overdue = %q, want count 2", a.alertText)
	}
}

func TestAlertsWelcomeState(t *testing.T) {
	conn, err := db.Open(t.TempDir() + "/welcome.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()

	a := New(conn)
	a.Init()
	if !strings.Contains(a.alertText, "drip import") {
		t.Errorf("fresh db alert = %q, want import hint", a.alertText)
	}
}

func TestBannerWidthFallback(t *testing.T) {
	conn, err := db.Open(t.TempDir() + "/banner.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()

	a := New(conn)
	a.Init()

	a.Update(tea.WindowSizeMsg{Width: 110, Height: 40})
	v := a.View()
	if !strings.Contains(v, "$$$$$$$") {
		t.Error("wide terminal: wordmark missing")
	}

	a.Update(tea.WindowSizeMsg{Width: 60, Height: 40})
	v = a.View()
	if strings.Contains(v, "$$$$$$$") {
		t.Error("narrow terminal: wordmark should be hidden")
	}
	if !strings.Contains(v, "Drip — subscription tracker") {
		t.Error("narrow terminal: fallback title missing")
	}
}
