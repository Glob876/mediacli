package ui

import (
	"fmt"
	"mediacli/pkg/core"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type Theme struct {
	ID     string
	NameEN string
	NameRU string
	AnsiBg int // 1..7 (ANSI-индекс из палитры терминала)
	AnsiFg int // 0 (Черный) или 7 (Белый)
}

// Темы используют нативные ANSI-цвета терминала (0..7), адаптируясь под системную тему
var Themes = map[string]Theme{
	"cyan":    {ID: "cyan", NameEN: "Arch Cyan (Default)", NameRU: "Arch Cyan (По умолчанию)", AnsiBg: 6, AnsiFg: 0},
	"nord":    {ID: "nord", NameEN: "Nord Blue", NameRU: "Nord Blue", AnsiBg: 4, AnsiFg: 0},
	"matrix":  {ID: "matrix", NameEN: "Matrix Green", NameRU: "Matrix Green", AnsiBg: 2, AnsiFg: 0},
	"dracula": {ID: "dracula", NameEN: "Dracula Magenta", NameRU: "Dracula Magenta", AnsiBg: 5, AnsiFg: 0},
	"gruvbox": {ID: "gruvbox", NameEN: "Gruvbox Yellow", NameRU: "Gruvbox Yellow", AnsiBg: 3, AnsiFg: 0},
	"fire":    {ID: "fire", NameEN: "Fire Red", NameRU: "Fire Red", AnsiBg: 1, AnsiFg: 0},
	"classic": {ID: "classic", NameEN: "Classic White", NameRU: "Классический (Белый)", AnsiBg: 7, AnsiFg: 0},
}

func GetTheme(cfg core.Config) Theme {
	if t, ok := Themes[cfg.Theme]; ok {
		return t
	}
	return Themes["cyan"]
}

func GetBaseStyle(cfg core.Config) tcell.Style {
	if cfg.UseTerminalBG {
		return tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	}
	return tcell.StyleDefault.Background(tcell.PaletteColor(0)).Foreground(tcell.PaletteColor(7))
}

func GetHighlightStyle(cfg core.Config) tcell.Style {
	th := GetTheme(cfg)
	// PaletteColor(N) берет точный цвет N из палитры твоего терминала
	fg := tcell.PaletteColor(th.AnsiFg)
	bg := tcell.PaletteColor(th.AnsiBg)
	return tcell.StyleDefault.Foreground(fg).Background(bg).Bold(true)
}

func GetAccentStyle(cfg core.Config) tcell.Style {
	th := GetTheme(cfg)
	accent := tcell.PaletteColor(th.AnsiBg)
	return GetBaseStyle(cfg).Foreground(accent).Bold(true)
}

func GetDimStyle(cfg core.Config) tcell.Style {
	return GetBaseStyle(cfg).Dim(true)
}

func DrawString(s tcell.Screen, x, y int, text string, maxW int, style tcell.Style) {
	if maxW <= 0 {
		return
	}
	runes := []rune(text)
	for i, r := range runes {
		if i >= maxW {
			break
		}
		s.SetContent(x+i, y, r, nil, style)
	}
}

func DrawHeader(s tcell.Screen, title string, w int, cfg core.Config) {
	titleStyle := GetAccentStyle(cfg)
	dimStyle := GetDimStyle(cfg)

	DrawString(s, 2, 0, title, w-4, titleStyle)
	DrawString(s, 0, 1, strings.Repeat("─", w), w, dimStyle)
}

func DrawFooter(s tcell.Screen, text string, w, h int) {
	dimStyle := tcell.StyleDefault.Background(tcell.ColorReset).Dim(true)
	bgStyle := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.PaletteColor(3)).Bold(true)

	DrawString(s, 0, h-2, strings.Repeat("─", w), w, dimStyle)
	DrawString(s, 2, h-1, text, w-28, dimStyle)

	summary := core.GlobalQueue.GetSummary()
	if summary != "" {
		DrawString(s, w-len(summary)-2, h-1, summary, len(summary)+2, bgStyle)
	}
}

func RenderProgressBar(pct float64, barW int, style string) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	innerW := barW - 2
	if innerW < 1 {
		innerW = 1
	}

	filled := int(float64(innerW) * (pct / 100.0))
	if filled > innerW {
		filled = innerW
	}
	unfilled := innerW - filled

	var bar string
	switch style {
	case "dots":
		bar = strings.Repeat("●", filled) + strings.Repeat("○", unfilled)
	case "minimal":
		bar = strings.Repeat("#", filled) + strings.Repeat("-", unfilled)
	case "classic":
		arrow := ""
		if filled < innerW && filled > 0 {
			arrow = ">"
		}
		spaces := strings.Repeat(" ", max(0, unfilled-len(arrow)))
		bar = strings.Repeat("=", filled) + arrow + spaces
	default: // "blocks"
		bar = strings.Repeat("█", filled) + strings.Repeat("░", unfilled)
	}
	return fmt.Sprintf("[%s]", bar)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}