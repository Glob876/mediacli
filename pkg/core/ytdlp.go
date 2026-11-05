package core

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// 7. Сборщики аргументов yt-dlp

func BuildCookieArgs(cfg Config, f map[string]interface{}) []string {
	mode := cfg.CookiesMode
	if f != nil {
		if m := GetString(f, "cookies_mode"); m != "" && m != "default" {
			mode = m
		}
	}

	if mode == "default" {
		mode = cfg.CookiesMode
	}

	switch mode {
	case "none":
		return nil
	case "file":
		cFile := cfg.CookiesFile
		if f != nil {
			if cf := GetString(f, "cookies_file"); cf != "" {
				cFile = cf
			}
		}
		if cFile != "" {
			p := ParseUserPath(cFile)
			if _, err := os.Stat(p); err == nil {
				return []string{"--cookies", p}
			}
			if !filepath.IsAbs(p) {
				home, _ := os.UserHomeDir()
				altP := filepath.Join(home, p)
				if _, err := os.Stat(altP); err == nil {
					return []string{"--cookies", altP}
				}
				altDownloads := filepath.Join(home, "Downloads", p)
				if _, err := os.Stat(altDownloads); err == nil {
					return []string{"--cookies", altDownloads}
				}
			}
			return []string{"--cookies", p}
		}
	case "browser":
		browser := cfg.CookiesBrowser
		if f != nil {
			if cb := GetString(f, "cookies_browser"); cb != "" {
				browser = cb
			}
		}
		if browser == "" {
			browser = "chrome"
		}
		return []string{"--cookies-from-browser", browser}
	}
	return nil
}

func BuildProxyArgs(cfg Config, overrideMode, overrideURL string) []string {
	mode := cfg.ProxyMode
	if overrideMode != "" && overrideMode != "default" {
		mode = overrideMode
	}

	url := cfg.ProxyURL
	if overrideURL != "" {
		url = overrideURL
	}

	if mode == "custom" && strings.TrimSpace(url) != "" {
		return []string{"--proxy", strings.TrimSpace(url)}
	} else if mode == "none" {
		return []string{"--proxy", ""}
	}
	return nil
}

func BuildYtDlpArgs(preset DownloadPreset, cfg Config, outDir string, isPlaylist bool) []string {
	f := preset.Fields
	var cmd []string

	frags := cfg.ConcurrentFragments
	if frags <= 0 {
		frags = 4
	}
	if fragStr := GetString(f, "concurrent_fragments"); fragStr != "" {
		if val, err := strconv.Atoi(fragStr); err == nil {
			frags = val
		}
	}
	cmd = append(cmd, "--concurrent-fragments", strconv.Itoa(frags))

	retries := "10"
	if r := GetString(f, "retries"); r != "" {
		retries = r
	}
	cmd = append(cmd, "--retries", retries, "--fragment-retries", retries)
	cmd = append(cmd, "--buffer-size", "16M")

	// Явный путь к ffmpeg: чинит случай «программа видит ffmpeg, а yt-dlp —
	// нет» (урезанный PATH у GUI/сервисов). yt-dlp сам найдёт ffprobe рядом.
	if bin := strings.TrimSpace(cfg.FfmpegPath); bin != "" {
		cmd = append(cmd, "--ffmpeg-location", ParseUserPath(bin))
	}

	if cfg.NoMtime || GetBool(f, "no_mtime") {
		cmd = append(cmd, "--no-mtime")
	}
	if cfg.WindowsFilenames || GetBool(f, "windows_filenames") {
		cmd = append(cmd, "--windows-filenames")
	}
	if GetBool(f, "restrict_filenames") {
		cmd = append(cmd, "--restrict-filenames")
	}

	if (cfg.UseArchive || GetBool(f, "use_archive")) && cfg.ArchiveFile != "" {
		cmd = append(cmd, "--download-archive", ParseUserPath(cfg.ArchiveFile))
	}

	if GetBool(f, "force_overwrite") {
		cmd = append(cmd, "--force-overwrites")
	}

	if pass := GetString(f, "video_password"); pass != "" {
		cmd = append(cmd, "--video-password", pass)
	}

	if sec := GetString(f, "download_section"); sec != "" {
		if !strings.HasPrefix(sec, "*") {
			sec = "*" + sec
		}
		cmd = append(cmd, "--download-sections", sec)
	}

	if GetBool(f, "audio_only") {
		fmtStr := GetString(f, "audio_format")
		if fmtStr == "" {
			fmtStr = cfg.AudioFormat
		}
		qStr := GetString(f, "audio_quality")
		if qStr == "" {
			qStr = "0"
		}
		cmd = append(cmd, "-x", "--audio-format", fmtStr, "--audio-quality", qStr)
	} else {
		quality := GetString(f, "quality")
		fps := GetString(f, "fps_limit")
		vcodec := GetString(f, "vcodec")

		formatExpr := BuildFormatExpr(quality, fps, vcodec)
		cmd = append(cmd, "-f", formatExpr)
		// Prefer higher resolution and mp4/m4a when yt-dlp has to choose between equal heights
		cmd = append(cmd, "-S", "res,ext:mp4:m4a:webm,codec:h264:vp9:av01")

		vPresetKey := GetString(f, "video_preset")
		if vPresetKey == "" {
			vPresetKey = cfg.VideoPreset
		}
		if vPresetKey == "" {
			vPresetKey = "default"
		}

		if IsExternalTranscodeEnabled(cfg, f) {
			// Внешний ffmpeg: yt-dlp только скачивает и мержит (без --recode), транскод отдельным ffmpeg → паритет байтов/логов
			cmd = append(cmd, "--merge-output-format", "mp4")
		} else if vPresetKey == "custom" {
			ext := GetString(f, "custom_ext")
			if ext == "" {
				ext = "mp4"
			}
			flags := GetString(f, "custom_flags")
			if flags == "" {
				flags = "-c:v libx264 -crf 18 -c:a aac"
			}
			cmd = append(cmd, "--recode-video", strings.TrimPrefix(ext, "."), "--postprocessor-args", "ffmpeg:"+flags)
		} else if p, ok := VideoPresets[vPresetKey]; ok {
			cmd = append(cmd, p.Args...)
		}
	}

	if GetBool(f, "geobypass") {
		cmd = append(cmd, "--geo-bypass")
	}

	cmd = append(cmd, BuildProxyArgs(cfg, GetString(f, "proxy_mode"), GetString(f, "proxy_url"))...)

	if rate := GetString(f, "ratelimit"); rate != "" {
		cmd = append(cmd, "--limit-rate", rate)
	}
	if GetBool(f, "live_start") {
		cmd = append(cmd, "--live-from-start")
	}
	if GetBool(f, "split_chapters") {
		cmd = append(cmd, "--split-chapters")
	}
	if GetBool(f, "write_extra") {
		cmd = append(cmd, "--write-description", "--write-thumbnail")
	}

	sb := GetString(f, "sponsorblock")
	// Совместимость со старым Python-прототипом (sponsors/sponsors_promo).
	if sb == "remove" || sb == "sponsors" || sb == "sponsors_promo" {
		cmd = append(cmd, "--sponsorblock-remove", "sponsor,selfpromo,interaction")
	} else if sb == "mark" {
		cmd = append(cmd, "--sponsorblock-mark", "all")
	}

	if !GetBool(f, "audio_only") {
		if GetBoolDefault(f, "embed_metadata", true) {
			cmd = append(cmd, "--embed-metadata")
		}
		// Обложка — отдельный флаг: её поиск (перебор миниатюр 43…37) и
		// вшивание через mutagen (перезапись всего файла) — самая долгая
		// часть постобработки, отключается без потери видео/аудио.
		if GetBoolDefault(f, "embed_thumbnail", true) {
			cmd = append(cmd, "--embed-thumbnail")
		}
		if GetBoolDefault(f, "embed_chapters", true) {
			cmd = append(cmd, "--embed-chapters")
		}
	}

	if GetBool(f, "subs_enabled") {
		langs := GetString(f, "sub_langs")
		if langs == "" {
			langs = cfg.SubLangs
		}
		cmd = append(cmd, "--write-subs", "--sub-langs", langs)
		if GetBool(f, "auto_subs") {
			cmd = append(cmd, "--write-auto-subs")
		}
		if GetBool(f, "embed_subs") {
			cmd = append(cmd, "--embed-subs")
		}
	}

	template := GetString(f, "output_template")
	if template == "" {
		if isPlaylist {
			template = "%(playlist_title)s/%(playlist_index)03d - %(title)s.%(ext)s"
		} else {
			template = "%(title)s.%(ext)s"
		}
	}

	cmd = append(cmd, "-o", filepath.Join(outDir, template))
	cmd = append(cmd, BuildCookieArgs(cfg, f)...)
	return cmd
}
