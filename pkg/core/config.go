package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	FirstRunCompleted     bool                   `json:"first_run_completed"`
	DownloadDir           string                 `json:"download_dir"`
	AudioFormat           string                 `json:"audio_format"`
	SubLangs              string                 `json:"sub_langs"`
	Language              string                 `json:"language"`
	UserGoal              string                 `json:"user_goal"`
	CookiesMode           string                 `json:"cookies_mode"`
	CookiesFile           string                 `json:"cookies_file"`
	CookiesBrowser        string                 `json:"cookies_browser"`
	VideoPreset           string                 `json:"video_preset"`
	Theme                 string                 `json:"theme"`
	UseTerminalBG         bool                   `json:"use_terminal_bg"`
	ProxyMode             string                 `json:"proxy_mode"`
	ProxyURL              string                 `json:"proxy_url"`
	ProgressStyle         string                 `json:"progress_style"`
	BGQueueMax            int                    `json:"bg_queue_max"`
	NotifyBell            bool                   `json:"notify_bell"`
	AutoCheckDeps         bool                   `json:"auto_check_deps"`
	DefaultEditor         string                 `json:"default_editor"`
	ConcurrentFragments   int                    `json:"concurrent_fragments"`
	NoMtime               bool                   `json:"no_mtime"`
	WindowsFilenames      bool                   `json:"windows_filenames"`
	ThumbnailFormat       string                 `json:"thumbnail_format"`
	UseArchive            bool                   `json:"use_archive"`
	ArchiveFile           string                 `json:"archive_file"`
	UseFFmpegSuffix       bool                   `json:"use_ffmpeg_suffix"`
	OverwriteOriginal     bool                   `json:"overwrite_original"`
	LogoMode              string                 `json:"logo_mode"`
	LogoProtocol          string                 `json:"logo_protocol"`
	LogoAsciiPreset       string                 `json:"logo_ascii_preset"`
	LogoImagePath         string                 `json:"logo_image_path"`
	DownloadPresets       []DownloadPreset       `json:"download_presets"`
	DefaultDownloadPreset string                 `json:"default_download_preset"`
	PresetDefaults        map[string]interface{} `json:"preset_defaults"`
	TranscodeMode         string                 `json:"transcode_mode"`          // "embedded" (yt-dlp --recode) | "external" (yt-dlp merge + ffmpeg)
	AccentColor           string                 `json:"accent_color"`            // акцент Electron-оболочки, напр. "#bfff00"
	FfmpegPath            string                 `json:"ffmpeg_path"`             // явный путь к ffmpeg (для yt-dlp --ffmpeg-location); пусто = PATH
	ShowHomeLogo          bool                   `json:"show_home_logo"`          // показывать логотип на главном экране Electron
	MediaPresets          []DownloadPreset       `json:"media_presets,omitempty"` // legacy alias, use media_presets.json instead
	FfprobePath           string                 `json:"ffprobe_path,omitempty"`
}

type DownloadPreset struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Fields map[string]interface{} `json:"fields"`
}

var SupportedBrowsers = []string{"chrome", "firefox", "brave", "edge", "opera", "vivaldi", "chromium", "safari"}

func GetConfigDir() string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "mediacli")
		}
		return filepath.Join(home, "AppData", "Roaming", "mediacli")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "mediacli")
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "mediacli")
		}
		return filepath.Join(home, ".config", "mediacli")
	}
}

func GetDefaultConfig() Config {
	home, _ := os.UserHomeDir()
	configDir := GetConfigDir()

	return Config{
		FirstRunCompleted:     false,
		DownloadDir:           filepath.Join(home, "Downloads", "MediaCLI"),
		AudioFormat:           "mp3",
		SubLangs:              "ru,en",
		Language:              "en",
		UserGoal:              "editing",
		CookiesMode:           "none",
		CookiesFile:           filepath.Join(configDir, "cookies.txt"),
		CookiesBrowser:        "chrome",
		VideoPreset:           "default",
		Theme:                 "cyan",
		UseTerminalBG:         true,
		ProxyMode:             "system",
		ProxyURL:              "",
		ProgressStyle:         "blocks",
		BGQueueMax:            3,
		NotifyBell:            true,
		AutoCheckDeps:         true,
		DefaultEditor:         "",
		ConcurrentFragments:   4,
		NoMtime:               true,
		WindowsFilenames:      true,
		ThumbnailFormat:       "png",
		UseArchive:            false,
		ArchiveFile:           filepath.Join(configDir, "archive.txt"),
		UseFFmpegSuffix:       true,
		OverwriteOriginal:     false,
		LogoMode:              "ascii",
		LogoProtocol:          "kitty",
		LogoAsciiPreset:       "standard",
		LogoImagePath:         "",
		DownloadPresets:       []DownloadPreset{},
		DefaultDownloadPreset: "",
		PresetDefaults:        GetInitialPresetFields(),
		TranscodeMode:         "embedded",
		AccentColor:           "#bfff00",
		ShowHomeLogo:          true,
	}
}

func GetInitialPresetFields() map[string]interface{} {
	return map[string]interface{}{
		"quality":              "",
		"video_preset":         "default",
		"vcodec":               "auto",
		"fps_limit":            "",
		"audio_only":           false,
		"audio_format":         "mp3",
		"audio_quality":        "0",
		"download_section":     "",
		"subs_enabled":         false,
		"sub_langs":            "ru,en",
		"auto_subs":            false,
		"embed_subs":           false,
		"embed_metadata":       true,
		"embed_thumbnail":      true,
		"embed_chapters":       true,
		"split_chapters":       false,
		"write_extra":          false,
		"sponsorblock":         "off",
		"concurrent_fragments": "4",
		"retries":              "10",
		"ratelimit":            "",
		"geobypass":            false,
		"live_start":           false,
		"video_password":       "",
		"restrict_filenames":   false,
		"windows_filenames":    true,
		"no_mtime":             true,
		"use_archive":          false,
		"cookies_mode":         "default",
		"cookies_browser":      "",
		"cookies_file":         "",
		"proxy_mode":           "default",
		"proxy_url":            "",
		"output_template":      "",
		"force_overwrite":      false,
	}
}

func ParseUserPath(rawPath string) string {
	s := strings.TrimSpace(rawPath)
	if s == "" {
		return "."
	}
	// Снимаем только одну пару окружающих кавычек, а не все кавычки по краям.
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			s = strings.TrimSpace(s[1 : len(s)-1])
		}
	}
	if strings.HasPrefix(s, "~") {
		home, _ := os.UserHomeDir()
		s = filepath.Join(home, s[1:])
	}
	return filepath.Clean(s)
}

// SplitShellFields разбивает строку флагов с учётом одинарных/двойных кавычек
// (strings.Fields ломал значения с пробелами внутри кавычек).
func SplitShellFields(raw string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range raw {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

// MaskCmdForLog скрывает секреты (--video-password, --proxy) в строке,
// которая показывается в UI/логах/history. Исполняется реальный cmdList.
func MaskCmdForLog(cmd []string) []string {
	masked := append([]string{}, cmd...)
	for i := 0; i < len(masked); i++ {
		if masked[i] == "--video-password" && i+1 < len(masked) {
			masked[i+1] = "***"
			i++
		} else if masked[i] == "--proxy" && i+1 < len(masked) {
			if strings.TrimSpace(masked[i+1]) != "" {
				masked[i+1] = "***"
			}
			i++
		}
	}
	return masked
}

func LoadConfig() (Config, error) {
	cfg := GetDefaultConfig()
	configPath := filepath.Join(GetConfigDir(), "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

func SaveConfig(cfg Config) error {
	dir := GetConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0644)
}
