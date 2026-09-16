package balance

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/jscodelab/mybasics-expenses/internal/data"
	"github.com/jscodelab/mybasics-expenses/internal/security"
)

type mockService struct {
	summary *Summary
	periods []PeriodSummary
	err     error

	gotUserID      int
	gotFrom, gotTo string
}

func (m *mockService) GetBalance(_ context.Context, userID int, dateFrom, dateTo string) (*Summary, error) {
	m.gotUserID, m.gotFrom, m.gotTo = userID, dateFrom, dateTo
	return m.summary, m.err
}
func (m *mockService) GetPeriods(_ context.Context, userID int) ([]PeriodSummary, error) {
	m.gotUserID = userID
	return m.periods, m.err
}

const testUserID = 42

func serve(t *testing.T, svc Service, guarded bool, target string) *httptest.ResponseRecorder {
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

	req := httptest.NewRequest(http.MethodGet, target, nil)
	req = security.ContextSetUser(req, &data.User{ID: testUserID, Activated: true})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestHandlerGet_PassesTheDateRangeThroughAndScopesTheUser(t *testing.T) {
	svc := &mockService{summary: &Summary{Balance: 600}}

	rec := serve(t, svc, true, "/balance?date_from=2026-07-01&date_to=2026-07-31")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.gotFrom != "2026-07-01" || svc.gotTo != "2026-07-31" {
		t.Errorf("dates = (%q, %q)", svc.gotFrom, svc.gotTo)
	}
	if svc.gotUserID != testUserID {
		t.Errorf("service got userID = %d, want %d", svc.gotUserID, testUserID)
	}
}

func TestHandlerGet_MalformedDateIs400NotA500(t *testing.T) {
	// A bad date is the caller's mistake. The handler distinguishes it from a
	// genuine failure by the message, so this pins that mapping.
	svc := &mockService{err: errors.New("invalid date_from: expected YYYY-MM-DD, got \"nope\"")}

	rec := serve(t, svc, true, "/balance?date_from=nope")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerGet_OtherServiceErrorIs500(t *testing.T) {
	rec := serve(t, &mockService{err: errors.New("db down")}, true, "/balance")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerGet_WithoutAGuardIs401(t *testing.T) {
	rec := serve(t, &mockService{}, false, "/balance")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestHandlerPeriods_ReturnsThePeriods(t *testing.T) {
	svc := &mockService{periods: []PeriodSummary{{Balance: 100}}}

	rec := serve(t, svc, true, "/balance/periods")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.gotUserID != testUserID {
		t.Errorf("service got userID = %d, want %d", svc.gotUserID, testUserID)
	}
}

func TestHandlerPeriods_ServiceErrorIs500(t *testing.T) {
	rec := serve(t, &mockService{err: errors.New("db down")}, true, "/balance/periods")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerPeriods_WithoutAGuardIs401(t *testing.T) {
	rec := serve(t, &mockService{}, false, "/balance/periods")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestIsDateError(t *testing.T) {
	// The discriminator is the format hint the service puts in the message.
	if !isDateError(errors.New(`invalid date_to: expected YYYY-MM-DD, got "x"`)) {
		t.Error("a date parse error was not recognised")
	}
	if isDateError(errors.New("connection refused")) {
		t.Error("an unrelated error was misread as a date error")
	}
}
