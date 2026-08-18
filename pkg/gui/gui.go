package gui

import (
	"bufio"
	"fmt"
	"image/color"
	"mediacli/pkg/core"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ============================================================================
// Кастомная современная темная тема (Modern Dark Theme)
// ============================================================================

type modernDarkTheme struct{}

func (m *modernDarkTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.RGBA{R: 18, G: 20, B: 26, A: 255} // Глубокий темный фон
	case theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground:
		return color.RGBA{R: 26, G: 29, B: 38, A: 255} // Фон модальных окон и блоков
	case theme.ColorNameButton:
		return color.RGBA{R: 36, G: 41, B: 54, A: 255} // Цвет кнопок
	case theme.ColorNamePrimary:
		return color.RGBA{R: 0, G: 210, B: 196, A: 255} // Неоновый циан/акцент
	case theme.ColorNameHover:
		return color.RGBA{R: 48, G: 56, B: 74, A: 255}
	case theme.ColorNameInputBackground:
		return color.RGBA{R: 28, G: 32, B: 42, A: 255}
	case theme.ColorNameForeground:
		return color.RGBA{R: 242, G: 245, B: 250, A: 255}
	case theme.ColorNamePlaceHolder:
		return color.RGBA{R: 115, G: 125, B: 142, A: 255}
	case theme.ColorNameSeparator:
		return color.RGBA{R: 42, G: 47, B: 62, A: 255}
	default:
		return theme.DefaultTheme().Color(name, theme.VariantDark)
	}
}

func (m *modernDarkTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m *modernDarkTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *modernDarkTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 10.0
	case theme.SizeNameInnerPadding:
		return 12.0
	case theme.SizeNameInputRadius:
		return 8.0
	default:
		return theme.DefaultTheme().Size(name)
	}
}

// ============================================================================
// Структура карточки-куба
// ============================================================================

type DownloadCard struct {
	Container   *fyne.Container
	TitleLabel  *widget.Label
	StageLabel  *widget.Label
	ProgressBar *widget.ProgressBar
	Status      string
	URL         string
}

// ============================================================================
// Главное окно GUI
// ============================================================================

func RunGUI() {
	a := app.New()
	a.Settings().SetTheme(&modernDarkTheme{})

	w := a.NewWindow("MediaCLI — Advanced Media Suite")
	w.Resize(fyne.NewSize(1080, 720))
	w.CenterOnScreen()

	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.GetDefaultConfig()
	}

	// Сетка карточек с гарантированными отступами
	cardsGrid := container.New(layout.NewGridLayout(3))
	scrollContainer := container.NewVScroll(container.NewPadded(cardsGrid))

	// Загрузка истории
	historyEntries := core.GetHistory()
	for _, h := range historyEntries {
		card := createHistoryCard(h)
		cardsGrid.Add(card)
	}

	// Верхняя панель: Поле ввода ссылки
	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("Paste video, playlist, or audio URL and press Enter...")

	// Селекторы панели подтверждения
	presetSelect := widget.NewSelect([]string{
		"1. Standard MP4 (H.264 + AAC) [Universal]",
		"2. Modern MKV (AV1 + Opus/AAC) [Next-Gen]",
		"3. High Efficiency MP4 (H.265 / HEVC)",
		"4. DaVinci Resolve (DNxHR HQ + PCM .mov)",
		"5. DaVinci Resolve (Apple ProRes 422 .mov)",
		"6. Audio Only: MP3 320 kbps",
		"7. Audio Only: FLAC Lossless",
	}, func(string) {})
	presetSelect.SetSelected("1. Standard MP4 (H.264 + AAC) [Universal]")

	qualitySelect := widget.NewSelect([]string{
		"Best Available (Max)",
		"4K Ultra HD (2160p)",
		"2K Quad HD (1440p)",
		"Full HD (1080p)",
		"HD (720p)",
		"SD (480p)",
	}, func(string) {})
	qualitySelect.SetSelected("Best Available (Max)")

	timeRangeEntry := widget.NewEntry()
	timeRangeEntry.SetPlaceHolder("Time range (e.g. 00:01:00-00:03:30 or leave blank for full video)")

	subsCheck := widget.NewCheck("Download Subtitles (ru,en)", func(bool) {})
	sponsorCheck := widget.NewCheck("SponsorBlock (Cut Sponsorships)", func(bool) {})

	var confirmPanel *fyne.Container

	// Действие старта загрузки
	startDownloadAction := func() {
		targetURL := strings.TrimSpace(urlEntry.Text)
		if targetURL == "" {
			return
		}

		confirmPanel.Hide()

		fields := core.GetInitialPresetFields()
		switch presetSelect.Selected {
		case "1. Standard MP4 (H.264 + AAC) [Universal]":
			fields["video_preset"] = "standard_mp4"
		case "2. Modern MKV (AV1 + Opus/AAC) [Next-Gen]":
			fields["video_preset"] = "mkv_av1"
		case "3. High Efficiency MP4 (H.265 / HEVC)":
			fields["video_preset"] = "hevc_mp4"
		case "4. DaVinci Resolve (DNxHR HQ + PCM .mov)":
			fields["video_preset"] = "davinci_dnxhr"
		case "5. DaVinci Resolve (Apple ProRes 422 .mov)":
			fields["video_preset"] = "davinci_prores"
		case "6. Audio Only: MP3 320 kbps":
			fields["audio_only"] = true
			fields["audio_format"] = "mp3"
		case "7. Audio Only: FLAC Lossless":
			fields["audio_only"] = true
			fields["audio_format"] = "flac"
		}

		switch qualitySelect.Selected {
		case "4K Ultra HD (2160p)":
			fields["quality"] = "2160"
		case "2K Quad HD (1440p)":
			fields["quality"] = "1440"
		case "Full HD (1080p)":
			fields["quality"] = "1080"
		case "HD (720p)":
			fields["quality"] = "720"
		case "SD (480p)":
			fields["quality"] = "480"
		}

		if strings.TrimSpace(timeRangeEntry.Text) != "" {
			fields["download_section"] = strings.TrimSpace(timeRangeEntry.Text)
		}
		if subsCheck.Checked {
			fields["subs_enabled"] = true
			fields["embed_subs"] = true
		}
		if sponsorCheck.Checked {
			fields["sponsorblock"] = "remove"
		}

		preset := core.DownloadPreset{
			ID:     fmt.Sprintf("gui_%d", time.Now().Unix()),
			Name:   "GUI Download",
			Fields: fields,
		}

		card := createActiveDownloadCard(targetURL)
		cardsGrid.Objects = append([]fyne.CanvasObject{card.Container}, cardsGrid.Objects...)
		cardsGrid.Refresh()

		urlEntry.SetText("")

		go executeGUIDownload(card, preset, cfg, targetURL)
	}

	btnCancel := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		confirmPanel.Hide()
	})

	btnStart := widget.NewButtonWithIcon("Start Download", theme.ConfirmIcon(), startDownloadAction)
	btnStart.Importance = widget.HighImportance

	panelBackground := canvas.NewRectangle(color.RGBA{R: 24, G: 28, B: 38, A: 255})
	panelBackground.StrokeColor = color.RGBA{R: 44, G: 52, B: 70, A: 255}
	panelBackground.StrokeWidth = 1.0
	panelBackground.CornerRadius = 8.0

	panelContent := container.NewVBox(
		container.NewGridWithColumns(2,
			container.NewVBox(widget.NewLabelWithStyle("Target Codec Preset:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), presetSelect),
			container.NewVBox(widget.NewLabelWithStyle("Max Resolution:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), qualitySelect),
		),
		timeRangeEntry,
		container.NewHBox(subsCheck, widget.NewLabel("    "), sponsorCheck),
		widget.NewSeparator(),
		container.NewHBox(
			layout.NewSpacer(),
			btnCancel,
			widget.NewLabel("  "),
			btnStart,
		),
	)

	confirmPanel = container.NewStack(
		panelBackground,
		container.NewPadded(panelContent),
	)
	confirmPanel.Hide()

	urlEntry.OnSubmitted = func(s string) {
		if strings.TrimSpace(s) == "" {
			return
		}
		confirmPanel.Show()
	}

	btnSettings := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), func() {
		openSettingsDialog(w, &cfg)
	})

	btnPaste := widget.NewButtonWithIcon("Paste URL", theme.ContentPasteIcon(), func() {
		if cb := w.Clipboard().Content(); cb != "" {
			urlEntry.SetText(cb)
			confirmPanel.Show()
		}
	})

	btnGo := widget.NewButtonWithIcon("Configure", theme.DownloadIcon(), func() {
		if strings.TrimSpace(urlEntry.Text) != "" {
			confirmPanel.Show()
		}
	})
	btnGo.Importance = widget.HighImportance

	leftToolbar := container.NewHBox(btnSettings, widget.NewLabel(" "))
	rightToolbar := container.NewHBox(widget.NewLabel(" "), btnPaste, widget.NewLabel(" "), btnGo)

	topBar := container.NewBorder(
		nil, nil,
		leftToolbar,
		rightToolbar,
		urlEntry,
	)

	headerContainer := container.NewVBox(
		container.NewPadded(topBar),
		container.NewPadded(confirmPanel),
		widget.NewSeparator(),
	)

	mainLayout := container.NewBorder(
		headerContainer,
		nil, nil, nil,
		scrollContainer,
	)

	w.SetContent(mainLayout)
	w.ShowAndRun()
}

// ============================================================================
// Отрисовка интерактивной карточки-куба (Active Card)
// ============================================================================

func createActiveDownloadCard(url string) *DownloadCard {
	card := &DownloadCard{
		URL:    url,
		Status: "Initializing...",
	}

	cardBg := canvas.NewRectangle(color.RGBA{R: 26, G: 30, B: 40, A: 255})
	cardBg.StrokeColor = color.RGBA{R: 0, G: 210, B: 196, A: 180}
	cardBg.StrokeWidth = 1.5
	cardBg.CornerRadius = 10.0

	thumbBg := canvas.NewRectangle(color.RGBA{R: 36, G: 42, B: 56, A: 255})
	thumbBg.SetMinSize(fyne.NewSize(280, 130))
	thumbBg.CornerRadius = 6.0

	mediaIcon := widget.NewIcon(theme.MediaPlayIcon())
	thumbStack := container.NewStack(thumbBg, container.NewCenter(mediaIcon))

	titleLabel := widget.NewLabelWithStyle(url, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	titleLabel.Truncation = fyne.TextTruncateEllipsis
	card.TitleLabel = titleLabel

	stageLabel := widget.NewLabel("Initializing pipeline...")
	stageLabel.TextStyle = fyne.TextStyle{Italic: true}
	stageLabel.Truncation = fyne.TextTruncateEllipsis
	card.StageLabel = stageLabel

	pBar := widget.NewProgressBar()
	pBar.SetValue(0.0)
	card.ProgressBar = pBar

	cardInner := container.NewVBox(
		thumbStack,
		titleLabel,
		stageLabel,
		pBar,
	)

	card.Container = container.NewPadded(container.NewStack(
		cardBg,
		container.NewPadded(cardInner),
	))
	return card
}

// ============================================================================
// Отрисовка карточки завершенной истории
// ============================================================================

func createHistoryCard(entry core.HistoryEntry) fyne.CanvasObject {
	cardBg := canvas.NewRectangle(color.RGBA{R: 23, G: 26, B: 35, A: 255})
	cardBg.StrokeColor = color.RGBA{R: 42, G: 48, B: 64, A: 255}
	cardBg.StrokeWidth = 1.0
	cardBg.CornerRadius = 10.0

	thumbBg := canvas.NewRectangle(color.RGBA{R: 30, G: 34, B: 46, A: 255})
	thumbBg.SetMinSize(fyne.NewSize(280, 130))
	thumbBg.CornerRadius = 6.0

	statusIcon := theme.ConfirmIcon()
	if strings.Contains(strings.ToLower(entry.Status), "fail") {
		statusIcon = theme.ErrorIcon()
	}

	thumbStack := container.NewStack(thumbBg, container.NewCenter(widget.NewIcon(statusIcon)))

	title := entry.Target
	if title == "" {
		title = entry.Source
	}
	titleLabel := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	titleLabel.Truncation = fyne.TextTruncateEllipsis

	infoLabel := widget.NewLabel(fmt.Sprintf("%s • %s", entry.Time, entry.Status))
	infoLabel.TextStyle = fyne.TextStyle{Italic: true}
	infoLabel.Truncation = fyne.TextTruncateEllipsis

	cardInner := container.NewVBox(
		thumbStack,
		titleLabel,
		infoLabel,
	)

	return container.NewPadded(container.NewStack(
		cardBg,
		container.NewPadded(cardInner),
	))
}

// ============================================================================
// Фоновый воркер выполнения задачи
// ============================================================================

func executeGUIDownload(card *DownloadCard, preset core.DownloadPreset, cfg core.Config, url string) {
	outDir := core.ParseUserPath(cfg.DownloadDir)
	_ = os.MkdirAll(outDir, 0755)

	cmdList := append([]string{"yt-dlp"}, core.BuildYtDlpArgs(preset, cfg, outDir, false)...)
	cmdList = append(cmdList, url)

	cmd := exec.Command(cmdList[0], cmdList[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		card.StageLabel.SetText("Error spawning process")
		return
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		card.StageLabel.SetText("Failed to start yt-dlp")
		return
	}

	scanner := bufio.NewScanner(stdout)
	currentStage := "Downloading..."
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		currentStage = core.DetectStage(line, currentStage)
		card.StageLabel.SetText(currentStage)

		if strings.HasPrefix(line, "[download] Destination:") {
			dest := strings.TrimPrefix(line, "[download] Destination:")
			card.TitleLabel.SetText(filepath.Base(strings.TrimSpace(dest)))
		}

		if pct, speed, ok := core.ExtractProgress(line); ok {
			card.ProgressBar.SetValue(pct / 100.0)
			if speed != "" {
				card.StageLabel.SetText(fmt.Sprintf("%s (%s)", currentStage, speed))
			}
		}
	}

	exitCode := 0
	if err := cmd.Wait(); err != nil {
		exitCode = 1
	}

	if exitCode == 0 {
		card.ProgressBar.SetValue(1.0)
		card.StageLabel.SetText("Completed successfully")
		_ = core.AddHistoryEntry("Download", url, card.TitleLabel.Text, "Success")
	} else {
		card.StageLabel.SetText("Download failed")
		_ = core.AddHistoryEntry("Download", url, card.TitleLabel.Text, "Failed")
	}
}

// ============================================================================
// Вкладки настроек программы (Modern Settings Tabs)
// ============================================================================

func openSettingsDialog(parent fyne.Window, cfg *core.Config) {
	dirEntry := widget.NewEntry()
	dirEntry.SetText(cfg.DownloadDir)

	proxyEntry := widget.NewEntry()
	proxyEntry.SetText(cfg.ProxyURL)

	generalTab := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Download Directory", dirEntry),
			widget.NewFormItem("Network Proxy URL", proxyEntry),
		),
	)

	presetKeys := core.OrderedVideoPresetKeys
	var presetNames []string
	for _, k := range presetKeys {
		presetNames = append(presetNames, core.VideoPresets[k].NameEN)
	}
	videoPresetSelect := widget.NewSelect(presetNames, func(string) {})
	if p, ok := core.VideoPresets[cfg.VideoPreset]; ok {
		videoPresetSelect.SetSelected(p.NameEN)
	}

	audioFmtSelect := widget.NewSelect([]string{"mp3", "flac", "wav", "m4a", "opus"}, func(string) {})
	audioFmtSelect.SetSelected(cfg.AudioFormat)

	subLangsEntry := widget.NewEntry()
	subLangsEntry.SetText(cfg.SubLangs)

	thumbFmtSelect := widget.NewSelect([]string{"png", "jpg", "webp"}, func(string) {})
	thumbFmtSelect.SetSelected(cfg.ThumbnailFormat)

	codecsTab := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Default Video Codec", videoPresetSelect),
			widget.NewFormItem("Default Audio Format", audioFmtSelect),
			widget.NewFormItem("Subtitle Languages", subLangsEntry),
			widget.NewFormItem("Thumbnail Format", thumbFmtSelect),
		),
	)

	fragmentsSelect := widget.NewSelect([]string{"2", "4", "8", "16"}, func(string) {})
	fragmentsSelect.SetSelected(fmt.Sprintf("%d", cfg.ConcurrentFragments))

	noMtimeCheck := widget.NewCheck("Keep current download timestamp (--no-mtime)", func(bool) {})
	noMtimeCheck.SetChecked(cfg.NoMtime)

	winNamesCheck := widget.NewCheck("Safe NTFS/FAT32 filenames (--windows-filenames)", func(bool) {})
	winNamesCheck.SetChecked(cfg.WindowsFilenames)

	archiveCheck := widget.NewCheck("Enable download deduplication archive (--download-archive)", func(bool) {})
	archiveCheck.SetChecked(cfg.UseArchive)

	accelTab := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Concurrent Threads", fragmentsSelect),
		),
		noMtimeCheck,
		winNamesCheck,
		archiveCheck,
	)

	cookieModeSelect := widget.NewSelect([]string{"none", "browser", "file"}, func(string) {})
	cookieModeSelect.SetSelected(cfg.CookiesMode)

	cookieBrowserSelect := widget.NewSelect(core.SupportedBrowsers, func(string) {})
	cookieBrowserSelect.SetSelected(cfg.CookiesBrowser)

	cookieFileEntry := widget.NewEntry()
	cookieFileEntry.SetText(cfg.CookiesFile)

	cookiesTab := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Cookies Strategy", cookieModeSelect),
			widget.NewFormItem("Import Browser", cookieBrowserSelect),
			widget.NewFormItem("Custom File Path", cookieFileEntry),
		),
	)

	tabs := container.NewAppTabs(
		container.NewTabItem("General", container.NewPadded(generalTab)),
		container.NewTabItem("Codecs", container.NewPadded(codecsTab)),
		container.NewTabItem("Acceleration", container.NewPadded(accelTab)),
		container.NewTabItem("Cookies", container.NewPadded(cookiesTab)),
	)

	d := dialog.NewCustomConfirm("MediaCLI Preferences", "Save Changes", "Cancel", container.NewPadded(tabs), func(save bool) {
		if !save {
			return
		}
		cfg.DownloadDir = strings.TrimSpace(dirEntry.Text)
		cfg.ProxyURL = strings.TrimSpace(proxyEntry.Text)
		if fragmentsSelect.Selected != "" {
			fmt.Sscanf(fragmentsSelect.Selected, "%d", &cfg.ConcurrentFragments)
		}
		cfg.AudioFormat = audioFmtSelect.Selected
		cfg.SubLangs = strings.TrimSpace(subLangsEntry.Text)
		cfg.ThumbnailFormat = thumbFmtSelect.Selected
		cfg.NoMtime = noMtimeCheck.Checked
		cfg.WindowsFilenames = winNamesCheck.Checked
		cfg.UseArchive = archiveCheck.Checked
		cfg.CookiesMode = cookieModeSelect.Selected
		cfg.CookiesBrowser = cookieBrowserSelect.Selected
		cfg.CookiesFile = strings.TrimSpace(cookieFileEntry.Text)

		for _, k := range presetKeys {
			if core.VideoPresets[k].NameEN == videoPresetSelect.Selected {
				cfg.VideoPreset = k
				break
			}
		}

		_ = core.SaveConfig(*cfg)
	}, parent)

	d.Resize(fyne.NewSize(640, 480))
	d.Show()
}