package rendering

import (
	"unicode/utf8"

	"github.com/energye/gpui/render"
)

// PaintContext is the UI tree paint-walk cursor (not a second 2D engine).
// Drawing goes through DC (*render.Context); UI controls order, origin, and CompositeOnly.
//
// Default fonts live in render/text (LoadDefaultFace / SystemFontCandidates),
// not here — see default_font.go for the thin UI wrapper.
type PaintContext struct {
	DC               *render.Context
	OriginX, OriginY float64
	Scale            float64
	CompositeOnly    bool
	PaintVisits      *int64
	// LayerBudget limits SaveLayer ops this frame (F16). Nil = unlimited.
	LayerBudget *SaveLayerBudget
	// saveLayerDepth tracks unmatched SaveLayer pushes (Restore pairs).
	saveLayerDepth int
}

// NewPaintContext roots a paint walk at (0,0).
func NewPaintContext(dc *render.Context, scale float64) *PaintContext {
	if scale <= 0 {
		scale = 1
	}
	return &PaintContext{DC: dc, Scale: scale}
}

// WithOrigin returns a child cursor with absolute origin (logical Y-down).
// Shares LayerBudget and saveLayerDepth with the parent walk.
func (pc *PaintContext) WithOrigin(absX, absY float64) *PaintContext {
	if pc == nil {
		return &PaintContext{OriginX: absX, OriginY: absY, Scale: 1}
	}
	return &PaintContext{
		DC:             pc.DC,
		OriginX:        absX,
		OriginY:        absY,
		Scale:          pc.Scale,
		CompositeOnly:  pc.CompositeOnly,
		PaintVisits:    pc.PaintVisits,
		LayerBudget:    pc.LayerBudget,
		saveLayerDepth: pc.saveLayerDepth,
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

// --- unexported aliases (RO paint paths; public API is FillRect etc. in draw.go) ---

func fillRect(pc *PaintContext, x, y, w, h, r, g, b, a float64) {
	FillRect(pc, x, y, w, h, r, g, b, a)
}

// PushClipRect clips subsequent draws to a logical-axis-aligned rect (Y-down),
// origin-relative. Pairs with PopClip (render.Push/Pop).
func (pc *PaintContext) PushClipRect(x, y, w, h float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	ax, ay := pc.Abs(x, y)
	pc.DC.Push()
	pc.DC.ClipRect(ax, ay, w, h)
}

// PushClipRRect clips subsequent draws to a rounded rect in logical coordinates
// (uniform corner radius). radius<=0 falls back to a hard rect clip.
// Origin-aware like PushClipRect; pairs with PopClip.
func (pc *PaintContext) PushClipRRect(x, y, w, h, radius float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	ax, ay := pc.Abs(x, y)
	pc.DC.Push()
	if radius <= 0 {
		pc.DC.ClipRect(ax, ay, w, h)
		return
	}
	pc.DC.ClipRoundRect(ax, ay, w, h, radius)
}

// PushClipPath clips subsequent draws to path in local coordinates (origin
// applied by translating a clone — caller path is not mutated). Pairs with PopClip.
// CTM is not left translated; FillRect/etc. still use Abs() as usual.
func (pc *PaintContext) PushClipPath(path *render.Path) {
	if pc == nil || pc.DC == nil || path == nil || path.NumVerbs() == 0 {
		return
	}
	pc.DC.Push()
	abs := path.Clone()
	if pc.OriginX != 0 || pc.OriginY != 0 {
		abs.Transform(render.Translate(pc.OriginX, pc.OriginY))
	}
	pc.DC.ClearPath()
	pc.DC.AppendPath(abs)
	pc.DC.Clip()
}

// PopClip restores the clip/transform stack after PushClipRect, PushClipRRect,
// or PushClipPath.
func (pc *PaintContext) PopClip() {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.DC.Pop()
}

// SaveLayer begins an isolated offscreen layer (Flutter Canvas.saveLayer subset).
//
//	boundsW/H — logical size used for SaveLayerBudget area accounting (must be >0
//	             when a budget is set; full-surface isolation is still used by render).
//	opacity   — group opacity 0..1 when compositing back (≤0 treated as 1).
//
// Returns false if LayerBudget rejects the op (no layer pushed). Pair with Restore.
// Uses render.PushLayerIsolated (true offscreen, not F1 opacity-group).
func (pc *PaintContext) SaveLayer(boundsW, boundsH, opacity float64) bool {
	if pc == nil || pc.DC == nil {
		return false
	}
	if boundsW < 0 {
		boundsW = 0
	}
	if boundsH < 0 {
		boundsH = 0
	}
	if opacity <= 0 {
		opacity = 1
	}
	if opacity > 1 {
		opacity = 1
	}
	if pc.LayerBudget != nil && !pc.LayerBudget.Allow(boundsW, boundsH) {
		return false
	}
	pc.DC.PushLayerIsolated(opacity)
	pc.saveLayerDepth++
	return true
}

// Restore ends the most recent successful SaveLayer (PopLayer).
// No-op if depth is 0.
func (pc *PaintContext) Restore() {
	if pc == nil || pc.DC == nil || pc.saveLayerDepth <= 0 {
		return
	}
	pc.DC.PopLayer()
	pc.saveLayerDepth--
}

// SaveLayerDepth returns unmatched SaveLayer count (tests / diagnostics).
func (pc *PaintContext) SaveLayerDepth() int {
	if pc == nil {
		return 0
	}
	return pc.saveLayerDepth
}

// Package-level aliases used by RO paint paths in this package.
func pushClipRect(pc *PaintContext, x, y, w, h float64) { pc.PushClipRect(x, y, w, h) }
func popClip(pc *PaintContext)                          { pc.PopClip() }

// drawImageBuf is the unexported alias used by RenderImage.Paint.
func drawImageBuf(pc *PaintContext, img *render.ImageBuf, x, y, dstW, dstH float64) {
	DrawImageBuf(pc, img, x, y, dstW, dstH)
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
