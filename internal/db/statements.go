package db

import (
	"context"
	"database/sql"
	"fmt"

	"drip/internal/model"
)

// InsertStatements batch-inserts parsed statement records in a transaction.
// It returns the number of rows inserted.
func InsertStatements(ctx context.Context, q *sql.DB, recs []model.StatementRecord) (int64, error) {
	if len(recs) == 0 {
		return 0, nil
	}
	tx, err := q.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin statements tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO statements (date, value_date, description, amount, balance, source_file, account)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare statements insert: %w", err)
	}
	defer stmt.Close()

	var n int64
	for _, r := range recs {
		var bal any
		if r.Balance != nil {
			bal = *r.Balance
		}
		if _, err := stmt.ExecContext(ctx, r.Date, r.ValueDate, r.Description, r.Amount, bal, r.SourceFile, r.Account); err != nil {
			return n, fmt.Errorf("insert statement %q: %w", r.Description, err)
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return n, fmt.Errorf("commit statements: %w", err)
	}
	return n, nil
}

// CountStatements returns the total number of raw statement records stored.
func CountStatements(ctx context.Context, q *sql.DB) (int, error) {
	var n int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM statements`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count statements: %w", err)
	}
	return n, nil
}

// ListDebits returns all debit statement records (amount < 0), oldest first.
// These are the raw inputs for recurring-detection.
func ListDebits(ctx context.Context, q *sql.DB) ([]model.StatementRecord, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, date, value_date, description, amount, balance, source_file, account, imported_at
		FROM statements WHERE amount < 0 ORDER BY date ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list debits: %w", err)
	}
	defer rows.Close()

	var recs []model.StatementRecord
	for rows.Next() {
		var r model.StatementRecord
		var bal sql.NullInt64
		if err := rows.Scan(&r.ID, &r.Date, &r.ValueDate, &r.Description, &r.Amount, &bal, &r.SourceFile, &r.Account, &r.ImportedAt); err != nil {
			return nil, fmt.Errorf("scan debit: %w", err)
		}
		if bal.Valid {
			b := bal.Int64
			r.Balance = &b
		}
		recs = append(recs, r)
	}
	return recs, rows.Err()
}

// ListStatements returns the most recent statement records (newest first).
func ListStatements(ctx context.Context, q *sql.DB, limit int) ([]model.StatementRecord, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, date, value_date, description, amount, balance, source_file, account, imported_at
		FROM statements ORDER BY date DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list statements: %w", err)
	}
	defer rows.Close()

	var recs []model.StatementRecord
	for rows.Next() {
		var r model.StatementRecord
		var bal sql.NullInt64
		if err := rows.Scan(&r.ID, &r.Date, &r.ValueDate, &r.Description, &r.Amount, &bal, &r.SourceFile, &r.Account, &r.ImportedAt); err != nil {
			return nil, fmt.Errorf("scan statement: %w", err)
		}
		if bal.Valid {
			b := bal.Int64
			r.Balance = &b
		}
		recs = append(recs, r)
	}
	return recs, rows.Err()
}
