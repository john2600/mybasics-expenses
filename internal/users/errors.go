package users

import (
	"errors"
)

var (
	ErrNoRecord = errors.New("models: no matching record found")
	// Add a new ErrInvalidCredentials error. We'll use this later if a user
	// tries to login with an incorrect email address or password.
	ErrInvalidCredentials = errors.New("models: invalid credentials")
	// Add a new ErrDuplicateEmail error. We'll use this later if a user
	// tries to signup with an email address that's already in use.
	ErrDuplicateEmail = errors.New("models: duplicate email")
	// ErrDuplicateUser is returned when a unique constraint (username or email)
	// is violated on insert. It is deliberately generic — it never says which
	// field nor exposes the underlying DB error — to avoid leaking schema
	// details or enabling account enumeration.
	ErrDuplicateUser = errors.New("username or email already in use")
)
