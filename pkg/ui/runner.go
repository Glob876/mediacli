package ui

import (
	"strings"
	"time"

	"mediacli/pkg/core"

	"github.com/gdamore/tcell/v2"
)

func RunWithLog(s tcell.Screen, cfg *core.Config, cmdList []string, opType, source, target string) {
	RunWithLogHook(s, cfg, cmdList, opType, source, target, nil)
}

func RunWithLogHook(s tcell.Screen, cfg *core.Config, cmdList []string, opType, source, target string, onComplete func(exitCode int) error) {
	s.HideCursor()
	task := core.GlobalQueue.EnqueueWithHook(cmdList, opType, source, target, onComplete)
	taskID := task.ID
	commandLine := "[cmd] " + strings.Join(core.MaskCmdForLog(cmdList), " ")
	showRawLogs := false
	completionAnnounced := false
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()

	for {
		lines, stage, progress, transferSpeed, completed, exists := loadRunnerView(cfg, taskID, commandLine)
		if !exists {
			return
		}
		if completed {
			if !completionAnnounced && cfg.NotifyBell {
				print("\a")
			}
			completionAnnounced = true
		}
		renderRunnerUI(s, cfg, lines, stage, opType, source, progress, transferSpeed, showRawLogs || completed, completed)
		// Only the UI goroutine consumes keyboard events, including in nested dialogs.
		if s.HasPendingEvent() {
			event := s.PollEvent()
			if _, ok := event.(*tcell.EventResize); ok {
				s.Sync()
				invalidateLogo()
				continue
			}
			if keyEvent, ok := event.(*tcell.EventKey); ok {
				if CheckPaletteHotkey(keyEvent) {
					GlobalPalette.OpenOver(s, cfg, func(screen tcell.Screen) {
						freshLines, freshStage, freshProgress, freshSpeed, freshDone, ok := loadRunnerView(cfg, taskID, commandLine)
						if !ok {
							return
						}
						drawRunnerFrame(screen, cfg, freshLines, freshStage, opType, source, freshProgress, freshSpeed, showRawLogs || freshDone, freshDone)
					})
					continue
				}
				if keyEvent.Key() == tcell.KeyF10 {
					showRawLogs = !showRawLogs
				}
				leaveScreen := keyEvent.Key() == tcell.KeyEscape || keyEvent.Rune() == 'q'
				if completed && (leaveScreen || keyEvent.Key() == tcell.KeyEnter) {
					return
				}
				if !completed && leaveScreen {
					ShowMessage(s, cfg, T(*cfg, "log_title"), []string{T(*cfg, "bg_transferred")}, T(*cfg, "footer_message"))
					return
				}
				if !completed && keyEvent.Key() == tcell.KeyCtrlC {
					core.GlobalQueue.CancelTask(taskID)
				}
			}
			continue
		}
		<-ticker.C
	}
}

func loadRunnerView(cfg *core.Config, taskID int, commandLine string) (lines []string, stage string, progress float64, speed string, completed bool, ok bool) {
	snapshot, exists := core.GlobalQueue.Snapshot(taskID)
	if !exists {
		return nil, "", 0, "", false, false
	}
	completed = snapshot.Status == core.StatusDone || snapshot.Status == core.StatusFailed || snapshot.Status == core.StatusCancelled
	lines = []string{commandLine, ""}
	taskLogs := core.GlobalQueue.TaskLogs(taskID)
	lines = append(lines, taskLogs...)
	for index := len(taskLogs) - 1; index >= 0; index-- {
		_, foundSpeed, hasProgress := core.ExtractProgress(taskLogs[index])
		if hasProgress && foundSpeed != "" {
			speed = foundSpeed
			break
		}
	}
	if completed {
		statusMessage := T(*cfg, "log_finished_ok")
		if snapshot.Status == core.StatusCancelled {
			statusMessage = T(*cfg, "log_cancelled")
		} else if snapshot.Status == core.StatusFailed {
			statusMessage = T(*cfg, "log_finished_err", snapshot.ExitCode)
		}
		lines = append(lines, statusMessage)
	}
	return lines, snapshot.Stage, snapshot.Progress, speed, completed, true
}

func renderRunnerUI(s tcell.Screen, cfg *core.Config, lines []string, stage, opType, source string, pct float64, speed string, rawLogs bool, completed bool) {
	drawRunnerFrame(s, cfg, lines, stage, opType, source, pct, speed, rawLogs, completed)
	s.Show()
}

func drawRunnerFrame(s tcell.Screen, cfg *core.Config, lines []string, stage, opType, source string, pct float64, speed string, rawLogs bool, completed bool) {
	w, h := s.Size()
	s.Clear()

	DrawHeader(s, T(*cfg, "log_title"), w, *cfg)

	if rawLogs {
		maxRows := max(0, h-4)
		visible := lines
		if len(visible) > maxRows {
			visible = visible[len(visible)-maxRows:]
		}
		for i, line := range visible {
			DrawString(s, 2, 3+i, line, w-4, GetBaseStyle(*cfg))
		}
	} else {
		DrawString(s, 4, 3, T(*cfg, "log_stage", stage), w-8, GetAccentStyle(*cfg))
		DrawString(s, 4, 4, strings.Repeat("─", max(0, w-8)), w-8, GetDimStyle(*cfg))

		DrawString(s, 4, 6, T(*cfg, "log_operation", opType), w-8, GetBaseStyle(*cfg))
		if source != "" {
			DrawString(s, 4, 7, T(*cfg, "log_source", source), w-8, GetDimStyle(*cfg))
		}

		barStr := RenderProgressBar(pct, min(44, max(20, w-24)), cfg.ProgressStyle)
		progressText := T(*cfg, "log_progress", barStr, pct)
		if speed != "" {
			progressText += T(*cfg, "log_speed", speed)
		}
		DrawString(s, 4, 9, progressText, w-8, GetBaseStyle(*cfg).Bold(true))
		DrawString(s, 4, 12, T(*cfg, "log_hint"), w-8, GetDimStyle(*cfg))
	}

	footerKey := "log_footer_running"
	if completed {
		footerKey = "log_footer_done"
	}
	DrawFooter(s, T(*cfg, footerKey), w, h, *cfg)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
