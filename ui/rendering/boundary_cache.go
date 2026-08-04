package rendering

import (
	"reflect"
	"sync/atomic"

	"github.com/energye/gpui/ui/scene"
)

// BoundaryCache holds Picture-backed caches for RepaintBoundary nodes (R3).
// Clean boundaries Replay the stored Picture (skip re-record); dirty ones
// re-paint and re-record. Works under FullPaint so skip is observable without
// Retained Present (ENGINE_UI_WIDGET_RENDER R3).
//
// Nested model (Flutter layers, Picture MVP):
//   - Each RepaintBoundary owns a Picture of **own content only**.
//   - Nested IsRepaintBoundary children are NEVER baked into the parent Picture;
//     after a container tryReplay the caller still walks nested RB children so
//     each child can tryReplay/rerecord independently.
//   - Therefore only-inner-dirty never forces an outer rerecord, and a clean
//     outer Replay cannot show stale child colors (R3 内脏不外溢).
//
// Cacheability (correctness-first): a boundary whose own content contains RO
// types the MVP recorder cannot faithfully capture (e.g. Viewport) is marked
// not-cacheable — tryReplay/store are no-ops for it, so the subtree always
// live-paints and stale-frame content loss is impossible.
type BoundaryCache struct {
	entries map[uint64]*boundaryEntry
	// Cumulative counters (process lifetime of this cache).
	Rerecord int64
	Skip     int64
	// Per root paint-walk frame counters (reset by BeginFrame).
	FrameRerecord int64
	FrameSkip     int64
	// FrameMiss counts boundaries that wanted a Replay but had none (diagnostics).
	FrameMiss int64
	// Shell/content partitioning (W2 R21): boundaries tagged with
	// SetShellBoundary are counted separately, so a scrolling body can prove
	// the shell's Picture cache is never re-recorded (shell rerecord == 0).
	// Global counters above still include shell boundaries (backward compat).
	ShellRerecord      int64
	ShellSkip          int64
	FrameShellRerecord int64
	FrameShellSkip     int64
}

type boundaryEntry struct {
	pic        scene.Picture
	ox, oy     float64
	w, h       float64
	contentKey uint64
	valid      bool
	cacheable  bool
}

var nextBoundaryCacheID uint64

// NewBoundaryCache creates an empty cache.
func NewBoundaryCache() *BoundaryCache {
	return &BoundaryCache{entries: make(map[uint64]*boundaryEntry)}
}

// BeginFrame resets per-frame skip/rerecord/miss counters (call once per present paint).
func (c *BoundaryCache) BeginFrame() {
	if c == nil {
		return
	}
	c.FrameRerecord = 0
	c.FrameSkip = 0
	c.FrameMiss = 0
	c.FrameShellRerecord = 0
	c.FrameShellSkip = 0
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

// EnsureCacheID assigns a stable identity on first call and returns it.
// Any RenderObject may hold an identity (texture cache keys, debug tags);
// BoundaryCache keys are a subset of these ids.
func (b *Base) EnsureCacheID() uint64 {
	return b.ensureCacheID()
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
	return e != nil && e.valid && e.cacheable && e.pic.Valid && !e.pic.IsEmpty()
}

// tryReplay returns true if this boundary's **own** Picture was drawn from cache.
//
// Own content only: nested IsRepaintBoundary children are not in the Picture
// (see recordOwnContent). Callers of container boundaries must still paint
// nested RB children after a successful tryReplay (AbsoluteBox.Paint).
//
// Self NeedsPaint → miss. Descendant dirtiness does **not** block own Replay
// (outer can skip while inner rerecords). Non-cacheable boundaries always miss.
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
	if e == nil || !e.valid || !e.cacheable || !e.pic.Valid || e.pic.IsEmpty() {
		if e != nil {
			c.FrameMiss++
		}
		return false
	}
	sz := n.Size()
	if e.w != sz.Width || e.h != sz.Height {
		e.valid = false
		c.FrameMiss++
		return false
	}
	// Content fingerprint must match (content changed → invalidate).
	// Origin shift is allowed: scroll reuse (R7b) moves the cell without changing
	// its Picture content. We replay translated to the current origin instead of
	// invalidating. Static trees (R3/R3b) have zero shift → behavior unchanged.
	curKey := contentKeyOf(n, sz.Width, sz.Height)
	if curKey != 0 && e.contentKey != 0 && curKey != e.contentKey {
		e.valid = false
		c.FrameMiss++
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
	if shellOf(n) {
		c.ShellSkip++
		c.FrameShellSkip++
	}
	pc.NotePaintVisit()
	return true
}

// Store records n's own content into the cache after a live paint. No-op for
// non-boundaries, non-cacheable content, or when caching is disabled.
// Callers call this only when the boundary itself was dirty (or unrecorded).
func (c *BoundaryCache) Store(pc *PaintContext, n RenderObject) {
	if c == nil || pc == nil || n == nil || !pc.UseBoundaryCache || !n.IsRepaintBoundary() {
		return
	}
	if !boundaryCacheable(n) {
		return
	}
	b, ok := baseOf(n)
	if !ok {
		return
	}
	id := b.ensureCacheID()
	sz := n.Size()
	ox, oy := pc.OriginX, pc.OriginY
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		recordOwnContent(r, n, ox, oy)
	})
	c.entries[id] = &boundaryEntry{
		pic: pic,
		ox:  ox, oy: oy,
		w: sz.Width, h: sz.Height,
		contentKey: contentKeyOf(n, sz.Width, sz.Height),
		valid:      pic.Valid && !pic.IsEmpty(),
		cacheable:  true,
	}
	c.Rerecord++
	c.FrameRerecord++
	if shellOf(n) {
		c.ShellRerecord++
		c.FrameShellRerecord++
	}
	// No ancestor invalidate: parents do not bake nested RB children.
}

// shellOf reports whether n was tagged as window-shell content (R21). A node
// without a Base (or a non-boundary node) is never shell.
func shellOf(n RenderObject) bool {
	if n == nil || !n.IsRepaintBoundary() {
		return false
	}
	if b, ok := baseOf(n); ok && b != nil {
		return b.shellBoundary
	}
	return false
}

// ShellFrameCounts returns the current frame's shell skip/rerecord (R21).
func (c *BoundaryCache) ShellFrameCounts() (rerecord, skip int64) {
	if c == nil {
		return 0, 0
	}
	return c.FrameShellRerecord, c.FrameShellSkip
}

// storeColorBox is the legacy-specific entry point used by RenderColorBox.Paint.
func (c *BoundaryCache) storeColorBox(pc *PaintContext, box *RenderColorBox) {
	c.Store(pc, box)
}

// storeAbsoluteColorChildren is the legacy entry point used by AbsoluteBox.Paint.
func (c *BoundaryCache) storeAbsoluteColorChildren(pc *PaintContext, a *AbsoluteBox) {
	c.Store(pc, a)
}

// boundaryCacheable reports whether this boundary's own content can be
// faithfully recorded + fingerprinted by the MVP recorder. Anything else
// (Viewport, VirtualList, custom RO) must never be cached — Replaying an
// incomplete Picture would drop live content (stale-frame bug).
func boundaryCacheable(n RenderObject) bool {
	if n == nil {
		return false
	}
	switch t := n.(type) {
	case *RenderColorBox, *RenderText, *RenderImage:
		return true
	case *AbsoluteBox:
		return absoluteContentCacheable(t)
	default:
		return false
	}
}

func absoluteContentCacheable(a *AbsoluteBox) bool {
	if a == nil {
		return false
	}
	for _, ch := range a.children {
		if ch == nil || ch.IsRepaintBoundary() {
			continue // nested boundaries keep their own caches
		}
		switch t := ch.(type) {
		case *RenderColorBox, *RenderText, *RenderImage:
			continue
		case *AbsoluteBox:
			if !absoluteContentCacheable(t) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// contentKeyOf fingerprints a boundary node's **current** own content (the same
// scope recordOwnContent captures). Returns 0 when the node type has no MVP
// fingerprint (caller treats 0 as "no fingerprint available" and skips the
// content-match gate — origin/size already gate it).
func contentKeyOf(n RenderObject, w, h float64) uint64 {
	if n == nil {
		return 0
	}
	switch t := n.(type) {
	case *RenderColorBox:
		return colorKey(t.R, t.G, t.B, t.A, w, h)
	case *RenderText:
		return textContentKey(t, w, h)
	case *RenderImage:
		return imageContentKey(t, w, h)
	case *AbsoluteBox:
		return absoluteContentKey(t, w, h)
	default:
		return 0
	}
}

// absoluteContentKey fingerprints an AbsoluteBox boundary's own content
// (background + non-RepaintBoundary descendants). Mirrors recordOwnContent
// selection so scroll reuse (R7b) detects content change without re-recording.
func absoluteContentKey(a *AbsoluteBox, w, h float64) uint64 {
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
		case *RenderText:
			key ^= textContentKey(t, t.Size().Width, t.Size().Height) ^ uint64(off.X)<<3 ^ uint64(off.Y)<<13
		case *RenderImage:
			key ^= imageContentKey(t, t.Size().Width, t.Size().Height) ^ uint64(off.X)<<3 ^ uint64(off.Y)<<13
		case *AbsoluteBox:
			key ^= absoluteContentKey(t, t.Size().Width, t.Size().Height) ^ uint64(off.X)<<3 ^ uint64(off.Y)<<13
		default:
			return 0
		}
	}
	return key
}

// textContentKey fingerprints a RenderText's visible glyph identity:
// text content, size, face identity, color, wrap/overflow settings.
func textContentKey(t *RenderText, w, h float64) uint64 {
	if t == nil {
		return 0
	}
	faceID := uint64(0)
	if t.Face != nil {
		faceID = uint64(reflectValuePointer(t.Face.Source())) ^ uint64(t.Face.Size()*16)
	}
	k := uint64(0)
	for i := 0; i < len(t.Text) && i < 64; i++ {
		k = k*31 + uint64(t.Text[i])
	}
	k ^= faceID << 12
	k ^= uint64(t.FontSize*16) << 24
	k ^= uint64(t.R*255) << 32
	k ^= uint64(t.G*255) << 40
	k ^= uint64(t.B*255) << 48
	k ^= uint64(t.A * 255)
	k ^= uint64(w)<<20 ^ uint64(h)<<36
	if t.MaxWidth > 0 {
		k ^= uint64(t.MaxWidth*4) << 8
	}
	if len(t.Runs) > 0 {
		k ^= uint64(len(t.Runs)) << 52
	}
	return k
}

// imageContentKey fingerprints a RenderImage's visible state: image pointer,
// state, placeholder color, size.
func imageContentKey(im *RenderImage, w, h float64) uint64 {
	if im == nil {
		return 0
	}
	ptr := uint64(0)
	if im.Img != nil {
		ptr = uint64(reflectValuePointer(im.Img))
	}
	k := uint64(im.State)<<8 ^ ptr ^ uint64(w)<<20 ^ uint64(h)<<36
	k ^= uint64(im.PR*255)<<40 ^ uint64(im.PG*255)<<48 ^ uint64(im.PB*255)
	return k
}

// reflectValuePointer converts a pointer to a stable uint64 identity (no deref).
func reflectValuePointer(p interface{}) uintptr {
	if p == nil {
		return 0
	}
	v := reflect.ValueOf(p)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return 0
	}
	return v.Pointer()
}

// recordOwnContent draws n's own content (background + non-RepaintBoundary
// descendants) into a PictureRecorder at absolute origin (ox, oy).
// IsRepaintBoundary children are omitted (own cache).
func recordOwnContent(r *scene.PictureRecorder, n RenderObject, ox, oy float64) {
	if r == nil || n == nil {
		return
	}
	switch t := n.(type) {
	case *RenderColorBox:
		sz := t.Size()
		cw, chh := sz.Width, sz.Height
		if cw <= 0 {
			cw = t.Width
		}
		if chh <= 0 {
			chh = t.Height
		}
		r.FillRect(ox, oy, cw, chh, t.R, t.G, t.B, t.A)
	case *RenderText:
		sz := t.Size()
		f := t.Face
		if ef := t.effectiveFace(); ef != nil {
			f = ef
		}
		r.DrawString(t.Text, ox, oy+fontBaselineY(t), f, t.R, t.G, t.B, t.A)
		_ = sz
	case *RenderImage:
		sz := t.Size()
		dw, dh := sz.Width, sz.Height
		if dw <= 0 {
			dw = t.Width
		}
		if dh <= 0 {
			dh = t.Height
		}
		if t.Img != nil && !t.Img.Disposed() {
			r.DrawImage(t.Img, ox, oy, dw, dh)
		} else {
			r.FillRect(ox, oy, dw, dh, t.PR, t.PG, t.PB, 1)
		}
	case *AbsoluteBox:
		recordAbsoluteOwnContent(r, t, ox, oy)
	}
}

// fontBaselineY maps RenderText's logical baseline convention to the absolute
// DrawString baseline used by Paint (see RenderText.Paint).
func fontBaselineY(t *RenderText) float64 {
	if t == nil {
		return 0
	}
	return t.FontSize
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
			f := t.Face
			if ef := t.effectiveFace(); ef != nil {
				f = ef
			}
			r.DrawString(t.Text, ax, ay+t.FontSize, f, t.R, t.G, t.B, t.A)
		case *RenderImage:
			dw, dh := t.Size().Width, t.Size().Height
			if dw <= 0 {
				dw = t.Width
			}
			if dh <= 0 {
				dh = t.Height
			}
			if t.Img != nil && !t.Img.Disposed() {
				r.DrawImage(t.Img, ax, ay, dw, dh)
			} else {
				r.FillRect(ax, ay, dw, dh, t.PR, t.PG, t.PB, 1)
			}
		case *AbsoluteBox:
			recordAbsoluteOwnContent(r, t, ax, ay)
		default:
			// Unreachable after boundaryCacheable gate; keep safe no-op.
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
