package users

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

var errBoom = errors.New("db down")

func newRepoMock(t *testing.T) (Repository, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("opening sqlmock: %v", err)
	}

	return NewMySQLRepository(db), mock, func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet sqlmock expectations: %v", err)
		}
		_ = db.Close()
	}
}

func TestActivate_MarksTheUser(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`UPDATE users SET activated = 1 WHERE id = \?`).
		WithArgs(7).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Activate(context.Background(), 7); err != nil {
		t.Fatalf("Activate returned error: %v", err)
	}
}

func TestActivate_NoRowsIsErrNoRecord(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	// An activation link for a deleted account matches nothing. That must be
	// reported, not swallowed as a successful activation.
	mock.ExpectExec(`UPDATE users SET activated = 1`).WithArgs(99).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := repo.Activate(context.Background(), 99); !errors.Is(err, ErrNoRecord) {
		t.Errorf("err = %v, want ErrNoRecord", err)
	}
}

func TestActivate_WrapsExecAndRowsAffectedErrors(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`UPDATE users SET activated = 1`).WillReturnError(errBoom)
	if err := repo.Activate(context.Background(), 7); err == nil {
		t.Error("expected an error when the update fails")
	}

	mock.ExpectExec(`UPDATE users SET activated = 1`).
		WillReturnResult(sqlmock.NewErrorResult(errBoom))
	if err := repo.Activate(context.Background(), 7); err == nil {
		t.Error("expected an error when RowsAffected fails")
	}
}

func TestCreate_PersistsTheHashedPasswordAndBackfillsTheID(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	u := &User{User: "john", Name: "John Doe", Email: "john@example.com", HashedPassword: []byte("hashed")}
	mock.ExpectExec(`INSERT INTO users \(username, name, email, hashed_password\)`).
		WithArgs("john", "John Doe", "john@example.com", []byte("hashed")).
		WillReturnResult(sqlmock.NewResult(7, 1))

	if err := repo.Create(context.Background(), u); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	// The caller needs the id to issue the activation token.
	if u.ID != 7 {
		t.Errorf("ID = %d, want the generated 7", u.ID)
	}
}

func TestCreate_DuplicateBecomesAGenericSentinel(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	// MySQL 1062 is the unique-constraint violation. It must be translated so
	// the raw error — which names the index and the offending value — never
	// reaches the client and cannot be used to enumerate accounts.
	mock.ExpectExec(`INSERT INTO users`).
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'john@example.com' for key 'users.email'"})

	err := repo.Create(context.Background(), &User{})
	if !errors.Is(err, ErrDuplicateUser) {
		t.Fatalf("err = %v, want ErrDuplicateUser", err)
	}
	if strings.Contains(err.Error(), "john@example.com") {
		t.Error("the duplicated value leaked into the error")
	}
	if strings.Contains(err.Error(), "users.email") {
		t.Error("the index name leaked into the error")
	}
}

func TestCreate_OtherDatabaseErrorsAreWrappedNotTranslated(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`INSERT INTO users`).
		WillReturnError(&mysql.MySQLError{Number: 1146, Message: "Table doesn't exist"})

	err := repo.Create(context.Background(), &User{})
	if errors.Is(err, ErrDuplicateUser) {
		t.Error("an unrelated MySQL error was reported as a duplicate")
	}
	if err == nil {
		t.Error("expected an error")
	}
}

func TestCreate_WrapsLastInsertIdError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`INSERT INTO users`).WillReturnResult(sqlmock.NewErrorResult(errBoom))

	if err := repo.Create(context.Background(), &User{}); err == nil {
		t.Fatal("expected an error when LastInsertId fails")
	}
}

func TestGetUserID_ReturnsTheIDWhenThePasswordMatches(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	hash, _ := bcrypt.GenerateFromPassword([]byte("supersecret"), bcrypt.MinCost)
	mock.ExpectQuery(`SELECT id, hashed_password FROM users WHERE email = \?`).
		WithArgs("john@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "hashed_password"}).AddRow(7, hash))

	got, err := repo.GetUserID(context.Background(), &User{Email: "john@example.com", Password: "supersecret"})
	if err != nil {
		t.Fatalf("GetUserID returned error: %v", err)
	}
	if got != 7 {
		t.Errorf("id = %d, want 7", got)
	}
}

func TestGetUserID_UnknownEmailIsInvalidCredentials(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	// Same sentinel as a wrong password, so the caller cannot tell an unknown
	// address from a bad password.
	mock.ExpectQuery(`SELECT id, hashed_password`).WillReturnError(sql.ErrNoRows)

	_, err := repo.GetUserID(context.Background(), &User{Email: "nobody@example.com"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestGetUserID_WrongPasswordFails(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	hash, _ := bcrypt.GenerateFromPassword([]byte("supersecret"), bcrypt.MinCost)
	mock.ExpectQuery(`SELECT id, hashed_password`).WithArgs("john@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "hashed_password"}).AddRow(7, hash))

	if _, err := repo.GetUserID(context.Background(), &User{Email: "john@example.com", Password: "wrong"}); err == nil {
		t.Fatal("expected an error for a wrong password")
	}
}

func TestGetUserID_PropagatesOtherQueryErrors(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`SELECT id, hashed_password`).WillReturnError(errBoom)

	_, err := repo.GetUserID(context.Background(), &User{Email: "john@example.com"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, ErrInvalidCredentials) {
		t.Error("an infrastructure error was reported as invalid credentials")
	}
}

func TestGetUserByEmail_WrapsOtherQueryErrors(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)FROM\s+users`).WillReturnError(errBoom)

	_, err := repo.GetUserByEmail(context.Background(), "john@example.com")
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, ErrNoRecord) {
		t.Error("an infrastructure error was reported as a missing record")
	}
}

func TestUpdatePassword_WrapsExecAndRowsAffectedErrors(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`UPDATE users SET hashed_password`).WillReturnError(errBoom)
	if err := repo.UpdatePassword(context.Background(), 7, []byte("x")); err == nil {
		t.Error("expected an error when the update fails")
	}

	mock.ExpectExec(`UPDATE users SET hashed_password`).
		WillReturnResult(sqlmock.NewErrorResult(errBoom))
	if err := repo.UpdatePassword(context.Background(), 7, []byte("x")); err == nil {
		t.Error("expected an error when RowsAffected fails")
	}
}
