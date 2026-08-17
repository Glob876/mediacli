package ui

import (
	"bufio"
	"fmt"
	"mediacli/pkg/core"
	"os/exec"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

func RunWithLog(s tcell.Screen, cfg *core.Config, cmdList []string, opType, source, target string) {
	s.HideCursor()

	cmd := exec.Command(cmdList[0], cmdList[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ShowMessage(s, cfg, T(*cfg, "log_title"), []string{fmt.Sprintf("Failed to spawn process: %v", err)}, T(*cfg, "footer_message"))
		return
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		ShowMessage(s, cfg, T(*cfg, "log_title"), []string{fmt.Sprintf("Start error: %v", err)}, T(*cfg, "footer_message"))
		return
	}

	logChan := make(chan string, 200)
	doneChan := make(chan struct{})

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			logChan <- scanner.Text()
		}
		close(doneChan)
	}()

	lines := []string{"[cmd] " + strings.Join(cmdList, " "), ""}
	currentStage := "Initializing process..."
	var pct float64
	var speedStr string
	showRawLogs := false
	eventChan := make(chan tcell.Event)

	go func() {
		for {
			eventChan <- s.PollEvent()
		}
	}()

	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()

	completed := false
	exitCode := 0

	for !completed {
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
					core.GlobalQueue.AdoptRunning(cmd, cmdList, opType, source, target, lines)
					ShowMessage(s, cfg, T(*cfg, "log_title"), []string{T(*cfg, "bg_transferred")}, T(*cfg, "footer_message"))
					return
				} else if kEv.Key() == tcell.KeyCtrlC {
					_ = cmd.Process.Kill()
					completed = true
				}
			}
		case text := <-logChan:
			lines = append(lines, text)
			currentStage = core.DetectStage(text, currentStage)
			if p, sp, ok := core.ExtractProgress(text); ok {
				pct = p
				if sp != "" {
					speedStr = sp
				}
			}
		case <-doneChan:
			if err := cmd.Wait(); err != nil {
				if exErr, ok := err.(*exec.ExitError); ok {
					exitCode = exErr.ExitCode()
				} else {
					exitCode = 1
				}
			}
			completed = true
		case <-ticker.C:
			renderRunnerUI(s, cfg, lines, currentStage, opType, source, pct, speedStr, showRawLogs)
		}
	}

	statusMsg := T(*cfg, "log_finished_ok")
	if exitCode != 0 {
		statusMsg = T(*cfg, "log_finished_err", exitCode)
	}
	lines = append(lines, statusMsg)

	if cfg.NotifyBell {
		print("\a")
	}

	_ = core.AddHistoryEntry(opType, source, target, func() string {
		if exitCode == 0 {
			return "Success"
		}
		return fmt.Sprintf("Failed (%d)", exitCode)
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

func renderRunnerUI(s tcell.Screen, cfg *core.Config, lines []string, stage, opType, source string, pct float64, speed string, rawLogs bool) {
	w, h := s.Size()
	s.Clear()

	DrawHeader(s, T(*cfg, "log_title"), w, *cfg)

	if rawLogs {
		maxRows := h - 4
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
		DrawString(s, 4, 4, strings.Repeat("─", w-8), w-8, tcell.StyleDefault.Dim(true))

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