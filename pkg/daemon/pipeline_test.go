package daemon

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditorPreviewAndMediaStream(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe unavailable")
	}
	input := filepath.Join(t.TempDir(), "input.wav")
	cmd := exec.Command("ffmpeg", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=2", input)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v: %s", err, out)
	}
	srv := testServer()
	body := `{"inputs":[` + quoteJSON(input) + `],"operations":[{"type":"trim","config":{"start":"0.5","end":"1.5","mode":"reencode"}}],"duration":"0.5"}`
	rec := doReq(t, srv, "POST", "/api/pipeline/preview", body, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		PreviewPath string `json:"preview_path"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(response.PreviewPath)
	if response.PreviewPath == "" {
		t.Fatal("no preview path")
	}
	target := "/api/media/file?path=" + url.QueryEscape(response.PreviewPath)
	stream := doReq(t, srv, "GET", target, "", "secret")
	if stream.Code != http.StatusOK || stream.Body.Len() == 0 {
		t.Fatalf("stream: %d bytes=%d", stream.Code, stream.Body.Len())
	}
	if unauth := doReq(t, srv, "GET", target, "", ""); unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated stream: %d", unauth.Code)
	}
}

func quoteJSON(value string) string { data, _ := json.Marshal(value); return string(data) }

func TestProbeValidation(t *testing.T) {
	srv := testServer()
	if rec := doReq(t, srv, "GET", "/api/probe", "", "secret"); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing path: want 400, got %d", rec.Code)
	}
	if rec := doReq(t, srv, "GET", "/api/probe?path=/nonexistent_xyz_12345.mp4", "", "secret"); rec.Code != http.StatusNotFound {
		t.Fatalf("missing file: want 404, got %d", rec.Code)
	}
}

func TestPipelineDryRunValidation(t *testing.T) {
	srv := testServer()
	// empty inputs → 400
	body := `{"inputs":[],"operations":[]}`
	if rec := doReq(t, srv, "POST", "/api/pipeline/dry-run", body, "secret"); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty inputs: want 400, got %d %s", rec.Code, rec.Body.String())
	}
	// valid transcode dry-run
	body = `{"inputs":["/tmp/in.mp4"],"operations":[{"type":"transcode","config":{"vcodec":"libx264","acodec":"aac","crf":"18"}}],"output":{"path":"/tmp/out.mp4"}}`
	rec := doReq(t, srv, "POST", "/api/pipeline/dry-run", body, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("valid pipeline: want 200, got %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Commands []string `json:"commands"`
		Count    int      `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Commands) == 0 || out.Count == 0 {
		t.Fatalf("commands empty: %+v", out)
	}
	if !strings.Contains(out.Commands[0], "libx264") {
		t.Fatalf("cmd missing libx264: %v", out.Commands[0])
	}
}

func TestMediaPresetsCRUD(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	srv := testServer()
	// list empty
	rec := doReq(t, srv, "GET", "/api/pipeline/presets", "", "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d", rec.Code)
	}
	// create
	body := `{"name":"Test Preset","operations":[{"type":"transcode","config":{"vcodec":"libx264"}}]}`
	rec = doReq(t, srv, "POST", "/api/pipeline/presets", body, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("create: want 200, got %d %s", rec.Code, rec.Body.String())
	}
	// list again
	rec = doReq(t, srv, "GET", "/api/pipeline/presets", "", "secret")
	var out struct {
		Presets []map[string]interface{} `json:"presets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Presets) == 0 {
		t.Fatalf("presets should contain 1")
	}
}

func TestToolsFfprobe(t *testing.T) {
	srv := testServer()
	rec := doReq(t, srv, "GET", "/api/tools/ffprobe?path=/no/such/ffprobe-xyz", "", "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("ffprobe: want 200, got %d", rec.Code)
	}
	var out struct {
		Found bool   `json:"found"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Found || out.Error == "" {
		t.Fatalf("bad ffprobe must report error: %+v", out)
	}
}

func TestOpAspectValidation(t *testing.T) {
	srv := testServer()
	body := `{"input":"","aspect":{"ratio_w":16,"ratio_h":9,"mode":"crop"}}`
	if rec := doReq(t, srv, "POST", "/api/operations/aspect", body, "secret"); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty input: want 400, got %d", rec.Code)
	}
}

func TestBatchValidation(t *testing.T) {
	srv := testServer()
	body := `{"inputs":[],"pipeline":{"operations":[]}}`
	if rec := doReq(t, srv, "POST", "/api/batch", body, "secret"); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty batch: want 400, got %d", rec.Code)
	}
}
