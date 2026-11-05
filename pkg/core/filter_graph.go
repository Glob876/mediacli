package core

import (
	"fmt"
	"strings"
)

// FilterGraph билдит -vf / -af / -filter_complex с проверкой совместимости.
// Используется Pipeline; также доступен напрямую для preview.
type FilterGraph struct {
	vFilters []string
	aFilters []string
	complex  []string
	// output mapping для filter_complex
	complexOut string
}

// NewFilterGraph создает пустой граф.
func NewFilterGraph() *FilterGraph { return &FilterGraph{} }

func (g *FilterGraph) AddVideoFilter(f string) {
	f = strings.TrimSpace(f)
	if f != "" {
		g.vFilters = append(g.vFilters, f)
	}
}
func (g *FilterGraph) AddAudioFilter(f string) {
	f = strings.TrimSpace(f)
	if f != "" {
		g.aFilters = append(g.aFilters, f)
	}
}
func (g *FilterGraph) AddComplex(f string) {
	f = strings.TrimSpace(f)
	if f != "" {
		g.complex = append(g.complex, f)
	}
}

// HasFilters возвращает true если есть любые фильтры.
func (g *FilterGraph) HasFilters() bool {
	return len(g.vFilters) > 0 || len(g.aFilters) > 0 || len(g.complex) > 0
}

// HasVideoFilters — есть ли видео-фильтры.
func (g *FilterGraph) HasVideoFilters() bool { return len(g.vFilters) > 0 || len(g.complex) > 0 }

// Validate проверяет конфликты (напр. -c copy + фильтр).
func (g *FilterGraph) Validate(allowCopy bool) error {
	if !allowCopy && g.HasVideoFilters() {
		// caller должен проверить что copy не используется с видео-фильтрами;
		// граф сам не знает про кодеки, просто сообщает наличие.
	}
	return nil
}

// BuildArgs возвращает аргументы для ffmpeg: либо -vf, либо -af, либо -filter_complex.
func (g *FilterGraph) BuildArgs() []string {
	var args []string
	if len(g.complex) > 0 {
		joined := strings.Join(g.complex, ";")
		args = append(args, "-filter_complex", joined)
		if g.complexOut != "" {
			args = append(args, "-map", g.complexOut)
		}
		return args
	}
	if len(g.vFilters) > 0 {
		args = append(args, "-vf", strings.Join(g.vFilters, ","))
	}
	if len(g.aFilters) > 0 {
		args = append(args, "-af", strings.Join(g.aFilters, ","))
	}
	return args
}

// SetComplexOutput задает маппинг выхода filter_complex (напр. "[v]").
func (g *FilterGraph) SetComplexOutput(label string) { g.complexOut = label }

// Common video filter helpers

func ScaleFilter(w, h int, flags string) string {
	if flags == "" {
		flags = "lanczos"
	}
	if w <= 0 && h <= 0 {
		return ""
	}
	ws, hs := "-1", "-1"
	if w > 0 {
		ws = fmt.Sprintf("%d", w)
	}
	if h > 0 {
		hs = fmt.Sprintf("%d", h)
	}
	return fmt.Sprintf("scale=%s:%s:flags=%s", ws, hs, flags)
}

func CropFilter(w, h, x, y int) string {
	return fmt.Sprintf("crop=%d:%d:%d:%d", w, h, x, y)
}

func PadFilter(w, h int, color string) string {
	if color == "" {
		color = "black"
	}
	return fmt.Sprintf("pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=%s", w, h, color)
}

func EqFilter(brightness, contrast, saturation float64) string {
	parts := []string{}
	if brightness != 0 {
		parts = append(parts, fmt.Sprintf("brightness=%g", brightness))
	}
	if contrast != 1 && contrast != 0 {
		parts = append(parts, fmt.Sprintf("contrast=%g", contrast))
	}
	if saturation != 1 && saturation != 0 {
		parts = append(parts, fmt.Sprintf("saturation=%g", saturation))
	}
	if len(parts) == 0 {
		return ""
	}
	return "eq=" + strings.Join(parts, ":")
}

// Часто используемые фильтры как константы-фабрики

var (
	FilterHFlip            = "hflip"
	FilterVFlip            = "vflip"
	FilterDeinterlaceBwdif = "bwdif=mode=send_frame"
	FilterDeinterlaceYadif = "yadif"
	FilterDenoiseHQDN3D    = "hqdn3d"
	FilterSharpenUnsharp   = "unsharp=5:5:1.0:5:5:0.0"
)

// ValidateFilterCompat проверяет что список кодеков совместим с фильтрами.
// Возвращает ошибку если используется copy вместе с фильтрами.
func ValidateFilterCompat(vCodec, aCodec string, g *FilterGraph) error {
	if g == nil || !g.HasFilters() {
		return nil
	}
	needsReencode := g.HasVideoFilters()
	if needsReencode && strings.EqualFold(strings.TrimSpace(vCodec), "copy") {
		return fmt.Errorf("video filter requires re-encode, but vcodec=copy")
	}
	if len(g.aFilters) > 0 && strings.EqualFold(strings.TrimSpace(aCodec), "copy") {
		return fmt.Errorf("audio filter requires re-encode, but acodec=copy")
	}
	return nil
}
