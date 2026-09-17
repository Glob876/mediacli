package core

import "testing"

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
