//go:build !nogpu

package gpu

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

// Texture-related errors.
var (
	// ErrTextureReleased is returned when operating on a released texture.
	ErrTextureReleased = errors.New("wgpu: texture has been released")

	// ErrTextureSizeMismatch is returned when pixmap size doesn't match texture.
	ErrTextureSizeMismatch = errors.New("wgpu: pixmap size does not match texture")

	// ErrNilPixmap is returned when pixmap is nil.
	ErrNilPixmap = errors.New("wgpu: pixmap is nil")

	// ErrTextureReadbackNotSupported is returned when readback is not available.
	ErrTextureReadbackNotSupported = errors.New("wgpu: texture readback not available")
)

// TextureFormat represents the pixel format of a GPU texture.
type TextureFormat uint8

const (
	// TextureFormatRGBA8 is the standard RGBA format with 8 bits per channel.
	TextureFormatRGBA8 TextureFormat = iota

	// TextureFormatBGRA8 is BGRA format, often used for surface presentation.
	TextureFormatBGRA8

	// TextureFormatR8 is single-channel 8-bit format, used for masks.
	TextureFormatR8
)

// String returns a human-readable name for the format.
func (f TextureFormat) String() string {
	switch f {
	case TextureFormatRGBA8:
		return "RGBA8"
	case TextureFormatBGRA8:
		return "BGRA8"
	case TextureFormatR8:
		return "R8"
	default:
		return fmt.Sprintf("Unknown(%d)", f)
	}
}

// BytesPerPixel returns the number of bytes per pixel for the format.
func (f TextureFormat) BytesPerPixel() int {
	switch f {
	case TextureFormatRGBA8, TextureFormatBGRA8:
		return 4
	case TextureFormatR8:
		return 1
	default:
		return 4
	}
}

// ToWGPUFormat converts to WebGPU TextureFormat.
func (f TextureFormat) ToWGPUFormat() types.TextureFormat {
	switch f {
	case TextureFormatRGBA8:
		return types.TextureFormatRGBA8Unorm
	case TextureFormatBGRA8:
		return types.TextureFormatBGRA8Unorm
	case TextureFormatR8:
		return types.TextureFormatR8Unorm
	default:
		return types.TextureFormatRGBA8Unorm
	}
}

// GPUTexture represents a GPU texture resource.
// It wraps the underlying wgpu texture and provides a high-level interface
// for texture operations including upload and download.
//
// GPUTexture is safe for concurrent read access. Write operations
// (Upload, Close) should be synchronized externally.
type GPUTexture struct {
	mu sync.RWMutex

	// GPU resources. These are nil only for legacy tests that create textures
	// without an initialized backend.
	device  *webgpu.Device
	texture *webgpu.Texture
	view    *webgpu.TextureView
	queue   *webgpu.Queue

	// Texture properties
	width  int
	height int
	format TextureFormat

	// Memory tracking
	sizeBytes uint64
	manager   *MemoryManager // optional, for memory tracking

	// State
	released atomic.Bool
	label    string
}

// TextureConfig holds configuration for creating a new texture.
type TextureConfig struct {
	// Width is the texture width in pixels.
	Width int

	// Height is the texture height in pixels.
	Height int

	// Format is the pixel format.
	Format TextureFormat

	// Label is an optional debug label.
	Label string

	// Usage flags (default: CopySrc | CopyDst | TextureBinding)
	Usage types.TextureUsage
}

// DefaultTextureUsage is the default usage for textures created without specific flags.
const DefaultTextureUsage = types.TextureUsageCopySrc | types.TextureUsageCopyDst | types.TextureUsageTextureBinding

// copyPitchAlignment is the WebGPU minimum bytesPerRow alignment for texture
// copies and queue.WriteTexture when more than one row is written.
const copyPitchAlignment = 256

func alignTextureBytesPerRow(bytesPerRow uint32) uint32 {
	return (bytesPerRow + copyPitchAlignment - 1) &^ (copyPitchAlignment - 1)
}

// packTextureUpload prepares tightly or pitch-aligned texture upload bytes.
// For multi-row uploads WebGPU requires BytesPerRow to be a multiple of 256.
// R8 textures accept either a packed R8 plane (len=w*h) or an RGBA pixmap
// (len=w*h*4), in which case the alpha channel is used as the mask.
func packTextureUpload(format TextureFormat, width, height int, src []byte) (data []byte, bytesPerRow uint32, err error) {
	if width <= 0 || height <= 0 {
		return nil, 0, ErrInvalidDimensions
	}
	bpp := format.BytesPerPixel()
	tightRow := width * bpp
	need := tightRow * height

	switch format {
	case TextureFormatR8:
		switch len(src) {
		case width * height:
			// already packed R8
		case width * height * 4:
			// extract alpha plane from RGBA pixmap
			packed := make([]byte, width*height)
			for i := 0; i < width*height; i++ {
				packed[i] = src[i*4+3]
			}
			src = packed
		default:
			return nil, 0, fmt.Errorf("%w: R8 upload expects %d or %d bytes, got %d",
				ErrTextureSizeMismatch, width*height, width*height*4, len(src))
		}
	default:
		if len(src) < need {
			return nil, 0, fmt.Errorf("%w: upload expects at least %d bytes, got %d",
				ErrTextureSizeMismatch, need, len(src))
		}
	}

	alignedRow := alignTextureBytesPerRow(uint32(tightRow)) //nolint:gosec // positive dims
	if int(alignedRow) == tightRow {
		// No padding required (single-row or already aligned).
		if height == 1 || len(src) == need {
			return src[:need], alignedRow, nil
		}
		// src may be longer; take exact plane
		return src[:need], alignedRow, nil
	}

	// Pad each row to the WebGPU copy pitch.
	out := make([]byte, int(alignedRow)*height)
	for y := 0; y < height; y++ {
		copy(out[y*int(alignedRow):y*int(alignedRow)+tightRow], src[y*tightRow:(y+1)*tightRow])
	}
	return out, alignedRow, nil
}

// CreateTexture creates a new GPU texture with the given configuration.
// The texture is uninitialized and should be filled with UploadPixmap.
func CreateTexture(backend *Backend, config TextureConfig) (*GPUTexture, error) {
	if config.Width <= 0 || config.Height <= 0 {
		return nil, ErrInvalidDimensions
	}

	// Allow nil backend for stub/testing mode
	// When backend is nil, we create a logical texture without GPU resources
	if backend != nil && !backend.IsInitialized() {
		return nil, ErrNotInitialized
	}

	// Calculate memory size
	//nolint:gosec // G115: dimensions are validated positive, overflow is acceptable for this use case
	sizeBytes := uint64(config.Width * config.Height * config.Format.BytesPerPixel())

	usage := config.Usage
	if usage == 0 {
		usage = DefaultTextureUsage
	}

	tex := &GPUTexture{
		width:     config.Width,
		height:    config.Height,
		format:    config.Format,
		sizeBytes: sizeBytes,
		label:     config.Label,
	}

	if backend == nil {
		return tex, nil
	}

	device := backend.Device()
	if device == nil {
		return nil, ErrNotInitialized
	}

	wtex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label: config.Label,
		Size: webgpu.Extent3D{
			Width:              uint32(config.Width),  //nolint:gosec // dimensions are validated positive
			Height:             uint32(config.Height), //nolint:gosec // dimensions are validated positive
			DepthOrArrayLayers: 1,
		},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        config.Format.ToWGPUFormat(),
		Usage:         usage,
	})
	if err != nil {
		return nil, err
	}

	view, err := device.CreateTextureView(wtex, &webgpu.TextureViewDescriptor{
		Label:           config.Label + "-view",
		Format:          config.Format.ToWGPUFormat(),
		Dimension:       types.TextureViewDimension2D,
		Aspect:          types.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		wtex.Release()
		return nil, err
	}

	tex.texture = wtex
	tex.view = view
	tex.device = device
	tex.queue = backend.Queue()

	return tex, nil
}

// CreateTextureFromPixmap creates a GPU texture from a pixmap, uploading
// the pixel data immediately.
func CreateTextureFromPixmap(backend *Backend, pixmap *render.Pixmap, label string) (*GPUTexture, error) {
	if pixmap == nil {
		return nil, ErrNilPixmap
	}

	tex, err := CreateTexture(backend, TextureConfig{
		Width:  pixmap.Width(),
		Height: pixmap.Height(),
		Format: TextureFormatRGBA8,
		Label:  label,
	})
	if err != nil {
		return nil, err
	}

	if err := tex.UploadPixmap(pixmap); err != nil {
		tex.Close()
		return nil, err
	}

	return tex, nil
}

// Width returns the texture width in pixels.
func (t *GPUTexture) Width() int {
	return t.width
}

// Height returns the texture height in pixels.
func (t *GPUTexture) Height() int {
	return t.height
}

// Format returns the texture format.
func (t *GPUTexture) Format() TextureFormat {
	return t.format
}

// SizeBytes returns the texture size in bytes.
func (t *GPUTexture) SizeBytes() uint64 {
	return t.sizeBytes
}

// Label returns the debug label.
func (t *GPUTexture) Label() string {
	return t.label
}

// IsReleased returns true if the texture has been released.
func (t *GPUTexture) IsReleased() bool {
	return t.released.Load()
}

// Texture returns the underlying WebGPU texture.
func (t *GPUTexture) Texture() *webgpu.Texture {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.texture
}

// View returns the default WebGPU texture view.
func (t *GPUTexture) View() *webgpu.TextureView {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.view
}

// UploadPixmap uploads pixel data from a Pixmap to the GPU texture.
// The pixmap dimensions must match the texture dimensions.
func (t *GPUTexture) UploadPixmap(pixmap *render.Pixmap) error {
	if t.released.Load() {
		return ErrTextureReleased
	}

	if pixmap == nil {
		return ErrNilPixmap
	}

	if pixmap.Width() != t.width || pixmap.Height() != t.height {
		return fmt.Errorf("%w: expected %dx%d, got %dx%d",
			ErrTextureSizeMismatch, t.width, t.height, pixmap.Width(), pixmap.Height())
	}

	t.mu.RLock()
	texture := t.texture
	queue := t.queue
	format := t.format
	width := t.width
	height := t.height
	t.mu.RUnlock()
	if texture == nil || queue == nil {
		return nil
	}

	data, bytesPerRow, err := packTextureUpload(format, width, height, pixmap.Data())
	if err != nil {
		return err
	}

	return queue.WriteTexture(&webgpu.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Origin:   webgpu.Origin3D{},
		Aspect:   types.TextureAspectAll,
	}, data, &webgpu.ImageDataLayout{
		Offset:       0,
		BytesPerRow:  bytesPerRow,
		RowsPerImage: uint32(height), //nolint:gosec // dimensions validated
	}, &webgpu.Extent3D{
		Width:              uint32(width),  //nolint:gosec // dimensions validated
		Height:             uint32(height), //nolint:gosec // dimensions validated
		DepthOrArrayLayers: 1,
	})
}

// UploadRegion uploads pixel data to a region of the texture.
// This is useful for texture atlas updates.
func (t *GPUTexture) UploadRegion(x, y int, pixmap *render.Pixmap) error {
	if t.released.Load() {
		return ErrTextureReleased
	}

	if pixmap == nil {
		return ErrNilPixmap
	}

	// Bounds check
	if x < 0 || y < 0 || x+pixmap.Width() > t.width || y+pixmap.Height() > t.height {
		return fmt.Errorf("%w: region (%d,%d)+(%dx%d) exceeds texture bounds (%dx%d)",
			ErrInvalidDimensions, x, y, pixmap.Width(), pixmap.Height(), t.width, t.height)
	}

	t.mu.RLock()
	texture := t.texture
	queue := t.queue
	format := t.format
	t.mu.RUnlock()
	if texture == nil || queue == nil {
		return nil
	}

	data, bytesPerRow, err := packTextureUpload(format, pixmap.Width(), pixmap.Height(), pixmap.Data())
	if err != nil {
		return err
	}

	return queue.WriteTexture(&webgpu.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Origin: webgpu.Origin3D{
			X: uint32(x), //nolint:gosec // bounds checked above
			Y: uint32(y), //nolint:gosec // bounds checked above
			Z: 0,
		},
		Aspect: types.TextureAspectAll,
	}, data, &webgpu.ImageDataLayout{
		Offset:       0,
		BytesPerRow:  bytesPerRow,
		RowsPerImage: uint32(pixmap.Height()), //nolint:gosec // bounds checked
	}, &webgpu.Extent3D{
		Width:              uint32(pixmap.Width()),  //nolint:gosec // bounds checked
		Height:             uint32(pixmap.Height()), //nolint:gosec // bounds checked
		DepthOrArrayLayers: 1,
	})
}

// DownloadPixmap downloads pixel data from GPU to a new Pixmap.
// This operation requires the texture to have CopySrc usage.
func (t *GPUTexture) DownloadPixmap() (*render.Pixmap, error) {
	if t.released.Load() {
		return nil, ErrTextureReleased
	}

	t.mu.RLock()
	device := t.device
	texture := t.texture
	queue := t.queue
	width := t.width
	height := t.height
	format := t.format
	t.mu.RUnlock()
	if device == nil || texture == nil || queue == nil {
		return nil, ErrTextureReadbackNotSupported
	}

	bytesPerPixel := format.BytesPerPixel()
	bytesPerRow := uint32(width * bytesPerPixel) //nolint:gosec // dimensions validated at creation
	alignedBytesPerRow := alignTextureBytesPerRow(bytesPerRow)
	stagingBufSize := uint64(alignedBytesPerRow) * uint64(height)

	stagingBuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "gpu_texture_readback_staging",
		Size:  stagingBufSize,
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create texture readback staging buffer: %w", err)
	}
	defer stagingBuf.Release()

	encoder, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{
		Label: "gpu_texture_readback_encoder",
	})
	if err != nil {
		return nil, fmt.Errorf("create texture readback encoder: %w", err)
	}

	encoder.CopyTextureToBuffer(texture, stagingBuf, []webgpu.BufferTextureCopy{{
		BufferLayout: webgpu.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  alignedBytesPerRow,
			RowsPerImage: uint32(height), //nolint:gosec // dimensions validated at creation
		},
		TextureBase: webgpu.ImageCopyTexture{
			Texture:  texture,
			MipLevel: 0,
			Origin:   webgpu.Origin3D{},
			Aspect:   types.TextureAspectAll,
		},
		Size: webgpu.Extent3D{
			Width:              uint32(width),  //nolint:gosec // dimensions validated at creation
			Height:             uint32(height), //nolint:gosec // dimensions validated at creation
			DepthOrArrayLayers: 1,
		},
	}})

	cmdBuf, err := encoder.Finish()
	if err != nil {
		return nil, fmt.Errorf("finish texture readback encoder: %w", err)
	}
	defer cmdBuf.Release()

	if _, err := queue.Submit(cmdBuf); err != nil {
		return nil, fmt.Errorf("submit texture readback: %w", err)
	}

	if err := stagingBuf.Map(context.Background(), webgpu.MapModeRead, 0, stagingBufSize); err != nil {
		return nil, fmt.Errorf("map texture readback staging buffer: %w", err)
	}
	mapped, err := stagingBuf.MappedRange(0, stagingBufSize)
	if err != nil {
		if unmapErr := stagingBuf.Unmap(); unmapErr != nil {
			return nil, fmt.Errorf("mapped texture readback range: %w (also failed to unmap: %v)", err, unmapErr)
		}
		return nil, fmt.Errorf("mapped texture readback range: %w", err)
	}
	readback := make([]byte, stagingBufSize)
	copy(readback, mapped.Bytes())
	if err := stagingBuf.Unmap(); err != nil {
		return nil, fmt.Errorf("unmap texture readback staging buffer: %w", err)
	}

	pixmap := render.NewPixmap(width, height)
	dst := pixmap.Data()
	for y := 0; y < height; y++ {
		srcRow := readback[y*int(alignedBytesPerRow) : y*int(alignedBytesPerRow)+int(bytesPerRow)]
		dstRow := dst[y*width*4 : (y+1)*width*4]
		switch format {
		case TextureFormatRGBA8:
			copy(dstRow, srcRow)
		case TextureFormatBGRA8:
			convertBGRAToRGBA(srcRow, dstRow, width)
		case TextureFormatR8:
			for x := 0; x < width; x++ {
				v := srcRow[x]
				off := x * 4
				dstRow[off+0] = 255
				dstRow[off+1] = 255
				dstRow[off+2] = 255
				dstRow[off+3] = v
			}
		default:
			return nil, fmt.Errorf("wgpu: unsupported texture readback format %s", format)
		}
	}
	pixmap.NotifyPixelsChanged()

	return pixmap, nil
}

// SetMemoryManager sets the memory manager for tracking.
// This is called internally when allocating through MemoryManager.
func (t *GPUTexture) SetMemoryManager(m *MemoryManager) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.manager = m
}

// Close releases the GPU texture resources.
// The texture should not be used after Close is called.
func (t *GPUTexture) Close() {
	if t.released.Swap(true) {
		return // Already released
	}

	t.mu.Lock()
	manager := t.manager
	t.mu.Unlock()

	// Notify memory manager if present
	if manager != nil {
		manager.unregisterTexture(t)
	}

	t.mu.Lock()
	view := t.view
	texture := t.texture
	t.view = nil
	t.texture = nil
	t.device = nil
	t.queue = nil
	t.manager = nil
	t.mu.Unlock()

	if view != nil {
		view.Release()
	}
	if texture != nil {
		texture.Release()
	}
}

// String returns a string representation of the texture.
func (t *GPUTexture) String() string {
	status := "active"
	if t.released.Load() {
		status = "released"
	}
	return fmt.Sprintf("GPUTexture[%s %dx%d %s %d bytes %s]",
		t.label, t.width, t.height, t.format, t.sizeBytes, status)
}
