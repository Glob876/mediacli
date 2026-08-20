package ui

import (
	"fmt"
	"mediacli/pkg/core"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
)

func ScreenConvert(s tcell.Screen, cfg *core.Config) {
	fpath, ok := TextInput(s, cfg, T(*cfg, "convert_title"), T(*cfg, "convert_prompt_file"), "", T(*cfg, "footer_input"))
	if !ok || strings.TrimSpace(fpath) == "" {
		return
	}

	p := core.ParseUserPath(fpath)
	if _, err := os.Stat(p); err != nil {
		ShowMessage(s, cfg, T(*cfg, "convert_title"), []string{T(*cfg, "convert_err_notfound", p)}, T(*cfg, "footer_message"))
		return
	}

	names := make([]string, len(core.ConvertPresets))
	for i, pr := range core.ConvertPresets {
		if cfg.Language == "ru" {
			names[i] = pr.NameRU
		} else {
			names[i] = pr.NameEN
		}
	}

	pi := RunMenu(s, cfg, T(*cfg, "convert_title"), names, "Choose target conversion preset (1. MP4 / 2. MKV):", T(*cfg, "footer_nav"))
	if pi < 0 {
		return
	}

	preset := core.ConvertPresets[pi]
	plan := core.PrepareFFmpegOutput(p, preset.Ext, preset.Suffix, *cfg)

	cmdList := append([]string{"ffmpeg", "-y", "-i", p}, preset.FFmpegFlags...)
	cmdList = append(cmdList, plan.TempOutputPath)
	RunWithLogHook(s, cfg, cmdList, "Convert File", filepath.Base(p), plan.TargetDisplay, plan.OnComplete)
}

func ScreenTrim(s tcell.Screen, cfg *core.Config) {
	fpath, ok := TextInput(s, cfg, T(*cfg, "trim_title"), T(*cfg, "trim_prompt_file"), "", T(*cfg, "footer_input"))
	if !ok || strings.TrimSpace(fpath) == "" {
		return
	}
	p := core.ParseUserPath(fpath)
	if _, err := os.Stat(p); err != nil {
		return
	}

	startT, ok := TextInput(s, cfg, T(*cfg, "trim_title"), T(*cfg, "trim_prompt_start"), "00:00:00", T(*cfg, "footer_input"))
	if !ok {
		return
	}
	endT, ok := TextInput(s, cfg, T(*cfg, "trim_title"), T(*cfg, "trim_prompt_end"), "", T(*cfg, "footer_input"))
	if !ok {
		return
	}

	modes := []string{T(*cfg, "trim_mode_copy"), T(*cfg, "trim_mode_reencode")}
	m := RunMenu(s, cfg, T(*cfg, "trim_title"), modes, "Mode:", T(*cfg, "footer_nav"))
	if m < 0 {
		return
	}

	ext := filepath.Ext(p)
	plan := core.PrepareFFmpegOutput(p, ext, "_trimmed", *cfg)

	cmdList := []string{"ffmpeg", "-y", "-ss", strings.TrimSpace(startT)}
	if strings.TrimSpace(endT) != "" {
		cmdList = append(cmdList, "-to", strings.TrimSpace(endT))
	}
	cmdList = append(cmdList, "-i", p)
	if m == 0 {
		cmdList = append(cmdList, "-c", "copy", plan.TempOutputPath)
	} else {
		cmdList = append(cmdList, "-c:v", "libx264", "-c:a", "aac", plan.TempOutputPath)
	}

	RunWithLogHook(s, cfg, cmdList, "Trim Media", filepath.Base(p), plan.TargetDisplay, plan.OnComplete)
}

func ScreenProbe(s tcell.Screen, cfg *core.Config) {
	fpath, ok := TextInput(s, cfg, T(*cfg, "probe_title"), T(*cfg, "probe_prompt_file"), "", T(*cfg, "footer_input"))
	if !ok || strings.TrimSpace(fpath) == "" {
		return
	}
	p := core.ParseUserPath(fpath)
	info, err := core.ProbeMedia(p)
	if err != nil {
		ShowMessage(s, cfg, T(*cfg, "probe_title"), []string{fmt.Sprintf("Error inspecting file: %v", err)}, T(*cfg, "footer_message"))
		return
	}

	lines := []string{
		"File: " + filepath.Base(p),
		"Duration: " + info.Format.Duration + "s",
		"Streams:",
	}
	for _, stream := range info.Streams {
		dim := ""
		if stream.Width > 0 {
			dim = fmt.Sprintf(" (%dx%d)", stream.Width, stream.Height)
		}
		lines = append(lines, fmt.Sprintf("  [%s] %s%s", stream.CodecType, stream.CodecName, dim))
	}
	ShowMessage(s, cfg, T(*cfg, "probe_title"), lines, T(*cfg, "footer_message"))
}