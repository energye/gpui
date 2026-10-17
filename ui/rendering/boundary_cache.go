//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering

import (
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/energye/gpui/ui/scene"
)

// BoundaryCache holds Picture-backed caches for RepaintBoundary nodes.
// Clean boundaries Replay the stored Picture (skip re-record); dirty ones
// re-paint and re-record. Works under FullPaint so skip is observable without
// Retained Present.
//
// Nested model (Flutter layers, Picture MVP):
//   - Each RepaintBoundary owns a Picture of **own content only**.
//   - Nested IsRepaintBoundary children are NEVER baked into the parent Picture;
//     after a container tryReplay the caller still walks nested RB children so
//     each child can tryReplay/rerecord independently.
//   - Therefore only-inner-dirty never forces an outer rerecord, and a clean
//     outer Replay cannot show stale child colors.
//
// Cacheability (correctness-first): a boundary whose own content contains RO
// types the MVP recorder cannot faithfully capture (e.g. Viewport) is marked
// not-cacheable — tryReplay/store are no-ops for it, so the subtree always
// live-paints and stale-frame content loss is impossible.
type BoundaryCache struct {
	// mu guards every field below. Paint runs on the UI thread (inline,
	// snapshot) and the raster thread (retained record) concurrently
	// post-T2, and Clear arrives on the UI event thread — an unguarded
	// map is a fatal concurrent read/write, not just a race.
	// Methods take the lock for their whole body; none call back into
	// another locked method (recordOwnContent never touches the cache),
	// so method-scoped locking cannot deadlock.
	mu      sync.Mutex
	entries map[uint64]*boundaryEntry
	// Cumulative counters (process lifetime of this cache).
	Rerecord int64
	Skip     int64
	// Per root paint-walk frame counters (reset by BeginFrame).
	FrameRerecord int64
	FrameSkip     int64
	// FrameMiss counts boundaries that wanted a Replay but had none (diagnostics).
	FrameMiss int64
	// Shell/content partitioning: boundaries tagged with
	// SetShellBoundary are counted separately, so a scrolling body can prove
	// the shell's Picture cache is never re-recorded (shell rerecord == 0).
	// Global counters above still include shell boundaries (backward compat).
	ShellRerecord      int64
	ShellSkip          int64
	FrameShellRerecord int64
	FrameShellSkip     int64

	// Evictions is the cumulative count of entries dropped by the budget
	// (explicit cap via SetMaxEntries) or by generational sweep.
	Evictions int64

	// Worth-caching policy counters (see the admission policy block above).
	// AdmissionWaits counts live paints held back until the access threshold
	// was reached; WillChangeRejects counts content that changed since its last
	// live paint (Flutter will_change); ComplexityRejects counts scores at or
	// below the complexity gate; LimitRejects counts admissions deferred by the
	// per-frame new-cache limit or by the entry budget; MaxScore is the highest
	// score admitted to the gate so far (calibration aid).
	AdmissionWaits    int64
	WillChangeRejects int64
	ComplexityRejects int64
	LimitRejects      int64
	MaxScore          int

	// frame is the present-paint generation (BeginFrame counter); it drives
	// generational eviction of untouched entries below.
	frame uint64
	// maxEntries is the explicit budget cap (R14; 0 = unlimited, the default —
	// generational sweep still bounds long-run growth).
	maxEntries int
	// accessThreshold / maxNewPerFrame / complexityThreshold are the policy
	// knobs (Flutter raster cache defaults); see the constants above.
	accessThreshold     int
	maxNewPerFrame      int
	complexityThreshold int
	newThisFrame        int
	// admissions holds worth-caching state for boundaries that have no entry yet.
	admissions map[uint64]*admitState
}

// Generational eviction tuning: an entry untouched for evictFrames presents
// (~2s at 60Hz) is dropped at the next sweep. Scrolled-out VirtualList cells
// must release their recorded Pictures — without eviction a long scroll
// retains every cell ever seen.
// Live boundaries (shell, static panels) are touched every frame by
// tryReplay/Store and are never evicted; a re-entered cell simply records
// once again (honest scroll_rerecord cost).
const (
	evictFrames     = 120
	evictSweepEvery = 30
)

// Worth-caching / admission policy, aligned to Flutter's raster cache:
//
//   - Worth gate: content that will change is never cached, otherwise the
//     boundary's own-content complexity score must pass the gate
//     (flow/layers/display_list_raster_cache_item.cc: IsDisplayListWorthRasterizing
//     — will_change → false, else complexity_calculator->ShouldBeCached).
//   - Score: Flutter's naive calculator is the draw-op count with
//     ShouldBeCached = score > 5 (display_list/benchmarking/dl_complexity.h:39-51).
//   - Access threshold: an item must be prepared and drawn on
//     access_threshold+1 consecutive frames before it is cached
//     (flow/raster_cache.h:141 default 3, raster_cache.cc MarkSeen); 0 disables
//     caching entirely ("If the number is 0, then no picture will ever be
//     cached").
//   - Per-frame limit: at most kDefaultPictureAndDisplayListCacheLimitPerFrame
//     new caches per frame (flow/raster_cache_util.h:21 default 3,
//     raster_cache.h GenerateNewCacheInThisFrame), so one frame can never spend
//     its whole budget recording several new caches.
const (
	defaultAccessThreshold     = 3
	defaultMaxNewPerFrame      = 3
	defaultComplexityThreshold = 5
)

type boundaryEntry struct {
	pic        scene.Picture
	ox, oy     float64
	w, h       float64
	contentKey uint64
	valid      bool
	cacheable  bool
	lastSeen   uint64 // generation of last hit/store (eviction liveness)
}

// admitState is the per-boundary worth-caching state; it exists while a
// boundary is still building up its access count (no cache entry yet).
type admitState struct {
	key      uint64 // own-content fingerprint of the last live paint generation
	hasKey   bool
	frames   uint32 // consecutive live paints of the same content generation
	lastSeen uint64 // generation of last live paint (eviction liveness)
}

var nextBoundaryCacheID uint64

// NewBoundaryCache creates an empty cache with Flutter's raster-cache policy
// defaults (access threshold 3, 3 new caches per frame, naive complexity gate 5).
func NewBoundaryCache() *BoundaryCache {
	return &BoundaryCache{
		entries:             make(map[uint64]*boundaryEntry),
		admissions:          make(map[uint64]*admitState),
		accessThreshold:     defaultAccessThreshold,
		maxNewPerFrame:      defaultMaxNewPerFrame,
		complexityThreshold: defaultComplexityThreshold,
	}
}

// BeginFrame resets per-frame skip/rerecord/miss counters (call once per present paint)
// and advances the eviction generation, periodically sweeping untouched entries.
func (c *BoundaryCache) BeginFrame() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.FrameRerecord = 0
	c.FrameSkip = 0
	c.FrameMiss = 0
	c.FrameShellRerecord = 0
	c.FrameShellSkip = 0
	c.newThisFrame = 0
	c.frame++
	if c.frame%evictSweepEvery == 0 {
		for id, e := range c.entries {
			if c.frame-e.lastSeen > evictFrames {
				delete(c.entries, id)
				c.Evictions++
			}
		}
		for id, st := range c.admissions {
			if c.frame-st.lastSeen > evictFrames {
				delete(c.admissions, id)
			}
		}
	}
	c.enforceBudgetLocked()
}

// SetAccessThreshold sets how many consecutive frames a boundary must be
// prepared and drawn with unchanged content before it may enter the cache
// (Flutter raster_cache.h access_threshold; 0 disables caching entirely).
func (c *BoundaryCache) SetAccessThreshold(n int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessThreshold = n
}

// AccessThreshold returns the current access threshold (-1 for a nil cache).
func (c *BoundaryCache) AccessThreshold() int {
	if c == nil {
		return -1
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.accessThreshold
}

// SetMaxNewPerFrame sets the per-frame cap on newly recorded boundary caches
// (Flutter raster_cache_util.h kDefaultPictureAndDisplayListCacheLimitPerFrame).
func (c *BoundaryCache) SetMaxNewPerFrame(n int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxNewPerFrame = n
}

// MaxNewPerFrame returns the current per-frame new-cache cap (-1 for nil).
func (c *BoundaryCache) MaxNewPerFrame() int {
	if c == nil {
		return -1
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.maxNewPerFrame
}

// SetComplexityThreshold sets the naive complexity gate: a boundary's own
// content is cached only when its op count is greater than n (Flutter
// dl_complexity.h naive ShouldBeCached = score > 5; 0 admits any non-empty
// content).
func (c *BoundaryCache) SetComplexityThreshold(n int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.complexityThreshold = n
}

// ComplexityThreshold returns the current complexity gate (-1 for nil).
func (c *BoundaryCache) ComplexityThreshold() int {
	if c == nil {
		return -1
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.complexityThreshold
}

// PolicyCounts snapshots the worth-caching policy counters.
func (c *BoundaryCache) PolicyCounts() (admissionWaits, willChangeRejects, complexityRejects, limitRejects int64, maxScore int) {
	if c == nil {
		return 0, 0, 0, 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.AdmissionWaits, c.WillChangeRejects, c.ComplexityRejects, c.LimitRejects, c.MaxScore
}

// SetMaxEntries sets the explicit budget cap. 0 = unlimited (default;
// generational sweep still bounds long-run growth). The cap binds at admission
// and in the per-frame sweep: entries used in the current frame are never
// sacrificed (the retained texture cache's LRU rule; Flutter evicts only
// entries not encountered this frame), so a cap below the single-frame live
// set cannot bind — new entries are then refused instead of evicting in-use
// ones, which keeps the map at the live-set size without re-record thrash.
func (c *BoundaryCache) SetMaxEntries(n int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxEntries = n
	c.enforceBudgetLocked()
}

// MaxEntries returns the explicit budget cap (0 = unlimited).
func (c *BoundaryCache) MaxEntries() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.maxEntries
}

// EvictionCount reports cumulative evictions (budget + generational sweep).
// Lock-guarded: read this, not the bare Evictions field, off the owner thread.
func (c *BoundaryCache) EvictionCount() int64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Evictions
}

// enforceBudgetLocked drops entries that were NOT touched in the current frame
// while over the explicit cap, oldest first. Entries used this frame are never
// sacrificed (same rule as the retained texture cache; Flutter evicts only
// entries not encountered this frame). When the live set alone exceeds the cap
// nothing is evictable here and the cap stops new admissions instead
// (admitLivePaintLocked), so the map never grows past the live set.
func (c *BoundaryCache) enforceBudgetLocked() {
	if c.maxEntries <= 0 || len(c.entries) <= c.maxEntries {
		return
	}
	for len(c.entries) > c.maxEntries {
		var victim uint64
		oldest := ^uint64(0)
		for id, e := range c.entries {
			if e == nil || e.lastSeen >= c.frame {
				continue // used this frame: never sacrificed
			}
			if e.lastSeen < oldest {
				oldest = e.lastSeen
				victim = id
			}
		}
		if victim == 0 {
			return // nothing evictable this frame
		}
		delete(c.entries, victim)
		c.Evictions++
	}
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

// Clear drops all Picture entries and worth-caching state.
func (c *BoundaryCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[uint64]*boundaryEntry)
	c.admissions = make(map[uint64]*admitState)
	c.newThisFrame = 0
}

// Len returns the number of cached boundary entries.
func (c *BoundaryCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[b.cacheID]
	if e != nil && e.valid && e.cacheable && e.pic.Valid && !e.pic.IsEmpty() {
		e.lastSeen = c.frame
		return true
	}
	return false
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
	// Whole-body lock (see mu doc): entry validation mutates liveness and
	// counters, and the map itself is shared across threads. Replay draws
	// but never re-enters the cache, so holding the lock across it cannot
	// deadlock — only serialize with a concurrent paint.
	c.mu.Lock()
	defer c.mu.Unlock()
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
	// invalidating. Static trees have zero shift → behavior unchanged.
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
	e.lastSeen = c.frame
	c.Skip++
	c.FrameSkip++
	if shellOf(n) {
		c.ShellSkip++
		c.FrameShellSkip++
	}
	pc.NotePaintVisit()
	return true
}

// admitLivePaintLocked runs the worth-caching gate for one live paint of
// boundary id, in Flutter's order (IsDisplayListWorthRasterizing, then MarkSeen,
// then GenerateNewCacheInThisFrame):
//
//  1. content that changed since the last live paint is will_change → never
//     cached, the previous entry is dropped, access count restarts;
//  2. the own-content score must beat the complexity gate;
//  3. the boundary must have been prepared and drawn on accessThreshold+1
//     consecutive frames;
//  4. at most maxNewPerFrame new caches per frame;
//  5. the entry budget must have room, and the budget never sacrifices an
//     entry used in the current frame (it refuses new entries instead).
//
// key is the own-content fingerprint (0 = no fingerprint available, stability
// then comes from changed alone). Returns true when the caller may record.
func (c *BoundaryCache) admitLivePaintLocked(id, key uint64, score int, changed bool) bool {
	if c.accessThreshold <= 0 {
		// Flutter: access_threshold 0 disables caching entirely
		// (raster_cache.h: "If the number is 0, then no picture will ever be cached").
		return false
	}
	st := c.admissions[id]
	if st == nil {
		st = &admitState{}
		c.admissions[id] = st
	}
	st.lastSeen = c.frame
	if score > c.MaxScore {
		c.MaxScore = score
	}
	if changed {
		// will_change: the content is going to change again, no point caching it.
		// The stored entry belongs to the previous content generation and is now
		// unreachable — Flutter keys entries by the display list's unique_id, so
		// a repaint starts a fresh entry. The picture-box path has no content
		// fingerprint, so without dropping it the next clean frame would replay
		// the stale Picture (NeedsPaint is a one-frame signal).
		delete(c.entries, id)
		c.WillChangeRejects++
		st.frames = 0
		st.key, st.hasKey = key, true
		return false
	}
	if score <= c.complexityThreshold {
		c.ComplexityRejects++
		st.frames = 0
		st.key, st.hasKey = key, true
		return false
	}
	switch {
	case !st.hasKey:
		st.frames = 1 // first sight: prepared and drawn on this frame
	case key != 0 && st.key == key:
		st.frames++ // same content generation continued
	case key == 0:
		st.frames++ // no fingerprint: stability signal is !changed
	default:
		st.frames = 1 // fingerprint changed since the last live paint
	}
	st.key, st.hasKey = key, true
	if int(st.frames) <= c.accessThreshold {
		c.AdmissionWaits++
		return false
	}
	if c.newThisFrame >= c.maxNewPerFrame {
		c.LimitRejects++
		return false
	}
	if c.maxEntries > 0 && len(c.entries) >= c.maxEntries {
		// Budget: free room by dropping entries not touched this frame. When the
		// live set alone fills the cap, defer admission (Flutter blocks new
		// rasterization over budget instead of sacrificing in-use entries).
		c.enforceBudgetLocked()
		if len(c.entries) >= c.maxEntries {
			c.LimitRejects++
			return false
		}
	}
	c.newThisFrame++
	return true
}

// boundaryScore is the naive complexity score of a boundary's own content: the
// number of draw ops recordOwnContent emits, mirroring Flutter's naive
// calculator (display_list/benchmarking/dl_complexity.h: Compute = op count).
// It is computed before recording so gated-out content never pays the record.
func boundaryScore(n RenderObject) int {
	switch t := n.(type) {
	case *RenderColorBox, *RenderImage:
		return 1
	case *RenderText:
		return textScore(t)
	case *AbsoluteBox:
		return absoluteOwnScore(t)
	default:
		// RenderPictureBox content is opaque until its Record closure runs;
		// StorePicture scores the built Picture instead.
		return 0
	}
}

// textScore mirrors recordRenderText's op emission: one draw op per visible
// row (shaped rows and DrawString rows both emit one op per row).
func textScore(t *RenderText) int {
	if t == nil {
		return 0
	}
	if t.hasRuns() {
		if n := len(t.layoutRunLines()); n > 0 {
			return n
		}
	}
	if lines := t.DisplayLines(); len(lines) > 0 {
		n := 0
		for _, ln := range lines {
			if ln != "" {
				n++
			}
		}
		if n > 0 {
			return n
		}
	}
	return 1
}

// absoluteOwnScore mirrors recordAbsoluteOwnContent: background (one fill) plus
// every non-RepaintBoundary child's own ops (nested boundaries keep their own
// caches and are not part of this Picture).
func absoluteOwnScore(a *AbsoluteBox) int {
	if a == nil {
		return 0
	}
	score := 0
	if a.Background != nil {
		score++
	}
	for _, ch := range a.children {
		if ch == nil || ch.IsRepaintBoundary() {
			continue
		}
		score += boundaryScore(ch)
	}
	return score
}

// storeEntryLocked writes one boundary entry and bumps rerecord counters.
// Callers hold no lock; single entry point for Store/StorePicture so budget,
// shell accounting and ancestor policy cannot drift apart.
func (c *BoundaryCache) storeEntryLocked(n RenderObject, pic scene.Picture, ox, oy float64, valid bool) {
	b, ok := baseOf(n)
	if !ok {
		return
	}
	id := b.ensureCacheID()
	sz := n.Size()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[id] = &boundaryEntry{
		pic: pic,
		ox:  ox, oy: oy,
		w: sz.Width, h: sz.Height,
		contentKey: contentKeyOf(n, sz.Width, sz.Height),
		valid:      valid,
		cacheable:  true,
		lastSeen:   c.frame,
	}
	c.enforceBudgetLocked()
	c.Rerecord++
	c.FrameRerecord++
	if shellOf(n) {
		c.ShellRerecord++
		c.FrameShellRerecord++
	}
	// No ancestor invalidate: parents do not bake nested RB children.
}

// Store records n's own content into the cache after a live paint. No-op for
// non-boundaries, non-cacheable content, or when caching is disabled.
// Callers call this only when the boundary itself was dirty (or unrecorded).
func (c *BoundaryCache) Store(pc *PaintContext, n RenderObject) {
	c.storeWithChange(pc, n, false)
}

// storeWithChange is Store plus the caller's own-content change signal
// (changed = the boundary was self-dirty this frame; the will_change analog).
func (c *BoundaryCache) storeWithChange(pc *PaintContext, n RenderObject, changed bool) {
	if c == nil || pc == nil || n == nil || !pc.UseBoundaryCache || !n.IsRepaintBoundary() {
		return
	}
	if !boundaryCacheable(n) {
		return
	}
	if !c.admit(n, boundaryScore(n), changed) {
		return
	}
	ox, oy := pc.OriginX, pc.OriginY
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		recordOwnContent(r, n, ox, oy)
	})
	// Nothing own to draw (all visual content lives in nested boundaries):
	// an empty Picture can never replay usefully (tryReplay rejects empty
	// entries), so do not create an entry — otherwise this boundary would
	// re-store (and count a rerecord) every frame forever.
	if pic.IsEmpty() {
		return
	}
	c.storeEntryLocked(n, pic, ox, oy, pic.Valid && !pic.IsEmpty())
}

// admit runs the worth-caching gate for one live paint of n.
func (c *BoundaryCache) admit(n RenderObject, score int, changed bool) bool {
	b, ok := baseOf(n)
	if !ok {
		return false
	}
	sz := n.Size()
	id := b.ensureCacheID()
	key := contentKeyOf(n, sz.Width, sz.Height)
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.admitLivePaintLocked(id, key, score, changed)
}

// StorePicture records a caller-built picture for boundary n (single-build
// path used by RenderPictureBox.Paint: live paint already replayed these
// ops, so no second record pass). Same gates as Store plus Valid/non-empty:
// empty pictures can never replay usefully (tryReplay rejects them), so
// storing one would re-store every frame forever.
func (c *BoundaryCache) StorePicture(pc *PaintContext, n RenderObject, pic scene.Picture) {
	c.storePictureWithChange(pc, n, pic, false)
}

// storePictureWithChange is StorePicture plus the caller's content change
// signal (the picture box was self-dirty this frame).
func (c *BoundaryCache) storePictureWithChange(pc *PaintContext, n RenderObject, pic scene.Picture, changed bool) {
	if c == nil || pc == nil || n == nil || !pc.UseBoundaryCache || !n.IsRepaintBoundary() {
		return
	}
	if !boundaryCacheable(n) {
		return
	}
	if !pic.Valid || pic.IsEmpty() {
		return
	}
	// Score the content that was actually built (RenderPictureBox Record
	// closures are opaque until they run).
	if !c.admit(n, pic.OpCount(), changed) {
		return
	}
	ox, oy := pc.OriginX, pc.OriginY
	c.storeEntryLocked(n, pic, ox, oy, true)
}

// shellOf reports whether n was tagged as window-shell content. A node
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

// ShellFrameCounts returns the current frame's shell skip/rerecord.
func (c *BoundaryCache) ShellFrameCounts() (rerecord, skip int64) {
	if c == nil {
		return 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.FrameShellRerecord, c.FrameShellSkip
}

// FrameCounts snapshots the per-frame counters (R1: the paint threads write
// these while metrics/example readers sample them — read the snapshot, not
// the bare fields).
func (c *BoundaryCache) FrameCounts() (rerecord, skip, shellRerecord, shellSkip int64) {
	if c == nil {
		return 0, 0, 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.FrameRerecord, c.FrameSkip, c.FrameShellRerecord, c.FrameShellSkip
}

// storeColorBox is the legacy-specific entry point used by RenderColorBox.Paint.
func (c *BoundaryCache) storeColorBox(pc *PaintContext, box *RenderColorBox) {
	c.Store(pc, box)
}

// storeAbsoluteColorChildren is the entry point used by AbsoluteBox.Paint;
// changed carries the boundary's own must-change signal (selfDirty).
func (c *BoundaryCache) storeAbsoluteColorChildren(pc *PaintContext, a *AbsoluteBox, changed bool) {
	c.storeWithChange(pc, a, changed)
}

// boundaryCacheable reports whether this boundary's own content can be
// faithfully recorded + fingerprinted by the recorder. RenderPictureBox
// carries its own Record closure (pure function of box state; NeedsPaint is
// the invalidation signal). Anything else unrepresentable (Viewport,
// VirtualList, custom RO) must never be cached — Replaying an
// incomplete Picture would drop live content (stale-frame bug).
func boundaryCacheable(n RenderObject) bool {
	if n == nil {
		return false
	}
	switch t := n.(type) {
	case *RenderColorBox, *RenderText, *RenderImage:
		return true
	case *RenderPictureBox:
		return t.Record != nil
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
		case *RenderPictureBox:
			// Non-boundary picture boxes carry a caller-owned Record
			// closure with no content fingerprint — inlining them would
			// risk a stale parent bake. Use picture boxes as boundaries.
			return false
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
	case *RenderPictureBox:
		// No fingerprint: Record is a caller-owned closure. NeedsPaint is
		// the invalidation signal (SetRecord dirties). tryReplay skips the
		// content-match gate when both keys are 0; origin/size still gate.
		return 0
	case *AbsoluteBox:
		return absoluteContentKey(t, w, h)
	default:
		return 0
	}
}

// absoluteContentKey fingerprints an AbsoluteBox boundary's own content
// (background + non-RepaintBoundary descendants).
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
// text content, size, face identity, color, wrap/overflow settings,
// letter/word spacing (6a: spaced pictures must not replay unspaced),
// font weight (6b: bold masks must not replay regular).
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
	if t.LetterSpacing != 0 || t.WordSpacing != 0 {
		// Via int64 (float→uint64 of negatives is fragile; int64→uint64
		// wraps deterministically).
		k ^= uint64(spacingQuant(t.LetterSpacing)) << 4
		k ^= uint64(spacingQuant(t.WordSpacing)) << 20
	}
	if t.FontWeight != 0 {
		k ^= uint64(t.FontWeight) << 44
	}
	if len(t.Runs) > 0 {
		k ^= uint64(len(t.Runs)) << 52
	}
	return k
}

// spacingQuant quantizes a spacing value to 1/16px for cache fingerprints.
func spacingQuant(v float64) int64 {
	if v >= 0 {
		return int64(v*16 + 0.5)
	}
	return -int64(-v*16 + 0.5)
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
		recordRenderText(r, t, ox, oy)
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
	case *RenderPictureBox:
		if t.Record != nil {
			t.Record(r, ox, oy)
		}
	case *AbsoluteBox:
		recordAbsoluteOwnContent(r, t, ox, oy)
	}
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
			recordRenderText(r, t, ax, ay)
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
