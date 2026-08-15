//go:build linux && !nogpu

package embedder_test

// P14 regression (render.PresentTarget): an Idle PresentWithAuto must release
// the acquired swapchain frame. Before the fix, present() left an unpaired
// BeginFrame ("frame already in flight") after an idle present — every later
// frame then failed, producing a permanent black window after resize storms /
// min-max drag cycles. Lives here (external, GPU-gated); sibling unit test for
// the post-resize full-recovery state machine: render/present_resize_test.go.

import (
	"os"
	"testing"
	"time"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/render"
	rendgpu "github.com/energye/gpui/render/gpu"
)

type idleX11 struct {
	display uintptr
	window  uintptr
	close   func()
}

func openIdleX11(t *testing.T, w, h int) *idleX11 {
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
	)
	purego.RegisterLibFunc(&xOpenDisplay, lib, "XOpenDisplay")
	purego.RegisterLibFunc(&xCloseDisplay, lib, "XCloseDisplay")
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
		xCloseDisplay(dpy)
		t.Skip("XCreateSimpleWindow failed")
	}
	xMapWindow(dpy, win)
	xFlush(dpy)
	var closed bool
	return &idleX11{
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

func TestP14_PresentWithAuto_IdleDoesNotPoisonSwapchain(t *testing.T) {
	if os.Getenv("GPUI_FORCE_NO_X11") == "1" {
		t.Skip("GPUI_FORCE_NO_X11=1")
	}
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY")
	}

	xw := openIdleX11(t, 320, 240)
	t.Cleanup(xw.close)

	render.SetMSAASampleCount(render.MSAASampleCount1)
	defer render.SetMSAASampleCount(0)
	_ = rendgpu.ResetAccelerator()

	pt, err := render.NewPresentTarget(render.PresentNativeSurface{
		Platform: render.PresentPlatformX11,
		Display:  xw.display,
		Window:   xw.window,
	}, 320, 240, 1.0)
	if err != nil {
		t.Skipf("NewPresentTarget: %v", err)
	}
	t.Cleanup(func() { _ = pt.Close() })

	// Full present: draws a frame and presents normally.
	if err := pt.PresentWith(func(dc *render.Context) {
		dc.SetRGBA(0.1, 0.2, 0.3, 1)
		dc.DrawRectangle(0, 0, 320, 240)
		_ = dc.Fill()
	}); err != nil {
		t.Fatalf("full present: %v", err)
	}

	// Idle present: nothing drawn, nothing dirty → PresentFrameAuto Idle.
	out, err := pt.PresentWithAuto(func(dc *render.Context) {})
	if err != nil {
		t.Fatalf("idle present: %v", err)
	}
	if !out.Idle {
		t.Fatalf("empty draw want idle outcome, got %v", out.Mode)
	}

	// A following present must still acquire + present cleanly (would fail
	// with "frame already in flight" before the idle-discard fix).
	out2, err := pt.PresentWithAuto(func(dc *render.Context) {
		dc.SetRGBA(0.2, 0.4, 0.6, 1)
		dc.DrawRectangle(10, 10, 60, 40)
		_ = dc.Fill()
	})
	if err != nil {
		t.Fatalf("present after idle: %v (swapchain frame leaked)", err)
	}
	if out2.Mode == render.PresentModeIdle {
		t.Fatalf("drawn frame still idle")
	}

	// And a second full present stays healthy too.
	if err := pt.PresentWith(func(dc *render.Context) {
		dc.SetRGBA(0.1, 0.2, 0.3, 1)
		dc.DrawRectangle(0, 0, 320, 240)
		_ = dc.Fill()
	}); err != nil {
		t.Fatalf("full present after idle: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
}
