package ui

import (
	"fmt"
	"mediacli/pkg/core"

	"github.com/gdamore/tcell/v2"
)

func ScreenHistory(s tcell.Screen, cfg *core.Config) {
	entries := core.GetHistory()
	if len(entries) == 0 {
		ShowMessage(s, cfg, T(*cfg, "history_title"), []string{T(*cfg, "history_empty")}, T(*cfg, "footer_message"))
		return
	}
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("[%s] %s (%s)\n  %s -> %s", e.Time, e.Type, e.Status, e.Source, e.Target))
	}
	ShowMessage(s, cfg, T(*cfg, "history_title"), lines, T(*cfg, "footer_message"))
}