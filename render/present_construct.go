//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"

	// Register the pure-Go GL backend for SelectBackend (BackendGo).
	_ "github.com/energye/gpui/gpu/gwgpu/gles"
)

// requestPresentDeviceWithRetry retries device creation when the adapter is
// temporarily out of GPU memory (multi-window stolen-memory budget on iGPUs).
// Other windows/processes may release memory between retries; this mirrors
// the degrade-not-crash behavior on transient resource pressure.
// Implemented here rather than render/internal/gpu to avoid an import cycle.
func requestPresentDeviceWithRetry(adapter hal.Adapter, desc *hal.DeviceDescriptor, label string) (hal.Device, error) {
	if adapter == nil {
		return nil, fmt.Errorf("adapter is nil")
	}
	var device hal.Device
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		device, err = adapter.RequestDevice(desc)
		if err == nil {
			return device, nil
		}
		if !IsGPUOutOfMemory(err) {
			return nil, err
		}
		// Present device creation is not on a hot path; a short backoff
		// gives other processes time to release GPU memory.
		time.Sleep(time.Second)
	}
	return nil, err
}

// waitDeviceReady polls a 1x1 probe texture until the driver heap accepts
// allocations or the deadline passes. Deadline from GPUI_GPU_READY_TIMEOUT_MS
// (default 8000, 0 = skip the gate). Timeout returns an OOM-class error so
// callers degrade (CPU fallback) instead of dying on the first real texture.
// Open-time half of the single OOM policy (render.OOMExitThreshold): the gate
// degrades through present levels here, the runtime present loop exits after
// the same threshold — one threshold and one OOM phrasing decide both.
// waitDeviceReady polls a 1x1 probe texture until the driver heap accepts
// allocations or the deadline passes. Deadline from GPUI_GPU_READY_TIMEOUT_MS
// (default 8000, 0 = skip the gate). Timeout returns an OOM-class error so
// callers degrade (CPU fallback) instead of dying on the first real texture.
// Open-time half of the single OOM policy (render.OOMExitThreshold): the gate
// degrades through present levels here, the runtime present loop exits after
// the same threshold — one threshold and one OOM phrasing decide both.
func waitDeviceReady(inst hal.Instance, device hal.Device, label string) error {
	deadlineMs := int64(8000)
	if v := os.Getenv("GPUI_GPU_READY_TIMEOUT_MS"); v != "" {
		var n int64
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n >= 0 {
			deadlineMs = n
		}
	}
	if deadlineMs <= 0 || device == nil {
		return nil
	}
	deadline := time.Now().Add(time.Duration(deadlineMs) * time.Millisecond)
	probed := false
	for {
		tex, err := device.CreateTexture(&hal.TextureDescriptor{
			Label:         label + "_ready_probe",
			Size:          hal.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
			MipLevelCount: 1,
			SampleCount:   1,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatBGRA8Unorm,
			Usage:         types.TextureUsageCopySrc,
		})
		if err == nil {
			tex.Destroy()
			if probed {
				fmt.Fprintf(os.Stderr, "render: device ready after reclaim wait (%s)\n", label)
			}
			return nil
		}
		if !IsGPUOutOfMemory(err) {
			return fmt.Errorf("render: device readiness probe failed (%s): %w", label, err)
		}
		probed = true
		if inst != nil {
			inst.ProcessEvents()
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("render: GPU heap not reclaiming for %s after %dms: %w; close other GPU windows or set GPUI_POWER to a less-loaded GPU",
				label, deadlineMs, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// PresentPlatform identifies the native windowing backend for surface creation.
type PresentPlatform int

const (
	PresentPlatformX11 PresentPlatform = iota
	PresentPlatformWayland
	PresentPlatformWin32
	PresentPlatformAppKit
)

// PresentNativeSurface holds OS handles required to create a GPU present surface.
//
//	Linux X11: Display=Display*, Window=Window (XID)
//	Linux Wayland: Display=wl_display*, Window=wl_surface*
//	Windows: Display=0 or HINSTANCE, Window=HWND
//	macOS: Display=0, Window=CAMetalLayer* / NSView* per gpu binding
type PresentNativeSurface struct {
	Platform PresentPlatform
	Display  uintptr
	Window   uintptr
}

// PresentTarget is a render-facing present surface for L1 UI (ui must not import gpu).
// Owns surface/swapchain + a drawing Context. The wgpu instance/adapter/device
// are shared per process (sharedPresentDevice below, Skia GrDirectContext
// pattern): every window's surface is created on the same instance and bound
// to the same device, so N windows present concurrently without N devices.
// The shared device is released when the last PresentTarget closes.
type PresentTarget struct {
	mu sync.Mutex

	ns     PresentNativeSurface
	logicW int
	logicH int
	scale  float64

	inst    hal.Instance
	adapter hal.Adapter
	device  hal.Device
	// shared marks instance/adapter/device as borrowed from the process
	// share (Close must not release them; the share does on last close).
	shared bool
	surf   hal.Surface
	sc     hal.Swapchain
	dc     *Context

	closed bool

	// postResizeFull counts full frames still owed to swapchain buffers after a
	// physical reconfigure. Reconfiguring a swapchain leaves every buffer's
	// content undefined (Vulkan VK_IMAGE_LAYOUT_UNDEFINED semantics; Skia
	// requires a full repaint after surface recreate). Retained damage frames
	// LoadOpLoad their surface, so any buffer that was not fully written since
	// the reconfigure shows stale/black pixels. The count is decremented only
	// after a successful EndFrame (failed/timeout BeginFrames do not consume
	// budget), and is 3 to cover double/triple buffering.
	postResizeFull int

	// resizeStormWindow / lastResizeAt implement storm-aware full recovery:
	// while a resize storm is active (each step arrives faster than the window,
	// e.g. continuous drag-resize), the fixed 3-frame budget can be exhausted
	// between steps and a retained damage frame then LoadOpLoads a buffer that
	// was never fully written at the new size → black/aliased edges. The window
	// forces the full path for every present while the last resize is fresher
	// than resizeStormWindow, so a storm stays fully written; after the storm
	// ends, the last resize re-armed postResizeFull=3 covers the tail.
	resizeStormWindow time.Duration
	lastResizeAt      time.Time

	// swapchainPending marks a resize recorded by the UI thread (Resize) that
	// the raster thread must apply at the next present boundary, serialized
	// with BeginFrame/EndFrame (Skia/Flutter: the raster thread owns the
	// surface). This keeps the expensive wgpu Surface.Configure off the UI
	// thread (a ~40–100ms stall per resize step on llvmpipe) and removes the
	// UI-configure-vs-raster-present race that left the swapchain stuck at a
	// stale extent during interactive resize drags (frames then dropped on
	// "surface outdated" and content froze at the old size).
	swapchainPending bool

	// vsyncOn / vsyncPending implement the runtime present-mode switch
	// (SetVsync): Fifo steady ↔ Mailbox/Immediate while an interactive resize
	// storm is active. Like swapchainPending it is record-only — the mode
	// change is applied by applyPendingSwapchainLocked at the next present
	// boundary on the raster thread, so the UI thread never blocks on the
	// surface.
	//
	// fixedNoVsync: platforms where present is permanently non-blocking
	// (Wayland — the compositor swaps at vblank, a client-side Fifo wait
	// would only block; ENGINE_FRAME_PRESENT_STANDARD.md 块3). SetVsync is a
	// no-op there and the initial present mode is FifoRelaxed-preferring.
	vsyncOn      bool
	vsyncPending bool
	fixedNoVsync bool

	// onSwapchainResized is invoked on the raster thread inside the present
	// critical section right after the swapchain has been reconfigured to a
	// new logical size — i.e., just before the first buffer at that size is
	// attached and committed. The X11 host/renderer uses the replace-call
	// hook pattern; the Wayland host uses it to declare xdg window geometry
	// so the declaration reaches the compositor in the same wire batch as
	// the new-size buffer (declaring a geometry larger than the current
	// surface makes mutter cache negative frame extents, which corrupt
	// maximize/unmaximize restore: the size-hints round trip flips the
	// restored size to work-area − content).
	onSwapchainResized func(logicalW, logicalH int)

	// lastOutcome / lastDamageArea are set by PresentWith / PresentWithAuto for metrics.
	lastOutcome    PresentOutcome
	lastDamageArea int64 // physical px² of last FrameDamage union (0 if idle/empty)

	// fallbacks counts downgrade levels retried before this target opened
	// (0 = first level succeeded). Set once by NewPresentTarget.
	fallbacks int
}

// sharedPresentDevice is the process-wide wgpu device share (Skia
// GrDirectContext pattern): the first PresentTarget opens it, later windows
// borrow it (own surface + swapchain, shared instance/adapter/device), the
// last Close releases it. Guarded by shareMu; refcount includes in-flight
// opens (an open that fails after acquiring never leaks a count).
var (
	shareMu      sync.Mutex
	shareInst    hal.Instance
	shareAdapter hal.Adapter
	shareDevice  hal.Device
	shareRefs    int
)

// shareOpenMu serializes present-target construction (buildPresentTarget's
// instance/adapter/device creation, surface creation, swapchain configure).
// wgpu-native Instance/Adapter/Surface creation is not thread-safe on all
// backends (see gpu/rwgpu/thread_safety_test.go), and Xlib calls ride the
// same path. Borrows (buildSharedSurface) still run concurrently: the share
// pin keeps the device alive, only opens serialize. Per-window Run loops
// stay concurrent; only window creation waits here.
var shareOpenMu sync.Mutex

// AcquireSharedPresentDevice reports the share state for diagnostics.
// Borrowable windows go through buildSharedSurface directly; standalone
// (probe/offscreen) devices borrow through here so only one device is live
// per process on 1GB cards. Exported for render/internal/gpu; the refcount
// is owned by PresentTarget Close — borrowers must NOT Release the handle.
// Only answers "is a shared device open" without taking a refcount.
// AcquireSharedPresentDevice reports the share state for diagnostics.
// Borrowable windows go through buildSharedSurface directly; standalone
// (probe/offscreen) devices borrow through here so only one device is live
// per process on 1GB cards. Exported for render/internal/gpu; the refcount
// is owned by PresentTarget Close — borrowers must NOT Release the handle.
// Only answers "is a shared device open" without taking a refcount.
func AcquireSharedPresentDevice() (inst hal.Instance, adapter hal.Adapter, device hal.Device, borrowable bool, err error) {
	return acquireSharedPresentDevice()
}

// acquireSharedPresentDevice reports the share state for diagnostics.
// Borrowable windows go through buildSharedSurface directly; this helper
// only answers "is a shared device open" without taking a refcount.
// acquireSharedPresentDevice reports the share state for diagnostics.
// Borrowable windows go through buildSharedSurface directly; this helper
// only answers "is a shared device open" without taking a refcount.
func acquireSharedPresentDevice() (inst hal.Instance, adapter hal.Adapter, device hal.Device, borrowable bool, err error) {
	shareMu.Lock()
	defer shareMu.Unlock()
	if shareDevice != nil {
		return shareInst, shareAdapter, shareDevice, true, nil
	}
	return nil, nil, nil, false, errSharedNotOpen
}

// errSharedNotOpen signals "no shared device yet — open one with a surface".
var errSharedNotOpen = errors.New("render: shared present device not open")

// releaseShared drops one share refcount; the last one releases the device.
// releaseShared drops one share refcount; the last one releases the device.
func releaseShared() {
	shareMu.Lock()
	defer shareMu.Unlock()
	if shareRefs > 0 {
		shareRefs--
	}
	if shareRefs > 0 || shareDevice == nil {
		return
	}
	shareDevice.Release()
	shareDevice = nil
	if shareAdapter != nil {
		shareAdapter.Release()
		shareAdapter = nil
	}
	if shareInst != nil {
		shareInst.Release()
		shareInst = nil
	}
}

// presentLevel is one step of the open-time downgrade chain
// (discrete-first → integrated-first → software fallback). Levels reuse
// RequestAdapterWithPolicy's ordered-try semantics; the last level requests
// the software adapter directly.
type presentLevel struct {
	name     string
	policy   AdapterPolicy
	software bool
}

// presentLevels orders downgrade attempts from the resolved policy downward.
// The first level is the 1.2 behavior; lower levels only run after an
// OOM-class failure, so resource-sufficient opens follow the old path.
// NewPresentTarget creates a GPU present path from native window handles.
// logicalW/H are layout pixels; scale is device pixel ratio (≥1).
// The GPU implementation follows GPUI_BACKEND (see ResolveBackend).
func NewPresentTarget(ns PresentNativeSurface, logicalW, logicalH int, scale float64) (*PresentTarget, error) {
	return NewPresentTargetWithBackend(ns, logicalW, logicalH, scale, BackendNative)
}

// NewPresentTargetWithBackend creates a GPU present path with an explicit
// GPU implementation choice. BackendNative (zero value) means default;
// GPUI_BACKEND env still wins when set (see ResolveBackend).
// NewPresentTargetWithBackend creates a GPU present path with an explicit
// GPU implementation choice. BackendNative (zero value) means default;
// GPUI_BACKEND env still wins when set (see ResolveBackend).
func NewPresentTargetWithBackend(ns PresentNativeSurface, logicalW, logicalH int, scale float64, backend Backend) (*PresentTarget, error) {
	if ns.Window == 0 {
		return nil, errors.New("render: PresentNativeSurface.Window is zero")
	}
	if logicalW < 1 {
		logicalW = 1
	}
	if logicalH < 1 {
		logicalH = 1
	}
	if scale <= 0 {
		scale = 1
	}

	levels := presentLevels(ResolveAdapterPolicy())
	var lastErr error
	for i, lv := range levels {
		t, err := buildPresentTarget(ns, logicalW, logicalH, scale, lv, backend)
		if err == nil {
			t.fallbacks = i
			return t, nil
		}
		// Only OOM walks down; any other failure returns as before.
		if !IsGPUOutOfMemory(err) {
			return nil, err
		}
		lastErr = err
		if i < len(levels)-1 {
			fmt.Fprintf(os.Stderr, "render: present level %q out of memory, trying %q\n",
				lv.name, levels[i+1].name)
		}
	}
	tried := make([]string, len(levels))
	for i, lv := range levels {
		tried[i] = lv.name
	}
	return nil, fmt.Errorf("render: out of GPU memory opening %dx%d window (tried %s): %v; close other GPU windows or set GPUI_POWER to a less-loaded GPU",
		logicalW, logicalH, strings.Join(tried, " → "), lastErr)
}

// buildPresentTarget builds one downgrade level: surface → adapter → device
// → swapchain. The first window in the process opens (instance, adapter,
// device) and publishes them as the share; later windows create only their
// own surface + swapchain on the shared device (Skia GrDirectContext: one
// device, N surfaces). Any step's failure releases that level's resources
// (Close() reverse order) before returning.
//
// Construction is serialized by shareOpenMu (concurrent opens abort inside
// wgpu-native/Xlib on some backends; borrows still run concurrently).
// buildPresentTarget builds one downgrade level: surface → adapter → device
// → swapchain. The first window in the process opens (instance, adapter,
// device) and publishes them as the share; later windows create only their
// own surface + swapchain on the shared device (Skia GrDirectContext: one
// device, N surfaces). Any step's failure releases that level's resources
// (Close() reverse order) before returning.
//
// Construction is serialized by shareOpenMu (concurrent opens abort inside
// wgpu-native/Xlib on some backends; borrows still run concurrently).
func buildPresentTarget(ns PresentNativeSurface, logicalW, logicalH int, scale float64, lv presentLevel, backend Backend) (*PresentTarget, error) {
	shareOpenMu.Lock()
	defer shareOpenMu.Unlock()
	if _, _, _, borrowable, _ := peekShared(); borrowable {
		if t, err := buildSharedSurface(ns, logicalW, logicalH, scale); err == nil {
			return t, nil
		} else if !IsGPUOutOfMemory(err) {
			// non-OOM borrow failures return as-is on purpose — a
			// published share means the device is fine, so the failure is
			// the caller's (bad handles, torn-down window). Falling through
			// to open a second device would fork the single-device invariant
			// (one device, N surfaces); only OOM retries at a lower level.
			return nil, err
		}
		// OOM-class borrow failure: fall through to the downgrade chain so
		// the window can still try a lower-power adapter or software.
	}

	// NewPresentTargetWithBackend threads through; the backend arg selects it.
	want, err := ResolveBackendFor(backend)
	if err != nil {
		return nil, err
	}
	if want == BackendGo {
		// GL path: X11/Wayland only (Windows/macOS follow their own steps).
		if ns.Platform != PresentPlatformX11 && ns.Platform != PresentPlatformWayland {
			return nil, fmt.Errorf("render: Go GL backend only supports X11/Wayland for now (platform=%d)", ns.Platform)
		}
	}
	inst, err := SelectBackend(want)
	if err != nil {
		return nil, err
	}

	// Surface backend MUST match handle types (Xlib Display*/Window vs wl_*).
	// Driven by PresentNativeSurface.Platform — never guess from env alone.
	// hal.SurfaceTarget carries the kind; webgpu maps it to its backend path.
	surfaceBackend := surfaceBackendFor(ns.Platform)
	surf, err := inst.CreateSurface(surfaceTargetFor(ns))
	if err != nil {
		inst.Release()
		return nil, fmt.Errorf("render: CreateSurface(%s): %w", surfaceBackend, err)
	}

	policy := lv.policy
	var adapter hal.Adapter
	var forceFallback bool
	if lv.software {
		adapter, err = inst.RequestAdapter(&hal.RequestAdapterOptions{
			PowerPreference:      types.PowerPreferenceNone,
			ForceFallbackAdapter: true,
			CompatibleSurface:    surf,
		})
		forceFallback = err == nil
		if err != nil {
			surf.Destroy()
			inst.Release()
			return nil, fmt.Errorf("render: RequestAdapter: %w", err)
		}
	} else {
		adapter, forceFallback, err = RequestAdapterWithPolicy(inst, surf, policy)
		if err != nil {
			surf.Destroy()
			inst.Release()
			return nil, fmt.Errorf("render: RequestAdapter: %w", err)
		}
	}
	ai := types.AdapterInfo{}
	if adapter != nil {
		ai = adapter.Info()
	}
	if forceFallback {
		fmt.Fprintf(os.Stderr, "render: adapter policy=%s fell back to software adapter %q (type=%v)\n",
			policy, ai.Name, ai.DeviceType)
	}
	if os.Getenv("WR_RESIZE_DBG") == "1" {
		fmt.Fprintf(os.Stderr, "DBG adapter=%q vendor=%q type=%v backend=%v\n",
			ai.Name, ai.Vendor, ai.DeviceType, ai.Backend)
	}

	device, err := requestPresentDeviceWithRetry(adapter, DeviceDescriptorForAdapter("ui-l1-present", adapter), "ui-l1-present")
	if err != nil {
		adapter.Release()
		surf.Destroy()
		inst.Release()
		return nil, fmt.Errorf("render: RequestDevice: %w", err)
	}

	// Device-readiness gate: a previous GPU user in this process (probe pass,
	// another window) may still pin driver heap blocks after Release — the
	// driver reclaims asynchronously. Poll a 1x1 probe alloc until it succeeds
	// or the deadline passes so the first real texture does not die on a
	// half-reclaimed heap (measured 2026-09-17: 3.66MB depth dead on arrival
	// on a 1GB card while 660MB read free). Waits are logged, never silent.
	if err := waitDeviceReady(inst, device, "ui-l1-present"); err != nil {
		device.Release()
		adapter.Release()
		surf.Destroy()
		inst.Release()
		return nil, err
	}

	physW, physH := physicalSize(logicalW, logicalH, scale)
	sc, err := newPresentSwapchain(surf, device, physW, physH)
	if err != nil {
		surf.Destroy()
		device.Release()
		adapter.Release()
		inst.Release()
		return nil, err
	}
	// CopyDst allows GOGPU_RENDER_MODE=cpu window presents to upload the CPU
	// rasterized pixmap directly into the swapchain texture
	// (uploadPixmapToView — the CPU mode renders shapes on the CPU and the
	// present no longer depends on the GPU raster pipelines).
	sc.SetUsage(types.TextureUsageRenderAttachment | types.TextureUsageCopyDst)
	// 块3 present 策略：Wayland 与 X11 同为 Fifo 稳态（阻塞式 vsync 把提交
	// 相位锁到显示刷新）。历史上 Wayland 用 FifoRelaxed 恒不阻塞，但实测
	// （pelican GNOME/mutter 2026-08-26）UI 软件边界 16.0ms 与显示刷新
	// 16.7ms 自由漂移 → 周期性错过合成 deadline → 上屏内容步距忽大忽小
	// judder；X11 的 Fifo 阻塞语义天然锁相无此问题。Fifo 在 wayland
	// 下由 frame callback 节流、raster 线程阻塞在 present（UI 线程异步，
	// pipeline depth=2 可提前构建），不会卡 UI。
	fixedNoVsync := false
	if fixedNoVsync {
		sc.SetPreferFifoRelaxed()
	} else {
		sc.SetPreferVSync()
	}
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		surf.Destroy()
		device.Release()
		adapter.Release()
		inst.Release()
		return nil, fmt.Errorf("render: ConfigureFromCapabilities: %w", err)
	}

	// Publish the first window's chain as the process share (later windows
	// borrow it; last Close releases it).
	// R0-2: never overwrite an existing share. Concurrent opens that lose
	// the race drop their own chain and borrow the winner instead of
	// leaking the first device and resetting its refcount to 1.
	shareMu.Lock()
	if shareDevice != nil {
		shareMu.Unlock()
		sc.Release()
		surf.Destroy()
		device.Release()
		adapter.Release()
		inst.Release()
		return buildSharedSurface(ns, logicalW, logicalH, scale)
	}
	shareInst, shareAdapter, shareDevice = inst, adapter, device
	shareRefs = 1
	shareMu.Unlock()

	// Bind shared device for GPU-accelerated draws (blank clear still works
	// if this fails). Bound only on the winning publish above, so a loser
	// never rebinds the accelerator to a device it just released.
	// Neutral boxed provider (halDeviceProvider): works for every backend.
	_ = SetAcceleratorDeviceProvider(&halDeviceProvider{
		Dev: device, Adpt: adapter, Format: sc.GetFormat(),
	})

	dc := NewContext(logicalW, logicalH, WithDeviceScale(scale))

	t := &PresentTarget{
		ns:                ns,
		logicW:            logicalW,
		logicH:            logicalH,
		scale:             scale,
		inst:              inst,
		adapter:           adapter,
		device:            device,
		surf:              surf,
		sc:                sc,
		dc:                dc,
		resizeStormWindow: 300 * time.Millisecond,
		// Fifo (vsync) is the steady-state present mode; SetVsync(false)
		// switches to Mailbox/Immediate during resize storms. Wayland
		// (fixedNoVsync) stays non-blocking permanently instead.
		vsyncOn:      !fixedNoVsync,
		fixedNoVsync: fixedNoVsync,
	}
	// R0-6: track every live target so a shared recovery can reconfigure
	// all swapchains exactly once.
	shareMu.Lock()
	shareGen++
	registerSharedTarget(t)
	shareMu.Unlock()
	return t, nil
}

// peekShared snapshots the share state (borrowable = device published).
