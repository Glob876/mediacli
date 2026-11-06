package ui

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"mediacli/pkg/core"

	"github.com/gdamore/tcell/v2"
)

var ASCIILogos = map[string][]string{
	"standard": {
		` _____ ______   _______   ________  ___  ________  ________  ___       ___     `,
		`|\   _ \  _   \|\  ___ \ |\   ___ \|\  \|\   __  \|\   ____\|\  \     |\  \    `,
		`\ \  \\\__\ \  \ \   __/|\ \  \_|\ \ \  \ \  \|\  \ \  \___|\ \  \    \ \  \   `,
		` \ \  \\|__| \  \ \  \_|/_\ \  \ \\ \ \  \ \   __  \ \  \    \ \  \    \ \  \  `,
		`  \ \  \    \ \  \ \  \_|\ \ \  \_\\ \ \  \ \  \ \  \ \  \____\ \  \____\ \  \ `,
		`   \ \__\    \ \__\ \_______\ \_______\ \__\ \__\ \__\ \_______\ \_______\ \__\`,
		`    \|__|     \|__|\|_______|\|_______|\|__|\|__|\|__|\|_______|\|_______|\|__|`,
	},
	"coder_mini": {
		`▄▄▄      ▄▄▄          ▄▄            ▄▄▄▄▄▄▄ ▄▄▄      ▄▄▄▄▄ `,
		`████▄  ▄████          ██ ▀▀        ███▀▀▀▀▀ ███       ███  `,
		`███▀████▀███ ▄█▀█▄ ▄████ ██   ▀▀█▄ ███      ███       ███  `,
		`███  ▀▀  ███ ██▄█▀ ██ ██ ██  ▄█▀██ ███      ███       ███  `,
		`███      ███ ▀█▄▄▄ ▀████ ██▄ ▀█▄██ ▀███████ ████████ ▄███▄ `,
	},
	"toilet": {
		` mmm  mmm                  mm     ##                  mmmm   mm         mmmmmm  `,
		` ###  ###                  ##     ""                ##""""#  ##         ""##""  `,
		` ########   m####m    m###m##   ####      m#####m  ##"       ##           ##    `,
		` ## ## ##  ##mmmm##  ##"  "##     ##      " mmm##  ##        ##           ##    `,
		` ## "" ##  ##""""""  ##    ##     ##     m##"""##  ##m       ##           ##    `,
		` ##    ##  "##mmmm#  "##mm###  mmm##mmm  ##mmm###   ##mmmm#  ##mmmmmm   mm##mm  `,
		` ""    ""    """""     """ ""  """"""""   """" ""     """"   """"""""   """"""  `,
	},
	"rubifont": {
		`▗▖  ▗▖▗▄▄▄▖▗▄▄▄ ▗▄▄▄▖ ▗▄▖  ▗▄▄▖▗▖   ▗▄▄▄▖`,
		`▐▛▚▞▜▌▐▌   ▐▌  █  █  ▐▌ ▐▌▐▌   ▐▌     █  `,
		`▐▌  ▐▌▐▛▀▀▘▐▌  █  █  ▐▛▀▜▌▐▌   ▐▌     █  `,
		`▐▌  ▐▌▐▙▄▄▖▐▙▄▄▀▗▄█▄▖▐▌ ▐▌▝▚▄▄▖▐▙▄▄▖▗▄█▄▖`,
	},
}

type Tab struct {
	ID       string
	TitleKey string
	Items    []MenuItem
}

type MenuItem struct {
	LabelKey string
	Action   func(s tcell.Screen, cfg *core.Config)
}

func renderKittyImage(path string, x, y, rows int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	b64 := base64.StdEncoding.EncodeToString(b)

	f := 100 // PNG
	low := strings.ToLower(path)
	if strings.HasSuffix(low, ".jpg") || strings.HasSuffix(low, ".jpeg") {
		f = 10
	}

	var sb strings.Builder
	// Перемещаем курсор
	sb.WriteString(fmt.Sprintf("\033[%d;%dH", y+1, x+1))

	chunkSize := 4096
	for i := 0; i < len(b64); i += chunkSize {
		end := i + chunkSize
		m := 1
		if end >= len(b64) {
			end = len(b64)
			m = 0
		}
		if i == 0 {
			// a=T (transmit and display), t=d (base64 direct)
			// r=rows (высота в строках терминала, ширина подстроится автоматически для сохранения пропорций)
			sb.WriteString(fmt.Sprintf("\033_Ga=T,f=%d,t=d,r=%d,m=%d;%s\033\\", f, rows, m, b64[i:end]))
		} else {
			sb.WriteString(fmt.Sprintf("\033_Gm=%d;%s\033\\", m, b64[i:end]))
		}
	}
	return sb.String()
}

func renderIterm2Image(path string, x, y, rows int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	b64 := base64.StdEncoding.EncodeToString(b)
	pos := fmt.Sprintf("\033[%d;%dH", y+1, x+1)
	// iTerm2 protocol: height=...c, preserveAspectRatio=1
	return fmt.Sprintf("%s\033]1337;File=inline=1;height=%dc;preserveAspectRatio=1:%s\a", pos, rows, b64)
}

func clearTerminalImages() {
	fmt.Print("\033_Ga=d,d=A;\033\\")
	logoStamp = ""
}

func RunApp() error {
	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.GetDefaultConfig()
	}
	core.GlobalQueue.SyncMaxTasksFromConfig(cfg)

	s, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("tcell.NewScreen failed (TERM=%q): %w — убедись что запускаешь в интерактивном терминале с TERM=xterm-256color", os.Getenv("TERM"), err)
	}
	if err := s.Init(); err != nil {
		return fmt.Errorf("tcell.Init failed (TERM=%q): %w — нет TTY (/dev/tty) или TERM=dumb; запусти в реальном терминале", os.Getenv("TERM"), err)
	}
	defer s.Fini()
	s.HideCursor()
	s.EnablePaste()
	s.EnableMouse()

	tabs := []Tab{
		{
			ID:       "online",
			TitleKey: "tab_online",
			Items: []MenuItem{
				{LabelKey: "menu_video", Action: ScreenVideo},
				{LabelKey: "menu_audio", Action: ScreenAudio},
				{LabelKey: "menu_thumbnail", Action: ScreenThumbnail},
				{LabelKey: "menu_probe", Action: ScreenProbe},
			},
		},
		{
			ID:       "local",
			TitleKey: "tab_local",
			Items: []MenuItem{
				{LabelKey: "menu_pipeline", Action: ScreenPipeline},
				{LabelKey: "menu_mux", Action: ScreenMux},
				{LabelKey: "menu_batch_convert", Action: ScreenBatch},
				{LabelKey: "menu_inspector", Action: ScreenInspector},
				{LabelKey: "menu_aspect_ratio", Action: ScreenAspectRatio},
				{LabelKey: "menu_convert", Action: ScreenConvert},
				{LabelKey: "menu_trim", Action: ScreenTrim},
			},
		},
		{
			ID:       "system",
			TitleKey: "tab_system",
			Items: []MenuItem{
				{LabelKey: "menu_history", Action: ScreenHistory},
				{LabelKey: "menu_settings", Action: ScreenSettingsVertical},
				{LabelKey: "menu_exit", Action: nil},
			},
		},
	}

	curTab := 0
	curItem := 0

	for {
		w, h := s.Size()
		s.Clear()

		DrawHeader(s, T(cfg, "app_title")+" "+core.Version, w, cfg)
		row := 2

		if h >= 24 && w >= 82 {
			if cfg.LogoMode == "image" && cfg.LogoImagePath != "" {
				row += 13
			} else {
				art := ASCIILogos[cfg.LogoAsciiPreset]
				if len(art) == 0 {
					art = ASCIILogos["standard"]
				}
				artStyle := GetAccentStyle(cfg)
				for _, line := range art {
					lineWidth := StringWidth(line)
					DrawString(s, max(2, (w-lineWidth)/2), row, line, w-4, artStyle)
					row++
				}
				row++
			}
		}

		tabX := 4
		tabRow := row
		tabHits := make([]menuHit, 0, len(tabs))
		for idx, tab := range tabs {
			tName := " [ " + T(cfg, tab.TitleKey) + " ] "
			style := GetDimStyle(cfg)
			if idx == curTab {
				style = GetHighlightStyle(cfg)
			}
			DrawString(s, tabX, row, tName, max(0, w-tabX), style)
			nameWidth := StringWidth(tName)
			tabHits = append(tabHits, menuHit{minX: tabX, maxX: tabX + nameWidth, y: tabRow, index: idx})
			tabX += nameWidth + 2
		}
		row += 2

		activeItems := tabs[curTab].Items
		if curItem >= len(activeItems) {
			curItem = 0
		}

		row = min(row, max(0, h-4))
		visibleRows := max(1, h-row-3)
		firstVisible := windowStart(curItem, max(0, curItem-visibleRows+1), visibleRows)
		hlStyle := GetHighlightStyle(cfg)
		baseStyle := GetBaseStyle(cfg)
		itemHits := make([]menuHit, 0, visibleRows)
		for idx := firstVisible; idx < len(activeItems); idx++ {
			itm := activeItems[idx]
			y := row + idx - firstVisible
			if y >= h-3 {
				break
			}
			label := "  " + T(cfg, itm.LabelKey)
			style := baseStyle
			if idx == curItem {
				label = "> " + T(cfg, itm.LabelKey)
				style = hlStyle
			}
			DrawSpan(s, 6, y, max(0, w-12), label, style)
			itemHits = append(itemHits, menuHit{minX: 6, maxX: max(6, w-6), y: y, index: idx})
		}

		DrawFooter(s, T(cfg, "footer_nav"), w, h, cfg)

		s.Show()

		if h >= 24 && w >= 82 && cfg.LogoMode == "image" && cfg.LogoImagePath != "" {
			placeLogo(cfg.LogoImagePath, cfg.LogoProtocol, max(2, (w-40)/2), 2, 11)
		} else {
			invalidateLogo()
		}

		ev, resized := pollUI(s)
		if resized {
			continue
		}
		activate := func() bool {
			itm := activeItems[curItem]
			invalidateLogo()
			if itm.Action == nil {
				return true
			}
			itm.Action(s, &cfg)
			return false
		}
		switch ev := ev.(type) {
		case *tcell.EventMouse:
			x, y := ev.Position()
			if ev.Buttons()&tcell.WheelUp != 0 {
				curItem = wrapIndex(curItem, -1, len(activeItems))
			} else if ev.Buttons()&tcell.WheelDown != 0 {
				curItem = wrapIndex(curItem, 1, len(activeItems))
			} else if ev.Buttons()&tcell.Button1 != 0 {
				if hit, ok := hitIndex(tabHits, x, y); ok {
					curTab = hit
					curItem = 0
				} else if hit, ok := hitIndex(itemHits, x, y); ok {
					curItem = hit
					if activate() {
						return nil
					}
				}
			}
		case *tcell.EventKey:
			if CheckPaletteHotkey(ev) {
				invalidateLogo()
				GlobalPalette.Open(s, &cfg)
				continue
			}
			if ev.Modifiers()&tcell.ModAlt != 0 && ev.Key() == tcell.KeyRune && ev.Rune() == '3' {
				invalidateLogo()
				ScreenUniversal(s, &cfg, "")
				continue
			}
			switch ev.Key() {
			case tcell.KeyLeft:
				curTab = wrapIndex(curTab, -1, len(tabs))
				curItem = 0
			case tcell.KeyRight:
				curTab = wrapIndex(curTab, 1, len(tabs))
				curItem = 0
			case tcell.KeyUp:
				curItem = wrapIndex(curItem, -1, len(activeItems))
			case tcell.KeyDown:
				curItem = wrapIndex(curItem, 1, len(activeItems))
			case tcell.KeyEnter:
				if activate() {
					return nil
				}
			case tcell.KeyEscape:
				invalidateLogo()
				return nil
			case tcell.KeyRune:
				switch ev.Rune() {
				case 'h':
					curTab = wrapIndex(curTab, -1, len(tabs))
					curItem = 0
				case 'l':
					curTab = wrapIndex(curTab, 1, len(tabs))
					curItem = 0
				case 'k':
					curItem = wrapIndex(curItem, -1, len(activeItems))
				case 'j':
					curItem = wrapIndex(curItem, 1, len(activeItems))
				case 'q':
					invalidateLogo()
					return nil
				}
			}
		}
	}
}
