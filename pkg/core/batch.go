package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BatchJob — пакетное применение пайплайна к списку файлов.
type BatchJob struct {
	Inputs         []string `json:"inputs"`
	Pipeline       Pipeline `json:"pipeline"`
	OutputDir      string   `json:"output_dir,omitempty"`
	OutputTemplate string   `json:"output_template,omitempty"` // e.g. "%(basename)s_converted.%(ext)s"
	Overwrite      bool     `json:"overwrite,omitempty"`
}

// ExpandBatch создаёт N задач из BatchJob (по одной на файл).
// outputTemplate поддерживает %(basename)s и %(ext)s.
func ExpandBatch(job BatchJob, cfg Config) ([]Pipeline, error) {
	if len(job.Inputs) == 0 {
		return nil, fmt.Errorf("no inputs")
	}
	var pipelines []Pipeline
	for _, inp := range job.Inputs {
		abs := ParseUserPath(inp)
		if st, err := os.Stat(abs); err != nil || st.IsDir() {
			// пропускаем несуществующие, но сообщаем
			continue
		}
		out := job.OutputTemplate
		if out == "" {
			out = "%(basename)s_converted.%(ext)s"
		}
		// resolve output path
		base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
		ext := ".mp4"
		// guess ext from pipeline's transcode
		for _, op := range job.Pipeline.Operations {
			if t, ok := op.(*TranscodeOp); ok && t.Container != "" {
				ext = "." + strings.TrimPrefix(t.Container, ".")
				break
			}
		}
		// if pipeline has explicit output ext, use it
		if job.Pipeline.Output.Path != "" {
			if e := filepath.Ext(job.Pipeline.Output.Path); e != "" {
				ext = e
			}
		}
		name := strings.ReplaceAll(out, "%(basename)s", base)
		name = strings.ReplaceAll(name, "%(ext)s", strings.TrimPrefix(ext, "."))
		name = strings.ReplaceAll(name, "%(filename)s", filepath.Base(abs))
		dir := job.OutputDir
		if dir == "" {
			dir = filepath.Dir(abs)
		}
		dir = ParseUserPath(dir)
		_ = os.MkdirAll(dir, 0755)
		final := filepath.Join(dir, name)
		// защита от перезаписи исходника
		if strings.EqualFold(filepath.Clean(abs), filepath.Clean(final)) {
			final = filepath.Join(dir, base+"_out"+ext)
		}
		p := Pipeline{
			ID:     fmt.Sprintf("%s_%s", job.Pipeline.ID, base),
			Name:   job.Pipeline.Name,
			Inputs: []string{abs},
			Output: OutputSpec{Path: final, Overwrite: job.Overwrite},
		}
		// копируем операции
		p.Operations = append([]Operation{}, job.Pipeline.Operations...)
		pipelines = append(pipelines, p)
	}
	if len(pipelines) == 0 {
		return nil, fmt.Errorf("no valid inputs for batch")
	}
	return pipelines, nil
}

// EnqueueBatch ставит пакет в очередь и возвращает IDs.
func EnqueueBatch(job BatchJob, cfg Config) ([]int, error) {
	if len(job.Inputs) == 0 {
		return nil, fmt.Errorf("no inputs")
	}
	report := PlanBatch(job, cfg)
	for _, item := range report.Items {
		if item.Error != "" {
			return nil, fmt.Errorf("%s: %s", item.Input, item.Error)
		}
	}
	return report.Enqueue(), nil
}
