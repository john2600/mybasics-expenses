package movement

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Service defines the business logic for movements. Every operation is scoped
// to a single user via userID (for lists, it travels inside Filter.UserID).
type Service interface {
	CreateMovement(ctx context.Context, userID int, req CreateRequest) (*Movement, error)
	ListMovements(ctx context.Context, f Filter) ([]GroupedByCategory, error)
	ListExpenses(ctx context.Context, f Filter) ([]Movement, error)
	GetMovement(ctx context.Context, userID int, id int64) (*Movement, error)
	UpdateMovement(ctx context.Context, userID int, id int64, req UpdateRequest) (*Movement, error)
	DeleteMovement(ctx context.Context, userID int, id int64) error
	GetMonthlySummary(ctx context.Context, userID int) ([]MonthlySummary, error)
}

type service struct {
	repo Repository
}

// NewService creates a new movement Service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreateMovement(ctx context.Context, userID int, req CreateRequest) (*Movement, error) {
	if req.Amount <= 0 {
		return nil, errors.New("amount must be greater than zero")
	}
	if req.Description == "" {
		return nil, errors.New("description is required")
	}
	if req.CategoryID == 0 {
		return nil, errors.New("category_id is required")
	}
	if req.Type == "" {
		req.Type = "E"
	}
	if req.Type != "I" && req.Type != "E" {
		return nil, errors.New("type must be 'I' (ingreso) or 'E' (egreso)")
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, fmt.Errorf("invalid date format, expected YYYY-MM-DD: %w", err)
	}

	m := &Movement{
		CategoryID:  req.CategoryID,
		Type:        req.Type,
		Amount:      req.Amount,
		Description: req.Description,
		Date:        date,
	}
	if req.Hour != "" {
		m.Hour = &req.Hour
	}

	return s.repo.Create(ctx, userID, m)
}

func (s *service) ListMovements(ctx context.Context, f Filter) ([]GroupedByCategory, error) {
	movements, err := s.repo.FindAll(ctx, f)
	if err != nil {
		return nil, err
	}

	groupMap := make(map[string]*GroupedByCategory)
	for _, m := range movements {
		g, ok := groupMap[m.Category]
		if !ok {
			g = &GroupedByCategory{Category: m.Category}
			groupMap[m.Category] = g
		}
		g.Movements = append(g.Movements, m)
		g.Total += m.Amount
	}

	result := make([]GroupedByCategory, 0, len(groupMap))
	for _, g := range groupMap {
		result = append(result, *g)
	}

	return result, nil
}

func (s *service) ListExpenses(ctx context.Context, f Filter) ([]Movement, error) {
	typeE := "E"
	f.Type = &typeE
	movements, err := s.repo.FindAll(ctx, f)
	if err != nil {
		return nil, err
	}
	if movements == nil {
		return []Movement{}, nil
	}
	return movements, nil
}

func (s *service) GetMovement(ctx context.Context, userID int, id int64) (*Movement, error) {
	return s.repo.FindByID(ctx, userID, id)
}

func (s *service) UpdateMovement(ctx context.Context, userID int, id int64, req UpdateRequest) (*Movement, error) {
	if req.Type != nil && *req.Type != "I" && *req.Type != "E" {
		return nil, errors.New("type must be 'I' (ingreso) or 'E' (egreso)")
	}
	return s.repo.Update(ctx, userID, id, req)
}

func (s *service) DeleteMovement(ctx context.Context, userID int, id int64) error {
	return s.repo.Delete(ctx, userID, id)
}

func (s *service) GetMonthlySummary(ctx context.Context, userID int) ([]MonthlySummary, error) {
	return s.repo.MonthlySummary(ctx, userID)
}
