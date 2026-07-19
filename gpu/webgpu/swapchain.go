//go:build !(js && wasm)

package webgpu

import (
	"fmt"
	"image"
	"sync"
	"time"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/rwgpu"
	"github.com/energye/gpui/gpu/types"
)

// Swapchain manages Configure → GetCurrentTexture → Present for a platform Surface (S.03/S6.8).
// It is the production path for window presentation; offscreen textures remain a headless stand-in.
//
// S6.8 additions:
//   - Stats (acquire/present/reconfigure/suboptimal)
//   - Auto reconfigure on suboptimal / outdated surface
//   - EndFrameWithDamage hook
//   - Present-mode preference (Fifo vs low-latency Mailbox)
type Swapchain struct {
	Surface     *Surface
	Device      *Device
	Width       uint32
	Height      uint32
	Format      TextureFormat
	Usage       TextureUsage
	PresentMode PresentMode
	AlphaMode   CompositeAlphaMode

	// PreferPresentModes, when non-empty, is tried in order during
	// ConfigureFromCapabilities (S6.8). Empty → prefer Fifo then first available.
	PreferPresentModes []PresentMode

	configured         bool
	pendingReconfigure bool
	// suboptHandledW/H: last extent for which we already acted on a suboptimal
	// signal. Prevents Configure thrash (and visible flicker) when the
	// compositor keeps reporting suboptimal for an unchanged size.
	suboptHandledW uint32
	suboptHandledH uint32

	// stats
	acquires       uint64
	presents       uint64
	discards       uint64
	reconfigures   uint64
	suboptimal     uint64
	acquireRetries uint64
	lastAcquireNs  int64
	lastPresentNs  int64

	// lastReconfig rate-limits native Surface.Configure. Continuous reconfigure
	// under long stress (S14) can abort wgpu-native ("failed to initiate panic").
	lastReconfig time.Time

	// frameOpen is true between a successful BeginFrame and EndFrame/DiscardFrame.
	// Enforces one-in-flight pairing: BeginFrame while open is an error.
	// Protected by frameMu for concurrent BeginFrame/EndFrame/DiscardFrame.
	frameMu   sync.Mutex
	frameOpen bool
}

// Frame is one acquired swapchain image ready for rendering.
type Frame struct {
	SurfaceTexture *SurfaceTexture
	View           *TextureView
	// Handle is the gpucontext-facing view for render.FlushGPUWithView.
	Handle     gpucontext.TextureView
	Suboptimal bool
	Width      uint32
	Height     uint32
	// DamageRects optionally records dirty regions for EndFrameWithDamage (S6.8).
	// wgpu-native currently ignores them at present; still used for diagnostics
	// and future partial-present backends.
	DamageRects []image.Rectangle
}

// SwapchainStats is S6.8 diagnostics for the window present path.
type SwapchainStats struct {
	Acquires       uint64
	Presents       uint64
	Discards       uint64
	Reconfigures   uint64
	Suboptimal     uint64
	AcquireRetries uint64
	LastAcquireNs  int64
	LastPresentNs  int64
	// Derived: last present wall time in milliseconds.
	LastPresentMs float64
	LastAcquireMs float64
}

// NewSwapchain builds a swapchain for an existing surface + device.
// Call Configure before BeginFrame.
func NewSwapchain(surface *Surface, device *Device, width, height uint32) *Swapchain {
	return &Swapchain{
		Surface:     surface,
		Device:      device,
		Width:       width,
		Height:      height,
		Format:      types.TextureFormatBGRA8Unorm,
		Usage:       types.TextureUsageRenderAttachment,
		PresentMode: PresentModeFifo,
		AlphaMode:   types.CompositeAlphaModeOpaque,
	}
}

// Stats returns cumulative present-path counters.
func (sc *Swapchain) Stats() SwapchainStats {
	if sc == nil {
		return SwapchainStats{}
	}
	st := SwapchainStats{
		Acquires:       sc.acquires,
		Presents:       sc.presents,
		Discards:       sc.discards,
		Reconfigures:   sc.reconfigures,
		Suboptimal:     sc.suboptimal,
		AcquireRetries: sc.acquireRetries,
		LastAcquireNs:  sc.lastAcquireNs,
		LastPresentNs:  sc.lastPresentNs,
	}
	st.LastAcquireMs = float64(sc.lastAcquireNs) / 1e6
	st.LastPresentMs = float64(sc.lastPresentNs) / 1e6
	return st
}

// ResetStats clears counters (configuration retained).
func (sc *Swapchain) ResetStats() {
	if sc == nil {
		return
	}
	sc.acquires = 0
	sc.presents = 0
	sc.discards = 0
	sc.reconfigures = 0
	sc.suboptimal = 0
	sc.acquireRetries = 0
	sc.lastAcquireNs = 0
	sc.lastPresentNs = 0
}

// SetPreferVSync selects Fifo when available (default production UI path).
func (sc *Swapchain) SetPreferVSync() {
	if sc == nil {
		return
	}
	sc.PreferPresentModes = []PresentMode{PresentModeFifo, PresentModeFifoRelaxed, PresentModeMailbox, PresentModeImmediate}
}

// SetPreferLowLatency prefers Mailbox/Immediate when supported (gamesing; may tear).
func (sc *Swapchain) SetPreferLowLatency() {
	if sc == nil {
		return
	}
	sc.PreferPresentModes = []PresentMode{PresentModeMailbox, PresentModeFifoRelaxed, PresentModeFifo, PresentModeImmediate}
}

// Configure applies SurfaceConfiguration to the underlying surface.
// Prefer choosing Format/PresentMode from Adapter.GetSurfaceCapabilities when available.
func (sc *Swapchain) Configure() error {
	if sc == nil {
		return fmt.Errorf("wgpu: swapchain is nil")
	}
	if sc.Surface == nil {
		return fmt.Errorf("wgpu: swapchain surface is nil")
	}
	if sc.Device == nil {
		return fmt.Errorf("wgpu: swapchain device is nil")
	}
	if sc.Width == 0 || sc.Height == 0 {
		return fmt.Errorf("wgpu: swapchain extent must be non-zero")
	}
	if sc.Usage == 0 {
		sc.Usage = types.TextureUsageRenderAttachment
	}
	if sc.Format == 0 {
		sc.Format = types.TextureFormatBGRA8Unorm
	}
	if sc.PresentMode == 0 {
		sc.PresentMode = PresentModeFifo
	}
	if sc.AlphaMode == 0 {
		sc.AlphaMode = types.CompositeAlphaModeOpaque
	}
	cfg := &SurfaceConfiguration{
		Width:       sc.Width,
		Height:      sc.Height,
		Format:      sc.Format,
		Usage:       sc.Usage,
		PresentMode: sc.PresentMode,
		AlphaMode:   sc.AlphaMode,
	}
	if err := sc.Surface.Configure(sc.Device, cfg); err != nil {
		return err
	}
	sc.configured = true
	sc.pendingReconfigure = false
	sc.suboptHandledW = sc.Width
	sc.suboptHandledH = sc.Height
	sc.reconfigures++
	return nil
}

// ConfigureFromCapabilities picks a supported format/present/alpha mode then configures.
func (sc *Swapchain) ConfigureFromCapabilities(adapter *Adapter) error {
	if sc == nil || adapter == nil {
		return fmt.Errorf("wgpu: swapchain/adapter nil")
	}
	caps := adapter.GetSurfaceCapabilities(sc.Surface)
	if caps != nil {
		if len(caps.Formats) > 0 {
			sc.Format = caps.Formats[0]
			// Prefer BGRA8Unorm when listed (common on Windows/Linux).
			for _, f := range caps.Formats {
				if f == types.TextureFormatBGRA8Unorm || f == types.TextureFormatRGBA8Unorm {
					sc.Format = f
					break
				}
			}
		}
		sc.PresentMode = pickPresentMode(caps.PresentModes, sc.PreferPresentModes)
		if len(caps.AlphaModes) > 0 {
			sc.AlphaMode = caps.AlphaModes[0]
			for _, am := range caps.AlphaModes {
				if am == types.CompositeAlphaModeOpaque {
					sc.AlphaMode = am
					break
				}
			}
		}
	}
	return sc.Configure()
}

func pickPresentMode(available []PresentMode, prefer []PresentMode) PresentMode {
	if len(available) == 0 {
		return PresentModeFifo
	}
	has := func(m PresentMode) bool {
		for _, a := range available {
			if a == m {
				return true
			}
		}
		return false
	}
	// Explicit preference list.
	for _, p := range prefer {
		if has(p) {
			return p
		}
	}
	// Default: Fifo (vsync) for steady UI.
	if has(PresentModeFifo) {
		return PresentModeFifo
	}
	return available[0]
}

// Resize updates extent and reconfigures.
func (sc *Swapchain) Resize(width, height uint32) error {
	if sc == nil {
		return fmt.Errorf("wgpu: swapchain is nil")
	}
	sc.Width = width
	sc.Height = height
	// New extent: allow a single post-resize suboptimal recovery if needed.
	sc.suboptHandledW = 0
	sc.suboptHandledH = 0
	return sc.Configure()
}

// MarkNeedsReconfigure schedules a reconfigure on the next BeginFrame (S6.8).
// Call after window resize events or when the compositor reports outdated.
func (sc *Swapchain) MarkNeedsReconfigure() {
	if sc != nil {
		sc.pendingReconfigure = true
	}
}

// BeginFrame acquires the next surface texture and creates a render view.
// Caller must EndFrame (or DiscardFrame) exactly once per successful BeginFrame.
//
// Pre-checks (all refuse before native acquire):
//   - sticky/process device-lost fuse
//   - device IsLost
//   - surface non-nil / not released
//   - no frame already open (pairing)
//
// Error policy (library-generic; window policy is downstream):
//   - DeviceLost: return ErrDeviceLost; never reconfigure (native may abort)
//   - Occluded / Timeout: return as-is; skip frame, do not reconfigure
//   - Outdated / SurfaceLost: reconfigure once and retry acquire
//   - Other: reconfigure once and retry (best-effort recovery)
func (sc *Swapchain) BeginFrame() (*Frame, error) {
	if sc == nil {
		return nil, fmt.Errorf("wgpu: swapchain is nil")
	}
	sc.frameMu.Lock()
	defer sc.frameMu.Unlock()
	// Triple pre-check: fuse / device / surface.
	if rwgpu.AnyDeviceLost() || (sc.Device != nil && sc.Device.IsLost()) {
		return nil, ErrDeviceLost
	}
	if sc.Surface == nil {
		return nil, ErrInvalidHandle
	}
	if sc.Surface.released {
		return nil, ErrReleased
	}
	if sc.frameOpen {
		return nil, ErrFrameInFlight
	}
	if sc.Width == 0 || sc.Height == 0 {
		return nil, fmt.Errorf("wgpu: swapchain extent must be non-zero")
	}
	if sc.pendingReconfigure || !sc.configured {
		if err := sc.reconfigureThrottled(); err != nil {
			// Keep pending; caller should skip frame rather than thrash native.
			return nil, err
		}
	}

	// Pump device-lost / uncaptured callbacks before native acquire. Without
	// this, a spontaneous device-lost (common after minimize / long occlude)
	// may not set the sticky fuse until after GetCurrentTexture aborts.
	if sc.Device != nil {
		sc.Device.FlushCallbacks()
		if sc.Device.IsLost() || rwgpu.AnyDeviceLost() {
			return nil, ErrDeviceLost
		}
	}

	t0 := time.Now()
	st, suboptimal, err := sc.Surface.GetCurrentTexture()
	if err != nil {
		// Terminal / skip paths: do not thrash Configure.
		if isDeviceLostErr(err) || (sc.Device != nil && sc.Device.IsLost()) {
			return nil, ErrDeviceLost
		}
		if isSkipFrameSurfaceErr(err) {
			return nil, err
		}
		// Outdated / lost surface / other hard acquire failure: reconfigure once.
		if sc.Surface != nil {
			sc.Surface.DiscardTexture()
		}
		if !isOutdatedSurfaceErr(err) {
			// Unknown error: still try one reconfigure (historical recovery),
			// but never on skip/device-lost (handled above).
		}
		if cfgErr := sc.Configure(); cfgErr != nil {
			if isDeviceLostErr(cfgErr) {
				return nil, ErrDeviceLost
			}
			return nil, fmt.Errorf("%w (reconfigure: %v)", err, cfgErr)
		}
		sc.lastReconfig = time.Now()
		sc.acquireRetries++
		st, suboptimal, err = sc.Surface.GetCurrentTexture()
		if err != nil {
			if isDeviceLostErr(err) {
				return nil, ErrDeviceLost
			}
			// Second failure: propagate mapped sentinel; do not loop.
			return nil, err
		}
	}
	sc.lastAcquireNs = time.Since(t0).Nanoseconds()
	sc.acquires++

	view, err := st.CreateView(nil)
	if err != nil {
		// Drop acquired surface texture or next Configure panics native:
		// "SurfaceOutput must be dropped before a new Surface is made".
		if st != nil {
			st.Release()
		}
		return nil, fmt.Errorf("wgpu: surface texture CreateView: %w", err)
	}
	if suboptimal {
		sc.suboptimal++
		// Act once per extent. Continuous reconfigure of the same size causes
		// black flashes and burns CPU without improving the surface.
		if sc.suboptHandledW != sc.Width || sc.suboptHandledH != sc.Height {
			sc.pendingReconfigure = true
		}
	}
	sc.frameOpen = true
	return &Frame{
		SurfaceTexture: st,
		View:           view,
		Handle:         TextureViewToHandle(view),
		Suboptimal:     suboptimal,
		Width:          sc.Width,
		Height:         sc.Height,
	}, nil
}

// reconfigureThrottled runs Configure at most once per 500ms to avoid native
// Surface.Configure thrash under long multi-module stress (S14 soak crash).
func (sc *Swapchain) reconfigureThrottled() error {
	const minInterval = 500 * time.Millisecond
	if sc == nil {
		return fmt.Errorf("wgpu: swapchain is nil")
	}
	if sc.Device != nil && sc.Device.IsLost() {
		return ErrDeviceLost
	}
	if !sc.lastReconfig.IsZero() && time.Since(sc.lastReconfig) < minInterval {
		sc.pendingReconfigure = true
		return fmt.Errorf("wgpu: surface reconfigure rate-limited")
	}
	// Best-effort drop of any dangling surface output before reconfigure.
	if sc.Surface != nil {
		sc.Surface.DiscardTexture()
	}
	if err := sc.Configure(); err != nil {
		return err
	}
	sc.lastReconfig = time.Now()
	return nil
}

// EndFrame presents the frame to the platform surface.
func (sc *Swapchain) EndFrame(frame *Frame) error {
	return sc.endFrame(frame, nil)
}

// EndFrameWithDamage presents the frame, forwarding damage rects when the
// backend supports partial present (wgpu-native currently ignores them).
func (sc *Swapchain) EndFrameWithDamage(frame *Frame, rects []image.Rectangle) error {
	return sc.endFrame(frame, rects)
}

func (sc *Swapchain) endFrame(frame *Frame, rects []image.Rectangle) error {
	if sc == nil {
		return fmt.Errorf("wgpu: swapchain is nil")
	}
	if frame == nil {
		return fmt.Errorf("wgpu: frame is nil")
	}
	sc.frameMu.Lock()
	defer sc.frameMu.Unlock()
	if !sc.frameOpen {
		return ErrNoFrame
	}
	if frame.View != nil {
		frame.View.Release()
		frame.View = nil
	}
	if frame.Suboptimal {
		if sc.suboptHandledW != sc.Width || sc.suboptHandledH != sc.Height {
			sc.pendingReconfigure = true
		}
	}
	t0 := time.Now()
	var err error
	if sc.Surface == nil {
		err = ErrInvalidHandle
	} else if len(rects) > 0 {
		err = sc.Surface.PresentWithDamage(frame.SurfaceTexture, rects)
	} else if len(frame.DamageRects) > 0 {
		err = sc.Surface.PresentWithDamage(frame.SurfaceTexture, frame.DamageRects)
	} else {
		err = sc.Surface.Present(frame.SurfaceTexture)
	}
	sc.lastPresentNs = time.Since(t0).Nanoseconds()
	sc.presents++
	// After Present, drop ReturnedWithOwnership surface texture.
	// Must happen after Present so surface no longer holds "current" image.
	if frame.SurfaceTexture != nil {
		frame.SurfaceTexture.Release()
		frame.SurfaceTexture = nil
	}
	sc.frameOpen = false
	return err
}

// DiscardFrame drops an acquired frame without presenting.
// Releases the surface texture immediately (ReturnedWithOwnership).
func (sc *Swapchain) DiscardFrame(frame *Frame) {
	if frame == nil {
		return
	}
	if frame.View != nil {
		frame.View.Release()
		frame.View = nil
	}
	if frame.SurfaceTexture != nil {
		frame.SurfaceTexture.Release()
		frame.SurfaceTexture = nil
	}
	if sc != nil {
		sc.frameMu.Lock()
		sc.discards++
		sc.frameOpen = false
		sc.frameMu.Unlock()
	}
}

// PresentModeName returns a short label for the active present mode.
func (sc *Swapchain) PresentModeName() string {
	if sc == nil {
		return "nil"
	}
	switch sc.PresentMode {
	case PresentModeFifo:
		return "fifo"
	case PresentModeFifoRelaxed:
		return "fifo-relaxed"
	case PresentModeMailbox:
		return "mailbox"
	case PresentModeImmediate:
		return "immediate"
	default:
		return fmt.Sprintf("mode(%d)", int(sc.PresentMode))
	}
}

// Release unconfigures the surface; does not release Surface/Device ownership.
func (sc *Swapchain) Release() {
	if sc == nil || sc.Surface == nil {
		return
	}
	sc.Surface.Unconfigure()
	sc.configured = false
}
