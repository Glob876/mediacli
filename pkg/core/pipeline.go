package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Pipeline — цепочка операций, превращающаяся в один или несколько ffmpeg вызовов.
type Pipeline struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Inputs     []string    `json:"inputs"`
	Operations []Operation `json:"operations"`
	Output     OutputSpec  `json:"output"`
}

type OutputSpec struct {
	TargetMegabytes float64 `json:"target_megabytes,omitempty"`
	TwoPass         bool    `json:"two_pass,omitempty"`
	Path            string  `json:"path"`      // шаблон или явный путь
	Overwrite       bool    `json:"overwrite"` // -y
	Dir             string  `json:"dir,omitempty"`
}

// Operation — полиморфная операция пайплайна.
type Operation interface {
	Type() string
	Validate(info *MediaInfo) error
	Build(ctx *BuildContext) error
	Describe() string
}

// BuildContext накапливает аргументы ffmpeg.
type BuildContext struct {
	Config Config
	// Входные файлы (первый — основной)
	Inputs []string
	// Output
	Output string
	// Аккумулятор
	Args []string
	// Фильтры
	FilterGraph *FilterGraph
	// Маппинги и кодеки
	VideoCodec string
	AudioCodec string
	ExtraArgs  []string
	// Для двухпроходных операций
	Passes [][]string
	// Мета
	HasVideoFilter bool
	HasAudioFilter bool
	NeedsReencode  bool
	InputHasVideo  bool
	InputHasAudio  bool
}

func NewBuildContext(cfg Config, inputs []string, output string) *BuildContext {
	return &BuildContext{
		Config:      cfg,
		Inputs:      inputs,
		Output:      output,
		FilterGraph: NewFilterGraph(),
		VideoCodec:  "copy",
		AudioCodec:  "copy",
	}
}

// OperationEnvelope — JSON-обёртка для полиморфного маршалинга.
type OperationEnvelope struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

// MarshalPipeline сериализует пайплайн с типизированными операциями.
func MarshalPipeline(p Pipeline) ([]byte, error) {
	type rawOp struct {
		Type   string      `json:"type"`
		Config interface{} `json:"config"`
	}
	ops := make([]rawOp, 0, len(p.Operations))
	for _, op := range p.Operations {
		ops = append(ops, rawOp{Type: op.Type(), Config: op})
	}
	aux := struct {
		ID         string     `json:"id"`
		Name       string     `json:"name"`
		Inputs     []string   `json:"inputs"`
		Operations []rawOp    `json:"operations"`
		Output     OutputSpec `json:"output"`
	}{ID: p.ID, Name: p.Name, Inputs: p.Inputs, Operations: ops, Output: p.Output}
	return json.Marshal(aux)
}

// UnmarshalPipeline парсит пайплайн из JSON.
func UnmarshalPipeline(data []byte) (Pipeline, error) {
	var aux struct {
		ID         string              `json:"id"`
		Name       string              `json:"name"`
		Inputs     []string            `json:"inputs"`
		Operations []OperationEnvelope `json:"operations"`
		Output     OutputSpec          `json:"output"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return Pipeline{}, err
	}
	p := Pipeline{ID: aux.ID, Name: aux.Name, Inputs: aux.Inputs, Output: aux.Output}
	for _, env := range aux.Operations {
		op, err := UnmarshalOperation(env)
		if err != nil {
			return Pipeline{}, fmt.Errorf("operation %q: %w", env.Type, err)
		}
		p.Operations = append(p.Operations, op)
	}
	return p, nil
}

func UnmarshalOperation(env OperationEnvelope) (Operation, error) {
	switch env.Type {
	case "transcode":
		var o TranscodeOp
		if err := json.Unmarshal(env.Config, &o); err != nil {
			return nil, err
		}
		return &o, nil
	case "aspect":
		var o AspectOp
		if err := json.Unmarshal(env.Config, &o); err != nil {
			return nil, err
		}
		return &o, nil
	case "mux":
		var o MuxOp
		if err := json.Unmarshal(env.Config, &o); err != nil {
			return nil, err
		}
		return &o, nil
	case "demux":
		var o DemuxOp
		if err := json.Unmarshal(env.Config, &o); err != nil {
			return nil, err
		}
		return &o, nil
	case "trim":
		var o TrimOp
		if err := json.Unmarshal(env.Config, &o); err != nil {
			return nil, err
		}
		return &o, nil
	case "concat":
		var o ConcatOp
		if err := json.Unmarshal(env.Config, &o); err != nil {
			return nil, err
		}
		return &o, nil
	case "filter":
		var o FilterOp
		if err := json.Unmarshal(env.Config, &o); err != nil {
			return nil, err
		}
		return &o, nil
	case "audio":
		var o AudioOp
		if err := json.Unmarshal(env.Config, &o); err != nil {
			return nil, err
		}
		return &o, nil
	case "subtitle":
		var o SubtitleOp
		if err := json.Unmarshal(env.Config, &o); err != nil {
			return nil, err
		}
		return &o, nil
	case "metadata":
		var o MetadataOp
		if err := json.Unmarshal(env.Config, &o); err != nil {
			return nil, err
		}
		return &o, nil
	case "scale":
		var o ScaleOp
		if err := json.Unmarshal(env.Config, &o); err != nil {
			return nil, err
		}
		return &o, nil
	case "fps":
		var o FpsOp
		if err := json.Unmarshal(env.Config, &o); err != nil {
			return nil, err
		}
		return &o, nil
	case "stabilize":
		var o StabilizeOp
		if err := json.Unmarshal(env.Config, &o); err != nil {
			return nil, err
		}
		return &o, nil
	default:
		return nil, fmt.Errorf("unknown operation type %q", env.Type)
	}
}

// BuildPipeline собирает финальные ffmpeg аргументы.
func BuildPipeline(p Pipeline, cfg Config) ([][]string, error) {
	if len(p.Inputs) == 0 {
		return nil, fmt.Errorf("no inputs")
	}
	probe, _ := ProbeMediaInfoWithConfig(p.Inputs[0], cfg)
	hasVideo, hasAudio := false, false
	if probe != nil {
		for _, stream := range probe.Streams {
			if stream.CodecType == "video" {
				hasVideo = true
			}
			if stream.CodecType == "audio" {
				hasAudio = true
			}
		}
	}
	output := p.Output.Path
	if output == "" {
		// дефолт: первый инпут + _out
		ext := ".mp4"
		// попытаемся угадать по transcode операции
		for _, op := range p.Operations {
			if t, ok := op.(*TranscodeOp); ok && t.Container != "" {
				ext = "." + strings.TrimPrefix(t.Container, ".")
				break
			}
		}
		copyOnly := len(p.Operations) > 0
		for _, op := range p.Operations {
			trim, ok := op.(*TrimOp)
			if !ok || trim.Mode != "copy" {
				copyOnly = false
				break
			}
		}
		if copyOnly && ext == ".mp4" && filepath.Ext(p.Inputs[0]) != "" {
			ext = filepath.Ext(p.Inputs[0])
		} else if probe != nil && hasAudio && !hasVideo && ext == ".mp4" {
			ext = ".m4a"
		}
		base := strings.TrimSuffix(p.Inputs[0], filepath.Ext(p.Inputs[0])) + "_pipelined"
		output = base + ext
		for n := 2; ; n++ {
			_, err := os.Stat(output)
			if os.IsNotExist(err) {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("check output %q: %w", output, err)
			}
			output = fmt.Sprintf("%s_%d%s", base, n, ext)
		}
	}
	// Безопасность: output не должен совпадать с input
	for _, inp := range p.Inputs {
		if strings.EqualFold(filepath.Clean(inp), filepath.Clean(output)) {
			return nil, fmt.Errorf("output %q overwrites input", output)
		}
	}
	if !p.Output.Overwrite {
		if _, err := os.Stat(output); err == nil {
			return nil, fmt.Errorf("output %q already exists", output)
		}
	}
	ctx := NewBuildContext(cfg, p.Inputs, output)
	ctx.InputHasVideo, ctx.InputHasAudio = probe == nil || hasVideo, probe == nil || hasAudio
	// Валидация + build каждой операции
	for _, op := range p.Operations {
		if err := op.Validate(probe); err != nil {
			return nil, fmt.Errorf("%s: %w", op.Type(), err)
		}
		if err := op.Build(ctx); err != nil {
			return nil, fmt.Errorf("%s build: %w", op.Type(), err)
		}
	}
	if probe != nil && !hasVideo {
		ctx.VideoCodec = ""
	}
	if probe != nil && !hasAudio {
		ctx.AudioCodec = ""
	}
	// Проверка совместимости фильтров с copy
	if err := ValidateFilterCompat(ctx.VideoCodec, ctx.AudioCodec, ctx.FilterGraph); err != nil {
		return nil, err
	}
	cmd := buildFFmpegArgs(ctx, p.Output.Overwrite)
	if len(ctx.Passes) > 0 {
		// многопроходные (напр. vidstab) — возвращаем все проходы + финальный
		all := append([][]string{}, ctx.Passes...)
		all = append(all, cmd)
		return all, nil
	}
	return [][]string{cmd}, nil
}

func buildFFmpegArgs(ctx *BuildContext, overwrite bool) []string {
	bin := FFmpegBin(ctx.Config)
	mode := "-n"
	if overwrite {
		mode = "-y"
	}
	args := []string{bin, mode}
	// seek / trim докидывают -ss/-to до -i сами; здесь базовые инпуты
	for _, inp := range ctx.Inputs {
		args = append(args, "-i", inp)
	}
	// фильтры
	if ctx.FilterGraph != nil && ctx.FilterGraph.HasFilters() {
		args = append(args, ctx.FilterGraph.BuildArgs()...)
	}
	// кодеки
	if ctx.VideoCodec != "" {
		if ctx.VideoCodec == "copy" {
			args = append(args, "-c:v", "copy")
		} else {
			args = append(args, "-c:v", ctx.VideoCodec)
		}
	}
	if ctx.AudioCodec != "" {
		if ctx.AudioCodec == "copy" {
			args = append(args, "-c:a", "copy")
		} else {
			args = append(args, "-c:a", ctx.AudioCodec)
		}
	}
	// extra args от операций
	args = append(args, ctx.ExtraArgs...)
	// маппинги по умолчанию: если не заданы явно — ffmpeg сам выберет
	args = append(args, ctx.Output)
	return args
}

// DryRunPipeline возвращает команды без выполнения.
func DryRunPipeline(p Pipeline, cfg Config) ([]string, error) {
	passes, err := BuildPipeline(p, cfg)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, cmd := range passes {
		out = append(out, strings.Join(MaskCmdForLog(cmd), " "))
	}
	return out, nil
}

// SupportedContainers — валидные контейнеры для транскодинга.
var SupportedContainers = map[string]bool{
	"mp4": true, "mkv": true, "mov": true, "webm": true,
	"avi": true, "m4v": true, "mp3": true, "flac": true,
	"wav": true, "m4a": true, "opus": true, "ogg": true,
}

// SupportedVideoCodecs
var SupportedVideoCodecs = map[string]bool{
	"libx264": true, "libx265": true, "libsvtav1": true, "libvpx-vp9": true,
	"prores_ks": true, "dnxhd": true, "copy": true, "libaom-av1": true, "h264": true, "hevc": true,
}

// SupportedAudioCodecs
var SupportedAudioCodecs = map[string]bool{
	"aac": true, "opus": true, "mp3": true, "flac": true, "pcm_s16le": true, "copy": true, "libopus": true, "libmp3lame": true,
}
