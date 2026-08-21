package data

import (
	"crypto/rand"
	"crypto/sha256"
	"time"
)

const (
	ScopeActivation = "activation"
)

type Token struct {
	Plaintext string
	Hash      []byte
	UserID    int
	Expiry    time.Time
	Scope     string
}

// GenerateToken builds a token for the user with the given TTL and scope. The
// plaintext is a high-entropy random string (sent to the user, never stored);
// only its SHA-256 hash is persisted. It returns no error: rand.Text panics if
// the system RNG fails, which is fatal.
func GenerateToken(userID int, ttl time.Duration, scope string) *Token {
	token := &Token{
		Plaintext: rand.Text(),
		UserID:    userID,
		Expiry:    time.Now().Add(ttl),
		Scope:     scope,
	}

	hash := sha256.Sum256([]byte(token.Plaintext))
	token.Hash = hash[:]

	return token
}
