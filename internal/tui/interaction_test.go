package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbletea"

	"drip/internal/db"
	"drip/internal/model"
)

func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(string(r))}
}

func key(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func TestSubscriptionsEditFlow(t *testing.T) {
	conn, err := db.Open(t.TempDir() + "/interact.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()
	ctx := context.Background()

	id, err := db.CreateSubscription(ctx, conn, &model.Subscription{
		Service: "Netflix", Amount: 64900, Cycle: "monthly",
		Rail: "upi_autopay", NextPaymentDate: "2026-08-23", Status: "active",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	a := New(conn)
	a.Init()
	a.Update(tea.WindowSizeMsg{Width: 110, Height: 40})

	// go to subscriptions tab, open detail, open edit form
	a.Update(key(tea.KeyRunes))
	a.Update(keyRune('2'))
	a.Update(key(tea.KeyEnter))
	a.Update(keyRune('e'))
	if a.subsMode != subsForm || a.form == nil {
		t.Fatalf("expected form mode, got mode=%d form=%v", a.subsMode, a.form != nil)
	}

	// edit the amount field: focus is on Service; move to Amount, clear, type new
	a.Update(key(tea.KeyDown)) // → Amount
	// clear "649.00" with backspaces (cursor sits at end of the field)
	for i := 0; i < 6; i++ {
		a.Update(key(tea.KeyBackspace))
	}
	for _, r := range "799.00" {
		a.Update(keyRune(r))
	}
	a.Update(key(tea.KeyCtrlS))
	if a.subsMode != subsDetail || a.form != nil {
		t.Fatalf("expected back to detail after save, mode=%d", a.subsMode)
	}
	if a.err != nil {
		t.Fatalf("err after save: %v", a.err)
	}

	got, err := db.GetSubscription(ctx, conn, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Amount != 79900 {
		t.Errorf("amount = %d, want 79900", got.Amount)
	}

	// delete flow: confirm + y
	a.Update(keyRune('d'))
	if a.subsMode != subsConfirm {
		t.Fatalf("expected confirm mode, got %d", a.subsMode)
	}
	a.Update(keyRune('y'))
	if a.subsMode != subsList {
		t.Fatalf("expected list after delete, got %d", a.subsMode)
	}
	if _, err := db.GetSubscription(ctx, conn, id); err == nil {
		t.Error("subscription still exists after delete")
	}
}

func TestFormValidation(t *testing.T) {
	conn, err := db.Open(t.TempDir() + "/form.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()
	ctx := context.Background()

	id, err := db.CreateSubscription(ctx, conn, &model.Subscription{
		Service: "X", Amount: 10000, Cycle: "monthly", Status: "active",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	s, _ := db.GetSubscription(ctx, conn, id)

	f := newEditForm(s)
	// empty service → error
	f.fields[0].input.SetValue("")
	if _, err := f.Build(s); err == nil || !strings.Contains(err.Error(), "service") {
		t.Errorf("empty service: want error mentioning service, got %v", err)
	}
	// bad amount → error
	f.fields[0].input.SetValue("Netflix")
	f.fields[1].input.SetValue("abc")
	if _, err := f.Build(s); err == nil || !strings.Contains(err.Error(), "amount") {
		t.Errorf("bad amount: want error, got %v", err)
	}
	// bad date → error
	f.fields[1].input.SetValue("649")
	f.fields[3].input.SetValue("23/08/2026")
	if _, err := f.Build(s); err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Errorf("bad date: want error, got %v", err)
	}
	// valid input → persisted model
	f.fields[3].input.SetValue("2026-09-01")
	f.fields[4].value = "yearly"
	f.fields[5].value = "paused"
	got, err := f.Build(s)
	if err != nil {
		t.Fatalf("valid build: %v", err)
	}
	if got.Service != "Netflix" || got.Amount != 64900 || got.NextPaymentDate != "2026-09-01" ||
		got.Cycle != "yearly" || got.Status != "paused" {
		t.Errorf("build result unexpected: %+v", got)
	}
}

func TestParseRupees(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"649", 64900},
		{"649.50", 64950},
		{"1,299.50", 129950},
		{"₹799", 79900},
		{" 100 ", 10000},
	}
	for _, c := range cases {
		got, err := parseRupees(c.in)
		if err != nil || got != c.want {
			t.Errorf("parseRupees(%q) = %d, %v; want %d", c.in, got, err, c.want)
		}
	}
	for _, in := range []string{"", "-5", "abc", "0"} {
		if _, err := parseRupees(in); err == nil {
			t.Errorf("parseRupees(%q): expected error", in)
		}
	}
}
