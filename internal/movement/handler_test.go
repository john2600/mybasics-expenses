package movement

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/jscodelab/mybasics-expenses/internal/data"
	"github.com/jscodelab/mybasics-expenses/internal/security"
)

// mockService is a test double for Service. Each method returns what the test
// set, and records the arguments the handler passed down — which is how the
// tests pin that the user id comes from the context and never from the client.
type mockService struct {
	movement  *Movement
	grouped   []GroupedByCategory
	expenses  *ExpenseList
	summaries []MonthlySummary
	err       error

	gotUserID int
	gotID     int64
	gotFilter Filter
	gotCreate CreateRequest
	gotUpdate UpdateRequest
}

func (m *mockService) CreateMovement(_ context.Context, userID int, req CreateRequest) (*Movement, error) {
	m.gotUserID, m.gotCreate = userID, req
	return m.movement, m.err
}
func (m *mockService) ListMovements(_ context.Context, f Filter) ([]GroupedByCategory, error) {
	m.gotFilter = f
	return m.grouped, m.err
}
func (m *mockService) ListExpenses(_ context.Context, f Filter) (*ExpenseList, error) {
	m.gotFilter = f
	return m.expenses, m.err
}
func (m *mockService) GetMovement(_ context.Context, userID int, id int64) (*Movement, error) {
	m.gotUserID, m.gotID = userID, id
	return m.movement, m.err
}
func (m *mockService) UpdateMovement(_ context.Context, userID int, id int64, req UpdateRequest) (*Movement, error) {
	m.gotUserID, m.gotID, m.gotUpdate = userID, id, req
	return m.movement, m.err
}
func (m *mockService) DeleteMovement(_ context.Context, userID int, id int64) error {
	m.gotUserID, m.gotID = userID, id
	return m.err
}
func (m *mockService) GetMonthlySummary(_ context.Context, userID int) ([]MonthlySummary, error) {
	m.gotUserID = userID
	return m.summaries, m.err
}

const testUserID = 42

// serve routes a request through the same middleware chain production uses: the
// security guard resolves the user and puts the id on the context, then the
// movement routes run. guarded=false mounts the routes bare, which is how the
// handlers' own "no user id" branch is reachable.
func serve(t *testing.T, svc Service, guarded bool, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	r := chi.NewRouter()
	h := NewHandler(svc)
	if guarded {
		r.Group(func(r chi.Router) {
			r.Use(security.NewHandler(nil).RequireAuthentication)
			h.RegisterRoutes(r)
		})
	} else {
		h.RegisterRoutes(r)
	}

	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req = security.ContextSetUser(req, &data.User{ID: testUserID, Activated: true})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func decodeData(t *testing.T, rec *httptest.ResponseRecorder, into any) {
	t.Helper()
	var env struct {
		Data  json.RawMessage `json:"data"`
		Error string          `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decoding envelope: %v (body %q)", err, rec.Body.String())
	}
	if into != nil {
		if err := json.Unmarshal(env.Data, into); err != nil {
			t.Fatalf("decoding data: %v (data %q)", err, env.Data)
		}
	}
}

// --- create -----------------------------------------------------------------

func TestHandlerCreate_Returns201AndTakesTheUserFromContext(t *testing.T) {
	svc := &mockService{movement: &Movement{ID: 1, Amount: 100}}

	rec := serve(t, svc, true, http.MethodPost, "/movements",
		`{"category_id":2,"type":"E","amount":100,"description":"x","date":"2026-07-15"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %q)", rec.Code, rec.Body.String())
	}
	// The owner must come from the authenticated context, not the payload.
	if svc.gotUserID != testUserID {
		t.Errorf("service got userID = %d, want %d", svc.gotUserID, testUserID)
	}
	if svc.gotCreate.CategoryID != 2 || svc.gotCreate.Type != "E" {
		t.Errorf("decoded request = %+v", svc.gotCreate)
	}
}

func TestHandlerCreate_MalformedJSONIs400(t *testing.T) {
	rec := serve(t, &mockService{}, true, http.MethodPost, "/movements", `{"amount":`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerCreate_ServiceValidationErrorIs400(t *testing.T) {
	svc := &mockService{err: ErrNotFound}
	rec := serve(t, svc, true, http.MethodPost, "/movements", `{"category_id":2}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerCreate_WithoutAGuardIs401(t *testing.T) {
	// Mounted outside the protected group there is no user id on the context.
	// The handler must refuse rather than default the owner to zero.
	rec := serve(t, &mockService{}, false, http.MethodPost, "/movements", `{"category_id":2}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- list -------------------------------------------------------------------

func TestHandlerList_ParsesEveryFilterAndForcesTheUserScope(t *testing.T) {
	svc := &mockService{grouped: []GroupedByCategory{{Category: "Alimentacion", Total: 100}}}

	rec := serve(t, svc, true, http.MethodGet,
		"/movements?category_id=2&type=E&date_from=2026-07-01&date_to=2026-07-31&limit=5", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	f := svc.gotFilter
	if f.UserID != testUserID {
		t.Errorf("filter UserID = %d, want %d", f.UserID, testUserID)
	}
	if f.CategoryID == nil || *f.CategoryID != 2 {
		t.Errorf("CategoryID = %v, want 2", f.CategoryID)
	}
	if f.Type == nil || *f.Type != "E" {
		t.Errorf("Type = %v, want E", f.Type)
	}
	if f.Limit == nil || *f.Limit != 5 {
		t.Errorf("Limit = %v, want 5", f.Limit)
	}
	if f.DateFrom == nil || f.DateFrom.Format("2006-01-02") != "2026-07-01" {
		t.Errorf("DateFrom = %v", f.DateFrom)
	}
	if f.DateTo == nil || f.DateTo.Format("2006-01-02") != "2026-07-31" {
		t.Errorf("DateTo = %v", f.DateTo)
	}
}

func TestHandlerList_EmptyResultIsAnArrayNotNull(t *testing.T) {
	// A JSON null would break clients that iterate the result directly.
	rec := serve(t, &mockService{grouped: nil}, true, http.MethodGet, "/movements", "")

	var got []GroupedByCategory
	decodeData(t, rec, &got)
	if got == nil {
		t.Error("data decoded to nil, want an empty array")
	}
	if !strings.Contains(rec.Body.String(), `"data":[]`) {
		t.Errorf("body = %q, want an empty JSON array", rec.Body.String())
	}
}

func TestHandlerList_ServiceErrorIs500(t *testing.T) {
	rec := serve(t, &mockService{err: ErrNotFound}, true, http.MethodGet, "/movements", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerList_RejectsInvalidFilters(t *testing.T) {
	tests := []struct{ name, query string }{
		{"limit not a number", "?limit=abc"},
		{"limit zero", "?limit=0"},
		{"limit negative", "?limit=-3"},
		{"category_id not a number", "?category_id=abc"},
		{"type not I or E", "?type=X"},
		{"date_from malformed", "?date_from=15-07-2026"},
		{"date_to malformed", "?date_to=nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(t, &mockService{}, true, http.MethodGet, "/movements"+tt.query, "")
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestHandlerList_WithoutAGuardIs401(t *testing.T) {
	rec := serve(t, &mockService{}, false, http.MethodGet, "/movements", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- get --------------------------------------------------------------------

func TestHandlerGet_ReturnsTheMovement(t *testing.T) {
	svc := &mockService{movement: &Movement{ID: 7, Amount: 100}}

	rec := serve(t, svc, true, http.MethodGet, "/movements/7", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.gotID != 7 {
		t.Errorf("service got id = %d, want 7", svc.gotID)
	}
	if svc.gotUserID != testUserID {
		t.Errorf("service got userID = %d, want %d", svc.gotUserID, testUserID)
	}
}

func TestHandlerGet_NilMovementIs404(t *testing.T) {
	rec := serve(t, &mockService{movement: nil}, true, http.MethodGet, "/movements/7", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandlerGet_NonNumericIDIs400(t *testing.T) {
	rec := serve(t, &mockService{}, true, http.MethodGet, "/movements/abc", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerGet_ServiceErrorIs500(t *testing.T) {
	rec := serve(t, &mockService{err: ErrNotFound}, true, http.MethodGet, "/movements/7", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerGet_WithoutAGuardIs401(t *testing.T) {
	rec := serve(t, &mockService{}, false, http.MethodGet, "/movements/7", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- update -----------------------------------------------------------------

func TestHandlerUpdate_AppliesThePatch(t *testing.T) {
	svc := &mockService{movement: &Movement{ID: 7, Amount: 999}}

	rec := serve(t, svc, true, http.MethodPut, "/movements/7", `{"amount":999}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.gotUpdate.Amount == nil || *svc.gotUpdate.Amount != 999 {
		t.Errorf("patched amount = %v, want 999", svc.gotUpdate.Amount)
	}
}

func TestHandlerUpdate_ErrNotFoundIs404(t *testing.T) {
	rec := serve(t, &mockService{err: ErrNotFound}, true, http.MethodPut, "/movements/7", `{"amount":1}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandlerUpdate_NilResultIs404(t *testing.T) {
	rec := serve(t, &mockService{movement: nil}, true, http.MethodPut, "/movements/7", `{"amount":1}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandlerUpdate_OtherServiceErrorIs400(t *testing.T) {
	rec := serve(t, &mockService{err: errStub}, true, http.MethodPut, "/movements/7", `{"amount":1}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerUpdate_MalformedJSONIs400(t *testing.T) {
	rec := serve(t, &mockService{}, true, http.MethodPut, "/movements/7", `{`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerUpdate_NonNumericIDIs400(t *testing.T) {
	rec := serve(t, &mockService{}, true, http.MethodPut, "/movements/abc", `{"amount":1}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerUpdate_WithoutAGuardIs401(t *testing.T) {
	rec := serve(t, &mockService{}, false, http.MethodPut, "/movements/7", `{"amount":1}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- delete -----------------------------------------------------------------

func TestHandlerDelete_Returns204(t *testing.T) {
	svc := &mockService{}
	rec := serve(t, svc, true, http.MethodDelete, "/movements/7", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if svc.gotUserID != testUserID || svc.gotID != 7 {
		t.Errorf("service got (user %d, id %d), want (%d, 7)", svc.gotUserID, svc.gotID, testUserID)
	}
}

func TestHandlerDelete_ErrNotFoundIs404(t *testing.T) {
	rec := serve(t, &mockService{err: ErrNotFound}, true, http.MethodDelete, "/movements/7", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandlerDelete_OtherErrorIs500(t *testing.T) {
	rec := serve(t, &mockService{err: errStub}, true, http.MethodDelete, "/movements/7", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerDelete_NonNumericIDIs400(t *testing.T) {
	rec := serve(t, &mockService{}, true, http.MethodDelete, "/movements/abc", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerDelete_WithoutAGuardIs401(t *testing.T) {
	rec := serve(t, &mockService{}, false, http.MethodDelete, "/movements/7", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- expenses ---------------------------------------------------------------

func TestHandlerExpenses_ReturnsTotalAndList(t *testing.T) {
	svc := &mockService{expenses: &ExpenseList{Total: 148150, Movements: []Movement{{ID: 501}}}}

	rec := serve(t, svc, true, http.MethodGet, "/movements/expenses?category_id=2", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got ExpenseList
	decodeData(t, rec, &got)
	if got.Total != 148150 {
		t.Errorf("total = %v, want 148150", got.Total)
	}
	if svc.gotFilter.UserID != testUserID {
		t.Errorf("filter UserID = %d, want %d", svc.gotFilter.UserID, testUserID)
	}
}

func TestHandlerExpenses_InvalidFilterIs400(t *testing.T) {
	rec := serve(t, &mockService{}, true, http.MethodGet, "/movements/expenses?limit=abc", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerExpenses_ServiceErrorIs500(t *testing.T) {
	rec := serve(t, &mockService{err: errStub}, true, http.MethodGet, "/movements/expenses", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerExpenses_WithoutAGuardIs401(t *testing.T) {
	rec := serve(t, &mockService{}, false, http.MethodGet, "/movements/expenses", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// --- summary ----------------------------------------------------------------

func TestHandlerSummary_ReturnsMonthlyTotals(t *testing.T) {
	svc := &mockService{summaries: []MonthlySummary{{Year: 2026, Month: 8, Total: 1500}}}

	rec := serve(t, svc, true, http.MethodGet, "/movements/summary", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.gotUserID != testUserID {
		t.Errorf("service got userID = %d, want %d", svc.gotUserID, testUserID)
	}
}

func TestHandlerSummary_EmptyResultIsAnArrayNotNull(t *testing.T) {
	rec := serve(t, &mockService{summaries: nil}, true, http.MethodGet, "/movements/summary", "")
	if !strings.Contains(rec.Body.String(), `"data":[]`) {
		t.Errorf("body = %q, want an empty JSON array", rec.Body.String())
	}
}

func TestHandlerSummary_ServiceErrorIs500(t *testing.T) {
	rec := serve(t, &mockService{err: errStub}, true, http.MethodGet, "/movements/summary", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerSummary_WithoutAGuardIs401(t *testing.T) {
	rec := serve(t, &mockService{}, false, http.MethodGet, "/movements/summary", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
