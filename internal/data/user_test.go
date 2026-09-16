package data

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestPassword_MatchesTheOriginalPlaintext(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("supersecret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	p := NewPassword(hash)

	ok, err := p.Matches("supersecret")
	if err != nil {
		t.Fatalf("Matches returned error: %v", err)
	}
	if !ok {
		t.Error("Matches = false, want true for the original password")
	}
}

func TestPassword_WrongPlaintextIsFalseNotAnError(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("supersecret"), bcrypt.MinCost)
	p := NewPassword(hash)

	ok, err := p.Matches("wrong-password")

	// A mismatch is an expected outcome, not a failure: callers distinguish
	// "wrong password" (401) from "something broke" (500) by this err being nil.
	if err != nil {
		t.Errorf("err = %v, want nil for a simple mismatch", err)
	}
	if ok {
		t.Error("Matches = true, want false for the wrong password")
	}
}

func TestPassword_MalformedHashIsAnError(t *testing.T) {
	p := NewPassword([]byte("not-a-bcrypt-hash"))

	ok, err := p.Matches("anything")

	// A corrupt stored hash is a real problem and must not be reported as a
	// plain "wrong password".
	if err == nil {
		t.Error("err = nil, want an error for a malformed hash")
	}
	if ok {
		t.Error("Matches = true, want false")
	}
}

func TestLoginRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     LoginRequest
		wantErr string
	}{
		{"valid", LoginRequest{Email: "john@example.com", Password: "supersecret"}, ""},
		{"exactly the minimum length", LoginRequest{Email: "j@e.com", Password: "12345678"}, ""},
		{"exactly the maximum length", LoginRequest{Email: "j@e.com", Password: strings.Repeat("a", 72)}, ""},
		{"missing email", LoginRequest{Password: "supersecret"}, "email"},
		{"missing password", LoginRequest{Email: "john@example.com"}, "password"},
		{"password too short", LoginRequest{Email: "j@e.com", Password: "1234567"}, "8 caracteres"},
		// bcrypt silently truncates beyond 72 bytes, so it is rejected up front
		// rather than letting two different passwords hash to the same value.
		{"password too long", LoginRequest{Email: "j@e.com", Password: strings.Repeat("a", 73)}, "72 caracteres"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()

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

func TestIsAnonymous(t *testing.T) {
	if !AnonymousUser.IsAnonymous() {
		t.Error("AnonymousUser.IsAnonymous() = false, want true")
	}

	real := &User{ID: 1, Email: "john@example.com"}
	if real.IsAnonymous() {
		t.Error("a real user reported as anonymous")
	}

	// IsAnonymous compares identity, not contents: a zero-valued user that is
	// not the shared sentinel must not pass as anonymous, or a guard could be
	// fooled by an empty struct.
	empty := &User{}
	if empty.IsAnonymous() {
		t.Error("a distinct zero-valued User reported as anonymous")
	}
}
