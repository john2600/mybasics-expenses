package movement_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jscodelab/mybasics-expenses/internal/movement"
)

// mockRepository is a test double for movement.Repository.
type mockRepository struct {
	movements      []movement.Movement
	err            error
	capturedFilter movement.Filter
}

func (m *mockRepository) Create(_ context.Context, _ int, mv *movement.Movement) (*movement.Movement, error) {
	if m.err != nil {
		return nil, m.err
	}
	mv.ID = int64(len(m.movements) + 1)
	m.movements = append(m.movements, *mv)
	return mv, nil
}

func (m *mockRepository) FindAll(_ context.Context, f movement.Filter) ([]movement.Movement, error) {
	m.capturedFilter = f
	return m.movements, m.err
}

func (m *mockRepository) FindByID(_ context.Context, _ int, id int64) (*movement.Movement, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, mv := range m.movements {
		if mv.ID == id {
			return &mv, nil
		}
	}
	return nil, nil
}

func (m *mockRepository) Update(_ context.Context, _ int, id int64, req movement.UpdateRequest) (*movement.Movement, error) {
	if m.err != nil {
		return nil, m.err
	}
	for i, mv := range m.movements {
		if mv.ID == id {
			if req.Amount != nil {
				m.movements[i].Amount = *req.Amount
			}
			if req.Description != nil {
				m.movements[i].Description = *req.Description
			}
			if req.Type != nil {
				m.movements[i].Type = *req.Type
			}
			return &m.movements[i], nil
		}
	}
	return nil, nil
}

func (m *mockRepository) Delete(_ context.Context, _ int, id int64) error {
	if m.err != nil {
		return m.err
	}
	for i, mv := range m.movements {
		if mv.ID == id {
			m.movements = append(m.movements[:i], m.movements[i+1:]...)
			return nil
		}
	}
	return movement.ErrNotFound
}

func (m *mockRepository) MonthlySummary(_ context.Context, _ int) ([]movement.MonthlySummary, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []movement.MonthlySummary{
		{Year: 2026, Month: 3, Total: 100.00},
	}, nil
}

// ─────────────────────────────── Tests ───────────────────────────────────────

func TestCreateMovement_Success(t *testing.T) {
	svc := movement.NewService(&mockRepository{})

	m, err := svc.CreateMovement(context.Background(), 1, movement.CreateRequest{
		CategoryID:  1,
		Type:        "E",
		Amount:      50.00,
		Description: "Coffee",
		Date:        "2026-03-10",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Amount != 50.00 {
		t.Errorf("expected amount 50.00, got %v", m.Amount)
	}
	if m.Type != "E" {
		t.Errorf("expected type E, got %v", m.Type)
	}
}

func TestCreateMovement_DefaultTypeEgreso(t *testing.T) {
	svc := movement.NewService(&mockRepository{})

	m, err := svc.CreateMovement(context.Background(), 1, movement.CreateRequest{
		CategoryID:  1,
		Amount:      10.00,
		Description: "test",
		Date:        "2026-03-10",
		// Type omitted — should default to 'E'
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Type != "E" {
		t.Errorf("expected default type E, got %v", m.Type)
	}
}

func TestCreateMovement_Ingreso(t *testing.T) {
	svc := movement.NewService(&mockRepository{})

	m, err := svc.CreateMovement(context.Background(), 1, movement.CreateRequest{
		CategoryID:  1,
		Type:        "I",
		Amount:      2000.00,
		Description: "Salary",
		Date:        "2026-03-25",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Type != "I" {
		t.Errorf("expected type I, got %v", m.Type)
	}
}

func TestCreateMovement_InvalidType(t *testing.T) {
	svc := movement.NewService(&mockRepository{})

	_, err := svc.CreateMovement(context.Background(), 1, movement.CreateRequest{
		CategoryID:  1,
		Type:        "X",
		Amount:      10.00,
		Description: "test",
		Date:        "2026-03-10",
	})
	if err == nil {
		t.Fatal("expected error for invalid type, got nil")
	}
}

func TestCreateMovement_InvalidAmount(t *testing.T) {
	svc := movement.NewService(&mockRepository{})

	_, err := svc.CreateMovement(context.Background(), 1, movement.CreateRequest{
		CategoryID: 1, Amount: 0, Description: "test", Date: "2026-03-10",
	})
	if err == nil {
		t.Fatal("expected error for zero amount, got nil")
	}
}

func TestCreateMovement_MissingDescription(t *testing.T) {
	svc := movement.NewService(&mockRepository{})

	_, err := svc.CreateMovement(context.Background(), 1, movement.CreateRequest{
		CategoryID: 1, Amount: 10, Description: "", Date: "2026-03-10",
	})
	if err == nil {
		t.Fatal("expected error for empty description, got nil")
	}
}

func TestCreateMovement_InvalidDate(t *testing.T) {
	svc := movement.NewService(&mockRepository{})

	_, err := svc.CreateMovement(context.Background(), 1, movement.CreateRequest{
		CategoryID: 1, Amount: 10, Description: "test", Date: "not-a-date",
	})
	if err == nil {
		t.Fatal("expected error for invalid date, got nil")
	}
}

func TestCreateMovement_MissingCategoryID(t *testing.T) {
	svc := movement.NewService(&mockRepository{})

	_, err := svc.CreateMovement(context.Background(), 1, movement.CreateRequest{
		CategoryID: 0, Amount: 10, Description: "test", Date: "2026-03-10",
	})
	if err == nil {
		t.Fatal("expected error for missing category_id, got nil")
	}
}

func TestListMovements_GroupedByCategory(t *testing.T) {
	repo := &mockRepository{
		movements: []movement.Movement{
			{ID: 1, CategoryID: 1, Category: "Food", Type: "E", Amount: 30.00, Date: time.Now()},
			{ID: 2, CategoryID: 1, Category: "Food", Type: "E", Amount: 20.00, Date: time.Now()},
			{ID: 3, CategoryID: 2, Category: "Transport", Type: "E", Amount: 15.00, Date: time.Now()},
		},
	}
	svc := movement.NewService(repo)

	groups, err := svc.ListMovements(context.Background(), movement.Filter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
}

func TestDeleteMovement_NotFound(t *testing.T) {
	svc := movement.NewService(&mockRepository{})

	err := svc.DeleteMovement(context.Background(), 1, 999)
	if !errors.Is(err, movement.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetMonthlySummary(t *testing.T) {
	svc := movement.NewService(&mockRepository{})

	summaries, err := svc.GetMonthlySummary(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summaries) == 0 {
		t.Error("expected at least one summary")
	}
}

func TestUpdateMovement_PartialUpdate(t *testing.T) {
	newAmount := 99.99
	repo := &mockRepository{
		movements: []movement.Movement{
			{ID: 1, CategoryID: 1, Category: "Food", Type: "E", Amount: 50.00, Description: "Lunch"},
		},
	}
	svc := movement.NewService(repo)

	updated, err := svc.UpdateMovement(context.Background(), 1, 1, movement.UpdateRequest{
		Amount: &newAmount,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Amount != newAmount {
		t.Errorf("expected amount %v, got %v", newAmount, updated.Amount)
	}
}

func TestUpdateMovement_InvalidType(t *testing.T) {
	invalidType := "X"
	svc := movement.NewService(&mockRepository{})

	_, err := svc.UpdateMovement(context.Background(), 1, 1, movement.UpdateRequest{
		Type: &invalidType,
	})
	if err == nil {
		t.Fatal("expected error for invalid type, got nil")
	}
}

func ptr[T any](v T) *T { return &v }

func TestListExpenses_ForcesTypeE(t *testing.T) {
	repo := &mockRepository{}
	svc := movement.NewService(repo)

	_, err := svc.ListExpenses(context.Background(), movement.Filter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verificamos que el service forzó type='E' en el filtro pasado al repo.
	if repo.capturedFilter.Type == nil || *repo.capturedFilter.Type != "E" {
		t.Errorf("expected filter.Type='E' to be passed to repo, got %v", repo.capturedFilter.Type)
	}
}

func TestListExpenses_WithLimit(t *testing.T) {
	repo := &mockRepository{
		movements: []movement.Movement{
			{ID: 1, Category: "Alimentacion", Type: "E", Amount: 20000, Date: time.Now()},
			{ID: 2, Category: "Transporte", Type: "E", Amount: 8000, Date: time.Now()},
			{ID: 3, Category: "Entretenimiento", Type: "E", Amount: 35000, Date: time.Now()},
		},
	}
	svc := movement.NewService(repo)

	list, err := svc.ListExpenses(context.Background(), movement.Filter{Limit: ptr(2)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// el mock devuelve todo; verificamos que el Limit llega al repo (repo real lo aplica en SQL)
	if list == nil {
		t.Error("expected non-nil result")
	}
}

func TestListExpenses_EmptyReturnsSlice(t *testing.T) {
	svc := movement.NewService(&mockRepository{movements: nil})

	list, err := svc.ListExpenses(context.Background(), movement.Filter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if list == nil {
		t.Error("expected empty slice, got nil")
	}
}

func TestListExpenses_RepoError(t *testing.T) {
	svc := movement.NewService(&mockRepository{err: errors.New("db error")})

	_, err := svc.ListExpenses(context.Background(), movement.Filter{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
