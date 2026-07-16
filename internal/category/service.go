package category

import (
	"context"
	"errors"
)

// Service defines the business logic for categories.
type Service interface {
	ListCategories(ctx context.Context) ([]Category, error)
	GetCategory(ctx context.Context, id int64) (*Category, error)
	CreateCategory(ctx context.Context, req CreateRequest) (*Category, error)
	UpdateCategory(ctx context.Context, id int64, req UpdateRequest) (*Category, error)
	DeleteCategory(ctx context.Context, id int64) error
}

type service struct {
	repo Repository
}

// NewService creates a new category Service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) ListCategories(ctx context.Context) ([]Category, error) {
	return s.repo.FindAll(ctx)
}

func (s *service) GetCategory(ctx context.Context, id int64) (*Category, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *service) CreateCategory(ctx context.Context, req CreateRequest) (*Category, error) {
	if req.Name == "" {
		return nil, errors.New("name is required")
	}

	c := &Category{
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
	}

	return s.repo.Create(ctx, c)
}

func (s *service) UpdateCategory(ctx context.Context, id int64, req UpdateRequest) (*Category, error) {
	return s.repo.Update(ctx, id, req)
}

func (s *service) DeleteCategory(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
