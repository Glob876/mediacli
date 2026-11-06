package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mediacli/pkg/core"
	"mediacli/pkg/daemon"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func invoke(t *testing.T, input string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := RunContext(context.Background(), args, strings.NewReader(input), &out, &errOut)
	return code, out.String(), errOut.String()
}

func fixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe unavailable")
	}
	path := filepath.Join(t.TempDir(), "clip.mp4")
	command := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=128x128:rate=25:duration=2", "-f", "lavfi", "-i", "sine=duration=2", "-c:v", "libx264", "-c:a", "aac", "-shortest", path)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	return path
}

func TestHelpAndArgumentErrorsWithoutTTY(t *testing.T) {
	t.Setenv("MEDIACLI_SERVER", "")
	t.Setenv("MEDIACLI_TOKEN", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, command := range []string{"download", "thumbnail", "probe", "waveform", "convert", "trim", "aspect", "mux", "demux", "concat", "filter", "audio", "subtitle", "metadata", "scale", "fps", "stabilize", "size", "ranges", "pipeline", "editor", "batch", "config", "presets", "history", "doctor", "cookies", "library", "queue"} {
		code, out, errOut := invoke(t, "", command, "--help")
		if code != 0 || !strings.Contains(out, "Usage:") || errOut != "" {
			t.Fatalf("help %s: %d %s %s", command, code, out, errOut)
		}
	}
	for _, args := range [][]string{{"unknown"}, {"trim"}, {"convert", "--preset", "standard_mp4"}, {"probe", "one", "two"}, {"download", "--no-such-option"}, {"download", "--background", "https://example.com"}, {"config", "set", "nonexistent", "1"}, {"config", "set", "bg_queue_max", "0"}} {
		code, _, _ := invoke(t, "", args...)
		if code != 2 {
			t.Fatalf("%v: expected usage error, got %d", args, code)
		}
	}
}

func TestAllMediaCommandPlans(t *testing.T) {
	t.Setenv("MEDIACLI_SERVER", "")
	t.Setenv("MEDIACLI_TOKEN", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	input := fixture(t)
	dir := t.TempDir()
	sub := filepath.Join(dir, "captions.srt")
	os.WriteFile(sub, []byte("1\n00:00:00,000 --> 00:00:01,000\nHello\n"), 0600)
	commands := [][]string{
		{"convert", input, "--vcodec", "libx264", "--crf", "22"},
		{"convert", input, "--preset", "audio_flac"},
		{"trim", input, "--start", "0.2", "--end", "1.2"},
		{"aspect", input, "--ratio", "9:16", "--mode", "blur"},
		{"mux", input, "--extra-inputs", input, "--shortest"},
		{"demux", input, "--stream-type", "audio", "-o", filepath.Join(dir, "a.m4a")},
		{"concat", input, input, "--mode", "demuxer"},
		{"concat", input, input, "--mode", "filter"},
		{"filter", input, "--vf", "hflip"},
		{"audio", input, "--volume", "0.5", "--normalize"},
		{"subtitle", input, "--mode", "burn", "--subtitle-file", sub},
		{"metadata", input, "--title", "Name with spaces", "--extra", `{"artist":"Someone"}`},
		{"scale", input, "--width", "64", "--height", "64"},
		{"fps", input, "--fps", "10"},
		{"stabilize", input},
		{"size", input, "--size-mb", "0.2", "--two-pass"},
		{"ranges", input, "--range", "00:00:00.1..00:00:00.5", "--join"},
		{"pipeline", input, "--op", `transcode={"vcodec":"libx264","acodec":"aac"}`, "--preview", "0.5"},
		{"batch", input, filepath.Join(dir, "missing.mp4"), "--op", `transcode={"vcodec":"libx264","container":"mp4"}`},
	}
	for _, args := range commands {
		args = append(args, "--dry-run", "--json")
		code, out, errOut := invoke(t, "", args...)
		if code != 0 || !json.Valid([]byte(out)) {
			t.Fatalf("%v: %d %s %s", args, code, out, errOut)
		}
	}
	code, _, errOut := invoke(t, "", "audio", input, "--options", `{"not_a_field":true}`, "--dry-run")
	if code != 2 {
		t.Fatalf("unknown options accepted: %s", errOut)
	}
	code, out, errOut := invoke(t, "", "download", "https://example.com/video", "--proxy-mode", "custom", "--proxy-url", "http://user:secret@127.0.0.1:9999", "--dry-run", "--json")
	if code != 0 || strings.Contains(out, "secret") {
		t.Fatalf("credentials in plan: %d %s %s", code, out, errOut)
	}
}

func TestLocalMediaExecutionAndProjects(t *testing.T) {
	t.Setenv("MEDIACLI_SERVER", "")
	t.Setenv("MEDIACLI_TOKEN", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	input := fixture(t)
	dir := t.TempDir()
	output := filepath.Join(dir, "trimmed.mp4")
	code, out, errOut := invoke(t, "", "trim", input, "--start", "0.25", "--end", "1.25", "-o", output, "--json")
	if code != 0 || !json.Valid([]byte(out)) {
		t.Fatalf("trim: %d %s %s", code, out, errOut)
	}
	info, err := core.ProbeMediaInfo(output)
	if err != nil || info.Format.DurationSec < 0.95 || info.Format.DurationSec > 1.1 {
		t.Fatalf("bad duration: %+v %v", info, err)
	}
	// Source aliases must never bypass the input guard, even with --overwrite.
	alias := filepath.Join(dir, "alias.mp4")
	if err := os.Link(input, alias); err == nil {
		code, _, _ := invoke(t, "", "convert", input, "-o", alias, "--overwrite")
		if code == 0 {
			t.Fatal("source overwritten through hard link")
		}
	}
	project := filepath.Join(dir, "project.json")
	code, _, errOut = invoke(t, "", "pipeline", input, "--op", `scale={"width":64,"height":64}`, "--op", `transcode={"vcodec":"libx264","acodec":"aac"}`, "--save-project", project)
	if code != 0 {
		t.Fatal(errOut)
	}
	code, _, errOut = invoke(t, "", "editor", "--project", project, "-o", filepath.Join(dir, "scaled.mp4"))
	if code != 0 {
		t.Fatal(errOut)
	}
	data, _ := os.ReadFile(project)
	code, _, errOut = invoke(t, string(data), "pipeline", "--project", "-", "--dry-run")
	if code != 0 {
		t.Fatal(errOut)
	}
	for _, mode := range []string{"demuxer", "filter"} {
		result := filepath.Join(dir, mode+".mp4")
		code, _, errOut = invoke(t, "", "concat", input, input, "--mode", mode, "-o", result)
		if code != 0 {
			t.Fatalf("concat %s: %s", mode, errOut)
		}
		info, err := core.ProbeMediaInfo(result)
		if err != nil || info.Format.DurationSec < 3.9 {
			t.Fatalf("concat did not join inputs: %+v %v", info, err)
		}
	}
	code, out, errOut = invoke(t, "", "history", "list", "--search", "trim", "--json")
	if code != 0 || !json.Valid([]byte(out)) {
		t.Fatalf("history: %s %s", out, errOut)
	}
}

func TestConfigPresetAndDryRunPersistence(t *testing.T) {
	t.Setenv("MEDIACLI_SERVER", "")
	t.Setenv("MEDIACLI_TOKEN", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, language := range []string{"ru", "en"} {
		code, _, errOut := invoke(t, "", "config", "set", "language", language)
		if code != 0 {
			t.Fatal(errOut)
		}
	}
	backups, _ := filepath.Glob(filepath.Join(core.GetConfigDir(), "config.json.bak-*"))
	if len(backups) != 1 {
		t.Fatal("configuration backup missing")
	}
	code, _, errOut := invoke(t, "", "config", "set", "language", "ru", "--dry-run")
	if code != 0 {
		t.Fatal(errOut)
	}
	cfg, _ := core.LoadConfig()
	if cfg.Language != "en" {
		t.Fatal("dry run wrote config")
	}
	file := filepath.Join(t.TempDir(), "preset.json")
	os.WriteFile(file, []byte(`{"name":"Test","operations":[{"type":"trim","config":{"start":"0","end":"1"}}]}`), 0600)
	code, _, errOut = invoke(t, "", "presets", "save", "test", "--file", file)
	if code != 0 {
		t.Fatal(errOut)
	}
	code, out, errOut := invoke(t, "", "presets", "show", "test", "--json")
	if code != 0 || !json.Valid([]byte(out)) {
		t.Fatal(out, errOut)
	}
	code, _, errOut = invoke(t, "", "presets", "delete", "test")
	if code != 0 {
		t.Fatal(errOut)
	}
	if _, ok := core.GetMediaPresetByID("test"); ok {
		t.Fatal("preset not deleted")
	}
}

func TestDaemonBatchQueueLogsRetryAndAuthorization(t *testing.T) {
	t.Setenv("MEDIACLI_SERVER", "")
	t.Setenv("MEDIACLI_TOKEN", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	input := fixture(t)
	dir := t.TempDir()
	server := httptest.NewServer(daemon.NewServer("test-token").Handler())
	defer server.Close()
	common := []string{"--server", server.URL, "--token", "test-token", "--json"}
	code, _, _ := invoke(t, "", append([]string{"queue", "list", "--server", server.URL}, "--json")...)
	if code == 0 {
		t.Fatal("daemon accepted missing token")
	}
	args := []string{"batch", input, "--output-dir", dir, "--op", `transcode={"vcodec":"libx264","acodec":"aac","container":"mp4"}`, "--background"}
	code, out, errOut := invoke(t, "", append(args, common...)...)
	if code != 0 {
		t.Fatalf("remote batch: %s %s", out, errOut)
	}
	var queued struct {
		IDs []int `json:"task_ids"`
	}
	if err := json.Unmarshal([]byte(out), &queued); err != nil || len(queued.IDs) != 1 {
		t.Fatalf("bad IDs: %s", out)
	}
	id := strconvID(queued.IDs[0])
	code, _, errOut = invoke(t, "", append([]string{"queue", "wait", id}, common...)...)
	if code != 0 {
		t.Fatal(errOut)
	}
	code, out, errOut = invoke(t, "", append([]string{"queue", "logs", id}, common...)...)
	if code != 0 || !strings.Contains(out, "logs") {
		t.Fatal(out, errOut)
	}
	code, out, errOut = invoke(t, "", append([]string{"queue", "retry", id, "--background"}, common...)...)
	if code != 0 {
		t.Fatal(out, errOut)
	}
	json.Unmarshal([]byte(out), &queued)
	for _, id := range queued.IDs {
		code, _, errOut = invoke(t, "", append([]string{"queue", "wait", strconvID(id)}, common...)...)
		if code != 0 {
			t.Fatal(errOut)
		}
	}
}

func strconvID(id int) string { return fmt.Sprint(id) }

func TestDownloadFromLocalHTTPAndExternalConversion(t *testing.T) {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skip("yt-dlp unavailable")
	}
	t.Setenv("MEDIACLI_SERVER", "")
	t.Setenv("MEDIACLI_TOKEN", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	input := fixture(t)
	server := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir(input))))
	defer server.Close()
	for _, external := range []bool{false, true} {
		dir := t.TempDir()
		mode := "embedded"
		if external {
			mode = "external"
		}
		args := []string{"download", server.URL + "/clip.mp4", "--output-dir", dir, "--transcode", mode, "--proxy-mode", "none", "--fields", `{"embed_metadata":false,"embed_thumbnail":false,"embed_chapters":false}`, "--json"}
		if external {
			args = append(args, "--preset", "standard_mp4")
		}
		code, out, errOut := invoke(t, "", args...)
		if code != 0 {
			t.Fatalf("download %s: %d %s %s", mode, code, out, errOut)
		}
		files, _ := filepath.Glob(filepath.Join(dir, "*.mp4"))
		if len(files) == 0 {
			t.Fatal("no downloaded files")
		}
		if external {
			found := false
			for _, file := range files {
				found = found || strings.Contains(file, "_converted")
			}
			if !found {
				t.Fatal("external phase did not run")
			}
		}
	}
}

func TestCancellationWaitsForTaskFinalization(t *testing.T) {
	t.Setenv("MEDIACLI_SERVER", "")
	t.Setenv("MEDIACLI_TOKEN", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	input := fixture(t)
	var out, errOut bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	code := RunContext(ctx, []string{"filter", input, "--vf", "realtime", "-o", filepath.Join(t.TempDir(), "cancel.mp4")}, strings.NewReader(""), &out, &errOut)
	if code != 124 {
		t.Fatalf("cancel: %d %s %s", code, out.String(), errOut.String())
	}
	for _, snapshot := range core.GlobalQueue.Snapshots() {
		if snapshot.Status == core.StatusCancelled && !snapshot.Finalized {
			t.Fatalf("exited before cancellation finalized: %+v", snapshot)
		}
	}
}

func TestBuiltInAudioAndRangeExports(t *testing.T) {
	t.Setenv("MEDIACLI_SERVER", "")
	t.Setenv("MEDIACLI_TOKEN", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	input := fixture(t)
	dir := t.TempDir()
	for _, profile := range []string{"audio_mp3", "audio_flac", "audio_wav"} {
		code, _, errOut := invoke(t, "", "convert", input, "--preset", profile)
		if code != 0 {
			t.Fatalf("%s: %s", profile, errOut)
		}
	}
	output := filepath.Join(dir, "ranges.mp4")
	code, _, errOut := invoke(t, "", "ranges", input, "--range", "0..0.5", "--range", "1..1.5", "--join", "-o", output)
	if code != 0 {
		t.Fatal(errOut)
	}
	// The same output requires explicit overwrite and still cannot alias an input.
	code, _, errOut = invoke(t, "", "ranges", input, "--range", "0..0.5", "--join", "-o", output, "--overwrite")
	if code != 0 {
		t.Fatal(errOut)
	}
}
