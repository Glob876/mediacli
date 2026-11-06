package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type savedTask struct {
	Task     BackgroundTask
	Previous int
	HasHook  bool
}

// EnablePersistence is opt-in for the terminal app; the daemon keeps its existing lifecycle.
// Interrupted work is never executed automatically.
func (qm *BackgroundQueueManager) EnablePersistence(path string) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var records []savedTask
	if len(data) > 0 {
		if err := json.Unmarshal(data, &records); err != nil {
			return err
		}
	}
	ids := map[int]bool{}
	for _, record := range records {
		if record.Task.ID < 1 || ids[record.Task.ID] || len(record.Task.Cmd) == 0 {
			return fmt.Errorf("invalid queue record")
		}
		ids[record.Task.ID] = true
	}
	for _, record := range records {
		if record.Previous != 0 && (!ids[record.Previous] || record.Previous >= record.Task.ID) {
			return fmt.Errorf("invalid queue dependency")
		}
	}
	for _, record := range records {
		task := record.Task
		task.finalized = true
		task.previousTaskID = record.Previous
		task.unrecoverableHook = record.HasHook && task.Replacement == nil
		if task.Replacement != nil && (task.Status == StatusRunning || task.Status == StatusQueued || task.Status == StatusInterrupted) {
			task.onComplete = task.Replacement.Complete
		}
		if task.Status == StatusRunning || task.Status == StatusQueued {
			task.Status = StatusInterrupted
			task.Stage = "Interrupted; retry from the queue"
		}
		qm.tasks = append(qm.tasks, &task)
		if task.ID >= qm.nextID {
			qm.nextID = task.ID + 1
		}
	}
	qm.storagePath = path
	return qm.persistLocked()
}

func (qm *BackgroundQueueManager) persistLocked() error {
	if qm.storagePath == "" {
		return nil
	}
	records := make([]savedTask, 0, len(qm.tasks))
	for _, task := range qm.tasks {
		saved := *task
		saved.cmdObj = nil
		saved.onComplete = nil
		saved.LogLines = nil
		records = append(records, savedTask{Task: saved, Previous: task.previousTaskID, HasHook: task.unrecoverableHook || task.onComplete != nil})
	}
	data, err := json.Marshal(records)
	if err == nil {
		err = WritePrivateFile(qm.storagePath, data)
	}
	qm.persistenceError = err
	return err
}

func (qm *BackgroundQueueManager) PersistenceError() error {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	return qm.persistenceError
}

// RetryTask repeats the whole dependency chain. Existing media are preserved.
func (qm *BackgroundQueueManager) RetryTask(id int) ([]*BackgroundTask, error) {
	qm.mu.Lock()
	var selected *BackgroundTask
	for _, task := range qm.tasks {
		if task.ID == id {
			selected = task
			break
		}
	}
	if selected == nil {
		qm.mu.Unlock()
		return nil, fmt.Errorf("task not found")
	}
	if selected.Status == StatusRunning || selected.Status == StatusQueued {
		qm.mu.Unlock()
		return nil, fmt.Errorf("task is active")
	}
	if selected.Replacement != nil {
		command, replacement := selected.Replacement.RetryCommand(selected.Cmd)
		title, source, target := selected.Title, selected.Source, selected.Target
		qm.mu.Unlock()
		task := qm.EnqueueWithHook(command, title, source, target, replacement.Complete)
		return []*BackgroundTask{task}, nil
	}
	var chain []*BackgroundTask
	root := selected
	for root.previousTaskID != 0 {
		found := false
		for _, task := range qm.tasks {
			if task.ID == root.previousTaskID {
				root = task
				found = true
				break
			}
		}
		if !found {
			qm.mu.Unlock()
			return nil, fmt.Errorf("missing previous step")
		}
	}
	chain = append(chain, root)
	for {
		var next *BackgroundTask
		for _, task := range qm.tasks {
			if task.previousTaskID == chain[len(chain)-1].ID {
				next = task
				break
			}
		}
		if next == nil {
			break
		}
		chain = append(chain, next)
	}
	commands := make([][]string, 0, len(chain))
	for _, task := range chain {
		if task.Status == StatusRunning || task.Status == StatusQueued {
			qm.mu.Unlock()
			return nil, fmt.Errorf("chain is active")
		}
		if task.unrecoverableHook || task.onComplete != nil {
			qm.mu.Unlock()
			return nil, fmt.Errorf("this task replaces an original file; open its tool again to rebuild the safe output plan")
		}
		commands = append(commands, append([]string(nil), task.Cmd...))
	}
	title, source, target := root.Title, root.Source, root.Target
	qm.mu.Unlock()
	return qm.ReplayCommands(commands, title, source, target)
}

func (qm *BackgroundQueueManager) ReplayCommands(commands [][]string, title, source, target string) ([]*BackgroundTask, error) {
	if len(commands) == 0 {
		return nil, fmt.Errorf("no replayable command saved")
	}
	cloned := make([][]string, len(commands))
	for index, command := range commands {
		if len(command) == 0 {
			return nil, fmt.Errorf("empty command")
		}
		cloned[index] = append([]string(nil), command...)
	}
	commands = cloned
	passLogs := map[string]string{}
	for _, command := range commands {
		for index, argument := range command {
			if argument == "-passlogfile" && index+1 < len(command) {
				old := command[index+1]
				replacement, exists := passLogs[old]
				if !exists {
					replacement = filepath.Join(os.TempDir(), fmt.Sprintf("mediacli-pass-%d", time.Now().UnixNano()))
					passLogs[old] = replacement
				}
				command[index+1] = replacement
			}
		}
	}
	// A failed encode can leave a partial output. Use another name, never remove it.
	last := commands[len(commands)-1]
	isFFmpeg := len(last) > 0 && strings.Contains(strings.ToLower(filepath.Base(last[0])), "ffmpeg")
	for _, argument := range last {
		if argument == "-i" {
			isFFmpeg = true
		}
	}
	if isFFmpeg {
		old := last[len(last)-1]
		if _, err := os.Stat(old); err == nil {
			ext := filepath.Ext(old)
			target = strings.TrimSuffix(old, ext) + fmt.Sprintf("_retry_%d", time.Now().UnixNano()) + ext
			last[len(last)-1] = target
		}
	}
	return qm.EnqueueChain(commands, title, source, target, fmt.Sprintf("retry_%d", time.Now().UnixNano())), nil
}
