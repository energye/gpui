//go:build linux && !nogpu

package render

import (
	"os"
	"runtime"
	"testing"

	"github.com/ebitengine/purego"
)

// P1-2 online smoke: GPUI_BACKEND=go opens an X11 window through the real
// online path (NewPresentTarget -> GL swapchain -> PresentClear) and
// presents two frames. Proves the thin interface wiring end to end;
// pixel-exact pelican validation stays in the offscreen feature gate.
func TestP1GLPresentX11OnlineSmoke(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY")
	}
	t.Setenv("GPUI_BACKEND", "go")

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
	win := xCreateSimple(dpy, root, 40, 40, 64, 64, 1, 0, 0)
	if win == 0 {
		t.Skip("XCreateSimpleWindow failed")
	}
	xMapWindow(dpy, win)
	xFlush(dpy)
	defer func() {
		xDestroyWindow(dpy, win)
		xFlush(dpy)
	}()

	pt, err := NewPresentTarget(PresentNativeSurface{
		Platform: PresentPlatformX11,
		Display:  dpy,
		Window:   win,
	}, 64, 64, 1)
	if err != nil {
		t.Fatalf("NewPresentTarget(GL): %v", err)
	}
	defer pt.Close()

	if pt.GPUBackend() == "unknown" {
		t.Fatalf("GPUBackend = unknown, want discrete/integrated/software")
	}
	for i := 0; i < 2; i++ {
		if err := pt.PresentWith(func(dc *Context) {
			dc.SetRGBA(1, 0, 0, 1)
			dc.DrawRectangle(0, 0, 64, 64)
			_ = dc.Fill()
		}); err != nil {
			t.Fatalf("PresentWith(frame %d): %v", i, err)
		}
	}
	t.Logf("P1-2 GL online smoke: 2 frames presented via thin swapchain (backend=%s)", pt.GPUBackend())
}
