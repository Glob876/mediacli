package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// MediaPreset — сохранённый Pipeline как пресет.
type MediaPreset struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Operations  []OperationEnvelope `json:"operations"`
	CreatedAt   string          `json:"created_at"`
}

// MediaPresets хранятся в Config (аналог DownloadPresets).
func GetMediaPresets(cfg Config) []MediaPreset {
	// Храним в файле media_presets.json рядом с config.json
	path := filepath.Join(GetConfigDir(), "media_presets.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return []MediaPreset{}
	}
	var out []MediaPreset
	if err := json.Unmarshal(data, &out); err != nil {
		return []MediaPreset{}
	}
	return out
}

func SaveMediaPreset(p MediaPreset) error {
	dir := GetConfigDir()
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "media_presets.json")
	existing := GetMediaPresets(GetDefaultConfig())
	// upsert
	found := false
	for i, e := range existing {
		if e.ID == p.ID {
			existing[i] = p
			found = true
			break
		}
	}
	if !found {
		if p.ID == "" {
			p.ID = fmt.Sprintf("mp_%d", time.Now().UnixNano())
		}
		if p.CreatedAt == "" {
			p.CreatedAt = time.Now().Format(time.RFC3339)
		}
		existing = append(existing, p)
	}
	sort.Slice(existing, func(i, j int) bool { return existing[i].Name < existing[j].Name })
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func DeleteMediaPreset(id string) error {
	dir := GetConfigDir()
	path := filepath.Join(dir, "media_presets.json")
	existing := GetMediaPresets(GetDefaultConfig())
	var out []MediaPreset
	for _, e := range existing {
		if e.ID != id {
			out = append(out, e)
		}
	}
	if out == nil {
		out = []MediaPreset{}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func GetMediaPresetByID(id string) (MediaPreset, bool) {
	for _, p := range GetMediaPresets(GetDefaultConfig()) {
		if p.ID == id {
			return p, true
		}
	}
	return MediaPreset{}, false
}

// PresetToPipeline конвертирует пресет в Pipeline для заданных инпутов/аутпута.
func PresetToPipeline(preset MediaPreset, inputs []string, output OutputSpec) (Pipeline, error) {
	p := Pipeline{
		ID:     preset.ID,
		Name:   preset.Name,
		Inputs: inputs,
		Output: output,
	}
	for _, env := range preset.Operations {
		op, err := UnmarshalOperation(env)
		if err != nil {
			return p, err
		}
		p.Operations = append(p.Operations, op)
	}
	return p, nil
}
