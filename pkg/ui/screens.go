package ui

import (
	"fmt"
	"mediacli/pkg/core"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
)

func ScreenVideo(s tcell.Screen, cfg *core.Config) {
	url, ok := TextInput(s, cfg, T(*cfg, "video_title"), T(*cfg, "video_prompt_url"), "", T(*cfg, "footer_input"))
	if !ok || strings.TrimSpace(url) == "" {
		return
	}

	modes := []string{
		T(*cfg, "download_mode_quick"),
		T(*cfg, "download_mode_preset"),
		T(*cfg, "download_mode_manual"),
	}
	m := RunMenu(s, cfg, T(*cfg, "video_title"), modes, T(*cfg, "download_mode_subtitle"), T(*cfg, "footer_nav"))
	if m < 0 {
		return
	}

	outDir := core.ParseUserPath(cfg.DownloadDir)
	_ = os.MkdirAll(outDir, 0755)

	var preset core.DownloadPreset
	if m == 0 {
		preset = core.DownloadPreset{ID: "default", Name: "Default", Fields: cfg.PresetDefaults}
	} else if m == 1 {
		if len(cfg.DownloadPresets) == 0 {
			ShowMessage(s, cfg, T(*cfg, "video_title"), []string{"No presets saved yet."}, T(*cfg, "footer_message"))
			return
		}
		names := make([]string, len(cfg.DownloadPresets))
		for i, p := range cfg.DownloadPresets {
			names[i] = p.Name
		}
		pi := RunMenu(s, cfg, T(*cfg, "video_title"), names, "Choose preset:", T(*cfg, "footer_nav"))
		if pi < 0 {
			return
		}
		preset = cfg.DownloadPresets[pi]
	} else {
		preset = core.DownloadPreset{ID: "manual", Name: "Manual", Fields: cfg.PresetDefaults}
	}

	cmdList := append([]string{"yt-dlp"}, core.BuildYtDlpArgs(preset, *cfg, outDir, false)...)
	cmdList = append(cmdList, url)
	RunWithLog(s, cfg, cmdList, "Download Video", url, outDir)
}

func ScreenAudio(s tcell.Screen, cfg *core.Config) {
	url, ok := TextInput(s, cfg, T(*cfg, "audio_title"), T(*cfg, "video_prompt_url"), "", T(*cfg, "footer_input"))
	if !ok || strings.TrimSpace(url) == "" {
		return
	}

	formats := []string{"MP3", "WAV", "M4A", "OPUS", "FLAC"}
	fi := RunMenu(s, cfg, T(*cfg, "audio_title"), formats, "Select Audio Format", T(*cfg, "footer_nav"))
	if fi < 0 {
		return
	}

	fmtStr := strings.ToLower(formats[fi])
	outDir := core.ParseUserPath(cfg.DownloadDir)
	_ = os.MkdirAll(outDir, 0755)

	cmdList := []string{
		"yt-dlp", "-x", "--audio-format", fmtStr, "--audio-quality", "0",
		"-o", filepath.Join(outDir, "%(title)s.%(ext)s"),
	}
	cmdList = append(cmdList, core.BuildCookieArgs(*cfg)...)
	cmdList = append(cmdList, core.BuildProxyArgs(*cfg, "", "")...)
	cmdList = append(cmdList, url)

	RunWithLog(s, cfg, cmdList, "Download Audio", url, outDir)
}

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

	pi := RunMenu(s, cfg, T(*cfg, "convert_title"), names, "Choose target preset:", T(*cfg, "footer_nav"))
	if pi < 0 {
		return
	}

	preset := core.ConvertPresets[pi]
	base := strings.TrimSuffix(p, filepath.Ext(p))
	outF := base + preset.Suffix + "." + preset.Ext

	cmdList := append([]string{"ffmpeg", "-y", "-i", p}, preset.FFmpegFlags...)
	cmdList = append(cmdList, outF)
	RunWithLog(s, cfg, cmdList, "Convert File", filepath.Base(p), filepath.Base(outF))
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
	base := strings.TrimSuffix(p, ext)
	outF := base + "_trimmed" + ext

	cmdList := []string{"ffmpeg", "-y", "-ss", strings.TrimSpace(startT)}
	if strings.TrimSpace(endT) != "" {
		cmdList = append(cmdList, "-to", strings.TrimSpace(endT))
	}
	cmdList = append(cmdList, "-i", p)
	if m == 0 {
		cmdList = append(cmdList, "-c", "copy", outF)
	} else {
		cmdList = append(cmdList, "-c:v", "libx264", "-c:a", "aac", outF)
	}

	RunWithLog(s, cfg, cmdList, "Trim Media", filepath.Base(p), filepath.Base(outF))
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

func ScreenSettingsVertical(s tcell.Screen, cfg *core.Config) {
	leftIdx := 0
	rightIdx := 0
	inRightPane := false

	categories := []string{
		T(*cfg, "tab_set_gen"),
		T(*cfg, "tab_set_conv"),
		T(*cfg, "tab_set_ui"),
		T(*cfg, "tab_set_app"),
	}

	type settingItem struct {
		Label string
		CLI   string
		Key   string
	}

	for {
		w, h := s.Size()
		s.Clear()

		DrawHeader(s, T(*cfg, "settings_title"), w, *cfg)
		leftW := 24
		divX := leftW + 2

		for i, cat := range categories {
			y := 3 + i
			isSel := (i == leftIdx)
			style := GetBaseStyle(*cfg)
			marker := "  "
			if isSel && !inRightPane {
				style = GetHighlightStyle(*cfg)
				marker = "► "
			} else if isSel {
				style = GetBaseStyle(*cfg).Bold(true)
				marker = "► "
			}
			DrawString(s, 2, y, marker+cat, leftW, style)
		}

		for y := 2; y < h-2; y++ {
			s.SetContent(divX, y, '│', nil, GetDimStyle(*cfg))
		}

		var rightItems []settingItem
		switch leftIdx {
		case 0:
			rightItems = []settingItem{
				{Label: T(*cfg, "settings_download_dir", cfg.DownloadDir), CLI: "-o", Key: "download_dir"},
				{Label: T(*cfg, "settings_language", cfg.Language), CLI: "i18n", Key: "language"},
				{Label: T(*cfg, "settings_proxy", cfg.ProxyMode), CLI: "--proxy", Key: "proxy"},
			}
		case 1:
			pName := cfg.VideoPreset
			if p, ok := core.VideoPresets[cfg.VideoPreset]; ok {
				pName = PresetDisplayName(*cfg, p)
			}
			rightItems = []settingItem{
				{Label: T(*cfg, "settings_preset", pName), CLI: "--recode-video", Key: "video_preset"},
				{Label: T(*cfg, "settings_audio_format", cfg.AudioFormat), CLI: "--audio-format", Key: "audio_format"},
				{Label: T(*cfg, "settings_sub_langs", cfg.SubLangs), CLI: "--sub-langs", Key: "sub_langs"},
			}
		case 2:
			themeName := GetTheme(*cfg).NameEN
			if cfg.Language == "ru" {
				themeName = GetTheme(*cfg).NameRU
			}
			bgName := T(*cfg, "bg_option_solid")
			if cfg.UseTerminalBG {
				bgName = T(*cfg, "bg_option_keep")
			}
			rightItems = []settingItem{
				{Label: T(*cfg, "settings_theme", themeName), CLI: "color_scheme", Key: "theme"},
				{Label: T(*cfg, "settings_bg", bgName), CLI: "transparency", Key: "terminal_bg"},
				{Label: T(*cfg, "settings_style", cfg.ProgressStyle), CLI: "style", Key: "progress_style"},
			}
		case 3:
			rightItems = []settingItem{
				{Label: T(*cfg, "settings_bg_queue_max", cfg.BGQueueMax), CLI: "queue_slots", Key: "queue_max"},
				{Label: T(*cfg, "settings_reset"), CLI: "factory_wipe", Key: "reset"},
			}
		}

		rightX := divX + 3
		rightW := w - rightX - 2
		if rightIdx >= len(rightItems) {
			rightIdx = 0
		}

		for idx, itm := range rightItems {
			y := 3 + idx
			if y >= h-4 {
				break
			}
			isSel := (idx == rightIdx && inRightPane)
			style := GetBaseStyle(*cfg)
			marker := "  "
			if isSel {
				style = GetHighlightStyle(*cfg)
				marker = "> "
			}
			DrawString(s, rightX, y, marker+itm.Label, rightW-len(itm.CLI)-3, style)
			DrawString(s, w-len(itm.CLI)-4, y, "["+itm.CLI+"]", len(itm.CLI)+2, GetDimStyle(*cfg))
		}

		footer := T(*cfg, "footer_vertical_tabs")
		if inRightPane {
			footer = T(*cfg, "footer_vertical_items")
		}
		DrawFooter(s, footer, w, h)
		s.Show()

		ev := s.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if CheckTerminalHotkey(ev) {
				GlobalTerminal.Open(s, cfg)
				continue
			}
			if !inRightPane {
				switch ev.Key() {
				case tcell.KeyUp:
					if leftIdx > 0 {
						leftIdx--
					} else {
						leftIdx = len(categories) - 1
					}
					rightIdx = 0
				case tcell.KeyDown:
					if leftIdx < len(categories)-1 {
						leftIdx++
					} else {
						leftIdx = 0
					}
					rightIdx = 0
				case tcell.KeyEnter, tcell.KeyRight:
					inRightPane = true
					rightIdx = 0
				case tcell.KeyEscape:
					return
				case tcell.KeyRune:
					if ev.Rune() == 'q' {
						return
					} else if ev.Rune() == 'l' {
						inRightPane = true
					}
				}
			} else {
				switch ev.Key() {
				case tcell.KeyUp:
					if rightIdx > 0 {
						rightIdx--
					} else {
						rightIdx = len(rightItems) - 1
					}
				case tcell.KeyDown:
					if rightIdx < len(rightItems)-1 {
						rightIdx++
					} else {
						rightIdx = 0
					}
				case tcell.KeyEscape, tcell.KeyLeft:
					inRightPane = false
				case tcell.KeyEnter:
					handleSettingEdit(s, cfg, rightItems[rightIdx].Key)
				}
			}
		}
	}
}

func handleSettingEdit(s tcell.Screen, cfg *core.Config, key string) {
	switch key {
	case "download_dir":
		if val, ok := TextInput(s, cfg, T(*cfg, "settings_title"), "New Download Directory:", cfg.DownloadDir, T(*cfg, "footer_input")); ok {
			cfg.DownloadDir = val
			_ = core.SaveConfig(*cfg)
		}
	case "language":
		if cfg.Language == "en" {
			cfg.Language = "ru"
		} else {
			cfg.Language = "en"
		}
		_ = core.SaveConfig(*cfg)
	case "theme":
		var tKeys []string
		var tNames []string
		for k, th := range Themes {
			tKeys = append(tKeys, k)
			if cfg.Language == "ru" {
				tNames = append(tNames, th.NameRU)
			} else {
				tNames = append(tNames, th.NameEN)
			}
		}
		ti := RunMenu(s, cfg, T(*cfg, "settings_title"), tNames, "Choose Theme", T(*cfg, "footer_nav"))
		if ti >= 0 {
			cfg.Theme = tKeys[ti]
			_ = core.SaveConfig(*cfg)
		}
	case "terminal_bg":
		cfg.UseTerminalBG = !cfg.UseTerminalBG
		_ = core.SaveConfig(*cfg)
	case "progress_style":
		styles := []string{"blocks", "classic", "dots", "minimal"}
		si := RunMenu(s, cfg, T(*cfg, "settings_title"), styles, "Choose Style", T(*cfg, "footer_nav"))
		if si >= 0 {
			cfg.ProgressStyle = styles[si]
			_ = core.SaveConfig(*cfg)
		}
	case "reset":
		cfgDefault := core.GetDefaultConfig()
		*cfg = cfgDefault
		_ = core.SaveConfig(*cfg)
		ShowMessage(s, cfg, T(*cfg, "settings_title"), []string{"Settings reset to defaults!"}, T(*cfg, "footer_message"))
	}
}