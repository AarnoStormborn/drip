package importer

import "testing"

func TestParseMoney(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"1,000.00", 100000},
		{"415.00", 41500},
		{"20,000.00", 2000000},
		{"3,272,664.40", 327266440},
		{"0.00", 0},
		{"595,948.13", 59594813},
	}
	for _, c := range cases {
		got, err := parseMoney(c.in)
		if err != nil {
			t.Errorf("parseMoney(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseMoney(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseMoneyInvalid(t *testing.T) {
	if _, err := parseMoney(""); err == nil {
		t.Error("parseMoney(\"\"): expected error")
	}
	if _, err := parseMoney("abc"); err == nil {
		t.Error("parseMoney(\"abc\"): expected error")
	}
	// lenient by design: commas stripped, sub-paise rounded
	if got, err := parseMoney("1,00.0"); err != nil || got != 10000 {
		t.Errorf("parseMoney(1,00.0) = %d, %v; want 10000", got, err)
	}
	if got, err := parseMoney("12.345"); err != nil || got != 1235 {
		t.Errorf("parseMoney(12.345) = %d, %v; want 1235 (rounded)", got, err)
	}
}

func TestParseDDMMYY(t *testing.T) {
	cases := []struct{ in, want string }{
		{"01/05/26", "2026-05-01"},
		{"31/12/26", "2026-12-31"},
		{"25/07/26", "2026-07-25"},
		{"10/12/25", "2025-12-10"},
		{"15/01/50", "1950-01-15"},
	}
	for _, c := range cases {
		got, err := parseDDMMYY(c.in)
		if err != nil {
			t.Errorf("parseDDMMYY(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseDDMMYY(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	for _, in := range []string{"", "01/13/26", "32/01/26", "01-05-26", "01/05/2026"} {
		if _, err := parseDDMMYY(in); err == nil {
			t.Errorf("parseDDMMYY(%q): expected error", in)
		}
	}
}

func TestClassifyMoneySide(t *testing.T) {
	cases := []struct {
		x       float64
		raw     string
		want    int
		comment string
	}{
		{448.2, "415.00", -1, "short withdrawal"},
		{452.2, "65.00", -1, "short withdrawal"},
		{512.2, "354,361.80", 1, "long deposit"},
		{520.2, "5,000.00", 1, "deposit"},
		{440.0, "20,000.00", -1, "withdrawal"},
	}
	for _, c := range cases {
		if got := classifyMoneySide(c.x, c.raw); got != c.want {
			t.Errorf("classifyMoneySide(%.1f, %q) = %d, want %d (%s)", c.x, c.raw, got, c.want, c.comment)
		}
	}
}
