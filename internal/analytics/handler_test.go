package analytics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/jscodelab/mybasics-expenses/internal/data"
	"github.com/jscodelab/mybasics-expenses/internal/security"
)

type mockService struct {
	err error

	gotFilter Filter
	gotLimit  int
}

func (m *mockService) Summary(_ context.Context, f Filter) (*Summary, error) {
	m.gotFilter = f
	return &Summary{}, m.err
}
func (m *mockService) ByCategory(_ context.Context, f Filter) (*ByCategory, error) {
	m.gotFilter = f
	return &ByCategory{}, m.err
}
func (m *mockService) Trend(_ context.Context, f Filter) (*Trend, error) {
	m.gotFilter = f
	return &Trend{}, m.err
}
func (m *mockService) TopExpenses(_ context.Context, f Filter, limit int) (*TopExpenses, error) {
	m.gotFilter, m.gotLimit = f, limit
	return &TopExpenses{}, m.err
}
func (m *mockService) IncomeVsExpense(_ context.Context, f Filter) (*IncomeVsExpense, error) {
	m.gotFilter = f
	return &IncomeVsExpense{}, m.err
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

// allRoutes is every analytics endpoint. They share parseFilter and the same
// error mapping, so the common behaviour is asserted across all of them rather
// than repeated five times.
var allRoutes = []string{
	"/analytics/summary",
	"/analytics/by-category",
	"/analytics/trend",
	"/analytics/top-expenses",
	"/analytics/income-vs-expense",
}

func TestHandlers_ReturnOKAndScopeToTheAuthenticatedUser(t *testing.T) {
	for _, route := range allRoutes {
		t.Run(route, func(t *testing.T) {
			svc := &mockService{}
			rec := serve(t, svc, true, route)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if svc.gotFilter.UserID != testUserID {
				t.Errorf("filter UserID = %d, want %d", svc.gotFilter.UserID, testUserID)
			}
			if svc.gotFilter.Months != 3 {
				t.Errorf("months = %d, want the default 3", svc.gotFilter.Months)
			}
		})
	}
}

func TestHandlers_ServiceErrorIs500(t *testing.T) {
	for _, route := range allRoutes {
		t.Run(route, func(t *testing.T) {
			rec := serve(t, &mockService{err: errStub}, true, route)
			if rec.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want 500", rec.Code)
			}
		})
	}
}

func TestHandlers_WithoutAGuardAre401(t *testing.T) {
	for _, route := range allRoutes {
		t.Run(route, func(t *testing.T) {
			rec := serve(t, &mockService{}, false, route)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
		})
	}
}

func TestHandlers_RejectAnOutOfRangeMonths(t *testing.T) {
	for _, route := range allRoutes {
		for _, months := range []string{"0", "13", "-1", "abc"} {
			t.Run(route+"?months="+months, func(t *testing.T) {
				rec := serve(t, &mockService{}, true, route+"?months="+months)
				if rec.Code != http.StatusBadRequest {
					t.Errorf("status = %d, want 400", rec.Code)
				}
			})
		}
	}
}

func TestHandlers_AcceptTheBoundaryMonths(t *testing.T) {
	for _, months := range []string{"1", "12"} {
		t.Run("months="+months, func(t *testing.T) {
			svc := &mockService{}
			rec := serve(t, svc, true, "/analytics/summary?months="+months)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if svc.gotFilter.Months < 1 || svc.gotFilter.Months > 12 {
				t.Errorf("months = %d, outside the accepted range", svc.gotFilter.Months)
			}
		})
	}
}

func TestTopExpenses_DefaultsTheLimitToTen(t *testing.T) {
	svc := &mockService{}

	serve(t, svc, true, "/analytics/top-expenses")

	if svc.gotLimit != 10 {
		t.Errorf("limit = %d, want the default 10", svc.gotLimit)
	}
}

func TestTopExpenses_AcceptsAnExplicitLimit(t *testing.T) {
	svc := &mockService{}

	serve(t, svc, true, "/analytics/top-expenses?limit=25")

	if svc.gotLimit != 25 {
		t.Errorf("limit = %d, want 25", svc.gotLimit)
	}
}

func TestTopExpenses_RejectsAnOutOfRangeLimit(t *testing.T) {
	// The cap exists so a single request cannot ask for an unbounded result.
	for _, limit := range []string{"0", "51", "-1", "abc"} {
		t.Run("limit="+limit, func(t *testing.T) {
			rec := serve(t, &mockService{}, true, "/analytics/top-expenses?limit="+limit)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestTopExpenses_AcceptsTheBoundaryLimits(t *testing.T) {
	for _, limit := range []string{"1", "50"} {
		t.Run("limit="+limit, func(t *testing.T) {
			rec := serve(t, &mockService{}, true, "/analytics/top-expenses?limit="+limit)
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want 200 for a valid boundary", rec.Code)
			}
		})
	}
}
