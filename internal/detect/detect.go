package detect

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"drip/internal/db"
	"drip/internal/model"
)

// Result summarizes one detection run.
type Result struct {
	Created    int // subscriptions created
	Updated    int // subscriptions refreshed
	Candidates int // merchant debits seen once (not yet recurring) — not created
	Total      int // distinct recurring patterns found
}

// Run scans all debit statement records, detects recurring patterns, and
// upserts subscriptions. Idempotent: re-running refreshes dates/prices.
//
// Records are grouped by resolved service name (multiple merchant keys can
// map to one service, e.g. "WWWAIRTEL" and "AIRTEL" → Airtel). The dominant
// (most frequent, tie → most recent) amount is chosen for the subscription;
// variable charges (e.g. Apple Media ₹75/₹219/₹499) stay in the ledger and
// descriptor list without churning the stored amount each run.
func Run(ctx context.Context, q *sql.DB) (Result, error) {
	recs, err := db.ListDebits(ctx, q)
	if err != nil {
		return Result{}, err
	}

	// Group by service name.
	services := map[string][]model.StatementRecord{}
	merchantOf := map[string]string{}
	for _, r := range recs {
		m, ok := ExtractMerchant(r.Description)
		if !ok {
			continue
		}
		svc := ServiceName(m)
		services[svc] = append(services[svc], r)
		if merchantOf[svc] == "" {
			merchantOf[svc] = m
		}
	}

	var res Result
	for svc, recs := range services {
		autopay := anyAutopay(recs)

		// dominant amount (positive; records are debits): most frequent,
		// tie → most recent occurrence
		counts := map[int64]int{}
		latestOf := map[int64]string{}
		for _, r := range recs {
			amt := -r.Amount
			counts[amt]++
			if d := latestOf[amt]; d == "" || r.Date > d {
				latestOf[amt] = r.Date
			}
		}
		var dominant int64
		best := 0
		for amt, n := range counts {
			if n > best || (n == best && latestOf[amt] > latestOf[dominant]) {
				dominant, best = amt, n
			}
		}

		dates := distinctDatesOfAmount(recs, dominant)
		if !autopay && len(dates) < 2 {
			res.Candidates++
			continue
		}

		cycle, next := inferCycle(dates)
		rail := "merchant_direct"
		if autopay {
			rail = "upi_autopay"
		}

		created, err := upsert(ctx, q, svc, merchantOf[svc], dominant, cycle, rail,
			dates[0], dates[len(dates)-1], next, recs)
		if err != nil {
			return res, fmt.Errorf("upsert %s: %w", svc, err)
		}
		if created {
			res.Created++
		} else {
			res.Updated++
		}
		res.Total++
	}
	return res, nil
}

// anyAutopay reports whether any record in the group carries the UPI AutoPay
// e-mandate marker.
func anyAutopay(group []model.StatementRecord) bool {
	for _, r := range group {
		if containsKey(r.Description, "upiautopay") {
			return true
		}
	}
	return false
}

func containsKey(desc, key string) bool {
	k := NormalizeDescriptor(desc)
	return len(k) >= len(key) && (k == key || (len(k) > len(key) && contains(k, key)))
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// distinctDatesOfAmount returns the unique ISO dates of records matching the
// given (positive) amount, sorted. Used to infer cadence for the dominant
// amount. Ledger amounts are negative debits, so both signs are accepted.
func distinctDatesOfAmount(group []model.StatementRecord, amount int64) []string {
	set := map[string]bool{}
	for _, r := range group {
		amt := r.Amount
		if amt < 0 {
			amt = -amt
		}
		if amt == amount && r.Date != "" {
			set[r.Date] = true
		}
	}
	dates := make([]string, 0, len(set))
	for d := range set {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	return dates
}

// inferCycle classifies the cadence from the median gap between consecutive
// charges and returns the cycle plus the projected next payment date.
func inferCycle(dates []string) (cycle, next string) {
	parsed := make([]time.Time, 0, len(dates))
	for _, d := range dates {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			parsed = append(parsed, t)
		}
	}
	last := parsed[len(parsed)-1]

	var gaps []int
	for i := 1; i < len(parsed); i++ {
		gaps = append(gaps, int(parsed[i].Sub(parsed[i-1]).Hours()/24))
	}
	if len(gaps) == 0 {
		return "monthly", last.AddDate(0, 1, 0).Format("2006-01-02")
	}
	sort.Ints(gaps)
	median := gaps[len(gaps)/2]

	switch {
	case median >= 5 && median <= 9:
		return "weekly", last.AddDate(0, 0, 7).Format("2006-01-02")
	case median >= 26 && median <= 34:
		return "monthly", last.AddDate(0, 1, 0).Format("2006-01-02")
	case median >= 85 && median <= 95:
		return "quarterly", last.AddDate(0, 3, 0).Format("2006-01-02")
	case median >= 170 && median <= 185:
		return "half-yearly", last.AddDate(0, 6, 0).Format("2006-01-02")
	case median >= 350 && median <= 380:
		return "yearly", last.AddDate(1, 0, 0).Format("2006-01-02")
	default:
		return "monthly", last.AddDate(0, 1, 0).Format("2006-01-02")
	}
}

// upsert creates or refreshes a subscription, recording a price change when
// the amount moved. firstDate/lastDate are the earliest/latest charges of the
// dominant amount; descriptors covers all records of the service.
func upsert(ctx context.Context, q *sql.DB, service, merchant string, amount int64, cycle, rail, firstDate, lastDate, nextDate string, group []model.StatementRecord) (created bool, err error) {
	existing, ok, err := db.GetSubscriptionByService(ctx, q, service)
	if err != nil {
		return false, err
	}

	descriptors := descriptorsJSON(group)

	if !ok {
		_, err := db.CreateSubscription(ctx, q, &model.Subscription{
			Service:         service,
			Amount:          amount,
			Currency:        "INR",
			Cycle:           cycle,
			Rail:            rail,
			NextPaymentDate: nextDate,
			LastPaymentDate: lastDate,
			Status:          "active",
			FirstSeen:       firstDate,
			Descriptors:     descriptors,
		})
		return true, err
	}

	updated := existing
	updated.Amount = amount
	updated.Cycle = cycle
	updated.Rail = rail
	updated.LastPaymentDate = lastDate
	updated.NextPaymentDate = nextDate
	updated.Descriptors = mergeDescriptors(existing.Descriptors, descriptors)

	if existing.Status == "cancelled" {
		// a fresh debit proves the subscription is still alive
		updated.Status = "active"
		updated.Notes = existing.Notes + " [reactivated by detection]"
	}
	if existing.Amount != amount && existing.Amount != 0 {
		if _, err := db.InsertPriceHistory(ctx, q, existing.ID, amount); err != nil {
			return false, err
		}
	}
	if err := db.UpdateSubscription(ctx, q, &updated); err != nil {
		return false, err
	}
	return false, nil
}

// descriptorsJSON builds the JSON array of normalized descriptor keys.
func descriptorsJSON(group []model.StatementRecord) string {
	set := map[string]bool{}
	for _, r := range group {
		k := NormalizeDescriptor(r.Description)
		if k != "" {
			set[k] = true
		}
	}
	list := make([]string, 0, len(set))
	for k := range set {
		list = append(list, k)
	}
	sort.Strings(list)
	b, _ := json.Marshal(list)
	return string(b)
}

// mergeDescriptors unions two JSON descriptor arrays.
func mergeDescriptors(a, b string) string {
	var merged []string
	for _, raw := range []string{a, b} {
		var list []string
		_ = json.Unmarshal([]byte(raw), &list)
		merged = append(merged, list...)
	}
	set := map[string]bool{}
	for _, k := range merged {
		set[k] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	raw, _ := json.Marshal(out)
	return string(raw)
}
