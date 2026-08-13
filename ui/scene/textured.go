package scene

import (
	"fmt"
	"image"
	"math"
	"os"
	"sync"
	"sync/atomic"

	"github.com/energye/gpui/render"
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
	// entries keyed by stable CacheKey.
	entries map[uint64]*pictureTextureEntry
	// stamp is a monotonically-increasing last-use counter for LRU eviction.
	stamp   uint64
	usedNow map[uint64]struct{}
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
		dc:      dc,
		max:     max,
		width:   w,
		height:  h,
		entries: make(map[uint64]*pictureTextureEntry),
		usedNow: make(map[uint64]struct{}),
	}
	if dc != nil {
		c.slotAlloc = defaultSlotAlloc
	}
	return c
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
	if len(c.deferred) > c.max*3 {
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
	c.drainDeferred(c.recordFrame)
}

// EndFrame evicts entries unused this frame and resets frame counters.
// Runs on the UI thread while the raster thread may still be compositing the
// previous frame — mutex-guarded (see type doc).
func (c *PictureTextureCache) EndFrame() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, e := range c.entries {
		if e == nil {
			delete(c.entries, id)
			continue
		}
		if _, used := c.usedNow[id]; !used {
			c.releaseEntryLocked(e)
			delete(c.entries, id)
		}
	}
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
			// no GPU (or alloc failure): keep the slot's stale content and let
			// callers fall back to direct vector replay this frame
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
// from font metrics (geometry ops have no tracked bounds, so text layers would
// otherwise always record full-window textures with an empty damage rect).
func (c *PictureTextureCache) measureTextBounds(pic *Picture) (image.Rectangle, bool) {
	if c == nil || c.dc == nil || pic == nil || pic.IsEmpty() {
		return image.Rectangle{}, false
	}
	var b image.Rectangle
	found := false
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
		m := face.Metrics()
		r := image.Rect(
			int(math.Floor(op.X)),
			int(math.Floor(op.Y-m.Ascent)),
			int(math.Ceil(op.X+face.Advance(op.Text))),
			int(math.Ceil(op.Y+m.Descent)),
		)
		if !found {
			b = r
			found = true
		} else {
			b = b.Union(r)
		}
	}
	return b, found
}

// evictForNew makes room for a new texture entry when the LRU cap is reached.
// Prefers evicting an entry unused this frame; falls back to the globally
// oldest. Always returns true (callers proceed with creation).
func (c *PictureTextureCache) evictForNew() bool {
	if c.max <= 0 || len(c.entries) < c.max {
		return true
	}
	var victim uint64
	var oldest uint64 = ^uint64(0)
	for id, e := range c.entries {
		if e == nil {
			continue
		}
		if _, used := c.usedNow[id]; used {
			continue
		}
		if e.lastUse < oldest {
			oldest = e.lastUse
			victim = id
		}
	}
	if victim == 0 {
		oldest = ^uint64(0)
		for id, e := range c.entries {
			if e == nil {
				continue
			}
			if e.lastUse < oldest {
				oldest = e.lastUse
				victim = id
			}
		}
	}
	if victim != 0 {
		if e := c.entries[victim]; e != nil {
			c.releaseEntryLocked(e)
		}
		delete(c.entries, victim)
	}
	return true
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
	dirty := map[uint64]struct{}{}
	for _, id := range pkt.DirtyLayerIDs {
		dirty[id] = struct{}{}
	}

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
	var rasterWalk func(l Layer, restr int)
	rasterWalk = func(l Layer, restr int) {
		if l == nil {
			return
		}
		pl, isPic := l.(*PictureLayer)
		pushCtx := pushCompositeCTM(dc, l)
		if isPic {
			_, dirtyID := dirty[pl.LayerID()]
			if pl.CacheKey != 0 && (dirtyID || pl.NeedsRaster || !tex.Has(pl.CacheKey)) {
				b := pl.Picture.Bounds
				if b.Empty() && restr == 0 {
					if tb, ok := tex.measureTextBounds(&pl.Picture); ok {
						b = tb
					}
				}
				// OnPaint layers (RasterExtra) fall back to their declared
				// paint bounds so they still get cheap bounds-sized textures.
				if b.Empty() && restr == 0 && !pl.ExtraBounds.Empty() {
					b = pl.ExtraBounds
				}
				var ok bool
				if restr == 0 && !b.Empty() {
					_, ok = tex.recordLocalWith(pl.CacheKey, &pl.Picture, b, pl.RasterExtra)
				} else {
					_, ok = tex.recordWith(pl.CacheKey, &pl.Picture, pl.RasterExtra)
				}
				if ok {
					st.RasterLayerCount++
				}
				pl.NeedsRaster = false
			}
		}
		for _, ch := range l.Children() {
			rasterWalk(ch, restr+restrictive(l))
		}
		popCompositeCTM(dc, pushCtx)
	}
	rasterWalk(pkt.Root, 0)
	rasterWalk(pkt.Overlay, 0)

	// Phase 2: composite paint order — texture blits for cached pictures,
	// vector replay fallback otherwise, attribute layers via canvas state.
	// Damage rects for re-recorded layers are emitted here under the composite
	// CTM (bounds are layer-local; TrackDamageRect needs surface coords).
	var compositeWalk func(l Layer)
	compositeWalk = func(l Layer) {
		if l == nil {
			return
		}
		if pl, ok := l.(*PictureLayer); ok {
			if len(pl.Picture.Ops) > 0 || pl.RasterExtra != nil {
				if pl.CacheKey == 0 || !tex.blit(pl.CacheKey) {
					// cache miss / no cache key: vector replay fallback
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
			}
			return // leaf
		}
		pushCtx := pushCompositeCTM(dc, l)
		switch t := l.(type) {
		case *ContainerLayer:
			for _, ch := range l.Children() {
				compositeWalk(ch)
			}
		case *OffsetLayer, *BoundaryLayer, *ClipRectLayer, *ClipRRectLayer,
			*TransformLayer:
			for _, ch := range l.Children() {
				compositeWalk(ch)
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
					compositeWalk(ch)
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
				compositeWalk(ch)
			}
			dc.PopLayer()
		default:
			// Filter / backdrop / unknown layers: isolated vector path
			// (honest blit-only fallback per layer).
			fst := CompositeStats{}
			compositeLayer(l, dc, &fst)
		}
		popCompositeCTM(dc, pushCtx)
	}
	compositeWalk(pkt.Root)
	compositeWalk(pkt.Overlay)
	return st
}

// restrictive reports whether a layer imposes a clip / rotation / scale —
// i.e. whether its subtree must record into a full-surface texture (pure
// translate is required for bounds-sized recordLocal).
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
		if t.Rotation != 0 {
			dc.Rotate(t.Rotation)
		}
		if sx != 1 || sy != 1 {
			dc.Scale(sx, sy)
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
