package incomes

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jscodelab/mybasics-expenses/pkg/response"
)

// Handler handles HTTP requests for income config.
type Handler struct {
	svc Service
}

// NewHandler creates a new incomes Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers incomes routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/incomes/config", h.get)
	r.Put("/incomes/config", h.update)
}

// get returns the current income config.
// GET /api/v1/incomes/config
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.svc.GetConfig(r.Context())
	if err != nil {
		response.InternalError(w, err)
		return
	}
	response.Success(w, cfg)
}

// update modifies the income config (partial update).
// PUT /api/v1/incomes/config
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, err)
		return
	}

	cfg, err := h.svc.UpdateConfig(r.Context(), req)
	if err != nil {
		response.BadRequest(w, err)
		return
	}
	response.Success(w, cfg)
}
