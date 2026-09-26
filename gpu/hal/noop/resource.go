//go:build !(js && wasm)

package noop

import (
	"sync/atomic"
	"time"

	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// Resource is a placeholder implementation for most HAL resource types.
// It implements the hal.Resource interface with a no-op Destroy method.
type Resource struct{}

// Destroy is a no-op.
func (r *Resource) Destroy() {}

// NativeHandle returns 0 for noop resources (no real handle).
func (r *Resource) NativeHandle() uintptr { return 0 }

// Buffer implements hal.Buffer with in-memory data storage.
type Buffer struct {
	Resource
	data  []byte
	size  uint64
	usage gputypes.BufferUsage
	label string
}

// NativeHandle returns 0 for noop buffers.
func (b *Buffer) NativeHandle() uintptr { return 0 }

// Size returns the buffer size in bytes.
func (b *Buffer) Size() uint64 { return b.size }

// Usage returns the buffer's usage flags.
func (b *Buffer) Usage() gputypes.BufferUsage { return b.usage }

// Label returns the buffer's debug label.
func (b *Buffer) Label() string { return b.label }

// Texture implements hal.Texture.
type Texture struct {
	Resource
	format gputypes.TextureFormat
}

// Format returns the texture format.
func (t *Texture) Format() gputypes.TextureFormat { return t.format }

// CurrentUsage returns 0 — noop backend has no resource state tracking.
func (t *Texture) CurrentUsage() gputypes.TextureUsage { return 0 }
func (t *Texture) AddPendingRef()                      {}
func (t *Texture) DecPendingRef()                      {}

// NativeHandle returns 0 for noop textures.
func (t *Texture) NativeHandle() uintptr { return 0 }

// TextureView implements hal.TextureView for the noop backend.
type TextureView struct {
	Resource
	parent hal.Texture
}

// Texture returns the parent texture.
func (v *TextureView) Texture() hal.Texture { return v.parent }

// Surface implements hal.Surface for the noop backend.
type Surface struct {
	Resource
	configured bool
}

// Configure marks the surface as configured.
func (s *Surface) Configure(_ hal.Device, _ *hal.SurfaceConfiguration) error {
	s.configured = true
	return nil
}

// Unconfigure marks the surface as unconfigured.
func (s *Surface) Unconfigure(_ hal.Device) {
	s.configured = false
}

// AcquireTexture returns a placeholder surface texture.
// The fence parameter is ignored.
func (s *Surface) AcquireTexture(_ hal.Fence) (*hal.AcquiredSurfaceTexture, error) {
	return &hal.AcquiredSurfaceTexture{
		Texture:    &SurfaceTexture{},
		Suboptimal: false,
	}, nil
}

// DiscardTexture is a no-op.
func (s *Surface) DiscardTexture(_ hal.SurfaceTexture) {}

// ActualExtent returns (0, 0) for the noop backend.
// The noop backend does not track configured dimensions.
func (s *Surface) ActualExtent() (width, height uint32) {
	return 0, 0
}

// SurfaceTexture implements hal.SurfaceTexture.
type SurfaceTexture struct {
	Texture
}

// Fence implements hal.Fence with an atomic counter for synchronization.
type Fence struct {
	Resource
	value atomic.Uint64
}

// Wait simulates waiting for the fence to reach a value.
// Returns true if the fence has reached the value, false otherwise.
// The timeout parameter is ignored in the noop implementation.
func (f *Fence) Wait(value uint64, _ time.Duration) bool {
	return f.value.Load() >= value
}

// Signal sets the fence value.
func (f *Fence) Signal(value uint64) {
	f.value.Store(value)
}

// GetValue returns the current fence value.
func (f *Fence) GetValue() uint64 {
	return f.value.Load()
}
