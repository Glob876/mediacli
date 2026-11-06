package ui

import (
	"encoding/json"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"math"
	"mediacli/pkg/core"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type rangeDraft struct {
	Path   string
	Ranges []core.MediaRange
}

func screenRanges(s tcell.Screen, cfg *core.Config, path string) {
	title := paletteText(*cfg, "Фрагменты", "Ranges")
	info, err := core.ProbeMediaInfoWithConfig(path, *cfg)
	if err != nil {
		ShowMessage(s, cfg, title, []string{err.Error()}, T(*cfg, "footer_message"))
		return
	}
	duration := info.Format.DurationSec
	if duration <= 0 {
		ShowMessage(s, cfg, title, []string{"Unknown duration"}, T(*cfg, "footer_message"))
		return
	}
	fps := 25.0
	for _, stream := range info.Streams {
		if stream.CodecType == "video" {
			parts := strings.Split(stream.AvgFrameRate, "/")
			if len(parts) == 2 {
				n, _ := strconv.ParseFloat(parts[0], 64)
				d, _ := strconv.ParseFloat(parts[1], 64)
				if n > 0 && d > 0 {
					fps = n / d
				}
			}
			break
		}
	}
	draftPath := filepath.Join(core.GetConfigDir(), "ranges_draft.json")
	draft := rangeDraft{Path: path}
	if data, err := os.ReadFile(draftPath); err == nil {
		var saved rangeDraft
		if json.Unmarshal(data, &saved) == nil && saved.Path == path && core.ValidateRanges(saved.Ranges, duration) == nil {
			draft = saved
		}
	}
	cursor, mark := 0.0, 0.0
	window := -1.0
	var waveform []float64
	waveError := ""
	var undo, redo [][]core.MediaRange
	save := func() {
		data, _ := json.Marshal(draft)
		if err := core.WritePrivateFile(draftPath, data); err != nil {
			ShowMessage(s, cfg, title, []string{err.Error()}, T(*cfg, "footer_message"))
		}
	}
	remember := func() { undo = append(undo, append([]core.MediaRange(nil), draft.Ranges...)); redo = nil }
	for {
		w, h := s.Size()
		columns := max(1, min(100, w-8))
		nextWindow := math.Floor(math.Max(0, cursor-0.000001)/60) * 60
		span := math.Min(60, duration-nextWindow)
		if nextWindow != window || len(waveform) != columns {
			window = nextWindow
			waveform, err = core.Waveform(path, *cfg, window, columns)
			waveError = ""
			if err != nil {
				waveError = paletteText(*cfg, "Звуковая волна недоступна (нет аудио или ошибка FFmpeg)", "Waveform unavailable (no audio or FFmpeg error)")
			}
		}
		s.Clear()
		DrawHeader(s, title, w, *cfg)
		DrawString(s, 4, 3, filepath.Base(path), w-8, GetBaseStyle(*cfg))
		DrawString(s, 4, 4, fmt.Sprintf("%.3f / %.3f s · IN %.3f · %.3f FPS", cursor, duration, mark, fps), w-8, GetBaseStyle(*cfg))
		glyphs := []rune(" ▁▂▃▄▅▆▇█")
		var wave strings.Builder
		for _, level := range waveform {
			index := int(math.Min(8, math.Max(0, level*8)))
			wave.WriteRune(glyphs[index])
		}
		DrawString(s, 4, 6, wave.String(), w-8, GetBaseStyle(*cfg))
		DrawString(s, 4+min(columns-1, int((cursor-window)/span*float64(columns))), 7, "▲", 1, GetHighlightStyle(*cfg))
		DrawString(s, 4, 8, fmt.Sprintf("%.0f–%.0f s %s", window, math.Min(window+60, duration), waveError), w-8, GetDimStyle(*cfg))
		start := max(0, len(draft.Ranges)-max(1, h-15))
		for index := start; index < len(draft.Ranges); index++ {
			r := draft.Ranges[index]
			DrawString(s, 4, 10+index-start, fmt.Sprintf("%d. %.3f → %.3f", index+1, r.Start, r.End), w-8, GetBaseStyle(*cfg))
		}
		DrawString(s, 4, h-3, "Ctrl+Z / Ctrl+Y · G: seconds · Home / End", w-8, GetDimStyle(*cfg))
		DrawString(s, 4, h-4, paletteText(*cfg, "←/→ кадр · ↑/↓ 1с · I начало · O добавить · D удалить", "←/→ frame · ↑/↓ 1s · I in · O add · D delete"), w-8, GetDimStyle(*cfg))
		footer := paletteText(*cfg, "Enter — экспорт · Esc — назад", "Enter — export · Esc — back")
		DrawFooter(s, footer, w, h, *cfg)
		s.Show()
		event, resized := pollUI(s)
		if resized {
			continue
		}
		key, ok := event.(*tcell.EventKey)
		if !ok {
			continue
		}
		switch key.Key() {
		case tcell.KeyEscape:
			return
		case tcell.KeyLeft:
			cursor -= 1 / fps
		case tcell.KeyRight:
			cursor += 1 / fps
		case tcell.KeyUp:
			cursor += 1
		case tcell.KeyDown:
			cursor -= 1
		case tcell.KeyHome:
			cursor = 0
		case tcell.KeyEnd:
			cursor = duration
		case tcell.KeyCtrlZ:
			if len(undo) > 0 {
				redo = append(redo, append([]core.MediaRange(nil), draft.Ranges...))
				draft.Ranges = undo[len(undo)-1]
				undo = undo[:len(undo)-1]
				save()
			}
		case tcell.KeyCtrlY:
			if len(redo) > 0 {
				undo = append(undo, append([]core.MediaRange(nil), draft.Ranges...))
				draft.Ranges = redo[len(redo)-1]
				redo = redo[:len(redo)-1]
				save()
			}
		case tcell.KeyEnter:
			mode := RunMenu(s, cfg, title, []string{paletteText(*cfg, "Сохранить отдельно", "Save separately"), paletteText(*cfg, "Склеить в указанном порядке", "Join in listed order")}, paletteText(*cfg, "Точная обрезка с перекодированием. Действия основного редактора не применяются.", "Accurate cuts with re-encoding. Main editor actions are not applied."), T(*cfg, "footer_nav"))
			if mode < 0 {
				continue
			}
			commands, err := core.BuildRangeExport(path, *cfg, draft.Ranges, mode == 1)
			if err != nil {
				ShowMessage(s, cfg, title, []string{err.Error()}, T(*cfg, "footer_message"))
				continue
			}
			for _, command := range commands {
				core.GlobalQueue.Enqueue(command, title, path, command[len(command)-1])
			}
			ShowMessage(s, cfg, title, []string{fmt.Sprintf("%d tasks", len(commands))}, T(*cfg, "footer_message"))
		case tcell.KeyRune:
			switch key.Rune() {
			case 'i', 'I':
				mark = cursor
			case 'o', 'O':
				if cursor > mark {
					remember()
					draft.Ranges = append(draft.Ranges, core.MediaRange{Start: mark, End: cursor})
					save()
				}
			case 'd', 'D':
				if len(draft.Ranges) > 0 {
					remember()
					draft.Ranges = draft.Ranges[:len(draft.Ranges)-1]
					save()
				}
			case 'g', 'G':
				value, ok := TextInput(s, cfg, title, paletteText(*cfg, "Позиция в секундах:", "Position in seconds:"), fmt.Sprintf("%.3f", cursor), T(*cfg, "footer_input"))
				if ok {
					position, err := strconv.ParseFloat(value, 64)
					if err == nil && !math.IsNaN(position) && !math.IsInf(position, 0) {
						cursor = position
					}
				}
			}
		}
		cursor = math.Min(duration, math.Max(0, cursor))
	}
}
