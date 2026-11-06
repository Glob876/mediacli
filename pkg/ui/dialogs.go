package ui

import (
	"strings"
	"unicode"

	"mediacli/pkg/core"

	"github.com/gdamore/tcell/v2"
)

func RunMenu(s tcell.Screen, cfg *core.Config, title string, items []string, subtitle string, footer string) int {
	if len(items) == 0 {
		return -1
	}
	idx := 0
	firstVisible := 0
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

		if row >= h-3 {
			row = max(0, h-4)
		}
		visibleRows := max(1, h-row-3)
		firstVisible = windowStart(idx, firstVisible, visibleRows)
		hlStyle := GetHighlightStyle(*cfg)
		baseStyle := GetBaseStyle(*cfg)
		hits := make([]menuHit, 0, visibleRows)
		for i := firstVisible; i < len(items); i++ {
			item := items[i]
			y := row + i - firstVisible
			if y >= h-3 {
				break
			}
			label := "  " + item
			style := baseStyle
			if i == idx {
				label = "> " + item
				style = hlStyle
			}
			DrawSpan(s, 4, y, max(0, w-8), label, style)
			hits = append(hits, menuHit{minX: 4, maxX: max(4, w-4), y: y, index: i})
		}

		DrawFooter(s, footer, w, h, *cfg)
		s.Show()

		ev, resized := pollUI(s)
		if resized {
			continue
		}
		switch ev := ev.(type) {
		case *tcell.EventMouse:
			if ev.Buttons()&tcell.WheelUp != 0 {
				idx = wrapIndex(idx, -1, len(items))
			} else if ev.Buttons()&tcell.WheelDown != 0 {
				idx = wrapIndex(idx, 1, len(items))
			} else if ev.Buttons()&tcell.Button1 != 0 {
				x, y := ev.Position()
				if hit, ok := hitIndex(hits, x, y); ok {
					return hit
				}
			}
		case *tcell.EventKey:
			if CheckPaletteHotkey(ev) {
				GlobalPalette.Open(s, cfg)
				continue
			}
			switch ev.Key() {
			case tcell.KeyUp:
				idx = wrapIndex(idx, -1, len(items))
			case tcell.KeyDown:
				idx = wrapIndex(idx, 1, len(items))
			case tcell.KeyEnter:
				return idx
			case tcell.KeyEscape:
				return -1
			case tcell.KeyRune:
				switch ev.Rune() {
				case 'k':
					idx = wrapIndex(idx, -1, len(items))
				case 'j':
					idx = wrapIndex(idx, 1, len(items))
				case 'q':
					return -1
				}
			}
		}
	}
}

func TextInput(s tcell.Screen, cfg *core.Config, title, prompt, defaultVal, footer string) (string, bool) {
	return editText(s, cfg, title, prompt, defaultVal, footer, false)
}

func TextInputSecret(s tcell.Screen, cfg *core.Config, title, prompt, defaultVal, footer string) (string, bool) {
	return editText(s, cfg, title, prompt, defaultVal, footer, true)
}

func editText(s tcell.Screen, cfg *core.Config, title, prompt, defaultVal, footer string, secret bool) (string, bool) {
	buf := []rune(defaultVal)
	cursorPos := len(buf)
	pasting := false

	for {
		w, h := s.Size()
		s.Clear()

		DrawHeader(s, title, w, *cfg)
		DrawString(s, 4, 3, prompt, max(0, w-8), GetBaseStyle(*cfg).Bold(true))
		fieldWidth := max(1, w-8)
		textWidth := max(1, fieldWidth-2)
		start := inputWindowStart(buf, cursorPos, textWidth, secret)
		visible := buf[start:]
		shown := string(visible)
		if secret {
			shown = strings.Repeat("•", len(visible))
		}
		DrawSpan(s, 4, 5, fieldWidth, "> "+shown, GetBaseStyle(*cfg))
		cursorCell := 2 + displayWidth(buf[start:cursorPos], secret)
		if cursorCell >= fieldWidth {
			cursorCell = fieldWidth - 1
		}
		s.ShowCursor(4+cursorCell, 5)
		DrawFooter(s, footer, w, h, *cfg)
		s.Show()

		ev, resized := pollUI(s)
		if resized {
			continue
		}
		switch ev := ev.(type) {
		case *tcell.EventPaste:
			pasting = ev.Start()
		case *tcell.EventKey:
			if pasting {
				switch ev.Key() {
				case tcell.KeyRune:
					buf, cursorPos = insertRunes(buf, cursorPos, []rune{ev.Rune()})
				case tcell.KeyEnter:
					buf, cursorPos = insertRunes(buf, cursorPos, []rune{' '})
				case tcell.KeyTab:
					buf, cursorPos = insertRunes(buf, cursorPos, []rune{' '})
				}
				continue
			}
			if CheckPaletteHotkey(ev) {
				s.HideCursor()
				GlobalPalette.Open(s, cfg)
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
			case tcell.KeyDelete:
				if cursorPos < len(buf) {
					buf = append(buf[:cursorPos], buf[cursorPos+1:]...)
				}
			case tcell.KeyLeft:
				if cursorPos > 0 {
					cursorPos--
				}
			case tcell.KeyRight:
				if cursorPos < len(buf) {
					cursorPos++
				}
			case tcell.KeyHome, tcell.KeyCtrlA:
				cursorPos = 0
			case tcell.KeyEnd, tcell.KeyCtrlE:
				cursorPos = len(buf)
			case tcell.KeyCtrlK:
				buf = buf[:cursorPos]
			case tcell.KeyCtrlU:
				buf = buf[cursorPos:]
				cursorPos = 0
			case tcell.KeyCtrlW:
				buf, cursorPos = deleteWordBack(buf, cursorPos)
			case tcell.KeyCtrlO:
				s.HideCursor()
				if path, ok := BrowsePath(s, cfg, string(buf)); ok {
					buf = []rune(path)
					cursorPos = len(buf)
				}
			case tcell.KeyRune:
				if ev.Rune() >= 32 && ev.Modifiers()&tcell.ModAlt == 0 {
					buf, cursorPos = insertRunes(buf, cursorPos, []rune{ev.Rune()})
				}
			}
		}
	}
}

func insertRunes(buf []rune, cursor int, extra []rune) ([]rune, int) {
	clean := make([]rune, 0, len(extra))
	for _, r := range extra {
		if r == '\r' {
			continue
		}
		if r == '\n' || r == '\t' {
			r = ' '
		}
		if r >= 32 || unicode.IsGraphic(r) {
			clean = append(clean, r)
		}
	}
	buf = append(buf[:cursor], append(clean, buf[cursor:]...)...)
	return buf, cursor + len(clean)
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

		DrawFooter(s, footer, w, h, *cfg)
		s.Show()

		ev, resized := pollUI(s)
		if resized {
			continue
		}
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if CheckPaletteHotkey(ev) {
				GlobalPalette.Open(s, cfg)
				continue
			}
			if ev.Key() == tcell.KeyEnter || ev.Key() == tcell.KeyEscape || ev.Rune() == 'q' {
				return
			}
		case *tcell.EventMouse:
			if ev.Buttons()&tcell.Button1 != 0 {
				return
			}
		}
	}
}
