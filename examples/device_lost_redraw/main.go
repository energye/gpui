//go:build linux && !nogpu

// device_lost_redraw — zero-config: RequestRedraw 60fps + render goroutine + device-lost lifecycle.
//
//	go run ./examples/device_lost_redraw
//
// No flags / env required when run from repo root with lib/libwgpu_native.so present.
// Window is freely resizable. Close the window or Ctrl+C to exit.
package main

import (
	"errors"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	rendgpu "github.com/energye/gpui/render/gpu"
)

const (
	defaultW      = 960
	defaultH      = 640
	targetFPS     = 60
	logEveryFrame = 60
	// Apply Surface.Configure only after ConfigureNotify is idle this long.
	// Mid-drag we keep presenting the existing surface (compositor scales) so
	// there is no black frame from swapchain recreation — Skia/Flutter model.
	resizeConfigureIdle = 32 * time.Millisecond
)

// redrawHost coalesces frame requests (cap=1). Safe from any goroutine.
type redrawHost struct {
	ch chan struct{}
}

func newRedrawHost() *redrawHost {
	return &redrawHost{ch: make(chan struct{}, 1)}
}

// RequestRedraw is non-blocking; at most one pending redraw.
func (h *redrawHost) RequestRedraw() {
	if h == nil {
		return
	}
	select {
	case h.ch <- struct{}{}:
	default:
	}
}

func (h *redrawHost) wait(stop <-chan struct{}) bool {
	select {
	case <-h.ch:
		return true
	case <-stop:
		return false
	}
}

// hostState is updated on the X11 thread and snapshotted by the render thread.
type hostState struct {
	mu sync.Mutex

	// Live client size (updated on every ConfigureNotify).
	w, h int
	// lastCfgAt: last ConfigureNotify. While "hot", avoid Surface.Configure and
	// keep presenting the current surface (compositor scales) — Skia/Flutter style
	// zero-flash resize. When quiet, apply Resize once + full present.
	lastCfgAt time.Time

	minimized bool
	obscured  bool
	focused   bool
	mapped    bool

	forceFull bool
	quit      bool

	hiddenSince time.Time
	wasHidden   bool
}

func (s *hostState) snapshot() (w, h int, presentable, forceFull, quit bool, hiddenFor time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, h = s.w, s.h
	quit = s.quit
	forceFull = s.forceFull
	hidden := !s.mapped || s.minimized || s.obscured
	presentable = !hidden && !quit
	if s.wasHidden && presentable && !s.hiddenSince.IsZero() {
		hiddenFor = time.Since(s.hiddenSince)
	}
	return
}

func (s *hostState) clearForceFull() {
	s.mu.Lock()
	s.forceFull = false
	s.mu.Unlock()
}

func (s *hostState) markForceFull() {
	s.mu.Lock()
	s.forceFull = true
	s.mu.Unlock()
}

// setLiveSize updates the target client size immediately (free resize).
func (s *hostState) setLiveSize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastCfgAt = time.Now()
	if w == s.w && h == s.h {
		return
	}
	s.w, s.h = w, h
	s.forceFull = true
}

// resizeQuiet reports whether ConfigureNotify has been idle long enough to
// safely Surface.Configure (sharp pixels) without thrashing mid-drag.
func (s *hostState) resizeQuiet(idle time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastCfgAt.IsZero() {
		return true
	}
	return time.Since(s.lastCfgAt) >= idle
}

func (s *hostState) setPresentableFlags(mapped, minimized, obscured bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	was := !s.mapped || s.minimized || s.obscured
	s.mapped, s.minimized, s.obscured = mapped, minimized, obscured
	nowHidden := !s.mapped || s.minimized || s.obscured
	if nowHidden && !was {
		s.wasHidden = true
		s.hiddenSince = time.Now()
		log.Printf("host: unpresentable (mapped=%v minimized=%v obscured=%v) — pause acquire",
			s.mapped, s.minimized, s.obscured)
	}
	if !nowHidden && was {
		log.Printf("host: presentable again (hidden≈%.1fs)", time.Since(s.hiddenSince).Seconds())
		s.forceFull = true
	}
}

// bootstrapDefaults makes the sample runnable without env vars when started
// from the repo (or with lib next to the binary).

// fpsCounter estimates FPS as successful presents in the last second.
type fpsCounter struct {
	times []time.Time
}

func (c *fpsCounter) add(now time.Time) {
	c.times = append(c.times, now)
	cut := now.Add(-time.Second)
	i := 0
	for i < len(c.times) && !c.times[i].After(cut) {
		i++
	}
	if i > 0 {
		c.times = append([]time.Time(nil), c.times[i:]...)
	}
}

func (c *fpsCounter) rate(now time.Time) float64 {
	cut := now.Add(-time.Second)
	n := 0
	for _, t := range c.times {
		if t.After(cut) {
			n++
		}
	}
	if n == 0 {
		return 0
	}
	// If window shorter than 1s (startup), scale by elapsed span.
	if len(c.times) >= 2 {
		span := c.times[len(c.times)-1].Sub(c.times[0]).Seconds()
		if span > 0.05 && span < 0.95 {
			return float64(len(c.times)-1) / span
		}
	}
	return float64(n)
}

func bootstrapDefaults() {
	// Prefer in-repo native lib for dlopen (also helps when LD_LIBRARY_PATH unset).
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		for _, p := range candidateNativeLibs() {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				_ = os.Setenv("WGPU_NATIVE_PATH", p)
				// Best-effort for transitive loads; already-running process may ignore.
				if cur := os.Getenv("LD_LIBRARY_PATH"); cur == "" {
					_ = os.Setenv("LD_LIBRARY_PATH", filepath.Dir(p))
				} else {
					_ = os.Setenv("LD_LIBRARY_PATH", filepath.Dir(p)+string(os.PathListSeparator)+cur)
				}
				log.Printf("using native lib %s", p)
				break
			}
		}
	}
	// DISPLAY: leave as-is if set; openX11Window tries :0/:1 fallbacks.
	if os.Getenv("GPUI_SURFACE_SAMPLE_COUNT") == "" {
		_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")
	}
}

func candidateNativeLibs() []string {
	name := "libwgpu_native.so"
	var out []string
	// CWD variants (go run from repo root).
	out = append(out,
		filepath.Join("lib", name),
		name,
		filepath.Join("..", "lib", name),
		filepath.Join("..", "..", "lib", name),
	)
	// Executable directory (built binary next to / under repo).
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		out = append(out,
			filepath.Join(dir, "lib", name),
			filepath.Join(dir, name),
			filepath.Join(dir, "..", "lib", name),
			filepath.Join(dir, "..", "..", "lib", name),
		)
	}
	// Absolute repo-style path if user runs from elsewhere but home tree is fixed — skip.
	return out
}

func main() {
	runtime.LockOSThread()
	bootstrapDefaults()

	framePeriod := time.Second / time.Duration(targetFPS)

	xw, err := openX11Window(defaultW, defaultH, "gpui device_lost_redraw")
	if err != nil {
		log.Fatalf("X11: %v", err)
	}
	defer xw.Close()

	// Drain early configure (real WM size; free resize thereafter).
	winW, winH := defaultW, defaultH
	for i := 0; i < 32; i++ {
		for xw.Pending() {
			ev := xw.NextEvent()
			if ev.Type == xConfigureNotify && ev.Width >= 1 && ev.Height >= 1 {
				if ev.Width >= 64 {
					winW = ev.Width
				}
				if ev.Height >= 64 {
					winH = ev.Height
				}
			}
		}
		time.Sleep(4 * time.Millisecond)
	}

	inst, err := webgpu.CreateInstance(&webgpu.InstanceDescriptor{Backends: webgpu.BackendsPrimary})
	if err != nil {
		log.Fatalf("CreateInstance: %v", err)
	}
	defer inst.Release()

	surf, err := inst.CreateSurface(xw.Display, xw.Window)
	if err != nil {
		log.Fatalf("CreateSurface: %v", err)
	}
	defer surf.Release()

	adapter, err := inst.RequestAdapter(&webgpu.RequestAdapterOptions{
		PowerPreference:   webgpu.PowerPreferenceHighPerformance,
		CompatibleSurface: surf,
	})
	if err != nil {
		log.Fatalf("RequestAdapter: %v", err)
	}
	defer adapter.Release()

	device, err := adapter.RequestDevice(rendgpu.DeviceDescriptor("device-lost-redraw"))
	if err != nil {
		log.Fatalf("RequestDevice: %v", err)
	}
	var deviceMu sync.Mutex
	getDevice := func() *webgpu.Device {
		deviceMu.Lock()
		defer deviceMu.Unlock()
		return device
	}
	setDevice := func(d *webgpu.Device) {
		deviceMu.Lock()
		device = d
		deviceMu.Unlock()
	}
	defer func() {
		if d := getDevice(); d != nil {
			d.Release()
		}
	}()

	sc := webgpu.NewSwapchain(surf, device, uint32(winW), uint32(winH))
	sc.Usage = types.TextureUsageRenderAttachment
	sc.SetPreferVSync()
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		log.Fatalf("Configure: %v", err)
	}
	defer sc.Release()

	if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: device, Adpt: adapter, Format: sc.Format,
	}); err != nil {
		log.Fatalf("SetDeviceProvider: %v", err)
	}
	defer func() { _ = rendgpu.ResetAccelerator() }()

	sc.OnDeviceAbandon = func(_ *webgpu.Device) {
		rendgpu.AbandonDevice() // free GPUShared before old Destroy/Release
	}
	sc.EnableAutoRecover(adapter, "device-lost-redraw", func(dev *webgpu.Device) {
		setDevice(dev)
		if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
			Dev: dev, Adpt: adapter, Format: sc.Format,
		}); err != nil {
			log.Printf("SetDeviceProvider after recover: %v", err)
		}
		log.Printf("GPU device recovered (recoveries→%d)", sc.Recoveries()+1) // counter bumps after callback
	})

	dc := render.NewContext(winW, winH)
	defer dc.Close()

	host := &hostState{w: winW, h: winH, focused: true, mapped: true}
	redraw := newRedrawHost()

	stop := make(chan struct{})
	var stopOnce sync.Once
	doStop := func() {
		stopOnce.Do(func() {
			host.mu.Lock()
			host.quit = true
			host.mu.Unlock()
			close(stop)
			redraw.RequestRedraw()
		})
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Printf("signal — exit")
		doStop()
	}()

	var (
		frameN    atomic.Uint64
		presentN  atomic.Uint64
		skipLostN atomic.Uint64
		skipSoftN atomic.Uint64
	)

	// Keep-alive when unpresentable (pump callbacks). Visible path: Fifo Present
	// + frame-budget sleep to targetFPS, then RequestRedraw for the next frame.
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				_, _, presentable, _, quit, _ := host.snapshot()
				if quit {
					return
				}
				if !presentable {
					redraw.RequestRedraw()
				} else {
					// Drive quiet-configure transition after drag stops.
					hw, hh, _, _, _, _ := host.snapshot()
					if uint32(hw) != sc.Width || uint32(hh) != sc.Height {
						redraw.RequestRedraw()
					}
				}
			}
		}
	}()

	// Render goroutine — sole GPU owner
	var renderWG sync.WaitGroup
	renderWG.Add(1)
	go func() {
		defer renderWG.Done()
		runtime.LockOSThread()
		start := time.Now()
		var lastLog time.Time
		var lastResizeLog time.Time
		var fpsWindow fpsCounter
		for {
			if !redraw.wait(stop) {
				return
			}
			w, h, presentable, forceFull, quit, hiddenFor := host.snapshot()
			if quit {
				return
			}
			dev := getDevice()
			if dev != nil {
				dev.FlushCallbacks()
			}

			if !presentable {
				time.Sleep(8 * time.Millisecond)
				continue
			}

			frameStart := time.Now()

			if hiddenFor > 0 {
				host.mu.Lock()
				host.wasHidden = false
				host.hiddenSince = time.Time{}
				host.mu.Unlock()
				sc.MarkNeedsReconfigure()
				sc.ClearRecoverCooldown()
				host.markForceFull()
			}

			// Free resize (Skia/Flutter zero-flash model):
			//  1) Mid-drag (ConfigureNotify still firing): do NOT Surface.Configure.
			//     Present at current surface size; compositor scales → continuous image.
			//  2) Idle ≥ resizeConfigureIdle: one Resize + full present (sharp).
			//  3) If BeginFrame reports Outdated: force Resize (must reconfigure).
			if w < 1 {
				w = 1
			}
			if h < 1 {
				h = 1
			}
			targetW, targetH := w, h
			surfMismatch := uint32(targetW) != sc.Width || uint32(targetH) != sc.Height
			sizeChanged := false
			if surfMismatch && host.resizeQuiet(resizeConfigureIdle) {
				if err := sc.Resize(uint32(targetW), uint32(targetH)); err != nil {
					log.Printf("sc.Resize: %v", err)
				} else if err := dc.Resize(targetW, targetH); err != nil {
					log.Printf("dc.Resize: %v", err)
				} else {
					sizeChanged = true
					forceFull = true
					w, h = targetW, targetH
					if time.Since(lastResizeLog) > 200*time.Millisecond {
						lastResizeLog = time.Now()
						log.Printf("resized → %dx%d (quiet configure)", w, h)
					}
				}
			}
			// While surface lags window mid-drag, draw for the SURFACE size (scaled by WM).
			if !sizeChanged {
				w, h = int(sc.Width), int(sc.Height)
				if w < 1 {
					w = 1
				}
				if h < 1 {
					h = 1
				}
				if w != dc.Width() || h != dc.Height() {
					_ = dc.Resize(w, h)
					forceFull = true
				}
			}

			// Skia order: ensure surface/device (recover) BEFORE recording draw commands.
			// Drawing first then recovering leaves pipelines/bind groups on the old device
			// → Present "resource already released" (seen after minimize/restore).
			fb, err := sc.BeginFrame()
			if err != nil {
				if errors.Is(err, webgpu.ErrDeviceLost) || (dev != nil && dev.IsLost()) {
					skipLostN.Add(1)
					log.Printf("BeginFrame: device lost/recovering (recoveries=%d)", sc.Recoveries())
					sc.ClearRecoverCooldown()
					if sc.Device != nil {
						setDevice(sc.Device)
					}
					host.markForceFull()
					time.Sleep(framePeriod)
					redraw.RequestRedraw()
					continue
				}
				skipSoftN.Add(1)
				host.markForceFull()
				if errors.Is(err, webgpu.ErrSurfaceOutdated) || errors.Is(err, webgpu.ErrSurfaceLost) {
					// Outdated: must reconfigure now (cannot present old surface).
					tw, th, _, _, _, _ := host.snapshot()
					if tw < 1 {
						tw = 1
					}
					if th < 1 {
						th = 1
					}
					if rerr := sc.Resize(uint32(tw), uint32(th)); rerr != nil {
						log.Printf("sc.Resize(outdated): %v", rerr)
						sc.MarkNeedsReconfigure()
					} else {
						_ = dc.Resize(tw, th)
					}
				}
				// Do not sleep long — keep presenting to avoid black gap.
				redraw.RequestRedraw()
				continue
			}
			if sc.Device != nil && sc.Device != getDevice() {
				setDevice(sc.Device)
			}

			elapsed := time.Since(start).Seconds()
			n := frameN.Add(1)
			// Wall-clock FPS over a ~1s window (stable; not 1/dt which spikes on fast Present).
			hud := drawStats{Frame: n, FPS: fpsWindow.rate(time.Now()), Seconds: elapsed}

			dc.BeginFrame()
			if forceFull || sizeChanged {
				dc.MarkFullRedraw()
			}
			drawCompositeFrame(dc, w, h, elapsed, hud)
			if forceFull {
				host.clearForceFull()
			}

			presentFn := func() error { return sc.EndFrame(fb) }
			var perr error
			if sizeChanged || forceFull {
				// New swapchain extent: always full clear+draw path (no partial damage idle/black).
				perr = dc.PresentFrameFull(fb.Handle, fb.Width, fb.Height, presentFn)
			} else {
				_, perr = dc.PresentFrameAuto(fb.Handle, fb.Width, fb.Height, presentFn)
			}
			if perr != nil {
				sc.DiscardFrame(fb)
				if errors.Is(perr, webgpu.ErrDeviceLost) || (getDevice() != nil && getDevice().IsLost()) {
					skipLostN.Add(1)
					log.Printf("Present: device lost/recovering (recoveries=%d)", sc.Recoveries())
					sc.ClearRecoverCooldown()
					if sc.Device != nil {
						setDevice(sc.Device)
					}
					host.markForceFull()
					time.Sleep(framePeriod)
					redraw.RequestRedraw()
					continue
				}
				// After recover mid-lifecycle, stale GPU objects can still surface once.
				skipSoftN.Add(1)
				log.Printf("Present: %v", perr)
				host.markForceFull()
				if sc.Device != nil {
					_ = rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
						Dev: sc.Device, Adpt: adapter, Format: sc.Format,
					})
					setDevice(sc.Device)
				}
				time.Sleep(framePeriod / 4)
				redraw.RequestRedraw()
				continue
			}
			// No XFlush from render thread — main loop owns XNextEvent; XInitThreads
			// covers any remaining Xlib cross-thread use from wgpu surface present.
			pn := presentN.Add(1)
			fpsWindow.add(time.Now())
			// Cap at targetFPS when Present returns early. During live resize skip
			// the budget sleep so the next frame presents ASAP (closes expose gap).
			if !sizeChanged {
				if d := framePeriod - time.Since(frameStart); d > time.Millisecond {
					time.Sleep(d)
				}
			}
			redraw.RequestRedraw()

			if pn%uint64(logEveryFrame) == 0 || time.Since(lastLog) > 2*time.Second {
				lastLog = time.Now()
				log.Printf("fps=%.1f frame=%d t=%.1fs present=%d mode=%s lost_skip=%d soft_skip=%d recoveries=%d size=%dx%d",
					fpsWindow.rate(time.Now()), n, elapsed, pn, sc.PresentModeName(), skipLostN.Load(), skipSoftN.Load(), sc.Recoveries(), w, h)
			}
		}
	}()

	log.Printf("device_lost_redraw ready — target=%dfps present=%s (vsync chain), free resize", targetFPS, sc.PresentModeName())
	redraw.RequestRedraw()

	for {
		host.mu.Lock()
		quit := host.quit
		host.mu.Unlock()
		if quit {
			break
		}

		drained := false
		for xw.Pending() {
			drained = true
			ev := xw.NextEvent()
			switch ev.Type {
			case xDestroyNotify:
				doStop()
			case xClientMessage:
				if xw.IsDeleteMessage(ev) {
					doStop()
				}
			case xMapNotify:
				host.setPresentableFlags(true, false, false)
				host.markForceFull()
				redraw.RequestRedraw()
			case xUnmapNotify:
				host.mu.Lock()
				obsc := host.obscured
				host.mu.Unlock()
				host.setPresentableFlags(false, true, obsc)
			case xConfigureNotify:
				nw, nh := ev.Width, ev.Height
				if nw < 1 {
					nw = 1
				}
				if nh < 1 {
					nh = 1
				}
				// Live size: render thread coalesces to 1 Configure + full present / frame.
				host.setLiveSize(nw, nh)
				redraw.RequestRedraw()
			case xVisibilityNotify:
				full := ev.Visibility == xVisibilityFullyObscured
				host.mu.Lock()
				host.obscured = full
				mapped, mini := host.mapped, host.minimized
				host.mu.Unlock()
				host.setPresentableFlags(mapped, mini, full)
				if !full {
					host.markForceFull()
					redraw.RequestRedraw()
				}
			case xFocusIn:
				host.mu.Lock()
				host.focused = true
				host.mu.Unlock()
				redraw.RequestRedraw()
			case xFocusOut:
				host.mu.Lock()
				host.focused = false
				host.mu.Unlock()
			case xExposure:
				host.markForceFull()
				redraw.RequestRedraw()
			case xKeyPress:
				if ev.KeyCode == 9 || ev.KeyCode == 24 { // Esc / q (best-effort)
					doStop()
				}
			}
		}
		if !drained {
			time.Sleep(2 * time.Millisecond)
		}
	}

	doStop()
	renderWG.Wait()
	log.Printf("done present=%d frames=%d lost_skip=%d soft_skip=%d recoveries=%d",
		presentN.Load(), frameN.Load(), skipLostN.Load(), skipSoftN.Load(), sc.Recoveries())
}
