package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mediacli/pkg/core"

	"github.com/gdamore/tcell/v2"
)

// ScreenUniversal is the keyboard editor opened with Alt+3.
func ScreenUniversal(s tcell.Screen, cfg *core.Config, initialPath string) {
	tr := func(ru, en string) string {
		if strings.HasPrefix(cfg.Language, "ru") {
			return ru
		}
		return en
	}
	path := strings.TrimSpace(initialPath)
	var ops []core.Operation
	for {
		if path == "" {
			input, ok := TextInput(s, cfg, tr("Редактор Alt+3", "Alt+3 editor"), tr("Путь к видео или аудио:", "Video or audio file path:"), "", T(*cfg, "footer_input"))
			if !ok {
				return
			}
			candidate := core.ParseUserPath(strings.TrimSpace(input))
			if st, err := os.Stat(candidate); err != nil || st.IsDir() {
				ShowMessage(s, cfg, tr("Файл не найден", "File not found"), []string{candidate}, T(*cfg, "footer_message"))
				continue
			}
			path, ops = candidate, nil
		}
		info, _ := core.ProbeMediaInfoWithConfig(path, *cfg)
		summary := filepath.Base(path)
		if info != nil {
			summary += fmt.Sprintf(" · %s s · %d streams", info.Format.Duration, len(info.Streams))
		}
		for n, op := range ops {
			summary += fmt.Sprintf("\n%d. %s", n+1, op.Describe())
		}
		items := []string{
			tr("Обрезать длину", "Trim duration"),
			tr("Соотношение сторон: обрезать / поля", "Aspect ratio: crop / bars"),
			tr("Громкость / нормализация", "Volume / normalization"),
			tr("Видеофильтр / аудиофильтр", "Video / audio filter"),
			tr("Кодек и контейнер", "Codec and container"),
			tr("Показать команду", "Show command"),
			tr("Экспортировать", "Export"),
			tr("Убрать последнее действие", "Remove last action"),
			tr("Другой файл", "Choose another file"),
			tr("Расширенный Pipeline", "Advanced pipeline"),
			tr("Закрыть", "Close"),
		}
		choice := RunMenu(s, cfg, tr("Редактор Alt+3", "Alt+3 editor"), items, summary, T(*cfg, "footer_nav"))
		if choice < 0 || choice == 10 {
			return
		}
		switch choice {
		case 0:
			start, ok := TextInput(s, cfg, T(*cfg, "trim_title"), T(*cfg, "trim_prompt_start"), "00:00:00", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			end, ok := TextInput(s, cfg, T(*cfg, "trim_title"), T(*cfg, "trim_prompt_end"), "", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			mode := "reencode"
			if RunMenu(s, cfg, T(*cfg, "trim_title"), []string{tr("Точно (перекодировать)", "Accurate (re-encode)"), tr("Быстро (ключевые кадры)", "Fast (keyframes)")}, "", T(*cfg, "footer_nav")) == 1 {
				mode = "copy"
			}
			op := &core.TrimOp{Start: strings.TrimSpace(start), End: strings.TrimSpace(end), Mode: mode}
			if err := op.Validate(info); err != nil {
				ShowMessage(s, cfg, "Trim", []string{err.Error()}, T(*cfg, "footer_message"))
				continue
			}
			ops = replaceEditorOp(ops, op)
		case 1:
			ratio, ok := TextInput(s, cfg, tr("Соотношение сторон", "Aspect ratio"), "W:H (16:9, 9:16, 1:1, 4:5):", "9:16", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			parts := strings.Split(ratio, ":")
			if len(parts) != 2 {
				ShowMessage(s, cfg, "Aspect", []string{"Use W:H"}, T(*cfg, "footer_message"))
				continue
			}
			w, we := strconv.Atoi(strings.TrimSpace(parts[0]))
			h, he := strconv.Atoi(strings.TrimSpace(parts[1]))
			if we != nil || he != nil || w < 1 || h < 1 || w > 100 || h > 100 {
				ShowMessage(s, cfg, "Aspect", []string{"Invalid W:H"}, T(*cfg, "footer_message"))
				continue
			}
			modes := []string{tr("Без полей (обрезать)", "Fill (crop)"), tr("Без потерь (с полями)", "Fit (bars)"), tr("Размытые поля", "Blurred bars"), tr("Растянуть", "Stretch")}
			mi := RunMenu(s, cfg, "Aspect", modes, "", T(*cfg, "footer_nav"))
			if mi < 0 {
				continue
			}
			mode := []string{"crop", "letterbox", "blur", "stretch"}[mi]
			op := &core.AspectOp{RatioW: w, RatioH: h, Mode: mode, SizePreset: "1080p"}
			if err := op.Validate(info); err != nil {
				ShowMessage(s, cfg, "Aspect", []string{err.Error()}, T(*cfg, "footer_message"))
				continue
			}
			ops = replaceEditorOp(ops, op)
		case 2:
			value, ok := TextInput(s, cfg, "Audio", tr("Громкость: 1.0 исходная, 0.5 половина:", "Volume: 1.0 original, 0.5 half:"), "1.0", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			volume, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil || volume < 0 || volume > 10 {
				ShowMessage(s, cfg, "Audio", []string{"Invalid volume (0–10)"}, T(*cfg, "footer_message"))
				continue
			}
			norm := RunMenu(s, cfg, "Loudnorm", []string{tr("Нет", "No"), tr("Да", "Yes")}, "", T(*cfg, "footer_nav")) == 1
			ops = replaceEditorOp(ops, &core.AudioOp{Volume: &volume, Normalize: norm})
		case 3:
			vf, ok := TextInput(s, cfg, "Video filter", "VF (e.g. hflip):", "", T(*cfg, "footer_input"))
			if !ok {
				continue
			}
			af, ok := TextInput(s, cfg, "Audio filter", "AF (e.g. volume=0.8):", "", T(*cfg, "footer_input"))
			if ok && (vf != "" || af != "") {
				ops = append(ops, &core.FilterOp{VF: strings.TrimSpace(vf), AF: strings.TrimSpace(af)})
			}
		case 4:
			presets := []string{"H.264 MP4", "H.265 MP4", "AV1 MKV", "Remux MKV"}
			index := RunMenu(s, cfg, "Transcode", presets, "", T(*cfg, "footer_nav"))
			if index < 0 {
				continue
			}
			options := []*core.TranscodeOp{{Container: "mp4", VCodec: "libx264", ACodec: "aac", CRF: "20", Preset: "medium"}, {Container: "mp4", VCodec: "libx265", ACodec: "aac", CRF: "23", Preset: "medium"}, {Container: "mkv", VCodec: "libsvtav1", ACodec: "aac", CRF: "24", Preset: "5"}, {Container: "mkv", Remux: true}}
			ops = replaceEditorOp(ops, options[index])
		case 5, 6:
			if len(ops) == 0 {
				ShowMessage(s, cfg, "Editor", []string{tr("Добавьте действие", "Add an action")}, T(*cfg, "footer_message"))
				continue
			}
			pipe := core.Pipeline{Inputs: []string{path}, Operations: ops}
			commands, err := core.BuildPipeline(pipe, *cfg)
			if err != nil {
				ShowMessage(s, cfg, "Editor", []string{err.Error()}, T(*cfg, "footer_message"))
				continue
			}
			if choice == 5 {
				var lines []string
				for _, command := range commands {
					lines = append(lines, strings.Join(core.MaskCmdForLog(command), " "))
				}
				ShowMessage(s, cfg, "FFmpeg", lines, T(*cfg, "footer_message"))
			} else {
				target := commands[len(commands)-1][len(commands[len(commands)-1])-1]
				if len(commands) == 1 {
					RunWithLogHook(s, cfg, commands[0], "Editor", filepath.Base(path), filepath.Base(target), nil)
				} else {
					core.GlobalQueue.EnqueueChain(commands, "Editor", path, target, fmt.Sprintf("editor_%s", filepath.Base(path)))
					ShowMessage(s, cfg, "Editor", []string{tr("Экспорт поставлен в очередь", "Export queued")}, T(*cfg, "footer_message"))
				}
			}
		case 7:
			if len(ops) > 0 {
				ops = ops[:len(ops)-1]
			}
		case 8:
			path, ops = "", nil
		case 9:
			ScreenPipeline(s, cfg)
		}
	}
}

func replaceEditorOp(ops []core.Operation, next core.Operation) []core.Operation {
	filtered := make([]core.Operation, 0, len(ops)+1)
	for _, op := range ops {
		if op.Type() != next.Type() {
			filtered = append(filtered, op)
		}
	}
	if next.Type() == "trim" {
		return append([]core.Operation{next}, filtered...)
	}
	return append(filtered, next)
}
