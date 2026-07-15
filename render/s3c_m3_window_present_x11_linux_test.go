//go:build linux && !nogpu && gpui_x11_present

package render_test

import (
	"image"
	"os"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	rendgpu "github.com/energye/gpui/render/gpu"
)

// TestS3c_M3_WindowPresentFrame_X11Draw is the real-display S.03 gate:
// X11 window → swapchain → shared device → PresentFrame with scene draws.
// Headless / no DISPLAY: skip. Sandbox without X11 auth: skip.
func TestS3c_M3_WindowPresentFrame_X11Draw(t *testing.T) {
	if os.Getenv("GPUI_FORCE_NO_X11") == "1" {
		t.Skip("GPUI_FORCE_NO_X11=1")
	}
	// Free prior package GPU memory; use 1x samples on surface path for
	// software-backend headroom in long suites (example still uses 4x).
	_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")
	_ = rendgpu.ResetAccelerator()

	// Do NOT call s3cRequireGPU first: inject the window device first.

	xw := tryOpenX11Window(t, 96, 64)
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

	device, err := adapter.RequestDevice(rendgpu.DeviceDescriptor("s3c-x11-draw"))
	if err != nil {
		t.Fatalf("RequestDevice: %v", err)
	}
	defer device.Release()

	sc := webgpu.NewSwapchain(surf, device, 96, 64)
	sc.Usage = types.TextureUsageRenderAttachment
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	defer sc.Release()

	if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: device, Adpt: adapter, Format: sc.Format,
	}); err != nil {
		t.Fatalf("SetDeviceProvider: %v", err)
	}
	t.Cleanup(func() {
		// External device is released when this test ends; re-seed accelerator
		// so subsequent package tests do not use a freed device.
		_ = rendgpu.ResetAccelerator()
	})

	dc := render.NewContext(96, 64)
	defer dc.Close()

	// Multi-frame present with real draws (catches dual-device MSAA bugs).
	const frames = 3
	for i := 0; i < frames; i++ {
		dc.ResetRenderPathStats()
		dc.ClearWithColor(render.RGBA{R: 0.1, G: 0.1, B: 0.15, A: 1})
		dc.SetRGB(0.2+0.2*float64(i), 0.6, 0.9)
		dc.DrawRoundedRectangle(8, 8, 48, 32, 6)
		_ = dc.Fill()
		dc.SetRGB(1, 0.4, 0.2)
		dc.DrawCircle(70, 32, 14)
		_ = dc.Fill()

		frame, err := sc.BeginFrame()
		if err != nil {
			t.Fatalf("BeginFrame[%d]: %v", i, err)
		}
		if err := dc.PresentFrame(frame.Handle, frame.Width, frame.Height, func() error {
			return sc.EndFrame(frame)
		}); err != nil {
			sc.DiscardFrame(frame)
			t.Fatalf("PresentFrame[%d]: %v", i, err)
		}
		stats := dc.RenderPathStats()
		t.Logf("frame %d %s", i, stats.LogLine())
		if stats.GPUOps == 0 {
			t.Fatalf("frame %d needs GPUOps>0: %s", i, stats.LogLine())
		}
	}

	// Multi-rect damage present on same device/session (avoids dual-device VRAM OOM).
	dc.ResetRenderPathStats()
	dc.ResetFrameDamage()
	dc.SetRGB(0.2, 0.8, 0.35)
	dc.DrawRoundedRectangle(10, 10, 40, 24, 4)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.75, 0.1)
	dc.DrawRoundedRectangle(50, 34, 36, 22, 4)
	_ = dc.Fill()
	rects := dc.FrameDamage()
	if len(rects) < 2 {
		rects = []image.Rectangle{
			{Min: image.Pt(8, 8), Max: image.Pt(52, 36)},
			{Min: image.Pt(48, 32), Max: image.Pt(90, 58)},
		}
	}
	t.Logf("damage rects=%v", rects)
	frame, err := sc.BeginFrame()
	if err != nil {
		t.Fatalf("BeginFrame damage: %v", err)
	}
	if err := dc.PresentFrameDamageRects(frame.Handle, frame.Width, frame.Height, rects, func() error {
		return sc.EndFrame(frame)
	}); err != nil {
		sc.DiscardFrame(frame)
		t.Fatalf("PresentFrameDamageRects: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("damage frame %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("damage frame needs GPUOps>0: %s", stats.LogLine())
	}
	t.Log("X11 PresentFrame multi-frame + multi-rect damage OK")
}

type x11Win struct {
	lib     uintptr
	display uintptr
	window  uintptr
	close   func()
}

func tryOpenX11Window(t *testing.T, w, h int) *x11Win {
	t.Helper()
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		t.Skipf("libX11 not available: %v", err)
	}

	var (
		xOpenDisplay   func(name *byte) uintptr
		xCloseDisplay  func(dpy uintptr) int
		xDefaultScreen func(dpy uintptr) int
		xRootWindow    func(dpy uintptr, screen int) uintptr
		xCreateSimple  func(dpy uintptr, parent uintptr, x, y int, width, height, borderWidth uint, border, background uint64) uintptr
		xMapWindow     func(dpy uintptr, w uintptr) int
		xFlush         func(dpy uintptr) int
		xDestroyWindow func(dpy uintptr, w uintptr) int
		xStoreName     func(dpy uintptr, w uintptr, name *byte) int
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
		t.Skip("XOpenDisplay failed (no usable DISPLAY)")
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	win := xCreateSimple(dpy, root, 0, 0, uint(w), uint(h), 1, 0, 0)
	if win == 0 {
		xCloseDisplay(dpy)
		_ = purego.Dlclose(lib)
		t.Skip("XCreateSimpleWindow failed")
	}
	name := append([]byte("gpui-s3c-present-draw"), 0)
	xStoreName(dpy, win, &name[0])
	xMapWindow(dpy, win)
	xFlush(dpy)

	return &x11Win{
		lib:     lib,
		display: dpy,
		window:  win,
		close: func() {
			xDestroyWindow(dpy, win)
			xCloseDisplay(dpy)
			_ = purego.Dlclose(lib)
		},
	}
}

var _ = unsafe.Pointer(nil)
