package core

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// 5. Детектор этапов и прогресса

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
			Regex:     regexp.MustCompile(`\[SponsorBlock\]\s*(.*)`),
			Formatter: func(m []string) string { return fmt.Sprintf("[SponsorBlock] %s", m[1]) },
		},
		{
			Regex:     regexp.MustCompile(`\[Merger\]\s*(.*)`),
			Formatter: func(m []string) string { return fmt.Sprintf("[Merger] %s", m[1]) },
		},
		{
			Regex:     regexp.MustCompile(`\[VideoConvertor\]\s*(.*)`),
			Formatter: func(m []string) string { return fmt.Sprintf("[Convert] %s", m[1]) },
		},
		{
			Regex:     regexp.MustCompile(`\[ExtractAudio\]\s*(.*)`),
			Formatter: func(m []string) string { return fmt.Sprintf("[AudioExtract] %s", m[1]) },
		},
		{
			Regex:     regexp.MustCompile(`\[ThumbnailsConvertor\]\s*(.*)`),
			Formatter: func(m []string) string { return fmt.Sprintf("[ThumbConvert] %s", m[1]) },
		},
		{
			Regex:     regexp.MustCompile(`\[download\]\s+Destination:\s*(.+)`),
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
