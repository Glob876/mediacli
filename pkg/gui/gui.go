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
// 1. Пастельно-зеленая тема (Pastel Sage & Matcha Theme)
// ============================================================================

type pastelGreenTheme struct{}

func (m *pastelGreenTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.RGBA{R: 17, G: 22, B: 20, A: 255}
	case theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground:
		return color.RGBA{R: 24, G: 32, B: 28, A: 255}
	case theme.ColorNameButton:
		return color.RGBA{R: 35, G: 48, B: 41, A: 255}
	case theme.ColorNamePrimary:
		return color.RGBA{R: 123, G: 198, B: 154, A: 255}
	case theme.ColorNameHover:
		return color.RGBA{R: 48, G: 66, B: 56, A: 255}
	case theme.ColorNameInputBackground:
		return color.RGBA{R: 22, G: 29, B: 25, A: 255}
	case theme.ColorNameForeground:
		return color.RGBA{R: 232, G: 239, B: 234, A: 255}
	case theme.ColorNamePlaceHolder:
		return color.RGBA{R: 120, G: 145, B: 130, A: 255}
	case theme.ColorNameSeparator:
		return color.RGBA{R: 38, G: 50, B: 44, A: 255}
	default:
		return theme.DefaultTheme().Color(name, theme.VariantDark)
	}
}

func (m *pastelGreenTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m *pastelGreenTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *pastelGreenTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 12.0
	case theme.SizeNameInnerPadding:
		return 12.0
	case theme.SizeNameInputRadius:
		return 10.0
	default:
		return theme.DefaultTheme().Size(name)
	}
}

// ============================================================================
// 2. Карточка загрузки
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
// 3. Главный интерфейс GUI с анимацией
// ============================================================================

func RunGUI() {
	a := app.New()
	a.Settings().SetTheme(&pastelGreenTheme{})

	w := a.NewWindow("MediaCLI — Pastel Media Suite")
	w.Resize(fyne.NewSize(1140, 750))
	w.CenterOnScreen()

	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.GetDefaultConfig()
	}

	cardsGrid := container.New(layout.NewGridLayout(3))
	scrollContainer := container.NewVScroll(container.NewPadded(cardsGrid))

	reloadCards := func() {
		cardsGrid.Objects = nil
		historyEntries := core.GetHistory()
		for _, h := range historyEntries {
			card := createHistoryCard(w, h, &cfg, func() {
				cardsGrid.Objects = nil
				for _, updatedH := range core.GetHistory() {
					cardsGrid.Add(createHistoryCard(w, updatedH, &cfg, nil))
				}
				cardsGrid.Refresh()
			})
			cardsGrid.Add(card)
		}
		cardsGrid.Refresh()
	}

	reloadCards()

	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("Paste media URL (YouTube, VK, Twitch, etc.) and press Enter...")

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
	timeRangeEntry.SetPlaceHolder("Time range cut (e.g. 00:01:00-00:03:30 or leave empty)")

	subsCheck := widget.NewCheck("Download Subtitles (ru,en)", func(bool) {})
	sponsorCheck := widget.NewCheck("SponsorBlock (Cut Sponsorships)", func(bool) {})

	var confirmPanel *fyne.Container
	var confirmBg *canvas.Rectangle
	confirmExpanded := false

	animateConfirmDrawer := func(expand bool) {
		if expand == confirmExpanded {
			return
		}
		confirmExpanded = expand

		if expand {
			confirmPanel.Show()
		}

		anim := fyne.NewAnimation(240*time.Millisecond, func(p float32) {
			if !expand {
				p = 1.0 - p
			}
			alpha := uint8(p * 255)
			confirmBg.FillColor = color.RGBA{R: 24, G: 32, B: 28, A: alpha}
			confirmBg.StrokeColor = color.RGBA{R: 123, G: 198, B: 154, A: uint8(p * 180)}
			confirmBg.Refresh()
		})
		anim.Curve = fyne.AnimationEaseOut
		anim.Start()

		if !expand {
			time.AfterFunc(250*time.Millisecond, func() {
				if !confirmExpanded {
					confirmPanel.Hide()
				}
			})
		}
	}

	startDownloadAction := func() {
		targetURL := strings.TrimSpace(urlEntry.Text)
		if targetURL == "" {
			return
		}

		animateConfirmDrawer(false)

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
		animateConfirmDrawer(false)
	})

	btnStart := widget.NewButtonWithIcon("Start Download", theme.ConfirmIcon(), startDownloadAction)
	btnStart.Importance = widget.HighImportance

	confirmBg = canvas.NewRectangle(color.RGBA{R: 24, G: 32, B: 28, A: 255})
	confirmBg.StrokeColor = color.RGBA{R: 123, G: 198, B: 154, A: 180}
	confirmBg.StrokeWidth = 1.2
	confirmBg.CornerRadius = 10.0

	confirmContent := container.NewVBox(
		container.NewGridWithColumns(2,
			container.NewVBox(widget.NewLabelWithStyle("Target Codec Preset:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), presetSelect),
			container.NewVBox(widget.NewLabelWithStyle("Max Quality:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), qualitySelect),
		),
		timeRangeEntry,
		container.NewHBox(subsCheck, layout.NewSpacer(), sponsorCheck, layout.NewSpacer()),
		widget.NewSeparator(),
		container.NewHBox(
			layout.NewSpacer(),
			btnCancel,
			layout.NewSpacer(),
			btnStart,
		),
	)

	confirmPanel = container.NewStack(
		confirmBg,
		container.NewPadded(confirmContent),
	)
	confirmPanel.Hide()

	urlEntry.OnSubmitted = func(s string) {
		if strings.TrimSpace(s) == "" {
			return
		}
		animateConfirmDrawer(true)
	}

	var settingsDrawer *fyne.Container
	var settingsBg *canvas.Rectangle
	settingsExpanded := false

	settingsFormContainer := buildFullSettingsView(&cfg, func() {
		_ = core.SaveConfig(cfg)
	})

	btnSettingsClose := widget.NewButtonWithIcon("Close Settings", theme.CancelIcon(), func() {
		if settingsExpanded {
			settingsExpanded = false
			anim := fyne.NewAnimation(220*time.Millisecond, func(p float32) {
				alpha := uint8((1.0 - p) * 255)
				settingsBg.FillColor = color.RGBA{R: 20, G: 26, B: 23, A: alpha}
				settingsBg.Refresh()
			})
			anim.Curve = fyne.AnimationEaseOut
			anim.Start()
			time.AfterFunc(230*time.Millisecond, func() {
				settingsDrawer.Hide()
			})
		}
	})

	settingsHeader := container.NewBorder(
		nil, nil,
		widget.NewLabelWithStyle("MediaCLI Preferences", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		btnSettingsClose,
	)

	settingsBg = canvas.NewRectangle(color.RGBA{R: 20, G: 26, B: 23, A: 255})
	settingsBg.StrokeColor = color.RGBA{R: 123, G: 198, B: 154, A: 200}
	settingsBg.StrokeWidth = 1.2
	settingsBg.CornerRadius = 12.0

	settingsContent := container.NewBorder(
		container.NewVBox(settingsHeader, widget.NewSeparator()),
		nil, nil, nil,
		container.NewVScroll(settingsFormContainer),
	)

	settingsDrawer = container.NewStack(
		settingsBg,
		container.NewPadded(settingsContent),
	)
	settingsDrawer.Hide()

	animateSettingsDrawer := func(expand bool) {
		if expand == settingsExpanded {
			return
		}
		settingsExpanded = expand

		if expand {
			settingsDrawer.Show()
		}

		anim := fyne.NewAnimation(240*time.Millisecond, func(p float32) {
			if !expand {
				p = 1.0 - p
			}
			alpha := uint8(p * 255)
			settingsBg.FillColor = color.RGBA{R: 20, G: 26, B: 23, A: alpha}
			settingsBg.StrokeColor = color.RGBA{R: 123, G: 198, B: 154, A: uint8(p * 200)}
			settingsBg.Refresh()
		})
		anim.Curve = fyne.AnimationEaseOut
		anim.Start()

		if !expand {
			time.AfterFunc(250*time.Millisecond, func() {
				if !settingsExpanded {
					settingsDrawer.Hide()
				}
			})
		}
	}

	btnSettings := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), func() {
		animateSettingsDrawer(!settingsExpanded)
	})

	btnPaste := widget.NewButtonWithIcon("Paste URL", theme.ContentPasteIcon(), func() {
		if cb := w.Clipboard().Content(); cb != "" {
			urlEntry.SetText(cb)
			animateConfirmDrawer(true)
		}
	})

	btnGo := widget.NewButtonWithIcon("Configure", theme.DownloadIcon(), func() {
		if strings.TrimSpace(urlEntry.Text) != "" {
			animateConfirmDrawer(true)
		}
	})
	btnGo.Importance = widget.HighImportance

	leftToolbar := container.NewHBox(btnSettings, layout.NewSpacer())
	rightToolbar := container.NewHBox(layout.NewSpacer(), btnPaste, layout.NewSpacer(), btnGo)

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

	mainContent := container.NewBorder(
		headerContainer,
		nil, nil, nil,
		scrollContainer,
	)

	rootStack := container.NewStack(
		mainContent,
		container.NewPadded(settingsDrawer),
	)

	w.SetContent(rootStack)
	w.ShowAndRun()
}

// ============================================================================
// 4. Карточка активной задачи
// ============================================================================

func createActiveDownloadCard(url string) *DownloadCard {
	card := &DownloadCard{
		URL:    url,
		Status: "Initializing...",
	}

	cardBg := canvas.NewRectangle(color.RGBA{R: 22, G: 29, B: 25, A: 255})
	cardBg.StrokeColor = color.RGBA{R: 123, G: 198, B: 154, A: 220}
	cardBg.StrokeWidth = 1.5
	cardBg.CornerRadius = 12.0

	thumbBg := canvas.NewRectangle(color.RGBA{R: 30, G: 40, B: 34, A: 255})
	thumbBg.SetMinSize(fyne.NewSize(300, 135))
	thumbBg.CornerRadius = 8.0

	mediaIcon := widget.NewIcon(theme.MediaPlayIcon())
	thumbStack := container.NewStack(thumbBg, container.NewCenter(mediaIcon))

	titleLabel := widget.NewLabelWithStyle(url, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	titleLabel.Truncation = fyne.TextTruncateEllipsis
	card.TitleLabel = titleLabel

	stageLabel := widget.NewLabel("Starting engine...")
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
// 5. Карточка истории
// ============================================================================

func createHistoryCard(parent fyne.Window, entry core.HistoryEntry, cfg *core.Config, onDeleted func()) fyne.CanvasObject {
	cardBg := canvas.NewRectangle(color.RGBA{R: 20, G: 26, B: 23, A: 255})
	cardBg.StrokeColor = color.RGBA{R: 38, G: 50, B: 44, A: 255}
	cardBg.StrokeWidth = 1.0
	cardBg.CornerRadius = 12.0

	thumbBg := canvas.NewRectangle(color.RGBA{R: 28, G: 36, B: 31, A: 255})
	thumbBg.SetMinSize(fyne.NewSize(300, 135))
	thumbBg.CornerRadius = 8.0

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

	btnDelete := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		showDeleteDialog(parent, entry, cfg, onDeleted)
	})

	bottomRow := container.NewBorder(nil, nil, nil, btnDelete, infoLabel)

	cardInner := container.NewVBox(
		thumbStack,
		titleLabel,
		bottomRow,
	)

	return container.NewPadded(container.NewStack(
		cardBg,
		container.NewPadded(cardInner),
	))
}

func showDeleteDialog(parent fyne.Window, entry core.HistoryEntry, cfg *core.Config, onDeleted func()) {
	var deleteChoice *widget.RadioGroup
	deleteChoice = widget.NewRadioGroup([]string{
		"1. Remove from history only (Keep file on disk)",
		"2. Delete file from disk & remove from history",
	}, func(string) {})
	deleteChoice.SetSelected("1. Remove from history only (Keep file on disk)")

	messageLabel := widget.NewLabel(fmt.Sprintf("Target: %s\nSelect deletion action:", entry.Target))
	messageLabel.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(
		messageLabel,
		widget.NewSeparator(),
		deleteChoice,
	)

	d := dialog.NewCustomConfirm("Delete Item", "Confirm Delete", "Cancel", container.NewPadded(content), func(confirm bool) {
		if !confirm {
			return
		}
		deleteFromDisk := (deleteChoice.Selected == "2. Delete file from disk & remove from history")
		_ = core.DeleteHistoryItem(entry, deleteFromDisk, cfg.DownloadDir)
		if onDeleted != nil {
			onDeleted()
		}
	}, parent)

	d.Resize(fyne.NewSize(540, 270))
	d.Show()
}

// ============================================================================
// 6. Фоновый исполнитель загрузки
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
// 7. Полнофункциональная боковая панель настроек
// ============================================================================

func buildFullSettingsView(cfg *core.Config, onSave func()) fyne.CanvasObject {
	dirEntry := widget.NewEntry()
	dirEntry.SetText(cfg.DownloadDir)

	proxyEntry := widget.NewEntry()
	proxyEntry.SetText(cfg.ProxyURL)

	langSelect := widget.NewSelect([]string{"en", "ru"}, func(s string) {
		cfg.Language = s
	})
	langSelect.SetSelected(cfg.Language)

	generalTab := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Download Directory", dirEntry),
			widget.NewFormItem("Network Proxy URL", proxyEntry),
			widget.NewFormItem("Interface Language", langSelect),
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

	noMtimeCheck := widget.NewCheck("Keep current download timestamp (--no-mtime)", func(b bool) {
		cfg.NoMtime = b
	})
	noMtimeCheck.SetChecked(cfg.NoMtime)

	winNamesCheck := widget.NewCheck("Safe NTFS/FAT32 filenames (--windows-filenames)", func(b bool) {
		cfg.WindowsFilenames = b
	})
	winNamesCheck.SetChecked(cfg.WindowsFilenames)

	archiveCheck := widget.NewCheck("Enable download deduplication archive (--download-archive)", func(b bool) {
		cfg.UseArchive = b
	})
	archiveCheck.SetChecked(cfg.UseArchive)

	accelTab := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Concurrent Threads", fragmentsSelect),
		),
		noMtimeCheck,
		winNamesCheck,
		archiveCheck,
	)

	cookieModeSelect := widget.NewSelect([]string{"none", "browser", "file"}, func(s string) {
		cfg.CookiesMode = s
	})
	cookieModeSelect.SetSelected(cfg.CookiesMode)

	cookieBrowserSelect := widget.NewSelect(core.SupportedBrowsers, func(s string) {
		cfg.CookiesBrowser = s
	})
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

	btnSave := widget.NewButtonWithIcon("Save All Preferences", theme.ConfirmIcon(), func() {
		cfg.DownloadDir = strings.TrimSpace(dirEntry.Text)
		cfg.ProxyURL = strings.TrimSpace(proxyEntry.Text)
		if fragmentsSelect.Selected != "" {
			fmt.Sscanf(fragmentsSelect.Selected, "%d", &cfg.ConcurrentFragments)
		}
		cfg.AudioFormat = audioFmtSelect.Selected
		cfg.SubLangs = strings.TrimSpace(subLangsEntry.Text)
		cfg.ThumbnailFormat = thumbFmtSelect.Selected
		cfg.CookiesFile = strings.TrimSpace(cookieFileEntry.Text)

		for _, k := range presetKeys {
			if core.VideoPresets[k].NameEN == videoPresetSelect.Selected {
				cfg.VideoPreset = k
				break
			}
		}

		if onSave != nil {
			onSave()
		}
	})
	btnSave.Importance = widget.HighImportance

	tabs := container.NewAppTabs(
		container.NewTabItem("General", container.NewPadded(generalTab)),
		container.NewTabItem("Codecs", container.NewPadded(codecsTab)),
		container.NewTabItem("Acceleration", container.NewPadded(accelTab)),
		container.NewTabItem("Cookies", container.NewPadded(cookiesTab)),
	)

	return container.NewVBox(
		tabs,
		widget.NewSeparator(),
		container.NewHBox(layout.NewSpacer(), btnSave, layout.NewSpacer()),
	)
}