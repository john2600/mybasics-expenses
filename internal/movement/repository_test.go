package movement

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// newRepoMock wires a repository over a mocked *sql.DB. Every test asserts the
// expectations were met, so an unexecuted or unexpected query fails the test.
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

func movementRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "category_id", "name", "type", "amount",
		"description", "date", "hour", "created_at", "updated_at",
	})
}

var (
	testDate    = time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	testCreated = time.Date(2026, 7, 15, 10, 30, 0, 0, time.UTC)
)

// --- Create -----------------------------------------------------------------

func TestCreate_InsertsScopedToUserAndReturnsTheStoredRow(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	m := &Movement{CategoryID: 2, Type: "E", Amount: 42500, Description: "Weekly groceries", Date: testDate}

	// The user id must be the first bound argument: ownership is set by the
	// repository, never taken from the payload.
	mock.ExpectExec(`(?s)INSERT INTO movements`).
		WithArgs(7, int64(2), "E", 42500.0, "Weekly groceries", testDate, nil).
		WillReturnResult(sqlmock.NewResult(99, 1))

	mock.ExpectQuery(`(?s)SELECT .+ FROM movements m`).
		WithArgs(int64(99), 7).
		WillReturnRows(movementRows().AddRow(99, 2, "Alimentacion", "E", 42500.0,
			"Weekly groceries", testDate, nil, testCreated, testCreated))

	got, err := repo.Create(context.Background(), 7, m)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if got.ID != 99 {
		t.Errorf("ID = %d, want 99", got.ID)
	}
	if got.Category != "Alimentacion" {
		t.Errorf("Category = %q, want %q", got.Category, "Alimentacion")
	}
}

func TestCreate_WrapsInsertError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`(?s)INSERT INTO movements`).WillReturnError(errors.New("boom"))

	if _, err := repo.Create(context.Background(), 7, &Movement{}); err == nil {
		t.Fatal("expected an error when the insert fails")
	}
}

func TestCreate_WrapsLastInsertIdError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`(?s)INSERT INTO movements`).
		WillReturnResult(sqlmock.NewErrorResult(errors.New("no last id")))

	if _, err := repo.Create(context.Background(), 7, &Movement{}); err == nil {
		t.Fatal("expected an error when LastInsertId fails")
	}
}

// --- FindAll ----------------------------------------------------------------

func TestFindAll_AlwaysScopesByUserAndMapsRows(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	hour := "10:30:00"
	mock.ExpectQuery(`(?s)SELECT .+ FROM movements m.+WHERE m.user_id = \?`).
		WithArgs(7).
		WillReturnRows(movementRows().
			AddRow(1, 2, "Alimentacion", "E", 100.0, "a", testDate, hour, testCreated, testCreated).
			AddRow(2, 3, "Transporte", "E", 50.0, "b", testDate, nil, testCreated, testCreated))

	got, err := repo.FindAll(context.Background(), Filter{UserID: 7})
	if err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	// A NULL hour must come back as nil, not as the empty string.
	if got[0].Hour == nil || *got[0].Hour != hour {
		t.Errorf("Hour = %v, want %q", got[0].Hour, hour)
	}
	if got[1].Hour != nil {
		t.Errorf("Hour = %v, want nil for a NULL column", *got[1].Hour)
	}
}

func TestFindAll_AppendsEveryFilterAsABoundArgument(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	catID := int64(2)
	typ := "E"
	from := testDate
	to := testDate.AddDate(0, 1, 0)
	limit := 5

	// Order matters: user first, then each optional filter in declaration order.
	mock.ExpectQuery(`(?s)WHERE m.user_id = \? AND m.category_id = \? AND m.type = \? AND m.date >= \? AND m.date <= \?.+LIMIT 5`).
		WithArgs(7, catID, typ, from, to).
		WillReturnRows(movementRows())

	if _, err := repo.FindAll(context.Background(), Filter{
		UserID: 7, CategoryID: &catID, Type: &typ, DateFrom: &from, DateTo: &to, Limit: &limit,
	}); err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}
}

func TestFindAll_IgnoresNonPositiveLimit(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	zero := 0
	// No LIMIT clause must be emitted for a zero limit.
	mock.ExpectQuery(`(?s)ORDER BY m.date DESC, m.hour DESC\s*$`).
		WithArgs(7).
		WillReturnRows(movementRows())

	if _, err := repo.FindAll(context.Background(), Filter{UserID: 7, Limit: &zero}); err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}
}

func TestFindAll_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT`).WillReturnError(errors.New("boom"))

	if _, err := repo.FindAll(context.Background(), Filter{UserID: 7}); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

func TestFindAll_WrapsScanError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	// A row whose amount is not numeric fails Scan rather than being coerced.
	mock.ExpectQuery(`(?s)SELECT`).WithArgs(7).
		WillReturnRows(movementRows().
			AddRow(1, 2, "Alimentacion", "E", "not-a-number", "a", testDate, nil, testCreated, testCreated))

	if _, err := repo.FindAll(context.Background(), Filter{UserID: 7}); err == nil {
		t.Fatal("expected a scan error for a malformed row")
	}
}

// --- FindByID ---------------------------------------------------------------

func TestFindByID_ReturnsNilNilWhenMissing(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)WHERE m.id = \? AND m.user_id = \?`).
		WithArgs(int64(1), 7).
		WillReturnError(sql.ErrNoRows)

	got, err := repo.FindByID(context.Background(), 7, 1)
	// "Not found" is (nil, nil) here — the service turns it into a 404, so an
	// error would change the status the client sees.
	if err != nil {
		t.Fatalf("err = %v, want nil for a missing row", err)
	}
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestFindByID_AnotherUsersRowIsNotVisible(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	// The query is scoped by user id, so a row owned by someone else simply
	// does not match — this pins that the user id reaches the WHERE clause.
	mock.ExpectQuery(`(?s)WHERE m.id = \? AND m.user_id = \?`).
		WithArgs(int64(501), 999).
		WillReturnError(sql.ErrNoRows)

	got, err := repo.FindByID(context.Background(), 999, 501)
	if err != nil || got != nil {
		t.Errorf("got = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestFindByID_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT`).WillReturnError(errors.New("boom"))

	if _, err := repo.FindByID(context.Background(), 7, 1); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

// --- Update -----------------------------------------------------------------

func TestUpdate_BuildsSetClauseFromSuppliedFieldsOnly(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	amount := 999.0
	desc := "updated"

	// Only the two supplied fields appear, then id and user id close the args.
	mock.ExpectExec(`UPDATE movements SET amount = \?, description = \? WHERE id = \? AND user_id = \?`).
		WithArgs(amount, desc, int64(1), 7).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(`(?s)SELECT`).WithArgs(int64(1), 7).
		WillReturnRows(movementRows().AddRow(1, 2, "Alimentacion", "E", amount, desc, testDate, nil, testCreated, testCreated))

	got, err := repo.Update(context.Background(), 7, 1, UpdateRequest{Amount: &amount, Description: &desc})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if got.Amount != amount {
		t.Errorf("Amount = %v, want %v", got.Amount, amount)
	}
}

func TestUpdate_WithNoFieldsSkipsTheWriteEntirely(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	// No ExpectExec: an empty patch must not issue an UPDATE at all. The
	// deferred ExpectationsWereMet also catches an unexpected one.
	mock.ExpectQuery(`(?s)SELECT`).WithArgs(int64(1), 7).
		WillReturnRows(movementRows().AddRow(1, 2, "Alimentacion", "E", 10.0, "a", testDate, nil, testCreated, testCreated))

	if _, err := repo.Update(context.Background(), 7, 1, UpdateRequest{}); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
}

func TestUpdate_AcceptsEveryField(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	catID := int64(3)
	typ := "I"
	amount := 10.0
	desc := "d"
	date := "2026-08-01"
	hour := "09:00"

	mock.ExpectExec(`UPDATE movements SET category_id = \?, type = \?, amount = \?, description = \?, date = \?, hour = \?`).
		WithArgs(catID, typ, amount, desc, date, hour, int64(1), 7).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)SELECT`).WithArgs(int64(1), 7).WillReturnRows(movementRows().
		AddRow(1, catID, "Salario", typ, amount, desc, testDate, hour, testCreated, testCreated))

	if _, err := repo.Update(context.Background(), 7, 1, UpdateRequest{
		CategoryID: &catID, Type: &typ, Amount: &amount, Description: &desc, Date: &date, Hour: &hour,
	}); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
}

func TestUpdate_WrapsExecError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	amount := 1.0
	mock.ExpectExec(`UPDATE movements`).WillReturnError(errors.New("boom"))

	if _, err := repo.Update(context.Background(), 7, 1, UpdateRequest{Amount: &amount}); err == nil {
		t.Fatal("expected an error when the update fails")
	}
}

// --- Delete -----------------------------------------------------------------

func TestDelete_RemovesTheRow(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`DELETE FROM movements WHERE id = \? AND user_id = \?`).
		WithArgs(int64(1), 7).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Delete(context.Background(), 7, 1); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}

func TestDelete_ZeroRowsAffectedIsNotFound(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	// Deleting someone else's movement matches no rows. It must surface as
	// ErrNotFound (404), never as a silent success.
	mock.ExpectExec(`DELETE FROM movements`).
		WithArgs(int64(1), 999).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Delete(context.Background(), 999, 1)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDelete_WrapsExecError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`DELETE FROM movements`).WillReturnError(errors.New("boom"))

	if err := repo.Delete(context.Background(), 7, 1); err == nil {
		t.Fatal("expected an error when the delete fails")
	}
}

func TestDelete_WrapsRowsAffectedError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`DELETE FROM movements`).
		WillReturnResult(sqlmock.NewErrorResult(errors.New("no rows affected")))

	if err := repo.Delete(context.Background(), 7, 1); err == nil {
		t.Fatal("expected an error when RowsAffected fails")
	}
}

// --- MonthlySummary ---------------------------------------------------------

func TestMonthlySummary_AggregatesExpensesForTheUser(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT YEAR\(date\).+WHERE type='E' AND user_id = \?`).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"year", "month", "total"}).
			AddRow(2026, 8, 1500.0).
			AddRow(2026, 7, 900.0))

	got, err := repo.MonthlySummary(context.Background(), 7)
	if err != nil {
		t.Fatalf("MonthlySummary returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Year != 2026 || got[0].Month != 8 || got[0].Total != 1500.0 {
		t.Errorf("first row = %+v, want {2026 8 1500}", got[0])
	}
}

func TestMonthlySummary_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT YEAR`).WillReturnError(errors.New("boom"))

	if _, err := repo.MonthlySummary(context.Background(), 7); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

func TestMonthlySummary_WrapsScanError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT YEAR`).WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"year", "month", "total"}).
			AddRow("nope", 8, 1500.0))

	if _, err := repo.MonthlySummary(context.Background(), 7); err == nil {
		t.Fatal("expected a scan error for a malformed row")
	}
}

// errStub is a plain error used where the test only cares that it is not
// ErrNotFound, so the handler takes the generic branch.
var errStub = errors.New("service failed")
