package incomes

import (
	"context"
	"errors"
)

// Service handles income config business logic. Every operation is scoped to a
// single user via userID.
type Service interface {
	GetConfig(ctx context.Context, userID int) (*Config, error)
	UpdateConfig(ctx context.Context, userID int, req UpdateRequest) (*Config, error)
}

type service struct {
	repo Repository
}

// NewService creates an incomes Service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetConfig(ctx context.Context, userID int) (*Config, error) {
	return s.repo.Get(ctx, userID)
}

func (s *service) UpdateConfig(ctx context.Context, userID int, req UpdateRequest) (*Config, error) {
	if req.Amount != nil && *req.Amount < 0 {
		return nil, errors.New("amount must be greater than or equal to 0")
	}
	if req.CutDay != nil && (*req.CutDay < 1 || *req.CutDay > 28) {
		return nil, errors.New("cut_day must be between 1 and 28")
	}
	return s.repo.Update(ctx, userID, req)
}
