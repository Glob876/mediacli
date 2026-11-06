package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"mediacli/pkg/core"
)

func TestDrawStringUsesDisplayWidth(t *testing.T) {
	if StringWidth("你") != 2 {
		t.Fatalf("width = %d", StringWidth("你"))
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(10, 3)
	DrawString(screen, 0, 0, "你a", 2, tcell.StyleDefault)
	screen.Show()
	main, _, _, _ := screen.GetContent(0, 0)
	if main != '你' {
		t.Fatalf("first cell %q", string(main))
	}
	next, _, _, _ := screen.GetContent(2, 0)
	if next == 'a' {
		t.Fatal("narrow character was drawn over the wide rune")
	}
}

func TestFooterKeepsSummaryClearOfTheHint(t *testing.T) {
	textLimit, summaryX := footerLayout("keyboard help that is quite long", "queue ● 2", 40)
	if summaryX < 0 || textLimit <= 0 || textLimit > summaryX-2 {
		t.Fatalf("layout text=%d summary=%d", textLimit, summaryX)
	}
	if StringWidth("queue ● 2") != 40-2-summaryX {
		t.Fatalf("summary anchor %d", summaryX)
	}
}

func TestTextInputEditsByCellsAndScrolls(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(24, 12)
	screen.EnablePaste()
	long := strings.Repeat("п", 30)
	events := []tcell.Event{
		tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone),
		tcell.NewEventPaste(true),
		tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModNone),
		tcell.NewEventPaste(false),
		tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone),
	}
	go func() {
		for _, event := range events {
			screen.PostEventWait(event)
		}
	}()
	config := core.GetDefaultConfig()
	value, ok := TextInput(screen, &config, "Title", "Prompt", long, "footer")
	if !ok || value != "ab xyz" {
		t.Fatalf("edited value %q ok=%v", value, ok)
	}
	start := inputWindowStart([]rune(long), len([]rune(long)), 8, false)
	if start == 0 || displayWidth([]rune(long)[start:], false) > 8 {
		t.Fatalf("window start %d", start)
	}
}

func TestSecretInputHidesThePassword(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(40, 12)
	screen.PostEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	config := core.GetDefaultConfig()
	value, ok := TextInputSecret(screen, &config, "Title", "Prompt", "secret", "footer")
	if !ok || value != "secret" {
		t.Fatalf("value %q", value)
	}
	var contents strings.Builder
	cells, _, _ := screen.GetContents()
	for _, cell := range cells {
		contents.WriteString(string(cell.Runes))
	}
	if strings.Contains(contents.String(), "secret") {
		t.Fatal("password was rendered in clear text")
	}
	if !strings.Contains(contents.String(), "••••••") {
		t.Fatal("password mask was not rendered")
	}
}
