package ui

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"mediacli/pkg/core"

	"github.com/gdamore/tcell/v2"
)

var AsciiLogos = map[string][]string{
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
	// Kitty очистка всех изображений
	fmt.Print("\033_Ga=d,d=A;\033\\")
}

func RunApp() error {
	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.GetDefaultConfig()
	}

	s, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := s.Init(); err != nil {
		return err
	}
	defer s.Fini()
	s.HideCursor()

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

		DrawHeader(s, T(cfg, "app_title"), w, cfg)
		row := 2

		if h >= 24 && w >= 82 {
			if cfg.LogoMode == "image" && cfg.LogoImagePath != "" {
				row += 13
			} else {
				art := AsciiLogos[cfg.LogoAsciiPreset]
				if len(art) == 0 {
					art = AsciiLogos["standard"]
				}
				artStyle := GetAccentStyle(cfg)
				for _, line := range art {
					DrawString(s, max(2, (w-len(line))/2), row, line, w-4, artStyle)
					row++
				}
				row++
			}
		}

		tabX := 4
		for idx, tab := range tabs {
			tName := " [ " + T(cfg, tab.TitleKey) + " ] "
			style := GetDimStyle(cfg)
			if idx == curTab {
				style = GetHighlightStyle(cfg)
			}
			DrawString(s, tabX, row, tName, w-tabX, style)
			tabX += len(tName) + 2
		}
		row += 2

		activeItems := tabs[curTab].Items
		if curItem >= len(activeItems) {
			curItem = 0
		}

		hlStyle := GetHighlightStyle(cfg)
		baseStyle := GetBaseStyle(cfg)
		for idx, itm := range activeItems {
			y := row + idx
			if y >= h-3 {
				break
			}
			if idx == curItem {
				DrawString(s, 6, y, "> "+T(cfg, itm.LabelKey), w-12, hlStyle)
			} else {
				DrawString(s, 6, y, "  "+T(cfg, itm.LabelKey), w-12, baseStyle)
			}
		}

		DrawFooter(s, T(cfg, "footer_nav"), w, h)
		
		s.Show()

		if h >= 24 && w >= 82 && cfg.LogoMode == "image" && cfg.LogoImagePath != "" {
			ix := max(2, (w-40)/2)
			iy := 2
			ir := 11
			switch cfg.LogoProtocol {
			case "iterm2":
				fmt.Print(renderIterm2Image(cfg.LogoImagePath, ix, iy, ir))
			default:
				fmt.Print(renderKittyImage(cfg.LogoImagePath, ix, iy, ir))
			}
		}

		ev := s.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if CheckTerminalHotkey(ev) {
				clearTerminalImages()
				GlobalTerminal.Open(s, &cfg)
				continue
			}
			switch ev.Key() {
			case tcell.KeyLeft:
				if curTab > 0 {
					curTab--
				} else {
					curTab = len(tabs) - 1
				}
				curItem = 0
			case tcell.KeyRight:
				if curTab < len(tabs)-1 {
					curTab++
				} else {
					curTab = 0
				}
				curItem = 0
			case tcell.KeyUp:
				if curItem > 0 {
					curItem--
				} else {
					curItem = len(activeItems) - 1
				}
			case tcell.KeyDown:
				if curItem < len(activeItems)-1 {
					curItem++
				} else {
					curItem = 0
				}
			case tcell.KeyEnter:
				itm := activeItems[curItem]
				clearTerminalImages()
				if itm.Action == nil {
					return nil
				}
				itm.Action(s, &cfg)
			case tcell.KeyEscape:
				clearTerminalImages()
				return nil
			case tcell.KeyRune:
				if ev.Rune() == 'h' {
					if curTab > 0 {
						curTab--
					}
					curItem = 0
				} else if ev.Rune() == 'l' {
					if curTab < len(tabs)-1 {
						curTab++
					}
					curItem = 0
				} else if ev.Rune() == 'k' {
					if curItem > 0 {
						curItem--
					}
				} else if ev.Rune() == 'j' {
					if curItem < len(activeItems)-1 {
						curItem++
					}
				} else if ev.Rune() == 'q' {
					clearTerminalImages()
					return nil
				}
			}
		}
	}
}