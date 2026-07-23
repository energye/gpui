package layer

import (
	"image"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// BaseKey is reserved if a future split-chrome path stores base in Cache.
const BaseKey uintptr = 0

// Compositor retains main + overlay bases plus per-RepaintBoundary layers.
//
//	full=true  — rebuild mainBase (and overlayBase when portals exist)
//	full=false — keep bases; only dirty boundaries re-rasterize
//
// Present Z-order (Flutter Overlay above app retained layers):
//
//	mainBase → mainLayers → overlayBase → overlayLayers
//
// Modal masks paint into overlayBase so Tabs ScrollViewport layers never sit above the dimmer.
type Compositor struct {
	base         *Entry
	overlayBase  *Entry
	cache        *Cache
	W, H         int
	Scale        float64
	BG           render.RGBA
	baseReady    bool
	overlayReady bool
}

// NewCompositor creates a compositor with a boundary layer cache.
func NewCompositor() *Compositor {
	return &Compositor{cache: NewCache()}
}

// LayerCache returns the boundary texture cache (never nil after NewCompositor).
func (c *Compositor) LayerCache() core.LayerCache {
	if c == nil {
		return nil
	}
	return c.ensureCache()
}

func (c *Compositor) ensureCache() *Cache {
	if c.cache == nil {
		c.cache = NewCache()
	}
	return c.cache
}

// Resize updates logical size; drops retained surfaces on size/scale change.
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

// ReleaseAll drops base RTs and boundary cache.
func (c *Compositor) ReleaseAll() {
	if c == nil {
		return
	}
	if c.base != nil {
		c.base.Release()
		c.base = nil
	}
	if c.overlayBase != nil {
		c.overlayBase.Release()
		c.overlayBase = nil
	}
	if c.cache != nil {
		c.cache.ReleaseAll()
	}
	c.baseReady = false
	c.overlayReady = false
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
	c.baseReady = false
	return c.base
}

func (c *Compositor) ensureOverlayBase() *Entry {
	if c == nil || c.W < 1 || c.H < 1 {
		return nil
	}
	if c.overlayBase != nil && c.overlayBase.DC != nil && c.overlayBase.W == c.W && c.overlayBase.H == c.H {
		return c.overlayBase
	}
	if c.overlayBase != nil {
		c.overlayBase.Release()
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
	c.overlayBase = &Entry{DC: dc, W: c.W, H: c.H, PW: pw, PH: ph}
	c.overlayReady = false
	return c.overlayBase
}

func (c *Compositor) releaseOverlayBase() {
	if c == nil || c.overlayBase == nil {
		return
	}
	c.overlayBase.Release()
	c.overlayBase = nil
	c.overlayReady = false
}

func fillBackground(dc *render.Context, bg render.RGBA, w, h int) {
	if dc == nil || w < 1 || h < 1 {
		return
	}
	if bg.A <= 0 {
		bg = render.RGBA{A: 1}
	}
	dc.SetRGBA(bg.R, bg.G, bg.B, bg.A)
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()
}

// paintCtx builds a tree paint context for retained composition.
func (c *Compositor) paintCtx(base *Entry, theme *core.Theme, full bool, band core.LayerBand) *core.PaintContext {
	return &core.PaintContext{
		DC:             base.DC,
		Scale:          c.Scale,
		Theme:          theme,
		CompositeOnly:  !full,
		ForceFullPaint: full,
		LayerCache:     c.ensureCache(),
		DeferLayerBlit: true, // host BlitTo composites layers by band
		LayerBand:      band,
	}
}

// Frame updates retained surfaces (main + optional overlay).
//
// Hosts pass full=true on first frame, resize, FullPaintRequired, or when
// Tree.NonBoundaryPaintDirty() (any paint dirty outside a RepaintBoundary).
func (c *Compositor) Frame(tree *core.Tree, theme *core.Theme, full bool) bool {
	if c == nil || tree == nil {
		return false
	}
	base := c.ensureBase()
	if base == nil || base.DC == nil {
		return false
	}
	if c.Scale <= 0 {
		c.Scale = 1
	}
	cache := c.ensureCache()
	viewport := core.Size{Width: float64(c.W), Height: float64(c.H)}
	rebuildMain := full || !c.baseReady || !base.Valid

	// Layout once (paint nil).
	tree.Frame(nil, viewport)

	cache.BeginFrame()

	// ---- Main band: root tree → mainBase + mainLayers ----
	cache.SetPaintBand(core.LayerBandMain)
	if rebuildMain {
		base.DC.BeginFrame()
		fillBackground(base.DC, c.BG, c.W, c.H)
	}
	tree.PaintMain(c.paintCtx(base, theme, rebuildMain, core.LayerBandMain))
	if rebuildMain {
		if !base.Rasterize() {
			c.baseReady = false
			return false
		}
		c.baseReady = true
	}

	// ---- Overlay band: portals → overlayBase + overlayLayers ----
	if !tree.HasOverlays() {
		c.releaseOverlayBase()
	} else {
		oBase := c.ensureOverlayBase()
		if oBase == nil || oBase.DC == nil {
			return false
		}
		rebuildOverlay := full || !c.overlayReady || !oBase.Valid || tree.OverlayNonBoundaryPaintDirty()
		cache.SetPaintBand(core.LayerBandOverlay)
		if rebuildOverlay {
			oBase.DC.BeginFrame()
			// Transparent clear — dimmer/panel drawn by overlay nodes.
			oBase.DC.Clear()
		}
		tree.PaintOverlays(c.paintCtx(oBase, theme, rebuildOverlay, core.LayerBandOverlay))
		if rebuildOverlay {
			if !oBase.Rasterize() {
				c.overlayReady = false
				return false
			}
			c.overlayReady = true
		}
	}

	tree.FinishPaint()
	return true
}

// BlitTo draws mainBase → mainLayers → overlayBase → overlayLayers (G2.b blit-only).
func (c *Compositor) BlitTo(surface *render.Context) {
	if c == nil || surface == nil || c.base == nil || !c.base.Valid {
		return
	}
	c.base.Blit(surface, 0, 0, c.W, c.H)
	if c.cache != nil {
		c.cache.CompositeBand(surface, core.LayerBandMain)
	}
	if c.overlayBase != nil && c.overlayBase.Valid && c.overlayReady {
		c.overlayBase.Blit(surface, 0, 0, c.W, c.H)
	}
	if c.cache != nil {
		c.cache.CompositeBand(surface, core.LayerBandOverlay)
		c.cache.ReleaseUnvisited()
	}
}

// HasBase reports whether a valid main base texture is ready to blit.
func (c *Compositor) HasBase() bool {
	return c != nil && c.base != nil && c.base.Valid && !c.base.View.IsNil() && c.baseReady
}

// DirtyLayerRects returns logical rects of boundary layers re-rasterized this frame.
func (c *Compositor) DirtyLayerRects() []image.Rectangle {
	if c == nil || c.cache == nil {
		return nil
	}
	return c.cache.DirtyLayerRects()
}

// BlitDirtyLayers blits only re-rasterized layers (experimental partial present).
// Prefer BlitTo for correct Z-order with overlays.
func (c *Compositor) BlitDirtyLayers(surface *render.Context) {
	if c == nil || surface == nil || c.cache == nil {
		return
	}
	c.cache.CompositeDirtyOnly(surface)
}
