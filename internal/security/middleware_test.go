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

// serveGuard runs RequireActivatedUserForThisEndpoint over a request whose
// context carries the given user, mirroring what the authenticate middleware
// does in production. A nil user means authenticate never stored one.
func serveGuard(t *testing.T, user *data.User) (*httptest.ResponseRecorder, bool, int, bool) {
	t.Helper()

	var called, gotOK bool
	var gotID int

	s := NewHandler(nil) // the token guard never touches the session manager
	r := httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil)
	if user != nil {
		r = ContextSetUser(r, user)
	}

	rec := httptest.NewRecorder()
	s.RequireActivatedUserForThisEndpoint(nextProbe(&called, &gotID, &gotOK)).ServeHTTP(rec, r)

	return rec, called, gotID, gotOK
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
	if body := strings.TrimSpace(rec.Body.String()); body != `{"error":"user not is not active"}` {
		t.Errorf("body = %q, want %q", body, `{"error":"user not is not active"}`)
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
