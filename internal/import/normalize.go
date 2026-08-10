// Package importer turns raw statement files (PDF, CSV, XLSX, …) into
// normalized model.StatementRecord rows. Parsers are registered per
// (source, format) pair; see detect.go.
package importer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Money is stored as integer paise (INR). A debit is negative.
func parseMoney(s string) (int64, error) {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse money %q: %w", s, err)
	}
	return int64(f*100 + 0.5), nil
}

// parseDDMMYY converts "01/05/26" to ISO "2026-05-01".
// yy >= 50 is treated as 19yy, otherwise 20yy.
func parseDDMMYY(s string) (string, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, "/")
	if len(parts) != 3 {
		return "", fmt.Errorf("parse date %q: want DD/MM/YY", s)
	}
	dd, err1 := strconv.Atoi(parts[0])
	mm, err2 := strconv.Atoi(parts[1])
	yy, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return "", fmt.Errorf("parse date %q: non-numeric component", s)
	}
	if len(parts[2]) != 2 {
		return "", fmt.Errorf("parse date %q: year must be 2 digits (DD/MM/YY)", s)
	}
	if dd < 1 || dd > 31 || mm < 1 || mm > 12 {
		return "", fmt.Errorf("parse date %q: out of range", s)
	}
	century := 2000
	if yy >= 50 {
		century = 1900
	}
	return fmt.Sprintf("%04d-%02d-%02d", century+yy, mm, dd), nil
}

var (
	dateRE  = regexp.MustCompile(`^\d{2}/\d{2}/\d{2}$`)
	moneyRE = regexp.MustCompile(`^\d{1,3}(,\d{3})*\.\d{2}$`)
	// HDFC cheque/ref numbers: 16 digits, usually leading zeros.
	refRE = regexp.MustCompile(`^0\d{14,15}$`)
)

// classifyMoneySide guesses debit vs credit from the amount cell's right
// edge (x + approx char width), used only as a fallback when running-balance
// math cannot resolve the sign. HDFC right-aligns amounts into two columns:
// withdrawals end near x≈472pt, deposits near x≈550pt.
func classifyMoneySide(startX float64, raw string) int {
	rightEdge := startX + 4.0*float64(len(raw))
	if rightEdge < 500 {
		return -1 // withdrawal column
	}
	return 1 // deposit column
}
