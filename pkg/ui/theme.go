package ui

import (
	"fmt"
	"strings"

	"mediacli/pkg/core"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

type Theme struct {
	ID     string
	NameEN string
	NameRU string
	Bg     tcell.Color
	Fg     tcell.Color
	Accent tcell.Color
	SelFg  tcell.Color
	SelBg  tcell.Color
	Dim    tcell.Color
}

func hexColor(value int32) tcell.Color {
	return tcell.NewHexColor(value)
}

var Themes = map[string]Theme{
	"cyan": {
		ID: "cyan", NameEN: "Arch Cyan (Default)", NameRU: "Arch Cyan (По умолчанию)",
		Bg: hexColor(0x0c1116), Fg: hexColor(0xd6dee6), Accent: hexColor(0x3dd6d0),
		SelFg: hexColor(0x041014), SelBg: hexColor(0x3dd6d0), Dim: hexColor(0x6d7c89),
	},
	"nord": {
		ID: "nord", NameEN: "Nord", NameRU: "Nord",
		Bg: hexColor(0x2e3440), Fg: hexColor(0xeceff4), Accent: hexColor(0x88c0d0),
		SelFg: hexColor(0x2e3440), SelBg: hexColor(0x88c0d0), Dim: hexColor(0x7b88a1),
	},
	"matrix": {
		ID: "matrix", NameEN: "Matrix Green", NameRU: "Matrix Green",
		Bg: hexColor(0x0b120b), Fg: hexColor(0xb6f5c2), Accent: hexColor(0x39ff14),
		SelFg: hexColor(0x041204), SelBg: hexColor(0x39ff14), Dim: hexColor(0x4d7a55),
	},
	"dracula": {
		ID: "dracula", NameEN: "Dracula", NameRU: "Dracula",
		Bg: hexColor(0x282a36), Fg: hexColor(0xf8f8f2), Accent: hexColor(0xbd93f9),
		SelFg: hexColor(0x282a36), SelBg: hexColor(0xff79c6), Dim: hexColor(0x6272a4),
	},
	"gruvbox": {
		ID: "gruvbox", NameEN: "Gruvbox", NameRU: "Gruvbox",
		Bg: hexColor(0x282828), Fg: hexColor(0xebdbb2), Accent: hexColor(0xfabd2f),
		SelFg: hexColor(0x282828), SelBg: hexColor(0xfabd2f), Dim: hexColor(0xa89984),
	},
	"fire": {
		ID: "fire", NameEN: "Fire Red", NameRU: "Fire Red",
		Bg: hexColor(0x1c1013), Fg: hexColor(0xf8e4e4), Accent: hexColor(0xff5c57),
		SelFg: hexColor(0x1c1013), SelBg: hexColor(0xff5c57), Dim: hexColor(0xb08989),
	},
	"classic": {
		ID: "classic", NameEN: "Classic White", NameRU: "Классический (Белый)",
		Bg: hexColor(0x101010), Fg: hexColor(0xf4f4f4), Accent: hexColor(0xffffff),
		SelFg: hexColor(0x101010), SelBg: hexColor(0xf4f4f4), Dim: hexColor(0x9a9a9a),
	},
}

func GetTheme(cfg core.Config) Theme {
	if theme, ok := Themes[cfg.Theme]; ok {
		return theme
	}
	return Themes["cyan"]
}

func (theme Theme) DisplayName(cfg core.Config) string {
	if IsRussian(cfg) {
		return theme.NameRU
	}
	return theme.NameEN
}

func GetBaseStyle(cfg core.Config) tcell.Style {
	theme := GetTheme(cfg)
	style := tcell.StyleDefault.Foreground(theme.Fg)
	if cfg.UseTerminalBG {
		return style.Background(tcell.ColorReset)
	}
	return style.Background(theme.Bg)
}

func GetHighlightStyle(cfg core.Config) tcell.Style {
	theme := GetTheme(cfg)
	return tcell.StyleDefault.Foreground(theme.SelFg).Background(theme.SelBg).Bold(true)
}

func GetAccentStyle(cfg core.Config) tcell.Style {
	theme := GetTheme(cfg)
	return GetBaseStyle(cfg).Foreground(theme.Accent).Bold(true)
}

func GetDimStyle(cfg core.Config) tcell.Style {
	theme := GetTheme(cfg)
	return GetBaseStyle(cfg).Foreground(theme.Dim)
}

func StringWidth(text string) int {
	return uniseg.StringWidth(text)
}

func DrawString(s tcell.Screen, x, y int, text string, maxW int, style tcell.Style) {
	if maxW <= 0 || y < 0 || x < 0 {
		return
	}
	width := 0
	state := -1
	for len(text) > 0 && width < maxW {
		cluster, rest, _, next := uniseg.FirstGraphemeClusterInString(text, state)
		clusterWidth := uniseg.StringWidth(cluster)
		if clusterWidth == 0 {
			text = rest
			state = next
			continue
		}
		if width+clusterWidth > maxW {
			break
		}
		runes := []rune(cluster)
		var combining []rune
		if len(runes) > 1 {
			combining = runes[1:]
		}
		s.SetContent(x+width, y, runes[0], combining, style)
		width += clusterWidth
		text = rest
		state = next
	}
}

func DrawSpan(s tcell.Screen, x, y, width int, text string, style tcell.Style) {
	if width <= 0 || y < 0 || x < 0 {
		return
	}
	for column := 0; column < width; column++ {
		s.SetContent(x+column, y, ' ', nil, style)
	}
	DrawString(s, x, y, text, width, style)
}

func DrawHeader(s tcell.Screen, title string, w int, cfg core.Config) {
	titleStyle := GetAccentStyle(cfg)
	dimStyle := GetDimStyle(cfg)
	DrawString(s, 2, 0, title, max(0, w-4), titleStyle)
	if w > 0 {
		DrawString(s, 0, 1, strings.Repeat("─", w), w, dimStyle)
	}
}

func footerLayout(text, summary string, width int) (textLimit int, summaryX int) {
	if width <= 0 {
		return 0, -1
	}
	summaryWidth := StringWidth(summary)
	if summary == "" || summaryWidth+4 >= width {
		return max(0, width-4), -1
	}
	summaryX = width - summaryWidth - 2
	textLimit = summaryX - 4
	if textLimit < 0 {
		textLimit = 0
	}
	return textLimit, summaryX
}

func DrawFooter(s tcell.Screen, text string, w, h int, cfg core.Config) {
	dimStyle := GetDimStyle(cfg)
	summary := core.GlobalQueue.GetSummary()
	textLimit, summaryX := footerLayout(text, summary, w)
	if h >= 2 && w > 0 {
		DrawString(s, 0, h-2, strings.Repeat("─", w), w, dimStyle)
	}
	if h >= 1 {
		DrawString(s, 2, h-1, text, textLimit, dimStyle)
		if summaryX >= 0 {
			DrawString(s, summaryX, h-1, summary, StringWidth(summary), GetAccentStyle(cfg))
		}
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
	default:
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
