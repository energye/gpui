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
// Compositor implements that split. Cache stores per-boundary textures and
// tracks which keys were visited this frame so unmounted controls do not ghost.
package layer

import (
	"image"
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
//
// Live-set protocol (per composition frame):
//
//	BeginFrame → paint (BlitBoundary/RasterizeBoundary markLive) → CompositeLive
//
// Keys not marked live are unmounted ghosts and are released.
type Cache struct {
	mu          sync.Mutex
	m           map[uintptr]*Entry
	live        map[uintptr]struct{} // reused; cleared in BeginFrame
	liveOrder   []uintptr            // paint order within current band (deprecated for dual: use main/overlay)
	liveMain    []uintptr            // Main band paint order
	liveOverlay []uintptr            // Overlay band paint order (blit after main)
	rasterized  []uintptr            // keys re-rasterized this frame
	paintBand   core.LayerBand       // current band for markLive (set by compositor)
}

// NewCache creates an empty layer cache.
func NewCache() *Cache {
	return &Cache{
		m:    make(map[uintptr]*Entry),
		live: make(map[uintptr]struct{}),
	}
}

var _ core.LayerCache = (*Cache)(nil)

// BeginFrame clears the live set for a new composition frame (reuses map storage).
func (c *Cache) BeginFrame() {
	if c == nil {
		return
	}
	c.mu.Lock()
	clearLive(c)
	c.mu.Unlock()
}

func clearLive(c *Cache) {
	if c.live == nil {
		c.live = make(map[uintptr]struct{})
	} else {
		for k := range c.live {
			delete(c.live, k)
		}
	}
	c.liveOrder = c.liveOrder[:0]
	c.liveMain = c.liveMain[:0]
	c.liveOverlay = c.liveOverlay[:0]
	c.rasterized = c.rasterized[:0]
	c.paintBand = core.LayerBandMain
}

// SetPaintBand selects which composite band subsequent markLive calls join.
// Call before Tree.PaintMain (Main) and Tree.PaintOverlays (Overlay).
func (c *Cache) SetPaintBand(band core.LayerBand) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.paintBand = band
	c.mu.Unlock()
}

// markLive requires c.mu held. Preserves first-seen paint order within band.
func (c *Cache) markLive(key uintptr) {
	if c.live == nil {
		c.live = make(map[uintptr]struct{})
	}
	if _, ok := c.live[key]; ok {
		return
	}
	c.live[key] = struct{}{}
	c.liveOrder = append(c.liveOrder, key)
	if c.paintBand == core.LayerBandOverlay {
		c.liveOverlay = append(c.liveOverlay, key)
	} else {
		c.liveMain = append(c.liveMain, key)
	}
}

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
// When parent is nil, only updates stored origin if the entry is valid.
func (c *Cache) BlitBoundary(key uintptr, parent *render.Context, x, y float64, w, h int) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	e := c.m[key]
	if e == nil || !e.Matches(w, h) {
		c.mu.Unlock()
		return false
	}
	e.X, e.Y = x, y
	c.markLive(key)
	c.mu.Unlock()
	if parent != nil {
		e.Blit(parent, x, y, w, h)
	}
	return true
}

// RasterizeBoundary implements core.LayerCache: update offscreen cache only.
// Parent blit is deferred to CompositeLive so the swapchain frame stays blit-only (G2.b).
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
	_ = parent
	e := c.Ensure(key, w, h, scale)
	if e == nil || e.DC == nil {
		return false
	}
	e.X, e.Y = x, y
	e.DC.BeginFrame()
	e.DC.Clear() // transparent
	paintFn(&core.PaintContext{
		DC:             e.DC,
		Origin:         core.Point{},
		Scale:          scale,
		ForceFullPaint: true,
	})
	if !e.Rasterize() {
		return false
	}
	c.mu.Lock()
	c.markLive(key)
	c.rasterized = append(c.rasterized, key)
	c.mu.Unlock()
	return true
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

// DirtyLayerRects returns logical rects of layers re-rasterized this frame.
// Used for partial swapchain present (single Spin should not full-blit the window).
func (c *Cache) DirtyLayerRects() []image.Rectangle {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.rasterized) == 0 {
		return nil
	}
	out := make([]image.Rectangle, 0, len(c.rasterized))
	seen := make(map[uintptr]struct{}, len(c.rasterized))
	for _, k := range c.rasterized {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		e := c.m[k]
		if e == nil || !e.Valid || e.W < 1 || e.H < 1 {
			continue
		}
		x0 := int(e.X)
		y0 := int(e.Y)
		// Expand 1px for AA / subpixel origin.
		out = append(out, image.Rect(x0-1, y0-1, x0+e.W+1, y0+e.H+1))
	}
	return out
}

// CompositeDirtyOnly blits only layers re-rasterized this frame into parent
// (partial present path). Parent surface must use LoadOpLoad for undamaged pixels.
func (c *Cache) CompositeDirtyOnly(parent *render.Context) {
	if c == nil || parent == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	seen := make(map[uintptr]struct{}, len(c.rasterized))
	for _, k := range c.rasterized {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		e := c.m[k]
		if e == nil || !e.Valid || e.View.IsNil() {
			continue
		}
		e.Blit(parent, e.X, e.Y, e.W, e.H)
	}
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
	clearLive(c)
}

// CompositeLive blits Main-band layers then Overlay-band layers (paint order
// within each band), then releases unvisited entries. Modal masks in overlayBase
// are blitted by the compositor between these bands.
func (c *Cache) CompositeLive(parent *render.Context) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if parent != nil {
		c.blitOrderLocked(parent, c.liveMain)
		c.blitOrderLocked(parent, c.liveOverlay)
	}
	c.releaseUnvisitedLocked()
}

// CompositeBand blits only one band (Main or Overlay). Used when the host
// interleaves overlayBase between main and overlay layers.
func (c *Cache) CompositeBand(parent *render.Context, band core.LayerBand) {
	if c == nil || parent == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if band == core.LayerBandOverlay {
		c.blitOrderLocked(parent, c.liveOverlay)
	} else {
		c.blitOrderLocked(parent, c.liveMain)
	}
}

// ReleaseUnvisited drops ghost layers not marked live this frame.
// Call once after all CompositeBand passes for the frame.
func (c *Cache) ReleaseUnvisited() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.releaseUnvisitedLocked()
}

func (c *Cache) blitOrderLocked(parent *render.Context, order []uintptr) {
	for _, k := range order {
		e := c.m[k]
		if e == nil || !e.Valid || e.View.IsNil() {
			continue
		}
		e.Blit(parent, e.X, e.Y, e.W, e.H)
	}
}

func (c *Cache) releaseUnvisitedLocked() {
	for k, e := range c.m {
		if _, ok := c.live[k]; ok {
			continue
		}
		if e != nil {
			e.Release()
		}
		delete(c.m, k)
	}
}
