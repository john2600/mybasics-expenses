package analytics

import (
	"context"
	"math"
	"time"
)

// Service defines the business logic for analytics.
type Service interface {
	Summary(ctx context.Context, f Filter) (*Summary, error)
	ByCategory(ctx context.Context, f Filter) (*ByCategory, error)
	Trend(ctx context.Context, f Filter) (*Trend, error)
	TopExpenses(ctx context.Context, f Filter, limit int) (*TopExpenses, error)
	IncomeVsExpense(ctx context.Context, f Filter) (*IncomeVsExpense, error)
}

type service struct {
	repo Repository
}

// NewService creates a new analytics service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Summary(ctx context.Context, f Filter) (*Summary, error) {
	from, to := dateRange(f.Months)

	totalSpent, count, points, err := s.repo.Summary(ctx, from, to)
	if err != nil {
		return nil, err
	}

	var peak PeakMonth
	var monthlyAvg float64
	if len(points) > 0 {
		for _, p := range points {
			if p.Total > peak.Total {
				peak = PeakMonth{Year: p.Year, Month: p.Month, Label: monthLabel(p.Year, p.Month), Total: p.Total}
			}
		}
		monthlyAvg = round2(totalSpent / float64(len(points)))
	}

	return &Summary{
		PeriodFrom:     from.Format("2006-01-02"),
		PeriodTo:       to.Format("2006-01-02"),
		Months:         f.Months,
		TotalSpent:     totalSpent,
		MonthlyAverage: monthlyAvg,
		PeakMonth:      peak,
		ExpenseCount:   count,
	}, nil
}

func (s *service) ByCategory(ctx context.Context, f Filter) (*ByCategory, error) {
	from, to := dateRange(f.Months)

	cats, err := s.repo.ByCategory(ctx, from, to)
	if err != nil {
		return nil, err
	}

	var total float64
	for _, c := range cats {
		total += c.Total
	}

	for i := range cats {
		if total > 0 {
			cats[i].Percentage = round2(cats[i].Total / total * 100)
		}
	}

	if cats == nil {
		cats = []CategoryBreakdown{}
	}

	return &ByCategory{
		PeriodFrom: from.Format("2006-01-02"),
		PeriodTo:   to.Format("2006-01-02"),
		TotalSpent: total,
		Categories: cats,
	}, nil
}

func (s *service) Trend(ctx context.Context, f Filter) (*Trend, error) {
	from, to := dateRange(f.Months)

	points, err := s.repo.Trend(ctx, from, to)
	if err != nil {
		return nil, err
	}

	for i := range points {
		points[i].Label = monthLabel(points[i].Year, points[i].Month)
	}

	if points == nil {
		points = []TrendPoint{}
	}

	return &Trend{
		PeriodFrom: from.Format("2006-01-02"),
		PeriodTo:   to.Format("2006-01-02"),
		Points:     points,
	}, nil
}

func (s *service) TopExpenses(ctx context.Context, f Filter, limit int) (*TopExpenses, error) {
	from, to := dateRange(f.Months)

	expenses, err := s.repo.TopExpenses(ctx, from, to, limit)
	if err != nil {
		return nil, err
	}

	if expenses == nil {
		expenses = []TopExpense{}
	}

	return &TopExpenses{
		PeriodFrom: from.Format("2006-01-02"),
		PeriodTo:   to.Format("2006-01-02"),
		Expenses:   expenses,
	}, nil
}

func (s *service) IncomeVsExpense(ctx context.Context, f Filter) (*IncomeVsExpense, error) {
	from, to := dateRange(f.Months)

	months, err := s.repo.IncomeVsExpense(ctx, from, to)
	if err != nil {
		return nil, err
	}

	var totals Totals
	for i := range months {
		months[i].Label = monthLabel(months[i].Year, months[i].Month)
		months[i].Balance = months[i].Income - months[i].Expense
		totals.Income += months[i].Income
		totals.Expense += months[i].Expense
	}
	totals.Balance = totals.Income - totals.Expense

	if months == nil {
		months = []MonthIVE{}
	}

	return &IncomeVsExpense{
		PeriodFrom: from.Format("2006-01-02"),
		PeriodTo:   to.Format("2006-01-02"),
		Months:     months,
		Totals:     totals,
	}, nil
}

// dateRange returns the inclusive date window for the last N months.
func dateRange(months int) (from, to time.Time) {
	now := time.Now().UTC()
	to = time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	from = time.Date(now.Year(), now.Month()-time.Month(months-1), 1, 0, 0, 0, 0, time.UTC)
	return
}

func monthLabel(year, month int) string {
	t := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	return t.Format("Jan 2006")
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
