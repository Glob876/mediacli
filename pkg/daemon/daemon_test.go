package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"mediacli/pkg/core"
	"mediacli/pkg/ui"
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

func TestLibraryThumbValidation(t *testing.T) {
	srv := testServer()
	// Пустое имя: guard resolveLibraryPath не проходит.
	if rec := doReq(t, srv, "GET", "/api/library/thumb", "", "secret"); rec.Code != http.StatusForbidden {
		t.Fatalf("empty name: want 403, got %d", rec.Code)
	}
	// Несуществующий файл внутри downloadDir → 404.
	if rec := doReq(t, srv, "GET", "/api/library/thumb?name=no-such-video.mp4", "", "secret"); rec.Code != http.StatusNotFound {
		t.Fatalf("missing file: want 404, got %d", rec.Code)
	}
	// Выход за пределы downloadDir → 403.
	if rec := doReq(t, srv, "GET", "/api/library/thumb?name=/etc/passwd", "", "secret"); rec.Code != http.StatusForbidden {
		t.Fatalf("traversal: want 403, got %d", rec.Code)
	}
	// Аутентификация обязательна и здесь.
	if rec := doReq(t, srv, "GET", "/api/library/thumb?name=x.mp4", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %d", rec.Code)
	}
}

func TestResolveLibraryPath(t *testing.T) {
	base := string(filepath.Separator) + "dl"
	if p, ok := resolveLibraryPath("a.mp4", base); !ok || p != filepath.Join(base, "a.mp4") {
		t.Fatalf("basename: got %q, %v", p, ok)
	}
	if _, ok := resolveLibraryPath("", base); ok {
		t.Fatal("empty name must not resolve")
	}
	if _, ok := resolveLibraryPath("/etc/passwd", base); ok {
		t.Fatal("absolute path outside base must not resolve")
	}
	// basename нейтрализует traversal: "../evil.mp4" → base/evil.mp4 (внутри).
	if p, ok := resolveLibraryPath("../evil.mp4", base); !ok || p != filepath.Join(base, "evil.mp4") {
		t.Fatalf("traversal basename must stay inside base: got %q, %v", p, ok)
	}
}

func TestDefaultAccentColor(t *testing.T) {
	cfg := core.GetDefaultConfig()
	if cfg.AccentColor == "" {
		t.Fatal("default accent color must not be empty")
	}
}

func containsArg(args []string, key, val string) bool {
	for i := 0; i < len(args); i++ {
		if args[i] == key && i+1 < len(args) && args[i+1] == val {
			return true
		}
		if args[i] == key && val == "" {
			return true
		}
	}
	return false
}

func TestFfmpegLocationArg(t *testing.T) {
	preset := core.DownloadPreset{ID: "x", Name: "x", Fields: core.GetInitialPresetFields()}
	cfg := core.GetDefaultConfig()
	cfg.FfmpegPath = ""
	if args := core.BuildYtDlpArgs(preset, cfg, "/tmp", false); containsArg(args, "--ffmpeg-location", "") {
		t.Fatalf("empty ffmpeg_path must not emit --ffmpeg-location: %v", args)
	}
	cfg.FfmpegPath = "/opt/ffmpeg/bin/ffmpeg"
	args := core.BuildYtDlpArgs(preset, cfg, "/tmp", false)
	if !containsArg(args, "--ffmpeg-location", "/opt/ffmpeg/bin/ffmpeg") {
		t.Fatalf("--ffmpeg-location missing: %v", args)
	}
	if got := core.FfmpegBin(cfg); got != "/opt/ffmpeg/bin/ffmpeg" {
		t.Fatalf("FfmpegBin: got %q", got)
	}
	cfg.FfmpegPath = ""
	if got := core.FfmpegBin(cfg); got != "ffmpeg" {
		t.Fatalf("FfmpegBin default: got %q", got)
	}
}

func TestToolsFfmpegEndpoint(t *testing.T) {
	srv := testServer()
	// Заведомо битый путь → found=false + текст ошибки.
	rec := doReq(t, srv, "GET", "/api/tools/ffmpeg?path=/no/such/ffmpeg-xyz", "", "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("tools/ffmpeg: want 200, got %d", rec.Code)
	}
	var out struct {
		Configured string `json:"configured"`
		Resolved   string `json:"resolved"`
		Found      bool   `json:"found"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Found || out.Error == "" {
		t.Fatalf("bad path must report found=false + error: %+v", out)
	}
	if out.Resolved != "/no/such/ffmpeg-xyz" {
		t.Fatalf("resolved must echo candidate: %+v", out)
	}
	// Форма ответа без кандидата: поля configured/resolved/found обязаны быть.
	rec = doReq(t, srv, "GET", "/api/tools/ffmpeg", "", "secret")
	var out2 struct {
		Resolved string `json:"resolved"`
		Found    bool   `json:"found"`
		Version  string `json:"version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out2); err != nil {
		t.Fatal(err)
	}
	if out2.Resolved == "" {
		t.Fatal("resolved must not be empty")
	}
	if out2.Found && out2.Version == "" {
		t.Fatal("found ffmpeg must report a version")
	}
}

func TestMetaEndpoint(t *testing.T) {
	rec := doReq(t, testServer(), "GET", "/api/meta", "", "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("meta: want 200, got %d", rec.Code)
	}
	var out struct {
		Themes []struct {
			ID     string `json:"id"`
			NameEN string `json:"name_en"`
			NameRU string `json:"name_ru"`
		} `json:"themes"`
		Browsers []string `json:"browsers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Themes) == 0 || len(out.Browsers) == 0 {
		t.Fatalf("meta must list themes and browsers: %+v", out)
	}
}

func TestMetaThemesMatchUI(t *testing.T) {
	if len(metaThemes) != len(ui.Themes) {
		t.Fatalf("metaThemes (%d) out of sync with ui.Themes (%d)", len(metaThemes), len(ui.Themes))
	}
	for _, m := range metaThemes {
		th, ok := ui.Themes[m["id"]]
		if !ok {
			t.Fatalf("theme %q missing in ui.Themes", m["id"])
		}
		if th.NameEN != m["name_en"] || th.NameRU != m["name_ru"] {
			t.Fatalf("theme %q names differ: ui=%q/%q meta=%q/%q",
				m["id"], th.NameEN, th.NameRU, m["name_en"], m["name_ru"])
		}
	}
}
