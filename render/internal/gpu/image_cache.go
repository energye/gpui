//go:build !nogpu

package gpu

import (
	"fmt"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// defaultImageCacheBudget is the maximum number of cached image textures.
// LRU eviction removes the least recently used entry when exceeded.
const defaultImageCacheBudget = 64

// imageCacheEntry holds a GPU texture and view for a cached image.
type imageCacheEntry struct {
	texture *webgpu.Texture
	view    *webgpu.TextureView
	width   int
	height  int
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
type ImageCache struct {
	device *webgpu.Device
	queue  *webgpu.Queue

	entries map[uint64]*imageCacheEntry // keyed by Pixmap.GenerationID()
	budget  int
	gen     uint64 // global LRU generation counter

	// S4.3 texture cache stats
	hits    uint64
	misses  uint64
	uploads uint64
}

// NewImageCache creates a new image texture cache with the given device and queue.
func NewImageCache(device *webgpu.Device, queue *webgpu.Queue) *ImageCache {
	return &ImageCache{
		device:  device,
		queue:   queue,
		entries: make(map[uint64]*imageCacheEntry),
		budget:  defaultImageCacheBudget,
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
		// No generation ID — upload without caching (temporary data).
		entry, err := c.uploadImage(cmd)
		if err != nil {
			return nil, err
		}
		return entry.view, nil
	}

	if entry, ok := c.entries[key]; ok {
		c.gen++
		entry.gen = c.gen
		c.hits++
		return entry.view, nil
	}

	c.misses++
	if len(c.entries) >= c.budget {
		c.evictOldest()
	}

	entry, err := c.uploadImage(cmd)
	if err != nil {
		return nil, err
	}
	c.uploads++

	c.gen++
	entry.gen = c.gen
	c.entries[key] = entry

	return entry.view, nil
}

// Destroy releases all cached GPU textures and views.
func (c *ImageCache) Destroy() {
	for key, entry := range c.entries {
		entry.view.Release()
		entry.texture.Release()
		delete(c.entries, key)
	}
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
	var pixelData []byte
	stride := cmd.ImgStride
	if stride == 0 {
		stride = w * 4
	}
	if stride == w*4 {
		pixelData = cmd.PixelData[:w*h*4]
	} else {
		pixelData = make([]byte, w*h*4)
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
		view.Release()
		tex.Release()
		return nil, fmt.Errorf("upload image pixels: %w", err)
	}

	return &imageCacheEntry{
		texture: tex,
		view:    view,
		width:   w,
		height:  h,
	}, nil
}

// evictOldest removes the least recently used cache entry.
func (c *ImageCache) evictOldest() {
	var oldestKey uint64
	oldestGen := ^uint64(0)
	for key, entry := range c.entries {
		if entry.gen < oldestGen {
			oldestGen = entry.gen
			oldestKey = key
		}
	}
	if entry, ok := c.entries[oldestKey]; ok {
		entry.view.Release()
		entry.texture.Release()
		delete(c.entries, oldestKey)
	}
}

// ImageCacheStats returns cache statistics for diagnostics (S4.3).
type ImageCacheStats struct {
	Entries     int
	Budget      int
	Generations uint64
	Hits        uint64
	Misses      uint64
	Uploads     uint64
}

// Stats returns cache statistics.
func (c *ImageCache) Stats() ImageCacheStats {
	if c == nil {
		return ImageCacheStats{}
	}
	return ImageCacheStats{
		Entries:     len(c.entries),
		Budget:      c.budget,
		Generations: c.gen,
		Hits:        c.hits,
		Misses:      c.misses,
		Uploads:     c.uploads,
	}
}
