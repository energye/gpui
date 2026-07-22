// Package layer holds retained GPU surfaces for Flutter-style composition.
//
// G2 contract (ENGINE_GAPS):
//
//	Vector Fill/Stroke frames → LoadOpClear (cannot retain undamaged surface pixels).
//	Blit-only frames (only DrawGPUTexture*) → LoadOpLoad + scissor (true partial update).
//
// Therefore a correct retained UI path is:
//
//  1. Rasterize chrome + each RepaintBoundary into offscreen RTs (vector allowed).
//  2. Present to the swapchain with ONLY DrawGPUTexture blits (G2.b).
//
// Compositor implements that split. Cache stores per-boundary (and base) textures.
package layer

import (
	"sync"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// Entry is one cached offscreen surface.
type Entry struct {
	DC      *render.Context
	View    gpucontext.TextureView
	release func()
	W, H    int     // logical
	PW, PH  int     // physical
	X, Y    float64 // last blit origin (logical)
	Valid   bool
}

// Matches reports whether the entry can be reused for the given logical size.
func (e *Entry) Matches(w, h int) bool {
	return e != nil && e.Valid && e.W == w && e.H == h && !e.View.IsNil()
}

// Release frees GPU and CPU resources.
func (e *Entry) Release() {
	if e == nil {
		return
	}
	e.Valid = false
	if e.release != nil {
		e.release()
		e.release = nil
	}
	e.View = gpucontext.TextureView{}
	if e.DC != nil {
		_ = e.DC.Close()
		e.DC = nil
	}
	e.W, e.H, e.PW, e.PH = 0, 0, 0, 0
}

// Cache maps boundary keys to Entries. Implements core.LayerCache.
type Cache struct {
	mu sync.Mutex
	m  map[uintptr]*Entry
}

// NewCache creates an empty layer cache.
func NewCache() *Cache {
	return &Cache{m: make(map[uintptr]*Entry)}
}

var _ core.LayerCache = (*Cache)(nil)

// Get returns the entry for key, or nil.
func (c *Cache) Get(key uintptr) *Entry {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.m[key]
}

// Ensure returns a usable entry of the given logical size, recreating if needed.
func (c *Cache) Ensure(key uintptr, w, h int, scale float64) *Entry {
	if c == nil || w < 1 || h < 1 {
		return nil
	}
	if scale <= 0 {
		scale = 1
	}
	pw := int(float64(w)*scale + 0.5)
	ph := int(float64(h)*scale + 0.5)
	if pw < 1 {
		pw = 1
	}
	if ph < 1 {
		ph = 1
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[uintptr]*Entry)
	}
	e := c.m[key]
	if e != nil && e.W == w && e.H == h && e.PW == pw && e.PH == ph && e.DC != nil {
		return e
	}
	if e != nil {
		e.Release()
	}
	dc := render.NewContext(w, h, render.WithDeviceScale(scale))
	e = &Entry{DC: dc, W: w, H: h, PW: pw, PH: ph}
	c.m[key] = e
	return e
}

// Rasterize flushes e.DC into a GPU offscreen texture.
func (e *Entry) Rasterize() bool {
	if e == nil || e.DC == nil {
		return false
	}
	if e.View.IsNil() || e.release == nil {
		if e.release != nil {
			e.release()
			e.release = nil
		}
		view, rel := e.DC.CreateOffscreenTexture(e.PW, e.PH)
		if rel == nil || view.IsNil() {
			e.View = gpucontext.TextureView{}
			e.Valid = false
			return false
		}
		e.View = view
		e.release = rel
	}
	if err := e.DC.FlushGPUWithView(e.View, uint32(e.PW), uint32(e.PH)); err != nil { //nolint:gosec
		e.Valid = false
		return false
	}
	e.Valid = true
	return true
}

// Blit draws the cached texture into parent at logical (x,y).
func (e *Entry) Blit(parent *render.Context, x, y float64, w, h int) {
	if e == nil || parent == nil || !e.Valid || e.View.IsNil() || w < 1 || h < 1 {
		return
	}
	parent.DrawGPUTexture(e.View, x, y, w, h)
}

// BlitBoundary implements core.LayerCache.
// When parent is nil, only updates stored origin (for compositor) if the entry is valid.
func (c *Cache) BlitBoundary(key uintptr, parent *render.Context, x, y float64, w, h int) bool {
	e := c.Get(key)
	if e == nil || !e.Matches(w, h) {
		return false
	}
	e.X, e.Y = x, y
	if parent != nil {
		e.Blit(parent, x, y, w, h)
	}
	return true
}

// RasterizeBoundary implements core.LayerCache: update offscreen cache only (no parent blit).
// Parent blit is done by BlitBoundary or Compositor.BlitLayers so the swapchain frame
// can stay blit-only (G2.b).
func (c *Cache) RasterizeBoundary(
	key uintptr,
	parent *render.Context,
	x, y float64,
	w, h int,
	scale float64,
	paintFn func(pc *core.PaintContext),
) bool {
	if c == nil || paintFn == nil || w < 1 || h < 1 {
		return false
	}
	_ = parent // parent is not drawn into here
	e := c.Ensure(key, w, h, scale)
	if e == nil || e.DC == nil {
		return false
	}
	e.X, e.Y = x, y
	e.DC.BeginFrame()
	e.DC.Clear() // transparent
	childPC := &core.PaintContext{
		DC:             e.DC,
		Origin:         core.Point{},
		Scale:          scale,
		ForceFullPaint: true,
		CompositeOnly:  false,
	}
	paintFn(childPC)
	return e.Rasterize()
}

// InvalidateBoundary implements core.LayerCache.
func (c *Cache) InvalidateBoundary(key uintptr) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.m[key]; e != nil {
		e.Valid = false
	}
}

// Len reports how many entries are tracked.
func (c *Cache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

// ReleaseAll implements core.LayerCache.
func (c *Cache) ReleaseAll() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.m {
		e.Release()
		delete(c.m, k)
	}
}

// ForEachValid walks valid entries (for compositor blit of all boundaries).
func (c *Cache) ForEachValid(fn func(key uintptr, e *Entry)) {
	if c == nil || fn == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.m {
		if e != nil && e.Valid && !e.View.IsNil() {
			fn(k, e)
		}
	}
}
