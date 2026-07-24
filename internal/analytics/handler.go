package analytics

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jscodelab/mybasics-expenses/internal/security"
	"github.com/jscodelab/mybasics-expenses/pkg/response"
)

// Handler handles HTTP requests for analytics endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a new analytics Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all analytics routes under the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/analytics/summary", h.Summary)
	r.Get("/analytics/by-category", h.ByCategory)
	r.Get("/analytics/trend", h.Trend)
	r.Get("/analytics/top-expenses", h.TopExpenses)
	r.Get("/analytics/income-vs-expense", h.IncomeVsExpense)
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	f, ok := parseFilter(w, r)
	if !ok {
		return
	}
	data, err := h.svc.Summary(r.Context(), f)
	if err != nil {
		response.InternalError(w, err)
		return
	}
	response.Success(w, data)
}

func (h *Handler) ByCategory(w http.ResponseWriter, r *http.Request) {
	f, ok := parseFilter(w, r)
	if !ok {
		return
	}
	data, err := h.svc.ByCategory(r.Context(), f)
	if err != nil {
		response.InternalError(w, err)
		return
	}
	response.Success(w, data)
}

func (h *Handler) Trend(w http.ResponseWriter, r *http.Request) {
	f, ok := parseFilter(w, r)
	if !ok {
		return
	}
	data, err := h.svc.Trend(r.Context(), f)
	if err != nil {
		response.InternalError(w, err)
		return
	}
	response.Success(w, data)
}

func (h *Handler) TopExpenses(w http.ResponseWriter, r *http.Request) {
	f, ok := parseFilter(w, r)
	if !ok {
		return
	}

	limit := 10
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 50 {
			response.BadRequest(w, errors.New("limit must be an integer between 1 and 50"))
			return
		}
		limit = n
	}

	data, err := h.svc.TopExpenses(r.Context(), f, limit)
	if err != nil {
		response.InternalError(w, err)
		return
	}
	response.Success(w, data)
}

func (h *Handler) IncomeVsExpense(w http.ResponseWriter, r *http.Request) {
	f, ok := parseFilter(w, r)
	if !ok {
		return
	}
	data, err := h.svc.IncomeVsExpense(r.Context(), f)
	if err != nil {
		response.InternalError(w, err)
		return
	}
	response.Success(w, data)
}

// parseFilter builds the analytics Filter from the request. The user id comes
// from the request context (populated by the security.RestrictEndpoint
// middleware), never from client-controlled input. The only query parameter is:
//   - ?months (optional): size of the trailing window, defaults to 3.
func parseFilter(w http.ResponseWriter, r *http.Request) (Filter, bool) {
	months := 3
	if raw := r.URL.Query().Get("months"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 12 {
			response.BadRequest(w, errors.New("months must be an integer between 1 and 12"))
			return Filter{}, false
		}
		months = n
	}
	userID, _ := security.UserID(r.Context())
	return Filter{Months: months, UserID: userID}, true
}
