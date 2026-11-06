package daemon

import (
	"mediacli/pkg/core"
	"net/http"
	"strconv"
)

func (s *Server) handleRetryTask(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(r)
	if !ok {
		writeErr(w, 400, "invalid task ID")
		return
	}
	tasks, err := core.GlobalQueue.RetryTask(id)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	var ids []int
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"task_ids": ids})
}

func (s *Server) handleRetryHistory(w http.ResponseWriter, r *http.Request) {
	index, err := strconv.Atoi(r.PathValue("index"))
	entries := core.GetHistory()
	if err != nil || index < 0 || index >= len(entries) {
		writeErr(w, 400, "invalid history index")
		return
	}
	entry := entries[index]
	tasks, err := core.GlobalQueue.ReplayCommands(entry.Commands, entry.Type, entry.Source, entry.Target)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	var ids []int
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"task_ids": ids})
}

func (s *Server) handleTaskLogs(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(r)
	if !ok {
		writeErr(w, 400, "invalid task ID")
		return
	}
	if _, exists := core.GlobalQueue.Snapshot(id); !exists {
		writeErr(w, 404, "task not found")
		return
	}
	after := 0
	if raw := r.URL.Query().Get("after"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			writeErr(w, 400, "invalid log offset")
			return
		}
		after = value
	}
	lines := core.GlobalQueue.TaskLogs(id)
	if after > len(lines) {
		after = len(lines)
	}
	end := min(len(lines), after+2000)
	writeJSON(w, 200, map[string]any{"logs": lines[after:end], "next": end, "total": len(lines)})
}
