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
	"log/slog"

	"sync"
	"sync/atomic"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/gpu/res"
	"github.com/energye/gpui/render/text"
)

// gpuRenderStrategy controls which GPU rendering tiers are available based on
// adapter capabilities. This follows the Skia Graphite PathRendererStrategy
// pattern where the rendering approach is
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
	strategyFull gpuRenderStrategy = iota

	// strategyNoMSAA — GPU shapes without MSAA (hardware adapter, 1x sample).
	// The GPU handles SDF, stencil, text, compute — but render passes use
	// sampleCount=1 instead of 4. This produces lower-quality edges but avoids
	// MSAA texture creation failures on backends with limited support.
	strategyNoMSAA

	// strategyRasterAtlas — CPU shapes, GPU textures only (software adapter).
	// GPU is used only for texture upload and compositing. Prevents SDF
	// pipeline hangs on software/CPU adapters.
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

// GPUShared holds GPU resources that are shared across all render.Context instances.
// This includes the device, queue, pipelines, and atlas engines — expensive to
// create, immutable or append-only after initialization.
//
// This follows the enterprise pattern observed in Skia (GrDirectContext),
// Vello (Renderer), Qt Quick (QRhi), and Flutter Impeller (Context):
// shared device + pipelines + glyph atlas, per-context pending commands.
//
// GPUShared is created once per application via RegisterAccelerator and
// accessed by all render.Context instances through the global singleton.
type GPUShared struct {
	mu sync.Mutex

	// userShader holds the P3.2 user-shader pass-1 cache (per-effect
	// pipelines + uniform buffers; bound images upload per draw).
	userShader userShaderCache

	// linearFixedFunctionFallbacks counts paints that requested linear
	// blending (P3.1 BlendSpaceLinear) on a mode whose only GPU path is a
	// fixed-function blend state: those draws proceed in the sRGB domain
	// until the P3.1b sRGB-target format switch. Telemetry only — the
	// drawing result is identical to an sRGB-space paint by design.
	linearFixedFunctionFallbacks atomic.Int64

	// submitMu serializes present submits (Flutter draws every view on the
	// raster thread; Chromium funnels GPU work onto one thread). Record
	// paths stay parallel on their own raster threads; only the submit /
	// present critical section takes this leaf lock (never hold s.mu
	// while acquiring it).
	submitMu sync.Mutex

	instance hal.Instance // standalone mode only; nil when using external device
	adapter  hal.Adapter  // standalone mode only; must Release on Close
	device   hal.Device
	queue    hal.Queue

	// Pipelines (immutable after creation, safe to share).
	sdfRenderPipeline *SDFRenderPipeline
	convexRenderer    *ConvexRenderer
	stencilRenderer   *StencilRenderer

	// Shared clip/mask layouts baked into the pipelines above. Owned by
	// GPUShared, created once under mu, destroyed only on shared teardown
	// or device loss. Sessions borrow the handles and never destroy them;
	// a window close only drops its own surface/frame resources. This is
	// the Skia GrDirectContext rule: one context, immutable shared
	// pipelines, N surfaces.
	clipBindLayout hal.BindGroupLayout
	maskBindLayout hal.BindGroupLayout
	// sharedLayoutsDev is the device the two layouts above were created on.
	// A nil-check alone cannot tell a live handle from a foreign device's:
	// passing another device's bind group layout to CreatePipelineLayout is a
	// non-unwinding abort inside wgpu-native (H19), not an error return.
	sharedLayoutsDev hal.Device

	// Text/glyph atlas engines (append-only, shared across contexts).
	textEngine       *GPUTextEngine    // MSDF atlas (Tier 4)
	glyphMaskEngine  *GlyphMaskEngine  // R8 alpha atlas (Tier 6)
	colorGlyphEngine *ColorGlyphEngine // RGBA color atlas (Tier 6 color path)

	// Per-atlas-index GPU textures for the MSDF atlas pages (owned by
	// GPUShared, NOT per-session). All contexts reference these so offscreen
	// sessions never see a stale atlas; each TextBatch binds the page that
	// matches its AtlasIndex (Latin 0.., CJK cjkAtlasOffset..) instead of a
	// single shared view — a single view meant the last-uploaded page
	// shadowed all others when Latin and CJK text shared a frame.
	msdfAtlasTexes map[int]hal.Texture
	msdfAtlasViews map[int]hal.TextureView

	// Compute pipeline.
	velloAccel *VelloAccelerator

	pathGeomCache   *PathGeometryCache
	strokeGeomCache *StrokeGeometryCache
	dashGeomCache   *DashGeometryCache
	convexPathCache *ConvexPathCache

	// resReg is the device-level resource state machine shared by every window
	// session on this device (Skia GrContext: one cache, N surfaces). Sessions
	// register through their own owner handle, so a window close drops exactly
	// its entries; device loss invalidates it wholesale.
	resReg *res.Registry

	// Dual-texture advanced blend (B.03 Multiply/Screen/Overlay) pipeline cache.
	dualTexBlend           dualTexBlendCache
	maskR8                 maskR8Cache
	linearRampMask         linearRampMaskCache
	patternMaskSample      patternMaskSampleCache
	texturedStencilLinear  texturedStencilLinearCache
	texturedStencilPattern texturedStencilPatternCache
	filterGPU              filterGPUCache

	// surfaceCache is the device-level surface trio pool (P2.4d): session,
	// shared stencil renderer and textured stencil caches acquire same-size
	// MSAA/depth/resolve sets from this one cache instead of keeping four
	// private copies. Created lazily while s.mu is held (the struct only —
	// natives are allocated from Acquire, outside s.mu).
	surfaceCache *surfaceCache

	// direct is the device-level submission gate for the shared pools that
	// record and submit their own command buffers (filter graph, dual-tex
	// blend): it holds the pooled natives in flight until the queue reports
	// that work done, so a same-size reuse can never hand out a texture the
	// GPU is still writing.
	direct *directSubmit

	// poolLabel names this device holder in the pool probe output
	// (render.GPUPoolStats), so a multi-window diagnostic can tell which
	// holder's domains reported.
	poolLabel string

	// L.06 GPU-resident alpha mask (MaskAware). Full-surface R8 plane + texture.
	maskData   []byte
	maskW      int
	maskH      int
	maskTex    hal.Texture
	maskView   hal.TextureView
	maskActive bool
	maskKeyPtr uintptr
	maskKeyW   int
	maskKeyH   int

	// CPU SDF fallback accelerator.
	cpuFallback render.SDFAccelerator

	// sampleCount is the MSAA sample count resolved at GPU init time.
	// Normally 4 (4x MSAA). Falls back to 1 on backends that don't
	// support multisampled textures (e.g., software Vulkan / llvmpipe).
	// Resolved via resolveSampleCount() which probes the device.
	sampleCount uint32

	// msaaDowngraded latches when texture-allocation OOM forces the shared
	// pipeline set to 1x.
	msaaDowngraded bool

	deviceReady    bool              // device available for texture/buffer ops (true on all strategies incl. rasterAtlas)
	gpuReady       bool              // shape/text rendering pipelines initialized (false on rasterAtlas)
	softwareMode   bool              // true when software/CPU adapter detected (informational, does not disable GPU)
	strategy       gpuRenderStrategy // auto-detected rendering strategy
	externalDevice bool              // true when using shared device (don't destroy on Close)

	// deviceGen increments on every logical device install (SetDeviceProvider / initGPU).
	// Per-context GPURenderSession must rebuild when gen changes.
	deviceGen uint64

	// liveCtxs: per-window GPURenderContexts that hold session MSAA/depth textures.
	// AbandonExternalDevice must Destroy these *before* Device.Release or VRAM
	// stays pinned → CreateTexture OOM on the recovered device (minimize recover).
	liveCtxs map[*GPURenderContext]struct{}

	// fieldCache memoizes ColorAt gradient fields (C1): stable brushes stop
	// rebuilding a ≤512² RGBA buffer every fill.
	fieldCache fieldCache
}

// deviceSerial numbers device holders within the process, for the pool probe
// label only (no behavior depends on it).
var deviceSerial atomic.Uint64

// NewGPUShared creates a new shared GPU resource holder. GPU initialization
// is deferred until the first render or SetDeviceProvider call to avoid
// creating a standalone Vulkan device that may interfere with an external
// DX12/Metal device.
func NewGPUShared() *GPUShared {
	s := &GPUShared{
		resReg:         res.NewRegistry(),
		liveCtxs:       make(map[*GPURenderContext]struct{}),
		msdfAtlasTexes: make(map[int]hal.Texture),
		msdfAtlasViews: make(map[int]hal.TextureView),
		poolLabel:      fmt.Sprintf("device-%d", deviceSerial.Add(1)),
	}
	// The device-level shared pools (dual-tex snaps, filter render targets)
	// hold rebuildable VRAM and are the first thing resize churn piles up, so
	// they join the texture-OOM purge chain for as long as this holder lives.
	render.RegisterPurgeEvictable("gpu-shared-pools", s)
	// Same lifetime for the reuse probe: a real window asserts hits > 0 on the
	// domains it exercised instead of trusting that pooling happened.
	render.RegisterGPUPoolStatsCollector(s, s.poolStats)
	// ShaderEffect.Close evicts this holder's per-effect pipeline caches
	// (L2-P0-1); keyed by the pool label so GPUShared.Close unregisters it.
	render.RegisterShaderEffectEvictor(s.poolLabel, s.userShader.evictEffect)
	return s
}

// poolStats snapshots every res.Cache domain of this device holder: the
// device-level surface trio pool and shared pools, plus each live context's
// layer RT pool. Lock order matches the purge path — s.mu is held only while
// collecting the live objects, then released before any pool lock.
func (s *GPUShared) poolStats() []render.PoolDomainStats {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	surfaces := s.surfaceCache
	ctxs := make([]*GPURenderContext, 0, len(s.liveCtxs))
	for rc := range s.liveCtxs {
		ctxs = append(ctxs, rc)
	}
	label := s.poolLabel
	s.mu.Unlock()

	out := make([]render.PoolDomainStats, 0, 3+len(ctxs))
	appendDomain := func(st res.CacheStats) {
		if st.Name == "" {
			return // domain never built: nothing to report
		}
		out = append(out, render.PoolDomainStats{
			Device:    label,
			Name:      st.Name,
			Hits:      st.Hits,
			Misses:    st.Misses,
			Evictions: st.Evictions,
			Purged:    st.Purged,
			Entries:   st.Entries,
			Idle:      st.Idle,
			Bytes:     st.Bytes,
			PeakBytes: st.PeakBytes,
			Budget:    st.Budget,
		})
	}
	appendDomain(surfaces.stats())
	appendDomain(s.dualTexBlend.stats())
	appendDomain(s.filterGPU.stats())
	for _, rc := range ctxs {
		appendDomain(rc.offscreenStats())
	}
	return out
}

// PurgeEvictable drains this device's shared pool domains under VRAM pressure.
// It takes only the pool locks and never s.mu: the OOM retry chain runs inside
// texture creates that already hold the shared lock, and a purge that grabbed
// it would deadlock on itself.
func (s *GPUShared) PurgeEvictable() (freed int64) {
	if s == nil {
		return 0
	}
	return int64(s.dualTexBlend.releasePooledVRAM() + s.filterGPU.releasePooledVRAM())
}

// NewRenderContext creates a new per-context GPU render context that references
// this shared resource holder. Each render.Context should have its own
// GPURenderContext for isolated pending command queues and frame tracking.
func (s *GPUShared) NewRenderContext() *GPURenderContext {
	// GPU initialization is deferred to first Flush() or SetDeviceProvider().
	// This avoids creating a standalone Vulkan instance before gogpu has a
	// chance to provide its DeviceProvider (which may be software/CPU).
	rc := &GPURenderContext{
		shared:    s,
		antiAlias: true,
	}
	s.mu.Lock()
	if s.liveCtxs == nil {
		s.liveCtxs = make(map[*GPURenderContext]struct{})
	}
	s.liveCtxs[rc] = struct{}{}
	s.mu.Unlock()
	return rc
}

func (s *GPUShared) unregisterContext(rc *GPURenderContext) {
	if s == nil || rc == nil {
		return
	}
	s.mu.Lock()
	delete(s.liveCtxs, rc)
	s.mu.Unlock()
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
func (s *GPUShared) Device() hal.Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.device
}

// Queue returns the shared wgpu queue, or nil if not initialized.
func (s *GPUShared) Queue() hal.Queue {
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
// (e.g., gogpu). The provider's Device() must return a gpucontext handle
// wrapping a hal.Device.
//
// Software adapters (llvmpipe, SwiftShader, WARP) are treated as full GPU
// implementations per enterprise pattern (ADR-046): Skia Graphite runs CI on
// SwiftShader, wgpu treats CPU adapters identically, Flutter Impeller uses
// SwiftShader for testing. Capability differences are handled via probing
// (e.g., MSAA fallback to 1x), not blanket disabling.
func (s *GPUShared) SetDeviceProvider(provider gpucontext.DeviceProvider) error {
	if provider == nil {
		// Clearing the external provider is a no-op: keep current device binding.
		return nil
	}
	if adapter := provider.Adapter(); !adapter.IsNil() {
		if ai := provider.AdapterInfo(); ai.Type == gpucontext.AdapterTypeSoftware {
			slogger().Info("gpu-shared: software adapter detected — GPU features available, performance may be reduced",
				"adapter", ai.Name)
			s.mu.Lock()
			s.softwareMode = true
			s.mu.Unlock()
		}
	}

	dev := provider.Device()
	if dev.IsNil() {
		return fmt.Errorf("gpu-shared: provider Device is nil")
	}

	// Prefer the boxed hal.Device encoding (PackDevice); legacy raw-pointer
	// handles fall back to the concrete assert. Foreign boxed handles
	// fail closed to nil, never garbage.
	halDev := gpucontext.UnpackDevice(dev)
	if halDev == nil {
		halDev = webgpu.DeviceFromHandle(dev)
	}
	if halDev == nil {
		return fmt.Errorf("gpu-shared: provider Device handle is invalid")
	}
	halQueue := halDev.Queue()
	if halQueue == nil {
		return fmt.Errorf("gpu-shared: provider Queue is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Full Skia abandon of device-owned objects (pipelines, caches, pools, mask).
	// Incomplete teardown left dualTexBlend/mask caches + texture pool + sessions
	// on the sticky-lost device → "resource already released" after AutoRecover.
	s.abandonDeviceOwnedLocked(false /* releaseOwnedDevice */)

	// Use provided resources (caller owns device/queue lifetime).
	s.device = halDev
	s.queue = halQueue
	s.externalDevice = true
	s.deviceGen++

	// MSAA sample count for the window/external device comes straight from
	// render.MSAASampleCount() — the code-level config or the engine default
	// of 1x. No extra branching.
	s.sampleCount = resolveSampleCount(s.device)

	// Auto-detect rendering strategy.
	s.strategy = s.detectStrategy()

	s.deviceReady = true

	if s.strategy == strategyRasterAtlas {
		slogger().Info("gpu-shared: rasterAtlas strategy — SDF pipelines skipped, device ready for textures",
			"strategy", s.strategy.String(),
			"softwareMode", s.softwareMode,
		)
		return nil
	}

	// Lazy pipelines after external bind/recover (first Flush ensurePipelines).
	s.gpuReady = true
	s.registerFilterGraphIfNeeded()

	slogger().Info("gpu-shared: switched to shared GPU device",
		"strategy", s.strategy.String(),
		"msaa_samples", s.sampleCount,
		"deviceGen", s.deviceGen,
	)
	return nil
}

// AbandonExternalDevice drops every GPU object still bound to the current
// external device (live sessions + shared caches) and bumps deviceGen.
//
// Skia/Flutter order for device-lost recover:
//
//	abandon all GrGpuResources → destroy context → recreate.
//
// Session MSAA/depth must be Released *before* Device.Release; otherwise this
// native build keeps VRAM pinned and the new device OOMs on CreateTexture.

// PurgeAllSurfaceResources frees surface-bound GPU memory on every live
// context (depth/MSAA/offscreen pools) without abandoning the device.
// Call when the window is unpresentable.
func (s *GPUShared) PurgeAllSurfaceResources() {
	if s == nil {
		return
	}
	hal.WithForceNativeReleaseOnLost(func() {
		s.mu.Lock()
		ctxs := make([]*GPURenderContext, 0, len(s.liveCtxs))
		for rc := range s.liveCtxs {
			ctxs = append(ctxs, rc)
		}
		s.mu.Unlock()
		for _, rc := range ctxs {
			if rc != nil {
				rc.PurgeSurfaceResources()
			}
		}
	})
}

func (s *GPUShared) AbandonExternalDevice() {
	if s == nil {
		return
	}
	// Soft-native reclaim: child Release must run even when parent is sticky-lost.
	hal.WithForceNativeReleaseOnLost(func() {
		s.mu.Lock()
		ctxs := make([]*GPURenderContext, 0, len(s.liveCtxs))
		for rc := range s.liveCtxs {
			ctxs = append(ctxs, rc)
		}
		s.mu.Unlock()

		// Fully Close each GPURenderContext (Destroy session + unregister liveCtxs).
		// Destroy-only left zombie contexts; offscreen NewContext RTs then re-attached
		// half-state and OOM'd CreateTexture after AutoRecover on 1GB GPUs.
		for _, rc := range ctxs {
			if rc == nil {
				continue
			}
			rc.Close()
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		// liveCtxs should be empty after Close; clear defensively.
		if s.liveCtxs != nil {
			clear(s.liveCtxs)
		}
		s.abandonDeviceOwnedLocked(false)
		s.deviceGen++
	})
}

// CanRenderDirect reports whether the GPU is initialized and can render
// directly to a surface. Returns false when the rendering strategy is
// strategyRasterAtlas (software/CPU adapters) — shapes route to CPU
// rasterizer instead.
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
	if s.device == nil || !s.deviceReady {
		return false
	}
	if s.velloAccel == nil {
		s.initVelloAccelerator(s.device, s.queue)
	}
	return s.velloAccel != nil && s.velloAccel.CanCompute()
}

// abandonDeviceOwnedLocked drops every GPU object tied to the current device.
// Caller holds s.mu. releaseOwnedDevice=true releases owned instance/adapter/device
// (Close path for non-external). false keeps external device ownership with the host
// (SetDeviceProvider switch / AutoRecover).
//
// Must stay complete: partial teardown after AutoRecover left session/pipeline
// caches on a released device → "resource already released" on CreateShaderModule.
func (s *GPUShared) abandonDeviceOwnedLocked(releaseOwnedDevice bool) {
	if s == nil {
		return
	}
	// Best-effort drain only while device can still Poll.
	if s.device != nil && !s.device.IsLost() {
		_ = s.device.WaitIdle()
	}

	// Device-owned registry entries point at natives the teardown below
	// destroys directly (shared caches/pipelines): drop the bookkeeping
	// without Release (single release outlet). Session-owned entries stay
	// with their sessions, which release them on Destroy.
	//
	// Surface trio + offscreen caches hold device-owned natives: a healthy
	// switch force-releases them (device still alive), device loss only drops
	// bookkeeping (the abandon flow owns native teardown).
	deviceLost := s.device != nil && s.device.IsLost()
	// Shared-pool submits (filter/dualtex) may still be held in flight. The
	// WaitIdle above proved the GPU finished, so a healthy switch resolves
	// them normally; a lost device never gets its fence, so the holds drop
	// without touching in-flight counters (the abandon flow owns teardown).
	if deviceLost {
		s.direct.invalidate()
	} else {
		s.direct.drain()
	}
	if s.surfaceCache != nil {
		if deviceLost {
			s.surfaceCache.invalidateAll()
		} else {
			s.surfaceCache.releaseAll()
		}
	}
	for rc := range s.liveCtxs {
		if rc != nil {
			rc.abandonOffscreenCache(deviceLost)
		}
	}
	s.resReg.DeviceOwner().InvalidateOwned()

	s.textEngine = nil
	for _, v := range s.msdfAtlasViews {
		v.Destroy()
	}
	for _, t := range s.msdfAtlasTexes {
		t.Destroy()
	}
	// Keep the maps non-nil: syncTextAtlases registers newly created pages
	// with `s.msdfAtlasTexes[idx] = tex` after a device switch. Nil maps
	// panicked on the first MSDF glyph following SetDeviceProvider.
	s.msdfAtlasViews = make(map[int]hal.TextureView)
	s.msdfAtlasTexes = make(map[int]hal.Texture)
	if s.glyphMaskEngine != nil {
		if s.device != nil {
			s.glyphMaskEngine.Destroy(s.device)
		}
		s.glyphMaskEngine = nil
	}
	if s.velloAccel != nil {
		s.velloAccel.Close()
		s.velloAccel = nil
	}
	s.pathGeomCache = nil
	s.strokeGeomCache = nil
	s.dashGeomCache = nil
	s.convexPathCache = nil
	s.dualTexBlend.release()
	s.maskR8.release()
	s.linearRampMask.release()
	s.patternMaskSample.release()
	s.texturedStencilLinear.release()
	s.texturedStencilPattern.release()
	s.filterGPU.release()
	s.userShader.destroyLocked()
	s.clearMaskLocked()
	s.destroyPipelinesLocked()

	oldExternal := s.externalDevice
	if releaseOwnedDevice && !oldExternal {
		if s.device != nil {
			s.device.Release()
		}
		if s.adapter != nil {
			s.adapter.Release()
		}
		if s.instance != nil {
			s.instance.Release()
		}
	}
	// Drop Go refs; external device lifetime stays with the window/swapchain host.
	s.device = nil
	s.queue = nil
	s.adapter = nil
	s.instance = nil
	s.gpuReady = false
	s.deviceReady = false
}

// installDeviceGuardLocked runs the rebind half of the device funnel: when the
// incoming device is not the one already bound, everything the old device owns
// has to be torn down before the caller installs the new one. Caller holds s.mu.
//
// SetDeviceProvider always abandoned; the standalone paths (ensureGPU → initGPU)
// used to assign a new device with no abandon at all, so cached device-owned
// handles — shared clip/mask bind group layouts first — survived the swap and
// were fed to the new device. wgpu-native treats that as a non-unwinding abort,
// not an error return (H19). Identity decides because abandoning on an identical
// rebind would throw away caches that are still valid for this device.
func (s *GPUShared) installDeviceGuardLocked(dev hal.Device) {
	if s.device == nil || dev == nil || s.device == dev {
		return
	}
	// abandonDeviceOwnedLocked releases the old device only when this shared
	// holder opened it (externalDevice), so borrowed window devices stay alive.
	s.abandonDeviceOwnedLocked(true /* releaseOwnedDevice */)
}

// Close releases all shared GPU resources. After this call, GPU rendering
// is no longer possible. Idempotent.
func (s *GPUShared) Close() {
	render.UnregisterPurgeEvictable(s)
	render.UnregisterGPUPoolStatsCollector(s)
	render.UnregisterShaderEffectEvictor(s.poolLabel)
	s.mu.Lock()
	defer s.mu.Unlock()

	s.abandonDeviceOwnedLocked(true /* releaseOwnedDevice */)
	s.sampleCount = 0
	s.strategy = 0
	s.softwareMode = false
	s.externalDevice = false
}

func (s *GPUShared) SampleCount() uint32 {
	if s.sampleCount == 0 {
		// Before init: use the configured value or engine default (1x).
		return render.MSAASampleCount()
	}
	return s.sampleCount
}

// RequestMSAADowngrade drops the shared MSAA sample count to 1 after a
// device-memory OOM (e.g., multi-window stolen-memory budget on iGPUs).
// of failing frames. The 4x pipelines are destroyed so ensurePipelines
// rebuilds them at 1x, and deviceGen bumps so every render context rebuilds
// its session at the lower sample count on the next flush. Idempotent; the
// first OOM latches the downgrade.
func (s *GPUShared) RequestMSAADowngrade() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.msaaDowngraded || s.sampleCount <= 1 {
		return
	}
	s.msaaDowngraded = true
	s.sampleCount = 1
	// Drain so pooled 4x textures are safe to destroy, then release every
	// 4x-bound shared resource: shape pipelines and glyph atlas (sessions
	// rebuild their 4x surface trios on the deviceGen bump below).
	if s.device != nil && !s.device.IsLost() {
		_ = s.device.WaitIdle()
	}
	s.destroyPipelinesLocked()
	if s.glyphMaskEngine != nil {
		s.glyphMaskEngine.Destroy(s.device)
		s.glyphMaskEngine = nil
	}
	s.deviceGen++
	slogger().Info("gpu-shared: OOM — MSAA downgraded to 1x, sessions rebuild at 1x")
}

// detectStrategy determines the rendering strategy based on adapter type and
// MSAA support. Must be called with s.mu held, after softwareMode and
// sampleCount are resolved.
//
// This follows the Skia Graphite RendererProvider pattern
//
//	prefer compute > tessellation > raster atlas
//
// Our equivalent:
//
//	softwareMode=false, MSAA=4x → strategyFull (all GPU tiers)
//	softwareMode=false, MSAA=1x → strategyNoMSAA (GPU without MSAA)
//	softwareMode=true → strategyRasterAtlas (CPU shapes, GPU textures)
func (s *GPUShared) detectStrategy() gpuRenderStrategy {
	if s.softwareMode {
		return strategyRasterAtlas
	}
	if s.sampleCount <= 1 {
		return strategyNoMSAA
	}
	return strategyFull
}

// resolveSampleCount returns the MSAA sample count for a GPU session:
// render.MSAASampleCount() — the code-level config or the engine default of
// 1x. No device probing: the
// engine default is 1 sample per pixel; 4x is an explicit opt-in via
// SetMSAASampleCount.
func resolveSampleCount(_ hal.Device) uint32 {
	return render.MSAASampleCount()
}

// ensureGPU lazily initializes a standalone GPU device if no shared device
// was provided. Must be called with s.mu held.
func (s *GPUShared) ensureGPU() error {
	// J5 campaign: a Released device reports IsLost — re-create instead of
	// handing sessions a released device (the stale wrapper SIGSEGVs on the
	// next CreateTexture).
	if s.device != nil && !s.device.IsLost() {
		s.registerFilterGraphIfNeeded()
		return nil
	}
	if s.device != nil {
		// The bound device is lost (its owner released it): tear down everything
		// it owns before re-creating. Dropping only the Go reference left the
		// shared clip/mask layouts and the device-owned caches pointing at the
		// dead device, which the next install then reused (H19's abort path).
		s.abandonDeviceOwnedLocked(false /* already released by its owner */)
	}
	if err := s.initGPU(); err != nil {
		return err
	}
	s.registerFilterGraphIfNeeded()
	return nil
}

// ensureSharedLayoutsLocked creates the shared clip/mask layouts for the device
// currently bound. Must be called with s.mu held and a live device; sessions
// borrow the handles and never destroy them.
func (s *GPUShared) ensureSharedLayoutsLocked() error {
	if s.device == nil {
		return fmt.Errorf("gpu-shared: no device for shared layouts")
	}
	if s.sharedLayoutsDev != s.device {
		// The device changed under us (install, loss-recreate, provider swap):
		// the cached pair belongs to the previous one. Recreating beats
		// handing those handles to this device's CreatePipelineLayout, which
		// aborts inside wgpu-native rather than returning an error (H19).
		// Sessions rebuild on the deviceGen bump that accompanies every install.
		s.destroySharedLayoutsLocked()
	}
	if s.clipBindLayout == nil {
		layout, err := createClipBindGroupLayout(s.device, "shared_clip_bind_layout")
		if err != nil {
			return fmt.Errorf("shared clip bind layout: %w", err)
		}
		s.clipBindLayout = layout
	}
	if s.maskBindLayout == nil {
		layout, err := createMaskBindGroupLayout(s.device, "shared_mask_bind_layout")
		if err != nil {
			return fmt.Errorf("shared mask bind layout: %w", err)
		}
		s.maskBindLayout = layout
	}
	s.sharedLayoutsDev = s.device
	return nil
}

// destroySharedLayoutsLocked releases the shared clip/mask layouts, if any.
// Caller holds s.mu. Single release outlet for the pair.
func (s *GPUShared) destroySharedLayoutsLocked() {
	if s.maskBindLayout != nil {
		s.maskBindLayout.Destroy()
		s.maskBindLayout = nil
	}
	if s.clipBindLayout != nil {
		s.clipBindLayout.Destroy()
		s.clipBindLayout = nil
	}
	s.sharedLayoutsDev = nil
}

// surfaceCacheLocked returns the device-level surface trio cache, creating the
// (empty) cache on first use. Caller holds s.mu. No native allocation happens
// here: res.Cache runs create from Acquire, outside s.mu.
func (s *GPUShared) surfaceCacheLocked() *surfaceCache {
	if s == nil {
		return nil
	}
	if s.surfaceCache == nil {
		s.surfaceCache = newSurfaceCache(s.resReg.DeviceOwner(), "device-surface")
	}
	return s.surfaceCache
}

// directSubmitLocked returns the device-level submission gate for the shared
// pools that submit outside a session frame. Caller holds s.mu.
func (s *GPUShared) directSubmitLocked() *directSubmit {
	if s == nil {
		return nil
	}
	if s.direct == nil {
		s.direct = newDirectSubmit(s.resReg)
	}
	return s.direct
}

// dualTexBlendLocked returns the device-level dual-tex blend cache, bound to
// this device's registry owner and submission gate (idempotent). Caller holds
// s.mu; the binding installs pointers only — no native allocation.
func (s *GPUShared) dualTexBlendLocked() *dualTexBlendCache {
	if s == nil {
		return nil
	}
	c := &s.dualTexBlend
	c.bindRes(s.resReg.DeviceOwner(), s.directSubmitLocked())
	return c
}

// filterGPULocked returns the device-level filter graph cache, bound to this
// device's registry owner and submission gate. Caller holds s.mu.
func (s *GPUShared) filterGPULocked() *filterGPUCache {
	if s == nil {
		return nil
	}
	c := &s.filterGPU
	c.bindRes(s.resReg.DeviceOwner(), s.directSubmitLocked())
	return c
}

// wireStencilSurfaceCacheLocked binds the shared stencil renderer's texture

// set to the device-level trio cache. That renderer submits outside
// res.Submission, so its sets retire on rebuild instead of pooling.
func (s *GPUShared) wireStencilSurfaceCacheLocked() {
	if s == nil || s.stencilRenderer == nil {
		return
	}
	s.stencilRenderer.setSurfaceCache(s.surfaceCacheLocked())
}

// ensurePipelines lazily creates shape rendering pipelines. Skipped on
// rasterAtlas — SDF/stencil/convex pipelines hang on software SPIR-V
// interpreter. Must be called with s.mu held.
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
	s.wireStencilSurfaceCacheLocked()
	// Base pipelines are built once here under mu: every window's session
	// then only reads pipeline handles. Lazy per-frame create/destroy on the
	// shared objects is what let one window's close free handles another
	// window's frame was creating. Variant pipelines (depth-clip /
	// cover-blend) stay lazy behind each renderer's own creation lock.
	// Build failures only log; sessions retry on the next flush through
	// EnsureSharedBase.
	if s.device == nil {
		return
	}
	if err := s.ensureSharedLayoutsLocked(); err != nil {
		slogger().Warn("gpu-shared: shared layouts", "err", err)
		return
	}
	s.sdfRenderPipeline.SetClipBindLayout(s.clipBindLayout)
	s.sdfRenderPipeline.SetMaskBindLayout(s.maskBindLayout)
	if err := s.sdfRenderPipeline.EnsureSharedBase(); err != nil {
		slogger().Warn("gpu-shared: sdf base pipelines", "err", err)
	}
	s.convexRenderer.SetClipBindLayout(s.clipBindLayout)
	s.convexRenderer.SetMaskBindLayout(s.maskBindLayout)
	if err := s.convexRenderer.EnsureSharedBase(); err != nil {
		slogger().Warn("gpu-shared: convex base pipelines", "err", err)
	}
	s.stencilRenderer.SetClipBindLayout(s.clipBindLayout)
	s.stencilRenderer.SetMaskBindLayout(s.maskBindLayout)
	if err := s.stencilRenderer.EnsureSharedBase(); err != nil {
		slogger().Warn("gpu-shared: stencil base pipelines", "err", err)
	}
}

// ensureGlyphMaskEngine lazily creates the glyph mask engine. Must be called
// with s.mu held.
func (s *GPUShared) ensureGlyphMaskEngine() {
	if s.glyphMaskEngine == nil {
		s.glyphMaskEngine = NewGlyphMaskEngine()
	}
}

// ensureColorGlyphEngine lazily creates the color glyph engine. Must be called
// with s.mu held.
func (s *GPUShared) ensureColorGlyphEngine() {
	if s.colorGlyphEngine == nil {
		s.colorGlyphEngine = NewColorGlyphEngine()
	}
}

// ensureTextEngine lazily creates the text engine. Must be called with s.mu held.
func (s *GPUShared) ensureTextEngine() {
	if s.textEngine == nil {
		s.textEngine = NewGPUTextEngine()
	}
}

// initGPU opens the standalone (probe/offscreen) device. Adapter choice
// follows the window policy and honors a live present share: when a window
// already published the shared device, reuse it instead of opening a second
// device on possibly another adapter (single device, N surfaces —
// present_target.go buildSharedSurface). Window opens publish the share;
// probe/offscreen passes run before any window exists, borrow when present,
// open policy-selected only when absent.
func (s *GPUShared) initGPU() error {
	// A live window already published the shared present device: borrow it
	// (same device, N surfaces) instead of opening a second device.
	if inst, adpt, dev, borrowable, _ := render.AcquireSharedPresentDevice(); borrowable && dev != nil {
		s.installDeviceGuardLocked(dev)
		s.instance = inst
		s.adapter = adpt
		s.device = dev
		s.queue = dev.Queue()
		s.externalDevice = true
		s.deviceGen++
		s.sampleCount = resolveSampleCount(s.device)
		s.strategy = s.detectStrategy()
		s.deviceReady = true
		if s.strategy == strategyRasterAtlas {
			return nil
		}
		s.gpuReady = true
		s.registerFilterGraphIfNeeded()
		slogger().Info("gpu-shared: borrowed shared present device for standalone use")
		return nil
	}
	// Standalone open goes through the single creation funnel
	// (render.OpenStandaloneDevice, P2.4a): strict Vulkan mask, window-policy
	// adapter selection (Default prefers integrated on hybrid machines so a
	// probe device never parks on the small dGPU while windows present on
	// the iGPU), OOM-retried device request. Cleanup on partial failure
	// moved into the entry.
	sd, err := render.OpenStandaloneDevice("gg-shared")
	if err != nil {
		return err
	}
	// Same funnel as SetDeviceProvider: when this is not the device we already
	// hold, everything the old one owned (pipelines, shared layouts, surface
	// trio, mask/dual-tex/filter caches) has to go before the new one is bound.
	s.installDeviceGuardLocked(sd.Device)
	s.instance = sd.Instance
	s.adapter = sd.Adapter

	// Check for software/CPU adapter before creating device.
	adapterInfo := sd.Adapter.Info()
	if sd.ForceFallback || adapterInfo.DeviceType == types.DeviceTypeCPU {
		slogger().Info("gpu-shared: software adapter detected, SDF pipeline disabled",
			"adapter", adapterInfo.Name)
		s.softwareMode = true
	}

	s.device = sd.Device
	s.queue = sd.Device.Queue()
	// This one we opened ourselves: borrow-after-window bookkeeping from a
	// previous install must not mark it external, or nothing releases it.
	s.externalDevice = false
	s.deviceGen++

	// Probe MSAA support.
	s.sampleCount = resolveSampleCount(s.device)

	// Auto-detect rendering strategy.
	s.strategy = s.detectStrategy()

	s.deviceReady = true

	// Create pipelines (device stays alive for texture ops even in softwareMode).
	s.sdfRenderPipeline = NewSDFRenderPipeline(s.device, s.queue, s.sampleCount)
	s.convexRenderer = NewConvexRenderer(s.device, s.queue, s.sampleCount)
	s.stencilRenderer = NewStencilRenderer(s.device, s.queue, s.sampleCount)

	s.gpuReady = true

	// Vello compute is initialized lazily on first CanCompute/compute path
	// (initVelloAccelerator). Eager init of 10 compute stages is costly on
	// low-VRAM hosts and is not needed for the default render-pass UI path.

	// Standalone init: ensure filter graph is available even if callers
	// never re-enter ensureGPU after device assignment.
	s.registerFilterGraphIfNeeded()

	slogger().Info("gpu-shared: GPU initialized",
		"strategy", s.strategy.String(),
		"adapter", adapterInfo.Name,
		"msaa_samples", s.sampleCount,
		"softwareMode", s.softwareMode,
	)
	return nil
}

func (s *GPUShared) initVelloAccelerator(device hal.Device, queue hal.Queue) {
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
	// Shared layouts die with the pipelines that embed them. Sessions that
	// borrowed the handles re-borrow the recreated ones on the next flush.
	s.destroySharedLayoutsLocked()
}

// LockSubmit acquires the present-submit serialization lock (leaf lock:
// never hold s.mu while acquiring it).
func (s *GPUShared) LockSubmit() {
	if s == nil {
		return
	}
	s.submitMu.Lock()
}

// UnlockSubmit releases the present-submit serialization lock.
func (s *GPUShared) UnlockSubmit() {
	if s == nil {
		return
	}
	s.submitMu.Unlock()
}

// submitLocked runs fn with the present-submit serialization held (one
// Queue.Submit + error-scope Push/Pop at a time on the shared device).
// Converges the context-level submit sites onto a single lock discipline;
// the session error-scope path holds the same mutex via submitLock.
func (s *GPUShared) submitLocked(fn func() (uint64, error)) (uint64, error) {
	s.LockSubmit()
	defer s.UnlockSubmit()
	return fn()
}

// GPUMemoryStats holds diagnostic information about GPU resource usage.
type GPUMemoryStats struct {
	TilePoolPooled int
	// ImageCache* filled when a live session image cache is reported via session; shared path leaves 0.
	ImageCacheEntries int
	ImageCacheBytes   int64
	ImageCacheHits    uint64
	ImageCacheMisses  uint64
	ImageUploadBytes  uint64
}

// MemoryStats returns GPU memory statistics for diagnostics.
func (s *GPUShared) MemoryStats() GPUMemoryStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := GPUMemoryStats{}
	stats.TilePoolPooled = globalTilePool.Stats().Pooled
	return stats
}

// clearMaskLocked releases GPU mask resources. Caller must hold s.mu.
func (s *GPUShared) clearMaskLocked() {
	if s.maskView != nil {
		s.maskView.Destroy()
		s.maskView = nil
	}
	if s.maskTex != nil {
		s.maskTex.Destroy()
		s.maskTex = nil
	}
	s.maskData = nil
	s.maskW = 0
	s.maskH = 0
	s.maskActive = false
	s.maskKeyPtr = 0
	s.maskKeyW = 0
	s.maskKeyH = 0
}

// deactivateMaskLocked stops sampling the mask without destroying its R8
// texture/plane. Clip masks are installed around every masked fill and removed
// in the cleanup callback; keeping the upload lets the next fill with the same
// clip (R7 replays one viewport clip across many pictures) reactivate the
// existing texture instead of copying ~1MB and re-uploading it each time.
// Caller must hold s.mu.
func (s *GPUShared) deactivateMaskLocked() {
	s.maskActive = false
}

// maskDataKey identifies the caller-owned plane. A new upload is required when
// the pointer or dimensions change. Mutating a Mask in place and re-setting it
// must go through Context.SetMask, which invalidates this cache explicitly.
func maskDataKey(data []byte) uintptr {
	if len(data) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&data[0]))
}

// SetMaskTexture uploads a full-surface R8 alpha mask (L.06 MaskAware).
// data is width*height bytes; nil deactivates the mask but keeps its texture
// for cheap reactivation. Pass through ClearMaskTexture to release memory.
func (s *GPUShared) SetMaskTexture(data []byte, width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if data == nil || width <= 0 || height <= 0 {
		s.deactivateMaskLocked()
		return
	}
	need := width * height
	if len(data) < need {
		s.deactivateMaskLocked()
		return
	}

	key := maskDataKey(data)
	samePlane := s.maskData != nil &&
		s.maskKeyPtr == key && s.maskKeyW == width && s.maskKeyH == height
	if samePlane {
		// Reactivate a retained mask without copying or re-uploading. If the
		// earlier GPU upload failed, maskTex stays nil and HasGPUMask reports
		// false; fillMaskedAsImage then uses the CPU plane.
		s.maskActive = true
		return
	}

	s.clearMaskLocked()
	s.maskData = append([]byte(nil), data[:need]...)
	s.maskW = width
	s.maskH = height
	s.maskKeyPtr = key
	s.maskKeyW = width
	s.maskKeyH = height
	s.maskActive = true

	if err := s.ensureGPU(); err != nil || s.device == nil || s.queue == nil {
		// CPU plane retained for fillMaskedAsImage region sampling.
		return
	}

	tex, err := s.device.CreateTexture(&hal.TextureDescriptor{
		Label: "l06_mask_r8",
		Size: hal.Extent3D{
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
	view, err := s.device.CreateTextureView(tex, &hal.TextureViewDescriptor{
		Label:         "l06_mask_r8_view",
		Format:        types.TextureFormatR8Unorm,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		tex.Destroy()
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
		&hal.ImageCopyTexture{Texture: tex, MipLevel: 0},
		upload,
		&hal.ImageDataLayout{BytesPerRow: aligned, RowsPerImage: uint32(height)},           //nolint:gosec
		&hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}, //nolint:gosec
	); err != nil {
		view.Destroy()
		tex.Destroy()
		return
	}
	s.maskTex = tex
	s.maskView = view
}

// ClearMaskTexture removes the GPU-resident alpha mask and releases its plane.
func (s *GPUShared) ClearMaskTexture() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearMaskLocked()
}

// MaskPlane returns a borrowed view of the active R8 mask plane when set.
// The slice is read-only and valid until the next SetMaskTexture /
// ClearMaskTexture on this shared context (same raster thread). ok is false
// when no mask is active.
func (s *GPUShared) MaskPlane() (data []byte, w, h int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.maskActive || s.maskW <= 0 || s.maskH <= 0 || len(s.maskData) < s.maskW*s.maskH {
		return nil, 0, 0, false
	}
	return s.maskData[:s.maskW*s.maskH], s.maskW, s.maskH, true
}

// HasGPUMask reports whether a native R8 mask texture is currently bound.
func (s *GPUShared) HasGPUMask() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maskActive && s.maskTex != nil && s.maskView != nil
}

// MaskTextureView returns the active R8 mask view when a GPU mask is bound.
// Caller must not release the view; ownership remains with GPUShared.
func (s *GPUShared) MaskTextureView() (hal.TextureView, bool) {
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
		cache := s.filterGPULocked()
		s.mu.Unlock()
		return runGPUFilterGraph(device, queue, cache, src, w, h, nodes)
	})
	render.RegisterGPUFilterGraphTexture(func(src []byte, w, h int, nodes []render.ImageFilterNode) (gpucontext.TextureView, func(), error) {
		s.mu.Lock()
		if err := s.ensureGPU(); err != nil {
			s.mu.Unlock()
			return gpucontext.TextureView{}, nil, err
		}
		device, queue := s.device, s.queue
		cache := s.filterGPULocked()
		s.mu.Unlock()
		return runGPUFilterGraphGPUOnly(device, queue, cache, src, w, h, nodes)
	})
	render.RegisterGPUFilterGraphFromView(func(srcView gpucontext.TextureView, w, h int, nodes []render.ImageFilterNode) (gpucontext.TextureView, func(), error) {
		s.mu.Lock()
		if err := s.ensureGPU(); err != nil {
			s.mu.Unlock()
			return gpucontext.TextureView{}, nil, err
		}
		device, queue := s.device, s.queue
		cache := s.filterGPULocked()
		s.mu.Unlock()
		return runGPUFilterGraphFromView(device, queue, cache, srcView, w, h, nodes)
	})
	gpuFilterGraphRegistered = true
}

// PathGeomCache returns the shared path tessellation cache, creating if needed.
func (s *GPUShared) PathGeomCache() *PathGeometryCache {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pathGeomCache == nil {
		s.pathGeomCache = NewPathGeometryCache()
	}
	return s.pathGeomCache
}

// StrokeGeomCache returns the shared stroke expansion cache.
func (s *GPUShared) StrokeGeomCache() *StrokeGeometryCache {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.strokeGeomCache == nil {
		s.strokeGeomCache = NewStrokeGeometryCache()
	}
	return s.strokeGeomCache
}

// DashGeomCache returns the shared dash geometry cache.
func (s *GPUShared) DashGeomCache() *DashGeometryCache {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dashGeomCache == nil {
		s.dashGeomCache = NewDashGeometryCache()
	}
	return s.dashGeomCache
}

// ConvexPathCache returns the shared convex classification cache.
func (s *GPUShared) ConvexPathCache() *ConvexPathCache {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.convexPathCache == nil {
		s.convexPathCache = NewConvexPathCache()
	}
	return s.convexPathCache
}

// GeometryCacheStats aggregates path/stroke/dash/convex cache counters.
func (s *GPUShared) GeometryCacheStats() GeometryCacheStats {
	if s == nil {
		return GeometryCacheStats{}
	}
	var st GeometryCacheStats
	if c := s.PathGeomCache(); c != nil {
		st.PathHits, st.PathMisses, st.PathEntries = c.Stats()
	}
	if c := s.StrokeGeomCache(); c != nil {
		st.StrokeHits, st.StrokeMisses, st.StrokeEntries = c.Stats()
	}
	if c := s.DashGeomCache(); c != nil {
		st.DashHits, st.DashMisses, st.DashEntries = c.Stats()
	}
	if c := s.ConvexPathCache(); c != nil {
		st.ConvexHits, st.ConvexMisses, st.ConvexEntries = c.Stats()
	}
	return st
}

// ResetGeometryCacheStats clears hit/miss counters for geometry caches.
func (s *GPUShared) ResetGeometryCacheStats() {
	if s == nil {
		return
	}
	if c := s.PathGeomCache(); c != nil {
		c.ResetStats()
	}
	if c := s.StrokeGeomCache(); c != nil {
		c.ResetStats()
	}
	if c := s.DashGeomCache(); c != nil {
		c.ResetStats()
	}
	if c := s.ConvexPathCache(); c != nil {
		c.ResetStats()
	}
}
