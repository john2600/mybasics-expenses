package main

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/jscodelab/mybasics-expenses/pkg/response"
)

// NewRouter builds the HTTP handler from the wired application: global
// middlewares, the public health check, and the /api/v1 tree (public user routes
// + the session-protected group). Serving is the caller's responsibility (main).
func NewRouter(app *Application) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(app.Sessions.LoadAndSave)
	r.Use(corsMiddleware)
	// authenticate resolves identity (anonymous or the user behind a bearer token)
	// for every request; ProtectEndpoint below depends on it having run.
	r.Use(app.authenticate)
	r.Get("/health", app.healthCheck)

	r.Route("/api/v1", func(r chi.Router) {
		// Public.
		app.Users.Handlers.RegisterRoutes(r)
		// New token-based authentication (legacy session login stays in users).
		app.Security.Auth.RegisterRoutes(r)

		// Protected group. Both guards feed the same userIDKey, so handlers
		// (RequireUserID) don't care which one ran:
		//   - RestrictEndpoint (legacy) sources the id from the session cookie.
		//   - ProtectEndpoint  (new)    sources it from the bearer token via the
		//     global authenticate middleware.
		r.Group(func(r chi.Router) {

			// Legacy session guard — kept for reference while migrating.
			// r.Use(app.Security.Handlers.RestrictEndpoint)
			r.Use(app.Security.Handlers.ProtectEndpoint)
			app.Users.Handlers.RegisterProtectedRoutes(r)
			app.Movements.Handlers.RegisterRoutes(r)
			app.Balances.Handlers.RegisterRoutes(r)
			app.Analytics.Handlers.RegisterRoutes(r)
			app.Reports.Handlers.RegisterRoutes(r)
			app.Incomes.Handlers.RegisterRoutes(r)
			app.Categories.Handlers.RegisterRoutes(r)

		})
	})

	return r
}

// healthCheck reports liveness and pings the database.
func (app *Application) healthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := app.Database.DB.PingContext(ctx); err != nil {
		response.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "error": err.Error()})
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// corsMiddleware adds CORS headers so web/mobile clients can consume the API.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
