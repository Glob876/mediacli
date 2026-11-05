package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// 8. Медиа-утилиты

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
	Required  bool
}

// FFmpegBin возвращает бинарник ffmpeg для дочерних процессов:
// явный путь из настроек (чинит урезанный PATH у GUI/сервисов) или PATH.
func FFmpegBin(cfg Config) string {
	if bin := strings.TrimSpace(cfg.FfmpegPath); bin != "" {
		return ParseUserPath(bin)
	}
	return "ffmpeg"
}

// ResolveFfmpeg проверяет доступность ffmpeg: возвращает путь, первую строку
// `ffmpeg -version` и ошибку. candidate перекрывает конфиг (кнопка Проверить).
func ResolveFfmpeg(cfg Config, candidate string) (bin, version string, err error) {
	bin = FFmpegBin(cfg)
	if c := strings.TrimSpace(candidate); c != "" {
		bin = ParseUserPath(c)
	}
	if filepath.IsAbs(bin) || strings.Contains(bin, string(filepath.Separator)) {
		st, statErr := os.Stat(bin)
		if statErr != nil || st.IsDir() {
			return bin, "", fmt.Errorf("file not found: %s", bin)
		}
		if statErr == nil && st.Mode().Perm()&0111 == 0 {
			return bin, "", fmt.Errorf("not executable: %s", bin)
		}
	} else if p, lookErr := exec.LookPath(bin); lookErr == nil {
		bin = p
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	//nolint:gosec // bin — путь из локальных настроек/проверки пользователя.
	out, runErr := exec.CommandContext(ctx, bin, "-version").CombinedOutput()
	if runErr != nil {
		detail := strings.TrimSpace(string(out))
		if len(detail) > 220 {
			detail = detail[:220] + "…"
		}
		if detail == "" {
			detail = runErr.Error()
		}
		return bin, "", fmt.Errorf("cannot run: %s", detail)
	}
	first, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return bin, first, nil
}

func CheckDependencies() []DependencyStatus {
	required := []string{"yt-dlp", "ffmpeg", "ffprobe"}
	optional := []string{"atomicparsley", "deno"}
	var statuses []DependencyStatus

	for _, b := range required {
		path, err := exec.LookPath(b)
		statuses = append(statuses, DependencyStatus{
			Name:      b,
			Available: err == nil,
			Path:      path,
			Required:  true,
		})
	}
	for _, b := range optional {
		path, err := exec.LookPath(b)
		statuses = append(statuses, DependencyStatus{
			Name:      b,
			Available: err == nil,
			Path:      path,
			Required:  false,
		})
	}
	return statuses
}
