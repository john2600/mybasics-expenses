package users

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetUserByEmail_ReturnsFullRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("opening sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewMySQLRepository(db)

	hashed := []byte("$2a$12$C6UzMDM.H6dfI/f/IKxGhu.abcdefghijklmnopqrstuvwx0123")
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)

	rows := sqlmock.NewRows([]string{"id", "username", "name", "email", "hashed_password", "created_at", "updated_at"}).
		AddRow(7, "john", "John Doe", "john@example.com", hashed, created, updated)

	mock.ExpectQuery(`(?s)SELECT .+ FROM\s+users\s+WHERE email = \?`).
		WithArgs("john@example.com").
		WillReturnRows(rows)

	got, err := repo.GetUserByEmail(context.Background(), "john@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail returned error: %v", err)
	}

	if got.ID != 7 {
		t.Errorf("ID = %d, want 7", got.ID)
	}
	if got.User != "john" {
		t.Errorf("User = %q, want %q", got.User, "john")
	}
	if got.Name != "John Doe" {
		t.Errorf("Name = %q, want %q", got.Name, "John Doe")
	}
	if got.Email != "john@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "john@example.com")
	}
	if string(got.HashedPassword) != string(hashed) {
		t.Errorf("HashedPassword = %q, want %q", got.HashedPassword, hashed)
	}
	if !got.Created.Equal(created) {
		t.Errorf("Created = %v, want %v", got.Created, created)
	}
	if !got.Updated.Equal(updated) {
		t.Errorf("Updated = %v, want %v", got.Updated, updated)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestUpdatePassword_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("opening sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewMySQLRepository(db)

	id := 7
	hashed := []byte("$2a$12$newlyhashedpasswordvalue0123456789abcdefghij")

	mock.ExpectExec(`(?s)UPDATE users SET hashed_password = \? WHERE id = \?`).
		WithArgs(hashed, id).
		WillReturnResult(sqlmock.NewResult(0, 1)) // 1 row affected

	if err := repo.UpdatePassword(context.Background(), id, hashed); err != nil {
		t.Fatalf("UpdatePassword returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestUpdatePassword_NoRowsAffected(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("opening sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewMySQLRepository(db)

	id := 999
	hashed := []byte("$2a$12$whatever")

	mock.ExpectExec(`(?s)UPDATE users SET hashed_password = \? WHERE id = \?`).
		WithArgs(hashed, id).
		WillReturnResult(sqlmock.NewResult(0, 0)) // no rows matched

	if err := repo.UpdatePassword(context.Background(), id, hashed); !errors.Is(err, ErrNoRecord) {
		t.Errorf("error = %v, want ErrNoRecord", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestGetUserByEmail_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("opening sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewMySQLRepository(db)

	mock.ExpectQuery(`(?s)SELECT .+ FROM\s+users\s+WHERE email = \?`).
		WithArgs("missing@example.com").
		WillReturnError(sql.ErrNoRows)

	_, err = repo.GetUserByEmail(context.Background(), "missing@example.com")
	if !errors.Is(err, ErrNoRecord) {
		t.Errorf("error = %v, want ErrNoRecord", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}
