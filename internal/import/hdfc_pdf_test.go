package importer

import (
	"os"
	"testing"
)

// realPDFPath points at the user's actual HDFC statement (gitignored inbox).
// Tests skip when it is absent so CI / fresh clones stay green.
func realPDFPath(t *testing.T) string {
	t.Helper()
	for _, p := range []string{"../../inbox/hdfc.pdf", "inbox/hdfc.pdf"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("inbox/hdfc.pdf not present — skipping real-statement test")
	return ""
}

// Expected values from the statement's own summary row:
//
//	OpeningBalance 596,948.13 · DrCount 120 · CrCount 38
//	Debits 3,272,664.40 · Credits 3,082,809.84 · ClosingBal 407,093.57
const (
	wantTxns        = 158
	wantDebits      = 120
	wantCredits     = 38
	wantDebitTotal  = int64(327266440)
	wantCreditTotal = int64(308280984)
	wantOpening     = int64(59694813)
	wantClosing     = int64(40709357)
)

func TestParseHDFCPDFReal(t *testing.T) {
	path := realPDFPath(t)
	recs, err := ParseHDFCPDF(path)
	if err != nil {
		t.Fatalf("ParseHDFCPDF: %v", err)
	}

	if len(recs) != wantTxns {
		t.Fatalf("want %d transactions, got %d", wantTxns, len(recs))
	}

	// debit/credit split and totals
	var dr, cr, drSum, crSum int
	for _, r := range recs {
		if r.Amount < 0 {
			dr++
			drSum -= int(r.Amount)
		} else {
			cr++
			crSum += int(r.Amount)
		}
	}
	if dr != wantDebits || cr != wantCredits {
		t.Errorf("want %d/%d debit/credit, got %d/%d", wantDebits, wantCredits, dr, cr)
	}
	if drSum != int(wantDebitTotal) || crSum != int(wantCreditTotal) {
		t.Errorf("totals mismatch: debits=%d (want %d), credits=%d (want %d)", drSum, wantDebitTotal, crSum, wantCreditTotal)
	}

	// running-balance chain must be unbroken, opening → closing
	prev := wantOpening
	for i, r := range recs {
		if r.Balance == nil {
			t.Fatalf("record %d has nil balance", i)
		}
		if prev-r.Amount != *r.Balance && prev+r.Amount != *r.Balance {
			t.Fatalf("record %d (%s %s): balance chain broken: prev=%d amount=%d balance=%d",
				i, r.Date, r.Description, prev, r.Amount, *r.Balance)
		}
		prev = *r.Balance
	}
	if prev != wantClosing {
		t.Errorf("closing balance = %d, want %d", prev, wantClosing)
	}

	// first record spot check
	first := recs[0]
	if first.Date != "2026-05-01" || first.Amount != -100000 || *first.Balance != wantOpening-100000 {
		t.Errorf("first record unexpected: %+v", first)
	}
	if first.Account != "HDFC-savings" {
		t.Errorf("first record account = %q", first.Account)
	}

	// subscription signals present in descriptors
	found := map[string]bool{}
	for _, r := range recs {
		d := r.Description
		switch {
		case contains(d, "NETFLIX"):
			found["netflix"] = true
		case contains(d, "SPOTIFY"):
			found["spotify"] = true
		case contains(d, "AUTOPAY-APPLEMEDIA"):
			found["apple"] = true
		case contains(d, "AUTOPAY-WWWAIRTEL"):
			found["airtel"] = true
		}
	}
	for k := range found {
		if !found[k] {
			t.Errorf("missing descriptor signal %s", k)
		}
	}
	if !found["netflix"] || !found["spotify"] {
		t.Errorf("expected netflix/spotify descriptors, got %v", found)
	}
}

func TestParseHDFCPDFSourceFile(t *testing.T) {
	path := realPDFPath(t)
	recs, err := ParseHDFCPDF(path)
	if err != nil {
		t.Fatalf("ParseHDFCPDF: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("no records")
	}
	if recs[0].SourceFile != "hdfc.pdf" {
		t.Errorf("SourceFile = %q, want hdfc.pdf", recs[0].SourceFile)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
