package ui

import (
	"fmt"
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
		snapshot, exists := core.GlobalQueue.Snapshot(taskID)
		if !exists {
			return
		}
		completed := snapshot.Status == core.StatusDone || snapshot.Status == core.StatusFailed || snapshot.Status == core.StatusCancelled
		lines := []string{commandLine, ""}
		taskLogs := core.GlobalQueue.TaskLogs(taskID)
		lines = append(lines, taskLogs...)
		transferSpeed := ""
		for index := len(taskLogs) - 1; index >= 0; index-- {
			_, speed, hasProgress := core.ExtractProgress(taskLogs[index])
			if hasProgress && speed != "" {
				transferSpeed = speed
				break
			}
		}
		if completed {
			statusMessage := T(*cfg, "log_finished_ok")
			if snapshot.Status == core.StatusCancelled {
				statusMessage = "Cancelled"
			} else if snapshot.Status == core.StatusFailed {
				statusMessage = T(*cfg, "log_finished_err", snapshot.ExitCode)
			}
			lines = append(lines, statusMessage)
			if !completionAnnounced && cfg.NotifyBell {
				print("\a")
			}
			completionAnnounced = true
		}
		renderRunnerUI(s, cfg, lines, snapshot.Stage, opType, source, snapshot.Progress, transferSpeed, showRawLogs || completed)
		if completed {
			width, height := s.Size()
			DrawFooter(s, T(*cfg, "log_footer_done"), width, height)
			s.Show()
		}
		// Only the UI goroutine consumes keyboard events, including in nested dialogs.
		if s.HasPendingEvent() {
			event := s.PollEvent()
			if keyEvent, ok := event.(*tcell.EventKey); ok {
				if CheckPaletteHotkey(keyEvent) {
					GlobalPalette.Open(s, cfg)
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

func renderRunnerUI(s tcell.Screen, cfg *core.Config, lines []string, stage, opType, source string, pct float64, speed string, rawLogs bool) {
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
			DrawString(s, 2, 3+i, line, w-4, tcell.StyleDefault)
		}
	} else {
		stageStyle := tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)
		DrawString(s, 4, 3, "Active Stage: "+stage, w-8, stageStyle)
		DrawString(s, 4, 4, strings.Repeat("─", max(0, w-8)), w-8, tcell.StyleDefault.Dim(true))

		DrawString(s, 4, 6, "Operation : "+opType, w-8, tcell.StyleDefault)
		if source != "" {
			DrawString(s, 4, 7, "Source    : "+source, w-8, tcell.StyleDefault.Dim(true))
		}

		barStr := RenderProgressBar(pct, min(44, max(20, w-24)), cfg.ProgressStyle)
		progressText := fmt.Sprintf("Progress  : %s %5.1f%%", barStr, pct)
		if speed != "" {
			progressText += " (" + speed + ")"
		}
		DrawString(s, 4, 9, progressText, w-8, tcell.StyleDefault.Bold(true))
		DrawString(s, 4, 12, "[Press F10 for raw logs | 'q' to move to background queue]", w-8, tcell.StyleDefault.Dim(true))
	}

	DrawFooter(s, T(*cfg, "log_footer_running"), w, h)
	s.Show()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
