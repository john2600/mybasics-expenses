package incomes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/jscodelab/mybasics-expenses/internal/data"
	"github.com/jscodelab/mybasics-expenses/internal/security"
)

type mockService struct {
	config *Config
	err    error

	gotUserID int
	gotUpdate UpdateRequest
}

func (m *mockService) GetConfig(_ context.Context, userID int) (*Config, error) {
	m.gotUserID = userID
	return m.config, m.err
}
func (m *mockService) UpdateConfig(_ context.Context, userID int, req UpdateRequest) (*Config, error) {
	m.gotUserID, m.gotUpdate = userID, req
	return m.config, m.err
}

const testUserID = 42

// serve runs the request through the real security guard so the user id lands
// on the context the way it does in production. guarded=false leaves it off,
// which is how the handler's own "no user id" branch is reachable.
func serve(t *testing.T, svc Service, guarded bool, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	r := chi.NewRouter()
	h := NewHandler(svc)
	if guarded {
		r.Group(func(r chi.Router) {
			r.Use(security.NewHandler(nil).RequireAuthentication)
			h.RegisterRoutes(r)
		})
	} else {
		h.RegisterRoutes(r)
	}

	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	req = security.ContextSetUser(req, &data.User{ID: testUserID, Activated: true})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestHandlerGet_ReturnsTheConfigForTheAuthenticatedUser(t *testing.T) {
	svc := &mockService{config: &Config{Amount: 3000000, CutDay: 24}}

	rec := serve(t, svc, true, http.MethodGet, "/incomes/config", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// The owner comes from the token, never from the query string.
	if svc.gotUserID != testUserID {
		t.Errorf("service got userID = %d, want %d", svc.gotUserID, testUserID)
	}
}

func TestHandlerGet_ServiceErrorIs500(t *testing.T) {
	rec := serve(t, &mockService{err: errStub}, true, http.MethodGet, "/incomes/config", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerGet_WithoutAGuardIs401(t *testing.T) {
	rec := serve(t, &mockService{}, false, http.MethodGet, "/incomes/config", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestHandlerUpdate_DecodesThePatch(t *testing.T) {
	svc := &mockService{config: &Config{Amount: 4000000, CutDay: 15}}

	rec := serve(t, svc, true, http.MethodPut, "/incomes/config", `{"amount":4000000,"cut_day":15}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.gotUpdate.Amount == nil || *svc.gotUpdate.Amount != 4000000 {
		t.Errorf("amount = %v, want 4000000", svc.gotUpdate.Amount)
	}
	if svc.gotUpdate.CutDay == nil || *svc.gotUpdate.CutDay != 15 {
		t.Errorf("cut_day = %v, want 15", svc.gotUpdate.CutDay)
	}
	// An omitted field stays nil so the repository keeps its previous value.
	if svc.gotUpdate.Description != nil {
		t.Errorf("description = %v, want nil", *svc.gotUpdate.Description)
	}
}

func TestHandlerUpdate_MalformedJSONIs400(t *testing.T) {
	rec := serve(t, &mockService{}, true, http.MethodPut, "/incomes/config", `{"amount":`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerUpdate_ValidationErrorIs400(t *testing.T) {
	rec := serve(t, &mockService{err: errStub}, true, http.MethodPut, "/incomes/config", `{"cut_day":99}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerUpdate_WithoutAGuardIs401(t *testing.T) {
	rec := serve(t, &mockService{}, false, http.MethodPut, "/incomes/config", `{"amount":1}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
