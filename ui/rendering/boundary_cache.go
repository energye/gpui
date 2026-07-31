package rendering

import (
	"sync/atomic"

	"github.com/energye/gpui/ui/scene"
)

// BoundaryCache holds Picture-backed caches for RepaintBoundary nodes (W1).
// Clean boundaries Replay the stored Picture (skip re-record); dirty ones
// re-paint and re-record. Works under FullPaint so skip is observable without
// Retained Present (ENGINE_UI_WIDGET_RENDER R3).
//
// Nested model (Flutter-like layers, Picture MVP):
//   - Each RepaintBoundary owns its Picture of **own content only**.
//   - Nested IsRepaintBoundary children are NOT baked into the parent Picture.
//   - After a container tryReplay, AbsoluteBox still walks nested RB children
//     so each child can tryReplay/rerecord independently.
//   - Therefore only-inner-dirty does not force outer rerecord, and clean
//     outer Replay cannot show stale child colors.
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

// Clear drops all Picture entries (resize / DPR change — R11).
func (c *BoundaryCache) Clear() {
	if c == nil {
		return
	}
	c.entries = make(map[uint64]*boundaryEntry)
}

// Len returns the number of cached boundary entries.
func (c *BoundaryCache) Len() int {
	if c == nil {
		return 0
	}
	return len(c.entries)
}

// Invalidate drops the Picture for this node (size/content change).
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

// tryReplay returns true if this boundary's **own** Picture was drawn from cache.
//
// Own content only: nested IsRepaintBoundary children are not in the Picture
// (see recordAbsoluteOwnContent). Callers of container boundaries must still
// paint nested RB children after a successful tryReplay (AbsoluteBox.Paint).
//
// Self NeedsPaint → miss. Descendant dirtiness does **not** block own Replay
// (outer can skip while inner rerecords).
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
	// Content fingerprint must match (content changed → invalidate).
	// Origin shift is allowed: scroll reuse (R7b) moves the cell without changing
	// its Picture content. We replay translated to the current origin instead of
	// invalidating. Static trees (R3/R3b) have zero shift → behavior unchanged.
	curKey := currentContentKey(n, sz.Width, sz.Height)
	if curKey != 0 && e.contentKey != 0 && curKey != e.contentKey {
		e.valid = false
		return false
	}
	if e.ox != pc.OriginX || e.oy != pc.OriginY {
		// Translate replay to current origin (scroll reuse / layout pan).
		// e.ox/e.oy stay frozen at record-time origin so each replay translates
		// by the FULL record→current delta (correct total displacement), not a
		// per-frame delta that would drift and misposition the cached Picture.
		pc.DC.Push()
		pc.DC.Translate(pc.OriginX-e.ox, pc.OriginY-e.oy)
		e.pic.Replay(pc.DC)
		pc.DC.Pop()
	} else {
		e.pic.Replay(pc.DC)
	}
	c.Skip++
	c.FrameSkip++
	pc.NotePaintVisit()
	return true
}

// currentContentKey fingerprints a boundary node's **current** own content
// (the same scope storeColorBox / storeAbsoluteColorChildren record). Returns 0
// when the node type has no MVP recorder (caller treats 0 as "no fingerprint
// available" and skips the content-match gate — origin/size already gate it).
func currentContentKey(n RenderObject, w, h float64) uint64 {
	if n == nil {
		return 0
	}
	switch t := n.(type) {
	case *RenderColorBox:
		return colorKey(t.R, t.G, t.B, t.A, w, h)
	case *AbsoluteBox:
		return absoluteOwnContentKey(t, w, h)
	default:
		return 0
	}
}

// absoluteOwnContentKey fingerprints an AbsoluteBox boundary's own content
// (background + non-RepaintBoundary descendants). Mirrors recordAbsoluteOwnContent
// selection so scroll reuse (R7b) detects content change without re-recording.
func absoluteOwnContentKey(a *AbsoluteBox, w, h float64) uint64 {
	if a == nil {
		return 0
	}
	var key uint64
	if a.Background != nil {
		bg := a.Background
		key = colorKey(bg.R, bg.G, bg.B, bg.A, w, h)
	}
	for _, ch := range a.children {
		if ch == nil || ch.IsRepaintBoundary() {
			continue
		}
		off := ch.Offset()
		switch t := ch.(type) {
		case *RenderColorBox:
			cw, chh := t.Size().Width, t.Size().Height
			if cw <= 0 {
				cw = t.Width
			}
			if chh <= 0 {
				chh = t.Height
			}
			key ^= colorKey(t.R, t.G, t.B, t.A, cw, chh) ^ uint64(off.X)<<3 ^ uint64(off.Y)<<13
		case *AbsoluteBox:
			// Non-RB nested AbsoluteBox: fold its own-content key in.
			key ^= absoluteOwnContentKey(t, t.Size().Width, t.Size().Height) ^ uint64(off.X)<<3 ^ uint64(off.Y)<<13
		default:
			// Other RO types not in MVP recorder: any such child forces a miss
			// by returning 0 (no fingerprint → caller skips content gate, origin
			// shift still allowed but content change not detected → conservative).
			return 0
		}
	}
	return key
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
	// No ancestor invalidate: parents do not bake nested RB children.
}

// storeAbsoluteColorChildren records an AbsoluteBox boundary Picture of **own
// content only** (bg + non-RepaintBoundary descendants). Nested RB children keep
// their own cache entries; parent skip does not freeze their pixels.
func (c *BoundaryCache) storeAbsoluteColorChildren(pc *PaintContext, a *AbsoluteBox) {
	if c == nil || pc == nil || a == nil || !pc.UseBoundaryCache || !a.IsRepaintBoundary() {
		return
	}
	b := &a.Base
	id := b.ensureCacheID()
	sz := a.Size()
	ox, oy := pc.OriginX, pc.OriginY
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		recordAbsoluteOwnContent(r, a, ox, oy)
	})
	// contentKey fingerprints own content (bg + non-RB descendants) so tryReplay
	// can detect scroll-shift cells whose content is unchanged (R7b scroll reuse).
	key := absoluteOwnContentKey(a, sz.Width, sz.Height)
	c.entries[id] = &boundaryEntry{
		pic: pic, ox: ox, oy: oy, w: sz.Width, h: sz.Height,
		contentKey: key, valid: pic.Valid && !pic.IsEmpty(),
	}
	c.Rerecord++
	c.FrameRerecord++
}

// recordAbsoluteOwnContent draws AbsoluteBox bg + non-RepaintBoundary children
// into a PictureRecorder. IsRepaintBoundary children are omitted (own cache).
func recordAbsoluteOwnContent(r *scene.PictureRecorder, a *AbsoluteBox, ox, oy float64) {
	if r == nil || a == nil {
		return
	}
	sz := a.Size()
	if a.Background != nil {
		bg := a.Background
		r.FillRect(ox, oy, sz.Width, sz.Height, bg.R, bg.G, bg.B, bg.A)
	}
	for _, ch := range a.children {
		if ch == nil || ch.IsRepaintBoundary() {
			continue
		}
		off := ch.Offset()
		ax, ay := ox+off.X, oy+off.Y
		switch t := ch.(type) {
		case *RenderColorBox:
			cw, chh := t.Size().Width, t.Size().Height
			if cw <= 0 {
				cw = t.Width
			}
			if chh <= 0 {
				chh = t.Height
			}
			r.FillRect(ax, ay, cw, chh, t.R, t.G, t.B, t.A)
		case *RenderText:
			// Bake label text into own-content Picture (DrawString baseline at ay+FontSize).
			// Without this case, labels fall through to default → skipped → not drawn
			// (R7b cells had invisible labels). face may be nil (replay needs SetFont on dc).
			r.DrawString(t.Text, ax, ay+t.FontSize, t.Face, t.R, t.G, t.B, t.A)
		case *AbsoluteBox:
			// Non-RB AbsoluteBox: bake its own content; still skip its RB kids.
			recordAbsoluteOwnContent(r, t, ax, ay)
		default:
			// Other RO types not in AbsoluteBox Picture MVP.
		}
	}
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
