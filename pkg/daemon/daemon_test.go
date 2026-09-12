package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testServer() *Server {
	return NewServer("secret")
}

func doReq(t *testing.T, srv *Server, method, target, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestAuthRequired(t *testing.T) {
	srv := testServer()
	if rec := doReq(t, srv, "GET", "/api/status", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %d", rec.Code)
	}
	if rec := doReq(t, srv, "GET", "/api/status", "", "wrong"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad token: want 401, got %d", rec.Code)
	}
	if rec := doReq(t, srv, "GET", "/api/status", "", "secret"); rec.Code != http.StatusOK {
		t.Fatalf("good token: want 200, got %d", rec.Code)
	}
}

func TestStatusShape(t *testing.T) {
	rec := doReq(t, testServer(), "GET", "/api/status", "", "secret")
	var out struct {
		OK           bool   `json:"ok"`
		Version      string `json:"version"`
		Dependencies []struct {
			Name      string `json:"name"`
			Available bool   `json:"available"`
			Required  bool   `json:"required"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.OK || out.Version == "" {
		t.Fatalf("bad status payload: %+v", out)
	}
	seenRequired := false
	for _, d := range out.Dependencies {
		if d.Name == "yt-dlp" && d.Required {
			seenRequired = true
		}
	}
	if !seenRequired {
		t.Fatalf("yt-dlp must be listed as required: %+v", out.Dependencies)
	}
}

func TestCreateDownloadValidation(t *testing.T) {
	srv := testServer()
	rec := doReq(t, srv, "POST", "/api/downloads", `{"url":""}`, "secret")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty url: want 400, got %d", rec.Code)
	}
	rec = doReq(t, srv, "POST", "/api/downloads", `not json`, "secret")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad json: want 400, got %d", rec.Code)
	}
}

func TestTaskNotFound(t *testing.T) {
	srv := testServer()
	if rec := doReq(t, srv, "GET", "/api/tasks/999999", "", "secret"); rec.Code != http.StatusNotFound {
		t.Fatalf("get missing: want 404, got %d", rec.Code)
	}
	if rec := doReq(t, srv, "POST", "/api/tasks/999999/cancel", "", "secret"); rec.Code != http.StatusNotFound {
		t.Fatalf("cancel missing: want 404, got %d", rec.Code)
	}
	if rec := doReq(t, srv, "GET", "/api/tasks/abc", "", "secret"); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad id: want 400, got %d", rec.Code)
	}
}

func TestTasksListEmptyShape(t *testing.T) {
	rec := doReq(t, testServer(), "GET", "/api/tasks", "", "secret")
	var out struct {
		Tasks []map[string]interface{} `json:"tasks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Tasks == nil {
		t.Fatal("tasks must serialize as [] not null")
	}
}

func TestHistoryEndpoints(t *testing.T) {
	srv := testServer()
	if rec := doReq(t, srv, "GET", "/api/history", "", "secret"); rec.Code != http.StatusOK {
		t.Fatalf("history: want 200, got %d", rec.Code)
	}
	del := `{"time":"x","source":"s","target":"t","delete_file":false}`
	if rec := doReq(t, srv, "POST", "/api/history/delete", del, "secret"); rec.Code != http.StatusOK {
		t.Fatalf("history delete: want 200, got %d", rec.Code)
	}
	if rec := doReq(t, srv, "POST", "/api/history/delete", `bad`, "secret"); rec.Code != http.StatusBadRequest {
		t.Fatalf("history delete bad json: want 400, got %d", rec.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	rec := doReq(t, testServer(), "OPTIONS", "/api/tasks", "", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight: want 204, got %d", rec.Code)
	}
}

func TestQueryTokenFallback(t *testing.T) {
	srv := testServer()
	req := httptest.NewRequest("GET", "/api/status?token=secret", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("query token: want 200, got %d", rec.Code)
	}
	req = httptest.NewRequest("GET", "/api/status?token=wrong", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad query token: want 401, got %d", rec.Code)
	}
}
