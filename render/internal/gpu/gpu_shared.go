//go:build !nogpu

package gpu

import (
	"fmt"
	"log/slog"
	"os"

	"sync"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// gpuRenderStrategy controls which GPU rendering tiers are available based on
// adapter capabilities. This follows the Skia Graphite PathRendererStrategy
// pattern (RendererProvider.cpp:39-97) where the rendering approach is
// auto-selected at init time based on adapter type and MSAA support:
//
//   - strategyFull: all tiers (SDF, stencil, MSDF text, compute) with MSAA
//   - strategyNoMSAA: all tiers but without multi-sample anti-aliasing
//   - strategyRasterAtlas: CPU shapes, GPU textures only (software adapter)
//
// Strategy is set once at GPU init and never changes (deterministic).
type gpuRenderStrategy int

const (
	// strategyFull — all shapes via GPU with MSAA (hardware adapter, 4x MSAA).
	// Matches Skia Graphite kComputeMSAA8 / kTessellation path.
	strategyFull gpuRenderStrategy = iota

	// strategyNoMSAA — GPU shapes without MSAA (hardware adapter, 1x sample).
	// The GPU handles SDF, stencil, text, compute — but render passes use
	// sampleCount=1 instead of 4. This produces lower-quality edges but avoids
	// MSAA texture creation failures on backends with limited support.
	strategyNoMSAA

	// strategyRasterAtlas — CPU shapes, GPU textures only (software adapter).
	// Matches Skia Graphite kRasterAtlas: shapes route to CPU rasterizer,
	// GPU is used only for texture upload and compositing. Prevents SDF
	// pipeline hangs on software/CPU adapters (BUG-SW-002).
	strategyRasterAtlas
)

// String returns a human-readable description of the rendering strategy.
func (s gpuRenderStrategy) String() string {
	switch s {
	case strategyFull:
		return "Full GPU"
	case strategyNoMSAA:
		return "GPU (no MSAA)"
	case strategyRasterAtlas:
		return "Raster Atlas (CPU shapes, GPU textures)"
	default:
		return "Unknown"
	}
}

// GPUShared holds GPU resources that are shared across all gg.Context instances.
// This includes the device, queue, pipelines, and atlas engines — expensive to
// create, immutable or append-only after initialization.
//
// This follows the enterprise pattern observed in Skia (GrDirectContext),
// Vello (Renderer), Qt Quick (QRhi), and Flutter Impeller (Context):
// shared device + pipelines + glyph atlas, per-context pending commands.
//
// GPUShared is created once per application via RegisterAccelerator and
// accessed by all gg.Context instances through the global singleton.
type GPUShared struct {
	mu sync.Mutex

	instance *webgpu.Instance // standalone mode only; nil when using external device
	device   *webgpu.Device
	queue    *webgpu.Queue

	// Pipelines (immutable after creation, safe to share).
	sdfRenderPipeline *SDFRenderPipeline
	convexRenderer    *ConvexRenderer
	stencilRenderer   *StencilRenderer

	// Text/glyph atlas engines (append-only, shared across contexts).
	textEngine      *GPUTextEngine   // MSDF atlas (Tier 4)
	glyphMaskEngine *GlyphMaskEngine // R8 alpha atlas (Tier 6)

	// Shared atlas GPU textures (owned by GPUShared, NOT per-session).
	// All contexts reference these — prevents stale atlas in offscreen sessions.
	sharedAtlasTex  *webgpu.Texture
	sharedAtlasView *webgpu.TextureView

	// Compute pipeline.
	velloAccel *VelloAccelerator

	// S4.3 geometry caches (shared across contexts).
	pathGeomCache   *PathGeometryCache
	strokeGeomCache *StrokeGeometryCache

	// Texture pool for per-context MSAA/stencil textures (Flutter RenderTargetCache pattern).
	texturePool *TexturePool

	// Dual-texture advanced blend (B.03 Multiply/Screen/Overlay) pipeline cache.
	dualTexBlend dualTexBlendCache
	maskR8       maskR8Cache
	filterGPU    filterGPUCache

	// L.06 GPU-resident alpha mask (MaskAware). Full-surface R8 plane + texture.
	maskData   []byte
	maskW      int
	maskH      int
	maskTex    *webgpu.Texture
	maskView   *webgpu.TextureView
	maskActive bool

	// CPU SDF fallback accelerator.
	cpuFallback render.SDFAccelerator

	// sampleCount is the MSAA sample count resolved at GPU init time.
	// Normally 4 (4x MSAA). Falls back to 1 on backends that don't
	// support multisampled textures (e.g., software Vulkan / llvmpipe).
	// Resolved via resolveSampleCount() which probes the device.
	sampleCount uint32

	deviceReady    bool              // device available for texture/buffer ops (true on all strategies incl. rasterAtlas)
	gpuReady       bool              // shape/text rendering pipelines initialized (false on rasterAtlas)
	softwareMode   bool              // true when software/CPU adapter detected (informational, does not disable GPU)
	strategy       gpuRenderStrategy // auto-detected rendering strategy (Skia PathRendererStrategy pattern)
	externalDevice bool              // true when using shared device (don't destroy on Close)
}

// NewGPUShared creates a new shared GPU resource holder. GPU initialization
// is deferred until the first render or SetDeviceProvider call to avoid
// creating a standalone Vulkan device that may interfere with an external
// DX12/Metal device.
func NewGPUShared() *GPUShared {
	return &GPUShared{
		texturePool: NewTexturePool(defaultTexturePoolBudgetMB),
	}
}

// NewRenderContext creates a new per-context GPU render context that references
// this shared resource holder. Each gg.Context should have its own
// GPURenderContext for isolated pending command queues and frame tracking.
func (s *GPUShared) NewRenderContext() *GPURenderContext {
	// GPU initialization is deferred to first Flush() or SetDeviceProvider().
	// This avoids creating a standalone Vulkan instance before gogpu has a
	// chance to provide its DeviceProvider (which may be software/CPU).
	return &GPURenderContext{
		shared:    s,
		antiAlias: true,
	}
}

// IsReady reports whether the GPU shape/text pipelines are initialized.
func (s *GPUShared) IsReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gpuReady
}

// IsDeviceReady reports whether the GPU device is available for texture
// and buffer operations. True on all strategies including rasterAtlas.
// Skia Graphite: TextureProxy::Make() works under kRasterAtlas.
func (s *GPUShared) IsDeviceReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deviceReady
}

// Device returns the shared wgpu device, or nil if not initialized.
func (s *GPUShared) Device() *webgpu.Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.device
}

// Queue returns the shared wgpu queue, or nil if not initialized.
func (s *GPUShared) Queue() *webgpu.Queue {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.queue
}

// SetLogger sets the logger for the GPU subsystem.
func (s *GPUShared) SetLogger(l *slog.Logger) {
	setLogger(l)
}

// SetLCDLayout propagates the LCD subpixel layout to the glyph mask engine.
func (s *GPUShared) SetLCDLayout(layout text.LCDLayout) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureGlyphMaskEngine()
	s.glyphMaskEngine.SetLCDLayout(layout)
}

// SetForceSDF propagates the force-SDF flag to the CPU fallback accelerator.
func (s *GPUShared) SetForceSDF(force bool) {
	s.cpuFallback.SetForceSDF(force)
}

// SetDeviceProvider switches to a shared GPU device from an external provider
// (e.g., gogpu). The provider's Device() must return a *wgpu.Device.
//
// Software adapters (llvmpipe, SwiftShader, WARP) are treated as full GPU
// implementations per enterprise pattern (ADR-046): Skia Graphite runs CI on
// SwiftShader, wgpu treats CPU adapters identically, Flutter Impeller uses
// SwiftShader for testing. Capability differences are handled via probing
// (e.g., MSAA fallback to 1x), not blanket disabling.
func (s *GPUShared) SetDeviceProvider(provider gpucontext.DeviceProvider) error {
	if adapter := provider.Adapter(); !adapter.IsNil() {
		wgpuAdapter := webgpu.AdapterFromHandle(adapter)
		if wgpuAdapter != nil && wgpuAdapter.Info().DeviceType == types.DeviceTypeCPU {
			slogger().Info("gpu-shared: software adapter detected — GPU features available, performance may be reduced",
				"adapter", wgpuAdapter.Info().Name)
			s.mu.Lock()
			s.softwareMode = true
			s.mu.Unlock()
		}
	}

	dev := provider.Device()
	if dev.IsNil() {
		return fmt.Errorf("gpu-shared: provider Device is nil")
	}

	wgpuDev := webgpu.DeviceFromHandle(dev)
	if wgpuDev == nil {
		return fmt.Errorf("gpu-shared: provider Device handle is invalid")
	}
	wgpuQueue := wgpuDev.Queue()
	if wgpuQueue == nil {
		return fmt.Errorf("gpu-shared: provider Queue is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Destroy own resources if we created them.
	s.destroyPipelinesLocked()
	if !s.externalDevice && s.device != nil {
		s.device.Release()
	}
	if s.instance != nil {
		s.instance.Release()
		s.instance = nil
	}

	// Use provided resources.
	s.device = wgpuDev
	s.queue = wgpuQueue
	s.externalDevice = true

	// External/window devices: avoid MSAA probe CreateTexture (can abort via
	// uncaptured OOM on software backends after long suites). Prefer prior
	// sampleCount when set; otherwise default to 4. Callers that need lower
	// memory can set GPUI_SURFACE_SAMPLE_COUNT=1 before SetDeviceProvider.
	if sc := os.Getenv("GPUI_SURFACE_SAMPLE_COUNT"); sc == "1" {
		s.sampleCount = 1
	} else if s.sampleCount == 0 {
		s.sampleCount = 4
	}

	// Auto-detect rendering strategy (Skia PathRendererStrategy pattern).
	s.strategy = s.detectStrategy()

	s.deviceReady = true

	if s.strategy == strategyRasterAtlas {
		slogger().Info("gpu-shared: rasterAtlas strategy — SDF pipelines skipped, device ready for textures",
			"strategy", s.strategy.String(),
			"softwareMode", s.softwareMode,
		)
		return nil
	}

	// Create pipelines with shared device (hardware adapters only).
	s.sdfRenderPipeline = NewSDFRenderPipeline(s.device, s.queue, s.sampleCount)
	s.convexRenderer = NewConvexRenderer(s.device, s.queue, s.sampleCount)
	s.stencilRenderer = NewStencilRenderer(s.device, s.queue, s.sampleCount)

	s.gpuReady = true

	// Initialize internal VelloAccelerator with the shared device.
	s.initVelloAccelerator(s.device, s.queue)

	slogger().Info("gpu-shared: switched to shared GPU device",
		"strategy", s.strategy.String(),
		"adapter", fmt.Sprintf("%T", s.device),
		"msaa_samples", s.sampleCount,
	)
	return nil
}

// CanRenderDirect reports whether the GPU is initialized and can render
// directly to a surface. Returns false when the rendering strategy is
// strategyRasterAtlas (software/CPU adapters) — shapes route to CPU
// rasterizer instead (BUG-SW-002, Skia kRasterAtlas pattern).
func (s *GPUShared) CanRenderDirect() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.strategy == strategyRasterAtlas {
		return false
	}
	return s.gpuReady
}

// CanCompute reports whether the compute pipeline is available.
func (s *GPUShared) CanCompute() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.velloAccel != nil && s.velloAccel.CanCompute()
}

// SetTexturePoolBudget sets the maximum memory budget for the texture pool
// in megabytes. Default is 128MB (~5 concurrent 1080p MSAA4x contexts).
func (s *GPUShared) SetTexturePoolBudget(mb int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.texturePool.SetBudget(mb)
}

// Close releases all shared GPU resources. After this call, GPU rendering
// is no longer possible. Idempotent.
func (s *GPUShared) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.textEngine = nil
	if s.sharedAtlasView != nil {
		s.sharedAtlasView.Release()
		s.sharedAtlasView = nil
	}
	if s.sharedAtlasTex != nil {
		s.sharedAtlasTex.Release()
		s.sharedAtlasTex = nil
	}
	if s.glyphMaskEngine != nil && s.device != nil {
		s.glyphMaskEngine.Destroy(s.device)
		s.glyphMaskEngine = nil
	}
	if s.velloAccel != nil {
		s.velloAccel.Close()
		s.velloAccel = nil
	}
	if s.texturePool != nil {
		s.texturePool.DestroyAll()
	}
	s.dualTexBlend.release()
	s.maskR8.release()
	s.filterGPU.release()
	s.clearMaskLocked()
	s.destroyPipelinesLocked()
	if !s.externalDevice {
		if s.device != nil {
			s.device.Release()
			s.device = nil
		}
		if s.instance != nil {
			s.instance.Release()
			s.instance = nil
		}
	} else {
		s.device = nil
		s.instance = nil
	}
	s.queue = nil
	s.deviceReady = false
	s.gpuReady = false
	s.externalDevice = false
}

// SampleCount returns the resolved MSAA sample count (4 or 1).
// Returns 4 before GPU initialization (safe default for pipeline descriptors).
func (s *GPUShared) SampleCount() uint32 {
	if s.sampleCount == 0 {
		return 4 // default before init
	}
	return s.sampleCount
}

// detectStrategy determines the rendering strategy based on adapter type and
// MSAA support. Must be called with s.mu held, after softwareMode and
// sampleCount are resolved.
//
// This follows the Skia Graphite RendererProvider pattern
// (RendererProvider.cpp:84-97):
//
//	prefer compute > tessellation > raster atlas
//
// Our equivalent:
//
//	softwareMode=false, MSAA=4x → strategyFull (all GPU tiers)
//	softwareMode=false, MSAA=1x → strategyNoMSAA (GPU without MSAA)
//	softwareMode=true            → strategyRasterAtlas (CPU shapes, GPU textures)
func (s *GPUShared) detectStrategy() gpuRenderStrategy {
	if s.softwareMode {
		return strategyRasterAtlas
	}
	if s.sampleCount <= 1 {
		return strategyNoMSAA
	}
	return strategyFull
}

// resolveSampleCount probes the device for 4x MSAA support by attempting
// to create a small multisampled texture. If creation fails (e.g., software
// Vulkan / llvmpipe), falls back to 1x. This follows the Skia Graphite
// pattern (Caps::getCompatibleMSAASampleCount): try preferred, downgrade
// on failure.
//
// The WebGPU spec guarantees sampleCount=4 for standard formats on compliant
// implementations, but software backends may not be fully compliant.
func resolveSampleCount(device *webgpu.Device) uint32 {
	tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "msaa_probe",
		Size:          webgpu.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   4,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatBGRA8Unorm,
		Usage:         types.TextureUsageRenderAttachment,
	})
	if err != nil {
		slogger().Info("4x MSAA not supported, falling back to 1x", "error", err)
		return 1
	}
	tex.Release()
	return 4
}

// ensureGPU lazily initializes a standalone GPU device if no shared device
// was provided. Must be called with s.mu held.
func (s *GPUShared) ensureGPU() error {
	if s.device != nil {
		s.registerFilterGraphIfNeeded()
		return nil
	}
	if err := s.initGPU(); err != nil {
		return err
	}
	s.registerFilterGraphIfNeeded()
	return nil
}

// ensurePipelines lazily creates shape rendering pipelines. Skipped on
// rasterAtlas — SDF/stencil/convex pipelines hang on software SPIR-V
// interpreter (BUG-SW-002). Must be called with s.mu held.
func (s *GPUShared) ensurePipelines() {
	if s.strategy == strategyRasterAtlas {
		return
	}
	if s.sdfRenderPipeline == nil {
		s.sdfRenderPipeline = NewSDFRenderPipeline(s.device, s.queue, s.sampleCount)
	}
	if s.convexRenderer == nil {
		s.convexRenderer = NewConvexRenderer(s.device, s.queue, s.sampleCount)
	}
	if s.stencilRenderer == nil {
		s.stencilRenderer = NewStencilRenderer(s.device, s.queue, s.sampleCount)
	}
}

// ensureGlyphMaskEngine lazily creates the glyph mask engine. Must be called
// with s.mu held.
func (s *GPUShared) ensureGlyphMaskEngine() {
	if s.glyphMaskEngine == nil {
		s.glyphMaskEngine = NewGlyphMaskEngine()
	}
}

// ensureTextEngine lazily creates the text engine. Must be called with s.mu held.
func (s *GPUShared) ensureTextEngine() {
	if s.textEngine == nil {
		s.textEngine = NewGPUTextEngine()
	}
}

func (s *GPUShared) initGPU() error {
	instance, err := webgpu.CreateInstance(&webgpu.InstanceDescriptor{
		Backends: webgpu.BackendsVulkan,
	})
	if err != nil {
		return fmt.Errorf("create instance: %w", err)
	}
	s.instance = instance

	adapter, err := instance.RequestAdapter(&webgpu.RequestAdapterOptions{
		PowerPreference: webgpu.PowerPreferenceHighPerformance,
	})
	if err != nil {
		return fmt.Errorf("request adapter: %w", err)
	}

	// Check for software/CPU adapter before creating device.
	adapterInfo := adapter.Info()
	if adapterInfo.DeviceType == types.DeviceTypeCPU {
		slogger().Info("gpu-shared: software adapter detected, SDF pipeline disabled",
			"adapter", adapterInfo.Name)
		s.softwareMode = true
	}

	device, err := adapter.RequestDevice(renderDeviceDescriptor("gg-shared"))
	if err != nil {
		return fmt.Errorf("request device: %w", err)
	}
	s.device = device
	s.queue = device.Queue()

	// Probe MSAA support (Skia Graphite pattern: try 4x, fallback to 1x).
	s.sampleCount = resolveSampleCount(s.device)

	// Auto-detect rendering strategy (Skia PathRendererStrategy pattern).
	s.strategy = s.detectStrategy()

	s.deviceReady = true

	// Create pipelines (device stays alive for texture ops even in softwareMode).
	s.sdfRenderPipeline = NewSDFRenderPipeline(s.device, s.queue, s.sampleCount)
	s.convexRenderer = NewConvexRenderer(s.device, s.queue, s.sampleCount)
	s.stencilRenderer = NewStencilRenderer(s.device, s.queue, s.sampleCount)

	s.gpuReady = true

	// Initialize internal VelloAccelerator for compute routing.
	s.initVelloAccelerator(s.device, s.queue)

	slogger().Info("gpu-shared: GPU initialized",
		"strategy", s.strategy.String(),
		"adapter", adapterInfo.Name,
		"msaa_samples", s.sampleCount,
		"softwareMode", s.softwareMode,
	)
	return nil
}

func (s *GPUShared) initVelloAccelerator(device *webgpu.Device, queue *webgpu.Queue) {
	va := &VelloAccelerator{}
	va.device = device
	va.queue = queue
	va.externalDevice = true
	va.gpuReady = true

	dispatcher := NewVelloComputeDispatcher(device, queue)
	if err := dispatcher.Init(); err != nil {
		slogger().Debug("gpu-shared: compute pipeline unavailable", "error", err)
		s.velloAccel = va
		return
	}
	va.dispatcher = dispatcher
	s.velloAccel = va
	slogger().Debug("gpu-shared: VelloAccelerator initialized for compute routing")
}

func (s *GPUShared) destroyPipelinesLocked() {
	if s.sdfRenderPipeline != nil {
		s.sdfRenderPipeline.Destroy()
		s.sdfRenderPipeline = nil
	}
	if s.convexRenderer != nil {
		s.convexRenderer.Destroy()
		s.convexRenderer = nil
	}
	if s.stencilRenderer != nil {
		s.stencilRenderer.Destroy()
		s.stencilRenderer = nil
	}
}

// GPUMemoryStats holds diagnostic information about GPU resource usage.
type GPUMemoryStats struct {
	TexturePoolPooled int
	TexturePoolMB     int
	TilePoolPooled    int
}

// MemoryStats returns GPU memory statistics for diagnostics.
func (s *GPUShared) MemoryStats() GPUMemoryStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := GPUMemoryStats{}
	if s.texturePool != nil {
		stats.TexturePoolPooled = s.texturePool.PooledCount()
		stats.TexturePoolMB = s.texturePool.EstimatedUsageMB()
	}
	stats.TilePoolPooled = globalTilePool.Stats().Pooled
	return stats
}

// clearMaskLocked releases GPU mask resources. Caller must hold s.mu.
func (s *GPUShared) clearMaskLocked() {
	if s.maskView != nil {
		s.maskView.Release()
		s.maskView = nil
	}
	if s.maskTex != nil {
		s.maskTex.Release()
		s.maskTex = nil
	}
	s.maskData = nil
	s.maskW = 0
	s.maskH = 0
	s.maskActive = false
}

// SetMaskTexture uploads a full-surface R8 alpha mask (L.06 MaskAware).
// data is width*height bytes; nil clears the mask.
func (s *GPUShared) SetMaskTexture(data []byte, width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearMaskLocked()
	if data == nil || width <= 0 || height <= 0 {
		return
	}
	need := width * height
	if len(data) < need {
		return
	}
	s.maskData = append([]byte(nil), data[:need]...)
	s.maskW = width
	s.maskH = height
	s.maskActive = true

	if err := s.ensureGPU(); err != nil || s.device == nil || s.queue == nil {
		// CPU plane retained for fillMaskedAsImage region sampling.
		return
	}

	tex, err := s.device.CreateTexture(&webgpu.TextureDescriptor{
		Label: "l06_mask_r8",
		Size: webgpu.Extent3D{
			Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1, //nolint:gosec
		},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatR8Unorm,
		Usage:         types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		return
	}
	view, err := s.device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
		Label:         "l06_mask_r8_view",
		Format:        types.TextureFormatR8Unorm,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		tex.Release()
		return
	}
	tight := uint32(width) //nolint:gosec
	aligned := alignTextureBytesPerRow(tight)
	upload := s.maskData
	if aligned != tight {
		padded := make([]byte, int(aligned)*height)
		for y := 0; y < height; y++ {
			copy(padded[y*int(aligned):y*int(aligned)+width], s.maskData[y*width:(y+1)*width])
		}
		upload = padded
	}
	if err := s.queue.WriteTexture(
		&webgpu.ImageCopyTexture{Texture: tex, MipLevel: 0},
		upload,
		&webgpu.ImageDataLayout{BytesPerRow: aligned, RowsPerImage: uint32(height)},           //nolint:gosec
		&webgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}, //nolint:gosec
	); err != nil {
		view.Release()
		tex.Release()
		return
	}
	s.maskTex = tex
	s.maskView = view
}

// ClearMaskTexture removes the GPU-resident alpha mask.
func (s *GPUShared) ClearMaskTexture() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearMaskLocked()
}

// MaskPlane returns a copy of the active R8 mask plane when set.
// ok is false when no mask is active.
func (s *GPUShared) MaskPlane() (data []byte, w, h int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.maskActive || s.maskW <= 0 || s.maskH <= 0 || len(s.maskData) < s.maskW*s.maskH {
		return nil, 0, 0, false
	}
	out := append([]byte(nil), s.maskData[:s.maskW*s.maskH]...)
	return out, s.maskW, s.maskH, true
}

// HasGPUMask reports whether a native R8 mask texture is currently bound.
func (s *GPUShared) HasGPUMask() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maskActive && s.maskTex != nil && s.maskView != nil
}

// MaskTextureView returns the active R8 mask view when a GPU mask is bound.
// Caller must not release the view; ownership remains with GPUShared.
func (s *GPUShared) MaskTextureView() (*webgpu.TextureView, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.maskActive || s.maskView == nil {
		return nil, false
	}
	return s.maskView, true
}

var gpuFilterGraphRegistered bool

func (s *GPUShared) registerFilterGraphIfNeeded() {
	if gpuFilterGraphRegistered {
		return
	}
	render.RegisterGPUFilterGraph(func(src []byte, w, h int, nodes []render.ImageFilterNode) ([]byte, error) {
		s.mu.Lock()
		if err := s.ensureGPU(); err != nil {
			s.mu.Unlock()
			return nil, err
		}
		device, queue := s.device, s.queue
		cache := &s.filterGPU
		s.mu.Unlock()
		return runGPUFilterGraph(device, queue, cache, src, w, h, nodes)
	})
	gpuFilterGraphRegistered = true
}

// PathGeomCache returns the shared path tessellation cache (S4.3), creating if needed.
func (s *GPUShared) PathGeomCache() *PathGeometryCache {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pathGeomCache == nil {
		s.pathGeomCache = NewPathGeometryCache()
	}
	return s.pathGeomCache
}

// StrokeGeomCache returns the shared stroke expansion cache (S4.3).
func (s *GPUShared) StrokeGeomCache() *StrokeGeometryCache {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.strokeGeomCache == nil {
		s.strokeGeomCache = NewStrokeGeometryCache()
	}
	return s.strokeGeomCache
}
