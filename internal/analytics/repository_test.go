package analytics

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

// fromStr and toStr are what the repository binds: dates are formatted to
// YYYY-MM-DD strings rather than passed as time values.
const (
	fromStr = "2026-05-01"
	toStr   = "2026-07-31"
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

func trendRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"year", "month", "total", "count"})
}

func TestRepoSummary_ReturnsTotalsAndDelegatesTheBreakdownToTrend(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT COALESCE\(SUM\(amount\), 0\), COUNT\(\*\)`).
		WithArgs(7, fromStr, toStr).
		WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(1500.0, 12))
	// Summary reuses Trend for the per-month points rather than a second
	// aggregate query, so the trend query must follow.
	mock.ExpectQuery(`(?s)SELECT YEAR\(date\), MONTH\(date\), SUM\(amount\), COUNT`).
		WithArgs(7, fromStr, toStr).
		WillReturnRows(trendRows().AddRow(2026, 7, 900.0, 7))

	total, count, points, err := repo.Summary(context.Background(), 7, testFrom, testTo)
	if err != nil {
		t.Fatalf("Summary returned error: %v", err)
	}
	if total != 1500 || count != 12 {
		t.Errorf("got (%v, %d), want (1500, 12)", total, count)
	}
	if len(points) != 1 || points[0].Total != 900 {
		t.Errorf("points = %+v", points)
	}
}

func TestRepoSummary_WrapsTotalsError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT COALESCE`).WillReturnError(errStub)

	if _, _, _, err := repo.Summary(context.Background(), 7, testFrom, testTo); err == nil {
		t.Fatal("expected an error when the totals query fails")
	}
}

func TestRepoSummary_PropagatesTrendError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT COALESCE`).WithArgs(7, fromStr, toStr).
		WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(1.0, 1))
	mock.ExpectQuery(`(?s)SELECT YEAR`).WillReturnError(errStub)

	if _, _, _, err := repo.Summary(context.Background(), 7, testFrom, testTo); err == nil {
		t.Fatal("expected the trend error to propagate")
	}
}

func TestRepoByCategory_ScopesToTheUserAndOrdersBySpend(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)WHERE  m.user_id = \? AND m.type = 'E'.+GROUP  BY m.category_id`).
		WithArgs(7, fromStr, toStr).
		WillReturnRows(sqlmock.NewRows([]string{"category_id", "name", "total", "count"}).
			AddRow(2, "Alimentacion", 900.0, 7).
			AddRow(3, "Transporte", 600.0, 5))

	got, err := repo.ByCategory(context.Background(), 7, testFrom, testTo)
	if err != nil {
		t.Fatalf("ByCategory returned error: %v", err)
	}
	if len(got) != 2 || got[0].Category != "Alimentacion" {
		t.Errorf("got = %+v", got)
	}
}

func TestRepoByCategory_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)GROUP  BY m.category_id`).WillReturnError(errStub)

	if _, err := repo.ByCategory(context.Background(), 7, testFrom, testTo); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

func TestRepoByCategory_WrapsScanError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)GROUP  BY m.category_id`).WithArgs(7, fromStr, toStr).
		WillReturnRows(sqlmock.NewRows([]string{"category_id", "name", "total", "count"}).
			AddRow("x", "Alimentacion", 900.0, 7))

	if _, err := repo.ByCategory(context.Background(), 7, testFrom, testTo); err == nil {
		t.Fatal("expected a scan error for a malformed row")
	}
}

func TestRepoTrend_ReturnsMonthlyPoints(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT YEAR\(date\), MONTH\(date\), SUM\(amount\), COUNT`).
		WithArgs(7, fromStr, toStr).
		WillReturnRows(trendRows().AddRow(2026, 5, 100.0, 2).AddRow(2026, 7, 300.0, 4))

	got, err := repo.Trend(context.Background(), 7, testFrom, testTo)
	if err != nil {
		t.Fatalf("Trend returned error: %v", err)
	}
	if len(got) != 2 || got[1].Count != 4 {
		t.Errorf("got = %+v", got)
	}
}

func TestRepoTrend_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT YEAR`).WillReturnError(errStub)

	if _, err := repo.Trend(context.Background(), 7, testFrom, testTo); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

func TestRepoTrend_WrapsScanError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT YEAR`).WithArgs(7, fromStr, toStr).
		WillReturnRows(trendRows().AddRow("x", 5, 100.0, 2))

	if _, err := repo.Trend(context.Background(), 7, testFrom, testTo); err == nil {
		t.Fatal("expected a scan error for a malformed row")
	}
}

func TestRepoTopExpenses_BindsTheLimitAndFormatsDates(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	date := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)ORDER  BY m.amount DESC\s+LIMIT  \?`).
		WithArgs(7, fromStr, toStr, 5).
		WillReturnRows(sqlmock.NewRows([]string{"id", "date", "name", "description", "amount"}).
			AddRow(1, date, "Alimentacion", "RAPPI", 26900.0))

	got, err := repo.TopExpenses(context.Background(), 7, testFrom, testTo, 5)
	if err != nil {
		t.Fatalf("TopExpenses returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Date != "2026-07-15" {
		t.Errorf("Date = %q, want 2026-07-15", got[0].Date)
	}
}

func TestRepoTopExpenses_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)LIMIT  \?`).WillReturnError(errStub)

	if _, err := repo.TopExpenses(context.Background(), 7, testFrom, testTo, 5); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

func TestRepoTopExpenses_WrapsScanError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)LIMIT  \?`).WithArgs(7, fromStr, toStr, 5).
		WillReturnRows(sqlmock.NewRows([]string{"id", "date", "name", "description", "amount"}).
			AddRow("x", time.Now(), "c", "d", 1.0))

	if _, err := repo.TopExpenses(context.Background(), 7, testFrom, testTo, 5); err == nil {
		t.Fatal("expected a scan error for a malformed row")
	}
}

func TestRepoIncomeVsExpense_SplitsBothTypesPerMonth(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	// This query does not filter by type — it pivots both into one row per
	// month, so a missing CASE branch would silently zero one of the series.
	mock.ExpectQuery(`(?s)SUM\(CASE WHEN type = 'I'.+SUM\(CASE WHEN type = 'E'`).
		WithArgs(7, fromStr, toStr).
		WillReturnRows(sqlmock.NewRows([]string{"year", "month", "income", "expense"}).
			AddRow(2026, 7, 3000000.0, 900.0))

	got, err := repo.IncomeVsExpense(context.Background(), 7, testFrom, testTo)
	if err != nil {
		t.Fatalf("IncomeVsExpense returned error: %v", err)
	}
	if len(got) != 1 || got[0].Income != 3000000 || got[0].Expense != 900 {
		t.Errorf("got = %+v", got)
	}
}

func TestRepoIncomeVsExpense_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SUM\(CASE WHEN type`).WillReturnError(errStub)

	if _, err := repo.IncomeVsExpense(context.Background(), 7, testFrom, testTo); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

func TestRepoIncomeVsExpense_WrapsScanError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SUM\(CASE WHEN type`).WithArgs(7, fromStr, toStr).
		WillReturnRows(sqlmock.NewRows([]string{"year", "month", "income", "expense"}).
			AddRow("x", 7, 1.0, 1.0))

	if _, err := repo.IncomeVsExpense(context.Background(), 7, testFrom, testTo); err == nil {
		t.Fatal("expected a scan error for a malformed row")
	}
}
