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

// FillLocalRoundRect fills a rounded rect relative to Origin.
func (pc *PaintContext) FillLocalRoundRect(x, y, w, h, radius float64, col render.RGBA) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 || col.A <= 0 {
		return
	}
	pc.DC.SetRGBA(col.R, col.G, col.B, col.A)
	if radius <= 0 {
		pc.DC.DrawRectangle(pc.Origin.X+x, pc.Origin.Y+y, w, h)
	} else {
		pc.DC.DrawRoundedRectangle(pc.Origin.X+x, pc.Origin.Y+y, w, h, radius)
	}
	_ = pc.DC.Fill()
}

// StrokeLocalRoundRect strokes a rounded rect relative to Origin.
func (pc *PaintContext) StrokeLocalRoundRect(x, y, w, h, radius, lineWidth float64, col render.RGBA) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 || col.A <= 0 {
		return
	}
	if lineWidth <= 0 {
		lineWidth = 1
	}
	pc.DC.SetRGBA(col.R, col.G, col.B, col.A)
	pc.DC.SetLineWidth(lineWidth)
	inset := lineWidth / 2
	if radius <= 0 {
		pc.DC.DrawRectangle(pc.Origin.X+x+inset, pc.Origin.Y+y+inset, w-lineWidth, h-lineWidth)
	} else {
		r := radius - inset
		if r < 0 {
			r = 0
		}
		pc.DC.DrawRoundedRectangle(pc.Origin.X+x+inset, pc.Origin.Y+y+inset, w-lineWidth, h-lineWidth, r)
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
