package users

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jscodelab/mybasics-expenses/pkg/response"
)

// Handler handles HTTP requests for users.
type Handler struct {
	svc Service
	// sm writes the authenticated user id into the session on login. It is the
	// only place in the user handler that touches session state.
	sm *scs.SessionManager
}

// NewHandler creates a new users Handler.
func NewHandler(svc Service, sm *scs.SessionManager) *Handler {
	return &Handler{svc: svc, sm: sm}
}

// RegisterRoutes registers the public user routes (no session required).
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/user", h.create)
	r.Post("/change_password", h.updatePassword)
	r.Post("/user/login", h.login)
}

// RegisterProtectedRoutes registers user routes that require an active session.
// Mount this inside the authenticated group.
func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.Post("/user/logout", h.logout)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req UserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, err)
		return
	}

	err := h.svc.InsertUser(r.Context(), req)
	if err != nil {
		response.BadRequest(w, err)
		return
	}

	response.Created(w, "user created")
}

func (h *Handler) updatePassword(w http.ResponseWriter, r *http.Request) {
	var passwordRequest ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&passwordRequest); err != nil {
		response.BadRequest(w, err)
		return
	}

	// si el usuario ya
	err := h.svc.ChangePassword(r.Context(), passwordRequest)
	if err != nil {
		response.BadRequest(w, err)
		return
	}

	response.Success(w, "user created")
}

// login authenticates a user and stores their id in the session.
// POST /api/v1/user/login
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, err)
		return
	}

	id, err := h.svc.Authenticate(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			response.Unauthorized(w, errors.New("invalid email or password"))
			return
		}
		response.InternalError(w, err)
		return
	}

	// Renew the session token on login to prevent session fixation, then store
	// the authenticated user id. The rest of the app reads it from the session
	// (via the security middleware), never from the request body.
	if err := h.sm.RenewToken(r.Context()); err != nil {
		response.InternalError(w, err)
		return
	}

	h.sm.Put(r.Context(), "authenticatedUserID", id)

	response.Success(w, "login successful")
}

// logout ends the current session: it destroys the session data in the store
// and instructs the middleware to expire the cookie.
// POST /api/v1/user/logout
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.sm.Destroy(r.Context()); err != nil {
		response.InternalError(w, err)
		return
	}
	response.Success(w, "logout successful")
}
