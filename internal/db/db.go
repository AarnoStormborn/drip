// Package db owns the SQLite connection, schema, and data access.
//
// The database file is personal financial data and must never be committed —
// see .gitignore (/data/ is ignored; the CLI default lives outside the repo).
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no CGO
)

// Open opens (creating if needed) the SQLite database at path, applies the
// schema, and returns a ready connection.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir %s: %w", dir, err)
		}
	}

	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := sqldb.Ping(); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("ping %s: %w", path, err)
	}
	if _, err := sqldb.Exec("PRAGMA journal_mode = WAL; PRAGMA foreign_keys = ON;"); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("configure %s: %w", path, err)
	}
	if err := migrate(sqldb); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("migrate %s: %w", path, err)
	}
	return sqldb, nil
}

func migrate(sqldb *sql.DB) error {
	if _, err := sqldb.Exec(schema); err != nil {
		return err
	}
	return nil
}
