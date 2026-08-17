package ui

import (
	"fmt"
	"mediacli/pkg/core"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type TerminalOverlay struct {
	HistoryLines []string
	CmdHistory   []string
	CmdIdx       int
	CurrentBuf   []rune
}

var GlobalTerminal = &TerminalOverlay{
	HistoryLines: []string{
		"MediaCLI Interactive Console. Type 'help' or '?' for commands.",
		"Press F12, Alt+Shift+P, or Esc to minimize. Type 'exit' to close.",
		"",
	},
}

func (term *TerminalOverlay) Open(s tcell.Screen, cfg *core.Config) {
	for {
		w, h := s.Size()
		s.Clear()

		boxStyle := tcell.StyleDefault.Dim(true)
		titleStyle := tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)
		promptStyle := tcell.StyleDefault.Foreground(tcell.ColorCadetBlue).Bold(true)

		for y := 0; y < h; y++ {
			s.SetContent(0, y, '│', nil, boxStyle)
			s.SetContent(w-1, y, '│', nil, boxStyle)
		}
		DrawString(s, 0, 0, "┌"+strings.Repeat("─", w-2)+"┐", w, boxStyle)
		DrawString(s, 0, h-1, "└"+strings.Repeat("─", w-2)+"┘", w, boxStyle)

		title := " [ MediaCLI Terminal Overlay — F12 / Esc to close ] "
		DrawString(s, max(2, (w-len(title))/2), 0, title, w-4, titleStyle)

		maxLogs := h - 4
		visible := term.HistoryLines
		if len(visible) > maxLogs {
			visible = visible[len(visible)-maxLogs:]
		}
		for i, line := range visible {
			DrawString(s, 2, 1+i, line, w-4, tcell.StyleDefault)
		}

		prompt := "mediacli> "
		DrawString(s, 2, h-2, strings.Repeat("─", w-4), w-4, boxStyle)
		DrawString(s, 2, h-2, prompt, len(prompt), promptStyle)
		inputStr := string(term.CurrentBuf)
		DrawString(s, 2+len(prompt), h-2, inputStr, w-len(prompt)-4, tcell.StyleDefault)

		s.ShowCursor(2+len(prompt)+len(term.CurrentBuf), h-2)
		s.Show()

		ev := s.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyF12 || ev.Key() == tcell.KeyEscape {
				s.HideCursor()
				return
			}
			if ev.Key() == tcell.KeyEnter {
				cmd := strings.TrimSpace(string(term.CurrentBuf))
				term.CurrentBuf = []rune{}
				if cmd == "" {
					continue
				}
				term.CmdHistory = append(term.CmdHistory, cmd)
				term.CmdIdx = len(term.CmdHistory)
				term.HistoryLines = append(term.HistoryLines, "mediacli> "+cmd)

				lower := strings.ToLower(cmd)
				if lower == "exit" || lower == "quit" {
					s.HideCursor()
					return
				} else if lower == "clear" || lower == "cls" {
					term.HistoryLines = []string{}
				} else {
					term.dispatchCommand(cmd, cfg)
				}
			} else if ev.Key() == tcell.KeyUp {
				if len(term.CmdHistory) > 0 && term.CmdIdx > 0 {
					term.CmdIdx--
					term.CurrentBuf = []rune(term.CmdHistory[term.CmdIdx])
				}
			} else if ev.Key() == tcell.KeyDown {
				if term.CmdIdx < len(term.CmdHistory)-1 {
					term.CmdIdx++
					term.CurrentBuf = []rune(term.CmdHistory[term.CmdIdx])
				} else {
					term.CmdIdx = len(term.CmdHistory)
					term.CurrentBuf = []rune{}
				}
			} else if ev.Key() == tcell.KeyBackspace || ev.Key() == tcell.KeyBackspace2 {
				if len(term.CurrentBuf) > 0 {
					term.CurrentBuf = term.CurrentBuf[:len(term.CurrentBuf)-1]
				}
			} else if ev.Rune() >= 32 {
				term.CurrentBuf = append(term.CurrentBuf, ev.Rune())
			}
		}
	}
}

func (term *TerminalOverlay) dispatchCommand(line string, cfg *core.Config) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return
	}
	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "help", "?":
		term.HistoryLines = append(term.HistoryLines,
			"Available Terminal Commands:",
			"  config list / get <k> / set <k> <v> - Inspect and modify settings",
			"  preset list / delete <id>           - Manage download presets",
			"  queue [list|cancel <id>]            - View/cancel background tasks",
			"  history [list|clear]               - View or wipe operation history",
			"  cookies [list|add <dom> <k> <v>]   - Manage cookies.txt file",
			"  theme <name>                       - Switch color theme directly",
			"  doctor                             - Diagnose dependencies & tools",
			"  dl <url>                           - Queue download to background",
			"  clear / cls                        - Clear console buffer",
			"  exit / quit                        - Close terminal overlay",
		)
	case "config":
		if len(args) == 0 || args[0] == "list" {
			term.HistoryLines = append(term.HistoryLines,
				fmt.Sprintf("  download_dir = %s", cfg.DownloadDir),
				fmt.Sprintf("  audio_format = %s", cfg.AudioFormat),
				fmt.Sprintf("  language     = %s", cfg.Language),
				fmt.Sprintf("  theme        = %s", cfg.Theme),
				fmt.Sprintf("  video_preset = %s", cfg.VideoPreset),
			)
		} else if args[0] == "set" && len(args) >= 3 {
			key, val := args[1], strings.Join(args[2:], " ")
			switch key {
			case "download_dir":
				cfg.DownloadDir = val
			case "theme":
				cfg.Theme = val
			case "language":
				cfg.Language = val
			}
			_ = core.SaveConfig(*cfg)
			term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [✓] Set %s = %s", key, val))
		}
	case "queue", "bg":
		tasks := core.GlobalQueue.GetTasks()
		if len(tasks) == 0 {
			term.HistoryLines = append(term.HistoryLines, "  Background queue is empty.")
		}
		for _, t := range tasks {
			term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [%d] %s | %s | %s (%.1f%%)", t.ID, t.Title, t.Status, t.Stage, t.Progress))
		}
	case "history":
		if len(args) > 0 && args[0] == "clear" {
			_ = core.ClearHistory()
			term.HistoryLines = append(term.HistoryLines, "  [✓] History cleared.")
		} else {
			entries := core.GetHistory()
			for i, e := range entries {
				if i >= 8 {
					break
				}
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [%s] %s: %s (%s)", e.Time, e.Type, e.Target, e.Status))
			}
		}
	case "theme":
		if len(args) > 0 && Themes[args[0]].ID != "" {
			cfg.Theme = args[0]
			_ = core.SaveConfig(*cfg)
			term.HistoryLines = append(term.HistoryLines, "  [✓] Theme switched to "+args[0])
		} else {
			keys := make([]string, 0, len(Themes))
			for k := range Themes {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			term.HistoryLines = append(term.HistoryLines, "  Themes: "+strings.Join(keys, ", "))
		}
	case "doctor":
		deps := core.CheckDependencies()
		for _, d := range deps {
			status := "MISSING"
			if d.Available {
				status = "FOUND (" + d.Path + ")"
			}
			term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  - %-15s: %s", d.Name, status))
		}
	case "dl":
		if len(args) > 0 {
			url := args[0]
			p := core.DownloadPreset{ID: "bg_dl", Name: "CLI DL", Fields: cfg.PresetDefaults}
			cmdList := append([]string{"yt-dlp"}, core.BuildYtDlpArgs(p, *cfg, cfg.DownloadDir, false)...)
			cmdList = append(cmdList, url)
			task := core.GlobalQueue.Enqueue(cmdList, "Download Video", url, cfg.DownloadDir)
			term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [✓] Enqueued Task #%d", task.ID))
		}
	default:
		term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [!] Unknown command: '%s'. Type 'help'.", cmd))
	}
}

func CheckTerminalHotkey(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyF12 {
		return true
	}
	if ev.Modifiers()&(tcell.ModAlt|tcell.ModShift) != 0 && (ev.Rune() == 'P' || ev.Rune() == 'p') {
		return true
	}
	return false
}