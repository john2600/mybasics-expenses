package balance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

var (
	errStub = errors.New("stub failure")
	from    = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to      = time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
)

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

func TestGetSummary_ComputesBalanceAsIncomesMinusExpenses(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT.+FROM movements\s+WHERE user_id = \?`).
		WithArgs(7, from, from, to, to).
		WillReturnRows(sqlmock.NewRows([]string{"expenses", "incomes"}).AddRow(400.0, 1000.0))

	got, err := repo.GetSummary(context.Background(), 7, &from, &to)
	if err != nil {
		t.Fatalf("GetSummary returned error: %v", err)
	}
	if got.Balance != 600 {
		t.Errorf("Balance = %v, want 600 (1000 - 400)", got.Balance)
	}
	if got.Expenses != 400 || got.Incomes != 1000 {
		t.Errorf("got = %+v", got)
	}
}

func TestGetSummary_NilDatesBindAsNullAndSelectEverything(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	// The query is written so a NULL bound date disables that side of the
	// range; a nil pointer must reach the driver as NULL, not as a zero time.
	mock.ExpectQuery(`(?s)SELECT.+FROM movements`).
		WithArgs(7, nil, nil, nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"expenses", "incomes"}).AddRow(0.0, 0.0))

	got, err := repo.GetSummary(context.Background(), 7, nil, nil)
	if err != nil {
		t.Fatalf("GetSummary returned error: %v", err)
	}
	if got.Balance != 0 {
		t.Errorf("Balance = %v, want 0", got.Balance)
	}
}

func TestGetSummary_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT`).WillReturnError(errStub)

	if _, err := repo.GetSummary(context.Background(), 7, nil, nil); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

func TestGetPeriodSummary_ReturnsExpensesAndIncomes(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	// Half-open range: >= from and < to, so a movement on the cut day belongs
	// to exactly one period and is never counted twice.
	mock.ExpectQuery(`(?s)WHERE user_id = \? AND date >= \? AND date < \?`).
		WithArgs(7, from, to).
		WillReturnRows(sqlmock.NewRows([]string{"expenses", "incomes"}).AddRow(250.0, 900.0))

	expenses, incomes, err := repo.GetPeriodSummary(context.Background(), 7, from, to)
	if err != nil {
		t.Fatalf("GetPeriodSummary returned error: %v", err)
	}
	if expenses != 250 || incomes != 900 {
		t.Errorf("got = (%v, %v), want (250, 900)", expenses, incomes)
	}
}

func TestGetPeriodSummary_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT`).WillReturnError(errStub)

	if _, _, err := repo.GetPeriodSummary(context.Background(), 7, from, to); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

func TestGetEarliestMovementDate_ReturnsTheDate(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`SELECT MIN\(date\) FROM movements WHERE user_id = \?`).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"min"}).AddRow(from))

	got, err := repo.GetEarliestMovementDate(context.Background(), 7)
	if err != nil {
		t.Fatalf("GetEarliestMovementDate returned error: %v", err)
	}
	if got == nil || !got.Equal(from) {
		t.Errorf("got = %v, want %v", got, from)
	}
}

func TestGetEarliestMovementDate_NoMovementsIsNilNotAnError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	// MIN() over no rows returns a single NULL row, not zero rows. A new user
	// with no movements must get nil so the caller skips period generation
	// rather than failing the whole request.
	mock.ExpectQuery(`SELECT MIN\(date\)`).WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"min"}).AddRow(nil))

	got, err := repo.GetEarliestMovementDate(context.Background(), 7)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestGetEarliestMovementDate_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`SELECT MIN\(date\)`).WillReturnError(errStub)

	if _, err := repo.GetEarliestMovementDate(context.Background(), 7); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}
