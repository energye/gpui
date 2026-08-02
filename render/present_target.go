package render

import (
	"errors"
	"fmt"
	"sync"

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

	device, err := adapter.RequestDevice(presentDeviceDescriptor("ui-l1-present"))
	if err != nil {
		adapter.Release()
		surf.Release()
		inst.Release()
		return nil, fmt.Errorf("render: RequestDevice: %w", err)
	}

	physW, physH := physicalSize(logicalW, logicalH, scale)
	sc := webgpu.NewSwapchain(surf, device, physW, physH)
	sc.Usage = types.TextureUsageRenderAttachment
	sc.SetPreferVSync()
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		device.Release()
		adapter.Release()
		surf.Release()
		inst.Release()
		return nil, fmt.Errorf("render: Configure swapchain: %w", err)
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

// Resize updates logical size / scale and reconfigures the swapchain.
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
	pw, ph := physicalSize(logicalW, logicalH, scale)
	if t.sc != nil {
		if err := t.sc.Resize(pw, ph); err != nil {
			return err
		}
		// Swapchain reconfigured → every buffer is undefined. Owe full frames
		// so each buffer is fully written before retained LoadOpLoad frames
		// read it again (otherwise resize storms leave black regions).
		t.postResizeFull = 3
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
// frames are still owed to its buffers. Callers that would skip work for
// retained steady frames (e.g. compositeOnly) must disable that path while
// this is true so every new buffer gets fully written.
func (t *PresentTarget) InFullRecovery() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.postResizeFull > 0
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
	if t.postResizeFull > 0 {
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
		return out, fmt.Errorf("render: BeginFrame: %w", err)
	}
	// Failed/timeout BeginFrames above return early and do NOT consume the
	// post-resize full budget: the next frame still owes a full write.
	presentFn := func() error {
		if err := t.sc.EndFrame(frame); err != nil {
			return err
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
