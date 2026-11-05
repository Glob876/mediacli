package core

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func GetString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case string:
			return val
		case float64:
			if val == float64(int64(val)) {
				return strconv.Itoa(int(val))
			}
			return strconv.FormatFloat(val, 'f', -1, 64)
		case float32:
			if val == float32(int(val)) {
				return strconv.Itoa(int(val))
			}
			return strconv.FormatFloat(float64(val), 'f', -1, 64)
		case int:
			return strconv.Itoa(val)
		case int64:
			return strconv.FormatInt(val, 10)
		case int32:
			return strconv.Itoa(int(val))
		case json.Number:
			return val.String()
		}
	}
	return ""
}

// BuildFormatExpr builds a yt-dlp -f expression that respects quality/fps/vcodec
// and never falls back to unconditional /best (which yields 360p progressive).
func BuildFormatExpr(quality, fps, vcodec string) string {
	fpsSuffix := ""
	if fps != "" {
		fpsSuffix = fmt.Sprintf("[fps<=%s]", fps)
	}
	qStr := ""
	if quality != "" {
		qStr = fmt.Sprintf("[height<=%s]", quality)
	}
	vcFilter := ""
	switch vcodec {
	case "av1":
		vcFilter = "[vcodec~='^av0?1']"
	case "vp9":
		vcFilter = "[vcodec~='^vp0?9']"
	case "h264":
		vcFilter = "[vcodec~='^avc']"
	}
	// Prefer bestvideo+bestaudio with height/fps + optional codec; progressive fallback keeps height/fps(+codec)
	// Using bv/ba aliases (yt-dlp) and b for best progressive.
	if vcFilter != "" {
		// With codec: try codec-constrained bv+ba, then codec-constrained progressive, then any codec bv+ba, then any progressive
		return fmt.Sprintf("bv%s%s%s+ba/b%s%s%s/bv%s%s+ba/b%s%s", qStr, fpsSuffix, vcFilter, qStr, fpsSuffix, vcFilter, qStr, fpsSuffix, qStr, fpsSuffix)
	}
	return fmt.Sprintf("bv%s%s+ba/b%s%s", qStr, fpsSuffix, qStr, fpsSuffix)
}

func GetBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func GetBoolDefault(m map[string]interface{}, key string, def bool) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}
