package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPipelineProtectsAuxiliaryInputs(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.mp4")
	extra := filepath.Join(dir, "extra.m4a")
	os.WriteFile(source, nil, 0600)
	os.WriteFile(extra, []byte("audio"), 0600)
	_, err := BuildPipeline(Pipeline{Inputs: []string{source}, Operations: []Operation{&MuxOp{ExtraInputs: []string{extra}}}, Output: OutputSpec{Path: extra, Overwrite: true}}, GetDefaultConfig())
	if err == nil {
		t.Fatal("auxiliary input would be overwritten")
	}
}

func TestConcatManifestHandlesQuotesAndVideoOnly(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	source := filepath.Join(dir, "a 'quoted' source.mp4")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=64x64:rate=10:duration=1", "-c:v", "libx264", source).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	for _, mode := range []string{"demuxer", "filter"} {
		commands, err := BuildPipeline(Pipeline{Inputs: []string{source, source}, Operations: []Operation{&ConcatOp{Inputs: []string{source, source}, Mode: mode}}, Output: OutputSpec{Path: filepath.Join(dir, mode+".mp4")}}, GetDefaultConfig())
		if err != nil {
			t.Fatal(err)
		}
		command := commands[0]
		if out, err := exec.Command(command[0], command[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v %s", mode, err, out)
		}
	}
}
