package security

import (
	"errors"
	"log"
	"net/http"

	"context"

	"github.com/alexedwards/scs/v2"
	"github.com/jscodelab/mybasics-expenses/pkg/response"
)

type Security struct {
	SessionManager *scs.SessionManager
}

func NewHandler(session *scs.SessionManager) *Security {
	return &Security{SessionManager: session}
}

func (app *Security) RestrictEndpoint(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticated := app.isAuthenticated(r)
		log.Printf("security: %s %s -> authenticated=%t", r.Method, r.URL.Path, authenticated)

		if !authenticated {
			log.Printf("security: rejecting %s %s (not authenticated)", r.Method, r.URL.Path)
			response.Unauthorized(w, errors.New("not authenticated"))
			return
		}

		userID := app.SessionManager.GetInt(r.Context(), "authenticatedUserID")
		log.Printf("security: allowing %s %s for userID=%d", r.Method, r.URL.Path, userID)

		// Setting a value
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		req := r.WithContext(ctx)

		w.Header().Add("Cache-Control", "no-store")
		next.ServeHTTP(w, req)
	})
}

func (s *Security) isAuthenticated(r *http.Request) bool {
	return s.SessionManager.Exists(r.Context(), "authenticatedUserID")
}

type contextKey string

const userIDKey contextKey = "userID"

func UserID(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userIDKey).(int)
	return id, ok
}
