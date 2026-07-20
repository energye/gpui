//go:build linux && !nogpu

// device_lost_redraw — RequestRedraw-driven 60fps + dedicated render goroutine.
//
// Focus: Skia/Flutter device-lost / surface lifecycle under occlusion, without
// blocking the X11 event loop on GPU work.
//
//	export LD_LIBRARY_PATH=$PWD/lib
//	export DISPLAY=:1
//	go run ./examples/device_lost_redraw
//
// See README.md.
package main

import (
	"errors"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strconv"
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
	defaultW   = 960
	defaultH   = 640
	defaultFPS = 60
)

// redrawHost coalesces frame requests (cap=1). Safe from any goroutine.
type redrawHost struct {
	ch chan struct{}
}

func newRedrawHost() *redrawHost {
	return &redrawHost{ch: make(chan struct{}, 1)}
}

// RequestRedraw implements the gpu/context.WindowProvider pattern: non-blocking,
// at most one pending redraw.
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

	w, h int

	minimized bool
	obscured  bool // VisibilityFullyObscured
	focused   bool
	mapped    bool

	forceFull bool
	quit      bool

	// After long hide, force abandon on resume.
	hiddenSince time.Time
	wasHidden   bool
}

func (s *hostState) snapshot() (w, h int, presentable, forceFull, quit bool, hiddenFor time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, h = s.w, s.h
	quit = s.quit
	forceFull = s.forceFull
	forceRender := envBool("GPUI_FORCE_RENDER_WHEN_HIDDEN", false)
	hidden := (!s.mapped || s.minimized || s.obscured) && !forceRender
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

func (s *hostState) setPresentableFlags(mapped, minimized, obscured bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	was := (!s.mapped || s.minimized || s.obscured)
	s.mapped, s.minimized, s.obscured = mapped, minimized, obscured
	nowHidden := (!s.mapped || s.minimized || s.obscured)
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

func main() {
	runtime.LockOSThread() // X11 + purego callbacks more predictable on main

	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		if _, err := os.Stat("lib/libwgpu_native.so"); err == nil {
			_ = os.Setenv("WGPU_NATIVE_PATH", "lib/libwgpu_native.so")
		}
	}
	if os.Getenv("GPUI_SURFACE_SAMPLE_COUNT") == "" {
		_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")
	}

	targetFPS := envInt("GPUI_TARGET_FPS", defaultFPS)
	if targetFPS < 15 {
		targetFPS = 15
	}
	if targetFPS > 120 {
		targetFPS = 120
	}
	animSeconds := envInt("GPUI_ANIM_SECONDS", 0)
	logEvery := envInt("GPUI_LOG_EVERY", 60)
	if logEvery < 1 {
		logEvery = 60
	}
	framePeriod := time.Second / time.Duration(targetFPS)

	xw, err := openX11Window(defaultW, defaultH, "gpui device_lost_redraw (RequestRedraw + render thread)")
	if err != nil {
		log.Fatalf("X11: %v", err)
	}
	defer xw.Close()

	// Drain early configure.
	winW, winH := defaultW, defaultH
	for i := 0; i < 24; i++ {
		for xw.Pending() {
			ev := xw.NextEvent()
			if ev.Type == xConfigureNotify && ev.Width >= 64 && ev.Height >= 64 {
				winW, winH = ev.Width, ev.Height
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

	sc.EnableAutoRecover(adapter, "device-lost-redraw", func(dev *webgpu.Device) {
		setDevice(dev)
		if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
			Dev: dev, Adpt: adapter, Format: sc.Format,
		}); err != nil {
			log.Printf("SetDeviceProvider after recover: %v", err)
		}
		log.Printf("GPU device recovered (recoveries=%d) — continue RequestRedraw loop", sc.Recoveries())
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
			redraw.RequestRedraw() // wake render if waiting
		})
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Printf("signal — shutting down")
		doStop()
	}()

	var (
		frameN      atomic.Uint64
		presentN    atomic.Uint64
		skipLostN   atomic.Uint64
		skipSoftN   atomic.Uint64
		visibleTime atomic.Int64 // nanoseconds accumulated while presentable
	)

	// --- Animation clock: RequestRedraw at target FPS while presentable ---
	go func() {
		ticker := time.NewTicker(framePeriod)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				_, _, _, _, quit, _ := host.snapshot()
				if quit {
					return
				}
				// Always wake render: presents when visible; FlushCallbacks when not.
				redraw.RequestRedraw()
			}
		}
	}()

	// --- Render goroutine: only GPU consumer ---
	var renderWG sync.WaitGroup
	renderWG.Add(1)
	go func() {
		defer renderWG.Done()
		runtime.LockOSThread()
		start := time.Now()
		var lastLog time.Time
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

			// Unpresentable: pump callbacks only — never GCT (native may SIGABRT).
			if !presentable {
				time.Sleep(8 * time.Millisecond)
				continue
			}

			// Resume after long cover: abandon device (probe false-negatives).
			if hiddenFor >= 2*time.Second && dev != nil && !dev.IsLost() {
				log.Printf("resume after %.1fs unpresentable — MarkLost (force recreate)", hiddenFor.Seconds())
				dev.MarkLost()
				sc.ClearRecoverCooldown()
				host.mu.Lock()
				host.wasHidden = false
				host.hiddenSince = time.Time{}
				host.mu.Unlock()
			} else if hiddenFor > 0 {
				host.mu.Lock()
				host.wasHidden = false
				host.hiddenSince = time.Time{}
				sc.MarkNeedsReconfigure()
				sc.ClearRecoverCooldown()
				host.mu.Unlock()
			}

			if w != dc.Width() || h != dc.Height() {
				if err := sc.Resize(uint32(w), uint32(h)); err != nil {
					log.Printf("sc.Resize: %v", err)
				} else if err := dc.Resize(w, h); err != nil {
					log.Printf("dc.Resize: %v", err)
				} else {
					forceFull = true
					log.Printf("resized → %dx%d", w, h)
				}
			}

			t := time.Since(start).Seconds()
			// Subtract is approximate; animSeconds uses visibleTime counter instead.
			n := frameN.Add(1)
			dc.BeginFrame()
			if forceFull {
				dc.MarkFullRedraw()
			}
			drawCompositeFrame(dc, w, h, t, int(n))
			if forceFull {
				host.clearForceFull()
			}

			fb, err := sc.BeginFrame()
			if err != nil {
				if errors.Is(err, webgpu.ErrDeviceLost) || (dev != nil && dev.IsLost()) {
					skipLostN.Add(1)
					log.Printf("BeginFrame: device lost/recovering (recoveries=%d) — skip", sc.Recoveries())
					sc.ClearRecoverCooldown()
					if sc.Device != nil {
						setDevice(sc.Device)
					}
					host.markForceFull()
					time.Sleep(16 * time.Millisecond)
					continue
				}
				if errors.Is(err, webgpu.ErrSurfaceOccluded) || errors.Is(err, webgpu.ErrTimeout) ||
					errors.Is(err, webgpu.ErrSurfaceOutdated) || errors.Is(err, webgpu.ErrSurfaceLost) {
					skipSoftN.Add(1)
					host.markForceFull()
					sc.MarkNeedsReconfigure()
					time.Sleep(4 * time.Millisecond)
					continue
				}
				skipSoftN.Add(1)
				log.Printf("BeginFrame: %v — skip", err)
				time.Sleep(4 * time.Millisecond)
				continue
			}

			if _, err := dc.PresentFrameAuto(fb.Handle, fb.Width, fb.Height, func() error {
				return sc.EndFrame(fb)
			}); err != nil {
				sc.DiscardFrame(fb)
				if errors.Is(err, webgpu.ErrDeviceLost) || (getDevice() != nil && getDevice().IsLost()) {
					skipLostN.Add(1)
					log.Printf("Present: device lost/recovering (recoveries=%d) — skip", sc.Recoveries())
					host.markForceFull()
					continue
				}
				skipSoftN.Add(1)
				log.Printf("Present: %v — skip", err)
				continue
			}
			xw.Flush()
			pn := presentN.Add(1)
			visibleTime.Add(int64(framePeriod))

			if logEvery > 0 && (pn%uint64(logEvery) == 0 || time.Since(lastLog) > 2*time.Second) {
				lastLog = time.Now()
				log.Printf("frame=%d present=%d lost_skip=%d soft_skip=%d recoveries=%d size=%dx%d focused_tick presentable=1",
					n, pn, skipLostN.Load(), skipSoftN.Load(), sc.Recoveries(), w, h)
			}

			if animSeconds > 0 && time.Duration(visibleTime.Load()) >= time.Duration(animSeconds)*time.Second {
				log.Printf("GPUI_ANIM_SECONDS=%d visible time reached — exit", animSeconds)
				doStop()
				return
			}
		}
	}()

	log.Printf("device_lost_redraw: RequestRedraw @ %dfps render=goroutine present=%s size=%dx%d",
		targetFPS, sc.PresentModeName(), winW, winH)
	log.Printf("cover window fully to soak device-lost path; unfocused-but-visible keeps drawing")
	if animSeconds > 0 {
		log.Printf("auto-exit after %ds visible time", animSeconds)
	}

	// --- Main: X11 event loop (no GPU) ---
	redraw.RequestRedraw() // first frame
	for {
		host.mu.Lock()
		quit := host.quit
		host.mu.Unlock()
		if quit {
			break
		}

		// Non-blocking drain; sleep briefly if idle so ticker/redraw can run.
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
				if nw >= 64 && nh >= 64 {
					host.mu.Lock()
					if nw != host.w || nh != host.h {
						host.w, host.h = nw, nh
						host.forceFull = true
						host.mu.Unlock()
						redraw.RequestRedraw()
					} else {
						host.mu.Unlock()
					}
				}
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
				// Unfocused-but-visible must keep drawing; focus alone is not pause.
				redraw.RequestRedraw()
			case xFocusOut:
				host.mu.Lock()
				host.focused = false
				host.mu.Unlock()
			case xExposure:
				host.markForceFull()
				redraw.RequestRedraw()
			case xKeyPress:
				// Best-effort: Esc ~9 / q varies by keyboard map — also Ctrl+C.
				if ev.KeyCode == 9 || ev.KeyCode == 24 {
					doStop()
				}
			}
		}
		if !drained {
			// Allow ticker / render to progress without spinning.
			time.Sleep(2 * time.Millisecond)
		}
	}

	doStop()
	renderWG.Wait()
	log.Printf("done present=%d frames=%d lost_skip=%d soft_skip=%d recoveries=%d",
		presentN.Load(), frameN.Load(), skipLostN.Load(), skipSoftN.Load(), sc.Recoveries())
}

func envInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	case "0", "false", "FALSE", "no", "NO", "off", "OFF":
		return false
	default:
		return def
	}
}
