package users

import (
	"context"
	"errors"

	"github.com/jscodelab/mybasics-expenses/internal/data"
	"github.com/jscodelab/mybasics-expenses/internal/security"
)

// userFinder adapts the users repository to security.UserFinder, converting
// *User to *data.User so the security package can look up users by email without
// importing this package (which would create an import cycle).
type userFinder struct {
	repo Repository
}

// NewUserFinder returns a security.UserFinder backed by the users repository.
func NewUserFinder(repo Repository) security.UserFinder {
	return &userFinder{repo: repo}
}

func (f *userFinder) GetUserByEmail(ctx context.Context, email string) (*data.User, error) {
	u, err := f.repo.GetUserByEmail(ctx, email)
	if err != nil {
		// Translate the users-package "no record" sentinel into the security one
		// so the security package can detect it without importing users.
		if errors.Is(err, ErrNoRecord) {
			return nil, security.ErrUserNotFound
		}
		return nil, err
	}
	return &data.User{
		ID:             u.ID,
		Username:       u.User,
		Name:           u.Name,
		Email:          u.Email,
		Activated:      u.Activated,
		HashedPassword: u.HashedPassword,
		Password:       data.NewPassword(u.HashedPassword),
	}, nil
}
