package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"mediacli/pkg/core"

	"github.com/gdamore/tcell/v2"
)

var logoStamp string

func pollUI(s tcell.Screen) (tcell.Event, bool) {
	event := s.PollEvent()
	if _, ok := event.(*tcell.EventResize); ok {
		s.Sync()
		invalidateLogo()
		return event, true
	}
	return event, false
}

func windowStart(index, start, visible int) int {
	if visible < 1 {
		visible = 1
	}
	if index < 0 {
		index = 0
	}
	if start < 0 {
		start = 0
	}
	if index < start {
		return index
	}
	if index >= start+visible {
		return index - visible + 1
	}
	return start
}

func wrapIndex(index, delta, count int) int {
	if count <= 0 {
		return 0
	}
	return (index + delta%count + count) % count
}

func titleWord(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return value
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func inputWindowStart(text []rune, cursor, width int, secret bool) int {
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(text) {
		cursor = len(text)
	}
	limit := width - 1
	if limit < 0 {
		return cursor
	}
	start := 0
	for start < cursor && displayWidth(text[start:cursor], secret) > limit {
		start++
	}
	return start
}

func displayWidth(text []rune, secret bool) int {
	if secret {
		return len(text)
	}
	return StringWidth(string(text))
}

func deleteWordBack(text []rune, cursor int) ([]rune, int) {
	if cursor <= 0 {
		return text, cursor
	}
	end := cursor
	for cursor > 0 && unicode.IsSpace(text[cursor-1]) {
		cursor--
	}
	for cursor > 0 && !unicode.IsSpace(text[cursor-1]) {
		cursor--
	}
	return append(text[:cursor], text[end:]...), cursor
}

type menuHit struct {
	minX, maxX, y, index int
}

func hitIndex(hits []menuHit, x, y int) (int, bool) {
	for _, hit := range hits {
		if y == hit.y && x >= hit.minX && x < hit.maxX {
			return hit.index, true
		}
	}
	return 0, false
}

func BrowsePath(s tcell.Screen, cfg *core.Config, current string) (string, bool) {
	dir := strings.TrimSpace(current)
	if dir == "" {
		dir = cfg.DownloadDir
	}
	dir = core.ParseUserPath(dir)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		next := filepath.Dir(dir)
		if next == "" || next == "." {
			if home, err := os.UserHomeDir(); err == nil {
				next = home
			} else {
				next = "."
			}
		}
		dir = next
	}

	for {
		entries, err := os.ReadDir(dir)
		if err != nil {
			ShowMessage(s, cfg, T(*cfg, "browse_title"), []string{err.Error()}, T(*cfg, "footer_message"))
			return "", false
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			names = append(names, name)
		}
		sort.Slice(names, func(i, j int) bool {
			leftInfo, leftErr := os.Stat(filepath.Join(dir, names[i]))
			rightInfo, rightErr := os.Stat(filepath.Join(dir, names[j]))
			leftDir := leftErr == nil && leftInfo.IsDir()
			rightDir := rightErr == nil && rightInfo.IsDir()
			if leftDir != rightDir {
				return leftDir
			}
			return strings.ToLower(names[i]) < strings.ToLower(names[j])
		})

		labels := []string{T(*cfg, "browse_select_dir")}
		targets := []string{dir}
		parent := filepath.Dir(dir)
		if parent != dir {
			labels = append(labels, "..")
			targets = append(targets, parent)
		}
		for _, name := range names {
			path := filepath.Join(dir, name)
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.IsDir() {
				labels = append(labels, name+"/")
			} else {
				labels = append(labels, name)
			}
			targets = append(targets, path)
		}

		choice := RunMenu(s, cfg, T(*cfg, "browse_title"), labels, dir, T(*cfg, "browse_footer"))
		if choice < 0 || choice >= len(targets) {
			return "", false
		}
		selected := targets[choice]
		if choice == 0 {
			return selected, true
		}
		info, err := os.Stat(selected)
		if err == nil && info.IsDir() {
			dir = selected
			continue
		}
		return selected, true
	}
}

func placeLogo(path, protocol string, x, y, rows int) {
	stamp := fmt.Sprintf("%s\x00%s\x00%d\x00%d\x00%d", path, protocol, x, y, rows)
	if stamp == logoStamp {
		return
	}
	clearTerminalImages()
	switch protocol {
	case "iterm2":
		fmt.Print(renderIterm2Image(path, x, y, rows))
	default:
		fmt.Print(renderKittyImage(path, x, y, rows))
	}
	logoStamp = stamp
}

func invalidateLogo() {
	if logoStamp == "" {
		return
	}
	clearTerminalImages()
}
