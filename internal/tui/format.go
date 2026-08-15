package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/AarnoStormborn/drip/internal/model"
)

// FormatINR renders paise as an Indian-grouped rupee string,
// e.g. 12345678 paise → "₹1,23,456.78".
func FormatINR(paise int64) string {
	neg := paise < 0
	if neg {
		paise = -paise
	}
	s := "₹" + indianGroup(paise/100) + "." + fmt.Sprintf("%02d", paise%100)
	if neg {
		s = "-" + s
	}
	return s
}

func indianGroup(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	parts := []string{s[len(s)-3:]}
	rest := s[:len(s)-3]
	for len(rest) > 2 {
		parts = append([]string{rest[len(rest)-2:]}, parts...)
		rest = rest[:len(rest)-2]
	}
	return strings.Join(append([]string{rest}, parts...), ",")
}

// MonthlyAmount converts a subscription's amount to its monthly equivalent
// in paise (used for "monthly spend" figures). Integer math: weekly ≈ ×52/12.
func MonthlyAmount(s model.Subscription) int64 {
	switch s.Cycle {
	case "weekly":
		return s.Amount * 52 / 12
	case "quarterly":
		return s.Amount / 3
	case "yearly":
		return s.Amount / 12
	default:
		return s.Amount // monthly, once, unknown cycles
	}
}

// CycleLabel renders a human-friendly cycle name.
func CycleLabel(c string) string {
	switch c {
	case "weekly":
		return "Weekly"
	case "monthly":
		return "Monthly"
	case "quarterly":
		return "Quarterly"
	case "yearly":
		return "Yearly"
	case "once":
		return "One-time"
	default:
		return c
	}
}

// FormatDate renders ISO YYYY-MM-DD as "02 Jan 2006"; passes through on error.
func FormatDate(iso string) string {
	if iso == "" {
		return "—"
	}
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.Format("02 Jan 2006")
}

// formatRupees renders paise as a plain decimal string, e.g. 64900 → "649.00".
func formatRupees(paise int64) string {
	neg := paise < 0
	if neg {
		paise = -paise
	}
	s := fmt.Sprintf("%d.%02d", paise/100, paise%100)
	if neg {
		s = "-" + s
	}
	return s
}

// parseRupees converts a user-entered rupee amount ("649", "649.50",
// "1,299.50") to paise. Rejects empty/negative/zero.
func parseRupees(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "₹")
	s = strings.ReplaceAll(s, ",", "")
	if s == "" {
		return 0, fmt.Errorf("amount is empty")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("amount %q is not a number", s)
	}
	if f <= 0 {
		return 0, fmt.Errorf("amount must be positive")
	}
	return int64(f*100 + 0.5), nil
}

// StatusLabel styles a status string (kept plain; colors applied separately).
func StatusLabel(status string) string {
	switch status {
	case "active":
		return "active"
	case "paused":
		return "paused"
	case "cancelled":
		return "cancelled"
	default:
		return status
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}
