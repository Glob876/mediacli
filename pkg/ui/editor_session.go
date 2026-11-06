package ui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"mediacli/pkg/core"
)

type editorSession struct {
	output  core.OutputSpec
	path    string
	current []byte
	undo    [][]byte
	redo    [][]byte
}

func newEditorSession(name string) *editorSession {
	return &editorSession{path: filepath.Join(core.GetConfigDir(), name+"_draft.json")}
}

func (session *editorSession) restore(s tcell.Screen, cfg *core.Config) (string, []core.Operation) {
	data, err := os.ReadFile(session.path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		ShowMessage(s, cfg, "Editor", []string{err.Error()}, T(*cfg, "footer_message"))
		return "", nil
	}
	pipeline, err := core.UnmarshalPipeline(data)
	if err != nil || len(pipeline.Inputs) != 1 {
		ShowMessage(s, cfg, "Editor", []string{paletteText(*cfg, "Не удалось прочитать черновик", "Cannot read draft")}, T(*cfg, "footer_message"))
		return "", nil
	}
	choice := RunMenu(s, cfg, paletteText(*cfg, "Сохранённый проект", "Saved project"), []string{paletteText(*cfg, "Восстановить", "Restore"), paletteText(*cfg, "Начать новый", "Start new")}, pipeline.Inputs[0], T(*cfg, "footer_nav"))
	if choice != 0 {
		return "", nil
	}
	if stat, err := os.Stat(pipeline.Inputs[0]); err != nil || stat.IsDir() {
		ShowMessage(s, cfg, "Editor", []string{pipeline.Inputs[0], paletteText(*cfg, "Исходный файл недоступен", "Source file unavailable")}, T(*cfg, "footer_message"))
		return "", nil
	}
	session.output = pipeline.Output
	return pipeline.Inputs[0], pipeline.Operations
}

func (session *editorSession) capture(path string, ops []core.Operation) error {
	data, err := core.MarshalPipeline(core.Pipeline{Inputs: []string{path}, Operations: ops, Output: session.output})
	if err != nil {
		return err
	}
	if bytes.Equal(session.current, data) {
		return nil
	}
	if session.current != nil {
		session.undo = append(session.undo, session.current)
		if len(session.undo) > 100 {
			session.undo = session.undo[1:]
		}
	}
	session.redo = nil
	session.current = data
	return core.WritePrivateFile(session.path, data)
}

func (session *editorSession) move(redo bool) (string, []core.Operation, error) {
	source, destination := &session.undo, &session.redo
	if redo {
		source, destination = &session.redo, &session.undo
	}
	if len(*source) == 0 {
		return "", nil, fmt.Errorf("%s", "No more changes")
	}
	data := (*source)[len(*source)-1]
	pipeline, err := core.UnmarshalPipeline(data)
	if err != nil {
		return "", nil, err
	}
	if err := core.WritePrivateFile(session.path, data); err != nil {
		return "", nil, err
	}
	*source = (*source)[:len(*source)-1]
	*destination = append(*destination, session.current)
	session.current = data
	session.output = pipeline.Output
	return pipeline.Inputs[0], pipeline.Operations, nil
}
