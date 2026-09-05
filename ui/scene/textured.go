package scene

import (
	"fmt"
	"image"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// PictureTextureCache keeps offscreen GPU textures for PictureLayers across
// frames (W2 R4 true-retained layer compositing).
//
// Clean layers blit their cached texture (zero raster); dirty layers re-record
// their display list into the texture via FlushGPUWithView (offscreen RTs
// always LoadOpClear, so re-record replaces stale content). The composited
// frame therefore contains only textured-quad draws → GPU blit-only path →
// LoadOpLoad + per-rect scissor for the dirty layer rects.
//
// Persistent double-buffered layer surfaces (skia / Flutter raster-thread
// pattern): each entry owns TWO texture slots that are reused across frames.
// Every re-record alternates the write slot, so a texture is never both
// sampled (blit RESOURCE) and re-recorded (COLOR_TARGET) within the same
// usage scope — wgpu rejects exclusive-usage conflicts inside one render
// pass (observed as TXFLUSHERR "submit failed … conflicting usages
// RESOURCE/COLOR_TARGET" whenever a re-record coalesced with a stashed blit
// of the same texture). The other slot keeps serving blits while the written
// slot is refreshed; slots are reallocated only when their size changes
// (resize / recordLocal↔recordWith switch), which keeps animated layers at
// zero GPU allocations per frame (no RSS slope from per-frame churn). The
// released view stays deferred a couple of frames so in-flight command
// buffers never see a destroyed texture.
//
// Textures are normally bounds-sized (picture geometry + AA pad) for the
// common pure-translate case, so a large grid of small layers stays far below
// the GPU's texture budget; layers under clips / transforms / rotation or with
// text-only pictures fall back to full-window textures. Picture ops are
// recorded in the same layer-local coordinate space the composite walk
// establishes (via translate/clip), then blit back 1:1. Entries not used
// for a frame are evicted (LRU cap). Without GPU (CreateOffscreenTexture
// returns nil) every operation degrades to a no-op and callers fall back to
// direct vector replay — correctness never depends on the cache.
//
// Thread-safety (Flutter raster-thread pattern): the cache lives on the GPU
// (raster) thread during composite, but the UI thread also touches it for
// frame-boundary housekeeping (EndFrame/Resize/Clear). Like Skia's GPU
// resource cache, all map/state access is mutex-guarded; internal helpers
// (evictForNew/releaseDeferred/drainDeferred) must only be called while the
// caller already holds mu.
type PictureTextureCache struct {
	mu     sync.Mutex
	dc     *render.Context
	max    int
	width  int
	height int
	// filterCache caches FILTERED ColorFilter/ImageFilter subtree results
	// (R20): unchanged frames blit instead of re-paying the CPU filter pass.
	filterCache *FilterResultCache
	// entries keyed by stable CacheKey.
	entries map[uint64]*pictureTextureEntry
	// liveKeys is the current frame's in-tree cache keys (UI thread publishes
	// via SetLiveKeys; evictForNew never victimizes a live key).
	liveKeys map[uint64]struct{}
	// stamp is a monotonically-increasing last-use counter for LRU eviction.
	stamp   uint64
	usedNow map[uint64]struct{}
	// explicitMax is the pinned budget from SetBudget (0 = automatic sizing
	// via EnsureCapacity; the pinned value REPLACES it for eviction).
	explicitMax int
	// recordFrame is incremented per frame; entries store the frame they were
	// re-recorded in so damage rects cover exactly the refreshed layers
	// (blit also bumps lastUse, which must not count as a re-record).
	recordFrame uint64
	// slotAlloc is the slot texture factory; defaultSlotAlloc allocates a
	// real offscreen texture. Injectable in package-internal tests to observe
	// ring alternation / reallocation without a GPU.
	slotAlloc func(c *PictureTextureCache, w, h int) (*pictureTextureSlot, bool)
	// deferred holds release closures held back for a few frames so a view is
	// never destroyed while earlier frames' command buffers (and the render
	// session's view→bind-group slot cache) may still reference it.
	deferred []deferredRelease
	// FrameRerecord / FrameSkip are per-frame counters for boundary metrics
	// (texture re-record = rerecord, cached blit = skip). Atomic: EndFrame
	// (UI thread) resets them while the raster thread may still be reading.
	FrameRerecord atomic.Int64
	FrameSkip     atomic.Int64
	// Evictions is the cumulative count of entries dropped by the capacity
	// LRU in evictForNew (R14 observability). Guarded by mu.
	Evictions int64
}

type pictureTextureSlot struct {
	view    render.TextureView
	release func()
	// w, h are the texture extent in LOGICAL pixels (blit size at identity).
	// The backing texture is w×h physical (deviceScale applied at creation).
	w, h int
	// lastWriteFrame is the recordFrame of the slot's most recent write
	// (ring alternation — the write target must not be in flight).
	lastWriteFrame uint64
}

type pictureTextureEntry struct {
	// slots are the layer's persistent GPU surfaces (double-buffer ring).
	slots [2]pictureTextureSlot
	// contentSlot is the slot holding the current content for blit.
	contentSlot int
	lastUse     uint64
	bounds      image.Rectangle
	// recordedIn is the recordFrame in which the texture was last re-recorded.
	recordedIn uint64
	// off is the layer-local blit offset (bounds.Min of the recorded geometry,
	// nonzero for text bounds whose Min may be negative above the baseline).
	off image.Point
}

// NewPictureTextureCache creates a cache bound to dc (logical size) with a
// max-entry LRU cap. max <= 0 defaults to 64.
func NewPictureTextureCache(dc *render.Context, max int) *PictureTextureCache {
	if max <= 0 {
		max = 64
	}
	w, h := 0, 0
	if dc != nil {
		w, h = dc.Width(), dc.Height()
	}
	c := &PictureTextureCache{
		dc:          dc,
		max:         max,
		width:       w,
		height:      h,
		entries:     make(map[uint64]*pictureTextureEntry),
		usedNow:     make(map[uint64]struct{}),
		filterCache: NewFilterResultCache(),
	}
	if dc != nil {
		c.slotAlloc = defaultSlotAlloc
	}
	render.RegisterPurgeEvictable("picture-texture", c)
	return c
}

// EnsureCapacity grows the LRU cap to cover n entries (never shrinks — a
// shrink mid-run would thrash evict→record every frame). Callers size the cap
// from the current frame's cacheable picture-layer count so scenes with more
// layers than the default 64 keep their whole working set cached instead of
// cycling through the LRU (each eviction forces an offscreen re-record).
//
// SetLiveKeys records the current frame's cache keys so evictForNew never
// picks a live-tree layer as its victim (a live layer not yet blitted this
// frame would otherwise look "old" during phase 1 and get evicted mid-frame,
// forcing a re-record next frame).
func (c *PictureTextureCache) EnsureCapacity(n int) {
	if c == nil || n <= c.max {
		return
	}
	c.max = n
}

// SetBudget pins an explicit entry budget (R14). 0 = automatic (default: the
// composite path sizes the LRU to the live working set via EnsureCapacity).
// When set, the pinned value REPLACES the automatic cap for eviction purposes.
//
// Correctness note: evictForNew never victimizes an entry used in the current
// frame, so a budget BELOW the per-frame live working set cannot bind — entry
// count floats up to the per-frame distinct-key count instead (same-frame
// eviction would thrash re-records). A budget ABOVE the working set bounds the
// cross-frame accumulation (scroll-in/out churn) and is the honest stress
// configuration. Either way correctness never depends on the cache: an
// evicted layer re-records (vector replay fallback).
func (c *PictureTextureCache) SetBudget(n int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.explicitMax = n
}

// Budget returns the pinned explicit budget (0 = automatic sizing).
func (c *PictureTextureCache) Budget() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.explicitMax
}

// effMaxLocked is the effective LRU cap for eviction: the pinned budget when
// set, otherwise the automatic working-set size.
func (c *PictureTextureCache) effMaxLocked() int {
	if c.explicitMax > 0 {
		return c.explicitMax
	}
	return c.max
}

// SetLiveKeys publishes the frame's live cache-key set (UI thread, before the
// raster job runs). Guarded by mu like every other cross-thread field.
func (c *PictureTextureCache) SetLiveKeys(keys []uint64) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.liveKeys == nil {
		c.liveKeys = make(map[uint64]struct{}, len(keys))
	}
	for k := range c.liveKeys {
		delete(c.liveKeys, k)
	}
	for _, k := range keys {
		c.liveKeys[k] = struct{}{}
	}
}

func (c *PictureTextureCache) isLive(id uint64) bool {
	if len(c.liveKeys) == 0 {
		return false // no live info yet: behave as before
	}
	_, ok := c.liveKeys[id]
	return ok
}

// CountCacheablePictureLayers walks pkt and counts picture layers that would
// occupy a texture-cache entry (CacheKey != 0, main + overlay bands). Used to
// size the texture cache to the frame's working set.
func CountCacheablePictureLayers(pkt *FramePacket) int {
	_, n := CollectCacheableKeys(pkt)
	return n
}

// CollectCacheableKeys returns every cacheable picture-layer key in pkt
// (main + overlay bands) plus its count — the frame's live working set.
func CollectCacheableKeys(pkt *FramePacket) ([]uint64, int) {
	if pkt == nil {
		return nil, 0
	}
	var keys []uint64
	n := 0
	Walk(pkt.Root, func(l Layer) {
		if pl, ok := l.(*PictureLayer); ok && pl.CacheKey != 0 {
			keys = append(keys, pl.CacheKey)
			n++
		}
	})
	Walk(pkt.Overlay, func(l Layer) {
		if pl, ok := l.(*PictureLayer); ok && pl.CacheKey != 0 {
			keys = append(keys, pl.CacheKey)
			n++
		}
	})
	return keys, n
}

// defaultSlotAlloc allocates a fresh offscreen texture via the render context.
func defaultSlotAlloc(c *PictureTextureCache, w, h int) (*pictureTextureSlot, bool) {
	view, release := c.dc.CreateOffscreenTexture(w, h)
	if view.IsNil() || release == nil {
		return nil, false
	}
	return &pictureTextureSlot{view: view, release: release, w: w, h: h}, true
}

// Resize updates the full-window texture budget. Entries are NOT cleared:
// recordWith/recordLocalWith rebuild a view when its recorded size no longer
// matches (w != c.width || h != c.height), so unchanged layers keep their
// textures across a resize and only affected layers re-record.
func (c *PictureTextureCache) Resize(w, h int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.width = w
	c.height = h
}

// Clear releases all cached textures (force frame / resize / cache drop).
// Releases are deferred a couple of frames so in-flight command buffers that
// reference the views are never left dangling.
func (c *PictureTextureCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ClearLocked()
	c.filterCache.Clear()
}

func (c *PictureTextureCache) ClearLocked() {
	for id, e := range c.entries {
		if e != nil {
			c.releaseEntryLocked(e)
		}
		delete(c.entries, id)
	}
	c.usedNow = make(map[uint64]struct{})
	c.FrameRerecord.Store(0)
	c.FrameSkip.Store(0)
}

type deferredRelease struct {
	release func()
	frame   uint64
}

// releaseDeferred queues a texture release until deferredFrames frames have
// passed. The render session's view→bind-group slot cache and in-flight
// command buffers may still reference the view; destroying it immediately can
// panic the wgpu backend ("invalid bind group entry"). Keeping the view alive
// a couple of frames past its last use makes destruction always safe.
func (c *PictureTextureCache) releaseDeferred(release func(), frame uint64) {
	if release == nil {
		return
	}
	c.deferred = append(c.deferred, deferredRelease{release: release, frame: frame})
	if len(c.deferred) > c.effMaxLocked()*3 {
		c.drainDeferred(c.recordFrame)
	}
}

// drainDeferred executes queued releases whose frame is at least
// deferredFrames behind the current frame.
func (c *PictureTextureCache) drainDeferred(current uint64) {
	const deferredFrames = 2
	keep := c.deferred[:0]
	for _, d := range c.deferred {
		if current >= d.frame+deferredFrames {
			d.release()
			continue
		}
		keep = append(keep, d)
	}
	c.deferred = keep
}

// BeginFrame marks the start of a composite frame (increments the record
// frame counter used by RecordedThisFrame) and drains releases that are safe
// to execute now.
func (c *PictureTextureCache) BeginFrame() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recordFrame++
	c.stamp++ // advance the LRU clock once per composite frame
	c.drainDeferred(c.recordFrame)
}

// EndFrame resets per-frame counters and the usedNow scratch set. It does NOT
// evict: usedNow is filled by the raster thread's composite of the *previous*
// job, so any UI-side eviction decision races the raster thread (whenever UI
// runs ahead, every entry looks unused → full-band re-record bursts, measured
// in C4). Eviction is handled solely by the capacity LRU in evictForNew
// (oldest lastUse wins), which consults the stamp clock advanced by BeginFrame.
func (c *PictureTextureCache) EndFrame() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.usedNow = make(map[uint64]struct{})
	c.FrameRerecord.Store(0)
	c.FrameSkip.Store(0)
}

// allocEntry returns a usable cache entry for id at size (w, h), reusing the
// entry's persistent double-buffered slots. Re-records (dirty layers) must
// never target a texture that is still referenced by in-flight or stashed
// commands: an earlier submission may be sampling it (blit RESOURCE) while
// the re-record takes it as COLOR_TARGET — wgpu rejects conflicting exclusive
// usages within one usage scope (observed as TXFLUSHERR "submit failed …
// conflicting usages" whenever a re-record coalesced with a stashed blit of
// the same texture). The ring alternates the write slot per frame, so the
// write target is by construction the slot last written two frames ago —
// never the one the previous submission sampled. Slots are reallocated only
// when their size changes; the replaced view is deferred-released, so it
// stays alive until every in-flight submission referencing it has completed.
func (c *PictureTextureCache) allocEntry(id uint64, w, h int) *pictureTextureEntry {
	e := c.entries[id]
	if e == nil {
		if !c.evictForNew() {
			return nil
		}
		e = &pictureTextureEntry{contentSlot: -1}
		c.entries[id] = e
	}
	// Ring selection: any slot not written this frame; among those the one
	// with the oldest lastWriteFrame (the least likely to be in flight).
	slot := -1
	for i := range e.slots {
		if e.slots[i].lastWriteFrame == c.recordFrame {
			continue
		}
		if slot < 0 || e.slots[i].lastWriteFrame < e.slots[slot].lastWriteFrame {
			slot = i
		}
	}
	if slot < 0 {
		// Pathological double-record within one frame (both slots written):
		// reuse slot 0 — two writes of the same texture are both COLOR_TARGET
		// (same usage, no blit between them), so no conflict.
		slot = 0
	}
	s := &e.slots[slot]
	if s.w != w || s.h != h {
		if s.release != nil {
			c.releaseDeferred(s.release, c.recordFrame)
		}
		ns, ok := c.slotAlloc(c, w, h)
		if !ok {
			// no GPU (or alloc failure): keep the slot's stale content and
			// let callers fall back to direct vector replay this frame.
			// A brand-new entry (no live slot yet) is dropped so the failed
			// first alloc doesn't leave a husk that blocks later attempts.
			if e.contentSlot < 0 {
				delete(c.entries, id)
			}
			return nil
		}
		*s = *ns
	}
	s.lastWriteFrame = c.recordFrame
	e.contentSlot = slot
	return e
}

// releaseEntryLocked deferred-releases every slot view of an entry. The
// caller must already hold mu.
func (c *PictureTextureCache) releaseEntryLocked(e *pictureTextureEntry) {
	if e == nil {
		return
	}
	for i := range e.slots {
		if e.slots[i].release != nil {
			c.releaseDeferred(e.slots[i].release, c.recordFrame)
			e.slots[i].view = render.TextureView{}
			e.slots[i].release = nil
		}
	}
}

// record re-records picture content into the cached texture for id (or creates
// the texture on first use). Returns the picture geometry bounds. Degrades to a
// no-op returning false when GPU textures are unavailable.
func (c *PictureTextureCache) record(id uint64, pic *Picture) (image.Rectangle, bool) {
	return c.recordWith(id, pic, nil)
}

// recordWith is record plus an optional raster-thread extra paint (RenderBox
// OnPaint layers): when extra is non-nil it draws after pic.Replay (or alone
// for empty pictures) in the same surface coordinate space.
func (c *PictureTextureCache) recordWith(id uint64, pic *Picture, extra func(dc *render.Context)) (image.Rectangle, bool) {
	if c == nil || c.dc == nil || id == 0 || (pic != nil && pic.IsEmpty() && extra == nil) {
		return image.Rectangle{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.allocEntry(id, c.width, c.height)
	if e == nil {
		return image.Rectangle{}, false
	}
	// Replay into the offscreen RT (FlushGPUWithView resolves queued ops and
	// offscreen RTs LoadOpClear first — stale content is replaced). The replay
	// targets the offscreen view, so its Fill/Stroke must not register damage
	// on the surface dc (otherwise every re-record would add layer-local
	// rects at the origin to the present damage union).
	//
	// Isolated sub-pass: same pass-ownership suspension as recordLocalWith —
	// the record's commands/flush must not touch the suspended main stream
	// (C8 root-cause fix; see the longer comment in recordLocalWith).
	defer c.dc.BeginOffscreenPass()()
	c.dc.SetDamageTracking(false)
	if pic != nil {
		pic.Replay(c.dc)
	}
	if extra != nil {
		c.dc.Push()
		extra(c.dc)
		c.dc.Pop()
	}
	err := c.dc.FlushGPUWithView(e.slots[e.contentSlot].view, uint32(c.width), uint32(c.height)) //nolint:gosec
	c.dc.SetDamageTracking(true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "TXFLUSHERR id=%d w=%d h=%d err=%#v\n", id, c.width, c.height, err)
		// The fresh view may hold undefined content; drop the entry so later
		// blits replay vector instead of showing garbage.
		c.releaseEntryLocked(e)
		delete(c.entries, id)
		return image.Rectangle{}, false
	}
	if pic != nil {
		e.bounds = pic.Bounds
	} else {
		e.bounds = image.Rectangle{}
	}
	if os.Getenv("WR_RESIZE_DBG") == "1" {
		fmt.Fprintf(os.Stderr, "DBG record full id=%d w=%d h=%d\n", id, c.width, c.height)
	}
	e.lastUse = c.stamp
	e.recordedIn = c.recordFrame
	c.usedNow[id] = struct{}{}
	c.FrameRerecord.Add(1)
	return e.bounds, true
}

// recordLocal records a picture into a texture sized to its geometry bounds
// (plus AA pad) instead of the full window — the common retained case (a cell
// or block) then costs a tiny RT instead of a full-surface one. The caller must
// be at a pure-translate CTM (no clips, rotation or scale): the layer-local
// origin is cancelled with a translate so ops land in the RT starting at
// b.Min (text bounds may start above the baseline, hence the offset).
func (c *PictureTextureCache) recordLocal(id uint64, pic *Picture, b image.Rectangle) (image.Rectangle, bool) {
	return c.recordLocalWith(id, pic, b, nil)
}

// recordLocalWith is recordLocal plus an optional raster-thread extra paint
// (RenderBox OnPaint layers, see PictureLayer.RasterExtra). The extra callback
// draws after pic.Replay in the same layer-local (translated) coordinate space.
func (c *PictureTextureCache) recordLocalWith(id uint64, pic *Picture, b image.Rectangle, extra func(dc *render.Context)) (image.Rectangle, bool) {
	if c == nil || c.dc == nil || id == 0 || (pic != nil && pic.IsEmpty() && extra == nil) {
		return image.Rectangle{}, false
	}
	if b.Empty() {
		return image.Rectangle{}, false
	}
	w := b.Dx() + 2
	h := b.Dy() + 2
	if w > c.width {
		w = c.width
	}
	if h > c.height {
		h = c.height
	}
	if w <= 0 || h <= 0 {
		return image.Rectangle{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.allocEntry(id, w, h)
	if e == nil {
		return image.Rectangle{}, false
	}
	// Cancel the layer-local origin (device coords → user translate by /scale)
	// and shift by -b.Min so the picture replays into the RT starting at (0,0).
	scale := c.dc.DeviceScale()
	if scale <= 0 {
		scale = 1
	}
	// Isolated sub-pass: suspend the main frame stream so this record's
	// commands and flush never interleave with (or stash away) the surface's
	// queued ops — Skia per-surface GrRenderTargetContext ownership. Without
	// it, the mid-frame flush triggered prepareTarget to stash the mixed
	// queue and the texture recorded black (C8 root cause).
	defer c.dc.BeginOffscreenPass()()
	ox, oy := c.dc.TransformPoint(0, 0)
	c.dc.Push()
	c.dc.Translate(-ox/scale-float64(b.Min.X), -oy/scale-float64(b.Min.Y))
	c.dc.SetDamageTracking(false)
	if pic != nil {
		pic.Replay(c.dc)
	}
	if extra != nil {
		extra(c.dc)
	}
	err := c.dc.FlushGPUWithView(e.slots[e.contentSlot].view, uint32(w), uint32(h)) //nolint:gosec
	c.dc.SetDamageTracking(true)
	c.dc.Pop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "TXFLUSHERR id=%d w=%d h=%d err=%v\n", id, w, h, err)
		// The fresh view may hold undefined content; drop the entry so later
		// blits replay vector instead of showing garbage.
		c.releaseEntryLocked(e)
		delete(c.entries, id)
		return image.Rectangle{}, false
	}
	e.bounds = b
	e.off = b.Min
	if os.Getenv("WR_RESIZE_DBG") == "1" {
		fmt.Fprintf(os.Stderr, "DBG record local id=%d w=%d h=%d b=%v\n", id, w, h, b)
	}
	e.lastUse = c.stamp
	e.recordedIn = c.recordFrame
	c.usedNow[id] = struct{}{}
	c.FrameRerecord.Add(1)
	return e.bounds, true
}

// measureTextBounds estimates the layer-local bounds of a text-only picture
// (geometry ops have no tracked bounds, so text layers would otherwise always
// record full-window textures with an empty damage rect).
//
// Bounds are computed per fallback run with each run face's own metrics and
// glyph ink boxes (Flutter RenderParagraph / Skia glyph-bounds semantics):
//   - A MultiFace's aggregate metrics reflect the first face only, and CJK
//     ink can rise ~0.12em above the Latin ascent (top strokes of 局/屏/重/
//     景/损 were clipped by the offscreen viewport).
//   - Latin accents / descenders and right-side-bearing overhang exceed
//     metrics/advance estimates too, so each glyph's outline bbox (scaled to
//     pixels, Y-down from the baseline) is unioned in, padded by 2px to
//     absorb raster pixel-fit and AA spread (Skia pads glyph bounds +1).
func (c *PictureTextureCache) measureTextBounds(pic *Picture) (image.Rectangle, bool) {
	if c == nil || c.dc == nil || pic == nil || pic.IsEmpty() {
		return image.Rectangle{}, false
	}
	var b image.Rectangle
	found := false
	union := func(r image.Rectangle) {
		if r.Empty() {
			return
		}
		if !found {
			b = r
			found = true
		} else {
			b = b.Union(r)
		}
	}
	for i := range pic.Ops {
		op := &pic.Ops[i]
		if op.Kind != OpDrawString || op.Text == "" {
			continue
		}
		face := op.Face
		if face == nil {
			face = c.dc.Font()
		}
		if face == nil {
			continue
		}
		runs := []text.FaceRun{{Face: face, Text: op.Text, X: 0}}
		if mf, ok := face.(*text.MultiFace); ok {
			runs = mf.Runs(op.Text)
		}
		for _, run := range runs {
			if run.Face == nil || run.Text == "" {
				continue
			}
			m := run.Face.Metrics()
			// Metrics floor (per-run face) so shaped/glyph-less cases still
			// have a bound; per-glyph ink boxes below tighten it.
			union(image.Rect(
				int(math.Floor(op.X+run.X)),
				int(math.Floor(op.Y-m.Ascent)),
				int(math.Ceil(op.X+run.X+run.Face.Advance(run.Text))),
				int(math.Ceil(op.Y+m.Descent)),
			))
			// Exact per-glyph ink bounds (glyf/CFF bbox scaled to pixels,
			// Y-down from the baseline). The pen advances with the same
			// per-glyph rounding as the glyph-mask engine (device-pixel grid
			// snap, Skia strikeToSource), so long strings don't drift out of
			// the estimate; the 2px inset absorbs raster pixel-fit + AA.
			scale := c.dc.DeviceScale()
			if scale <= 0 {
				scale = 1
			}
			pen := 0.0
			for g := range run.Face.Glyphs(run.Text) {
				gb := g.Bounds
				if gb.MinX == 0 && gb.MaxX == 0 && gb.MinY == 0 && gb.MaxY == 0 {
					pen += math.Round(g.Advance * scale)
					continue // space / empty glyph
				}
				union(image.Rect(
					int(math.Floor(op.X+run.X+pen+gb.MinX)),
					int(math.Floor(op.Y+g.Y+gb.MinY)),
					int(math.Ceil(op.X+run.X+pen+gb.MaxX)),
					int(math.Ceil(op.Y+g.Y+gb.MaxY)),
				).Inset(-2))
				pen += math.Round(g.Advance * scale)
			}
		}
	}
	return b, found
}

// evictForNew makes room for a new texture entry when the LRU cap is reached.
// NEVER victimizes a live-tree key (see SetLiveKeys): a live layer not yet
// blitted this frame looks "old" during phase 1 (lastUse = previous frame's
// stamp), and evicting it forces a needless re-record next frame — measured
// as sporadic single-layer shell rerecords in C4 at popup close. Victims come
// from non-live entries first (dismissed popups' orphans); if none exist the
// cache is genuinely over capacity with live content, so fall back to the
// oldest entry (correctness never depends on the cache). Always returns true.
func (c *PictureTextureCache) evictForNew() bool {
	max := c.effMaxLocked()
	if max <= 0 || len(c.entries) < max {
		return true
	}
	pick := func(skipLive bool) uint64 {
		var victim uint64
		var oldest uint64 = ^uint64(0)
		for id, e := range c.entries {
			if e == nil {
				continue
			}
			if _, used := c.usedNow[id]; used {
				continue
			}
			if skipLive && c.isLive(id) {
				continue
			}
			if e.lastUse < oldest {
				oldest = e.lastUse
				victim = id
			}
		}
		return victim
	}
	victim := pick(true)
	if victim == 0 {
		victim = pick(false)
	}
	if victim != 0 {
		if e := c.entries[victim]; e != nil {
			c.releaseEntryLocked(e)
		}
		delete(c.entries, victim)
		c.Evictions++
	}
	return true
}

// PurgeEvictable drops every cached layer texture that is neither in-flight
// this frame (usedNow) nor live in the current tree (liveKeys) for the
// texture-OOM recovery round. Releases are deferred so in-flight submissions
// keep sampling valid views; misses replay vector content on next blit.
func (c *PictureTextureCache) PurgeEvictable() (freed int64) {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for {
		victim := c.purgeVictimLocked()
		if victim == 0 {
			return freed
		}
		if e := c.entries[victim]; e != nil {
			c.releaseEntryLocked(e)
		}
		delete(c.entries, victim)
		c.Evictions++
		freed++
	}
}

// purgeVictimLocked picks the oldest entry outside the in-flight set and the
// live tree. 0 means only live/in-flight entries remain (kept: evicting them
// would force immediate re-records without relieving steady pressure).
func (c *PictureTextureCache) purgeVictimLocked() uint64 {
	var victim uint64
	var oldest uint64 = ^uint64(0)
	for id, e := range c.entries {
		if e == nil {
			continue
		}
		if _, used := c.usedNow[id]; used {
			continue
		}
		if c.isLive(id) {
			continue
		}
		if e.lastUse < oldest {
			oldest = e.lastUse
			victim = id
		}
	}
	return victim
}

// blit draws the cached texture for id at the current CTM (layer-local
// position). Returns false when the texture is absent (caller replays vector).
func (c *PictureTextureCache) blit(id uint64) bool {
	if c == nil || id == 0 || c.dc == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[id]
	if e == nil || e.contentSlot < 0 {
		return false // cache miss: caller replays vector
	}
	s := &e.slots[e.contentSlot]
	if s.view.IsNil() {
		return false
	}
	c.dc.DrawGPUTexture(s.view, float64(e.off.X), float64(e.off.Y), s.w, s.h)
	if os.Getenv("WR_RESIZE_DBG") == "1" {
		fmt.Fprintf(os.Stderr, "DBG blit id=%d w=%d h=%d off=%v\n", id, s.w, s.h, e.off)
	}
	e.lastUse = c.stamp
	c.usedNow[id] = struct{}{}
	c.FrameSkip.Add(1)
	return true
}

// Bounds returns the last recorded geometry bounds for id.
func (c *PictureTextureCache) Bounds(id uint64) image.Rectangle {
	if c == nil {
		return image.Rectangle{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[id]; e != nil {
		return e.bounds
	}
	return image.Rectangle{}
}

// Has reports whether a live texture entry exists for id.
func (c *PictureTextureCache) Has(id uint64) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[id]
	return e != nil && e.contentSlot >= 0 && !e.slots[e.contentSlot].view.IsNil()
}

// RecordedThisFrame reports whether id was re-recorded in the current frame
// (recordedIn matches the current recordFrame). Used to emit damage rects for
// exactly the layers whose textures were refreshed — blits do NOT count.
func (c *PictureTextureCache) RecordedThisFrame(id uint64) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[id]
	return e != nil && e.recordedIn == c.recordFrame
}

// Len returns the number of live texture entries (tests).
func (c *PictureTextureCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// RecordForTest is the exported record for unit tests (no-GPU degrades).
func (c *PictureTextureCache) RecordForTest(id uint64, pic *Picture) (image.Rectangle, bool) {
	return c.record(id, pic)
}

// TexturedStats mirrors RasterStats for the textured composite path.
type TexturedStats struct {
	RasterLayerCount  int
	SkippedLayerCount int
	ReplayedOps       int
	// FiltersApplied counts color/image filter layers applied this frame
	// (isolated vector fallback path; R20 filter_layer_count).
	FiltersApplied int
	// ShellSkip / ShellRerecord are the shell-partition portions of this
	// frame's blits / texture re-records (R21 shell/content layering).
	ShellSkip     int64
	ShellRerecord int64
	// DamageRects are the dirty layer geometry rects (logical coords) for
	// PresentFrameDamageRects / TrackDamageRect.
	DamageRects []image.Rectangle
}

// CompositeFramePacketTextured composites a packet via the retained texture
// path: dirty PictureLayers re-record into cached offscreen textures (phase 1,
// same CTM/clip walk as composite so picture ops land correctly), then the
// tree is composited with texture blits only (phase 2) so the frame is GPU
// blit-only and damage rects (dirty layer bounds) drive LoadOpLoad scissor.
//
// Layers without a cacheable texture (CacheKey==0, no GPU, filter/backdrop
// layers) fall back to direct vector replay in phase 2 — the frame may then
// leave blit-only (honest degradation: MSAA full clear, content still correct).
func CompositeFramePacketTextured(pkt *FramePacket, dc *render.Context, tex *PictureTextureCache) TexturedStats {
	st := TexturedStats{}
	if pkt == nil || dc == nil || tex == nil {
		return st
	}
	tex.BeginFrame()
	if tex.filterCache != nil {
		tex.filterCache.BeginFrame()
	}
	dirty := map[uint64]struct{}{}
	for _, id := range pkt.DirtyLayerIDs {
		dirty[id] = struct{}{}
	}
	// Shell partitioning (R21): shell-tagged boundaries report their
	// skip/rerecord separately from the totals.
	var frameShellSkip, frameShellRerecord atomic.Int64

	// Phase 1: re-record every dirty / NeedsRaster PictureLayer into its
	// texture, walking the same CTM/clip chain as the composite below so
	// layer-local picture ops land at the correct surface position. Layers
	// without a cached texture yet (first retained frame after a warm-up /
	// resize full paint, or cache eviction) re-record too — otherwise the
	// phase-2 fallback would vector-replay them every frame (blit-only lost).
	// Under a pure-translate CTM (no clip / rotate / scale) a picture with
	// geometry records into a bounds-sized texture (recordLocal) instead of a
	// full-surface one — that is what keeps a big grid within the GPU's
	// texture budget.
	// underShell tracks whether the current walk path is inside a
	// Shell-tagged boundary — re-records/blits there count into the shell
	// partition (R21) instead of the main totals only.
	//
	// Restrictive subtrees (rotation / non-1 scale, tracked by the noTex
	// counter below) never touch the texture cache: the cache records
	// axis-aligned bounds textures and blits them 1:1, which cannot
	// represent rotated/scaled content (double transform / cropped shards).
	// Flutter's raster cache makes the same decision at the same place —
	// caching is decided while walking the layer tree, and a transform that
	// is not a pure 2D translation disqualifies the subtree (Engine
	// RasterCache::CanRasterCachePicture). Those layers vector-replay in
	// phase 2 under the live CTM, which is always correct.
	// Clip-only chains stay CACHEABLE: a clip does not bend coordinates
	// (the CTM stays a pure translate), so recordLocalWith's translation
	// cancel is exact and the composite-time clip crops the bounds texture
	// back — this keeps viewport-clipped list cells on cheap textures (C4).
	var rasterWalk func(l Layer, noTex int, underShell bool)
	rasterWalk = func(l Layer, noTex int, underShell bool) {
		if l == nil {
			return
		}
		pl, isPic := l.(*PictureLayer)
		if bl, isBnd := l.(*BoundaryLayer); isBnd && bl.Shell {
			underShell = true
		}
		pushCtx := pushCompositeCTM(dc, l)
		if isPic && noTex == 0 {
			_, dirtyID := dirty[pl.LayerID()]
			if pl.CacheKey != 0 && (dirtyID || pl.NeedsRaster || !tex.Has(pl.CacheKey)) {
				b := pl.Picture.Bounds
				if b.Empty() {
					if tb, ok := tex.measureTextBounds(&pl.Picture); ok {
						b = tb
					}
				}
				// OnPaint layers (RasterExtra) fall back to their declared
				// paint bounds so they still get cheap bounds-sized textures.
				if b.Empty() && !pl.ExtraBounds.Empty() {
					b = pl.ExtraBounds
				}
				var ok bool
				// Bounds-sized recordLocal also under clip/rotate CTM
				// (restr > 0): the texture only covers the picture geometry;
				// the composite-time scissor/clip crops it back, identical to
				// what a full-surface texture would show. This keeps scrolled
				// list cells (viewport clip → restr=1) off full-window
				// textures — 200+ of those exhaust GPU memory (C4 measured
				// "Not enough memory left" + endless shell re-records).
				if !b.Empty() {
					_, ok = tex.recordLocalWith(pl.CacheKey, &pl.Picture, b, pl.RasterExtra)
				} else {
					_, ok = tex.recordWith(pl.CacheKey, &pl.Picture, pl.RasterExtra)
				}
				if ok {
					st.RasterLayerCount++
					if underShell {
						frameShellRerecord.Add(1)
						if os.Getenv("WR_SHELL_DBG") == "1" {
							fmt.Fprintf(os.Stderr, "SHELLDBG rr id=%d dirty=%v has=%v t=%v\n",
								pl.CacheKey, dirtyID, tex.Has(pl.CacheKey), time.Now().Format("15:04:05.000"))
						}
					}
				}
				pl.NeedsRaster = false
			}
		}
		for _, ch := range l.Children() {
			rasterWalk(ch, noTex+nonTranslating(l), underShell)
		}
		popCompositeCTM(dc, pushCtx)
	}
	rasterWalk(pkt.Root, 0, false)
	rasterWalk(pkt.Overlay, 0, false)

	// Phase 2: composite paint order — texture blits for cached pictures,
	// vector replay fallback otherwise, attribute layers via canvas state.
	// Damage rects for re-recorded layers are emitted here under the composite
	// CTM (bounds are layer-local; TrackDamageRect needs surface coords).
	// Restrictive subtrees (noTex > 0: a rotation / non-1 scale ancestor)
	// always vector-replay (phase 1 never textured them); their damage rect
	// is the full four-corner CTM transform of the picture bounds so a
	// rotated layer dirties its rotated footprint.
	var compositeWalk func(l Layer, noTex int, underShell bool)
	compositeWalk = func(l Layer, noTex int, underShell bool) {
		if l == nil {
			return
		}
		if bl, isBnd := l.(*BoundaryLayer); isBnd && bl.Shell {
			underShell = true
		}
		if pl, ok := l.(*PictureLayer); ok {
			if len(pl.Picture.Ops) > 0 || pl.RasterExtra != nil {
				blitOK := false
				if noTex == 0 && pl.CacheKey != 0 {
					if underShell && tex.blit(pl.CacheKey) {
						frameShellSkip.Add(1)
						blitOK = true
					} else {
						blitOK = tex.blit(pl.CacheKey)
					}
				}
				if !blitOK {
					// cache miss / no cache key / non-translating ancestor:
					// vector replay fallback
					n := pl.Picture.OpCount()
					pl.Picture.Replay(dc)
					st.ReplayedOps += n
				}
				// Re-recorded this frame → the layer's region changed.
				if pl.CacheKey != 0 && tex.RecordedThisFrame(pl.CacheKey) {
					b := tex.Bounds(pl.CacheKey)
					if !b.Empty() {
						st.DamageRects = append(st.DamageRects, transformBounds(dc, b))
					}
				}
				// Non-translating ancestor (vector replay): the picture's
				// rotated / scaled footprint is the four-corner CTM transform
				// of its bounds — an axis-aligned bounds transform would
				// under-dirty the corners and leave stale shards on screen.
				if noTex > 0 && !pl.Picture.Bounds.Empty() {
					st.DamageRects = append(st.DamageRects,
						transformBounds4(dc, pl.Picture.Bounds))
				}
			}
			return // leaf
		}
		pushCtx := pushCompositeCTM(dc, l)
		switch t := l.(type) {
		case *ContainerLayer:
			for _, ch := range l.Children() {
				compositeWalk(ch, noTex, underShell)
			}
		case *OffsetLayer, *BoundaryLayer, *ClipRectLayer, *ClipRRectLayer,
			*TransformLayer:
			r := noTex + nonTranslating(l)
			for _, ch := range l.Children() {
				compositeWalk(ch, r, underShell)
			}
		case *OpacityLayer:
			op := t.Opacity
			if op < 0 {
				op = 0
			}
			if op > 1 {
				op = 1
			}
			if op >= 1-1e-9 {
				for _, ch := range l.Children() {
					compositeWalk(ch, noTex, underShell)
				}
				break
			}
			if op <= 1e-9 {
				break
			}
			// Normal opacity group is a paint-alpha fast path (no RT, no
			// vector draws) — keeps the frame blit-only.
			dc.PushLayer(render.BlendNormal, op)
			for _, ch := range l.Children() {
				compositeWalk(ch, noTex, underShell)
			}
			dc.PopLayer()
		case *ColorFilterLayer, *ImageFilterLayer:
			// R20 filtered-subtree result cache: unchanged frames blit the
			// previous result (DrawImage genID keeps the GPU image-cache
			// entry warm → blit-only); only parameter changes or dirty
			// subtrees re-pay the CPU filter pass. Transform subtrees keep
			// the honest per-frame isolated path (bounds under rotation is
			// not approximated here).
			if fc := tex.filterCache; fc != nil && l.LayerID() != 0 {
				if cf, ok := l.(*ColorFilterLayer); ok && cf.CacheKey != 0 && !subtreeHasTransform(l) {
					key := cf.CacheKey
					fp := filterFingerprint(l, dirty, fc.seed)
					if e := fc.get(key, fp); e != nil {
						dc.DrawImage(e.buf, float64(e.offX), float64(e.offY))
						break
					}
					if b, okB := filterSubtreeBounds(l); okB {
						const pad = 16
						w, h := b.Dx()+2*pad, b.Dy()+2*pad
						tmp := render.NewContext(w, h)
						tmp.Translate(float64(-b.Min.X+pad), float64(-b.Min.Y+pad))
						fst := CompositeStats{}
						for _, ch := range l.Children() {
							compositeLayer(ch, tmp, &fst)
						}
						tmp.ApplyColorMatrix(cf.Matrix)
						var buf *render.ImageBuf
						if tmp.ExportImageBuf(&buf) {
							fc.put(key, fp, buf, b.Min.X-pad, b.Min.Y-pad, w, h)
							dc.DrawImage(buf, float64(b.Min.X-pad), float64(b.Min.Y-pad))
							st.FiltersApplied += fst.FiltersApplied + 1
							break
						}
						// Export failed: fall through to the isolated path.
					}
				} else if imf, ok := l.(*ImageFilterLayer); ok && imf.CacheKey != 0 && !subtreeHasTransform(l) {
					key := imf.CacheKey
					fp := filterFingerprint(l, dirty, fc.seed)
					if e := fc.get(key, fp); e != nil {
						dc.DrawImage(e.buf, float64(e.offX), float64(e.offY))
						break
					}
					if b, okB := filterSubtreeBounds(l); okB {
						const pad = 16
						w, h := b.Dx()+2*pad, b.Dy()+2*pad
						tmp := render.NewContext(w, h)
						tmp.Translate(float64(-b.Min.X+pad), float64(-b.Min.Y+pad))
						fst := CompositeStats{}
						for _, ch := range l.Children() {
							compositeLayer(ch, tmp, &fst)
						}
						if imf.BlurRadius > 0 {
							tmp.ApplyBlur(imf.BlurRadius)
						}
						var buf *render.ImageBuf
						if tmp.ExportImageBuf(&buf) {
							fc.put(key, fp, buf, b.Min.X-pad, b.Min.Y-pad, w, h)
							dc.DrawImage(buf, float64(b.Min.X-pad), float64(b.Min.Y-pad))
							st.FiltersApplied += fst.FiltersApplied + 1
							break
						}
					}
				}
			}
			// Filter layers on the cache-miss / unsupported path, plus
			// backdrop / unknown layers: isolated vector path (honest
			// blit-only fallback per layer).
			fst := CompositeStats{}
			compositeLayer(l, dc, &fst)
			st.FiltersApplied += fst.FiltersApplied
		default:
			// Unknown layer kinds: isolated vector path.
			fst := CompositeStats{}
			compositeLayer(l, dc, &fst)
			st.FiltersApplied += fst.FiltersApplied
		}
		popCompositeCTM(dc, pushCtx)
	}
	compositeWalk(pkt.Root, 0, false)
	compositeWalk(pkt.Overlay, 0, false)
	st.ShellSkip = frameShellSkip.Load()
	st.ShellRerecord = frameShellRerecord.Load()
	return st
}

// nonTranslating reports whether a layer applies a NON-translation transform
// (rotation, or scale ≠ 1) to its subtree. Such subtrees are excluded from
// the texture cache entirely: the cache records axis-aligned bounds textures
// and blits them 1:1, which cannot represent rotated/scaled content — the
// pre-C8 double-transform / cropped-shard bug. Flutter's raster cache makes
// the same call at the same place (RasterCache::CanRasterCachePicture).
// Clips are NOT in this set: a clip does not bend coordinates, so bounds
// recording is exact; their mid-frame flush safety is owned by the gpu-side
// offscreen pass (BeginOffscreenPass), not by refusing to cache (the earlier
// clip noTextureCache workaround cost viewport-list blit throughput).
func nonTranslating(l Layer) int {
	switch t := l.(type) {
	case *TransformLayer:
		if t.Rotation != 0 {
			return 1
		}
		sx, sy := t.EffectiveScale()
		if sx != 1 || sy != 1 {
			return 1
		}
	}
	return 0
}

// restrictive reports whether a layer imposes a clip / rotation / scale —
// i.e. whether its subtree must record into a full-surface texture (pure
// translate is required for bounds-sized recordLocal). Retained for the
// full-window-texture sizing decision in recordWith callers; the cache
// EXCLUSION decision is nonTranslating() above.
func restrictive(l Layer) int {
	switch t := l.(type) {
	case *ClipRectLayer:
		if t.W > 0 && t.H > 0 {
			return 1
		}
	case *ClipRRectLayer:
		if t.W > 0 && t.H > 0 {
			return 1
		}
	case *TransformLayer:
		if t.Rotation != 0 {
			return 1
		}
		sx, sy := t.EffectiveScale()
		if sx != 1 || sy != 1 {
			return 1
		}
	}
	return 0
}

// pushCompositeCTM applies the layer's canvas-state effect (translate / clip /
// transform) for the textured walks. Returns a restore func. Leaf layers apply
// nothing.
func pushCompositeCTM(dc *render.Context, l Layer) func() {
	if dc == nil || l == nil {
		return func() {}
	}
	switch t := l.(type) {
	case *OffsetLayer:
		dc.Push()
		dc.Translate(t.DX, t.DY)
		return dc.Pop
	case *BoundaryLayer:
		dc.Push()
		dc.Translate(t.DX, t.DY)
		return dc.Pop
	case *ClipRectLayer:
		if t.W > 0 && t.H > 0 {
			dc.Push()
			dc.ClipRect(t.X, t.Y, t.W, t.H)
			return dc.Pop
		}
	case *ClipRRectLayer:
		if t.W > 0 && t.H > 0 {
			dc.Push()
			if t.Radius > 0 {
				dc.ClipRoundRect(t.X, t.Y, t.W, t.H, t.Radius)
			} else {
				dc.ClipRect(t.X, t.Y, t.W, t.H)
			}
			return dc.Pop
		}
	case *TransformLayer:
		sx, sy := t.EffectiveScale()
		dc.Push()
		if t.TX != 0 || t.TY != 0 {
			dc.Translate(t.TX, t.TY)
		}
		// Pivot rotation/scale around the subtree center (CX,CY) — identical
		// to RenderTransform.Paint's vector path. Rotating about the layer
		// origin instead makes retained frames spin around the top-left
		// corner while vector repaints (post-resize) spin around the center.
		if t.Rotation != 0 || sx != 1 || sy != 1 {
			dc.Translate(t.CX, t.CY)
			if t.Rotation != 0 {
				dc.Rotate(t.Rotation)
			}
			if sx != 1 || sy != 1 {
				dc.Scale(sx, sy)
			}
			dc.Translate(-t.CX, -t.CY)
		}
		return dc.Pop
	}
	return func() {}
}

// popCompositeCTM restores state after a pushCompositeCTM call.
func popCompositeCTM(dc *render.Context, restore func()) {
	if restore != nil {
		restore()
	}
}

// transformBounds maps a layer-local rect through the current CTM into logical
// surface coordinates (TrackDamageRect expects logical, the CTM is device).
func transformBounds(dc *render.Context, b image.Rectangle) image.Rectangle {
	if dc == nil || b.Empty() {
		return b
	}
	scale := dc.DeviceScale()
	if scale <= 0 {
		scale = 1
	}
	var minX, minY, maxX, maxY float64
	first := true
	apply := func(x, y float64) {
		x, y = dc.TransformPoint(x, y)
		x /= scale
		y /= scale
		if first {
			minX, maxX, minY, maxY = x, x, y, y
			first = false
			return
		}
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	apply(float64(b.Min.X), float64(b.Min.Y))
	apply(float64(b.Max.X), float64(b.Min.Y))
	apply(float64(b.Max.X), float64(b.Max.Y))
	apply(float64(b.Min.X), float64(b.Max.Y))
	return image.Rect(int(minX), int(minY), int(maxX), int(maxY))
}

// transformBounds4 is transformBounds with an explicit four-corner transform
// (same math today, kept separate so call sites document WHY the damage rect
// is computed: rotated/scaled replay footprints need all four corners mapped,
// not just bounds corners — which are identical here but semantically
// distinct from a blit-bounds rect).
func transformBounds4(dc *render.Context, b image.Rectangle) image.Rectangle {
	return transformBounds(dc, b)
}

// DebugDumpEntries logs every cache entry (id, size, offset) — C8 live-probe
// diagnostics only, guarded by the caller.
func (c *PictureTextureCache) DebugDumpEntries(tag string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, e := range c.entries {
		if e == nil {
			continue
		}
		s := &e.slots[e.contentSlot]
		fmt.Fprintf(os.Stderr, "%s: id=%d tex=%dx%d off=(%d,%d) recordedIn=%d\n",
			tag, id, s.w, s.h, e.off.X, e.off.Y, e.recordedIn)
	}
}

// DebugDumpEntryTexture renders one cache entry's current content into a PNG
// (C8 live-probe diagnostics).
func (c *PictureTextureCache) DebugDumpEntryTexture(dc *render.Context, path string, id uint64) {
	if c == nil || dc == nil {
		return
	}
	c.mu.Lock()
	e := c.entries[id]
	if e == nil {
		c.mu.Unlock()
		fmt.Fprintf(os.Stderr, "dump %s: id=%d not in cache\n", path, id)
		return
	}
	s := &e.slots[e.contentSlot]
	w, h := s.w, s.h
	view := s.view
	c.mu.Unlock()
	out := render.NewContext(w, h)
	defer out.Close()
	out.ClearWithColor(render.Black)
	out.DrawGPUTexture(view, 0, 0, w, h)
	_ = out.FlushGPU()
	img := out.Image()
	cc := map[[3]uint32]int{}
	for y := 0; y < h; y += 2 {
		for x := 0; x < w; x += 2 {
			r, g, b, _ := img.At(x, y).RGBA()
			cc[[3]uint32{r >> 8, g >> 8, b >> 8}]++
		}
	}
	top := 0
	var topc [3]uint32
	for c, n := range cc {
		if n > top {
			top = n
			topc = c
		}
	}
	fmt.Fprintf(os.Stderr, "dump %s: id=%d %dx%d dominant rgb=%d,%d,%d count=%d\n",
		path, id, w, h, topc[0], topc[1], topc[2], top)
}

// DebugEntrySlot exposes one entry's current content slot dimensions for
// GPU-pixel diagnostics (gpupixel test package). Returns ok=false when the
// entry is absent.
func (c *PictureTextureCache) DebugEntrySlot(id uint64) (w, h int, ok bool) {
	if c == nil {
		return 0, 0, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[id]
	if e == nil || e.contentSlot < 0 {
		return 0, 0, false
	}
	s := &e.slots[e.contentSlot]
	return s.w, s.h, !s.view.IsNil()
}
