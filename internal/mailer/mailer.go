// Package mailer is the outbound-email layer. It wraps github.com/wneessen/go-mail
// behind a small Mailer interface so the rest of the app depends on the
// abstraction, not on the concrete SMTP client.
//
// For local development it points at Mailtrap's sandbox SMTP
// (sandbox.smtp.mailtrap.io): a hosted "trap" server that captures every
// message in a web inbox instead of delivering it. No local mail server is
// required — only the SMTP_* environment variables and outbound network access.
package mailer

import (
	"context"
	"fmt"

	mail "github.com/wneessen/go-mail"
)

// Config holds the SMTP connection and envelope settings. For Mailtrap sandbox:
//
//	Host=sandbox.smtp.mailtrap.io  Port=587
//	Username / Password come from the Mailtrap inbox (SMTP Settings).
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string // header/envelope From, e.g. "MyBasics <no-reply@mybasics.local>"
}

// Mailer sends a plain-text email. It takes a context so callers can bound the
// SMTP dial+send with a timeout or cancellation.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// goMailer is the go-mail backed implementation of Mailer.
type goMailer struct {
	client *mail.Client
	from   string
}

// New builds a Mailer from cfg. The go-mail client defaults to STARTTLS
// (mandatory), which is what Mailtrap sandbox expects on port 587.
func New(cfg Config) (Mailer, error) {
	client, err := mail.NewClient(cfg.Host,
		mail.WithPort(cfg.Port),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(cfg.Username),
		mail.WithPassword(cfg.Password),
	)
	if err != nil {
		return nil, fmt.Errorf("mailer: creating smtp client: %w", err)
	}
	return &goMailer{client: client, from: cfg.From}, nil
}

// Send composes a plain-text message and delivers it over SMTP.
func (m *goMailer) Send(ctx context.Context, to, subject, body string) error {
	msg := mail.NewMsg()
	if err := msg.From(m.from); err != nil {
		return fmt.Errorf("mailer: invalid From %q: %w", m.from, err)
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("mailer: invalid To %q: %w", to, err)
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, body)

	if err := m.client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("mailer: sending to %q: %w", to, err)
	}
	return nil
}
