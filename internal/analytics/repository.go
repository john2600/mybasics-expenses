package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository defines the data access contract for analytics queries.
type Repository interface {
	Summary(ctx context.Context, from, to time.Time) (totalSpent float64, count int, points []TrendPoint, err error)
	ByCategory(ctx context.Context, from, to time.Time) ([]CategoryBreakdown, error)
	Trend(ctx context.Context, from, to time.Time) ([]TrendPoint, error)
	TopExpenses(ctx context.Context, from, to time.Time, limit int) ([]TopExpense, error)
	IncomeVsExpense(ctx context.Context, from, to time.Time) ([]MonthIVE, error)
}

type mysqlRepository struct {
	db *sql.DB
}

// NewMySQLRepository creates a MySQL-backed analytics repository.
func NewMySQLRepository(db *sql.DB) Repository {
	return &mysqlRepository{db: db}
}

func (r *mysqlRepository) Summary(ctx context.Context, from, to time.Time) (float64, int, []TrendPoint, error) {
	// Total spent and expense count.
	var totalSpent float64
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0), COUNT(*)
		FROM   movements
		WHERE  type = 'E' AND date BETWEEN ? AND ?`,
		from.Format("2006-01-02"), to.Format("2006-01-02"),
	).Scan(&totalSpent, &count)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("querying summary totals: %w", err)
	}

	// Monthly breakdown (used for average and peak).
	points, err := r.Trend(ctx, from, to)
	if err != nil {
		return 0, 0, nil, err
	}
	return totalSpent, count, points, nil
}

func (r *mysqlRepository) ByCategory(ctx context.Context, from, to time.Time) ([]CategoryBreakdown, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.category_id, c.name, SUM(m.amount), COUNT(*)
		FROM   movements m
		JOIN   categories c ON c.id = m.category_id
		WHERE  m.type = 'E' AND m.date BETWEEN ? AND ?
		GROUP  BY m.category_id, c.name
		ORDER  BY SUM(m.amount) DESC`,
		from.Format("2006-01-02"), to.Format("2006-01-02"),
	)
	if err != nil {
		return nil, fmt.Errorf("querying by-category: %w", err)
	}
	defer rows.Close()

	var cats []CategoryBreakdown
	for rows.Next() {
		var c CategoryBreakdown
		if err := rows.Scan(&c.CategoryID, &c.Category, &c.Total, &c.Count); err != nil {
			return nil, fmt.Errorf("scanning category row: %w", err)
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func (r *mysqlRepository) Trend(ctx context.Context, from, to time.Time) ([]TrendPoint, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT YEAR(date), MONTH(date), SUM(amount), COUNT(*)
		FROM   movements
		WHERE  type = 'E' AND date BETWEEN ? AND ?
		GROUP  BY YEAR(date), MONTH(date)
		ORDER  BY YEAR(date), MONTH(date)`,
		from.Format("2006-01-02"), to.Format("2006-01-02"),
	)
	if err != nil {
		return nil, fmt.Errorf("querying trend: %w", err)
	}
	defer rows.Close()

	var points []TrendPoint
	for rows.Next() {
		var p TrendPoint
		if err := rows.Scan(&p.Year, &p.Month, &p.Total, &p.Count); err != nil {
			return nil, fmt.Errorf("scanning trend row: %w", err)
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

func (r *mysqlRepository) TopExpenses(ctx context.Context, from, to time.Time, limit int) ([]TopExpense, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.id, m.date, c.name, m.description, m.amount
		FROM   movements m
		JOIN   categories c ON c.id = m.category_id
		WHERE  m.type = 'E' AND m.date BETWEEN ? AND ?
		ORDER  BY m.amount DESC
		LIMIT  ?`,
		from.Format("2006-01-02"), to.Format("2006-01-02"), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying top expenses: %w", err)
	}
	defer rows.Close()

	var expenses []TopExpense
	for rows.Next() {
		var e TopExpense
		var date time.Time
		if err := rows.Scan(&e.ID, &date, &e.Category, &e.Description, &e.Amount); err != nil {
			return nil, fmt.Errorf("scanning top expense row: %w", err)
		}
		e.Date = date.Format("2006-01-02")
		expenses = append(expenses, e)
	}
	return expenses, rows.Err()
}

func (r *mysqlRepository) IncomeVsExpense(ctx context.Context, from, to time.Time) ([]MonthIVE, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
		  YEAR(date), MONTH(date),
		  SUM(CASE WHEN type = 'I' THEN amount ELSE 0 END),
		  SUM(CASE WHEN type = 'E' THEN amount ELSE 0 END)
		FROM   movements
		WHERE  date BETWEEN ? AND ?
		GROUP  BY YEAR(date), MONTH(date)
		ORDER  BY YEAR(date), MONTH(date)`,
		from.Format("2006-01-02"), to.Format("2006-01-02"),
	)
	if err != nil {
		return nil, fmt.Errorf("querying income-vs-expense: %w", err)
	}
	defer rows.Close()

	var months []MonthIVE
	for rows.Next() {
		var m MonthIVE
		if err := rows.Scan(&m.Year, &m.Month, &m.Income, &m.Expense); err != nil {
			return nil, fmt.Errorf("scanning ive row: %w", err)
		}
		months = append(months, m)
	}
	return months, rows.Err()
}
