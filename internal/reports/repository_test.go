package reports

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

var (
	errStub  = errors.New("stub failure")
	testFrom = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	testTo   = time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
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

func TestQueryExpenses_ScopesToTheUserAndFormatsDates(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	date := time.Date(2026, 7, 15, 10, 30, 0, 0, time.UTC)

	// Dates are bound as YYYY-MM-DD strings, not as time values, so the range
	// matches how the column is compared.
	mock.ExpectQuery(`(?s)WHERE  m.user_id = \? AND m.type = 'E'`).
		WithArgs(7, "2026-05-01", "2026-07-31").
		WillReturnRows(sqlmock.NewRows([]string{"id", "date", "name", "description", "amount"}).
			AddRow(1, date, "Alimentacion", "RAPPI", 26900.0))

	got, err := repo.QueryExpenses(context.Background(), 7, testFrom, testTo)
	if err != nil {
		t.Fatalf("QueryExpenses returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	// The row carries a date-only string, since that is what the export shows.
	if got[0].Date != "2026-07-15" {
		t.Errorf("Date = %q, want 2026-07-15", got[0].Date)
	}
	if got[0].Category != "Alimentacion" {
		t.Errorf("Category = %q", got[0].Category)
	}
}

func TestQueryExpenses_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT`).WillReturnError(errStub)

	if _, err := repo.QueryExpenses(context.Background(), 7, testFrom, testTo); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

func TestQueryExpenses_WrapsScanError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT`).WithArgs(7, "2026-05-01", "2026-07-31").
		WillReturnRows(sqlmock.NewRows([]string{"id", "date", "name", "description", "amount"}).
			AddRow("not-an-id", time.Now(), "c", "d", 1.0))

	if _, err := repo.QueryExpenses(context.Background(), 7, testFrom, testTo); err == nil {
		t.Fatal("expected a scan error for a malformed row")
	}
}

func TestMonthlySummary_LabelsEachMonth(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT YEAR\(date\).+GROUP  BY`).
		WithArgs(7, "2026-05-01", "2026-07-31").
		WillReturnRows(sqlmock.NewRows([]string{"year", "month", "total"}).
			AddRow(2026, 5, 100.0).
			AddRow(2026, 7, 300.0))

	got, err := repo.MonthlySummary(context.Background(), 7, testFrom, testTo)
	if err != nil {
		t.Fatalf("MonthlySummary returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	// The label is derived, not stored, so it is worth pinning.
	if got[0].Label != "May 2026" {
		t.Errorf("Label = %q, want %q", got[0].Label, "May 2026")
	}
	if got[1].Label != "Jul 2026" {
		t.Errorf("Label = %q, want %q", got[1].Label, "Jul 2026")
	}
}

func TestMonthlySummary_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT YEAR`).WillReturnError(errStub)

	if _, err := repo.MonthlySummary(context.Background(), 7, testFrom, testTo); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

func TestMonthlySummary_WrapsScanError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT YEAR`).WithArgs(7, "2026-05-01", "2026-07-31").
		WillReturnRows(sqlmock.NewRows([]string{"year", "month", "total"}).AddRow("x", 5, 100.0))

	if _, err := repo.MonthlySummary(context.Background(), 7, testFrom, testTo); err == nil {
		t.Fatal("expected a scan error for a malformed row")
	}
}

func TestMonthLabel(t *testing.T) {
	if got := monthLabel(2026, 1); got != "Jan 2026" {
		t.Errorf("monthLabel(2026, 1) = %q, want Jan 2026", got)
	}
	if got := monthLabel(2026, 12); got != "Dec 2026" {
		t.Errorf("monthLabel(2026, 12) = %q, want Dec 2026", got)
	}
}
