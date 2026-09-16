package reports

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/jscodelab/mybasics-expenses/internal/data"
	"github.com/jscodelab/mybasics-expenses/internal/security"
)

type mockService struct {
	report    *ExportReport
	csv, pdf  []byte
	err       error
	renderErr error

	gotFilter ExportFilter
}

func (m *mockService) Export(_ context.Context, f ExportFilter) (*ExportReport, error) {
	m.gotFilter = f
	if m.err != nil {
		return nil, m.err
	}
	return m.report, nil
}
func (m *mockService) RenderCSV(*ExportReport) ([]byte, error) {
	return m.csv, m.renderErr
}
func (m *mockService) RenderPDF(*ExportReport) ([]byte, error) {
	return m.pdf, m.renderErr
}

const testUserID = 42

func okReport() *ExportReport {
	return &ExportReport{PeriodFrom: "2026-05-01", PeriodTo: "2026-07-31", Total: 100}
}

func serve(t *testing.T, svc Service, guarded bool, target string) *httptest.ResponseRecorder {
	t.Helper()

	r := chi.NewRouter()
	h := NewHandler(svc)
	if guarded {
		r.Group(func(r chi.Router) {
			r.Use(security.NewHandler(nil).RequireAuthentication)
			h.RegisterRoutes(r)
		})
	} else {
		h.RegisterRoutes(r)
	}

	req := httptest.NewRequest(http.MethodGet, target, nil)
	req = security.ContextSetUser(req, &data.User{ID: testUserID, Activated: true})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestExport_DefaultsToJSONAndThreeMonths(t *testing.T) {
	svc := &mockService{report: okReport()}

	rec := serve(t, svc, true, "/reports/export")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.gotFilter.Format != "json" {
		t.Errorf("format = %q, want the json default", svc.gotFilter.Format)
	}
	if svc.gotFilter.Months != 3 {
		t.Errorf("months = %d, want the default 3", svc.gotFilter.Months)
	}
	if svc.gotFilter.UserID != testUserID {
		t.Errorf("userID = %d, want %d", svc.gotFilter.UserID, testUserID)
	}
}

func TestExport_CSVIsADownloadWithTheRightHeaders(t *testing.T) {
	svc := &mockService{report: okReport(), csv: []byte("id,date\n1,2026-07-15\n")}

	rec := serve(t, svc, true, "/reports/export?format=csv")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("Content-Type = %q, want text/csv", ct)
	}
	// The filename is built from the report period, so it is worth pinning.
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, `expenses_2026-05_2026-07.csv`) {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if !strings.HasPrefix(rec.Body.String(), "id,date") {
		t.Errorf("body = %q, want the rendered CSV", rec.Body.String())
	}
}

func TestExport_PDFIsADownloadWithTheRightHeaders(t *testing.T) {
	svc := &mockService{report: okReport(), pdf: []byte("%PDF-1.4")}

	rec := serve(t, svc, true, "/reports/export?format=pdf")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q, want application/pdf", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, `expenses_2026-05_2026-07.pdf`) {
		t.Errorf("Content-Disposition = %q", cd)
	}
}

func TestExport_RejectsAnUnknownFormat(t *testing.T) {
	rec := serve(t, &mockService{report: okReport()}, true, "/reports/export?format=xlsx")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestExport_RejectsAnOutOfRangeMonths(t *testing.T) {
	tests := []string{"0", "13", "-1", "abc"}
	for _, months := range tests {
		t.Run("months="+months, func(t *testing.T) {
			rec := serve(t, &mockService{report: okReport()}, true, "/reports/export?months="+months)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestExport_AcceptsTheBoundaryMonths(t *testing.T) {
	for _, months := range []string{"1", "12"} {
		t.Run("months="+months, func(t *testing.T) {
			rec := serve(t, &mockService{report: okReport()}, true, "/reports/export?months="+months)
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want 200 for a valid boundary", rec.Code)
			}
		})
	}
}

func TestExport_ServiceErrorIs500(t *testing.T) {
	rec := serve(t, &mockService{err: errStub}, true, "/reports/export")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestExport_RenderErrorIs500(t *testing.T) {
	for _, format := range []string{"csv", "pdf"} {
		t.Run(format, func(t *testing.T) {
			svc := &mockService{report: okReport(), renderErr: errStub}
			rec := serve(t, svc, true, "/reports/export?format="+format)
			if rec.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want 500", rec.Code)
			}
		})
	}
}

func TestExport_WithoutAGuardIs401(t *testing.T) {
	rec := serve(t, &mockService{report: okReport()}, false, "/reports/export")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
