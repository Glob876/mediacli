package ui

import (
	"fmt"
	"mediacli/pkg/core"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

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

	ratioIdx := 1
	if srcW < srcH {
		ratioIdx = 0
	}
	customW, customH := 16, 9

	modeIdx := 0
	modes := []string{
		T(*cfg, "aspect_mode_crop"),
		T(*cfg, "aspect_mode_letterbox"),
		T(*cfg, "aspect_mode_blur"),
		T(*cfg, "aspect_mode_stretch"),
	}

	resIdx := 0
	resLabels := []string{"Auto (Max Quality)", "1080p (Full HD)", "720p (HD)", "4K (2160p UHD)"}

	codecIdx := 0
	codecLabels := []string{"H.264 (Universal MP4)", "H.265 / HEVC (High Efficiency)", "Apple ProRes 422 (.mov)", "DaVinci Resolve DNxHR HQ (.mov)"}

	curSel := 0
	totalOptions := 6

	for {
		w, h := s.Size()
		s.Clear()

		DrawHeader(s, T(*cfg, "aspect_title"), w, *cfg)

		leftW := min(46, w/2)
		divX := leftW + 2

		for y := 2; y < h-2; y++ {
			s.SetContent(divX, y, '│', nil, GetDimStyle(*cfg))
		}

		activePreset := AspectRatioPresets[ratioIdx]
		rW, rH := activePreset.W, activePreset.H
		if activePreset.ID == "custom" {
			rW, rH = customW, customH
		}
		ratioVal := float64(rW) / float64(rH)

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

			if i >= 4 {
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

		rightX := divX + 3
		rightW := w - rightX - 2
		rightH := h - 6
		drawAspectRatioVisualizer(s, *cfg, rightX, 3, rightW, rightH, srcW, srcH, rW, rH, ratioVal, modeIdx, resIdx, activePreset.ID)

		DrawFooter(s, T(*cfg, "aspect_footer_nav"), w, h)
		s.Show()

		triggerAction := func() bool {
			switch curSel {
			case 0:
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
			case 1:
				mi := RunMenu(s, cfg, T(*cfg, "aspect_mode_opt"), modes, "Choose how frame adapts to new ratio:", T(*cfg, "footer_nav"))
				if mi >= 0 {
					modeIdx = mi
				}
			case 2:
				ri := RunMenu(s, cfg, T(*cfg, "aspect_res_opt"), resLabels, "Select Output Resolution:", T(*cfg, "footer_nav"))
				if ri >= 0 {
					resIdx = ri
				}
			case 3:
				ci := RunMenu(s, cfg, T(*cfg, "aspect_codec_opt"), codecLabels, "Select Video Encoding Codec:", T(*cfg, "footer_nav"))
				if ci >= 0 {
					codecIdx = ci
				}
			case 4:
				outW, outH := calcTargetDimensions(srcW, srcH, rW, rH, resIdx)
				cmdList, plan := buildAspectRatioFFmpegArgs(p, outW, outH, rW, rH, modeIdx, codecIdx, *cfg)
				RunWithLogHook(s, cfg, cmdList, "Aspect Ratio Crop", filepath.Base(p), plan.TargetDisplay, plan.OnComplete)
				return true
			case 5:
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

func drawAspectRatioVisualizer(s tcell.Screen, cfg core.Config, startX, startY, availW, availH int, srcW, srcH, rW, rH int, targetRatio float64, modeIdx, resIdx int, presetID string) {
	if availW < 20 || availH < 10 {
		return
	}

	titleStyle := GetAccentStyle(cfg)
	DrawString(s, startX, startY, "► "+T(cfg, "aspect_preview_title"), availW, titleStyle)
	DrawString(s, startX, startY+1, strings.Repeat("─", availW), availW, GetDimStyle(cfg))

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

	srcRatio := float64(srcW) / float64(srcH)
	innerW := boxW - 2
	innerH := boxH - 2

	switch modeIdx {
	case 1: // Letterbox
		if srcRatio > targetRatio {
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
		} else if srcRatio < targetRatio {
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

	case 2: // Blur BG
		if srcRatio > targetRatio {
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
		} else {
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

	case 0: // Crop to Fill
		for y := 1; y < boxH-1; y++ {
			for x := 1; x < boxW-1; x++ {
				s.SetContent(boxX+x, boxY+y, '█', nil, innerStyle)
			}
		}
		if srcRatio > targetRatio && boxW >= 10 {
			DrawString(s, boxX+1, boxY+boxH/2, "✂", 2, cropCutStyle)
			DrawString(s, boxX+boxW-2, boxY+boxH/2, "✂", 2, cropCutStyle)
		} else if srcRatio < targetRatio && boxH >= 5 {
			DrawString(s, boxX+boxW/2, boxY+1, "✂", 2, cropCutStyle)
			DrawString(s, boxX+boxW/2, boxY+boxH-2, "✂", 2, cropCutStyle)
		}

	case 3: // Stretch
		for y := 1; y < boxH-1; y++ {
			for x := 1; x < boxW-1; x++ {
				s.SetContent(boxX+x, boxY+y, '≈', nil, innerStyle)
			}
		}
	}

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
	case 1:
		baseH = 1080
	case 2:
		baseH = 720
	case 3:
		baseH = 2160
	case 0:
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
	if rW >= rH {
		outH = baseH
		outW = roundEven(float64(outH) * ratio)
	} else {
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
	case 0:
		codecFlags = []string{"-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "medium", "-crf", "20", "-c:a", "aac", "-b:a", "192k"}
		ext = ".mp4"
	case 1:
		codecFlags = []string{"-c:v", "libx265", "-pix_fmt", "yuv420p", "-preset", "medium", "-crf", "23", "-c:a", "aac", "-b:a", "192k"}
		ext = ".mp4"
	case 2:
		codecFlags = []string{"-c:v", "prores_ks", "-profile:v", "3", "-c:a", "pcm_s16le"}
		ext = ".mov"
	case 3:
		codecFlags = []string{"-c:v", "dnxhd", "-profile:v", "dnxhr_hq", "-c:a", "pcm_s16le"}
		ext = ".mov"
	}

	var filterArgs []string
	modeSuffix := "_crop"

	switch modeIdx {
	case 0:
		modeSuffix = fmt.Sprintf("_%dx%d_crop", rW, rH)
		filterArgs = []string{"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,setsar=1", outW, outH, outW, outH)}
	case 1:
		modeSuffix = fmt.Sprintf("_%dx%d_letterbox", rW, rH)
		filterArgs = []string{"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black,setsar=1", outW, outH, outW, outH)}
	case 2:
		modeSuffix = fmt.Sprintf("_%dx%d_blur", rW, rH)
		filterComplex := fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,boxblur=25:5[bg];[0:v]scale=%d:%d:force_original_aspect_ratio=decrease[fg];[bg][fg]overlay=(W-w)/2:(H-h)/2,setsar=1", outW, outH, outW, outH, outW, outH)
		filterArgs = []string{"-filter_complex", filterComplex}
	case 3:
		modeSuffix = fmt.Sprintf("_%dx%d_stretch", rW, rH)
		filterArgs = []string{"-vf", fmt.Sprintf("scale=%d:%d,setsar=1", outW, outH)}
	}

	plan := core.PrepareFFmpegOutput(inputPath, ext, modeSuffix, cfg)

	cmdList := append([]string{"ffmpeg", "-y", "-i", inputPath}, filterArgs...)
	cmdList = append(cmdList, codecFlags...)
	cmdList = append(cmdList, plan.TempOutputPath)

	return cmdList, plan
}