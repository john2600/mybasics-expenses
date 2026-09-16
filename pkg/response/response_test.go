package response

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// decode reads the recorded body back as an Envelope, so the tests assert the
// wire shape clients actually receive rather than the struct that produced it.
func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	if rec.Body.Len() == 0 {
		return nil
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding body %q: %v", rec.Body.String(), err)
	}
	return got
}

func TestJSON_SetsStatusContentTypeAndBody(t *testing.T) {
	rec := httptest.NewRecorder()

	JSON(rec, http.StatusTeapot, map[string]string{"hello": "world"})

	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if got := decode(t, rec)["hello"]; got != "world" {
		t.Errorf("body hello = %v, want world", got)
	}
}

func TestSuccess_Is200WithDataOnly(t *testing.T) {
	rec := httptest.NewRecorder()

	Success(rec, map[string]int{"id": 1})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body := decode(t, rec)
	if _, ok := body["data"]; !ok {
		t.Error("body has no data field")
	}
	// omitempty must keep error and message out of a success payload.
	if _, ok := body["error"]; ok {
		t.Error("a success response must not carry an error field")
	}
	if _, ok := body["message"]; ok {
		t.Error("a success response must not carry a message field")
	}
}

func TestCreated_Is201(t *testing.T) {
	rec := httptest.NewRecorder()

	Created(rec, map[string]int{"id": 1})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if _, ok := decode(t, rec)["data"]; !ok {
		t.Error("body has no data field")
	}
}

func TestNoContent_Is204WithAnEmptyBody(t *testing.T) {
	rec := httptest.NewRecorder()

	NoContent(rec)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
}

func TestErrorHelpers_StatusAndMessage(t *testing.T) {
	tests := []struct {
		name     string
		call     func(*httptest.ResponseRecorder)
		want     int
		wantBody string
	}{
		{"BadRequest", func(r *httptest.ResponseRecorder) { BadRequest(r, errStub) }, http.StatusBadRequest, "stub failure"},
		{"Unauthorized", func(r *httptest.ResponseRecorder) { Unauthorized(r, errStub) }, http.StatusUnauthorized, "stub failure"},
		{"NotActivateAccount", func(r *httptest.ResponseRecorder) { NotActivateAccount(r, errStub) }, http.StatusForbidden, "stub failure"},
		{"NotFound", func(r *httptest.ResponseRecorder) { NotFound(r, "movement not found") }, http.StatusNotFound, "movement not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.call(rec)

			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
			if got := decode(t, rec)["error"]; got != tt.wantBody {
				t.Errorf("error = %v, want %q", got, tt.wantBody)
			}
			// An error envelope must not smuggle a data field.
			if _, ok := decode(t, rec)["data"]; ok {
				t.Error("an error response must not carry a data field")
			}
		})
	}
}

func TestInternalError_HidesTheCauseFromTheClientButLogsIt(t *testing.T) {
	var logged strings.Builder
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	rec := httptest.NewRecorder()
	InternalError(rec, errSecret)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	// The whole point of this helper: the client gets a generic message so
	// table names and query fragments never leak.
	if got := decode(t, rec)["error"]; got != "internal server error" {
		t.Errorf("error = %v, want the generic message", got)
	}
	if strings.Contains(rec.Body.String(), "movements_ibfk_1") {
		t.Errorf("the real cause leaked to the client: %q", rec.Body.String())
	}
	// ...but the operator still gets it in the log.
	if !strings.Contains(logged.String(), "movements_ibfk_1") {
		t.Errorf("log = %q, want it to contain the real cause", logged.String())
	}
}
