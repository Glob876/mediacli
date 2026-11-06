package ui

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"mediacli/pkg/core"
	"strings"
)

func ScreenHistory(s tcell.Screen, cfg *core.Config) {
	query := ""
	title := T(*cfg, "history_title")
	for {
		entries := core.GetHistory()
		filtered := make([]core.HistoryEntry, 0, len(entries))
		labels := []string{paletteText(*cfg, "Поиск…", "Search…")}
		for _, entry := range entries {
			text := fmt.Sprintf("[%s] %s (%s) %s → %s", entry.Time, entry.Type, entry.Status, entry.Source, entry.Target)
			if strings.Contains(strings.ToLower(text), strings.ToLower(query)) {
				filtered = append(filtered, entry)
				labels = append(labels, text)
			}
		}
		index := RunMenu(s, cfg, title, labels, query, T(*cfg, "footer_nav"))
		if index < 0 {
			return
		}
		if index == 0 {
			value, ok := TextInput(s, cfg, title, paletteText(*cfg, "Поиск по источнику, результату, дате или статусу:", "Search source, output, date or status:"), query, T(*cfg, "footer_input"))
			if ok {
				query = value
			}
			continue
		}
		entry := filtered[index-1]
		actions := []string{paletteText(*cfg, "Подробности", "Details")}
		if len(entry.Commands) > 0 {
			actions = append(actions, paletteText(*cfg, "Повторить операцию", "Repeat operation"))
		}
		action := RunMenu(s, cfg, title, actions, entry.Source, T(*cfg, "footer_nav"))
		if action == 0 {
			lines := []string{entry.Time, entry.Type, entry.Status, entry.Source, entry.Target}
			for _, command := range entry.Commands {
				lines = append(lines, strings.Join(core.MaskCmdForLog(command), " "))
			}
			ShowMessage(s, cfg, title, lines, T(*cfg, "footer_message"))
		}
		if action == 1 {
			_, err := core.GlobalQueue.ReplayCommands(entry.Commands, entry.Type, entry.Source, entry.Target)
			message := paletteText(*cfg, "Добавлено в очередь", "Added to queue")
			if err != nil {
				message = err.Error()
			}
			ShowMessage(s, cfg, title, []string{message}, T(*cfg, "footer_message"))
		}
	}
}
