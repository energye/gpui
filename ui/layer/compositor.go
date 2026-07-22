package layer

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// BaseKey is reserved if a future split-chrome path stores base in Cache.
// Currently base is held only on Compositor.base (not in Cache) so boundary
// keys never collide with chrome.
const BaseKey uintptr = 0

// Compositor retains a full-window base RT and presents with blit-only draws
// so the swapchain flush stays on the G2.b path (DrawGPUTexture only).
//
// Recommended default for UI hosts (exboot): often lower CPU than painting
// vectors straight into the surface every frame (GPUI_COMPOSITOR=0), because
// the surface pass is a single texture blit while vector work targets a stable
// offscreen RT.
//
// Strategy:
//
//	Each Frame paints the ENTIRE tree into base.DC with GPU vector ops.
//	BlitTo(surface) then DrawGPUTexture(base) only.
//	Partial boundary-only updates can layer on later; correctness first.
type Compositor struct {
	base  *Entry
	W, H  int
	Scale float64
	BG    render.RGBA
}

// NewCompositor creates a compositor.
func NewCompositor() *Compositor {
	return &Compositor{}
}

// LayerCache is nil — base path paints the full tree without per-boundary split.
// Kept for API compatibility with earlier experiments.
func (c *Compositor) LayerCache() core.LayerCache { return nil }

// Resize updates logical size; drops the base RT if size/scale change.
func (c *Compositor) Resize(w, h int, scale float64) {
	if c == nil {
		return
	}
	if scale <= 0 {
		scale = 1
	}
	if c.W == w && c.H == h && c.Scale == scale && c.base != nil && c.base.DC != nil {
		return
	}
	c.ReleaseAll()
	c.W, c.H, c.Scale = w, h, scale
}

// ReleaseAll drops the base RT.
func (c *Compositor) ReleaseAll() {
	if c == nil {
		return
	}
	if c.base != nil {
		c.base.Release()
		c.base = nil
	}
}

func (c *Compositor) ensureBase() *Entry {
	if c == nil || c.W < 1 || c.H < 1 {
		return nil
	}
	if c.base != nil && c.base.DC != nil && c.base.W == c.W && c.base.H == c.H {
		return c.base
	}
	if c.base != nil {
		c.base.Release()
	}
	if c.Scale <= 0 {
		c.Scale = 1
	}
	pw := int(float64(c.W)*c.Scale + 0.5)
	ph := int(float64(c.H)*c.Scale + 0.5)
	if pw < 1 {
		pw = 1
	}
	if ph < 1 {
		ph = 1
	}
	dc := render.NewContext(c.W, c.H, render.WithDeviceScale(c.Scale))
	c.base = &Entry{DC: dc, W: c.W, H: c.H, PW: pw, PH: ph}
	return c.base
}

// fillBackground records a GPU-visible full-rect clear (ClearWithColor alone is
// CPU pixmap only and does not appear on the offscreen GPU texture).
func fillBackground(dc *render.Context, bg render.RGBA, w, h int) {
	if dc == nil || w < 1 || h < 1 {
		return
	}
	dc.SetRGBA(bg.R, bg.G, bg.B, bg.A)
	if bg.A <= 0 {
		dc.SetRGBA(0, 0, 0, 1)
	}
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()
}

// Frame paints the full tree into the base offscreen RT and rasterizes to GPU.
// full is accepted for API stability; every present currently full-repaints base
// for correctness (partial base updates come after boundary split is re-enabled).
func (c *Compositor) Frame(tree *core.Tree, theme *core.Theme, full bool) bool {
	if c == nil || tree == nil {
		return false
	}
	_ = full
	base := c.ensureBase()
	if base == nil || base.DC == nil {
		return false
	}

	base.DC.BeginFrame()
	// GPU-visible background (not ClearWithColor — that stays on CPU pixmap only).
	fillBackground(base.DC, c.BG, c.W, c.H)

	// Full tree into base — no LayerCache, so every widget (static + animated)
	// is recorded into this RT. Matches direct present content, then blit.
	pc := &core.PaintContext{
		DC:             base.DC,
		Scale:          c.Scale,
		Theme:          theme,
		CompositeOnly:  false,
		ForceFullPaint: true,
		LayerCache:     nil,
		DeferLayerBlit: false,
	}
	tree.Frame(pc, core.Size{Width: float64(c.W), Height: float64(c.H)})

	if !base.Rasterize() {
		return false
	}
	return true
}

// BlitTo draws the base RT into surface with GPU blits only (no Fill/Stroke).
func (c *Compositor) BlitTo(surface *render.Context) {
	if c == nil || surface == nil || c.base == nil || !c.base.Valid {
		return
	}
	c.base.Blit(surface, 0, 0, c.W, c.H)
}

// HasBase reports whether a valid base texture is ready to blit.
func (c *Compositor) HasBase() bool {
	return c != nil && c.base != nil && c.base.Valid && !c.base.View.IsNil()
}
