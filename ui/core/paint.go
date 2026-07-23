package core

import "github.com/energye/gpui/render"

// LayerCache is the host-provided retained GPU surface store for RepaintBoundary
// (ui/layer.Cache). Defined as an interface here so core does not import ui/layer.
type LayerCache interface {
	// BlitBoundary draws a cached layer for key at origin if valid; returns false if miss.
	BlitBoundary(key uintptr, parent *render.Context, x, y float64, w, h int) bool
	// RasterizeBoundary ensures key has a fresh texture painted via paintFn into
	// an offscreen context of size (w,h) at scale; then blits to parent. paintFn
	// receives a PaintContext whose DC is the offscreen surface and Origin is 0.
	// Returns false if GPU cache is unavailable (caller falls back to direct paint).
	RasterizeBoundary(key uintptr, parent *render.Context, x, y float64, w, h int, scale float64, paintFn func(pc *PaintContext)) bool
	// InvalidateBoundary marks key invalid.
	InvalidateBoundary(key uintptr)
	// ReleaseAll drops all layers.
	ReleaseAll()
}

// LayerBand selects the compositor stacking band for RepaintBoundary layers.
// Main layers blit before Overlay (Flutter: Overlay above the app retained tree).
type LayerBand int

const (
	LayerBandMain LayerBand = iota
	LayerBandOverlay
)

// PaintContext is the only drawing surface for nodes.
// DC is a render.Context; final pixels go through PresentFrame* at host level.
// Nodes must not open a silent CPU bitmap as the final frame.
type PaintContext struct {
	// DC is the active render context (required for real paint).
	DC *render.Context
	// Origin is the absolute top-left of the current node in logical pixels.
	Origin Point
	// Scale is the device pixel ratio (1.0 = 96 DPI).
	Scale float64
	// Theme is optional token/skin access.
	Theme *Theme
	// Clip is the active absolute clip in logical pixels (optional advisory).
	// Used by DefaultPaintChildren to skip fully off-screen subtrees (scroll cull).
	Clip Rect
	// prevClips + clipDepth restore Clip across PushClipLocal/Pop without heap.
	prevClips [6]Rect
	clipDepth int
	// CompositeOnly: retained frame — skip clean non-boundary subtrees; RepaintBoundary
	// nodes blit cached layers. Requires prior full frame + LoadOpLoad-capable present
	// (or accept holes if the surface was cleared). Hosts set this after the first
	// full paint when only boundary layers are dirty.
	CompositeOnly bool
	// ForceFullPaint disables CompositeOnly skip for this subtree (used when a node
	// itself is paint-dirty and must redraw non-boundary children).
	ForceFullPaint bool
	// LayerCache optional GPU boundary texture cache (Phase B).
	LayerCache LayerCache
	// DeferLayerBlit: when true with LayerCache, RepaintBoundary only updates the
	// offscreen RT (RasterizeBoundary) and does not blit into DC. The host
	// Compositor blits all layers in a later blit-only pass (G2.b).
	DeferLayerBlit bool
	// SkipRepaintBoundaries: DefaultPaintChildren skips IsRepaintBoundary children
	// (transparent holes). Used when ScrollViewport rasterizes its RT so nested
	// Spin/Skeleton keep independent layers and are not re-baked every frame.
	// CPU for a single Spin must not scale with window/content size.
	SkipRepaintBoundaries bool
	// LayerBand is Main (default) or Overlay. Portal/modal paint sets Overlay so
	// retained layers under the mask never composite above the dimmer.
	LayerBand LayerBand
}

// WithOrigin returns a child paint context with a new absolute origin.
func (pc *PaintContext) WithOrigin(origin Point) *PaintContext {
	if pc == nil {
		return &PaintContext{Origin: origin, Scale: 1}
	}
	out := *pc
	out.Origin = origin
	return &out
}

// WithForceFullPaint returns a paint context that paints all children (no skip).
func (pc *PaintContext) WithForceFullPaint() *PaintContext {
	if pc == nil {
		return &PaintContext{ForceFullPaint: true, Scale: 1}
	}
	out := *pc
	out.ForceFullPaint = true
	out.CompositeOnly = false
	return &out
}

// WithClip returns a paint context with an updated advisory clip.
func (pc *PaintContext) WithClip(clip Rect) *PaintContext {
	if pc == nil {
		return &PaintContext{Clip: clip, Scale: 1}
	}
	out := *pc
	if !pc.Clip.Empty() {
		out.Clip = pc.Clip.Intersect(clip)
	} else {
		out.Clip = clip
	}
	return &out
}

// FillRect draws an axis-aligned filled rectangle at absolute logical coords.
func (pc *PaintContext) FillRect(r Rect, col render.RGBA) {
	if pc == nil || pc.DC == nil || r.Empty() {
		return
	}
	pc.DC.SetRGBA(col.R, col.G, col.B, col.A)
	pc.DC.DrawRectangle(r.Min.X, r.Min.Y, r.Width(), r.Height())
	_ = pc.DC.Fill()
}

// FillLocalRect fills a rect relative to Origin.
func (pc *PaintContext) FillLocalRect(x, y, w, h float64, col render.RGBA) {
	if pc == nil {
		return
	}
	pc.FillRect(NewRect(pc.Origin.X+x, pc.Origin.Y+y, w, h), col)
}

// clampCornerRadius clamps a corner radius to the geometric maximum for the rect.
func clampCornerRadius(w, h, radius float64) float64 {
	if radius <= 0 || w <= 0 || h <= 0 {
		return 0
	}
	maxR := w
	if h < maxR {
		maxR = h
	}
	maxR *= 0.5
	if radius > maxR {
		return maxR
	}
	return radius
}

// PushClipLocal clips to a local rect (relative to Origin) via render.Context
// and updates the advisory Clip for subtree paint culling.
// Caller must Pop after painting children.
func (pc *PaintContext) PushClipLocal(x, y, w, h float64) {
	if pc == nil {
		return
	}
	if pc.DC != nil {
		pc.DC.Push()
		pc.DC.ClipRect(pc.Origin.X+x, pc.Origin.Y+y, w, h)
	}
	// Advisory cull stack (fixed depth — no heap on the scroll hot path).
	if pc.clipDepth < len(pc.prevClips) {
		pc.prevClips[pc.clipDepth] = pc.Clip
		pc.clipDepth++
		r := NewRect(pc.Origin.X+x, pc.Origin.Y+y, w, h)
		if pc.Clip.Empty() {
			pc.Clip = r
		} else {
			pc.Clip = pc.Clip.Intersect(r)
		}
	}
}

// Pop restores render state after PushClipLocal.
func (pc *PaintContext) Pop() {
	if pc == nil {
		return
	}
	if pc.DC != nil {
		pc.DC.Pop()
	}
	if pc.clipDepth > 0 {
		pc.clipDepth--
		pc.Clip = pc.prevClips[pc.clipDepth]
	}
}
