//go:build linux && !nogpu

package main

import (
	"fmt"
	"os"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// ---------- X11 ----------

const (
	xConfigureNotify         = 22
	xClientMessage           = 33
	xPropertyNotify          = 28
	xVisibilityNotify        = 15
	xStructureNotify         = int64(1 << 17)
	xExposureMask            = int64(1 << 15)
	xVisibilityChangeMask    = int64(1 << 16)
	xPropertyChangeMask      = int64(1 << 22)
	xVisibilityFullyObscured = 2
	xWMStateIconic           = 3
	xWMStateWithdrawn        = 0
)

type x11Event struct {
	Type          int
	Width, Height int
	Visibility    int
	raw           [192]byte
}

type x11Win struct {
	lib                uintptr
	Display            uintptr
	Window             uintptr
	wmDeleteAtom       uintptr
	wmStateAtom        uintptr
	xPending           func(dpy uintptr) int
	xNextEvent         func(dpy uintptr, ev *byte) int
	xFlush             func(dpy uintptr) int
	xDestroyWindow     func(dpy uintptr, win uintptr) int
	xCloseDisplay      func(dpy uintptr) int
	xInternAtom        func(dpy uintptr, name *byte, onlyIfExists int) uintptr
	xSetWMProtocols    func(dpy uintptr, win uintptr, protocols *uintptr, count int) int
	xSetWMNormalHints  func(dpy uintptr, win uintptr, hints *byte) int
	xGetWindowProperty func(dpy, win, prop uintptr, offset, length int64, delete, reqType int, actualType *uintptr, actualFormat *int32, nitems, bytesAfter *uint64, propRet **byte) int
	xFree              func(ptr uintptr) int
}

// IsIconic reports GNOME-style minimize (WM_STATE Iconic).
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

// LockSize sets min=max size hints so the WM cannot maximize/tile during soaks.
func (w *x11Win) LockSize(width, height int) {
	if w == nil || w.xSetWMNormalHints == nil || w.Display == 0 || w.Window == 0 {
		return
	}
	if width < 64 {
		width = 64
	}
	if height < 64 {
		height = 64
	}
	// XSizeHints on LP64 Linux (xlib): long flags; int x,y,width,height,min_*,max_*,...
	// flags: PSize(8)|PMinSize(16)|PMaxSize(32) = 56
	var buf [128]byte
	*(*int64)(unsafe.Pointer(&buf[0])) = 8 | 16 | 32
	*(*int32)(unsafe.Pointer(&buf[8])) = 60  // x
	*(*int32)(unsafe.Pointer(&buf[12])) = 40 // y
	*(*int32)(unsafe.Pointer(&buf[16])) = int32(width)
	*(*int32)(unsafe.Pointer(&buf[20])) = int32(height)
	*(*int32)(unsafe.Pointer(&buf[24])) = int32(width)  // min_width
	*(*int32)(unsafe.Pointer(&buf[28])) = int32(height) // min_height
	*(*int32)(unsafe.Pointer(&buf[32])) = int32(width)  // max_width
	*(*int32)(unsafe.Pointer(&buf[36])) = int32(height) // max_height
	w.xSetWMNormalHints(w.Display, w.Window, &buf[0])
	if w.xFlush != nil {
		w.xFlush(w.Display)
	}
}

func (w *x11Win) Close() {
	if w == nil {
		return
	}
	if w.xDestroyWindow != nil && w.Display != 0 && w.Window != 0 {
		w.xDestroyWindow(w.Display, w.Window)
	}
	if w.xCloseDisplay != nil && w.Display != 0 {
		w.xCloseDisplay(w.Display)
	}
	if w.lib != 0 {
		_ = purego.Dlclose(w.lib)
	}
}
func (w *x11Win) Flush() {
	if w != nil && w.xFlush != nil {
		w.xFlush(w.Display)
	}
}
func (w *x11Win) Pending() bool {
	return w != nil && w.xPending != nil && w.xPending(w.Display) > 0
}
func (w *x11Win) NextEvent() x11Event {
	var ev x11Event
	// filled below
	if w == nil || w.xNextEvent == nil {
		return ev
	}
	w.xNextEvent(w.Display, &ev.raw[0])
	ev.Type = int(*(*int32)(unsafe.Pointer(&ev.raw[0])))
	if ev.Type == xVisibilityNotify {
		ev.Visibility = int(*(*int32)(unsafe.Pointer(&ev.raw[8])))
	}
	if ev.Type == xConfigureNotify {
		// LP64 XConfigureEvent: width@56 height@60
		ev.Width = int(*(*int32)(unsafe.Pointer(&ev.raw[56])))
		ev.Height = int(*(*int32)(unsafe.Pointer(&ev.raw[60])))
	}
	return ev
}
func (w *x11Win) IsDelete(ev x11Event) bool {
	if w == nil || ev.Type != xClientMessage || w.wmDeleteAtom == 0 {
		return false
	}
	msgType := *(*uintptr)(unsafe.Pointer(&ev.raw[40]))
	data0 := *(*uintptr)(unsafe.Pointer(&ev.raw[56]))
	return data0 == w.wmDeleteAtom || msgType == w.wmDeleteAtom
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
	var xGetWindowProperty func(dpy, win, prop uintptr, offset, length int64, delete, reqType int, actualType *uintptr, actualFormat *int32, nitems, bytesAfter *uint64, propRet **byte) int
	var xFree func(ptr uintptr) int
	purego.RegisterLibFunc(&xGetWindowProperty, lib, "XGetWindowProperty")
	purego.RegisterLibFunc(&xFree, lib, "XFree")

	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		_ = purego.Dlclose(lib)
		return nil, fmt.Errorf("XOpenDisplay failed (DISPLAY=%q)", os.Getenv("DISPLAY"))
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	win := xCreateSimple(dpy, root, 60, 40, uint(w), uint(h), 1, 0, 0)
	if win == 0 {
		xCloseDisplay(dpy)
		_ = purego.Dlclose(lib)
		return nil, fmt.Errorf("XCreateSimpleWindow failed")
	}
	name := append([]byte(title), 0)
	xStoreName(dpy, win, &name[0])
	xSelectInput(dpy, win, xStructureNotify|xExposureMask|xVisibilityChangeMask|xPropertyChangeMask)

	atomName := append([]byte("WM_DELETE_WINDOW"), 0)
	delAtom := xInternAtom(dpy, &atomName[0], 0)
	if delAtom != 0 {
		prot := delAtom
		xSetWMProtocols(dpy, win, &prot, 1)
	}
	stName := append([]byte("WM_STATE"), 0)
	wmState := xInternAtom(dpy, &stName[0], 0)
	// Register size-hint setter for LockSize (timed soaks keep 800x600).
	var xSetWMNormalHints func(dpy uintptr, win uintptr, hints *byte) int
	purego.RegisterLibFunc(&xSetWMNormalHints, lib, "XSetWMNormalHints")

	xMapWindow(dpy, win)
	xFlush(dpy)
	time.Sleep(50 * time.Millisecond)

	xw := &x11Win{
		lib: lib, Display: dpy, Window: win, wmDeleteAtom: delAtom, wmStateAtom: wmState,
		xPending: xPending, xNextEvent: xNextEvent, xFlush: xFlush,
		xDestroyWindow: xDestroyWindow, xCloseDisplay: xCloseDisplay,
		xInternAtom: xInternAtom, xSetWMProtocols: xSetWMProtocols,
		xSetWMNormalHints:  xSetWMNormalHints,
		xGetWindowProperty: xGetWindowProperty, xFree: xFree,
	}
	xw.LockSize(w, h)
	return xw, nil
}
