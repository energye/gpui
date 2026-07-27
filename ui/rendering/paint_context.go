package rendering

import (
	"fmt"
	"os"
	"unicode/utf8"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// PaintContext is the UI tree paint-walk cursor (not a second 2D engine).
// Drawing goes through DC (*render.Context); UI controls order, origin, and CompositeOnly.
type PaintContext struct {
	DC               *render.Context
	OriginX, OriginY float64
	Scale            float64
	CompositeOnly    bool
	PaintVisits      *int64
}

// NewPaintContext roots a paint walk at (0,0).
func NewPaintContext(dc *render.Context, scale float64) *PaintContext {
	if scale <= 0 {
		scale = 1
	}
	return &PaintContext{DC: dc, Scale: scale}
}

// WithOrigin returns a child cursor with absolute origin (logical Y-down).
func (pc *PaintContext) WithOrigin(absX, absY float64) *PaintContext {
	if pc == nil {
		return &PaintContext{OriginX: absX, OriginY: absY, Scale: 1}
	}
	return &PaintContext{
		DC:            pc.DC,
		OriginX:       absX,
		OriginY:       absY,
		Scale:         pc.Scale,
		CompositeOnly: pc.CompositeOnly,
		PaintVisits:   pc.PaintVisits,
	}
}

// NotePaintVisit increments PaintVisits when non-nil.
func (pc *PaintContext) NotePaintVisit() {
	if pc != nil && pc.PaintVisits != nil {
		*pc.PaintVisits++
	}
}

// Abs maps local logical (x,y) to absolute canvas coordinates.
func (pc *PaintContext) Abs(x, y float64) (ax, ay float64) {
	if pc == nil {
		return x, y
	}
	return pc.OriginX + x, pc.OriginY + y
}

// --- unexported draw helpers (same package only; call render, keep RO readable) ---

func fillRect(pc *PaintContext, x, y, w, h, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	ax, ay := pc.Abs(x, y)
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
}

func pushClipRect(pc *PaintContext, x, y, w, h float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	ax, ay := pc.Abs(x, y)
	pc.DC.Push()
	pc.DC.ClipRect(ax, ay, w, h)
}

func popClip(pc *PaintContext) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.DC.Pop()
}

func drawImageBuf(pc *PaintContext, img *render.ImageBuf, x, y, dstW, dstH float64) {
	if pc == nil || pc.DC == nil || img == nil {
		return
	}
	ax, ay := pc.Abs(x, y)
	if dstW <= 0 || dstH <= 0 {
		pc.DC.DrawImage(img, ax, ay)
		return
	}
	pc.DC.DrawImageEx(img, render.DrawImageOptions{
		X: ax, Y: ay, DstWidth: dstW, DstHeight: dstH,
	})
}

func drawTextColored(pc *PaintContext, s string, x, y, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || s == "" {
		return
	}
	ax, ay := pc.Abs(x, y)
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.DrawString(s, ax, ay)
}

func drawTextWrapped(pc *PaintContext, s string, x, y, width, lineSpacing float64, align render.Align, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || s == "" || width <= 0 {
		return
	}
	if lineSpacing <= 0 {
		lineSpacing = 1.2
	}
	ax, ay := pc.Abs(x, y)
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.DrawStringWrapped(s, ax, ay, 0, 0, width, lineSpacing, align)
}

// EstimateTextSize estimates layout size without a font (rune-based).
func EstimateTextSize(s string, fontSize, approxCharW float64) (w, h float64) {
	if fontSize <= 0 {
		fontSize = 14
	}
	if approxCharW <= 0 {
		approxCharW = 0.55
	}
	n := float64(utf8.RuneCountInString(s))
	return n * fontSize * approxCharW, fontSize * 1.25
}

// SaveLayerBudget limits expensive saveLayer-style ops (F16).
type SaveLayerBudget struct {
	MaxOps  int
	MaxArea float64
	ops     int
	area    float64
}

// Reset clears per-frame counters.
func (b *SaveLayerBudget) Reset() {
	if b == nil {
		return
	}
	b.ops = 0
	b.area = 0
}

// Allow reports whether another saveLayer of the given logical area is within budget.
func (b *SaveLayerBudget) Allow(w, h float64) bool {
	if b == nil {
		return true
	}
	a := w * h
	if b.MaxOps > 0 && b.ops+1 > b.MaxOps {
		return false
	}
	if b.MaxArea > 0 && b.area+a > b.MaxArea {
		return false
	}
	b.ops++
	b.area += a
	return true
}

var defaultFontCandidates = []string{
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/TTF/DejaVuSans.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
	"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
	"/usr/share/fonts/truetype/arphic/uming.ttc",
}

// TryLoadDefaultFace loads a system UI font. Override with GPUI_UI_FONT.
func TryLoadDefaultFace(points float64) (text.Face, string, error) {
	if points <= 0 {
		points = 14
	}
	candidates := defaultFontCandidates
	if p := os.Getenv("GPUI_UI_FONT"); p != "" {
		candidates = append([]string{p}, candidates...)
	}
	var lastErr error
	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		src, err := text.NewFontSourceFromFile(path)
		if err != nil {
			lastErr = err
			continue
		}
		return src.Face(points), path, nil
	}
	if lastErr != nil {
		return nil, "", fmt.Errorf("rendering: no default font (last err: %w)", lastErr)
	}
	return nil, "", fmt.Errorf("rendering: no default font found (set GPUI_UI_FONT)")
}
