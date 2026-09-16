package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jscodelab/mybasics-expenses/internal/data"
	"github.com/jscodelab/mybasics-expenses/internal/security"
)

// authTokens drives the authenticate middleware: it resolves a bearer token to
// a user, or fails the way the real token service does.
type authTokens struct {
	user      *data.User
	err       error
	gotScope  string
	gotToken  string
	callCount int
}

func (a *authTokens) New(context.Context, int, time.Duration, string) (*security.Token, error) {
	return &security.Token{}, nil
}
func (a *authTokens) SaveToken(context.Context, *security.Token) error       { return nil }
func (a *authTokens) DeleteAllTokensUser(context.Context, string, int) error { return nil }
func (a *authTokens) GetForToken(_ context.Context, scope, token string) (*data.User, error) {
	a.gotScope, a.gotToken = scope, token
	a.callCount++
	return a.user, a.err
}
func (a *authTokens) CreateAuthentication(context.Context, data.LoginRequest) (security.Token, error) {
	return security.Token{}, nil
}

// serveAuthenticate runs the middleware and reports the status plus the user it
// put on the context.
func serveAuthenticate(t *testing.T, tokens security.TokenService, header string) (*httptest.ResponseRecorder, *data.User) {
	t.Helper()

	app := &Application{}
	app.Tokens.Services = tokens

	var got *data.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = security.UserFromContext(r)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}

	rec := httptest.NewRecorder()
	app.authenticate(next).ServeHTTP(rec, req)
	return rec, got
}

func TestAuthenticate_NoHeaderContinuesAsAnonymous(t *testing.T) {
	tokens := &authTokens{}

	rec, got := serveAuthenticate(t, tokens, "")

	// No header is not an error here — public routes still need to work. The
	// guards further down decide whether anonymous is acceptable.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got != data.AnonymousUser {
		t.Errorf("context user = %v, want the anonymous sentinel", got)
	}
	if tokens.callCount != 0 {
		t.Error("the database was queried for a request carrying no token")
	}
}

func TestAuthenticate_ValidTokenPutsTheUserOnTheContext(t *testing.T) {
	tokens := &authTokens{user: &data.User{ID: 7, Email: "john@example.com", Activated: true}}

	rec, got := serveAuthenticate(t, tokens, "Bearer the-token")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got == nil || got.ID != 7 {
		t.Fatalf("context user = %v, want the resolved user", got)
	}
	if tokens.gotToken != "the-token" {
		t.Errorf("token = %q, want the value after the scheme", tokens.gotToken)
	}
	// Only authentication-scoped tokens may authenticate: an activation token
	// must not double as a session.
	if tokens.gotScope != security.ScopeAuthentication {
		t.Errorf("scope = %q, want %q", tokens.gotScope, security.ScopeAuthentication)
	}
}

func TestAuthenticate_SetsVaryOnAuthorization(t *testing.T) {
	// Responses differ by token, so a shared cache must key on the header or it
	// could serve one user's data to another.
	rec, _ := serveAuthenticate(t, &authTokens{}, "")

	if v := rec.Header().Get("Vary"); !strings.Contains(v, "Authorization") {
		t.Errorf("Vary = %q, want it to include Authorization", v)
	}
}

func TestAuthenticate_RejectsAMalformedHeader(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{"missing scheme", "the-token"},
		{"wrong scheme", "Basic dXNlcjpwYXNz"},
		{"too many parts", "Bearer a b"},
		{"empty token", "Bearer "},
		{"lowercase scheme", "bearer the-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := &authTokens{}
			rec, _ := serveAuthenticate(t, tokens, tt.header)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			// The header must be rejected before any database work.
			if tokens.callCount != 0 {
				t.Error("a malformed header still triggered a token lookup")
			}
			if a := rec.Header().Get("WWW-Authenticate"); a != "Bearer" {
				t.Errorf("WWW-Authenticate = %q, want Bearer", a)
			}
		})
	}
}

func TestAuthenticate_UnknownTokenIs401WithoutRevealingWhy(t *testing.T) {
	rec, _ := serveAuthenticate(t, &authTokens{err: security.ErrTokenNotFound}, "Bearer stale-token")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "invalid or missing authentication token") {
		t.Errorf("body = %q, want the generic message", body)
	}
	// Expired and unknown must look identical, so a caller cannot probe which
	// tokens once existed.
	if strings.Contains(body, "expired") {
		t.Errorf("body = %q, leaks why the token was rejected", body)
	}
}

func TestAuthenticate_InfrastructureErrorIs500NotA401(t *testing.T) {
	// A broken database must not be served as "your token is bad" — that would
	// hide an outage behind a login problem.
	rec, _ := serveAuthenticate(t, &authTokens{err: errors.New("db down")}, "Bearer the-token")

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}
