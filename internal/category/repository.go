package category

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Repository defines the data access contract for categories.
type Repository interface {
	FindAll(ctx context.Context) ([]Category, error)
	FindByID(ctx context.Context, id int64) (*Category, error)
	Create(ctx context.Context, c *Category) (*Category, error)
	Update(ctx context.Context, id int64, req UpdateRequest) (*Category, error)
	Delete(ctx context.Context, id int64) error
}

type mysqlRepository struct {
	db *sql.DB
}

// NewMySQLRepository creates a MySQL-backed Category repository.
func NewMySQLRepository(db *sql.DB) Repository {
	return &mysqlRepository{db: db}
}

func (r *mysqlRepository) FindAll(ctx context.Context) ([]Category, error) {
	const q = `
		SELECT id, name, description, color, created_at, updated_at
		FROM categories
		ORDER BY name ASC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("querying categories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var categories []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Color, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}
		categories = append(categories, c)
	}

	return categories, rows.Err()
}

func (r *mysqlRepository) FindByID(ctx context.Context, id int64) (*Category, error) {
	const q = `
		SELECT id, name, description, color, created_at, updated_at
		FROM categories
		WHERE id = ?`

	var c Category
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&c.ID, &c.Name, &c.Description, &c.Color, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding category by id: %w", err)
	}

	return &c, nil
}

func (r *mysqlRepository) Create(ctx context.Context, c *Category) (*Category, error) {
	const q = `INSERT INTO categories (name, description, color) VALUES (?, ?, ?)`

	res, err := r.db.ExecContext(ctx, q, c.Name, c.Description, c.Color)
	if err != nil {
		return nil, fmt.Errorf("inserting category: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *mysqlRepository) Update(ctx context.Context, id int64, req UpdateRequest) (*Category, error) {
	var setClauses []string
	var args []any

	if req.Name != nil {
		setClauses = append(setClauses, "name = ?")
		args = append(args, *req.Name)
	}
	if req.Description != nil {
		setClauses = append(setClauses, "description = ?")
		args = append(args, *req.Description)
	}
	if req.Color != nil {
		setClauses = append(setClauses, "color = ?")
		args = append(args, *req.Color)
	}

	if len(setClauses) == 0 {
		return r.FindByID(ctx, id)
	}

	args = append(args, id)
	q := fmt.Sprintf("UPDATE categories SET %s WHERE id = ?", strings.Join(setClauses, ", "))

	if _, err := r.db.ExecContext(ctx, q, args...); err != nil {
		return nil, fmt.Errorf("updating category: %w", err)
	}

	return r.FindByID(ctx, id)
}

func (r *mysqlRepository) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM categories WHERE id = ?`
	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("deleting category: %w", err)
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
