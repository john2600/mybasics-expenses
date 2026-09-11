package security

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jscodelab/mybasics-expenses/internal/data"
)

// nextProbe returns a handler that records whether it ran, so each test can
// assert that the guard let the request through — or stopped it before the
// protected handler ever saw it.
func nextProbe(called *bool, gotID *int, gotOK *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		id, ok := UserID(r.Context())
		*gotID, *gotOK = id, ok
		w.WriteHeader(http.StatusOK)
	})
}

// guard is a middleware under test, taken from *Security.
type guard func(*Security) func(http.Handler) http.Handler

var (
	activatedGuard guard = func(s *Security) func(http.Handler) http.Handler {
		return s.RequireActivatedUserForThisEndpoint
	}
	authOnlyGuard guard = func(s *Security) func(http.Handler) http.Handler {
		return s.RequireAuthentication
	}
)

// serveWith runs the given guard over a request whose context carries the given
// user, mirroring what the authenticate middleware does in production. A nil
// user means authenticate never stored one.
func serveWith(t *testing.T, g guard, user *data.User) (*httptest.ResponseRecorder, bool, int, bool) {
	t.Helper()

	var called, gotOK bool
	var gotID int

	s := NewHandler(nil) // the token guards never touch the session manager
	r := httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil)
	if user != nil {
		r = ContextSetUser(r, user)
	}

	rec := httptest.NewRecorder()
	g(s)(nextProbe(&called, &gotID, &gotOK)).ServeHTTP(rec, r)

	return rec, called, gotID, gotOK
}

// serveGuard runs the activation guard, the stricter of the two.
func serveGuard(t *testing.T, user *data.User) (*httptest.ResponseRecorder, bool, int, bool) {
	t.Helper()
	return serveWith(t, activatedGuard, user)
}

func TestRequireActivatedUser_ActivatedUserPasses(t *testing.T) {
	user := &data.User{ID: 42, Email: "john@example.com", Activated: true}

	rec, called, gotID, gotOK := serveGuard(t, user)

	if !called {
		t.Fatal("expected the guard to call the next handler for an activated user")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	// The whole point of the guard: bridge the token user into userIDKey so
	// downstream handlers keep working through RequireUserID, unchanged.
	if !gotOK {
		t.Fatal("expected the user id to be present in the downstream context")
	}
	if gotID != 42 {
		t.Errorf("downstream user id = %d, want 42", gotID)
	}
}

func TestRequireActivatedUser_AnonymousRejected(t *testing.T) {
	rec, called, _, _ := serveGuard(t, data.AnonymousUser)

	if called {
		t.Fatal("the next handler must not run for an anonymous user")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"error":"not authenticated"}` {
		t.Errorf("body = %q, want %q", body, `{"error":"not authenticated"}`)
	}
}

func TestRequireActivatedUser_NoUserInContextRejected(t *testing.T) {
	// authenticate did not run (or a route was mounted outside it): there is no
	// user at all. That must fail closed, exactly like an anonymous user.
	rec, called, _, _ := serveGuard(t, nil)

	if called {
		t.Fatal("the next handler must not run when no user is in the context")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"error":"not authenticated"}` {
		t.Errorf("body = %q, want %q", body, `{"error":"not authenticated"}`)
	}
}

func TestRequireActivatedUser_NotActivatedRejected(t *testing.T) {
	// A real, authenticated user who never followed the activation link: the
	// token is valid, so this is 403 (identified but not allowed), not 401.
	user := &data.User{ID: 7, Email: "pending@example.com", Activated: false}

	rec, called, _, _ := serveGuard(t, user)

	if called {
		t.Fatal("the next handler must not run for a non-activated user")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"error":"account not activated"}` {
		t.Errorf("body = %q, want %q", body, `{"error":"account not activated"}`)
	}
}

func TestRequireActivatedUser_DoesNotLeakIDOnRejection(t *testing.T) {
	// A rejected request must never reach a handler, but guard against a future
	// refactor that writes the id before checking: RequireUserID on the original
	// request must still report "not authenticated".
	user := &data.User{ID: 99, Activated: false}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil)
	r = ContextSetUser(r, user)

	s := NewHandler(nil)
	s.RequireActivatedUserForThisEndpoint(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	).ServeHTTP(httptest.NewRecorder(), r)

	if _, ok := UserID(r.Context()); ok {
		t.Error("the guard must not put a user id in the context of a rejected request")
	}
}

func TestContextSetUser_RoundTrip(t *testing.T) {
	user := &data.User{ID: 3, Email: "round@example.com", Activated: true}

	r := ContextSetUser(httptest.NewRequest(http.MethodGet, "/", nil), user)

	if got := UserFromContext(r); got != user {
		t.Errorf("UserFromContext = %v, want the user that was set", got)
	}
}

func TestUserFromContext_AbsentIsNil(t *testing.T) {
	// UserFromContext must distinguish "authenticate never ran" (nil) from
	// "anonymous" (AnonymousUser) — the guard relies on both being rejected.
	if got := UserFromContext(httptest.NewRequest(http.MethodGet, "/", nil)); got != nil {
		t.Errorf("UserFromContext on a bare request = %v, want nil", got)
	}
}

// --- RequireAuthentication: identity only, no activation check ---------------

func TestRequireAuthentication_ActivatedUserPasses(t *testing.T) {
	user := &data.User{ID: 42, Email: "john@example.com", Activated: true}

	rec, called, gotID, gotOK := serveWith(t, authOnlyGuard, user)

	if !called {
		t.Fatal("expected the guard to call the next handler for an activated user")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !gotOK || gotID != 42 {
		t.Errorf("downstream user id = (%d, %t), want (42, true)", gotID, gotOK)
	}
}

func TestRequireAuthentication_NotActivatedAlsoPasses(t *testing.T) {
	// This is the whole reason the two guards exist separately: a registered but
	// never-activated user still holds a valid token, and must be able to log out
	// and change their password. Only the activation guard turns this into a 403.
	user := &data.User{ID: 7, Email: "pending@example.com", Activated: false}

	rec, called, gotID, gotOK := serveWith(t, authOnlyGuard, user)

	if !called {
		t.Fatal("RequireAuthentication must let a non-activated user through")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !gotOK || gotID != 7 {
		t.Errorf("downstream user id = (%d, %t), want (7, true)", gotID, gotOK)
	}
}

func TestRequireAuthentication_AnonymousRejected(t *testing.T) {
	rec, called, _, _ := serveWith(t, authOnlyGuard, data.AnonymousUser)

	if called {
		t.Fatal("the next handler must not run for an anonymous user")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"error":"not authenticated"}` {
		t.Errorf("body = %q, want %q", body, `{"error":"not authenticated"}`)
	}
}

func TestRequireAuthentication_NoUserInContextRejected(t *testing.T) {
	rec, called, _, _ := serveWith(t, authOnlyGuard, nil)

	if called {
		t.Fatal("the next handler must not run when no user is in the context")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// TestGuards_DifferOnlyOnActivation states the contract between the two guards
// as a single table: identical for every input except the one case the
// activation check exists for.
func TestGuards_DifferOnlyOnActivation(t *testing.T) {
	tests := []struct {
		name          string
		user          *data.User
		wantAuthOnly  int
		wantActivated int
	}{
		{"no user", nil, http.StatusUnauthorized, http.StatusUnauthorized},
		{"anonymous", data.AnonymousUser, http.StatusUnauthorized, http.StatusUnauthorized},
		{"activated", &data.User{ID: 1, Activated: true}, http.StatusOK, http.StatusOK},
		{"not activated", &data.User{ID: 2, Activated: false}, http.StatusOK, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec, _, _, _ := serveWith(t, authOnlyGuard, tt.user); rec.Code != tt.wantAuthOnly {
				t.Errorf("RequireAuthentication status = %d, want %d", rec.Code, tt.wantAuthOnly)
			}
			if rec, _, _, _ := serveWith(t, activatedGuard, tt.user); rec.Code != tt.wantActivated {
				t.Errorf("RequireActivatedUserForThisEndpoint status = %d, want %d", rec.Code, tt.wantActivated)
			}
		})
	}
}
