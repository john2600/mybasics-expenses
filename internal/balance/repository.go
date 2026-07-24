package balance

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository defines the data access contract for balance.
// Every query is scoped to a single user via userID.
type Repository interface {
	GetSummary(ctx context.Context, userID int, dateFrom, dateTo *time.Time) (*Summary, error)
	GetPeriodSummary(ctx context.Context, userID int, from, to time.Time) (expenses, incomes float64, err error)
	GetEarliestMovementDate(ctx context.Context, userID int) (*time.Time, error)
}

type mysqlRepository struct {
	db *sql.DB
}

// NewMySQLRepository creates a MySQL-backed balance Repository.
func NewMySQLRepository(db *sql.DB) Repository {
	return &mysqlRepository{db: db}
}

func (r *mysqlRepository) GetSummary(ctx context.Context, userID int, dateFrom, dateTo *time.Time) (*Summary, error) {
	const q = `
		SELECT
			COALESCE(SUM(CASE WHEN type = 'E' THEN amount ELSE 0 END), 0) AS expenses,
			COALESCE(SUM(CASE WHEN type = 'I' THEN amount ELSE 0 END), 0) AS incomes
		FROM movements
		WHERE user_id = ?
		  AND (? IS NULL OR date >= ?)
		  AND (? IS NULL OR date <= ?)`

	var expenses, incomes float64
	err := r.db.QueryRowContext(ctx, q, userID, dateFrom, dateFrom, dateTo, dateTo).
		Scan(&expenses, &incomes)
	if err != nil {
		return nil, fmt.Errorf("querying balance summary: %w", err)
	}

	return &Summary{
		Expenses: expenses,
		Incomes:  incomes,
		Balance:  incomes - expenses,
	}, nil
}

func (r *mysqlRepository) GetPeriodSummary(ctx context.Context, userID int, from, to time.Time) (float64, float64, error) {
	const q = `
		SELECT
			COALESCE(SUM(CASE WHEN type = 'E' THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN type = 'I' THEN amount ELSE 0 END), 0)
		FROM movements
		WHERE user_id = ? AND date >= ? AND date < ?`

	var expenses, incomes float64
	if err := r.db.QueryRowContext(ctx, q, userID, from, to).Scan(&expenses, &incomes); err != nil {
		return 0, 0, fmt.Errorf("querying period summary: %w", err)
	}
	return expenses, incomes, nil
}

func (r *mysqlRepository) GetEarliestMovementDate(ctx context.Context, userID int) (*time.Time, error) {
	const q = `SELECT MIN(date) FROM movements WHERE user_id = ?`

	var d sql.NullTime
	if err := r.db.QueryRowContext(ctx, q, userID).Scan(&d); err != nil {
		return nil, fmt.Errorf("querying earliest movement date: %w", err)
	}
	if !d.Valid {
		return nil, nil
	}
	return &d.Time, nil
}
