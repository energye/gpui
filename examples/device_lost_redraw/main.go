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
	"fmt"
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

	// GPU is created on the render OS thread (sole owner). Creating the surface
	// on main and recovering on the render thread left the adapter unable to
	// CreateTexture after Release (1x1 OOM) on this libwgpu_native/NVIDIA path.
	gpuReady := make(chan struct{})
	gpuErr := make(chan error, 1)
	var scStore atomic.Value // *webgpu.Swapchain

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
				scAny := scStore.Load()
				if scAny == nil {
					continue
				}
				sc := scAny.(*webgpu.Swapchain)
				if !presentable {
					redraw.RequestRedraw()
				} else {
					hw, hh, _, _, _, _ := host.snapshot()
					if uint32(hw) != sc.Width || uint32(hh) != sc.Height {
						redraw.RequestRedraw()
					}
				}
			}
		}
	}()

	// Render goroutine — sole GPU owner (create + present + recover on this OS thread)
	var renderWG sync.WaitGroup
	renderWG.Add(1)
	go func() {
		defer renderWG.Done()
		runtime.LockOSThread()
		inst, err := webgpu.CreateInstance(&webgpu.InstanceDescriptor{Backends: webgpu.BackendsPrimary})
		if err != nil {
			gpuErr <- err
			return
		}
		surf, err := inst.CreateSurface(xw.Display, xw.Window)
		if err != nil {
			inst.Release()
			gpuErr <- err
			return
		}
		adapter, err := inst.RequestAdapter(&webgpu.RequestAdapterOptions{
			PowerPreference:   webgpu.PowerPreferenceHighPerformance,
			CompatibleSurface: surf,
		})
		if err != nil {
			surf.Release()
			inst.Release()
			gpuErr <- err
			return
		}
		device, err := adapter.RequestDevice(rendgpu.DeviceDescriptor("device-lost-redraw"))
		if err != nil {
			adapter.Release()
			surf.Release()
			inst.Release()
			gpuErr <- err
			return
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
		sc := webgpu.NewSwapchain(surf, device, uint32(winW), uint32(winH))
		sc.Usage = types.TextureUsageRenderAttachment
		sc.SetPreferVSync()
		if err := sc.ConfigureFromCapabilities(adapter); err != nil {
			device.Release()
			adapter.Release()
			surf.Release()
			inst.Release()
			gpuErr <- err
			return
		}
		if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
			Dev: device, Adpt: adapter, Format: sc.Format,
		}); err != nil {
			sc.Release()
			device.Release()
			adapter.Release()
			surf.Release()
			inst.Release()
			gpuErr <- err
			return
		}
		var dc *render.Context
		sc.OnDeviceAbandon = func(_ *webgpu.Device) {
			rendgpu.AbandonDevice()
		}
		sc.EnableAutoRecover(adapter, "device-lost-redraw", func(dev *webgpu.Device) {
			setDevice(dev)
			if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
				Dev: dev, Adpt: adapter, Format: sc.Format,
			}); err != nil {
				log.Printf("SetDeviceProvider after recover: %v", err)
			}
			if dc != nil {
				dc.DropGPURenderContext()
			}
			log.Printf("GPU device recovered (recoveries→%d)", sc.Recoveries()+1)
		})
		dc = render.NewContext(winW, winH)
		defer func() {
			dc.Close()
			_ = rendgpu.ResetAccelerator()
			sc.Release()
			if d := getDevice(); d != nil {
				d.Release()
			}
			adapter.Release()
			if sc.Surface != nil {
				sc.Surface.Release()
			}
			inst.Release()
		}()
		// Publish sc for keepalive size checks.
		scStore.Store(sc)
		close(gpuReady)

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
				// Free swapchain images while device is healthy (GNOME Iconic / Unmap).
				// Prevents VRAM pin if device later goes sticky-lost under cover.
				if sc.Surface != nil && sc.Device != nil && !sc.Device.IsLost() {
					sc.Surface.Unconfigure()
					sc.MarkNeedsReconfigure()
				}
				if dev != nil {
					dev.FlushCallbacks()
				}
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
				if errors.Is(err, webgpu.ErrRecovered) {
					// Post-recover settle: never Present in the same tick as recreate.
					log.Printf("BeginFrame: device recovered — settle frame (skip Present)")
					host.markForceFull()
					redraw.RequestRedraw()
					continue
				}
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
			if os.Getenv("GPUI_SIMPLE_DRAW") == "1" {
				dc.SetRGB(0.1, 0.2, 0.3)
				dc.DrawRectangle(0, 0, float64(w), float64(h))
				_ = dc.Fill()
				dc.SetRGB(1, 0.5, 0)
				dc.DrawCircle(float64(w)/2, float64(h)/2, 40)
				_ = dc.Fill()
			} else {
				drawCompositeFrame(dc, w, h, elapsed, hud)
			}
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
			// Automated soak: force sticky lost once after N presents (UI recover proof).
			if v := os.Getenv("GPUI_FORCE_LOST_AFTER"); v != "" {
				var n uint64
				fmt.Sscanf(v, "%d", &n)
				if n > 0 && pn == n {
					if d := getDevice(); d != nil {
						log.Printf("GPUI_FORCE_LOST_AFTER=%d MarkLost (AutoRecover)", n)
						d.MarkLost()
					}
				}
			}
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

	select {
	case err := <-gpuErr:
		log.Fatalf("GPU init: %v", err)
	case <-gpuReady:
	}

	mode := "fifo"
	if scAny := scStore.Load(); scAny != nil {
		mode = scAny.(*webgpu.Swapchain).PresentModeName()
	}
	log.Printf("device_lost_redraw ready — target=%dfps present=%s (vsync chain), free resize", targetFPS, mode)
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
			case xPropertyNotify:
				// GNOME: Iconify via WM_STATE without Unmap — pause acquire (prevent TDR).
				if xw.IsWMStateProperty(ev) {
					iconic := xw.IsIconic()
					host.mu.Lock()
					mapped := host.mapped
					obsc := host.obscured
					host.mu.Unlock()
					host.setPresentableFlags(mapped, iconic, obsc)
					if !iconic {
						host.markForceFull()
						if scAny := scStore.Load(); scAny != nil {
							sc := scAny.(*webgpu.Swapchain)
							sc.MarkNeedsReconfigure()
							sc.ClearRecoverCooldown()
						}
						redraw.RequestRedraw()
					}
				}
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
	rec := uint64(0)
	if scAny := scStore.Load(); scAny != nil {
		rec = scAny.(*webgpu.Swapchain).Recoveries()
	}
	log.Printf("done present=%d frames=%d lost_skip=%d soft_skip=%d recoveries=%d",
		presentN.Load(), frameN.Load(), skipLostN.Load(), skipSoftN.Load(), rec)
}
