package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jscodelab/mybasics-expenses/internal/data"
	"github.com/jscodelab/mybasics-expenses/internal/security"
	"github.com/jscodelab/mybasics-expenses/pkg/response"
)

// authenticate inspects the Authorization header for a "Bearer <token>"
// authentication token. When absent, the request proceeds as the anonymous user;
// when present and valid, the matching user is put on the request context; when
// present but malformed or unknown, it responds 401.
func (app *Application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Responses may differ based on the Authorization header, so caches must
		// key on it.
		w.Header().Add("Vary", "Authorization")

		authorizationHeader := r.Header.Get("Authorization")

		// No header: treat as anonymous and continue.
		if authorizationHeader == "" {
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		// Expect "Bearer <token>".
		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			app.invalidAuthenticationToken(w)
			return
		}
		token := headerParts[1]
		if token == "" {
			app.invalidAuthenticationToken(w)
			return
		}

		// Resolve the token to its user. GetForToken hashes the plaintext and
		// looks it up by hash, honouring scope and expiry.
		user, err := app.Tokens.Services.GetForToken(r.Context(), security.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, security.ErrTokenNotFound):
				app.invalidAuthenticationToken(w)
			default:
				response.InternalError(w, err)
			}
			return
		}

		r = app.contextSetUser(r, user)
		next.ServeHTTP(w, r)
	})
}

// invalidAuthenticationToken writes a 401 and signals the expected scheme via
// the WWW-Authenticate header, without revealing why the token was rejected.
func (app *Application) invalidAuthenticationToken(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	response.Unauthorized(w, errors.New("invalid or missing authentication token"))
}
