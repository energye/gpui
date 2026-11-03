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
	intImage "github.com/energye/gpui/render/internal/image"
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
// imageCacheKey is one cached texture's identity: the content generation whose
// pixels it holds, and how many mip levels it was created with. Level count is
// part of the key because a texture's mip chain is fixed at creation
// (TextureDescriptor.MipLevelCount) — the same picture drawn once with
// MipmapNone and once with MipmapLinear needs two entries, and neither may
// silently serve the other (P3.6 batch 2, D13).
type imageCacheKey struct {
	gen    uint64
	levels uint32
}

type imageCacheEntry struct {
	texture hal.Texture
	view    hal.TextureView
	width   int
	height  int
	bytes   int64
	gen     uint64 // LRU generation counter
	levels  uint32 // created mip level count (1 = single level)

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

	entries     map[imageCacheKey]*imageCacheEntry // keyed by (content generation, level count)
	budget      int
	budgetBytes int64
	// explicitBudgets marks SetBudgets overrides: explicit per-cache tuning
	// wins over the multi-window fair share below.
	explicitBudgets bool
	// statsMu protects the usedBytes/entries diagnostic snapshot read (D20
	// forwarding reads from other threads); the write side stays on the raster
	// thread, so the cache's single-thread operation contract is unchanged.
	statsMu   sync.Mutex
	usedBytes int64
	gen       uint64 // global LRU generation counter

	// Stats
	hits             atomic.Uint64
	misses           atomic.Uint64
	uploads          atomic.Uint64
	uploadBytes      atomic.Uint64
	lastUploadBytes  int64
	evictions        atomic.Uint64
	ephemeralUploads atomic.Uint64
	// creates counts native texture allocations this cache performed. An
	// animation that recycles instead of allocating keeps creates flat after
	// the first frame; a rising count is texture churn.
	creates atomic.Uint64

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
	retiredMu       sync.Mutex
	retired         map[uint64]struct{}
	bySrc           map[uint64]map[imageCacheKey]struct{}
	retiredGens     atomic.Uint64
	mipLevelUploads atomic.Uint64

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
		entries:     make(map[imageCacheKey]*imageCacheEntry),
		retired:     make(map[uint64]struct{}),
		bySrc:       make(map[uint64]map[imageCacheKey]struct{}),
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

	// Decide the level count first: it is part of the key, and the chain is
	// resolved once here so the upload and the budget agree on it.
	levels, chain, release := c.resolveLevels(cmd)
	if release != nil {
		defer release()
	}
	key := imageCacheKey{gen: cmd.GenerationID, levels: levels}

	if key.gen == 0 {
		// No generation ID — upload without long-term caching (temporary data).
		entry, err := c.uploadImage(cmd, levels, chain)
		if err != nil {
			return nil, err
		}
		c.uploads.Add(1)
		c.ephemeralUploads.Add(1)
		c.retireEntry(entry)
		return entry.view, nil
	}

	if entry, ok := c.entries[key]; ok {
		// Size mismatch with same gen should not happen; re-upload defensively.
		if entry.width == cmd.ImgWidth && entry.height == cmd.ImgHeight {
			c.gen++
			entry.gen = c.gen
			c.hits.Add(1)
			return entry.view, nil
		}
		// Generation reused with different dimensions — replace.
		c.removeEntry(key, entry)
	}

	c.misses.Add(1)
	c.evictIfNeeded(c.texelsFor(cmd.ImgWidth, cmd.ImgHeight, levels)*4 + 1)

	entry, err := c.uploadImage(cmd, levels, chain)
	if err != nil {
		return nil, err
	}
	c.uploads.Add(1)

	c.gen++
	entry.gen = c.gen
	c.entries[key] = entry
	c.statsMu.Lock()
	c.usedBytes += entry.bytes
	c.statsMu.Unlock()
	c.trackSource(key, entry, cmd)

	return entry.view, nil
}

// mipEntryCount counts live multi-level entries.
func (c *ImageCache) mipEntryCount() int {
	n := 0
	for _, e := range c.entries {
		if e.levels > 1 {
			n++
		}
	}
	return n
}

// texelsFor is the texture's whole mip area; one formula shared with the hal
// ledger (hal.MipChainTexels) so the render-side budget and the VRAM account
// charge the same bytes (D12).
func (c *ImageCache) texelsFor(w, h int, levels uint32) int64 {
	return int64(hal.MipChainTexels(uint32(w), uint32(h), levels)) //nolint:gosec // areas fit int64
}

// resolveLevels reports how many mip levels this draw needs and hands back the
// chain to upload from (release it when done).
//
// MipmapNone needs one level and no chain — that is the historic path and stays
// byte-identical. A mip mode needs the CPU chain; when there is no watchable
// source or the chain cannot be built, the draw degrades to one level rather
// than creating a multi-level texture whose deeper levels would be read as
// garbage by a mip-selecting sampler.
func (c *ImageCache) resolveLevels(cmd *ImageDrawCommand) (levels uint32, chain *intImage.MipmapChain, release func()) {
	levels = 1
	if !cmd.Mip.UsesChain() || cmd.Src == nil {
		return levels, nil, nil
	}
	want := hal.MipLevelCount(uint32(cmd.ImgWidth), uint32(cmd.ImgHeight))
	if want <= 1 {
		return levels, nil, nil
	}
	// The chain is built from an ImageBuf. A Pixmap-backed draw (the context's
	// own surface used as an image) keeps one level for now rather than
	// converting on every upload — registered as a follow-up in the batch
	// evidence, not silently dropped.
	buf, isBuf := cmd.Src.(*intImage.ImageBuf)
	if !isBuf {
		return levels, nil, nil
	}
	chain, release, ok := intImage.MipChainFor(buf)
	if !ok || chain == nil {
		return levels, nil, nil
	}
	have := uint32(chain.NumLevels()) //nolint:gosec // level counts are tiny
	if have < want {
		want = have
	}
	// A chain with a hole (pool exhaustion during generation) cannot back a
	// multi-level texture: deeper levels would sample uninitialised memory.
	for i := uint32(1); i < want; i++ {
		if chain.Level(int(i)) == nil {
			want = i
			break
		}
	}
	if want < 2 {
		if release != nil {
			release()
		}
		return 1, nil, nil
	}
	return want, chain, release
}

// trackSource records which content generation an entry holds and watches the
// source for its retirement. A command whose pixels are already behind the
// source's current generation is retired at once: it drew this frame but nothing
// will ever ask for that ID again.
func (c *ImageCache) trackSource(key imageCacheKey, entry *imageCacheEntry, cmd *ImageDrawCommand) {
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
func (c *ImageCache) untrackSource(key imageCacheKey, entry *imageCacheEntry) {
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
		c.retiredGens.Add(1)
	}
	c.retiredMu.Unlock()
}

func (c *ImageCache) noteWatched(srcGen uint64, key imageCacheKey) {
	bucket := c.bySrc[srcGen]
	if bucket == nil {
		bucket = make(map[imageCacheKey]struct{}, 1)
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
	c.bySrc = make(map[uint64]map[imageCacheKey]struct{})
	c.retiredMu.Lock()
	c.retired = make(map[uint64]struct{})
	c.retiredMu.Unlock()
	c.statsMu.Lock()
	c.usedBytes = 0
	c.statsMu.Unlock()
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
func (c *ImageCache) uploadImage(cmd *ImageDrawCommand, levels uint32, chain *intImage.MipmapChain) (*imageCacheEntry, error) {
	w := cmd.ImgWidth
	h := cmd.ImgHeight
	if w == 0 || h == 0 {
		return nil, fmt.Errorf("empty image (%dx%d)", w, h)
	}
	c.creates.Add(1)

	tex, err := c.device.CreateTexture(&hal.TextureDescriptor{
		Label:         "image_cache_tex",
		Size:          hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec // image dimensions fit uint32
		MipLevelCount: levels,
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
		MipLevelCount: levels,
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

	// Deeper levels come from the same cached chain the CPU fallback samples, so
	// both ends filter identically (D3: one chain, bit-comparable across
	// backends). resolveLevels already proved every level exists.
	total := int64(need)
	for i := uint32(1); i < levels; i++ {
		lw, lh := hal.MipLevelSize(uint32(w), uint32(h), i)
		lvl := chain.Level(int(i))
		if lvl == nil {
			view.Destroy()
			tex.Destroy()
			return nil, fmt.Errorf("mip level %d missing for %dx%d", i, w, h)
		}
		lsrc := lvl.PremultipliedData()
		lstride := lvl.Stride()
		lneed := int(lw) * int(lh) * 4
		if len(lsrc) < lstride*int(lh) || lneed <= 0 {
			view.Destroy()
			tex.Destroy()
			return nil, fmt.Errorf("mip level %d data too small (%d for %d)", i, len(lsrc), lstride*int(lh))
		}
		ldata := lsrc
		var lstaging *[]byte
		if lstride != int(lw)*4 {
			lstaging = acquireImageStaging(lneed)
			ldata = *lstaging
			for row := 0; row < int(lh); row++ {
				copy(ldata[row*int(lw)*4:(row+1)*int(lw)*4], lsrc[row*lstride:row*lstride+int(lw)*4])
			}
		}
		err := c.queue.WriteTexture(
			&hal.ImageCopyTexture{Texture: tex, MipLevel: i},
			ldata,
			&hal.ImageDataLayout{Offset: 0, BytesPerRow: uint32(lw) * 4, RowsPerImage: lh},
			&hal.Extent3D{Width: lw, Height: lh, DepthOrArrayLayers: 1},
		)
		if lstaging != nil {
			releaseImageStaging(lstaging)
		}
		if err != nil {
			view.Destroy()
			tex.Destroy()
			return nil, fmt.Errorf("upload mip level %d: %w", i, err)
		}
		total += int64(lneed)
		c.mipLevelUploads.Add(1)
	}

	// Entry bytes are the whole mip area, matching what the hal ledger charges for
	// this texture (D12) — before this the cache counted level 0 only while the
	// ledger already charged the chain, so the 128-entry/64-MiB gate judged
	// capacity on numbers nobody had spent.
	nbytes := total
	c.lastUploadBytes = nbytes
	c.uploadBytes.Add(uint64(nbytes)) //nolint:gosec // non-negative

	return &imageCacheEntry{
		texture: tex,
		view:    view,
		width:   w,
		height:  h,
		levels:  levels,
		bytes:   nbytes,
	}, nil
}

func (c *ImageCache) removeEntry(key imageCacheKey, entry *imageCacheEntry) {
	if entry == nil {
		return
	}
	c.untrackSource(key, entry)
	// Hand the native pair to the retire outlet: this frame's already-encoded
	// commands may still reference the view, so release waits for the
	// submission-completion sync point.
	c.retireEntry(entry)
	c.statsMu.Lock()
	c.usedBytes -= entry.bytes
	if c.usedBytes < 0 {
		c.usedBytes = 0
	}
	c.statsMu.Unlock()
	delete(c.entries, key)
	c.evictions.Add(1)
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
	var oldestKey imageCacheKey
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
	// MipEntries counts live entries created with more than one level, and
	// MipLevelUploads the deeper-level uploads they cost. Both are the visible
	// proof that a mip mode actually built and filled a chain — without them a
	// "mipmap supported" claim is unfalsifiable.
	MipEntries      int
	MipLevelUploads uint64
}

// Stats returns cache statistics.
func (c *ImageCache) Stats() ImageCacheStats {
	if c == nil {
		return ImageCacheStats{}
	}
	// 诊断快照读可来自任意线程（D20 转发口）：计数器原子读，entries/usedBytes
	// 由 statsMu 保护；写侧仍局限在光栅线程，单线程契约不变。
	c.statsMu.Lock()
	entries := len(c.entries)
	used := c.usedBytes
	c.statsMu.Unlock()
	return ImageCacheStats{
		Entries:          entries,
		Budget:           c.budget,
		BudgetBytes:      c.budgetBytes,
		UsedBytes:        used,
		Generations:      c.gen,
		Hits:             c.hits.Load(),
		Misses:           c.misses.Load(),
		Uploads:          c.uploads.Load(),
		UploadBytes:      c.uploadBytes.Load(),
		LastUploadBytes:  c.lastUploadBytes,
		Evictions:        c.evictions.Load(),
		EphemeralUploads: c.ephemeralUploads.Load(),
		Creates:          c.creates.Load(),
		MipEntries:       c.mipEntryCount(),
		MipLevelUploads:  c.mipLevelUploads.Load(),
		RetiredGens:      c.retiredGens.Load(),
		Watched:          c.watchedCount(),
	}
}

// watchedCount snapshots the watched-source count under retiredMu (the same
// lock the watch/unwatch paths hold; D20 readers may run on other threads).
func (c *ImageCache) watchedCount() int {
	if c == nil {
		return 0
	}
	c.retiredMu.Lock()
	defer c.retiredMu.Unlock()
	return len(c.bySrc)
}

// ResetStats clears hit/miss/upload counters (entries retained).
func (c *ImageCache) ResetStats() {
	if c == nil {
		return
	}
	c.hits.Store(0)
	c.misses.Store(0)
	c.uploads.Store(0)
	c.uploadBytes.Store(0)
	c.lastUploadBytes = 0
	c.evictions.Store(0)
	c.ephemeralUploads.Store(0)
}
