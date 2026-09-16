package security

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"

	"github.com/jscodelab/mybasics-expenses/internal/data"
)

// authStubTokens drives the auth handler: CreateAuthentication decides the
// login outcome, DeleteAllTokensUser records the logout.
type authStubTokens struct {
	token     Token
	createErr error
	deleteErr error

	deletedScope  string
	deletedUserID int
	deleteCalls   int
}

func (s *authStubTokens) New(context.Context, int, time.Duration, string) (*Token, error) {
	return &Token{}, nil
}
func (s *authStubTokens) SaveToken(context.Context, *Token) error { return nil }
func (s *authStubTokens) DeleteAllTokensUser(_ context.Context, scope string, userID int) error {
	s.deletedScope, s.deletedUserID = scope, userID
	s.deleteCalls++
	return s.deleteErr
}
func (s *authStubTokens) GetForToken(context.Context, string, string) (*data.User, error) {
	return nil, ErrTokenNotFound
}
func (s *authStubTokens) CreateAuthentication(context.Context, data.LoginRequest) (Token, error) {
	return s.token, s.createErr
}

func serveAuth(t *testing.T, tokens TokenService, guarded bool, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	r := chi.NewRouter()
	h := NewAuthHandler(tokens)
	sec := NewHandler(scs.New())
	if guarded {
		r.Group(func(r chi.Router) {
			r.Use(sec.RequireAuthentication)
			h.RegisterRoutes(r)
			h.RegisterProtectedRoutes(r)
		})
	} else {
		h.RegisterRoutes(r)
		h.RegisterProtectedRoutes(r)
	}

	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	req = ContextSetUser(req, &data.User{ID: 42, Activated: true})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestCreateAuthenticationToken_Returns201WithTheToken(t *testing.T) {
	tokens := &authStubTokens{token: Token{Plaintext: "the-token", Expiry: time.Now().Add(time.Hour)}}

	rec := serveAuth(t, tokens, false, http.MethodPost, "/tokens/authentication",
		`{"email":"john@example.com","password":"supersecret"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %q)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "authentication_token") {
		t.Errorf("body = %q, want it to wrap the token", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "the-token") {
		t.Errorf("body = %q, want the plaintext token", rec.Body.String())
	}
}

func TestCreateAuthenticationToken_NeverSerialisesTheHashOrScope(t *testing.T) {
	// Token's Hash and Scope are json:"-": leaking the hash would hand out the
	// exact value stored in the database.
	tokens := &authStubTokens{token: Token{
		Plaintext: "the-token", Hash: []byte("secret-hash"),
		Scope: ScopeAuthentication, UserID: 42, Expiry: time.Now().Add(time.Hour),
	}}

	rec := serveAuth(t, tokens, false, http.MethodPost, "/tokens/authentication",
		`{"email":"john@example.com","password":"supersecret"}`)

	body := rec.Body.String()
	if strings.Contains(body, "secret-hash") {
		t.Error("the token hash leaked into the response")
	}
	// Checked as a field name: the literal scope value ("authentication") is a
	// substring of "authentication_token", so matching it raw is meaningless.
	if strings.Contains(body, `"scope"`) {
		t.Errorf("the token scope leaked into the response: %q", body)
	}
	if strings.Contains(body, `"user_id"`) {
		t.Errorf("the token user id leaked into the response: %q", body)
	}
}

func TestCreateAuthenticationToken_BadCredentialsAre401WithAGenericMessage(t *testing.T) {
	tokens := &authStubTokens{createErr: ErrInvalidCredentials}

	rec := serveAuth(t, tokens, false, http.MethodPost, "/tokens/authentication",
		`{"email":"john@example.com","password":"wrongpassword"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid email or password") {
		t.Errorf("body = %q, want the generic message", rec.Body.String())
	}
}

func TestCreateAuthenticationToken_MalformedJSONIs400(t *testing.T) {
	rec := serveAuth(t, &authStubTokens{}, false, http.MethodPost, "/tokens/authentication", `{"email":`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateAuthenticationToken_ValidationIs400NotA401(t *testing.T) {
	// A short password is a format error, not a credential error: answering
	// 401 here would be misleading, and the message leaks nothing.
	rec := serveAuth(t, &authStubTokens{}, false, http.MethodPost, "/tokens/authentication",
		`{"email":"john@example.com","password":"short"}`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateAuthenticationToken_InfrastructureErrorIs500(t *testing.T) {
	rec := serveAuth(t, &authStubTokens{createErr: errBoom}, false, http.MethodPost,
		"/tokens/authentication", `{"email":"john@example.com","password":"supersecret"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestLogout_DeletesEveryAuthenticationTokenOfTheUser(t *testing.T) {
	tokens := &authStubTokens{}

	rec := serveAuth(t, tokens, true, http.MethodPost, "/tokens/logout", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if tokens.deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", tokens.deleteCalls)
	}
	// The user comes from the context, never from the body — otherwise one user
	// could log another one out.
	if tokens.deletedUserID != 42 {
		t.Errorf("deleted userID = %d, want 42", tokens.deletedUserID)
	}
	// Scoped, so a pending activation token survives a logout.
	if tokens.deletedScope != ScopeAuthentication {
		t.Errorf("deleted scope = %q, want %q", tokens.deletedScope, ScopeAuthentication)
	}
}

func TestLogout_RepositoryErrorIs500(t *testing.T) {
	rec := serveAuth(t, &authStubTokens{deleteErr: errBoom}, true, http.MethodPost, "/tokens/logout", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestLogout_WithoutAGuardIs401AndDeletesNothing(t *testing.T) {
	tokens := &authStubTokens{}

	rec := serveAuth(t, tokens, false, http.MethodPost, "/tokens/logout", "")

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if tokens.deleteCalls != 0 {
		t.Error("tokens were deleted for an unidentified caller")
	}
}
