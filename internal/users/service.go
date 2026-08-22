package users

import (
	"context"
	"fmt"
	"time"

	"github.com/jscodelab/mybasics-expenses/internal/mailer"
	"github.com/jscodelab/mybasics-expenses/internal/security"
)

type service struct {
	repo   Repository
	mailer mailer.Mailer
	tokens security.TokenService
}

// NewService creates a new user Service. The mailer is used to send the welcome
// email on registration; it may be nil (welcome email is skipped). tokens is
// used for the activation flow.
func NewService(repo Repository, m mailer.Mailer, tokens security.TokenService) Service {
	return &service{repo: repo, mailer: m, tokens: tokens}
}

type Service interface {
	InsertUser(ctx context.Context, req UserRequest) error
	ChangePassword(ctx context.Context, req ChangePasswordRequest) error
	Authenticate(ctx context.Context, user, password string) (int, error)
	ActiveUser(ctx context.Context, tokenPlainText string) error
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

	if err := s.repo.Create(ctx, &user); err != nil {
		return err
	}

	// Issue the activation token for the new user.
	token, err := s.tokens.New(ctx, user.ID, 3*24*time.Hour, security.ScopeActivation)
	if err != nil {
		return fmt.Errorf("error creating activation token: %w", err)
	}

	// Best-effort welcome email carrying the token — never fails the
	// registration. Async so the response isn't blocked; uses a fresh context
	// because the request ctx is cancelled as soon as InsertUser returns.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.sendWelcomeEmail(ctx, &user, token.Plaintext)
	}()

	return nil
}

func (s *service) ChangePassword(ctx context.Context, req ChangePasswordRequest) error {
	err := req.Validate()
	if err != nil {
		return fmt.Errorf("error updating password: %w", err)
	}

	// obtain the password
	resp, err := s.repo.GetUserByEmail(ctx, req.LoginRequest.Email)

	if err != nil {
		return fmt.Errorf("Error updating password  %w", err)
	}

	err = ComparePassword(resp.HashedPassword, req.LoginRequest.Password)
	if err != nil {
		return fmt.Errorf("password not coincidences  %w", err)
	}

	newPassword, err := EncriptPassword(string(req.NewPassword))
	if err != nil {
		return fmt.Errorf("error encripting password  %w", err)
	}

	return s.repo.UpdatePassword(ctx, resp.ID, newPassword)
}

func (s *service) Authenticate(ctx context.Context, email, password string) (int, error) {
	user := &User{
		Email:    email,
		Password: password,
	}

	return s.repo.GetUserID(ctx, user)
}

// ActiveUser activates the account tied to a valid activation token: it resolves
// the token to a user, marks the user activated, and invalidates the activation
// tokens so the link can't be reused.
func (s *service) ActiveUser(ctx context.Context, tokenPlain string) error {
	if tokenPlain == "" || len(tokenPlain) != 26 {
		return fmt.Errorf("token invalid")
	}

	user, err := s.tokens.GetForToken(ctx, security.ScopeActivation, tokenPlain)
	if err != nil {
		return fmt.Errorf("activating user: %w", err)
	}

	if err := s.repo.Activate(ctx, user.ID); err != nil {
		return fmt.Errorf("activating user: %w", err)
	}

	return s.tokens.DeleteAllTokensUser(ctx, security.ScopeActivation, user.ID)
}

func (s *service) Exists(id int) (bool, error) {
	return false, nil
}
