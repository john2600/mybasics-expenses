package incomes_test

import (
	"context"
	"testing"
	"time"

	"github.com/jscodelab/mybasics-expenses/internal/incomes"
)

type mockRepository struct {
	cfg *incomes.Config
	err error
}

func (m *mockRepository) Get(_ context.Context, _ int) (*incomes.Config, error) {
	return m.cfg, m.err
}

func (m *mockRepository) GetForMonth(_ context.Context, _ int, _ time.Time) (*incomes.Config, error) {
	return m.cfg, m.err
}

func (m *mockRepository) Update(_ context.Context, _ int, _ incomes.UpdateRequest) (*incomes.Config, error) {
	return m.cfg, m.err
}

func ptr[T any](v T) *T { return &v }

func TestGetConfig_Success(t *testing.T) {
	want := &incomes.Config{Amount: 5000000, CutDay: 24, Description: "Salario"}
	svc := incomes.NewService(&mockRepository{cfg: want})

	got, err := svc.GetConfig(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Amount != want.Amount {
		t.Errorf("amount: want %.2f, got %.2f", want.Amount, got.Amount)
	}
	if got.CutDay != want.CutDay {
		t.Errorf("cut_day: want %d, got %d", want.CutDay, got.CutDay)
	}
}

func TestUpdateConfig_ValidAmount(t *testing.T) {
	want := &incomes.Config{Amount: 3000000, CutDay: 15}
	svc := incomes.NewService(&mockRepository{cfg: want})

	got, err := svc.UpdateConfig(context.Background(), 1, incomes.UpdateRequest{Amount: ptr(3000000.0)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Amount != 3000000 {
		t.Errorf("amount: want 3000000, got %.2f", got.Amount)
	}
}

func TestUpdateConfig_NegativeAmount(t *testing.T) {
	svc := incomes.NewService(&mockRepository{})

	_, err := svc.UpdateConfig(context.Background(), 1, incomes.UpdateRequest{Amount: ptr(-1.0)})
	if err == nil {
		t.Fatal("expected error for negative amount, got nil")
	}
}

func TestUpdateConfig_CutDayZero(t *testing.T) {
	svc := incomes.NewService(&mockRepository{})

	_, err := svc.UpdateConfig(context.Background(), 1, incomes.UpdateRequest{CutDay: ptr(0)})
	if err == nil {
		t.Fatal("expected error for cut_day=0, got nil")
	}
}

func TestUpdateConfig_CutDay29(t *testing.T) {
	svc := incomes.NewService(&mockRepository{})

	_, err := svc.UpdateConfig(context.Background(), 1, incomes.UpdateRequest{CutDay: ptr(29)})
	if err == nil {
		t.Fatal("expected error for cut_day=29, got nil")
	}
}

func TestUpdateConfig_CutDayBoundary(t *testing.T) {
	want := &incomes.Config{Amount: 0, CutDay: 1}
	svc := incomes.NewService(&mockRepository{cfg: want})

	_, err := svc.UpdateConfig(context.Background(), 1, incomes.UpdateRequest{CutDay: ptr(1)})
	if err != nil {
		t.Fatalf("unexpected error for cut_day=1: %v", err)
	}

	want.CutDay = 28
	_, err = svc.UpdateConfig(context.Background(), 1, incomes.UpdateRequest{CutDay: ptr(28)})
	if err != nil {
		t.Fatalf("unexpected error for cut_day=28: %v", err)
	}
}

func TestUpdateConfig_ZeroAmountAllowed(t *testing.T) {
	want := &incomes.Config{Amount: 0, CutDay: 24}
	svc := incomes.NewService(&mockRepository{cfg: want})

	_, err := svc.UpdateConfig(context.Background(), 1, incomes.UpdateRequest{Amount: ptr(0.0)})
	if err != nil {
		t.Fatalf("amount=0 should be valid, got error: %v", err)
	}
}
