package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mediacli/pkg/core"

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
		if cfg.PresetDefaults == nil {
			cfg.PresetDefaults = core.GetInitialPresetFields()
		}
		// Normalize numeric quality persisted as float64 so GetString coercion works
		preset = core.DownloadPreset{ID: "default", Name: "Default", Fields: cfg.PresetDefaults}
		if q := core.GetString(preset.Fields, "quality"); q == "" {
			// Best Available is intentional; inform user once per session via footer hint
			// (no blocking prompt — format now correctly picks highest, not 360p)
		}
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

	if core.IsExternalTranscodeEnabled(*cfg, preset.Fields) {
		RunDownloadExternal(s, cfg, preset, outDir, url)
	} else {
		cmdList := append([]string{"yt-dlp"}, core.BuildYtDlpArgs(preset, *cfg, outDir, false)...)
		cmdList = append(cmdList, url)
		RunWithLog(s, cfg, cmdList, "Download Video", url, outDir)
	}
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
	leftFirst := 0
	rightFirst := 0
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

		leftHits := make([]menuHit, 0, len(categories))
		visibleLeft := max(1, h-7)
		leftFirst = windowStart(leftIdx, leftFirst, visibleLeft)
		for i := leftFirst; i < len(categories); i++ {
			cat := categories[i]
			y := 3 + i - leftFirst
			if y >= h-4 {
				break
			}
			isSel := i == leftIdx
			style := GetBaseStyle(*cfg)
			marker := "  "
			if isSel && !inRightPane {
				style = GetHighlightStyle(*cfg)
				marker = "► "
			} else if isSel {
				style = GetBaseStyle(*cfg).Bold(true)
				marker = "► "
			}
			DrawSpan(s, 2, y, leftW, marker+cat, style)
			leftHits = append(leftHits, menuHit{minX: 2, maxX: 2 + leftW, y: y, index: i})
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
				qVal = T(*cfg, "wiz_best")
			} else {
				qVal += "p"
			}
			secVal := core.GetString(fields, "download_section")
			if secVal == "" {
				secVal = T(*cfg, "wiz_full")
			}
			tmplVal := core.GetString(fields, "output_template")
			if tmplVal == "" {
				tmplVal = T(*cfg, "wiz_default_template")
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
			forceVal := noStr
			if core.GetBool(fields, "force_overwrite") {
				forceVal = yesStr
			}

			rightItems = []settingItem{
				{Label: T(*cfg, "wiz_quality", qVal), CLI: "-f bestvideo[height<=?]", Key: "quality"},
				{Label: T(*cfg, "wiz_section", secVal), CLI: "--download-sections", Key: "section"},
				{Label: T(*cfg, "wiz_template", tmplVal), CLI: "-o template", Key: "template"},
				{Label: T(*cfg, "wiz_win_names", winVal), CLI: "--windows-filenames", Key: "windows_filenames"},
				{Label: T(*cfg, "wiz_restrict", restVal), CLI: "--restrict-filenames", Key: "restrict_filenames"},
				{Label: T(*cfg, "wiz_mtime", noMtimeVal), CLI: "--no-mtime", Key: "no_mtime"},
				{Label: T(*cfg, "wiz_archive", archVal), CLI: "--download-archive", Key: "use_archive"},
				{Label: T(*cfg, "wiz_overwrite", forceVal), CLI: "--force-overwrites", Key: "force_overwrite"},
			}

		case 1: // Video & Audio Codecs
			pName := core.GetString(fields, "video_preset")
			if p, ok := core.VideoPresets[pName]; ok {
				pName = PresetDisplayName(*cfg, p)
			}
			vcodec := core.GetString(fields, "vcodec")
			fps := core.GetString(fields, "fps_limit")
			if fps == "" {
				fps = T(*cfg, "wiz_max")
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
				{Label: T(*cfg, "wiz_preset", pName), CLI: "--recode-video", Key: "video_preset"},
				{Label: T(*cfg, "wiz_vcodec", strings.ToUpper(vcodec)), CLI: "-f [vcodec^=?]", Key: "vcodec"},
				{Label: T(*cfg, "wiz_fps", fps), CLI: "-f [fps<=?]", Key: "fps_limit"},
				{Label: T(*cfg, "wiz_audio_only", audioOnly), CLI: "-x / --extract-audio", Key: "audio_only"},
				{Label: T(*cfg, "wiz_audio_fmt", audioFmt), CLI: "--audio-format", Key: "audio_format"},
				{Label: T(*cfg, "wiz_audio_q", audioQ), CLI: "--audio-quality", Key: "audio_quality"},
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
				{Label: T(*cfg, "wiz_subs", subsVal), CLI: "--write-subs", Key: "subs_enabled"},
				{Label: T(*cfg, "wiz_sub_langs", langsVal), CLI: "--sub-langs", Key: "sub_langs"},
				{Label: T(*cfg, "wiz_auto_subs", autoVal), CLI: "--write-auto-subs", Key: "auto_subs"},
				{Label: T(*cfg, "wiz_embed_subs", embedSubsVal), CLI: "--embed-subs", Key: "embed_subs"},
				{Label: T(*cfg, "wiz_embed_meta", embedMetaVal), CLI: "--embed-metadata", Key: "embed_metadata"},
				{Label: T(*cfg, "wiz_embed_chap", embedChapVal), CLI: "--embed-chapters", Key: "embed_chapters"},
				{Label: T(*cfg, "wiz_split", splitChapVal), CLI: "--split-chapters", Key: "split_chapters"},
				{Label: T(*cfg, "wiz_sponsor", strings.ToUpper(sbVal)), CLI: "--sponsorblock", Key: "sponsorblock"},
				{Label: T(*cfg, "wiz_extra", extraVal), CLI: "--write-description", Key: "write_extra"},
			}

		case 3: // Network & Acceleration
			fragsVal := core.GetString(fields, "concurrent_fragments")
			retriesVal := core.GetString(fields, "retries")
			rateVal := core.GetString(fields, "ratelimit")
			if rateVal == "" {
				rateVal = T(*cfg, "wiz_unlimited")
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
				passVal = T(*cfg, "wiz_none")
			} else {
				passVal = strings.Repeat("•", len([]rune(passVal)))
			}

			rightItems = []settingItem{
				{Label: T(*cfg, "wiz_frags", fragsVal), CLI: "--concurrent-fragments", Key: "concurrent_fragments"},
				{Label: T(*cfg, "wiz_retries", retriesVal), CLI: "--retries", Key: "retries"},
				{Label: T(*cfg, "wiz_rate", rateVal), CLI: "--limit-rate", Key: "ratelimit"},
				{Label: T(*cfg, "wiz_geo", geoVal), CLI: "--geo-bypass", Key: "geobypass"},
				{Label: T(*cfg, "wiz_live", liveVal), CLI: "--live-from-start", Key: "live_start"},
				{Label: T(*cfg, "wiz_password", passVal), CLI: "--video-password", Key: "video_password"},
			}

		case 4: // Cookies & Auth
			cMode := core.GetString(fields, "cookies_mode")
			cBrowser := core.GetString(fields, "cookies_browser")
			if cBrowser == "" {
				cBrowser = T(*cfg, "wiz_none")
			} else {
				cBrowser = titleWord(cBrowser)
			}
			cFile := core.GetString(fields, "cookies_file")
			if cFile == "" {
				cFile = T(*cfg, "wiz_none")
			}
			pMode := core.GetString(fields, "proxy_mode")
			pURL := core.GetString(fields, "proxy_url")
			if pURL == "" {
				pURL = T(*cfg, "wiz_none")
			}

			rightItems = []settingItem{
				{Label: T(*cfg, "wiz_cookies_mode", strings.ToUpper(cMode)), CLI: "--cookies", Key: "cookies_mode"},
				{Label: T(*cfg, "wiz_cookies_browser", cBrowser), CLI: "--cookies-from-browser", Key: "cookies_browser"},
				{Label: T(*cfg, "wiz_cookies_file", cFile), CLI: "--cookies file", Key: "cookies_file"},
				{Label: T(*cfg, "wiz_proxy_mode", strings.ToUpper(pMode)), CLI: "--proxy", Key: "proxy_mode"},
				{Label: T(*cfg, "wiz_proxy_url", pURL), CLI: "--proxy URL", Key: "proxy_url"},
			}

		case 5: // Actions & Save
			rightItems = []settingItem{
				{Label: T(*cfg, "wiz_act_run"), CLI: "execute", Key: "act_run"},
				{Label: T(*cfg, "wiz_act_save"), CLI: "save_and_run", Key: "act_save"},
				{Label: T(*cfg, "wiz_act_cancel"), CLI: "back", Key: "act_cancel"},
			}
		}

		rightX := divX + 3
		rightW := w - rightX - 2
		if rightIdx >= len(rightItems) {
			rightIdx = 0
		}
		rightHits := make([]menuHit, 0, len(rightItems))
		visibleRight := max(1, h-7)
		rightFirst = windowStart(rightIdx, rightFirst, visibleRight)
		for idx := rightFirst; idx < len(rightItems); idx++ {
			itm := rightItems[idx]
			y := 3 + idx - rightFirst
			if y >= h-4 {
				break
			}
			isSel := idx == rightIdx && inRightPane
			style := GetBaseStyle(*cfg)
			marker := "  "
			if isSel {
				style = GetHighlightStyle(*cfg)
				marker = "> "
			}
			tag := "[" + itm.CLI + "]"
			tagW := StringWidth(tag)
			labelW := rightW
			if rightW > tagW+4 {
				labelW = rightW - tagW - 2
			}
			DrawSpan(s, rightX, y, max(0, labelW), marker+itm.Label, style)
			if rightW > tagW+4 {
				DrawString(s, rightX+labelW, y, tag, tagW, GetDimStyle(*cfg))
			}
			rightHits = append(rightHits, menuHit{minX: rightX, maxX: rightX + max(0, rightW), y: y, index: idx})
		}

		footer := T(*cfg, "footer_vertical_tabs")
		if inRightPane {
			footer = T(*cfg, "footer_vertical_items")
		}
		DrawFooter(s, footer, w, h, *cfg)
		s.Show()

		confirmManual := func(index int) (*core.DownloadPreset, bool, bool) {
			if index < 0 || index >= len(rightItems) {
				return nil, false, false
			}
			rightIdx = index
			inRightPane = true
			action, finish := handleManualFieldEdit(s, cfg, fields, rightItems[index].Key)
			if !finish {
				return nil, false, false
			}
			preset := &core.DownloadPreset{
				ID:     fmt.Sprintf("preset_%d", time.Now().Unix()),
				Name:   T(*cfg, "wiz_manual_name"),
				Fields: fields,
			}
			if action == "save" {
				pName, ok := TextInput(s, cfg, T(*cfg, "wiz_save_title"), T(*cfg, "wiz_save_prompt"), T(*cfg, "wiz_save_default"), T(*cfg, "footer_input"))
				if ok && strings.TrimSpace(pName) != "" {
					preset.Name = strings.TrimSpace(pName)
					cfg.DownloadPresets = append(cfg.DownloadPresets, *preset)
					_ = core.SaveConfig(*cfg)
				}
			}
			return preset, action != "cancel", true
		}
		ev, resized := pollUI(s)
		if resized {
			continue
		}
		switch ev := ev.(type) {
		case *tcell.EventMouse:
			x, y := ev.Position()
			if ev.Buttons()&tcell.WheelUp != 0 {
				if inRightPane {
					rightIdx = wrapIndex(rightIdx, -1, len(rightItems))
				} else {
					leftIdx = wrapIndex(leftIdx, -1, len(categories))
					rightIdx = 0
				}
			} else if ev.Buttons()&tcell.WheelDown != 0 {
				if inRightPane {
					rightIdx = wrapIndex(rightIdx, 1, len(rightItems))
				} else {
					leftIdx = wrapIndex(leftIdx, 1, len(categories))
					rightIdx = 0
				}
			} else if ev.Buttons()&tcell.Button1 != 0 {
				if hit, ok := hitIndex(leftHits, x, y); ok {
					leftIdx = hit
					rightIdx = 0
					inRightPane = false
				} else if hit, ok := hitIndex(rightHits, x, y); ok {
					if preset, proceed, done := confirmManual(hit); done {
						return preset, proceed
					}
				}
			}
		case *tcell.EventKey:
			if CheckPaletteHotkey(ev) {
				GlobalPalette.Open(s, cfg)
				continue
			}
			if !inRightPane {
				switch ev.Key() {
				case tcell.KeyUp:
					leftIdx = wrapIndex(leftIdx, -1, len(categories))
					rightIdx = 0
				case tcell.KeyDown:
					leftIdx = wrapIndex(leftIdx, 1, len(categories))
					rightIdx = 0
				case tcell.KeyEnter, tcell.KeyRight:
					inRightPane = true
					rightIdx = 0
				case tcell.KeyEscape:
					return nil, false
				case tcell.KeyRune:
					switch ev.Rune() {
					case 'q':
						return nil, false
					case 'k':
						leftIdx = wrapIndex(leftIdx, -1, len(categories))
						rightIdx = 0
					case 'j':
						leftIdx = wrapIndex(leftIdx, 1, len(categories))
						rightIdx = 0
					case 'l':
						inRightPane = true
					}
				}
			} else {
				switch ev.Key() {
				case tcell.KeyUp:
					rightIdx = wrapIndex(rightIdx, -1, len(rightItems))
				case tcell.KeyDown:
					rightIdx = wrapIndex(rightIdx, 1, len(rightItems))
				case tcell.KeyEscape, tcell.KeyLeft:
					inRightPane = false
				case tcell.KeyEnter:
					if preset, proceed, done := confirmManual(rightIdx); done {
						return preset, proceed
					}
				case tcell.KeyRune:
					switch ev.Rune() {
					case 'k':
						rightIdx = wrapIndex(rightIdx, -1, len(rightItems))
					case 'j':
						rightIdx = wrapIndex(rightIdx, 1, len(rightItems))
					case 'h':
						inRightPane = false
					case 'q':
						return nil, false
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
		qLabels := []string{T(*cfg, "wiz_q_best"), T(*cfg, "wiz_q_2160"), T(*cfg, "wiz_q_1440"), T(*cfg, "wiz_q_1080"), T(*cfg, "wiz_q_720"), T(*cfg, "wiz_q_480"), T(*cfg, "wiz_q_custom")}
		qVals := []string{"", "2160", "1440", "1080", "720", "480", "custom"}
		qi := RunMenu(s, cfg, T(*cfg, "wiz_quality_title"), qLabels, T(*cfg, "wiz_quality_hint"), T(*cfg, "footer_nav"))
		if qi >= 0 {
			if qVals[qi] == "custom" {
				if val, ok := TextInput(s, cfg, T(*cfg, "wiz_quality_title"), T(*cfg, "wiz_quality_prompt"), "", T(*cfg, "footer_input")); ok {
					fields["quality"] = strings.TrimSpace(val)
				}
			} else {
				fields["quality"] = qVals[qi]
			}
		}
	case "section":
		if val, ok := TextInput(s, cfg, T(*cfg, "wiz_section_title"), T(*cfg, "wiz_section_prompt"), core.GetString(fields, "download_section"), T(*cfg, "footer_input")); ok {
			fields["download_section"] = strings.TrimSpace(val)
		}
	case "template":
		if val, ok := TextInput(s, cfg, T(*cfg, "wiz_template_title"), T(*cfg, "wiz_template_prompt"), core.GetString(fields, "output_template"), T(*cfg, "footer_input")); ok {
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
	case "force_overwrite":
		fields["force_overwrite"] = !core.GetBool(fields, "force_overwrite")

	case "video_preset":
		keys := core.OrderedVideoPresetKeys
		var pNames []string
		for _, k := range keys {
			pNames = append(pNames, PresetDisplayName(*cfg, core.VideoPresets[k]))
		}
		pNames = append(pNames, T(*cfg, "wiz_custom_ffmpeg"))
		pi := RunMenu(s, cfg, T(*cfg, "wiz_preset_title"), pNames, T(*cfg, "wiz_preset_hint"), T(*cfg, "footer_nav"))
		if pi >= 0 {
			if pi < len(keys) {
				fields["video_preset"] = keys[pi]
			} else {
				if flags, ok := TextInput(s, cfg, T(*cfg, "wiz_custom_ffmpeg_title"), T(*cfg, "wiz_custom_ffmpeg_prompt"), "-c:v libx264 -c:a aac", T(*cfg, "footer_input")); ok {
					fields["video_preset"] = "custom"
					fields["custom_flags"] = flags
				}
			}
		}
	case "vcodec":
		vLabels := []string{T(*cfg, "wiz_vc_auto"), T(*cfg, "wiz_vc_av1"), T(*cfg, "wiz_vc_vp9"), T(*cfg, "wiz_vc_h264")}
		vVals := []string{"auto", "av1", "vp9", "h264"}
		vi := RunMenu(s, cfg, T(*cfg, "wiz_vcodec_title"), vLabels, T(*cfg, "wiz_vcodec_hint"), T(*cfg, "footer_nav"))
		if vi >= 0 {
			fields["vcodec"] = vVals[vi]
		}
	case "fps_limit":
		fpsLabels := []string{T(*cfg, "wiz_fps_max"), T(*cfg, "wiz_fps_60"), T(*cfg, "wiz_fps_30")}
		fpsVals := []string{"", "60", "30"}
		fi := RunMenu(s, cfg, T(*cfg, "wiz_fps_title"), fpsLabels, T(*cfg, "wiz_fps_hint"), T(*cfg, "footer_nav"))
		if fi >= 0 {
			fields["fps_limit"] = fpsVals[fi]
		}
	case "audio_only":
		fields["audio_only"] = !core.GetBool(fields, "audio_only")
	case "audio_format":
		fmts := []string{"mp3", "flac", "wav", "m4a", "opus"}
		fi := RunMenu(s, cfg, T(*cfg, "wiz_audio_fmt_title"), fmts, T(*cfg, "wiz_audio_fmt_hint"), T(*cfg, "footer_nav"))
		if fi >= 0 {
			fields["audio_format"] = fmts[fi]
		}
	case "audio_quality":
		qLabels := []string{T(*cfg, "wiz_aq_best"), T(*cfg, "wiz_aq_high"), T(*cfg, "wiz_aq_medium"), T(*cfg, "wiz_aq_small")}
		qVals := []string{"0", "2", "5", "9"}
		qi := RunMenu(s, cfg, T(*cfg, "wiz_audio_q_title"), qLabels, T(*cfg, "wiz_audio_q_hint"), T(*cfg, "footer_nav"))
		if qi >= 0 {
			fields["audio_quality"] = qVals[qi]
		}

	case "subs_enabled":
		fields["subs_enabled"] = !core.GetBool(fields, "subs_enabled")
	case "sub_langs":
		if val, ok := TextInput(s, cfg, T(*cfg, "wiz_sub_langs_title"), T(*cfg, "wiz_sub_langs_prompt"), core.GetString(fields, "sub_langs"), T(*cfg, "footer_input")); ok {
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
		sbLabels := []string{T(*cfg, "wiz_sb_off"), T(*cfg, "wiz_sb_remove"), T(*cfg, "wiz_sb_mark")}
		sbVals := []string{"off", "remove", "mark"}
		sbi := RunMenu(s, cfg, T(*cfg, "wiz_sponsor_title"), sbLabels, T(*cfg, "wiz_sponsor_hint"), T(*cfg, "footer_nav"))
		if sbi >= 0 {
			fields["sponsorblock"] = sbVals[sbi]
		}
	case "write_extra":
		fields["write_extra"] = !core.GetBool(fields, "write_extra")

	case "concurrent_fragments":
		cLabels := []string{T(*cfg, "wiz_frags_2"), T(*cfg, "wiz_frags_4"), T(*cfg, "wiz_frags_8"), T(*cfg, "wiz_frags_16")}
		cVals := []string{"2", "4", "8", "16"}
		ci := RunMenu(s, cfg, T(*cfg, "wiz_frags_title"), cLabels, T(*cfg, "wiz_frags_hint"), T(*cfg, "footer_nav"))
		if ci >= 0 {
			fields["concurrent_fragments"] = cVals[ci]
		}
	case "retries":
		rLabels := []string{T(*cfg, "wiz_retry_5"), T(*cfg, "wiz_retry_10"), T(*cfg, "wiz_retry_20"), T(*cfg, "wiz_retry_inf")}
		rVals := []string{"5", "10", "20", "infinite"}
		ri := RunMenu(s, cfg, T(*cfg, "wiz_retries_title"), rLabels, T(*cfg, "wiz_retries_hint"), T(*cfg, "footer_nav"))
		if ri >= 0 {
			fields["retries"] = rVals[ri]
		}
	case "ratelimit":
		rtLabels := []string{T(*cfg, "wiz_unlimited"), "10 MB/s", "5 MB/s", "2 MB/s", T(*cfg, "wiz_rate_custom")}
		rtVals := []string{"", "10M", "5M", "2M", "custom"}
		rti := RunMenu(s, cfg, T(*cfg, "wiz_rate_title"), rtLabels, T(*cfg, "wiz_rate_hint"), T(*cfg, "footer_nav"))
		if rti >= 0 {
			if rtVals[rti] == "custom" {
				if val, ok := TextInput(s, cfg, T(*cfg, "wiz_rate_title"), T(*cfg, "wiz_rate_prompt"), "", T(*cfg, "footer_input")); ok {
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
		if val, ok := TextInputSecret(s, cfg, T(*cfg, "wiz_password_title"), T(*cfg, "wiz_password_prompt"), core.GetString(fields, "video_password"), T(*cfg, "footer_input")); ok {
			fields["video_password"] = strings.TrimSpace(val)
		}

	case "cookies_mode":
		mLabels := []string{T(*cfg, "wiz_ck_default"), T(*cfg, "wiz_ck_none"), T(*cfg, "wiz_ck_browser"), T(*cfg, "wiz_ck_file")}
		mVals := []string{"default", "none", "browser", "file"}
		mi := RunMenu(s, cfg, T(*cfg, "wiz_cookies_title"), mLabels, T(*cfg, "wiz_cookies_hint"), T(*cfg, "footer_nav"))
		if mi >= 0 {
			fields["cookies_mode"] = mVals[mi]
		}
	case "cookies_browser":
		var bLabels []string
		for _, b := range core.SupportedBrowsers {
			bLabels = append(bLabels, titleWord(b))
		}
		bi := RunMenu(s, cfg, T(*cfg, "wiz_browser_title"), bLabels, T(*cfg, "wiz_browser_hint"), T(*cfg, "footer_nav"))
		if bi >= 0 {
			fields["cookies_mode"] = "browser"
			fields["cookies_browser"] = core.SupportedBrowsers[bi]
		}
	case "cookies_file":
		if val, ok := TextInput(s, cfg, T(*cfg, "wiz_ck_file_title"), T(*cfg, "wiz_ck_file_prompt"), core.GetString(fields, "cookies_file"), T(*cfg, "footer_input")); ok {
			fields["cookies_mode"] = "file"
			fields["cookies_file"] = strings.TrimSpace(val)
		}
	case "proxy_mode":
		pLabels := []string{T(*cfg, "wiz_px_default"), T(*cfg, "wiz_px_direct"), T(*cfg, "wiz_px_custom")}
		pVals := []string{"default", "none", "custom"}
		pi := RunMenu(s, cfg, T(*cfg, "wiz_proxy_title"), pLabels, T(*cfg, "wiz_proxy_hint"), T(*cfg, "footer_nav"))
		if pi >= 0 {
			fields["proxy_mode"] = pVals[pi]
		}
	case "proxy_url":
		if val, ok := TextInput(s, cfg, T(*cfg, "wiz_proxy_url_title"), T(*cfg, "wiz_proxy_url_prompt"), core.GetString(fields, "proxy_url"), T(*cfg, "footer_input")); ok {
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
