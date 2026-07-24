package users

import (
	"context"
	"database/sql"
	"fmt"

	"errors"
)

type Repository interface {
	Create(ctx context.Context, m *User) error
	GetUserID(ctx context.Context, m *User) (int, error)
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

func (r *mysqlRepository) GetUserID(ctx context.Context, user *User) (int, error) {
	var id int
	var hashedPassword []byte
	stmt := "SELECT id, hashed_password FROM users WHERE email = ?"
	err := r.db.QueryRow(stmt, user.Email).Scan(&id, &hashedPassword)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrInvalidCredentials
		}
		return 0, err
	}

	err = user.ComparePassword(hashedPassword)
	if err != nil {
		return 0, fmt.Errorf("error getting user :%w", err)
	}

	return id, nil
}
