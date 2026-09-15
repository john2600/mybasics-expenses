package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/jscodelab/mybasics-expenses/internal/analytics"
	"github.com/jscodelab/mybasics-expenses/internal/balance"
	"github.com/jscodelab/mybasics-expenses/internal/category"
	"github.com/jscodelab/mybasics-expenses/internal/data"
	"github.com/jscodelab/mybasics-expenses/internal/incomes"
	"github.com/jscodelab/mybasics-expenses/internal/movement"
	"github.com/jscodelab/mybasics-expenses/internal/reports"
	"github.com/jscodelab/mybasics-expenses/internal/security"
	"github.com/jscodelab/mybasics-expenses/internal/users"
)

// These tests cover the *wiring* in NewRouter: which guard protects which route.
// They deliberately say nothing about what a handler does once the guard lets
// the request through — that belongs to each module's own tests. The assertion
// is therefore the guard's decision: 401 (identity not proven), 403 (identity
// known, account not activated), or "allowed through".
//
// Without this, moving a RegisterRoutes call between the two r.Group blocks in
// router.go silently changes who can read financial data, and every other test
// in the repo stays green.

// --- stubs ------------------------------------------------------------------

// stubTokens resolves a bearer token to a user from a fixed map, standing in for
// the database lookup that authenticate performs.
type stubTokens struct{ byToken map[string]*data.User }

func (s stubTokens) GetForToken(_ context.Context, _, token string) (*data.User, error) {
	if u, ok := s.byToken[token]; ok {
		return u, nil
	}
	return nil, security.ErrTokenNotFound
}
func (s stubTokens) New(context.Context, int, time.Duration, string) (*security.Token, error) {
	return &security.Token{}, nil
}
func (s stubTokens) SaveToken(context.Context, *security.Token) error       { return nil }
func (s stubTokens) DeleteAllTokensUser(context.Context, string, int) error { return nil }
func (s stubTokens) CreateAuthentication(context.Context, data.LoginRequest) (security.Token, error) {
	return security.Token{}, nil
}

type stubUsers struct{}

func (stubUsers) InsertUser(context.Context, users.UserRequest) error               { return nil }
func (stubUsers) ChangePassword(context.Context, users.ChangePasswordRequest) error { return nil }
func (stubUsers) Authenticate(context.Context, string, string) (int, error)         { return 1, nil }
func (stubUsers) ActiveUser(context.Context, string) error                          { return nil }

type stubMovements struct{}

func (stubMovements) CreateMovement(context.Context, int, movement.CreateRequest) (*movement.Movement, error) {
	return &movement.Movement{}, nil
}
func (stubMovements) ListMovements(context.Context, movement.Filter) ([]movement.GroupedByCategory, error) {
	return nil, nil
}
func (stubMovements) ListExpenses(context.Context, movement.Filter) (*movement.ExpenseList, error) {
	return &movement.ExpenseList{}, nil
}
func (stubMovements) GetMovement(context.Context, int, int64) (*movement.Movement, error) {
	return &movement.Movement{}, nil
}
func (stubMovements) UpdateMovement(context.Context, int, int64, movement.UpdateRequest) (*movement.Movement, error) {
	return &movement.Movement{}, nil
}
func (stubMovements) DeleteMovement(context.Context, int, int64) error { return nil }
func (stubMovements) GetMonthlySummary(context.Context, int) ([]movement.MonthlySummary, error) {
	return nil, nil
}

type stubBalance struct{}

func (stubBalance) GetBalance(context.Context, int, string, string) (*balance.Summary, error) {
	return &balance.Summary{}, nil
}
func (stubBalance) GetPeriods(context.Context, int) ([]balance.PeriodSummary, error) {
	return nil, nil
}

type stubAnalytics struct{}

func (stubAnalytics) Summary(context.Context, analytics.Filter) (*analytics.Summary, error) {
	return &analytics.Summary{}, nil
}
func (stubAnalytics) ByCategory(context.Context, analytics.Filter) (*analytics.ByCategory, error) {
	return &analytics.ByCategory{}, nil
}
func (stubAnalytics) Trend(context.Context, analytics.Filter) (*analytics.Trend, error) {
	return &analytics.Trend{}, nil
}
func (stubAnalytics) TopExpenses(context.Context, analytics.Filter, int) (*analytics.TopExpenses, error) {
	return &analytics.TopExpenses{}, nil
}
func (stubAnalytics) IncomeVsExpense(context.Context, analytics.Filter) (*analytics.IncomeVsExpense, error) {
	return &analytics.IncomeVsExpense{}, nil
}

type stubReports struct{}

func (stubReports) Export(context.Context, reports.ExportFilter) (*reports.ExportReport, error) {
	return &reports.ExportReport{}, nil
}
func (stubReports) RenderCSV(*reports.ExportReport) ([]byte, error) { return []byte{}, nil }
func (stubReports) RenderPDF(*reports.ExportReport) ([]byte, error) { return []byte{}, nil }

type stubIncomes struct{}

func (stubIncomes) GetConfig(context.Context, int) (*incomes.Config, error) {
	return &incomes.Config{}, nil
}
func (stubIncomes) UpdateConfig(context.Context, int, incomes.UpdateRequest) (*incomes.Config, error) {
	return &incomes.Config{}, nil
}

type stubCategories struct{}

func (stubCategories) ListCategories(context.Context) ([]category.Category, error) { return nil, nil }
func (stubCategories) GetCategory(context.Context, int64) (*category.Category, error) {
	return &category.Category{}, nil
}
func (stubCategories) CreateCategory(context.Context, category.CreateRequest) (*category.Category, error) {
	return &category.Category{}, nil
}
func (stubCategories) UpdateCategory(context.Context, int64, category.UpdateRequest) (*category.Category, error) {
	return &category.Category{}, nil
}
func (stubCategories) DeleteCategory(context.Context, int64) error { return nil }

// --- harness ----------------------------------------------------------------

const (
	tokenActivated    = "token-activated"
	tokenNotActivated = "token-not-activated"
	tokenUnknown      = "token-unknown"
)

// newTestRouter wires a real Application with stub services and returns the real
// router, so the route tree and middleware chain under test are the production
// ones.
func newTestRouter(t *testing.T) http.Handler {
	t.Helper()

	sm := scs.New()
	tokens := stubTokens{byToken: map[string]*data.User{
		tokenActivated:    {ID: 1, Email: "active@example.com", Activated: true},
		tokenNotActivated: {ID: 2, Email: "pending@example.com", Activated: false},
	}}

	app := &Application{Sessions: sm}
	app.Tokens.Services = tokens
	app.Security.Handlers = security.NewHandler(sm)
	app.Security.Auth = security.NewAuthHandler(tokens)
	app.Users.Handlers = users.NewHandler(stubUsers{}, sm)
	app.Movements.Handlers = movement.NewHandler(stubMovements{})
	app.Balances.Handlers = balance.NewHandler(stubBalance{})
	app.Analytics.Handlers = analytics.NewHandler(stubAnalytics{})
	app.Reports.Handlers = reports.NewHandler(stubReports{})
	app.Incomes.Handlers = incomes.NewHandler(stubIncomes{})
	app.Categories.Handlers = category.NewHandler(stubCategories{})

	return NewRouter(app)
}

func call(t *testing.T, r http.Handler, method, path, token string) int {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Code
}

// tier is what a route demands from its caller.
type tier int

const (
	public tier = iota
	authenticated
	activated
)

// --- the matrix -------------------------------------------------------------

func TestRouter_RouteProtection(t *testing.T) {
	r := newTestRouter(t)

	routes := []struct {
		method string
		path   string
		tier   tier
	}{
		// Public — credentials are the proof, no token possible yet.
		{http.MethodPost, "/api/v1/user", public},
		{http.MethodGet, "/api/v1/user/activate", public},
		{http.MethodPost, "/api/v1/tokens/authentication", public},
		{http.MethodPost, "/api/v1/user/login", public},

		// Authenticated only — must work before the account is activated.
		{http.MethodPost, "/api/v1/tokens/logout", authenticated},
		{http.MethodPost, "/api/v1/change_password", authenticated},
		{http.MethodPost, "/api/v1/user/logout", authenticated},

		// Authenticated + activated — everything touching financial data.
		{http.MethodGet, "/api/v1/movements", activated},
		{http.MethodGet, "/api/v1/movements/expenses", activated},
		{http.MethodGet, "/api/v1/movements/summary", activated},
		{http.MethodGet, "/api/v1/balance", activated},
		{http.MethodGet, "/api/v1/balance/periods", activated},
		{http.MethodGet, "/api/v1/incomes/config", activated},
		{http.MethodGet, "/api/v1/reports/export", activated},
		{http.MethodGet, "/api/v1/analytics/summary", activated},
		{http.MethodGet, "/api/v1/analytics/by-category", activated},
		{http.MethodGet, "/api/v1/categories", activated},
	}

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			// No credentials at all.
			got := call(t, r, rt.method, rt.path, "")
			if rt.tier == public {
				assertAllowed(t, got, "anonymous on a public route")
			} else {
				assertStatus(t, got, http.StatusUnauthorized, "anonymous")
			}

			// A token that resolves to nobody.
			got = call(t, r, rt.method, rt.path, tokenUnknown)
			assertStatus(t, got, http.StatusUnauthorized, "unknown token")

			// A valid token for an account that was never activated.
			got = call(t, r, rt.method, rt.path, tokenNotActivated)
			if rt.tier == activated {
				assertStatus(t, got, http.StatusForbidden, "non-activated user")
			} else {
				assertAllowed(t, got, "non-activated user")
			}

			// A valid token for an activated account.
			got = call(t, r, rt.method, rt.path, tokenActivated)
			assertAllowed(t, got, "activated user")
		})
	}
}

// TestRouter_HealthIsPublic pins that the liveness probe stays outside /api/v1
// and needs no credentials — monitoring depends on it.
func TestRouter_HealthIsPublic(t *testing.T) {
	// healthCheck pings the database, which is nil here, so it answers 503
	// rather than 200. What matters is that no guard rejected it first.
	if got := call(t, newTestRouter(t), http.MethodGet, "/health", ""); got == http.StatusUnauthorized || got == http.StatusForbidden {
		t.Errorf("/health status = %d, want it reachable without credentials", got)
	}
}

func assertStatus(t *testing.T, got, want int, who string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: status = %d, want %d", who, got, want)
	}
}

// assertAllowed checks the guard did not reject. What the handler answers after
// that is out of scope for a wiring test.
func assertAllowed(t *testing.T, got int, who string) {
	t.Helper()
	if got == http.StatusUnauthorized || got == http.StatusForbidden {
		t.Errorf("%s: status = %d, want the guard to allow the request through", who, got)
	}
}
