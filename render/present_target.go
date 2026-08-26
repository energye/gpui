package render

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// presentDeviceDescriptor builds device limits for UI present (avoid import cycle
// with render/gpu which imports render).
func presentDeviceDescriptor(label string) *webgpu.DeviceDescriptor {
	const minStorageBuffers = 9
	limits := webgpu.DefaultLimits()
	if limits.MaxStorageBuffersPerShaderStage < minStorageBuffers {
		limits.MaxStorageBuffersPerShaderStage = minStorageBuffers
	}
	return &webgpu.DeviceDescriptor{
		Label:          label,
		RequiredLimits: limits,
	}
}

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
		low := strings.ToLower(err.Error())
		if !strings.Contains(low, "not enough memory") && !strings.Contains(low, "out of memory") {
			return nil, err
		}
		// Present device creation is not on a hot path; a short backoff
		// gives other processes time to release GPU memory.
		time.Sleep(time.Second)
	}
	return nil, err
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
// Owns instance/adapter/device/surface/swapchain + a drawing Context.
type PresentTarget struct {
	mu sync.Mutex

	ns     PresentNativeSurface
	logicW int
	logicH int
	scale  float64

	inst    *webgpu.Instance
	adapter *webgpu.Adapter
	device  *webgpu.Device
	surf    *webgpu.Surface
	sc      *webgpu.Swapchain
	dc      *Context

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

	adapter, err := inst.RequestAdapter(&webgpu.RequestAdapterOptions{
		PowerPreference: webgpu.PowerPreferenceHighPerformance,
	})
	if err != nil {
		surf.Release()
		inst.Release()
		return nil, fmt.Errorf("render: RequestAdapter: %w", err)
	}
	if os.Getenv("WR_RESIZE_DBG") == "1" {
		ai := adapter.Info()
		fmt.Fprintf(os.Stderr, "DBG adapter=%q vendor=%q type=%v backend=%v\n",
			ai.Name, ai.Vendor, ai.DeviceType, ai.Backend)
	}

	device, err := requestPresentDeviceWithRetry(adapter, presentDeviceDescriptor("ui-l1-present"), "ui-l1-present")
	if err != nil {
		adapter.Release()
		surf.Release()
		inst.Release()
		return nil, fmt.Errorf("render: RequestDevice: %w", err)
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

	// Bind shared device for GPU-accelerated draws (blank clear still works if this fails).
	_ = SetAcceleratorDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: device, Adpt: adapter, Format: sc.Format,
	})

	dc := NewContext(logicalW, logicalH, WithDeviceScale(scale))

	return &PresentTarget{
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
	}, nil
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

// Resize records a new logical size / scale. The swapchain reconfigure itself
// is deferred to the raster present boundary (applyPendingSwapchain) so the
// UI thread never blocks on the surface (and never races the raster thread's
// present). Same-size calls are no-ops.
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
	if t.dc != nil {
		_ = t.dc.Resize(logicalW, logicalH)
		t.dc.SetDeviceScale(scale)
	}
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
// X11/Xwayland: the reconfigure is deferred to BeginFrame's outdated-retry
// (outdated acquire → probe the live window size → Configure → re-acquire)
// instead of a synchronous Configure here — on X11/Xwayland each Configure
// costs 26–383ms (scaling with window size), so configuring synchronously on
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
	union := t.dc.FrameDamageUnion()
	t.lastDamageArea = int64(union.Dx()) * int64(union.Dy())
	if t.lastDamageArea < 0 {
		t.lastDamageArea = 0
	}

	frame, err := t.sc.BeginFrame()
	if err != nil {
		if os.Getenv("WR_RESIZE_DBG") == "1" {
			fmt.Fprintf(os.Stderr, "DBG present BeginFrame err=%v (logic %dx%d)\n", err, t.logicW, t.logicH)
		}
		return out, fmt.Errorf("render: BeginFrame: %w", err)
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
