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
	// BoundaryCache enables W1 Picture-backed RepaintBoundary reuse (R3).
	BoundaryCache *BoundaryCache
	// UseBoundaryCache gates tryReplay/store on repaint boundaries.
	UseBoundaryCache bool
	// DebugRepaint (R12b): after a live (non-Replay) paint of a node, draw a
	// translucent overlay so humans can see who is re-painting this frame.
	DebugRepaint bool
	// DebugRepaintDraws counts overlay strokes this walk (optional metrics).
	DebugRepaintDraws *int64
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
		DC:                pc.DC,
		OriginX:           absX,
		OriginY:           absY,
		Scale:             pc.Scale,
		CompositeOnly:     pc.CompositeOnly,
		PaintVisits:       pc.PaintVisits,
		LayerBudget:       pc.LayerBudget,
		saveLayerDepth:    pc.saveLayerDepth,
		BoundaryCache:     pc.BoundaryCache,
		UseBoundaryCache:  pc.UseBoundaryCache,
		DebugRepaint:      pc.DebugRepaint,
		DebugRepaintDraws: pc.DebugRepaintDraws,
	}
}

// NoteDebugRepaint draws a translucent magenta overlay at the current origin
// covering w×h (logical). Used by R12b to mark live re-paints. No-op when
// DebugRepaint is false or size non-positive.
func (pc *PaintContext) NoteDebugRepaint(w, h float64) {
	if pc == nil || !pc.DebugRepaint || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	// Magenta flash, semi-transparent — visible over both light and dark fills.
	fillRect(pc, 0, 0, w, h, 0.95, 0.15, 0.85, 0.35)
	if pc.DebugRepaintDraws != nil {
		*pc.DebugRepaintDraws++
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

// RestoreLayer ends the most recent successful SaveLayer (PopLayer).
// No-op if depth is 0. Prefer this name over Restore to avoid confusion with
// canvas Save/Restore (CTM+clip stack).
func (pc *PaintContext) RestoreLayer() {
	if pc == nil || pc.DC == nil || pc.saveLayerDepth <= 0 {
		return
	}
	pc.DC.PopLayer()
	pc.saveLayerDepth--
}

// Restore is an alias of RestoreLayer for Flutter-like saveLayer/restore pairing.
// Does not pop CTM/clip — use RestoreCanvas for that.
func (pc *PaintContext) Restore() { pc.RestoreLayer() }

// SaveLayerDepth returns unmatched SaveLayer count (tests / diagnostics).
func (pc *PaintContext) SaveLayerDepth() int {
	if pc == nil {
		return 0
	}
	return pc.saveLayerDepth
}

// --- CTM / transform (FC-TRANSLATE / SCALE / ROTATE / SKEW / TRANSFORM) ---

// Matrix is a 2D affine matrix alias (render.Matrix: x'=ax+by+c, y'=dx+ey+f).
type Matrix = render.Matrix

// Save pushes canvas state (CTM + clip + paint bits) — Canvas.save.
// Pair with RestoreCanvas. Does not push a saveLayer.
func (pc *PaintContext) Save() {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.DC.Push()
}

// RestoreCanvas pops the last Save() (CTM+clip). No-op if stack empty.
func (pc *PaintContext) RestoreCanvas() {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.DC.Pop()
}

// localToAbsMatrix maps a local-space affine M so Abs-based draws transform as
// if M were applied in local coordinates about the paint origin:
//
//	M_abs = T(origin) · M · T(-origin)
func (pc *PaintContext) localToAbsMatrix(m render.Matrix) render.Matrix {
	ox, oy := float64(0), float64(0)
	if pc != nil {
		ox, oy = pc.OriginX, pc.OriginY
	}
	if ox == 0 && oy == 0 {
		return m
	}
	return render.Translate(ox, oy).Multiply(m).Multiply(render.Translate(-ox, -oy))
}

// Concat multiplies the CTM by m in local paint space (Canvas.transform subset).
// 2D affine only — not full Matrix4 / perspective.
func (pc *PaintContext) Concat(m render.Matrix) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.DC.Transform(pc.localToAbsMatrix(m))
}

// PushTransform saves CTM then Concats m (local space). Pair with PopTransform.
func (pc *PaintContext) PushTransform(m render.Matrix) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.DC.Push()
	pc.DC.Transform(pc.localToAbsMatrix(m))
}

// PopTransform restores CTM after PushTransform (same as RestoreCanvas).
func (pc *PaintContext) PopTransform() {
	pc.RestoreCanvas()
}

// Translate applies a local-space translation (Canvas.translate).
func (pc *PaintContext) Translate(dx, dy float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.Concat(render.Translate(dx, dy))
}

// ScaleXY applies local-space scale about the paint origin (Canvas.scale).
// Named ScaleXY to avoid clashing with the DPR Scale field on PaintContext.
func (pc *PaintContext) ScaleXY(sx, sy float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.Concat(render.Scale(sx, sy))
}

// Rotate applies local-space rotation in radians about the paint origin (Canvas.rotate).
func (pc *PaintContext) Rotate(radians float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.Concat(render.Rotate(radians))
}

// RotateAbout rotates about a local-space point (cx,cy).
func (pc *PaintContext) RotateAbout(radians, cx, cy float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.Concat(render.Translate(cx, cy).Multiply(render.Rotate(radians)).Multiply(render.Translate(-cx, -cy)))
}

// Shear applies local-space shear (Canvas.skew subset via Shear).
func (pc *PaintContext) Shear(shx, shy float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.Concat(render.Shear(shx, shy))
}

// GetTransform returns the current DC user matrix (copy).
func (pc *PaintContext) GetTransform() render.Matrix {
	if pc == nil || pc.DC == nil {
		return render.Identity()
	}
	return pc.DC.GetTransform()
}

// IdentityMatrix returns the 2D identity matrix.
func IdentityMatrix() render.Matrix { return render.Identity() }

// TranslateMatrix / ScaleMatrix / RotateMatrix build common affines.
func TranslateMatrix(dx, dy float64) render.Matrix { return render.Translate(dx, dy) }
func ScaleMatrix(sx, sy float64) render.Matrix     { return render.Scale(sx, sy) }
func RotateMatrix(radians float64) render.Matrix   { return render.Rotate(radians) }

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
