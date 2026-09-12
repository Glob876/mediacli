// Package daemon exposes MediaCLI core (downloads, queue, history, presets,
// config) over a loopback HTTP API with JSON payloads. It is the backend for
// the future Electron shell: the shell spawns `mediacli daemon --port 0`,
// reads MEDIACLI_DAEMON_PORT from stdout and talks to 127.0.0.1 only.
//
// Auth: if a token is configured (--token flag or MEDIACLI_TOKEN env), every
// request must carry `Authorization: Bearer <token>`.
package daemon

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"mediacli/pkg/core"
)

// Version is reported by GET /api/status.
const Version = "0.1.0-dev"

// Server is a loopback HTTP API over core.GlobalQueue.
type Server struct {
	token string
	mux   *http.ServeMux
}

// NewServer builds the API server. Empty token disables auth (dev mode).
func NewServer(token string) *Server {
	s := &Server{token: token, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	s.mux.HandleFunc("POST /api/downloads", s.handleCreateDownload)
	s.mux.HandleFunc("GET /api/tasks", s.handleListTasks)
	s.mux.HandleFunc("GET /api/tasks/{id}", s.handleGetTask)
	s.mux.HandleFunc("POST /api/tasks/{id}/cancel", s.handleCancelTask)
	s.mux.HandleFunc("GET /api/tasks/{id}/events", s.handleTaskEvents)
	s.mux.HandleFunc("GET /api/history", s.handleHistory)
	s.mux.HandleFunc("DELETE /api/history", s.handleClearHistory)
	s.mux.HandleFunc("POST /api/history/delete", s.handleDeleteHistoryItem)
	s.mux.HandleFunc("GET /api/presets", s.handlePresets)
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("PUT /api/config", s.handlePutConfig)
	s.mux.HandleFunc("GET /api/convert/presets", s.handleConvertPresets)
	s.mux.HandleFunc("POST /api/convert", s.handleCreateConvert)
	s.mux.HandleFunc("GET /api/library", s.handleLibrary)
	s.mux.HandleFunc("GET /api/library/file", s.handleLibraryFile)
	s.mux.HandleFunc("GET /api/browse", s.handleBrowse)
}

// Handler wraps the mux with CORS (loopback-only server) and token auth.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if s.token != "" {
			user, pass, _ := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
			// EventSource не умеет ставить заголовки, поэтому для SSE
			// разрешён ?token= в query (только loopback, см. Run).
			if !strings.EqualFold(user, "Bearer") || pass != s.token {
				if r.URL.Query().Get("token") != s.token {
					writeErr(w, http.StatusUnauthorized, "missing or invalid bearer token")
					return
				}
			}
		}
		s.mux.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func taskID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		return 0, false
	}
	return id, true
}

// GET /api/status — versions and tool availability (see doctor).
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	type dep struct {
		Name      string `json:"name"`
		Available bool   `json:"available"`
		Path      string `json:"path,omitempty"`
		Required  bool   `json:"required"`
	}
	deps := core.CheckDependencies()
	out := make([]dep, 0, len(deps))
	for _, d := range deps {
		out = append(out, dep{Name: d.Name, Available: d.Available, Path: d.Path, Required: d.Required})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":           true,
		"version":      Version,
		"dependencies": out,
	})
}

type createDownloadReq struct {
	URL        string                 `json:"url"`
	OutDir     string                 `json:"out_dir"`
	IsPlaylist bool                   `json:"is_playlist"`
	Fields     map[string]interface{} `json:"fields"`
}

// POST /api/downloads — validate, build yt-dlp args via core, enqueue.
// NOTE: the queue worker runs yt-dlp only; the "external" FFmpeg second
// phase currently lives in the TUI/GUI runners, so daemon downloads in
// external-transcode mode stop after the merge step (parity gap, TODO).
func (s *Server) handleCreateDownload(w http.ResponseWriter, r *http.Request) {
	var req createDownloadReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	url := strings.TrimSpace(req.URL)
	if url == "" {
		writeErr(w, http.StatusBadRequest, "url is required")
		return
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.GetDefaultConfig()
	}
	outDir := core.ParseUserPath(cfg.DownloadDir)
	if strings.TrimSpace(req.OutDir) != "" {
		outDir = core.ParseUserPath(req.OutDir)
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		writeErr(w, http.StatusBadRequest, "cannot create output dir: "+err.Error())
		return
	}

	fields := core.GetInitialPresetFields()
	for k, v := range req.Fields {
		fields[k] = v
	}
	preset := core.DownloadPreset{
		ID:     fmt.Sprintf("daemon_%d", time.Now().UnixNano()),
		Name:   "Daemon Download",
		Fields: fields,
	}
	cmdList := append([]string{"yt-dlp"}, core.BuildYtDlpArgs(preset, cfg, outDir, req.IsPlaylist)...)
	cmdList = append(cmdList, url)

	task := core.GlobalQueue.Enqueue(cmdList, "Download", url, outDir)
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"task_id": task.ID})
}

// GET /api/tasks — snapshots of all tasks.
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	snaps := core.GlobalQueue.Snapshots()
	if snaps == nil {
		snaps = []core.TaskSnapshot{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"tasks": snaps})
}

// GET /api/tasks/{id} — one snapshot incl. log tail.
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid task id")
		return
	}
	snap, found := core.GlobalQueue.Snapshot(id)
	if !found {
		writeErr(w, http.StatusNotFound, "task not found")
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// POST /api/tasks/{id}/cancel — kill running / drop queued task.
func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid task id")
		return
	}
	if !core.GlobalQueue.CancelTask(id) {
		writeErr(w, http.StatusNotFound, "task not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "id": id})
}

func isTerminal(st core.TaskStatus) bool {
	return st == core.StatusDone || st == core.StatusFailed || st == core.StatusCancelled
}

// GET /api/tasks/{id}/events — SSE stream of stage/progress, closes after
// the task reaches a terminal state.
func (s *Server) handleTaskEvents(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid task id")
		return
	}
	if _, found := core.GlobalQueue.Snapshot(id); !found {
		writeErr(w, http.StatusNotFound, "task not found")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	ctx := r.Context()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap, found := core.GlobalQueue.Snapshot(id)
			if !found {
				return
			}
			data, _ := json.Marshal(snap)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			if isTerminal(snap.Status) {
				return
			}
		}
	}
}

// GET /api/history — operation log (newest first, max 50, see core).
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	entries := core.GetHistory()
	if entries == nil {
		entries = []core.HistoryEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"history": entries})
}

// DELETE /api/history — wipe the operation log.
func (s *Server) handleClearHistory(w http.ResponseWriter, r *http.Request) {
	if err := core.ClearHistory(); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot clear history: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

type deleteHistoryReq struct {
	Time       string `json:"time"`
	Source     string `json:"source"`
	Target     string `json:"target"`
	DeleteFile bool   `json:"delete_file"`
}

// POST /api/history/delete — remove one entry, optionally with the file.
// File deletion is guarded to the download dir (see core.DeleteHistoryItem).
func (s *Server) handleDeleteHistoryItem(w http.ResponseWriter, r *http.Request) {
	var req deleteHistoryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.GetDefaultConfig()
	}
	entry := core.HistoryEntry{Time: req.Time, Source: req.Source, Target: req.Target}
	if err := core.DeleteHistoryItem(entry, req.DeleteFile, cfg.DownloadDir); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot delete: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// GET /api/presets — codec presets + saved download presets (single source
// of truth lives in Go; the TS frontend must not hardcode these).
func (s *Server) handlePresets(w http.ResponseWriter, r *http.Request) {
	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.GetDefaultConfig()
	}
	type vp struct {
		ID     string   `json:"id"`
		NameEN string   `json:"name_en"`
		NameRU string   `json:"name_ru"`
		DescEN string   `json:"desc_en"`
		DescRU string   `json:"desc_ru"`
		Args   []string `json:"args"`
	}
	videos := make([]vp, 0, len(core.OrderedVideoPresetKeys))
	for _, k := range core.OrderedVideoPresetKeys {
		p := core.VideoPresets[k]
		videos = append(videos, vp{ID: p.ID, NameEN: p.NameEN, NameRU: p.NameRU, DescEN: p.DescEN, DescRU: p.DescRU, Args: p.Args})
	}
	saved := cfg.DownloadPresets
	if saved == nil {
		saved = []core.DownloadPreset{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"video_presets":    videos,
		"download_presets": saved,
	})
}

// GET /api/config — full runtime config.
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.GetDefaultConfig()
	}
	writeJSON(w, http.StatusOK, cfg)
}

// PUT /api/config — replace full config, persist, resync queue limit.
func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var cfg core.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if cfg.BGQueueMax < 1 {
		cfg.BGQueueMax = 1
	}
	if cfg.BGQueueMax > 8 {
		cfg.BGQueueMax = 8
	}
	if err := core.SaveConfig(cfg); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot save config: "+err.Error())
		return
	}
	core.GlobalQueue.SyncMaxTasksFromConfig(cfg)
	writeJSON(w, http.StatusOK, cfg)
}

// GET /api/convert/presets — локальные пресеты конвертации (ffmpeg).
// Источник правды — core.ConvertPresets, фронт их не хардкодит.
func (s *Server) handleConvertPresets(w http.ResponseWriter, r *http.Request) {
	type cp struct {
		ID     string   `json:"id"`
		NameEN string   `json:"name_en"`
		NameRU string   `json:"name_ru"`
		DescEN string   `json:"desc_en"`
		DescRU string   `json:"desc_ru"`
		Ext    string   `json:"ext"`
		Suffix string   `json:"suffix"`
		Flags  []string `json:"flags"`
	}
	out := make([]cp, 0, len(core.ConvertPresets))
	for _, p := range core.ConvertPresets {
		out = append(out, cp{
			ID: p.ID, NameEN: p.NameEN, NameRU: p.NameRU,
			DescEN: p.DescEN, DescRU: p.DescRU,
			Ext: p.Ext, Suffix: p.Suffix, Flags: p.FFmpegFlags,
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"convert_presets": out})
}

type createConvertReq struct {
	Input    string `json:"input"`
	PresetID string `json:"preset_id"`
	Output   string `json:"output"`
}

// resolveConvertPath: абсолютный путь (если существует) либо имя файла
// внутри downloadDir. Возвращает абсолютный путь.
func resolveConvertPath(raw, downloadDir string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	cleaned := core.ParseUserPath(raw)
	if filepath.IsAbs(cleaned) {
		return filepath.Clean(cleaned)
	}
	return filepath.Clean(filepath.Join(core.ParseUserPath(downloadDir), cleaned))
}

// POST /api/convert — поставить ffmpeg-конвертацию в общую очередь.
// Тело: {"input": "<файл>", "preset_id": "standard_mp4", "output": "<опц.>"}.
func (s *Server) handleCreateConvert(w http.ResponseWriter, r *http.Request) {
	var req createConvertReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.GetDefaultConfig()
	}
	inputAbs := resolveConvertPath(req.Input, cfg.DownloadDir)
	if inputAbs == "" {
		writeErr(w, http.StatusBadRequest, "input is required")
		return
	}
	if st, err := os.Stat(inputAbs); err != nil || st.IsDir() {
		writeErr(w, http.StatusBadRequest, "input file not found: "+req.Input)
		return
	}
	var preset *core.ConvertPreset
	for i := range core.ConvertPresets {
		if core.ConvertPresets[i].ID == strings.TrimSpace(req.PresetID) {
			preset = &core.ConvertPresets[i]
			break
		}
	}
	if preset == nil {
		writeErr(w, http.StatusBadRequest, "unknown preset_id: "+req.PresetID)
		return
	}
	finalPath := ""
	if strings.TrimSpace(req.Output) != "" {
		finalPath = resolveConvertPath(req.Output, cfg.DownloadDir)
		if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
			writeErr(w, http.StatusBadRequest, "cannot create output dir: "+err.Error())
			return
		}
	} else {
		plan := core.PrepareFFmpegOutput(inputAbs, preset.Ext, preset.Suffix, cfg)
		finalPath = plan.FinalPath
		if plan.OnComplete != nil {
			// PrepareFFmpegOutput с OverwriteOriginal возвращает колбэк
			// для файловой ротации — в daemon-очереди его выполнить негде,
			// поэтому запрещаем overwrite-режим для API-конвертаций.
			writeErr(w, http.StatusBadRequest, "disable OverwriteOriginal for API convert or pass explicit output")
			return
		}
	}
	if strings.EqualFold(filepath.Clean(inputAbs), filepath.Clean(finalPath)) {
		writeErr(w, http.StatusBadRequest, "input and output are the same file")
		return
	}
	cmdList := []string{"ffmpeg", "-y", "-i", inputAbs}
	cmdList = append(cmdList, preset.FFmpegFlags...)
	cmdList = append(cmdList, finalPath)

	task := core.GlobalQueue.Enqueue(cmdList, "Convert", inputAbs, finalPath)
	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"task_id": task.ID,
		"output":  finalPath,
	})
}

var mediaExts = map[string]bool{
	".mp4": true, ".mkv": true, ".mov": true, ".webm": true,
	".avi": true, ".m4v": true, ".mp3": true, ".flac": true,
	".wav": true, ".m4a": true, ".opus": true, ".ogg": true,
}

type libraryItem struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"mtime"`
	IsMedia bool   `json:"is_media"`
}

// GET /api/library — файлы папки загрузок (для вкладки предпросмотра).
func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.GetDefaultConfig()
	}
	outDir := core.ParseUserPath(cfg.DownloadDir)
	entries, err := os.ReadDir(outDir)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"dir": outDir, "files": []libraryItem{},
		})
		return
	}
	files := make([]libraryItem, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ln := strings.ToLower(name)
		// Технический мусор yt-dlp/ffmpeg не показываем в предпросмотре.
		if strings.HasSuffix(ln, ".part") || strings.HasSuffix(ln, ".ytdl") ||
			strings.HasSuffix(ln, ".tmp") || strings.Contains(ln, ".temp.") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		files = append(files, libraryItem{
			Name:    e.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
			IsMedia: mediaExts[ext],
		})
	}
	// Сначала новые.
	sort.Slice(files, func(i, j int) bool { return files[i].ModTime > files[j].ModTime })
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"dir": outDir, "files": files,
	})
}

// GET /api/library/file?name=<file> — отдать файл для <video>/<audio>
// предпросмотра. Токен можно передать ?token= (тег не умеет в заголовки).
// Guard от traversal: только basename внутри downloadDir.
func (s *Server) handleLibraryFile(w http.ResponseWriter, r *http.Request) {
	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.GetDefaultConfig()
	}
	outDir := core.ParseUserPath(cfg.DownloadDir)
	name := r.URL.Query().Get("name")
	if name == "" {
		name = r.URL.Query().Get("path")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	// Разрешаем либо basename, либо абсолютный путь строго внутри outDir.
	candidate := name
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(outDir, filepath.Base(candidate))
	}
	candidate = filepath.Clean(candidate)
	base := filepath.Clean(outDir)
	rel, err := filepath.Rel(base, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		writeErr(w, http.StatusForbidden, "outside download dir")
		return
	}
	if st, err := os.Stat(candidate); err != nil || st.IsDir() {
		writeErr(w, http.StatusNotFound, "file not found")
		return
	}
	http.ServeFile(w, r, candidate)
}

type browseFile struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"mtime"`
	IsMedia bool   `json:"is_media"`
}

// GET /api/browse?path=<dir> — обзор файловой системы для вкладки
// конвертации. Loopback + токен, листинг без чтения содержимого файлов.
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.GetDefaultConfig()
	}
	raw := strings.TrimSpace(r.URL.Query().Get("path"))
	dir := core.ParseUserPath(cfg.DownloadDir)
	if raw != "" {
		dir = core.ParseUserPath(raw)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		// Откатываемся к папке загрузок, чтобы UI не упирался в 400.
		dir = core.ParseUserPath(cfg.DownloadDir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "cannot read dir: "+err.Error())
		return
	}
	dirs := make([]string, 0)
	files := make([]browseFile, 0)
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
			continue
		}
		ln := strings.ToLower(e.Name())
		if strings.HasSuffix(ln, ".part") || strings.HasSuffix(ln, ".ytdl") ||
			strings.HasSuffix(ln, ".tmp") || strings.Contains(ln, ".temp.") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		files = append(files, browseFile{
			Name:    e.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
			IsMedia: mediaExts[ext],
		})
	}
	sort.Strings(dirs)
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	parent := filepath.Dir(dir)
	if parent == dir {
		parent = ""
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path": dir, "parent": parent, "dirs": dirs, "files": files,
	})
}

// Run starts the loopback daemon. args are the CLI args after `daemon`.
// Prints MEDIACLI_DAEMON_PORT=<port> to stdout for the Electron launcher.
func Run(args []string) int {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	port := fs.Int("port", 0, "TCP port on 127.0.0.1 (0 = pick free)")
	token := fs.String("token", "", "Bearer token (fallback: MEDIACLI_TOKEN env, empty = no auth)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	tok := *token
	if tok == "" {
		tok = os.Getenv("MEDIACLI_TOKEN")
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		cfg = core.GetDefaultConfig()
	}
	core.GlobalQueue.SyncMaxTasksFromConfig(cfg)

	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(*port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "daemon listen error:", err)
		return 1
	}
	actual := ln.Addr().(*net.TCPAddr).Port
	fmt.Printf("MEDIACLI_DAEMON_PORT=%d\n", actual)

	srv := &http.Server{Handler: NewServer(tok).Handler()}
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, "daemon serve error:", err)
		return 1
	}
	return 0
}
