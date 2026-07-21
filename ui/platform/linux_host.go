//go:build linux && !nouiplatform

package platform

import (
	"fmt"
	"os"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Linux X11 thin adapter (M0). Real present/GPU stays in the app/example host
// loop via NativeHandles; this package only owns window + input.

const (
	xKeyPress         = 2
	xKeyRelease       = 3
	xButtonPress      = 4
	xButtonRelease    = 5
	xMotionNotify     = 6
	xFocusIn          = 9
	xFocusOut         = 10
	xExpose           = 12
	xVisibilityNotify = 15
	xConfigureNotify  = 22
	xClientMessage    = 33

	xKeyPressMask         = int64(1 << 0)
	xKeyReleaseMask       = int64(1 << 1)
	xButtonPressMask      = int64(1 << 2)
	xButtonReleaseMask    = int64(1 << 3)
	xPointerMotionMask    = int64(1 << 6)
	xExposureMask         = int64(1 << 15)
	xVisibilityChangeMask = int64(1 << 16)
	xStructureNotifyMask  = int64(1 << 17)
	xFocusChangeMask      = int64(1 << 21)
)

// LinuxHost is a minimal X11 window Host.
type LinuxHost struct {
	lib     uintptr
	display uintptr
	window  uintptr
	screen  int
	width   int
	height  int
	scale   float64
	title   string

	wmDelete uintptr

	xPending       func(dpy uintptr) int
	xNextEvent     func(dpy uintptr, ev *byte) int
	xFlush         func(dpy uintptr) int
	xDestroyWindow func(dpy uintptr, win uintptr) int
	xCloseDisplay  func(dpy uintptr) int
	xStoreName     func(dpy uintptr, win uintptr, name *byte) int

	queue  []Event
	closed bool
}

// LinuxOptions configures NewLinuxHost.
type LinuxOptions struct {
	Width, Height int
	Title         string
	Scale         float64
}

// NewLinuxHost opens a simple X11 window. Requires DISPLAY.
func NewLinuxHost(opts LinuxOptions) (*LinuxHost, error) {
	if opts.Width <= 0 {
		opts.Width = 640
	}
	if opts.Height <= 0 {
		opts.Height = 480
	}
	if opts.Title == "" {
		opts.Title = "gpui"
	}
	if opts.Scale <= 0 {
		opts.Scale = 1
	}
	if os.Getenv("DISPLAY") == "" {
		_ = os.Setenv("DISPLAY", ":1")
	}

	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		return nil, fmt.Errorf("platform/linux: dlopen libX11: %w", err)
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
		_ = purego.Dlclose(lib)
		return nil, fmt.Errorf("platform/linux: XOpenDisplay failed (DISPLAY=%q)", os.Getenv("DISPLAY"))
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	win := xCreateSimple(dpy, root, 80, 60, uint(opts.Width), uint(opts.Height), 1, 0, 0x202020)
	if win == 0 {
		xCloseDisplay(dpy)
		_ = purego.Dlclose(lib)
		return nil, fmt.Errorf("platform/linux: XCreateSimpleWindow failed")
	}
	name := append([]byte(opts.Title), 0)
	xStoreName(dpy, win, &name[0])

	mask := xStructureNotifyMask | xExposureMask | xVisibilityChangeMask |
		xFocusChangeMask | xButtonPressMask | xButtonReleaseMask |
		xPointerMotionMask | xKeyPressMask | xKeyReleaseMask
	xSelectInput(dpy, win, mask)

	atomName := append([]byte("WM_DELETE_WINDOW"), 0)
	delAtom := xInternAtom(dpy, &atomName[0], 0)
	if delAtom != 0 {
		prot := delAtom
		xSetWMProtocols(dpy, win, &prot, 1)
	}

	xMapWindow(dpy, win)
	xFlush(dpy)
	time.Sleep(30 * time.Millisecond)

	h := &LinuxHost{
		lib: lib, display: dpy, window: win, screen: screen,
		width: opts.Width, height: opts.Height, scale: opts.Scale, title: opts.Title,
		wmDelete: delAtom,
		xPending: xPending, xNextEvent: xNextEvent, xFlush: xFlush,
		xDestroyWindow: xDestroyWindow, xCloseDisplay: xCloseDisplay,
		xStoreName: xStoreName,
	}
	return h, nil
}

// Caps implements Host.
func (h *LinuxHost) Caps() Caps {
	return CapWindow | CapPointer | CapKeyboard | CapTextInput | CapPresent | CapSurfaceLifecycle | CapCursor
}

// Size implements Host.
func (h *LinuxHost) Size() (int, int) { return h.width, h.height }

// ScaleFactor implements Host.
func (h *LinuxHost) ScaleFactor() float64 {
	if h.scale <= 0 {
		return 1
	}
	return h.scale
}

// Display implements NativeHandles.
func (h *LinuxHost) Display() uintptr { return h.display }

// Window implements NativeHandles.
func (h *LinuxHost) Window() uintptr { return h.window }

// Flush flushes the X connection.
func (h *LinuxHost) Flush() {
	if h != nil && h.xFlush != nil && h.display != 0 {
		h.xFlush(h.display)
	}
}

// PumpEvents implements Host.
func (h *LinuxHost) PumpEvents() []Event {
	if h == nil || h.closed {
		return nil
	}
	for h.xPending != nil && h.xPending(h.display) > 0 {
		var raw [192]byte
		h.xNextEvent(h.display, &raw[0])
		h.handleRaw(&raw)
	}
	if len(h.queue) == 0 {
		return nil
	}
	out := h.queue
	h.queue = nil
	return out
}

func (h *LinuxHost) handleRaw(raw *[192]byte) {
	typ := int(*(*int32)(unsafe.Pointer(&raw[0])))
	switch typ {
	case xConfigureNotify:
		// LP64 XConfigureEvent: width@56 height@60
		w := int(*(*int32)(unsafe.Pointer(&raw[56])))
		ht := int(*(*int32)(unsafe.Pointer(&raw[60])))
		if w > 0 && ht > 0 && (w != h.width || ht != h.height) {
			h.width, h.height = w, ht
			h.queue = append(h.queue, Event{Type: EventResize, Width: w, Height: ht})
		}
	case xButtonPress, xButtonRelease:
		// XButtonEvent LP64: x@64 y@68 button@84 (approx on amd64)
		x := float64(*(*int32)(unsafe.Pointer(&raw[64])))
		y := float64(*(*int32)(unsafe.Pointer(&raw[68])))
		btnN := int(*(*uint32)(unsafe.Pointer(&raw[84])))
		btn := BtnNone
		switch btnN {
		case 1:
			btn = BtnLeft
		case 2:
			btn = BtnMiddle
		case 3:
			btn = BtnRight
		}
		kind := PointerDown
		if typ == xButtonRelease {
			kind = PointerUp
		}
		h.queue = append(h.queue, Event{Type: EventPointer, Pointer: kind, X: x, Y: y, Button: btn})
	case xMotionNotify:
		x := float64(*(*int32)(unsafe.Pointer(&raw[64])))
		y := float64(*(*int32)(unsafe.Pointer(&raw[68])))
		h.queue = append(h.queue, Event{Type: EventPointer, Pointer: PointerMove, X: x, Y: y})
	case xExpose:
		h.queue = append(h.queue, Event{Type: EventRedraw})
	case xFocusIn:
		h.queue = append(h.queue, Event{Type: EventFocus, Focused: true})
	case xFocusOut:
		h.queue = append(h.queue, Event{Type: EventFocus, Focused: false})
	case xClientMessage:
		msgType := *(*uintptr)(unsafe.Pointer(&raw[40]))
		data0 := *(*uintptr)(unsafe.Pointer(&raw[56]))
		if h.wmDelete != 0 && (data0 == h.wmDelete || msgType == h.wmDelete) {
			h.queue = append(h.queue, Event{Type: EventClose})
		}
	}
}

// RequestRedraw implements Host (X11 expose is async; we just enqueue).
func (h *LinuxHost) RequestRedraw() {
	h.queue = append(h.queue, Event{Type: EventRedraw})
}

// Close implements Host.
func (h *LinuxHost) Close() error {
	if h == nil || h.closed {
		return nil
	}
	h.closed = true
	if h.xDestroyWindow != nil && h.display != 0 && h.window != 0 {
		h.xDestroyWindow(h.display, h.window)
	}
	if h.xCloseDisplay != nil && h.display != 0 {
		h.xCloseDisplay(h.display)
	}
	if h.lib != 0 {
		_ = purego.Dlclose(h.lib)
	}
	h.display, h.window, h.lib = 0, 0, 0
	return nil
}

var (
	_ Host          = (*LinuxHost)(nil)
	_ NativeHandles = (*LinuxHost)(nil)
)
