package users

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/jscodelab/mybasics-expenses/internal/data"
)

func TestComparePassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("supersecret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}

	if err := ComparePassword(hash, "supersecret"); err != nil {
		t.Errorf("err = %v, want nil for the right password", err)
	}

	// A mismatch maps to the package sentinel so callers answer 401 rather
	// than surfacing a bcrypt internal.
	if err := ComparePassword(hash, "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}

	// A corrupt stored hash is a real failure and must stay distinguishable.
	err = ComparePassword([]byte("not-a-hash"), "supersecret")
	if err == nil {
		t.Error("err = nil, want an error for a malformed hash")
	}
	if errors.Is(err, ErrInvalidCredentials) {
		t.Error("a malformed hash was reported as invalid credentials")
	}
}

func TestEncriptPassword(t *testing.T) {
	hash, err := EncriptPassword("supersecret")
	if err != nil {
		t.Fatalf("EncriptPassword returned error: %v", err)
	}
	if len(hash) == 0 {
		t.Fatal("hash is empty")
	}
	if string(hash) == "supersecret" {
		t.Error("the password was returned in plaintext")
	}
	// The result must verify against the original, or stored passwords would
	// be unusable.
	if err := bcrypt.CompareHashAndPassword(hash, []byte("supersecret")); err != nil {
		t.Errorf("the produced hash does not verify: %v", err)
	}

	// An empty password yields an empty hash rather than hashing "".
	empty, err := EncriptPassword("")
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if len(empty) != 0 {
		t.Errorf("hash = %q, want empty for an empty password", empty)
	}
}

func TestEncriptPassword_TooLongForBcrypt(t *testing.T) {
	// bcrypt refuses more than 72 bytes; the error must propagate instead of
	// silently producing an unusable hash.
	if _, err := EncriptPassword(strings.Repeat("a", 100)); err == nil {
		t.Error("expected an error for a password beyond bcrypt's limit")
	}
}

func TestDecriptPassword_IsAMisnamedDuplicateOfEncript(t *testing.T) {
	// Despite the name it does not decrypt anything — bcrypt is one-way. It
	// hashes, exactly like EncriptPassword. Pinned so the behaviour is not
	// mistaken for a reversal.
	hash, err := DecriptPassword("supersecret")
	if err != nil {
		t.Fatalf("DecriptPassword returned error: %v", err)
	}
	if string(hash) == "supersecret" {
		t.Error("returned the plaintext")
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte("supersecret")); err != nil {
		t.Errorf("the produced value is not a hash of the input: %v", err)
	}

	empty, err := DecriptPassword("")
	if err != nil || len(empty) != 0 {
		t.Errorf("got = (%q, %v), want (empty, nil)", empty, err)
	}
}

func TestUserRequest_Validate(t *testing.T) {
	valid := func() UserRequest {
		return UserRequest{UserName: "john", Name: "John Doe", Email: "john@example.com", Password: "supersecret"}
	}

	tests := []struct {
		name    string
		mutate  func(*UserRequest)
		wantErr string
	}{
		{"valid", func(*UserRequest) {}, ""},
		{"missing username", func(r *UserRequest) { r.UserName = "" }, "username"},
		{"whitespace-only username", func(r *UserRequest) { r.UserName = "   " }, "username"},
		{"missing name", func(r *UserRequest) { r.Name = "" }, "name"},
		{"missing email", func(r *UserRequest) { r.Email = "" }, "email"},
		{"malformed email", func(r *UserRequest) { r.Email = "not-an-email" }, "inválido"},
		{"missing password", func(r *UserRequest) { r.Password = "" }, "password"},
		{"password too short", func(r *UserRequest) { r.Password = "1234567" }, "8 caracteres"},
		{"password too long", func(r *UserRequest) { r.Password = strings.Repeat("a", 73) }, "72 caracteres"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := valid()
			tt.mutate(&req)

			err := req.Validate()

			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("err = nil, want one mentioning %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestUserRequest_ValidateNormalisesInPlace(t *testing.T) {
	// Validate lowercases and trims before checking, so the stored values are
	// already canonical — two accounts cannot differ only by case or spaces.
	req := UserRequest{
		UserName: "  JOHN  ", Name: "  John Doe  ",
		Email: "  JOHN@Example.COM ", Password: "supersecret",
	}

	if err := req.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if req.UserName != "john" {
		t.Errorf("UserName = %q, want %q", req.UserName, "john")
	}
	if req.Email != "john@example.com" {
		t.Errorf("Email = %q, want %q", req.Email, "john@example.com")
	}
	if req.Name != "John Doe" {
		t.Errorf("Name = %q, want %q", req.Name, "John Doe")
	}
}

func TestChangePasswordRequest_Validate(t *testing.T) {
	valid := func() ChangePasswordRequest {
		return ChangePasswordRequest{
			LoginRequest: data.LoginRequest{Email: "john@example.com", Password: "currentPass123"},
			NewPassword:  "brandNewPass456",
		}
	}

	tests := []struct {
		name    string
		mutate  func(*ChangePasswordRequest)
		wantErr string
	}{
		{"valid", func(*ChangePasswordRequest) {}, ""},
		{"invalid login request", func(r *ChangePasswordRequest) { r.LoginRequest.Email = "" }, "email"},
		{"missing new password", func(r *ChangePasswordRequest) { r.NewPassword = "" }, "needs to be filled"},
		{"new password too short", func(r *ChangePasswordRequest) { r.NewPassword = "short" }, "8 caracteres"},
		{"new password too long", func(r *ChangePasswordRequest) { r.NewPassword = strings.Repeat("a", 73) }, "72 caracteres"},
		// Reusing the current password would make the change a no-op while
		// still reporting success.
		{"new password equals the current one", func(r *ChangePasswordRequest) { r.NewPassword = r.LoginRequest.Password }, "different"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := valid()
			tt.mutate(&req)

			err := req.Validate()

			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("err = nil, want one mentioning %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestUser_NormalizeRejectsAPasswordBeyondBcryptsLimit(t *testing.T) {
	u := User{User: "john", Email: "john@example.com", Password: strings.Repeat("a", 100)}

	if err := u.Normalize(); err == nil {
		t.Error("expected Normalize to propagate the bcrypt failure")
	}
}
