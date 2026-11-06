package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mediacli/pkg/core"
	"mediacli/pkg/daemon"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func (cli *app) admin(command string, args []string) error {
	flags := cli.flags(command)
	kind := optionalString(flags, command == "presets", "kind", "media", "Preset kind: media, download, convert, video")
	file := optionalString(flags, command == "presets" || command == "cookies", "file", "", "JSON import/export file, or cookies file")
	search := optionalString(flags, command == "history", "search", "", "Filter history by text")
	deleteFile := optionalBool(flags, command == "history", "delete-file", false, "Also delete the selected history output inside download_dir")
	if err := parse(flags, args); err != nil {
		return err
	}
	cancel, err := cli.setup()
	if err != nil {
		return err
	}
	defer cancel()
	positional := flags.Args()
	action := "list"
	if len(positional) > 0 {
		action = positional[0]
		positional = positional[1:]
	}
	switch command {
	case "config":
		if action == "list" {
			action = "get"
		}
		return cli.configuration(action, positional)
	case "presets":
		return cli.presets(action, positional, *kind, *file)
	case "history":
		return cli.history(action, positional, *search, *deleteFile)
	case "queue":
		return cli.queue(action, positional)
	case "doctor":
		if cli.server != "" {
			return cli.remote("GET", "/api/status", nil)
		}
		dependencies := core.CheckDependencies()
		for index := range dependencies {
			dependency := &dependencies[index]
			if dependency.Name == "ffmpeg" {
				path, _, err := core.ResolveFfmpeg(cli.cfg, "")
				dependency.Path = path
				dependency.Available = err == nil
			}
			if dependency.Name == "ffprobe" {
				path, _, err := core.ResolveFfprobe(cli.cfg, "")
				dependency.Path = path
				dependency.Available = err == nil
			}
		}
		if err := cli.emit(dependencies); err != nil {
			return err
		}
		for _, dependency := range dependencies {
			if dependency.Required && !dependency.Available {
				return fmt.Errorf("required dependency missing: %s", dependency.Name)
			}
		}
		return nil
	case "library":
		if cli.server != "" {
			if flags.NArg() > 0 {
				return cli.remote("GET", "/api/browse?path="+query(flags.Arg(0)), nil)
			}
			return cli.remote("GET", "/api/library", nil)
		}
		path := cli.cfg.DownloadDir
		if flags.NArg() > 0 {
			path = flags.Arg(0)
		}
		entries, err := os.ReadDir(core.ParseUserPath(path))
		if err != nil {
			return err
		}
		type item struct {
			Name      string `json:"name"`
			Size      int64  `json:"size"`
			Directory bool   `json:"directory"`
			Media     bool   `json:"media"`
		}
		items := []item{}
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			items = append(items, item{entry.Name(), info.Size(), entry.IsDir(), mediaExtension(filepath.Ext(entry.Name()))})
		}
		return cli.emit(items)
	case "cookies":
		if cli.server != "" {
			return usage("cookies manages a local cookie file; omit --server")
		}
		path := *file
		if path == "" {
			path = cli.cfg.CookiesFile
		}
		if action == "list" {
			cookies, err := core.ParseCookiesFile(path)
			if err != nil {
				return err
			}
			return cli.emit(cookies)
		}
		if action != "add" || len(positional) != 3 {
			return usage("cookies add DOMAIN NAME VALUE [--file FILE]")
		}
		for _, value := range positional {
			if strings.ContainsAny(value, "\r\n\t") {
				return usage("cookie fields cannot contain tabs or newlines")
			}
		}
		if cli.dry {
			return cli.emit(map[string]any{"file": path, "domain": positional[0], "name": positional[1], "value": "***"})
		}
		if err := core.AppendCookie(path, positional[0], positional[1], positional[2]); err != nil {
			return err
		}
		if err := os.Chmod(core.ParseUserPath(path), 0600); err != nil {
			return err
		}
		return cli.emit(map[string]any{"saved": path})
	}
	return usage("unknown command")
}

func (cli *app) configuration(action string, args []string) error {
	if action == "options" {
		if cli.server != "" {
			return cli.remote("GET", "/api/meta", nil)
		}
		return cli.emit(daemon.Metadata())
	}
	var config map[string]any
	raw, _ := json.Marshal(cli.cfg)
	if cli.server != "" {
		var err error
		raw, err = cli.request("GET", "/api/config", nil)
		if err != nil {
			return err
		}
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return err
	}
	switch action {
	case "get":
		if len(args) == 0 {
			return cli.emit(config)
		}
		if len(args) != 1 {
			return usage("config get [KEY]")
		}
		value, exists := config[args[0]]
		if !exists {
			structure := reflect.TypeOf(core.Config{})
			for index := 0; index < structure.NumField(); index++ {
				field := structure.Field(index)
				if strings.Split(field.Tag.Get("json"), ",")[0] == args[0] {
					return cli.emit(reflect.Zero(field.Type).Interface())
				}
			}
			return usage("unknown configuration key: " + args[0])
		}
		return cli.emit(value)
	case "export":
		if cli.dry {
			return cli.emit(config)
		}
		if len(args) != 1 {
			return usage("config export FILE")
		}
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return err
		}
		if err := core.WritePrivateFile(core.ParseUserPath(args[0]), data); err != nil {
			return err
		}
		return cli.emit(map[string]any{"saved": args[0]})
	case "set":
		if len(args) != 2 {
			return usage("config set KEY VALUE")
		}
		// Optional fields may be absent because of omitempty; validate against the typed structure below.
		config[args[0]] = parseValue(args[1])
	case "reset":
		if len(args) != 0 {
			return usage("config reset takes no arguments")
		}
		data, _ := json.Marshal(core.GetDefaultConfig())
		config = nil
		json.Unmarshal(data, &config)
	case "import":
		if len(args) != 1 {
			return usage("config import FILE")
		}
		data, err := cli.read(args[0])
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &config); err != nil {
			return err
		}
	default:
		return usage("config get|set|reset|import|export")
	}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	var typed core.Config
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&typed); err != nil {
		return usage(err.Error())
	}
	if typed.BGQueueMax < 1 || typed.BGQueueMax > 8 {
		return usage("bg_queue_max must be between 1 and 8")
	}
	if cli.dry {
		return cli.emit(typed)
	}
	if cli.server != "" {
		return cli.remote("PUT", "/api/config", typed)
	}
	configPath := filepath.Join(core.GetConfigDir(), "config.json")
	if old, err := os.ReadFile(configPath); err == nil {
		if err := core.WritePrivateFile(configPath+fmt.Sprintf(".bak-%d", time.Now().UnixNano()), old); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := core.WritePrivateFile(configPath, data); err != nil {
		return err
	}
	return cli.emit(map[string]any{"saved": configPath})
}

func (cli *app) presets(action string, args []string, kind, file string) error {
	if cli.server != "" {
		return cli.remotePresets(action, args, kind, file)
	}
	var presets any
	switch kind {
	case "media":
		presets = core.GetMediaPresets(cli.cfg)
	case "download":
		presets = cli.cfg.DownloadPresets
	case "convert":
		presets = core.ConvertPresets
	case "video":
		presets = core.VideoPresets
	default:
		return usage("unknown preset kind")
	}
	if action == "list" {
		return cli.emit(presets)
	}
	if len(args) != 1 {
		return usage("presets " + action + " ID --kind " + kind)
	}
	id := args[0]
	if action == "show" {
		switch kind {
		case "media":
			for _, preset := range core.GetMediaPresets(cli.cfg) {
				if preset.ID == id {
					return cli.emit(preset)
				}
			}
		case "download":
			for _, preset := range cli.cfg.DownloadPresets {
				if preset.ID == id {
					return cli.emit(preset)
				}
			}
		case "convert":
			for _, preset := range core.ConvertPresets {
				if preset.ID == id {
					return cli.emit(preset)
				}
			}
		case "video":
			if preset, ok := core.VideoPresets[id]; ok {
				return cli.emit(preset)
			}
		}
		return usage("unknown preset: " + id)
	}
	if kind == "convert" || kind == "video" {
		return usage("built-in profiles are read-only; save a media preset instead")
	}
	if action != "save" && action != "delete" {
		return usage("presets list|show|save|delete")
	}
	if action == "delete" {
		if cli.dry {
			return cli.emit(map[string]any{"delete": id, "kind": kind})
		}
		if kind == "media" {
			if _, ok := core.GetMediaPresetByID(id); !ok {
				return usage("unknown preset")
			}
			if err := core.DeleteMediaPreset(id); err != nil {
				return err
			}
		} else {
			found := false
			var kept []core.DownloadPreset
			for _, preset := range cli.cfg.DownloadPresets {
				if preset.ID == id {
					found = true
				} else {
					kept = append(kept, preset)
				}
			}
			if !found {
				return usage("unknown preset")
			}
			cli.cfg.DownloadPresets = kept
			if err := core.SaveConfig(cli.cfg); err != nil {
				return err
			}
		}
		return cli.emit(map[string]any{"deleted": id})
	}
	if file == "" {
		return usage("presets save ID requires --file FILE")
	}
	data, err := cli.read(file)
	if err != nil {
		return err
	}
	if kind == "media" {
		var preset core.MediaPreset
		if err := json.Unmarshal(data, &preset); err != nil {
			return err
		}
		preset.ID = id
		if preset.Name == "" {
			preset.Name = id
		}
		for _, operation := range preset.Operations {
			var options map[string]any
			if err := json.Unmarshal(operation.Config, &options); err != nil {
				return err
			}
			if _, err := strictOperation(operation.Type, options); err != nil {
				return err
			}
		}
		if cli.dry {
			return cli.emit(preset)
		}
		if err := core.SaveMediaPreset(preset); err != nil {
			return err
		}
	} else {
		var preset core.DownloadPreset
		if err := json.Unmarshal(data, &preset); err != nil {
			return err
		}
		preset.ID = id
		if preset.Name == "" {
			preset.Name = id
		}
		if preset.Fields == nil {
			return usage("download preset requires fields")
		}
		if cli.dry {
			return cli.emit(preset)
		}
		found := false
		for index, existing := range cli.cfg.DownloadPresets {
			if existing.ID == id {
				cli.cfg.DownloadPresets[index] = preset
				found = true
				break
			}
		}
		if !found {
			cli.cfg.DownloadPresets = append(cli.cfg.DownloadPresets, preset)
		}
		if err := core.SaveConfig(cli.cfg); err != nil {
			return err
		}
	}
	return cli.emit(map[string]any{"saved": id, "kind": kind})
}

func (cli *app) history(action string, args []string, search string, deleteFile bool) error {
	entries := core.GetHistory()
	if cli.server != "" {
		data, err := cli.request("GET", "/api/history", nil)
		if err != nil {
			return err
		}
		var response struct {
			History []core.HistoryEntry `json:"history"`
		}
		if err := json.Unmarshal(data, &response); err != nil {
			return err
		}
		entries = response.History
	}
	if action == "list" {
		type row struct {
			Index int `json:"index"`
			core.HistoryEntry
		}
		rows := []row{}
		for index, entry := range entries {
			data, _ := json.Marshal(entry)
			if strings.Contains(strings.ToLower(string(data)), strings.ToLower(search)) {
				entry.Commands = maskedCommands(entry.Commands)
				rows = append(rows, row{index, entry})
			}
		}
		return cli.emit(rows)
	}
	if action == "clear" {
		if cli.dry {
			return cli.emit("Clear history entries")
		}
		if cli.server != "" {
			return cli.remote("DELETE", "/api/history", nil)
		}
		if err := core.ClearHistory(); err != nil {
			return err
		}
		return cli.emit(map[string]bool{"cleared": true})
	}
	if len(args) != 1 {
		return usage("history show|retry|delete INDEX")
	}
	index, err := strconv.Atoi(args[0])
	if err != nil || index < 0 || index >= len(entries) {
		return usage("invalid history index")
	}
	entry := entries[index]
	switch action {
	case "show":
		entry.Commands = maskedCommands(entry.Commands)
		return cli.emit(entry)
	case "delete":
		if cli.dry {
			return cli.emit(map[string]any{"delete": index, "delete_file": deleteFile})
		}
		if cli.server != "" {
			return cli.remote("POST", "/api/history/delete", map[string]any{"time": entry.Time, "source": entry.Source, "target": entry.Target, "delete_file": deleteFile})
		}
		if err := core.DeleteHistoryItem(entry, deleteFile, cli.cfg.DownloadDir); err != nil {
			return err
		}
		return cli.emit(map[string]any{"deleted": index})
	case "retry":
		if cli.dry {
			return cli.emit(map[string]any{"commands": maskedCommands(entry.Commands)})
		}
		if cli.server != "" {
			result, err := cli.request("POST", fmt.Sprintf("/api/history/%d/retry", index), nil)
			if err != nil {
				return err
			}
			return cli.submitted(result)
		}
		tasks, err := core.GlobalQueue.ReplayCommands(entry.Commands, entry.Type, entry.Source, entry.Target)
		if err != nil {
			return err
		}
		var ids []int
		for _, task := range tasks {
			ids = append(ids, task.ID)
		}
		if err := cli.waitLocal(ids); err != nil {
			return err
		}
		return cli.emit(map[string]any{"task_ids": ids, "status": "done"})
	}
	return usage("history list|show|retry|clear|delete")
}

func (cli *app) queue(action string, args []string) error {
	if cli.server == "" {
		return usage("queue requires --server URL; start mediacli daemon --port 8181")
	}
	if action == "list" {
		return cli.remote("GET", "/api/tasks", nil)
	}
	if len(args) != 1 {
		return usage("queue " + action + " ID --server URL")
	}
	if action == "cancel-group" {
		if cli.dry {
			return cli.emit(map[string]any{"cancel_group": args[0]})
		}
		return cli.remote("POST", "/api/tasks/group/"+urlPath(args[0])+"/cancel", nil)
	}
	id, err := strconv.Atoi(args[0])
	if err != nil || id < 1 {
		return usage("invalid task ID")
	}
	path := fmt.Sprintf("/api/tasks/%d", id)
	switch action {
	case "show":
		return cli.remote("GET", path, nil)
	case "logs":
		return cli.remote("GET", path+"/logs", nil)
	case "wait":
		if err := cli.waitRemote(id); err != nil {
			return err
		}
		return cli.remote("GET", path, nil)
	case "cancel", "retry":
		if cli.dry {
			return cli.emit(map[string]any{action: id})
		}
		result, err := cli.request("POST", path+"/"+action, nil)
		if err != nil {
			return err
		}
		if action == "retry" {
			return cli.submitted(result)
		}
		return cli.emit(result)
	}
	return usage("queue list|show|logs|wait|cancel|retry|cancel-group")
}
