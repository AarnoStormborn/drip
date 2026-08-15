package detect

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/AarnoStormborn/drip/internal/db"
	"github.com/AarnoStormborn/drip/internal/model"
)

func seed(t *testing.T, recs []model.StatementRecord) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "detect.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := db.InsertStatements(context.Background(), conn, recs); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return conn
}

func sub(t *testing.T, conn *sql.DB, service string) (model.Subscription, bool) {
	t.Helper()
	s, ok, err := db.GetSubscriptionByService(context.Background(), conn, service)
	if err != nil {
		t.Fatalf("get %s: %v", service, err)
	}
	return s, ok
}

func debit(date, desc string, amount int64) model.StatementRecord {
	return model.StatementRecord{Date: date, Description: desc, Amount: -amount}
}

func TestDetectAutopayCreatesSubscription(t *testing.T) {
	conn := seed(t, []model.StatementRecord{
		debit("2026-05-23", "UPI-AUTOPAY-NETFLIX-NETFLIX.BD@AXISBANK-747434051436-MANDATEEXECUTE", 64900),
		debit("2026-06-23", "UPI-AUTOPAY-NETFLIX-NETFLIX.BD@AXISBANK-817530221746-MANDATEEXECUTE", 64900),
		debit("2026-07-23", "UPI-AUTOPAY-NETFLIX-NETFLIX.BD@AXISBANK-926811242046-MANDATEEXECUTE", 64900),
	})
	ctx := context.Background()

	res, err := Run(ctx, conn)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Created != 1 || res.Updated != 0 || res.Candidates != 0 {
		t.Fatalf("res = %+v", res)
	}

	s, ok := sub(t, conn, "Netflix")
	if !ok {
		t.Fatal("Netflix not created")
	}
	if s.Amount != 64900 || s.Cycle != "monthly" || s.Rail != "upi_autopay" {
		t.Errorf("unexpected: %+v", s)
	}
	if s.FirstSeen != "2026-05-23" || s.LastPaymentDate != "2026-07-23" || s.NextPaymentDate != "2026-08-23" {
		t.Errorf("dates: first=%s last=%s next=%s", s.FirstSeen, s.LastPaymentDate, s.NextPaymentDate)
	}
	if s.Status != "active" {
		t.Errorf("status = %q", s.Status)
	}

	// re-run is idempotent (update, not create)
	res2, _ := Run(ctx, conn)
	if res2.Created != 0 || res2.Updated != 1 {
		t.Errorf("re-run res = %+v", res2)
	}
}

func TestDetectSingleAutopayCharge(t *testing.T) {
	conn := seed(t, []model.StatementRecord{
		debit("2026-07-25", "UPI-AUTOPAY-SONYLIV-SONYLIVHYP@YESPAY-YESB0YESUPI-620689270973-RECURRINGTXN", 39900),
	})
	res, err := Run(context.Background(), conn)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("single autopay charge should create: %+v", res)
	}
	s, ok := sub(t, conn, "SonyLIV")
	if !ok {
		t.Fatal("SonyLIV not created")
	}
	if s.NextPaymentDate != "2026-08-25" {
		t.Errorf("next = %s", s.NextPaymentDate)
	}
}

func TestDetectSingleChargeNonAutopayIsCandidate(t *testing.T) {
	conn := seed(t, []model.StatementRecord{
		debit("2026-07-11", "UPI-GPAY-TOLL-PAYTM-123@OKPAY-YESB0PTMUPI", 10000),
	})
	res, err := Run(context.Background(), conn)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Candidates != 1 || res.Created != 0 {
		t.Fatalf("res = %+v", res)
	}
	if _, ok := sub(t, conn, "Gpay Toll"); ok {
		t.Fatal("candidate should not be created")
	}
}

func TestDetectMaskedTransferSkipped(t *testing.T) {
	conn := seed(t, []model.StatementRecord{
		debit("2026-07-15", "UPI-XXXXXXXX0711-ICIC0001045-122949801216-UPI", 5000000),
		debit("2026-08-15", "UPI-XXXXXXXX0711-ICIC0001045-126336245406-UPI", 5000000),
	})
	res, err := Run(context.Background(), conn)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Created != 0 || res.Candidates != 0 {
		t.Fatalf("masked transfers must be ignored: %+v", res)
	}
}

func TestDetectVariableAmountsUseDominant(t *testing.T) {
	conn := seed(t, []model.StatementRecord{
		debit("2026-05-08", "UPI-AUTOPAY-APPLEMEDIA SERVICES-APPLESE RVICES.BDSI@HDFCBANK", 7500),
		debit("2026-06-08", "UPI-AUTOPAY-APPLEMEDIA SERVICES-APPLESE RVICES.BDSI@HDFCBANK", 7500),
		debit("2026-07-08", "UPI-AUTOPAY-APPLEMEDIA SERVICES-APPLESE RVICES.BDSI@HDFCBANK", 7500),
		debit("2026-05-29", "UPI-AUTOPAY-APPLEMEDIA SERVICES-APPLESE RVICES.BDSI@HDFCBANK", 21900),
		debit("2026-06-29", "UPI-AUTOPAY-APPLEMEDIA SERVICES-APPLESE RVICES.BDSI@HDFCBANK", 21900),
		debit("2026-07-29", "UPI-AUTOPAY-APPLEMEDIA SERVICES-APPLESE RVICES.BDSI@HDFCBANK", 21900),
		debit("2026-06-01", "UPI-AUTOPAY-APPLEMEDIA SERVICES-APPLESE RVICES.BDSI@HDFCBANK", 49900),
	})
	_, err := Run(context.Background(), conn)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	s, ok := sub(t, conn, "Apple Media")
	if !ok {
		t.Fatal("Apple Media not created")
	}
	// 75 and 219 tie at 3; most-recent tie-break picks 219
	if s.Amount != 21900 {
		t.Errorf("dominant amount = %d, want 21900", s.Amount)
	}
}

func TestDetectPriceChangeRecordsHistory(t *testing.T) {
	conn := seed(t, []model.StatementRecord{
		debit("2026-05-23", "UPI-AUTOPAY-NETFLIX-NETFLIX.BD@AXISBANK-1-MANDATEEXECUTE", 64900),
		debit("2026-06-23", "UPI-AUTOPAY-NETFLIX-NETFLIX.BD@AXISBANK-2-MANDATEEXECUTE", 64900),
	})
	ctx := context.Background()
	if _, err := Run(ctx, conn); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// price hike: same service, new amount, two more months
	if _, err := db.InsertStatements(ctx, conn, []model.StatementRecord{
		debit("2026-07-23", "UPI-AUTOPAY-NETFLIX-NETFLIX.BD@AXISBANK-3-MANDATEEXECUTE", 69900),
		debit("2026-08-23", "UPI-AUTOPAY-NETFLIX-NETFLIX.BD@AXISBANK-4-MANDATEEXECUTE", 69900),
	}); err != nil {
		t.Fatalf("insert more: %v", err)
	}
	if _, err := Run(ctx, conn); err != nil {
		t.Fatalf("Run 2: %v", err)
	}

	s, _ := sub(t, conn, "Netflix")
	if s.Amount != 69900 {
		t.Errorf("amount = %d, want 69900", s.Amount)
	}

	var hist int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM price_history WHERE subscription_id = ?`, s.ID).Scan(&hist); err != nil {
		t.Fatalf("price_history: %v", err)
	}
	if hist != 1 {
		t.Errorf("price_history rows = %d, want 1", hist)
	}
}

func TestDetectCancelledReactivated(t *testing.T) {
	conn := seed(t, []model.StatementRecord{
		debit("2026-05-23", "UPI-AUTOPAY-NETFLIX-NETFLIX.BD@AXISBANK-1-MANDATEEXECUTE", 64900),
		debit("2026-06-23", "UPI-AUTOPAY-NETFLIX-NETFLIX.BD@AXISBANK-2-MANDATEEXECUTE", 64900),
	})
	ctx := context.Background()
	if _, err := Run(ctx, conn); err != nil {
		t.Fatalf("Run: %v", err)
	}
	s, ok := sub(t, conn, "Netflix")
	if !ok {
		t.Fatal("not created")
	}
	if err := db.SetSubscriptionStatus(ctx, conn, s.ID, "cancelled"); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	// fresh debit later — detection should reactivate
	if _, err := db.InsertStatements(ctx, conn, []model.StatementRecord{
		debit("2026-07-23", "UPI-AUTOPAY-NETFLIX-NETFLIX.BD@AXISBANK-3-MANDATEEXECUTE", 64900),
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := Run(ctx, conn); err != nil {
		t.Fatalf("Run 2: %v", err)
	}
	s, _ = sub(t, conn, "Netflix")
	if s.Status != "active" {
		t.Errorf("status = %q, want active (reactivated)", s.Status)
	}
	if s.NextPaymentDate != "2026-08-23" {
		t.Errorf("next = %s", s.NextPaymentDate)
	}
}
