package balance

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jscodelab/mybasics-expenses/internal/security"
	"github.com/jscodelab/mybasics-expenses/pkg/response"
)

// Handler handles HTTP requests for balance.
type Handler struct {
	svc Service
}

// NewHandler creates a new balance Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers balance routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/balance", h.get)
	r.Get("/balance/periods", h.periods)
}

// get returns the balance summary for the given date range.
// GET /api/v1/balance?date_from=YYYY-MM-DD&date_to=YYYY-MM-DD
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	dateFrom := r.URL.Query().Get("date_from")
	dateTo := r.URL.Query().Get("date_to")

	userID, ok := security.RequireUserID(w, r)
	if !ok {
		return
	}
	summary, err := h.svc.GetBalance(r.Context(), userID, dateFrom, dateTo)
	if err != nil {
		if isDateError(err) {
			response.BadRequest(w, err)
			return
		}
		response.InternalError(w, err)
		return
	}

	response.Success(w, summary)
}

// periods returns the carry-over balance per billing period.
// GET /api/v1/balance/periods
func (h *Handler) periods(w http.ResponseWriter, r *http.Request) {
	userID, ok := security.RequireUserID(w, r)
	if !ok {
		return
	}
	periods, err := h.svc.GetPeriods(r.Context(), userID)
	if err != nil {
		response.InternalError(w, err)
		return
	}
	response.Success(w, periods)
}

func isDateError(err error) bool {
	return strings.Contains(err.Error(), "YYYY-MM-DD")
}
