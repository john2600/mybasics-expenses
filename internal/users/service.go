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
	InsertUser(ctx context.Context, req UserRequest) error
	ChangePassword(ctx context.Context, req ChangePasswordRequest) error
	Authenticate(ctx context.Context, user, password string) (int, error)
}

func (s *service) InsertUser(ctx context.Context, req UserRequest) error {
	err := req.Validate()
	if err != nil {
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

func (s *service) ChangePassword(ctx context.Context, req ChangePasswordRequest) error {
	err := req.Validate()
	if err != nil {
		return fmt.Errorf("error updating password: %w", err)
	}

	resp, err := s.repo.GetUserByEmail(ctx, req.LoginRequest.Email)

	if err != nil {
		return fmt.Errorf("Error updating password  %w", err)
	}

	user := UserPassword{
		ID:       resp.ID,
		Email:    resp.Email,
		Password: string(resp.HashedPassword),
	}

	// it needs to be hashed
	err = user.ComparePassword([]byte(req.LoginRequest.Password))
	if err != nil {
		return fmt.Errorf("password not coincidences  %w", err)
	}

	// update pasword
	//return s.repo.Create(ctx, &user)
	return nil
}

func (s *service) Authenticate(ctx context.Context, email, password string) (int, error) {
	user := &User{
		Email:    email,
		Password: password,
	}

	return s.repo.GetUserID(ctx, user)
}

func (s *service) Exists(id int) (bool, error) {
	return false, nil
}
