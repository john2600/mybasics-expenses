package security

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"

	"github.com/jscodelab/mybasics-expenses/internal/data"
)

var errBoom = errors.New("db down")

// --- generateToken ----------------------------------------------------------

func TestGenerateToken_StoresOnlyTheHashAndNeverThePlaintext(t *testing.T) {
	tok := generateToken(7, time.Hour, ScopeAuthentication)

	if tok.Plaintext == "" {
		t.Fatal("Plaintext is empty")
	}
	// The hash is what gets persisted; it must be the SHA-256 of the plaintext
	// so a stolen database cannot be replayed as tokens.
	want := sha256.Sum256([]byte(tok.Plaintext))
	if string(tok.Hash) != string(want[:]) {
		t.Error("Hash is not the SHA-256 of the plaintext")
	}
	if string(tok.Hash) == tok.Plaintext {
		t.Error("the plaintext was stored as the hash")
	}
	if tok.UserID != 7 || tok.Scope != ScopeAuthentication {
		t.Errorf("got = %+v", tok)
	}
	if !tok.Expiry.After(time.Now()) {
		t.Errorf("Expiry = %v, want a future time", tok.Expiry)
	}
}

func TestGenerateToken_IsDifferentEveryTime(t *testing.T) {
	// Two tokens issued back to back must not collide, or one user's token
	// could authenticate as another.
	a := generateToken(1, time.Hour, ScopeAuthentication)
	b := generateToken(1, time.Hour, ScopeAuthentication)

	if a.Plaintext == b.Plaintext {
		t.Error("two generated tokens share the same plaintext")
	}
}

// --- repository -------------------------------------------------------------

func newTokenRepoMock(t *testing.T) (TokenRepository, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("opening sqlmock: %v", err)
	}

	return NewMySQLTokenRepository(db), mock, func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet sqlmock expectations: %v", err)
		}
		_ = db.Close()
	}
}

func TestTokenRepoInsert_PersistsTheHash(t *testing.T) {
	repo, mock, done := newTokenRepoMock(t)
	defer done()

	tok := generateToken(7, time.Hour, ScopeAuthentication)
	mock.ExpectExec(`INSERT INTO tokens \(hash, user_id, expiry, scope\)`).
		WithArgs(tok.Hash, 7, tok.Expiry, ScopeAuthentication).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.Insert(context.Background(), tok); err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}
}

func TestTokenRepoInsert_WrapsExecError(t *testing.T) {
	repo, mock, done := newTokenRepoMock(t)
	defer done()

	mock.ExpectExec(`INSERT INTO tokens`).WillReturnError(errBoom)

	if err := repo.Insert(context.Background(), generateToken(7, time.Hour, ScopeAuthentication)); err == nil {
		t.Fatal("expected an error when the insert fails")
	}
}

func TestTokenRepoDeleteAllForUser_ScopesTheDeletion(t *testing.T) {
	repo, mock, done := newTokenRepoMock(t)
	defer done()

	// Logging out of authentication must not wipe a pending activation token.
	mock.ExpectExec(`DELETE FROM tokens WHERE scope = \? AND user_id = \?`).
		WithArgs(ScopeAuthentication, 7).
		WillReturnResult(sqlmock.NewResult(0, 3))

	if err := repo.DeleteAllForUser(context.Background(), ScopeAuthentication, 7); err != nil {
		t.Fatalf("DeleteAllForUser returned error: %v", err)
	}
}

func TestTokenRepoDeleteAllForUser_WrapsExecError(t *testing.T) {
	repo, mock, done := newTokenRepoMock(t)
	defer done()

	mock.ExpectExec(`DELETE FROM tokens`).WillReturnError(errBoom)

	if err := repo.DeleteAllForUser(context.Background(), ScopeAuthentication, 7); err == nil {
		t.Fatal("expected an error when the delete fails")
	}
}

func TestTokenRepoGetForToken_ReturnsTheOwnerForAnUnexpiredToken(t *testing.T) {
	repo, mock, done := newTokenRepoMock(t)
	defer done()

	hash := []byte("hash-bytes")
	// The expiry filter lives in SQL (expiry > NOW()), so an expired row simply
	// does not match — nothing in Go has to compare times.
	mock.ExpectQuery(`(?s)FROM tokens t.+WHERE t.hash = \? AND t.scope = \? AND t.expiry > NOW\(\)`).
		WithArgs(hash, ScopeAuthentication).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "name", "email", "activated"}).
			AddRow(7, "john", "John Doe", "john@example.com", true))

	got, err := repo.GetForToken(context.Background(), ScopeAuthentication, hash)
	if err != nil {
		t.Fatalf("GetForToken returned error: %v", err)
	}
	if got.ID != 7 || got.Email != "john@example.com" || !got.Activated {
		t.Errorf("got = %+v", got)
	}
}

func TestTokenRepoGetForToken_NoMatchIsErrTokenNotFound(t *testing.T) {
	repo, mock, done := newTokenRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)FROM tokens t`).WillReturnError(sql.ErrNoRows)

	_, err := repo.GetForToken(context.Background(), ScopeAuthentication, []byte("x"))
	// A missing or expired token is a 401, so it must be this sentinel and not
	// a generic wrapped error.
	if !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("err = %v, want ErrTokenNotFound", err)
	}
}

func TestTokenRepoGetForToken_WrapsOtherErrors(t *testing.T) {
	repo, mock, done := newTokenRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)FROM tokens t`).WillReturnError(errBoom)

	_, err := repo.GetForToken(context.Background(), ScopeAuthentication, []byte("x"))
	if err == nil {
		t.Fatal("expected an error")
	}
	// A broken database must not look like a bad token.
	if errors.Is(err, ErrTokenNotFound) {
		t.Error("an infrastructure failure was reported as ErrTokenNotFound")
	}
}

// --- service ----------------------------------------------------------------

type stubTokenRepo struct {
	insertErr error
	deleteErr error
	user      *data.User
	getErr    error

	insertedScope string
	gotHash       []byte
}

func (s *stubTokenRepo) Insert(_ context.Context, token *Token) error {
	s.insertedScope = token.Scope
	return s.insertErr
}
func (s *stubTokenRepo) DeleteAllForUser(context.Context, string, int) error { return s.deleteErr }
func (s *stubTokenRepo) GetForToken(_ context.Context, _ string, hash []byte) (*data.User, error) {
	s.gotHash = hash
	return s.user, s.getErr
}

type stubUserFinder struct {
	user *data.User
	err  error
}

func (s *stubUserFinder) GetUserByEmail(context.Context, string) (*data.User, error) {
	return s.user, s.err
}

func userWithPassword(t *testing.T, plaintext string) *data.User {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	return &data.User{ID: 7, Email: "john@example.com", Activated: true, Password: data.NewPassword(hash)}
}

func TestTokenServiceNew_GeneratesAndPersistsInOneStep(t *testing.T) {
	repo := &stubTokenRepo{}
	svc := NewTokenService(repo, &stubUserFinder{})

	tok, err := svc.New(context.Background(), 7, time.Hour, ScopeActivation)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if tok.Plaintext == "" {
		t.Error("the caller needs the plaintext to send it to the user")
	}
	if repo.insertedScope != ScopeActivation {
		t.Errorf("persisted scope = %q, want %q", repo.insertedScope, ScopeActivation)
	}
}

func TestTokenServiceNew_PropagatesPersistenceError(t *testing.T) {
	svc := NewTokenService(&stubTokenRepo{insertErr: errBoom}, &stubUserFinder{})

	if _, err := svc.New(context.Background(), 7, time.Hour, ScopeActivation); err == nil {
		t.Fatal("expected the insert error to propagate")
	}
}

func TestTokenServiceSaveAndDelete_Delegate(t *testing.T) {
	svc := NewTokenService(&stubTokenRepo{}, &stubUserFinder{})

	if err := svc.SaveToken(context.Background(), generateToken(1, time.Hour, ScopeActivation)); err != nil {
		t.Errorf("SaveToken returned error: %v", err)
	}
	if err := svc.DeleteAllTokensUser(context.Background(), ScopeAuthentication, 1); err != nil {
		t.Errorf("DeleteAllTokensUser returned error: %v", err)
	}

	failing := NewTokenService(&stubTokenRepo{insertErr: errBoom, deleteErr: errBoom}, &stubUserFinder{})
	if err := failing.SaveToken(context.Background(), generateToken(1, time.Hour, ScopeActivation)); err == nil {
		t.Error("expected SaveToken to propagate the error")
	}
	if err := failing.DeleteAllTokensUser(context.Background(), ScopeAuthentication, 1); err == nil {
		t.Error("expected DeleteAllTokensUser to propagate the error")
	}
}

func TestTokenServiceGetForToken_LooksUpByHashNotPlaintext(t *testing.T) {
	repo := &stubTokenRepo{user: &data.User{ID: 7}}
	svc := NewTokenService(repo, &stubUserFinder{})

	if _, err := svc.GetForToken(context.Background(), ScopeAuthentication, "the-plaintext"); err != nil {
		t.Fatalf("GetForToken returned error: %v", err)
	}

	want := sha256.Sum256([]byte("the-plaintext"))
	if string(repo.gotHash) != string(want[:]) {
		t.Error("the repository was queried with something other than the SHA-256 of the plaintext")
	}
}

func TestCreateAuthentication_IssuesATokenForValidCredentials(t *testing.T) {
	repo := &stubTokenRepo{}
	svc := NewTokenService(repo, &stubUserFinder{user: userWithPassword(t, "supersecret")})

	tok, err := svc.CreateAuthentication(context.Background(), data.LoginRequest{
		Email: "john@example.com", Password: "supersecret",
	})
	if err != nil {
		t.Fatalf("CreateAuthentication returned error: %v", err)
	}
	if tok.Plaintext == "" {
		t.Error("no plaintext token returned")
	}
	if repo.insertedScope != ScopeAuthentication {
		t.Errorf("scope = %q, want %q", repo.insertedScope, ScopeAuthentication)
	}
}

func TestCreateAuthentication_UnknownEmailAndWrongPasswordAreIndistinguishable(t *testing.T) {
	// User enumeration defence: both paths must return the very same error, so
	// a caller cannot probe which emails are registered.
	unknown := NewTokenService(&stubTokenRepo{}, &stubUserFinder{err: ErrUserNotFound})
	_, errUnknown := unknown.CreateAuthentication(context.Background(), data.LoginRequest{
		Email: "nobody@example.com", Password: "supersecret",
	})

	wrongPass := NewTokenService(&stubTokenRepo{}, &stubUserFinder{user: userWithPassword(t, "supersecret")})
	_, errWrong := wrongPass.CreateAuthentication(context.Background(), data.LoginRequest{
		Email: "john@example.com", Password: "not-the-password",
	})

	if !errors.Is(errUnknown, ErrInvalidCredentials) {
		t.Errorf("unknown email error = %v, want ErrInvalidCredentials", errUnknown)
	}
	if !errors.Is(errWrong, ErrInvalidCredentials) {
		t.Errorf("wrong password error = %v, want ErrInvalidCredentials", errWrong)
	}
	if errUnknown.Error() != errWrong.Error() {
		t.Errorf("the two errors differ (%q vs %q), which leaks whether an account exists",
			errUnknown, errWrong)
	}
}

func TestCreateAuthentication_InfrastructureErrorIsNotMaskedAsBadCredentials(t *testing.T) {
	// A broken database must stay distinguishable from a wrong login, or a
	// 500 would be served to the user as a 401 and hide the outage.
	svc := NewTokenService(&stubTokenRepo{}, &stubUserFinder{err: errBoom})

	_, err := svc.CreateAuthentication(context.Background(), data.LoginRequest{
		Email: "john@example.com", Password: "supersecret",
	})
	if errors.Is(err, ErrInvalidCredentials) {
		t.Error("an infrastructure error was reported as invalid credentials")
	}
	if err == nil {
		t.Error("expected an error")
	}
}

func TestCreateAuthentication_RejectsAnInvalidRequestBeforeTouchingTheDatabase(t *testing.T) {
	finder := &stubUserFinder{err: errBoom} // would fail if it were reached
	svc := NewTokenService(&stubTokenRepo{}, finder)

	if _, err := svc.CreateAuthentication(context.Background(), data.LoginRequest{}); err == nil {
		t.Fatal("expected a validation error for an empty request")
	}
}

func TestCreateAuthentication_PropagatesAMalformedStoredHash(t *testing.T) {
	svc := NewTokenService(&stubTokenRepo{}, &stubUserFinder{
		user: &data.User{ID: 7, Email: "john@example.com", Password: data.NewPassword([]byte("garbage"))},
	})

	_, err := svc.CreateAuthentication(context.Background(), data.LoginRequest{
		Email: "john@example.com", Password: "supersecret",
	})
	if err == nil {
		t.Fatal("expected an error for a corrupt stored hash")
	}
	if errors.Is(err, ErrInvalidCredentials) {
		t.Error("a corrupt hash was reported as invalid credentials, hiding a real problem")
	}
}

func TestCreateAuthentication_PropagatesTokenPersistenceError(t *testing.T) {
	svc := NewTokenService(&stubTokenRepo{insertErr: errBoom}, &stubUserFinder{user: userWithPassword(t, "supersecret")})

	if _, err := svc.CreateAuthentication(context.Background(), data.LoginRequest{
		Email: "john@example.com", Password: "supersecret",
	}); err == nil {
		t.Fatal("expected the persistence error to propagate")
	}
}
