//go:build linux && !nogpu

package main

import (
	"fmt"
	"log"
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
	xPropertyNotify   = 28
	xClientMessage    = 33
	xFocusIn          = 9
	xFocusOut         = 10

	xVisibilityUnobscured        = 0
	xVisibilityPartiallyObscured = 1
	xVisibilityFullyObscured     = 2

	xKeyPressMask         = 1 << 0
	xExposureMask         = 1 << 15
	xVisibilityChangeMask = 1 << 16
	xStructureNotifyMask  = 1 << 17
	xFocusChangeMask      = 1 << 21
	xPropertyChangeMask   = 1 << 22

	xNone             = 0
	xNorthWestGravity = 1
	xWhenMapped       = 1
	xCWBackPixmap     = 1 << 0
	xCWBitGravity     = 1 << 4
	xCWWinGravity     = 1 << 5
	xCWBackingStore   = 1 << 6

	xWMStateWithdrawn = 0
	xWMStateNormal    = 1
	xWMStateIconic    = 3
)

type x11Event struct {
	Type       int
	Width      int
	Height     int
	Visibility int
	KeyCode    int
	Atom       uintptr
}

type x11Win struct {
	lib     uintptr
	Display uintptr
	Window  uintptr
	Screen  int

	xPending           func(dpy uintptr) int
	xNextEvent         func(dpy uintptr, ev unsafe.Pointer)
	xFlush             func(dpy uintptr) int
	xClose             func(dpy uintptr) int
	xDestroy           func(dpy uintptr, win uintptr) int
	xGetWindowProperty func(dpy, win, prop uintptr, offset, length int64, delete, reqType int, actualType *uintptr, actualFormat *int32, nitems, bytesAfter *uint64, propRet **byte) int
	xFree              func(p uintptr) int
	eventBytes         []byte
	wmDelete           uintptr
	wmStateAtom        uintptr
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
		xOpenDisplay       func(name *byte) uintptr
		xCloseDisplay      func(dpy uintptr) int
		xDefaultScreen     func(dpy uintptr) int
		xRootWindow        func(dpy uintptr, screen int) uintptr
		xCreateSimple      func(dpy uintptr, parent uintptr, x, y int, width, height, borderWidth uint, border, background uint64) uintptr
		xMapWindow         func(dpy uintptr, win uintptr) int
		xFlush             func(dpy uintptr) int
		xDestroyWindow     func(dpy uintptr, win uintptr) int
		xStoreName         func(dpy uintptr, win uintptr, name *byte) int
		xSelectInput       func(dpy uintptr, win uintptr, mask int64) int
		xPending           func(dpy uintptr) int
		xNextEvent         func(dpy uintptr, ev unsafe.Pointer)
		xInternAtom        func(dpy uintptr, name *byte, onlyIfExists int) uintptr
		xSetWMProtocols    func(dpy uintptr, win uintptr, protocols *uintptr, count int) int
		xGetWindowProperty func(dpy, win, prop uintptr, offset, length int64, delete, reqType int, actualType *uintptr, actualFormat *int32, nitems, bytesAfter *uint64, propRet **byte) int
		xFree              func(p uintptr) int
		xInitThreads       func() int
		xSetBgPixmap       func(dpy, win, pixmap uintptr) int
		xChangeAttr        func(dpy, win uintptr, mask uint64, attrs unsafe.Pointer) int
	)
	purego.RegisterLibFunc(&xInitThreads, lib, "XInitThreads")
	_ = xInitThreads()

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
	purego.RegisterLibFunc(&xGetWindowProperty, lib, "XGetWindowProperty")
	purego.RegisterLibFunc(&xFree, lib, "XFree")
	purego.RegisterLibFunc(&xSetBgPixmap, lib, "XSetWindowBackgroundPixmap")
	purego.RegisterLibFunc(&xChangeAttr, lib, "XChangeWindowAttributes")

	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		for _, name := range []string{":0", ":1", ":0.0", ":1.0"} {
			b := append([]byte(name), 0)
			dpy = xOpenDisplay(&b[0])
			if dpy != 0 {
				if os.Getenv("DISPLAY") == "" {
					_ = os.Setenv("DISPLAY", name)
				}
				log.Printf("XOpenDisplay fallback %s", name)
				break
			}
		}
	}
	if dpy == 0 {
		_ = purego.Dlclose(lib)
		return nil, fmt.Errorf("XOpenDisplay failed (DISPLAY=%q); start X11 or set DISPLAY", os.Getenv("DISPLAY"))
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	win := xCreateSimple(dpy, root, 100, 80, uint(w), uint(h), 1, 0, 0)
	if win == 0 {
		xCloseDisplay(dpy)
		_ = purego.Dlclose(lib)
		return nil, fmt.Errorf("XCreateSimpleWindow failed")
	}

	// Anti-flash resize: no black background fill; keep NW gravity.
	xSetBgPixmap(dpy, win, uintptr(xNone))
	attrs := make([]byte, 128)
	*(*int32)(unsafe.Pointer(&attrs[32])) = int32(xNorthWestGravity)
	*(*int32)(unsafe.Pointer(&attrs[36])) = int32(xNorthWestGravity)
	*(*int32)(unsafe.Pointer(&attrs[40])) = int32(xWhenMapped)
	xChangeAttr(dpy, win, uint64(xCWBackPixmap|xCWBitGravity|xCWWinGravity|xCWBackingStore), unsafe.Pointer(&attrs[0]))

	name := append([]byte(title), 0)
	xStoreName(dpy, win, &name[0])

	evMask := int64(xStructureNotifyMask | xExposureMask | xVisibilityChangeMask | xFocusChangeMask | xKeyPressMask | xPropertyChangeMask)
	xSelectInput(dpy, win, evMask)

	delName := append([]byte("WM_DELETE_WINDOW"), 0)
	wmDelete := xInternAtom(dpy, &delName[0], 0)
	stName := append([]byte("WM_STATE"), 0)
	wmState := xInternAtom(dpy, &stName[0], 0)
	if wmDelete != 0 {
		prot := wmDelete
		xSetWMProtocols(dpy, win, &prot, 1)
	}

	xMapWindow(dpy, win)
	xFlush(dpy)
	time.Sleep(40 * time.Millisecond)

	return &x11Win{
		lib:                lib,
		Display:            dpy,
		Screen:             screen,
		Window:             win,
		xPending:           xPending,
		xNextEvent:         xNextEvent,
		xFlush:             xFlush,
		xClose:             xCloseDisplay,
		xDestroy:           xDestroyWindow,
		xGetWindowProperty: xGetWindowProperty,
		xFree:              xFree,
		eventBytes:         make([]byte, 192),
		wmDelete:           wmDelete,
		wmStateAtom:        wmState,
	}, nil
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

func (w *x11Win) NextEvent() x11Event {
	if w == nil || w.xNextEvent == nil {
		return x11Event{}
	}
	for i := range w.eventBytes {
		w.eventBytes[i] = 0
	}
	w.xNextEvent(w.Display, unsafe.Pointer(&w.eventBytes[0]))
	typ := int(*(*int32)(unsafe.Pointer(&w.eventBytes[0])))
	ev := x11Event{Type: typ}
	switch typ {
	case xConfigureNotify:
		ev.Width = int(*(*int32)(unsafe.Pointer(&w.eventBytes[56])))
		ev.Height = int(*(*int32)(unsafe.Pointer(&w.eventBytes[60])))
		if ev.Width < 1 || ev.Height < 1 {
			ev.Width = int(*(*int32)(unsafe.Pointer(&w.eventBytes[40])))
			ev.Height = int(*(*int32)(unsafe.Pointer(&w.eventBytes[44])))
		}
	case xVisibilityNotify:
		ev.Visibility = int(*(*int32)(unsafe.Pointer(&w.eventBytes[48])))
		if ev.Visibility < 0 || ev.Visibility > 2 {
			ev.Visibility = int(w.eventBytes[32])
		}
	case xKeyPress:
		ev.KeyCode = int(w.eventBytes[84])
	case xPropertyNotify:
		ev.Atom = *(*uintptr)(unsafe.Pointer(&w.eventBytes[40]))
	}
	return ev
}

func (w *x11Win) IsDeleteMessage(ev x11Event) bool {
	if w == nil || ev.Type != xClientMessage || w.wmDelete == 0 {
		return false
	}
	atom := *(*uintptr)(unsafe.Pointer(&w.eventBytes[56]))
	return atom == w.wmDelete
}

func (w *x11Win) IsWMStateProperty(ev x11Event) bool {
	return w != nil && ev.Type == xPropertyNotify && w.wmStateAtom != 0 && ev.Atom == w.wmStateAtom
}

// IsIconic reports GNOME-style minimize (WM_STATE Iconic without UnmapNotify).
func (w *x11Win) IsIconic() bool {
	if w == nil || w.xGetWindowProperty == nil || w.wmStateAtom == 0 || w.Display == 0 || w.Window == 0 {
		return false
	}
	var actualType uintptr
	var actualFormat int32
	var nitems, bytesAfter uint64
	var prop *byte
	status := w.xGetWindowProperty(
		w.Display, w.Window, w.wmStateAtom,
		0, 2, 0, 0,
		&actualType, &actualFormat, &nitems, &bytesAfter, &prop,
	)
	if status != 0 || prop == nil || nitems < 1 {
		if prop != nil && w.xFree != nil {
			w.xFree(uintptr(unsafe.Pointer(prop)))
		}
		return false
	}
	state := *(*uint32)(unsafe.Pointer(prop))
	if w.xFree != nil {
		w.xFree(uintptr(unsafe.Pointer(prop)))
	}
	return state == uint32(xWMStateIconic) || state == uint32(xWMStateWithdrawn)
}
