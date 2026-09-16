package balance_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jscodelab/mybasics-expenses/internal/balance"
	"github.com/jscodelab/mybasics-expenses/internal/incomes"
)

var errBoom = errors.New("db down")

// failingRepo lets each method fail independently, so the tests can pin which
// call site produced the error instead of having one failure mask another.
type failingRepo struct {
	summaryErr  error
	periodErr   error
	earliestErr error

	earliest *time.Time
}

func (f *failingRepo) GetSummary(_ context.Context, _ int, _, _ *time.Time) (*balance.Summary, error) {
	if f.summaryErr != nil {
		return nil, f.summaryErr
	}
	return &balance.Summary{}, nil
}

func (f *failingRepo) GetPeriodSummary(_ context.Context, _ int, _, _ time.Time) (float64, float64, error) {
	if f.periodErr != nil {
		return 0, 0, f.periodErr
	}
	return 0, 0, nil
}

func (f *failingRepo) GetEarliestMovementDate(_ context.Context, _ int) (*time.Time, error) {
	if f.earliestErr != nil {
		return nil, f.earliestErr
	}
	return f.earliest, nil
}

// splitIncomesRepo distinguishes Get from GetForMonth, which the service calls
// at different points — GetBalance picks one or the other depending on whether
// a date range was supplied.
type splitIncomesRepo struct {
	cfg         *incomes.Config
	getErr      error
	forMonthErr error
}

func (s *splitIncomesRepo) Get(_ context.Context, _ int) (*incomes.Config, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.cfg, nil
}

func (s *splitIncomesRepo) GetForMonth(_ context.Context, _ int, _ time.Time) (*incomes.Config, error) {
	if s.forMonthErr != nil {
		return nil, s.forMonthErr
	}
	return s.cfg, nil
}

func (s *splitIncomesRepo) Update(_ context.Context, _ int, _ incomes.UpdateRequest) (*incomes.Config, error) {
	return s.cfg, nil
}

func okConfig() *incomes.Config {
	return &incomes.Config{Amount: 1000, CutDay: 24, Description: "Salario"}
}

func TestGetBalance_WithDatesUsesTheConfigEffectiveForThatMonth(t *testing.T) {
	// With a date range the service must ask for the config in force during
	// that month, not the latest one, so past balances stay reproducible.
	inc := &splitIncomesRepo{cfg: okConfig(), forMonthErr: errBoom}
	svc := balance.NewService(&failingRepo{}, inc)

	if _, err := svc.GetBalance(context.Background(), 1, "2026-07-01", "2026-07-31"); err == nil {
		t.Fatal("expected GetForMonth to be the call that fails")
	}
}

func TestGetBalance_WithoutDatesUsesTheLatestConfig(t *testing.T) {
	inc := &splitIncomesRepo{cfg: okConfig(), getErr: errBoom}
	svc := balance.NewService(&failingRepo{}, inc)

	if _, err := svc.GetBalance(context.Background(), 1, "", ""); err == nil {
		t.Fatal("expected Get to be the call that fails")
	}
}

func TestGetBalance_PropagatesSummaryError(t *testing.T) {
	svc := balance.NewService(&failingRepo{summaryErr: errBoom}, &splitIncomesRepo{cfg: okConfig()})

	if _, err := svc.GetBalance(context.Background(), 1, "", ""); err == nil {
		t.Fatal("expected the summary error to propagate")
	}
}

func TestGetPeriods_PropagatesConfigError(t *testing.T) {
	svc := balance.NewService(&failingRepo{}, &splitIncomesRepo{getErr: errBoom})

	if _, err := svc.GetPeriods(context.Background(), 1); err == nil {
		t.Fatal("expected the config error to propagate")
	}
}

func TestGetPeriods_PropagatesEarliestDateError(t *testing.T) {
	svc := balance.NewService(&failingRepo{earliestErr: errBoom}, &splitIncomesRepo{cfg: okConfig()})

	if _, err := svc.GetPeriods(context.Background(), 1); err == nil {
		t.Fatal("expected the earliest-date error to propagate")
	}
}

func TestGetPeriods_PropagatesPerPeriodConfigError(t *testing.T) {
	earliest := time.Now().AddDate(0, -2, 0)
	// Get succeeds (it drives cut_day) but the per-period lookup fails.
	inc := &splitIncomesRepo{cfg: okConfig(), forMonthErr: errBoom}
	svc := balance.NewService(&failingRepo{earliest: &earliest}, inc)

	if _, err := svc.GetPeriods(context.Background(), 1); err == nil {
		t.Fatal("expected the per-period config error to propagate")
	}
}

func TestGetPeriods_PropagatesPeriodSummaryError(t *testing.T) {
	earliest := time.Now().AddDate(0, -2, 0)
	svc := balance.NewService(
		&failingRepo{earliest: &earliest, periodErr: errBoom},
		&splitIncomesRepo{cfg: okConfig()},
	)

	if _, err := svc.GetPeriods(context.Background(), 1); err == nil {
		t.Fatal("expected the period summary error to propagate")
	}
}

func TestGetPeriods_PeriodStartsBeforeTheCutDayBelongToThePreviousMonth(t *testing.T) {
	// A movement on the 5th with cut_day 24 belongs to the period that opened
	// on the 24th of the *previous* month. This drives the branch of
	// periodStart that steps a month back.
	earliest := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	svc := balance.NewService(
		&failingRepo{earliest: &earliest},
		&splitIncomesRepo{cfg: okConfig()},
	)

	got, err := svc.GetPeriods(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetPeriods returned error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one period")
	}
	first := got[0].PeriodStart
	if first.Day() != 24 {
		t.Errorf("first period starts on day %d, want the cut day 24", first.Day())
	}
	if first.Month() != time.June {
		t.Errorf("first period month = %v, want June (the period containing July 5)", first.Month())
	}
}
