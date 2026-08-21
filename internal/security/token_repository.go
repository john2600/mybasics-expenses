package security

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jscodelab/mybasics-expenses/internal/data"
)

// TokenRepository persists tokens in the `tokens` table. Only the token hash is
// stored — never the plaintext, which is sent to the user and kept nowhere.
type TokenRepository interface {
	Insert(ctx context.Context, token *data.Token) error
	DeleteAllForUser(ctx context.Context, scope string, userID int) error
}

type mysqlTokenRepository struct {
	db *sql.DB
}

func NewMySQLTokenRepository(db *sql.DB) TokenRepository {
	return &mysqlTokenRepository{db: db}
}

// Insert stores a token row. token.Hash is the SHA-256 of the plaintext.
func (r *mysqlTokenRepository) Insert(ctx context.Context, token *data.Token) error {
	const q = `INSERT INTO tokens (hash, user_id, expiry, scope) VALUES (?, ?, ?, ?)`

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

	_, err := r.db.ExecContext(ctx, q, scope, userID)
	if err != nil {
		return fmt.Errorf("deleting tokens for user: %w", err)
	}
	return nil
}
