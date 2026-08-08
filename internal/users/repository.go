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
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	UpdatePassword(ctx context.Context, userId int, newPassowrd []byte) error
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

	err = ComparePassword(hashedPassword, user.Password)
	if err != nil {
		return 0, fmt.Errorf("error getting user :%w", err)
	}

	return id, nil
}

// GetUserByEmail returns the full user record for the given email, including
// the id and the hashed password. It returns ErrNoRecord when no user matches.
func (r *mysqlRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	const q = `SELECT id, username, name, email, hashed_password, created_at, updated_at
	           FROM users
	           WHERE email = ?`

	var u User
	err := r.db.QueryRowContext(ctx, q, email).Scan(
		&u.ID, &u.User, &u.Name, &u.Email, &u.HashedPassword, &u.Created, &u.Updated,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRecord
		}
		return nil, fmt.Errorf("fetching user by email: %w", err)
	}

	return &u, nil
}

// UpdatePassword sets a new hashed password for the user identified by ID.
// The password must already be hashed (bcrypt); the repository never hashes.
// updated_at is refreshed automatically by the column's ON UPDATE clause.
// Returns ErrNoRecord when no user matches the id.
func (r *mysqlRepository) UpdatePassword(ctx context.Context, userID int, password []byte) error {
	const q = `UPDATE users SET hashed_password = ? WHERE id = ?`

	res, err := r.db.ExecContext(ctx, q, password, userID)
	if err != nil {
		return fmt.Errorf("updating password: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNoRecord
	}

	return nil
}
