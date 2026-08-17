package ui

import (
	"mediacli/pkg/core"
	"strings"

	"github.com/gdamore/tcell/v2"
)

func RunMenu(s tcell.Screen, cfg *core.Config, title string, items []string, subtitle string, footer string) int {
	idx := 0
	s.HideCursor()

	for {
		w, h := s.Size()
		s.Clear()

		DrawHeader(s, title, w, *cfg)
		row := 3
		if subtitle != "" {
			for _, line := range strings.Split(subtitle, "\n") {
				DrawString(s, 4, row, line, w-8, GetDimStyle(*cfg))
				row++
			}
			row++
		}

		hlStyle := GetHighlightStyle(*cfg)
		baseStyle := GetBaseStyle(*cfg)
		for i, item := range items {
			y := row + i
			if y >= h-3 {
				break
			}
			if i == idx {
				DrawString(s, 4, y, "> "+item, w-8, hlStyle)
			} else {
				DrawString(s, 4, y, "  "+item, w-8, baseStyle)
			}
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
			switch ev.Key() {
			case tcell.KeyUp:
				if idx > 0 {
					idx--
				} else {
					idx = len(items) - 1
				}
			case tcell.KeyDown:
				if idx < len(items)-1 {
					idx++
				} else {
					idx = 0
				}
			case tcell.KeyEnter:
				return idx
			case tcell.KeyEscape:
				return -1
			case tcell.KeyRune:
				if ev.Rune() == 'k' {
					if idx > 0 {
						idx--
					}
				} else if ev.Rune() == 'j' {
					if idx < len(items)-1 {
						idx++
					}
				} else if ev.Rune() == 'q' {
					return -1
				}
			}
		}
	}
}

func TextInput(s tcell.Screen, cfg *core.Config, title, prompt, defaultVal, footer string) (string, bool) {
	buf := []rune(defaultVal)
	cursorPos := len(buf)

	for {
		w, h := s.Size()
		s.Clear()

		DrawHeader(s, title, w, *cfg)
		DrawString(s, 4, 3, prompt, w-8, GetBaseStyle(*cfg).Bold(true))
		DrawString(s, 4, 5, "> "+string(buf), w-8, GetBaseStyle(*cfg))

		s.ShowCursor(6+cursorPos, 5)
		DrawFooter(s, footer, w, h)
		s.Show()

		ev := s.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if CheckTerminalHotkey(ev) {
				GlobalTerminal.Open(s, cfg)
				continue
			}
			switch ev.Key() {
			case tcell.KeyEnter:
				s.HideCursor()
				return string(buf), true
			case tcell.KeyEscape:
				s.HideCursor()
				return "", false
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if cursorPos > 0 {
					buf = append(buf[:cursorPos-1], buf[cursorPos:]...)
					cursorPos--
				}
			case tcell.KeyLeft:
				if cursorPos > 0 {
					cursorPos--
				}
			case tcell.KeyRight:
				if cursorPos < len(buf) {
					cursorPos++
				}
			case tcell.KeyRune:
				if ev.Rune() >= 32 {
					buf = append(buf[:cursorPos], append([]rune{ev.Rune()}, buf[cursorPos:]...)...)
					cursorPos++
				}
			}
		}
	}
}

func ShowMessage(s tcell.Screen, cfg *core.Config, title string, lines []string, footer string) {
	s.HideCursor()
	for {
		w, h := s.Size()
		s.Clear()

		DrawHeader(s, title, w, *cfg)
		for i, line := range lines {
			if 3+i >= h-3 {
				break
			}
			DrawString(s, 4, 3+i, line, w-8, GetBaseStyle(*cfg))
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
			if ev.Key() == tcell.KeyEnter || ev.Key() == tcell.KeyEscape || ev.Rune() == 'q' {
				return
			}
		}
	}
}