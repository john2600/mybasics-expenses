package security

import "errors"

// ErrInvalidCredentials is returned by CreateAuthentication when authentication
// fails. It is deliberately generic and identical for a missing account and a
// wrong password, so it never reveals whether an email is registered (this
// avoids user enumeration). No internal detail is ever wrapped into it.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrUserNotFound is what a UserFinder returns when no user matches the email.
// It is internal to the auth flow; CreateAuthentication maps it to
// ErrInvalidCredentials so the caller can't distinguish it from a wrong password.
var ErrUserNotFound = errors.New("user not found")
