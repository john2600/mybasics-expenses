package security

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
)

// loadedSession returns a request whose context carries a live scs session,
// which is what LoadAndSave produces in production.
func loadedSession(t *testing.T, sm *scs.SessionManager, userID *int) *http.Request {
	t.Helper()

	ctx, err := sm.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("loading session: %v", err)
	}
	if userID != nil {
		sm.Put(ctx, "authenticatedUserID", *userID)
	}

	return httptest.NewRequest(http.MethodGet, "/movements", nil).WithContext(ctx)
}

// RestrictEndpoint is the legacy cookie-session guard. It is currently commented
// out in the router while auth migrates to bearer tokens, but it still compiles
// and is still exported, so its contract is pinned here — including that it
// feeds the same userIDKey as the token guards, which is what lets handlers stay
// agnostic to the auth method.
func TestRestrictEndpoint_AllowsASessionWithAUserAndBridgesTheID(t *testing.T) {
	sm := scs.New()
	s := NewHandler(sm)

	var gotID int
	var gotOK bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID, gotOK = UserID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	id := 42
	rec := httptest.NewRecorder()
	s.RestrictEndpoint(next).ServeHTTP(rec, loadedSession(t, sm, &id))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !gotOK || gotID != 42 {
		t.Errorf("downstream user id = (%d, %t), want (42, true)", gotID, gotOK)
	}
}

func TestRestrictEndpoint_RejectsASessionWithoutAUser(t *testing.T) {
	sm := scs.New()
	s := NewHandler(sm)

	called := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })

	rec := httptest.NewRecorder()
	s.RestrictEndpoint(next).ServeHTTP(rec, loadedSession(t, sm, nil))

	if called {
		t.Fatal("the next handler ran for an unauthenticated session")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	// The same body as the token guard, so a client cannot tell which
	// mechanism rejected it.
	if body := strings.TrimSpace(rec.Body.String()); body != `{"error":"not authenticated"}` {
		t.Errorf("body = %q, want %q", body, `{"error":"not authenticated"}`)
	}
}

func TestRestrictEndpoint_SetsNoStoreOnAllowedResponses(t *testing.T) {
	// Responses behind the guard are per-user, so they must never be cached by
	// a shared proxy and served to somebody else.
	sm := scs.New()
	s := NewHandler(sm)

	id := 7
	rec := httptest.NewRecorder()
	s.RestrictEndpoint(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(rec, loadedSession(t, sm, &id))

	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
}

func TestIsAuthenticated(t *testing.T) {
	sm := scs.New()
	s := NewHandler(sm)

	if s.isAuthenticated(loadedSession(t, sm, nil)) {
		t.Error("an empty session reported as authenticated")
	}

	id := 1
	if !s.isAuthenticated(loadedSession(t, sm, &id)) {
		t.Error("a session holding a user id reported as unauthenticated")
	}
}
