package db

import (
	"context"
	"database/sql"
	"fmt"

	"drip/internal/model"
)

// ListPriceHistory returns a subscription's amount changes, newest first.
func ListPriceHistory(ctx context.Context, q *sql.DB, subID int64) ([]model.PriceHistory, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, subscription_id, amount, changed_at
		FROM price_history WHERE subscription_id = ? ORDER BY id DESC`, subID)
	if err != nil {
		return nil, fmt.Errorf("list price history %d: %w", subID, err)
	}
	defer rows.Close()

	var hist []model.PriceHistory
	for rows.Next() {
		var h model.PriceHistory
		if err := rows.Scan(&h.ID, &h.SubscriptionID, &h.Amount, &h.ChangedAt); err != nil {
			return nil, fmt.Errorf("scan price history: %w", err)
		}
		hist = append(hist, h)
	}
	return hist, rows.Err()
}
