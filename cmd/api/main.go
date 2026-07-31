package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/alexedwards/scs/mysqlstore" // New import
	"github.com/alexedwards/scs/v2"
	"github.com/joho/godotenv"

	"github.com/jscodelab/mybasics-expenses/internal/analytics"
	"github.com/jscodelab/mybasics-expenses/internal/balance"
	"github.com/jscodelab/mybasics-expenses/internal/category"
	"github.com/jscodelab/mybasics-expenses/internal/incomes"
	"github.com/jscodelab/mybasics-expenses/internal/movement"
	"github.com/jscodelab/mybasics-expenses/internal/platform/database"
	"github.com/jscodelab/mybasics-expenses/internal/reports"
	"github.com/jscodelab/mybasics-expenses/internal/security"
	"github.com/jscodelab/mybasics-expenses/internal/users"
	"github.com/jscodelab/mybasics-expenses/pkg/response"
)

func main() {
	// Load .env if present (ignored in production where env vars are set directly).
	_ = godotenv.Load()

	db, err := database.NewMySQL(database.Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "3306"),
		User:     getEnv("DB_USER", "root"),
		Password: getEnv("DB_PASSWORD", ""),
		Name:     getEnv("DB_NAME", "mybasics_expenses"),
	})
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer db.Close()

	sessionManager := scs.New()
	sessionManager.Store = mysqlstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour

	// Wire up repositories → services → handlers.
	categoryRepo := category.NewMySQLRepository(db)
	categoryService := category.NewService(categoryRepo)
	categoryHandler := category.NewHandler(categoryService)

	movementRepo := movement.NewMySQLRepository(db)
	movementService := movement.NewService(movementRepo)
	movementHandler := movement.NewHandler(movementService)

	incomesRepo := incomes.NewMySQLRepository(db)
	incomesService := incomes.NewService(incomesRepo)
	incomesHandler := incomes.NewHandler(incomesService)

	balanceRepo := balance.NewMySQLRepository(db)
	balanceService := balance.NewService(balanceRepo, incomesRepo)
	balanceHandler := balance.NewHandler(balanceService)

	reportsRepo := reports.NewMySQLRepository(db)
	reportsService := reports.NewService(reportsRepo)
	reportsHandler := reports.NewHandler(reportsService)

	analyticsRepo := analytics.NewMySQLRepository(db)
	analyticsService := analytics.NewService(analyticsRepo)
	analyticsHandler := analytics.NewHandler(analyticsService)

	usersRepo := users.NewMySQLRepository(db)
	usersService := users.NewService(usersRepo)
	usersHandler := users.NewHandler(usersService, sessionManager)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(sessionManager.LoadAndSave)
	r.Use(corsMiddleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			response.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "error": err.Error()})
			return
		}
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	protect := security.NewHandler(sessionManager)

	r.Route("/api/v1", func(r chi.Router) {
		usersHandler.RegisterRoutes(r)

		// protegidas
		r.Group(func(r chi.Router) {
			r.Use(protect.RestrictEndpoint) // ← el middleware aquí
			usersHandler.RegisterProtectedRoutes(r)
			movementHandler.RegisterRoutes(r)
			balanceHandler.RegisterRoutes(r)
			analyticsHandler.RegisterRoutes(r)
			reportsHandler.RegisterRoutes(r)
			incomesHandler.RegisterRoutes(r)
			categoryHandler.RegisterRoutes(r)
		})
	})

	addr := fmt.Sprintf(":%s", getEnv("PORT", "8080"))
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 10 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("server listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
