package reports

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jscodelab/mybasics-expenses/pkg/response"
)

// Handler handles HTTP requests for report exports.
type Handler struct {
	svc Service
}

// NewHandler creates a new reports Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers report routes under the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/reports/export", h.Export)
}

// Export handles GET /api/v1/reports/export
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	months := 3
	if raw := r.URL.Query().Get("months"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 12 {
			response.BadRequest(w, errors.New("months must be an integer between 1 and 12"))
			return
		}
		months = n
	}

	switch format {
	case "json", "csv", "pdf":
	default:
		response.BadRequest(w, errors.New("format must be json, csv or pdf"))
		return
	}

	report, err := h.svc.Export(r.Context(), ExportFilter{Months: months, Format: format})
	if err != nil {
		response.InternalError(w, err)
		return
	}

	switch format {
	case "json":
		response.Success(w, report)

	case "csv":
		data, err := h.svc.RenderCSV(report)
		if err != nil {
			response.InternalError(w, err)
			return
		}
		filename := fmt.Sprintf("expenses_%s_%s.csv", report.PeriodFrom[:7], report.PeriodTo[:7])
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)

	case "pdf":
		data, err := h.svc.RenderPDF(report)
		if err != nil {
			response.InternalError(w, err)
			return
		}
		filename := fmt.Sprintf("expenses_%s_%s.pdf", report.PeriodFrom[:7], report.PeriodTo[:7])
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}
