package core

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

// ================= Transcode =================

type TranscodeOp struct {
	Container string   `json:"container,omitempty"` // mp4/mkv/mov/webm etc
	VCodec    string   `json:"vcodec,omitempty"`    // libx264, copy, etc
	ACodec    string   `json:"acodec,omitempty"`
	CRF       string   `json:"crf,omitempty"`
	Preset    string   `json:"preset,omitempty"`
	BitrateV  string   `json:"bitrate_v,omitempty"` // 2M, 500k
	BitrateA  string   `json:"bitrate_a,omitempty"`
	PixFmt    string   `json:"pix_fmt,omitempty"`
	Profile   string   `json:"profile,omitempty"`
	MaxRate   string   `json:"maxrate,omitempty"`
	BufSize   string   `json:"bufsize,omitempty"`
	GOP       string   `json:"gop,omitempty"`
	Extra     []string `json:"extra,omitempty"`
	Remux     bool     `json:"remux,omitempty"` // -c copy
}

func (o *TranscodeOp) Type() string { return "transcode" }
func (o *TranscodeOp) Describe() string {
	if o.Remux {
		return fmt.Sprintf("Remux to %s (copy)", o.Container)
	}
	return fmt.Sprintf("Transcode %s v:%s a:%s crf:%s", o.Container, o.VCodec, o.ACodec, o.CRF)
}
func (o *TranscodeOp) Validate(info *MediaInfo) error {
	if o.Container != "" && !SupportedContainers[strings.ToLower(strings.TrimPrefix(o.Container, "."))] {
		return fmt.Errorf("unsupported container %q", o.Container)
	}
	if o.VCodec != "" && !SupportedVideoCodecs[o.VCodec] {
		// allow unknown but warn? strict for now
		// return fmt.Errorf("unsupported vcodec %q", o.VCodec)
	}
	if o.ACodec != "" && !SupportedAudioCodecs[o.ACodec] {
		// same
	}
	return nil
}
func (o *TranscodeOp) Build(ctx *BuildContext) error {
	if o.Remux || (o.VCodec == "copy" && o.ACodec == "copy") {
		ctx.VideoCodec = "copy"
		ctx.AudioCodec = "copy"
		return nil
	}
	if o.VCodec != "" {
		ctx.VideoCodec = o.VCodec
	}
	if o.ACodec != "" {
		ctx.AudioCodec = o.ACodec
	}
	if o.CRF != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-crf", o.CRF)
	}
	if o.Preset != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-preset", o.Preset)
	}
	if o.BitrateV != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-b:v", o.BitrateV)
	}
	if o.BitrateA != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-b:a", o.BitrateA)
	}
	if o.PixFmt != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-pix_fmt", o.PixFmt)
	}
	if o.Profile != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-profile:v", o.Profile)
	}
	if o.MaxRate != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-maxrate", o.MaxRate)
	}
	if o.BufSize != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-bufsize", o.BufSize)
	}
	if o.GOP != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-g", o.GOP)
	}
	ctx.ExtraArgs = append(ctx.ExtraArgs, o.Extra...)
	return nil
}

// ================= Aspect & Geometry =================

type AspectOp struct {
	RatioW     int    `json:"ratio_w"`
	RatioH     int    `json:"ratio_h"`
	Mode       string `json:"mode"`                  // crop, letterbox, blur, stretch, rotate, flip
	SizePreset string `json:"size_preset,omitempty"` // auto, 1080p, 720p, 4k
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	Rotate     int    `json:"rotate,omitempty"` // 0,90,180,270
	FlipH      bool   `json:"flip_h,omitempty"`
	FlipV      bool   `json:"flip_v,omitempty"`
	Codec      string `json:"codec,omitempty"` // h264, hevc, prores, dnxhr
	PadColor   string `json:"pad_color,omitempty"`
	ScaleAlgo  string `json:"scale_algo,omitempty"`
}

func (o *AspectOp) Type() string { return "aspect" }
func (o *AspectOp) Describe() string {
	return fmt.Sprintf("Aspect %d:%d %s %dx%d", o.RatioW, o.RatioH, o.Mode, o.Width, o.Height)
}
func (o *AspectOp) Validate(info *MediaInfo) error {
	if o.RatioW <= 0 || o.RatioH <= 0 {
		return fmt.Errorf("ratio must be positive")
	}
	if info != nil {
		hasVideo := false
		for _, stream := range info.Streams {
			if stream.CodecType == "video" {
				hasVideo = true
				break
			}
		}
		if !hasVideo {
			return fmt.Errorf("aspect ratio requires a video stream")
		}
	}
	switch o.Mode {
	case "crop", "letterbox", "blur", "stretch", "rotate", "flip", "":
	default:
		return fmt.Errorf("unknown aspect mode %q", o.Mode)
	}
	return nil
}
func (o *AspectOp) Build(ctx *BuildContext) error {
	// Determine target dimensions
	outW, outH := o.Width, o.Height
	if outW == 0 && outH == 0 {
		// derive from ratio and size preset
		baseH := 1080
		switch o.SizePreset {
		case "720p":
			baseH = 720
		case "4k":
			baseH = 2160
		case "1080p":
			baseH = 1080
		case "auto", "":
			baseH = 1080
		}
		if v := strings.TrimSuffix(o.SizePreset, "p"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				baseH = n
			}
		}
		ratio := float64(o.RatioW) / float64(o.RatioH)
		if o.RatioW >= o.RatioH {
			outH = baseH
			outW = int(float64(outH)*ratio + 0.5)
			if outW%2 != 0 {
				outW++
			}
		} else {
			outW = baseH
			outH = int(float64(outW)/ratio + 0.5)
			if outH%2 != 0 {
				outH++
			}
		}
	}
	// Ensure even
	if outW%2 != 0 {
		outW++
	}
	if outH%2 != 0 {
		outH++
	}
	mode := o.Mode
	if mode == "" {
		mode = "crop"
	}
	switch mode {
	case "crop":
		ctx.FilterGraph.AddVideoFilter(fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,setsar=1", outW, outH, outW, outH))
	case "letterbox":
		color := o.PadColor
		if color == "" {
			color = "black"
		}
		ctx.FilterGraph.AddVideoFilter(fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=%s,setsar=1", outW, outH, outW, outH, color))
	case "blur":
		fc := fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,boxblur=25:5[bg];[0:v]scale=%d:%d:force_original_aspect_ratio=decrease[fg];[bg][fg]overlay=(W-w)/2:(H-h)/2,setsar=1", outW, outH, outW, outH, outW, outH)
		ctx.FilterGraph.AddComplex(fc)
	case "stretch":
		ctx.FilterGraph.AddVideoFilter(fmt.Sprintf("scale=%d:%d,setsar=1", outW, outH))
	}
	// Rotate / Flip
	if o.Rotate != 0 {
		switch o.Rotate {
		case 90:
			ctx.FilterGraph.AddVideoFilter("transpose=1")
		case 180:
			ctx.FilterGraph.AddVideoFilter("transpose=1,transpose=1")
		case 270:
			ctx.FilterGraph.AddVideoFilter("transpose=2")
		default:
			ctx.FilterGraph.AddVideoFilter(fmt.Sprintf("rotate=%d*PI/180", o.Rotate))
		}
	}
	if o.FlipH {
		ctx.FilterGraph.AddVideoFilter("hflip")
	}
	if o.FlipV {
		ctx.FilterGraph.AddVideoFilter("vflip")
	}
	// Codec
	switch o.Codec {
	case "hevc", "h265":
		ctx.VideoCodec = "libx265"
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-pix_fmt", "yuv420p", "-preset", "medium", "-crf", "23")
	case "prores":
		ctx.VideoCodec = "prores_ks"
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-profile:v", "3")
	case "dnxhr":
		ctx.VideoCodec = "dnxhd"
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-profile:v", "dnxhr_hq")
	case "h264", "":
		if ctx.VideoCodec == "copy" {
			ctx.VideoCodec = "libx264"
			ctx.ExtraArgs = append(ctx.ExtraArgs, "-pix_fmt", "yuv420p", "-preset", "medium", "-crf", "20")
		}
	default:
		ctx.VideoCodec = o.Codec
	}
	if ctx.AudioCodec == "copy" {
		// keep copy unless re-encode needed elsewhere
	}
	return nil
}

// ================= Mux / Demux =================

type MuxOp struct {
	ExtraInputs []string `json:"extra_inputs"`
	Maps        []string `json:"maps,omitempty"` // e.g. "0:v:0", "1:a:0"
	Shortest    bool     `json:"shortest,omitempty"`
	MixAudio    bool     `json:"mix_audio,omitempty"` // amix two audios into one
	VideoCodec  string   `json:"vcodec,omitempty"`
	AudioCodec  string   `json:"acodec,omitempty"`
}

func (o *MuxOp) Type() string { return "mux" }
func (o *MuxOp) Describe() string {
	return fmt.Sprintf("Mux %d extra inputs", len(o.ExtraInputs))
}
func (o *MuxOp) Validate(info *MediaInfo) error {
	if len(o.ExtraInputs) == 0 {
		return fmt.Errorf("mux requires at least one extra input")
	}
	return nil
}
func (o *MuxOp) Build(ctx *BuildContext) error {
	// Append extra inputs
	ctx.Inputs = append(ctx.Inputs, o.ExtraInputs...)
	if len(o.Maps) > 0 {
		for _, m := range o.Maps {
			ctx.ExtraArgs = append(ctx.ExtraArgs, "-map", m)
		}
	} else {
		// default: map first video, and all audios
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-map", "0:v:0?", "-map", "1:a:0?")
		// if more inputs, map them too
		for i := 2; i < len(ctx.Inputs); i++ {
			ctx.ExtraArgs = append(ctx.ExtraArgs, "-map", fmt.Sprintf("%d:a:0?", i))
		}
	}
	if o.Shortest {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-shortest")
	}
	if o.MixAudio && len(ctx.Inputs) >= 2 {
		// amix filter_complex
		ctx.FilterGraph.AddComplex("[0:a][1:a]amix=inputs=2:duration=longest[aout]")
		ctx.FilterGraph.SetComplexOutput("[aout]")
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-map", "0:v:0?")
	}
	vc := o.VideoCodec
	if vc == "" {
		vc = ctx.VideoCodec
	}
	ac := o.AudioCodec
	if ac == "" {
		ac = ctx.AudioCodec
	}
	ctx.VideoCodec = vc
	ctx.AudioCodec = ac
	return nil
}

type DemuxOp struct {
	StreamIndex int    `json:"stream_index"` // which stream to extract
	StreamType  string `json:"stream_type"`  // audio/video/subtitle
	OutputExt   string `json:"output_ext,omitempty"`
	Codec       string `json:"codec,omitempty"`
}

func (o *DemuxOp) Type() string     { return "demux" }
func (o *DemuxOp) Describe() string { return fmt.Sprintf("Demux %s:%d", o.StreamType, o.StreamIndex) }
func (o *DemuxOp) Validate(info *MediaInfo) error {
	if o.StreamType != "audio" && o.StreamType != "video" && o.StreamType != "subtitle" {
		return fmt.Errorf("unknown stream_type %q", o.StreamType)
	}
	return nil
}
func (o *DemuxOp) Build(ctx *BuildContext) error {
	switch o.StreamType {
	case "audio":
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-map", fmt.Sprintf("0:a:%d", o.StreamIndex), "-vn")
		if o.Codec != "" {
			ctx.AudioCodec = o.Codec
		} else {
			ctx.AudioCodec = "copy"
		}
		ctx.VideoCodec = "" // no video
	case "video":
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-map", fmt.Sprintf("0:v:%d", o.StreamIndex), "-an")
		if o.Codec != "" {
			ctx.VideoCodec = o.Codec
		} else {
			ctx.VideoCodec = "copy"
		}
		ctx.AudioCodec = ""
	case "subtitle":
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-map", fmt.Sprintf("0:s:%d", o.StreamIndex))
		ctx.VideoCodec = ""
		ctx.AudioCodec = ""
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-c:s", "srt")
	}
	return nil
}

// ================= Trim =================

type TrimOp struct {
	Start string `json:"start"` // HH:MM:SS.ms or seconds
	End   string `json:"end"`   // HH:MM:SS.ms or seconds (to)
	Mode  string `json:"mode"`  // copy|reencode
}

func (o *TrimOp) Type() string     { return "trim" }
func (o *TrimOp) Describe() string { return fmt.Sprintf("Trim %s - %s (%s)", o.Start, o.End, o.Mode) }
func (o *TrimOp) Validate(info *MediaInfo) error {
	if strings.TrimSpace(o.Start) == "" && strings.TrimSpace(o.End) == "" {
		return fmt.Errorf("trim requires start or end")
	}
	if o.Mode != "" && o.Mode != "copy" && o.Mode != "reencode" {
		return fmt.Errorf("unknown trim mode %q", o.Mode)
	}
	start, end := 0.0, 0.0
	var err error
	if strings.TrimSpace(o.Start) != "" {
		start, err = parseMediaTime(o.Start)
		if err != nil {
			return fmt.Errorf("invalid start: %w", err)
		}
	}
	if strings.TrimSpace(o.End) != "" {
		end, err = parseMediaTime(o.End)
		if err != nil {
			return fmt.Errorf("invalid end: %w", err)
		}
		if end <= start {
			return fmt.Errorf("end must be after start")
		}
	}
	return nil
}
func (o *TrimOp) Build(ctx *BuildContext) error {
	// Output-side seek is frame accurate when re-encoding. -t is duration,
	// so End remains an absolute position even when Start is nonzero.
	if s := strings.TrimSpace(o.Start); s != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-ss", s)
	}
	if e := strings.TrimSpace(o.End); e != "" {
		end, _ := parseMediaTime(e)
		start := 0.0
		if o.Start != "" {
			start, _ = parseMediaTime(o.Start)
		}
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-t", strconv.FormatFloat(end-start, 'f', 3, 64))
	}
	if o.Mode == "copy" {
		ctx.VideoCodec = "copy"
		ctx.AudioCodec = "copy"
	} else {
		if ctx.InputHasVideo && ctx.VideoCodec == "copy" {
			ctx.VideoCodec = "libx264"
			ctx.ExtraArgs = append(ctx.ExtraArgs, "-preset", "medium", "-crf", "20")
		}
		if ctx.InputHasAudio && ctx.AudioCodec == "copy" {
			ctx.AudioCodec = "aac"
		}
	}
	return nil
}

func parseMediaTime(raw string) (float64, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) < 1 || len(parts) > 3 {
		return 0, fmt.Errorf("expected seconds or HH:MM:SS")
	}
	total := 0.0
	for _, part := range parts {
		value, err := strconv.ParseFloat(part, 64)
		if err != nil || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, fmt.Errorf("expected nonnegative time")
		}
		total = total*60 + value
	}
	return total, nil
}

// ================= Concat =================

type ConcatOp struct {
	Inputs []string `json:"inputs"`          // list of files to concat
	Mode   string   `json:"mode"`            // demuxer|filter
	Codec  string   `json:"codec,omitempty"` // for filter mode
}

func (o *ConcatOp) Type() string { return "concat" }
func (o *ConcatOp) Describe() string {
	return fmt.Sprintf("Concat %d files (%s)", len(o.Inputs), o.Mode)
}
func (o *ConcatOp) Validate(info *MediaInfo) error {
	if len(o.Inputs) < 2 {
		return fmt.Errorf("concat requires at least 2 inputs")
	}
	if o.Mode != "" && o.Mode != "demuxer" && o.Mode != "filter" {
		return fmt.Errorf("unknown concat mode %q", o.Mode)
	}
	return nil
}
func (o *ConcatOp) Build(ctx *BuildContext) error {
	mode := o.Mode
	if mode == "" {
		mode = "demuxer"
	}
	if mode == "demuxer" {
		// Use concat demuxer: need to create list file; but for pipeline we just append inputs and use filter?
		// Simplified: use filter_complex concat for both modes, demuxer would be separate task.
		// For demuxer we enqueue a single ffmpeg with -f concat -safe 0 -i list.txt -c copy
		// Here we just set inputs and copy codecs.
		ctx.Inputs = append([]string{}, o.Inputs...)
		ctx.VideoCodec = "copy"
		ctx.AudioCodec = "copy"
		// Caller (daemon) will generate list file and use concat demuxer; for generic pipeline fallback to filter
	} else {
		// filter mode: concat=n=N:v=1:a=1
		ctx.Inputs = append([]string{}, o.Inputs...)
		n := len(ctx.Inputs)
		var parts []string
		for i := 0; i < n; i++ {
			parts = append(parts, fmt.Sprintf("[%d:v][%d:a]", i, i))
		}
		fc := fmt.Sprintf("%sconcat=n=%d:v=1:a=1[v][a]", strings.Join(parts, ""), n)
		ctx.FilterGraph.AddComplex(fc)
		ctx.FilterGraph.SetComplexOutput("[v]")
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-map", "[a]")
		if o.Codec != "" {
			ctx.VideoCodec = o.Codec
		} else {
			ctx.VideoCodec = "libx264"
		}
		ctx.AudioCodec = "aac"
	}
	return nil
}

// ================= Filter (generic) =================

type FilterOp struct {
	VF      string `json:"vf,omitempty"`      // comma-separated -vf
	AF      string `json:"af,omitempty"`      // comma-separated -af
	Complex string `json:"complex,omitempty"` // filter_complex
}

func (o *FilterOp) Type() string { return "filter" }
func (o *FilterOp) Describe() string {
	if o.Complex != "" {
		return "Filter complex: " + o.Complex
	}
	return fmt.Sprintf("Filter vf:%s af:%s", o.VF, o.AF)
}
func (o *FilterOp) Validate(info *MediaInfo) error {
	if o.VF == "" && o.AF == "" && o.Complex == "" {
		return fmt.Errorf("filter requires vf, af or complex")
	}
	return nil
}
func (o *FilterOp) Build(ctx *BuildContext) error {
	if o.VF != "" {
		for _, f := range strings.Split(o.VF, ",") {
			ctx.FilterGraph.AddVideoFilter(strings.TrimSpace(f))
		}
	}
	if o.AF != "" {
		for _, f := range strings.Split(o.AF, ",") {
			ctx.FilterGraph.AddAudioFilter(strings.TrimSpace(f))
		}
	}
	if o.Complex != "" {
		ctx.FilterGraph.AddComplex(o.Complex)
	}
	// Generic filters imply re-encode if video filters present
	if o.VF != "" || o.Complex != "" {
		if ctx.VideoCodec == "copy" {
			ctx.VideoCodec = "libx264"
			ctx.ExtraArgs = append(ctx.ExtraArgs, "-preset", "medium", "-crf", "20")
		}
	}
	if o.AF != "" && ctx.AudioCodec == "copy" {
		ctx.AudioCodec = "aac"
	}
	return nil
}

// ================= Audio =================

type AudioOp struct {
	Volume        *float64 `json:"volume,omitempty"`      // 0.5 = -6dB, 2.0 = +6dB
	Normalize     bool     `json:"normalize,omitempty"`   // loudnorm
	Tempo         string   `json:"tempo,omitempty"`       // atempo 0.5-2.0
	SampleRate    string   `json:"sample_rate,omitempty"` // 48000
	Channels      string   `json:"channels,omitempty"`    // 1,2
	RemoveSilence bool     `json:"remove_silence,omitempty"`
	FadeIn        string   `json:"fade_in,omitempty"` // 2s
	FadeOut       string   `json:"fade_out,omitempty"`
	Acodec        string   `json:"acodec,omitempty"`
	Bitrate       string   `json:"bitrate,omitempty"`
}

func (o *AudioOp) Type() string                   { return "audio" }
func (o *AudioOp) Describe() string               { return "Audio processing" }
func (o *AudioOp) Validate(info *MediaInfo) error { return nil }
func (o *AudioOp) Build(ctx *BuildContext) error {
	if o.Volume != nil {
		ctx.FilterGraph.AddAudioFilter(fmt.Sprintf("volume=%g", *o.Volume))
	}
	if o.Normalize {
		ctx.FilterGraph.AddAudioFilter("loudnorm=I=-16:TP=-1.5:LRA=11")
	}
	if o.Tempo != "" {
		ctx.FilterGraph.AddAudioFilter(fmt.Sprintf("atempo=%s", o.Tempo))
	}
	if o.RemoveSilence {
		ctx.FilterGraph.AddAudioFilter("silenceremove=start_periods=1:start_duration=0.5:start_threshold=-60dB:detection=peak:audibility=1:window=0.02")
	}
	if o.FadeIn != "" {
		ctx.FilterGraph.AddAudioFilter(fmt.Sprintf("afade=t=in:st=0:d=%s", o.FadeIn))
	}
	if o.FadeOut != "" {
		// need duration; use generic fade out at end — use afade t=out
		ctx.FilterGraph.AddAudioFilter(fmt.Sprintf("afade=t=out:st=0:d=%s", o.FadeOut))
	}
	if o.SampleRate != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-ar", o.SampleRate)
	}
	if o.Channels != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-ac", o.Channels)
	}
	if o.Acodec != "" {
		ctx.AudioCodec = o.Acodec
	} else if ctx.FilterGraph.HasFilters() || o.Volume != nil || o.Normalize {
		if ctx.AudioCodec == "copy" {
			ctx.AudioCodec = "aac"
		}
	}
	if o.Bitrate != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-b:a", o.Bitrate)
	}
	return nil
}

// ================= Subtitle =================

type SubtitleOp struct {
	Mode     string `json:"mode"`            // extract, embed, burn
	Input    string `json:"input,omitempty"` // srt file for embed/burn
	Language string `json:"language,omitempty"`
	Index    int    `json:"index,omitempty"`
	Force    bool   `json:"force,omitempty"`
	Style    string `json:"style,omitempty"` // ASS style for burn
}

func (o *SubtitleOp) Type() string     { return "subtitle" }
func (o *SubtitleOp) Describe() string { return "Subtitle " + o.Mode }
func (o *SubtitleOp) Validate(info *MediaInfo) error {
	switch o.Mode {
	case "extract", "embed", "burn":
	default:
		return fmt.Errorf("unknown subtitle mode %q", o.Mode)
	}
	return nil
}
func (o *SubtitleOp) Build(ctx *BuildContext) error {
	switch o.Mode {
	case "extract":
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-map", fmt.Sprintf("0:s:%d", o.Index))
		ctx.VideoCodec = ""
		ctx.AudioCodec = ""
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-c:s", "srt")
	case "embed":
		if o.Input != "" {
			ctx.Inputs = append(ctx.Inputs, o.Input)
			ctx.ExtraArgs = append(ctx.ExtraArgs, "-map", "0", "-map", "1", "-c", "copy", "-c:s", "mov_text")
			if o.Language != "" {
				ctx.ExtraArgs = append(ctx.ExtraArgs, "-metadata:s:s:0", "language="+o.Language)
			}
			ctx.VideoCodec = ""
			ctx.AudioCodec = ""
		} else {
			ctx.ExtraArgs = append(ctx.ExtraArgs, "-c:s", "mov_text")
		}
	case "burn":
		if o.Input == "" {
			return fmt.Errorf("burn requires input srt")
		}
		escaped := strings.ReplaceAll(o.Input, ":", "\\:")
		style := ""
		if o.Style != "" {
			style = ":force_style=" + o.Style
		}
		ctx.FilterGraph.AddVideoFilter(fmt.Sprintf("subtitles=%s%s", escaped, style))
		if ctx.VideoCodec == "copy" {
			ctx.VideoCodec = "libx264"
		}
	}
	return nil
}

// ================= Metadata =================

type MetadataOp struct {
	Title    string            `json:"title,omitempty"`
	Artist   string            `json:"artist,omitempty"`
	Album    string            `json:"album,omitempty"`
	Comment  string            `json:"comment,omitempty"`
	Extra    map[string]string `json:"extra,omitempty"`
	StripAll bool              `json:"strip_all,omitempty"`
	Cover    string            `json:"cover,omitempty"` // path to image
}

func (o *MetadataOp) Type() string                   { return "metadata" }
func (o *MetadataOp) Describe() string               { return "Metadata" }
func (o *MetadataOp) Validate(info *MediaInfo) error { return nil }
func (o *MetadataOp) Build(ctx *BuildContext) error {
	if o.StripAll {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-map_metadata", "-1")
	}
	if o.Title != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-metadata", "title="+o.Title)
	}
	if o.Artist != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-metadata", "artist="+o.Artist)
	}
	if o.Album != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-metadata", "album="+o.Album)
	}
	if o.Comment != "" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-metadata", "comment="+o.Comment)
	}
	for k, v := range o.Extra {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-metadata", k+"="+v)
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-metadata:s:v:0", k+"="+v)
		// generic; dedup later
		_ = filepath.Base(k)
	}
	if o.Cover != "" {
		ctx.Inputs = append(ctx.Inputs, o.Cover)
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-map", "0", "-map", "1", "-c", "copy", "-disposition:v:1", "attached_pic")
		ctx.VideoCodec = ""
		ctx.AudioCodec = ""
	}
	return nil
}

// ================= Scale (standalone) =================

type ScaleOp struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Algo   string `json:"algo,omitempty"` // lanczos, bilinear
}

func (o *ScaleOp) Type() string     { return "scale" }
func (o *ScaleOp) Describe() string { return fmt.Sprintf("Scale %dx%d", o.Width, o.Height) }
func (o *ScaleOp) Validate(info *MediaInfo) error {
	if o.Width <= 0 && o.Height <= 0 {
		return fmt.Errorf("scale requires width or height")
	}
	return nil
}
func (o *ScaleOp) Build(ctx *BuildContext) error {
	algo := o.Algo
	if algo == "" {
		algo = "lanczos"
	}
	ws, hs := "-1", "-1"
	if o.Width > 0 {
		ws = strconv.Itoa(o.Width)
	}
	if o.Height > 0 {
		hs = strconv.Itoa(o.Height)
	}
	ctx.FilterGraph.AddVideoFilter(fmt.Sprintf("scale=%s:%s:flags=%s", ws, hs, algo))
	if ctx.VideoCodec == "copy" {
		ctx.VideoCodec = "libx264"
	}
	return nil
}

// ================= FPS =================

type FpsOp struct {
	FPS         string `json:"fps"`                   // 30, 60
	Mode        string `json:"mode,omitempty"`        // fps filter vs -r
	Interpolate bool   `json:"interpolate,omitempty"` // minterpolate
}

func (o *FpsOp) Type() string     { return "fps" }
func (o *FpsOp) Describe() string { return "FPS " + o.FPS }
func (o *FpsOp) Validate(info *MediaInfo) error {
	if o.FPS == "" {
		return fmt.Errorf("fps required")
	}
	return nil
}
func (o *FpsOp) Build(ctx *BuildContext) error {
	if o.Interpolate {
		ctx.FilterGraph.AddVideoFilter(fmt.Sprintf("minterpolate=fps=%s", o.FPS))
	} else {
		ctx.FilterGraph.AddVideoFilter(fmt.Sprintf("fps=%s", o.FPS))
	}
	if o.Mode == "r" {
		ctx.ExtraArgs = append(ctx.ExtraArgs, "-r", o.FPS)
	}
	if ctx.VideoCodec == "copy" {
		ctx.VideoCodec = "libx264"
	}
	return nil
}

// ================= Stabilize (vidstab) =================

type StabilizeOp struct {
	Shakiness int    `json:"shakiness,omitempty"`
	Accuracy  int    `json:"accuracy,omitempty"`
	Output    string `json:"output,omitempty"`
}

func (o *StabilizeOp) Type() string                   { return "stabilize" }
func (o *StabilizeOp) Describe() string               { return "Stabilize vidstab" }
func (o *StabilizeOp) Validate(info *MediaInfo) error { return nil }
func (o *StabilizeOp) Build(ctx *BuildContext) error {
	shakiness := o.Shakiness
	if shakiness == 0 {
		shakiness = 10
	}
	accuracy := o.Accuracy
	if accuracy == 0 {
		accuracy = 15
	}
	trf := "/tmp/vidstab_" + filepath.Base(ctx.Inputs[0]) + ".trf"
	pass1 := []string{FFmpegBin(ctx.Config), "-y", "-i", ctx.Inputs[0], "-vf", fmt.Sprintf("vidstabdetect=shakiness=%d:accuracy=%d:result=%s", shakiness, accuracy, trf), "-f", "null", "-"}
	ctx.Passes = append(ctx.Passes, pass1)
	ctx.FilterGraph.AddVideoFilter(fmt.Sprintf("vidstabtransform=input=%s", trf))
	if ctx.VideoCodec == "copy" {
		ctx.VideoCodec = "libx264"
	}
	return nil
}
