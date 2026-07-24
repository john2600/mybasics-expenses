package reports

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/go-pdf/fpdf"
)

// Service defines the business logic for report generation.
type Service interface {
	Export(ctx context.Context, f ExportFilter) (*ExportReport, error)
	RenderCSV(report *ExportReport) ([]byte, error)
	RenderPDF(report *ExportReport) ([]byte, error)
}

type service struct {
	repo Repository
}

// NewService creates a new reports service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Export(ctx context.Context, f ExportFilter) (*ExportReport, error) {
	from, to := dateRange(f.Months)

	expenses, err := s.repo.QueryExpenses(ctx, from, to)
	if err != nil {
		return nil, err
	}
	summary, err := s.repo.MonthlySummary(ctx, from, to)
	if err != nil {
		return nil, err
	}

	var total float64
	for _, e := range expenses {
		total += e.Amount
	}

	if expenses == nil {
		expenses = []ExportRow{}
	}
	if summary == nil {
		summary = []MonthlySummary{}
	}

	return &ExportReport{
		PeriodFrom:     from.Format(time.DateOnly),
		PeriodTo:       to.Format(time.DateOnly),
		Total:          total,
		MonthlySummary: summary,
		Expenses:       expenses,
	}, nil
}

func (s *service) RenderCSV(report *ExportReport) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	_ = w.Write([]string{"id", "date", "category", "description", "amount"})
	for _, e := range report.Expenses {
		_ = w.Write([]string{
			strconv.FormatInt(e.ID, 10),
			e.Date,
			e.Category,
			e.Description,
			strconv.FormatFloat(e.Amount, 'f', 2, 64),
		})
	}

	_ = w.Write([]string{})
	_ = w.Write([]string{"# Resumen mensual"})
	_ = w.Write([]string{"month", "total"})
	for _, m := range report.MonthlySummary {
		_ = w.Write([]string{m.Label, strconv.FormatFloat(m.Total, 'f', 2, 64)})
	}

	_ = w.Write([]string{})
	_ = w.Write([]string{"# Total", strconv.FormatFloat(report.Total, 'f', 2, 64)})

	w.Flush()
	return buf.Bytes(), w.Error()
}

func (s *service) RenderPDF(report *ExportReport) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 16)
	pdf.Cell(0, 10, "MyBasics-Expenses — Reporte de Gastos")
	pdf.Ln(10)

	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(0, 6, fmt.Sprintf("Período: %s – %s", report.PeriodFrom, report.PeriodTo))
	pdf.Ln(10)

	// Expense table
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(220, 220, 220)
	pdf.CellFormat(28, 7, "Fecha", "1", 0, "C", true, 0, "")
	pdf.CellFormat(42, 7, "Categoría", "1", 0, "C", true, 0, "")
	pdf.CellFormat(88, 7, "Descripción", "1", 0, "C", true, 0, "")
	pdf.CellFormat(28, 7, "Monto", "1", 1, "C", true, 0, "")

	pdf.SetFont("Helvetica", "", 8)
	pdf.SetFillColor(255, 255, 255)
	for _, e := range report.Expenses {
		desc := e.Description
		if len(desc) > 52 {
			desc = desc[:49] + "..."
		}
		pdf.CellFormat(28, 6, e.Date, "1", 0, "L", false, 0, "")
		pdf.CellFormat(42, 6, e.Category, "1", 0, "L", false, 0, "")
		pdf.CellFormat(88, 6, desc, "1", 0, "L", false, 0, "")
		pdf.CellFormat(28, 6, fmt.Sprintf("$%.2f", e.Amount), "1", 1, "R", false, 0, "")
	}

	pdf.Ln(8)

	// Monthly summary table
	pdf.SetFont("Helvetica", "B", 10)
	pdf.Cell(0, 7, "Resumen Mensual")
	pdf.Ln(8)

	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(220, 220, 220)
	pdf.CellFormat(100, 7, "Mes", "1", 0, "C", true, 0, "")
	pdf.CellFormat(40, 7, "Total", "1", 1, "C", true, 0, "")

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetFillColor(255, 255, 255)
	for _, m := range report.MonthlySummary {
		pdf.CellFormat(100, 6, m.Label, "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, 6, fmt.Sprintf("$%.2f", m.Total), "1", 1, "R", false, 0, "")
	}

	pdf.Ln(4)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(100, 7, "TOTAL GENERAL", "1", 0, "R", false, 0, "")
	pdf.CellFormat(40, 7, fmt.Sprintf("$%.2f", report.Total), "1", 1, "R", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("rendering PDF: %w", err)
	}
	return buf.Bytes(), nil
}

// dateRange returns the inclusive date window for the last N months.
func dateRange(months int) (from, to time.Time) {
	now := time.Now().UTC()
	to = time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	from = time.Date(now.Year(), now.Month()-time.Month(months-1), 1, 0, 0, 0, 0, time.UTC)
	return
}
