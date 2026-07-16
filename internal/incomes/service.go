package incomes

import (
	"context"
	"errors"
)

// Service handles income config business logic.
type Service interface {
	GetConfig(ctx context.Context) (*Config, error)
	UpdateConfig(ctx context.Context, req UpdateRequest) (*Config, error)
}

type service struct {
	repo Repository
}

// NewService creates an incomes Service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetConfig(ctx context.Context) (*Config, error) {
	return s.repo.Get(ctx)
}

func (s *service) UpdateConfig(ctx context.Context, req UpdateRequest) (*Config, error) {
	if req.Amount != nil && *req.Amount < 0 {
		return nil, errors.New("amount must be greater than or equal to 0")
	}
	if req.CutDay != nil && (*req.CutDay < 1 || *req.CutDay > 28) {
		return nil, errors.New("cut_day must be between 1 and 28")
	}
	return s.repo.Update(ctx, req)
}
