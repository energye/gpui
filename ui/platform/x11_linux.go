//go:build linux

package platform

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// x11Backend implements Backend for Xlib (Display* + Window). It is split
// into three concerns: Create/Adopt (window lifecycle), x11Host (event pump),
// and capability probing (IME nil for now; XIM lands with the IME milestone).
type x11Backend struct{}

func init() { Register(PlatformX11, &x11Backend{}) }

func (b *x11Backend) Kind() PlatformKind { return PlatformX11 }

// Create opens an X11 window and returns it wired to an event pump.
func (b *x11Backend) Create(opts Options) (*Window, error) {
	return x11Create(opts.Width, opts.Height, opts.Title)
}

// Adopt binds an existing X11 Display*/Window pair (embedding).
// Size is probed via XGetGeometry when available; falls back to defaults.
func (b *x11Backend) Adopt(ns NativeSurface) (*Window, error) {
	if ns.Display == 0 || ns.Window == 0 {
		return nil, fmt.Errorf("x11: adopt needs Display and Window handles")
	}
	lib, err := x11OpenLib()
	if err != nil {
		return nil, err
	}
	st := &x11State{
		display:   ns.Display,
		window:    ns.Window,
		w:         640,
		h:         480,
		scale:     1,
		keycodeToKeysym: x11KeycodeToKeysym(lib),
	}
	if w, h, ok := x11GetGeometry(st); ok {
		st.w, st.h = w, h
	}
	h := &x11Host{st: st, lib: lib}
	return newWindow(h, PlatformX11, nil, nil, h.destroy), nil
}

// --- Xlib binding (purego, no CGO) ---

const (
	xKeyPress         = 2
	xKeyRelease       = 3
	xButtonPress      = 4
	xButtonRelease    = 5
	xMotionNotify     = 6
	xExpose           = 12
	xDestroyNotify    = 17
	xConfigureNotify  = 22
	xClientMessage    = 33
	xNone             = 0
	xNorthWestGravity = 1
	xWhenMapped       = 1
	xCWBackPixmap     = 1 << 0
	xCWBitGravity     = 1 << 4
	xCWWinGravity     = 1 << 5
	xCWBackingStore   = 1 << 6

	xKeyPressMask        = 1 << 0
	xKeyReleaseMask      = 1 << 1
	xButtonPressMask     = 1 << 2
	xButtonReleaseMask   = 1 << 3
	xPointerMotionMask   = 1 << 6
	xExposureMask        = 1 << 15
	xStructureNotifyMask = 1 << 17

	xEventMask = xStructureNotifyMask | xExposureMask |
		xButtonPressMask | xButtonReleaseMask | xPointerMotionMask |
		xKeyPressMask | xKeyReleaseMask
)

// X event field offsets (linux amd64 Xlib layout — matches exhost verified).
const (
	xevTypeOff        = 0
	xevWidthOff       = 56 // XConfigureEvent
	xevHeightOff      = 60
	xevClientData0Off = 56 // XClientMessageEvent.data.l[0]
	xevPointerXOff    = 64
	xevPointerYOff    = 68
	xevButtonOff      = 84 // button (press/release) or keycode (key)
	xevKeycodeOff     = 84
)

// Common X11 keysyms.
const (
	xkTab    = 0xff09
	xkReturn = 0xff0d
	xkSpace  = 0x0020
	xkShiftL = 0xffe1
	xkShiftR = 0xffe2
)

// xSizeHints subset (Xutil.h) — layout matches linux/amd64 libX11.
type xSizeHints struct {
	Flags      int64
	X, Y       int32
	Width      int32
	Height     int32
	MinWidth   int32
	MinHeight  int32
	MaxWidth   int32
	MaxHeight  int32
	WidthInc   int32
	HeightInc  int32
	MinAspectN int32
	MinAspectD int32
	MaxAspectN int32
	MaxAspectD int32
	BaseWidth  int32
	BaseHeight int32
	WinGravity int32
}

// xClassHint (Xutil.h).
type xClassHint struct {
	ResName  *byte
	ResClass *byte
}

type x11Lib struct {
	lib uintptr
	// dynamic funcs (set per open to keep the struct small)
	keycodeToKeysym func(keycode uint, index int) uintptr
	closeDisplay    func(dpy uintptr) int
}

func x11OpenLib() (*x11Lib, error) {
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		return nil, err
	}
	x := &x11Lib{lib: lib}
	purego.RegisterLibFunc(&x.keycodeToKeysym, lib, "XKeycodeToKeysym")
	purego.RegisterLibFunc(&x.closeDisplay, lib, "XCloseDisplay")
	return x, nil
}

// x11Create opens a new X11 window. Ported from the verified exhost host,
// restructured so window lifecycle and event pumping are separate.
func x11Create(w, h int, title string) (*Window, error) {
	if !HasX11Display() {
		return nil, fmt.Errorf("x11: DISPLAY not set")
	}
	lib, err := x11OpenLib()
	if err != nil {
		return nil, err
	}
	var (
		xInitThreads     func() int
		xOpenDisplay     func(name *byte) uintptr
		xDefaultScreen   func(dpy uintptr) int
		xRootWindow      func(dpy uintptr, screen int) uintptr
		xCreateSimple    func(dpy uintptr, parent uintptr, x, y int, width, height, borderWidth uint, border, background uint64) uintptr
		xSetBgPixmap     func(dpy uintptr, win uintptr, pixmap uintptr) int
		xChangeAttr      func(dpy uintptr, win uintptr, valueMask uint64, attrs unsafe.Pointer) int
		xMapWindow       func(dpy uintptr, win uintptr) int
		xFlush           func(dpy uintptr) int
		xDestroyWindow   func(dpy uintptr, win uintptr) int
		xStoreName       func(dpy uintptr, win uintptr, name *byte) int
		xSelectInput     func(dpy uintptr, win uintptr, mask int64) int
		xPending         func(dpy uintptr) int
		xNextEvent       func(dpy uintptr, ev *byte) int
		xInternAtom      func(dpy uintptr, name *byte, onlyIfExists int) uintptr
		xSetWMProtocols  func(dpy uintptr, win uintptr, protocols *uintptr, count int) int
		xSetWMNormalHints func(dpy uintptr, win uintptr, hints *xSizeHints) int
		xSetClassHint     func(dpy uintptr, win uintptr, hint *xClassHint) int
		xChangeProperty   func(dpy uintptr, win uintptr, property, typ uintptr, format int, mode int, data *byte, nelements int) int
	)
	purego.RegisterLibFunc(&xInitThreads, lib.lib, "XInitThreads")
	purego.RegisterLibFunc(&xOpenDisplay, lib.lib, "XOpenDisplay")
	purego.RegisterLibFunc(&xDefaultScreen, lib.lib, "XDefaultScreen")
	purego.RegisterLibFunc(&xRootWindow, lib.lib, "XRootWindow")
	purego.RegisterLibFunc(&xCreateSimple, lib.lib, "XCreateSimpleWindow")
	purego.RegisterLibFunc(&xSetBgPixmap, lib.lib, "XSetWindowBackgroundPixmap")
	purego.RegisterLibFunc(&xChangeAttr, lib.lib, "XChangeWindowAttributes")
	purego.RegisterLibFunc(&xMapWindow, lib.lib, "XMapWindow")
	purego.RegisterLibFunc(&xFlush, lib.lib, "XFlush")
	purego.RegisterLibFunc(&xDestroyWindow, lib.lib, "XDestroyWindow")
	purego.RegisterLibFunc(&xStoreName, lib.lib, "XStoreName")
	purego.RegisterLibFunc(&xSelectInput, lib.lib, "XSelectInput")
	purego.RegisterLibFunc(&xPending, lib.lib, "XPending")
	purego.RegisterLibFunc(&xNextEvent, lib.lib, "XNextEvent")
	purego.RegisterLibFunc(&xInternAtom, lib.lib, "XInternAtom")
	purego.RegisterLibFunc(&xSetWMProtocols, lib.lib, "XSetWMProtocols")
	purego.RegisterLibFunc(&xSetWMNormalHints, lib.lib, "XSetWMNormalHints")
	purego.RegisterLibFunc(&xSetClassHint, lib.lib, "XSetClassHint")
	purego.RegisterLibFunc(&xChangeProperty, lib.lib, "XChangeProperty")

	if xInitThreads() == 0 {
		return nil, fmt.Errorf("x11: XInitThreads failed")
	}
	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		return nil, fmt.Errorf("x11: XOpenDisplay failed")
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	win := xCreateSimple(dpy, root, 80, 60, uint(w), uint(h), 0, 0, 0x001a2a3a)
	if win == 0 {
		lib.closeDisplay(dpy)
		return nil, fmt.Errorf("x11: XCreateSimpleWindow failed")
	}
	// Zero-flash live resize: keep existing pixels anchored at top-left until
	// the next full present catches up.
	xSetBgPixmap(dpy, win, uintptr(xNone))
	attrs := make([]byte, 128)
	*(*int32)(unsafe.Pointer(&attrs[32])) = int32(xNorthWestGravity)
	*(*int32)(unsafe.Pointer(&attrs[36])) = int32(xNorthWestGravity)
	*(*int32)(unsafe.Pointer(&attrs[40])) = int32(xWhenMapped)
	xChangeAttr(dpy, win, uint64(xCWBackPixmap|xCWBitGravity|xCWWinGravity|xCWBackingStore), unsafe.Pointer(&attrs[0]))

	tb := append([]byte(title), 0)
	xStoreName(dpy, win, &tb[0])

	// UTF-8 title.
	utf8Name := append([]byte("UTF8_STRING"), 0)
	netName := append([]byte("_NET_WM_NAME"), 0)
	if atUTF8 := xInternAtom(dpy, &utf8Name[0], 0); atUTF8 != 0 {
		if atNet := xInternAtom(dpy, &netName[0], 0); atNet != 0 {
			xChangeProperty(dpy, win, atNet, atUTF8, 8, 0, &tb[0], len(title))
		}
	}

	// WM_CLASS.
	resName := append([]byte("gpui"), 0)
	resClass := append([]byte("gpui"), 0)
	ch := xClassHint{ResName: &resName[0], ResClass: &resClass[0]}
	xSetClassHint(dpy, win, &ch)

	const (
		pSize     = 1 << 3
		pMinSize  = 1 << 4
		pBaseSize = 1 << 8
	)
	hints := xSizeHints{
		Flags:      pSize | pMinSize | pBaseSize,
		Width:      int32(w),
		Height:     int32(h),
		MinWidth:   160,
		MinHeight:  120,
		BaseWidth:  160,
		BaseHeight: 120,
	}
	xSetWMNormalHints(dpy, win, &hints)
	xSelectInput(dpy, win, xEventMask)

	delName := append([]byte("WM_DELETE_WINDOW"), 0)
	if wmDelete := xInternAtom(dpy, &delName[0], 0); wmDelete != 0 {
		atom := wmDelete
		xSetWMProtocols(dpy, win, &atom, 1)
	}

	// _NET_WM_WINDOW_TYPE_NORMAL.
	typeName := append([]byte("_NET_WM_WINDOW_TYPE"), 0)
	normalName := append([]byte("_NET_WM_WINDOW_TYPE_NORMAL"), 0)
	atomName := append([]byte("ATOM"), 0)
	if atType := xInternAtom(dpy, &typeName[0], 0); atType != 0 {
		if atNormal := xInternAtom(dpy, &normalName[0], 0); atNormal != 0 {
			if atAtom := xInternAtom(dpy, &atomName[0], 0); atAtom != 0 {
				var v uint64 = uint64(atNormal)
				xChangeProperty(dpy, win, atType, atAtom, 32, 0, (*byte)(unsafe.Pointer(&v)), 1)
			}
		}
	}

	xMapWindow(dpy, win)
	xFlush(dpy)
	time.Sleep(50 * time.Millisecond)
	// Drain map noise.
	var buf [256]byte
	for xPending(dpy) > 0 {
		xNextEvent(dpy, &buf[0])
	}

	st := &x11State{
		display:   dpy,
		window:    win,
		wmDelete:  0,
		w:         w,
		h:         h,
		scale:     1,
		pending:   func() int { return xPending(dpy) },
		nextEvent: func(ev *byte) int { return xNextEvent(dpy, ev) },
		flush:     func() { xFlush(dpy) },
		keycodeToKeysym: func(keycode uint, index int) uintptr {
			if lib.keycodeToKeysym == nil {
				return 0
			}
			return lib.keycodeToKeysym(keycode, index)
		},
	}
	// Re-resolve wmDelete after drain (atom still valid).
	{
		delName := append([]byte("WM_DELETE_WINDOW"), 0)
		st.wmDelete = xInternAtom(dpy, &delName[0], 0)
	}
	host := &x11Host{st: st, lib: lib}
	host.destroyFn = func() {
		xDestroyWindow(dpy, win)
		lib.closeDisplay(dpy)
	}
	return newWindow(host, PlatformX11, nil, nil, host.destroy), nil
}

// --- window state ---

type x11State struct {
	display, window uintptr
	wmDelete        uintptr
	mu              sync.Mutex
	w, h            int
	scale           float64
	pending         func() int
	nextEvent       func(ev *byte) int
	flush           func()
	keycodeToKeysym func(keycode uint, index int) uintptr
}

// x11Host implements Host for an X11 window (event pump). Destroying the
// window is the Window.Close callback.
type x11Host struct {
	st        *x11State
	lib       *x11Lib
	wake      chan struct{}
	destroyFn func()
}

func (h *x11Host) destroy() {
	if h == nil || h.destroyFn == nil {
		return
	}
	h.destroyFn()
	h.destroyFn = nil
}

func (h *x11Host) NativeSurface() NativeSurface {
	if h == nil || h.st == nil {
		return NativeSurface{}
	}
	return NativeSurface{Kind: PlatformX11, Display: h.st.display, Window: h.st.window}
}

func (h *x11Host) Size() (int, int) {
	if h == nil || h.st == nil {
		return 1, 1
	}
	h.st.mu.Lock()
	defer h.st.mu.Unlock()
	return h.st.w, h.st.h
}

func (h *x11Host) ScaleFactor() float64 {
	if h == nil || h.st == nil {
		return 1
	}
	h.st.mu.Lock()
	defer h.st.mu.Unlock()
	if h.st.scale <= 0 {
		return 1
	}
	return h.st.scale
}

// WaitVSync uses DRM vblank when available; the scheduler falls back to a
// software tick otherwise.
func (h *x11Host) WaitVSync() error { return WaitDRMVBlank() }

func (h *x11Host) setSize(w, ht int) bool {
	if h == nil || h.st == nil {
		return false
	}
	if w < 1 {
		w = 1
	}
	if ht < 1 {
		ht = 1
	}
	h.st.mu.Lock()
	defer h.st.mu.Unlock()
	if h.st.w == w && h.st.h == ht {
		return false
	}
	h.st.w, h.st.h = w, ht
	return true
}

// WaitEvents drains X11 events (pointer/key/configure/close). Expose is
// always stripped: the GPU backend owns pixels; Expose is not architectural
// IDLE. Resize / input / close still flow.
func (h *x11Host) WaitEvents(timeout time.Duration) []Event {
	if h == nil || h.st == nil {
		return nil
	}
	if h.wake == nil {
		h.wake = make(chan struct{}, 1)
	}
	if timeout == 0 {
		if evs := filterXNoise(h.drainX()); len(evs) > 0 {
			return evs
		}
		if h.st.flush != nil {
			h.st.flush()
		}
		return filterXNoise(h.drainX())
	}

	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	const pollSlice = 16 * time.Millisecond
	for {
		if evs := filterXNoise(h.drainX()); len(evs) > 0 {
			return evs
		}
		wait := pollSlice
		if !deadline.IsZero() {
			left := time.Until(deadline)
			if left <= 0 {
				if h.st.flush != nil {
					h.st.flush()
				}
				return filterXNoise(h.drainX())
			}
			if left < wait {
				wait = left
			}
		}
		select {
		case <-h.wake:
			if evs := filterXNoise(h.drainX()); len(evs) > 0 {
				return evs
			}
			return []Event{{Type: EventWake}}
		case <-time.After(wait):
			if h.st.flush != nil {
				h.st.flush()
			}
		}
	}
}

func (h *x11Host) WakeUp() {
	if h == nil {
		return
	}
	if h.wake == nil {
		h.wake = make(chan struct{}, 1)
	}
	select {
	case h.wake <- struct{}{}:
	default:
	}
}

// filterXNoise drops Expose (GPU present owns pixels). Keep input/lifecycle.
func filterXNoise(evs []Event) []Event {
	var out []Event
	for _, e := range evs {
		if e.Type == EventExpose {
			continue
		}
		out = append(out, e)
	}
	return out
}

func (h *x11Host) drainX() []Event {
	st := h.st
	if st == nil || st.pending == nil {
		return nil
	}
	var out []Event
	var buf [256]byte
	for st.pending() > 0 {
		st.nextEvent(&buf[0])
		t := int(readI32(buf[:], xevTypeOff))
		switch t {
		case xConfigureNotify:
			nw := int(readI32(buf[:], xevWidthOff))
			nh := int(readI32(buf[:], xevHeightOff))
			if nw > 0 && nh > 0 && h.setSize(nw, nh) {
				out = append(out, Event{
					Type: EventResize, Width: nw, Height: nh, Scale: h.ScaleFactor(),
				})
			}
		case xExpose:
			out = append(out, Event{Type: EventExpose})
		case xButtonPress, xButtonRelease, xMotionNotify:
			if ev, ok := h.decodePointer(t, buf[:]); ok {
				out = append(out, ev)
			}
		case xKeyPress, xKeyRelease:
			if ev, ok := h.decodeKey(t, buf[:]); ok {
				out = append(out, ev)
			}
		case xClientMessage:
			data0 := readU64(buf[:], xevClientData0Off)
			if st.wmDelete != 0 && uintptr(data0) == st.wmDelete {
				out = append(out, Event{Type: EventClose})
			}
		case xDestroyNotify:
			out = append(out, Event{Type: EventClose})
		}
	}
	return out
}

func (h *x11Host) decodePointer(t int, buf []byte) (Event, bool) {
	x := float64(readI32(buf, xevPointerXOff))
	y := float64(readI32(buf, xevPointerYOff))
	ev := Event{Type: EventPointer, X: x, Y: y}
	switch t {
	case xMotionNotify:
		ev.Pointer = PointerMove
		return ev, true
	case xButtonPress, xButtonRelease:
		btn := int(readU32(buf, xevButtonOff))
		if btn == 4 || btn == 5 {
			ev.Pointer = PointerScroll
			if btn == 4 {
				ev.ScrollY = -1
			} else {
				ev.ScrollY = 1
			}
			return ev, true
		}
		ev.Button = btn
		if t == xButtonPress {
			ev.Pointer = PointerDown
		} else {
			ev.Pointer = PointerUp
		}
		return ev, true
	}
	return Event{}, false
}

func (h *x11Host) decodeKey(t int, buf []byte) (Event, bool) {
	st := h.st
	keycode := uint(readU32(buf, xevKeycodeOff))
	ev := Event{Type: EventKey, Pressed: t == xKeyPress, KeyCode: int(keycode)}
	if st != nil && st.keycodeToKeysym != nil && keycode != 0 {
		ks := st.keycodeToKeysym(keycode, 0)
		ev.KeyCode = int(ks)
		if ks >= 0x20 && ks <= 0x7e {
			ev.Rune = rune(ks)
		}
		switch ks {
		case xkTab:
			ev.KeyCode = int(xkTab)
		case xkReturn:
			ev.KeyCode = int(xkReturn)
		case xkSpace:
			ev.KeyCode = int(xkSpace)
			ev.Rune = ' '
		case xkShiftL, xkShiftR:
			// keep keysym as KeyCode for focus.IsShift
		}
	}
	return ev, true
}

// --- geometry probe (Adopt) ---

func x11KeycodeToKeysym(lib *x11Lib) func(keycode uint, index int) uintptr {
	if lib == nil {
		return nil
	}
	return lib.keycodeToKeysym
}

// x11GetGeometry probes the current client size via XGetGeometry when the
// symbol is available; returns ok=false on failure (caller keeps defaults).
func x11GetGeometry(st *x11State) (w, h int, ok bool) {
	if st == nil || st.display == 0 || st.window == 0 {
		return 0, 0, false
	}
	lib, err := x11OpenLib()
	if err != nil {
		return 0, 0, false
	}
	var (
		xGetGeometry func(dpy, win, root uintptr, x, y *int32, w, h *uint, border *uint, depth *uint) int
	)
	purego.RegisterLibFunc(&xGetGeometry, lib.lib, "XGetGeometry")
	if xGetGeometry == nil {
		return 0, 0, false
	}
	var (
		rootRet uintptr
		x, y    int32
		wd, ht  uint
		border  uint
		depth   uint
	)
	if xGetGeometry(st.display, st.window, rootRet, &x, &y, &wd, &ht, &border, &depth) == 0 {
		return 0, 0, false
	}
	return int(wd), int(ht), true
}

func readU32(b []byte, off int) uint32 {
	if off+4 > len(b) {
		return 0
	}
	return uint32(b[off]) | uint32(b[off+1])<<8 | uint32(b[off+2])<<16 | uint32(b[off+3])<<24
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
