package incomes

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository defines the data access contract for income config.
type Repository interface {
	// Get returns the most recent income config entry.
	Get(ctx context.Context) (*Config, error)
	// GetForMonth returns the most recent entry whose year_month <= month.
	// Falls back to Get if no entry exists before that month.
	GetForMonth(ctx context.Context, month time.Time) (*Config, error)
	// Update upserts a config entry for the given month (defaults to current month).
	Update(ctx context.Context, req UpdateRequest) (*Config, error)
}

type mysqlRepository struct {
	db *sql.DB
}

// NewMySQLRepository creates a MySQL-backed incomes Repository.
func NewMySQLRepository(db *sql.DB) Repository {
	return &mysqlRepository{db: db}
}

const selectFields = "SELECT `year_month`, amount, cut_day, description, created_at FROM income_config_history"

func scanConfig(row *sql.Row) (*Config, error) {
	var c Config
	if err := row.Scan(&c.YearMonth, &c.Amount, &c.CutDay, &c.Description, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *mysqlRepository) Get(ctx context.Context) (*Config, error) {
	q := selectFields + " ORDER BY `year_month` DESC LIMIT 1"
	c, err := scanConfig(r.db.QueryRowContext(ctx, q))
	if err != nil {
		return nil, fmt.Errorf("fetching income config: %w", err)
	}
	return c, nil
}

func (r *mysqlRepository) GetForMonth(ctx context.Context, month time.Time) (*Config, error) {
	firstDay := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	q := selectFields + " WHERE `year_month` <= ? ORDER BY `year_month` DESC LIMIT 1"
	c, err := scanConfig(r.db.QueryRowContext(ctx, q, firstDay))
	if err == sql.ErrNoRows {
		// No entry at or before this month — use the latest known.
		return r.Get(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("fetching income config for %s: %w", firstDay.Format("2006-01"), err)
	}
	return c, nil
}

func (r *mysqlRepository) Update(ctx context.Context, req UpdateRequest) (*Config, error) {
	base, err := r.Get(ctx)
	if err != nil {
		return nil, err
	}

	// Target month: from request or current month.
	targetMonth := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC)
	if req.YearMonth != nil {
		parsed, err := parseYearMonth(*req.YearMonth)
		if err != nil {
			return nil, err
		}
		targetMonth = parsed
	}

	// Apply partial patches on top of the base (most recent) config.
	amount := base.Amount
	cutDay := base.CutDay
	description := base.Description
	if req.Amount != nil {
		amount = *req.Amount
	}
	if req.CutDay != nil {
		cutDay = *req.CutDay
	}
	if req.Description != nil {
		description = *req.Description
	}

	const q = "INSERT INTO income_config_history (`year_month`, amount, cut_day, description) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE amount = VALUES(amount), cut_day = VALUES(cut_day), description = VALUES(description)"
	if _, err := r.db.ExecContext(ctx, q, targetMonth, amount, cutDay, description); err != nil {
		return nil, fmt.Errorf("upserting income config: %w", err)
	}
	return r.GetForMonth(ctx, targetMonth)
}

func parseYearMonth(s string) (time.Time, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("year_month must be YYYY-MM, got %q", s)
	}
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), nil
}
