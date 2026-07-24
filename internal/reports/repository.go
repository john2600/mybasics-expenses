package reports

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository defines the data access contract for report generation.
type Repository interface {
	QueryExpenses(ctx context.Context, from, to time.Time) ([]ExportRow, error)
	MonthlySummary(ctx context.Context, from, to time.Time) ([]MonthlySummary, error)
}

type mysqlRepository struct {
	db *sql.DB
}

// NewMySQLRepository creates a MySQL-backed reports repository.
func NewMySQLRepository(db *sql.DB) Repository {
	return &mysqlRepository{db: db}
}

func (r *mysqlRepository) QueryExpenses(ctx context.Context, from, to time.Time) ([]ExportRow, error) {
	const q = `
		SELECT m.id, m.date, c.name, m.description, m.amount
		FROM   movements m
		JOIN   categories c ON c.id = m.category_id
		WHERE  m.type = 'E' AND m.date BETWEEN ? AND ?
		ORDER  BY m.date DESC`

	rows, err := r.db.QueryContext(ctx, q, from.Format(time.DateOnly), to.Format(time.DateOnly))
	if err != nil {
		return nil, fmt.Errorf("querying expenses for export: %w", err)
	}
	defer rows.Close()

	var expenses []ExportRow
	for rows.Next() {
		var e ExportRow
		var date time.Time
		if err := rows.Scan(&e.ID, &date, &e.Category, &e.Description, &e.Amount); err != nil {
			return nil, fmt.Errorf("scanning expense row: %w", err)
		}
		e.Date = date.Format(time.DateOnly)
		expenses = append(expenses, e)
	}
	return expenses, rows.Err()
}

func (r *mysqlRepository) MonthlySummary(ctx context.Context, from, to time.Time) ([]MonthlySummary, error) {
	const q = `
		SELECT YEAR(date), MONTH(date), SUM(amount)
		FROM   movements
		WHERE  type = 'E' AND date BETWEEN ? AND ?
		GROUP  BY YEAR(date), MONTH(date)
		ORDER  BY YEAR(date), MONTH(date)`

	rows, err := r.db.QueryContext(ctx, q, from.Format(time.DateOnly), to.Format(time.DateOnly))
	if err != nil {
		return nil, fmt.Errorf("querying monthly summary: %w", err)
	}
	defer rows.Close()

	var summaries []MonthlySummary
	for rows.Next() {
		var s MonthlySummary
		if err := rows.Scan(&s.Year, &s.Month, &s.Total); err != nil {
			return nil, fmt.Errorf("scanning summary row: %w", err)
		}
		s.Label = monthLabel(s.Year, s.Month)
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

func monthLabel(year, month int) string {
	t := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	return t.Format("Jan 2006")
}
