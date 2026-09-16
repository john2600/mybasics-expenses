package category

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

var (
	errStub  = errors.New("stub failure")
	testTime = time.Date(2026, 7, 15, 10, 30, 0, 0, time.UTC)
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

func categoryRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "name", "description", "color", "created_at", "updated_at"})
}

func TestFindAll_ReturnsCategoriesOrderedByName(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT .+ FROM categories\s+ORDER BY name ASC`).
		WillReturnRows(categoryRows().
			AddRow(1, "Alimentacion", "food", "#ff0000", testTime, testTime).
			AddRow(2, "Transporte", "travel", "#00ff00", testTime, testTime))

	got, err := repo.FindAll(context.Background())
	if err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Name != "Alimentacion" || got[0].Color != "#ff0000" {
		t.Errorf("first row = %+v", got[0])
	}
}

func TestFindAll_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT`).WillReturnError(errStub)

	if _, err := repo.FindAll(context.Background()); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

func TestFindAll_WrapsScanError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT`).
		WillReturnRows(categoryRows().AddRow("not-an-id", "x", "y", "z", testTime, testTime))

	if _, err := repo.FindAll(context.Background()); err == nil {
		t.Fatal("expected a scan error for a malformed row")
	}
}

func TestFindByID_MissingIsNilNil(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)WHERE id = \?`).WithArgs(int64(9)).WillReturnError(sql.ErrNoRows)

	got, err := repo.FindByID(context.Background(), 9)
	// The handler turns nil into a 404; an error here would become a 500.
	if err != nil || got != nil {
		t.Errorf("got = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestFindByID_ReturnsTheRow(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)WHERE id = \?`).WithArgs(int64(1)).
		WillReturnRows(categoryRows().AddRow(1, "Alimentacion", "food", "#ff0000", testTime, testTime))

	got, err := repo.FindByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if got.Name != "Alimentacion" {
		t.Errorf("Name = %q, want Alimentacion", got.Name)
	}
}

func TestFindByID_WrapsQueryError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectQuery(`(?s)SELECT`).WillReturnError(errStub)

	if _, err := repo.FindByID(context.Background(), 1); err == nil {
		t.Fatal("expected an error when the query fails")
	}
}

func TestCreate_InsertsAndReadsBack(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`INSERT INTO categories`).
		WithArgs("Ocio", "fun", "#0000ff").
		WillReturnResult(sqlmock.NewResult(5, 1))
	mock.ExpectQuery(`(?s)WHERE id = \?`).WithArgs(int64(5)).
		WillReturnRows(categoryRows().AddRow(5, "Ocio", "fun", "#0000ff", testTime, testTime))

	got, err := repo.Create(context.Background(), &Category{Name: "Ocio", Description: "fun", Color: "#0000ff"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if got.ID != 5 {
		t.Errorf("ID = %d, want 5", got.ID)
	}
}

func TestCreate_WrapsInsertError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`INSERT INTO categories`).WillReturnError(errStub)

	if _, err := repo.Create(context.Background(), &Category{}); err == nil {
		t.Fatal("expected an error when the insert fails")
	}
}

func TestCreate_WrapsLastInsertIdError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`INSERT INTO categories`).
		WillReturnResult(sqlmock.NewErrorResult(errStub))

	if _, err := repo.Create(context.Background(), &Category{}); err == nil {
		t.Fatal("expected an error when LastInsertId fails")
	}
}

func TestUpdate_BuildsSetClauseFromSuppliedFieldsOnly(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	name := "Renamed"
	color := "#123456"

	mock.ExpectExec(`UPDATE categories SET name = \?, color = \? WHERE id = \?`).
		WithArgs(name, color, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)WHERE id = \?`).WithArgs(int64(1)).
		WillReturnRows(categoryRows().AddRow(1, name, "d", color, testTime, testTime))

	got, err := repo.Update(context.Background(), 1, UpdateRequest{Name: &name, Color: &color})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if got.Name != name {
		t.Errorf("Name = %q, want %q", got.Name, name)
	}
}

func TestUpdate_AcceptsEveryField(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	name, desc, color := "n", "d", "c"
	mock.ExpectExec(`UPDATE categories SET name = \?, description = \?, color = \? WHERE id = \?`).
		WithArgs(name, desc, color, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)WHERE id = \?`).WithArgs(int64(1)).
		WillReturnRows(categoryRows().AddRow(1, name, desc, color, testTime, testTime))

	if _, err := repo.Update(context.Background(), 1, UpdateRequest{Name: &name, Description: &desc, Color: &color}); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
}

func TestUpdate_WithNoFieldsSkipsTheWrite(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	// No ExpectExec: an empty patch must not issue an UPDATE. ExpectationsWereMet
	// in the cleanup also rejects any unexpected statement.
	mock.ExpectQuery(`(?s)WHERE id = \?`).WithArgs(int64(1)).
		WillReturnRows(categoryRows().AddRow(1, "n", "d", "c", testTime, testTime))

	if _, err := repo.Update(context.Background(), 1, UpdateRequest{}); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
}

func TestUpdate_WrapsExecError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	name := "n"
	mock.ExpectExec(`UPDATE categories`).WillReturnError(errStub)

	if _, err := repo.Update(context.Background(), 1, UpdateRequest{Name: &name}); err == nil {
		t.Fatal("expected an error when the update fails")
	}
}

func TestDelete_RemovesTheRow(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`DELETE FROM categories WHERE id = \?`).WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Delete(context.Background(), 1); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}

func TestDelete_ZeroRowsAffectedIsNotFound(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`DELETE FROM categories`).WithArgs(int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := repo.Delete(context.Background(), 99); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDelete_WrapsExecError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	// A category still referenced by movements fails here on the foreign key;
	// the error must surface rather than be swallowed into a false success.
	mock.ExpectExec(`DELETE FROM categories`).WillReturnError(errStub)

	if err := repo.Delete(context.Background(), 1); err == nil {
		t.Fatal("expected an error when the delete fails")
	}
}

func TestDelete_WrapsRowsAffectedError(t *testing.T) {
	repo, mock, done := newRepoMock(t)
	defer done()

	mock.ExpectExec(`DELETE FROM categories`).
		WillReturnResult(sqlmock.NewErrorResult(errStub))

	if err := repo.Delete(context.Background(), 1); err == nil {
		t.Fatal("expected an error when RowsAffected fails")
	}
}
