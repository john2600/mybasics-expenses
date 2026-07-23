package users

import (
	"context"
	"database/sql"
	"fmt"
)


type Repository interface {
	Create(ctx context.Context, m *User) (error)
}

type mysqlRepository struct {
	db *sql.DB
}

func NewMySQLRepository(db *sql.DB) Repository {
	return &mysqlRepository{db: db}
}

// Create persists a new user. The password must already be hashed into
// HashedPassword (done by User.Normalize) before this is called.
func (r *mysqlRepository) Create(ctx context.Context, user *User) error {
	const q = `INSERT INTO users (username, name, email, hashed_password) VALUES (?, ?, ?, ?)`

	res, err := r.db.ExecContext(ctx, q, user.User, user.Name, user.Email, user.HashedPassword)
	if err != nil {
		return fmt.Errorf("inserting user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	user.ID = int(id)

	return nil
}