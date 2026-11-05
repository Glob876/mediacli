package core

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// 6. Менеджер фоновых задач

type TaskStatus string

const (
	StatusQueued    TaskStatus = "queued"
	StatusRunning   TaskStatus = "running"
	StatusDone      TaskStatus = "done"
	StatusFailed    TaskStatus = "failed"
	StatusCancelled TaskStatus = "cancelled"
)

type BackgroundTask struct {
	ID         int
	Cmd        []string
	Title      string
	Source     string
	Target     string
	Status     TaskStatus
	Stage      string
	Progress   float64
	LogLines   []string
	StartedAt  time.Time
	FinishedAt time.Time
	ExitCode   int
	GroupID    string `json:"group_id,omitempty"`
	ChainIndex int    `json:"chain_index,omitempty"`
	ChainTotal int    `json:"chain_total,omitempty"`

	cmdObj         *exec.Cmd
	onComplete     func(int) error
	previousTaskID int
}

type BackgroundQueueManager struct {
	tasks    []*BackgroundTask
	maxTasks int
	nextID   int
	mu       sync.Mutex
}

var GlobalQueue = NewBackgroundQueueManager(3)

func NewBackgroundQueueManager(maxTasks int) *BackgroundQueueManager {
	qm := &BackgroundQueueManager{
		tasks:    make([]*BackgroundTask, 0),
		maxTasks: maxTasks,
		nextID:   1,
	}
	go qm.workerLoop()
	return qm
}

func (qm *BackgroundQueueManager) Enqueue(cmd []string, title, source, target string) *BackgroundTask {
	task := qm.EnqueueWithHook(cmd, title, source, target, nil)
	return task
}

// EnqueueWithHook runs final file operations before publishing a terminal status.
// The hook must not call queue methods; it runs under the queue lock.
func (qm *BackgroundQueueManager) EnqueueWithHook(cmd []string, title, source, target string, onComplete func(int) error) *BackgroundTask {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	task := &BackgroundTask{
		ID:         qm.nextID,
		Cmd:        append([]string(nil), cmd...),
		onComplete: onComplete,
		Title:      title,
		Source:     source,
		Target:     target,
		Status:     StatusQueued,
		Stage:      "Queued...",
		LogLines:   make([]string, 0),
		StartedAt:  time.Now(),
	}
	qm.nextID++
	qm.tasks = append(qm.tasks, task)
	return task
}

func (qm *BackgroundQueueManager) SetMaxTasks(n int) {
	if n < 1 {
		n = 1
	}
	if n > 8 {
		n = 8
	}
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.maxTasks = n
}

// SyncMaxTasksFromConfig применяет cfg.BGQueueMax к глобальной очереди.
func (qm *BackgroundQueueManager) SyncMaxTasksFromConfig(cfg Config) {
	n := cfg.BGQueueMax
	if n < 1 {
		n = 1
	}
	if n > 8 {
		n = 8
	}
	qm.SetMaxTasks(n)
}

func (qm *BackgroundQueueManager) CancelTask(taskID int) bool {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	for _, t := range qm.tasks {
		if t.ID == taskID {
			if t.Status != StatusQueued && t.Status != StatusRunning {
				return false
			}
			wasQueued := t.Status == StatusQueued
			if t.Status == StatusRunning && t.cmdObj != nil && t.cmdObj.Process != nil {
				_ = t.cmdObj.Process.Kill()
			}
			t.Status = StatusCancelled
			t.FinishedAt = time.Now()
			if wasQueued {
				go qm.finishTask(t, -1, "")
			}
			return true
		}
	}
	return false
}

func (qm *BackgroundQueueManager) CancelGroup(groupID string) int {
	if groupID == "" {
		return 0
	}
	qm.mu.Lock()
	defer qm.mu.Unlock()
	n := 0
	for _, t := range qm.tasks {
		if t.GroupID == groupID && (t.Status == StatusQueued || t.Status == StatusRunning) {
			wasQueued := t.Status == StatusQueued
			if t.Status == StatusRunning && t.cmdObj != nil && t.cmdObj.Process != nil {
				_ = t.cmdObj.Process.Kill()
			}
			t.Status = StatusCancelled
			t.FinishedAt = time.Now()
			if wasQueued {
				go qm.finishTask(t, -1, "")
			}
			n++
		}
	}
	return n
}

func (qm *BackgroundQueueManager) EnqueueChain(cmds [][]string, title, source, target, groupID string) []*BackgroundTask {
	if len(cmds) == 0 {
		return nil
	}
	qm.mu.Lock()
	defer qm.mu.Unlock()
	var tasks []*BackgroundTask
	for i, cmd := range cmds {
		task := &BackgroundTask{
			ID:         qm.nextID,
			Cmd:        append([]string(nil), cmd...),
			Title:      fmt.Sprintf("%s [%d/%d]", title, i+1, len(cmds)),
			Source:     source,
			Target:     target,
			Status:     StatusQueued,
			Stage:      "Queued...",
			LogLines:   make([]string, 0),
			StartedAt:  time.Now(),
			GroupID:    groupID,
			ChainIndex: i + 1,
			ChainTotal: len(cmds),
		}
		if len(tasks) > 0 {
			task.previousTaskID = tasks[len(tasks)-1].ID
		}
		qm.nextID++
		qm.tasks = append(qm.tasks, task)
		tasks = append(tasks, task)
	}
	return tasks
}

// TaskSnapshot — потокобезопасная копия состояния задачи для TUI и daemon API. Поля BackgroundTask
// мутируют под мьютексом очереди, поэтому читать их напрямую нельзя.
type TaskSnapshot struct {
	ID         int        `json:"id"`
	Title      string     `json:"title"`
	Source     string     `json:"source"`
	Target     string     `json:"target"`
	Status     TaskStatus `json:"status"`
	Stage      string     `json:"stage"`
	Progress   float64    `json:"progress"`
	ExitCode   int        `json:"exit_code"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt time.Time  `json:"finished_at,omitempty"`
	LogTail    []string   `json:"log_tail,omitempty"`
	GroupID    string     `json:"group_id,omitempty"`
	ChainIndex int        `json:"chain_index,omitempty"`
	ChainTotal int        `json:"chain_total,omitempty"`
}

func snapshotTask(t *BackgroundTask) TaskSnapshot {
	snap := TaskSnapshot{
		ID:         t.ID,
		Title:      t.Title,
		Source:     t.Source,
		Target:     t.Target,
		Status:     t.Status,
		Stage:      t.Stage,
		Progress:   t.Progress,
		ExitCode:   t.ExitCode,
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
		GroupID:    t.GroupID,
		ChainIndex: t.ChainIndex,
		ChainTotal: t.ChainTotal,
	}
	if n := len(t.LogLines); n > 0 {
		from := n - 20
		if from < 0 {
			from = 0
		}
		snap.LogTail = append([]string{}, t.LogLines[from:]...)
	}
	return snap
}

// Snapshots возвращает копии всех задач. Безопасно для конкурентного чтения.
func (qm *BackgroundQueueManager) Snapshots() []TaskSnapshot {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	out := make([]TaskSnapshot, 0, len(qm.tasks))
	for _, t := range qm.tasks {
		out = append(out, snapshotTask(t))
	}
	return out
}

// Snapshot возвращает копию одной задачи по ID.
func (qm *BackgroundQueueManager) Snapshot(id int) (TaskSnapshot, bool) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	for _, t := range qm.tasks {
		if t.ID == id {
			return snapshotTask(t), true
		}
	}
	return TaskSnapshot{}, false
}

func (qm *BackgroundQueueManager) GetSummary() string {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	var running, queued []*BackgroundTask
	for _, t := range qm.tasks {
		if t.Status == StatusRunning {
			running = append(running, t)
		} else if t.Status == StatusQueued {
			queued = append(queued, t)
		}
	}

	total := len(running) + len(queued)
	if total == 0 {
		return ""
	}
	if len(running) > 0 {
		return fmt.Sprintf("[BG: %d active (%.1f%%)]", total, running[0].Progress)
	}
	return fmt.Sprintf("[BG: %d queued]", total)
}

func (qm *BackgroundQueueManager) workerLoop() {
	for {
		time.Sleep(500 * time.Millisecond)

		qm.mu.Lock()
		var runningCount int
		var nextTask *BackgroundTask

		for _, t := range qm.tasks {
			if t.Status == StatusRunning {
				runningCount++
			}
			if t.Status == StatusQueued && nextTask == nil {
				ready := true
				if t.previousTaskID != 0 {
					for _, previous := range qm.tasks {
						if previous.ID != t.previousTaskID {
							continue
						}
						ready = previous.Status == StatusDone
						if previous.Status == StatusFailed || previous.Status == StatusCancelled {
							t.Status = StatusCancelled
							t.Stage = "Previous step did not succeed"
							t.FinishedAt = time.Now()
						}
						break
					}
				}
				if ready {
					nextTask = t
				}
			}
		}

		maxSlots := qm.maxTasks
		if maxSlots < 1 {
			maxSlots = 1
		}
		if runningCount < maxSlots && nextTask != nil {
			nextTask.Status = StatusRunning
			nextTask.StartedAt = time.Now()
			if len(nextTask.Cmd) > 0 {
				nextTask.cmdObj = exec.Command(nextTask.Cmd[0], nextTask.Cmd[1:]...)
			}
			go qm.monitorTask(nextTask)
		}
		qm.mu.Unlock()
	}
}

func (qm *BackgroundQueueManager) monitorTask(task *BackgroundTask) {
	qm.mu.Lock()
	if task.Status == StatusCancelled {
		qm.mu.Unlock()
		qm.finishTask(task, -1, "")
		return
	}
	if task.cmdObj == nil {
		qm.mu.Unlock()
		qm.finishTask(task, -1, "empty command")
		return
	}
	stdout, err := task.cmdObj.StdoutPipe()
	if err == nil {
		task.cmdObj.Stderr = task.cmdObj.Stdout
		err = task.cmdObj.Start()
	}
	qm.mu.Unlock()
	if err != nil {
		qm.finishTask(task, -1, err.Error())
		return
	}

	// Robust chunk reader: yt-dlp прогресс пишет через \r без \n,
	// а bufio.Scanner падает с token too long >64KB и блокирует pipe → hang на [Merger].
	buf := make([]byte, 4096)
	leftover := ""
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			chunk := leftover + string(buf[:n])
			chunk = strings.ReplaceAll(chunk, "\r", "\n")
			parts := strings.Split(chunk, "\n")
			leftover = parts[len(parts)-1]
			for _, raw := range parts[:len(parts)-1] {
				text := strings.TrimSpace(raw)
				if text == "" {
					continue
				}
				qm.mu.Lock()
				task.LogLines = append(task.LogLines, text)
				task.Stage = DetectStage(text, task.Stage)
				if pct, _, ok := ExtractProgress(text); ok {
					task.Progress = pct
				}
				qm.mu.Unlock()
			}
			if err != nil {
				leftover = strings.TrimSpace(leftover)
				if leftover != "" {
					qm.mu.Lock()
					task.LogLines = append(task.LogLines, leftover)
					task.Stage = DetectStage(leftover, task.Stage)
					if pct, _, ok := ExtractProgress(leftover); ok {
						task.Progress = pct
					}
					qm.mu.Unlock()
				}
				break
			}
		}
		if err != nil {
			if err != io.EOF && leftover != "" {
				leftover = strings.TrimSpace(leftover)
				if leftover != "" {
					qm.mu.Lock()
					task.LogLines = append(task.LogLines, leftover)
					task.Stage = DetectStage(leftover, task.Stage)
					if pct, _, ok := ExtractProgress(leftover); ok {
						task.Progress = pct
					}
					qm.mu.Unlock()
				}
			} else if err == io.EOF && leftover != "" {
				leftover = strings.TrimSpace(leftover)
				if leftover != "" {
					qm.mu.Lock()
					task.LogLines = append(task.LogLines, leftover)
					task.Stage = DetectStage(leftover, task.Stage)
					if pct, _, ok := ExtractProgress(leftover); ok {
						task.Progress = pct
					}
					qm.mu.Unlock()
				}
			}
			break
		}
	}

	exitCode := 0
	if err := task.cmdObj.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	qm.finishTask(task, exitCode, "")
}

func (qm *BackgroundQueueManager) finishTask(task *BackgroundTask, exitCode int, errStr string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	cancelled := task.Status == StatusCancelled
	if cancelled && exitCode == 0 {
		exitCode = -1
	}
	if task.onComplete != nil {
		if err := task.onComplete(exitCode); err != nil {
			exitCode = 1
			errStr = err.Error()
		}
		task.onComplete = nil
	}
	task.ExitCode = exitCode
	task.FinishedAt = time.Now()
	historyStatus := "Success"
	if cancelled {
		task.Status = StatusCancelled
		task.Stage = "Cancelled"
		historyStatus = "Cancelled"
	} else if exitCode == 0 {
		task.Status = StatusDone
		task.Stage = "Finished"
	} else {
		task.Status = StatusFailed
		task.Stage = "Failed"
		historyStatus = fmt.Sprintf("Failed (%d)", exitCode)
	}
	if errStr != "" {
		task.LogLines = append(task.LogLines, "[Error] "+errStr)
	}
	_ = AddHistoryEntry(task.Title, task.Source, task.Target, historyStatus)
}

// TaskLogs returns an independent copy for the foreground log viewer.
func (qm *BackgroundQueueManager) TaskLogs(taskID int) []string {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	for _, task := range qm.tasks {
		if task.ID == taskID {
			logs := append([]string(nil), task.LogLines...)
			return logs
		}
	}
	return nil
}
