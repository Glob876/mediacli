package ui

import (
	"mediacli/pkg/core"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
)

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
		case 0: // General & Output
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
			suffixStr := noStr
			if cfg.UseFFmpegSuffix {
				suffixStr = yesStr
			}
			overwriteStr := noStr
			if cfg.OverwriteOriginal {
				overwriteStr = yesStr
			}

			rightItems = []settingItem{
				{Label: T(*cfg, "settings_preset", pName), CLI: "--recode-video", Key: "video_preset"},
				{Label: T(*cfg, "settings_audio_format", cfg.AudioFormat), CLI: "--audio-format", Key: "audio_format"},
				{Label: T(*cfg, "settings_sub_langs", cfg.SubLangs), CLI: "--sub-langs", Key: "sub_langs"},
				{Label: T(*cfg, "settings_thumb_fmt", strings.ToUpper(cfg.ThumbnailFormat)), CLI: "--convert-thumbnails", Key: "thumb_fmt"},
				{Label: T(*cfg, "settings_ffmpeg_suffix", suffixStr), CLI: "clean_names", Key: "ffmpeg_suffix"},
				{Label: T(*cfg, "settings_overwrite_orig", overwriteStr), CLI: "replace_src", Key: "overwrite_orig"},
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
			}

			modeName := "ASCII"
			if cfg.LogoMode == "image" {
				modeName = T(*cfg, "logo_mode_img")
			}
			rightItems = append(rightItems, settingItem{Label: T(*cfg, "settings_logo_mode", modeName), CLI: "logo", Key: "logo_mode"})

			if cfg.LogoMode == "ascii" {
				rightItems = append(rightItems, settingItem{Label: T(*cfg, "settings_logo_ascii", cfg.LogoAsciiPreset), CLI: "ascii", Key: "logo_ascii"})
			} else {
				imgName := "(none)"
				if cfg.LogoImagePath != "" {
					imgName = filepath.Base(cfg.LogoImagePath)
				}
				protoName := T(*cfg, "logo_proto_"+cfg.LogoProtocol)
				if protoName == "logo_proto_"+cfg.LogoProtocol {
					protoName = cfg.LogoProtocol
				}
				rightItems = append(rightItems, settingItem{Label: T(*cfg, "settings_logo_protocol", protoName), CLI: "protocol", Key: "logo_protocol"})
				rightItems = append(rightItems, settingItem{Label: T(*cfg, "settings_logo_image", imgName), CLI: "image", Key: "logo_img"})
			}

			rightItems = append(rightItems, settingItem{Label: T(*cfg, "settings_reset"), CLI: "factory_wipe", Key: "reset"})
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
	case "ffmpeg_suffix":
		cfg.UseFFmpegSuffix = !cfg.UseFFmpegSuffix
		_ = core.SaveConfig(*cfg)
	case "overwrite_orig":
		cfg.OverwriteOriginal = !cfg.OverwriteOriginal
		_ = core.SaveConfig(*cfg)
	case "frags":
		choices := []string{"2 streams", "4 streams (default)", "8 streams (fast)", "16 streams (max)"}
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
	case "logo_mode":
		if cfg.LogoMode == "ascii" {
			cfg.LogoMode = "image"
		} else {
			cfg.LogoMode = "ascii"
		}
		_ = core.SaveConfig(*cfg)
	case "logo_protocol":
		labels := []string{
			T(*cfg, "logo_proto_kitty"),
			T(*cfg, "logo_proto_iterm2"),
		}
		vals := []string{"kitty", "iterm2"}
		idx := RunMenu(s, cfg, T(*cfg, "settings_title"), labels, "Choose Image Protocol:", T(*cfg, "footer_nav"))
		if idx >= 0 {
			cfg.LogoProtocol = vals[idx]
			_ = core.SaveConfig(*cfg)
		}
	case "logo_ascii":
		presets := []string{"standard", "coder_mini", "toilet", "rubifont"}
		labels := []string{"1. Standard", "2. Coder Mini", "3. TOIlet", "4. RubiFont"}
		idx := RunMenu(s, cfg, T(*cfg, "settings_title"), labels, "Choose Ascii Art Preset:", T(*cfg, "footer_nav"))
		if idx >= 0 {
			cfg.LogoAsciiPreset = presets[idx]
			_ = core.SaveConfig(*cfg)
		}
	case "logo_img":
		logoDir := filepath.Join(core.GetConfigDir(), "logos")
		_ = os.MkdirAll(logoDir, 0755)
		
		files, _ := os.ReadDir(logoDir)
		var validFiles []string
		for _, f := range files {
			if !f.IsDir() {
				ext := strings.ToLower(filepath.Ext(f.Name()))
				if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
					validFiles = append(validFiles, f.Name())
				}
			}
		}

		if len(validFiles) == 0 {
			ShowMessage(s, cfg, T(*cfg, "settings_title"), []string{
				T(*cfg, "logo_no_images", logoDir),
				"",
				T(*cfg, "logo_warning"),
			}, T(*cfg, "footer_message"))
		} else {
			idx := RunMenu(s, cfg, T(*cfg, "settings_title"), validFiles, T(*cfg, "logo_warning"), T(*cfg, "footer_nav"))
			if idx >= 0 {
				cfg.LogoImagePath = filepath.Join(logoDir, validFiles[idx])
				cfg.LogoMode = "image"
				_ = core.SaveConfig(*cfg)
			}
		}
	case "reset":
		cfgDefault := core.GetDefaultConfig()
		*cfg = cfgDefault
		_ = core.SaveConfig(*cfg)
		ShowMessage(s, cfg, T(*cfg, "settings_title"), []string{"Settings reset to defaults!"}, T(*cfg, "footer_message"))
	}
}