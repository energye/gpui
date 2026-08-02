//go:build linux && !nogpu

package render_test

// Post-resize full-recovery state machine (render.PresentTarget):
//
// A swapchain reconfigure (Resize) leaves every buffer's content undefined
// (Vulkan VK_IMAGE_LAYOUT_UNDEFINED; Skia requires a full repaint after
// surface recreate). Retained damage frames LoadOpLoad their surface, so any
// buffer not fully written since the reconfigure shows black/stale pixels.
// PresentTarget must therefore owe N full frames (N = 3, covers double/triple
// buffering) after every physical resize, and only spend the budget on
// successfully presented frames (timeout BeginFrames keep the budget).

import (
	"os"
	"testing"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/render"
)

type resizeX11 struct {
	display uintptr
	window  uintptr
	close   func()
}

func openResizeX11(t *testing.T, w, h int) *resizeX11 {
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
		xDefaultScreen func(dpy uintptr) int
		xRootWindow    func(dpy uintptr, screen int) uintptr
		xCreateSimple  func(dpy uintptr, parent uintptr, x, y int, width, height, borderWidth uint, border, background uint64) uintptr
		xMapWindow     func(dpy uintptr, win uintptr) int
		xFlush         func(dpy uintptr) int
		xDestroyWindow func(dpy uintptr, win uintptr) int
	)
	purego.RegisterLibFunc(&xOpenDisplay, lib, "XOpenDisplay")
	purego.RegisterLibFunc(&xDefaultScreen, lib, "XDefaultScreen")
	purego.RegisterLibFunc(&xRootWindow, lib, "XRootWindow")
	purego.RegisterLibFunc(&xCreateSimple, lib, "XCreateSimpleWindow")
	purego.RegisterLibFunc(&xMapWindow, lib, "XMapWindow")
	purego.RegisterLibFunc(&xFlush, lib, "XFlush")
	purego.RegisterLibFunc(&xDestroyWindow, lib, "XDestroyWindow")

	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		t.Skip("XOpenDisplay failed")
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	win := xCreateSimple(dpy, root, 40, 40, uint(w), uint(h), 1, 0, 0)
	if win == 0 {
		t.Skip("XCreateSimpleWindow failed")
	}
	xMapWindow(dpy, win)
	xFlush(dpy)
	var closed bool
	return &resizeX11{
		display: dpy, window: win,
		close: func() {
			if closed {
				return
			}
			closed = true
			xDestroyWindow(dpy, win)
			xFlush(dpy)
		},
	}
}

func newResizeTarget(t *testing.T, xw *resizeX11, w, h int) *render.PresentTarget {
	t.Helper()
	_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")
	pt, err := render.NewPresentTarget(render.PresentNativeSurface{
		Platform: render.PresentPlatformX11,
		Display:  xw.display,
		Window:   xw.window,
	}, w, h, 1.0)
	if err != nil {
		t.Skipf("NewPresentTarget: %v", err)
	}
	t.Cleanup(func() { _ = pt.Close() })
	return pt
}

func drawFill(dc *render.Context) {
	dc.SetRGBA(0.15, 0.35, 0.55, 1)
	dc.DrawRectangle(0, 0, 500, 500)
	_ = dc.Fill()
}

func TestPresentResize_FullRecoveryWritesEveryBuffer(t *testing.T) {
	if os.Getenv("GPUI_FORCE_NO_X11") == "1" {
		t.Skip("GPUI_FORCE_NO_X11=1")
	}
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY")
	}

	xw := openResizeX11(t, 320, 240)
	t.Cleanup(xw.close)
	pt := newResizeTarget(t, xw, 320, 240)

	// Baseline full present.
	if err := pt.PresentWith(drawFill); err != nil {
		t.Fatalf("baseline present: %v", err)
	}
	if pt.InFullRecovery() {
		t.Fatalf("fresh target must not be in full recovery")
	}

	// Physical resize arms the full budget.
	if err := pt.Resize(200, 150, 1.0); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if !pt.InFullRecovery() {
		t.Fatalf("Resize must arm full recovery")
	}

	// The next 3 presents must all be full writes (one per swapchain buffer),
	// even though the draw callback damages only a tiny region.
	for i := 0; i < 3; i++ {
		out, err := pt.PresentWithAuto(func(dc *render.Context) {
			dc.SetRGBA(0.2, 0.4, 0.6, 1)
			dc.DrawRectangle(5, 5, 10, 10)
			_ = dc.Fill()
		})
		if err != nil {
			t.Fatalf("recovery present %d: %v", i, err)
		}
		if out.Mode != render.PresentModeFull {
			t.Fatalf("recovery present %d: want full, got %v", i, out.Mode)
		}
	}

	if pt.InFullRecovery() {
		t.Fatalf("full budget must be spent after 3 full presents")
	}

	// Steady state restored: a drawn frame may be damage/idle, never forced full.
	out, err := pt.PresentWithAuto(func(dc *render.Context) {})
	if err != nil {
		t.Fatalf("steady present: %v", err)
	}
	if !out.Idle {
		t.Fatalf("steady empty present want idle, got %v", out.Mode)
	}

	// A second resize re-arms the budget from zero.
	if err := pt.Resize(320, 240, 1.0); err != nil {
		t.Fatalf("Resize back: %v", err)
	}
	if !pt.InFullRecovery() {
		t.Fatalf("second Resize must re-arm full recovery")
	}
	out2, err := pt.PresentWithAuto(func(dc *render.Context) {})
	if err != nil {
		t.Fatalf("recovery empty present: %v", err)
	}
	if out2.Mode != render.PresentModeFull {
		t.Fatalf("empty draw during recovery must still be full, got %v", out2.Mode)
	}
}
