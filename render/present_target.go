package render

import (
	"errors"
	"fmt"
	"image"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// requestPresentDeviceWithRetry retries device creation when the adapter is
// temporarily out of GPU memory (multi-window stolen-memory budget on iGPUs).
// Other windows/processes may release memory between retries; this mirrors
// Flutter's degrade-not-crash behavior on transient resource pressure.
// Implemented here rather than render/internal/gpu to avoid an import cycle.
func requestPresentDeviceWithRetry(adapter *webgpu.Adapter, desc *webgpu.DeviceDescriptor, label string) (*webgpu.Device, error) {
	if adapter == nil {
		return nil, fmt.Errorf("adapter is nil")
	}
	var device *webgpu.Device
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
func waitDeviceReady(inst *webgpu.Instance, device *webgpu.Device, label string) error {
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
		tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
			Label:         label + "_ready_probe",
			Size:          webgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
			MipLevelCount: 1,
			SampleCount:   1,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatBGRA8Unorm,
			Usage:         types.TextureUsageCopySrc,
		})
		if err == nil {
			tex.Release()
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
// Mirrors ui/platform kinds without importing ui (dependency: ui → render only).
type PresentPlatform int

const (
	PresentPlatformX11 PresentPlatform = iota
	PresentPlatformWayland
	PresentPlatformWin32
	PresentPlatformAppKit
)

// PresentNativeSurface holds OS handles required to create a GPU present surface.
//
//	Linux X11:     Display=Display*, Window=Window (XID)
//	Linux Wayland: Display=wl_display*, Window=wl_surface*
//	Windows:       Display=0 or HINSTANCE, Window=HWND
//	macOS:         Display=0, Window=CAMetalLayer* / NSView* per gpu binding
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

	inst    *webgpu.Instance
	adapter *webgpu.Adapter
	device  *webgpu.Device
	// shared marks instance/adapter/device as borrowed from the process
	// share (Close must not release them; the share does on last close).
	shared bool
	surf   *webgpu.Surface
	sc     *webgpu.Swapchain
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
	shareInst    *webgpu.Instance
	shareAdapter *webgpu.Adapter
	shareDevice  *webgpu.Device
	shareRefs    int
)

// AcquireSharedPresentDevice reports the share state for diagnostics.
// Borrowable windows go through buildSharedSurface directly; standalone
// (probe/offscreen) devices borrow through here so only one device is live
// per process on 1GB cards. Exported for render/internal/gpu; the refcount
// is owned by PresentTarget Close — borrowers must NOT Release the handle.
// Only answers "is a shared device open" without taking a refcount.
func AcquireSharedPresentDevice() (inst *webgpu.Instance, adapter *webgpu.Adapter, device *webgpu.Device, borrowable bool, err error) {
	return acquireSharedPresentDevice()
}

// acquireSharedPresentDevice reports the share state for diagnostics.
// Borrowable windows go through buildSharedSurface directly; this helper
// only answers "is a shared device open" without taking a refcount.
func acquireSharedPresentDevice() (inst *webgpu.Instance, adapter *webgpu.Adapter, device *webgpu.Device, borrowable bool, err error) {
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
func presentLevels(start AdapterPolicy) []presentLevel {
	levels := []presentLevel{{name: start.String(), policy: start}}
	if start != PolicyLow {
		levels = append(levels, presentLevel{name: PolicyLow.String(), policy: PolicyLow})
	}
	return append(levels, presentLevel{name: "software", software: true})
}

// NewPresentTarget creates a GPU present path from native window handles.
// logicalW/H are layout pixels; scale is device pixel ratio (≥1).
func NewPresentTarget(ns PresentNativeSurface, logicalW, logicalH int, scale float64) (*PresentTarget, error) {
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
		t, err := buildPresentTarget(ns, logicalW, logicalH, scale, lv)
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
func buildPresentTarget(ns PresentNativeSurface, logicalW, logicalH int, scale float64, lv presentLevel) (*PresentTarget, error) {
	if _, _, _, borrowable, _ := peekShared(); borrowable {
		if t, err := buildSharedSurface(ns, logicalW, logicalH, scale); err == nil {
			return t, nil
		} else if !IsGPUOutOfMemory(err) {
			// R4: non-OOM borrow failures return as-is on purpose — a
			// published share means the device is fine, so the failure is
			// the caller's (bad handles, torn-down window). Falling through
			// to open a second device would fork the single-device invariant
			// (one device, N surfaces); only OOM retries at a lower level.
			return nil, err
		}
		// OOM-class borrow failure: fall through to the downgrade chain so
		// the window can still try a lower-power adapter or software.
	}

	inst, err := webgpu.CreateInstance(&webgpu.InstanceDescriptor{Backends: webgpu.BackendsPrimary})
	if err != nil {
		return nil, fmt.Errorf("render: CreateInstance: %w", err)
	}

	// Surface backend MUST match handle types (Xlib Display*/Window vs wl_*).
	// Driven by PresentNativeSurface.Platform — never guess from env alone.
	backend := surfaceBackendFor(ns.Platform)
	surf, err := inst.CreateSurfaceFor(backend, ns.Display, ns.Window)
	if err != nil {
		inst.Release()
		return nil, fmt.Errorf("render: CreateSurface(%s): %w", backend, err)
	}

	policy := lv.policy
	var adapter *webgpu.Adapter
	var forceFallback bool
	if lv.software {
		adapter, err = inst.RequestAdapter(&webgpu.RequestAdapterOptions{
			PowerPreference:      webgpu.PowerPreferenceNone,
			ForceFallbackAdapter: true,
			CompatibleSurface:    surf,
		})
		forceFallback = err == nil
		if err != nil {
			surf.Release()
			inst.Release()
			return nil, fmt.Errorf("render: RequestAdapter: %w", err)
		}
	} else {
		adapter, forceFallback, err = RequestAdapterWithPolicy(inst, surf, policy)
		if err != nil {
			surf.Release()
			inst.Release()
			return nil, fmt.Errorf("render: RequestAdapter: %w", err)
		}
	}
	ai := webgpu.AdapterInfo{}
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
		surf.Release()
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
		surf.Release()
		inst.Release()
		return nil, err
	}

	physW, physH := physicalSize(logicalW, logicalH, scale)
	sc := webgpu.NewSwapchain(surf, device, physW, physH)
	// CopyDst allows GOGPU_RENDER_MODE=cpu window presents to upload the CPU
	// rasterized pixmap directly into the swapchain texture
	// (uploadPixmapToView — the CPU mode renders shapes on the CPU and the
	// present no longer depends on the GPU raster pipelines).
	sc.Usage = types.TextureUsageRenderAttachment | types.TextureUsageCopyDst
	// 块3 present 策略：Wayland 与 X11 同为 Fifo 稳态（阻塞式 vsync 把提交
	// 相位锁到显示刷新）。历史上 Wayland 用 FifoRelaxed 恒不阻塞，但实测
	// （pelican GNOME/mutter 2026-08-26）UI 软件边界 16.0ms 与显示刷新
	// 16.7ms 自由漂移 → 周期性错过合成 deadline → 上屏内容步距忽大忽小
	// judder；X11 的 Fifo 阻塞语义天然锁相无此问题。Fifo 在 wayland(wgpu)
	// 下由 frame callback 节流、raster 线程阻塞在 present（UI 线程异步，
	// pipeline depth=2 可提前构建），不会卡 UI。
	fixedNoVsync := false
	if fixedNoVsync {
		sc.SetPreferFifoRelaxed()
	} else {
		sc.SetPreferVSync()
	}
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		surf.Release()
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
		surf.Release()
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
	_ = SetAcceleratorDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: device, Adpt: adapter, Format: sc.Format,
	})

	dc := NewContext(logicalW, logicalH, WithDeviceScale(scale))

	t := &PresentTarget{
		ns:      ns,
		logicW:  logicalW,
		logicH:  logicalH,
		scale:   scale,
		inst:    inst,
		adapter: adapter,
		device:  device,
		surf:    surf,
		sc:      sc,
		dc:      dc,
		// Default storm window: 300ms ≈ 18 frames @60Hz — a drag-resize step
		// arriving faster than this keeps the full path active continuously.
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
func peekShared() (inst *webgpu.Instance, adapter *webgpu.Adapter, device *webgpu.Device, borrowable bool, refs int) {
	shareMu.Lock()
	defer shareMu.Unlock()
	return shareInst, shareAdapter, shareDevice, shareDevice != nil, shareRefs
}

// buildSharedSurface opens a window on the borrowed share: own surface +
// swapchain on the shared device. Non-OOM errors return directly; OOM-class
// errors return for the downgrade chain to try a lower level.
func buildSharedSurface(ns PresentNativeSurface, logicalW, logicalH int, scale float64) (*PresentTarget, error) {
	// R0-3: pin a share ref across the slow ops below. Snapshotting the
	// pointers without a ref lets the owner Close free them mid-build, so
	// the borrow continues on a dead device and then resurrects the count.
	shareMu.Lock()
	inst, device := shareInst, shareDevice
	if device == nil || inst == nil {
		shareMu.Unlock()
		return nil, errors.New("render: shared present device not open")
	}
	adapter := shareAdapter
	shareRefs++
	shareMu.Unlock()
	// dropPin releases the pin taken above; success paths keep it as the
	// borrower's own ref.
	dropPin := func() {
		shareMu.Lock()
		shareRefs--
		shareMu.Unlock()
	}
	backend := surfaceBackendFor(ns.Platform)
	surf, err := inst.CreateSurfaceFor(backend, ns.Display, ns.Window)
	if err != nil {
		dropPin()
		return nil, fmt.Errorf("render: CreateSurface(%s): %w", backend, err)
	}
	physW, physH := physicalSize(logicalW, logicalH, scale)
	sc := webgpu.NewSwapchain(surf, device, physW, physH)
	sc.Usage = types.TextureUsageRenderAttachment | types.TextureUsageCopyDst
	sc.SetPreferVSync()
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		surf.Release()
		dropPin()
		return nil, fmt.Errorf("render: ConfigureFromCapabilities: %w", err)
	}
	// Defensive: the share must still be the chain we pinned. With the pin
	// held the owner cannot free it, so any mismatch is a logic error —
	// fail closed instead of counting a dead device.
	shareMu.Lock()
	if shareDevice != device || shareInst != inst {
		shareRefs--
		shareMu.Unlock()
		sc.Release()
		surf.Release()
		return nil, fmt.Errorf("render: shared present device changed during open: %w", errSharedNotOpen)
	}
	shareMu.Unlock()
	// Same device as the share: the pin above is this borrower's refcount.
	// Do NOT rebind the
	// global accelerator provider here: the rebind invalidates every
	// live GPU session (abandonAllContextGPU) and a concurrent flush on
	// another window's raster thread rebuilds its session against the
	// same device anyway (deviceGen unchanged → no rebuild at all).
	// Rebinding with the identical device is a no-op for correctness
	// and a crash vector for concurrency (two sessions racing
	// ensureTexturesForView on the post-abandon rebuild → nil pass).
	dc := NewContext(logicalW, logicalH, WithDeviceScale(scale))
	t := &PresentTarget{
		ns:                ns,
		logicW:            logicalW,
		logicH:            logicalH,
		scale:             scale,
		inst:              inst,
		adapter:           adapter,
		device:            device,
		shared:            true,
		surf:              surf,
		sc:                sc,
		dc:                dc,
		resizeStormWindow: 300 * time.Millisecond,
		vsyncOn:           true,
	}
	// R0-6: track every live target so a shared recovery can reconfigure
	// all swapchains exactly once (the pin above is this borrower's ref).
	shareMu.Lock()
	registerSharedTarget(t)
	shareMu.Unlock()
	return t, nil
}

func surfaceBackendFor(p PresentPlatform) webgpu.SurfaceBackend {
	switch p {
	case PresentPlatformWayland:
		return webgpu.SurfaceBackendWayland
	case PresentPlatformWin32:
		return webgpu.SurfaceBackendWin32
	case PresentPlatformAppKit:
		return webgpu.SurfaceBackendMetal
	default:
		return webgpu.SurfaceBackendXlib
	}
}

func physicalSize(logicalW, logicalH int, scale float64) (uint32, uint32) {
	pw := int(float64(logicalW)*scale + 0.5)
	ph := int(float64(logicalH)*scale + 0.5)
	if pw < 1 {
		pw = 1
	}
	if ph < 1 {
		ph = 1
	}
	return uint32(pw), uint32(ph)
}

// Context returns the drawing context (logical size, device scale applied).
func (t *PresentTarget) Context() *Context {
	if t == nil {
		return nil
	}
	return t.dc
}

// LogicalSize returns layout size in logical pixels.
func (t *PresentTarget) LogicalSize() (w, h int) {
	if t == nil {
		return 0, 0
	}
	return t.logicW, t.logicH
}

// Scale returns device pixel ratio.
func (t *PresentTarget) Scale() float64 {
	if t == nil || t.scale <= 0 {
		return 1
	}
	return t.scale
}

// SetResizeStormWindow configures the storm window used by InFullRecovery /
// the full-path forcing (see resizeStormWindow). <=0 disables storm awareness
// (fixed postResizeFull budget only). Tests shrink the window to exercise the
// storm boundary without waiting.
func (t *PresentTarget) SetResizeStormWindow(d time.Duration) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.resizeStormWindow = d
}

// Resize records a new logical size / scale. Record-only (T3): the UI thread
// only stores the size and arms swapchainPending; the drawing Context sync
// (dc.Resize/SetDeviceScale) and the wgpu swapchain reconfigure both run on
// the raster thread at the present boundary (applyPendingSwapchainLocked),
// serialized with BeginFrame/EndFrame via mu. Same-size calls are no-ops.
// This keeps render.Context raster-exclusive: the UI thread never blocks on
// the surface and never touches dc off raster (T3 render-owns-raster).
func (t *PresentTarget) Resize(logicalW, logicalH int, scale float64) error {
	if t == nil {
		return errors.New("render: nil PresentTarget")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return errors.New("render: PresentTarget closed")
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
	if t.logicW == logicalW && t.logicH == logicalH && t.scale == scale {
		return nil
	}
	t.logicW, t.logicH, t.scale = logicalW, logicalH, scale
	t.swapchainPending = true
	return nil
}

// SetOnSwapchainResized registers a callback fired on the raster thread
// (inside the present critical section) immediately after the swapchain has
// been reconfigured to a new logical size, before the first buffer of that
// size is committed. The Wayland host uses it to declare xdg window geometry
// in the same wire batch as the new-size buffer. A nil callback is a no-op.
func (t *PresentTarget) SetOnSwapchainResized(fn func(logicalW, logicalH int)) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onSwapchainResized = fn
}

// SetVsync switches the swapchain present mode at the next present boundary
// (raster thread, serialized with BeginFrame/EndFrame): true = Fifo (vsync,
// steady UI), false = Mailbox/Immediate (low-latency — used while an
// interactive resize storm is active so content tracks the window instead of
// waiting one Fifo vblank per frame). Record-only, like Resize: the UI
// thread never blocks on the surface. The swapchain buffers are undefined
// after a mode switch, so the switch arms the post-resize full-write budget.
func (t *PresentTarget) SetVsync(vsync bool) {
	if t == nil || t.sc == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	// fixedNoVsync platforms (Wayland): present is permanently non-blocking
	// (compositor vblank swap; ENGINE_FRAME_PRESENT_STANDARD.md 块3), so the
	// Fifo↔low-latency switch is a no-op — the storm path already submits
	// without waiting.
	if t.fixedNoVsync {
		return
	}
	if t.vsyncOn == vsync {
		return
	}
	t.vsyncOn = vsync
	t.vsyncPending = true
}

// applyPendingSwapchain reconfigures the swapchain to the recorded logical
// size, then arms the post-resize full-write budget. Runs on the raster
// thread at the present boundary (serialized with BeginFrame/EndFrame via
// mu). Caller must hold mu.
//
// X11/Xwayland: the reconfigure is deferred to the present boundary (the
// extent check below, then BeginFrame's outdated-retry) instead of a
// synchronous Configure here — on X11/Xwayland each Configure costs
// 26–383ms (scaling with window size), so configuring synchronously on
// every resize step — and again in BeginFrame when the window moved during
// the Configure — doubled the per-step stall (the visible "content lags the
// window" gap); measured +70% drag presents.
//
// Other platforms (Wayland/Windows/macOS/browser): wgpu does not report
// "outdated" when the surface size changes, so the swapchain MUST be
// reconfigured here or it stays at the stale extent (presented content
// clipped/stretched); natively these Configure calls are ~1–5ms.
func (t *PresentTarget) applyPendingSwapchainLocked() error {
	if t == nil || t.sc == nil {
		return nil
	}
	if t.swapchainPending {
		t.swapchainPending = false
		// T3 raster-exclusive Context sync: the UI thread only recorded the
		// size above; the dc pixmap realloc + scale switch run here on the
		// raster thread, serialized with the draw below via mu — the packet
		// built at the new size is drawn at the new size, no UI/raster race.
		if t.dc != nil {
			_ = t.dc.Resize(t.logicW, t.logicH)
			t.dc.SetDeviceScale(t.scale)
		}
		if t.ns.Platform == PresentPlatformX11 {
			if os.Getenv("WR_RESIZE_DBG") == "1" {
				pw, ph := physicalSize(t.logicW, t.logicH, t.scale)
				fmt.Fprintf(os.Stderr, "DBG swapchain apply %dx%d (logic %dx%d, deferred)\n", pw, ph, t.logicW, t.logicH)
			}
			// The swapchain extent is reconfigured by BeginFrame's outdated-retry
			// at the live window size; nothing to configure synchronously here.
		} else {
			pw, ph := physicalSize(t.logicW, t.logicH, t.scale)
			if os.Getenv("WR_RESIZE_DBG") == "1" {
				fmt.Fprintf(os.Stderr, "DBG swapchain apply %dx%d (logic %dx%d)\n", pw, ph, t.logicW, t.logicH)
			}
			if err := t.sc.Resize(pw, ph); err != nil {
				// Keep the flag set: the next present retries the reconfigure.
				t.swapchainPending = true
				return err
			}
			// The swapchain now serves buffers at the new size; the next commit
			// (this same present critical section) attaches one of them. Notify
			// the host now — before that commit — so a platform that declares
			// window geometry (Wayland xdg_surface.set_window_geometry) has it
			// hit the compositor in the same batch as the new-size buffer.
			if t.onSwapchainResized != nil {
				t.onSwapchainResized(t.logicW, t.logicH)
			}
		}
		// Swapchain reconfigured → every buffer is undefined. Owe full frames
		// so each buffer is fully written before retained LoadOpLoad frames
		// read it again (otherwise resize storms leave black regions). Also
		// arm the storm window so every present while the resize storm is
		// active stays full even if the 3-frame budget is spent between steps.
		t.postResizeFull = 3
		t.lastResizeAt = time.Now()
	}
	// Runtime present-mode switch (SetVsync), applied at the same boundary so
	// the surface mode never changes mid-frame. A mode change is also a
	// surface reconfigure: buffers are undefined → owe full frames.
	if t.vsyncPending {
		t.vsyncPending = false
		mode := t.sc.PresentModeForVsync(t.vsyncOn)
		if mode != t.sc.PresentMode {
			if os.Getenv("WR_RESIZE_DBG") == "1" {
				fmt.Fprintf(os.Stderr, "DBG vsync %v -> %v\n", t.sc.PresentMode, mode)
			}
			if err := t.sc.SetPresentModeForce(mode); err != nil {
				t.vsyncPending = true
				return err
			}
			t.postResizeFull = 3
			t.lastResizeAt = time.Now()
		}
	}
	return nil
}

// PresentClear clears to RGBA (0–1) and presents one full frame (P0 path).
func (t *PresentTarget) PresentClear(r, g, b, a float64) error {
	return t.PresentWith(func(dc *Context) {
		dc.SetRGBA(r, g, b, a)
		dc.DrawRectangle(0, 0, float64(t.logicW), float64(t.logicH))
		_ = dc.Fill()
	})
}

// InFullRecovery reports whether the swapchain was reconfigured and full
// frames are still owed to its buffers — either by the fixed post-resize
// budget (postResizeFull) or by an active resize storm (lastResizeAt within
// the storm window). Callers that would skip work for retained steady frames
// (e.g. compositeOnly) must disable that path while this is true so every new
// buffer gets fully written.
func (t *PresentTarget) InFullRecovery() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.swapchainPending || t.postResizeFull > 0 || t.inResizeStormLocked()
}

// inResizeStormLocked reports whether a resize storm is active: the most
// recent physical resize is fresher than resizeStormWindow. Caller holds mu.
func (t *PresentTarget) inResizeStormLocked() bool {
	if t.resizeStormWindow <= 0 {
		return false
	}
	return !t.lastResizeAt.IsZero() && time.Since(t.lastResizeAt) < t.resizeStormWindow
}

// PresentWith begins a frame, runs draw (logical coords on dc), and presents
// via PresentFrameFull (explicit full-surface path). Prefer PresentWithAuto for
// steady retained UI frames that accumulate FrameDamage during draw.
// Safe to call from the raster thread only.
func (t *PresentTarget) PresentWith(draw func(dc *Context)) error {
	_, err := t.present(draw, true)
	return err
}

// PresentWithAuto begins a frame, runs draw, then presents via PresentFrameAuto
// so idle/damage/full modes follow FrameDamage from the draw callback.
// Bootstrap/resize callers that must force a full path should use PresentWith.
// Returns the PresentOutcome for metrics (damage mode, rect count).
func (t *PresentTarget) PresentWithAuto(draw func(dc *Context)) (PresentOutcome, error) {
	return t.present(draw, false)
}

// LastPresentOutcome returns the outcome of the most recent present call.
func (t *PresentTarget) LastPresentOutcome() PresentOutcome {
	if t == nil {
		return PresentOutcome{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastOutcome
}

// Fallbacks reports how many downgrade levels were retried before this
// target opened (0 = first level succeeded). Metrics JSON uses it.
func (t *PresentTarget) Fallbacks() int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.fallbacks
}

// GPUBackend reports the actual adapter category behind this target:
// discrete, integrated, or software (CPU fallback). Metrics JSON uses it.
func (t *PresentTarget) GPUBackend() string {
	if t == nil {
		return "unknown"
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.adapter == nil {
		return "unknown"
	}
	switch t.adapter.Info().DeviceType {
	case types.DeviceTypeDiscreteGPU:
		return "discrete"
	case types.DeviceTypeIntegratedGPU:
		return "integrated"
	default:
		return "software"
	}
}

// LastDamageAreaPx returns physical-pixel area of the last frame's damage union
// (0 when idle / unknown). Used for M-DAMAGE-AREA style metrics.
func (t *PresentTarget) LastDamageAreaPx() int64 {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastDamageArea
}

func (t *PresentTarget) present(draw func(dc *Context), forceFull bool) (PresentOutcome, error) {
	out := PresentOutcome{}
	if t == nil {
		return out, errors.New("render: nil PresentTarget")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return out, errors.New("render: PresentTarget closed")
	}
	if t.dc == nil || t.sc == nil || t.device == nil {
		return out, errors.New("render: PresentTarget not initialized")
	}

	// Reconfigured swapchain buffers are undefined until each is fully
	// written; owed full frames force the full path (Skia recreate semantics).
	// During an active resize storm every present stays full too, so a buffer
	// at an intermediate size is never LoadOpLoad'd half-written.
	// Apply a swapchain resize recorded on the UI thread here — on the raster
	// thread, serialized with BeginFrame/EndFrame — so the surface tracks the
	// window without stalling the UI thread and without Configure/present
	// races dropping frames during interactive resize drags.
	var pApply, pBegin, pEnd time.Time
	if os.Getenv("WR_RESIZE_DBG") == "1" {
		pApply = time.Now()
	}
	if err := t.applyPendingSwapchainLocked(); err != nil {
		return out, err
	}
	if os.Getenv("WR_RESIZE_DBG") == "1" {
		if d := time.Since(pApply); d > 20*time.Millisecond {
			fmt.Fprintf(os.Stderr, "DBG phase apply=%dms\n", d.Milliseconds())
		}
		pBegin = time.Now()
	}
	if t.postResizeFull > 0 || t.inResizeStormLocked() {
		forceFull = true
	}

	if t.device != nil {
		t.device.FlushCallbacks()
	}

	t.dc.BeginFrame()
	if draw != nil {
		draw(t.dc)
	}

	// Snapshot damage for metrics before PresentFrameAuto consumes the plan.
	// Clipped to the physical surface: damage fully outside it (e.g. spinner
	// rects below the viewport fold) presents nothing — acquiring a buffer
	// for them only to discard repeats the swapchain poison below every
	// frame, stalling the raster loop until acquire times out.
	union := t.dc.FrameDamageUnion()
	pw, ph := physicalSize(t.logicW, t.logicH, t.scale)
	clipped := union.Intersect(image.Rect(0, 0, int(pw), int(ph)))
	t.lastDamageArea = int64(clipped.Dx()) * int64(clipped.Dy())
	if t.lastDamageArea < 0 {
		t.lastDamageArea = 0
	}
	// Idle frames skip the swapchain entirely: acquiring a buffer and then
	// discarding it without present leaves the image unrecycled in the
	// driver, draining the swapchain until acquire times out (~250ms) and
	// forces a ~1s reconfigure loop on static windows.
	if !forceFull && clipped.Empty() && t.postResizeFull <= 0 && !t.inResizeStormLocked() {
		out = PresentOutcome{Mode: PresentModeIdle, Idle: true}
		t.lastOutcome = out
		return out, nil
	}

	frame, err := t.sc.BeginFrame()
	if err != nil {
		if os.Getenv("WR_RESIZE_DBG") == "1" {
			fmt.Fprintf(os.Stderr, "DBG present BeginFrame err=%v (logic %dx%d)\n", err, t.logicW, t.logicH)
		}
		// R0-6: a lost shared device recovers once for ALL windows
		// (abandon once + recreate once + reconfigure every swapchain)
		// instead of each window racing its own recovery on a device it
		// does not own. Drop t.mu across the slow recovery, then retry
		// the acquire exactly once.
		if t.shouldRecoverSharedLocked(err) {
			t.mu.Unlock()
			rerr := RecoverSharedDevice()
			t.mu.Lock()
			if rerr != nil {
				return out, fmt.Errorf("render: shared recovery: %v (present: %w)", rerr, err)
			}
			if t.closed || t.sc == nil || t.device == nil {
				return out, errors.New("render: PresentTarget closed during recovery")
			}
			frame, err = t.sc.BeginFrame()
			if err != nil {
				return out, fmt.Errorf("render: BeginFrame after recovery: %w", err)
			}
		} else {
			return out, fmt.Errorf("render: BeginFrame: %w", err)
		}
	}
	if os.Getenv("WR_RESIZE_DBG") == "1" {
		if d := time.Since(pBegin); d > 20*time.Millisecond {
			fmt.Fprintf(os.Stderr, "DBG phase begin=%dms (frame %dx%d)\n", d.Milliseconds(), frame.Width, frame.Height)
		}
		pEnd = time.Now()
	}
	if os.Getenv("WR_RESIZE_DBG") == "1" {
		fmt.Fprintf(os.Stderr, "DBG sc.frame %dx%d (logic %dx%d)\n", frame.Width, frame.Height, t.logicW, t.logicH)
	}
	// Extent check (Impeller KHRSwapchainVK::AcquireNextDrawable parity: a
	// successful acquire is still reconfigured when
	// `!out_of_date && size_ == GetSize()` fails). X11 does not report
	// "outdated" after programmatic resizes, so a matching acquire is not
	// proof the swapchain tracks the window — without this check the chain
	// stays at the stale extent forever and every present clips to it.
	// Like Impeller there is no storm deferral here: a mismatch reconfigures
	// at the recorded size and re-acquires once, every present if needed
	// (bounded: one forced reconfigure per present, convergence across
	// presents). Deferring inside the storm window left drag-resizes painting
	// clipped frames with blank regions until the storm settled; the
	// synchronous Configure costs ~1–5ms on real GPUs, and correctness
	// (content tracks the window) outranks saving it on software raster.
	if pw, ph := physicalSize(t.logicW, t.logicH, t.scale); frame.Width != pw || frame.Height != ph {
		if os.Getenv("WR_RESIZE_DBG") == "1" {
			fmt.Fprintf(os.Stderr, "DBG sc.frame stale %dx%d want %dx%d (force reconfig)\n", frame.Width, frame.Height, pw, ph)
		}
		t.sc.DiscardFrame(frame)
		var rApply time.Time
		if os.Getenv("WR_RESIZE_DBG") == "1" {
			rApply = time.Now()
		}
		if rerr := t.sc.Resize(pw, ph); rerr != nil {
			t.swapchainPending = true
			return out, fmt.Errorf("render: stale extent reconfigure: %w", rerr)
		}
		t.postResizeFull = 3
		t.lastResizeAt = time.Now()
		frame, err = t.sc.BeginFrame()
		if err != nil {
			if os.Getenv("WR_RESIZE_DBG") == "1" {
				fmt.Fprintf(os.Stderr, "DBG present BeginFrame err=%v (logic %dx%d)\n", err, t.logicW, t.logicH)
			}
			return out, fmt.Errorf("render: BeginFrame: %w", err)
		}
		if os.Getenv("WR_RESIZE_DBG") == "1" {
			fmt.Fprintf(os.Stderr, "DBG sc.frame %dx%d (logic %dx%d, forced reconfig=%dms)\n", frame.Width, frame.Height, t.logicW, t.logicH, time.Since(rApply).Milliseconds())
		}
	}
	// Failed/timeout BeginFrames above return early and do NOT consume the
	// post-resize full budget: the next frame still owes a full write.
	presentFn := func() error {
		if err := t.sc.EndFrame(frame); err != nil {
			return err
		}
		if os.Getenv("WR_RESIZE_DBG") == "1" {
			if d := time.Since(pEnd); d > 20*time.Millisecond {
				fmt.Fprintf(os.Stderr, "DBG phase endframe=%dms (frame %dx%d)\n", d.Milliseconds(), frame.Width, frame.Height)
			}
		}
		if t.postResizeFull > 0 {
			t.postResizeFull--
		}
		return nil
	}
	if forceFull {
		if err := t.dc.PresentFrameFull(frame.Handle, frame.Width, frame.Height, presentFn); err != nil {
			t.sc.DiscardFrame(frame)
			return out, fmt.Errorf("render: PresentFrameFull: %w", err)
		}
		out = PresentOutcome{Mode: PresentModeFull, Rects: 1}
		t.lastOutcome = out
		return out, nil
	}
	out, err = t.dc.PresentFrameAuto(frame.Handle, frame.Width, frame.Height, presentFn)
	if err != nil {
		t.sc.DiscardFrame(frame)
		return out, fmt.Errorf("render: PresentFrameAuto: %w", err)
	}
	if out.Idle {
		// Idle drew nothing and PresentFrameAuto does not call the present
		// callback, so the acquired swapchain frame would stay in-flight and
		// poison the next BeginFrame ("frame already in flight" → black
		// screen after resize storms / full-static retained frames). Release
		// the acquire explicitly to keep BeginFrame/Present paired.
		t.sc.DiscardFrame(frame)
	}
	t.lastOutcome = out
	return out, nil
}

// Close releases GPU resources. Safe to call multiple times.
// Windows on the borrowed share release only their own surface/swapchain/
// context; the shared instance/adapter/device go when the last PresentTarget
// (first or borrowed) closes.
func (t *PresentTarget) Close() error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true
	// R0-6: drop the recovery-registry entry on every close path below.
	defer unregisterSharedTarget(t)
	// R6 measurement: WR_MEMDIG=1 dumps the process VRAM ledger (live,
	// peak, count, budget) at window close for post-fix comparison.
	defer func() {
		if os.Getenv("WR_MEMDIG") == "1" {
			fmt.Fprintf(os.Stderr, "MEMDIG live=%.2fMiB peak=%.2fMiB count=%d budget=%dMiB\n",
				float64(VramLiveBytes())/(1024*1024),
				float64(VramPeakBytes())/(1024*1024),
				VramLiveCount(), VramBudgetMB())
		}
	}()
	if t.dc != nil {
		_ = t.dc.Close()
		t.dc = nil
	}
	if t.sc != nil {
		t.sc.Release()
		t.sc = nil
	}
	if t.surf != nil {
		t.surf.Release()
		t.surf = nil
	}
	if t.shared {
		t.device = nil
		t.adapter = nil
		t.inst = nil
		releaseShared()
		return nil
	}
	// R0-1: the owner window's device is the published share. Never Release
	// it directly while borrowers may hold refs — that leaves the share
	// pointer (and every borrower's handle) wild while releaseShared keeps
	// the non-nil pointer alive. Clear our aliases and let releaseShared
	// own the lifetime (last close releases).
	shareMu.Lock()
	isShare := t.device != nil && t.device == shareDevice
	shareMu.Unlock()
	if isShare {
		t.device = nil
		t.adapter = nil
		t.inst = nil
		releaseShared()
		return nil
	}
	// Orphan device (not the published share): release directly and do NOT
	// touch the share refcount, which belongs to another chain.
	if t.device != nil {
		t.device.Release()
		t.device = nil
	}
	if t.adapter != nil {
		t.adapter.Release()
		t.adapter = nil
	}
	if t.inst != nil {
		t.inst.Release()
		t.inst = nil
	}
	return nil
}
