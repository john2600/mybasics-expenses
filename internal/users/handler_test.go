package users

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"

	"github.com/jscodelab/mybasics-expenses/internal/security"
)

type mockService struct {
	insertErr   error
	changeErr   error
	authID      int
	authErr     error
	activateErr error

	gotInsert   UserRequest
	gotChange   ChangePasswordRequest
	gotEmail    string
	gotPassword string
	gotToken    string
}

func (m *mockService) InsertUser(_ context.Context, req UserRequest) error {
	m.gotInsert = req
	return m.insertErr
}
func (m *mockService) ChangePassword(_ context.Context, req ChangePasswordRequest) error {
	m.gotChange = req
	return m.changeErr
}
func (m *mockService) Authenticate(_ context.Context, email, password string) (int, error) {
	m.gotEmail, m.gotPassword = email, password
	return m.authID, m.authErr
}
func (m *mockService) ActiveUser(_ context.Context, token string) error {
	m.gotToken = token
	return m.activateErr
}

// serveUsers runs a request through the session middleware, since login and
// logout write to the session.
func serveUsers(t *testing.T, svc Service, method, target, body string) (*httptest.ResponseRecorder, *scs.SessionManager) {
	t.Helper()

	sm := scs.New()
	r := chi.NewRouter()
	r.Use(sm.LoadAndSave)
	h := NewHandler(svc, sm)
	h.RegisterRoutes(r)
	h.RegisterProtectedRoutes(r)

	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec, sm
}

func TestHandlerCreate_Returns201(t *testing.T) {
	svc := &mockService{}

	rec, _ := serveUsers(t, svc, http.MethodPost, "/user",
		`{"username":"john","name":"John Doe","email":"john@example.com","password":"supersecret"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %q)", rec.Code, rec.Body.String())
	}
	if svc.gotInsert.UserName != "john" || svc.gotInsert.Email != "john@example.com" {
		t.Errorf("decoded request = %+v", svc.gotInsert)
	}
}

func TestHandlerCreate_DuplicateIsAGeneric400(t *testing.T) {
	rec, _ := serveUsers(t, &mockService{insertErr: ErrDuplicateUser}, http.MethodPost, "/user",
		`{"username":"john","email":"john@example.com"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	// The message must not say which field collided, or it becomes an account
	// enumeration oracle.
	body := rec.Body.String()
	if !strings.Contains(body, "username or email already in use") {
		t.Errorf("body = %q, want the generic duplicate message", body)
	}
	if strings.Contains(body, "john@example.com") {
		t.Error("the submitted email was echoed back in the error")
	}
}

func TestHandlerCreate_MalformedJSONIs400(t *testing.T) {
	rec, _ := serveUsers(t, &mockService{}, http.MethodPost, "/user", `{"username":`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerCreate_ValidationErrorIs400(t *testing.T) {
	rec, _ := serveUsers(t, &mockService{insertErr: errBoom}, http.MethodPost, "/user", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerActivate_ReadsTheTokenFromTheQueryString(t *testing.T) {
	svc := &mockService{}

	rec, _ := serveUsers(t, svc, http.MethodGet, "/user/activate?token=abc123", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.gotToken != "abc123" {
		t.Errorf("token = %q, want abc123", svc.gotToken)
	}
	if !strings.Contains(rec.Body.String(), "account activated") {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestHandlerActivate_UnknownTokenGetsAFriendlyMessage(t *testing.T) {
	// The raw sentinel would say "token not found or expired"; the handler
	// rewrites it for someone who just clicked an email link.
	rec, _ := serveUsers(t, &mockService{activateErr: security.ErrTokenNotFound},
		http.MethodGet, "/user/activate?token=stale", "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid or expired activation link") {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestHandlerActivate_OtherErrorIs400(t *testing.T) {
	rec, _ := serveUsers(t, &mockService{activateErr: errBoom}, http.MethodGet, "/user/activate?token=x", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerUpdatePassword_DecodesTheNestedPayload(t *testing.T) {
	svc := &mockService{}

	rec, _ := serveUsers(t, svc, http.MethodPost, "/change_password",
		`{"login_request":{"email":"john@example.com","password":"currentPass123"},"new_password":"brandNewPass456"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if svc.gotChange.LoginRequest.Email != "john@example.com" {
		t.Errorf("email = %q", svc.gotChange.LoginRequest.Email)
	}
	if svc.gotChange.NewPassword != "brandNewPass456" {
		t.Errorf("new password not decoded: %+v", svc.gotChange)
	}
}

func TestHandlerUpdatePassword_MalformedJSONIs400(t *testing.T) {
	rec, _ := serveUsers(t, &mockService{}, http.MethodPost, "/change_password", `{`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerUpdatePassword_ServiceErrorIs400(t *testing.T) {
	rec, _ := serveUsers(t, &mockService{changeErr: errBoom}, http.MethodPost, "/change_password", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerLogin_StoresTheUserInTheSession(t *testing.T) {
	svc := &mockService{authID: 42}

	rec, _ := serveUsers(t, svc, http.MethodPost, "/user/login",
		`{"email":"john@example.com","password":"supersecret"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if svc.gotEmail != "john@example.com" || svc.gotPassword != "supersecret" {
		t.Errorf("credentials = (%q, %q)", svc.gotEmail, svc.gotPassword)
	}
	// A session cookie must be issued, otherwise the legacy flow cannot work.
	if len(rec.Result().Cookies()) == 0 {
		t.Error("no session cookie was set on login")
	}
}

func TestHandlerLogin_BadCredentialsAre401WithAGenericMessage(t *testing.T) {
	rec, _ := serveUsers(t, &mockService{authErr: ErrInvalidCredentials}, http.MethodPost,
		"/user/login", `{"email":"john@example.com","password":"wrong"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid email or password") {
		t.Errorf("body = %q, want the generic message", rec.Body.String())
	}
}

func TestHandlerLogin_InfrastructureErrorIs500(t *testing.T) {
	rec, _ := serveUsers(t, &mockService{authErr: errBoom}, http.MethodPost,
		"/user/login", `{"email":"john@example.com","password":"supersecret"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandlerLogin_MalformedJSONIs400(t *testing.T) {
	rec, _ := serveUsers(t, &mockService{}, http.MethodPost, "/user/login", `{"email":`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerLogout_DestroysTheSession(t *testing.T) {
	rec, _ := serveUsers(t, &mockService{}, http.MethodPost, "/user/logout", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "logout successful") {
		t.Errorf("body = %q", rec.Body.String())
	}
}
