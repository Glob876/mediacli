package ui

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"mediacli/pkg/core"
)

//go:embed locales/*.json
var localesFS embed.FS

var (
	stringsTable     map[string]map[string]string
	stringsTableOnce sync.Once
)

func normalizeLang(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	l = strings.ReplaceAll(l, "-", "_")
	switch l {
	case "ru", "ru_ru":
		return "ru_ru"
	case "en", "en_gb", "en_gb_english":
		return "en_gb"
	case "en_us", "en-us", "en_us_english":
		return "en_us"
	default:
		// fallback: try exact, then prefix
		if _, ok := stringsTable[l]; ok {
			return l
		}
		if strings.HasPrefix(l, "ru") {
			return "ru_ru"
		}
		if strings.HasPrefix(l, "en") {
			// en without region -> en_gb (British) as before
			return "en_gb"
		}
		return "en_gb"
	}
}

func loadStringsTable() {
	stringsTable = make(map[string]map[string]string)
	// 1. Встроенные локали
	entries, err := localesFS.ReadDir("locales")
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := localesFS.ReadFile(filepath.Join("locales", e.Name()))
			if err != nil {
				continue
			}
			var m map[string]string
			if err := json.Unmarshal(data, &m); err != nil {
				continue
			}
			key := strings.TrimSuffix(strings.ToLower(e.Name()), ".json")
			stringsTable[key] = m
		}
	}
	// 2. Внешние переопределения: ~/.config/mediacli/locales/*.json и ./locales/*.json (для разработки)
	// Позволяет пользователю править переводы без пересборки.
	candidates := []string{
		filepath.Join(core.GetConfigDir(), "locales"),
		"locales",
		filepath.Join("pkg", "ui", "locales"),
	}
	for _, dir := range candidates {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			var m map[string]string
			if err := json.Unmarshal(data, &m); err != nil {
				continue
			}
			key := strings.TrimSuffix(strings.ToLower(e.Name()), ".json")
			if existing, ok := stringsTable[key]; ok {
				for k, v := range m {
					existing[k] = v
				}
			} else {
				stringsTable[key] = m
			}
		}
	}
	// 3. Алиасы для обратной совместимости: en -> en_gb, ru -> ru_ru
	if _, ok := stringsTable["en"]; !ok {
		if m, ok := stringsTable["en_gb"]; ok {
			stringsTable["en"] = m
		}
	}
	if _, ok := stringsTable["ru"]; !ok {
		if m, ok := stringsTable["ru_ru"]; ok {
			stringsTable["ru"] = m
		}
	}
}

func getTable(lang string) map[string]string {
	stringsTableOnce.Do(loadStringsTable)
	norm := normalizeLang(lang)
	if tbl, ok := stringsTable[norm]; ok {
		return tbl
	}
	// fallback
	if tbl, ok := stringsTable["en_gb"]; ok {
		return tbl
	}
	if tbl, ok := stringsTable["en"]; ok {
		return tbl
	}
	for _, tbl := range stringsTable {
		return tbl
	}
	return map[string]string{}
}

func T(cfg core.Config, key string, args ...interface{}) string {
	lang := cfg.Language
	table := getTable(lang)
	msg, ok := table[key]
	if !ok {
		// fallback to en_gb
		if fb, ok := stringsTable["en_gb"]; ok {
			if v, ok := fb[key]; ok {
				msg = v
				ok = true
			}
		}
		if !ok {
			if fb, ok := stringsTable["en"]; ok {
				if v, ok := fb[key]; ok {
					msg = v
					ok = true
				}
			}
		}
		if !ok {
			msg = key
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

func IsRussian(cfg core.Config) bool {
	return normalizeLang(cfg.Language) == "ru_ru"
}

func LanguageName(cfg core.Config) string {
	switch normalizeLang(cfg.Language) {
	case "ru_ru":
		return T(cfg, "lang_ru")
	case "en_us":
		return T(cfg, "lang_en_us")
	default:
		return T(cfg, "lang_en_gb")
	}
}

func PresetDisplayName(cfg core.Config, p core.VideoPreset) string {
	if IsRussian(cfg) {
		return p.NameRU
	}
	return p.NameEN
}
