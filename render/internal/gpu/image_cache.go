//go:build !nogpu

package gpu

import (
	"fmt"
	"sync"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// defaultImageCacheBudget is the maximum number of cached image textures.
// LRU eviction removes the least recently used entry when exceeded.
const defaultImageCacheBudget = 128 // S6.7: raised from 64 for denser UI icon sets

// defaultImageCacheBudgetBytes caps resident image texture bytes (~64 MiB).
const defaultImageCacheBudgetBytes int64 = 64 << 20

// imageCacheEntry holds a GPU texture and view for a cached image.
type imageCacheEntry struct {
	texture *webgpu.Texture
	view    *webgpu.TextureView
	width   int
	height  int
	bytes   int64
	gen     uint64 // LRU generation counter
}

// ImageCache manages GPU textures for image patterns. Images are uploaded
// on first use and reused on subsequent frames. The cache is keyed by
// Pixmap.GenerationID() — a monotonic counter that guarantees unique identity
// even when Go's GC reuses memory addresses (ADR-014).
//
// This follows the enterprise pattern:
//   - Skia: GrResourceCache keyed by SkPixelRef::getGenerationID()
//   - Vello: image_cache keyed by peniko::Blob::id() (AtomicU64)
//   - femtovg: SlotMap with generational index
//
// The cache is NOT thread-safe — accessed only from the render path
// which is serialized per GPURenderContext.
//
// S6.7: entry + byte budgets, upload diagnostics, ephemeral (gen=0) release,
// staging scratch pool for non-tight stride copies.
type ImageCache struct {
	device *webgpu.Device
	queue  *webgpu.Queue

	entries     map[uint64]*imageCacheEntry // keyed by Pixmap.GenerationID()
	budget      int
	budgetBytes int64
	usedBytes   int64
	gen         uint64 // global LRU generation counter

	// Stats
	hits             uint64
	misses           uint64
	uploads          uint64
	uploadBytes      uint64
	lastUploadBytes  int64
	evictions        uint64
	ephemeralUploads uint64

	// gen==0 textures live for one frame then must be released (S6.7 leak fix).
	ephemeral []*imageCacheEntry
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
func NewImageCache(device *webgpu.Device, queue *webgpu.Queue) *ImageCache {
	return &ImageCache{
		device:      device,
		queue:       queue,
		entries:     make(map[uint64]*imageCacheEntry),
		budget:      defaultImageCacheBudget,
		budgetBytes: defaultImageCacheBudgetBytes,
	}
}

// SetBudgets updates entry and byte soft limits (tests/tuning).
func (c *ImageCache) SetBudgets(entries int, bytes int64) {
	if c == nil {
		return
	}
	if entries > 0 {
		c.budget = entries
	}
	if bytes > 0 {
		c.budgetBytes = bytes
	}
}

// GetOrUpload returns the cached GPU texture view for the given image data,
// uploading it if not already cached. The cache key is ImageDrawCommand.GenerationID
// (from Pixmap.GenerationID()), not a pointer.
func (c *ImageCache) GetOrUpload(cmd *ImageDrawCommand) (*webgpu.TextureView, error) {
	if len(cmd.PixelData) == 0 {
		return nil, fmt.Errorf("empty pixel data")
	}

	key := cmd.GenerationID
	if key == 0 {
		// No generation ID — upload without long-term caching (temporary data).
		// S6.7: track as ephemeral and release via ReleaseEphemeral after submit.
		entry, err := c.uploadImage(cmd)
		if err != nil {
			return nil, err
		}
		c.uploads++
		c.ephemeralUploads++
		c.ephemeral = append(c.ephemeral, entry)
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

	return entry.view, nil
}

// ReleaseEphemeral frees gen==0 textures from the previous frame.
// Safe to call even when none exist. Call after GPU submit for that frame.
func (c *ImageCache) ReleaseEphemeral() {
	if c == nil || len(c.ephemeral) == 0 {
		return
	}
	for _, entry := range c.ephemeral {
		if entry.view != nil {
			entry.view.Release()
		}
		if entry.texture != nil {
			entry.texture.Release()
		}
	}
	c.ephemeral = c.ephemeral[:0]
}

// Destroy releases all cached GPU textures and views.
func (c *ImageCache) Destroy() {
	if c == nil {
		return
	}
	c.ReleaseEphemeral()
	for key, entry := range c.entries {
		entry.view.Release()
		entry.texture.Release()
		delete(c.entries, key)
	}
	c.usedBytes = 0
}

// uploadImage creates a GPU texture and uploads pixel data from an ImageDrawCommand.
func (c *ImageCache) uploadImage(cmd *ImageDrawCommand) (*imageCacheEntry, error) {
	w := cmd.ImgWidth
	h := cmd.ImgHeight
	if w == 0 || h == 0 {
		return nil, fmt.Errorf("empty image (%dx%d)", w, h)
	}

	tex, err := c.device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "image_cache_tex",
		Size:          webgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec // image dimensions fit uint32
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatRGBA8Unorm,
		Usage:         types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create image texture: %w", err)
	}

	view, err := c.device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
		Label:         "image_cache_view",
		Format:        types.TextureFormatRGBA8Unorm,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		tex.Release()
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
		// S6.7: pool staging for non-tight rows.
		staging = acquireImageStaging(need)
		pixelData = *staging
		for row := 0; row < h; row++ {
			srcOff := row * stride
			dstOff := row * w * 4
			copy(pixelData[dstOff:dstOff+w*4], cmd.PixelData[srcOff:srcOff+w*4])
		}
	}

	if err := c.queue.WriteTexture(
		&webgpu.ImageCopyTexture{Texture: tex, MipLevel: 0},
		pixelData,
		&webgpu.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  bytesPerRow,
			RowsPerImage: uint32(h), //nolint:gosec // image height fits uint32
		},
		&webgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec // image dimensions fit uint32
	); err != nil {
		if staging != nil {
			releaseImageStaging(staging)
		}
		view.Release()
		tex.Release()
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
	entry.view.Release()
	entry.texture.Release()
	c.usedBytes -= entry.bytes
	if c.usedBytes < 0 {
		c.usedBytes = 0
	}
	delete(c.entries, key)
	c.evictions++
}

// evictIfNeeded frees LRU entries until under entry and byte budgets and room for needBytes.
func (c *ImageCache) evictIfNeeded(needBytes int64) {
	for len(c.entries) >= c.budget || (c.budgetBytes > 0 && c.usedBytes+needBytes > c.budgetBytes) {
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

// ImageCacheStats returns cache statistics for diagnostics (S4.3/S6.7).
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
	EphemeralPending int
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
		EphemeralPending: len(c.ephemeral),
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
