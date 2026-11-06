package ui

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"mediacli/pkg/core"
	"strconv"
	"strings"
	"time"
)

func screenSizeExport(s tcell.Screen, cfg *core.Config, path string, ops []core.Operation, session *editorSession) {
	title := paletteText(*cfg, "Экспорт под размер", "Export to size")
	defaultSize := "50"
	if session.output.TargetMegabytes > 0 {
		defaultSize = strconv.FormatFloat(session.output.TargetMegabytes, 'f', -1, 64)
	}
	value, ok := TextInput(s, cfg, title, paletteText(*cfg, "Размер в МБ (1 МБ = 1 000 000 байт):", "Size in MB (1 MB = 1,000,000 bytes):"), defaultSize, T(*cfg, "footer_input"))
	if !ok {
		return
	}
	size, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		ShowMessage(s, cfg, title, []string{err.Error()}, T(*cfg, "footer_message"))
		return
	}
	modes := []string{paletteText(*cfg, "Два прохода — точнее размер", "Two passes — better size accuracy"), paletteText(*cfg, "Один проход — быстрее", "One pass — faster")}
	twoPassModes := []bool{true, false}
	if session.output.TargetMegabytes > 0 && !session.output.TwoPass {
		modes[0], modes[1] = modes[1], modes[0]
		twoPassModes[0], twoPassModes[1] = false, true
	}
	mode := RunMenu(s, cfg, title, modes, paletteText(*cfg, "H.264 + AAC. Размер приблизительный, с резервом 4%.", "H.264 + AAC. Estimated size, with a 4% reserve."), T(*cfg, "footer_nav"))
	if mode < 0 {
		return
	}
	commands, err := core.BuildSizeExport(core.Pipeline{Inputs: []string{path}, Operations: ops}, *cfg, size, twoPassModes[mode])
	if err != nil {
		ShowMessage(s, cfg, title, []string{err.Error()}, T(*cfg, "footer_message"))
		return
	}
	session.output.TargetMegabytes = size
	session.output.TwoPass = twoPassModes[mode]
	if err := session.capture(path, ops); err != nil {
		ShowMessage(s, cfg, title, []string{err.Error()}, T(*cfg, "footer_message"))
		return
	}
	target := commands[len(commands)-1][len(commands[len(commands)-1])-1]
	core.GlobalQueue.EnqueueChain(commands, title, path, target, fmt.Sprintf("size_%d", time.Now().UnixNano()))
	ShowMessage(s, cfg, title, []string{paletteText(*cfg, "Экспорт поставлен в очередь", "Export queued"), target}, T(*cfg, "footer_message"))
}
