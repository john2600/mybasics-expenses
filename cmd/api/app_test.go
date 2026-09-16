package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetEnv(t *testing.T) {
	t.Setenv("MYBASICS_TEST_KEY", "from-env")
	if got := getEnv("MYBASICS_TEST_KEY", "fallback"); got != "from-env" {
		t.Errorf("got = %q, want the environment value", got)
	}

	if got := getEnv("MYBASICS_TEST_MISSING", "fallback"); got != "fallback" {
		t.Errorf("got = %q, want the fallback", got)
	}

	// An empty variable counts as unset, so a blank env var cannot silently
	// blank out a default like the database name.
	t.Setenv("MYBASICS_TEST_EMPTY", "")
	if got := getEnv("MYBASICS_TEST_EMPTY", "fallback"); got != "fallback" {
		t.Errorf("got = %q, want the fallback for an empty value", got)
	}
}

// mockedDB returns a *sql.DB backed by sqlmock, enough for the init* wiring
// which stores the handle without dialling.
func mockedDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("opening sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

func TestInitWiring_BuildsEveryModuleInDependencyOrder(t *testing.T) {
	db, _ := mockedDB(t)

	app := &Application{}
	app.Database.DB = db
	app.initSessions()
	app.initIncomes()
	app.initBalances()
	app.initCategories()
	app.initMovements()
	app.initReports()
	app.initAnalytics()
	app.initUsers()
	app.initTokens()
	app.initUserService()
	app.initSecurity()

	// A nil handler here would panic only later, when NewRouter registers its
	// routes — this catches a wiring gap at the point it is introduced.
	checks := []struct {
		name string
		got  any
	}{
		{"Sessions", app.Sessions},
		{"Incomes.Handlers", app.Incomes.Handlers},
		{"Balances.Handlers", app.Balances.Handlers},
		{"Categories.Handlers", app.Categories.Handlers},
		{"Movements.Handlers", app.Movements.Handlers},
		{"Reports.Handlers", app.Reports.Handlers},
		{"Analytics.Handlers", app.Analytics.Handlers},
		{"Users.Repos", app.Users.Repos},
		{"Users.Handlers", app.Users.Handlers},
		{"Tokens.Services", app.Tokens.Services},
		{"Security.Handlers", app.Security.Handlers},
		{"Security.Auth", app.Security.Auth},
	}
	for _, c := range checks {
		if c.got == nil {
			t.Errorf("%s is nil after wiring", c.name)
		}
	}
}

func TestInitBalances_IsWiredWithTheIncomesRepository(t *testing.T) {
	// balance is the one module built from two repositories: the available
	// balance needs the fixed income and its cut day. initIncomes must run
	// first, and this pins that ordering requirement.
	db, _ := mockedDB(t)

	app := &Application{}
	app.Database.DB = db
	app.initIncomes()
	app.initBalances()

	if app.Incomes.Repos == nil {
		t.Fatal("the incomes repository balance depends on was not built")
	}
	if app.Balances.Services == nil {
		t.Error("the balance service was not built")
	}
}

func TestInitMailer_ReadsTheEnvironment(t *testing.T) {
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_USERNAME", "user")
	t.Setenv("SMTP_PASSWORD", "pass")
	t.Setenv("SMTP_FROM", "MyBasics <no-reply@example.com>")

	app := &Application{}
	if err := app.initMailer(); err != nil {
		t.Fatalf("initMailer returned error: %v", err)
	}
	if app.Mailer.Sender == nil {
		t.Error("no mailer was built")
	}
}

func TestInitMailer_RejectsANonNumericPort(t *testing.T) {
	t.Setenv("SMTP_PORT", "not-a-number")

	app := &Application{}
	err := app.initMailer()
	if err == nil {
		t.Fatal("expected an error for a malformed SMTP_PORT")
	}
	if !strings.Contains(err.Error(), "SMTP_PORT") {
		t.Errorf("err = %v, want it to name the offending variable", err)
	}
}

func TestInitDataBase_FailsFastWhenTheServerIsUnreachable(t *testing.T) {
	// Port 1 is reserved and never has a listener, so the ping fails at once.
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", "1")
	t.Setenv("DB_USER", "root")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "mybasics_expenses")

	app := &Application{}
	if err := app.initDataBase(); err == nil {
		t.Fatal("expected an error when the database is unreachable")
	}
}

func TestNewApp_StopsAtTheFirstFailure(t *testing.T) {
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", "1")

	app, err := NewApp()
	if err == nil {
		t.Fatal("expected NewApp to fail when the database is unreachable")
	}
	// A half-built Application must never be handed back: the caller would
	// dereference nil handlers.
	if app != nil {
		t.Error("a partially wired Application was returned alongside the error")
	}
}

func TestHealthCheck_OKWhenTheDatabaseAnswers(t *testing.T) {
	db, mock := mockedDB(t)
	mock.ExpectPing()

	app := &Application{}
	app.Database.DB = db

	rec := httptest.NewRecorder()
	app.healthCheck(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestHealthCheck_DegradedWhenThePingFails(t *testing.T) {
	db, mock := mockedDB(t)
	mock.ExpectPing().WillReturnError(errPingFailed)

	app := &Application{}
	app.Database.DB = db

	rec := httptest.NewRecorder()
	app.healthCheck(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	// 503 rather than 500: the process is alive but cannot serve, which is what
	// a load balancer needs in order to take it out of rotation.
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"degraded"`) {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestCorsMiddleware_AnswersPreflightWithoutReachingTheHandler(t *testing.T) {
	called := false
	h := corsMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/api/v1/movements", nil))

	if called {
		t.Error("the preflight request reached the handler")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	// Authorization must be allowed or the browser never sends the bearer token.
	if h := rec.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(h, "Authorization") {
		t.Errorf("Allow-Headers = %q, want it to include Authorization", h)
	}
}

func TestCorsMiddleware_PassesOtherMethodsThrough(t *testing.T) {
	called := false
	h := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/movements", nil))

	if !called {
		t.Error("a normal request did not reach the handler")
	}
	if rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("no CORS origin header on a normal response")
	}
}

func TestRun_ReturnsTheWiringErrorInsteadOfExiting(t *testing.T) {
	// run returns an error rather than calling log.Fatal deep inside, so the
	// deferred cleanup gets a chance to execute. With an unreachable database
	// it must fail before binding a port — otherwise a broken deploy would
	// come up listening and answer every request with a 500.
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", "1")

	if err := run(); err == nil {
		t.Fatal("expected run to fail when the database is unreachable")
	}
}
