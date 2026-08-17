package ui

import (
	"mediacli/pkg/core"

	"github.com/gdamore/tcell/v2"
)

var asciiArt = []string{
	` _____ ______   _______   ________  ___  ________  ________  ___       ___     `,
	`|\   _ \  _   \|\  ___ \ |\   ___ \|\  \|\   __  \|\   ____\|\  \     |\  \    `,
	`\ \  \\\__\ \  \ \   __/|\ \  \_|\ \ \  \ \  \|\  \ \  \___|\ \  \    \ \  \   `,
	` \ \  \\|__| \  \ \  \_|/_\ \  \ \\ \ \  \ \   __  \ \  \    \ \  \    \ \  \  `,
	`  \ \  \    \ \  \ \  \_|\ \ \  \_\\ \ \  \ \  \ \  \ \  \____\ \  \____\ \  \ `,
	`   \ \__\    \ \__\ \_______\ \_______\ \__\ \__\ \__\ \_______\ \_______\ \__\`,
	`    \|__|     \|__|\|_______|\|_______|\|__|\|__|\|__|\|_______|\|_______|\|__|`,
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
				{LabelKey: "menu_probe", Action: ScreenProbe},
			},
		},
		{
			ID:       "local",
			TitleKey: "tab_local",
			Items: []MenuItem{
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
			artStyle := GetAccentStyle(cfg)
			for _, line := range asciiArt {
				DrawString(s, max(2, (w-len(line))/2), row, line, w-4, artStyle)
				row++
			}
			row++
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

		ev := s.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if CheckTerminalHotkey(ev) {
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
				if itm.Action == nil {
					return nil // Exit
				}
				itm.Action(s, &cfg)
			case tcell.KeyEscape:
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
					return nil
				}
			}
		}
	}
}