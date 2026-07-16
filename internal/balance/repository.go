package balance

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository defines the data access contract for balance.
type Repository interface {
	GetSummary(ctx context.Context, dateFrom, dateTo *time.Time) (*Summary, error)
	GetPeriodSummary(ctx context.Context, from, to time.Time) (expenses, incomes float64, err error)
	GetEarliestMovementDate(ctx context.Context) (*time.Time, error)
}

type mysqlRepository struct {
	db *sql.DB
}

// NewMySQLRepository creates a MySQL-backed balance Repository.
func NewMySQLRepository(db *sql.DB) Repository {
	return &mysqlRepository{db: db}
}

func (r *mysqlRepository) GetSummary(ctx context.Context, dateFrom, dateTo *time.Time) (*Summary, error) {
	const q = `
		SELECT
			COALESCE(SUM(CASE WHEN type = 'E' THEN amount ELSE 0 END), 0) AS expenses,
			COALESCE(SUM(CASE WHEN type = 'I' THEN amount ELSE 0 END), 0) AS incomes
		FROM movements
		WHERE (? IS NULL OR date >= ?)
		  AND (? IS NULL OR date <= ?)`

	var expenses, incomes float64
	err := r.db.QueryRowContext(ctx, q, dateFrom, dateFrom, dateTo, dateTo).
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

func (r *mysqlRepository) GetPeriodSummary(ctx context.Context, from, to time.Time) (float64, float64, error) {
	const q = `
		SELECT
			COALESCE(SUM(CASE WHEN type = 'E' THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN type = 'I' THEN amount ELSE 0 END), 0)
		FROM movements
		WHERE date >= ? AND date < ?`

	var expenses, incomes float64
	if err := r.db.QueryRowContext(ctx, q, from, to).Scan(&expenses, &incomes); err != nil {
		return 0, 0, fmt.Errorf("querying period summary: %w", err)
	}
	return expenses, incomes, nil
}

func (r *mysqlRepository) GetEarliestMovementDate(ctx context.Context) (*time.Time, error) {
	const q = `SELECT MIN(date) FROM movements`

	var d sql.NullTime
	if err := r.db.QueryRowContext(ctx, q).Scan(&d); err != nil {
		return nil, fmt.Errorf("querying earliest movement date: %w", err)
	}
	if !d.Valid {
		return nil, nil
	}
	return &d.Time, nil
}
