// Package model holds the domain types for Drip.
//
// Money is stored as integer paise (INR) everywhere to avoid float
// rounding errors. A negative amount means a debit / money going out.
package model

// Subscription is one tracked recurring payment.
type Subscription struct {
	ID              int64
	Service         string // e.g. "Netflix", "YouTube Premium"
	Category        string // e.g. "streaming", "storage", "music"
	Amount          int64  // paise, always positive (the charge amount)
	Currency        string // "INR"
	Cycle           string // weekly|monthly|quarterly|yearly|once
	Rail            string // upi_autopay|card_emandate|app_store|merchant_direct|bank_si
	MandateID       string // UPI AutoPay mandate id when rail == upi_autopay
	UPIApp          string // e.g. "gpay"
	CardLast4       string // last 4 digits when rail == card_emandate
	Issuer          string // e.g. "HDFC", "ICICI"
	Platform        string // e.g. "google_play", "apple"
	NextPaymentDate string // ISO YYYY-MM-DD
	LastPaymentDate string // ISO YYYY-MM-DD
	Status          string // active|paused|cancelled
	FirstSeen       string // ISO YYYY-MM-DD (when the sub was first detected)
	Notes           string
	CreatedAt       string
	UpdatedAt       string
}

// StatementRecord is one line from a parsed bank / card / GPay statement.
type StatementRecord struct {
	ID          int64
	Date        string // transaction date, ISO YYYY-MM-DD
	ValueDate   string // optional value date
	Description string // raw merchant descriptor, e.g. "UPI/…/NETFLIX/NETFLIX.COM"
	Amount      int64  // paise; negative = debit
	Balance     *int64 // paise, nullable
	SourceFile  string
	Account     string // e.g. "HDFC-savings", "HDFC-card-3456"
	ImportedAt  string
}

// ImportAudit records one import run.
type ImportAudit struct {
	ID         int64
	Source     string // hdfc_csv|icici_csv|hdfc_card_pdf|icici_card_pdf|gpay_pdf|gpay_mandates|manual
	Format     string // csv|pdf|xls|text
	File       string
	ImportedAt string
	Matched    int
	Unmatched  int
}

// PriceHistory records an amount change for a subscription.
type PriceHistory struct {
	ID             int64
	SubscriptionID int64
	Amount         int64 // paise
	ChangedAt      string
}
