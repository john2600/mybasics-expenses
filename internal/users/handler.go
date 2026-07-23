package users

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jscodelab/mybasics-expenses/pkg/response"
)

// Handler handles HTTP requests for movements.
type Handler struct {
	svc Service
}

// NewHandler creates a new movement Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers movement routes under the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/user", h.create)
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
