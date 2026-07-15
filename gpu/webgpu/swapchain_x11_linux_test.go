//go:build linux && !(js && wasm) && !nogpu

package webgpu_test

import (
	"os"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// Minimal X11 window for Swapchain Present e2e when a real display is available.
type x11Win struct {
	lib     uintptr
	display uintptr
	window  uintptr
	close   func()
}

func tryOpenX11Window(t *testing.T, w, h int) *x11Win {
	t.Helper()
	if os.Getenv("GPUI_FORCE_NO_X11") == "1" {
		t.Skip("GPUI_FORCE_NO_X11=1")
	}
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
	name := append([]byte("gpui-swapchain-test"), 0)
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

func TestSwapchain_WindowPresentE2E(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Log("WGPU_NATIVE_PATH unset; relying on default discovery")
	}
	xw := tryOpenX11Window(t, 128, 96)
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

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Fatalf("RequestDevice: %v", err)
	}
	defer device.Release()

	sc := webgpu.NewSwapchain(surf, device, 128, 96)
	sc.Usage = types.TextureUsageRenderAttachment
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		t.Fatalf("ConfigureFromCapabilities: %v", err)
	}
	defer sc.Release()

	frame, err := sc.BeginFrame()
	if err != nil {
		t.Fatalf("BeginFrame: %v", err)
	}
	if frame.View == nil || frame.Handle.IsNil() {
		t.Fatal("frame view/handle nil")
	}
	if frame.Width != 128 || frame.Height != 96 {
		t.Fatalf("frame extent %dx%d", frame.Width, frame.Height)
	}
	// Present without rendering still exercises swapchain Present ABI.
	if err := sc.EndFrame(frame); err != nil {
		t.Fatalf("EndFrame/Present: %v", err)
	}
	t.Log("window swapchain Configure/BeginFrame/Present OK")
}

// silence unused unsafe if RegisterLibFunc signatures change
var _ = unsafe.Pointer(nil)

func TestS68_Swapchain_X11_MultiFramePresent(t *testing.T) {
	if os.Getenv("GPUI_FORCE_NO_X11") == "1" {
		t.Skip("GPUI_FORCE_NO_X11")
	}
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY")
	}
	xw := tryOpenX11Window(t, 160, 120)
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

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Fatalf("RequestDevice: %v", err)
	}
	defer device.Release()

	sc := webgpu.NewSwapchain(surf, device, 160, 120)
	sc.Usage = types.TextureUsageRenderAttachment
	sc.SetPreferVSync()
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	defer sc.Release()
	sc.ResetStats()

	const n = 12
	for i := 0; i < n; i++ {
		frame, err := sc.BeginFrame()
		if err != nil {
			t.Fatalf("BeginFrame %d: %v", i, err)
		}
		if err := sc.EndFrameWithDamage(frame, nil); err != nil {
			t.Fatalf("EndFrame %d: %v", i, err)
		}
		if i == 5 {
			if err := sc.Resize(160, 120); err != nil {
				t.Fatalf("Resize: %v", err)
			}
		}
	}
	st := sc.Stats()
	t.Logf("S6.8 swapchain multi-frame mode=%s acquires=%d presents=%d reconfig=%d suboptimal=%d lastPresentMs=%.3f",
		sc.PresentModeName(), st.Acquires, st.Presents, st.Reconfigures, st.Suboptimal, st.LastPresentMs)
	if st.Acquires != n || st.Presents != n {
		t.Fatalf("stats acquires=%d presents=%d want %d", st.Acquires, st.Presents, n)
	}
	if st.Reconfigures < 1 {
		t.Fatalf("expected reconfigure counter, got %d", st.Reconfigures)
	}
}

func TestS68_Swapchain_X11_SuboptimalReconfigureFlag(t *testing.T) {
	if os.Getenv("GPUI_FORCE_NO_X11") == "1" || os.Getenv("DISPLAY") == "" {
		t.Skip("no X11 display")
	}
	xw := tryOpenX11Window(t, 96, 64)
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
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Skipf("RequestDevice: %v", err)
	}
	defer device.Release()
	sc := webgpu.NewSwapchain(surf, device, 96, 64)
	sc.Usage = types.TextureUsageRenderAttachment
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		t.Fatal(err)
	}
	defer sc.Release()
	sc.ResetStats()
	sc.MarkNeedsReconfigure()
	frame, err := sc.BeginFrame()
	if err != nil {
		t.Fatal(err)
	}
	if err := sc.EndFrame(frame); err != nil {
		t.Fatal(err)
	}
	st := sc.Stats()
	// MarkNeedsReconfigure + BeginFrame should call Configure at least once after reset.
	if st.Reconfigures < 1 {
		t.Fatalf("expected reconfigure after MarkNeedsReconfigure, got %d", st.Reconfigures)
	}
	t.Logf("suboptimal path reconfigures=%d acquires=%d", st.Reconfigures, st.Acquires)
}
