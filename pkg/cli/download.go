package cli

import (
	"encoding/json"
	"fmt"
	"mediacli/pkg/core"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func (cli *app) download(command string, args []string) error {
	flags := cli.flags(command)
	output := flags.String("output-dir", cli.cfg.DownloadDir, "Destination directory")
	presetID := flags.String("preset", "", "Saved download preset ID or built-in video profile ID")
	playlist := flags.Bool("playlist", false, "Download a playlist")
	audio := flags.Bool("audio-only", false, "Download/extract audio")
	audioFormat := flags.String("audio-format", "", "Audio format (mp3, flac, opus, ...)")
	format := flags.String("format", cli.cfg.ThumbnailFormat, "Thumbnail format: png, jpg or webp")
	transcode := flags.String("transcode", cli.cfg.TranscodeMode, "embedded or external conversion")
	fieldsJSON := flags.String("fields", "", "Download fields JSON or @FILE (all manual TUI options)")
	var fieldsList stringList
	flags.Var(&fieldsList, "set", "Override download field KEY=VALUE; repeatable, VALUE may be JSON")
	// Frequently used options also have direct flags.
	overrides := map[string]any{}
	defaults := core.GetInitialPresetFields()
	for key, value := range cli.cfg.PresetDefaults {
		defaults[key] = value
	}
	for _, name := range []string{"quality", "vcodec", "fps_limit", "download_section", "output_template", "cookies_mode", "cookies_browser", "cookies_file", "proxy_mode", "proxy_url", "sub_langs", "sponsorblock", "ratelimit", "retries", "concurrent_fragments"} {
		flags.Var(&operationFlag{name: name, kind: reflectString, values: overrides}, strings.ReplaceAll(name, "_", "-"), "Download field "+name)
	}
	for _, name := range []string{"subs_enabled", "auto_subs", "embed_subs", "embed_metadata", "embed_thumbnail", "embed_chapters", "split_chapters", "write_extra", "use_archive", "force_overwrite"} {
		flags.Var(&operationFlag{name: name, kind: reflectBool, values: overrides}, strings.ReplaceAll(name, "_", "-"), "Download field "+name)
	}
	if err := parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() == 0 {
		return usage(command + " requires at least one URL")
	}
	cancel, err := cli.setup()
	if err != nil {
		return err
	}
	defer cancel()
	if *transcode != "embedded" && *transcode != "external" {
		return usage("--transcode must be embedded or external")
	}
	cfg := cli.cfg
	cfg.TranscodeMode = *transcode
	if *presetID != "" {
		found := false
		for _, preset := range cfg.DownloadPresets {
			if preset.ID == *presetID {
				for key, value := range preset.Fields {
					defaults[key] = value
				}
				found = true
				break
			}
		}
		if !found {
			if _, ok := core.VideoPresets[*presetID]; !ok {
				return usage("unknown download preset: " + *presetID)
			}
			defaults["video_preset"] = *presetID
		}
	}
	if *fieldsJSON != "" {
		data := []byte(*fieldsJSON)
		if strings.HasPrefix(*fieldsJSON, "@") {
			data, err = cli.read((*fieldsJSON)[1:])
			if err != nil {
				return err
			}
		}
		var fields map[string]any
		if err := json.Unmarshal(data, &fields); err != nil {
			return usage(err.Error())
		}
		for key, value := range fields {
			defaults[key] = value
		}
	}
	for _, definition := range fieldsList {
		key, raw, ok := strings.Cut(definition, "=")
		if !ok {
			return usage("--set requires KEY=VALUE")
		}
		defaults[key] = parseValue(raw)
	}
	for key, value := range overrides {
		defaults[key] = value
	}
	if *audio {
		defaults["audio_only"] = true
	}
	if *audioFormat != "" {
		defaults["audio_format"] = *audioFormat
	}
	outputDir := core.ParseUserPath(*output)
	preset := core.DownloadPreset{ID: "cli", Name: "CLI", Fields: defaults}
	external := core.IsExternalTranscodeEnabled(cfg, preset.Fields)
	if cli.server != "" && external {
		return usage("external download conversion requires local execution; use --transcode embedded with --server")
	}
	if cli.server != "" && command == "thumbnail" {
		return usage("thumbnail download uses local execution; omit --server")
	}
	var commands [][]string
	for _, address := range flags.Args() {
		parsed, err := url.Parse(address)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return usage("URL must start with http:// or https://")
		}
		var commandLine []string
		if command == "thumbnail" {
			if *format != "png" && *format != "jpg" && *format != "webp" {
				return usage("thumbnail format must be png, jpg or webp")
			}
			commandLine = []string{"yt-dlp", "--skip-download", "--write-thumbnail", "--convert-thumbnails", *format, "-P", outputDir}
			commandLine = append(commandLine, core.BuildCookieArgs(cfg, preset.Fields)...)
			commandLine = append(commandLine, core.BuildProxyArgs(cfg, core.GetString(defaults, "proxy_mode"), core.GetString(defaults, "proxy_url"))...)
			if cfg.FfmpegPath != "" {
				commandLine = append(commandLine, "--ffmpeg-location", core.FFmpegBin(cfg))
			}
		} else {
			commandLine = append([]string{"yt-dlp"}, core.BuildYtDlpArgs(preset, cfg, outputDir, *playlist)...)
			commandLine = append(commandLine, "--newline", "--print", "after_move:MEDIACLI_FILE=%(filepath)j")
		}
		commandLine = append(commandLine, "--", address)
		commands = append(commands, commandLine)
	}
	if cli.server != "" {
		return cli.remoteDownloads(flags.Args(), outputDir, *playlist, defaults)
	}
	if cli.dry {
		if external {
			return cli.emit(map[string]any{"commands": maskedCommands(commands), "external_conversion": "After each download, FFmpeg converts the exact reported output files using the selected profile"})
		}
		return cli.execute(commands, "Download", strings.Join(flags.Args(), ","), nil)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	if !external {
		return cli.execute(commands, "Download", strings.Join(flags.Args(), ","), nil)
	}
	// Execute each download and use yt-dlp's explicit after_move path, including playlist entries.
	for index, commandLine := range commands {
		task := core.GlobalQueue.Enqueue(commandLine, "Download", flags.Arg(index), outputDir)
		if err := cli.waitLocal([]int{task.ID}); err != nil {
			return err
		}
		var paths []string
		seen := map[string]bool{}
		for _, line := range core.GlobalQueue.TaskLogs(task.ID) {
			if raw, ok := strings.CutPrefix(line, "MEDIACLI_FILE="); ok {
				var path string
				if json.Unmarshal([]byte(raw), &path) == nil && !seen[path] {
					seen[path] = true
					paths = append(paths, path)
				}
			}
		}
		if len(paths) == 0 {
			return fmt.Errorf("yt-dlp did not report an output file for external conversion")
		}
		profile := core.GetString(defaults, "video_preset")
		if profile == "" {
			profile = cfg.VideoPreset
		}
		extension, fflags, ok := core.GetExternalFFmpegPlan(profile, defaults)
		if !ok {
			return fmt.Errorf("no external conversion plan for %s", profile)
		}
		for _, path := range paths {
			plan := core.PrepareFFmpegOutput(path, extension, "_converted", cfg)
			if plan.OnComplete == nil {
				if err := checkOutput(plan.TempOutputPath, []string{path}, false); err != nil {
					return err
				}
			}
			ffcommand := []string{core.FFmpegBin(cfg), "-n", "-i", path}
			ffcommand = append(ffcommand, fflags...)
			ffcommand = append(ffcommand, plan.TempOutputPath)
			if err := cli.execute([][]string{ffcommand}, "Download conversion", filepath.Base(path), plan.OnComplete); err != nil {
				return err
			}
		}
	}
	return nil
}

func maskedCommands(commands [][]string) [][]string {
	masked := make([][]string, len(commands))
	for index, command := range commands {
		masked[index] = core.MaskCmdForLog(command)
	}
	return masked
}
func parseValue(raw string) any {
	var value any
	if json.Unmarshal([]byte(raw), &value) == nil {
		return value
	}
	return raw
}
