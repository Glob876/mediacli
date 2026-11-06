package core

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type MediaRange struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

func ValidateRanges(ranges []MediaRange, duration float64) error {
	if len(ranges) == 0 {
		return fmt.Errorf("select at least one range")
	}
	for _, r := range ranges {
		if math.IsNaN(r.Start) || math.IsNaN(r.End) || math.IsInf(r.Start, 0) || math.IsInf(r.End, 0) || r.Start < 0 || r.End <= r.Start || r.End > duration+0.001 {
			return fmt.Errorf("invalid range %.3f–%.3f (duration %.3f)", r.Start, r.End, duration)
		}
	}
	return nil
}

// Waveform returns peak levels for a 60-second window. Output is bounded independently of input length.
func Waveform(path string, cfg Config, start float64, columns int) ([]float64, error) {
	if columns < 1 || columns > 1000 {
		return nil, fmt.Errorf("invalid waveform width")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, FFmpegBin(cfg), "-v", "error", "-ss", fmt.Sprint(start), "-i", path, "-t", "60", "-vn", "-ac", "1", "-ar", "1000", "-f", "f32le", "pipe:1")
	pipe, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(pipe, 240001))
	waitErr := command.Wait()
	if err != nil {
		return nil, err
	}
	if waitErr != nil {
		return nil, waitErr
	}
	levels := make([]float64, columns)
	for i := 0; i+4 <= len(data); i += 4 {
		value := math.Abs(float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i : i+4]))))
		column := (i / 4) * columns / max(1, len(data)/4)
		if column < columns && value > levels[column] {
			levels[column] = value
		}
	}
	return levels, nil
}

// BuildRangeExport re-encodes for accurate cuts, supporting video-only and audio-only files.
func BuildRangeExport(path string, cfg Config, ranges []MediaRange, join bool) ([][]string, error) {
	info, err := ProbeMediaInfoWithConfig(path, cfg)
	if err != nil {
		return nil, err
	}
	if err := ValidateRanges(ranges, info.Format.DurationSec); err != nil {
		return nil, err
	}
	video, audio := false, false
	for _, stream := range info.Streams {
		video = video || stream.CodecType == "video"
		audio = audio || stream.CodecType == "audio"
	}
	if !video && !audio {
		return nil, fmt.Errorf("no audio or video streams")
	}
	extension := ".mp4"
	if !video {
		extension = ".m4a"
	}
	base := strings.TrimSuffix(path, filepath.Ext(path)) + fmt.Sprintf("_ranges_%d", time.Now().UnixNano())
	var commands [][]string
	if !join {
		for index, r := range ranges {
			args := []string{FFmpegBin(cfg), "-n", "-i", path, "-ss", fmt.Sprint(r.Start), "-t", fmt.Sprint(r.End - r.Start)}
			if video {
				args = append(args, "-map", "0:v:0", "-c:v", "libx264", "-pix_fmt", "yuv420p")
			}
			if audio {
				args = append(args, "-map", "0:a:0", "-c:a", "aac")
			}
			args = append(args, fmt.Sprintf("%s_%02d%s", base, index+1, extension))
			commands = append(commands, args)
		}
		return commands, nil
	}
	var graph, labels []string
	for index, r := range ranges {
		if video {
			graph = append(graph, fmt.Sprintf("[0:v:0]trim=start=%g:end=%g,setpts=PTS-STARTPTS[v%d]", r.Start, r.End, index))
			labels = append(labels, fmt.Sprintf("[v%d]", index))
		}
		if audio {
			graph = append(graph, fmt.Sprintf("[0:a:0]atrim=start=%g:end=%g,asetpts=PTS-STARTPTS[a%d]", r.Start, r.End, index))
			labels = append(labels, fmt.Sprintf("[a%d]", index))
		}
	}
	v, a := 0, 0
	outputLabels := ""
	if video {
		v = 1
		outputLabels += "[v]"
	}
	if audio {
		a = 1
		outputLabels += "[a]"
	}
	graph = append(graph, fmt.Sprintf("%sconcat=n=%d:v=%d:a=%d%s", strings.Join(labels, ""), len(ranges), v, a, outputLabels))
	args := []string{FFmpegBin(cfg), "-n", "-i", path, "-filter_complex", strings.Join(graph, ";")}
	if video {
		args = append(args, "-map", "[v]", "-c:v", "libx264", "-pix_fmt", "yuv420p")
	}
	if audio {
		args = append(args, "-map", "[a]", "-c:a", "aac")
	}
	args = append(args, base+extension)
	return [][]string{args}, nil
}

// CleanupPassLogs removes only FFmpeg's known statistics files after a second pass.
func CleanupPassLogs(command []string) {
	second := false
	prefix := ""
	for index, arg := range command {
		if index+1 >= len(command) {
			continue
		}
		if arg == "-pass" && command[index+1] == "2" {
			second = true
		}
		if arg == "-passlogfile" {
			prefix = command[index+1]
		}
	}
	if !second || !strings.HasPrefix(filepath.Base(prefix), "mediacli-pass-") {
		return
	}
	for _, suffix := range []string{"-0.log", "-0.log.mbtree"} {
		_ = os.Remove(prefix + suffix)
	}
}
