package core

import (
	"fmt"
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
		replacement := OutputReplacement{Input: inputPath, Temporary: temporaryPath, Final: finalTarget}
		replacements.Store(temporaryPath, replacement)
		return FFmpegOutputPlan{TempOutputPath: temporaryPath, FinalPath: finalTarget, TargetDisplay: filepath.Base(finalTarget) + " (Replace Original)", OnComplete: replacement.Complete}
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
