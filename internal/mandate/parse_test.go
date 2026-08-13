package mandate

import "testing"

func TestParseLine(t *testing.T) {
	cases := []struct {
		line         string
		wantService  string
		wantAmount   int64
		wantCycle    string
		wantNextDate string
	}{
		{"Netflix ₹649.00 monthly next debit 23/08/2026", "Netflix", 64900, "monthly", "2026-08-23"},
		{"Spotify — ₹149 — Monthly — 25 Aug 2026", "Spotify", 14900, "monthly", "2026-08-25"},
		{"YouTube Premium Rs. 129.00 every month", "YouTube Premium", 12900, "monthly", ""},
		{"Airtel ₹529.82 quarterly", "Airtel", 52982, "quarterly", ""},
		{"Netflix   ₹649  yearly", "Netflix", 64900, "yearly", ""},
		{"Paytm ₹3297.66 next debit on 05-08-2026", "Paytm", 329766, "monthly", "2026-08-05"},
	}
	for _, c := range cases {
		m, err := ParseLine(c.line)
		if err != nil {
			t.Errorf("ParseLine(%q): %v", c.line, err)
			continue
		}
		if m.Service != c.wantService {
			t.Errorf("ParseLine(%q) service = %q, want %q", c.line, m.Service, c.wantService)
		}
		if m.Amount != c.wantAmount {
			t.Errorf("ParseLine(%q) amount = %d, want %d", c.line, m.Amount, c.wantAmount)
		}
		if m.Cycle != c.wantCycle {
			t.Errorf("ParseLine(%q) cycle = %q, want %q", c.line, m.Cycle, c.wantCycle)
		}
		if m.NextDebitDate != c.wantNextDate {
			t.Errorf("ParseLine(%q) next = %q, want %q", c.line, m.NextDebitDate, c.wantNextDate)
		}
	}
}

func TestParseLineErrors(t *testing.T) {
	for _, line := range []string{"", "   ", "no amount here", "₹99"} {
		if _, err := ParseLine(line); err == nil {
			t.Errorf("ParseLine(%q): expected error", line)
		}
	}
}

func TestParseFile(t *testing.T) {
	content := "Netflix ₹649.00 monthly 23/08/2026\n\nSpotify — ₹149 — Monthly — 25 Aug 2026\nthis line is garbage\n"
	mandates, errs := ParseFile(content)
	if len(mandates) != 2 {
		t.Errorf("parsed %d mandates, want 2", len(mandates))
	}
	if len(errs) != 1 {
		t.Errorf("errors = %v, want 1", errs)
	}
	if errs[0] == nil || !containsStr(errs[0].Error(), "line 4") {
		t.Errorf("error should mention line 4, got %v", errs[0])
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
