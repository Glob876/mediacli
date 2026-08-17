package ui

import (
	"fmt"
	"mediacli/pkg/core"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	if m == 0 { // Quick Download
		preset = core.DownloadPreset{ID: "default", Name: "Default", Fields: cfg.PresetDefaults}
	} else if m == 1 { // Choose saved preset
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
	} else { // Step-by-Step Manual Setup Wizard
		pObj, ok := runManualDownloadWizard(s, cfg)
		if !ok {
			return
		}
		preset = *pObj
	}

	cmdList := append([]string{"yt-dlp"}, core.BuildYtDlpArgs(preset, *cfg, outDir, false)...)
	cmdList = append(cmdList, url)
	RunWithLog(s, cfg, cmdList, "Download Video", url, outDir)
}

func runManualDownloadWizard(s tcell.Screen, cfg *core.Config) (*core.DownloadPreset, bool) {
	fields := make(map[string]interface{})

	// 1. Качество видео
	qItems := []string{
		"1. Best Available (Лучшее доступное)",
		"2. 4K Ultra HD (2160p)",
		"3. 2K Quad HD (1440p)",
		"4. Full HD (1080p)",
		"5. HD (720p)",
		"6. SD (480p)",
		"7. Audio Only (Только звуковая дорожка)",
	}
	qVals := []string{"", "2160", "1440", "1080", "720", "480", "audio"}
	qi := RunMenu(s, cfg, T(*cfg, "wizard_step1_title"), qItems, T(*cfg, "wizard_step1_sub"), T(*cfg, "footer_nav"))
	if qi < 0 {
		return nil, false
	}
	if qVals[qi] == "audio" {
		fields["audio_only"] = true
	} else if qVals[qi] != "" {
		fields["quality"] = qVals[qi]
	}

	// 2. Кодек (если не только аудио)
	if !core.GetBool(fields, "audio_only") {
		keys := core.OrderedVideoPresetKeys
		var pNames []string
		for _, k := range keys {
			pNames = append(pNames, PresetDisplayName(*cfg, core.VideoPresets[k]))
		}
		pNames = append(pNames, "Custom FFmpeg Flags...")

		ci := RunMenu(s, cfg, T(*cfg, "wizard_step2_title"), pNames, T(*cfg, "wizard_step2_sub"), T(*cfg, "footer_nav"))
		if ci < 0 {
			return nil, false
		}
		if ci < len(keys) {
			fields["video_preset"] = keys[ci]
		} else {
			flags, ok := TextInput(s, cfg, "Custom FFmpeg", "Enter custom FFmpeg flags:", "-c:v libx264 -c:a aac", T(*cfg, "footer_input"))
			if !ok {
				return nil, false
			}
			fields["video_preset"] = "custom"
			fields["custom_flags"] = flags
		}
	}

	// 3. Диапазон времени (Таймкод)
	timeRange, ok := TextInput(s, cfg, T(*cfg, "wizard_step3_title"), T(*cfg, "wizard_step3_prompt"), "", T(*cfg, "footer_input"))
	if !ok {
		return nil, false
	}
	if strings.TrimSpace(timeRange) != "" {
		fields["download_section"] = strings.TrimSpace(timeRange)
	}

	// 4. Субтитры
	subItems := []string{
		"1. No Subtitles (Без субтитров)",
		"2. Download Subs (Отдельный файл субтитров ru,en)",
		"3. Embed Subs (Вшить субтитры в контейнер)",
		"4. Embed Auto-Generated Subs (Вшить автосабы)",
	}
	si := RunMenu(s, cfg, T(*cfg, "wizard_step4_title"), subItems, T(*cfg, "wizard_step4_sub"), T(*cfg, "footer_nav"))
	if si < 0 {
		return nil, false
	}
	if si == 1 {
		fields["subs_enabled"] = true
	} else if si == 2 {
		fields["subs_enabled"] = true
		fields["embed_subs"] = true
	} else if si == 3 {
		fields["subs_enabled"] = true
		fields["embed_subs"] = true
		fields["auto_subs"] = true
	}

	// 5. SponsorBlock
	sbItems := []string{
		"1. Disabled (Оставить видео оригинальным)",
		"2. Remove Sponsors (Вырезать рекламу и интеграции физически)",
		"3. Mark Chapters (Только пометить рекламу главами в плеере)",
	}
	sbi := RunMenu(s, cfg, T(*cfg, "wizard_step5_title"), sbItems, T(*cfg, "wizard_step5_sub"), T(*cfg, "footer_nav"))
	if sbi < 0 {
		return nil, false
	}
	if sbi == 1 {
		fields["sponsorblock"] = "remove"
	} else if sbi == 2 {
		fields["sponsorblock"] = "mark"
	}

	// 6. Нарезка по главам
	chapItems := []string{
		"1. Single File (Один цельный файл, по умолчанию)",
		"2. Split by Chapters (Разрезать на отдельные файлы по главам)",
	}
	chapi := RunMenu(s, cfg, T(*cfg, "wizard_step6_title"), chapItems, T(*cfg, "wizard_step6_sub"), T(*cfg, "footer_nav"))
	if chapi < 0 {
		return nil, false
	}
	if chapi == 1 {
		fields["split_chapters"] = true
	}

	// 7. Метаданные и постер
	metaItems := []string{
		"1. Embed Metadata & Thumbnail (Вшить обложку и теги, рекомендуется)",
		"2. Clean File (Без добавления метаданных)",
	}
	mi := RunMenu(s, cfg, T(*cfg, "wizard_step7_title"), metaItems, T(*cfg, "wizard_step7_sub"), T(*cfg, "footer_nav"))
	if mi < 0 {
		return nil, false
	}
	fields["embed_metadata"] = (mi == 0)

	// 8. Финал: Запустить или Сохранить как пресет
	finishItems := []string{
		T(*cfg, "wizard_act_run"),
		T(*cfg, "wizard_act_save_run"),
		T(*cfg, "wizard_act_cancel"),
	}
	fi := RunMenu(s, cfg, T(*cfg, "wizard_finish_title"), finishItems, T(*cfg, "wizard_finish_sub"), T(*cfg, "footer_nav"))
	if fi < 0 || fi == 2 {
		return nil, false
	}

	preset := &core.DownloadPreset{
		ID:     fmt.Sprintf("custom_%d", time.Now().Unix()),
		Name:   "Manual Setup",
		Fields: fields,
	}

	if fi == 1 {
		pName, ok := TextInput(s, cfg, "Save Preset", "Enter name for this new preset:", "My Custom Preset", T(*cfg, "footer_input"))
		if ok && strings.TrimSpace(pName) != "" {
			preset.Name = strings.TrimSpace(pName)
			cfg.DownloadPresets = append(cfg.DownloadPresets, *preset)
			_ = core.SaveConfig(*cfg)
		}
	}

	return preset, true
}

func ScreenThumbnail(s tcell.Screen, cfg *core.Config) {
	url, ok := TextInput(s, cfg, T(*cfg, "thumb_title"), T(*cfg, "video_prompt_url"), "", T(*cfg, "footer_input"))
	if !ok || strings.TrimSpace(url) == "" {
		return
	}

	fmts := []string{"PNG (Lossless)", "JPG (Compact)", "WEBP (Original)"}
	rawFmts := []string{"png", "jpg", "webp"}
	fi := RunMenu(s, cfg, T(*cfg, "thumb_title"), fmts, T(*cfg, "thumb_format_subtitle"), T(*cfg, "footer_nav"))
	if fi < 0 {
		return
	}
	targetFmt := rawFmts[fi]

	outDir := core.ParseUserPath(cfg.DownloadDir)
	_ = os.MkdirAll(outDir, 0755)

	cmdList := []string{
		"yt-dlp", "--write-thumbnail", "--skip-download",
		"--convert-thumbnails", targetFmt,
		"-o", filepath.Join(outDir, "%(title)s.%(ext)s"),
	}
	cmdList = append(cmdList, core.BuildCookieArgs(*cfg)...)
	cmdList = append(cmdList, core.BuildProxyArgs(*cfg, "", "")...)
	cmdList = append(cmdList, url)

	RunWithLog(s, cfg, cmdList, "Download Thumbnail", url, outDir)
}

func ScreenAudio(s tcell.Screen, cfg *core.Config) {
	url, ok := TextInput(s, cfg, T(*cfg, "audio_title"), T(*cfg, "video_prompt_url"), "", T(*cfg, "footer_input"))
	if !ok || strings.TrimSpace(url) == "" {
		return
	}

	formats := []string{"MP3 (320k)", "FLAC (Lossless)", "WAV (Uncompressed)", "M4A (AAC)", "OPUS"}
	rawFmts := []string{"mp3", "flac", "wav", "m4a", "opus"}
	fi := RunMenu(s, cfg, T(*cfg, "audio_title"), formats, "Select Audio Format", T(*cfg, "footer_nav"))
	if fi < 0 {
		return
	}

	fmtStr := rawFmts[fi]
	outDir := core.ParseUserPath(cfg.DownloadDir)
	_ = os.MkdirAll(outDir, 0755)

	cmdList := []string{
		"yt-dlp", "-x", "--audio-format", fmtStr, "--audio-quality", "0",
		"--concurrent-fragments", fmt.Sprintf("%d", cfg.ConcurrentFragments),
		"-o", filepath.Join(outDir, "%(title)s.%(ext)s"),
	}
	if cfg.NoMtime {
		cmdList = append(cmdList, "--no-mtime")
	}
	if cfg.WindowsFilenames {
		cmdList = append(cmdList, "--windows-filenames")
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

	pi := RunMenu(s, cfg, T(*cfg, "convert_title"), names, "Choose target conversion preset (1. MP4 / 2. MKV):", T(*cfg, "footer_nav"))
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
		T(*cfg, "tab_set_accel"),
		T(*cfg, "tab_set_ui"),
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
		leftW := 26
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
		yesStr := T(*cfg, "val_yes")
		noStr := T(*cfg, "val_no")

		switch leftIdx {
		case 0: // General
			archStr := noStr
			if cfg.UseArchive {
				archStr = yesStr
			}
			rightItems = []settingItem{
				{Label: T(*cfg, "settings_download_dir", cfg.DownloadDir), CLI: "-o", Key: "download_dir"},
				{Label: T(*cfg, "settings_language", cfg.Language), CLI: "i18n", Key: "language"},
				{Label: T(*cfg, "settings_proxy", cfg.ProxyMode), CLI: "--proxy", Key: "proxy"},
				{Label: T(*cfg, "settings_archive", archStr), CLI: "--download-archive", Key: "archive"},
			}
		case 1: // Video & Codecs
			pName := cfg.VideoPreset
			if p, ok := core.VideoPresets[cfg.VideoPreset]; ok {
				pName = PresetDisplayName(*cfg, p)
			}
			rightItems = []settingItem{
				{Label: T(*cfg, "settings_preset", pName), CLI: "--recode-video", Key: "video_preset"},
				{Label: T(*cfg, "settings_audio_format", cfg.AudioFormat), CLI: "--audio-format", Key: "audio_format"},
				{Label: T(*cfg, "settings_sub_langs", cfg.SubLangs), CLI: "--sub-langs", Key: "sub_langs"},
				{Label: T(*cfg, "settings_thumb_fmt", strings.ToUpper(cfg.ThumbnailFormat)), CLI: "--convert-thumbnails", Key: "thumb_fmt"},
			}
		case 2: // Acceleration & Network
			noMtimeStr := noStr
			if cfg.NoMtime {
				noMtimeStr = yesStr
			}
			winNamesStr := noStr
			if cfg.WindowsFilenames {
				winNamesStr = yesStr
			}
			rightItems = []settingItem{
				{Label: T(*cfg, "settings_accel_frags", cfg.ConcurrentFragments), CLI: "--concurrent-fragments", Key: "frags"},
				{Label: T(*cfg, "settings_no_mtime", noMtimeStr), CLI: "--no-mtime", Key: "no_mtime"},
				{Label: T(*cfg, "settings_win_names", winNamesStr), CLI: "--windows-filenames", Key: "win_names"},
			}
		case 3: // Interface & System
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
	case "archive":
		cfg.UseArchive = !cfg.UseArchive
		_ = core.SaveConfig(*cfg)
	case "video_preset":
		keys := core.OrderedVideoPresetKeys
		var pNames []string
		for _, k := range keys {
			pNames = append(pNames, PresetDisplayName(*cfg, core.VideoPresets[k]))
		}
		pi := RunMenu(s, cfg, T(*cfg, "settings_title"), pNames, "Choose Default Video Codec (1. MP4 / 2. MKV):", T(*cfg, "footer_nav"))
		if pi >= 0 {
			cfg.VideoPreset = keys[pi]
			_ = core.SaveConfig(*cfg)
		}
	case "audio_format":
		fmts := []string{"mp3", "flac", "wav", "m4a", "opus"}
		fi := RunMenu(s, cfg, T(*cfg, "settings_title"), fmts, "Default Audio Format", T(*cfg, "footer_nav"))
		if fi >= 0 {
			cfg.AudioFormat = fmts[fi]
			_ = core.SaveConfig(*cfg)
		}
	case "thumb_fmt":
		fmts := []string{"png", "jpg", "webp"}
		fi := RunMenu(s, cfg, T(*cfg, "settings_title"), fmts, "Default Thumbnail Format", T(*cfg, "footer_nav"))
		if fi >= 0 {
			cfg.ThumbnailFormat = fmts[fi]
			_ = core.SaveConfig(*cfg)
		}
	case "frags":
		choices := []string{"2 потока", "4 потока (по умолчанию)", "8 потоков (быстро)", "16 потоков (максимум)"}
		vals := []int{2, 4, 8, 16}
		ci := RunMenu(s, cfg, T(*cfg, "settings_title"), choices, "Concurrent Fragment Downloads", T(*cfg, "footer_nav"))
		if ci >= 0 {
			cfg.ConcurrentFragments = vals[ci]
			_ = core.SaveConfig(*cfg)
		}
	case "no_mtime":
		cfg.NoMtime = !cfg.NoMtime
		_ = core.SaveConfig(*cfg)
	case "win_names":
		cfg.WindowsFilenames = !cfg.WindowsFilenames
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