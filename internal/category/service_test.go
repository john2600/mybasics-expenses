package category_test

import (
	"context"
	"testing"

	"github.com/jscodelab/mybasics-expenses/internal/category"
)

type mockRepository struct {
	categories []category.Category
	err        error
	nextID     int64
}

func (m *mockRepository) FindAll(_ context.Context) ([]category.Category, error) {
	return m.categories, m.err
}

func (m *mockRepository) FindByID(_ context.Context, id int64) (*category.Category, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, c := range m.categories {
		if c.ID == id {
			return &c, nil
		}
	}
	return nil, nil
}

func (m *mockRepository) Create(_ context.Context, c *category.Category) (*category.Category, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.nextID++
	c.ID = m.nextID
	m.categories = append(m.categories, *c)
	return c, nil
}

func (m *mockRepository) Update(_ context.Context, id int64, req category.UpdateRequest) (*category.Category, error) {
	if m.err != nil {
		return nil, m.err
	}
	for i, c := range m.categories {
		if c.ID == id {
			if req.Name != nil {
				m.categories[i].Name = *req.Name
			}
			if req.Description != nil {
				m.categories[i].Description = *req.Description
			}
			if req.Color != nil {
				m.categories[i].Color = *req.Color
			}
			updated := m.categories[i]
			return &updated, nil
		}
	}
	return nil, nil
}

func (m *mockRepository) Delete(_ context.Context, id int64) error {
	if m.err != nil {
		return m.err
	}
	for i, c := range m.categories {
		if c.ID == id {
			m.categories = append(m.categories[:i], m.categories[i+1:]...)
			return nil
		}
	}
	return category.ErrNotFound
}

func TestListCategories(t *testing.T) {
	repo := &mockRepository{
		categories: []category.Category{
			{ID: 1, Name: "Food & Dining"},
			{ID: 2, Name: "Transportation"},
		},
	}
	svc := category.NewService(repo)

	cats, err := svc.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cats) != 2 {
		t.Errorf("expected 2 categories, got %d", len(cats))
	}
}

func TestGetCategory_Found(t *testing.T) {
	repo := &mockRepository{
		categories: []category.Category{
			{ID: 1, Name: "Food & Dining"},
		},
	}
	svc := category.NewService(repo)

	cat, err := svc.GetCategory(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat == nil {
		t.Fatal("expected category, got nil")
	}
	if cat.Name != "Food & Dining" {
		t.Errorf("expected name 'Food & Dining', got %q", cat.Name)
	}
}

func TestGetCategory_NotFound(t *testing.T) {
	svc := category.NewService(&mockRepository{})

	cat, err := svc.GetCategory(context.Background(), 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat != nil {
		t.Errorf("expected nil category, got %+v", cat)
	}
}

func TestCreateCategory_Success(t *testing.T) {
	svc := category.NewService(&mockRepository{})

	cat, err := svc.CreateCategory(context.Background(), category.CreateRequest{
		Name:        "Health",
		Description: "Medical and wellness",
		Color:       "#00FF00",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if cat.Name != "Health" {
		t.Errorf("expected name 'Health', got %q", cat.Name)
	}
}

func TestCreateCategory_MissingName(t *testing.T) {
	svc := category.NewService(&mockRepository{})

	_, err := svc.CreateCategory(context.Background(), category.CreateRequest{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestUpdateCategory_Success(t *testing.T) {
	repo := &mockRepository{
		categories: []category.Category{{ID: 1, Name: "Food"}},
	}
	svc := category.NewService(repo)

	newName := "Food & Dining"
	cat, err := svc.UpdateCategory(context.Background(), 1, category.UpdateRequest{Name: &newName})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat.Name != newName {
		t.Errorf("expected name %q, got %q", newName, cat.Name)
	}
}

func TestUpdateCategory_NotFound(t *testing.T) {
	svc := category.NewService(&mockRepository{})

	cat, err := svc.UpdateCategory(context.Background(), 999, category.UpdateRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat != nil {
		t.Errorf("expected nil, got %+v", cat)
	}
}

func TestDeleteCategory_Success(t *testing.T) {
	repo := &mockRepository{
		categories: []category.Category{{ID: 1, Name: "Food"}},
	}
	svc := category.NewService(repo)

	if err := svc.DeleteCategory(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteCategory_NotFound(t *testing.T) {
	svc := category.NewService(&mockRepository{})

	err := svc.DeleteCategory(context.Background(), 999)
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
}
