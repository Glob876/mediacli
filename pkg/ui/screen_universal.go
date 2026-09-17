package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mediacli/pkg/core"

	"github.com/gdamore/tcell/v2"
)

// ScreenUniversal — отдельный универсальный редактор на Alt+3
// Сначала пусто (drop/выбор), после загрузки файла — единый редактор всего: микс звука, pipeline, фильтры.
func ScreenUniversal(s tcell.Screen, cfg *core.Config, initialPath string) {
	p := strings.TrimSpace(initialPath)
	if p != "" {
		p = core.ParseUserPath(p)
		if _, err := os.Stat(p); err != nil {
			p = ""
		}
	}
	// Empty state: предложить загрузить файл
	if p == "" {
		for {
			w, h := s.Size()
			s.Clear()
			DrawHeader(s, T(*cfg, "universal_title"), w, *cfg)
			msg := T(*cfg, "universal_empty_hint")
			lines := strings.Split(msg, "\n")
			y := h/2 - len(lines)/2
			for i, line := range lines {
				DrawString(s, max(2, (w-len(line))/2), y+i, line, w-4, GetBaseStyle(*cfg))
			}
			opts := []string{T(*cfg, "universal_load_file"), T(*cfg, "universal_cancel")}
			// нарисуем опции внизу
			for i, o := range opts {
				style := GetBaseStyle(*cfg)
				if i == 0 {
					style = GetHighlightStyle(*cfg)
				}
				DrawString(s, max(2, (w-len(o))/2), y+len(lines)+2+i, o, w-4, style)
			}
			DrawFooter(s, T(*cfg, "universal_footer_empty"), w, h)
			s.Show()
			ev := s.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventKey:
				if CheckTerminalHotkey(ev) {
					GlobalTerminal.Open(s, cfg)
					continue
				}
				switch ev.Key() {
				case tcell.KeyEscape:
					return
				case tcell.KeyEnter:
					fpath, ok := TextInput(s, cfg, T(*cfg, "universal_title"), T(*cfg, "aspect_prompt_file"), "", T(*cfg, "footer_input"))
					if !ok || strings.TrimSpace(fpath) == "" {
						continue
					}
					cand := core.ParseUserPath(fpath)
					if _, err := os.Stat(cand); err != nil {
						ShowMessage(s, cfg, T(*cfg, "universal_title"), []string{T(*cfg, "convert_err_notfound", cand)}, T(*cfg, "footer_message"))
						continue
					}
					p = cand
					goto EDITOR
				case tcell.KeyRune:
					if ev.Rune() == 'q' {
						return
					}
				}
			}
		}
	}
EDITOR:
	if p == "" {
		return
	}
	// Unified editor: показать probe и дать pipeline-меню прямо здесь (звук+pipeline)
	info, _ := core.ProbeMediaInfoWithConfig(p, *cfg)
	var ops []core.Operation
	for {
		w, h := s.Size()
		s.Clear()
		DrawHeader(s, T(*cfg, "universal_title")+" — "+filepath.Base(p), w, *cfg)
		// Левая часть — info, правая — pipeline
		// Упрощённо: заголовок файла + pipelines
		y := 2
		DrawString(s, 2, y, fmt.Sprintf("File: %s", filepath.Base(p)), w-4, GetAccentStyle(*cfg))
		y++
		if info != nil {
			DrawString(s, 2, y, fmt.Sprintf("Format: %s  Duration: %s  Streams: %d", info.Format.FormatName, info.Format.Duration, len(info.Streams)), w-4, GetDimStyle(*cfg))
			y++
			for _, st := range info.Streams {
				dim := ""
				if st.Width > 0 {
					dim = fmt.Sprintf(" %dx%d", st.Width, st.Height)
				}
				DrawString(s, 2, y, fmt.Sprintf(" [%s] %s%s", st.CodecType, st.CodecName, dim), w-4, GetBaseStyle(*cfg))
				y++
				if y >= h-10 {
					break
				}
			}
		}
		y++
		DrawString(s, 2, y, strings.Repeat("─", w-4), w-4, GetDimStyle(*cfg))
		y++
		if len(ops) == 0 {
			DrawString(s, 2, y, T(*cfg, "pipeline_empty"), w-4, GetDimStyle(*cfg))
		} else {
			DrawString(s, 2, y, "Pipeline:", w-4, GetAccentStyle(*cfg))
			y++
			for i, op := range ops {
				DrawString(s, 4, y, fmt.Sprintf("%d. [%s] %s", i+1, op.Type(), op.Describe()), w-8, GetBaseStyle(*cfg))
				y++
			}
		}
		DrawFooter(s, T(*cfg, "universal_footer"), w, h)
		s.Show()

		items := []string{
			T(*cfg, "pipeline_add_transcode"),
			T(*cfg, "pipeline_add_audio") + " (mix/volume/loudnorm)",
			T(*cfg, "pipeline_add_mux") + " (add audio track)",
			T(*cfg, "pipeline_add_filter"),
			T(*cfg, "pipeline_add_subtitle"),
			T(*cfg, "pipeline_add_trim"),
			T(*cfg, "pipeline_add_aspect"),
			T(*cfg, "pipeline_show_command"),
			T(*cfg, "pipeline_run"),
			T(*cfg, "universal_change_file"),
			T(*cfg, "universal_close"),
		}
		idx := RunMenu(s, cfg, T(*cfg, "universal_title"), items, fmt.Sprintf("File: %s — %d ops", filepath.Base(p), len(ops)), T(*cfg, "footer_nav"))
		if idx < 0 || idx == 10 {
			return
		}
		switch idx {
		case 0:
			presets := []string{"H.264 CRF18", "H.265 CRF23", "AV1", "copy (remux)", "Custom"}
			pi := RunMenu(s, cfg, T(*cfg, "pipeline_add_transcode"), presets, "Codec:", T(*cfg, "footer_nav"))
			if pi < 0 {
				continue
			}
			op := &core.TranscodeOp{}
			switch pi {
			case 0:
				op.VCodec = "libx264"; op.ACodec = "aac"; op.CRF = "18"; op.Preset = "medium"; op.Container = "mp4"
			case 1:
				op.VCodec = "libx265"; op.ACodec = "aac"; op.CRF = "23"; op.Preset = "medium"; op.Container = "mp4"
			case 2:
				op.VCodec = "libsvtav1"; op.ACodec = "aac"; op.CRF = "24"; op.Preset = "5"; op.Container = "mkv"
			case 3:
				op.Remux = true; op.Container = "mp4"; op.VCodec = "copy"; op.ACodec = "copy"
			case 4:
				if v, ok := TextInput(s, cfg, "Custom", "VCodec:", "libx264", T(*cfg, "footer_input")); ok {
					op.VCodec = strings.TrimSpace(v)
				}
				if v, ok := TextInput(s, cfg, "Custom", "ACodec:", "aac", T(*cfg, "footer_input")); ok {
					op.ACodec = strings.TrimSpace(v)
				}
			}
			ops = append(ops, op)
		case 1:
			volStr, ok := TextInput(s, cfg, "Audio Mix", "Volume (1.0=orig, 0.5=half, empty=skip):", "", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			aop := &core.AudioOp{}
			if v := strings.TrimSpace(volStr); v != "" {
				var f float64
				fmt.Sscanf(v, "%f", &f)
				aop.Volume = &f
			}
			if RunMenu(s, cfg, "Normalize", []string{"No", "Yes loudnorm"}, "", T(*cfg, "footer_nav")) == 1 {
				aop.Normalize = true
			}
			ops = append(ops, aop)
		case 2:
			extra, ok := TextInput(s, cfg, "Mux", "Path to extra audio to mix/add:", "", T(*cfg, "footer_input"))
			if !ok || strings.TrimSpace(extra) == "" {
				continue
			}
			ep := core.ParseUserPath(extra)
			if _, err := os.Stat(ep); err != nil {
				ShowMessage(s, cfg, "Mux", []string{fmt.Sprintf("Not found: %s", ep)}, T(*cfg, "footer_message"))
				continue
			}
			ops = append(ops, &core.MuxOp{ExtraInputs: []string{ep}, Shortest: true})
		case 3:
			vf, ok := TextInput(s, cfg, "Filter", "VF (e.g. eq=brightness=0.1,hflip):", "", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			if strings.TrimSpace(vf) == "" {
				continue
			}
			ops = append(ops, &core.FilterOp{VF: strings.TrimSpace(vf)})
		case 4:
			sub, ok := TextInput(s, cfg, "Subtitle", "Path to .srt (burn):", "", T(*cfg, "footer_input"))
			if !ok || strings.TrimSpace(sub) == "" {
				continue
			}
			ops = append(ops, &core.SubtitleOp{Mode: "burn", Input: core.ParseUserPath(sub)})
		case 5:
			start, ok := TextInput(s, cfg, T(*cfg, "trim_title"), T(*cfg, "trim_prompt_start"), "00:00:00", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			end, _ := TextInput(s, cfg, T(*cfg, "trim_title"), T(*cfg, "trim_prompt_end"), "", T(*cfg, "footer_input"))
			ops = append(ops, &core.TrimOp{Start: strings.TrimSpace(start), End: strings.TrimSpace(end), Mode: "reencode"})
		case 6:
			ratioStr, ok := TextInput(s, cfg, "Aspect", "Ratio W:H (9:16):", "9:16", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			parts := strings.Split(ratioStr, ":")
			rw, rh := 9, 16
			if len(parts) == 2 {
				fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &rw)
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &rh)
			}
			ops = append(ops, &core.AspectOp{RatioW: rw, RatioH: rh, Mode: "crop", SizePreset: "1080p"})
		case 7:
			if len(ops) == 0 {
				ShowMessage(s, cfg, "Pipeline", []string{"No ops"}, T(*cfg, "footer_message"))
				continue
			}
			pipe := core.Pipeline{Inputs: []string{p}, Operations: ops, Output: core.OutputSpec{Path: p + ".out.mp4"}}
			cmds, err := core.BuildPipeline(pipe, *cfg)
			if err != nil {
				ShowMessage(s, cfg, "Error", []string{err.Error()}, T(*cfg, "footer_message"))
				continue
			}
			var lines []string
			for _, c := range cmds {
				lines = append(lines, strings.Join(core.MaskCmdForLog(c), " "))
			}
			ShowMessage(s, cfg, "Command (dry-run)", lines, T(*cfg, "footer_message"))
		case 8:
			if len(ops) == 0 {
				ShowMessage(s, cfg, "Pipeline", []string{"No ops"}, T(*cfg, "footer_message"))
				continue
			}
			pipe := core.Pipeline{Inputs: []string{p}, Operations: ops, Output: core.OutputSpec{Path: ""}}
			cmds, err := core.BuildPipeline(pipe, *cfg)
			if err != nil {
				ShowMessage(s, cfg, "Error", []string{err.Error()}, T(*cfg, "footer_message"))
				continue
			}
			if len(cmds) == 1 {
				out := cmds[0][len(cmds[0])-1]
				RunWithLogHook(s, cfg, cmds[0], "Universal", filepath.Base(p), filepath.Base(out), nil)
			} else {
				groupID := fmt.Sprintf("uni_%d", len(cmds))
				tasks := core.GlobalQueue.EnqueueChain(cmds, "Universal (TUI)", p, p+".out.mp4", groupID)
				ShowMessage(s, cfg, "Queued", []string{fmt.Sprintf("Enqueued %d tasks group %s", len(tasks), groupID)}, T(*cfg, "footer_message"))
			}
			return
		case 9:
			// change file
			fpath, ok := TextInput(s, cfg, T(*cfg, "universal_title"), T(*cfg, "aspect_prompt_file"), p, T(*cfg, "footer_input"))
			if !ok || strings.TrimSpace(fpath) == "" {
				continue
			}
			cand := core.ParseUserPath(fpath)
			if _, err := os.Stat(cand); err != nil {
				ShowMessage(s, cfg, T(*cfg, "universal_title"), []string{T(*cfg, "convert_err_notfound", cand)}, T(*cfg, "footer_message"))
				continue
			}
			p = cand
			info, _ = core.ProbeMediaInfoWithConfig(p, *cfg)
			ops = nil
		}
	}
}
