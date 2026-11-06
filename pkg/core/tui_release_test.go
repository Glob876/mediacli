package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestPersistentQueueRecoveryAndSafeRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	output := filepath.Join(t.TempDir(), "out.mp4")
	if err := os.WriteFile(output, []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	queue := &BackgroundQueueManager{nextID: 1}
	if err := queue.EnablePersistence(path); err != nil {
		t.Fatal(err)
	}
	chain := queue.EnqueueChain([][]string{{"ffmpeg", "-y", "-i", "source.mp4", "-f", "null", os.DevNull}, {"ffmpeg", "-n", "-i", "source.mp4", output}}, "Encode", "source.mp4", output, "chain")
	queue.mu.Lock()
	chain[0].Status = StatusRunning
	queue.persistLocked()
	queue.mu.Unlock()
	restored := &BackgroundQueueManager{nextID: 1}
	if err := restored.EnablePersistence(path); err != nil {
		t.Fatal(err)
	}
	if len(restored.tasks) != 2 || restored.tasks[0].Status != StatusInterrupted || restored.tasks[1].Status != StatusInterrupted {
		t.Fatalf("unexpected recovery: %+v", restored.tasks)
	}
	retried, err := restored.RetryTask(chain[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(retried) != 2 || retried[1].previousTaskID != retried[0].ID {
		t.Fatal("lost dependencies")
	}
	last := retried[1].Cmd
	if last[len(last)-1] == output {
		t.Fatal("retry would overwrite partial media")
	}
	if data, _ := os.ReadFile(output); string(data) != "partial" {
		t.Fatal("partial media changed")
	}
	stat, _ := os.Stat(path)
	if stat.Mode().Perm()&0077 != 0 {
		t.Fatal("queue contains credentials but is not private")
	}
}

func TestRecoveryKeepsReplacementPlan(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "source.mp4")
	os.WriteFile(source, []byte("original"), 0600)
	cfg := GetDefaultConfig()
	cfg.OverwriteOriginal = true
	plan := PrepareFFmpegOutput(source, "mp4", "_out", cfg)
	queue := &BackgroundQueueManager{nextID: 1}
	path := filepath.Join(t.TempDir(), "queue.json")
	queue.EnablePersistence(path)
	task := queue.EnqueueWithHook([]string{"ffmpeg", "-i", source, plan.TempOutputPath}, "encode", source, plan.FinalPath, plan.OnComplete)
	restored := &BackgroundQueueManager{nextID: 1}
	if err := restored.EnablePersistence(path); err != nil {
		t.Fatal(err)
	}
	retried, err := restored.RetryTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	replacement := retried[0].Replacement
	if replacement == nil || replacement.Temporary == plan.TempOutputPath {
		t.Fatal("replacement plan not rebuilt")
	}
	os.WriteFile(replacement.Temporary, []byte("new"), 0600)
	if err := replacement.Complete(0); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(source); string(data) != "new" {
		t.Fatal("finalization lost")
	}
}

func TestConcurrentHistoryRetainsEntries(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var workers sync.WaitGroup
	for index := 0; index < 80; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			if err := AddHistoryEntry("test", fmt.Sprint(index), "out", "Success"); err != nil {
				t.Error(err)
			}
		}(index)
	}
	workers.Wait()
	if entries := GetHistory(); len(entries) != 80 {
		t.Fatalf("lost entries: %d", len(entries))
	}
	path := filepath.Join(GetConfigDir(), "history.json")
	os.WriteFile(path, []byte("broken"), 0600)
	if err := AddHistoryEntry("test", "x", "y", "Success"); err == nil {
		t.Fatal("must not overwrite corrupt history")
	}
}

func TestBatchReportsCollisionsAndMissingFiles(t *testing.T) {
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "same.mp4"), filepath.Join(dir, "same.mkv")}
	for _, path := range paths {
		os.WriteFile(path, nil, 0600)
	}
	paths = append(paths, filepath.Join(dir, "missing.mp4"))
	report := PlanBatch(BatchJob{Inputs: paths, Pipeline: Pipeline{Operations: []Operation{&TranscodeOp{Container: "mp4", VCodec: "libx264"}}}}, GetDefaultConfig())
	if len(report.Items) != 3 {
		t.Fatal("missing report entries")
	}
	for _, item := range report.Items {
		if item.Error == "" {
			t.Fatalf("silent skip/conflict: %+v", item)
		}
	}
	if _, err := EnqueueBatch(BatchJob{}, GetDefaultConfig()); err == nil {
		t.Fatal("empty batch accepted")
	}
}

func TestMediaExportsWithRealFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe unavailable")
	}
	cfg := GetDefaultConfig()
	dir := t.TempDir()
	input := filepath.Join(dir, "input.mp4")
	generate := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=128x128:rate=25:duration=3", "-f", "lavfi", "-i", "sine=frequency=440:duration=3", "-c:v", "libx264", "-c:a", "aac", "-shortest", input)
	if out, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("generate: %v %s", err, out)
	}
	run := func(commands [][]string) {
		t.Helper()
		for _, command := range commands {
			if out, err := exec.Command(command[0], command[1:]...).CombinedOutput(); err != nil {
				t.Fatalf("export: %v %s", err, out)
			}
		}
	}
	for _, join := range []bool{false, true} {
		commands, err := BuildRangeExport(input, cfg, []MediaRange{{Start: 0.2, End: 0.7}, {Start: 1.2, End: 1.7}}, join)
		if err != nil {
			t.Fatal(err)
		}
		run(commands)
		if join {
			last := commands[0]
			probe, err := ProbeMediaInfoWithConfig(last[len(last)-1], cfg)
			if err != nil || probe.Format.DurationSec < 0.9 || probe.Format.DurationSec > 1.2 {
				t.Fatalf("bad joined duration: %+v %v", probe, err)
			}
		}
	}
	levels, err := Waveform(input, cfg, 0, 60)
	if err != nil {
		t.Fatal(err)
	}
	peak := 0.0
	for _, level := range levels {
		if level > peak {
			peak = level
		}
	}
	if peak <= 0 {
		t.Fatal("empty waveform")
	}
	commands, err := BuildSizeExport(Pipeline{Inputs: []string{input}}, cfg, 0.15, true)
	if err != nil {
		t.Fatal(err)
	}
	run(commands)
	CleanupPassLogs(commands[1])
	output := commands[1][len(commands[1])-1]
	stat, err := os.Stat(output)
	if err != nil || stat.Size() > 180000 || stat.Size() < 30000 {
		t.Fatalf("size export: %+v %v", stat, err)
	}
	// Snapshots preserve concrete operation types across draft serialization.
	data, _ := MarshalPipeline(Pipeline{Inputs: []string{input}, Operations: []Operation{&TrimOp{Start: "0", End: "1"}}})
	restored, err := UnmarshalPipeline(data)
	if err != nil || restored.Operations[0].Type() != "trim" {
		t.Fatal("draft round trip")
	}
	if _, err := BuildSizeExport(Pipeline{Inputs: []string{input}}, cfg, 0.0001, true); err == nil {
		t.Fatal("impossible budget accepted")
	}
}

func TestSavedCommandsArePrivateAndMaskedForDisplay(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := addHistoryEntry("download", "url", "dir", "Success", [][]string{{"yt-dlp", "--proxy", "http://user:secret@localhost:8080", "url"}}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(GetConfigDir(), "history.json"))
	if !json.Valid(data) {
		t.Fatal("bad history JSON")
	}
	display := strings.Join(MaskCmdForLog(GetHistory()[0].Commands[0]), " ")
	if strings.Contains(display, "secret") {
		t.Fatalf("leaked credential: %s", display)
	}
}

func TestSizeExportTrimDoesNotUseCRF(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	input := filepath.Join(t.TempDir(), "input.mp4")
	out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=64x64:rate=25:duration=2", "-c:v", "libx264", input).CombinedOutput()
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	commands, err := BuildSizeExport(Pipeline{Inputs: []string{input}, Operations: []Operation{&TrimOp{Start: "0.5", End: "1.5", Mode: "copy"}}}, GetDefaultConfig(), 0.1, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range commands {
		for _, arg := range command {
			if arg == "-crf" {
				t.Fatal("CRF overrides target bitrate")
			}
		}
		if out, err := exec.Command(command[0], command[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("trimmed size export: %v %s", err, out)
		}
	}
	CleanupPassLogs(commands[1])
}

func TestRangeExportSingleStreamInputs(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	for _, audio := range []bool{true, false} {
		t.Run(fmt.Sprintf("audio=%v", audio), func(t *testing.T) {
			input := filepath.Join(t.TempDir(), "input.mkv")
			source := "testsrc2=size=64x64:rate=25:duration=2"
			if audio {
				source = "sine=duration=2"
			}
			if output, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", source, input).CombinedOutput(); err != nil {
				t.Fatalf("%v %s", err, output)
			}
			commands, err := BuildRangeExport(input, GetDefaultConfig(), []MediaRange{{Start: 0, End: 0.5}, {Start: 1, End: 1.5}}, true)
			if err != nil {
				t.Fatal(err)
			}
			command := commands[0]
			if output, err := exec.Command(command[0], command[1:]...).CombinedOutput(); err != nil {
				t.Fatalf("%v %s", err, output)
			}
		})
	}
}
