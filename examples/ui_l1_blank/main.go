// Command ui_l1_blank is the L1 P0 gate: X11 window + clear present via ui embedder.
//
//	export DISPLAY=:0
//	export LD_LIBRARY_PATH=$PWD/lib
//	export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	go run ./examples/ui_l1_blank
//
// Exits 0 after a few successful presents (or RUN_SECONDS).
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"

	// Register GPU accelerator for render.Context present path.
	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	if os.Getenv("DISPLAY") == "" {
		fmt.Fprintln(os.Stderr, "ui_l1_blank: DISPLAY not set")
		os.Exit(2)
	}
	secs := 3
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			secs = n
		}
	}
	const winW, winH = 480, 320
	xw, err := openX11(winW, winH, "gpui L1 blank (P0)")
	if err != nil {
		fmt.Fprintln(os.Stderr, "x11:", err)
		os.Exit(1)
	}
	defer xw.close()

	host := &x11Host{xw: xw, w: winW, h: winH, scale: 1}
	app := embedder.New(host, embedder.Options{
		ClearR:          0.10,
		ClearG:          0.45,
		ClearB:          0.75,
		ClearA:          1,
		ContinuousClear: true, // demo: keep refreshing while timed
		RunFor:          time.Duration(secs) * time.Second,
		MaxFrames:       180,
	})
	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "open present:", err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "no presents completed")
		os.Exit(1)
	}
	if b, err := app.Metrics().JSON(); err == nil {
		fmt.Println(string(b))
	}
	fmt.Fprintf(os.Stderr, "ui_l1_blank: presents=%d ok\n", app.PresentCount())
}

// --- minimal X11 host (example-local; not part of ui library) ---

type x11Win struct {
	lib     uintptr
	display uintptr
	window  uintptr
	close   func()
	flush   func()
}

func openX11(w, h int, title string) (*x11Win, error) {
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		return nil, err
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
		xPending       func(dpy uintptr) int
		xNextEvent     func(dpy uintptr, ev *byte) int
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
	purego.RegisterLibFunc(&xPending, lib, "XPending")
	purego.RegisterLibFunc(&xNextEvent, lib, "XNextEvent")

	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		return nil, fmtError("XOpenDisplay failed")
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	// background ~0x1a2a3a
	win := xCreateSimple(dpy, root, 64, 64, uint(w), uint(h), 1, 0, 0x001a2a3a)
	if win == 0 {
		xCloseDisplay(dpy)
		return nil, fmtError("XCreateSimpleWindow failed")
	}
	t := append([]byte(title), 0)
	xStoreName(dpy, win, &t[0])
	xMapWindow(dpy, win)
	xFlush(dpy)
	// Allow map to complete.
	time.Sleep(50 * time.Millisecond)

	xw := &x11Win{lib: lib, display: dpy, window: win}
	xw.flush = func() { xFlush(dpy) }
	xw.close = func() {
		xDestroyWindow(dpy, win)
		xCloseDisplay(dpy)
	}
	// Drain pending events once.
	var buf [256]byte
	for xPending(dpy) > 0 {
		xNextEvent(dpy, &buf[0])
	}
	return xw, nil
}

func fmtError(s string) error { return &strErr{s} }

type strErr struct{ s string }

func (e *strErr) Error() string { return e.s }

type x11Host struct {
	xw    *x11Win
	w, h  int
	scale float64
	wake  chan struct{}
}

func (h *x11Host) NativeSurface() platform.NativeSurface {
	return platform.NativeSurface{
		Kind:    platform.PlatformX11,
		Display: h.xw.display,
		Window:  h.xw.window,
	}
}

func (h *x11Host) Size() (int, int) { return h.w, h.h }

func (h *x11Host) ScaleFactor() float64 {
	if h.scale <= 0 {
		return 1
	}
	return h.scale
}

func (h *x11Host) WaitEvents(timeout time.Duration) []platform.Event {
	if h.wake == nil {
		h.wake = make(chan struct{}, 1)
	}
	// Minimal: sleep timeout or wake; expose as empty or wake event.
	// Real X11 event pump can be added later; P0 only needs timed presents.
	if timeout < 0 {
		timeout = 16 * time.Millisecond
	}
	if timeout == 0 {
		if h.xw.flush != nil {
			h.xw.flush()
		}
		return nil
	}
	select {
	case <-h.wake:
		return []platform.Event{{Type: platform.EventWake}}
	case <-time.After(timeout):
		if h.xw.flush != nil {
			h.xw.flush()
		}
		return nil
	}
}

func (h *x11Host) WakeUp() {
	if h.wake == nil {
		h.wake = make(chan struct{}, 1)
	}
	select {
	case h.wake <- struct{}{}:
	default:
	}
}

// silence unused unsafe in case purego needs it on some arch
var _ = unsafe.Pointer(nil)
