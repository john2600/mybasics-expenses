package incomes

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

var errStub = errors.New("stub failure")

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

func configRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"year_month", "amount", "cut_day", "description", "created_at"})
}

var july2026 = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

func TestGet_ReturnsTheMostRecentEntry(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery("(?s)WHERE user_id = .+ORDER BY .+DESC LIMIT 1").
		WithArgs(7).
		WillReturnRows(configRows().AddRow(july2026, 3000000.0, 24, "Salario", july2026))

	got, err := repo.Get(context.Background(), 7)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Amount != 3000000 || got.CutDay != 24 {
		t.Errorf("got = %+v", got)
	}
}

func TestGet_NoRowsFallsBackToANeutralDefault(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery("(?s)WHERE user_id").WithArgs(7).WillReturnError(sql.ErrNoRows)

	got, err := repo.Get(context.Background(), 7)
	// A user with no config must get a usable default, not an error: balance
	// and reports call this on every request.
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got.Amount != 0 {
		t.Errorf("Amount = %v, want 0", got.Amount)
	}
	if got.CutDay != 24 {
		t.Errorf("CutDay = %d, want the default 24", got.CutDay)
	}
	if got.YearMonth.Day() != 1 {
		t.Errorf("YearMonth = %v, want the first day of a month", got.YearMonth)
	}
}

func TestGet_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery("(?s)WHERE user_id").WithArgs(7).WillReturnError(errStub)

	if _, err := repo.Get(context.Background(), 7); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

func TestGetForMonth_PicksTheEntryEffectiveForThatMonth(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	// The versioned config means the row for a past month is the one still in
	// force until a newer one exists — that is what keeps old balances stable.
	mock.ExpectQuery("(?s)`year_month` <= \\?").
		WithArgs(7, july2026).
		WillReturnRows(configRows().AddRow(july2026, 2500000.0, 15, "Salario", july2026))

	got, err := repo.GetForMonth(context.Background(), 7, july2026.AddDate(0, 0, 20))
	if err != nil {
		t.Fatalf("GetForMonth returned error: %v", err)
	}
	if got.CutDay != 15 {
		t.Errorf("CutDay = %d, want 15", got.CutDay)
	}
}

func TestGetForMonth_NoEntryBeforeThatMonthFallsBackToLatest(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery("(?s)`year_month` <= \\?").WithArgs(7, july2026).WillReturnError(sql.ErrNoRows)
	// Falls through to Get, which finds the latest entry instead.
	mock.ExpectQuery("(?s)WHERE user_id = .+ORDER BY").WithArgs(7).
		WillReturnRows(configRows().AddRow(july2026, 100.0, 24, "later", july2026))

	got, err := repo.GetForMonth(context.Background(), 7, july2026)
	if err != nil {
		t.Fatalf("GetForMonth returned error: %v", err)
	}
	if got.Amount != 100 {
		t.Errorf("Amount = %v, want the fallback entry", got.Amount)
	}
}

func TestGetForMonth_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery("(?s)`year_month` <= \\?").WillReturnError(errStub)

	if _, err := repo.GetForMonth(context.Background(), 7, july2026); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

func TestUpdate_PatchesOnTopOfTheCurrentConfig(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	amount := 4000000.0
	ym := "2026-07"

	// Reads the base entry first, so fields absent from the patch keep their
	// previous values instead of resetting to zero.
	mock.ExpectQuery("(?s)WHERE user_id = .+ORDER BY").WithArgs(7).
		WillReturnRows(configRows().AddRow(july2026, 3000000.0, 15, "Salario", july2026))
	mock.ExpectExec("(?s)INSERT INTO income_config_history").
		WithArgs(7, july2026, amount, 15, "Salario").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("(?s)`year_month` <= \\?").WithArgs(7, july2026).
		WillReturnRows(configRows().AddRow(july2026, amount, 15, "Salario", july2026))

	got, err := repo.Update(context.Background(), 7, UpdateRequest{Amount: &amount, YearMonth: &ym})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if got.Amount != amount {
		t.Errorf("Amount = %v, want %v", got.Amount, amount)
	}
	if got.CutDay != 15 {
		t.Errorf("CutDay = %d, want the preserved 15", got.CutDay)
	}
}

func TestUpdate_AppliesEveryField(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	amount, cutDay, desc, ym := 1.0, 5, "new", "2026-07"

	mock.ExpectQuery("(?s)WHERE user_id = .+ORDER BY").WithArgs(7).
		WillReturnRows(configRows().AddRow(july2026, 0.0, 24, "old", july2026))
	mock.ExpectExec("(?s)INSERT INTO income_config_history").
		WithArgs(7, july2026, amount, cutDay, desc).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("(?s)`year_month` <= \\?").WithArgs(7, july2026).
		WillReturnRows(configRows().AddRow(july2026, amount, cutDay, desc, july2026))

	if _, err := repo.Update(context.Background(), 7, UpdateRequest{
		Amount: &amount, CutDay: &cutDay, Description: &desc, YearMonth: &ym,
	}); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
}

func TestUpdate_DefaultsToTheCurrentMonth(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	now := time.Now()
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	amount := 50.0

	mock.ExpectQuery("(?s)WHERE user_id = .+ORDER BY").WithArgs(7).
		WillReturnRows(configRows().AddRow(thisMonth, 0.0, 24, "d", thisMonth))
	mock.ExpectExec("(?s)INSERT INTO income_config_history").
		WithArgs(7, thisMonth, amount, 24, "d").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("(?s)`year_month` <= \\?").WithArgs(7, thisMonth).
		WillReturnRows(configRows().AddRow(thisMonth, amount, 24, "d", thisMonth))

	if _, err := repo.Update(context.Background(), 7, UpdateRequest{Amount: &amount}); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
}

func TestUpdate_RejectsAMalformedYearMonth(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery("(?s)WHERE user_id = .+ORDER BY").WithArgs(7).
		WillReturnRows(configRows().AddRow(july2026, 0.0, 24, "d", july2026))

	bad := "07-2026"
	_, err := repo.Update(context.Background(), 7, UpdateRequest{YearMonth: &bad})
	if err == nil {
		t.Fatal("expected an error for a malformed year_month")
	}
}

func TestUpdate_PropagatesBaseReadError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery("(?s)WHERE user_id").WillReturnError(errStub)

	if _, err := repo.Update(context.Background(), 7, UpdateRequest{}); err == nil {
		t.Fatal("expected the base read error to propagate")
	}
}

func TestUpdate_WrapsUpsertError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery("(?s)WHERE user_id = .+ORDER BY").WithArgs(7).
		WillReturnRows(configRows().AddRow(july2026, 0.0, 24, "d", july2026))
	mock.ExpectExec("(?s)INSERT INTO income_config_history").WillReturnError(errStub)

	if _, err := repo.Update(context.Background(), 7, UpdateRequest{}); err == nil {
		t.Fatal("expected an error when the upsert fails")
	}
}

func TestParseYearMonth(t *testing.T) {
	got, err := parseYearMonth("2026-07")
	if err != nil {
		t.Fatalf("parseYearMonth returned error: %v", err)
	}
	if !got.Equal(july2026) {
		t.Errorf("got = %v, want %v", got, july2026)
	}

	if _, err := parseYearMonth("nope"); err == nil {
		t.Error("expected an error for a malformed value")
	}
}
