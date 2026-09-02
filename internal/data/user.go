// Package data holds domain models shared across packages that would otherwise
// import each other. Keeping them here (a leaf package that imports nothing
// internal) breaks import cycles — e.g. security can return a User without
// importing the users package.
package data

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// maxPasswordBytes caps the password length: bcrypt refuses passwords longer
// than 72 bytes, so LoginRequest.Validate rejects them up front. Kept here with
// the moved LoginRequest; the users package keeps its own copy for its requests.
const maxPasswordBytes = 72

var AnonymousUser = &User{}

// User is the shared user view returned by cross-package lookups (e.g. the
// token → user join in security). It is intentionally light; the users package
// keeps its own richer model for user CRUD.
type User struct {
	ID             int `json:"id"`
	Username       string
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	Activated      bool     `json:"activated"`
	Password       Password `json:"-"`
	HashedPassword []byte
	version        int `json:"-"`
}

// LoginRequest is the payload for authenticating an existing user.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Password struct {
	plaintext *string
	hash      []byte
}

// NewPassword wraps an existing bcrypt hash into a Password. Callers outside this
// package can't set the unexported hash field directly, so the users finder uses
// this to build a Password from a stored hash.
func NewPassword(hash []byte) Password {
	return Password{hash: hash}
}

func (p *Password) Matches(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}

func (u *LoginRequest) Validate() error {
	if u.Email == "" {
		return errors.New("email es obligatorio")
	}

	if u.Password == "" {
		return errors.New("password es obligatorio")
	}
	if len(u.Password) < 8 {
		return errors.New("password debe tener al menos 8 caracteres")
	}
	if len(u.Password) > maxPasswordBytes {
		return errors.New("password no puede superar los 72 caracteres")
	}
	return nil
}

func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}
