package users

import (
	"fmt"
	"time"

	"errors"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is the work factor used to hash passwords. Higher is slower and
// more resistant to brute force; 12 is a sensible default for an API.
const bcryptCost = 12

type User struct {
	ID   int
	User string
	Name string
	// Password holds the plaintext password only transiently, between decoding
	// the request and calling Normalize. It is never persisted (json:"-") and is
	// cleared once hashed.
	Password       string `json:"-"`
	Email          string
	HashedPassword []byte
	Created        time.Time
	Updated        time.Time
}

// Normalize cleans up the user's fields and hashes the plaintext password into
// HashedPassword. It must be called before persisting the user. bcrypt can
// fail, so the error is propagated.
func (u *User) Normalize() error {
	u.User = strings.ToLower(strings.TrimSpace(u.User))
	u.Name = strings.TrimSpace(u.Name)
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))

	if u.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcryptCost)
		if err != nil {
			return fmt.Errorf("hashing password: %w", err)
		}
		u.HashedPassword = hashedPassword
		u.Password = "" // avoid keeping the plaintext around in memory
	}

	return nil
}

type UserRequest struct {
	UserName string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// bcrypt refuses passwords longer than 72 bytes, so we cap the length here to
// return a clean validation error instead of a hashing failure downstream.
const maxPasswordBytes = 72

func (u *UserRequest) Validate() error {
	u.UserName = strings.ToLower(strings.TrimSpace(u.UserName))
	u.Name = strings.TrimSpace(u.Name)
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))

	if u.UserName == "" {
		return errors.New("username es obligatorio")
	}
	if u.Name == "" {
		return errors.New("name es obligatorio")
	}
	if u.Email == "" {
		return errors.New("email es obligatorio")
	}
	if _, err := mail.ParseAddress(u.Email); err != nil {
		return errors.New("email inválido")
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
