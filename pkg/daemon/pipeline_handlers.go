package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"mediacli/pkg/core"
)

// GET /api/probe?path=<file> — расширенный ffprobe Inspector
func (s *Server) handleProbe(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("path"))
	if raw == "" {
		raw = strings.TrimSpace(r.URL.Query().Get("name"))
	}
	if raw == "" {
		writeErr(w, http.StatusBadRequest, "path is required")
		return
	}
	cfg, _ := core.LoadConfig()
	abs := core.ParseUserPath(raw)
	// Guard: путь должен существовать
	if st, err := os.Stat(abs); err != nil || st.IsDir() {
		// пробуем резолв через downloadDir (как library)
		outDir := core.ParseUserPath(cfg.DownloadDir)
		if cand, ok := resolveLibraryPath(raw, outDir); ok {
			abs = cand
		} else {
			writeErr(w, http.StatusNotFound, "file not found: "+raw)
			return
		}
		if st2, err2 := os.Stat(abs); err2 != nil || st2.IsDir() {
			writeErr(w, http.StatusNotFound, "file not found: "+raw)
			return
		}
	}
	info, err := core.ProbeMediaInfoWithConfig(abs, cfg)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "probe failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// GET /api/tools/ffprobe — диагностика ffprobe
func (s *Server) handleToolsFfprobe(w http.ResponseWriter, r *http.Request) {
	cfg, _ := core.LoadConfig()
	candidate := strings.TrimSpace(r.URL.Query().Get("path"))
	bin, version, resErr := core.ResolveFfprobe(cfg, candidate)
	out := map[string]interface{}{
		"configured": strings.TrimSpace(cfg.FfprobePath),
		"resolved":   bin,
		"found":      resErr == nil,
	}
	if resErr == nil {
		out["version"] = version
	} else {
		out["error"] = resErr.Error()
	}
	writeJSON(w, http.StatusOK, out)
}

// Pipeline request payload
type pipelineReq struct {
	Inputs     []string                `json:"inputs"`
	Operations []core.OperationEnvelope `json:"operations"`
	Output     core.OutputSpec         `json:"output"`
	Name       string                  `json:"name"`
}

func (s *Server) parsePipeline(r *http.Request) (core.Pipeline, error) {
	var req pipelineReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return core.Pipeline{}, err
	}
	cfg, _ := core.LoadConfig()
	// Resolve inputs via ParseUserPath + guard
	var absInputs []string
	for _, inp := range req.Inputs {
		abs := core.ParseUserPath(strings.TrimSpace(inp))
		if abs == "" {
			continue
		}
		// allow absolute paths outside downloadDir for pipeline (but warn)
		absInputs = append(absInputs, abs)
	}
	if len(absInputs) == 0 {
		return core.Pipeline{}, fmt.Errorf("no valid inputs")
	}
	// Resolve output
	outPath := strings.TrimSpace(req.Output.Path)
	if outPath != "" {
		outPath = core.ParseUserPath(outPath)
		_ = os.MkdirAll(filepath.Dir(outPath), 0755)
	}
	p := core.Pipeline{
		ID:     fmt.Sprintf("pipe_%d", time.Now().UnixNano()),
		Name:   req.Name,
		Inputs: absInputs,
		Output: core.OutputSpec{Path: outPath, Overwrite: req.Output.Overwrite, Dir: req.Output.Dir},
	}
	for _, env := range req.Operations {
		op, err := core.UnmarshalOperation(env)
		if err != nil {
			return p, fmt.Errorf("operation %q: %w", env.Type, err)
		}
		p.Operations = append(p.Operations, op)
	}
	_ = cfg
	return p, nil
}

// POST /api/pipeline/dry-run — показать команды без запуска
func (s *Server) handlePipelineDryRun(w http.ResponseWriter, r *http.Request) {
	p, err := s.parsePipeline(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid pipeline: "+err.Error())
		return
	}
	cfg, _ := core.LoadConfig()
	cmds, err := core.BuildPipeline(p, cfg)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "build failed: "+err.Error())
		return
	}
	var strCmds []string
	for _, c := range cmds {
		strCmds = append(strCmds, strings.Join(core.MaskCmdForLog(c), " "))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"commands": strCmds, "count": len(strCmds)})
}

// POST /api/pipeline/run — поставить пайплайн в очередь
func (s *Server) handlePipelineRun(w http.ResponseWriter, r *http.Request) {
	p, err := s.parsePipeline(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid pipeline: "+err.Error())
		return
	}
	cfg, _ := core.LoadConfig()
	cmds, err := core.BuildPipeline(p, cfg)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "build failed: "+err.Error())
		return
	}
	groupID := fmt.Sprintf("pipeline_%d", time.Now().UnixNano())
	tasks := core.GlobalQueue.EnqueueChain(cmds, "Pipeline: "+p.Name, strings.Join(p.Inputs, ","), p.Output.Path, groupID)
	var ids []int
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"task_ids": ids, "group_id": groupID, "commands": cmds})
}

// POST /api/pipeline/preview — сгенерить 5с превью (возвращает временный файл URL)
func (s *Server) handlePipelinePreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		pipelineReq
		Start    string `json:"start"`    // e.g. "5"
		Duration string `json:"duration"` // e.g. "5"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	cfg, _ := core.LoadConfig()
	// Build pipeline from req
	var absInputs []string
	for _, inp := range req.Inputs {
		abs := core.ParseUserPath(strings.TrimSpace(inp))
		if abs != "" {
			absInputs = append(absInputs, abs)
		}
	}
	if len(absInputs) == 0 {
		writeErr(w, http.StatusBadRequest, "no inputs")
		return
	}
	p := core.Pipeline{
		ID:     "preview",
		Name:   "preview",
		Inputs: absInputs,
		Output: req.Output,
	}
	for _, env := range req.Operations {
		op, err := core.UnmarshalOperation(env)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "operation: "+err.Error())
			return
		}
		p.Operations = append(p.Operations, op)
	}
	cmds, err := core.BuildPipeline(p, cfg)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "build failed: "+err.Error())
		return
	}
	cmd := cmds[0]
	// Convert last arg (output) to temp preview file
	tmpDir := os.TempDir()
	previewPath := filepath.Join(tmpDir, fmt.Sprintf("mediacli_preview_%d.mp4", time.Now().UnixNano()))
	// Replace output with previewPath, add -t and -ss
	// cmd = [ffmpeg -y -i in ... out] -> insert -ss/duration before -i or after
	start := strings.TrimSpace(req.Start)
	if start == "" {
		start = "1"
	}
	dur := strings.TrimSpace(req.Duration)
	if dur == "" {
		dur = "5"
	}
	// Rebuild with preview tweaks: limit to 5s
	// For simplicity, add -ss start -t dur after inputs but before filters is not trivial.
	// Instead do: ffmpeg -y -ss start -t dur -i in ... preview
	// Reconstruct: find first -i and prepend
	newCmd := []string{cmd[0], "-y", "-ss", start, "-t", dur}
	// copy rest from original cmd starting at first -i
	found := false
	for i, a := range cmd {
		if a == "-i" && !found {
			found = true
			// include -i and following inputs, but we already have -y -ss -t, so append from -i
			newCmd = append(newCmd, cmd[i:]...)
			break
		}
	}
	if !found {
		writeErr(w, http.StatusInternalServerError, "cannot build preview command")
		return
	}
	// replace last arg with previewPath
	newCmd[len(newCmd)-1] = previewPath
	// add faststart flags for preview playback
	newCmd = append(newCmd[:len(newCmd)-1], "-movflags", "frag_keyframe+empty_moov", previewPath)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, newCmd[0], newCmd[1:]...)
	out, err := c.CombinedOutput()
	if err != nil {
		_ = os.Remove(previewPath)
		writeErr(w, http.StatusInternalServerError, "preview failed: "+strings.TrimSpace(string(out)))
		return
	}
	// Serve temp file via /api/library/file style? For now return path
	writeJSON(w, http.StatusOK, map[string]interface{}{"preview_path": previewPath, "command": strings.Join(newCmd, " ")})
}

// Media presets CRUD
func (s *Server) handleListMediaPresets(w http.ResponseWriter, r *http.Request) {
	presets := core.GetMediaPresets(core.GetDefaultConfig())
	if presets == nil {
		presets = []core.MediaPreset{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"presets": presets})
}

func (s *Server) handleCreateMediaPreset(w http.ResponseWriter, r *http.Request) {
	var p core.MediaPreset
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if strings.TrimSpace(p.Name) == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := core.SaveMediaPreset(p); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot save: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "preset": p})
}

func (s *Server) handleDeleteMediaPreset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id required")
		return
	}
	if err := core.DeleteMediaPreset(id); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot delete: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// Generic operation helpers — each builds a one-op pipeline and enqueues

func (s *Server) enqueueOps(ops []core.Operation, inputs []string, output core.OutputSpec, title string) (int, string, error) {
	cfg, _ := core.LoadConfig()
	p := core.Pipeline{
		ID:         fmt.Sprintf("op_%d", time.Now().UnixNano()),
		Name:       title,
		Inputs:     inputs,
		Operations: ops,
		Output:     output,
	}
	cmds, err := core.BuildPipeline(p, cfg)
	if err != nil {
		return 0, "", err
	}
	groupID := fmt.Sprintf("op_%d", time.Now().UnixNano())
	tasks := core.GlobalQueue.EnqueueChain(cmds, title, strings.Join(inputs, ","), output.Path, groupID)
	if len(tasks) == 0 {
		return 0, "", fmt.Errorf("no tasks enqueued")
	}
	return tasks[0].ID, groupID, nil
}

func (s *Server) handleOpAspect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input  string        `json:"input"`
		Output string        `json:"output"`
		Aspect core.AspectOp `json:"aspect"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if strings.TrimSpace(body.Input) == "" {
		writeErr(w, http.StatusBadRequest, "input required")
		return
	}
	inp := core.ParseUserPath(body.Input)
	cfg, _ := core.LoadConfig()
	out := strings.TrimSpace(body.Output)
	if out == "" {
		plan := core.PrepareFFmpegOutput(inp, ".mp4", fmt.Sprintf("_%dx%d_%s", body.Aspect.RatioW, body.Aspect.RatioH, body.Aspect.Mode), cfg)
		out = plan.FinalPath
	} else {
		out = core.ParseUserPath(out)
	}
	id, group, err := s.enqueueOps([]core.Operation{&body.Aspect}, []string{inp}, core.OutputSpec{Path: out}, "Aspect")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "build failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"task_id": id, "group_id": group, "output": out})
}

func (s *Server) handleOpMux(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Inputs []string  `json:"inputs"`
		Output string    `json:"output"`
		Mux    core.MuxOp `json:"mux"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(body.Inputs) < 2 {
		writeErr(w, http.StatusBadRequest, "at least 2 inputs required for mux")
		return
	}
	var abs []string
	for _, p := range body.Inputs {
		abs = append(abs, core.ParseUserPath(p))
	}
	out := core.ParseUserPath(body.Output)
	if out == "" {
		cfg, _ := core.LoadConfig()
		plan := core.PrepareFFmpegOutput(abs[0], ".mp4", "_muxed", cfg)
		out = plan.FinalPath
	}
	// MuxOp extra_inputs are inputs[1:]
	mux := body.Mux
	if len(mux.ExtraInputs) == 0 {
		mux.ExtraInputs = abs[1:]
		abs = abs[:1]
	}
	id, group, err := s.enqueueOps([]core.Operation{&mux}, abs, core.OutputSpec{Path: out}, "Mux")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "build failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"task_id": id, "group_id": group, "output": out})
}

func (s *Server) handleOpDemux(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input  string       `json:"input"`
		Output string       `json:"output"`
		Demux  core.DemuxOp `json:"demux"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	inp := core.ParseUserPath(body.Input)
	out := core.ParseUserPath(body.Output)
	if out == "" {
		writeErr(w, http.StatusBadRequest, "output required for demux")
		return
	}
	id, group, err := s.enqueueOps([]core.Operation{&body.Demux}, []string{inp}, core.OutputSpec{Path: out}, "Demux")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "build failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"task_id": id, "group_id": group, "output": out})
}

func (s *Server) handleOpTrim(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input  string      `json:"input"`
		Output string      `json:"output"`
		Trim   core.TrimOp `json:"trim"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	inp := core.ParseUserPath(body.Input)
	cfg, _ := core.LoadConfig()
	out := strings.TrimSpace(body.Output)
	if out == "" {
		plan := core.PrepareFFmpegOutput(inp, filepath.Ext(inp), "_trimmed", cfg)
		out = plan.FinalPath
	} else {
		out = core.ParseUserPath(out)
	}
	id, group, err := s.enqueueOps([]core.Operation{&body.Trim}, []string{inp}, core.OutputSpec{Path: out}, "Trim")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "build failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"task_id": id, "group_id": group, "output": out})
}

func (s *Server) handleOpConcat(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Inputs []string     `json:"inputs"`
		Output string       `json:"output"`
		Concat core.ConcatOp `json:"concat"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(body.Inputs) < 2 {
		writeErr(w, http.StatusBadRequest, "at least 2 inputs required")
		return
	}
	var abs []string
	for _, p := range body.Inputs {
		abs = append(abs, core.ParseUserPath(p))
	}
	out := core.ParseUserPath(body.Output)
	if out == "" {
		cfg, _ := core.LoadConfig()
		plan := core.PrepareFFmpegOutput(abs[0], ".mp4", "_concat", cfg)
		out = plan.FinalPath
	}
	concat := body.Concat
	if len(concat.Inputs) == 0 {
		concat.Inputs = abs
		abs = abs[:1]
	}
	id, group, err := s.enqueueOps([]core.Operation{&concat}, abs, core.OutputSpec{Path: out}, "Concat")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "build failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"task_id": id, "group_id": group, "output": out})
}

func (s *Server) handleOpTranscode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input     string          `json:"input"`
		Output    string          `json:"output"`
		Transcode core.TranscodeOp `json:"transcode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	inp := core.ParseUserPath(body.Input)
	cfg, _ := core.LoadConfig()
	out := strings.TrimSpace(body.Output)
	if out == "" {
		ext := body.Transcode.Container
		if ext == "" {
			ext = "mp4"
		}
		plan := core.PrepareFFmpegOutput(inp, ext, "_transcoded", cfg)
		out = plan.FinalPath
	} else {
		out = core.ParseUserPath(out)
	}
	id, group, err := s.enqueueOps([]core.Operation{&body.Transcode}, []string{inp}, core.OutputSpec{Path: out}, "Transcode")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "build failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"task_id": id, "group_id": group, "output": out})
}

func (s *Server) handleOpFilter(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input  string        `json:"input"`
		Output string        `json:"output"`
		Filter core.FilterOp `json:"filter"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	inp := core.ParseUserPath(body.Input)
	cfg, _ := core.LoadConfig()
	out := strings.TrimSpace(body.Output)
	if out == "" {
		plan := core.PrepareFFmpegOutput(inp, ".mp4", "_filtered", cfg)
		out = plan.FinalPath
	} else {
		out = core.ParseUserPath(out)
	}
	id, group, err := s.enqueueOps([]core.Operation{&body.Filter}, []string{inp}, core.OutputSpec{Path: out}, "Filter")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "build failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"task_id": id, "group_id": group, "output": out})
}

func (s *Server) handleOpAudio(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input  string       `json:"input"`
		Output string       `json:"output"`
		Audio  core.AudioOp `json:"audio"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	inp := core.ParseUserPath(body.Input)
	cfg, _ := core.LoadConfig()
	out := strings.TrimSpace(body.Output)
	if out == "" {
		plan := core.PrepareFFmpegOutput(inp, ".mp3", "_audio", cfg)
		out = plan.FinalPath
	} else {
		out = core.ParseUserPath(out)
	}
	id, group, err := s.enqueueOps([]core.Operation{&body.Audio}, []string{inp}, core.OutputSpec{Path: out}, "Audio")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "build failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"task_id": id, "group_id": group, "output": out})
}

func (s *Server) handleOpSubtitle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input    string          `json:"input"`
		Output   string          `json:"output"`
		Subtitle core.SubtitleOp `json:"subtitle"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	inp := core.ParseUserPath(body.Input)
	out := core.ParseUserPath(body.Output)
	if out == "" {
		writeErr(w, http.StatusBadRequest, "output required")
		return
	}
	id, group, err := s.enqueueOps([]core.Operation{&body.Subtitle}, []string{inp}, core.OutputSpec{Path: out}, "Subtitle")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "build failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"task_id": id, "group_id": group, "output": out})
}

func (s *Server) handleOpMetadata(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input    string          `json:"input"`
		Output   string          `json:"output"`
		Metadata core.MetadataOp `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	inp := core.ParseUserPath(body.Input)
	cfg, _ := core.LoadConfig()
	out := strings.TrimSpace(body.Output)
	if out == "" {
		plan := core.PrepareFFmpegOutput(inp, filepath.Ext(inp), "_meta", cfg)
		out = plan.FinalPath
	} else {
		out = core.ParseUserPath(out)
	}
	id, group, err := s.enqueueOps([]core.Operation{&body.Metadata}, []string{inp}, core.OutputSpec{Path: out}, "Metadata")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "build failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"task_id": id, "group_id": group, "output": out})
}

// POST /api/batch — применить пайплайн к N файлам
func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	var job core.BatchJob
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	cfg, _ := core.LoadConfig()
	ids, err := core.EnqueueBatch(job, cfg)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "batch failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"task_ids": ids, "count": len(ids)})
}

func (s *Server) handleCancelGroup(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("groupId")
	if groupID == "" {
		groupID = r.PathValue("groupID")
	}
	if groupID == "" {
		writeErr(w, http.StatusBadRequest, "group id required")
		return
	}
	n := core.GlobalQueue.CancelGroup(groupID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"cancelled": n, "group_id": groupID})
}
