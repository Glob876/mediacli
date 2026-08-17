package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 1. Конфигурация и пути (Config & Paths)
// ============================================================================

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
	DownloadPresets       []DownloadPreset       `json:"download_presets"`
	DefaultDownloadPreset string                 `json:"default_download_preset"`
	PresetDefaults        map[string]interface{} `json:"preset_defaults"`
}

type DownloadPreset struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Fields map[string]interface{} `json:"fields"`
}

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
		CookiesBrowser:        "",
		VideoPreset:           "standard_mp4",
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
		DownloadPresets:       []DownloadPreset{},
		DefaultDownloadPreset: "",
		PresetDefaults:        make(map[string]interface{}),
	}
}

func ParseUserPath(rawPath string) string {
	s := strings.TrimSpace(rawPath)
	if s == "" {
		return "."
	}
	s = strings.Trim(s, `"'`)
	if strings.HasPrefix(s, "~") {
		home, _ := os.UserHomeDir()
		s = filepath.Join(home, s[1:])
	}
	return filepath.Clean(s)
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

// ============================================================================
// 2. Пресеты конвертации и видео (ПЕРВЫЕ ДВА ВСЕГДА MP4 и MKV)
// ============================================================================

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

// Порядок пресетов скачивания: 1. Standard MP4, 2. Modern MKV (AV1), далее остальные
var OrderedVideoPresetKeys = []string{
	"standard_mp4",
	"mkv_av1",
	"hevc_mp4",
	"davinci_dnxhr",
	"davinci_prores",
	"davinci_h264",
	"webm_vp9",
	"audio_mp3",
	"audio_flac",
	"default",
	"custom",
}

var VideoPresets = map[string]VideoPreset{
	"standard_mp4": {
		ID:     "standard_mp4",
		NameEN: "Standard MP4 (H.264 + AAC) [Universal]",
		NameRU: "Стандартный MP4 (H.264 + AAC) [Универсальный]",
		DescEN: "Universal MP4 container with H.264 video and AAC audio.",
		DescRU: "Универсальный MP4 с H.264 видео и AAC аудио. Максимальная совместимость.",
		Args:   []string{"--recode-video", "mp4", "--postprocessor-args", "ffmpeg:-c:v libx264 -pix_fmt yuv420p -c:a aac -b:a 192k"},
	},
	"mkv_av1": {
		ID:     "mkv_av1",
		NameEN: "Modern MKV (AV1 + Opus/AAC) [Next-Gen]",
		NameRU: "Современный MKV (AV1 + Opus/AAC) [Новое поколение]",
		DescEN: "Next-generation AV1 video codec in Matroska MKV container. Ultra high efficiency.",
		DescRU: "Видеокодек нового поколения AV1 в контейнере MKV. Высочайшая эффективность сжатия.",
		Args:   []string{"--recode-video", "mkv", "--postprocessor-args", "ffmpeg:-c:v libsvtav1 -crf 26 -c:a copy"},
	},
	"hevc_mp4": {
		ID:     "hevc_mp4",
		NameEN: "High Efficiency MP4 (H.265 / HEVC + AAC)",
		NameRU: "Высокоэффективный MP4 (H.265 / HEVC + AAC)",
		DescEN: "H.265/HEVC video codec for smaller file sizes.",
		DescRU: "Видеокодек H.265/HEVC для меньшего размера файла.",
		Args:   []string{"--recode-video", "mp4", "--postprocessor-args", "ffmpeg:-c:v libx265 -crf 24 -c:a aac -b:a 192k"},
	},
	"davinci_dnxhr": {
		ID:     "davinci_dnxhr",
		NameEN: "DaVinci Resolve: DNxHR HQ + PCM Audio (.mov)",
		NameRU: "DaVinci Resolve: DNxHR HQ + PCM Аудио (.mov)",
		DescEN: "Converts video to Avid DNxHR HQ with PCM 16-bit audio.",
		DescRU: "Конвертирует в Avid DNxHR HQ с PCM 16-бит аудио для монтажа.",
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
		Args:   []string{"--recode-video", "mov", "--postprocessor-args", "ffmpeg:-c:v libx264 -pix_fmt yuv420p -c:a pcm_s16le"},
	},
	"webm_vp9": {
		ID:     "webm_vp9",
		NameEN: "WebM (VP9 + Opus)",
		NameRU: "WebM (VP9 + Opus)",
		DescEN: "Open WebM media format with VP9 video and Opus audio.",
		DescRU: "Открытый веб-формат WebM с видео VP9 и аудио Opus.",
		Args:   []string{"--recode-video", "webm", "--postprocessor-args", "ffmpeg:-c:v libvpx-vp9 -c:a libopus"},
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
	"default": {
		ID:     "default",
		NameEN: "Original (yt-dlp auto merge)",
		NameRU: "Исходный (авто-объединение yt-dlp)",
		DescEN: "Merges best streams without re-encoding video.",
		DescRU: "Объединяет наилучшие потоки без перекодирования.",
		Args:   []string{"--merge-output-format", "mp4"},
	},
}

// Пресеты локальной конвертации FFmpeg: Первые два — MP4 и MKV
var ConvertPresets = []ConvertPreset{
	{
		ID:          "standard_mp4",
		NameEN:      "1. Standard MP4 (H.264 + AAC)",
		NameRU:      "1. Стандартный MP4 (H.264 + AAC)",
		Ext:         "mp4",
		Suffix:      "_mp4",
		FFmpegFlags: []string{"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "192k"},
	},
	{
		ID:          "modern_mkv",
		NameEN:      "2. Modern MKV (AV1 / High-Efficiency)",
		NameRU:      "2. Современный MKV (AV1 / Высокое сжатие)",
		Ext:         "mkv",
		Suffix:      "_av1",
		FFmpegFlags: []string{"-c:v", "libsvtav1", "-crf", "26", "-preset", "6", "-c:a", "aac", "-b:a", "192k"},
	},
	{
		ID:          "hevc_mp4",
		NameEN:      "3. High Efficiency MP4 (H.265 / HEVC)",
		NameRU:      "3. Высокоэффективный MP4 (H.265 / HEVC)",
		Ext:         "mp4",
		Suffix:      "_hevc",
		FFmpegFlags: []string{"-c:v", "libx265", "-crf", "24", "-c:a", "aac", "-b:a", "192k"},
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

// ============================================================================
// 3. История операций (History)
// ============================================================================

type HistoryEntry struct {
	Time   string `json:"time"`
	Type   string `json:"type"`
	Source string `json:"source"`
	Target string `json:"target"`
	Status string `json:"status"`
}

func AddHistoryEntry(opType, source, target, status string) error {
	dir := GetConfigDir()
	_ = os.MkdirAll(dir, 0755)
	historyPath := filepath.Join(dir, "history.json")

	var entries []HistoryEntry
	if data, err := os.ReadFile(historyPath); err == nil {
		_ = json.Unmarshal(data, &entries)
	}

	entry := HistoryEntry{
		Time:   time.Now().Format("2006-01-02 15:04:05"),
		Type:   opType,
		Source: source,
		Target: target,
		Status: status,
	}

	entries = append([]HistoryEntry{entry}, entries...)
	if len(entries) > 50 {
		entries = entries[:50]
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(historyPath, data, 0644)
}

func GetHistory() []HistoryEntry {
	historyPath := filepath.Join(GetConfigDir(), "history.json")
	var entries []HistoryEntry
	if data, err := os.ReadFile(historyPath); err == nil {
		_ = json.Unmarshal(data, &entries)
	}
	return entries
}

func ClearHistory() error {
	historyPath := filepath.Join(GetConfigDir(), "history.json")
	return os.WriteFile(historyPath, []byte("[]"), 0644)
}

// ============================================================================
// 4. Cookies парсер (Cookies)
// ============================================================================

type Cookie struct {
	Domain string
	Name   string
	Value  string
}

func ParseCookiesFile(path string) ([]Cookie, error) {
	p := ParseUserPath(path)
	file, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cookies []Cookie
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) == 7 {
			cookies = append(cookies, Cookie{
				Domain: parts[0],
				Name:   parts[5],
				Value:  parts[6],
			})
		}
	}
	return cookies, scanner.Err()
}

func AppendCookie(filePath, domain, name, val string) error {
	p := ParseUserPath(filePath)
	_ = os.MkdirAll(filepath.Dir(p), 0755)

	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	exp := strconv.FormatInt(time.Now().Unix()+3600*24*365, 10)
	flag := "FALSE"
	if strings.HasPrefix(domain, ".") {
		flag = "TRUE"
	}
	line := fmt.Sprintf("%s\t%s\t/\tFALSE\t%s\t%s\t%s\n", domain, flag, exp, name, val)
	_, err = f.WriteString(line)
	return err
}

// ============================================================================
// 5. Детектор этапов и прогресса (Dynamic Stage & Progress Parser)
// ============================================================================

type StagePattern struct {
	Regex     *regexp.Regexp
	Formatter func(matches []string) string
}

var (
	rePercent = regexp.MustCompile(`(\d+(?:\.\d+)?)%`)
	reSpeed   = regexp.MustCompile(`(?:at|speed=)\s*([^\s]+)`)

	stagePatterns = []StagePattern{
		{
			Regex: regexp.MustCompile(`\[(youtube|vk|twitch|generic|bilibili|soundcloud)[^\]]*\]\s*([^:]+):\s*(Downloading.*|Extracting.*)`),
			Formatter: func(m []string) string {
				return fmt.Sprintf("[%s] %s", strings.ToUpper(m[1]), m[3])
			},
		},
		{
			Regex: regexp.MustCompile(`\[SponsorBlock\]\s*(.*)`),
			Formatter: func(m []string) string { return fmt.Sprintf("[SponsorBlock] %s", m[1]) },
		},
		{
			Regex: regexp.MustCompile(`\[Merger\]\s*(.*)`),
			Formatter: func(m []string) string { return fmt.Sprintf("[Merger] %s", m[1]) },
		},
		{
			Regex: regexp.MustCompile(`\[VideoConvertor\]\s*(.*)`),
			Formatter: func(m []string) string { return fmt.Sprintf("[Convert] %s", m[1]) },
		},
		{
			Regex: regexp.MustCompile(`\[ExtractAudio\]\s*(.*)`),
			Formatter: func(m []string) string { return fmt.Sprintf("[AudioExtract] %s", m[1]) },
		},
		{
			Regex: regexp.MustCompile(`\[ThumbnailsConvertor\]\s*(.*)`),
			Formatter: func(m []string) string { return fmt.Sprintf("[ThumbConvert] %s", m[1]) },
		},
		{
			Regex: regexp.MustCompile(`\[download\]\s+Destination:\s*(.+)`),
			Formatter: func(m []string) string { return fmt.Sprintf("[Download] Saving %s", filepath.Base(m[1])) },
		},
		{
			Regex: regexp.MustCompile(`\[download\]\s+(\d+(?:\.\d+)?%).*at\s+([^\s]+)\s+ETA\s+([^\s]+)`),
			Formatter: func(m []string) string {
				return fmt.Sprintf("[Download] %s (Speed: %s, ETA: %s)", m[1], m[2], m[3])
			},
		},
		{
			Regex: regexp.MustCompile(`frame=\s*(\d+)\s+fps=\s*(\d+).*time=([^\s]+).*speed=\s*([^\s]+)`),
			Formatter: func(m []string) string {
				return fmt.Sprintf("[FFmpeg] Frame %s (%s fps, Time: %s, Speed: %s)", m[1], m[2], m[3], m[4])
			},
		},
	}
)

func DetectStage(line, currentStage string) string {
	s := strings.TrimSpace(line)
	if s == "" {
		return currentStage
	}

	for _, p := range stagePatterns {
		if match := p.Regex.FindStringSubmatch(s); len(match) > 0 {
			return p.Formatter(match)
		}
	}

	if strings.HasPrefix(s, "ERROR:") {
		msg := strings.TrimSpace(s[6:])
		if len(msg) > 70 {
			msg = msg[:70]
		}
		return "[Error] " + msg
	}
	if strings.HasPrefix(s, "WARNING:") {
		msg := strings.TrimSpace(s[8:])
		if len(msg) > 70 {
			msg = msg[:70]
		}
		return "[Warning] " + msg
	}

	return currentStage
}

func ExtractProgress(line string) (pct float64, speed string, ok bool) {
	if m := rePercent.FindStringSubmatch(line); len(m) > 1 {
		if p, err := strconv.ParseFloat(m[1], 64); err == nil {
			pct = p
			ok = true
		}
	}
	if m := reSpeed.FindStringSubmatch(line); len(m) > 1 {
		speed = m[1]
	}
	return
}

// ============================================================================
// 6. Менеджер фоновых задач (Background Queue Manager)
// ============================================================================

type TaskStatus string

const (
	StatusQueued    TaskStatus = "queued"
	StatusRunning   TaskStatus = "running"
	StatusDone      TaskStatus = "done"
	StatusFailed    TaskStatus = "failed"
	StatusCancelled TaskStatus = "cancelled"
)

type BackgroundTask struct {
	ID         int
	Cmd        []string
	Title      string
	Source     string
	Target     string
	Status     TaskStatus
	Stage      string
	Progress   float64
	LogLines   []string
	StartedAt  time.Time
	FinishedAt time.Time
	ExitCode   int

	cmdObj *exec.Cmd
}

type BackgroundQueueManager struct {
	tasks    []*BackgroundTask
	maxTasks int
	nextID   int
	mu       sync.Mutex
}

var GlobalQueue = NewBackgroundQueueManager(3)

func NewBackgroundQueueManager(maxTasks int) *BackgroundQueueManager {
	qm := &BackgroundQueueManager{
		tasks:    make([]*BackgroundTask, 0),
		maxTasks: maxTasks,
		nextID:   1,
	}
	go qm.workerLoop()
	return qm
}

func (qm *BackgroundQueueManager) Enqueue(cmd []string, title, source, target string) *BackgroundTask {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	task := &BackgroundTask{
		ID:        qm.nextID,
		Cmd:       cmd,
		Title:     title,
		Source:    source,
		Target:    target,
		Status:    StatusQueued,
		Stage:     "Queued...",
		LogLines:  make([]string, 0),
		StartedAt: time.Now(),
	}
	qm.nextID++
	qm.tasks = append(qm.tasks, task)
	return task
}

func (qm *BackgroundQueueManager) AdoptRunning(cmdObj *exec.Cmd, cmd []string, title, source, target string, logs []string) *BackgroundTask {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	task := &BackgroundTask{
		ID:        qm.nextID,
		Cmd:       cmd,
		Title:     title,
		Source:    source,
		Target:    target,
		Status:    StatusRunning,
		Stage:     "Running...",
		LogLines:  append([]string{}, logs...),
		StartedAt: time.Now(),
		cmdObj:    cmdObj,
	}
	qm.nextID++
	qm.tasks = append(qm.tasks, task)
	return task
}

func (qm *BackgroundQueueManager) CancelTask(taskID int) bool {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	for _, t := range qm.tasks {
		if t.ID == taskID {
			if t.Status == StatusRunning && t.cmdObj != nil && t.cmdObj.Process != nil {
				_ = t.cmdObj.Process.Kill()
			}
			t.Status = StatusCancelled
			t.FinishedAt = time.Now()
			return true
		}
	}
	return false
}

func (qm *BackgroundQueueManager) GetTasks() []*BackgroundTask {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	result := make([]*BackgroundTask, len(qm.tasks))
	copy(result, qm.tasks)
	return result
}

func (qm *BackgroundQueueManager) GetSummary() string {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	var running, queued []*BackgroundTask
	for _, t := range qm.tasks {
		if t.Status == StatusRunning {
			running = append(running, t)
		} else if t.Status == StatusQueued {
			queued = append(queued, t)
		}
	}

	total := len(running) + len(queued)
	if total == 0 {
		return ""
	}
	if len(running) > 0 {
		return fmt.Sprintf("[⏬ BG: %d active (%.1f%%)]", total, running[0].Progress)
	}
	return fmt.Sprintf("[⏬ BG: %d queued]", total)
}

func (qm *BackgroundQueueManager) workerLoop() {
	for {
		time.Sleep(500 * time.Millisecond)

		qm.mu.Lock()
		var runningCount int
		var nextTask *BackgroundTask

		for _, t := range qm.tasks {
			if t.Status == StatusRunning {
				runningCount++
			}
			if t.Status == StatusQueued && nextTask == nil {
				nextTask = t
			}
		}

		if runningCount < 1 && nextTask != nil {
			nextTask.Status = StatusRunning
			nextTask.cmdObj = exec.Command(nextTask.Cmd[0], nextTask.Cmd[1:]...)
			go qm.monitorTask(nextTask)
		}
		qm.mu.Unlock()
	}
}

func (qm *BackgroundQueueManager) monitorTask(task *BackgroundTask) {
	stdout, err := task.cmdObj.StdoutPipe()
	if err != nil {
		qm.finishTask(task, -1, err.Error())
		return
	}
	task.cmdObj.Stderr = task.cmdObj.Stdout

	if err := task.cmdObj.Start(); err != nil {
		qm.finishTask(task, -1, err.Error())
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		qm.mu.Lock()
		task.LogLines = append(task.LogLines, text)
		task.Stage = DetectStage(text, task.Stage)
		if pct, _, ok := ExtractProgress(text); ok {
			task.Progress = pct
		}
		qm.mu.Unlock()
	}

	exitCode := 0
	if err := task.cmdObj.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	qm.finishTask(task, exitCode, "")
}

func (qm *BackgroundQueueManager) finishTask(task *BackgroundTask, exitCode int, errStr string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	task.ExitCode = exitCode
	task.FinishedAt = time.Now()
	if exitCode == 0 {
		task.Status = StatusDone
		_ = AddHistoryEntry(task.Title, task.Source, task.Target, "Success")
	} else {
		task.Status = StatusFailed
		if errStr != "" {
			task.LogLines = append(task.LogLines, "[Error] "+errStr)
		}
		_ = AddHistoryEntry(task.Title, task.Source, task.Target, fmt.Sprintf("Failed (%d)", exitCode))
	}
}

// ============================================================================
// 7. Сборщики аргументов yt-dlp (С ускорением и защитой)
// ============================================================================

func BuildCookieArgs(cfg Config) []string {
	switch cfg.CookiesMode {
	case "file":
		if cfg.CookiesFile != "" {
			p := ParseUserPath(cfg.CookiesFile)
			if _, err := os.Stat(p); err == nil {
				return []string{"--cookies", p}
			}
		}
	case "browser":
		if cfg.CookiesBrowser != "" {
			return []string{"--cookies-from-browser", cfg.CookiesBrowser}
		}
	}
	return nil
}

func BuildProxyArgs(cfg Config, overrideMode, overrideURL string) []string {
	mode := cfg.ProxyMode
	if overrideMode != "" {
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

	// 1. Ускорение загрузки и стабильность сети
	frags := cfg.ConcurrentFragments
	if frags <= 0 {
		frags = 4
	}
	if fragStr := getString(f, "concurrent_fragments"); fragStr != "" {
		if val, err := strconv.Atoi(fragStr); err == nil {
			frags = val
		}
	}
	cmd = append(cmd, "--concurrent-fragments", strconv.Itoa(frags))
	cmd = append(cmd, "--retries", "10", "--fragment-retries", "10")
	cmd = append(cmd, "--buffer-size", "16M")
	cmd = append(cmd, "--extractor-args", "youtube:player_client=android,web")

	// 2. Системные флаги файлов
	if cfg.NoMtime || getBool(f, "no_mtime") {
		cmd = append(cmd, "--no-mtime")
	}
	if cfg.WindowsFilenames || getBool(f, "windows_filenames") {
		cmd = append(cmd, "--windows-filenames")
	}
	if getBool(f, "restrict_filenames") {
		cmd = append(cmd, "--restrict-filenames")
	}

	// 3. Архив скачиваний (защита от повторов)
	if cfg.UseArchive && cfg.ArchiveFile != "" {
		cmd = append(cmd, "--download-archive", ParseUserPath(cfg.ArchiveFile))
	}

	// 4. Форматы видео и аудио
	if getBool(f, "audio_only") {
		fmtStr := getString(f, "audio_format")
		if fmtStr == "" {
			fmtStr = cfg.AudioFormat
		}
		qStr := getString(f, "audio_quality")
		if qStr == "" {
			qStr = "0"
		}
		cmd = append(cmd, "-x", "--audio-format", fmtStr, "--audio-quality", qStr)
	} else {
		quality := getString(f, "quality")
		fps := getString(f, "fps_limit")
		fpsSuffix := ""
		if fps != "" {
			fpsSuffix = fmt.Sprintf("[fps<=%s]", fps)
		}
		qStr := ""
		if quality != "" {
			qStr = fmt.Sprintf("[height<=%s]", quality)
		}

		vcodec := getString(f, "vcodec")
		vcFilter := ""
		switch vcodec {
		case "av1":
			vcFilter = "[vcodec^=av01]"
		case "vp9":
			vcFilter = "[vcodec^=vp9]"
		case "h264":
			vcFilter = "[vcodec^=avc1]"
		}

		formatExpr := fmt.Sprintf("bestvideo%s%s%s+bestaudio/best%s%s", qStr, fpsSuffix, vcFilter, qStr, fpsSuffix)
		cmd = append(cmd, "-f", formatExpr)

		vPresetKey := getString(f, "video_preset")
		if vPresetKey == "" {
			vPresetKey = cfg.VideoPreset
		}

		if vPresetKey == "custom" {
			ext := getString(f, "custom_ext")
			if ext == "" {
				ext = "mp4"
			}
			flags := getString(f, "custom_flags")
			if flags == "" {
				flags = "-c:v libx264 -c:a aac"
			}
			cmd = append(cmd, "--recode-video", strings.TrimPrefix(ext, "."), "--postprocessor-args", "ffmpeg:"+flags)
		} else if p, ok := VideoPresets[vPresetKey]; ok {
			cmd = append(cmd, p.Args...)
		}
	}

	if getBool(f, "geobypass") {
		cmd = append(cmd, "--geo-bypass")
	}

	cmd = append(cmd, BuildProxyArgs(cfg, getString(f, "proxy_mode"), getString(f, "proxy_url"))...)

	if rate := getString(f, "ratelimit"); rate != "" {
		cmd = append(cmd, "--limit-rate", rate)
	}
	if getBool(f, "live_start") {
		cmd = append(cmd, "--live-from-start")
	}
	if getBool(f, "split_chapters") {
		cmd = append(cmd, "--split-chapters")
	}
	if getBool(f, "write_extra") {
		cmd = append(cmd, "--write-description", "--write-thumbnail")
	}

	sb := getString(f, "sponsorblock")
	if sb != "" && sb != "off" {
		cats := "sponsor"
		if sb == "sponsors_promo" {
			cats = "sponsor,selfpromo,interaction"
		}
		cmd = append(cmd, "--sponsorblock-remove", cats)
	}

	if !getBool(f, "audio_only") {
		if getBoolDefault(f, "embed_metadata", true) {
			cmd = append(cmd, "--embed-metadata", "--embed-thumbnail")
		}
		if getBoolDefault(f, "embed_chapters", true) {
			cmd = append(cmd, "--embed-chapters")
		}
	}

	if getBool(f, "subs_enabled") {
		langs := getString(f, "sub_langs")
		if langs == "" {
			langs = cfg.SubLangs
		}
		cmd = append(cmd, "--write-subs", "--sub-langs", langs)
		if getBool(f, "auto_subs") {
			cmd = append(cmd, "--write-auto-subs")
		}
		if getBool(f, "embed_subs") {
			cmd = append(cmd, "--embed-subs")
		}
	}

	template := getString(f, "output_template")
	if template == "" {
		if isPlaylist {
			template = "%(playlist_title)s/%(playlist_index)03d - %(title)s.%(ext)s"
		} else {
			template = "%(title)s.%(ext)s"
		}
	}

	cmd = append(cmd, "-o", filepath.Join(outDir, template))
	cmd = append(cmd, BuildCookieArgs(cfg)...)
	return cmd
}

// ============================================================================
// 8. Медиа-утилиты (FFprobe, Проверка зависимостей)
// ============================================================================

type MediaProbeResult struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"streams"`
}

func ProbeMedia(filePath string) (*MediaProbeResult, error) {
	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", filePath)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var res MediaProbeResult
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

type DependencyStatus struct {
	Name      string
	Available bool
	Path      string
}

func CheckDependencies() []DependencyStatus {
	bins := []string{"yt-dlp", "ffmpeg", "ffprobe", "atomicparsley", "deno"}
	var statuses []DependencyStatus

	for _, b := range bins {
		path, err := exec.LookPath(b)
		statuses = append(statuses, DependencyStatus{
			Name:      b,
			Available: err == nil,
			Path:      path,
		})
	}
	return statuses
}

// ============================================================================
// Вспомогательные функции для чтения map[string]interface{}
// ============================================================================

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func getBoolDefault(m map[string]interface{}, key string, def bool) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}