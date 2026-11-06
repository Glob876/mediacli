package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mediacli/pkg/core"

	"github.com/gdamore/tcell/v2"
)

func ScreenPipeline(s tcell.Screen, cfg *core.Config) {
	session := newEditorSession("pipeline")
	restored, restoredOps := session.restore(s, cfg)
	fpath, ok := TextInput(s, cfg, T(*cfg, "pipeline_title"), T(*cfg, "aspect_prompt_file"), restored, T(*cfg, "footer_input"))
	if !ok || strings.TrimSpace(fpath) == "" {
		return
	}
	p := core.ParseUserPath(fpath)
	if _, err := os.Stat(p); err != nil {
		ShowMessage(s, cfg, T(*cfg, "pipeline_title"), []string{T(*cfg, "convert_err_notfound", p)}, T(*cfg, "footer_message"))
		return
	}

	var ops []core.Operation
	if p == restored {
		ops = restoredOps
	}
	for {
		if err := session.capture(p, ops); err != nil {
			ShowMessage(s, cfg, "Autosave", []string{err.Error()}, T(*cfg, "footer_message"))
		}
		// Build menu labels
		items := []string{
			T(*cfg, "pipeline_add_transcode"),
			T(*cfg, "pipeline_add_aspect"),
			T(*cfg, "pipeline_add_trim"),
			T(*cfg, "pipeline_add_mux"),
			T(*cfg, "pipeline_add_audio"),
			T(*cfg, "pipeline_add_filter"),
			T(*cfg, "pipeline_add_subtitle"),
			T(*cfg, "pipeline_add_metadata"),
			T(*cfg, "pipeline_add_scale"),
			T(*cfg, "pipeline_add_fps"),
			T(*cfg, "pipeline_show_command"),
			T(*cfg, "pipeline_run"),
			T(*cfg, "pipeline_save_preset"),
			T(*cfg, "pipeline_clear"),
			paletteText(*cfg, "Экспорт под размер", "Export to size"),
			paletteText(*cfg, "Фрагменты / звуковая волна", "Ranges / waveform"),
			paletteText(*cfg, "Отменить (Ctrl+Z)", "Undo (Ctrl+Z)"),
			paletteText(*cfg, "Повторить (Ctrl+Y)", "Redo (Ctrl+Y)"),
			T(*cfg, "pipeline_cancel"),
		}
		if len(ops) > 0 {
			items[13] = fmt.Sprintf("%s (%d)", T(*cfg, "pipeline_clear"), len(ops))
		}
		sub := ""
		if len(ops) > 0 {
			sub = "Current pipeline:\n"
			for i, op := range ops {
				sub += fmt.Sprintf(" %d. [%s] %s\n", i+1, op.Type(), op.Describe())
			}
		} else {
			sub = T(*cfg, "pipeline_empty")
		}

		idx := runMenu(s, cfg, T(*cfg, "pipeline_title"), items, sub, T(*cfg, "footer_nav")+" · Ctrl+Z / Ctrl+Y", true)
		if idx == -2 {
			idx = 16
		}
		if idx == -3 {
			idx = 17
		}
		if idx < 0 || idx == 18 {
			return
		}
		switch idx {
		case 0: // transcode
			presets := []string{"H.264 CRF18 medium", "H.265 CRF23 medium", "AV1 CRF24", "VP9", "copy (remux)", "Custom"}
			pi := RunMenu(s, cfg, T(*cfg, "pipeline_add_transcode"), presets, "Select codec preset:", T(*cfg, "footer_nav"))
			if pi < 0 {
				continue
			}
			op := &core.TranscodeOp{}
			switch pi {
			case 0:
				op.VCodec = "libx264"
				op.ACodec = "aac"
				op.CRF = "18"
				op.Preset = "medium"
				op.PixFmt = "yuv420p"
				op.Container = "mp4"
			case 1:
				op.VCodec = "libx265"
				op.ACodec = "aac"
				op.CRF = "23"
				op.Preset = "medium"
				op.Container = "mp4"
			case 2:
				op.VCodec = "libsvtav1"
				op.ACodec = "aac"
				op.CRF = "24"
				op.Preset = "5"
				op.Container = "mkv"
			case 3:
				op.VCodec = "libvpx-vp9"
				op.ACodec = "libopus"
				op.CRF = "24"
				op.Container = "webm"
			case 4:
				op.Remux = true
				op.Container = "mp4"
				op.VCodec = "copy"
				op.ACodec = "copy"
			case 5:
				if v, ok := TextInput(s, cfg, "Custom Transcode", "VCodec (libx264/libx265/copy):", "libx264", T(*cfg, "footer_input")); ok {
					op.VCodec = strings.TrimSpace(v)
				}
				if v, ok := TextInput(s, cfg, "Custom Transcode", "ACodec (aac/copy):", "aac", T(*cfg, "footer_input")); ok {
					op.ACodec = strings.TrimSpace(v)
				}
				if v, ok := TextInput(s, cfg, "Custom Transcode", "CRF (empty=none):", "18", T(*cfg, "footer_input")); ok {
					op.CRF = strings.TrimSpace(v)
				}
				if v, ok := TextInput(s, cfg, "Custom Transcode", "Container (mp4/mkv):", "mp4", T(*cfg, "footer_input")); ok {
					op.Container = strings.TrimSpace(v)
				}
			}
			ops = append(ops, op)

		case 1: // aspect
			ratioStr, ok := TextInput(s, cfg, "Aspect Ratio", "Target ratio W:H (e.g. 9:16, 16:9):", "9:16", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			parts := strings.Split(ratioStr, ":")
			rw, rh := 9, 16
			if len(parts) == 2 {
				fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &rw)
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &rh)
			}
			modes := []string{"crop", "letterbox", "blur", "stretch"}
			mi := RunMenu(s, cfg, "Aspect Mode", modes, "Choose mode:", T(*cfg, "footer_nav"))
			mode := "crop"
			if mi >= 0 {
				mode = modes[mi]
			}
			ops = append(ops, &core.AspectOp{RatioW: rw, RatioH: rh, Mode: mode, SizePreset: "1080p"})

		case 2: // trim
			start, ok := TextInput(s, cfg, T(*cfg, "trim_title"), T(*cfg, "trim_prompt_start"), "00:00:00", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			end, ok := TextInput(s, cfg, T(*cfg, "trim_title"), T(*cfg, "trim_prompt_end"), "", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			modes := []string{T(*cfg, "trim_mode_copy"), T(*cfg, "trim_mode_reencode")}
			mi := RunMenu(s, cfg, T(*cfg, "trim_title"), modes, "Mode:", T(*cfg, "footer_nav"))
			mode := "reencode"
			if mi == 0 {
				mode = "copy"
			}
			ops = append(ops, &core.TrimOp{Start: strings.TrimSpace(start), End: strings.TrimSpace(end), Mode: mode})

		case 3: // mux
			extra, ok := TextInput(s, cfg, "Mux — Add Audio", "Path to audio/subtitle file to mux:", "", T(*cfg, "footer_input"))
			if !ok || strings.TrimSpace(extra) == "" {
				continue
			}
			ep := core.ParseUserPath(extra)
			if _, err := os.Stat(ep); err != nil {
				ShowMessage(s, cfg, "Mux", []string{fmt.Sprintf("File not found: %s", ep)}, T(*cfg, "footer_message"))
				continue
			}
			mux := &core.MuxOp{ExtraInputs: []string{ep}, Shortest: true}
			ops = append(ops, mux)

		case 4: // audio
			volStr, ok := TextInput(s, cfg, "Audio — Volume", "Volume multiplier (1.0=original, 0.5=half, 2.0=double, empty=skip):", "", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			aop := &core.AudioOp{}
			if v := strings.TrimSpace(volStr); v != "" {
				var f float64
				fmt.Sscanf(v, "%f", &f)
				aop.Volume = &f
			}
			// Normalize?
			normOpts := []string{"No", "Yes (loudnorm)"}
			ni := RunMenu(s, cfg, "Audio Normalize", normOpts, "Normalize loudness?", T(*cfg, "footer_nav"))
			if ni == 1 {
				aop.Normalize = true
			}
			ops = append(ops, aop)

		case 5: // filter
			vf, ok := TextInput(s, cfg, "Filter", "Video filter chain (-vf, e.g. eq=brightness=0.1, hflip):", "", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			af, ok := TextInput(s, cfg, "Filter", "Audio filter chain (-af, e.g. volume=0.8):", "", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			if strings.TrimSpace(vf) == "" && strings.TrimSpace(af) == "" {
				continue
			}
			ops = append(ops, &core.FilterOp{VF: strings.TrimSpace(vf), AF: strings.TrimSpace(af)})

		case 6: // subtitle burn
			subPath, ok := TextInput(s, cfg, "Subtitle Burn-in", "Path to .srt/.ass file (burn-in):", "", T(*cfg, "footer_input"))
			if !ok || strings.TrimSpace(subPath) == "" {
				continue
			}
			ops = append(ops, &core.SubtitleOp{Mode: "burn", Input: core.ParseUserPath(subPath)})

		case 7: // metadata
			title, ok := TextInput(s, cfg, "Metadata", "Title (empty=skip):", "", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			ops = append(ops, &core.MetadataOp{Title: strings.TrimSpace(title)})

		case 8: // scale
			wStr, ok := TextInput(s, cfg, "Scale", "Width (e.g. 1280, 0=auto):", "1280", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			hStr, ok := TextInput(s, cfg, "Scale", "Height (e.g. 720, 0=auto):", "0", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			w, h := 0, 0
			fmt.Sscanf(strings.TrimSpace(wStr), "%d", &w)
			fmt.Sscanf(strings.TrimSpace(hStr), "%d", &h)
			ops = append(ops, &core.ScaleOp{Width: w, Height: h, Algo: "lanczos"})

		case 9: // fps
			fpsStr, ok := TextInput(s, cfg, "FPS", "Target FPS (e.g. 30, 60):", "30", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			ops = append(ops, &core.FpsOp{FPS: strings.TrimSpace(fpsStr)})

		case 10: // show command dry-run
			if len(ops) == 0 {
				ShowMessage(s, cfg, "Pipeline", []string{"No operations added."}, T(*cfg, "footer_message"))
				continue
			}
			pipe := core.Pipeline{Inputs: []string{p}, Operations: ops, Output: core.OutputSpec{Path: p + ".out.mp4"}}
			cmds, err := core.BuildPipeline(pipe, *cfg)
			if err != nil {
				ShowMessage(s, cfg, "Build Error", []string{err.Error()}, T(*cfg, "footer_message"))
				continue
			}
			var lines []string
			for _, c := range cmds {
				lines = append(lines, strings.Join(core.MaskCmdForLog(c), " "))
			}
			ShowMessage(s, cfg, "Pipeline Command (dry-run)", lines, T(*cfg, "footer_message"))

		case 11: // run
			if len(ops) == 0 {
				ShowMessage(s, cfg, "Pipeline", []string{"No operations."}, T(*cfg, "footer_message"))
				continue
			}
			pipe := core.Pipeline{Inputs: []string{p}, Operations: ops, Output: core.OutputSpec{Path: ""}}
			cmds, err := core.BuildPipeline(pipe, *cfg)
			if err != nil {
				ShowMessage(s, cfg, "Build Error", []string{err.Error()}, T(*cfg, "footer_message"))
				continue
			}
			// Multi-pass operations use the queue to preserve dependencies.
			if len(cmds) == 1 {
				output := cmds[0][len(cmds[0])-1]
				RunWithLogHook(s, cfg, cmds[0], "Pipeline", filepath.Base(p), filepath.Base(output), nil)
			} else {
				groupID := fmt.Sprintf("tui_pipe_%d", time.Now().UnixNano())
				lastCommand := cmds[len(cmds)-1]
				outputPath := lastCommand[len(lastCommand)-1]
				tasks := core.GlobalQueue.EnqueueChain(cmds, "Pipeline (TUI)", p, outputPath, groupID)
				ShowMessage(s, cfg, "Pipeline Queued", []string{fmt.Sprintf("Enqueued %d tasks (group %s). See BG queue.", len(tasks), groupID)}, T(*cfg, "footer_message"))
			}
			return

		case 12: // save preset
			if len(ops) == 0 {
				ShowMessage(s, cfg, "Preset", []string{"No operations to save."}, T(*cfg, "footer_message"))
				continue
			}
			name, ok := TextInput(s, cfg, "Save Preset", "Preset name:", "My Pipeline", T(*cfg, "footer_input"))
			if !ok || strings.TrimSpace(name) == "" {
				continue
			}
			var envs []core.OperationEnvelope
			for _, op := range ops {
				b, _ := marshalOp(op)
				envs = append(envs, core.OperationEnvelope{Type: op.Type(), Config: b})
			}
			preset := core.MediaPreset{Name: strings.TrimSpace(name), Operations: envs}
			if err := core.SaveMediaPreset(preset); err != nil {
				ShowMessage(s, cfg, "Preset", []string{"Save failed: " + err.Error()}, T(*cfg, "footer_message"))
			} else {
				ShowMessage(s, cfg, "Preset", []string{"Saved: " + name}, T(*cfg, "footer_message"))
			}

		case 14:
			screenSizeExport(s, cfg, p, ops, session)
		case 15:
			screenRanges(s, cfg, p)
		case 16, 17:
			nextPath, nextOps, err := session.move(idx == 17)
			if err == nil {
				p, ops = nextPath, nextOps
			}
		case 13: // clear
			ops = nil
		}
	}
}

func marshalOp(op core.Operation) ([]byte, error) {
	// crude: use type switch to marshal concrete struct
	switch v := op.(type) {
	case *core.TranscodeOp:
		return marshalJSON(v)
	case *core.AspectOp:
		return marshalJSON(v)
	case *core.TrimOp:
		return marshalJSON(v)
	case *core.MuxOp:
		return marshalJSON(v)
	case *core.AudioOp:
		return marshalJSON(v)
	case *core.FilterOp:
		return marshalJSON(v)
	case *core.SubtitleOp:
		return marshalJSON(v)
	case *core.MetadataOp:
		return marshalJSON(v)
	case *core.ScaleOp:
		return marshalJSON(v)
	case *core.FpsOp:
		return marshalJSON(v)
	case *core.ConcatOp:
		return marshalJSON(v)
	case *core.DemuxOp:
		return marshalJSON(v)
	default:
		return marshalJSON(v)
	}
}

func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func ScreenMux(s tcell.Screen, cfg *core.Config) {
	videoPath, ok := TextInput(s, cfg, T(*cfg, "mux_title"), T(*cfg, "mux_prompt_video"), "", T(*cfg, "footer_input"))
	if !ok || strings.TrimSpace(videoPath) == "" {
		return
	}
	vp := core.ParseUserPath(videoPath)
	if _, err := os.Stat(vp); err != nil {
		ShowMessage(s, cfg, T(*cfg, "mux_title"), []string{T(*cfg, "convert_err_notfound", vp)}, T(*cfg, "footer_message"))
		return
	}
	audioPath, ok := TextInput(s, cfg, T(*cfg, "mux_title"), T(*cfg, "mux_prompt_audio"), "", T(*cfg, "footer_input"))
	if !ok || strings.TrimSpace(audioPath) == "" {
		return
	}
	ap := core.ParseUserPath(audioPath)
	if _, err := os.Stat(ap); err != nil {
		ShowMessage(s, cfg, T(*cfg, "mux_title"), []string{T(*cfg, "convert_err_notfound", ap)}, T(*cfg, "footer_message"))
		return
	}
	cfgCopy := *cfg
	op := &core.MuxOp{ExtraInputs: []string{ap}, Shortest: true}
	pipe := core.Pipeline{Inputs: []string{vp}, Operations: []core.Operation{op}, Output: core.OutputSpec{}}
	cmds, err := core.BuildPipeline(pipe, cfgCopy)
	if err != nil {
		ShowMessage(s, cfg, T(*cfg, "mux_title"), []string{err.Error()}, T(*cfg, "footer_message"))
		return
	}
	output := cmds[0][len(cmds[0])-1]
	RunWithLogHook(s, cfg, cmds[0], "Mux", filepath.Base(vp)+" + "+filepath.Base(ap), filepath.Base(output), nil)
}

func ScreenBatch(s tcell.Screen, cfg *core.Config) {
	folder, ok := TextInput(s, cfg, T(*cfg, "batch_title"), T(*cfg, "batch_prompt_folder"), cfg.DownloadDir, T(*cfg, "footer_input"))
	if !ok || strings.TrimSpace(folder) == "" {
		return
	}
	dir := core.ParseUserPath(folder)
	entries, err := os.ReadDir(dir)
	if err != nil {
		ShowMessage(s, cfg, T(*cfg, "batch_title"), []string{err.Error()}, T(*cfg, "footer_message"))
		return
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".mp4" || ext == ".mkv" || ext == ".mov" || ext == ".webm" || ext == ".avi" {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	if len(files) == 0 {
		ShowMessage(s, cfg, T(*cfg, "batch_title"), []string{T(*cfg, "batch_no_files")}, T(*cfg, "footer_message"))
		return
	}
	// Choose operation: simple transcode preset
	presets := []string{"H.264 CRF18", "H.265 CRF23", "Remux copy"}
	pi := RunMenu(s, cfg, T(*cfg, "batch_title"), presets, fmt.Sprintf("Found %d files. Choose preset:", len(files)), T(*cfg, "footer_nav"))
	if pi < 0 {
		return
	}
	var op core.Operation
	switch pi {
	case 0:
		op = &core.TranscodeOp{VCodec: "libx264", ACodec: "aac", CRF: "18", Preset: "medium", Container: "mp4"}
	case 1:
		op = &core.TranscodeOp{VCodec: "libx265", ACodec: "aac", CRF: "23", Preset: "medium", Container: "mp4"}
	default:
		op = &core.TranscodeOp{Remux: true, Container: "mp4", VCodec: "copy", ACodec: "copy"}
	}
	// Use batch helper
	job := core.BatchJob{
		Inputs:    files,
		Pipeline:  core.Pipeline{Operations: []core.Operation{op}},
		OutputDir: dir,
	}
	report := core.PlanBatch(job, *cfg)
	accepted := 0
	for _, item := range report.Items {
		if item.Error == "" {
			accepted++
		}
	}
	summary := fmt.Sprintf(paletteText(*cfg, "Готово: %d; пропущено: %d", "Ready: %d; skipped: %d"), accepted, len(report.Items)-accepted)
	ShowMessage(s, cfg, T(*cfg, "batch_title"), append([]string{summary}, report.Lines()...), T(*cfg, "footer_message"))
	if accepted == 0 {
		return
	}
	if RunMenu(s, cfg, T(*cfg, "batch_title"), []string{paletteText(*cfg, "Запустить проверенные файлы", "Run validated files"), T(*cfg, "pipeline_cancel")}, summary, T(*cfg, "footer_nav")) != 0 {
		return
	}
	ids := report.Enqueue()
	ShowMessage(s, cfg, T(*cfg, "batch_title"), []string{fmt.Sprintf("%d tasks", len(ids))}, T(*cfg, "footer_message"))
}

func ScreenInspector(s tcell.Screen, cfg *core.Config) {
	fpath, ok := TextInput(s, cfg, T(*cfg, "probe_title"), T(*cfg, "probe_prompt_file"), "", T(*cfg, "footer_input"))
	if !ok || strings.TrimSpace(fpath) == "" {
		return
	}
	p := core.ParseUserPath(fpath)
	info, err := core.ProbeMediaInfoWithConfig(p, *cfg)
	if err != nil {
		ShowMessage(s, cfg, T(*cfg, "probe_title"), []string{fmt.Sprintf("Error: %v", err)}, T(*cfg, "footer_message"))
		return
	}
	lines := []string{
		"File: " + filepath.Base(p),
		fmt.Sprintf("Format: %s (%s)", info.Format.FormatName, info.Format.FormatLong),
		fmt.Sprintf("Duration: %s s (%.2f sec)", info.Format.Duration, info.Format.DurationSec),
		fmt.Sprintf("Size: %s  Bitrate: %s", info.Format.Size, info.Format.BitRate),
		"Streams:",
	}
	for _, st := range info.Streams {
		dim := ""
		if st.Width > 0 {
			dim = fmt.Sprintf(" %dx%d", st.Width, st.Height)
		}
		lang := ""
		if st.Language != "" {
			lang = " lang=" + st.Language
		}
		lines = append(lines, fmt.Sprintf("  [%d %s] %s%s%s pix_fmt=%s", st.Index, st.CodecType, st.CodecName, dim, lang, st.PixFmt))
		if len(st.Tags) > 0 {
			lines = append(lines, fmt.Sprintf("      tags: %v", st.Tags))
		}
	}
	if len(info.Chapters) > 0 {
		lines = append(lines, "Chapters:")
		for _, ch := range info.Chapters {
			title := ch.Tags["title"]
			lines = append(lines, fmt.Sprintf("  %d: %s - %s %s", ch.ID, ch.StartTime, ch.EndTime, title))
		}
	}
	ShowMessage(s, cfg, T(*cfg, "probe_title"), lines, T(*cfg, "footer_message"))
}
