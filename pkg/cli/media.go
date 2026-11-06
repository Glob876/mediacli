package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"mediacli/pkg/core"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var reflectString = reflect.TypeOf("")
var reflectBool = reflect.TypeOf(false)

type operationFlag struct {
	name   string
	kind   reflect.Type
	values map[string]any
}

func (value *operationFlag) String() string {
	if item, ok := value.values[value.name]; ok {
		data, _ := json.Marshal(item)
		return string(data)
	}
	return ""
}
func (value *operationFlag) IsBoolFlag() bool { return value.kind.Kind() == reflect.Bool }
func (value *operationFlag) Set(raw string) error {
	var parsed any
	switch value.kind.Kind() {
	case reflect.String:
		parsed = raw
	case reflect.Slice:
		var items []string
		if strings.HasPrefix(raw, "[") {
			if err := json.Unmarshal([]byte(raw), &items); err != nil {
				return err
			}
		} else {
			items = []string{raw}
		}
		if previous, ok := value.values[value.name].([]string); ok {
			items = append(previous, items...)
		}
		parsed = items
	default:
		target := reflect.New(value.kind)
		if err := json.Unmarshal([]byte(raw), target.Interface()); err != nil {
			return err
		}
		parsed = target.Elem().Interface()
	}
	value.values[value.name] = parsed
	return nil
}

func strictOperation(kind string, options map[string]any) (core.Operation, error) {
	data, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}
	op, err := core.UnmarshalOperation(core.OperationEnvelope{Type: kind, Config: data})
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(op); err != nil {
		return nil, err
	}
	return op, nil
}

func (cli *app) loadPipeline(project, preset string) (core.Pipeline, error) {
	if project != "" && preset != "" {
		return core.Pipeline{}, usage("choose --project or --preset")
	}
	if project != "" {
		if project == "editor" || project == "pipeline" {
			project = filepath.Join(core.GetConfigDir(), project+"_draft.json")
		}
		data, err := cli.read(project)
		if err != nil {
			return core.Pipeline{}, err
		}
		return core.UnmarshalPipeline(data)
	}
	if preset != "" && cli.server != "" {
		data, err := cli.request("GET", "/api/pipeline/presets", nil)
		if err != nil {
			return core.Pipeline{}, err
		}
		var response struct {
			Presets []core.MediaPreset `json:"presets"`
		}
		if err := json.Unmarshal(data, &response); err != nil {
			return core.Pipeline{}, err
		}
		for _, saved := range response.Presets {
			if saved.ID == preset {
				return core.PresetToPipeline(saved, nil, core.OutputSpec{})
			}
		}
		return core.Pipeline{}, usage("unknown remote media preset: " + preset)
	}
	if preset != "" {
		saved, ok := core.GetMediaPresetByID(preset)
		if !ok {
			return core.Pipeline{}, usage("unknown media preset: " + preset)
		}
		return core.PresetToPipeline(saved, nil, core.OutputSpec{})
	}
	return core.Pipeline{}, nil
}

func (cli *app) media(command string, args []string) error {
	flags := cli.flags(command)
	var inputs, operations, ranges stringList
	flags.Var(&inputs, "input", "Source file; repeat for multiple inputs")
	if command != "ranges" {
		flags.Var(&operations, "op", "Append an operation: TYPE=JSON (repeatable)")
	}
	if command == "ranges" {
		flags.Var(&ranges, "range", "Range START..END in seconds or HH:MM:SS (repeatable)")
	}
	output := optionalString(flags, command != "batch", "output", "", "Output path (existing files preserved by default)")
	if command != "batch" {
		flags.StringVar(output, "o", "", "Output path")
	}
	project := optionalString(flags, command != "ranges", "project", "", "Pipeline JSON file, '-' for stdin, 'editor' or 'pipeline' for TUI draft")
	preset := optionalString(flags, command != "ranges", "preset", "", "Saved media preset ID (convert: built-in conversion profile ID)")
	save := optionalString(flags, command != "ranges", "save-project", "", "Write the configured project without executing it")
	options := optionalString(flags, command != "ranges" && command != "size" && command != "batch" && command != "pipeline" && command != "editor", "options", "", "Operation JSON object or @FILE; every field also has a flag")
	overwrite := flags.Bool("overwrite", false, "Allow replacement of an existing output (never an input)")
	ratio := optionalString(flags, command == "aspect", "ratio", "", "Aspect ratio W:H")
	size := optionalFloat(flags, command != "ranges" && command != "batch", "size-mb", 0, "Approximate output size in decimal MB")
	twoPass := optionalBool(flags, command != "ranges" && command != "batch", "two-pass", true, "Use two-pass H.264 for --size-mb")
	preview := optionalFloat(flags, command != "ranges" && command != "batch", "preview", 0, "Export this many seconds of the edited result (0.1–15)")
	join := optionalBool(flags, command == "ranges", "join", false, "Join selected ranges; otherwise export separately")
	directory := optionalString(flags, command == "batch", "directory", "", "Batch input directory (nonrecursive)")
	outputDir := optionalString(flags, command == "batch", "output-dir", "", "Batch destination directory")
	template := optionalString(flags, command == "batch", "output-template", "%(basename)s_converted.%(ext)s", "Batch filename template")
	skipInvalid := optionalBool(flags, command == "batch", "skip-invalid", false, "Run valid batch files after reporting errors")
	kind := command
	if kind == "convert" {
		kind = "transcode"
	}
	if kind == "size" || kind == "batch" || kind == "pipeline" || kind == "editor" || kind == "ranges" {
		kind = ""
	}
	defaults := map[string]any{}
	switch kind {
	case "transcode":
		defaults = map[string]any{"vcodec": "libx264", "acodec": "aac", "crf": "20", "container": "mp4", "preset": "medium"}
	case "trim":
		defaults = map[string]any{"start": "0", "mode": "reencode"}
	case "aspect":
		defaults = map[string]any{"ratio_w": 9, "ratio_h": 16, "mode": "crop", "size_preset": "1080p"}
	case "concat":
		defaults = map[string]any{"mode": "filter"}
	case "subtitle":
		defaults = map[string]any{"mode": "burn"}
	case "scale":
		defaults = map[string]any{"width": 1280, "algo": "lanczos"}
	case "fps":
		defaults = map[string]any{"fps": "30"}
	case "demux":
		defaults = map[string]any{"stream_type": "audio"}
	}
	overrides := map[string]any{}
	if kind != "" {
		op, err := strictOperation(kind, defaults)
		if err != nil {
			return err
		}
		structure := reflect.TypeOf(op).Elem()
		for index := 0; index < structure.NumField(); index++ {
			field := structure.Field(index)
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			flagName := strings.ReplaceAll(name, "_", "-")
			if name == "input" {
				flagName = "subtitle-file"
			}
			if flags.Lookup(flagName) != nil {
				flagName = "op-" + flagName
			}
			flags.Var(&operationFlag{name: name, kind: field.Type, values: overrides}, flagName, kind+"."+name+" ("+field.Type.String()+")")
		}
	}
	if err := parse(flags, args); err != nil {
		return err
	}
	cancel, err := cli.setup()
	if err != nil {
		return err
	}
	defer cancel()
	selectedPreset := *preset
	if command == "convert" {
		selectedPreset = ""
	}
	pipeline, err := cli.loadPipeline(*project, selectedPreset)
	if err != nil {
		return err
	}
	inputs = append(inputs, flags.Args()...)
	if len(inputs) > 0 {
		pipeline.Inputs = nil
		for _, input := range inputs {
			pipeline.Inputs = append(pipeline.Inputs, core.ParseUserPath(input))
		}
	}
	if command == "batch" && *directory != "" {
		entries, err := os.ReadDir(core.ParseUserPath(*directory))
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() && mediaExtension(filepath.Ext(entry.Name())) {
				pipeline.Inputs = append(pipeline.Inputs, filepath.Join(core.ParseUserPath(*directory), entry.Name()))
			}
		}
	}
	if *output != "" {
		pipeline.Output.Path = core.ParseUserPath(*output)
	}
	pipeline.Output.Overwrite = *overwrite
	sizeSet, twoPassSet := false, false
	flags.Visit(func(option *flag.Flag) {
		if option.Name == "size-mb" {
			sizeSet = true
		}
		if option.Name == "two-pass" {
			twoPassSet = true
		}
	})
	if !sizeSet && pipeline.Output.TargetMegabytes > 0 {
		*size = pipeline.Output.TargetMegabytes
		if !twoPassSet {
			*twoPass = pipeline.Output.TwoPass
		}
	}
	pipeline.Output.TargetMegabytes = *size
	pipeline.Output.TwoPass = *twoPass
	if command == "convert" && *preset != "" {
		if len(pipeline.Inputs) == 0 {
			return usage("convert requires an input")
		}
		var profile *core.ConvertPreset
		for _, candidate := range core.ConvertPresets {
			if candidate.ID == *preset {
				copy := candidate
				profile = &copy
				break
			}
		}
		if profile == nil {
			return usage("unknown conversion profile: " + *preset)
		}
		defaults = map[string]any{"container": profile.Ext}
		keys := map[string]string{"-c:v": "vcodec", "-c:a": "acodec", "-acodec": "acodec", "-crf": "crf", "-preset": "preset", "-pix_fmt": "pix_fmt", "-profile:v": "profile", "-b:a": "bitrate_a", "-ab": "bitrate_a"}
		var extra []string
		for index := 0; index < len(profile.FFmpegFlags); index++ {
			flag := profile.FFmpegFlags[index]
			if key, ok := keys[flag]; ok && index+1 < len(profile.FFmpegFlags) {
				index++
				defaults[key] = profile.FFmpegFlags[index]
			} else {
				extra = append(extra, flag)
			}
		}
		if len(extra) > 0 {
			defaults["extra"] = extra
		}
		if profile.Ext == "mp3" {
			defaults["acodec"] = "libmp3lame"
		}
		if profile.Ext == "gif" {
			defaults["vcodec"] = "gif"
		}
		if pipeline.Output.Path == "" {
			pipeline.Output.Path = strings.TrimSuffix(pipeline.Inputs[0], filepath.Ext(pipeline.Inputs[0])) + profile.Suffix + "." + profile.Ext
		}
	}
	if kind != "" {
		if *options != "" {
			data := []byte(*options)
			if strings.HasPrefix(*options, "@") {
				data, err = cli.read((*options)[1:])
				if err != nil {
					return err
				}
			}
			var supplied map[string]any
			if err := json.Unmarshal(data, &supplied); err != nil {
				return usage(err.Error())
			}
			for key, value := range supplied {
				defaults[key] = value
			}
		}
		for key, value := range overrides {
			defaults[key] = value
		}
		if *ratio != "" {
			if kind != "aspect" {
				return usage("--ratio requires aspect")
			}
			parts := strings.Split(*ratio, ":")
			if len(parts) != 2 {
				return usage("ratio must be W:H")
			}
			width, err := strconv.Atoi(parts[0])
			if err != nil {
				return usage(err.Error())
			}
			height, err := strconv.Atoi(parts[1])
			if err != nil {
				return usage(err.Error())
			}
			defaults["ratio_w"], defaults["ratio_h"] = width, height
		}
		if kind == "concat" {
			defaults["inputs"] = pipeline.Inputs
		}
		op, err := strictOperation(kind, defaults)
		if err != nil {
			return usage(err.Error())
		}
		pipeline.Operations = append(pipeline.Operations, op)
	}
	for _, definition := range operations {
		opType, raw, ok := strings.Cut(definition, "=")
		if !ok {
			return usage("--op must be TYPE=JSON")
		}
		var config map[string]any
		if err := json.Unmarshal([]byte(raw), &config); err != nil {
			return usage(err.Error())
		}
		op, err := strictOperation(opType, config)
		if err != nil {
			return usage(err.Error())
		}
		pipeline.Operations = append(pipeline.Operations, op)
	}
	if len(pipeline.Inputs) == 0 {
		return usage("at least one input is required (--input FILE or positional FILE)")
	}
	if cli.server == "" && command != "batch" {
		for _, input := range pipeline.Inputs {
			stat, err := os.Stat(input)
			if err != nil {
				return err
			}
			if !stat.Mode().IsRegular() {
				return usage("not a regular file: " + input)
			}
		}
	}
	if *save != "" {
		if cli.dry {
			data, err := core.MarshalPipeline(pipeline)
			if err != nil {
				return err
			}
			return cli.emit(json.RawMessage(data))
		}
		data, err := core.MarshalPipeline(pipeline)
		if err != nil {
			return err
		}
		if err := core.WritePrivateFile(core.ParseUserPath(*save), data); err != nil {
			return err
		}
		return cli.emit(map[string]any{"saved": *save})
	}
	if command == "batch" {
		job := core.BatchJob{Inputs: pipeline.Inputs, Pipeline: pipeline, OutputDir: *outputDir, OutputTemplate: *template, Overwrite: *overwrite}
		if cli.server != "" {
			return cli.remoteBatch(job, *skipInvalid)
		}
		report := core.PlanBatch(job, cli.cfg)
		failed := false
		for _, item := range report.Items {
			if item.Error != "" {
				failed = true
			}
		}
		if cli.dry {
			return cli.emit(report)
		}
		if failed {
			for _, line := range report.Lines() {
				fmt.Fprintln(cli.errOut, line)
			}
			if !*skipInvalid {
				return usage("batch validation failed; use --skip-invalid to run valid files")
			}
		}
		return cli.executeBatch(report)
	}
	if command == "ranges" {
		if cli.server != "" {
			return usage("ranges uses local files; omit --server")
		}
		var selected []core.MediaRange
		for _, raw := range ranges {
			start, end, ok := strings.Cut(raw, "..")
			if !ok {
				return usage("--range must be START..END")
			}
			a, err := core.ParseMediaTime(start)
			if err != nil {
				return usage(err.Error())
			}
			b, err := core.ParseMediaTime(end)
			if err != nil {
				return usage(err.Error())
			}
			selected = append(selected, core.MediaRange{Start: a, End: b})
		}
		commands, err := core.BuildRangeExport(pipeline.Inputs[0], cli.cfg, selected, *join)
		if err != nil {
			return err
		}
		if *overwrite {
			for _, command := range commands {
				command[1] = "-y"
			}
		}
		if *output != "" {
			if len(commands) != 1 {
				return usage("--output requires a single range or --join")
			}
			commands[0][len(commands[0])-1] = pipeline.Output.Path
		}
		if !*join && len(commands) > 1 {
			report := core.BatchReport{}
			for _, command := range commands {
				report.Items = append(report.Items, core.BatchItem{Input: pipeline.Inputs[0], Output: command[len(command)-1], Commands: [][]string{command}})
			}
			if !cli.dry {
				return cli.executeBatch(report)
			}
		}
		return cli.execute(commands, "Ranges", pipeline.Inputs[0], nil)
	}
	if cli.server != "" {
		if *size > 0 || command == "size" {
			return usage("target-size export uses local files; omit --server")
		}
		return cli.remotePipeline(pipeline, *preview)
	}
	var commands [][]string
	if *size > 0 || command == "size" {
		if *size <= 0 {
			return usage("size requires a positive --size-mb")
		}
		commands, err = core.BuildSizeExport(pipeline, cli.cfg, *size, *twoPass)
	} else {
		commands, err = core.BuildPipeline(pipeline, cli.cfg)
	}
	if err != nil {
		return err
	}
	if *preview != 0 {
		if *preview < 0.1 || *preview > 15 {
			return usage("--preview must be between 0.1 and 15 seconds")
		}
		if len(commands) != 1 {
			return usage("preview cannot combine multi-pass operations")
		}
		command := commands[0]
		output := command[len(command)-1]
		command = append(command[:len(command)-1], "-t", fmt.Sprint(*preview), "-c:v", "libx264", "-c:a", "aac", "-pix_fmt", "yuv420p", output)
		commands[0] = command
	}
	return cli.execute(commands, command, strings.Join(pipeline.Inputs, ","), nil)
}

func checkOutput(output string, inputs []string, overwrite bool) error {
	absolute, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	destination, _ := os.Stat(absolute)
	for _, input := range inputs {
		source, _ := filepath.Abs(input)
		stat, _ := os.Stat(source)
		if source == absolute || stat != nil && destination != nil && os.SameFile(stat, destination) {
			return usage("output would overwrite an input: " + output)
		}
	}
	if destination != nil && !overwrite {
		return usage("output exists: " + output)
	}
	return nil
}

func mediaExtension(extension string) bool {
	switch strings.ToLower(extension) {
	case ".mp4", ".mkv", ".mov", ".webm", ".avi", ".m4a", ".mp3", ".wav", ".flac", ".ogg", ".opus", ".aac":
		return true
	}
	return false
}

func (cli *app) inspect(command string, args []string) error {
	flags := cli.flags(command)
	start := flags.Float64("start", 0, "Waveform start in seconds")
	columns := flags.Int("columns", 80, "Waveform width (1–1000)")
	if err := parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return usage(command + " requires one file")
	}
	cancel, err := cli.setup()
	if err != nil {
		return err
	}
	defer cancel()
	path := core.ParseUserPath(flags.Arg(0))
	if command == "waveform" {
		if cli.server != "" {
			return usage("waveform uses a local file; omit --server")
		}
		levels, err := core.Waveform(path, cli.cfg, *start, *columns)
		if err != nil {
			return err
		}
		if cli.json {
			return cli.emit(levels)
		}
		glyphs := []rune(" ▁▂▃▄▅▆▇█")
		var drawing strings.Builder
		for _, level := range levels {
			index := int(math.Min(8, math.Max(0, level*8)))
			drawing.WriteRune(glyphs[index])
		}
		return cli.emit(drawing.String())
	}
	if cli.server != "" {
		return cli.remote("GET", "/api/probe?path="+query(path), nil)
	}
	info, err := core.ProbeMediaInfoWithConfig(path, cli.cfg)
	if err != nil {
		return err
	}
	return cli.emit(info)
}

// execute keeps the shared queue, cancellation, completion hooks and history semantics.
func (cli *app) execute(commands [][]string, title, source string, hook func(int) error) error {
	if cli.ctx.Err() != nil {
		return cli.ctx.Err()
	}
	if len(commands) == 0 {
		return usage("empty execution plan")
	}
	if cli.dry {
		masked := make([][]string, len(commands))
		for index, command := range commands {
			masked[index] = core.MaskCmdForLog(command)
		}
		return cli.emit(map[string]any{"commands": masked})
	}
	core.GlobalQueue.SyncMaxTasksFromConfig(cli.cfg)
	var tasks []*core.BackgroundTask
	target := commands[len(commands)-1][len(commands[len(commands)-1])-1]
	lastCommand := commands[len(commands)-1]
	if filepath.Base(lastCommand[0]) == "yt-dlp" {
		target = cli.cfg.DownloadDir
		for index, argument := range lastCommand {
			if (argument == "-o" || argument == "-P") && index+1 < len(lastCommand) {
				target = lastCommand[index+1]
			}
		}
	}
	for _, command := range commands {
		if len(command) < 2 {
			return usage("invalid command")
		}
		var inputs []string
		for index, argument := range command {
			if argument == "-i" && index+1 < len(command) {
				inputs = append(inputs, command[index+1])
			}
		}
		output := command[len(command)-1]
		if len(inputs) > 0 && output != "-" && output != os.DevNull {
			overwrite := false
			for _, argument := range command {
				if argument == "-y" {
					overwrite = true
				}
			}
			if err := checkOutput(output, inputs, overwrite); err != nil {
				return err
			}
		}
	}
	if hook != nil {
		tasks = []*core.BackgroundTask{core.GlobalQueue.EnqueueWithHook(commands[0], title, source, target, hook)}
	} else {
		tasks = core.GlobalQueue.EnqueueChain(commands, title, source, target, fmt.Sprintf("cli_%d", time.Now().UnixNano()))
	}
	ids := make([]int, len(tasks))
	for index, task := range tasks {
		ids[index] = task.ID
	}
	if err := cli.waitLocal(ids); err != nil {
		return err
	}
	return cli.emit(map[string]any{"status": "done", "task_ids": ids, "output": target})
}

func (cli *app) waitLocal(ids []int) error {
	positions := map[int]int{}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		complete := true
		for _, id := range ids {
			snapshot, ok := core.GlobalQueue.Snapshot(id)
			if !ok {
				return fmt.Errorf("task %d disappeared", id)
			}
			lines := core.GlobalQueue.TaskLogs(id)
			for _, line := range lines[positions[id]:] {
				fmt.Fprintln(cli.errOut, line)
			}
			positions[id] = len(lines)
			switch snapshot.Status {
			case core.StatusQueued, core.StatusRunning:
				complete = false
			case core.StatusFailed, core.StatusCancelled, core.StatusInterrupted:
				for _, pending := range ids {
					core.GlobalQueue.CancelTask(pending)
				}
				return fmt.Errorf("task %d: %s (exit %d): %s", id, snapshot.Status, snapshot.ExitCode, core.ErrorHint(lines, strings.HasPrefix(cli.cfg.Language, "ru")))
			}
		}
		if complete {
			return nil
		}
		select {
		case <-cli.ctx.Done():
			for _, id := range ids {
				core.GlobalQueue.CancelTask(id)
			}
			// Cancellation finalizes temporary-output hooks asynchronously; wait before exiting.
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				done := true
				for _, id := range ids {
					snapshot, _ := core.GlobalQueue.Snapshot(id)
					if !snapshot.Finalized {
						done = false
					}
				}
				if done {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			return cli.ctx.Err()
		case <-ticker.C:
		}
	}
}

func (cli *app) executeBatch(report core.BatchReport) error {
	if cli.ctx.Err() != nil {
		return cli.ctx.Err()
	}
	core.GlobalQueue.SyncMaxTasksFromConfig(cli.cfg)
	ids := report.Enqueue()
	if len(ids) == 0 {
		return usage("no valid inputs")
	}
	if err := cli.waitLocal(ids); err != nil {
		return err
	}
	var outputs []string
	for _, item := range report.Items {
		if item.Error == "" {
			outputs = append(outputs, item.Output)
		}
	}
	return cli.emit(map[string]any{"status": "done", "task_ids": ids, "outputs": outputs})
}
