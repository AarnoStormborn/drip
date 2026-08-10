package db

import (
	"context"
	"database/sql"
	"fmt"

	"drip/internal/model"
)

// InsertImport records an import audit row and returns its id.
func InsertImport(ctx context.Context, q *sql.DB, a *model.ImportAudit) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO imports (source, format, file, matched, unmatched)
		VALUES (?, ?, ?, ?, ?)`,
		a.Source, a.Format, a.File, a.Matched, a.Unmatched)
	if err != nil {
		return 0, fmt.Errorf("insert import audit: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("insert import audit id: %w", err)
	}
	return id, nil
}

// ListImports returns the most recent import audits, newest first.
func ListImports(ctx context.Context, q *sql.DB, limit int) ([]model.ImportAudit, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, source, format, file, imported_at, matched, unmatched
		FROM imports ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list imports: %w", err)
	}
	defer rows.Close()

	var audits []model.ImportAudit
	for rows.Next() {
		var a model.ImportAudit
		if err := rows.Scan(&a.ID, &a.Source, &a.Format, &a.File, &a.ImportedAt, &a.Matched, &a.Unmatched); err != nil {
			return nil, fmt.Errorf("scan import audit: %w", err)
		}
		audits = append(audits, a)
	}
	return audits, rows.Err()
}

// AllStatementKeys returns a set of "date|description|amount" keys for every
// stored statement record, used to deduplicate re-imports.
func AllStatementKeys(ctx context.Context, q *sql.DB) (map[string]bool, error) {
	rows, err := q.QueryContext(ctx, `SELECT date, description, amount FROM statements`)
	if err != nil {
		return nil, fmt.Errorf("select statement keys: %w", err)
	}
	defer rows.Close()

	keys := make(map[string]bool)
	for rows.Next() {
		var date, desc string
		var amount int64
		if err := rows.Scan(&date, &desc, &amount); err != nil {
			return nil, fmt.Errorf("scan statement key: %w", err)
		}
		keys[date+"|"+desc+"|"+fmt.Sprint(amount)] = true
	}
	return keys, rows.Err()
}
