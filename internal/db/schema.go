package db

// schema is the v1 SQLite schema. All amounts are integer paise (INR).
// This is idempotent and doubles as the migration for v1 — future schema
// changes should be applied as ordered migrations, not edits here.
const schema = `
CREATE TABLE IF NOT EXISTS subscriptions (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    service            TEXT    NOT NULL,
    category           TEXT,
    amount             INTEGER NOT NULL,                        -- paise
    currency           TEXT    NOT NULL DEFAULT 'INR',
    cycle              TEXT    NOT NULL DEFAULT 'monthly',      -- weekly|monthly|quarterly|yearly|once
    rail               TEXT    NOT NULL DEFAULT 'merchant_direct', -- upi_autopay|card_emandate|app_store|merchant_direct|bank_si
    mandate_id         TEXT,
    upi_app            TEXT,
    card_last4         TEXT,
    issuer             TEXT,
    platform           TEXT,
    next_payment_date  TEXT,                                    -- ISO YYYY-MM-DD
    last_payment_date  TEXT,                                    -- ISO YYYY-MM-DD
    status             TEXT    NOT NULL DEFAULT 'active',       -- active|paused|cancelled
    first_seen         TEXT,
    notes              TEXT,
    created_at         TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS statements (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    date         TEXT    NOT NULL,                              -- ISO YYYY-MM-DD
    value_date   TEXT,
    description  TEXT    NOT NULL,
    amount       INTEGER NOT NULL,                              -- paise; negative = debit
    balance      INTEGER,
    source_file  TEXT,
    account      TEXT,
    imported_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS imports (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    source      TEXT    NOT NULL,   -- hdfc_csv|icici_csv|hdfc_card_pdf|icici_card_pdf|gpay_pdf|gpay_mandates|manual
    format      TEXT    NOT NULL,   -- csv|pdf|xls|text
    file        TEXT,
    imported_at TEXT    NOT NULL DEFAULT (datetime('now')),
    matched     INTEGER NOT NULL DEFAULT 0,
    unmatched   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS price_history (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    amount          INTEGER NOT NULL,                           -- paise
    changed_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_statements_date      ON statements(date);
CREATE INDEX IF NOT EXISTS idx_statements_desc      ON statements(description);
CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON subscriptions(status);
CREATE INDEX IF NOT EXISTS idx_price_history_sub    ON price_history(subscription_id);
`
