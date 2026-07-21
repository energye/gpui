//go:build linux && !nogpu

// Window present demo (S.03 / S6.8): X11 → Swapchain (Fifo vsync) → PresentFrameAuto.
// Suboptimal reconfigure + swapchain stats; no artificial sleep under Fifo (present blocks).
//
//	export DISPLAY=:1
//	export WGPU_NATIVE_PATH=/path/to/libwgpu_native.so
//	go run ./examples/window_present
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/examples/exboot"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

func main() {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		// Prefer in-repo native lib when unset.
		if _, err := os.Stat("lib/libwgpu_native.so"); err == nil {
			_ = os.Setenv("WGPU_NATIVE_PATH", "lib/libwgpu_native.so")
		}
	}

	winW, winH := 480, 320
	frames := 90 // ~1.5s at 60fps-ish sleep
	if v := os.Getenv("GPUI_PRESENT_FRAMES"); v != "" {
		fmt.Sscanf(v, "%d", &frames)
		if frames < 10 {
			frames = 10
		}
	}

	xw, err := openX11Window(winW, winH, "gpui window present (S.03)")
	if err != nil {
		log.Fatalf("X11: %v", err)
	}
	defer xw.Close()
	log.Printf("X11 window mapped display=%#x window=%#x", xw.Display, xw.Window)

	exboot.InitEnv()
	inst, err := exboot.NewInstanceX11(xw.Display, 0)
	if err != nil {
		log.Fatalf("CreateInstance: %v", err)
	}
	defer inst.Release()

	surf, err := inst.CreateSurface(xw.Display, xw.Window)
	if err != nil {
		log.Fatalf("CreateSurface: %v", err)
	}
	defer surf.Release()

	adapter, device, err := exboot.OpenDevice(inst, surf, "window-present")
	if err != nil {
		log.Fatalf("OpenDevice: %v", err)
	}
	defer adapter.Release()
	defer func() {
		if device != nil {
			device.Release()
		}
	}()

	sc := webgpu.NewSwapchain(surf, device, uint32(winW), uint32(winH))
	sc.Usage = types.TextureUsageRenderAttachment
	sc.SetPreferVSync() // S6.8: Fifo when available
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		log.Fatalf("Configure: %v", err)
	}
	defer sc.Release()
	sc.ResetStats()

	// CRITICAL: render must use the SAME device that owns the swapchain.
	// GPUShared otherwise creates a second device; MSAA resolve into a
	// foreign surface texture fails native validation.
	if err := exboot.BindProvider(device, adapter, sc.Format); err != nil {
		log.Fatalf("SetDeviceProvider: %v", err)
	}
	defer exboot.ResetAccelerator()

	dc := render.NewContext(winW, winH)
	defer dc.Close()
	exboot.WireAutoRecover(sc, adapter, "window-present",
		func(dev *webgpu.Device) { device = dev },
		func() { dc.DropGPURenderContext() },
		nil,
	)

	for i := 0; i < frames; i++ {
		dc.BeginFrame()
		// Animated clear + shapes
		tt := float64(i) / float64(frames)
		dc.SetRGB(0.08, 0.10, 0.14)
		dc.DrawRectangle(0, 0, float64(winW), float64(winH))
		_ = dc.Fill()
		dc.SetRGB(0.2+0.6*tt, 0.5, 1.0-0.4*tt)
		dc.DrawRoundedRectangle(40+20*tt, 40, 200, 120, 16)
		_ = dc.Fill()
		dc.SetRGB(1, 0.4, 0.2)
		dc.DrawCircle(320, 160, 48+12*tt)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawRectangle(20, 250, 440*(0.2+0.8*tt), 12)
		_ = dc.Fill()

		if device != nil {
			device.FlushCallbacks()
		}
		frame, err := sc.BeginFrame()
		if err != nil {
			if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
				log.Printf("BeginFrame: device lost/recovering at frame %d (recoveries=%d) — skip",
					i, sc.Recoveries())
				time.Sleep(16 * time.Millisecond)
				continue
			}
			log.Printf("BeginFrame: %v — skip", err)
			continue
		}
		if _, err := dc.PresentFrameAuto(frame.Handle, frame.Width, frame.Height, func() error {
			return sc.EndFrame(frame)
		}); err != nil {
			sc.DiscardFrame(frame)
			if errors.Is(err, webgpu.ErrDeviceLost) || (device != nil && device.IsLost()) {
				log.Printf("PresentFrameAuto: device lost/recovering at frame %d (recoveries=%d) — skip",
					i, sc.Recoveries())
				time.Sleep(16 * time.Millisecond)
				continue
			}
			log.Printf("PresentFrameAuto: %v — skip", err)
			continue
		}
		xw.Flush()
		// Optional recover proof (same as mem_anim / device_lost_redraw).
		if v := os.Getenv("GPUI_FORCE_LOST_AFTER"); v != "" {
			var n int
			fmt.Sscanf(v, "%d", &n)
			if n > 0 && i+1 == n && sc != nil {
				log.Printf("GPUI_FORCE_LOST_AFTER=%d ForceRecoverHealthy", n)
				if err := sc.ForceRecoverHealthy(); err != nil {
					log.Printf("ForceRecoverHealthy: %v", err)
				} else if sc.Device != nil {
					device = sc.Device
				}
			}
		}
		// Fifo present already waits for vsync — only pad Immediate/Mailbox.
		if sc.PresentMode == webgpu.PresentModeImmediate {
			time.Sleep(16 * time.Millisecond)
		}
	}

	stats := dc.RenderPathStats()
	sst := sc.Stats()
	fmt.Printf("done frames=%d mode=%s %s swapchain acquires=%d presents=%d reconfig=%d recoveries=%d lastPresentMs=%.2f\n",
		frames, sc.PresentModeName(), stats.LogLine(), sst.Acquires, sst.Presents, sst.Reconfigures, sc.Recoveries(), sst.LastPresentMs)
}

type x11Win struct {
	lib     uintptr
	Display uintptr
	Window  uintptr
	closeF  func()
	flushF  func()
}

func (w *x11Win) Close() {
	if w != nil && w.closeF != nil {
		w.closeF()
	}
}

func (w *x11Win) Flush() {
	if w != nil && w.flushF != nil {
		w.flushF()
	}
}

func openX11Window(w, h int, title string) (*x11Win, error) {
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		return nil, fmt.Errorf("dlopen libX11: %w", err)
	}

	var (
		xOpenDisplay   func(name *byte) uintptr
		xCloseDisplay  func(dpy uintptr) int
		xDefaultScreen func(dpy uintptr) int
		xRootWindow    func(dpy uintptr, screen int) uintptr
		xCreateSimple  func(dpy uintptr, parent uintptr, x, y int, width, height, borderWidth uint, border, background uint64) uintptr
		xMapWindow     func(dpy uintptr, win uintptr) int
		xFlush         func(dpy uintptr) int
		xDestroyWindow func(dpy uintptr, win uintptr) int
		xStoreName     func(dpy uintptr, win uintptr, name *byte) int
		xSelectInput   func(dpy uintptr, win uintptr, mask int64) int
	)
	purego.RegisterLibFunc(&xOpenDisplay, lib, "XOpenDisplay")
	purego.RegisterLibFunc(&xCloseDisplay, lib, "XCloseDisplay")
	purego.RegisterLibFunc(&xDefaultScreen, lib, "XDefaultScreen")
	purego.RegisterLibFunc(&xRootWindow, lib, "XRootWindow")
	purego.RegisterLibFunc(&xCreateSimple, lib, "XCreateSimpleWindow")
	purego.RegisterLibFunc(&xMapWindow, lib, "XMapWindow")
	purego.RegisterLibFunc(&xFlush, lib, "XFlush")
	purego.RegisterLibFunc(&xDestroyWindow, lib, "XDestroyWindow")
	purego.RegisterLibFunc(&xStoreName, lib, "XStoreName")
	purego.RegisterLibFunc(&xSelectInput, lib, "XSelectInput")

	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		_ = purego.Dlclose(lib)
		return nil, fmt.Errorf("XOpenDisplay failed (DISPLAY=%q)", os.Getenv("DISPLAY"))
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	// black background
	win := xCreateSimple(dpy, root, 80, 80, uint(w), uint(h), 1, 0, 0)
	if win == 0 {
		xCloseDisplay(dpy)
		_ = purego.Dlclose(lib)
		return nil, fmt.Errorf("XCreateSimpleWindow failed")
	}
	name := append([]byte(title), 0)
	xStoreName(dpy, win, &name[0])
	// StructureNotifyMask | ExposureMask
	const structureNotify = 1 << 17
	const exposureMask = 1 << 15
	xSelectInput(dpy, win, structureNotify|exposureMask)
	xMapWindow(dpy, win)
	xFlush(dpy)
	// Give the WM a moment to map
	time.Sleep(50 * time.Millisecond)

	return &x11Win{
		lib:     lib,
		Display: dpy,
		Window:  win,
		flushF:  func() { xFlush(dpy) },
		closeF: func() {
			xDestroyWindow(dpy, win)
			xCloseDisplay(dpy)
			_ = purego.Dlclose(lib)
		},
	}, nil
}

// keep unsafe import for purego pointer patterns if needed later
var _ = unsafe.Sizeof(0)
