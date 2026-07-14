//go:build !nogpu

package gpu

import (
	"errors"
	"fmt"
	"sync"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
)

// BackendGPU is the identifier for the GPU backend.
const BackendGPU = "gpu"

// Backend is a GPU-accelerated rendering backend using gogpu/wgpu.
//
// The backend manages GPU resources including instance, adapter, device,
// and queue. It supports both immediate mode rendering (via NewRenderer)
// and retained mode rendering (via RenderScene).
type Backend struct {
	mu sync.RWMutex

	// GPU resources
	instance *webgpu.Instance
	adapter  *webgpu.Adapter
	device   *webgpu.Device
	queue    *webgpu.Queue

	// GPU information
	gpuInfo *GPUInfo

	// State
	initialized bool
}

// NewBackend creates a new Pure Go GPU rendering backend.
// The backend must be initialized with Init() before use.
func NewBackend() *Backend {
	return &Backend{}
}

// Name returns the backend identifier.
func (b *Backend) Name() string {
	return BackendGPU
}

// Init initializes the backend by creating GPU resources.
// This includes creating an instance, requesting an adapter,
// creating a device, and getting the command queue.
//
// Returns an error if GPU initialization fails.
func (b *Backend) Init() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.initialized {
		return nil
	}

	// Step 1: Create Instance
	desc := &webgpu.InstanceDescriptor{
		Backends: types.BackendsPrimary,
		Flags:    0,
	}
	instance, err := webgpu.CreateInstance(desc)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNoGPU, err)
	}
	b.instance = instance

	// Step 2: Request Adapter (prefer high performance GPU)
	adapter, err := b.instance.RequestAdapter(&webgpu.RequestAdapterOptions{
		PowerPreference: types.PowerPreferenceHighPerformance,
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNoGPU, err)
	}
	b.adapter = adapter

	// Log GPU information
	logGPUInfo(adapter)

	// Get GPU info for later use
	b.gpuInfo, _ = getGPUInfo(adapter)

	// Step 3: Create Device
	device, err := createDevice(adapter, "gg-wgpu-device")
	if err != nil {
		return fmt.Errorf("device creation failed: %w", err)
	}
	b.device = device

	// Step 4: Get Queue
	queue, err := getDeviceQueue(device)
	if err != nil {
		// Cleanup on failure
		_ = releaseDevice(device)
		return fmt.Errorf("queue retrieval failed: %w", err)
	}
	b.queue = queue

	b.initialized = true
	slogger().Debug("backend initialized")

	return nil
}

// Close releases all backend resources.
// The backend should not be used after Close is called.
func (b *Backend) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.initialized {
		return
	}

	// Release resources in reverse order of creation
	// Note: Queue is released when device is dropped

	if b.device != nil {
		if err := releaseDevice(b.device); err != nil {
			slogger().Warn("error releasing device", "err", err)
		}
		b.device = nil
	}

	if b.adapter != nil {
		if err := releaseAdapter(b.adapter); err != nil {
			slogger().Warn("error releasing adapter", "err", err)
		}
		b.adapter = nil
	}

	if b.instance != nil {
		b.instance.Release()
	}

	b.instance = nil
	b.queue = nil
	b.gpuInfo = nil
	b.initialized = false

	slogger().Debug("backend closed")
}

// NewRenderer creates a renderer for immediate mode rendering.
// The renderer is sized for the given dimensions.
//
// Note: This is a stub implementation that returns a GPURenderer.
// The actual GPU rendering will be implemented in TASK-110.
func (b *Backend) NewRenderer(width, height int) render.Renderer {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.initialized {
		slogger().Warn("creating renderer on uninitialized backend")
		return nil
	}

	if width <= 0 || height <= 0 {
		slogger().Warn("invalid dimensions", "width", width, "height", height)
		return nil
	}

	return newGPURenderer(b, width, height)
}

// RenderScene renders a scene to the target pixmap using retained mode.
// This method is optimized for complex scenes with many draw operations.
//
// The implementation uses GPUSceneRenderer for tessellation, strip
// rasterization, and layer compositing on the GPU, then reads the target
// texture back into the pixmap.
func (b *Backend) RenderScene(target *render.Pixmap, s *scene.Scene) error {
	b.mu.RLock()
	initialized := b.initialized
	b.mu.RUnlock()

	if !initialized {
		return ErrNotInitialized
	}

	if target == nil {
		return ErrNilTarget
	}

	if s == nil {
		return ErrNilScene
	}

	// Create GPU scene renderer for this frame
	renderer, err := NewGPUSceneRenderer(b, GPUSceneRendererConfig{
		Width:  target.Width(),
		Height: target.Height(),
	})
	if err != nil {
		return fmt.Errorf("failed to create GPU renderer: %w", err)
	}
	defer renderer.Close()

	// Render the scene to GPU
	if err := renderer.RenderToPixmap(target, s); err != nil {
		// Logical or degraded test backends may not have native resources to read back.
		if errors.Is(err, ErrTextureReadbackNotSupported) {
			slogger().Debug("RenderScene completed without readback", "err", err)
			return nil
		}
		return fmt.Errorf("GPU render failed: %w", err)
	}

	return nil
}

// IsInitialized returns true if the backend has been initialized.
func (b *Backend) IsInitialized() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.initialized
}

// GPUInfo returns information about the selected GPU.
// Returns nil if the backend is not initialized.
func (b *Backend) GPUInfo() *GPUInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.gpuInfo
}

// Device returns the GPU device ID.
// Returns a zero ID if the backend is not initialized.
func (b *Backend) Device() *webgpu.Device {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.device
}

// Queue returns the GPU queue ID.
// Returns a zero ID if the backend is not initialized.
func (b *Backend) Queue() *webgpu.Queue {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.queue
}

// GPURenderer is a GPU-backed renderer for immediate mode drawing.
// It implements the gg.Renderer interface.
//
// Note: This is a stub implementation. The actual GPU rendering
// will be implemented in TASK-110.
type GPURenderer struct {
	backend          *Backend
	width            int
	height           int
	softwareRenderer *render.SoftwareRenderer
}

// newGPURenderer creates a new GPU renderer.
func newGPURenderer(b *Backend, width, height int) *GPURenderer {
	return &GPURenderer{
		backend:          b,
		width:            width,
		height:           height,
		softwareRenderer: render.NewSoftwareRenderer(width, height),
	}
}

// Fill fills a path with the given paint.
//
// Phase 1 Implementation:
// Uses software rasterization via SoftwareRenderer. Future phases will add
// GPU texture upload and gpu GPU path rendering.
func (r *GPURenderer) Fill(pixmap *render.Pixmap, path *render.Path, paint *render.Paint) error {
	if pixmap == nil {
		return ErrNilTarget
	}
	if path == nil || paint == nil {
		return nil // No-op for nil path or paint
	}

	// Phase 1: Delegate to software renderer
	if err := r.softwareRenderer.Fill(pixmap, path, paint); err != nil {
		return fmt.Errorf("fill: %w", err)
	}

	// TODO Phase 2: Upload pixmap to GPU texture for compositing
	// TODO Phase 3: GPU path tessellation

	return nil
}

// Stroke strokes a path with the given paint.
//
// Phase 1 Implementation:
// Uses software rasterization via SoftwareRenderer. Future phases will add
// GPU texture upload and gpu GPU stroke expansion.
func (r *GPURenderer) Stroke(pixmap *render.Pixmap, path *render.Path, paint *render.Paint) error {
	if pixmap == nil {
		return ErrNilTarget
	}
	if path == nil || paint == nil {
		return nil // No-op for nil path or paint
	}

	// Phase 1: Delegate to software renderer
	if err := r.softwareRenderer.Stroke(pixmap, path, paint); err != nil {
		return fmt.Errorf("stroke: %w", err)
	}

	// TODO Phase 2: Upload pixmap to GPU texture for compositing
	// TODO Phase 3: GPU stroke expansion and tessellation

	return nil
}

// Width returns the renderer width.
func (r *GPURenderer) Width() int {
	return r.width
}

// Height returns the renderer height.
func (r *GPURenderer) Height() int {
	return r.height
}

// Close releases renderer resources.
// Note: This is a stub implementation.
func (r *GPURenderer) Close() {
	// TODO: Release GPU resources in TASK-110
}
