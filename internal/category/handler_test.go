package category

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// mockService is a test double for Service, recording what the handler passed
// down so the tests can assert the decoding as well as the status.
type mockService struct {
	categories []Category
	category   *Category
	err        error

	gotID     int64
	gotCreate CreateRequest
	gotUpdate UpdateRequest
}

func (m *mockService) ListCategories(context.Context) ([]Category, error) {
	return m.categories, m.err
}
func (m *mockService) GetCategory(_ context.Context, id int64) (*Category, error) {
	m.gotID = id
	return m.category, m.err
}
func (m *mockService) CreateCategory(_ context.Context, req CreateRequest) (*Category, error) {
	m.gotCreate = req
	return m.category, m.err
}
func (m *mockService) UpdateCategory(_ context.Context, id int64, req UpdateRequest) (*Category, error) {
	m.gotID, m.gotUpdate = id, req
	return m.category, m.err
}
func (m *mockService) DeleteCategory(_ context.Context, id int64) error {
	m.gotID = id
	return m.err
}

func serve(t *testing.T, svc Service, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	r := chi.NewRouter()
	NewHandler(svc).RegisterRoutes(r)

	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestHandlerList_ReturnsCategories(t *testing.T) {
	svc := &mockService{categories: []Category{{ID: 1, Name: "Alimentacion"}}}

	rec := serve(t, svc, http.MethodGet, "/categories", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var env struct {
		Data []Category `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(env.Data) != 1 || env.Data[0].Name != "Alimentacion" {
		t.Errorf("data = %+v", env.Data)
	}
}

func TestHandlerList_EmptyIsAnArrayNotNull(t *testing.T) {
	// A JSON null breaks clients that iterate the result directly.
	rec := serve(t, &mockService{categories: nil}, http.MethodGet, "/categories", "")

	if !strings.Contains(rec.Body.String(), `"data":[]`) {
		t.Errorf("body = %q, want an empty JSON array", rec.Body.String())
	}
}

func TestHandlerList_ServiceErrorIs500(t *testing.T) {
	rec := serve(t, &mockService{err: errStub}, http.MethodGet, "/categories", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerCreate_Returns201AndDecodesThePayload(t *testing.T) {
	svc := &mockService{category: &Category{ID: 1, Name: "Ocio"}}

	rec := serve(t, svc, http.MethodPost, "/categories",
		`{"name":"Ocio","description":"fun","color":"#0000ff"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if svc.gotCreate.Name != "Ocio" || svc.gotCreate.Color != "#0000ff" {
		t.Errorf("decoded request = %+v", svc.gotCreate)
	}
}

func TestHandlerCreate_MalformedJSONIs400(t *testing.T) {
	rec := serve(t, &mockService{}, http.MethodPost, "/categories", `{"name":`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerCreate_ValidationErrorIs400(t *testing.T) {
	// The service rejects an empty name; the handler must map that to 400,
	// not 500 — it is the caller's mistake, not the server's.
	rec := serve(t, &mockService{err: errStub}, http.MethodPost, "/categories", `{"name":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerGet_ReturnsTheCategory(t *testing.T) {
	svc := &mockService{category: &Category{ID: 3, Name: "Transporte"}}

	rec := serve(t, svc, http.MethodGet, "/categories/3", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.gotID != 3 {
		t.Errorf("service got id = %d, want 3", svc.gotID)
	}
}

func TestHandlerGet_NilIs404(t *testing.T) {
	rec := serve(t, &mockService{category: nil}, http.MethodGet, "/categories/3", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandlerGet_NonNumericIDIs400(t *testing.T) {
	rec := serve(t, &mockService{}, http.MethodGet, "/categories/abc", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerGet_ServiceErrorIs500(t *testing.T) {
	rec := serve(t, &mockService{err: errStub}, http.MethodGet, "/categories/3", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerUpdate_AppliesThePatch(t *testing.T) {
	svc := &mockService{category: &Category{ID: 1, Name: "Renamed"}}

	rec := serve(t, svc, http.MethodPut, "/categories/1", `{"name":"Renamed"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.gotUpdate.Name == nil || *svc.gotUpdate.Name != "Renamed" {
		t.Errorf("patched name = %v, want Renamed", svc.gotUpdate.Name)
	}
	// Fields absent from the body must stay nil so the repository leaves them
	// untouched instead of overwriting them with a zero value.
	if svc.gotUpdate.Color != nil {
		t.Errorf("Color = %v, want nil for a field not in the payload", *svc.gotUpdate.Color)
	}
}

func TestHandlerUpdate_NilIs404(t *testing.T) {
	rec := serve(t, &mockService{category: nil}, http.MethodPut, "/categories/1", `{"name":"x"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandlerUpdate_MalformedJSONIs400(t *testing.T) {
	rec := serve(t, &mockService{}, http.MethodPut, "/categories/1", `{`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerUpdate_NonNumericIDIs400(t *testing.T) {
	rec := serve(t, &mockService{}, http.MethodPut, "/categories/abc", `{"name":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerUpdate_ServiceErrorIs500(t *testing.T) {
	rec := serve(t, &mockService{err: errStub}, http.MethodPut, "/categories/1", `{"name":"x"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerDelete_Returns204(t *testing.T) {
	svc := &mockService{}

	rec := serve(t, svc, http.MethodDelete, "/categories/4", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if svc.gotID != 4 {
		t.Errorf("service got id = %d, want 4", svc.gotID)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
}

func TestHandlerDelete_ErrNotFoundIs404(t *testing.T) {
	rec := serve(t, &mockService{err: ErrNotFound}, http.MethodDelete, "/categories/4", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandlerDelete_OtherErrorIs500(t *testing.T) {
	// A category still referenced by movements fails on the foreign key. That
	// is not a 404 — the row exists, it just cannot be removed.
	rec := serve(t, &mockService{err: errStub}, http.MethodDelete, "/categories/4", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerDelete_NonNumericIDIs400(t *testing.T) {
	rec := serve(t, &mockService{}, http.MethodDelete, "/categories/abc", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
