package rendering

import (
	"github.com/energye/gpui/ui/scene"
)

// LayerCache holds cross-frame retained layer/Picture objects for the
// incremental layer build (Flutter RepaintBoundary semantics): a clean
// boundary reuses its previous BoundaryLayer object (children re-walked) and
// its display list is not re-recorded — only dirty paths rebuild. Without
// this, BuildLayerTree allocated the whole layer tree + recorded every
// Picture every frame (R7: ~4-5MB/s allocation, RSS climbs under GOGC).
//
// Liveness is bound to the render tree: Prune drops entries whose owner is no
// longer reachable (scrolled-out VirtualList cells release their caches).
type LayerCache struct {
	// subs maps boundary cacheID → the BoundaryLayer object to reuse.
	subs map[uint64]scene.Layer
	// pics maps leaf cacheKey → the recorded display list (Picture) to reuse.
	pics map[uint64]*scene.Picture
	// alive marks entries touched this build; Prune drops the rest.
	aliveSub map[uint64]struct{}
	alivePic map[uint64]struct{}
	// ReusedSubs / ReusedPics count cache hits per frame (diagnostics: a
	// clean frame should reuse everything — the Flutter _needsAddToScene
	// contract that removes per-frame allocation).
	ReusedSubs int
	ReusedPics int
}

// NewLayerCache creates an empty incremental-build cache.
func NewLayerCache() *LayerCache {
	return &LayerCache{
		subs:     make(map[uint64]scene.Layer),
		pics:     make(map[uint64]*scene.Picture),
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

// ReusePic returns a cached display list for cacheKey (or nil) and marks it
// alive.
func (c *LayerCache) ReusePic(cacheKey uint64) *scene.Picture {
	if c == nil || cacheKey == 0 {
		return nil
	}
	p, ok := c.pics[cacheKey]
	if !ok || p == nil {
		return nil
	}
	c.alivePic[cacheKey] = struct{}{}
	c.ReusedPics++
	return p
}

// StorePic caches the display list for cacheKey.
func (c *LayerCache) StorePic(cacheKey uint64, p *scene.Picture) {
	if c == nil || cacheKey == 0 || p == nil {
		return
	}
	c.pics[cacheKey] = p
	c.alivePic[cacheKey] = struct{}{}
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
