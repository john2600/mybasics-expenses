package movement

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jscodelab/mybasics-expenses/internal/security"
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
	r.Post("/movements", h.create)
	r.Get("/movements", h.list)
	r.Get("/movements/expenses", h.expenses)
	r.Get("/movements/summary", h.summary)
	r.Get("/movements/{id}", h.get)
	r.Put("/movements/{id}", h.update)
	r.Delete("/movements/{id}", h.delete)
}

// create processes and stores a new movement.
// POST /api/v1/movements
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, err)
		return
	}

	userID, ok := security.RequireUserID(w, r)
	if !ok {
		return
	}

	m, err := h.svc.CreateMovement(r.Context(), userID, req)
	if err != nil {
		response.BadRequest(w, err)
		return
	}

	response.Created(w, m)
}

// list returns all movements grouped by category with optional filters.
// GET /api/v1/movements?date_from=YYYY-MM-DD&date_to=YYYY-MM-DD&category_id=1&type=E
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	f, err := parseFilter(r)
	if err != nil {
		response.BadRequest(w, err)
		return
	}
	userID, ok := security.RequireUserID(w, r)
	if !ok {
		return
	}
	f.UserID = userID

	grouped, err := h.svc.ListMovements(r.Context(), f)
	if err != nil {
		response.InternalError(w, err)
		return
	}

	if grouped == nil {
		grouped = []GroupedByCategory{}
	}

	response.Success(w, grouped)
}

// get returns a single movement by ID.
// GET /api/v1/movements/{id}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		response.BadRequest(w, err)
		return
	}

	userID, ok := security.RequireUserID(w, r)
	if !ok {
		return
	}
	m, err := h.svc.GetMovement(r.Context(), userID, id)
	if err != nil {
		response.InternalError(w, err)
		return
	}
	if m == nil {
		response.NotFound(w, "movement not found")
		return
	}

	response.Success(w, m)
}

// update edits an existing movement.
// PUT /api/v1/movements/{id}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		response.BadRequest(w, err)
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, err)
		return
	}

	userID, ok := security.RequireUserID(w, r)
	if !ok {
		return
	}
	m, err := h.svc.UpdateMovement(r.Context(), userID, id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "movement not found")
			return
		}
		response.BadRequest(w, err)
		return
	}
	if m == nil {
		response.NotFound(w, "movement not found")
		return
	}

	response.Success(w, m)
}

// delete removes a movement by ID.
// DELETE /api/v1/movements/{id}
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		response.BadRequest(w, err)
		return
	}

	userID, ok := security.RequireUserID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteMovement(r.Context(), userID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(w, "movement not found")
			return
		}
		response.InternalError(w, err)
		return
	}

	response.NoContent(w)
}

// expenses returns a flat list of expense movements ordered by date desc.
// GET /api/v1/movements/expenses?limit=10&date_from=YYYY-MM-DD&date_to=YYYY-MM-DD
func (h *Handler) expenses(w http.ResponseWriter, r *http.Request) {
	f, err := parseFilter(r)
	if err != nil {
		response.BadRequest(w, err)
		return
	}
	userID, ok := security.RequireUserID(w, r)
	if !ok {
		return
	}
	f.UserID = userID

	list, err := h.svc.ListExpenses(r.Context(), f)
	if err != nil {
		response.InternalError(w, err)
		return
	}

	response.Success(w, list)
}

// summary returns totals grouped by month.
// GET /api/v1/movements/summary
func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	userID, ok := security.RequireUserID(w, r)
	if !ok {
		return
	}
	summaries, err := h.svc.GetMonthlySummary(r.Context(), userID)
	if err != nil {
		response.InternalError(w, err)
		return
	}

	if summaries == nil {
		summaries = []MonthlySummary{}
	}

	response.Success(w, summaries)
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func parseFilter(r *http.Request) (Filter, error) {
	var f Filter

	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return f, errors.New("invalid limit, must be a positive integer")
		}
		f.Limit = &n
	}

	if v := r.URL.Query().Get("category_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return f, errors.New("invalid category_id")
		}
		f.CategoryID = &id
	}

	if v := r.URL.Query().Get("type"); v != "" {
		if v != "I" && v != "E" {
			return f, errors.New("type must be 'I' or 'E'")
		}
		f.Type = &v
	}

	if v := r.URL.Query().Get("date_from"); v != "" {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			return f, errors.New("invalid date_from, expected YYYY-MM-DD")
		}
		f.DateFrom = &t
	}

	if v := r.URL.Query().Get("date_to"); v != "" {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			return f, errors.New("invalid date_to, expected YYYY-MM-DD")
		}
		f.DateTo = &t
	}

	return f, nil
}
