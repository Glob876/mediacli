package ui

import (
	"fmt"
	"io"
	"mediacli/pkg/core"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

// RunDownloadExternal выполняет yt-dlp (только merge) + отдельный ffmpeg транскод
// в ОДНОМ экране RunWithLog — логи [VideoConvertor]/[FFmpeg] идентичны встроенному --recode
func RunDownloadExternal(s tcell.Screen, cfg *core.Config, preset core.DownloadPreset, outDir string, url string) {
	s.HideCursor()

	presetID := core.GetString(preset.Fields, "video_preset")
	if presetID == "" {
		presetID = cfg.VideoPreset
	}
	ext, ffFlags, ok := core.GetExternalFFmpegPlan(presetID, preset.Fields)
	if !ok {
		// fallback на обычный путь если нет плана
		cmdList := append([]string{"yt-dlp"}, core.BuildYtDlpArgs(preset, *cfg, outDir, false)...)
		cmdList = append(cmdList, url)
		RunWithLog(s, cfg, cmdList, "Download Video", url, outDir)
		return
	}

	ytdlpCmdList := append([]string{"yt-dlp"}, core.BuildYtDlpArgs(preset, *cfg, outDir, false)...)
	ytdlpCmdList = append(ytdlpCmdList, url)

	startTime := time.Now()

	lines := []string{"[cmd] " + strings.Join(ytdlpCmdList, " "), ""}
	currentStage := "Initializing download..."
	var pct float64
	var speedStr string
	showRawLogs := false

	// общие каналы и тикер для обеих фаз
	eventChan := make(chan tcell.Event)
	go func() {
		for {
			eventChan <- s.PollEvent()
		}
	}()
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()

	// ---------- Фаза 1: yt-dlp ----------
	ytdlpCmd := exec.Command(ytdlpCmdList[0], ytdlpCmdList[1:]...)
	ytdlpOut, err := ytdlpCmd.StdoutPipe()
	if err != nil {
		ShowMessage(s, cfg, T(*cfg, "log_title"), []string{fmt.Sprintf("Failed to spawn yt-dlp: %v", err)}, T(*cfg, "footer_message"))
		return
	}
	ytdlpCmd.Stderr = ytdlpCmd.Stdout
	if err := ytdlpCmd.Start(); err != nil {
		ShowMessage(s, cfg, T(*cfg, "log_title"), []string{fmt.Sprintf("Start error: %v", err)}, T(*cfg, "footer_message"))
		return
	}

	ytdlpLogChan := make(chan string, 1024)
	ytdlpDone := make(chan struct{})
	go func() {
		defer close(ytdlpDone)
		buf := make([]byte, 4096)
		leftover := ""
		for {
			n, rerr := ytdlpOut.Read(buf)
			if n > 0 {
				chunk := leftover + string(buf[:n])
				chunk = strings.ReplaceAll(chunk, "\r", "\n")
				parts := strings.Split(chunk, "\n")
				leftover = parts[len(parts)-1]
				for _, p := range parts[:len(parts)-1] {
					p = strings.TrimSpace(p)
					if p != "" {
						ytdlpLogChan <- p
					}
				}
				if rerr != nil {
					leftover = strings.TrimSpace(leftover)
					if leftover != "" {
						ytdlpLogChan <- leftover
					}
					break
				}
			}
			if rerr != nil {
				if rerr != io.EOF && leftover != "" {
					leftover = strings.TrimSpace(leftover)
					if leftover != "" {
						ytdlpLogChan <- leftover
					}
				} else if rerr == io.EOF && leftover != "" {
					leftover = strings.TrimSpace(leftover)
					if leftover != "" {
						ytdlpLogChan <- leftover
					}
				}
				break
			}
		}
	}()

	ytdlpExit := 0
	ytdlpCompleted := false
	// крутим UI пока yt-dlp не завершится
	for !ytdlpCompleted {
		select {
		case ev := <-eventChan:
			if kEv, ok := ev.(*tcell.EventKey); ok {
				if CheckTerminalHotkey(kEv) {
					GlobalTerminal.Open(s, cfg)
					continue
				}
				if kEv.Key() == tcell.KeyF10 {
					showRawLogs = !showRawLogs
				} else if kEv.Key() == tcell.KeyEscape || kEv.Rune() == 'q' {
					// в external режиме не поддерживаем фон для первой фазы — просто Kill
					_ = ytdlpCmd.Process.Kill()
					ytdlpCompleted = true
					ytdlpExit = 1
				} else if kEv.Key() == tcell.KeyCtrlC {
					_ = ytdlpCmd.Process.Kill()
					ytdlpCompleted = true
					ytdlpExit = 1
				}
			}
		case text := <-ytdlpLogChan:
			lines = append(lines, text)
			currentStage = core.DetectStage(text, currentStage)
			if p, sp, ok := core.ExtractProgress(text); ok {
				pct = p
				if sp != "" {
					speedStr = sp
				}
			}
		case <-ytdlpDone:
			if err := ytdlpCmd.Wait(); err != nil {
				if exErr, ok := err.(*exec.ExitError); ok {
					ytdlpExit = exErr.ExitCode()
				} else {
					ytdlpExit = 1
				}
			}
			ytdlpCompleted = true
		case <-ticker.C:
			renderRunnerUI(s, cfg, lines, currentStage, "Download Video", url, pct, speedStr, showRawLogs)
		}
	}

	if ytdlpExit != 0 {
		statusMsg := T(*cfg, "log_finished_err", ytdlpExit)
		lines = append(lines, statusMsg)
		_ = core.AddHistoryEntry("Download Video", url, outDir, fmt.Sprintf("Failed yt-dlp (%d)", ytdlpExit))
		// показать финальный экран ошибки
		for {
			w, h := s.Size()
			s.Clear()
			DrawHeader(s, T(*cfg, "log_title"), w, *cfg)
			maxRows := h - 4
			visible := lines
			if len(visible) > maxRows {
				visible = visible[len(visible)-maxRows:]
			}
			for i, line := range visible {
				DrawString(s, 2, 3+i, line, w-4, tcell.StyleDefault)
			}
			DrawFooter(s, T(*cfg, "log_footer_done"), w, h)
			s.Show()
			ev := s.PollEvent()
			if kEv, ok := ev.(*tcell.EventKey); ok {
				if CheckTerminalHotkey(kEv) {
					GlobalTerminal.Open(s, cfg)
					continue
				}
				if kEv.Key() == tcell.KeyEnter || kEv.Key() == tcell.KeyEscape || kEv.Rune() == 'q' {
					break
				}
			}
		}
		return
	}

	// ---------- Поиск скачанного файла ----------
	// сначала пробуем достать из логов, затем fallback по времени
	candidate := core.ParseDestinationFromLogs(lines)
	if candidate != "" {
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(outDir, candidate)
		}
		if _, err := os.Stat(candidate); err != nil {
			candidate = ""
		}
	}
	if candidate == "" {
		candidate = core.FindNewestDownloadedFile(outDir, startTime)
	}
	if candidate == "" {
		lines = append(lines, "[Error] Could not locate downloaded file for external transcode")
		statusMsg := T(*cfg, "log_finished_err", 1)
		lines = append(lines, statusMsg)
		_ = core.AddHistoryEntry("Download Video", url, outDir, "Failed (locate)")
		for {
			w, h := s.Size()
			s.Clear()
			DrawHeader(s, T(*cfg, "log_title"), w, *cfg)
			maxRows := h - 4
			visible := lines
			if len(visible) > maxRows {
				visible = visible[len(visible)-maxRows:]
			}
			for i, line := range visible {
				DrawString(s, 2, 3+i, line, w-4, tcell.StyleDefault)
			}
			DrawFooter(s, T(*cfg, "log_footer_done"), w, h)
			s.Show()
			ev := s.PollEvent()
			if kEv, ok := ev.(*tcell.EventKey); ok {
				if kEv.Key() == tcell.KeyEnter || kEv.Key() == tcell.KeyEscape || kEv.Rune() == 'q' {
					break
				}
			}
		}
		return
	}

	// ---------- Подготовка ffmpeg ----------
	plan := core.PrepareFFmpegOutput(candidate, ext, "", *cfg)
	ffCmdList := []string{"ffmpeg", "-y", "-i", candidate}
	ffCmdList = append(ffCmdList, ffFlags...)
	ffCmdList = append(ffCmdList, plan.TempOutputPath)

	// синтетический лог для паритета с yt-dlp --recode (там [VideoConvertor] ... )
	synth := fmt.Sprintf("[VideoConvertor] Converting %s → %s via external FFmpeg", filepath.Base(candidate), filepath.Base(plan.FinalPath))
	lines = append(lines, "")
	lines = append(lines, synth)
	currentStage = core.DetectStage(synth, currentStage)
	// сбрасываем прогресс для фазы ffmpeg
	pct = 0
	speedStr = ""
	lines = append(lines, "[cmd] "+strings.Join(ffCmdList, " "))

	ffCmd := exec.Command(ffCmdList[0], ffCmdList[1:]...)
	ffOut, err := ffCmd.StdoutPipe()
	if err != nil {
		lines = append(lines, fmt.Sprintf("[Error] Failed to spawn ffmpeg: %v", err))
		statusMsg := T(*cfg, "log_finished_err", 1)
		lines = append(lines, statusMsg)
		_ = core.AddHistoryEntry("Download Video", url, plan.FinalPath, "Failed (ffmpeg spawn)")
		// показать ошибку
		for {
			w, h := s.Size()
			s.Clear()
			DrawHeader(s, T(*cfg, "log_title"), w, *cfg)
			maxRows := h - 4
			visible := lines
			if len(visible) > maxRows {
				visible = visible[len(visible)-maxRows:]
			}
			for i, line := range visible {
				DrawString(s, 2, 3+i, line, w-4, tcell.StyleDefault)
			}
			DrawFooter(s, T(*cfg, "log_footer_done"), w, h)
			s.Show()
			ev := s.PollEvent()
			if kEv, ok := ev.(*tcell.EventKey); ok {
				if kEv.Key() == tcell.KeyEnter || kEv.Key() == tcell.KeyEscape || kEv.Rune() == 'q' {
					break
				}
			}
		}
		return
	}
	ffCmd.Stderr = ffCmd.Stdout
	if err := ffCmd.Start(); err != nil {
		lines = append(lines, fmt.Sprintf("[Error] ffmpeg start: %v", err))
		statusMsg := T(*cfg, "log_finished_err", 1)
		lines = append(lines, statusMsg)
		_ = core.AddHistoryEntry("Download Video", url, plan.FinalPath, "Failed (ffmpeg start)")
		for {
			w, h := s.Size()
			s.Clear()
			DrawHeader(s, T(*cfg, "log_title"), w, *cfg)
			maxRows := h - 4
			visible := lines
			if len(visible) > maxRows {
				visible = visible[len(visible)-maxRows:]
			}
			for i, line := range visible {
				DrawString(s, 2, 3+i, line, w-4, tcell.StyleDefault)
			}
			DrawFooter(s, T(*cfg, "log_footer_done"), w, h)
			s.Show()
			ev := s.PollEvent()
			if kEv, ok := ev.(*tcell.EventKey); ok {
				if kEv.Key() == tcell.KeyEnter || kEv.Key() == tcell.KeyEscape || kEv.Rune() == 'q' {
					break
				}
			}
		}
		return
	}

	ffLogChan := make(chan string, 1024)
	ffDone := make(chan struct{})
	go func() {
		defer close(ffDone)
		buf := make([]byte, 4096)
		leftover := ""
		for {
			n, rerr := ffOut.Read(buf)
			if n > 0 {
				chunk := leftover + string(buf[:n])
				chunk = strings.ReplaceAll(chunk, "\r", "\n")
				parts := strings.Split(chunk, "\n")
				leftover = parts[len(parts)-1]
				for _, p := range parts[:len(parts)-1] {
					p = strings.TrimSpace(p)
					if p != "" {
						ffLogChan <- p
					}
				}
				if rerr != nil {
					leftover = strings.TrimSpace(leftover)
					if leftover != "" {
						ffLogChan <- leftover
					}
					break
				}
			}
			if rerr != nil {
				if rerr != io.EOF && leftover != "" {
					leftover = strings.TrimSpace(leftover)
					if leftover != "" {
						ffLogChan <- leftover
					}
				} else if rerr == io.EOF && leftover != "" {
					leftover = strings.TrimSpace(leftover)
					if leftover != "" {
						ffLogChan <- leftover
					}
				}
				break
			}
		}
	}()

	ffExit := 0
	ffCompleted := false
	for !ffCompleted {
		select {
		case ev := <-eventChan:
			if kEv, ok := ev.(*tcell.EventKey); ok {
				if CheckTerminalHotkey(kEv) {
					GlobalTerminal.Open(s, cfg)
					continue
				}
				if kEv.Key() == tcell.KeyF10 {
					showRawLogs = !showRawLogs
				} else if kEv.Key() == tcell.KeyEscape || kEv.Rune() == 'q' {
					_ = ffCmd.Process.Kill()
					ffCompleted = true
					ffExit = 1
				} else if kEv.Key() == tcell.KeyCtrlC {
					_ = ffCmd.Process.Kill()
					ffCompleted = true
					ffExit = 1
				}
			}
		case text := <-ffLogChan:
			lines = append(lines, text)
			currentStage = core.DetectStage(text, currentStage)
			if p, sp, ok := core.ExtractProgress(text); ok {
				pct = p
				if sp != "" {
					speedStr = sp
				}
			}
		case <-ffDone:
			if err := ffCmd.Wait(); err != nil {
				if exErr, ok := err.(*exec.ExitError); ok {
					ffExit = exErr.ExitCode()
				} else {
					ffExit = 1
				}
			}
			ffCompleted = true
		case <-ticker.C:
			renderRunnerUI(s, cfg, lines, currentStage, "Download Video", url, pct, speedStr, showRawLogs)
		}
	}

	if plan.OnComplete != nil {
		plan.OnComplete(ffExit)
	} else if ffExit == 0 {
		// если план без OnComplete (обычный случай без OverwriteOriginal), но candidate != finalPath, нужно удалить исходник?
		// Для external ожидаем удаление исходного merge-файла после успешного ffmpeg (как yt-dlp --recode удаляет .temp) ?
		// Оставляем исходник если ext совпадает, иначе удаляем временный исходник чтобы не дублировать
		if candidate != plan.FinalPath && candidate != plan.TempOutputPath {
			// если не перезаписываем и файл уже есть с другим расширением — исходный оставляем или удаляем по желанию
			// Сейчас удаляем исходный merge если успешно перекодировали (паритет с yt-dlp --recode который не оставляет исходный)
			// Чтобы не терять данные, оставляем только если OverwriteOriginal==false и UseFFmpegSuffix==false? Но для external we want parity: yt-dlp рекод удаляет промежуточный
			// Поэтому удаляем candidate если он не финальный
			if _, err := os.Stat(plan.FinalPath); err == nil {
				// если финальный уже существует и это не тот же файл — можно удалить исходник
				// но только если PrepareFFmpegOutput не требовал temp
				if plan.TempOutputPath == plan.FinalPath {
					// обычный случай: ffmpeg писал сразу в финальный — удаляем исходный если это не тот же путь
					if !strings.EqualFold(filepath.Clean(candidate), filepath.Clean(plan.FinalPath)) {
						_ = os.Remove(candidate)
					}
				}
			}
		}
	}

	statusMsg := T(*cfg, "log_finished_ok")
	if ffExit != 0 {
		statusMsg = T(*cfg, "log_finished_err", ffExit)
	}
	lines = append(lines, statusMsg)
	if cfg.NotifyBell {
		print("\a")
	}
	finalTarget := plan.FinalPath
	if ffExit != 0 {
		finalTarget = candidate
	}
	_ = core.AddHistoryEntry("Download Video", url, finalTarget, func() string {
		if ffExit == 0 {
			return "Success"
		}
		return fmt.Sprintf("Failed (%d)", ffExit)
	}())

	for {
		w, h := s.Size()
		s.Clear()
		DrawHeader(s, T(*cfg, "log_title"), w, *cfg)
		maxRows := h - 4
		visible := lines
		if len(visible) > maxRows {
			visible = visible[len(visible)-maxRows:]
		}
		for i, line := range visible {
			DrawString(s, 2, 3+i, line, w-4, tcell.StyleDefault)
		}
		DrawFooter(s, T(*cfg, "log_footer_done"), w, h)
		s.Show()
		ev := s.PollEvent()
		if kEv, ok := ev.(*tcell.EventKey); ok {
			if CheckTerminalHotkey(kEv) {
				GlobalTerminal.Open(s, cfg)
				continue
			}
			if kEv.Key() == tcell.KeyEnter || kEv.Key() == tcell.KeyEscape || kEv.Rune() == 'q' {
				break
			}
		}
	}
}
