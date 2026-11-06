package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"mediacli/pkg/core"
)

func TestPaletteSearchesRussianAndEnglish(t *testing.T) {
	config := core.GetDefaultConfig()
	config.Language = "ru"
	commands := paletteCommands(config)
	for query, expectedID := range map[string]string{"CONVERT": "convert", "пропорции": "aspect", "cancel": "queue", "ffmpeg yt-dlp": "doctor"} {
		matches := filterPaletteCommands(commands, query)
		found := false
		for _, command := range matches {
			if command.ID == expectedID {
				found = true
			}
		}
		if !found {
			t.Errorf("query %q did not find %s", query, expectedID)
		}
	}
	if matches := filterPaletteCommands(commands, "no-such-command-123"); len(matches) != 0 {
		t.Fatalf("unexpected matches: %v", matches)
	}
	seen := make(map[string]bool)
	for _, command := range commands {
		if seen[command.ID] || command.Action == nil || command.Title == "" || command.Description == "" {
			t.Fatalf("invalid command: %+v", command)
		}
		seen[command.ID] = true
	}
}

func TestPaletteSelectsCommandAndExecutesAction(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)
	executed := ""
	commands := []PaletteCommand{
		{ID: "one", Title: "First command", Description: "description", Action: func(tcell.Screen, *core.Config) { executed = "one" }},
		{ID: "two", Title: "Second command", Description: "description", Action: func(tcell.Screen, *core.Config) { executed = "two" }},
	}
	screen.PostEvent(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone))
	screen.PostEvent(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone))
	screen.PostEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	config := core.GetDefaultConfig()
	selected, confirmed := choosePaletteCommand(screen, &config, commands, nil)
	if !confirmed || selected.ID != "two" {
		t.Fatalf("wrong selection: %+v, %v", selected, confirmed)
	}
	selected.Action(screen, &config)
	if executed != "two" {
		t.Fatal("selected action was not executed")
	}
	cells, width, height := screen.GetContents()
	var contents strings.Builder
	for _, cell := range cells {
		contents.WriteString(string(cell.Runes))
	}
	if !strings.Contains(contents.String(), "Second command") {
		t.Fatal("selected result was not rendered")
	}
	if width != 100 || height != 30 {
		t.Fatal("palette changed the terminal size")
	}
}

func TestPaletteKeyboardAndTinyScreens(t *testing.T) {
	tests := []struct {
		key       tcell.Key
		character rune
		modifiers tcell.ModMask
		want      bool
	}{
		{tcell.KeyF12, 0, tcell.ModNone, true},
		{tcell.KeyCtrlP, 0, tcell.ModCtrl, true},
		{tcell.KeyRune, 'p', tcell.ModAlt, false},
		{tcell.KeyRune, 'P', tcell.ModAlt | tcell.ModShift, false},
		{tcell.KeyRune, 'p', tcell.ModNone, false},
	}
	for _, test := range tests {
		event := tcell.NewEventKey(test.key, test.character, test.modifiers)
		if result := CheckPaletteHotkey(event); result != test.want {
			t.Errorf("hotkey %v: got %v want %v", event, result, test.want)
		}
	}
	for _, size := range [][2]int{{1, 1}, {3, 2}, {30, 8}, {100, 30}} {
		screen := tcell.NewSimulationScreen("UTF-8")
		if err := screen.Init(); err != nil {
			t.Fatal(err)
		}
		screen.SetSize(size[0], size[1])
		screen.PostEvent(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
		config := core.GetDefaultConfig()
		GlobalPalette.Open(screen, &config)
		screen.Fini()
	}
}

func TestPaletteEmptyResultsDoNotExecute(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	screen.PostEvent(tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModNone))
	screen.PostEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	screen.PostEvent(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	config := core.GetDefaultConfig()
	_, confirmed := choosePaletteCommand(screen, &config, []PaletteCommand{{ID: "one", Title: "First"}}, nil)
	if confirmed {
		t.Fatal("empty search executed an action")
	}
}
