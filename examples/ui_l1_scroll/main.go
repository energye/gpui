// Command ui_l1_scroll is the L1 P4 demo: VirtualList 1000 rows + auto-scroll.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	go run ./examples/ui_l1_scroll
//
// Duration: default 60s; override with RUN_SECONDS (e.g. RUN_SECONDS=180).
// Window is resizable; list + viewport follow client size via ConfigureNotify.
package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	if os.Getenv("DISPLAY") == "" {
		fmt.Fprintln(os.Stderr, "ui_l1_scroll: DISPLAY not set")
		os.Exit(2)
	}
	secs := runSeconds(60)
	fmt.Fprintf(os.Stderr, "ui_l1_scroll: running %ds (RUN_SECONDS to override, e.g. 180)\n", secs)
	fmt.Fprintln(os.Stderr, "ui_l1_scroll: window is resizable — list tracks client size")
	const winW, winH = 400, 480
	const itemExtent = 40.0
	const itemCount = 1000

	xw, err := openX11(winW, winH, "gpui L1 scroll (P4) — resize me")
	if err != nil {
		fmt.Fprintln(os.Stderr, "x11:", err)
		os.Exit(1)
	}
	defer xw.close()

	host := &x11Host{xw: xw, w: winW, h: winH, scale: 1}

	// Rows take tight list width (0 preferred → expand with window).
	list := rendering.NewVirtualList(itemCount, itemExtent, func(i int) rendering.RenderObject {
		r, g, b := 0.35, 0.38, 0.42
		if i%2 == 0 {
			r, g, b = 0.25, 0.55, 0.85
		}
		return rendering.NewRenderColorBox(0, itemExtent, r, g, b, 1)
	})
	list.CacheExtent = itemExtent * 2
	vp := rendering.NewRenderViewport(list)

	app := embedder.NewPipelineApp(host, vp, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventResize {
				fmt.Fprintf(os.Stderr, "ui_l1_scroll: resize %dx%d\n", ev.Width, ev.Height)
			}
		},
	})

	// Auto-scroll; max uses live window height so resize clamps correctly.
	var scrollY float64
	app.Scheduler().Tickers().Add(&scrollTicker{
		on: func(dt float64) {
			scrollY += 80 * dt // ~80 px/s
			_, h := host.Size()
			max := float64(itemCount)*itemExtent - float64(h)
			if max < 0 {
				max = 0
			}
			if scrollY > max {
				scrollY = 0
			}
			vp.SetScrollOffset(0, scrollY)
			app.ScheduleFrame()
		},
	})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}

	m := app.Metrics().Snapshot()
	fps := float64(app.PresentCount()) / float64(secs)
	fw, fh := host.Size()
	fmt.Fprintf(os.Stderr, "ui_l1_scroll: presents=%d ~%.1f fps over %ds layout_flushes=%d bind=%d scrollY=%.0f size=%dx%d\n",
		app.PresentCount(), fps, secs, app.LayoutFlushCount(), list.BindCount, vp.ScrollOffset().Y, fw, fh)
	fmt.Fprintf(os.Stderr, "ui_l1_scroll: avg=%.2fms max=%.2fms last=%.2fms hitches(>%.1fms)=%d\n",
		m.AvgFrameIntervalMs, m.MaxFrameIntervalMs, m.LastFrameIntervalMs, scheduler.HitchThresholdMs, m.HitchCount)
	if list.BindCount >= itemCount {
		fmt.Fprintln(os.Stderr, "FAIL: virtualization mounted all rows")
		os.Exit(1)
	}
	if b, err := app.Metrics().JSON(); err == nil {
		fmt.Println(string(b))
	}
}

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

type scrollTicker struct {
	on func(dt float64)
}

func (s *scrollTicker) Tick(dt float64) bool {
	if s.on != nil {
		s.on(dt)
	}
	return true
}

// --- X11 host with StructureNotify resize ---

// X11 event type constants (X.h).
const (
	xConfigureNotify = 22
	xExpose          = 12
	xClientMessage   = 33
	// StructureNotifyMask | ExposureMask
	xEventMask = (1 << 17) | (1 << 15)
)

// XConfigureEvent layout on linux/amd64 (and arm64): see Xlib XEvent union.
// type@0, serial@8, send_event@16, display@24, event@32, window@40,
// x@48, y@52, width@56, height@60.
const (
	xevTypeOff   = 0
	xevWidthOff  = 56
	xevHeightOff = 60
)

type x11Win struct {
	display, window uintptr
	lib             uintptr
	close, flush    func()
	pending         func() int
	nextEvent       func(ev *byte) int
	// WM_DELETE_WINDOW atom for close (optional).
	wmDelete uintptr
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
		xOpenDisplay    func(name *byte) uintptr
		xCloseDisplay   func(dpy uintptr) int
		xDefaultScreen  func(dpy uintptr) int
		xRootWindow     func(dpy uintptr, screen int) uintptr
		xCreateSimple   func(dpy uintptr, parent uintptr, x, y int, width, height, borderWidth uint, border, background uint64) uintptr
		xMapWindow      func(dpy uintptr, win uintptr) int
		xFlush          func(dpy uintptr) int
		xDestroyWindow  func(dpy uintptr, win uintptr) int
		xStoreName      func(dpy uintptr, win uintptr, name *byte) int
		xSelectInput    func(dpy uintptr, win uintptr, mask int64) int
		xPending        func(dpy uintptr) int
		xNextEvent      func(dpy uintptr, ev *byte) int
		xInternAtom     func(dpy uintptr, name *byte, onlyIfExists int) uintptr
		xSetWMProtocols func(dpy uintptr, win uintptr, protocols *uintptr, count int) int
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
	purego.RegisterLibFunc(&xPending, lib, "XPending")
	purego.RegisterLibFunc(&xNextEvent, lib, "XNextEvent")
	purego.RegisterLibFunc(&xInternAtom, lib, "XInternAtom")
	purego.RegisterLibFunc(&xSetWMProtocols, lib, "XSetWMProtocols")

	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		return nil, errStr("XOpenDisplay failed")
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	win := xCreateSimple(dpy, root, 100, 60, uint(w), uint(h), 1, 0, 0x00101820)
	if win == 0 {
		xCloseDisplay(dpy)
		return nil, errStr("XCreateSimpleWindow failed")
	}
	tb := append([]byte(title), 0)
	xStoreName(dpy, win, &tb[0])
	// Enable ConfigureNotify + Expose so resize/redraw reach the host.
	xSelectInput(dpy, win, xEventMask)

	// Graceful close via window manager.
	delName := append([]byte("WM_DELETE_WINDOW"), 0)
	wmDelete := xInternAtom(dpy, &delName[0], 0)
	if wmDelete != 0 {
		atom := wmDelete
		xSetWMProtocols(dpy, win, &atom, 1)
	}

	xMapWindow(dpy, win)
	xFlush(dpy)
	time.Sleep(50 * time.Millisecond)

	xw := &x11Win{display: dpy, window: win, lib: lib, wmDelete: wmDelete}
	xw.flush = func() { xFlush(dpy) }
	xw.pending = func() int { return xPending(dpy) }
	xw.nextEvent = func(ev *byte) int { return xNextEvent(dpy, ev) }
	xw.close = func() {
		xDestroyWindow(dpy, win)
		xCloseDisplay(dpy)
	}
	// Drain map/configure noise once.
	var buf [256]byte
	for xPending(dpy) > 0 {
		xNextEvent(dpy, &buf[0])
	}
	return xw, nil
}

type strErr struct{ s string }

func errStr(s string) error     { return &strErr{s} }
func (e *strErr) Error() string { return e.s }

type x11Host struct {
	xw    *x11Win
	mu    sync.Mutex
	w, h  int
	scale float64
	wake  chan struct{}
}

func (h *x11Host) NativeSurface() platform.NativeSurface {
	return platform.NativeSurface{Kind: platform.PlatformX11, Display: h.xw.display, Window: h.xw.window}
}

func (h *x11Host) Size() (int, int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.w, h.h
}

func (h *x11Host) ScaleFactor() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.scale <= 0 {
		return 1
	}
	return h.scale
}

func (h *x11Host) setSize(w, ht int) (changed bool) {
	if w < 1 {
		w = 1
	}
	if ht < 1 {
		ht = 1
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.w == w && h.h == ht {
		return false
	}
	h.w, h.h = w, ht
	return true
}

func (h *x11Host) WaitEvents(timeout time.Duration) []platform.Event {
	if h.wake == nil {
		h.wake = make(chan struct{}, 1)
	}
	// Always drain X11 first (resize may already be pending).
	if evs := h.drainX(); len(evs) > 0 {
		return evs
	}
	if timeout < 0 {
		timeout = 16 * time.Millisecond
	}
	if timeout == 0 {
		if h.xw.flush != nil {
			h.xw.flush()
		}
		return h.drainX()
	}
	select {
	case <-h.wake:
		if evs := h.drainX(); len(evs) > 0 {
			return evs
		}
		return []platform.Event{{Type: platform.EventWake}}
	case <-time.After(timeout):
		if h.xw.flush != nil {
			h.xw.flush()
		}
		return h.drainX()
	}
}

func (h *x11Host) drainX() []platform.Event {
	if h.xw == nil || h.xw.pending == nil || h.xw.nextEvent == nil {
		return nil
	}
	var out []platform.Event
	var buf [256]byte
	for h.xw.pending() > 0 {
		h.xw.nextEvent(&buf[0])
		// XEvent.type is int at offset 0 (LE).
		t := int(readI32(buf[:], xevTypeOff))
		switch t {
		case xConfigureNotify:
			nw := int(readI32(buf[:], xevWidthOff))
			nh := int(readI32(buf[:], xevHeightOff))
			if nw > 0 && nh > 0 && h.setSize(nw, nh) {
				out = append(out, platform.Event{
					Type:   platform.EventResize,
					Width:  nw,
					Height: nh,
					Scale:  h.ScaleFactor(),
				})
			}
		case xExpose:
			out = append(out, platform.Event{Type: platform.EventExpose})
		case xClientMessage:
			// XClientMessageEvent (64-bit): data.l[0] @56 holds WM_DELETE_WINDOW.
			data0 := readU64(buf[:], 56)
			if h.xw.wmDelete != 0 && uintptr(data0) == h.xw.wmDelete {
				out = append(out, platform.Event{Type: platform.EventClose})
			}
		}
	}
	return out
}

func readI32(b []byte, off int) int32 {
	if off+4 > len(b) {
		return 0
	}
	return int32(b[off]) | int32(b[off+1])<<8 | int32(b[off+2])<<16 | int32(b[off+3])<<24
}

func readU64(b []byte, off int) uint64 {
	if off+8 > len(b) {
		return 0
	}
	var v uint64
	for i := 0; i < 8; i++ {
		v |= uint64(b[off+i]) << (8 * i)
	}
	return v
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
