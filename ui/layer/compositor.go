package layer

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// BaseKey is reserved if a future split-chrome path stores base in Cache.
const BaseKey uintptr = 0

// Compositor retains a full-window base RT plus per-RepaintBoundary layers.
//
//	full=true  — rebuild base; boundaries land in LayerCache (DeferLayerBlit)
//	full=false — keep base; only dirty boundaries re-rasterize
//
// Present is always blit(base) + CompositeLive(layers). A single Spin/Skeleton
// therefore re-rasterizes only its small RT, not the whole window tree.
type Compositor struct {
	base      *Entry
	cache     *Cache
	W, H      int
	Scale     float64
	BG        render.RGBA
	baseReady bool
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

// ReleaseAll drops base RT and boundary cache.
func (c *Compositor) ReleaseAll() {
	if c == nil {
		return
	}
	if c.base != nil {
		c.base.Release()
		c.base = nil
	}
	if c.cache != nil {
		c.cache.ReleaseAll()
	}
	c.baseReady = false
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
func (c *Compositor) paintCtx(base *Entry, theme *core.Theme, full bool) *core.PaintContext {
	return &core.PaintContext{
		DC:             base.DC,
		Scale:          c.Scale,
		Theme:          theme,
		CompositeOnly:  !full,
		ForceFullPaint: full,
		LayerCache:     c.ensureCache(),
		DeferLayerBlit: true, // host BlitTo composites layers
	}
}

// Frame updates retained surfaces.
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
	rebuildBase := full || !c.baseReady || !base.Valid

	cache.BeginFrame()
	if rebuildBase {
		base.DC.BeginFrame()
		fillBackground(base.DC, c.BG, c.W, c.H)
	}
	tree.Frame(c.paintCtx(base, theme, rebuildBase), viewport)
	if rebuildBase {
		if !base.Rasterize() {
			c.baseReady = false
			return false
		}
		c.baseReady = true
	}
	return true
}

// BlitTo draws base then live boundary layers (blit-only → G2.b).
func (c *Compositor) BlitTo(surface *render.Context) {
	if c == nil || surface == nil || c.base == nil || !c.base.Valid {
		return
	}
	c.base.Blit(surface, 0, 0, c.W, c.H)
	if c.cache != nil {
		c.cache.CompositeLive(surface)
	}
}

// HasBase reports whether a valid base texture is ready to blit.
func (c *Compositor) HasBase() bool {
	return c != nil && c.base != nil && c.base.Valid && !c.base.View.IsNil() && c.baseReady
}
