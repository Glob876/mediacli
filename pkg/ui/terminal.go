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
				fmt.Sprintf("  sub_langs    = %s", cfg.SubLangs),
				fmt.Sprintf("  language     = %s", cfg.Language),
				fmt.Sprintf("  theme        = %s", cfg.Theme),
				fmt.Sprintf("  video_preset = %s", cfg.VideoPreset),
				fmt.Sprintf("  proxy_mode   = %s", cfg.ProxyMode),
				fmt.Sprintf("  proxy_url    = %s", cfg.ProxyURL),
				fmt.Sprintf("  bg_queue_max = %d", cfg.BGQueueMax),
			)
		} else if args[0] == "get" && len(args) >= 2 {
			switch args[1] {
			case "download_dir":
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  download_dir = %s", cfg.DownloadDir))
			case "audio_format":
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  audio_format = %s", cfg.AudioFormat))
			case "sub_langs":
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  sub_langs = %s", cfg.SubLangs))
			case "language":
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  language = %s", cfg.Language))
			case "theme":
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  theme = %s", cfg.Theme))
			case "video_preset":
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  video_preset = %s", cfg.VideoPreset))
			case "proxy_mode":
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  proxy_mode = %s", cfg.ProxyMode))
			case "proxy_url":
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  proxy_url = %s", cfg.ProxyURL))
			case "bg_queue_max":
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  bg_queue_max = %d", cfg.BGQueueMax))
			default:
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [!] Unknown key: '%s'.", args[1]))
			}
		} else if args[0] == "set" && len(args) >= 3 {
			key, val := args[1], strings.Join(args[2:], " ")
			switch key {
			case "download_dir":
				cfg.DownloadDir = val
			case "theme":
				cfg.Theme = val
			case "language":
				cfg.Language = val
			case "video_preset":
				cfg.VideoPreset = val
			case "audio_format":
				cfg.AudioFormat = val
			case "sub_langs":
				cfg.SubLangs = val
			case "proxy_mode":
				cfg.ProxyMode = val
			case "proxy_url":
				cfg.ProxyURL = val
			case "bg_queue_max":
				var n int
				if _, err := fmt.Sscanf(val, "%d", &n); err != nil || n < 1 {
					term.HistoryLines = append(term.HistoryLines, "  [!] bg_queue_max must be a number >= 1.")
					break
				}
				cfg.BGQueueMax = n
				core.GlobalQueue.SyncMaxTasksFromConfig(*cfg)
			default:
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [!] Unknown key: '%s'.", key))
				break
			}
			_ = core.SaveConfig(*cfg)
			term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [✓] Set %s = %s", key, val))
		} else {
			term.HistoryLines = append(term.HistoryLines, "  Usage: config [list | get <k> | set <k> <v>]")
		}
	case "preset":
		if len(args) == 0 || args[0] == "list" {
			if len(cfg.DownloadPresets) == 0 {
				term.HistoryLines = append(term.HistoryLines, "  No saved presets.")
			}
			for _, p := range cfg.DownloadPresets {
				marker := ""
				if p.ID == cfg.DefaultDownloadPreset {
					marker = " [default]"
				}
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [%s] %s%s", p.ID, p.Name, marker))
			}
		} else if args[0] == "delete" && len(args) >= 2 {
			id := args[1]
			found := false
			kept := cfg.DownloadPresets[:0]
			for _, p := range cfg.DownloadPresets {
				if p.ID == id {
					found = true
					continue
				}
				kept = append(kept, p)
			}
			if found {
				cfg.DownloadPresets = append([]core.DownloadPreset{}, kept...)
				if cfg.DefaultDownloadPreset == id {
					cfg.DefaultDownloadPreset = ""
				}
				_ = core.SaveConfig(*cfg)
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [✓] Deleted preset '%s'.", id))
			} else {
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [!] Preset '%s' not found.", id))
			}
		} else {
			term.HistoryLines = append(term.HistoryLines, "  Usage: preset [list | delete <id>]")
		}
	case "cookies":
		if len(args) == 0 || args[0] == "list" {
			cookies, err := core.ParseCookiesFile(cfg.CookiesFile)
			if err != nil || len(cookies) == 0 {
				term.HistoryLines = append(term.HistoryLines, "  No cookies in "+cfg.CookiesFile)
			} else {
				n := 0
				for _, c := range cookies {
					if n >= 10 {
						term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  ... and %d more", len(cookies)-n))
						break
					}
					term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  %s: %s", c.Domain, c.Name))
					n++
				}
			}
		} else if args[0] == "add" && len(args) >= 4 {
			domain, name, value := args[1], args[2], strings.Join(args[3:], " ")
			if err := core.AppendCookie(cfg.CookiesFile, domain, name, value); err != nil {
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [!] Failed: %v", err))
			} else {
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [✓] Added cookie %s for %s.", name, domain))
			}
		} else {
			term.HistoryLines = append(term.HistoryLines, "  Usage: cookies [list | add <domain> <name> <value>]")
		}
	case "queue", "bg":
		if len(args) >= 2 && (args[0] == "cancel" || args[0] == "kill") {
			var id int
			if _, err := fmt.Sscanf(args[1], "%d", &id); err != nil {
				term.HistoryLines = append(term.HistoryLines, "  [!] Usage: queue cancel <id>")
				break
			}
			if core.GlobalQueue.CancelTask(id) {
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [✓] Cancelled task #%d.", id))
			} else {
				term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [!] Task #%d not found.", id))
			}
			break
		}
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
			tag := "required"
			if !d.Required {
				tag = "optional"
			}
			term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  - %-15s: %s [%s]", d.Name, status, tag))
		}
	case "dl":
		if len(args) > 0 {
			url := args[0]
			fields := cfg.PresetDefaults
			if fields == nil {
				fields = core.GetInitialPresetFields()
			}
			p := core.DownloadPreset{ID: "bg_dl", Name: "CLI DL", Fields: fields}
			cmdList := append([]string{"yt-dlp"}, core.BuildYtDlpArgs(p, *cfg, cfg.DownloadDir, false)...)
			cmdList = append(cmdList, url)
			task := core.GlobalQueue.Enqueue(cmdList, "Download Video", url, cfg.DownloadDir)
			term.HistoryLines = append(term.HistoryLines, fmt.Sprintf("  [✓] Enqueued Task #%d", task.ID))
		} else {
			term.HistoryLines = append(term.HistoryLines, "  Usage: dl <url>")
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