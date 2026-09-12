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
	s.mux.HandleFunc("GET /api/presets", s.handlePresets)
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("PUT /api/config", s.handlePutConfig)
}

// Handler wraps the mux with CORS (loopback-only server) and token auth.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if s.token != "" {
			user, pass, _ := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
			if !strings.EqualFold(user, "Bearer") || pass != s.token {
				writeErr(w, http.StatusUnauthorized, "missing or invalid bearer token")
				return
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
