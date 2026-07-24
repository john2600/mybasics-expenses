package balance

import (
	"context"
	"fmt"
	"time"

	"github.com/jscodelab/mybasics-expenses/internal/incomes"
)

// Service handles balance business logic.
type Service interface {
	GetBalance(ctx context.Context, userID int, dateFrom, dateTo string) (*Summary, error)
	GetPeriods(ctx context.Context, userID int) ([]PeriodSummary, error)
}

type service struct {
	repo        Repository
	incomesRepo incomes.Repository
}

// NewService creates a balance Service.
func NewService(repo Repository, incomesRepo incomes.Repository) Service {
	return &service{repo: repo, incomesRepo: incomesRepo}
}

func (s *service) GetBalance(ctx context.Context, userID int, dateFrom, dateTo string) (*Summary, error) {
	from, err := parseDate(dateFrom)
	if err != nil {
		return nil, fmt.Errorf("invalid date_from: %w", err)
	}
	to, err := parseDate(dateTo)
	if err != nil {
		return nil, fmt.Errorf("invalid date_to: %w", err)
	}

	// Use the config that was active for this period's month.
	var cfg *incomes.Config
	if from != nil {
		cfg, err = s.incomesRepo.GetForMonth(ctx, *from)
	} else {
		cfg, err = s.incomesRepo.Get(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("fetching income config: %w", err)
	}

	summary, err := s.repo.GetSummary(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}

	summary.IncomeConfig = cfg
	summary.Balance = (cfg.Amount + summary.Incomes) - summary.Expenses

	// Calculate carry_over: balance of the previous billing period.
	// If date_from is provided, previous period ends the day before date_from.
	// If not, derive the current period start from cut_day and go back one month.
	var prevFrom, prevTo time.Time
	if from != nil {
		prevTo = *from
		prevFrom = prevTo.AddDate(0, -1, 0)
	} else {
		currentStart := periodStart(time.Now(), cfg.CutDay)
		prevTo = currentStart
		prevFrom = currentStart.AddDate(0, -1, 0)
	}

	// Use the config that was active during the previous period.
	prevCfg, err := s.incomesRepo.GetForMonth(ctx, prevFrom)
	if err != nil {
		return nil, fmt.Errorf("fetching prev income config: %w", err)
	}

	prevExpenses, prevIncomes, err := s.repo.GetPeriodSummary(ctx, userID, prevFrom, prevTo)
	if err != nil {
		return nil, fmt.Errorf("fetching previous period: %w", err)
	}

	prevBalance := prevCfg.Amount + prevIncomes - prevExpenses
	if prevBalance > 0 {
		summary.CarryOverOut = prevBalance
	}

	return summary, nil
}

func (s *service) GetPeriods(ctx context.Context, userID int) ([]PeriodSummary, error) {
	// Latest config drives the cut_day used to build period boundaries.
	latestCfg, err := s.incomesRepo.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching income config: %w", err)
	}

	earliest, err := s.repo.GetEarliestMovementDate(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching earliest movement date: %w", err)
	}
	if earliest == nil {
		return []PeriodSummary{}, nil
	}

	now := time.Now()

	// Build all period start dates from the earliest movement to today.
	periodStarts := buildPeriodStarts(*earliest, now, latestCfg.CutDay)

	var result []PeriodSummary
	var carryOver float64

	for _, start := range periodStarts {
		end := start.AddDate(0, 1, 0) // exclusive upper bound

		// Use the income config that was active during this period's month.
		cfg, err := s.incomesRepo.GetForMonth(ctx, start)
		if err != nil {
			return nil, fmt.Errorf("fetching config for period %s: %w", start.Format("2006-01"), err)
		}

		expenses, regIncomes, err := s.repo.GetPeriodSummary(ctx, userID, start, end)
		if err != nil {
			return nil, fmt.Errorf("querying period %s: %w", start.Format(time.DateOnly), err)
		}

		totalIncome := cfg.Amount + regIncomes
		balance := totalIncome + carryOver - expenses

		var carryOut, deficit float64
		if balance >= 0 {
			carryOut = balance
		} else {
			deficit = -balance // show the deficit but don't carry it forward
			carryOut = 0
		}

		result = append(result, PeriodSummary{
			PeriodStart:       start,
			PeriodEnd:         end.AddDate(0, 0, -1), // inclusive last day for display
			FixedIncome:       cfg.Amount,
			RegisteredIncomes: regIncomes,
			TotalIncome:       totalIncome,
			Expenses:          expenses,
			CarryOverIn:       carryOver,
			Balance:           balance,
			CarryOverOut:      carryOut,
			Deficit:           deficit,
		})

		carryOver = carryOut
	}

	return result, nil
}

// buildPeriodStarts returns the start date of each billing period from the
// period containing earliest up to and including the period containing now.
func buildPeriodStarts(earliest, now time.Time, cutDay int) []time.Time {
	start := periodStart(earliest, cutDay)
	current := periodStart(now, cutDay)

	var starts []time.Time
	for !start.After(current) {
		starts = append(starts, start)
		start = start.AddDate(0, 1, 0)
	}
	return starts
}

// periodStart returns the start of the billing period that contains date.
func periodStart(date time.Time, cutDay int) time.Time {
	if date.Day() >= cutDay {
		return time.Date(date.Year(), date.Month(), cutDay, 0, 0, 0, 0, time.UTC)
	}
	prev := date.AddDate(0, -1, 0)
	return time.Date(prev.Year(), prev.Month(), cutDay, 0, 0, 0, 0, time.UTC)
}

func parseDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil, fmt.Errorf("expected YYYY-MM-DD, got %q", s)
	}
	return &t, nil
}
