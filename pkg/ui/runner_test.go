package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mediacli/pkg/core"

	"github.com/gdamore/tcell/v2"
)

func TestUIProcessHelper(t *testing.T) {
	if os.Getenv("MEDIACLI_UI_HELPER") != "1" {
		return
	}
	path := os.Args[len(os.Args)-1]
	if err := os.WriteFile(path, []byte("converted output"), 0600); err != nil {
		os.Exit(3)
	}
	fmt.Println("ready")
	if os.Getenv("MEDIACLI_UI_WAIT") == "1" {
		time.Sleep(20 * time.Second)
	}
	os.Exit(0)
}

func TestRunnerCancelAndBackgroundFinalization(t *testing.T) {
	for _, mode := range []string{"cancel", "background"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("MEDIACLI_UI_HELPER", "1")
			t.Setenv("MEDIACLI_UI_WAIT", "0")
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			if mode == "cancel" {
				t.Setenv("MEDIACLI_UI_WAIT", "1")
			}
			config := core.GetDefaultConfig()
			config.OverwriteOriginal = true
			source := filepath.Join(t.TempDir(), "source.mp4")
			if err := os.WriteFile(source, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			plan := core.PrepareFFmpegOutput(source, "mp4", "_out", config)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := []string{executable, "-test.run=^TestUIProcessHelper$", "--", plan.TempOutputPath}
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatal(err)
			}
			screen.SetSize(80, 24)
			finalized := make(chan int, 1)
			returned := make(chan struct{})
			go func() {
				RunWithLogHook(screen, &config, command, t.Name(), source, plan.FinalPath, func(code int) error {
					finalizationError := plan.OnComplete(code)
					finalized <- code
					return finalizationError
				})
				close(returned)
			}()
			if mode == "cancel" {
				deadline := time.Now().Add(6 * time.Second)
				ready := false
				for time.Now().Before(deadline) && !ready {
					for _, task := range core.GlobalQueue.Snapshots() {
						if task.Title == t.Name() && len(task.LogTail) > 0 {
							ready = true
							break
						}
					}
					if !ready {
						time.Sleep(10 * time.Millisecond)
					}
				}
				if !ready {
					t.Fatal("child did not start")
				}
				screen.PostEvent(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone))
			} else {
				screen.PostEvent(tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone))
				screen.PostEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
			}
			select {
			case code := <-finalized:
				if (mode == "cancel" && code == 0) || (mode == "background" && code != 0) {
					t.Fatalf("unexpected completion code %d for %s", code, mode)
				}
			case <-time.After(8 * time.Second):
				t.Fatal("completion hook was lost")
			}
			if mode == "cancel" {
				screen.PostEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
			}
			select {
			case <-returned:
			case <-time.After(3 * time.Second):
				t.Fatal("runner did not return")
			}
			expected := "converted output"
			if mode == "cancel" {
				expected = "original"
			}
			contents, err := os.ReadFile(source)
			if err != nil || string(contents) != expected {
				t.Fatalf("unexpected source: %q, %v", contents, err)
			}
			screen.PostEvent(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
			time.Sleep(80 * time.Millisecond)
			if !screen.HasPendingEvent() {
				t.Fatal("a leaked reader consumed the next screen's event")
			}
			event := screen.PollEvent()
			key, ok := event.(*tcell.EventKey)
			if !ok || key.Rune() != 'x' {
				t.Fatalf("unexpected remaining event %v", event)
			}
			screen.Fini()
		})
	}
}
