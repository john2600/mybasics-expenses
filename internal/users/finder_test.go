package users

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/jscodelab/mybasics-expenses/internal/security"
)

func TestUserFinder_ConvertsTheRecordAndBuildsAUsablePassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("supersecret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	repo := &stubRepo{user: &User{
		ID: 7, User: "john", Name: "John Doe",
		Email: "john@example.com", Activated: true, HashedPassword: hash,
	}}

	got, err := NewUserFinder(repo).GetUserByEmail(context.Background(), "john@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail returned error: %v", err)
	}
	if got.ID != 7 || got.Username != "john" || got.Email != "john@example.com" || !got.Activated {
		t.Errorf("got = %+v", got)
	}
	// The security package verifies credentials through data.Password, so the
	// conversion must produce one that actually matches the stored hash —
	// otherwise every login would fail with correct credentials.
	ok, err := got.Password.Matches("supersecret")
	if err != nil {
		t.Fatalf("Matches returned error: %v", err)
	}
	if !ok {
		t.Error("the converted password does not verify against the original")
	}
}

func TestUserFinder_TranslatesTheNotFoundSentinel(t *testing.T) {
	// security cannot import users, so the users sentinel is translated into
	// the security one. Without this, CreateAuthentication would treat a
	// missing account as an infrastructure failure and answer 500 instead of
	// the generic 401.
	repo := &stubRepo{getErr: ErrNoRecord}

	_, err := NewUserFinder(repo).GetUserByEmail(context.Background(), "nobody@example.com")
	if !errors.Is(err, security.ErrUserNotFound) {
		t.Errorf("err = %v, want security.ErrUserNotFound", err)
	}
	if errors.Is(err, ErrNoRecord) {
		t.Error("the users-package sentinel leaked out of the adapter")
	}
}

func TestUserFinder_PassesOtherErrorsThrough(t *testing.T) {
	repo := &stubRepo{getErr: errBoom}

	_, err := NewUserFinder(repo).GetUserByEmail(context.Background(), "john@example.com")
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, security.ErrUserNotFound) {
		t.Error("an infrastructure error was translated into a missing user")
	}
}
