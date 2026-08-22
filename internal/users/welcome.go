package users

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"log"
	"net/url"
	"text/template"
)

// welcomeTmplRaw is the welcome email body, embedded at build time so it ships
// inside the binary (works the same locally and in the Docker image).
//
//go:embed templates/user_welcome.tmpl
var welcomeTmplRaw string

// Parsed once at startup; template.Must panics on a malformed template, which is
// a programming error we want to catch immediately, not at request time.
var welcomeTmpl = template.Must(template.New("user_welcome").Parse(welcomeTmplRaw))

// welcomeSubject is hardcoded for now — this is a functionality test.
const welcomeSubject = "Welcome to MyBasics-Expenses 🎉"

// activationBaseURL is the public base URL used to build the activation link in
// the welcome email. TODO: make this configurable (env) instead of hardcoded.
const activationBaseURL = "http://localhost:8081"

type welcomeData struct {
	Name          string
	Username      string
	Email         string
	ActivationURL string
}

// sendWelcomeEmail renders the welcome template and sends it to the new user.
// It is best-effort and MUST NOT fail user creation: a nil mailer, a render
// error, or an SMTP error (e.g. missing Mailtrap credentials) is logged and
// swallowed. The caller creates the user first, then calls this.
func (s *service) sendWelcomeEmail(ctx context.Context, u *User, activationToken string) {
	if s.mailer == nil {
		return
	}

	activationURL := fmt.Sprintf("%s/api/v1/user/activate?id=%d&token=%s",
		activationBaseURL, u.ID, url.QueryEscape(activationToken))

	var body bytes.Buffer
	if err := welcomeTmpl.Execute(&body, welcomeData{
		Name:          u.Name,
		Username:      u.User,
		Email:         u.Email,
		ActivationURL: activationURL,
	}); err != nil {
		log.Printf("users: rendering welcome email for %q: %v", u.Email, err)
		return
	}

	if err := s.mailer.Send(ctx, u.Email, welcomeSubject, body.String()); err != nil {
		log.Printf("users: sending welcome email to %q: %v", u.Email, err)
		return
	}

	log.Printf("users: welcome email sent to %q", u.Email)
}
