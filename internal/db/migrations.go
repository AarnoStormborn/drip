package db

import (
	"database/sql"
	"fmt"
)

// migrations is the ordered list of schema migrations after the base schema
// in schema.go. Each entry must be idempotent-safe to run once; the runner
// tracks progress in PRAGMA user_version.
var migrations = []string{
	// v2: merchant descriptors on subscriptions (used by detection/reconcile)
	`ALTER TABLE subscriptions ADD COLUMN descriptors TEXT NOT NULL DEFAULT '[]';`,
	// v3: UPI AutoPay mandates (GPay mandate list) for reconcile
	`CREATE TABLE IF NOT EXISTS mandates (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		service         TEXT    NOT NULL,
		amount          INTEGER NOT NULL,
		cycle           TEXT    NOT NULL DEFAULT 'monthly',
		next_debit_date TEXT,
		upi_app         TEXT    NOT NULL DEFAULT 'gpay',
		status          TEXT    NOT NULL DEFAULT 'active',
		raw             TEXT,
		notes           TEXT,
		created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
		updated_at      TEXT    NOT NULL DEFAULT (datetime('now'))
	);`,
}

// migrate applies the base schema, then any pending migrations, tracking
// progress in PRAGMA user_version.
func migrate(sqldb *sql.DB) error {
	if _, err := sqldb.Exec(schema); err != nil {
		return fmt.Errorf("base schema: %w", err)
	}

	var version int
	if err := sqldb.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}

	for i := version; i < len(migrations); i++ {
		if _, err := sqldb.Exec(migrations[i]); err != nil {
			return fmt.Errorf("migration %d: %w", i+2, err)
		}
		if _, err := sqldb.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, i+2)); err != nil {
			return fmt.Errorf("set user_version %d: %w", i+2, err)
		}
	}
	return nil
}
