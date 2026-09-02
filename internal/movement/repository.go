package movement

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Repository defines the data access contract for movements. Every operation
// is scoped to a single user via userID (for FindAll it comes from Filter.UserID).
type Repository interface {
	Create(ctx context.Context, userID int, m *Movement) (*Movement, error)
	FindAll(ctx context.Context, f Filter) ([]Movement, error)
	FindByID(ctx context.Context, userID int, id int64) (*Movement, error)
	Update(ctx context.Context, userID int, id int64, req UpdateRequest) (*Movement, error)
	Delete(ctx context.Context, userID int, id int64) error
	MonthlySummary(ctx context.Context, userID int) ([]MonthlySummary, error)
}

type mysqlRepository struct {
	db *sql.DB
}

// NewMySQLRepository creates a MySQL-backed Movement repository.
func NewMySQLRepository(db *sql.DB) Repository {
	return &mysqlRepository{db: db}
}

func (r *mysqlRepository) Create(ctx context.Context, userID int, m *Movement) (*Movement, error) {
	const q = `
		INSERT INTO movements (user_id, category_id, type, amount, description, date, hour)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	res, err := r.db.ExecContext(ctx, q,
		userID, m.CategoryID, m.Type, m.Amount, m.Description, m.Date, m.Hour)
	if err != nil {
		return nil, fmt.Errorf("inserting movement: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}

	return r.FindByID(ctx, userID, id)
}

func (r *mysqlRepository) FindAll(ctx context.Context, f Filter) ([]Movement, error) {
	conditions := []string{"m.user_id = ?"}
	args := []any{f.UserID}

	if f.CategoryID != nil {
		conditions = append(conditions, "m.category_id = ?")
		args = append(args, *f.CategoryID)
	}
	if f.Type != nil {
		conditions = append(conditions, "m.type = ?")
		args = append(args, *f.Type)
	}
	if f.DateFrom != nil {
		conditions = append(conditions, "m.date >= ?")
		args = append(args, *f.DateFrom)
	}
	if f.DateTo != nil {
		conditions = append(conditions, "m.date <= ?")
		args = append(args, *f.DateTo)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	limit := ""
	if f.Limit != nil && *f.Limit > 0 {
		limit = fmt.Sprintf("LIMIT %d", *f.Limit)
	}

	q := fmt.Sprintf(`
		SELECT m.id, m.category_id, c.name, m.type, m.amount, m.description, m.date, m.hour, m.created_at, m.updated_at
		FROM movements m
		LEFT JOIN categories c ON c.id = m.category_id
		%s
		ORDER BY m.date DESC, m.hour DESC
		%s`, where, limit)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying movements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var movements []Movement
	for rows.Next() {
		var m Movement
		var hour sql.NullString
		if err := rows.Scan(
			&m.ID, &m.CategoryID, &m.Category, &m.Type, &m.Amount,
			&m.Description, &m.Date, &hour, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning movement: %w", err)
		}
		if hour.Valid {
			m.Hour = &hour.String
		}
		movements = append(movements, m)
	}

	return movements, rows.Err()
}

func (r *mysqlRepository) FindByID(ctx context.Context, userID int, id int64) (*Movement, error) {
	const q = `
		SELECT m.id, m.category_id, c.name, m.type, m.amount, m.description, m.date, m.hour, m.created_at, m.updated_at
		FROM movements m
		LEFT JOIN categories c ON c.id = m.category_id
		WHERE m.id = ? AND m.user_id = ?`

	var m Movement
	var hour sql.NullString
	err := r.db.QueryRowContext(ctx, q, id, userID).Scan(
		&m.ID, &m.CategoryID, &m.Category, &m.Type, &m.Amount,
		&m.Description, &m.Date, &hour, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding movement by id: %w", err)
	}
	if hour.Valid {
		m.Hour = &hour.String
	}

	return &m, nil
}

func (r *mysqlRepository) Update(ctx context.Context, userID int, id int64, req UpdateRequest) (*Movement, error) {
	var setClauses []string
	var args []any

	if req.CategoryID != nil {
		setClauses = append(setClauses, "category_id = ?")
		args = append(args, *req.CategoryID)
	}
	if req.Type != nil {
		setClauses = append(setClauses, "type = ?")
		args = append(args, *req.Type)
	}
	if req.Amount != nil {
		setClauses = append(setClauses, "amount = ?")
		args = append(args, *req.Amount)
	}
	if req.Description != nil {
		setClauses = append(setClauses, "description = ?")
		args = append(args, *req.Description)
	}
	if req.Date != nil {
		setClauses = append(setClauses, "date = ?")
		args = append(args, *req.Date)
	}
	if req.Hour != nil {
		setClauses = append(setClauses, "hour = ?")
		args = append(args, *req.Hour)
	}

	if len(setClauses) == 0 {
		return r.FindByID(ctx, userID, id)
	}

	args = append(args, id, userID)
	q := fmt.Sprintf("UPDATE movements SET %s WHERE id = ? AND user_id = ?", strings.Join(setClauses, ", "))

	if _, err := r.db.ExecContext(ctx, q, args...); err != nil {
		return nil, fmt.Errorf("updating movement: %w", err)
	}

	return r.FindByID(ctx, userID, id)
}

func (r *mysqlRepository) Delete(ctx context.Context, userID int, id int64) error {
	const q = `DELETE FROM movements WHERE id = ? AND user_id = ?`
	res, err := r.db.ExecContext(ctx, q, id, userID)
	if err != nil {
		return fmt.Errorf("deleting movement: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *mysqlRepository) MonthlySummary(ctx context.Context, userID int) ([]MonthlySummary, error) {
	const q = `
		SELECT YEAR(date) AS year, MONTH(date) AS month, SUM(amount) AS total
		FROM movements
		WHERE type='E' AND user_id = ?
		GROUP BY YEAR(date), MONTH(date)
		ORDER BY year DESC, month DESC`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("querying monthly summary: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summaries []MonthlySummary
	for rows.Next() {
		var s MonthlySummary
		if err := rows.Scan(&s.Year, &s.Month, &s.Total); err != nil {
			return nil, fmt.Errorf("scanning monthly summary: %w", err)
		}
		summaries = append(summaries, s)
	}

	return summaries, rows.Err()
}
