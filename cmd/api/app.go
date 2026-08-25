package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/alexedwards/scs/mysqlstore"
	"github.com/alexedwards/scs/v2"

	"github.com/jscodelab/mybasics-expenses/internal/analytics"
	"github.com/jscodelab/mybasics-expenses/internal/balance"
	"github.com/jscodelab/mybasics-expenses/internal/category"
	"github.com/jscodelab/mybasics-expenses/internal/incomes"
	"github.com/jscodelab/mybasics-expenses/internal/mailer"
	"github.com/jscodelab/mybasics-expenses/internal/movement"
	"github.com/jscodelab/mybasics-expenses/internal/platform/database"
	"github.com/jscodelab/mybasics-expenses/internal/reports"
	"github.com/jscodelab/mybasics-expenses/internal/security"
	"github.com/jscodelab/mybasics-expenses/internal/users"
)

// Application holds the shared infrastructure (DB, session manager) and, grouped
// per module, each module's repository / service / handler. Wire it by calling
// initializeApp, which runs every init* in the right order.
type Application struct {
	Configs struct {
	}

	Sessions *scs.SessionManager

	Database struct {
		DB *sql.DB
	}

	// Mailer sends outbound email (e.g. via Mailtrap sandbox in dev). Behind the
	// mailer.Mailer interface, so consumers depend on the abstraction.
	Mailer struct {
		Sender mailer.Mailer
	}

	Incomes struct {
		Repos    incomes.Repository
		Services incomes.Service
		Handlers *incomes.Handler
	}

	Balances struct {
		Repos    balance.Repository
		Services balance.Service
		Handlers *balance.Handler
	}

	Categories struct {
		Repos    category.Repository
		Services category.Service
		Handlers *category.Handler
	}

	Movements struct {
		Repos    movement.Repository
		Services movement.Service
		Handlers *movement.Handler
	}

	Reports struct {
		Repos    reports.Repository
		Services reports.Service
		Handlers *reports.Handler
	}

	Analytics struct {
		Repos    analytics.Repository
		Services analytics.Service
		Handlers *analytics.Handler
	}

	Users struct {
		Mailer   mailer.Mailer
		Repos    users.Repository
		Services users.Service
		Handlers *users.Handler
	}

	// Security has no repo/service — just the middleware handler built from the
	// session manager.
	Security struct {
		Handlers *security.Security
	}

	Tokens struct {
		Repos    security.TokenRepository
		Services security.TokenService
	}
}

func (app *Application) initDataBase() error {
	db, err := database.NewMySQL(database.Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "3306"),
		User:     getEnv("DB_USER", "root"),
		Password: getEnv("DB_PASSWORD", ""),
		Name:     getEnv("DB_NAME", "mybasics_expenses"),
	})
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}

	// Keep the pool open for the app's lifetime — do NOT db.Close() here (the
	// caller closes it at shutdown).
	app.Database.DB = db
	return nil
}

func (app *Application) initMailer() error {
	port, err := strconv.Atoi(getEnv("SMTP_PORT", "587"))
	if err != nil {
		return fmt.Errorf("invalid SMTP_PORT: %w", err)
	}
	// New only builds the client; it does not dial. Missing credentials fail
	// later on Send, not at startup, so the app boots even without SMTP set.
	m, err := mailer.New(mailer.Config{
		Host:     getEnv("SMTP_HOST", "sandbox.smtp.mailtrap.io"),
		Port:     port,
		Username: getEnv("SMTP_USERNAME", ""),
		Password: getEnv("SMTP_PASSWORD", ""),
		From:     getEnv("SMTP_FROM", "MyBasics-Expenses <no-reply@example.com>"),
	})
	if err != nil {
		return fmt.Errorf("initializing mailer: %w", err)
	}
	app.Mailer.Sender = m
	return nil
}

func (app *Application) initSessions() {
	sm := scs.New()
	sm.Store = mysqlstore.New(app.Database.DB)
	sm.Lifetime = 12 * time.Hour
	app.Sessions = sm
}

func (app *Application) initIncomes() {
	app.Incomes.Repos = incomes.NewMySQLRepository(app.Database.DB)
	app.Incomes.Services = incomes.NewService(app.Incomes.Repos)
	app.Incomes.Handlers = incomes.NewHandler(app.Incomes.Services)
}

func (app *Application) initBalances() {
	app.Balances.Repos = balance.NewMySQLRepository(app.Database.DB)
	// balance also needs the incomes repository (fixed income + cut day) — so
	// initIncomes must run before this.
	app.Balances.Services = balance.NewService(app.Balances.Repos, app.Incomes.Repos)
	app.Balances.Handlers = balance.NewHandler(app.Balances.Services)
}

func (app *Application) initCategories() {
	app.Categories.Repos = category.NewMySQLRepository(app.Database.DB)
	app.Categories.Services = category.NewService(app.Categories.Repos)
	app.Categories.Handlers = category.NewHandler(app.Categories.Services)
}

func (app *Application) initMovements() {
	app.Movements.Repos = movement.NewMySQLRepository(app.Database.DB)
	app.Movements.Services = movement.NewService(app.Movements.Repos)
	app.Movements.Handlers = movement.NewHandler(app.Movements.Services)
}

func (app *Application) initReports() {
	app.Reports.Repos = reports.NewMySQLRepository(app.Database.DB)
	app.Reports.Services = reports.NewService(app.Reports.Repos)
	app.Reports.Handlers = reports.NewHandler(app.Reports.Services)
}

func (app *Application) initAnalytics() {
	app.Analytics.Repos = analytics.NewMySQLRepository(app.Database.DB)
	app.Analytics.Services = analytics.NewService(app.Analytics.Repos)
	app.Analytics.Handlers = analytics.NewHandler(app.Analytics.Services)
}

func (app *Application) initUsers() {
	app.Users.Mailer = app.Mailer.Sender
	app.Users.Repos = users.NewMySQLRepository(app.Database.DB)
}

func (app *Application) initTokens() {
	// The token service looks up users by email via a UserFinder adapter, so it
	// needs the users repository (initUsers) to run first.
	app.Tokens.Repos = security.NewMySQLTokenRepository(app.Database.DB)
	app.Tokens.Services = security.NewTokenService(app.Tokens.Repos, users.NewUserFinder(app.Users.Repos))
}

func (app *Application) initUserService() {
	// The user service depends on the token service (activation flow), so this
	// runs after initTokens.
	app.Users.Services = users.NewService(app.Users.Repos, app.Mailer.Sender, app.Tokens.Services)
	app.Users.Handlers = users.NewHandler(app.Users.Services, app.Sessions)
}

func (app *Application) initSecurity() {
	app.Security.Handlers = security.NewHandler(app.Sessions)
}

// NewApp builds and wires the whole application in dependency order: DB and
// sessions first, then incomes (balance depends on it), then the remaining
// modules. It returns the wired *Application ready to serve.
func NewApp() (*Application, error) {
	app := &Application{}

	if err := app.initDataBase(); err != nil {
		return nil, err
	}
	app.initSessions()

	if err := app.initMailer(); err != nil {
		return nil, err
	}

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

	return app, nil
}

// getEnv returns the environment variable value or a fallback when unset.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
