package users

import (
	"context"
	"fmt"
)

type service struct {
	repo Repository
}

// NewService creates a new movement Service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}


type Service interface {
	InsertUser(ctx context.Context, req UserRequest) (error)
}


func (s *service) InsertUser(ctx context.Context, req UserRequest) (error) {
	err := req.Validate()
	if err !=nil {
		return fmt.Errorf("error creating user: %w", err)
	}

	user := User{
		User:     req.UserName,
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	}

	if err := user.Normalize(); err != nil {
		return fmt.Errorf("error creating user: %w", err)
	}

	return s.repo.Create(ctx, &user)
}


