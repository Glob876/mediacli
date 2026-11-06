package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"mediacli/pkg/core"

	"github.com/gdamore/tcell/v2"
)

type PaletteCommand struct {
	ID          string
	Title       string
	Description string
	Keywords    string
	Action      func(tcell.Screen, *core.Config)
}

type CommandPalette struct{}

var GlobalPalette = &CommandPalette{}

func paletteText(config core.Config, russian, english string) string {
	language := normalizeLang(config.Language)
	if language == "ru_ru" {
		return russian
	}
	return english
}

func paletteCommands(config core.Config) []PaletteCommand {
	commands := []PaletteCommand{
		{ID: "video", Title: T(config, "menu_video"), Description: paletteText(config, "Загрузить видео по ссылке", "Download a video from a URL"), Keywords: "video download видео загрузка", Action: ScreenVideo},
		{ID: "audio", Title: T(config, "menu_audio"), Description: paletteText(config, "Скачать аудио в выбранном формате", "Download audio in your preferred format"), Keywords: "audio music download аудио музыка", Action: ScreenAudio},
		{ID: "thumbnail", Title: T(config, "menu_thumbnail"), Description: paletteText(config, "Сохранить обложку видео", "Save a video thumbnail"), Keywords: "thumbnail poster artwork обложка", Action: ScreenThumbnail},
		{ID: "probe", Title: T(config, "menu_probe"), Description: paletteText(config, "Получить сведения о медиафайле", "Inspect media file details"), Keywords: "probe metadata информация", Action: ScreenProbe},
		{ID: "pipeline", Title: T(config, "menu_pipeline"), Description: paletteText(config, "Собрать последовательность операций", "Build a sequence of media operations"), Keywords: "pipeline operations цепочка", Action: ScreenPipeline},
		{ID: "editor", Title: paletteText(config, "Универсальный редактор", "Universal editor"), Description: paletteText(config, "Открыть файл для редактирования · Alt+3", "Open a file for editing · Alt+3"), Keywords: "editor edit редактор", Action: func(screen tcell.Screen, config *core.Config) { ScreenUniversal(screen, config, "") }},
		{ID: "convert", Title: T(config, "menu_convert"), Description: paletteText(config, "Поменять формат и кодеки файла", "Change a file's format and codecs"), Keywords: "convert conversion конвертация формат", Action: ScreenConvert},
		{ID: "trim", Title: T(config, "menu_trim"), Description: paletteText(config, "Сохранить выбранный фрагмент", "Save a selected time range"), Keywords: "trim cut обрезка фрагмент", Action: ScreenTrim},
		{ID: "aspect", Title: T(config, "menu_aspect_ratio"), Description: paletteText(config, "Изменить пропорции и размер кадра", "Change the frame size and aspect ratio"), Keywords: "aspect crop resize кадр пропорции", Action: ScreenAspectRatio},
		{ID: "mux", Title: T(config, "menu_mux"), Description: paletteText(config, "Объединить видео и аудио", "Combine video and audio tracks"), Keywords: "mux merge объединение", Action: ScreenMux},
		{ID: "batch", Title: T(config, "menu_batch_convert"), Description: paletteText(config, "Обработать несколько файлов", "Process multiple files"), Keywords: "batch files пакетная", Action: ScreenBatch},
		{ID: "inspector", Title: T(config, "menu_inspector"), Description: paletteText(config, "Посмотреть потоки и свойства файла", "Inspect streams and file properties"), Keywords: "inspector streams потоки", Action: ScreenInspector},
		{ID: "queue", Title: paletteText(config, "Очередь задач", "Task queue"), Description: paletteText(config, "Посмотреть прогресс и отменить задачу", "View progress or cancel a task"), Keywords: "queue background cancel очередь фон отмена", Action: screenPaletteQueue},
		{ID: "history", Title: T(config, "menu_history"), Description: paletteText(config, "Посмотреть результаты последних операций", "View the results of recent operations"), Keywords: "history logs история", Action: ScreenHistory},
		{ID: "settings", Title: T(config, "menu_settings"), Description: paletteText(config, "Параметры загрузки, cookies, прокси и пресеты", "Download settings, cookies, proxy and presets"), Keywords: "settings config cookies proxy preset настройки конфиг пресеты", Action: ScreenSettingsVertical},
		{ID: "theme", Title: paletteText(config, "Сменить тему", "Change theme"), Description: paletteText(config, "Выбрать цветовую тему терминала", "Choose terminal colours"), Keywords: "theme colors appearance тема цвета", Action: screenPaletteTheme},
		{ID: "doctor", Title: paletteText(config, "Проверить зависимости", "Check dependencies"), Description: paletteText(config, "Найти FFmpeg, ffprobe и yt-dlp", "Find FFmpeg, ffprobe and yt-dlp"), Keywords: "doctor dependencies ffmpeg yt-dlp зависимости диагностика", Action: screenPaletteDoctor},
		{ID: "clear-history", Title: paletteText(config, "Очистить историю", "Clear history"), Description: paletteText(config, "Удалить записи после подтверждения; файлы сохраняются", "Confirm clearing history entries; media files are retained"), Keywords: "clear history очистить историю", Action: screenPaletteClearHistory},
	}
	return commands
}

func filterPaletteCommands(commands []PaletteCommand, query string) []PaletteCommand {
	terms := strings.Fields(strings.ToLower(query))
	matches := make([]PaletteCommand, 0, len(commands))
	for _, command := range commands {
		searchable := strings.ToLower(command.Title + " " + command.Description + " " + command.Keywords)
		matched := true
		for _, term := range terms {
			if !strings.Contains(searchable, term) {
				matched = false
				break
			}
		}
		if matched {
			matches = append(matches, command)
		}
	}
	return matches
}

func (palette *CommandPalette) Open(screen tcell.Screen, config *core.Config) {
	palette.OpenOver(screen, config, nil)
}

func (palette *CommandPalette) OpenOver(screen tcell.Screen, config *core.Config, underlay func(tcell.Screen)) {
	commands := paletteCommands(*config)
	selectedCommand, confirmed := choosePaletteCommand(screen, config, commands, underlay)
	screen.HideCursor()
	if confirmed && selectedCommand.Action != nil {
		selectedCommand.Action(screen, config)
	}
}

func choosePaletteCommand(screen tcell.Screen, config *core.Config, commands []PaletteCommand, underlay func(tcell.Screen)) (PaletteCommand, bool) {
	query := []rune{}
	selected := 0
	if underlay != nil {
		stopRefresh := make(chan struct{})
		defer close(stopRefresh)
		go func() {
			ticker := time.NewTicker(80 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopRefresh:
					return
				case <-ticker.C:
					screen.PostEvent(tcell.NewEventInterrupt(nil))
				}
			}
		}()
	}
	for {
		matches := filterPaletteCommands(commands, string(query))
		if selected >= len(matches) {
			selected = max(0, len(matches)-1)
		}
		if underlay != nil {
			underlay(screen)
		}
		renderCommandPalette(screen, *config, query, matches, selected)
		event := screen.PollEvent()
		switch event := event.(type) {
		case *tcell.EventInterrupt:
			continue
		case *tcell.EventResize:
			screen.Sync()
			invalidateLogo()
		case *tcell.EventKey:
			if event.Key() == tcell.KeyEscape || CheckPaletteHotkey(event) {
				return PaletteCommand{}, false
			}
			switch event.Key() {
			case tcell.KeyUp:
				if len(matches) > 0 {
					selected = (selected + len(matches) - 1) % len(matches)
				}
			case tcell.KeyDown:
				if len(matches) > 0 {
					selected = (selected + 1) % len(matches)
				}
			case tcell.KeyEnter:
				if len(matches) > 0 {
					return matches[selected], true
				}
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if len(query) > 0 {
					query = query[:len(query)-1]
					selected = 0
				}
			case tcell.KeyCtrlU:
				query = nil
				selected = 0
			case tcell.KeyRune:
				if event.Rune() >= 32 {
					query = append(query, event.Rune())
					selected = 0
				}
			}
		}
	}
}

func renderCommandPalette(screen tcell.Screen, config core.Config, query []rune, commands []PaletteCommand, selected int) {
	width, height := screen.Size()
	boxWidth := min(88, width)
	boxHeight := min(19, height)
	left := (width - boxWidth) / 2
	top := (height - boxHeight) / 2
	baseStyle := GetBaseStyle(config)
	borderStyle := GetAccentStyle(config)
	for row := 0; row < boxHeight; row++ {
		for column := 0; column < boxWidth; column++ {
			screen.SetContent(left+column, top+row, ' ', nil, baseStyle)
		}
	}
	if boxWidth >= 2 && boxHeight >= 2 {
		horizontal := strings.Repeat("─", max(0, boxWidth-2))
		DrawString(screen, left, top, "┌"+horizontal+"┐", boxWidth, borderStyle)
		DrawString(screen, left, top+boxHeight-1, "└"+horizontal+"┘", boxWidth, borderStyle)
		for row := 1; row < boxHeight-1; row++ {
			screen.SetContent(left, top+row, '│', nil, borderStyle)
			screen.SetContent(left+boxWidth-1, top+row, '│', nil, borderStyle)
		}
	}
	contentWidth := max(0, boxWidth-4)
	title := paletteText(config, " Палитра команд ", " Command palette ")
	DrawString(screen, left+2, top, title, contentWidth, borderStyle.Bold(true))
	if boxHeight >= 3 {
		inputWidth := max(0, contentWidth-2)
		visibleQuery := query
		if len(visibleQuery) > inputWidth {
			visibleQuery = visibleQuery[len(visibleQuery)-inputWidth:]
		}
		prompt := "> " + string(visibleQuery)
		DrawString(screen, left+2, top+1, prompt, contentWidth, baseStyle)
		cursorX := left + 2 + min(StringWidth(prompt), max(0, contentWidth-1))
		if contentWidth > 0 {
			screen.ShowCursor(cursorX, top+1)
		} else {
			screen.HideCursor()
		}
	}
	visibleRows := max(0, (boxHeight-5)/2)
	if len(commands) == 0 && boxHeight >= 5 {
		DrawString(screen, left+2, top+3, paletteText(config, "Команды не найдены", "No matching commands"), contentWidth, GetDimStyle(config))
	}
	if visibleRows > 0 {
		firstVisible := max(0, selected-visibleRows+1)
		for index := firstVisible; index < len(commands) && index < firstVisible+visibleRows; index++ {
			row := top + 3 + (index-firstVisible)*2
			titleStyle := baseStyle
			marker := "  "
			if index == selected {
				titleStyle = GetHighlightStyle(config)
				marker = "> "
			}
			DrawString(screen, left+2, row, marker+commands[index].Title, contentWidth, titleStyle)
			DrawString(screen, left+4, row+1, commands[index].Description, max(0, contentWidth-2), GetDimStyle(config))
		}
	}
	if boxHeight >= 4 {
		footer := paletteText(config, "↑↓ выбор · Enter выполнить · Esc закрыть", "↑↓ select · Enter run · Esc close")
		DrawString(screen, left+2, top+boxHeight-2, footer, contentWidth, GetDimStyle(config))
	}
	screen.Show()
}

func CheckPaletteHotkey(event *tcell.EventKey) bool {
	result := event.Key() == tcell.KeyCtrlP || event.Key() == tcell.KeyF12
	return result
}

func screenPaletteQueue(screen tcell.Screen, config *core.Config) {
	title := paletteText(*config, "Очередь задач", "Task queue")
	for {
		tasks := core.GlobalQueue.Snapshots()
		if len(tasks) == 0 {
			ShowMessage(screen, config, title, []string{paletteText(*config, "Очередь пуста", "The queue is empty")}, T(*config, "footer_message"))
			return
		}
		labels := make([]string, 0, len(tasks))
		for _, task := range tasks {
			label := fmt.Sprintf("#%d · %s · %s · %.1f%%", task.ID, task.Title, task.Status, task.Progress)
			labels = append(labels, label)
		}
		selected := RunMenu(screen, config, title, labels, "", T(*config, "footer_nav"))
		if selected < 0 {
			return
		}
		task := tasks[selected]
		active := task.Status == core.StatusQueued || task.Status == core.StatusRunning
		actions := []string{paletteText(*config, "Посмотреть журнал", "View logs")}
		if active {
			actions = append(actions, paletteText(*config, "Отменить задачу", "Cancel task"))
		}
		action := RunMenu(screen, config, title, actions, task.Title, T(*config, "footer_nav"))
		if action == 0 {
			logs := core.GlobalQueue.TaskLogs(task.ID)
			ShowMessage(screen, config, task.Title, logs, T(*config, "footer_message"))
		}
		if action == 1 {
			core.GlobalQueue.CancelTask(task.ID)
		}
	}
}

func screenPaletteTheme(screen tcell.Screen, config *core.Config) {
	keys := make([]string, 0, len(Themes))
	for key := range Themes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	labels := make([]string, 0, len(keys))
	for _, key := range keys {
		theme := Themes[key]
		label := paletteText(*config, theme.NameRU, theme.NameEN)
		labels = append(labels, label)
	}
	selected := RunMenu(screen, config, paletteText(*config, "Цветовая тема", "Colour theme"), labels, "", T(*config, "footer_nav"))
	if selected < 0 {
		return
	}
	updatedConfig := *config
	updatedConfig.Theme = keys[selected]
	if err := core.SaveConfig(updatedConfig); err != nil {
		ShowMessage(screen, config, T(*config, "settings_title"), []string{err.Error()}, T(*config, "footer_message"))
		return
	}
	*config = updatedConfig
}

func screenPaletteDoctor(screen tcell.Screen, config *core.Config) {
	dependencies := core.CheckDependencies()
	lines := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		status := paletteText(*config, "не найден", "missing")
		if dependency.Available {
			status = dependency.Path
		}
		line := fmt.Sprintf("%s: %s", dependency.Name, status)
		lines = append(lines, line)
	}
	ShowMessage(screen, config, paletteText(*config, "Зависимости", "Dependencies"), lines, T(*config, "footer_message"))
}

func screenPaletteClearHistory(screen tcell.Screen, config *core.Config) {
	title := paletteText(*config, "Очистить историю?", "Clear history?")
	choices := []string{paletteText(*config, "Отмена", "Cancel"), paletteText(*config, "Очистить записи", "Clear entries")}
	selected := RunMenu(screen, config, title, choices, paletteText(*config, "Медиафайлы сохранятся", "Media files will be retained"), T(*config, "footer_nav"))
	if selected != 1 {
		return
	}
	message := paletteText(*config, "История очищена", "History cleared")
	if err := core.ClearHistory(); err != nil {
		message = err.Error()
	}
	ShowMessage(screen, config, title, []string{message}, T(*config, "footer_message"))
}
