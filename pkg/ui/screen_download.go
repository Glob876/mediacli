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