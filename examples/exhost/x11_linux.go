//go:build linux

package exhost

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/ui/platform"
)

const (
	xDestroyNotify    = 17
	xClientMessage    = 33
	xConfigureNotify  = 22
	xExpose           = 12
	xNone             = 0
	xNorthWestGravity = 1
	xWhenMapped       = 1
	xCWBackPixmap     = 1 << 0
	xCWBitGravity     = 1 << 4
	xCWWinGravity     = 1 << 5
	xCWBackingStore   = 1 << 6
	// StructureNotifyMask | ExposureMask
	xEventMask = (1 << 17) | (1 << 15)
)

const (
	xevTypeOff        = 0
	xevWidthOff       = 56
	xevHeightOff      = 60
	xevClientData0Off = 56
)

// XSizeHints subset (Xutil.h) — enough for PSize|PMinSize|PBaseSize.
// Layout on linux/amd64 matches libX11.
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

// XClassHint (Xutil.h).
type xClassHint struct {
	ResName  *byte
	ResClass *byte
}

func openX11(w, h int, title string) (*Window, error) {
	if !platform.HasX11Display() {
		return nil, fmt.Errorf("DISPLAY not set")
	}
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
		xSetBgPixmap    func(dpy uintptr, win uintptr, pixmap uintptr) int
		xChangeAttr     func(dpy uintptr, win uintptr, valueMask uint64, attrs unsafe.Pointer) int
		xMapWindow      func(dpy uintptr, win uintptr) int
		xFlush          func(dpy uintptr) int
		xDestroyWindow  func(dpy uintptr, win uintptr) int
		xStoreName      func(dpy uintptr, win uintptr, name *byte) int
		xSelectInput    func(dpy uintptr, win uintptr, mask int64) int
		xPending        func(dpy uintptr) int
		xNextEvent      func(dpy uintptr, ev *byte) int
		xInternAtom     func(dpy uintptr, name *byte, onlyIfExists int) uintptr
		xSetWMProtocols func(dpy uintptr, win uintptr, protocols *uintptr, count int) int
		// ICCCM / EWMH so WMs treat us as a normal decorated top-level.
		xSetWMNormalHints func(dpy uintptr, win uintptr, hints *xSizeHints) int
		xSetClassHint     func(dpy uintptr, win uintptr, hint *xClassHint) int
		xChangeProperty   func(dpy uintptr, win uintptr, property, typ uintptr, format int, mode int, data *byte, nelements int) int
	)
	purego.RegisterLibFunc(&xOpenDisplay, lib, "XOpenDisplay")
	purego.RegisterLibFunc(&xCloseDisplay, lib, "XCloseDisplay")
	purego.RegisterLibFunc(&xDefaultScreen, lib, "XDefaultScreen")
	purego.RegisterLibFunc(&xRootWindow, lib, "XRootWindow")
	purego.RegisterLibFunc(&xCreateSimple, lib, "XCreateSimpleWindow")
	purego.RegisterLibFunc(&xSetBgPixmap, lib, "XSetWindowBackgroundPixmap")
	purego.RegisterLibFunc(&xChangeAttr, lib, "XChangeWindowAttributes")
	purego.RegisterLibFunc(&xMapWindow, lib, "XMapWindow")
	purego.RegisterLibFunc(&xFlush, lib, "XFlush")
	purego.RegisterLibFunc(&xDestroyWindow, lib, "XDestroyWindow")
	purego.RegisterLibFunc(&xStoreName, lib, "XStoreName")
	purego.RegisterLibFunc(&xSelectInput, lib, "XSelectInput")
	purego.RegisterLibFunc(&xPending, lib, "XPending")
	purego.RegisterLibFunc(&xNextEvent, lib, "XNextEvent")
	purego.RegisterLibFunc(&xInternAtom, lib, "XInternAtom")
	purego.RegisterLibFunc(&xSetWMProtocols, lib, "XSetWMProtocols")
	purego.RegisterLibFunc(&xSetWMNormalHints, lib, "XSetWMNormalHints")
	purego.RegisterLibFunc(&xSetClassHint, lib, "XSetClassHint")
	purego.RegisterLibFunc(&xChangeProperty, lib, "XChangeProperty")

	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		return nil, fmt.Errorf("XOpenDisplay failed")
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	// borderWidth=0: border is drawn by the WM title bar, not a client black rim.
	win := xCreateSimple(dpy, root, 80, 60, uint(w), uint(h), 0, 0, 0x001a2a3a)
	if win == 0 {
		xCloseDisplay(dpy)
		return nil, fmt.Errorf("XCreateSimpleWindow failed")
	}
	// Zero-flash live resize: do not let X fill newly exposed regions with a
	// solid background while the GPU surface is resizing. Keep existing pixels
	// anchored at the top-left until the next full present catches up.
	xSetBgPixmap(dpy, win, uintptr(xNone))
	attrs := make([]byte, 128)
	*(*int32)(unsafe.Pointer(&attrs[32])) = int32(xNorthWestGravity) // bit_gravity
	*(*int32)(unsafe.Pointer(&attrs[36])) = int32(xNorthWestGravity) // win_gravity
	*(*int32)(unsafe.Pointer(&attrs[40])) = int32(xWhenMapped)       // backing_store
	xChangeAttr(dpy, win, uint64(xCWBackPixmap|xCWBitGravity|xCWWinGravity|xCWBackingStore), unsafe.Pointer(&attrs[0]))
	tb := append([]byte(title), 0)
	xStoreName(dpy, win, &tb[0])

	// UTF-8 title for modern WMs (GNOME/KDE title bar text).
	utf8AtomName := append([]byte("UTF8_STRING"), 0)
	netName := append([]byte("_NET_WM_NAME"), 0)
	atomUTF8 := xInternAtom(dpy, &utf8AtomName[0], 0)
	atomNetName := xInternAtom(dpy, &netName[0], 0)
	if atomUTF8 != 0 && atomNetName != 0 {
		// PropModeReplace = 0
		xChangeProperty(dpy, win, atomNetName, atomUTF8, 8, 0, &tb[0], len(title))
	}

	// WM_CLASS — required by many WMs for normal frame / taskbar grouping.
	resName := append([]byte("gpui"), 0)
	resClass := append([]byte("gpui"), 0)
	ch := xClassHint{ResName: &resName[0], ResClass: &resClass[0]}
	xSetClassHint(dpy, win, &ch)

	// Size hints so the WM maps us as a normal resizable top-level (not override-redirect).
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
	wmDelete := xInternAtom(dpy, &delName[0], 0)
	if wmDelete != 0 {
		atom := wmDelete
		xSetWMProtocols(dpy, win, &atom, 1)
	}

	// _NET_WM_WINDOW_TYPE_NORMAL — ask for standard decorated frame.
	typeName := append([]byte("_NET_WM_WINDOW_TYPE"), 0)
	normalName := append([]byte("_NET_WM_WINDOW_TYPE_NORMAL"), 0)
	atomType := xInternAtom(dpy, &typeName[0], 0)
	atomNormal := xInternAtom(dpy, &normalName[0], 0)
	atomAtomName := append([]byte("ATOM"), 0)
	atomAtom := xInternAtom(dpy, &atomAtomName[0], 0)
	if atomType != 0 && atomNormal != 0 && atomAtom != 0 {
		// 32-bit property: one Atom
		var v uint64 = uint64(atomNormal)
		xChangeProperty(dpy, win, atomType, atomAtom, 32, 0, (*byte)(unsafe.Pointer(&v)), 1)
	}

	xMapWindow(dpy, win)
	xFlush(dpy)
	time.Sleep(50 * time.Millisecond)

	// Drain map noise.
	var buf [256]byte
	for xPending(dpy) > 0 {
		xNextEvent(dpy, &buf[0])
	}

	xw := &x11State{
		display:   dpy,
		window:    win,
		wmDelete:  wmDelete,
		w:         w,
		h:         h,
		pending:   func() int { return xPending(dpy) },
		nextEvent: func(ev *byte) int { return xNextEvent(dpy, ev) },
		flush:     func() { xFlush(dpy) },
	}
	closed := false
	host := &x11Host{st: xw}
	return &Window{
		host:    host,
		kind:    platform.PlatformX11,
		backend: platform.DisplayX11,
		close: func() {
			if closed {
				return
			}
			closed = true
			xDestroyWindow(dpy, win)
			xCloseDisplay(dpy)
		},
	}, nil
}

type x11State struct {
	display, window uintptr
	wmDelete        uintptr
	mu              sync.Mutex
	w, h            int
	scale           float64
	pending         func() int
	nextEvent       func(ev *byte) int
	flush           func()
}

type x11Host struct {
	st   *x11State
	wake chan struct{}
}

func (h *x11Host) NativeSurface() platform.NativeSurface {
	return platform.NativeSurface{
		Kind:    platform.PlatformX11,
		Display: h.st.display,
		Window:  h.st.window,
	}
}

func (h *x11Host) Size() (int, int) {
	h.st.mu.Lock()
	defer h.st.mu.Unlock()
	return h.st.w, h.st.h
}

func (h *x11Host) ScaleFactor() float64 {
	h.st.mu.Lock()
	defer h.st.mu.Unlock()
	if h.st.scale <= 0 {
		return 1
	}
	return h.st.scale
}

func (h *x11Host) setSize(w, ht int) bool {
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

func (h *x11Host) WaitEvents(timeout time.Duration) []platform.Event {
	if h.wake == nil {
		h.wake = make(chan struct{}, 1)
	}
	if evs := h.drainX(); len(evs) > 0 {
		return evs
	}
	if timeout < 0 {
		timeout = 16 * time.Millisecond
	}
	if timeout == 0 {
		if h.st.flush != nil {
			h.st.flush()
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
		if h.st.flush != nil {
			h.st.flush()
		}
		return h.drainX()
	}
}

func (h *x11Host) drainX() []platform.Event {
	if h.st == nil || h.st.pending == nil {
		return nil
	}
	var out []platform.Event
	var buf [256]byte
	for h.st.pending() > 0 {
		h.st.nextEvent(&buf[0])
		t := int(readI32(buf[:], xevTypeOff))
		switch t {
		case xConfigureNotify:
			nw := int(readI32(buf[:], xevWidthOff))
			nh := int(readI32(buf[:], xevHeightOff))
			if nw > 0 && nh > 0 && h.setSize(nw, nh) {
				out = append(out, platform.Event{
					Type: platform.EventResize, Width: nw, Height: nh, Scale: h.ScaleFactor(),
				})
			}
		case xExpose:
			out = append(out, platform.Event{Type: platform.EventExpose})
		case xClientMessage:
			data0 := readU64(buf[:], xevClientData0Off)
			if h.st.wmDelete != 0 && uintptr(data0) == h.st.wmDelete {
				out = append(out, platform.Event{Type: platform.EventClose})
			}
		case xDestroyNotify:
			out = append(out, platform.Event{Type: platform.EventClose})
		}
	}
	return out
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

var _ = unsafe.Pointer(nil)
