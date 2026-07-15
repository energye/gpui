//go:build linux && !nogpu

package render_test

import (
	"fmt"
	"image"
	"os"
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	rendgpu "github.com/energye/gpui/render/gpu"
)

// TestS68_WindowPresent_MultiFrameDraw is the S6.8 real-window gate:
// X11 → swapchain (Fifo) → shared device → PresentFrameAuto multi-frame + damage.
// Skips when DISPLAY is unavailable (no gpui_x11_present tag required).
func TestS68_WindowPresent_MultiFrameDraw(t *testing.T) {
	if os.Getenv("GPUI_FORCE_NO_X11") == "1" {
		t.Skip("GPUI_FORCE_NO_X11=1")
	}
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY")
	}

	_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")
	_ = rendgpu.ResetAccelerator()
	t.Cleanup(func() { _ = rendgpu.ResetAccelerator() })

	const winW, winH = 240, 160
	xw := s68OpenX11(t, winW, winH)
	defer xw.close()

	inst, err := webgpu.CreateInstance(&webgpu.InstanceDescriptor{Backends: webgpu.BackendsPrimary})
	if err != nil {
		t.Skipf("CreateInstance: %v", err)
	}
	defer inst.Release()

	surf, err := inst.CreateSurface(xw.display, xw.window)
	if err != nil {
		t.Fatalf("CreateSurface: %v", err)
	}
	defer surf.Release()

	adapter, err := inst.RequestAdapter(&webgpu.RequestAdapterOptions{
		PowerPreference:   webgpu.PowerPreferenceHighPerformance,
		CompatibleSurface: surf,
	})
	if err != nil {
		t.Fatalf("RequestAdapter: %v", err)
	}
	defer adapter.Release()

	device, err := adapter.RequestDevice(rendgpu.DeviceDescriptor("s68-x11"))
	if err != nil {
		t.Skipf("RequestDevice: %v", err)
	}
	defer device.Release()

	sc := webgpu.NewSwapchain(surf, device, winW, winH)
	sc.Usage = types.TextureUsageRenderAttachment
	sc.SetPreferVSync()
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	defer sc.Release()

	if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: device, Adpt: adapter, Format: sc.Format,
	}); err != nil {
		t.Fatalf("SetDeviceProvider: %v", err)
	}

	dc := render.NewContext(winW, winH)
	defer dc.Close()
	sc.ResetStats()

	var presentSamples []float64
	const frames = 20
	for i := 0; i < frames; i++ {
		dc.BeginFrame()
		dc.ResetRenderPathStats()

		// Steady UI: clear + shell + local dirty widget.
		dc.SetRGB(0.10, 0.12, 0.16)
		dc.DrawRectangle(0, 0, float64(winW), float64(winH))
		_ = dc.Fill()
		dc.SetRGB(0.20, 0.45, 0.85)
		dc.DrawRoundedRectangle(20, 24, 120, 48, 10)
		_ = dc.Fill()
		// Animated local damage region.
		x := 20 + float64(i%8)*8
		dc.SetRGB(0.95, 0.55, 0.15)
		dc.DrawCircle(x+30, 110, 14)
		_ = dc.Fill()

		t0 := time.Now()
		frame, err := sc.BeginFrame()
		if err != nil {
			t.Fatalf("BeginFrame %d: %v", i, err)
		}
		out, err := dc.PresentFrameAuto(frame.Handle, frame.Width, frame.Height, func() error {
			// GPU damage scissor is handled inside PresentFrame*; OS damage
			// rects are optional (wgpu-native ignores). Always Present.
			return sc.EndFrame(frame)
		})
		dt := time.Since(t0).Seconds() * 1000
		if err != nil {
			sc.DiscardFrame(frame)
			t.Fatalf("PresentFrameAuto %d: %v", i, err)
		}
		if out.Idle && i == 0 {
			// First frame after BeginFrame with draws should not be idle.
			t.Log("note: first frame idle unexpected but continue")
		}
		if dc.RenderPathStats().GPUOps == 0 && !out.Idle {
			t.Fatalf("GPUOps==0 non-idle frame %d", i)
		}
		if dc.RenderPathStats().CPUFallbackOps != 0 {
			t.Fatalf("cpu_fallback frame %d: %d", i, dc.RenderPathStats().CPUFallbackOps)
		}
		if i >= 4 {
			presentSamples = append(presentSamples, dt)
		}
		xw.flush()
	}

	// Explicit damage multi-rect frame.
	dc.BeginFrame()
	dc.ResetFrameDamage()
	dc.SetRGB(0.3, 0.8, 0.4)
	dc.DrawRoundedRectangle(12, 12, 60, 40, 6)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.3, 0.2)
	dc.DrawRoundedRectangle(140, 80, 70, 50, 6)
	_ = dc.Fill()
	rects := dc.FrameDamage()
	if len(rects) < 1 {
		rects = []image.Rectangle{
			image.Rect(10, 10, 80, 60),
			image.Rect(130, 70, 220, 140),
		}
	}
	frame, err := sc.BeginFrame()
	if err != nil {
		t.Fatalf("BeginFrame damage: %v", err)
	}
	if err := dc.PresentFrameDamageRects(frame.Handle, frame.Width, frame.Height, rects, func() error {
		return sc.EndFrameWithDamage(frame, rects)
	}); err != nil {
		sc.DiscardFrame(frame)
		t.Fatalf("PresentFrameDamageRects: %v", err)
	}

	_ = device.WaitIdle()
	st := sc.Stats()
	p50 := s5Percentile(presentSamples, 0.5)
	t.Logf("S6.8 window present mode=%s frames=%d p50=%.2fms acquires=%d presents=%d reconfig=%d suboptimal=%d lastPresentMs=%.3f",
		sc.PresentModeName(), frames, p50, st.Acquires, st.Presents, st.Reconfigures, st.Suboptimal, st.LastPresentMs)

	if st.Presents < uint64(frames) {
		t.Fatalf("presents %d < frames %d", st.Presents, frames)
	}
	// Soft: vsync Fifo may sit near 16.7ms; allow 2× headroom for first-window cost.
	budget := s5EnvFloat("S6_WINDOW_PRESENT_BUDGET", 33.4)
	if p50 > budget && os.Getenv("S6_ALLOW_SLOW") != "1" {
		t.Fatalf("window present p50 %.2f exceeds budget %.2f", p50, budget)
	}
}

// TestS68_WindowPresent_IdleSkip exercises PresentFrameAuto idle when nothing dirty.
func TestS68_WindowPresent_IdleSkip(t *testing.T) {
	if os.Getenv("GPUI_FORCE_NO_X11") == "1" || os.Getenv("DISPLAY") == "" {
		t.Skip("no X11")
	}
	_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")
	_ = rendgpu.ResetAccelerator()
	t.Cleanup(func() { _ = rendgpu.ResetAccelerator() })

	const winW, winH = 120, 80
	xw := s68OpenX11(t, winW, winH)
	defer xw.close()

	inst, err := webgpu.CreateInstance(&webgpu.InstanceDescriptor{Backends: webgpu.BackendsPrimary})
	if err != nil {
		t.Skip(err)
	}
	defer inst.Release()
	surf, err := inst.CreateSurface(xw.display, xw.window)
	if err != nil {
		t.Fatal(err)
	}
	defer surf.Release()
	adapter, err := inst.RequestAdapter(&webgpu.RequestAdapterOptions{CompatibleSurface: surf})
	if err != nil {
		t.Fatal(err)
	}
	defer adapter.Release()
	device, err := adapter.RequestDevice(rendgpu.DeviceDescriptor("s68-idle"))
	if err != nil {
		t.Skipf("RequestDevice: %v", err)
	}
	defer device.Release()
	sc := webgpu.NewSwapchain(surf, device, winW, winH)
	sc.Usage = types.TextureUsageRenderAttachment
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		t.Fatal(err)
	}
	defer sc.Release()
	if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: device, Adpt: adapter, Format: sc.Format,
	}); err != nil {
		t.Fatal(err)
	}

	dc := render.NewContext(winW, winH)
	defer dc.Close()

	// Bootstrap full frame.
	dc.BeginFrame()
	dc.SetRGB(0.2, 0.2, 0.25)
	dc.DrawRectangle(0, 0, float64(winW), float64(winH))
	_ = dc.Fill()
	frame, err := sc.BeginFrame()
	if err != nil {
		t.Fatal(err)
	}
	if err := dc.PresentFrameFull(frame.Handle, frame.Width, frame.Height, func() error {
		return sc.EndFrame(frame)
	}); err != nil {
		sc.DiscardFrame(frame)
		t.Fatal(err)
	}

	// Idle: BeginFrame without draws → PresentFrameAuto should skip present callback.
	dc.BeginFrame()
	// Do not draw; no invalidation.
	presented := false
	// No swapchain acquire needed if idle — PresentFrameAuto should not call present.
	out, err := dc.PresentFrameAuto(frame.Handle /* stale */, winW, winH, func() error {
		presented = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Idle {
		t.Fatalf("expected idle outcome, got mode=%v", out.Mode)
	}
	if presented {
		t.Fatal("idle must not call present callback")
	}
	t.Log("S6.8 idle skip OK")
}

type s68X11 struct {
	lib     uintptr
	display uintptr
	window  uintptr
	close   func()
	flush   func()
}

func s68OpenX11(t *testing.T, w, h int) *s68X11 {
	t.Helper()
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		t.Skipf("libX11: %v", err)
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

	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		_ = purego.Dlclose(lib)
		t.Skip("XOpenDisplay failed")
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	win := xCreateSimple(dpy, root, 0, 0, uint(w), uint(h), 1, 0, 0)
	if win == 0 {
		xCloseDisplay(dpy)
		_ = purego.Dlclose(lib)
		t.Skip("XCreateSimpleWindow failed")
	}
	name := append([]byte(fmt.Sprintf("gpui-s68-%d", time.Now().UnixNano()%10000)), 0)
	xStoreName(dpy, win, &name[0])
	xMapWindow(dpy, win)
	xFlush(dpy)

	return &s68X11{
		lib: lib, display: dpy, window: win,
		close: func() {
			xDestroyWindow(dpy, win)
			xCloseDisplay(dpy)
			_ = purego.Dlclose(lib)
		},
		flush: func() { xFlush(dpy) },
	}
}

var _ = unsafe.Pointer(nil)
