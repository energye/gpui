//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package rendering

import (
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/ui/scene"
)

// LayerCache holds cross-frame retained layer/display-list objects for the
// incremental layer build (Flutter RepaintBoundary semantics): a clean
// boundary reuses its previous BoundaryLayer object (children re-walked) and
// its recording is not re-recorded — only dirty paths rebuild. Without
// this, BuildLayerTree allocated the whole layer tree + recorded every
// display list every frame.
//
// The per-layer recording reuse (pics) is wired into the retained packet
// build (O11): addOwnPicture/addLeafPicture consult ReuseRecording for clean
// nodes and StoreRecording after a fresh record, so an unchanged layer's
// display list is reused by pointer instead of re-executed every frame.
// Invalidation contract (O11): the caller gates on !NeedsPaint — the same
// trust the raster texture admission already exercises
// (scene/textured.go: changed := dirtyID || NeedsRaster) — plus a
// defense-in-depth content fingerprint and size match carried per entry;
// a fingerprint mismatch (content mutated without NeedsPaint) re-records and
// re-stores, so the next build is stable again. Recordings are immutable
// after FinishRecording, so sharing one pointer across frames/threads is the
// COW/pointer-reuse contract scene.Layers already declare (scene/layer.go).
//
// Liveness is bound to the render tree: Prune drops entries whose owner is no
// longer reachable (scrolled-out VirtualList cells release their caches).
type LayerCache struct {
	// subs maps boundary cacheID → the BoundaryLayer object to reuse.
	// (Layer-tree retention skeleton — step-2 scope, not wired into the
	// build yet; the build still allocates fresh layer structs per frame.)
	subs map[uint64]scene.Layer
	// pics maps layer cacheKey → the recorded display list (with the gate
	// meta captured at store time) to reuse.
	pics map[uint64]*layerPicEntry
	// alive marks entries touched this build; Prune drops the rest.
	aliveSub map[uint64]struct{}
	alivePic map[uint64]struct{}
	// ReusedSubs / ReusedPics count cache hits per frame (diagnostics: a
	// clean frame should reuse everything — the Flutter _needsAddToScene
	// contract that removes per-frame allocation).
	ReusedSubs int
	ReusedPics int
	// reuseTotal / storeTotal are process-lifetime counters for window-side
	// gates (a steady retained frame must grow reuse, not store).
	reuseTotal int64
	storeTotal int64
}

// layerPicEntry is one cached per-layer display list plus the gate meta the
// reuse decision needs: the content fingerprint at store time (0 = no
// fingerprint for this node type — NeedsPaint is the sole signal, exactly the
// raster admission contract) and the recording pixel size.
type layerPicEntry struct {
	rec  *recording.Recording
	fp   uint64
	w, h int
}

// NewLayerCache creates an empty incremental-build cache.
func NewLayerCache() *LayerCache {
	return &LayerCache{
		subs:     make(map[uint64]scene.Layer),
		pics:     make(map[uint64]*layerPicEntry),
		aliveSub: make(map[uint64]struct{}),
		alivePic: make(map[uint64]struct{}),
	}
}

// BeginFrame resets the per-frame alive sets and reuse counters.
func (c *LayerCache) BeginFrame() {
	if c == nil {
		return
	}
	c.aliveSub = make(map[uint64]struct{})
	c.alivePic = make(map[uint64]struct{})
	c.ReusedSubs = 0
	c.ReusedPics = 0
}

// ReuseSub returns a cached BoundaryLayer for cacheID (or nil) and marks it
// alive.
func (c *LayerCache) ReuseSub(cacheID uint64) scene.Layer {
	if c == nil || cacheID == 0 {
		return nil
	}
	l, ok := c.subs[cacheID]
	if !ok || l == nil {
		return nil
	}
	c.aliveSub[cacheID] = struct{}{}
	c.ReusedSubs++
	return l
}

// StoreSub caches the BoundaryLayer for cacheID.
func (c *LayerCache) StoreSub(cacheID uint64, l scene.Layer) {
	if c == nil || cacheID == 0 || l == nil {
		return
	}
	c.subs[cacheID] = l
	c.aliveSub[cacheID] = struct{}{}
}

// ReuseRecording returns a cached display list for cacheKey whose stored size
// and fingerprint match the current ones (or nil), and marks it alive.
//
// Gates: size mismatch re-records (layout changed); a fingerprint mismatch
// re-records when both sides carry one (content mutated without NeedsPaint —
// the stale safety net); fp==0 on either side skips the fingerprint gate
// (node types without a fingerprint rely on NeedsPaint alone, mirroring the
// raster texture admission contract).
func (c *LayerCache) ReuseRecording(cacheKey, fp uint64, w, h int) *recording.Recording {
	if c == nil || cacheKey == 0 {
		return nil
	}
	e := c.pics[cacheKey]
	if e == nil || e.rec == nil {
		return nil
	}
	if e.w != w || e.h != h {
		return nil
	}
	if e.fp != 0 && fp != 0 && e.fp != fp {
		return nil
	}
	c.alivePic[cacheKey] = struct{}{}
	c.ReusedPics++
	c.reuseTotal++
	return e.rec
}

// StoreRecording caches the display list for cacheKey together with the
// fingerprint/size meta the next build's reuse gate needs.
func (c *LayerCache) StoreRecording(cacheKey, fp uint64, rec *recording.Recording, w, h int) {
	if c == nil || cacheKey == 0 || rec == nil {
		return
	}
	c.pics[cacheKey] = &layerPicEntry{rec: rec, fp: fp, w: w, h: h}
	c.alivePic[cacheKey] = struct{}{}
	c.storeTotal++
}

// Totals reports the process-lifetime reuse/store counts (window-side gates
// read these via PipelineApp.LayerCache; a steady retained frame grows reuse
// while store stays flat).
func (c *LayerCache) Totals() (reuse, store int64) {
	if c == nil {
		return 0, 0
	}
	return c.reuseTotal, c.storeTotal
}

// Prune drops entries not touched this build (owner no longer reachable from
// the tree — unmounted cells release their layer + display list).
func (c *LayerCache) Prune() {
	if c == nil {
		return
	}
	for id := range c.subs {
		if _, ok := c.aliveSub[id]; !ok {
			delete(c.subs, id)
		}
	}
	for k := range c.pics {
		if _, ok := c.alivePic[k]; !ok {
			delete(c.pics, k)
		}
	}
}
