package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"drip/internal/model"
)

const subCols = `id, service, category, amount, currency, cycle, rail, mandate_id,
	upi_app, card_last4, issuer, platform, next_payment_date, last_payment_date,
	status, first_seen, descriptors, notes, created_at, updated_at`

type scanner interface {
	Scan(dest ...any) error
}

func scanSubscription(s scanner) (model.Subscription, error) {
	var sub model.Subscription
	err := s.Scan(&sub.ID, &sub.Service, &sub.Category, &sub.Amount, &sub.Currency,
		&sub.Cycle, &sub.Rail, &sub.MandateID, &sub.UPIApp, &sub.CardLast4,
		&sub.Issuer, &sub.Platform, &sub.NextPaymentDate, &sub.LastPaymentDate,
		&sub.Status, &sub.FirstSeen, &sub.Descriptors, &sub.Notes, &sub.CreatedAt, &sub.UpdatedAt)
	return sub, err
}

// ListSubscriptions returns subscriptions ordered by next payment date
// (earliest first). Pass status "" for all, or "active"/"paused"/"cancelled".
func ListSubscriptions(ctx context.Context, q *sql.DB, status string) ([]model.Subscription, error) {
	query := `SELECT ` + subCols + ` FROM subscriptions`
	args := []any{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY COALESCE(NULLIF(next_payment_date, ''), '9999-12-31') ASC, service COLLATE NOCASE ASC`

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []model.Subscription
	for rows.Next() {
		sub, err := scanSubscription(rows)
		if err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// GetSubscription fetches one subscription by id.
func GetSubscription(ctx context.Context, q *sql.DB, id int64) (model.Subscription, error) {
	row := q.QueryRowContext(ctx, `SELECT `+subCols+` FROM subscriptions WHERE id = ?`, id)
	sub, err := scanSubscription(row)
	if err != nil {
		return model.Subscription{}, fmt.Errorf("get subscription %d: %w", id, err)
	}
	return sub, nil
}

// CreateSubscription inserts a subscription and returns its new id.
func CreateSubscription(ctx context.Context, q *sql.DB, s *model.Subscription) (int64, error) {
	res, err := q.ExecContext(ctx, `
		INSERT INTO subscriptions
			(service, category, amount, currency, cycle, rail, mandate_id, upi_app,
			 card_last4, issuer, platform, next_payment_date, last_payment_date,
			 status, first_seen, descriptors, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.Service, s.Category, s.Amount, s.Currency, s.Cycle, s.Rail, s.MandateID,
		s.UPIApp, s.CardLast4, s.Issuer, s.Platform, s.NextPaymentDate,
		s.LastPaymentDate, s.Status, s.FirstSeen, s.Descriptors, s.Notes)
	if err != nil {
		return 0, fmt.Errorf("create subscription: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create subscription id: %w", err)
	}
	return id, nil
}

// UpdateSubscription overwrites a subscription by id.
func UpdateSubscription(ctx context.Context, q *sql.DB, s *model.Subscription) error {
	if _, err := q.ExecContext(ctx, `
		UPDATE subscriptions SET
			service = ?, category = ?, amount = ?, currency = ?, cycle = ?, rail = ?,
			mandate_id = ?, upi_app = ?, card_last4 = ?, issuer = ?, platform = ?,
			next_payment_date = ?, last_payment_date = ?, status = ?, first_seen = ?,
			descriptors = ?, notes = ?, updated_at = datetime('now')
		WHERE id = ?`,
		s.Service, s.Category, s.Amount, s.Currency, s.Cycle, s.Rail, s.MandateID,
		s.UPIApp, s.CardLast4, s.Issuer, s.Platform, s.NextPaymentDate,
		s.LastPaymentDate, s.Status, s.FirstSeen, s.Descriptors, s.Notes, s.ID); err != nil {
		return fmt.Errorf("update subscription %d: %w", s.ID, err)
	}
	return nil
}

// SetSubscriptionStatus flips a subscription's status.
func SetSubscriptionStatus(ctx context.Context, q *sql.DB, id int64, status string) error {
	if _, err := q.ExecContext(ctx,
		`UPDATE subscriptions SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		status, id); err != nil {
		return fmt.Errorf("set subscription %d status: %w", id, err)
	}
	return nil
}

// GetSubscriptionByService fetches a subscription by exact service name.
// Returns ok=false when none exists.
func GetSubscriptionByService(ctx context.Context, q *sql.DB, service string) (model.Subscription, bool, error) {
	row := q.QueryRowContext(ctx, `SELECT `+subCols+` FROM subscriptions WHERE service = ?`, service)
	sub, err := scanSubscription(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Subscription{}, false, nil
	}
	if err != nil {
		return model.Subscription{}, false, fmt.Errorf("get subscription by service %q: %w", service, err)
	}
	return sub, true, nil
}

// InsertPriceHistory records an amount change for a subscription.
func InsertPriceHistory(ctx context.Context, q *sql.DB, subID, amount int64) (int64, error) {
	res, err := q.ExecContext(ctx,
		`INSERT INTO price_history (subscription_id, amount) VALUES (?, ?)`, subID, amount)
	if err != nil {
		return 0, fmt.Errorf("insert price history: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("insert price history id: %w", err)
	}
	return id, nil
}

// DeleteSubscription removes a subscription (price_history cascades).
func DeleteSubscription(ctx context.Context, q *sql.DB, id int64) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM subscriptions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete subscription %d: %w", id, err)
	}
	return nil
}
