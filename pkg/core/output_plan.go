package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FFmpegOutputPlan struct {
	TempOutputPath string
	FinalPath      string
	TargetDisplay  string
	OnComplete     func(exitCode int) error
}

func PrepareFFmpegOutput(inputPath string, targetExt string, defaultSuffix string, cfg Config) FFmpegOutputPlan {
	inputExt := filepath.Ext(inputPath)
	targetExtWithDot := targetExt
	if !strings.HasPrefix(targetExtWithDot, ".") {
		targetExtWithDot = "." + targetExtWithDot
	}
	base := strings.TrimSuffix(inputPath, inputExt)
	dir := filepath.Dir(inputPath)
	filename := filepath.Base(base)

	if cfg.OverwriteOriginal {
		finalTarget := filepath.Join(dir, filename+targetExtWithDot)
		temporaryName := fmt.Sprintf(".%s_tmp_%d%s", filename, time.Now().UnixNano(), targetExtWithDot)
		temporaryPath := filepath.Join(dir, temporaryName)
		return FFmpegOutputPlan{
			TempOutputPath: temporaryPath,
			FinalPath:      finalTarget,
			TargetDisplay:  filepath.Base(finalTarget) + " (Replace Original)",
			OnComplete: func(exitCode int) error {
				if exitCode != 0 {
					err := os.Remove(temporaryPath)
					if os.IsNotExist(err) {
						return nil
					}
					return err
				}
				info, err := os.Stat(temporaryPath)
				if err != nil {
					return fmt.Errorf("inspect converted file: %w", err)
				}
				if !info.Mode().IsRegular() || info.Size() == 0 {
					return fmt.Errorf("converted file is empty or not a regular file: %s", temporaryPath)
				}
				sameSource := filepath.Clean(inputPath) == filepath.Clean(finalTarget)
				sourceInfo, sourceError := os.Stat(inputPath)
				destinationInfo, destinationError := os.Stat(finalTarget)
				if sourceError == nil && destinationError == nil {
					sameSource = sameSource || os.SameFile(sourceInfo, destinationInfo)
				}
				// Rename replaces the destination atomically on supported systems.
				// If replacement is unavailable, preserve both files and report the error.
				if err := os.Rename(temporaryPath, finalTarget); err != nil {
					return fmt.Errorf("install converted file (source preserved): %w", err)
				}
				if !sameSource {
					if err := os.Remove(inputPath); err != nil {
						return fmt.Errorf("converted file saved; remove original: %w", err)
					}
				}
				return nil
			},
		}
	}

	useSuffix := cfg.UseFFmpegSuffix
	isDiffExt := !strings.EqualFold(inputExt, targetExtWithDot)

	var finalName string
	if !useSuffix && isDiffExt {
		finalName = filename + targetExtWithDot
	} else {
		finalName = filename + defaultSuffix + targetExtWithDot
	}

	finalPath := filepath.Join(dir, finalName)

	return FFmpegOutputPlan{
		TempOutputPath: finalPath,
		FinalPath:      finalPath,
		TargetDisplay:  finalName,
		OnComplete:     nil,
	}
}
