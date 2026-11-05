package ui

import (
	"fmt"
	"strings"
	"testing"

	"mediacli/pkg/core"

	"github.com/gdamore/tcell/v2"
)

func TestMenuScrollsToSelectedItem(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 10)
	items := make([]string, 12)
	for index := range items {
		items[index] = fmt.Sprintf("item %02d", index)
	}
	for index := 0; index < 7; index++ {
		screen.PostEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	screen.PostEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	config := core.GetDefaultConfig()
	selected := RunMenu(screen, &config, "Menu", items, "", "footer")
	if selected != 7 {
		t.Fatalf("selected %d", selected)
	}
	cells, _, _ := screen.GetContents()
	var contents strings.Builder
	for _, cell := range cells {
		contents.WriteString(string(cell.Runes))
	}
	if !strings.Contains(contents.String(), "> item 07") {
		t.Fatalf("selected item is invisible: %s", contents.String())
	}
}
