package tui

import (
	"testing"

	"drip/internal/model"
)

func TestFormatINR(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "₹0.00"},
		{99, "₹0.99"},
		{100, "₹1.00"},
		{999, "₹9.99"},
		{1000, "₹10.00"},
		{123456, "₹1,234.56"},
		{12345678, "₹1,23,456.78"},
		{100000000, "₹10,00,000.00"},
		{-500, "-₹5.00"},
	}
	for _, c := range cases {
		if got := FormatINR(c.in); got != c.want {
			t.Errorf("FormatINR(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMonthlyAmount(t *testing.T) {
	sub := func(amount int64, cycle string) model.Subscription {
		return model.Subscription{Amount: amount, Cycle: cycle}
	}
	cases := []struct {
		in   model.Subscription
		want int64
	}{
		{sub(10000, "monthly"), 10000},
		{sub(10000, "weekly"), 10000 * 52 / 12},
		{sub(30000, "quarterly"), 10000},
		{sub(120000, "yearly"), 10000},
		{sub(10000, "once"), 10000},
		{sub(10000, ""), 10000},
	}
	for _, c := range cases {
		if got := MonthlyAmount(c.in); got != c.want {
			t.Errorf("MonthlyAmount(%+v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestFormatDate(t *testing.T) {
	if got := FormatDate("2026-08-15"); got != "15 Aug 2026" {
		t.Errorf("FormatDate = %q", got)
	}
	if got := FormatDate(""); got != "—" {
		t.Errorf("FormatDate(empty) = %q, want —", got)
	}
	if got := FormatDate("not-a-date"); got != "not-a-date" {
		t.Errorf("FormatDate(bad) = %q", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("Netflix", 26); got != "Netflix" {
		t.Errorf("truncate short = %q", got)
	}
	if got := truncate("A Very Long Subscription Name Indeed", 10); got != "A Very Lo…" {
		t.Errorf("truncate long = %q", got)
	}
}
