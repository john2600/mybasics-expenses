package mailer

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// closedPort returns a port that nothing is listening on, so a dial attempt
// fails immediately instead of hanging or reaching a real SMTP server.
func closedPort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func testConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		Host:     "127.0.0.1",
		Port:     closedPort(t),
		Username: "user",
		Password: "pass",
		From:     "MyBasics <no-reply@mybasics.local>",
	}
}

func TestNew_BuildsAClientWithoutDialling(t *testing.T) {
	// New must not touch the network: the app has to boot even with no SMTP
	// reachable, and a missing credential only fails later, on Send.
	m, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if m == nil {
		t.Fatal("New returned a nil Mailer")
	}
}

func TestNew_RejectsAnEmptyHost(t *testing.T) {
	if _, err := New(Config{Port: 587}); err == nil {
		t.Error("expected an error for an empty host")
	}
}

func TestSend_RejectsAnInvalidFromBeforeDialling(t *testing.T) {
	cfg := testConfig(t)
	cfg.From = "not a valid address"
	m, err := New(cfg)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	start := time.Now()
	err = m.Send(context.Background(), "john@example.com", "subject", "body")
	if err == nil {
		t.Fatal("expected an error for a malformed From")
	}
	if !strings.Contains(err.Error(), "invalid From") {
		t.Errorf("err = %v, want it to name the From header", err)
	}
	// It must fail on the header, not after three dial attempts with backoff.
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("took %v — it dialled before validating the envelope", elapsed)
	}
}

func TestSend_RejectsAnInvalidRecipient(t *testing.T) {
	m, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	err = m.Send(context.Background(), "not a valid address", "subject", "body")
	if err == nil {
		t.Fatal("expected an error for a malformed recipient")
	}
	if !strings.Contains(err.Error(), "invalid To") {
		t.Errorf("err = %v, want it to name the To header", err)
	}
}

func TestSend_RetriesThenGivesUpNamingTheAttempts(t *testing.T) {
	m, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	err = m.Send(context.Background(), "john@example.com", "subject", "body")
	if err == nil {
		t.Fatal("expected an error when nothing is listening")
	}
	// The message says how many attempts were made, which is what makes a
	// transient outage distinguishable in the logs.
	if !strings.Contains(err.Error(), "3 attempts") {
		t.Errorf("err = %v, want it to mention the attempt count", err)
	}
	if !strings.Contains(err.Error(), "john@example.com") {
		t.Errorf("err = %v, want it to name the recipient", err)
	}
}

func TestSend_AbortsEarlyWhenTheContextIsCancelled(t *testing.T) {
	m, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err = m.Send(ctx, "john@example.com", "subject", "body")
	if err == nil {
		t.Fatal("expected an error")
	}
	// With the caller gone there is no point sleeping through the backoff; it
	// must return without waiting out the remaining attempts.
	if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
		t.Errorf("took %v — it slept through the backoff despite the cancelled context", elapsed)
	}
}
