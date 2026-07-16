package balance_test

import (
	"context"
	"testing"
	"time"

	"github.com/jscodelab/mybasics-expenses/internal/balance"
	"github.com/jscodelab/mybasics-expenses/internal/incomes"
)


type mockRepository struct {
	summary         *balance.Summary
	err             error
	earliestDate    *time.Time
	periodExpenses  float64
	periodIncomes   float64
}

func (m *mockRepository) GetSummary(_ context.Context, _, _ *time.Time) (*balance.Summary, error) {
	return m.summary, m.err
}

func (m *mockRepository) GetPeriodSummary(_ context.Context, _, _ time.Time) (float64, float64, error) {
	return m.periodExpenses, m.periodIncomes, m.err
}

func (m *mockRepository) GetEarliestMovementDate(_ context.Context) (*time.Time, error) {
	return m.earliestDate, m.err
}

type mockIncomesRepository struct {
	cfg *incomes.Config
	err error
}

func (m *mockIncomesRepository) Get(_ context.Context) (*incomes.Config, error) {
	return m.cfg, m.err
}

func (m *mockIncomesRepository) GetForMonth(_ context.Context, _ time.Time) (*incomes.Config, error) {
	return m.cfg, m.err
}

func (m *mockIncomesRepository) Update(_ context.Context, _ incomes.UpdateRequest) (*incomes.Config, error) {
	return m.cfg, m.err
}

var defaultIncomesRepo = &mockIncomesRepository{
	cfg: &incomes.Config{Amount: 5000000, CutDay: 24, Description: "Salario"},
}

func TestGetBalance_WithDates(t *testing.T) {
	// income_config.amount=5_000_000, incomes=3_500_000, expenses=500_000
	// balance = (5_000_000 + 3_500_000) - 500_000 = 8_000_000
	repo := &mockRepository{
		summary: &balance.Summary{Expenses: 500000, Incomes: 3500000},
	}
	svc := balance.NewService(repo, defaultIncomesRepo)

	got, err := svc.GetBalance(context.Background(), "2026-03-01", "2026-03-30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Expenses != 500000 {
		t.Errorf("expenses: want 500000, got %.2f", got.Expenses)
	}
	if got.Incomes != 3500000 {
		t.Errorf("incomes: want 3500000, got %.2f", got.Incomes)
	}
	if got.Balance != 8000000 {
		t.Errorf("balance: want 8000000, got %.2f", got.Balance)
	}
}

func TestGetBalance_NoDates(t *testing.T) {
	repo := &mockRepository{
		summary: &balance.Summary{Expenses: 100000, Incomes: 200000, Balance: 100000},
	}
	svc := balance.NewService(repo, defaultIncomesRepo)

	_, err := svc.GetBalance(context.Background(), "", "")
	if err != nil {
		t.Fatalf("unexpected error with empty dates: %v", err)
	}
}

func TestGetBalance_InvalidDateFrom(t *testing.T) {
	svc := balance.NewService(&mockRepository{}, defaultIncomesRepo)

	_, err := svc.GetBalance(context.Background(), "not-a-date", "")
	if err == nil {
		t.Fatal("expected error for invalid date_from, got nil")
	}
}

func TestGetBalance_InvalidDateTo(t *testing.T) {
	svc := balance.NewService(&mockRepository{}, defaultIncomesRepo)

	_, err := svc.GetBalance(context.Background(), "", "not-a-date")
	if err == nil {
		t.Fatal("expected error for invalid date_to, got nil")
	}
}

func TestGetBalance_NegativeBalance(t *testing.T) {
	// income_config.amount=5_000_000, incomes=500_000, expenses=7_000_000
	// balance = (5_000_000 + 500_000) - 7_000_000 = -1_500_000
	repo := &mockRepository{
		summary: &balance.Summary{Expenses: 7000000, Incomes: 500000},
	}
	svc := balance.NewService(repo, defaultIncomesRepo)

	got, err := svc.GetBalance(context.Background(), "2026-03-01", "2026-03-30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Balance != -1500000 {
		t.Errorf("balance: want -1500000, got %.2f", got.Balance)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestGetPeriods_NoMovements(t *testing.T) {
	repo := &mockRepository{earliestDate: nil}
	svc := balance.NewService(repo, defaultIncomesRepo)

	periods, err := svc.GetPeriods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(periods) != 0 {
		t.Errorf("expected 0 periods, got %d", len(periods))
	}
}

func TestGetPeriods_SinglePeriodSurplus(t *testing.T) {
	// income_config: amount=5_000_000, cut_day=24
	// period expenses=2_000_000, registered_incomes=0
	// carry_over_in=0 → balance=3_000_000 → carry_over_out=3_000_000, deficit=0
	earliest := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC) // dentro del periodo 24 Mar-23 Abr
	repo := &mockRepository{
		earliestDate:   ptrTime(earliest),
		periodExpenses: 2_000_000,
		periodIncomes:  0,
	}
	svc := balance.NewService(repo, defaultIncomesRepo)

	periods, err := svc.GetPeriods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(periods) == 0 {
		t.Fatal("expected at least one period")
	}
	p := periods[len(periods)-1] // último periodo (el actual)
	if p.CarryOverOut != 3_000_000 {
		t.Errorf("carry_over_out: want 3000000, got %.2f", p.CarryOverOut)
	}
	if p.Deficit != 0 {
		t.Errorf("deficit: want 0, got %.2f", p.Deficit)
	}
}

func TestGetPeriods_DeficitNotCarriedForward(t *testing.T) {
	// income_config: amount=5_000_000, cut_day=24
	// expenses=7_000_000 → balance=-2_000_000 → carry_over_out=0, deficit=2_000_000
	earliest := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	repo := &mockRepository{
		earliestDate:   ptrTime(earliest),
		periodExpenses: 7_000_000,
		periodIncomes:  0,
	}
	svc := balance.NewService(repo, defaultIncomesRepo)

	periods, err := svc.GetPeriods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := periods[len(periods)-1]
	if p.CarryOverOut != 0 {
		t.Errorf("carry_over_out should be 0 on deficit, got %.2f", p.CarryOverOut)
	}
	if p.Deficit != 2_000_000 {
		t.Errorf("deficit: want 2000000, got %.2f", p.Deficit)
	}
}

func TestGetPeriods_CarryOverAddsToNextPeriod(t *testing.T) {
	// Dos periodos: mock devuelve siempre expenses=3_000_000, incomes=0
	// Periodo 1: income=5M + carry_in=0 - exp=3M = balance=2M → carry_out=2M
	// Periodo 2: income=5M + carry_in=2M - exp=3M = balance=4M → carry_out=4M
	earliest := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) // fuerza 2 periodos
	repo := &mockRepository{
		earliestDate:   ptrTime(earliest),
		periodExpenses: 3_000_000,
		periodIncomes:  0,
	}
	svc := balance.NewService(repo, defaultIncomesRepo)

	periods, err := svc.GetPeriods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(periods) < 2 {
		t.Fatalf("expected at least 2 periods, got %d", len(periods))
	}
	first := periods[0]
	second := periods[1]

	if first.CarryOverOut != 2_000_000 {
		t.Errorf("period 1 carry_over_out: want 2000000, got %.2f", first.CarryOverOut)
	}
	if second.CarryOverIn != 2_000_000 {
		t.Errorf("period 2 carry_over_in: want 2000000, got %.2f", second.CarryOverIn)
	}
	if second.CarryOverOut != 4_000_000 {
		t.Errorf("period 2 carry_over_out: want 4000000, got %.2f", second.CarryOverOut)
	}
}

func TestGetPeriods_RegisteredIncomesIncluded(t *testing.T) {
	// registered_incomes=500_000, expenses=3_000_000, fixed=5_000_000
	// balance = 5M + 500K - 3M = 2.5M
	earliest := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	repo := &mockRepository{
		earliestDate:   ptrTime(earliest),
		periodExpenses: 3_000_000,
		periodIncomes:  500_000,
	}
	svc := balance.NewService(repo, defaultIncomesRepo)

	periods, err := svc.GetPeriods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := periods[len(periods)-1]
	if p.TotalIncome != 5_500_000 {
		t.Errorf("total_income: want 5500000, got %.2f", p.TotalIncome)
	}
	if p.Balance != 2_500_000 {
		t.Errorf("balance: want 2500000, got %.2f", p.Balance)
	}
}

func TestGetBalance_CarryOverFromPreviousPeriod(t *testing.T) {
	// Periodo anterior: income=5M, expenses=3M → carry_over = 2M
	repo := &mockRepository{
		summary:        &balance.Summary{Expenses: 500000, Incomes: 0},
		periodExpenses: 3_000_000,
		periodIncomes:  0,
	}
	svc := balance.NewService(repo, defaultIncomesRepo)

	got, err := svc.GetBalance(context.Background(), "2026-03-24", "2026-04-18")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CarryOverOut != 2_000_000 {
		t.Errorf("carry_over: want 2000000, got %.2f", got.CarryOverOut)
	}
}

func TestGetBalance_CarryOverZeroWhenDeficit(t *testing.T) {
	// Periodo anterior: income=5M, expenses=7M → balance negativo → carry_over = 0
	repo := &mockRepository{
		summary:        &balance.Summary{Expenses: 500000, Incomes: 0},
		periodExpenses: 7_000_000,
		periodIncomes:  0,
	}
	svc := balance.NewService(repo, defaultIncomesRepo)

	got, err := svc.GetBalance(context.Background(), "2026-03-24", "2026-04-18")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CarryOverOut != 0 {
		t.Errorf("carry_over should be 0 on deficit, got %.2f", got.CarryOverOut)
	}
}

func TestGetBalance_CarryOverNoDates(t *testing.T) {
	// Sin fechas: usa cut_day=24 para calcular el periodo anterior
	// periodo anterior expenses=2M → carry_over = 5M - 2M = 3M
	repo := &mockRepository{
		summary:        &balance.Summary{Expenses: 500000, Incomes: 0},
		periodExpenses: 2_000_000,
		periodIncomes:  0,
	}
	svc := balance.NewService(repo, defaultIncomesRepo)

	got, err := svc.GetBalance(context.Background(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CarryOverOut != 3_000_000 {
		t.Errorf("carry_over: want 3000000, got %.2f", got.CarryOverOut)
	}
}

func TestGetBalance_IncludesIncomeConfig(t *testing.T) {
	repo := &mockRepository{
		summary: &balance.Summary{Expenses: 100000, Incomes: 0, Balance: -100000},
	}
	svc := balance.NewService(repo, defaultIncomesRepo)

	got, err := svc.GetBalance(context.Background(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.IncomeConfig == nil {
		t.Fatal("expected income_config in response, got nil")
	}
	if got.IncomeConfig.Amount != 5000000 {
		t.Errorf("income_config.amount: want 5000000, got %.2f", got.IncomeConfig.Amount)
	}
	if got.IncomeConfig.CutDay != 24 {
		t.Errorf("income_config.cut_day: want 24, got %d", got.IncomeConfig.CutDay)
	}
}
