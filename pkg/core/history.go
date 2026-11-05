package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 3. История операций и функции удаления

type HistoryEntry struct {
	Time       string `json:"time"`
	Type       string `json:"type"`
	Source     string `json:"source"`
	Target     string `json:"target"`
	Status     string `json:"status"`
	PipelineID string `json:"pipeline_id,omitempty"`
	Command    string `json:"command,omitempty"`
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

func DeleteHistoryItem(entry HistoryEntry, deleteFileFromDisk bool, downloadDir string) error {
	dir := GetConfigDir()
	_ = os.MkdirAll(dir, 0755)
	historyPath := filepath.Join(dir, "history.json")

	entries := GetHistory()
	var newEntries []HistoryEntry

	for _, e := range entries {
		if e.Time == entry.Time && e.Source == entry.Source && e.Target == entry.Target {
			if deleteFileFromDisk && entry.Target != "" {
				targetPath := entry.Target
				if !filepath.IsAbs(targetPath) {
					targetPath = filepath.Join(ParseUserPath(downloadDir), targetPath)
				}
				targetPath = filepath.Clean(targetPath)
				// Guard: удаляем файл только внутри downloadDir, чтобы подменённый
				// history.json не превратился в удаление произвольных путей.
				base := filepath.Clean(ParseUserPath(downloadDir))
				rel, err := filepath.Rel(base, targetPath)
				if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
					// Вне папки загрузок — снимаем только запись истории.
					continue
				}
				if _, err := os.Stat(targetPath); err == nil {
					_ = os.Remove(targetPath)
				}
			}
			continue
		}
		newEntries = append(newEntries, e)
	}

	data, err := json.MarshalIndent(newEntries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(historyPath, data, 0644)
}

func ClearHistory() error {
	historyPath := filepath.Join(GetConfigDir(), "history.json")
	_ = os.MkdirAll(GetConfigDir(), 0755)
	return os.WriteFile(historyPath, []byte("[]"), 0644)
}
