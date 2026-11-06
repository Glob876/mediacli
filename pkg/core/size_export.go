package core

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// BuildSizeExport reserves 4% for container overhead. It is an estimate, not a hard limit.
func BuildSizeExport(p Pipeline, cfg Config, megabytes float64, twoPass bool) ([][]string, error) {
	if len(p.Inputs) != 1 || megabytes <= 0 || math.IsNaN(megabytes) || math.IsInf(megabytes, 0) {
		return nil, fmt.Errorf("expected one input and a positive size")
	}
	info, err := ProbeMediaInfoWithConfig(p.Inputs[0], cfg)
	if err != nil {
		return nil, err
	}
	duration := info.Format.DurationSec
	hasVideo, hasAudio := false, false
	for _, stream := range info.Streams {
		hasVideo = hasVideo || stream.CodecType == "video"
		hasAudio = hasAudio || stream.CodecType == "audio"
	}
	if !hasVideo {
		return nil, fmt.Errorf("target-size export requires video")
	}
	var ops []Operation
	for _, op := range p.Operations {
		if trim, ok := op.(*TrimOp); ok {
			start, err := parseMediaTime(trim.Start)
			if err != nil {
				return nil, err
			}
			end := info.Format.DurationSec
			if trim.End != "" {
				end, err = parseMediaTime(trim.End)
				if err != nil {
					return nil, err
				}
			}
			if err := trim.Validate(info); err != nil {
				return nil, err
			}
			duration = math.Min(end, info.Format.DurationSec) - start
			ops = append(ops, &TrimOp{Start: trim.Start, End: trim.End, Mode: "reencode"})
		} else {
			switch value := op.(type) {
			case *TranscodeOp:
			case *AspectOp:
				aspect := *value
				aspect.Codec = ""
				ops = append(ops, &aspect)
			case *ScaleOp, *FpsOp, *FilterOp, *AudioOp, *SubtitleOp, *MetadataOp:
				ops = append(ops, op)
			default:
				return nil, fmt.Errorf("target-size export cannot combine operation %s", op.Type())
			}
		}
	}
	if duration <= 0 {
		return nil, fmt.Errorf("duration must be positive")
	}
	audioBits := 0.0
	if hasAudio {
		audioBits = 128000
	}
	videoBits := megabytes*1000000*8*0.96/duration - audioBits
	if videoBits < 32000 {
		return nil, fmt.Errorf("size too small for duration and 128 kbit/s audio")
	}
	ops = append(ops, &TranscodeOp{Container: "mp4", VCodec: "libx264", ACodec: "aac", BitrateV: strconv.FormatInt(int64(videoBits), 10), BitrateA: "128k", Preset: "medium", PixFmt: "yuv420p"})
	// Apply the bitrate encoder first so TrimOp does not inject its default CRF.
	encoder := ops[len(ops)-1]
	ops = append([]Operation{encoder}, ops[:len(ops)-1]...)
	p.Operations = ops
	commands, err := BuildPipeline(p, cfg)
	if err != nil {
		return nil, err
	}
	if len(commands) != 1 {
		return nil, fmt.Errorf("size export cannot combine other multi-pass operations")
	}
	if !twoPass {
		return commands, nil
	}
	logPath := filepath.Join(os.TempDir(), fmt.Sprintf("mediacli-pass-%d", time.Now().UnixNano()))
	final := commands[0]
	first := append([]string(nil), final[:len(final)-1]...)
	first = append(first, "-pass", "1", "-passlogfile", logPath, "-an", "-f", "null", os.DevNull)
	// -n would reject /dev/null. The first pass has no media output.
	first[1] = "-y"
	second := append([]string(nil), final[:len(final)-1]...)
	second = append(second, "-pass", "2", "-passlogfile", logPath, final[len(final)-1])
	return [][]string{first, second}, nil
}
