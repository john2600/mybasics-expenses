package users

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestUser_Normalize_HashesPasswordAndTrimsFields(t *testing.T) {
	u := User{
		User:     "  JohnDoe ",
		Name:     "  John Doe  ",
		Email:    "  John@Example.COM ",
		Password: "supersecret",
	}

	if err := u.Normalize(); err != nil {
		t.Fatalf("Normalize() returned error: %v", err)
	}

	if u.User != "johndoe" {
		t.Errorf("User = %q, want %q", u.User, "johndoe")
	}
	if u.Name != "John Doe" {
		t.Errorf("Name = %q, want %q", u.Name, "John Doe")
	}
	if u.Email != "john@example.com" {
		t.Errorf("Email = %q, want %q", u.Email, "john@example.com")
	}
	if u.Password != "" {
		t.Errorf("plaintext Password should be cleared, got %q", u.Password)
	}
	if len(u.HashedPassword) == 0 {
		t.Fatal("HashedPassword is empty")
	}
	if err := bcrypt.CompareHashAndPassword(u.HashedPassword, []byte("supersecret")); err != nil {
		t.Errorf("hash does not match original password: %v", err)
	}
}

func TestUser_Normalize_NoPasswordLeavesHashEmpty(t *testing.T) {
	u := User{User: "jane", Email: "jane@example.com"}

	if err := u.Normalize(); err != nil {
		t.Fatalf("Normalize() returned error: %v", err)
	}
	if len(u.HashedPassword) != 0 {
		t.Errorf("HashedPassword should stay empty when no password is set, got %d bytes", len(u.HashedPassword))
	}
}
