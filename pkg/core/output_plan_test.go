package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOutputReplacementPreservesSourceOnFailure(t *testing.T) {
	for _, scenario := range []string{"missing", "empty", "rename failure", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			directory := t.TempDir()
			source := filepath.Join(directory, "source.mp4")
			original := []byte("original media")
			if err := os.WriteFile(source, original, 0600); err != nil {
				t.Fatal(err)
			}
			config := GetDefaultConfig()
			config.OverwriteOriginal = true
			plan := PrepareFFmpegOutput(source, "mp4", "_out", config)
			exitCode := 0
			switch scenario {
			case "empty":
				if err := os.WriteFile(plan.TempOutputPath, nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "rename failure":
				if err := os.WriteFile(plan.TempOutputPath, []byte("converted"), 0600); err != nil {
					t.Fatal(err)
				}
				// Use a different-extension plan to leave the original source in place.
				plan = PrepareFFmpegOutput(source, "blocked", "_out", config)
				if err := os.Mkdir(plan.FinalPath, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(plan.TempOutputPath, []byte("converted"), 0600); err != nil {
					t.Fatal(err)
				}
			case "cancelled":
				exitCode = -1
				if err := os.WriteFile(plan.TempOutputPath, []byte("partial"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			completionError := plan.OnComplete(exitCode)
			if scenario != "cancelled" && completionError == nil {
				t.Fatal("expected finalization error")
			}
			contents, err := os.ReadFile(source)
			if err != nil || string(contents) != string(original) {
				t.Fatalf("source damaged: %q, %v", contents, err)
			}
			if scenario == "cancelled" {
				if _, err := os.Stat(plan.TempOutputPath); !os.IsNotExist(err) {
					t.Fatalf("partial output retained: %v", err)
				}
			}
		})
	}
}

func TestOutputReplacementSuccess(t *testing.T) {
	for _, extension := range []string{"mp4", "mkv"} {
		t.Run(extension, func(t *testing.T) {
			directory := t.TempDir()
			source := filepath.Join(directory, "source.mp4")
			if err := os.WriteFile(source, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			config := GetDefaultConfig()
			config.OverwriteOriginal = true
			plan := PrepareFFmpegOutput(source, extension, "_out", config)
			if err := os.WriteFile(plan.TempOutputPath, []byte("converted"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := plan.OnComplete(0); err != nil {
				t.Fatal(err)
			}
			contents, err := os.ReadFile(plan.FinalPath)
			if err != nil || string(contents) != "converted" {
				t.Fatalf("unexpected result: %q, %v", contents, err)
			}
			if extension != "mp4" {
				if _, err := os.Stat(source); !os.IsNotExist(err) {
					t.Fatal("original not removed after successful conversion")
				}
			}
		})
	}
}
