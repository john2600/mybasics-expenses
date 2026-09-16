package users

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/jscodelab/mybasics-expenses/internal/data"
	"github.com/jscodelab/mybasics-expenses/internal/security"
)

// --- doubles ----------------------------------------------------------------

type stubRepo struct {
	createErr   error
	activateErr error
	updateErr   error
	user        *User
	getErr      error
	userID      int
	getIDErr    error

	created         *User
	updatedPassword []byte
	activatedID     int
}

func (s *stubRepo) Create(_ context.Context, u *User) error {
	s.created = u
	if s.createErr != nil {
		return s.createErr
	}
	u.ID = 7
	return nil
}
func (s *stubRepo) GetUserID(context.Context, *User) (int, error) { return s.userID, s.getIDErr }
func (s *stubRepo) GetUserByEmail(context.Context, string) (*User, error) {
	return s.user, s.getErr
}
func (s *stubRepo) UpdatePassword(_ context.Context, _ int, p []byte) error {
	s.updatedPassword = p
	return s.updateErr
}
func (s *stubRepo) Activate(_ context.Context, id int) error {
	s.activatedID = id
	return s.activateErr
}

type stubTokens struct {
	mu sync.Mutex

	newErr    error
	getUser   *data.User
	getErr    error
	deleteErr error

	newScope     string
	deletedScope string
}

func (s *stubTokens) New(_ context.Context, userID int, _ time.Duration, scope string) (*security.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.newScope = scope
	if s.newErr != nil {
		return nil, s.newErr
	}
	return &security.Token{Plaintext: "plaintext-token", UserID: userID, Scope: scope}, nil
}
func (s *stubTokens) SaveToken(context.Context, *security.Token) error { return nil }
func (s *stubTokens) DeleteAllTokensUser(_ context.Context, scope string, _ int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletedScope = scope
	return s.deleteErr
}
func (s *stubTokens) GetForToken(context.Context, string, string) (*data.User, error) {
	return s.getUser, s.getErr
}
func (s *stubTokens) CreateAuthentication(context.Context, data.LoginRequest) (security.Token, error) {
	return security.Token{}, nil
}

// recordingMailer captures the message instead of dialling SMTP.
type recordingMailer struct {
	mu   sync.Mutex
	err  error
	sent bool
	to   string
	body string
}

func (m *recordingMailer) Send(_ context.Context, to, _, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent, m.to, m.body = true, to, body
	return m.err
}

func (m *recordingMailer) snapshot() (bool, string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sent, m.to, m.body
}

func validRequest() UserRequest {
	return UserRequest{UserName: "john", Name: "John Doe", Email: "john@example.com", Password: "supersecret"}
}

// --- InsertUser -------------------------------------------------------------

func TestInsertUser_HashesThePasswordBeforePersisting(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, nil, &stubTokens{})

	if err := svc.InsertUser(context.Background(), validRequest()); err != nil {
		t.Fatalf("InsertUser returned error: %v", err)
	}

	// The plaintext must never reach the repository.
	if repo.created.Password != "" {
		t.Error("the plaintext password survived into the persisted user")
	}
	if len(repo.created.HashedPassword) == 0 {
		t.Fatal("no hashed password was set")
	}
	if err := bcrypt.CompareHashAndPassword(repo.created.HashedPassword, []byte("supersecret")); err != nil {
		t.Errorf("the stored hash does not match the submitted password: %v", err)
	}
}

func TestInsertUser_IssuesAnActivationToken(t *testing.T) {
	tokens := &stubTokens{}
	svc := NewService(&stubRepo{}, nil, tokens)

	if err := svc.InsertUser(context.Background(), validRequest()); err != nil {
		t.Fatalf("InsertUser returned error: %v", err)
	}

	tokens.mu.Lock()
	defer tokens.mu.Unlock()
	if tokens.newScope != security.ScopeActivation {
		t.Errorf("token scope = %q, want %q", tokens.newScope, security.ScopeActivation)
	}
}

func TestInsertUser_RejectsAnInvalidRequestBeforeTouchingTheRepository(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, nil, &stubTokens{})

	if err := svc.InsertUser(context.Background(), UserRequest{}); err == nil {
		t.Fatal("expected a validation error")
	}
	if repo.created != nil {
		t.Error("an invalid request reached the repository")
	}
}

func TestInsertUser_PropagatesDuplicateUnchanged(t *testing.T) {
	svc := NewService(&stubRepo{createErr: ErrDuplicateUser}, nil, &stubTokens{})

	err := svc.InsertUser(context.Background(), validRequest())
	// The handler matches on this sentinel, so it must not be wrapped into
	// something errors.Is cannot see through.
	if !errors.Is(err, ErrDuplicateUser) {
		t.Errorf("err = %v, want ErrDuplicateUser", err)
	}
}

func TestInsertUser_PropagatesTokenError(t *testing.T) {
	svc := NewService(&stubRepo{}, nil, &stubTokens{newErr: errBoom})

	if err := svc.InsertUser(context.Background(), validRequest()); err == nil {
		t.Fatal("expected the activation token error to propagate")
	}
}

func TestInsertUser_SendsAWelcomeEmailCarryingTheActivationLink(t *testing.T) {
	m := &recordingMailer{}
	svc := NewService(&stubRepo{}, m, &stubTokens{})

	if err := svc.InsertUser(context.Background(), validRequest()); err != nil {
		t.Fatalf("InsertUser returned error: %v", err)
	}

	// The email goes out on a goroutine, so wait for it rather than sleeping a
	// fixed amount.
	var sent bool
	var to, body string
	for i := 0; i < 100; i++ {
		if sent, to, body = m.snapshot(); sent {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !sent {
		t.Fatal("no welcome email was sent")
	}
	if to != "john@example.com" {
		t.Errorf("recipient = %q", to)
	}
	if !strings.Contains(body, "plaintext-token") {
		t.Errorf("the activation token is missing from the email body: %q", body)
	}
	if !strings.Contains(body, "/api/v1/user/activate") {
		t.Errorf("the activation path is missing from the email body: %q", body)
	}
}

func TestInsertUser_SucceedsEvenIfTheEmailFails(t *testing.T) {
	// Registration must not fail because SMTP is down — the account exists and
	// the link can be resent.
	m := &recordingMailer{err: errBoom}
	svc := NewService(&stubRepo{}, m, &stubTokens{})

	if err := svc.InsertUser(context.Background(), validRequest()); err != nil {
		t.Errorf("err = %v, want nil despite the mail failure", err)
	}
}

func TestInsertUser_WithoutAMailerIsStillFine(t *testing.T) {
	svc := NewService(&stubRepo{}, nil, &stubTokens{})

	if err := svc.InsertUser(context.Background(), validRequest()); err != nil {
		t.Errorf("err = %v, want nil with a nil mailer", err)
	}
}

// --- ChangePassword ---------------------------------------------------------

func storedUser(t *testing.T, password string) *User {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	return &User{ID: 7, Email: "john@example.com", HashedPassword: hash}
}

func validChange() ChangePasswordRequest {
	return ChangePasswordRequest{
		LoginRequest: data.LoginRequest{Email: "john@example.com", Password: "currentPass123"},
		NewPassword:  "brandNewPass456",
	}
}

func TestChangePassword_StoresANewHashOfTheNewPassword(t *testing.T) {
	repo := &stubRepo{user: storedUser(t, "currentPass123")}
	svc := NewService(repo, nil, &stubTokens{})

	if err := svc.ChangePassword(context.Background(), validChange()); err != nil {
		t.Fatalf("ChangePassword returned error: %v", err)
	}
	if len(repo.updatedPassword) == 0 {
		t.Fatal("no new password was written")
	}
	if err := bcrypt.CompareHashAndPassword(repo.updatedPassword, []byte("brandNewPass456")); err != nil {
		t.Errorf("the stored hash is not the new password: %v", err)
	}
}

func TestChangePassword_RequiresTheCurrentPasswordToMatch(t *testing.T) {
	// Holding a session is not enough: the current password is re-verified, so
	// a stolen session cannot lock the owner out.
	repo := &stubRepo{user: storedUser(t, "somethingElse123")}
	svc := NewService(repo, nil, &stubTokens{})

	if err := svc.ChangePassword(context.Background(), validChange()); err == nil {
		t.Fatal("expected an error when the current password does not match")
	}
	if repo.updatedPassword != nil {
		t.Error("the password was changed despite the wrong current password")
	}
}

func TestChangePassword_RejectsAnInvalidRequest(t *testing.T) {
	repo := &stubRepo{user: storedUser(t, "currentPass123")}
	svc := NewService(repo, nil, &stubTokens{})

	req := validChange()
	req.NewPassword = req.LoginRequest.Password // unchanged

	if err := svc.ChangePassword(context.Background(), req); err == nil {
		t.Fatal("expected a validation error")
	}
	if repo.updatedPassword != nil {
		t.Error("an invalid request reached the repository")
	}
}

func TestChangePassword_PropagatesLookupAndUpdateErrors(t *testing.T) {
	svc := NewService(&stubRepo{getErr: errBoom}, nil, &stubTokens{})
	if err := svc.ChangePassword(context.Background(), validChange()); err == nil {
		t.Error("expected the lookup error to propagate")
	}

	svc = NewService(&stubRepo{user: storedUser(t, "currentPass123"), updateErr: errBoom}, nil, &stubTokens{})
	if err := svc.ChangePassword(context.Background(), validChange()); err == nil {
		t.Error("expected the update error to propagate")
	}
}

// --- Authenticate / ActiveUser ---------------------------------------------

func TestAuthenticate_DelegatesToTheRepository(t *testing.T) {
	svc := NewService(&stubRepo{userID: 7}, nil, &stubTokens{})

	got, err := svc.Authenticate(context.Background(), "john@example.com", "supersecret")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if got != 7 {
		t.Errorf("id = %d, want 7", got)
	}

	failing := NewService(&stubRepo{getIDErr: ErrInvalidCredentials}, nil, &stubTokens{})
	if _, err := failing.Authenticate(context.Background(), "x", "y"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestActiveUser_ActivatesAndBurnsTheToken(t *testing.T) {
	repo := &stubRepo{}
	tokens := &stubTokens{getUser: &data.User{ID: 7}}
	svc := NewService(repo, nil, tokens)

	if err := svc.ActiveUser(context.Background(), strings.Repeat("A", 26)); err != nil {
		t.Fatalf("ActiveUser returned error: %v", err)
	}
	if repo.activatedID != 7 {
		t.Errorf("activated id = %d, want 7", repo.activatedID)
	}
	// The activation tokens are deleted so the emailed link cannot be reused.
	tokens.mu.Lock()
	defer tokens.mu.Unlock()
	if tokens.deletedScope != security.ScopeActivation {
		t.Errorf("deleted scope = %q, want %q", tokens.deletedScope, security.ScopeActivation)
	}
}

func TestActiveUser_RejectsAMalformedTokenWithoutQueryingAnything(t *testing.T) {
	tokens := &stubTokens{getErr: errBoom} // would fail if it were reached
	svc := NewService(&stubRepo{}, nil, tokens)

	for _, token := range []string{"", "too-short", strings.Repeat("A", 27)} {
		if err := svc.ActiveUser(context.Background(), token); err == nil {
			t.Errorf("token %q: expected a validation error", token)
		}
	}
}

func TestActiveUser_PropagatesTokenLookupActivationAndDeleteErrors(t *testing.T) {
	valid := strings.Repeat("A", 26)

	svc := NewService(&stubRepo{}, nil, &stubTokens{getErr: security.ErrTokenNotFound})
	if err := svc.ActiveUser(context.Background(), valid); err == nil {
		t.Error("expected the token lookup error to propagate")
	}

	svc = NewService(&stubRepo{activateErr: errBoom}, nil, &stubTokens{getUser: &data.User{ID: 7}})
	if err := svc.ActiveUser(context.Background(), valid); err == nil {
		t.Error("expected the activation error to propagate")
	}

	svc = NewService(&stubRepo{}, nil, &stubTokens{getUser: &data.User{ID: 7}, deleteErr: errBoom})
	if err := svc.ActiveUser(context.Background(), valid); err == nil {
		t.Error("expected the token cleanup error to propagate")
	}
}
