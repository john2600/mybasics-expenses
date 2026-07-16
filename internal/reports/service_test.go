package reports_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jscodelab/mybasics-expenses/internal/reports"
)

type mockRepo struct {
	expenses []reports.ExportRow
	summary  []reports.MonthlySummary
}

func (m *mockRepo) QueryExpenses(_ context.Context, _, _ time.Time) ([]reports.ExportRow, error) {
	return m.expenses, nil
}

func (m *mockRepo) MonthlySummary(_ context.Context, _, _ time.Time) ([]reports.MonthlySummary, error) {
	return m.summary, nil
}

func TestExport_TotalSumsCorrectly(t *testing.T) {
	repo := &mockRepo{
		expenses: []reports.ExportRow{
			{ID: 1, Date: "2026-03-10", Category: "Food", Description: "Lunch", Amount: 100},
			{ID: 2, Date: "2026-04-05", Category: "Transport", Description: "Bus", Amount: 50.50},
		},
		summary: []reports.MonthlySummary{
			{Year: 2026, Month: 3, Label: "Mar 2026", Total: 100},
			{Year: 2026, Month: 4, Label: "Apr 2026", Total: 50.50},
		},
	}
	svc := reports.NewService(repo)

	report, err := svc.Export(context.Background(), reports.ExportFilter{Months: 3, Format: "json"})
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}
	if report.Total != 150.50 {
		t.Errorf("expected total 150.50, got %.2f", report.Total)
	}
	if len(report.Expenses) != 2 {
		t.Errorf("expected 2 expenses, got %d", len(report.Expenses))
	}
}

func TestExport_PeriodFrom_SingleMonth(t *testing.T) {
	svc := reports.NewService(&mockRepo{})

	report, err := svc.Export(context.Background(), reports.ExportFilter{Months: 1, Format: "json"})
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	now := time.Now().UTC()
	expectedFrom := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	if report.PeriodFrom != expectedFrom {
		t.Errorf("expected PeriodFrom %s, got %s", expectedFrom, report.PeriodFrom)
	}
}

func TestExport_EmptyExpensesNotNil(t *testing.T) {
	svc := reports.NewService(&mockRepo{})

	report, err := svc.Export(context.Background(), reports.ExportFilter{Months: 1})
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}
	if report.Expenses == nil {
		t.Error("expected non-nil expenses slice, got nil")
	}
	if report.MonthlySummary == nil {
		t.Error("expected non-nil monthly_summary slice, got nil")
	}
}

func TestRenderCSV_HasHeaderAndData(t *testing.T) {
	repo := &mockRepo{
		expenses: []reports.ExportRow{
			{ID: 1, Date: "2026-03-10", Category: "Food", Description: "Lunch", Amount: 85.50},
		},
		summary: []reports.MonthlySummary{
			{Year: 2026, Month: 3, Label: "Mar 2026", Total: 85.50},
		},
	}
	svc := reports.NewService(repo)
	report, _ := svc.Export(context.Background(), reports.ExportFilter{Months: 1})

	data, err := svc.RenderCSV(report)
	if err != nil {
		t.Fatalf("RenderCSV() error: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "id,date,category,description,amount") {
		t.Error("CSV missing header row")
	}
	if !strings.Contains(content, "85.50") {
		t.Error("CSV missing expense amount")
	}
	if !strings.Contains(content, "Resumen mensual") {
		t.Error("CSV missing monthly summary section")
	}
	if !strings.Contains(content, "Mar 2026") {
		t.Error("CSV missing month label")
	}
}

func TestRenderPDF_ProducesNonEmptyOutput(t *testing.T) {
	repo := &mockRepo{
		expenses: []reports.ExportRow{
			{ID: 1, Date: "2026-03-10", Category: "Food", Description: "Lunch", Amount: 85.50},
		},
		summary: []reports.MonthlySummary{
			{Year: 2026, Month: 3, Label: "Mar 2026", Total: 85.50},
		},
	}
	svc := reports.NewService(repo)
	report, _ := svc.Export(context.Background(), reports.ExportFilter{Months: 1})

	data, err := svc.RenderPDF(report)
	if err != nil {
		t.Fatalf("RenderPDF() error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty PDF output")
	}
	// PDF files start with %PDF
	if !strings.HasPrefix(string(data), "%PDF") {
		t.Error("output does not look like a valid PDF")
	}
}

func TestMonthLabel_FromSummary(t *testing.T) {
	repo := &mockRepo{
		summary: []reports.MonthlySummary{
			{Year: 2026, Month: 3, Label: "Mar 2026", Total: 100},
		},
	}
	svc := reports.NewService(repo)
	report, _ := svc.Export(context.Background(), reports.ExportFilter{Months: 1})

	if len(report.MonthlySummary) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(report.MonthlySummary))
	}
	if report.MonthlySummary[0].Label != "Mar 2026" {
		t.Errorf("expected label 'Mar 2026', got %q", report.MonthlySummary[0].Label)
	}
}
