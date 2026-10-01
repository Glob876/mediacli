package core

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPipelineTrimAndAspectWithFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe unavailable")
	}
	dir := t.TempDir()
	video := filepath.Join(dir, "source.mp4")
	audio := filepath.Join(dir, "source.wav")
	for _, args := range [][]string{
		{"-f", "lavfi", "-i", "color=c=red:s=160x90:r=10:d=3", "-c:v", "mpeg4", video},
		{"-f", "lavfi", "-i", "sine=frequency=440:duration=3", "-c:a", "pcm_s16le", audio},
	} {
		cmd := exec.Command("ffmpeg", append([]string{"-loglevel", "error", "-y"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture: %v: %s", err, out)
		}
	}
	cfg := GetDefaultConfig()
	for _, tc := range []struct {
		name      string
		input     string
		ops       []Operation
		wantExt   string
		wantWidth int
	}{
		{"video crop", video, []Operation{&TrimOp{Start: "1", End: "2", Mode: "reencode"}, &AspectOp{RatioW: 9, RatioH: 16, Mode: "crop", Width: 90, Height: 160}}, ".mp4", 90},
		{"video bars", video, []Operation{&AspectOp{RatioW: 9, RatioH: 16, Mode: "letterbox", Width: 90, Height: 160}}, ".mp4", 90},
		{"audio trim", audio, []Operation{&TrimOp{Start: "0.5", End: "1.5", Mode: "reencode"}}, ".m4a", 0},
		{"audio fast trim", audio, []Operation{&TrimOp{Start: "0.5", End: "1.5", Mode: "copy"}}, ".wav", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmds, err := BuildPipeline(Pipeline{Inputs: []string{tc.input}, Operations: tc.ops}, cfg)
			if err != nil {
				t.Fatal(err)
			}
			args := cmds[0]
			if !strings.HasSuffix(args[len(args)-1], tc.wantExt) {
				t.Fatalf("wrong output extension: %v", args)
			}
			cmd := exec.Command(args[0], args[1:]...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("ffmpeg: %v: %s", err, out)
			}
			info, err := ProbeMediaInfoWithConfig(args[len(args)-1], cfg)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantWidth > 0 {
				found := false
				for _, st := range info.Streams {
					if st.CodecType == "video" && st.Width == tc.wantWidth && st.Height == 160 {
						found = true
					}
				}
				if !found {
					t.Fatalf("unexpected video dimensions: %+v", info.Streams)
				}
			} else if info.Format.DurationSec < 0.8 || info.Format.DurationSec > 1.2 {
				t.Fatalf("audio trim duration: %f", info.Format.DurationSec)
			}
		})
	}
}

func TestBuildPipeline_Transcode(t *testing.T) {
	cfg := GetDefaultConfig()
	p := Pipeline{
		Inputs: []string{"/tmp/in.mp4"},
		Operations: []Operation{
			&TranscodeOp{VCodec: "libx264", ACodec: "aac", CRF: "18", Preset: "medium", Container: "mp4"},
		},
		Output: OutputSpec{Path: "/tmp/out.mp4"},
	}
	cmds, err := BuildPipeline(p, cfg)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("want 1 cmd, got %d", len(cmds))
	}
	// must contain -c:v libx264 and -crf 18
	foundV, foundCRF := false, false
	for i, a := range cmds[0] {
		if a == "-c:v" && i+1 < len(cmds[0]) && cmds[0][i+1] == "libx264" {
			foundV = true
		}
		if a == "-crf" && i+1 < len(cmds[0]) && cmds[0][i+1] == "18" {
			foundCRF = true
		}
	}
	if !foundV || !foundCRF {
		t.Fatalf("transcode args missing: %v", cmds[0])
	}
}

func TestBuildPipeline_AspectBlur(t *testing.T) {
	cfg := GetDefaultConfig()
	p := Pipeline{
		Inputs: []string{"/tmp/in.mp4"},
		Operations: []Operation{
			&AspectOp{RatioW: 9, RatioH: 16, Mode: "blur", SizePreset: "720p"},
		},
		Output: OutputSpec{Path: "/tmp/out.mp4"},
	}
	cmds, err := BuildPipeline(p, cfg)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	hasComplex := false
	for _, a := range cmds[0] {
		if a == "-filter_complex" {
			hasComplex = true
		}
	}
	if !hasComplex {
		t.Fatalf("blur mode must produce -filter_complex, got %v", cmds[0])
	}
}

func TestBuildPipeline_CopyWithFilterPromotes(t *testing.T) {
	cfg := GetDefaultConfig()
	p := Pipeline{
		Inputs: []string{"/tmp/in.mp4"},
		Operations: []Operation{
			&TranscodeOp{VCodec: "copy", ACodec: "copy"},
			&FilterOp{VF: "hflip"},
		},
		Output: OutputSpec{Path: "/tmp/out.mp4"},
	}
	cmds, err := BuildPipeline(p, cfg)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	// filter must have promoted vcodec from copy to libx264
	hasHflip := false
	for _, a := range cmds[0] {
		if a == "hflip" || a == "-vf" {
			hasHflip = true
		}
		if a == "hflip,eq" {
			hasHflip = true
		}
	}
	_ = hasHflip
	foundCopy := false
	for i, a := range cmds[0] {
		if a == "-c:v" && i+1 < len(cmds[0]) && cmds[0][i+1] == "copy" {
			foundCopy = true
		}
	}
	if foundCopy {
		t.Fatalf("filter should promote copy to libx264, got %v", cmds[0])
	}
}

func TestValidateFilterCompatFails(t *testing.T) {
	g := NewFilterGraph()
	g.AddVideoFilter("hflip")
	if err := ValidateFilterCompat("copy", "copy", g); err == nil {
		t.Fatalf("expected error for copy+video filter")
	}
}

func TestFilterGraph_Build(t *testing.T) {
	g := NewFilterGraph()
	g.AddVideoFilter("hflip")
	g.AddVideoFilter("eq=brightness=0.1")
	args := g.BuildArgs()
	if len(args) != 2 || args[0] != "-vf" {
		t.Fatalf("vf args bad: %v", args)
	}
	if args[1] != "hflip,eq=brightness=0.1" {
		t.Fatalf("vf join bad: %q", args[1])
	}
}

func TestOperationMarshalUnmarshal(t *testing.T) {
	orig := Pipeline{
		Inputs: []string{"/tmp/in.mp4"},
		Operations: []Operation{
			&TranscodeOp{VCodec: "libx264", CRF: "18"},
			&TrimOp{Start: "00:00:05", End: "00:00:10", Mode: "copy"},
		},
		Output: OutputSpec{Path: "/tmp/out.mp4"},
	}
	data, err := MarshalPipeline(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded, err := UnmarshalPipeline(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Operations) != 2 {
		t.Fatalf("ops count: %d", len(decoded.Operations))
	}
	if decoded.Operations[0].Type() != "transcode" || decoded.Operations[1].Type() != "trim" {
		t.Fatalf("types: %v %v", decoded.Operations[0].Type(), decoded.Operations[1].Type())
	}
}

func TestFfmpegBinFallback(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.FfmpegPath = ""
	if got := FfmpegBin(cfg); got != "ffmpeg" {
		t.Fatalf("fallback: got %q", got)
	}
	cfg.FfmpegPath = "/opt/ffmpeg/bin/ffmpeg"
	if got := FfmpegBin(cfg); got != "/opt/ffmpeg/bin/ffmpeg" {
		t.Fatalf("custom: got %q", got)
	}
}

func TestFfprobeBinFallback(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.FfmpegPath = ""
	if got := FfprobeBin(cfg); got != "ffprobe" {
		t.Fatalf("fallback: got %q", got)
	}
}
