//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package gpu

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

// Budget limits come from the single declaration table (P2.4b 预算统一,
// render.DefaultBudget(BudgetImageCache)): MaxEntries=128 textures,
// MaxBytes=64 MiB, fair-share floors 16 entries / 8 MiB. The vars keep the
// historic names so call sites are unchanged.
var (
	defaultImageCacheBudget      = render.DefaultBudget(render.BudgetImageCache).MaxEntries
	defaultImageCacheBudgetBytes = render.DefaultBudget(render.BudgetImageCache).MaxBytes
	minImageCacheBudget          = render.DefaultBudget(render.BudgetImageCache).MinEntries
	minImageCacheBudgetBytes     = render.DefaultBudget(render.BudgetImageCache).MinBytes
)

// imageCacheEntry holds a GPU texture and view for a cached image.
type imageCacheEntry struct {
	texture hal.Texture
	view    hal.TextureView
	width   int
	height  int
	bytes   int64
	gen     uint64 // LRU generation counter

	// srcGen is the content generation these pixels came from (never the
	// derived key), and unwatch drops the source watcher registered for it.
	// Together they let a rewritten buffer reclaim exactly its own textures.
	srcGen  uint64
	unwatch func()
}

// ImageCache manages GPU textures for image patterns. Images are uploaded
// on first use and reused on subsequent frames. The cache is keyed by the
// content generation ID (ADR-014) — one process-global allocator (see
// image.NextContentGenID) hands out those IDs, so a Pixmap and an ImageBuf can
// never be handed the same number.
//
// The ID identifies the *pixels*, not the buffer: rewriting a buffer in place
// retires its ID and hands out a new one. Because a retired ID is never looked
// up again, the texture cached under it would otherwise sit in the budget as a
// zombie until LRU got to it, so every entry registers a watcher on its source
// and the source tells the cache when its ID is retired (Skia does the same with
// SkPixelRef::addGenIDChangeListener / notifyAddedToCache).
//
// This follows the enterprise pattern:
//   - Skia: GrResourceCache keyed by SkPixelRef::getGenerationID()
//   - Vello: image_cache keyed by peniko::Blob::id() (AtomicU64)
//   - femtovg: SlotMap with generational index
//
// The cache is NOT thread-safe — entries, LRU and budgets are touched only from
// the render path, which is serialized per GPURenderContext. Generation
// notifications can arrive from another thread, so the watcher only appends the
// retired ID to a mutex-guarded set; the render path drains it (drainRetired).
type ImageCache struct {
	device hal.Device
	queue  hal.Queue

	entries     map[uint64]*imageCacheEntry // keyed by content generation (ImageBuf/Pixmap)
	budget      int
	budgetBytes int64
	// explicitBudgets marks SetBudgets overrides: explicit per-cache tuning
	// wins over the multi-window fair share below.
	explicitBudgets bool
	usedBytes       int64
	gen             uint64 // global LRU generation counter

	// Stats
	hits             uint64
	misses           uint64
	uploads          uint64
	uploadBytes      uint64
	lastUploadBytes  int64
	evictions        uint64
	ephemeralUploads uint64
	// creates counts native texture allocations this cache performed. An
	// animation that recycles instead of allocating keeps creates flat after
	// the first frame; a rising count is texture churn.
	creates uint64

	// gen==0 textures live for one frame then must be released; entries retired
	// by eviction / size-replace during encode also outlive their map slot until
	// the current submission completes. Both paths hand the native pair to
	// retire when set (the session defers release to a submission-completion
	// point through the res registry); without a hook the pair is destroyed
	// immediately (tests / standalone caches).
	retire func(tex hal.Texture, view hal.TextureView)

	// retired holds content IDs whose textures must go, set by watchers on any
	// thread and drained by the render path. bySrc indexes entries per source ID
	// (several entries can share one source: derived keys and tile grids).
	retiredMu   sync.Mutex
	retired     map[uint64]struct{}
	bySrc       map[uint64]map[uint64]struct{}
	retiredGens uint64

	// destroyed latches session teardown; a destroyed cache purges to zero
	// without touching native resources (multiwindow 1.3 purge chain).
	// Atomic: Destroy runs on teardown while purges run on the raster thread.
	destroyed atomic.Bool
}

// stagingScratch reuses CPU packing buffers for non-contiguous image uploads.
var imageStagingPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 64*1024)
		return &b
	},
}

func acquireImageStaging(n int) *[]byte {
	p := imageStagingPool.Get().(*[]byte)
	if cap(*p) < n {
		*p = make([]byte, n)
	} else {
		*p = (*p)[:n]
	}
	return p
}

func releaseImageStaging(p *[]byte) {
	if p == nil {
		return
	}
	*p = (*p)[:0]
	imageStagingPool.Put(p)
}

// NewImageCache creates a new image texture cache with the given device and queue.
func NewImageCache(device hal.Device, queue hal.Queue) *ImageCache {
	c := &ImageCache{
		device:      device,
		queue:       queue,
		entries:     make(map[uint64]*imageCacheEntry),
		retired:     make(map[uint64]struct{}),
		bySrc:       make(map[uint64]map[uint64]struct{}),
		budget:      defaultImageCacheBudget,
		budgetBytes: defaultImageCacheBudgetBytes,
	}
	render.RegisterPurgeEvictable("image-cache", c)
	return c
}

// SetRetireHook installs the deferred-release outlet for textures leaving the
// cache. The hook must guarantee the pair stays alive until the submission
// referencing it completes. Without a hook the pair is destroyed immediately.
func (c *ImageCache) SetRetireHook(fn func(tex hal.Texture, view hal.TextureView)) {
	if c == nil {
		return
	}
	c.retire = fn
}

// retireEntry hands one entry's native pair to the retire hook (immediate
// destroy when no hook is installed).
func (c *ImageCache) retireEntry(entry *imageCacheEntry) {
	if entry == nil {
		return
	}
	if c.retire != nil {
		c.retire(entry.texture, entry.view)
		return
	}
	if entry.view != nil {
		entry.view.Destroy()
	}
	if entry.texture != nil {
		entry.texture.Destroy()
	}
}

// SetBudgets updates entry and byte soft limits (tests/tuning).
// Explicit values win over the multi-window fair share.
func (c *ImageCache) SetBudgets(entries int, bytes int64) {
	if c == nil {
		return
	}
	if entries > 0 {
		c.budget = entries
		c.explicitBudgets = true
	}
	if bytes > 0 {
		c.budgetBytes = bytes
		c.explicitBudgets = true
	}
}

// effBudget returns the binding entry cap: explicit SetBudgets wins,
// otherwise the default divided across live shared-device windows (R0-5
// fair share — N per-window 128-entry caches would otherwise hold N× the
// process budget). Single-window behavior is unchanged.
func (c *ImageCache) effBudget() int {
	return c.effBudgetFor(render.SharedWindowCount())
}

// effBudgetFor is effBudget with the window count injected (test seam).
func (c *ImageCache) effBudgetFor(w int) int {
	if c != nil && c.explicitBudgets && c.budget > 0 {
		return c.budget
	}
	if w > 1 {
		return int(render.FairSplit(int64(defaultImageCacheBudget), int64(w), int64(minImageCacheBudget)))
	}
	if c != nil && c.budget > 0 {
		return c.budget
	}
	return defaultImageCacheBudget
}

// effBudgetBytes returns the binding byte cap (same fair-share rule).
func (c *ImageCache) effBudgetBytes() int64 {
	return c.effBudgetBytesFor(render.SharedWindowCount())
}

// effBudgetBytesFor is effBudgetBytes with the window count injected.
func (c *ImageCache) effBudgetBytesFor(w int) int64 {
	if c != nil && c.explicitBudgets && c.budgetBytes > 0 {
		return c.budgetBytes
	}
	if w > 1 {
		return render.FairSplit(defaultImageCacheBudgetBytes, int64(w), minImageCacheBudgetBytes)
	}
	if c != nil && c.budgetBytes > 0 {
		return c.budgetBytes
	}
	return defaultImageCacheBudgetBytes
}

// GetOrUpload returns the cached GPU texture view for the given image data,
// uploading it if not already cached. The cache key is ImageDrawCommand.GenerationID
// — the content generation handed out by image.NextContentGenID, never a pointer
// and never a per-type counter, so two different pixel sources cannot share a key.
func (c *ImageCache) GetOrUpload(cmd *ImageDrawCommand) (hal.TextureView, error) {
	if len(cmd.PixelData) == 0 {
		return nil, fmt.Errorf("empty pixel data")
	}

	c.drainRetired()

	key := cmd.GenerationID
	if key == 0 {
		// No generation ID — upload without long-term caching (temporary data).
		entry, err := c.uploadImage(cmd)
		if err != nil {
			return nil, err
		}
		c.uploads++
		c.ephemeralUploads++
		c.retireEntry(entry)
		return entry.view, nil
	}

	if entry, ok := c.entries[key]; ok {
		// Size mismatch with same gen should not happen; re-upload defensively.
		if entry.width == cmd.ImgWidth && entry.height == cmd.ImgHeight {
			c.gen++
			entry.gen = c.gen
			c.hits++
			return entry.view, nil
		}
		// Generation reused with different dimensions — replace.
		c.removeEntry(key, entry)
	}

	c.misses++
	c.evictIfNeeded(int64(cmd.ImgWidth)*int64(cmd.ImgHeight)*4 + 1)

	entry, err := c.uploadImage(cmd)
	if err != nil {
		return nil, err
	}
	c.uploads++

	c.gen++
	entry.gen = c.gen
	c.entries[key] = entry
	c.usedBytes += entry.bytes
	c.trackSource(key, entry, cmd)

	return entry.view, nil
}

// trackSource records which content generation an entry holds and watches the
// source for its retirement. A command whose pixels are already behind the
// source's current generation is retired at once: it drew this frame but nothing
// will ever ask for that ID again.
func (c *ImageCache) trackSource(key uint64, entry *imageCacheEntry, cmd *ImageDrawCommand) {
	if c == nil || entry == nil || cmd.Src == nil {
		return
	}
	entry.srcGen = cmd.SrcGen
	if entry.srcGen == 0 {
		return
	}
	entry.unwatch = cmd.Src.AddGenIDWatcher(c.noteRetiredGen)
	if src := cmd.Src.GenerationID(); src != entry.srcGen {
		c.noteRetiredGen(entry.srcGen)
	}
	c.noteWatched(entry.srcGen, key)
}

// untrackSource drops the watcher and the per-source index before an entry goes
// away, so a retired ID cannot leave a stale registration behind.
func (c *ImageCache) untrackSource(key uint64, entry *imageCacheEntry) {
	if entry == nil {
		return
	}
	if entry.unwatch != nil {
		entry.unwatch()
		entry.unwatch = nil
	}
	if entry.srcGen == 0 {
		return
	}
	if bucket := c.bySrc[entry.srcGen]; bucket != nil {
		delete(bucket, key)
		if len(bucket) == 0 {
			delete(c.bySrc, entry.srcGen)
		}
	}
	entry.srcGen = 0
}

// noteRetiredGen runs on the notifying thread: it only records the retired
// content ID. The cache itself is render-thread-owned (see type ImageCache).
func (c *ImageCache) noteRetiredGen(oldGenID uint64) {
	if c == nil || oldGenID == 0 {
		return
	}
	c.retiredMu.Lock()
	if _, ok := c.retired[oldGenID]; !ok {
		c.retired[oldGenID] = struct{}{}
		c.retiredGens++
	}
	c.retiredMu.Unlock()
}

func (c *ImageCache) noteWatched(srcGen, key uint64) {
	bucket := c.bySrc[srcGen]
	if bucket == nil {
		bucket = make(map[uint64]struct{}, 1)
		c.bySrc[srcGen] = bucket
	}
	bucket[key] = struct{}{}
}

// drainRetired reclaims every texture whose content generation was retired since
// the last drain. Called on the render path at the head of an upload.
func (c *ImageCache) drainRetired() {
	c.retiredMu.Lock()
	if len(c.retired) == 0 {
		c.retiredMu.Unlock()
		return
	}
	gens := make([]uint64, 0, len(c.retired))
	for g := range c.retired {
		delete(c.retired, g)
		gens = append(gens, g)
	}
	c.retiredMu.Unlock()
	for _, g := range gens {
		for key := range c.bySrc[g] {
			if entry, ok := c.entries[key]; ok {
				c.removeEntry(key, entry)
			}
		}
	}
}


// Destroy releases all cached GPU textures and views.
func (c *ImageCache) Destroy() {
	if c == nil {
		return
	}
	render.UnregisterPurgeEvictable(c)
	c.destroyed.Store(true)
	for key, entry := range c.entries {
		c.untrackSource(key, entry)
		c.retireEntry(entry)
		delete(c.entries, key)
	}
	c.bySrc = make(map[uint64]map[uint64]struct{})
	c.retiredMu.Lock()
	c.retired = make(map[uint64]struct{})
	c.retiredMu.Unlock()
	c.usedBytes = 0
}

// PurgeEvictable drops every entry through the deferred-release path
// (in-flight frames keep sampling until submit completes) for the
// texture-OOM recovery round. Misses re-upload on next use. A destroyed
// cache frees nothing.
func (c *ImageCache) PurgeEvictable() (freed int64) {
	if c == nil || c.destroyed.Load() {
		return 0
	}
	for len(c.entries) > 0 {
		c.evictOldest()
		freed++
	}
	return freed
}

// uploadImage creates a GPU texture and uploads pixel data from an ImageDrawCommand.
func (c *ImageCache) uploadImage(cmd *ImageDrawCommand) (*imageCacheEntry, error) {
	w := cmd.ImgWidth
	h := cmd.ImgHeight
	if w == 0 || h == 0 {
		return nil, fmt.Errorf("empty image (%dx%d)", w, h)
	}
	c.creates++

	tex, err := c.device.CreateTexture(&hal.TextureDescriptor{
		Label:         "image_cache_tex",
		Size:          hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec // image dimensions fit uint32
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatRGBA8Unorm,
		Usage:         types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create image texture: %w", err)
	}

	view, err := c.device.CreateTextureView(tex, &hal.TextureViewDescriptor{
		Label:         "image_cache_view",
		Format:        types.TextureFormatRGBA8Unorm,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		tex.Destroy()
		return nil, fmt.Errorf("create image texture view: %w", err)
	}

	bytesPerRow := uint32(w * 4) //nolint:gosec // image width fits uint32
	stride := cmd.ImgStride
	if stride == 0 {
		stride = w * 4
	}

	var pixelData []byte
	var staging *[]byte
	need := w * h * 4
	if stride == w*4 {
		pixelData = cmd.PixelData[:need]
	} else {
		staging = acquireImageStaging(need)
		pixelData = *staging
		for row := 0; row < h; row++ {
			srcOff := row * stride
			dstOff := row * w * 4
			copy(pixelData[dstOff:dstOff+w*4], cmd.PixelData[srcOff:srcOff+w*4])
		}
	}

	if err := c.queue.WriteTexture(
		&hal.ImageCopyTexture{Texture: tex, MipLevel: 0},
		pixelData,
		&hal.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  bytesPerRow,
			RowsPerImage: uint32(h), //nolint:gosec // image height fits uint32
		},
		&hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec // image dimensions fit uint32
	); err != nil {
		if staging != nil {
			releaseImageStaging(staging)
		}
		view.Destroy()
		tex.Destroy()
		return nil, fmt.Errorf("upload image pixels: %w", err)
	}
	if staging != nil {
		releaseImageStaging(staging)
	}

	nbytes := int64(need)
	c.lastUploadBytes = nbytes
	c.uploadBytes += uint64(nbytes) //nolint:gosec // non-negative

	return &imageCacheEntry{
		texture: tex,
		view:    view,
		width:   w,
		height:  h,
		bytes:   nbytes,
	}, nil
}

func (c *ImageCache) removeEntry(key uint64, entry *imageCacheEntry) {
	if entry == nil {
		return
	}
	c.untrackSource(key, entry)
	// Hand the native pair to the retire outlet: this frame's already-encoded
	// commands may still reference the view, so release waits for the
	// submission-completion sync point.
	c.retireEntry(entry)
	c.usedBytes -= entry.bytes
	if c.usedBytes < 0 {
		c.usedBytes = 0
	}
	delete(c.entries, key)
	c.evictions++
}

// evictIfNeeded frees LRU entries until under entry and byte budgets and room for needBytes.
func (c *ImageCache) evictIfNeeded(needBytes int64) {
	for len(c.entries) >= c.effBudget() || (c.effBudgetBytes() > 0 && c.usedBytes+needBytes > c.effBudgetBytes()) {
		if len(c.entries) == 0 {
			return
		}
		c.evictOldest()
	}
}

// evictOldest removes the least recently used cache entry.
func (c *ImageCache) evictOldest() {
	var oldestKey uint64
	oldestGen := ^uint64(0)
	found := false
	for key, entry := range c.entries {
		if entry.gen < oldestGen {
			oldestGen = entry.gen
			oldestKey = key
			found = true
		}
	}
	if !found {
		return
	}
	if entry, ok := c.entries[oldestKey]; ok {
		c.removeEntry(oldestKey, entry)
	}
}

// ImageCacheStats returns cache statistics for diagnostics.
type ImageCacheStats struct {
	Entries          int
	Budget           int
	BudgetBytes      int64
	UsedBytes        int64
	Generations      uint64
	Hits             uint64
	Misses           uint64
	Uploads          uint64
	UploadBytes      uint64
	LastUploadBytes  int64
	Evictions        uint64
	EphemeralUploads uint64
	// RetiredGens counts content generations reported as retired; Watched counts
	// sources still being watched. Together they show that an animation reclaims
	// its own textures instead of leaking IDs.
	RetiredGens uint64
	Watched     int
	// Creates is the number of native textures this cache allocated.
	Creates uint64
}

// Stats returns cache statistics.
func (c *ImageCache) Stats() ImageCacheStats {
	if c == nil {
		return ImageCacheStats{}
	}
	return ImageCacheStats{
		Entries:          len(c.entries),
		Budget:           c.budget,
		BudgetBytes:      c.budgetBytes,
		UsedBytes:        c.usedBytes,
		Generations:      c.gen,
		Hits:             c.hits,
		Misses:           c.misses,
		Uploads:          c.uploads,
		UploadBytes:      c.uploadBytes,
		LastUploadBytes:  c.lastUploadBytes,
		Evictions:        c.evictions,
		EphemeralUploads: c.ephemeralUploads,
		Creates:          c.creates,
		RetiredGens:      c.retiredGens,
		Watched:          len(c.bySrc),
	}
}

// ResetStats clears hit/miss/upload counters (entries retained).
func (c *ImageCache) ResetStats() {
	if c == nil {
		return
	}
	c.hits = 0
	c.misses = 0
	c.uploads = 0
	c.uploadBytes = 0
	c.lastUploadBytes = 0
	c.evictions = 0
	c.ephemeralUploads = 0
}
