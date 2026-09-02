package security

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jscodelab/mybasics-expenses/internal/data"
	"github.com/jscodelab/mybasics-expenses/pkg/response"
)

// AuthHandler owns the authentication HTTP endpoints — the new, token-based
// login. Authentication (verifying identity, issuing tokens) is this package's
// responsibility; account lifecycle (register, activate, change password) stays
// in the users package. The legacy session login/logout in users is untouched.
type AuthHandler struct {
	tokens TokenService
}

// NewAuthHandler builds the authentication handler from the token service.
func NewAuthHandler(tokens TokenService) *AuthHandler {
	return &AuthHandler{tokens: tokens}
}

// RegisterRoutes registers the public authentication routes (no session needed —
// credentials are the proof).
func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	r.Post("/tokens/authentication", h.createAuthenticationToken)
}

// createAuthenticationToken verifies credentials and returns a fresh
// authentication token in the response body. On bad credentials it returns 401
// with a generic message (it never reveals whether the email exists).
// POST /api/v1/tokens/authentication
func (h *AuthHandler) createAuthenticationToken(w http.ResponseWriter, r *http.Request) {
	var req data.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, err)
		return
	}

	// Validate the request shape at the boundary so malformed input is a 400,
	// not a 500. These are format errors (missing email, short password) — not
	// credential errors — so returning the message leaks nothing about accounts.
	if err := req.Validate(); err != nil {
		response.BadRequest(w, err)
		return
	}

	token, err := h.tokens.CreateAuthentication(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			response.Unauthorized(w, errors.New("invalid email or password"))
			return
		}
		response.InternalError(w, err)
		return
	}

	// Encode the token to JSON and send it with a 201 Created status code.
	response.Created(w, map[string]any{"authentication_token": token})
}
