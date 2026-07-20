//go:build linux && !nogpu

package main

import (
	"fmt"
	"os"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Minimal X11 host for RequestRedraw-driven examples.
// Event drain stays on the main goroutine; GPU work must not live here.

const (
	xKeyPress         = 2
	xExposure         = 12
	xVisibilityNotify = 15
	xDestroyNotify    = 17
	xUnmapNotify      = 18
	xMapNotify        = 19
	xConfigureNotify  = 22
	xClientMessage    = 33
	xFocusIn          = 9
	xFocusOut         = 10

	xVisibilityUnobscured        = 0
	xVisibilityPartiallyObscured = 1
	xVisibilityFullyObscured     = 2

	// event masks
	xKeyPressMask         = 1 << 0
	xExposureMask         = 1 << 15
	xStructureNotifyMask  = 1 << 17
	xVisibilityChangeMask = 1 << 16
	xFocusChangeMask      = 1 << 21
)

type x11Event struct {
	Type       int
	Width      int
	Height     int
	Visibility int
	KeyCode    int
}

type x11Win struct {
	lib     uintptr
	Display uintptr
	Window  uintptr

	xPending   func(dpy uintptr) int
	xNextEvent func(dpy uintptr, ev unsafe.Pointer)
	xFlush     func(dpy uintptr) int
	xClose     func(dpy uintptr) int
	xDestroy   func(dpy uintptr, win uintptr) int
	xGetAtom   func(dpy uintptr, name *byte, onlyIfExists int) uintptr
	eventBytes []byte
	wmDelete   uintptr
}

func openX11Window(w, h int, title string) (*x11Win, error) {
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		return nil, fmt.Errorf("dlopen libX11: %w", err)
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
		xNextEvent      func(dpy uintptr, ev unsafe.Pointer)
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
		return nil, fmt.Errorf("XOpenDisplay failed (DISPLAY=%q)", os.Getenv("DISPLAY"))
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	win := xCreateSimple(dpy, root, 100, 80, uint(w), uint(h), 1, 0, 0)
	if win == 0 {
		xCloseDisplay(dpy)
		_ = purego.Dlclose(lib)
		return nil, fmt.Errorf("XCreateSimpleWindow failed")
	}
	name := append([]byte(title), 0)
	xStoreName(dpy, win, &name[0])

	mask := int64(xStructureNotifyMask | xExposureMask | xVisibilityChangeMask | xFocusChangeMask | xKeyPressMask)
	xSelectInput(dpy, win, mask)

	delName := append([]byte("WM_DELETE_WINDOW"), 0)
	wmDelete := xInternAtom(dpy, &delName[0], 0)
	if wmDelete != 0 {
		prot := wmDelete
		xSetWMProtocols(dpy, win, &prot, 1)
	}

	xMapWindow(dpy, win)
	xFlush(dpy)
	time.Sleep(40 * time.Millisecond)

	xw := &x11Win{
		lib:        lib,
		Display:    dpy,
		Window:     win,
		xPending:   xPending,
		xNextEvent: xNextEvent,
		xFlush:     xFlush,
		xClose:     xCloseDisplay,
		xDestroy:   xDestroyWindow,
		eventBytes: make([]byte, 192), // XEvent is large; pad safely
		wmDelete:   wmDelete,
	}
	return xw, nil
}

func (w *x11Win) Close() {
	if w == nil {
		return
	}
	if w.xDestroy != nil && w.Display != 0 && w.Window != 0 {
		w.xDestroy(w.Display, w.Window)
	}
	if w.xClose != nil && w.Display != 0 {
		w.xClose(w.Display)
	}
	if w.lib != 0 {
		_ = purego.Dlclose(w.lib)
		w.lib = 0
	}
	w.Display, w.Window = 0, 0
}

func (w *x11Win) Flush() {
	if w != nil && w.xFlush != nil && w.Display != 0 {
		w.xFlush(w.Display)
	}
}

func (w *x11Win) Pending() bool {
	return w != nil && w.xPending != nil && w.xPending(w.Display) > 0
}

// NextEvent blocks until an event is available. Prefer Pending()+NextEvent in a drain loop.
func (w *x11Win) NextEvent() x11Event {
	if w == nil || w.xNextEvent == nil {
		return x11Event{}
	}
	// Zero buffer; XNextEvent fills an XEvent.
	for i := range w.eventBytes {
		w.eventBytes[i] = 0
	}
	w.xNextEvent(w.Display, unsafe.Pointer(&w.eventBytes[0]))
	// XEvent layout (Xlib): type is first int (32-bit).
	typ := int(*(*int32)(unsafe.Pointer(&w.eventBytes[0])))
	ev := x11Event{Type: typ}
	switch typ {
	case xConfigureNotify:
		// xconfigure: type, serial, send_event, display, event, window, x, y, width, height, ...
		// Offsets vary with 32/64; use conservative 64-bit Xlib offsets used elsewhere in tree.
		ev.Width = int(*(*int32)(unsafe.Pointer(&w.eventBytes[32])))
		ev.Height = int(*(*int32)(unsafe.Pointer(&w.eventBytes[36])))
		if ev.Width == 0 && ev.Height == 0 {
			// Fallback: common 64-bit packing after two window fields.
			ev.Width = int(*(*int32)(unsafe.Pointer(&w.eventBytes[40])))
			ev.Height = int(*(*int32)(unsafe.Pointer(&w.eventBytes[44])))
		}
	case xVisibilityNotify:
		// state is after window fields; try a few offsets.
		ev.Visibility = int(w.eventBytes[32])
		if ev.Visibility > 2 {
			ev.Visibility = int(w.eventBytes[24])
		}
	case xKeyPress:
		// keycode often at offset 84 on 64-bit — best-effort for q/esc.
		ev.KeyCode = int(w.eventBytes[84])
	case xClientMessage:
		// data.l[0] atom — if equals wmDelete, treat as close (checked in main).
	}
	return ev
}

func (w *x11Win) IsDeleteMessage(ev x11Event) bool {
	if w == nil || ev.Type != xClientMessage || w.wmDelete == 0 {
		return false
	}
	// ClientMessage data.l[0] typically at offset 56 on amd64 Xlib.
	atom := *(*uintptr)(unsafe.Pointer(&w.eventBytes[56]))
	return atom == w.wmDelete
}
