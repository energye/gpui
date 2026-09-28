//go:build linux

package platform

import (
	"os"
	"testing"

	"github.com/ebitengine/purego"
)

// 148.5MHz / (2200×1125) is the canonical 1080p60 timing.
func TestXrrModeRefreshHz(t *testing.T) {
	if got := xrrModeRefreshHz(148500, 2200, 1125); got < 59.99 || got > 60.01 {
		t.Fatalf("1080p60 timing=%v want 60Hz", got)
	}
	if got := xrrModeRefreshHz(297000, 2200, 1125); got < 119.99 || got > 120.01 {
		t.Fatalf("1080p120 timing=%v want 120Hz", got)
	}
	for _, tc := range [][3]uint64{{0, 2200, 1125}, {148500, 0, 1125}, {148500, 2200, 0}} {
		if got := xrrModeRefreshHz(tc[0], uint32(tc[1]), uint32(tc[2])); got != 0 {
			t.Fatalf("degenerate timing %v=%v want 0", tc, got)
		}
	}
}

// A null display must report unknown, never crash.
func TestX11DisplayRefreshHz_NullDisplay(t *testing.T) {
	if got := x11DisplayRefreshHz(0, 0); got != 0 {
		t.Fatalf("null display=%v want 0", got)
	}
}

// Live probe: reports 0 or a sane rate, never kills the process.
// The old offsets read outputs as CRTC ids and fed one (0x21) to
// XRRGetCrtcInfo, which kills via the Xlib error handler (recover
// cannot catch it).
func TestX11DisplayRefreshHz_Live(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display")
	}
	lib := xrandrLoad()
	if lib == nil || !xrandrOK || lib.getResources == nil {
		t.Skip("no RandR")
	}
	dpy, root, cleanup := x11TestDisplay(t)
	defer cleanup()
	if got := x11DisplayRefreshHz(dpy, root); got != 0 && (got < 20 || got > 240) {
		t.Fatalf("live display=%v want 0 or 20-240Hz", got)
	} else {
		t.Logf("live display refresh=%.2fHz", got)
	}
	if os.Getenv("GPUI_X11_REFRESH_STRICT") == "1" {
		// This box's Xwayland reports ~59.88Hz. Off by default.
		if got := x11DisplayRefreshHz(dpy, root); got < 59 || got > 61 {
			t.Fatalf("strict live display=%v want ~59.88Hz", got)
		}
	}
}

// x11TestDisplay opens the live display for tests (caller closes).
func x11TestDisplay(t *testing.T) (dpy, root uintptr, cleanup func()) {
	t.Helper()
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Skipf("libX11: %v", err)
	}
	var (
		xOpenDisplay func(name *byte) uintptr
		xDefScreen   func(dpy uintptr) int
		xRootWindow  func(dpy uintptr, screen int) uintptr
		xClose       func(dpy uintptr) int
	)
	purego.RegisterLibFunc(&xOpenDisplay, lib, "XOpenDisplay")
	purego.RegisterLibFunc(&xDefScreen, lib, "XDefaultScreen")
	purego.RegisterLibFunc(&xRootWindow, lib, "XRootWindow")
	purego.RegisterLibFunc(&xClose, lib, "XCloseDisplay")
	dpy = xOpenDisplay(nil)
	if dpy == 0 {
		t.Skip("XOpenDisplay failed")
	}
	root = xRootWindow(dpy, xDefScreen(dpy))
	return dpy, root, func() { xClose(dpy) }
}
