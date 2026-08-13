package db

import (
	"context"
	"database/sql"
	"fmt"

	"drip/internal/model"
)

const mandateCols = `id, service, amount, cycle, next_debit_date, upi_app, status, raw, notes, created_at, updated_at`

// ListMandates returns stored mandates, active first, newest first.
func ListMandates(ctx context.Context, q *sql.DB, status string) ([]model.Mandate, error) {
	query := `SELECT ` + mandateCols + ` FROM mandates`
	args := []any{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY (status = 'active') DESC, service COLLATE NOCASE ASC, id DESC`

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list mandates: %w", err)
	}
	defer rows.Close()

	var out []model.Mandate
	for rows.Next() {
		m, err := scanMandate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanMandate(s scanner) (model.Mandate, error) {
	var m model.Mandate
	err := s.Scan(&m.ID, &m.Service, &m.Amount, &m.Cycle, &m.NextDebitDate, &m.UPIApp,
		&m.Status, &m.Raw, &m.Notes, &m.CreatedAt, &m.UpdatedAt)
	return m, err
}

// CreateMandate inserts a mandate and returns its id.
func CreateMandate(ctx context.Context, q *sql.DB, m *model.Mandate) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO mandates (service, amount, cycle, next_debit_date, upi_app, status, raw, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.Service, m.Amount, m.Cycle, m.NextDebitDate, m.UPIApp, m.Status, m.Raw, m.Notes)
	if err != nil {
		return 0, fmt.Errorf("create mandate: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create mandate id: %w", err)
	}
	return id, nil
}

// DeleteMandate removes a mandate by id.
func DeleteMandate(ctx context.Context, q *sql.DB, id int64) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM mandates WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete mandate %d: %w", id, err)
	}
	return nil
}

// CountMandates returns how many mandates are stored.
func CountMandates(ctx context.Context, q *sql.DB) (int, error) {
	var n int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM mandates`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count mandates: %w", err)
	}
	return n, nil
}
