package security

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// withUserID returns a request whose context carries the given user id, the way
// RestrictEndpoint populates it in production.
func withUserID(id int) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	return r.WithContext(context.WithValue(r.Context(), userIDKey, id))
}

func TestRequireUserID_Authenticated(t *testing.T) {
	rec := httptest.NewRecorder()
	r := withUserID(42)

	id, ok := RequireUserID(rec, r)

	if !ok {
		t.Fatal("expected ok=true when the context has a user id")
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}
	// Nothing must be written to the response on the happy path.
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (untouched)", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body should be empty, got %q", rec.Body.String())
	}
}

func TestRequireUserID_Unauthenticated(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil) // no user id in context

	id, ok := RequireUserID(rec, r)

	if ok {
		t.Fatal("expected ok=false when the context has no user id")
	}
	if id != 0 {
		t.Errorf("id = %d, want 0", id)
	}
	// The exact 401 contract the frontend relies on must be preserved.
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"error":"not authenticated"}` {
		t.Errorf("body = %q, want %q", body, `{"error":"not authenticated"}`)
	}
}

func TestRequireUserID_WrongTypeInContext(t *testing.T) {
	// A non-int value under the key must be treated as "not authenticated",
	// never coerced — this guards against a future wiring mistake.
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(context.WithValue(r.Context(), userIDKey, "42"))

	if _, ok := RequireUserID(rec, r); ok {
		t.Fatal("expected ok=false for a non-int user id in context")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestUserID(t *testing.T) {
	if id, ok := UserID(withUserID(7).Context()); !ok || id != 7 {
		t.Errorf("UserID = (%d, %t), want (7, true)", id, ok)
	}
	if id, ok := UserID(context.Background()); ok || id != 0 {
		t.Errorf("UserID on empty ctx = (%d, %t), want (0, false)", id, ok)
	}
}
