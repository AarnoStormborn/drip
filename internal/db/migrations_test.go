package db

import (
	"testing"
)

// TestMigrationFromV2EraDB simulates a database created by an M3/M6-era build:
// descriptors migration applied (user_version=2) but no mandates table yet.
// The runner must apply only the pending v3 migration — this is the exact bug
// that made `drip mandates list` fail with "no such table: mandates" on
// pre-existing databases.
func TestMigrationFromV2EraDB(t *testing.T) {
	conn := openTest(t) // fresh DB: schema + v2 + v3 → user_version=3

	// rewind to the M3-era state: drop the mandates table, reset version
	if _, err := conn.Exec(`DROP TABLE mandates`); err != nil {
		t.Fatalf("drop mandates: %v", err)
	}
	if _, err := conn.Exec(`PRAGMA user_version = 2`); err != nil {
		t.Fatalf("set user_version: %v", err)
	}

	// re-run migrations — v2 must be skipped (already applied), v3 applied
	if err := migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var name string
	err := conn.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='mandates'`).Scan(&name)
	if err != nil {
		t.Fatalf("mandates table missing after re-migrate: %v", err)
	}

	var v int
	if err := conn.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if v != 3 {
		t.Errorf("user_version = %d, want 3", v)
	}

	// descriptors column must still exist (v2 wasn't re-run / clobbered)
	var cols int
	if err := conn.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('subscriptions') WHERE name='descriptors'`).Scan(&cols); err != nil {
		t.Fatalf("pragma_table_info: %v", err)
	}
	if cols != 1 {
		t.Errorf("descriptors column missing after re-migrate")
	}

	// idempotent: a second migrate is a no-op
	if err := migrate(conn); err != nil {
		t.Fatalf("migrate 2: %v", err)
	}
	if err := conn.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatalf("read user_version 2: %v", err)
	}
	if v != 3 {
		t.Errorf("user_version after second migrate = %d, want 3", v)
	}
}
