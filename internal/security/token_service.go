package security

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/jscodelab/mybasics-expenses/internal/data"
)

// TokenService is the business layer for token persistence. It depends on the
// TokenRepository interface, so it can be unit-tested with a mock.
type TokenService interface {
	New(ctx context.Context, userID int, ttl time.Duration, scope string) (*Token, error)
	SaveToken(ctx context.Context, token *Token) error
	DeleteAllTokensUser(ctx context.Context, scope string, userID int) error
	GetForToken(ctx context.Context, scope, tokenPlainText string) (*data.User, error)
}

type tokenService struct {
	repo TokenRepository
}

func NewTokenService(repo TokenRepository) TokenService {
	return &tokenService{repo: repo}
}

// New generates a token and persists it in one step, returning it so the caller
// can send token.Plaintext to the user. Mirrors the alexedwards TokenModel.New
// pattern.
func (s *tokenService) New(ctx context.Context, userID int, ttl time.Duration, scope string) (*Token, error) {
	token := generateToken(userID, ttl, scope)
	err := s.SaveToken(ctx, token)

	return token, err
}

// SaveToken persists an already-generated token (hash, user, expiry, scope).
func (s *tokenService) SaveToken(ctx context.Context, token *Token) error {
	return s.repo.Insert(ctx, token)
}

// DeleteAllTokensUser removes all tokens of the given scope for a user.
func (s *tokenService) DeleteAllTokensUser(ctx context.Context, scope string, userID int) error {
	return s.repo.DeleteAllForUser(ctx, scope, userID)
}

// GetForToken returns the id of the user owning a matching, unexpired token of
// the given scope. It hashes the plaintext and looks it up by hash.
func (s *tokenService) GetForToken(ctx context.Context, scope string, tokenPlainText string) (*data.User, error) {
	hash := sha256.Sum256([]byte(tokenPlainText))
	return s.repo.GetForToken(ctx, scope, hash[:])
}
