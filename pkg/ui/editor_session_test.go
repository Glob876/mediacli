package ui

import (
	"github.com/gdamore/tcell/v2"
	"mediacli/pkg/core"
	"os"
	"path/filepath"
	"testing"
)

func TestEditorUndoRedoAndRestore(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "input.mp4")
	os.WriteFile(path, nil, 0600)
	session := newEditorSession("test")
	if err := session.capture(path, nil); err != nil {
		t.Fatal(err)
	}
	op := &core.TrimOp{Start: "0", End: "2", Mode: "copy"}
	if err := session.capture(path, []core.Operation{op}); err != nil {
		t.Fatal(err)
	}
	// Mutating the live object cannot change a stored undo snapshot.
	op.End = "3"
	session.capture(path, []core.Operation{op})
	_, ops, err := session.move(false)
	if err != nil || ops[0].(*core.TrimOp).End != "2" {
		t.Fatalf("undo: %v %+v", err, ops)
	}
	_, ops, err = session.move(true)
	if err != nil || ops[0].(*core.TrimOp).End != "3" {
		t.Fatal("redo failed")
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	screen.Init()
	defer screen.Fini()
	screen.SetSize(80, 24)
	screen.PostEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	config := core.GetDefaultConfig()
	restoredPath, restoredOps := newEditorSession("test").restore(screen, &config)
	if restoredPath != path || len(restoredOps) != 1 {
		t.Fatal("draft not restored through TUI")
	}
	screen.PostEvent(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModNone))
	if choice := runMenu(screen, &config, "Editor", []string{"Action"}, "", "", true); choice != -2 {
		t.Fatal("Ctrl+Z not delivered to editor")
	}
	session.move(false)
	session.capture(path, nil)
	if _, _, err := session.move(true); err == nil {
		t.Fatal("new edit must discard redo branch")
	}
}

func TestUniversalEditorKeyboardChangesPersist(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "input.mp4")
	os.WriteFile(path, nil, 0600)
	screen := tcell.NewSimulationScreen("UTF-8")
	screen.Init()
	defer screen.Fini()
	screen.SetSize(90, 32)
	for i := 0; i < 4; i++ {
		screen.PostEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	for _, key := range []tcell.Key{tcell.KeyEnter, tcell.KeyEnter, tcell.KeyCtrlZ, tcell.KeyCtrlY, tcell.KeyEscape} {
		screen.PostEvent(tcell.NewEventKey(key, 0, tcell.ModNone))
	}
	cfg := core.GetDefaultConfig()
	ScreenUniversal(screen, &cfg, path)
	data, err := os.ReadFile(filepath.Join(core.GetConfigDir(), "editor_draft.json"))
	if err != nil {
		t.Fatal(err)
	}
	draft, err := core.UnmarshalPipeline(data)
	if err != nil || len(draft.Operations) != 1 || draft.Operations[0].Type() != "transcode" {
		t.Fatalf("TUI did not save redo state: %s %v", data, err)
	}
}
