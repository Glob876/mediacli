package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BatchItem struct {
	Input    string
	Output   string
	Commands [][]string
	Error    string
}

type BatchReport struct{ Items []BatchItem }

// PlanBatch checks every input before queueing anything, including collisions inside the batch.
func PlanBatch(job BatchJob, cfg Config) BatchReport {
	report := BatchReport{}
	outputs := map[string]int{}
	for _, input := range job.Inputs {
		item := BatchItem{Input: input}
		single := job
		single.Inputs = []string{input}
		pipelines, err := ExpandBatch(single, cfg)
		if err == nil {
			item.Output = pipelines[0].Output.Path
			key, _ := filepath.Abs(item.Output)
			if previous, exists := outputs[key]; exists {
				err = fmt.Errorf("output conflicts with %s: %s", report.Items[previous].Input, item.Output)
				report.Items[previous].Error = err.Error()
			} else {
				outputs[key] = len(report.Items)
			}
			for _, source := range job.Inputs {
				sourcePath, _ := filepath.Abs(ParseUserPath(source))
				if key == sourcePath {
					err = fmt.Errorf("output overlaps a batch input: %s", source)
				}
			}
			if err == nil {
				if stat, statErr := os.Stat(filepath.Dir(item.Output)); statErr != nil || !stat.IsDir() {
					err = fmt.Errorf("output directory unavailable: %s", filepath.Dir(item.Output))
				}
			}
			if err == nil {
				item.Commands, err = BuildPipeline(pipelines[0], cfg)
			}
		}
		if err != nil {
			item.Error = err.Error()
		}
		report.Items = append(report.Items, item)
	}
	return report
}

func (report BatchReport) Enqueue() []int {
	var ids []int
	for _, item := range report.Items {
		if item.Error != "" {
			continue
		}
		tasks := GlobalQueue.EnqueueChain(item.Commands, "Batch", item.Input, item.Output, fmt.Sprintf("batch_%d", time.Now().UnixNano()))
		for _, task := range tasks {
			ids = append(ids, task.ID)
		}
	}
	return ids
}

func (report BatchReport) Lines() []string {
	var lines []string
	for _, item := range report.Items {
		if item.Error != "" {
			lines = append(lines, fmt.Sprintf("[!] %s: %s", item.Input, item.Error))
		} else {
			lines = append(lines, strings.Join([]string{item.Input, item.Output}, " → "))
		}
	}
	return lines
}
