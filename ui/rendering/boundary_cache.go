package rendering

import (
	"sync/atomic"

	"github.com/energye/gpui/ui/scene"
)

// BoundaryCache holds Picture-backed caches for RepaintBoundary nodes (W1).
// Clean boundaries Replay the stored Picture (skip re-record); dirty ones
// re-paint and re-record. Works under FullPaint so skip is observable without
// Retained Present (ENGINE_UI_WIDGET_RENDER R3).
type BoundaryCache struct {
	entries map[uint64]*boundaryEntry
	// Cumulative counters (process lifetime of this cache).
	Rerecord int64
	Skip     int64
	// Per root paint-walk frame counters (reset by BeginFrame).
	FrameRerecord int64
	FrameSkip     int64
}

type boundaryEntry struct {
	pic        scene.Picture
	ox, oy     float64
	w, h       float64
	contentKey uint64
	valid      bool
}

var nextBoundaryCacheID uint64

// NewBoundaryCache creates an empty cache.
func NewBoundaryCache() *BoundaryCache {
	return &BoundaryCache{entries: make(map[uint64]*boundaryEntry)}
}

// BeginFrame resets per-frame skip/rerecord counters (call once per present paint).
func (c *BoundaryCache) BeginFrame() {
	if c == nil {
		return
	}
	c.FrameRerecord = 0
	c.FrameSkip = 0
}

// ensureID assigns a stable cache id on Base.
func (b *Base) ensureCacheID() uint64 {
	if b == nil {
		return 0
	}
	if b.cacheID == 0 {
		b.cacheID = atomic.AddUint64(&nextBoundaryCacheID, 1)
	}
	return b.cacheID
}

// CacheID returns the boundary cache identity (0 if never assigned).
func (b *Base) CacheID() uint64 {
	if b == nil {
		return 0
	}
	return b.cacheID
}

// InvalidateBoundaryCache drops the Picture for this node (size/content change).
func (c *BoundaryCache) Invalidate(n RenderObject) {
	if c == nil || n == nil {
		return
	}
	b, ok := baseOf(n)
	if !ok || b.cacheID == 0 {
		return
	}
	delete(c.entries, b.cacheID)
}

// HasValid reports whether n currently has a reusable Picture entry.
func (c *BoundaryCache) HasValid(n RenderObject) bool {
	if c == nil || n == nil {
		return false
	}
	b, ok := baseOf(n)
	if !ok || b.cacheID == 0 {
		return false
	}
	e := c.entries[b.cacheID]
	return e != nil && e.valid && e.pic.Valid && !e.pic.IsEmpty()
}

// tryReplay returns true if a clean repaint boundary was drawn from cache.
//
// Leaf boundaries (no children): skip when !NeedsPaint and entry valid.
// Container boundaries: full-picture Replay only when the entire subtree is clean
// (!NeedsPaint && !SubtreeNeedsPaint). When only an inner descendant is dirty,
// returns false so the container can walk children without counting as a skip;
// callers must not re-store the outer Picture in that case (see AbsoluteBox.Paint).
func (c *BoundaryCache) tryReplay(pc *PaintContext, n RenderObject) bool {
	if c == nil || pc == nil || pc.DC == nil || n == nil || !pc.UseBoundaryCache {
		return false
	}
	if !n.IsRepaintBoundary() {
		return false
	}
	if n.NeedsPaint() {
		return false
	}
	// Container: cannot full-replay while a descendant still needs paint.
	if len(n.Children()) > 0 && SubtreeNeedsPaint(n) {
		return false
	}
	b, ok := baseOf(n)
	if !ok {
		return false
	}
	id := b.ensureCacheID()
	e := c.entries[id]
	if e == nil || !e.valid || !e.pic.Valid || e.pic.IsEmpty() {
		return false
	}
	sz := n.Size()
	if e.w != sz.Width || e.h != sz.Height {
		e.valid = false
		return false
	}
	// Origin must match record-time absolute origin (FullPaint tree is stable).
	if abs64(e.ox-pc.OriginX) > 0.01 || abs64(e.oy-pc.OriginY) > 0.01 {
		e.valid = false
		return false
	}
	e.pic.Replay(pc.DC)
	c.Skip++
	c.FrameSkip++
	pc.NotePaintVisit()
	return true
}

// storeColorBox records a solid color boundary Picture after a live paint.
func (c *BoundaryCache) storeColorBox(pc *PaintContext, box *RenderColorBox) {
	if c == nil || pc == nil || box == nil || !pc.UseBoundaryCache || !box.IsRepaintBoundary() {
		return
	}
	b := &box.Base
	id := b.ensureCacheID()
	sz := box.Size()
	if sz.Width <= 0 {
		sz.Width = box.Width
	}
	if sz.Height <= 0 {
		sz.Height = box.Height
	}
	ox, oy := pc.OriginX, pc.OriginY
	key := colorKey(box.R, box.G, box.B, box.A, sz.Width, sz.Height)
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRect(ox, oy, sz.Width, sz.Height, box.R, box.G, box.B, box.A)
	})
	c.entries[id] = &boundaryEntry{
		pic: pic, ox: ox, oy: oy, w: sz.Width, h: sz.Height,
		contentKey: key, valid: pic.Valid && !pic.IsEmpty(),
	}
	c.Rerecord++
	c.FrameRerecord++
}

// storeAbsoluteColorChildren records bg + ColorBox children for an AbsoluteBox boundary.
func (c *BoundaryCache) storeAbsoluteColorChildren(pc *PaintContext, a *AbsoluteBox) {
	if c == nil || pc == nil || a == nil || !pc.UseBoundaryCache || !a.IsRepaintBoundary() {
		return
	}
	b := &a.Base
	id := b.ensureCacheID()
	sz := a.Size()
	ox, oy := pc.OriginX, pc.OriginY
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		if a.Background != nil {
			bg := a.Background
			r.FillRect(ox, oy, sz.Width, sz.Height, bg.R, bg.G, bg.B, bg.A)
		}
		for _, ch := range a.children {
			cb, ok := ch.(*RenderColorBox)
			if !ok {
				continue
			}
			off := ch.Offset()
			cw, chh := cb.Size().Width, cb.Size().Height
			if cw <= 0 {
				cw = cb.Width
			}
			if chh <= 0 {
				chh = cb.Height
			}
			r.FillRect(ox+off.X, oy+off.Y, cw, chh, cb.R, cb.G, cb.B, cb.A)
		}
	})
	c.entries[id] = &boundaryEntry{
		pic: pic, ox: ox, oy: oy, w: sz.Width, h: sz.Height,
		valid: pic.Valid && !pic.IsEmpty(),
	}
	c.Rerecord++
	c.FrameRerecord++
}

func colorKey(r, g, b, a, w, h float64) uint64 {
	// Coarse float fingerprint for invalidation (not cryptographic).
	return uint64(r*1000) ^ uint64(g*1000)<<10 ^ uint64(b*1000)<<20 ^
		uint64(a*1000)<<30 ^ uint64(w)<<40 ^ uint64(h)<<50
}

func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// CountRepaintBoundaries walks the tree and returns boundary count and max nesting depth.
// Used for R3b compositing-bits / boundary discovery gates.
func CountRepaintBoundaries(root RenderObject) (count int, maxDepth int) {
	var walk func(n RenderObject, depth int)
	walk = func(n RenderObject, depth int) {
		if n == nil {
			return
		}
		d := depth
		if n.IsRepaintBoundary() {
			count++
			d++
			if d > maxDepth {
				maxDepth = d
			}
		}
		for _, ch := range n.Children() {
			walk(ch, d)
		}
	}
	walk(root, 0)
	return count, maxDepth
}
