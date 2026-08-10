package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"drip/internal/model"
)

func openTest(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestOpenCreatesSchema(t *testing.T) {
	conn := openTest(t)
	for _, tbl := range []string{"subscriptions", "statements", "imports", "price_history"} {
		var name string
		err := conn.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&name)
		if err != nil {
			t.Fatalf("expected table %s to exist: %v", tbl, err)
		}
	}
}

func TestSubscriptionCRUD(t *testing.T) {
	ctx := context.Background()
	conn := openTest(t)

	id, err := CreateSubscription(ctx, conn, &model.Subscription{
		Service:         "Netflix",
		Amount:          19900,
		Currency:        "INR",
		Cycle:           "monthly",
		Rail:            "card_emandate",
		Issuer:          "HDFC",
		CardLast4:       "3456",
		NextPaymentDate: "2026-08-15",
		Status:          "active",
	})
	if err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}

	subs, err := ListSubscriptions(ctx, conn, "")
	if err != nil {
		t.Fatalf("ListSubscriptions: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("want 1 subscription, got %d", len(subs))
	}
	if subs[0].Service != "Netflix" || subs[0].Amount != 19900 {
		t.Fatalf("unexpected sub: %+v", subs[0])
	}

	active, _ := ListSubscriptions(ctx, conn, "active")
	if len(active) != 1 {
		t.Fatalf("want 1 active, got %d", len(active))
	}

	if err := SetSubscriptionStatus(ctx, conn, id, "cancelled"); err != nil {
		t.Fatalf("SetSubscriptionStatus: %v", err)
	}
	active, _ = ListSubscriptions(ctx, conn, "active")
	if len(active) != 0 {
		t.Fatalf("want 0 active after cancel, got %d", len(active))
	}

	got, err := GetSubscription(ctx, conn, id)
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if got.Status != "cancelled" {
		t.Fatalf("want status cancelled, got %q", got.Status)
	}

	got.Service = "Netflix (changed)"
	got.Amount = 29900
	if err := UpdateSubscription(ctx, conn, &got); err != nil {
		t.Fatalf("UpdateSubscription: %v", err)
	}
	got2, _ := GetSubscription(ctx, conn, id)
	if got2.Service != "Netflix (changed)" || got2.Amount != 29900 {
		t.Fatalf("update not persisted: %+v", got2)
	}

	if err := DeleteSubscription(ctx, conn, id); err != nil {
		t.Fatalf("DeleteSubscription: %v", err)
	}
	subs, _ = ListSubscriptions(ctx, conn, "")
	if len(subs) != 0 {
		t.Fatalf("want 0 subs after delete, got %d", len(subs))
	}
}

func TestStatements(t *testing.T) {
	ctx := context.Background()
	conn := openTest(t)

	recs := []model.StatementRecord{
		{Date: "2026-08-01", Description: "UPI/123/NETFLIX/NETFLIX.COM", Amount: -19900, SourceFile: "hdfc.csv", Account: "HDFC-savings"},
		{Date: "2026-08-05", Description: "UPI/456/SPOTIFY/SPOTIFY AB", Amount: -11900, SourceFile: "hdfc.csv", Account: "HDFC-savings"},
		{Date: "2026-08-10", Description: "Salary", Amount: 10000000, SourceFile: "hdfc.csv", Account: "HDFC-savings"},
	}
	n, err := InsertStatements(ctx, conn, recs)
	if err != nil {
		t.Fatalf("InsertStatements: %v", err)
	}
	if n != 3 {
		t.Fatalf("want 3 inserted, got %d", n)
	}

	cnt, err := CountStatements(ctx, conn)
	if err != nil {
		t.Fatalf("CountStatements: %v", err)
	}
	if cnt != 3 {
		t.Fatalf("want 3 counted, got %d", cnt)
	}

	recent, err := ListStatements(ctx, conn, 2)
	if err != nil {
		t.Fatalf("ListStatements: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("want 2 listed (limit), got %d", len(recent))
	}
	if recent[0].Description != "Salary" { // newest first
		t.Fatalf("want newest first, got %q", recent[0].Description)
	}
	if recent[0].Balance != nil {
		t.Fatalf("want nil balance, got %v", *recent[0].Balance)
	}
}
