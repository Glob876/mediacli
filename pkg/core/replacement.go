package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type OutputReplacement struct {
	Input     string
	Temporary string
	Final     string
}

var replacements sync.Map

func (replacement OutputReplacement) Complete(exitCode int) error {
	if exitCode != 0 {
		err := os.Remove(replacement.Temporary)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	info, err := os.Stat(replacement.Temporary)
	if err != nil {
		return fmt.Errorf("inspect converted file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("converted file is empty or not a regular file: %s", replacement.Temporary)
	}
	sameSource := filepath.Clean(replacement.Input) == filepath.Clean(replacement.Final)
	sourceInfo, sourceError := os.Stat(replacement.Input)
	destinationInfo, destinationError := os.Stat(replacement.Final)
	if sourceError == nil && destinationError == nil {
		sameSource = sameSource || os.SameFile(sourceInfo, destinationInfo)
	}
	if err := os.Rename(replacement.Temporary, replacement.Final); err != nil {
		return fmt.Errorf("install converted file (source preserved): %w", err)
	}
	if !sameSource {
		if err := os.Remove(replacement.Input); err != nil {
			return fmt.Errorf("converted file saved; remove original: %w", err)
		}
	}
	return nil
}

func (replacement OutputReplacement) RetryCommand(command []string) ([]string, OutputReplacement) {
	ext := filepath.Ext(replacement.Final)
	base := strings.TrimSuffix(filepath.Base(replacement.Final), ext)
	replacement.Temporary = filepath.Join(filepath.Dir(replacement.Final), fmt.Sprintf(".%s_tmp_%d%s", base, time.Now().UnixNano(), ext))
	command = append([]string(nil), command...)
	command[len(command)-1] = replacement.Temporary
	replacements.Store(replacement.Temporary, replacement)
	return command, replacement
}
