// Package mandate parses UPI AutoPay mandate lines copied from a UPI app
// (Google Pay → Profile → UPI Autopay). The on-screen format varies by app
// and language, so parsing is deliberately lenient: extract whatever fields
// are recognizable (amount, cycle, next debit date), keep the raw line, and
// require at least a service name and amount.
package mandate

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"drip/internal/model"
)

var (
	amountRe = regexp.MustCompile(`(?:₹|Rs\.?|INR)?\s*(\d+(?:,\d{3})*(?:\.\d{1,2})?)`)
	dateRe   = regexp.MustCompile(`\d{1,2}[/-]\d{1,2}[/-]\d{2,4}|\d{4}-\d{2}-\d{2}|\d{1,2}\s+(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*\.?\s+\d{2,4}`)
	cycleRe  = regexp.MustCompile(`(?i)(daily|weekly|monthly|quarterly|half[- ]?yearly|yearly|every\s+(?:\d+\s*)?(?:day|week|month|year))`)
)

// ParseLine converts one mandate list line into a Mandate. It must contain a
// service name and an amount; everything else is optional.
func ParseLine(raw string) (model.Mandate, error) {
	line := strings.TrimSpace(raw)
	if line == "" {
		return model.Mandate{}, fmt.Errorf("empty line")
	}
	m := model.Mandate{Raw: raw, UPIApp: "gpay", Status: "active", Cycle: "monthly"}

	rest := line

	// amount
	if loc := amountRe.FindStringSubmatch(rest); loc != nil && loc[1] != "" {
		amt, err := parseAmount(loc[1])
		if err != nil {
			return model.Mandate{}, fmt.Errorf("bad amount %q: %w", loc[1], err)
		}
		m.Amount = amt
		rest = strings.Replace(rest, loc[0], " ", 1)
	}

	// next debit date
	if loc := dateRe.FindString(rest); loc != "" {
		if d, err := parseDate(loc); err == nil {
			m.NextDebitDate = d
			rest = strings.Replace(rest, loc, " ", 1)
		}
	}

	// cycle
	if loc := cycleRe.FindString(rest); loc != "" {
		m.Cycle = normalizeCycle(loc)
		rest = strings.Replace(rest, loc, " ", 1)
	}

	// service = remaining tokens, cleaned
	service := cleanService(rest)
	if service == "" {
		return model.Mandate{}, fmt.Errorf("no service name found in %q", raw)
	}
	m.Service = service
	if m.Amount == 0 {
		return model.Mandate{}, fmt.Errorf("no amount found in %q", raw)
	}
	return m, nil
}

// ParseFile parses a whole mandate list file, one mandate per line.
// Returns the parsed mandates and any per-line errors (with line numbers).
func ParseFile(content string) ([]model.Mandate, []error) {
	var out []model.Mandate
	var errs []error
	for i, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		m, err := ParseLine(line)
		if err != nil {
			errs = append(errs, fmt.Errorf("line %d: %w", i+1, err))
			continue
		}
		out = append(out, m)
	}
	return out, errs
}

func parseAmount(s string) (int64, error) {
	clean := strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	var f float64
	if _, err := fmt.Sscanf(clean, "%f", &f); err != nil {
		return 0, err
	}
	if f <= 0 {
		return 0, fmt.Errorf("amount must be positive")
	}
	return int64(f*100 + 0.5), nil
}

func parseDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"02/01/2006", "02/01/06", "02-01-2006", "02-01-06", "2006-01-02", "02 Jan 2006", "02 Jan 06"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("unparseable date %q", s)
}

func normalizeCycle(s string) string {
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "daily"):
		return "weekly" // daily mandates are unusual; treat as weekly for now
	case strings.HasPrefix(lower, "week"):
		return "weekly"
	case strings.HasPrefix(lower, "month"), strings.Contains(lower, "every 1 month"):
		return "monthly"
	case strings.HasPrefix(lower, "quarter"):
		return "quarterly"
	case strings.HasPrefix(lower, "half"):
		return "half-yearly"
	case strings.HasPrefix(lower, "year"):
		return "yearly"
	default:
		return "monthly"
	}
}

// cleanService reduces leftover text to a display name, dropping filler and
// numeric residue (e.g. "7.66" or "05" fragments the regexes left behind).
func cleanService(s string) string {
	s = strings.ReplaceAll(s, "|", " ")
	s = strings.ReplaceAll(s, "—", " ")
	s = strings.ReplaceAll(s, "-", " ")
	fields := strings.Fields(s)
	var keep []string
	for _, f := range fields {
		lower := strings.ToLower(f)
		switch lower {
		case "next", "debit", "on", "due", "date", "autopay", "upi", "mandate", "recurring", "rs", "inr", "pay", "payment", "frequency", "every":
			continue
		}
		if isNumeric(f) {
			continue
		}
		keep = append(keep, f)
	}
	return strings.Join(keep, " ")
}

// isNumeric reports whether a token is a pure number or decimal (residue of
// an already-consumed amount or date), e.g. "3297", "7.66", "05", "2026".
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	dots := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == '.':
			dots++
			if dots > 1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
