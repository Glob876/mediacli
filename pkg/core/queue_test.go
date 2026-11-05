package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Invoke the test executable as a portable child process, without shell tools.
func TestQueueProcessHelper(t *testing.T) {
	if os.Getenv("MEDIACLI_QUEUE_HELPER") != "1" {
		return
	}
	arguments := os.Args
	separator := 0
	for index, argument := range arguments {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator == 0 || len(arguments) < separator+3 {
		os.Exit(2)
	}
	mode := arguments[separator+1]
	path := arguments[separator+2]
	switch mode {
	case "produce":
		time.Sleep(800 * time.Millisecond)
		if err := os.WriteFile(path, []byte("ready"), 0600); err != nil {
			os.Exit(3)
		}
	case "consume":
		if _, err := os.Stat(path); err != nil {
			os.Exit(4)
		}
	case "fail":
		os.Exit(5)
	case "wait":
		fmt.Println("ready to cancel")
		time.Sleep(20 * time.Second)
	case "partial":
		if err := os.WriteFile(path, []byte("partial"), 0600); err != nil {
			os.Exit(3)
		}
		fmt.Println("ready to cancel")
		time.Sleep(20 * time.Second)
	}
	os.Exit(0)
}

func queueHelperCommand(t *testing.T, mode, path string) []string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := []string{executable, "-test.run=^TestQueueProcessHelper$", "--", mode, path}
	return command
}

func waitForQueueState(t *testing.T, queue *BackgroundQueueManager, taskID int, condition func(TaskSnapshot) bool) TaskSnapshot {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, exists := queue.Snapshot(taskID)
		if exists && condition(snapshot) {
			return snapshot
		}
		time.Sleep(10 * time.Millisecond)
	}
	snapshot, _ := queue.Snapshot(taskID)
	t.Fatalf("timed out waiting for task: %+v", snapshot)
	return snapshot
}

func TestQueueChainDependencies(t *testing.T) {
	t.Setenv("MEDIACLI_QUEUE_HELPER", "1")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, firstMode := range []string{"produce", "fail"} {
		t.Run(firstMode, func(t *testing.T) {
			queue := NewBackgroundQueueManager(3)
			marker := filepath.Join(t.TempDir(), "ready")
			commands := [][]string{queueHelperCommand(t, firstMode, marker), queueHelperCommand(t, "consume", marker)}
			tasks := queue.EnqueueChain(commands, "chain", "", "", "reused-group")
			last := waitForQueueState(t, queue, tasks[1].ID, func(task TaskSnapshot) bool {
				return task.Status == StatusDone || task.Status == StatusCancelled || task.Status == StatusFailed
			})
			expected := StatusDone
			if firstMode == "fail" {
				expected = StatusCancelled
			}
			if last.Status != expected {
				t.Fatalf("last step status %s, want %s", last.Status, expected)
			}
			first, _ := queue.Snapshot(tasks[0].ID)
			if firstMode == "produce" && last.StartedAt.Before(first.FinishedAt) {
				t.Fatal("dependent command started before producer finished")
			}
			if queue.CancelTask(tasks[0].ID) {
				t.Fatal("completed task was cancelled")
			}
		})
	}
}

func TestQueueCancellationRunsFailureHook(t *testing.T) {
	t.Setenv("MEDIACLI_QUEUE_HELPER", "1")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	queue := NewBackgroundQueueManager(1)
	directory := t.TempDir()
	source := filepath.Join(directory, "source.mp4")
	if err := os.WriteFile(source, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	config := GetDefaultConfig()
	config.OverwriteOriginal = true
	plan := PrepareFFmpegOutput(source, "mp4", "_out", config)
	completed := make(chan int, 1)
	task := queue.EnqueueWithHook(queueHelperCommand(t, "partial", plan.TempOutputPath), "convert", source, plan.FinalPath, func(code int) error {
		err := plan.OnComplete(code)
		completed <- code
		return err
	})
	waitForQueueState(t, queue, task.ID, func(snapshot TaskSnapshot) bool { return len(snapshot.LogTail) > 0 })
	if !queue.CancelTask(task.ID) {
		t.Fatal("cancel rejected")
	}
	select {
	case code := <-completed:
		if code == 0 {
			t.Fatal("cancellation reported success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("completion hook not called")
	}
	snapshot, _ := queue.Snapshot(task.ID)
	if snapshot.Status != StatusCancelled {
		t.Fatalf("cancellation overwritten: %s", snapshot.Status)
	}
	contents, err := os.ReadFile(source)
	if err != nil || string(contents) != "original" {
		t.Fatalf("source damaged: %q, %v", contents, err)
	}
	if _, err := os.Stat(plan.TempOutputPath); !os.IsNotExist(err) {
		t.Fatalf("partial output not cleaned: %v", err)
	}
	if queue.CancelTask(task.ID) {
		t.Fatal("cancelled task accepted another cancellation")
	}
}

func TestQueuePreservesUnrelatedEmptyFileAndReportsHookError(t *testing.T) {
	t.Setenv("MEDIACLI_QUEUE_HELPER", "1")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	directory := t.TempDir()
	unrelated := filepath.Join(directory, "unrelated.txt")
	if err := os.WriteFile(unrelated, nil, 0600); err != nil {
		t.Fatal(err)
	}
	queue := NewBackgroundQueueManager(1)
	task := queue.EnqueueWithHook(queueHelperCommand(t, "noop", directory), "download", "", directory, func(int) error { return fmt.Errorf("cannot install output") })
	snapshot := waitForQueueState(t, queue, task.ID, func(snapshot TaskSnapshot) bool { return snapshot.Status == StatusFailed })
	if len(snapshot.LogTail) == 0 || snapshot.LogTail[len(snapshot.LogTail)-1] != "[Error] cannot install output" {
		t.Fatalf("missing hook error: %+v", snapshot)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatalf("unrelated file removed: %v", err)
	}
}

func TestQueuedCancellationFinalizesWithoutStarting(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	queue := &BackgroundQueueManager{nextID: 1, maxTasks: 1}
	completed := make(chan int, 1)
	task := queue.EnqueueWithHook([]string{"must-not-start"}, "cancel queued", "", "", func(code int) error {
		completed <- code
		return nil
	})
	if !queue.CancelTask(task.ID) {
		t.Fatal("queued cancellation rejected")
	}
	select {
	case code := <-completed:
		if code == 0 {
			t.Fatal("queued cancellation reported success")
		}
	case <-time.After(time.Second):
		t.Fatal("queued completion hook not called")
	}
	snapshot, _ := queue.Snapshot(task.ID)
	if snapshot.Status != StatusCancelled {
		t.Fatalf("incorrect status: %s", snapshot.Status)
	}
}
