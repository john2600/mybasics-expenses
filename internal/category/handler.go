package category

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jscodelab/mybasics-expenses/pkg/response"
)

// Handler handles HTTP requests for categories.
type Handler struct {
	svc Service
}

// NewHandler creates a new category Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers category routes under the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/categories", h.create)
	r.Get("/categories", h.list)
	r.Get("/categories/{id}", h.get)
	r.Put("/categories/{id}", h.update)
	r.Delete("/categories/{id}", h.delete)
}

// list returns all categories.
// GET /api/v1/categories
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	categories, err := h.svc.ListCategories(r.Context())
	if err != nil {
		response.InternalError(w, err)
		return
	}

	// Return empty array instead of null when there are no categories.
	if categories == nil {
		categories = []Category{}
	}

	response.Success(w, categories)
}

// create stores a new category from a JSON payload.
// POST /api/v1/categories
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, err)
		return
	}

	cat, err := h.svc.CreateCategory(r.Context(), req)
	if err != nil {
		response.BadRequest(w, err)
		return
	}

	response.Created(w, cat)
}

// get returns a single category by ID.
// GET /api/v1/categories/{id}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.BadRequest(w, err)
		return
	}

	cat, err := h.svc.GetCategory(r.Context(), id)
	if err != nil {
		response.InternalError(w, err)
		return
	}
	if cat == nil {
		response.NotFound(w, "category not found")
		return
	}

	response.Success(w, cat)
}

// update edits an existing category.
// PUT /api/v1/categories/{id}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.BadRequest(w, err)
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, err)
		return
	}

	cat, err := h.svc.UpdateCategory(r.Context(), id, req)
	if err != nil {
		response.InternalError(w, err)
		return
	}
	if cat == nil {
		response.NotFound(w, "category not found")
		return
	}

	response.Success(w, cat)
}

// delete removes a category by ID.
// DELETE /api/v1/categories/{id}
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.BadRequest(w, err)
		return
	}

	if err := h.svc.DeleteCategory(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "category not found")
			return
		}
		response.InternalError(w, err)
		return
	}

	response.NoContent(w)
}
