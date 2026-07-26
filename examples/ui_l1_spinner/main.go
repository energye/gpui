// Command ui_l1_spinner is the L1 P3 gate: spinner on RepaintBoundary + JSON metrics.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	go run ./examples/ui_l1_spinner
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/ui/animation"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	if os.Getenv("DISPLAY") == "" {
		fmt.Fprintln(os.Stderr, "ui_l1_spinner: DISPLAY not set")
		os.Exit(2)
	}
	secs := 3
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			secs = n
		}
	}
	const winW, winH = 480, 320
	xw, err := openX11(winW, winH, "gpui L1 spinner (P3)")
	if err != nil {
		fmt.Fprintln(os.Stderr, "x11:", err)
		os.Exit(1)
	}
	defer xw.close()

	// Static chrome + spinner boundary.
	spin := rendering.NewRenderSpinner(48)
	root := rendering.NewRenderBox(spin)
	root.FixedWidth, root.FixedHeight = float64(winW), float64(winH)
	for i := 0; i < 20; i++ {
		c := rendering.NewRenderColorBox(12, 12, 0.25, 0.28, 0.32, 1)
		root.AddChild(c)
	}
	// Position spinner roughly center via offset after layout in first frame —
	// for demo, pad root and let spinner at (0,0); add a spacer box first.
	spin.SetOffset(rendering.Point{X: float64(winW)/2 - 24, Y: float64(winH)/2 - 24})

	host := &x11Host{xw: xw, w: winW, h: winH, scale: 1}
	app := embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR:    0.10,
		ClearG:    0.12,
		ClearB:    0.16,
		ClearA:    1,
		RunFor:    time.Duration(secs) * time.Second,
		MaxFrames: 300,
		WarmUp:    true,
	})

	ctrl := animation.NewController(1.0)
	ctrl.SetRepeat(true)
	ctrl.OnValue(func(v float64) {
		spin.SetPhase(v)
		app.ScheduleFrame()
	})
	ctrl.Start(app.Scheduler().Tickers())

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}
	ctrl.Stop()

	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "no frames")
		os.Exit(1)
	}
	// S2-ish: after warm-up, layout should not keep growing every frame.
	// layoutFrames includes initial + resizes only ideally.
	fmt.Fprintf(os.Stderr, "ui_l1_spinner: presents=%d layout_flushes=%d last_raster_layers=%d\n",
		app.PresentCount(), app.LayoutFlushCount(), app.LastRasterStats().RasterLayerCount)
	if b, err := app.Metrics().JSON(); err == nil {
		fmt.Println(string(b))
	}
	// Soft gate: layout flushes should be small vs presents (not every frame).
	if app.LayoutFlushCount() > app.PresentCount()/2 && app.PresentCount() > 10 {
		fmt.Fprintf(os.Stderr, "warning: layout flushes high (%d / %d presents)\n",
			app.LayoutFlushCount(), app.PresentCount())
	}
}

// --- minimal X11 (same pattern as ui_l1_blank) ---

type x11Win struct {
	display, window uintptr
	close, flush    func()
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
		return nil, errStr("XOpenDisplay failed")
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	win := xCreateSimple(dpy, root, 80, 80, uint(w), uint(h), 1, 0, 0x001a2030)
	if win == 0 {
		xCloseDisplay(dpy)
		return nil, errStr("XCreateSimpleWindow failed")
	}
	t := append([]byte(title), 0)
	xStoreName(dpy, win, &t[0])
	xMapWindow(dpy, win)
	xFlush(dpy)
	time.Sleep(50 * time.Millisecond)
	xw := &x11Win{display: dpy, window: win}
	xw.flush = func() { xFlush(dpy) }
	xw.close = func() {
		xDestroyWindow(dpy, win)
		xCloseDisplay(dpy)
	}
	return xw, nil
}

type strErr struct{ s string }

func errStr(s string) error     { return &strErr{s} }
func (e *strErr) Error() string { return e.s }

type x11Host struct {
	xw    *x11Win
	w, h  int
	scale float64
	wake  chan struct{}
}

func (h *x11Host) NativeSurface() platform.NativeSurface {
	return platform.NativeSurface{Kind: platform.PlatformX11, Display: h.xw.display, Window: h.xw.window}
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

var _ = unsafe.Pointer(nil)
