// Package painting provides PaintingContext: logical-pixel drawing into render.Context.
package painting

import "github.com/energye/gpui/render"

// Context is the paint surface for a RenderObject tree.
// Coordinates are logical pixels, Y-down, origin top-left of the current node.
type Context struct {
	// DC is the underlying render context (may be nil for dry-run hit-only tests).
	DC *render.Context
	// Origin is the absolute logical offset of this paint context's (0,0).
	OriginX, OriginY float64
	// Scale is device pixel ratio (informational; DC should already use WithDeviceScale).
	Scale float64

	// CompositeOnly skips clean subtrees (Flutter retained paint). When true,
	// nodes with !NeedsPaint and no dirty descendants are not painted.
	CompositeOnly bool

	// PaintVisits increments once per RenderObject.Paint entry that proceeds
	// past the composite-only skip (for tests / metrics).
	PaintVisits *int64
}

// New creates a painting context rooted at (0,0).
func New(dc *render.Context, scale float64) *Context {
	if scale <= 0 {
		scale = 1
	}
	return &Context{DC: dc, Scale: scale}
}

// WithOrigin returns a child context with absolute origin (logical).
func (c *Context) WithOrigin(absX, absY float64) *Context {
	if c == nil {
		return &Context{OriginX: absX, OriginY: absY, Scale: 1}
	}
	return &Context{
		DC:            c.DC,
		OriginX:       absX,
		OriginY:       absY,
		Scale:         c.Scale,
		CompositeOnly: c.CompositeOnly,
		PaintVisits:   c.PaintVisits,
	}
}

// NotePaintVisit increments PaintVisits if non-nil.
func (c *Context) NotePaintVisit() {
	if c != nil && c.PaintVisits != nil {
		*c.PaintVisits++
	}
}

// FillRect fills a rectangle in local logical coordinates (Y-down).
//
// Uses the GPU draw path (DrawRectangle+Fill) so content appears in
// PresentFrame / swapchain presents. FillRectCPU only updates the CPU pixmap
// and is invisible on the true-window present path.
//
// For headless Image() tests, call dc.FlushGPU() (or Image()) after paint.
func (c *Context) FillRect(x, y, w, h, r, g, b, a float64) {
	if c == nil || c.DC == nil || w <= 0 || h <= 0 {
		return
	}
	ax := c.OriginX + x
	ay := c.OriginY + y
	c.DC.SetRGBA(r, g, b, a)
	c.DC.DrawRectangle(ax, ay, w, h)
	_ = c.DC.Fill()
}

// FillRectCPU writes directly to the CPU pixmap (tests / offscreen readback).
// Does not show up on GPU present unless something blits the pixmap to the surface.
func (c *Context) FillRectCPU(x, y, w, h, r, g, b, a float64) {
	if c == nil || c.DC == nil || w <= 0 || h <= 0 {
		return
	}
	ax := c.OriginX + x
	ay := c.OriginY + y
	c.DC.FillRectCPU(ax, ay, w, h, render.RGBA{R: r, G: g, B: b, A: a})
}
