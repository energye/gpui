package core

import "github.com/energye/gpui/render"

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
	Clip Rect
	// CompositeOnly: retained frame — skip clean non-boundary subtrees; RepaintBoundary
	// nodes blit cached layers. Requires prior full frame + LoadOpLoad-capable present
	// (or accept holes if the surface was cleared). Hosts set this after the first
	// full paint when only boundary layers are dirty.
	CompositeOnly bool
	// ForceFullPaint disables CompositeOnly skip for this subtree (used when a node
	// itself is paint-dirty and must redraw non-boundary children).
	ForceFullPaint bool
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

// FillLocalRoundRect fills a rounded rect relative to Origin.
// Radius is clamped to min(w,h)/2 so corners stay circular and match StrokeLocalRoundRect.
func (pc *PaintContext) FillLocalRoundRect(x, y, w, h, radius float64, col render.RGBA) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 || col.A <= 0 {
		return
	}
	ax := pc.Origin.X + x
	ay := pc.Origin.Y + y
	r := clampCornerRadius(w, h, radius)
	pc.DC.SetRGBA(col.R, col.G, col.B, col.A)
	if r <= 0 {
		pc.DC.DrawRectangle(ax, ay, w, h)
	} else {
		pc.DC.DrawRoundedRectangle(ax, ay, w, h, r)
	}
	_ = pc.DC.Fill()
}

// StrokeLocalRoundRect strokes a rounded rect relative to Origin using a
// border-box model: the stroke is fully inside [x,y,w,h].
//
// The path is inset by lineWidth/2 and the path radius is reduced by the same
// amount, so the outer edge of the stroke coincides with the outer edge of a
// matching FillLocalRoundRect at the same (x,y,w,h,radius). This keeps fill
// and stroke radii aligned and avoids a "double border" from painting the
// stroke centered on the outer path.
//
// For 1px borders the path sits on half-pixel centers, which keeps AA crisp.
func (pc *PaintContext) StrokeLocalRoundRect(x, y, w, h, radius, lineWidth float64, col render.RGBA) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 || col.A <= 0 {
		return
	}
	if lineWidth <= 0 {
		lineWidth = 1
	}
	// Degenerate: stroke thicker than the box — fill the whole box instead of
	// producing an inverted/empty path (common for very small indicators).
	if lineWidth >= w || lineWidth >= h {
		pc.FillLocalRoundRect(x, y, w, h, radius, col)
		return
	}

	outerR := clampCornerRadius(w, h, radius)
	inset := lineWidth / 2
	iw := w - lineWidth
	ih := h - lineWidth
	// Outer stroke radius should equal outerR:
	//   pathRadius + inset = outerR  →  pathRadius = outerR - inset
	pathR := outerR - inset
	if pathR < 0 {
		pathR = 0
	}
	// Keep path radius within the inset rect (defense in depth).
	pathR = clampCornerRadius(iw, ih, pathR)

	ax := pc.Origin.X + x + inset
	ay := pc.Origin.Y + y + inset
	pc.DC.SetRGBA(col.R, col.G, col.B, col.A)
	pc.DC.SetLineWidth(lineWidth)
	// Rounded rects are already smooth; Round join avoids spikes if the path
	// degenerates under extreme radii / widths.
	pc.DC.SetLineJoin(render.LineJoinRound)

	if pathR <= 0 {
		pc.DC.DrawRectangle(ax, ay, iw, ih)
	} else {
		pc.DC.DrawRoundedRectangle(ax, ay, iw, ih, pathR)
	}
	_ = pc.DC.Stroke()
}

// PushClipLocal clips to a local rect (relative to Origin) via render.Context.
// Caller must Pop after painting children.
func (pc *PaintContext) PushClipLocal(x, y, w, h float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.DC.Push()
	pc.DC.ClipRect(pc.Origin.X+x, pc.Origin.Y+y, w, h)
}

// Pop restores render state after PushClipLocal.
func (pc *PaintContext) Pop() {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.DC.Pop()
}
