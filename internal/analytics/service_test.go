package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/jscodelab/mybasics-expenses/internal/analytics"
)

// mockRepo is a test double for analytics.Repository.
type mockRepo struct {
	summaryTotal  float64
	summaryCount  int
	summaryPoints []analytics.TrendPoint
	categories    []analytics.CategoryBreakdown
	trendPoints   []analytics.TrendPoint
	topExpenses   []analytics.TopExpense
	iveMonths     []analytics.MonthIVE
}

func (m *mockRepo) Summary(_ context.Context, _ int, _, _ time.Time) (float64, int, []analytics.TrendPoint, error) {
	return m.summaryTotal, m.summaryCount, m.summaryPoints, nil
}
func (m *mockRepo) ByCategory(_ context.Context, _ int, _, _ time.Time) ([]analytics.CategoryBreakdown, error) {
	return m.categories, nil
}
func (m *mockRepo) Trend(_ context.Context, _ int, _, _ time.Time) ([]analytics.TrendPoint, error) {
	return m.trendPoints, nil
}
func (m *mockRepo) TopExpenses(_ context.Context, _ int, _, _ time.Time, limit int) ([]analytics.TopExpense, error) {
	if limit < len(m.topExpenses) {
		return m.topExpenses[:limit], nil
	}
	return m.topExpenses, nil
}
func (m *mockRepo) IncomeVsExpense(_ context.Context, _ int, _, _ time.Time) ([]analytics.MonthIVE, error) {
	return m.iveMonths, nil
}

func TestSummary_MonthlyAverage(t *testing.T) {
	repo := &mockRepo{
		summaryTotal: 3000,
		summaryCount: 20,
		summaryPoints: []analytics.TrendPoint{
			{Year: 2026, Month: 3, Total: 1000, Count: 7},
			{Year: 2026, Month: 4, Total: 800, Count: 6},
			{Year: 2026, Month: 5, Total: 1200, Count: 7},
		},
	}
	svc := analytics.NewService(repo)
	result, err := svc.Summary(context.Background(), analytics.Filter{Months: 3})
	if err != nil {
		t.Fatalf("Summary() error: %v", err)
	}
	if result.TotalSpent != 3000 {
		t.Errorf("expected TotalSpent 3000, got %.2f", result.TotalSpent)
	}
	if result.MonthlyAverage != 1000 {
		t.Errorf("expected MonthlyAverage 1000, got %.2f", result.MonthlyAverage)
	}
	if result.ExpenseCount != 20 {
		t.Errorf("expected ExpenseCount 20, got %d", result.ExpenseCount)
	}
}

func TestSummary_PeakMonth(t *testing.T) {
	repo := &mockRepo{
		summaryTotal: 3000,
		summaryPoints: []analytics.TrendPoint{
			{Year: 2026, Month: 3, Total: 1000},
			{Year: 2026, Month: 4, Total: 800},
			{Year: 2026, Month: 5, Total: 1200},
		},
	}
	svc := analytics.NewService(repo)
	result, _ := svc.Summary(context.Background(), analytics.Filter{Months: 3})

	if result.PeakMonth.Month != 5 {
		t.Errorf("expected peak month 5, got %d", result.PeakMonth.Month)
	}
	if result.PeakMonth.Total != 1200 {
		t.Errorf("expected peak total 1200, got %.2f", result.PeakMonth.Total)
	}
	if result.PeakMonth.Label != "May 2026" {
		t.Errorf("expected peak label 'May 2026', got %q", result.PeakMonth.Label)
	}
}

func TestByCategory_Percentage(t *testing.T) {
	repo := &mockRepo{
		categories: []analytics.CategoryBreakdown{
			{CategoryID: 1, Category: "Housing", Total: 1000, Count: 1},
			{CategoryID: 2, Category: "Food", Total: 500, Count: 5},
			{CategoryID: 3, Category: "Transport", Total: 500, Count: 3},
		},
	}
	svc := analytics.NewService(repo)
	result, err := svc.ByCategory(context.Background(), analytics.Filter{Months: 3})
	if err != nil {
		t.Fatalf("ByCategory() error: %v", err)
	}
	if result.TotalSpent != 2000 {
		t.Errorf("expected TotalSpent 2000, got %.2f", result.TotalSpent)
	}
	if result.Categories[0].Percentage != 50 {
		t.Errorf("expected Housing percentage 50, got %.2f", result.Categories[0].Percentage)
	}
	if result.Categories[1].Percentage != 25 {
		t.Errorf("expected Food percentage 25, got %.2f", result.Categories[1].Percentage)
	}
}

func TestTrend_LabelGeneration(t *testing.T) {
	cases := []struct {
		year, month int
		want        string
	}{
		{2026, 1, "Jan 2026"},
		{2026, 3, "Mar 2026"},
		{2026, 12, "Dec 2026"},
	}
	for _, tc := range cases {
		repo := &mockRepo{
			trendPoints: []analytics.TrendPoint{
				{Year: tc.year, Month: tc.month, Total: 100, Count: 3},
			},
		}
		svc := analytics.NewService(repo)
		result, err := svc.Trend(context.Background(), analytics.Filter{Months: 1})
		if err != nil {
			t.Fatalf("Trend() error: %v", err)
		}
		if result.Points[0].Label != tc.want {
			t.Errorf("month %d: expected label %q, got %q", tc.month, tc.want, result.Points[0].Label)
		}
	}
}

func TestTopExpenses_LimitApplied(t *testing.T) {
	expenses := make([]analytics.TopExpense, 15)
	for i := range expenses {
		expenses[i] = analytics.TopExpense{ID: int64(i + 1), Amount: float64(100 - i)}
	}
	repo := &mockRepo{topExpenses: expenses}
	svc := analytics.NewService(repo)

	result, err := svc.TopExpenses(context.Background(), analytics.Filter{Months: 3}, 5)
	if err != nil {
		t.Fatalf("TopExpenses() error: %v", err)
	}
	if len(result.Expenses) != 5 {
		t.Errorf("expected 5 expenses with limit=5, got %d", len(result.Expenses))
	}
}

func TestIncomeVsExpense_BalanceAndTotals(t *testing.T) {
	repo := &mockRepo{
		iveMonths: []analytics.MonthIVE{
			{Year: 2026, Month: 3, Income: 3500, Expense: 1800},
			{Year: 2026, Month: 4, Income: 3500, Expense: 1400},
			{Year: 2026, Month: 5, Income: 3500, Expense: 1600},
		},
	}
	svc := analytics.NewService(repo)
	result, err := svc.IncomeVsExpense(context.Background(), analytics.Filter{Months: 3})
	if err != nil {
		t.Fatalf("IncomeVsExpense() error: %v", err)
	}

	// Per-month balance
	if result.Months[0].Balance != 1700 {
		t.Errorf("month 0 balance: expected 1700, got %.2f", result.Months[0].Balance)
	}

	// Totals
	if result.Totals.Income != 10500 {
		t.Errorf("total income: expected 10500, got %.2f", result.Totals.Income)
	}
	if result.Totals.Expense != 4800 {
		t.Errorf("total expense: expected 4800, got %.2f", result.Totals.Expense)
	}
	if result.Totals.Balance != 5700 {
		t.Errorf("total balance: expected 5700, got %.2f", result.Totals.Balance)
	}

	// Labels
	if result.Months[0].Label != "Mar 2026" {
		t.Errorf("expected label 'Mar 2026', got %q", result.Months[0].Label)
	}
}

func TestDateRange_ThreeMonths(t *testing.T) {
	svc := analytics.NewService(&mockRepo{})
	result, err := svc.Summary(context.Background(), analytics.Filter{Months: 3})
	if err != nil {
		t.Fatalf("Summary() error: %v", err)
	}

	now := time.Now().UTC()
	expectedFrom := time.Date(now.Year(), now.Month()-2, 1, 0, 0, 0, 0, time.UTC).Format(time.DateOnly)
	if result.PeriodFrom != expectedFrom {
		t.Errorf("expected PeriodFrom %s, got %s", expectedFrom, result.PeriodFrom)
	}
}
