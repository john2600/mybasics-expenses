package security

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jscodelab/mybasics-expenses/internal/data"
)

// dbTimeout caps how long a single token query may run. It is derived from the
// caller's context, so the caller's cancellation/deadline still applies.
const dbTimeout = 3 * time.Second

// ErrTokenNotFound is returned when no matching, unexpired token exists.
var ErrTokenNotFound = errors.New("token not found or expired")

// TokenRepository persists tokens in the `tokens` table. Only the token hash is
// stored — never the plaintext, which is sent to the user and kept nowhere.
type TokenRepository interface {
	Insert(ctx context.Context, token *Token) error
	DeleteAllForUser(ctx context.Context, scope string, userID int) error
	GetForToken(ctx context.Context, scope string, hash []byte) (*data.User, error)
}

type mysqlTokenRepository struct {
	db *sql.DB
}

func NewMySQLTokenRepository(db *sql.DB) TokenRepository {
	return &mysqlTokenRepository{db: db}
}

// Insert stores a token row. token.Hash is the SHA-256 of the plaintext.
func (r *mysqlTokenRepository) Insert(ctx context.Context, token *Token) error {
	const q = `INSERT INTO tokens (hash, user_id, expiry, scope) VALUES (?, ?, ?, ?)`

	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	_, err := r.db.ExecContext(ctx, q, token.Hash, token.UserID, token.Expiry, token.Scope)
	if err != nil {
		return fmt.Errorf("inserting token: %w", err)
	}
	return nil
}

// DeleteAllForUser removes every token of a given scope for a user. Used to
// invalidate any previous tokens before issuing a fresh one.
func (r *mysqlTokenRepository) DeleteAllForUser(ctx context.Context, scope string, userID int) error {
	const q = `DELETE FROM tokens WHERE scope = ? AND user_id = ?`

	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	_, err := r.db.ExecContext(ctx, q, scope, userID)
	if err != nil {
		return fmt.Errorf("deleting tokens for user: %w", err)
	}
	return nil
}

// GetForToken returns the user owning a matching, unexpired token for the given
// scope, by joining tokens with users. hash is the SHA-256 of the plaintext
// presented by the user. Returns ErrTokenNotFound when there is no match.
func (r *mysqlTokenRepository) GetForToken(ctx context.Context, scope string, hash []byte) (*data.User, error) {
	const q = `SELECT u.id, u.username, u.name, u.email, u.activated
	           FROM tokens t
	           INNER JOIN users u ON u.id = t.user_id
	           WHERE t.hash = ? AND t.scope = ? AND t.expiry > NOW()`

	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	var u data.User
	if err := r.db.QueryRowContext(ctx, q, hash, scope).Scan(
		&u.ID, &u.Username, &u.Name, &u.Email, &u.Activated,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("getting user for token: %w", err)
	}
	return &u, nil
}
