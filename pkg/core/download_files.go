package core

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func FindNewestDownloadedFile(outDir string, before time.Time) string {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return ""
	}
	var bestPath string
	var bestTime time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(outDir, e.Name())
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		mt := info.ModTime()
		if mt.Before(before) {
			continue
		}
		if bestPath == "" || mt.After(bestTime) {
			bestPath = p
			bestTime = mt
		}
	}
	return bestPath
}

func ParseDestinationFromLogs(lines []string) string {
	// Ищет последний "Destination:" из логов yt-dlp
	for i := len(lines) - 1; i >= 0; i-- {
		l := lines[i]
		if strings.Contains(l, "Destination:") {
			parts := strings.SplitN(l, "Destination:", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(l, "Merging formats into") {
			parts := strings.SplitN(l, "Merging formats into", 2)
			if len(parts) == 2 {
				s := strings.TrimSpace(parts[1])
				s = strings.Trim(s, "\"'")
				return s
			}
		}
	}
	return ""
}
