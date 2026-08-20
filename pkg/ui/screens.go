package ui

import (
	"fmt"
	"mediacli/pkg/core"
	"os"
	"path/filepath"
	"strconv"
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
	} else { // 2-Pane Manual Preset Configuration Screen
		pObj, ok := ScreenManualPresetConfig(s, cfg, nil)
		if !ok {
			return
		}
		preset = *pObj
	}

	cmdList := append([]string{"yt-dlp"}, core.BuildYtDlpArgs(preset, *cfg, outDir, false)...)
	cmdList = append(cmdList, url)
	RunWithLog(s, cfg, cmdList, "Download Video", url, outDir)
}

// ScreenManualPresetConfig — Двухпанельный конфигуратор пресета со всеми настройками Stacher 7
func ScreenManualPresetConfig(s tcell.Screen, cfg *core.Config, initialFields map[string]interface{}) (*core.DownloadPreset, bool) {
	fields := core.GetInitialPresetFields()
	if initialFields != nil {
		for k, v := range initialFields {
			fields[k] = v
		}
	}

	leftIdx := 0
	rightIdx := 0
	inRightPane := false

	categories := []string{
		T(*cfg, "tab_set_gen"),
		T(*cfg, "tab_set_conv"),
		T(*cfg, "tab_set_subs_meta"),
		T(*cfg, "tab_set_accel"),
		T(*cfg, "tab_set_cookies_auth"),
		T(*cfg, "tab_set_action"),
	}

	type settingItem struct {
		Label string
		CLI   string
		Key   string
	}

	for {
		w, h := s.Size()
		s.Clear()

		DrawHeader(s, T(*cfg, "manual_config_title"), w, *cfg)
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
			qVal := core.GetString(fields, "quality")
			if qVal == "" {
				qVal = "Best Available"
			} else {
				qVal += "p"
			}
			secVal := core.GetString(fields, "download_section")
			if secVal == "" {
				secVal = "Full Video"
			}
			tmplVal := core.GetString(fields, "output_template")
			if tmplVal == "" {
				tmplVal = "%(title)s.%(ext)s"
			}
			winVal := noStr
			if core.GetBool(fields, "windows_filenames") {
				winVal = yesStr
			}
			restVal := noStr
			if core.GetBool(fields, "restrict_filenames") {
				restVal = yesStr
			}
			noMtimeVal := noStr
			if core.GetBool(fields, "no_mtime") {
				noMtimeVal = yesStr
			}
			archVal := noStr
			if core.GetBool(fields, "use_archive") {
				archVal = yesStr
			}

			rightItems = []settingItem{
				{Label: fmt.Sprintf("Max Quality: %s", qVal), CLI: "-f bestvideo[height<=?]", Key: "quality"},
				{Label: fmt.Sprintf("Time Range / Section: %s", secVal), CLI: "--download-sections", Key: "section"},
				{Label: fmt.Sprintf("Output Template: %s", tmplVal), CLI: "-o template", Key: "template"},
				{Label: fmt.Sprintf("Safe Filenames (NTFS/FAT32): %s", winVal), CLI: "--windows-filenames", Key: "windows_filenames"},
				{Label: fmt.Sprintf("Restrict Filenames (ASCII): %s", restVal), CLI: "--restrict-filenames", Key: "restrict_filenames"},
				{Label: fmt.Sprintf("Keep Download Timestamp: %s", noMtimeVal), CLI: "--no-mtime", Key: "no_mtime"},
				{Label: fmt.Sprintf("Download Archive: %s", archVal), CLI: "--download-archive", Key: "use_archive"},
			}

		case 1: // Video & Audio Codecs
			pName := core.GetString(fields, "video_preset")
			if p, ok := core.VideoPresets[pName]; ok {
				pName = PresetDisplayName(*cfg, p)
			}
			vcodec := core.GetString(fields, "vcodec")
			fps := core.GetString(fields, "fps_limit")
			if fps == "" {
				fps = "Max"
			} else {
				fps += " FPS"
			}
			audioOnly := noStr
			if core.GetBool(fields, "audio_only") {
				audioOnly = yesStr
			}
			audioFmt := strings.ToUpper(core.GetString(fields, "audio_format"))
			audioQ := core.GetString(fields, "audio_quality")

			rightItems = []settingItem{
				{Label: fmt.Sprintf("Video Codec Preset: %s", pName), CLI: "--recode-video", Key: "video_preset"},
				{Label: fmt.Sprintf("Video Codec Filter: %s", strings.ToUpper(vcodec)), CLI: "-f [vcodec^=?]", Key: "vcodec"},
				{Label: fmt.Sprintf("FPS Limit: %s", fps), CLI: "-f [fps<=?]", Key: "fps_limit"},
				{Label: fmt.Sprintf("Audio Only: %s", audioOnly), CLI: "-x / --extract-audio", Key: "audio_only"},
				{Label: fmt.Sprintf("Audio Format: %s", audioFmt), CLI: "--audio-format", Key: "audio_format"},
				{Label: fmt.Sprintf("Audio Quality: %s", audioQ), CLI: "--audio-quality", Key: "audio_quality"},
			}

		case 2: // Subtitles & Metadata
			subsVal := noStr
			if core.GetBool(fields, "subs_enabled") {
				subsVal = yesStr
			}
			langsVal := core.GetString(fields, "sub_langs")
			autoVal := noStr
			if core.GetBool(fields, "auto_subs") {
				autoVal = yesStr
			}
			embedSubsVal := noStr
			if core.GetBool(fields, "embed_subs") {
				embedSubsVal = yesStr
			}
			embedMetaVal := noStr
			if core.GetBool(fields, "embed_metadata") {
				embedMetaVal = yesStr
			}
			embedChapVal := noStr
			if core.GetBool(fields, "embed_chapters") {
				embedChapVal = yesStr
			}
			splitChapVal := noStr
			if core.GetBool(fields, "split_chapters") {
				splitChapVal = yesStr
			}
			sbVal := core.GetString(fields, "sponsorblock")
			extraVal := noStr
			if core.GetBool(fields, "write_extra") {
				extraVal = yesStr
			}

			rightItems = []settingItem{
				{Label: fmt.Sprintf("Download Subtitles: %s", subsVal), CLI: "--write-subs", Key: "subs_enabled"},
				{Label: fmt.Sprintf("Subtitle Languages: %s", langsVal), CLI: "--sub-langs", Key: "sub_langs"},
				{Label: fmt.Sprintf("Include Auto-Generated Subs: %s", autoVal), CLI: "--write-auto-subs", Key: "auto_subs"},
				{Label: fmt.Sprintf("Embed Subs in Container: %s", embedSubsVal), CLI: "--embed-subs", Key: "embed_subs"},
				{Label: fmt.Sprintf("Embed Metadata & Poster: %s", embedMetaVal), CLI: "--embed-metadata", Key: "embed_metadata"},
				{Label: fmt.Sprintf("Embed Chapters: %s", embedChapVal), CLI: "--embed-chapters", Key: "embed_chapters"},
				{Label: fmt.Sprintf("Split by Chapters: %s", splitChapVal), CLI: "--split-chapters", Key: "split_chapters"},
				{Label: fmt.Sprintf("SponsorBlock: %s", strings.ToUpper(sbVal)), CLI: "--sponsorblock", Key: "sponsorblock"},
				{Label: fmt.Sprintf("Save Description & Thumbnail: %s", extraVal), CLI: "--write-description", Key: "write_extra"},
			}

		case 3: // Network & Acceleration
			fragsVal := core.GetString(fields, "concurrent_fragments")
			retriesVal := core.GetString(fields, "retries")
			rateVal := core.GetString(fields, "ratelimit")
			if rateVal == "" {
				rateVal = "Unlimited"
			}
			geoVal := noStr
			if core.GetBool(fields, "geobypass") {
				geoVal = yesStr
			}
			liveVal := noStr
			if core.GetBool(fields, "live_start") {
				liveVal = yesStr
			}
			passVal := core.GetString(fields, "video_password")
			if passVal == "" {
				passVal = "(none)"
			}

			rightItems = []settingItem{
				{Label: fmt.Sprintf("Concurrent Fragments: %s", fragsVal), CLI: "--concurrent-fragments", Key: "concurrent_fragments"},
				{Label: fmt.Sprintf("Connection Retries: %s", retriesVal), CLI: "--retries", Key: "retries"},
				{Label: fmt.Sprintf("Speed Rate Limit: %s", rateVal), CLI: "--limit-rate", Key: "ratelimit"},
				{Label: fmt.Sprintf("Bypass Geo Restrictions: %s", geoVal), CLI: "--geo-bypass", Key: "geobypass"},
				{Label: fmt.Sprintf("Live Stream from Start: %s", liveVal), CLI: "--live-from-start", Key: "live_start"},
				{Label: fmt.Sprintf("Video Password: %s", passVal), CLI: "--video-password", Key: "video_password"},
			}

		case 4: // Cookies & Auth
			cMode := core.GetString(fields, "cookies_mode")
			cBrowser := core.GetString(fields, "cookies_browser")
			if cBrowser == "" {
				cBrowser = "(none)"
			}
			cFile := core.GetString(fields, "cookies_file")
			if cFile == "" {
				cFile = "(none)"
			}
			pMode := core.GetString(fields, "proxy_mode")
			pURL := core.GetString(fields, "proxy_url")
			if pURL == "" {
				pURL = "(none)"
			}

			rightItems = []settingItem{
				{Label: fmt.Sprintf("Cookies Mode: %s", strings.ToUpper(cMode)), CLI: "--cookies", Key: "cookies_mode"},
				{Label: fmt.Sprintf("Browser Cookies: %s", strings.Title(cBrowser)), CLI: "--cookies-from-browser", Key: "cookies_browser"},
				{Label: fmt.Sprintf("Cookies File: %s", cFile), CLI: "--cookies file", Key: "cookies_file"},
				{Label: fmt.Sprintf("Proxy Mode: %s", strings.ToUpper(pMode)), CLI: "--proxy", Key: "proxy_mode"},
				{Label: fmt.Sprintf("Proxy URL: %s", pURL), CLI: "--proxy URL", Key: "proxy_url"},
			}

		case 5: // Actions & Save
			rightItems = []settingItem{
				{Label: "Start Download", CLI: "execute", Key: "act_run"},
				{Label: "Save as Preset & Download", CLI: "save_and_run", Key: "act_save"},
				{Label: "Cancel", CLI: "back", Key: "act_cancel"},
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
					return nil, false
				case tcell.KeyRune:
					if ev.Rune() == 'q' {
						return nil, false
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
					action, finish := handleManualFieldEdit(s, cfg, fields, rightItems[rightIdx].Key)
					if finish {
						preset := &core.DownloadPreset{
							ID:     fmt.Sprintf("preset_%d", time.Now().Unix()),
							Name:   "Manual Download",
							Fields: fields,
						}
						if action == "save" {
							pName, ok := TextInput(s, cfg, "Save Preset", "Enter name for this preset:", "My Custom Preset", T(*cfg, "footer_input"))
							if ok && strings.TrimSpace(pName) != "" {
								preset.Name = strings.TrimSpace(pName)
								cfg.DownloadPresets = append(cfg.DownloadPresets, *preset)
								_ = core.SaveConfig(*cfg)
							}
						}
						return preset, (action != "cancel")
					}
				}
			}
		}
	}
}

func handleManualFieldEdit(s tcell.Screen, cfg *core.Config, fields map[string]interface{}, key string) (action string, finish bool) {
	switch key {
	case "act_run":
		return "run", true
	case "act_save":
		return "save", true
	case "act_cancel":
		return "cancel", true

	// General
	case "quality":
		qLabels := []string{"Best Available", "4K (2160p)", "2K (1440p)", "Full HD (1080p)", "HD (720p)", "SD (480p)", "Custom height in px..."}
		qVals := []string{"", "2160", "1440", "1080", "720", "480", "custom"}
		qi := RunMenu(s, cfg, "Max Resolution", qLabels, "Select maximum video height limit:", T(*cfg, "footer_nav"))
		if qi >= 0 {
			if qVals[qi] == "custom" {
				if val, ok := TextInput(s, cfg, "Max Quality", "Enter height limit in pixels (e.g. 1080):", "", T(*cfg, "footer_input")); ok {
					fields["quality"] = strings.TrimSpace(val)
				}
			} else {
				fields["quality"] = qVals[qi]
			}
		}
	case "section":
		if val, ok := TextInput(s, cfg, "Time Range Cut", "Enter timecode section (e.g. 00:01:00-00:03:30):", core.GetString(fields, "download_section"), T(*cfg, "footer_input")); ok {
			fields["download_section"] = strings.TrimSpace(val)
		}
	case "template":
		if val, ok := TextInput(s, cfg, "Filename Template", "Enter yt-dlp output template:", core.GetString(fields, "output_template"), T(*cfg, "footer_input")); ok {
			fields["output_template"] = strings.TrimSpace(val)
		}
	case "windows_filenames":
		fields["windows_filenames"] = !core.GetBool(fields, "windows_filenames")
	case "restrict_filenames":
		fields["restrict_filenames"] = !core.GetBool(fields, "restrict_filenames")
	case "no_mtime":
		fields["no_mtime"] = !core.GetBool(fields, "no_mtime")
	case "use_archive":
		fields["use_archive"] = !core.GetBool(fields, "use_archive")

	// Video & Audio Codecs
	case "video_preset":
		keys := core.OrderedVideoPresetKeys
		var pNames []string
		for _, k := range keys {
			pNames = append(pNames, PresetDisplayName(*cfg, core.VideoPresets[k]))
		}
		pNames = append(pNames, "Custom FFmpeg Flags...")
		pi := RunMenu(s, cfg, "Video Codec Preset", pNames, "Choose target codec preset (1. MP4 / 2. MKV on top):", T(*cfg, "footer_nav"))
		if pi >= 0 {
			if pi < len(keys) {
				fields["video_preset"] = keys[pi]
			} else {
				if flags, ok := TextInput(s, cfg, "Custom FFmpeg", "Enter custom FFmpeg flags:", "-c:v libx264 -c:a aac", T(*cfg, "footer_input")); ok {
					fields["video_preset"] = "custom"
					fields["custom_flags"] = flags
				}
			}
		}
	case "vcodec":
		vLabels := []string{"Auto (Best available)", "Force AV1 [vcodec^=av01]", "Force VP9 [vcodec^=vp9]", "Force H.264 [vcodec^=avc1]"}
		vVals := []string{"auto", "av1", "vp9", "h264"}
		vi := RunMenu(s, cfg, "Video Codec Preference", vLabels, "Select video stream codec:", T(*cfg, "footer_nav"))
		if vi >= 0 {
			fields["vcodec"] = vVals[vi]
		}
	case "fps_limit":
		fpsLabels := []string{"Max FPS (Unlimited)", "Limit to 60 FPS", "Limit to 30 FPS"}
		fpsVals := []string{"", "60", "30"}
		fi := RunMenu(s, cfg, "FPS Limit", fpsLabels, "Select frame rate limit:", T(*cfg, "footer_nav"))
		if fi >= 0 {
			fields["fps_limit"] = fpsVals[fi]
		}
	case "audio_only":
		fields["audio_only"] = !core.GetBool(fields, "audio_only")
	case "audio_format":
		fmts := []string{"mp3", "flac", "wav", "m4a", "opus"}
		fi := RunMenu(s, cfg, "Audio Format", fmts, "Select audio target format:", T(*cfg, "footer_nav"))
		if fi >= 0 {
			fields["audio_format"] = fmts[fi]
		}
	case "audio_quality":
		qLabels := []string{"Best / VBR 0 (320 kbps)", "High / VBR 2 (256 kbps)", "Medium / VBR 5 (192 kbps)", "Smallest / VBR 9 (128 kbps)"}
		qVals := []string{"0", "2", "5", "9"}
		qi := RunMenu(s, cfg, "Audio Quality", qLabels, "Select audio encoding quality:", T(*cfg, "footer_nav"))
		if qi >= 0 {
			fields["audio_quality"] = qVals[qi]
		}

	// Subtitles & Metadata
	case "subs_enabled":
		fields["subs_enabled"] = !core.GetBool(fields, "subs_enabled")
	case "sub_langs":
		if val, ok := TextInput(s, cfg, "Subtitle Languages", "Languages separated by comma (e.g. ru,en):", core.GetString(fields, "sub_langs"), T(*cfg, "footer_input")); ok {
			fields["sub_langs"] = strings.TrimSpace(val)
		}
	case "auto_subs":
		fields["auto_subs"] = !core.GetBool(fields, "auto_subs")
	case "embed_subs":
		fields["embed_subs"] = !core.GetBool(fields, "embed_subs")
	case "embed_metadata":
		fields["embed_metadata"] = !core.GetBool(fields, "embed_metadata")
	case "embed_chapters":
		fields["embed_chapters"] = !core.GetBool(fields, "embed_chapters")
	case "split_chapters":
		fields["split_chapters"] = !core.GetBool(fields, "split_chapters")
	case "sponsorblock":
		sbLabels := []string{"Disabled (Off)", "Remove Sponsors & Intros (Cut)", "Mark Chapters (No cutting)"}
		sbVals := []string{"off", "remove", "mark"}
		sbi := RunMenu(s, cfg, "SponsorBlock", sbLabels, "Choose SponsorBlock mode:", T(*cfg, "footer_nav"))
		if sbi >= 0 {
			fields["sponsorblock"] = sbVals[sbi]
		}
	case "write_extra":
		fields["write_extra"] = !core.GetBool(fields, "write_extra")

	// Network & Acceleration
	case "concurrent_fragments":
		cLabels := []string{"2 streams", "4 streams (default)", "8 streams (fast)", "16 streams (max)"}
		cVals := []string{"2", "4", "8", "16"}
		ci := RunMenu(s, cfg, "Concurrent Fragments", cLabels, "Parallel download threads for DASH/HLS:", T(*cfg, "footer_nav"))
		if ci >= 0 {
			fields["concurrent_fragments"] = cVals[ci]
		}
	case "retries":
		rLabels := []string{"5 retries", "10 retries (recommended)", "20 retries", "Infinite"}
		rVals := []string{"5", "10", "20", "infinite"}
		ri := RunMenu(s, cfg, "Connection Retries", rLabels, "Retries on connection failure:", T(*cfg, "footer_nav"))
		if ri >= 0 {
			fields["retries"] = rVals[ri]
		}
	case "ratelimit":
		rtLabels := []string{"Unlimited", "10 MB/s", "5 MB/s", "2 MB/s", "Custom limit..."}
		rtVals := []string{"", "10M", "5M", "2M", "custom"}
		rti := RunMenu(s, cfg, "Rate Limit", rtLabels, "Select download speed limit:", T(*cfg, "footer_nav"))
		if rti >= 0 {
			if rtVals[rti] == "custom" {
				if val, ok := TextInput(s, cfg, "Rate Limit", "Enter limit (e.g. 1M, 500K):", "", T(*cfg, "footer_input")); ok {
					fields["ratelimit"] = strings.TrimSpace(val)
				}
			} else {
				fields["ratelimit"] = rtVals[rti]
			}
		}
	case "geobypass":
		fields["geobypass"] = !core.GetBool(fields, "geobypass")
	case "live_start":
		fields["live_start"] = !core.GetBool(fields, "live_start")
	case "video_password":
		if val, ok := TextInput(s, cfg, "Video Password", "Enter password for protected video:", core.GetString(fields, "video_password"), T(*cfg, "footer_input")); ok {
			fields["video_password"] = strings.TrimSpace(val)
		}

	// Cookies & Auth
	case "cookies_mode":
		mLabels := []string{"Use Global Default", "Disabled (None)", "Import from Browser", "Use cookies.txt File"}
		mVals := []string{"default", "none", "browser", "file"}
		mi := RunMenu(s, cfg, "Cookies Mode", mLabels, "Select cookie strategy:", T(*cfg, "footer_nav"))
		if mi >= 0 {
			fields["cookies_mode"] = mVals[mi]
		}
	case "cookies_browser":
		var bLabels []string
		for _, b := range core.SupportedBrowsers {
			bLabels = append(bLabels, strings.Title(b))
		}
		bi := RunMenu(s, cfg, "Browser Cookies", bLabels, "Choose browser:", T(*cfg, "footer_nav"))
		if bi >= 0 {
			fields["cookies_mode"] = "browser"
			fields["cookies_browser"] = core.SupportedBrowsers[bi]
		}
	case "cookies_file":
		if val, ok := TextInput(s, cfg, "Cookies File", "Enter path to cookies.txt:", core.GetString(fields, "cookies_file"), T(*cfg, "footer_input")); ok {
			fields["cookies_mode"] = "file"
			fields["cookies_file"] = strings.TrimSpace(val)
		}
	case "proxy_mode":
		pLabels := []string{"Use Global Default", "Direct Connection (No proxy)", "Custom Proxy URL"}
		pVals := []string{"default", "none", "custom"}
		pi := RunMenu(s, cfg, "Proxy Mode", pLabels, "Select proxy mode:", T(*cfg, "footer_nav"))
		if pi >= 0 {
			fields["proxy_mode"] = pVals[pi]
		}
	case "proxy_url":
		if val, ok := TextInput(s, cfg, "Proxy URL", "Enter proxy (socks5://127.0.0.1:10808):", core.GetString(fields, "proxy_url"), T(*cfg, "footer_input")); ok {
			fields["proxy_mode"] = "custom"
			fields["proxy_url"] = strings.TrimSpace(val)
		}
	}

	return "", false
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
	cmdList = append(cmdList, core.BuildCookieArgs(*cfg, nil)...)
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
	cmdList = append(cmdList, core.BuildCookieArgs(*cfg, nil)...)
	cmdList = append(cmdList, core.BuildProxyArgs(*cfg, "", "")...)
	cmdList = append(cmdList, url)

	RunWithLog(s, cfg, cmdList, "Download Audio", url, outDir)
}

// ============================================================================
// Модуль кадрирования и изменения соотношения сторон (FFmpeg Aspect Ratio & Crop Engine)
// ============================================================================

type AspectPresetItem struct {
	ID       string
	Name     string
	Desc     string
	W        int
	H        int
	RatioVal float64
}

var AspectRatioPresets = []AspectPresetItem{
	{ID: "16:9", Name: "16:9  (Landscape)", Desc: "YouTube, TV, Standard Monitor", W: 16, H: 9, RatioVal: 16.0 / 9.0},
	{ID: "9:16", Name: "9:16  (Vertical)", Desc: "YouTube Shorts, TikTok, Reels, Stories", W: 9, H: 16, RatioVal: 9.0 / 16.0},
	{ID: "1:1", Name: "1:1   (Square)", Desc: "Instagram Square Post, Profile", W: 1, H: 1, RatioVal: 1.0},
	{ID: "4:5", Name: "4:5   (Portrait)", Desc: "Instagram Feed Portrait", W: 4, H: 5, RatioVal: 4.0 / 5.0},
	{ID: "4:3", Name: "4:3   (Classic TV)", Desc: "Retro SD, iPad, Old CRT", W: 4, H: 3, RatioVal: 4.0 / 3.0},
	{ID: "21:9", Name: "21:9  (Cinematic)", Desc: "Ultrawide Cinema Scope", W: 21, H: 9, RatioVal: 21.0 / 9.0},
	{ID: "9:21", Name: "9:21  (Ultra Tall)", Desc: "Tall Mobile Screen / Pinterest", W: 9, H: 21, RatioVal: 9.0 / 21.0},
	{ID: "custom", Name: "Custom Ratio...", Desc: "User defined proportion (e.g. 3:2)", W: 0, H: 0, RatioVal: 0},
}

func ScreenAspectRatio(s tcell.Screen, cfg *core.Config) {
	fpath, ok := TextInput(s, cfg, T(*cfg, "aspect_title"), T(*cfg, "aspect_prompt_file"), "", T(*cfg, "footer_input"))
	if !ok || strings.TrimSpace(fpath) == "" {
		return
	}

	p := core.ParseUserPath(fpath)
	if _, err := os.Stat(p); err != nil {
		ShowMessage(s, cfg, T(*cfg, "aspect_title"), []string{T(*cfg, "convert_err_notfound", p)}, T(*cfg, "footer_message"))
		return
	}

	srcW, srcH := 1920, 1080
	if probe, err := core.ProbeMedia(p); err == nil {
		for _, st := range probe.Streams {
			if st.CodecType == "video" && st.Width > 0 && st.Height > 0 {
				srcW = st.Width
				srcH = st.Height
				break
			}
		}
	}

	ratioIdx := 1 // По умолчанию 9:16 (популярно для мобильных видео)
	if srcW < srcH {
		ratioIdx = 0 // Если исходник вертикальный, предлагаем 16:9
	}
	customW, customH := 16, 9

	modeIdx := 0 // 0: Crop, 1: Letterbox, 2: Blur BG, 3: Stretch
	modes := []string{
		T(*cfg, "aspect_mode_crop"),
		T(*cfg, "aspect_mode_letterbox"),
		T(*cfg, "aspect_mode_blur"),
		T(*cfg, "aspect_mode_stretch"),
	}

	resIdx := 0 // 0: Auto, 1: 1080p, 2: 720p, 3: 4K (2160p)
	resLabels := []string{"Auto (Max Quality)", "1080p (Full HD)", "720p (HD)", "4K (2160p UHD)"}

	codecIdx := 0 // 0: H.264, 1: H.265, 2: ProRes, 3: DNxHR
	codecLabels := []string{"H.264 (Universal MP4)", "H.265 / HEVC (High Efficiency)", "Apple ProRes 422 (.mov)", "DaVinci Resolve DNxHR HQ (.mov)"}

	curSel := 0
	totalOptions := 6 // 0: Ratio, 1: Mode, 2: Res, 3: Codec, 4: Run, 5: Cancel

	for {
		w, h := s.Size()
		s.Clear()

		DrawHeader(s, T(*cfg, "aspect_title"), w, *cfg)

		leftW := min(46, w/2)
		divX := leftW + 2

		// Разделитель
		for y := 2; y < h-2; y++ {
			s.SetContent(divX, y, '│', nil, GetDimStyle(*cfg))
		}

		// Выбранные параметры
		activePreset := AspectRatioPresets[ratioIdx]
		rW, rH := activePreset.W, activePreset.H
		if activePreset.ID == "custom" {
			rW, rH = customW, customH
		}
		ratioVal := float64(rW) / float64(rH)

		// 1. Левая панель настроек
		y := 3
		items := []struct {
			Label string
			Value string
		}{
			{Label: T(*cfg, "aspect_ratio_opt"), Value: activePreset.Name},
			{Label: T(*cfg, "aspect_mode_opt"), Value: modes[modeIdx]},
			{Label: T(*cfg, "aspect_res_opt"), Value: resLabels[resIdx]},
			{Label: T(*cfg, "aspect_codec_opt"), Value: codecLabels[codecIdx]},
			{Label: T(*cfg, "aspect_btn_run"), Value: ""},
			{Label: T(*cfg, "aspect_btn_cancel"), Value: ""},
		}

		for i, item := range items {
			isSel := (i == curSel)
			style := GetBaseStyle(*cfg)
			marker := "  "
			if isSel {
				style = GetHighlightStyle(*cfg)
				marker = "► "
			}

			if i >= 4 { // Кнопки действий
				DrawString(s, 2, y, marker+item.Label, leftW-2, style)
				y += 2
				continue
			}

			DrawString(s, 2, y, marker+item.Label+":", leftW-2, style)
			valStyle := GetAccentStyle(*cfg)
			if isSel {
				valStyle = GetHighlightStyle(*cfg)
			}
			DrawString(s, 5, y+1, "« "+item.Value+" »", leftW-6, valStyle)
			y += 3
		}

		// 2. Правая панель: Интерактивный предпросмотр соотношения
		rightX := divX + 3
		rightW := w - rightX - 2
		rightH := h - 6
		drawAspectRatioVisualizer(s, *cfg, rightX, 3, rightW, rightH, srcW, srcH, rW, rH, ratioVal, modeIdx, resIdx, activePreset.ID)

		DrawFooter(s, T(*cfg, "aspect_footer_nav"), w, h)
		s.Show()

		triggerAction := func() bool {
			switch curSel {
			case 0: // Ratio
				var names []string
				for _, pr := range AspectRatioPresets {
					names = append(names, fmt.Sprintf("%-20s (%s)", pr.Name, pr.Desc))
				}
				ri := RunMenu(s, cfg, T(*cfg, "aspect_ratio_opt"), names, "Select Target Proportion:", T(*cfg, "footer_nav"))
				if ri >= 0 {
					ratioIdx = ri
					if AspectRatioPresets[ri].ID == "custom" {
						if val, ok := TextInput(s, cfg, "Custom Aspect Ratio", "Enter ratio as W:H (e.g. 3:2, 2.39:1):", "3:2", T(*cfg, "footer_input")); ok {
							parts := strings.Split(val, ":")
							if len(parts) == 2 {
								wInt, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
								hInt, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
								if wInt > 0 && hInt > 0 {
									customW = wInt
									customH = hInt
								}
							}
						}
					}
				}
			case 1: // Framing Mode
				mi := RunMenu(s, cfg, T(*cfg, "aspect_mode_opt"), modes, "Choose how frame adapts to new ratio:", T(*cfg, "footer_nav"))
				if mi >= 0 {
					modeIdx = mi
				}
			case 2: // Resolution
				ri := RunMenu(s, cfg, T(*cfg, "aspect_res_opt"), resLabels, "Select Output Resolution:", T(*cfg, "footer_nav"))
				if ri >= 0 {
					resIdx = ri
				}
			case 3: // Codec
				ci := RunMenu(s, cfg, T(*cfg, "aspect_codec_opt"), codecLabels, "Select Video Encoding Codec:", T(*cfg, "footer_nav"))
				if ci >= 0 {
					codecIdx = ci
				}
			case 4: // Run
				outW, outH := calcTargetDimensions(srcW, srcH, rW, rH, resIdx)
				cmdList, plan := buildAspectRatioFFmpegArgs(p, outW, outH, rW, rH, modeIdx, codecIdx, *cfg)
				RunWithLogHook(s, cfg, cmdList, "Aspect Ratio Crop", filepath.Base(p), plan.TargetDisplay, plan.OnComplete)
				return true
			case 5: // Cancel
				return true
			}
			return false
		}

		ev := s.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if CheckTerminalHotkey(ev) {
				GlobalTerminal.Open(s, cfg)
				continue
			}
			switch ev.Key() {
			case tcell.KeyUp:
				if curSel > 0 {
					curSel--
				} else {
					curSel = totalOptions - 1
				}
			case tcell.KeyDown:
				if curSel < totalOptions-1 {
					curSel++
				} else {
					curSel = 0
				}
			case tcell.KeyEscape:
				return
			case tcell.KeyEnter, tcell.KeyRight:
				if triggerAction() {
					return
				}
			case tcell.KeyRune:
				if ev.Rune() == 'q' {
					return
				} else if ev.Rune() == 'k' {
					if curSel > 0 {
						curSel--
					}
				} else if ev.Rune() == 'j' {
					if curSel < totalOptions-1 {
						curSel++
					}
				} else if ev.Rune() == ' ' || ev.Rune() == 'l' {
					if triggerAction() {
						return
					}
				}
			}
		}
	}
}

// Отрисовка точного пропорционального визуализатора соотношения сторон в TUI
func drawAspectRatioVisualizer(s tcell.Screen, cfg core.Config, startX, startY, availW, availH int, srcW, srcH, rW, rH int, targetRatio float64, modeIdx, resIdx int, presetID string) {
	if availW < 20 || availH < 10 {
		return
	}

	titleStyle := GetAccentStyle(cfg)
	DrawString(s, startX, startY, "► "+T(cfg, "aspect_preview_title"), availW, titleStyle)
	DrawString(s, startX, startY+1, strings.Repeat("─", availW), availW, GetDimStyle(cfg))

	// В текстовом терминале ячейка шрифта имеет пропорцию ~1:2 (высота вдвое больше ширины)
	// Для корректного визуала умножаем целевое соотношение W/H на 2.0 при расчете символов
	maxBoxW := min(availW-4, 38)
	maxBoxH := min(availH-9, 11)

	boxH := maxBoxH
	boxW := int(float64(boxH) * 2.0 * targetRatio + 0.5)

	if boxW > maxBoxW {
		boxW = maxBoxW
		boxH = int(float64(boxW) / (2.0 * targetRatio) + 0.5)
	}

	if boxW < 6 {
		boxW = 6
	}
	if boxH < 3 {
		boxH = 3
	}

	boxX := startX + max(0, (availW-boxW)/2)
	boxY := startY + 3

	frameStyle := GetHighlightStyle(cfg)
	innerStyle := GetBaseStyle(cfg)
	barStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray).Background(tcell.ColorBlack)
	blurStyle := tcell.StyleDefault.Foreground(tcell.ColorPurple).Background(tcell.ColorDarkSlateGray)
	cropCutStyle := tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)

	// Отрисовка рамки целевого соотношения
	for y := 0; y < boxH; y++ {
		for x := 0; x < boxW; x++ {
			isBorder := (y == 0 || y == boxH-1 || x == 0 || x == boxW-1)
			if isBorder {
				ch := '│'
				if y == 0 && x == 0 {
					ch = '┌'
				} else if y == 0 && x == boxW-1 {
					ch = '┐'
				} else if y == boxH-1 && x == 0 {
					ch = '└'
				} else if y == boxH-1 && x == boxW-1 {
					ch = '┘'
				} else if y == 0 || y == boxH-1 {
					ch = '─'
				}
				s.SetContent(boxX+x, boxY+y, ch, nil, frameStyle)
			} else {
				s.SetContent(boxX+x, boxY+y, ' ', nil, innerStyle)
			}
		}
	}

	// Визуализация содержимого и обрезки внутри рамки
	srcRatio := float64(srcW) / float64(srcH)
	innerW := boxW - 2
	innerH := boxH - 2

	switch modeIdx {
	case 1: // Letterbox (Черные полосы)
		if srcRatio > targetRatio { // Исходник шире -> полосы сверху и снизу
			barH := int(float64(innerH) * (1.0 - (targetRatio / srcRatio)) / 2.0)
			if barH < 1 && innerH >= 4 {
				barH = 1
			}
			for y := 1; y < boxH-1; y++ {
				isBar := (y <= barH || y >= boxH-1-barH)
				for x := 1; x < boxW-1; x++ {
					if isBar {
						s.SetContent(boxX+x, boxY+y, '░', nil, barStyle)
					} else {
						s.SetContent(boxX+x, boxY+y, '▓', nil, innerStyle)
					}
				}
			}
		} else if srcRatio < targetRatio { // Исходник уже -> полосы по бокам
			barW := int(float64(innerW) * (1.0 - (srcRatio / targetRatio)) / 2.0)
			if barW < 1 && innerW >= 6 {
				barW = 2
			}
			for y := 1; y < boxH-1; y++ {
				for x := 1; x < boxW-1; x++ {
					isBar := (x <= barW || x >= boxW-1-barW)
					if isBar {
						s.SetContent(boxX+x, boxY+y, '░', nil, barStyle)
					} else {
						s.SetContent(boxX+x, boxY+y, '▓', nil, innerStyle)
					}
				}
			}
		} else {
			for y := 1; y < boxH-1; y++ {
				for x := 1; x < boxW-1; x++ {
					s.SetContent(boxX+x, boxY+y, '▓', nil, innerStyle)
				}
			}
		}

	case 2: // Blur Background (Размытый фон)
		if srcRatio > targetRatio { // Поля сверху/снизу заполнены размытием
			barH := int(float64(innerH) * (1.0 - (targetRatio / srcRatio)) / 2.0)
			for y := 1; y < boxH-1; y++ {
				isBlur := (y <= barH || y >= boxH-1-barH)
				for x := 1; x < boxW-1; x++ {
					if isBlur {
						s.SetContent(boxX+x, boxY+y, '▒', nil, blurStyle)
					} else {
						s.SetContent(boxX+x, boxY+y, '█', nil, innerStyle)
					}
				}
			}
		} else { // Поля по бокам заполнены размытием
			barW := int(float64(innerW) * (1.0 - (srcRatio / targetRatio)) / 2.0)
			for y := 1; y < boxH-1; y++ {
				for x := 1; x < boxW-1; x++ {
					isBlur := (x <= barW || x >= boxW-1-barW)
					if isBlur {
						s.SetContent(boxX+x, boxY+y, '▒', nil, blurStyle)
					} else {
						s.SetContent(boxX+x, boxY+y, '█', nil, innerStyle)
					}
				}
			}
		}

	case 0: // Crop to Fill (Обрезка с заполнением)
		for y := 1; y < boxH-1; y++ {
			for x := 1; x < boxW-1; x++ {
				s.SetContent(boxX+x, boxY+y, '█', nil, innerStyle)
			}
		}
		// Индикаторы обрезки
		if srcRatio > targetRatio && boxW >= 10 {
			DrawString(s, boxX+1, boxY+boxH/2, "✂", 2, cropCutStyle)
			DrawString(s, boxX+boxW-2, boxY+boxH/2, "✂", 2, cropCutStyle)
		} else if srcRatio < targetRatio && boxH >= 5 {
			DrawString(s, boxX+boxW/2, boxY+1, "✂", 2, cropCutStyle)
			DrawString(s, boxX+boxW/2, boxY+boxH-2, "✂", 2, cropCutStyle)
		}

	case 3: // Stretch (Растяжение)
		for y := 1; y < boxH-1; y++ {
			for x := 1; x < boxW-1; x++ {
				s.SetContent(boxX+x, boxY+y, '≈', nil, innerStyle)
			}
		}
	}

	// Описание под визуализатором
	infoY := boxY + boxH + 1
	outW, outH := calcTargetDimensions(srcW, srcH, rW, rH, resIdx)

	DrawString(s, startX, infoY, fmt.Sprintf("Ratio  : %d:%d (%.2f:1)", rW, rH, targetRatio), availW, GetAccentStyle(cfg))
	DrawString(s, startX, infoY+1, fmt.Sprintf("Source : %dx%d (%.2f:1)", srcW, srcH, srcRatio), availW, GetDimStyle(cfg))
	DrawString(s, startX, infoY+2, fmt.Sprintf("Target : %dx%d (Canvas)", outW, outH), availW, GetHighlightStyle(cfg))

	modeDesc := ""
	switch modeIdx {
	case 0:
		modeDesc = "[Crop] Fills frame. Content outside ratio is cut."
		if cfg.Language == "ru" {
			modeDesc = "[Обрезка] Полный кадр без черных полос (края срезаются)."
		}
	case 1:
		modeDesc = "[Letterbox] 100% video kept. Clean black bars added."
		if cfg.Language == "ru" {
			modeDesc = "[Черные полосы] 100% видео без потерь (с аккуратными полями)."
		}
	case 2:
		modeDesc = "[Blur BG] 100% video kept. Padded with blurred video."
		if cfg.Language == "ru" {
			modeDesc = "[Размытие] 100% видео без потерь с красивым размытым фоном."
		}
	case 3:
		modeDesc = "[Stretch] Stretches geometry to match target proportion."
		if cfg.Language == "ru" {
			modeDesc = "[Растяжение] Заполняет экран без полей с искажением геометрии."
		}
	}

	DrawString(s, startX, infoY+4, modeDesc, availW, GetDimStyle(cfg))
}

func calcTargetDimensions(srcW, srcH, rW, rH, resIdx int) (int, int) {
	if rW <= 0 || rH <= 0 {
		rW, rH = 16, 9
	}
	ratio := float64(rW) / float64(rH)

	baseH := 1080
	switch resIdx {
	case 1: // 1080p
		baseH = 1080
	case 2: // 720p
		baseH = 720
	case 3: // 4K
		baseH = 2160
	case 0: // Auto (на базе разрешения исходника)
		maxDim := max(srcW, srcH)
		if maxDim >= 3000 {
			baseH = 2160
		} else if maxDim >= 1600 {
			baseH = 1080
		} else {
			baseH = 720
		}
	}

	var outW, outH int
	if rW >= rH { // Горизонтальное или квадратное
		outH = baseH
		outW = roundEven(float64(outH) * ratio)
	} else { // Вертикальное
		outW = baseH
		outH = roundEven(float64(outW) / ratio)
	}

	return outW, outH
}

func roundEven(val float64) int {
	i := int(val + 0.5)
	if i%2 != 0 {
		i++
	}
	return i
}

func buildAspectRatioFFmpegArgs(inputPath string, outW, outH, rW, rH, modeIdx, codecIdx int, cfg core.Config) ([]string, core.FFmpegOutputPlan) {
	ext := ".mp4"
	var codecFlags []string

	switch codecIdx {
	case 0: // H.264 MP4
		codecFlags = []string{"-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "medium", "-crf", "20", "-c:a", "aac", "-b:a", "192k"}
		ext = ".mp4"
	case 1: // H.265 / HEVC MP4
		codecFlags = []string{"-c:v", "libx265", "-pix_fmt", "yuv420p", "-preset", "medium", "-crf", "23", "-c:a", "aac", "-b:a", "192k"}
		ext = ".mp4"
	case 2: // Apple ProRes MOV
		codecFlags = []string{"-c:v", "prores_ks", "-profile:v", "3", "-c:a", "pcm_s16le"}
		ext = ".mov"
	case 3: // DaVinci DNxHR MOV
		codecFlags = []string{"-c:v", "dnxhd", "-profile:v", "dnxhr_hq", "-c:a", "pcm_s16le"}
		ext = ".mov"
	}

	var filterArgs []string
	modeSuffix := "_crop"

	switch modeIdx {
	case 0: // Crop to fill
		modeSuffix = fmt.Sprintf("_%dx%d_crop", rW, rH)
		filterArgs = []string{"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,setsar=1", outW, outH, outW, outH)}
	case 1: // Letterbox (Black bars)
		modeSuffix = fmt.Sprintf("_%dx%d_letterbox", rW, rH)
		filterArgs = []string{"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black,setsar=1", outW, outH, outW, outH)}
	case 2: // Blur background
		modeSuffix = fmt.Sprintf("_%dx%d_blur", rW, rH)
		filterComplex := fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,boxblur=25:5[bg];[0:v]scale=%d:%d:force_original_aspect_ratio=decrease[fg];[bg][fg]overlay=(W-w)/2:(H-h)/2,setsar=1", outW, outH, outW, outH, outW, outH)
		filterArgs = []string{"-filter_complex", filterComplex}
	case 3: // Stretch
		modeSuffix = fmt.Sprintf("_%dx%d_stretch", rW, rH)
		filterArgs = []string{"-vf", fmt.Sprintf("scale=%d:%d,setsar=1", outW, outH)}
	}

	plan := core.PrepareFFmpegOutput(inputPath, ext, modeSuffix, cfg)

	cmdList := append([]string{"ffmpeg", "-y", "-i", inputPath}, filterArgs...)
	cmdList = append(cmdList, codecFlags...)
	cmdList = append(cmdList, plan.TempOutputPath)

	return cmdList, plan
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
	case "reset":
		cfgDefault := core.GetDefaultConfig()
		*cfg = cfgDefault
		_ = core.SaveConfig(*cfg)
		ShowMessage(s, cfg, T(*cfg, "settings_title"), []string{"Settings reset to defaults!"}, T(*cfg, "footer_message"))
	}
}