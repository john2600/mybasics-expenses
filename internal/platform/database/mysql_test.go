package database

import (
	"net"
	"strings"
	"testing"
)

// closedPort returns a port nothing is listening on, so the ping fails fast
// instead of hanging or reaching a real server.
func closedPort(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	defer func() { _ = l.Close() }()

	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatalf("splitting address: %v", err)
	}
	return port
}

func TestNewMySQL_FailsWhenTheServerIsUnreachable(t *testing.T) {
	// NewMySQL pings before returning, so a bad connection surfaces at startup
	// rather than on the first request. sql.Open alone would not catch this —
	// it is lazy and never dials.
	db, err := NewMySQL(Config{
		Host: "127.0.0.1", Port: closedPort(t),
		User: "root", Password: "secret", Name: "mybasics_expenses",
	})
	if err == nil {
		if db != nil {
			_ = db.Close()
		}
		t.Fatal("expected an error when nothing is listening")
	}
	if !strings.Contains(err.Error(), "pinging database") {
		t.Errorf("err = %v, want it to name the ping step", err)
	}
	if db != nil {
		t.Error("a non-nil pool was returned alongside the error")
	}
}

func TestNewMySQL_FailsOnAMalformedDSN(t *testing.T) {
	// An unbalanced parenthesis in the host breaks DSN parsing, which is the
	// one thing sql.Open itself rejects.
	db, err := NewMySQL(Config{
		Host: "bad(host", Port: "3306",
		User: "root", Password: "secret", Name: "mybasics_expenses",
	})
	if err == nil {
		if db != nil {
			_ = db.Close()
		}
		t.Fatal("expected an error for a malformed DSN")
	}
	if db != nil {
		t.Error("a non-nil pool was returned alongside the error")
	}
}
