package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run builds the application, wires the HTTP server and serves with graceful
// shutdown. Returning an error (instead of log.Fatal deep inside) lets the
// deferred cleanup run.
func run() error {
	// Load .env if present (ignored in production where env vars are set directly).
	_ = godotenv.Load()

	app, err := NewApp()
	if err != nil {
		return err
	}
	defer app.Database.DB.Close()

	srv := &http.Server{
		Addr:         ":" + getEnv("PORT", "8080"),
		Handler:      NewRouter(app),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 10 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}

	// On SIGINT/SIGTERM, stop accepting new connections and give in-flight
	// requests up to 10s to finish before exiting.
	shutdownErr := make(chan error, 1)
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		log.Println("shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		shutdownErr <- srv.Shutdown(ctx)
	}()

	log.Printf("server listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	if err := <-shutdownErr; err != nil {
		return err
	}
	log.Println("server stopped")
	return nil
}
