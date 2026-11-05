package core

import (
	"strings"
)

// 2. Пресеты конвертации и видео

type VideoPreset struct {
	ID     string
	NameEN string
	NameRU string
	DescEN string
	DescRU string
	Args   []string
}

type ConvertPreset struct {
	ID          string
	NameEN      string
	NameRU      string
	DescEN      string
	DescRU      string
	Ext         string
	Suffix      string
	FFmpegFlags []string
}

var OrderedVideoPresetKeys = []string{
	"default",
	"standard_mp4",
	"mkv_av1",
	"hevc_mp4",
	"davinci_dnxhr",
	"davinci_prores",
	"davinci_h264",
	"webm_vp9",
	"audio_mp3",
	"audio_flac",
}

var VideoPresets = map[string]VideoPreset{
	"default": {
		ID:     "default",
		NameEN: "Original / Lossless Merge (.mp4) [Best Quality]",
		NameRU: "Исходное качество без пережатия (.mp4) [Максимальное]",
		DescEN: "Directly merges highest bitrate source video and audio into MP4 without generation loss.",
		DescRU: "Объединяет наилучшие видео и аудио потоки в MP4 без потери качества и перекодирования.",
		Args:   []string{"--merge-output-format", "mp4"},
	},
	"standard_mp4": {
		ID:     "standard_mp4",
		NameEN: "Re-encode MP4 (H.264 High Quality + AAC)",
		NameRU: "Перекодировать в MP4 (H.264 HQ + AAC)",
		DescEN: "Universal MP4 container re-encoded with H.264 (CRF 18) and AAC audio.",
		DescRU: "Перекодирует в универсальный MP4 (H.264 с высоким качеством CRF 18 и AAC 192k).",
		Args:   []string{"--recode-video", "mp4", "--postprocessor-args", "ffmpeg:-c:v libx264 -crf 18 -preset medium -pix_fmt yuv420p -c:a aac -b:a 192k"},
	},
	"mkv_av1": {
		ID:     "mkv_av1",
		NameEN: "Modern MKV (AV1 + Opus/AAC) [Next-Gen]",
		NameRU: "Современный MKV (AV1 + Opus/AAC) [Новое поколение]",
		DescEN: "Next-generation AV1 video codec in Matroska MKV container. Ultra high efficiency.",
		DescRU: "Видеокодек нового поколения AV1 в контейнере MKV. Высочайшая эффективность сжатия.",
		Args:   []string{"--recode-video", "mkv", "--postprocessor-args", "ffmpeg:-c:v libsvtav1 -crf 24 -preset 5 -c:a copy"},
	},
	"hevc_mp4": {
		ID:     "hevc_mp4",
		NameEN: "High Efficiency MP4 (H.265 / HEVC + AAC)",
		NameRU: "Высокоэффективный MP4 (H.265 / HEVC + AAC)",
		DescEN: "H.265/HEVC video codec for smaller file sizes with high quality.",
		DescRU: "Видеокодек H.265/HEVC (CRF 22) для компактного размера при высоком качестве.",
		Args:   []string{"--recode-video", "mp4", "--postprocessor-args", "ffmpeg:-c:v libx265 -crf 22 -preset medium -c:a aac -b:a 192k"},
	},
	"davinci_dnxhr": {
		ID:     "davinci_dnxhr",
		NameEN: "DaVinci Resolve: DNxHR HQ + PCM Audio (.mov)",
		NameRU: "DaVinci Resolve: DNxHR HQ + PCM Аудио (.mov)",
		DescEN: "Converts video to Avid DNxHR HQ with PCM 16-bit audio.",
		DescRU: "Конвертирует в Avid DNxHR HQ с PCM 16-бит аудио для монтажа без задержек.",
		Args:   []string{"--recode-video", "mov", "--postprocessor-args", "ffmpeg:-c:v dnxhd -profile:v dnxhr_hq -c:a pcm_s16le"},
	},
	"davinci_prores": {
		ID:     "davinci_prores",
		NameEN: "DaVinci Resolve: Apple ProRes 422 + PCM Audio (.mov)",
		NameRU: "DaVinci Resolve: Apple ProRes 422 + PCM Аудио (.mov)",
		DescEN: "Apple ProRes HQ video codec with uncompressed PCM audio in MOV container.",
		DescRU: "Видеокодек Apple ProRes HQ с несжатым PCM аудио в MOV контейнере.",
		Args:   []string{"--recode-video", "mov", "--postprocessor-args", "ffmpeg:-c:v prores_ks -profile:v 3 -c:a pcm_s16le"},
	},
	"davinci_h264": {
		ID:     "davinci_h264",
		NameEN: "DaVinci Resolve: H.264 + PCM Audio (.mov)",
		NameRU: "DaVinci Resolve: H.264 + PCM Аудио (.mov)",
		DescEN: "H.264 video with PCM audio in MOV container.",
		DescRU: "Видео H.264 с PCM аудио в MOV контейнере.",
		Args:   []string{"--recode-video", "mov", "--postprocessor-args", "ffmpeg:-c:v libx264 -crf 18 -pix_fmt yuv420p -c:a pcm_s16le"},
	},
	"webm_vp9": {
		ID:     "webm_vp9",
		NameEN: "WebM (VP9 + Opus)",
		NameRU: "WebM (VP9 + Opus)",
		DescEN: "Open WebM media format with VP9 video and Opus audio.",
		DescRU: "Открытый веб-формат WebM с видео VP9 и аудио Opus.",
		Args:   []string{"--recode-video", "webm", "--postprocessor-args", "ffmpeg:-c:v libvpx-vp9 -crf 24 -b:v 0 -c:a libopus"},
	},
	"audio_mp3": {
		ID:     "audio_mp3",
		NameEN: "Audio Only: MP3 320 kbps",
		NameRU: "Только аудио: MP3 320 кбит/с",
		DescEN: "Extracts audio stream into high quality 320 kbps MP3 format.",
		DescRU: "Извлекает звуковую дорожку в MP3 320 кбит/с.",
		Args:   []string{"-x", "--audio-format", "mp3", "--audio-quality", "0"},
	},
	"audio_flac": {
		ID:     "audio_flac",
		NameEN: "Audio Only: FLAC Lossless",
		NameRU: "Только аудио: FLAC без потерь",
		DescEN: "Extracts audio stream into lossless FLAC audio format.",
		DescRU: "Извлекает звуковую дорожку в сжатый формат без потерь FLAC.",
		Args:   []string{"-x", "--audio-format", "flac"},
	},
}

var ConvertPresets = []ConvertPreset{
	{
		ID:          "standard_mp4",
		NameEN:      "1. Standard MP4 (H.264 + AAC)",
		NameRU:      "1. Стандартный MP4 (H.264 + AAC)",
		Ext:         "mp4",
		Suffix:      "_mp4",
		FFmpegFlags: []string{"-c:v", "libx264", "-crf", "18", "-preset", "medium", "-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "192k"},
	},
	{
		ID:          "modern_mkv",
		NameEN:      "2. Modern MKV (AV1 / High-Efficiency)",
		NameRU:      "2. Современный MKV (AV1 / Высокое сжатие)",
		Ext:         "mkv",
		Suffix:      "_av1",
		FFmpegFlags: []string{"-c:v", "libsvtav1", "-crf", "24", "-preset", "5", "-c:a", "aac", "-b:a", "192k"},
	},
	{
		ID:          "hevc_mp4",
		NameEN:      "3. High Efficiency MP4 (H.265 / HEVC)",
		NameRU:      "3. Высокоэффективный MP4 (H.265 / HEVC)",
		Ext:         "mp4",
		Suffix:      "_hevc",
		FFmpegFlags: []string{"-c:v", "libx265", "-crf", "22", "-preset", "medium", "-c:a", "aac", "-b:a", "192k"},
	},
	{
		ID:          "davinci_dnxhr_hq",
		NameEN:      "4. DaVinci Resolve: DNxHR HQ + PCM (.mov)",
		NameRU:      "4. DaVinci Resolve: DNxHR HQ + PCM (.mov)",
		Ext:         "mov",
		Suffix:      "_dnxhr",
		FFmpegFlags: []string{"-c:v", "dnxhd", "-profile:v", "dnxhr_hq", "-c:a", "pcm_s16le"},
	},
	{
		ID:          "davinci_prores",
		NameEN:      "5. DaVinci Resolve: Apple ProRes 422 + PCM (.mov)",
		NameRU:      "5. DaVinci Resolve: Apple ProRes 422 + PCM (.mov)",
		Ext:         "mov",
		Suffix:      "_prores",
		FFmpegFlags: []string{"-c:v", "prores_ks", "-profile:v", "3", "-c:a", "pcm_s16le"},
	},
	{
		ID:          "fast_compress",
		NameEN:      "6. Fast Video Compress (H.264 CRF 28)",
		NameRU:      "6. Быстрое сжатие видео (H.264 CRF 28)",
		Ext:         "mp4",
		Suffix:      "_compressed",
		FFmpegFlags: []string{"-c:v", "libx264", "-crf", "28", "-preset", "faster", "-c:a", "aac", "-b:a", "128k"},
	},
	{
		ID:          "gif_anim",
		NameEN:      "7. High-Quality Animated GIF (.gif)",
		NameRU:      "7. Высококачественная GIF-анимация (.gif)",
		Ext:         "gif",
		Suffix:      "_anim",
		FFmpegFlags: []string{"-vf", "fps=15,scale=480:-1:flags=lanczos,split[s0][s1];[s0]palettegen[p];[s1][p]paletteuse"},
	},
	{
		ID:          "audio_mp3",
		NameEN:      "8. Extract Audio: MP3 320 kbps (.mp3)",
		NameRU:      "8. Извлечь аудио: MP3 320 кбит/с (.mp3)",
		Ext:         "mp3",
		Suffix:      "_audio",
		FFmpegFlags: []string{"-vn", "-ab", "320k"},
	},
	{
		ID:          "audio_flac",
		NameEN:      "9. Extract Audio: FLAC Lossless (.flac)",
		NameRU:      "9. Извлечь аудио: FLAC без потерь (.flac)",
		Ext:         "flac",
		Suffix:      "_lossless",
		FFmpegFlags: []string{"-vn", "-c:a", "flac"},
	},
	{
		ID:          "audio_wav",
		NameEN:      "10. Extract Audio: Uncompressed WAV (.wav)",
		NameRU:      "10. Извлечь аудио: WAV без сжатия (.wav)",
		Ext:         "wav",
		Suffix:      "_audio",
		FFmpegFlags: []string{"-vn", "-acodec", "pcm_s16le"},
	},
}

// 2b. Внешний FFmpeg транскод (отдельно от yt-dlp) — паритет с --recode-video

func IsExternalTranscodeEnabled(cfg Config, fields map[string]interface{}) bool {
	if cfg.TranscodeMode != "external" {
		return false
	}
	if GetBool(fields, "audio_only") {
		return false
	}
	presetID := GetString(fields, "video_preset")
	if presetID == "" {
		presetID = cfg.VideoPreset
	}
	if presetID == "" || presetID == "default" {
		return false
	}
	if presetID == "custom" {
		return true
	}
	if p, ok := VideoPresets[presetID]; ok {
		for _, a := range p.Args {
			if a == "--recode-video" {
				return true
			}
		}
	}
	return false
}

func GetExternalFFmpegPlan(presetID string, fields map[string]interface{}) (ext string, flags []string, ok bool) {
	if presetID == "" {
		return "", nil, false
	}
	if presetID == "custom" {
		ext = GetString(fields, "custom_ext")
		if ext == "" {
			ext = "mp4"
		}
		ext = strings.TrimPrefix(strings.TrimSpace(ext), ".")
		raw := GetString(fields, "custom_flags")
		if raw == "" {
			raw = "-c:v libx264 -crf 18 -preset medium -pix_fmt yuv420p -c:a aac -b:a 192k"
		}
		return ext, SplitShellFields(raw), true
	}
	p, exists := VideoPresets[presetID]
	if !exists {
		return "", nil, false
	}
	// default и audio пресеты не транскодируют — пропускаем
	if len(p.Args) == 2 && p.Args[0] == "--merge-output-format" {
		return "", nil, false
	}
	if p.Args[0] == "-x" {
		return "", nil, false
	}
	var recodeExt string
	var ffmpegRaw string
	for i, a := range p.Args {
		if a == "--recode-video" && i+1 < len(p.Args) {
			recodeExt = strings.TrimPrefix(p.Args[i+1], ".")
		}
		if a == "--postprocessor-args" && i+1 < len(p.Args) {
			v := p.Args[i+1]
			if strings.HasPrefix(v, "ffmpeg:") {
				ffmpegRaw = strings.TrimPrefix(v, "ffmpeg:")
			} else {
				ffmpegRaw = v
			}
		}
	}
	if recodeExt == "" {
		recodeExt = "mp4"
	}
	if ffmpegRaw == "" {
		return "", nil, false
	}
	return recodeExt, SplitShellFields(ffmpegRaw), true
}
