package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FfprobeBin возвращает бинарник ffprobe (аналог FfmpegBin).
func FfprobeBin(cfg Config) string {
	if bin := strings.TrimSpace(cfg.FfmpegPath); bin != "" {
		// Если задан явный ffmpeg, пробуем ffprobe рядом с ним.
		p := ParseUserPath(bin)
		dir := filepath.Dir(p)
		base := filepath.Base(p)
		// ffmpeg -> ffprobe
		if strings.EqualFold(base, "ffmpeg") || strings.HasSuffix(strings.ToLower(base), "ffmpeg.exe") {
			cand := filepath.Join(dir, strings.Replace(base, "ffmpeg", "ffprobe", 1))
			// на Windows ffmpeg.exe -> ffprobe.exe
			if strings.HasSuffix(strings.ToLower(p), ".exe") && !strings.HasSuffix(strings.ToLower(cand), ".exe") {
				cand += ".exe"
			}
			if st, err := os.Stat(cand); err == nil && !st.IsDir() {
				return cand
			}
		}
		// иначе пробуем ffprobe в той же директории
		cand2 := filepath.Join(dir, "ffprobe")
		if st, err := os.Stat(cand2); err == nil && !st.IsDir() {
			return cand2
		}
		if filepath.IsAbs(p) || strings.Contains(p, string(filepath.Separator)) {
			// если указан путь с директорией, но ffprobe не рядом — всё равно пробуем PATH
		}
	}
	return "ffprobe"
}

// ResolveFfprobe проверяет доступность ffprobe.
func ResolveFfprobe(cfg Config, candidate string) (bin, version string, err error) {
	bin = FfprobeBin(cfg)
	if c := strings.TrimSpace(candidate); c != "" {
		bin = ParseUserPath(c)
	}
	if filepath.IsAbs(bin) || strings.Contains(bin, string(filepath.Separator)) {
		st, statErr := os.Stat(bin)
		if statErr != nil || st.IsDir() {
			return bin, "", fmt.Errorf("file not found: %s", bin)
		}
		if statErr == nil && st.Mode().Perm()&0111 == 0 {
			return bin, "", fmt.Errorf("not executable: %s", bin)
		}
	} else if p, lookErr := exec.LookPath(bin); lookErr == nil {
		bin = p
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, runErr := exec.CommandContext(ctx, bin, "-version").CombinedOutput()
	if runErr != nil {
		detail := strings.TrimSpace(string(out))
		if len(detail) > 220 {
			detail = detail[:220] + "…"
		}
		if detail == "" {
			detail = runErr.Error()
		}
		return bin, "", fmt.Errorf("cannot run: %s", detail)
	}
	first, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return bin, first, nil
}

// MediaInfo — расширенный результат ffprobe.
type MediaInfo struct {
	Path     string        `json:"path"`
	Format   FormatInfo    `json:"format"`
	Streams  []StreamInfo  `json:"streams"`
	Chapters []ChapterInfo `json:"chapters"`
}

type FormatInfo struct {
	FormatName   string            `json:"format_name,omitempty"`
	FormatLong   string            `json:"format_long_name,omitempty"`
	Duration     string            `json:"duration,omitempty"`
	DurationSec  float64           `json:"duration_sec,omitempty"`
	Size         string            `json:"size,omitempty"`
	BitRate      string            `json:"bit_rate,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	ProbeScore   int               `json:"probe_score,omitempty"`
}

type Disposition struct {
	Default         int `json:"default"`
	Dub             int `json:"dub"`
	Original        int `json:"original"`
	Comment         int `json:"comment"`
	Lyrics          int `json:"lyrics"`
	Karaoke         int `json:"karaoke"`
	Forced          int `json:"forced"`
	HearingImpaired int `json:"hearing_impaired"`
	VisualImpaired  int `json:"visual_impaired"`
	CleanEffects    int `json:"clean_effects"`
	AttachedPic     int `json:"attached_pic"`
	TimedThumbnails int `json:"timed_thumbnails"`
}

type StreamInfo struct {
	Index              int               `json:"index"`
	CodecType          string            `json:"codec_type"`
	CodecName          string            `json:"codec_name"`
	CodecLongName      string            `json:"codec_long_name,omitempty"`
	Width              int               `json:"width,omitempty"`
	Height             int               `json:"height,omitempty"`
	CodedWidth         int               `json:"coded_width,omitempty"`
	CodedHeight        int               `json:"coded_height,omitempty"`
	DisplayAspectRatio string            `json:"display_aspect_ratio,omitempty"`
	SampleAspectRatio  string            `json:"sample_aspect_ratio,omitempty"`
	PixFmt             string            `json:"pix_fmt,omitempty"`
	Profile            string            `json:"profile,omitempty"`
	Level              int               `json:"level,omitempty"`
	BitRate            string            `json:"bit_rate,omitempty"`
	Duration           string            `json:"duration,omitempty"`
	SampleRate         string            `json:"sample_rate,omitempty"`
	Channels           int               `json:"channels,omitempty"`
	ChannelLayout      string            `json:"channel_layout,omitempty"`
	Language           string            `json:"language,omitempty"`
	Disposition        Disposition       `json:"disposition"`
	Tags               map[string]string `json:"tags,omitempty"`
	RFrameRate         string            `json:"r_frame_rate,omitempty"`
	AvgFrameRate       string            `json:"avg_frame_rate,omitempty"`
	TimeBase           string            `json:"time_base,omitempty"`
	StartTime          string            `json:"start_time,omitempty"`
	BitDepth           string            `json:"bits_per_raw_sample,omitempty"`
}

type ChapterInfo struct {
	ID        int               `json:"id"`
	TimeBase  string            `json:"time_base,omitempty"`
	Start     int64             `json:"start"`
	StartTime string            `json:"start_time,omitempty"`
	End       int64             `json:"end"`
	EndTime   string            `json:"end_time,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// rawProbe — сырой JSON ffprobe.
type rawProbe struct {
	Format struct {
		FormatName   string            `json:"format_name"`
		FormatLong   string            `json:"format_long_name"`
		Duration     string            `json:"duration"`
		Size         string            `json:"size"`
		BitRate      string            `json:"bit_rate"`
		Tags         map[string]string `json:"tags"`
		ProbeScore   int               `json:"probe_score"`
	} `json:"format"`
	Streams []struct {
		Index              int               `json:"index"`
		CodecType          string            `json:"codec_type"`
		CodecName          string            `json:"codec_name"`
		CodecLongName      string            `json:"codec_long_name"`
		Width              int               `json:"width"`
		Height             int               `json:"height"`
		CodedWidth         int               `json:"coded_width"`
		CodedHeight        int               `json:"coded_height"`
		DisplayAspectRatio string            `json:"display_aspect_ratio"`
		SampleAspectRatio  string            `json:"sample_aspect_ratio"`
		PixFmt             string            `json:"pix_fmt"`
		Profile            string            `json:"profile"`
		Level              int               `json:"level"`
		BitRate            string            `json:"bit_rate"`
		Duration           string            `json:"duration"`
		SampleRate         string            `json:"sample_rate"`
		Channels           int               `json:"channels"`
		ChannelLayout      string            `json:"channel_layout"`
		Tags               map[string]string `json:"tags"`
		Disposition        Disposition       `json:"disposition"`
		RFrameRate         string            `json:"r_frame_rate"`
		AvgFrameRate       string            `json:"avg_frame_rate"`
		TimeBase           string            `json:"time_base"`
		StartTime          string            `json:"start_time"`
		BitsPerRawSample   string            `json:"bits_per_raw_sample"`
		BitsPerSample      json.RawMessage   `json:"bits_per_sample"`
	} `json:"streams"`
	Chapters []struct {
		ID        int               `json:"id"`
		TimeBase  string            `json:"time_base"`
		Start     int64             `json:"start"`
		StartTime string            `json:"start_time"`
		End       int64             `json:"end"`
		EndTime   string            `json:"end_time"`
		Tags      map[string]string `json:"tags"`
	} `json:"chapters"`
}

// ProbeMediaInfo — расширенный probe (format + streams + chapters, все теги).
func ProbeMediaInfo(filePath string) (*MediaInfo, error) {
	cfg, _ := LoadConfig()
	return ProbeMediaInfoWithConfig(filePath, cfg)
}

func ProbeMediaInfoWithConfig(filePath string, cfg Config) (*MediaInfo, error) {
	bin := FfprobeBin(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", "-show_chapters", filePath)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var raw rawProbe
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	info := &MediaInfo{Path: filePath}
	info.Format.FormatName = raw.Format.FormatName
	info.Format.FormatLong = raw.Format.FormatLong
	info.Format.Duration = raw.Format.Duration
	info.Format.Size = raw.Format.Size
	info.Format.BitRate = raw.Format.BitRate
	info.Format.Tags = raw.Format.Tags
	info.Format.ProbeScore = raw.Format.ProbeScore
	if raw.Format.Duration != "" {
		if f, err := strconv.ParseFloat(raw.Format.Duration, 64); err == nil {
			info.Format.DurationSec = f
		}
	}
	for _, s := range raw.Streams {
		si := StreamInfo{
			Index:              s.Index,
			CodecType:          s.CodecType,
			CodecName:          s.CodecName,
			CodecLongName:      s.CodecLongName,
			Width:              s.Width,
			Height:             s.Height,
			CodedWidth:         s.CodedWidth,
			CodedHeight:        s.CodedHeight,
			DisplayAspectRatio: s.DisplayAspectRatio,
			SampleAspectRatio:  s.SampleAspectRatio,
			PixFmt:             s.PixFmt,
			Profile:            s.Profile,
			Level:              s.Level,
			BitRate:            s.BitRate,
			Duration:           s.Duration,
			SampleRate:         s.SampleRate,
			Channels:           s.Channels,
			ChannelLayout:      s.ChannelLayout,
			Disposition:        s.Disposition,
			Tags:               s.Tags,
			RFrameRate:         s.RFrameRate,
			AvgFrameRate:       s.AvgFrameRate,
			TimeBase:           s.TimeBase,
			StartTime:          s.StartTime,
			BitDepth:           s.BitsPerRawSample,
		}
		if lang, ok := s.Tags["language"]; ok {
			si.Language = lang
		}
		// bits_per_raw_sample иногда число, иногда строка
		if si.BitDepth == "" && len(s.BitsPerSample) > 0 {
			var asInt int
			if err := json.Unmarshal(s.BitsPerSample, &asInt); err == nil {
				si.BitDepth = strconv.Itoa(asInt)
			} else {
				var asStr string
				if err := json.Unmarshal(s.BitsPerSample, &asStr); err == nil {
					si.BitDepth = asStr
				}
			}
		}
		info.Streams = append(info.Streams, si)
	}
	for _, c := range raw.Chapters {
		info.Chapters = append(info.Chapters, ChapterInfo{
			ID: c.ID, TimeBase: c.TimeBase, Start: c.Start, StartTime: c.StartTime, End: c.End, EndTime: c.EndTime, Tags: c.Tags,
		})
	}
	return info, nil
}

// ProbeMediaDetailed — алиас для совместимости.
func ProbeMediaDetailed(filePath string) (*MediaInfo, error) {
	return ProbeMediaInfo(filePath)
}
