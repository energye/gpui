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
	xUnmapNotify     = 18
	xMapNotify       = 19
	xConfigureNotify = 22
	xPropertyNotify  = 28
	xClientMessage   = 33
	xStructureNotify = int64(1 << 17)
	xExposureMask    = int64(1 << 15)
	// PropertyChangeMask: GNOME Iconify often sets WM_STATE without UnmapNotify.
	xPropertyChangeMask = int64(1 << 22)

	// ICCCM WM_STATE values.
	xWMStateWithdrawn = 0
	xWMStateNormal    = 1
	xWMStateIconic    = 3
)

type x11Event struct {
	Type          int
	Width, Height int
	// Atom is set for PropertyNotify (property atom).
	Atom uintptr
	raw  [192]byte
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
	xResizeWindow      func(dpy uintptr, win uintptr, w, h uint32) int
	xGetWindowProperty func(
		dpy uintptr, win uintptr, property uintptr,
		offset, length int64, delete int, reqType uintptr,
		actualType *uintptr, actualFormat *int32,
		nitems *uint64, bytesAfter *uint64, prop **byte,
	) int
	xFree func(ptr uintptr) int
}

// Resize asks the X server to change the window size (real present-path resize).
func (w *x11Win) Resize(width, height int) {
	if w == nil || w.xResizeWindow == nil || w.Display == 0 || w.Window == 0 {
		return
	}
	if width < 64 {
		width = 64
	}
	if height < 64 {
		height = 64
	}
	w.xResizeWindow(w.Display, w.Window, uint32(width), uint32(height))
	if w.xFlush != nil {
		w.xFlush(w.Display)
	}
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
	if w == nil || w.xNextEvent == nil {
		return ev
	}
	w.xNextEvent(w.Display, &ev.raw[0])
	ev.Type = int(*(*int32)(unsafe.Pointer(&ev.raw[0])))
	if ev.Type == xConfigureNotify {
		// LP64 XConfigureEvent: width@56 height@60
		ev.Width = int(*(*int32)(unsafe.Pointer(&ev.raw[56])))
		ev.Height = int(*(*int32)(unsafe.Pointer(&ev.raw[60])))
	}
	if ev.Type == xPropertyNotify {
		// LP64 XPropertyEvent: atom@40
		ev.Atom = *(*uintptr)(unsafe.Pointer(&ev.raw[40]))
	}
	return ev
}

// IsWMStateProperty reports whether ev is a PropertyNotify for WM_STATE
// (GNOME Iconify path without UnmapNotify).
func (w *x11Win) IsWMStateProperty(ev x11Event) bool {
	return w != nil && ev.Type == xPropertyNotify && w.wmStateAtom != 0 && ev.Atom == w.wmStateAtom
}

// IsIconic queries ICCCM WM_STATE. Returns true when IconicState (minimized).
// Used for GNOME Iconify where UnmapNotify may not fire.
func (w *x11Win) IsIconic() bool {
	if w == nil || w.xGetWindowProperty == nil || w.wmStateAtom == 0 || w.Display == 0 || w.Window == 0 {
		return false
	}
	var actualType uintptr
	var actualFormat int32
	var nitems, bytesAfter uint64
	var prop *byte
	// long_length=2: state + icon window
	status := w.xGetWindowProperty(
		w.Display, w.Window, w.wmStateAtom,
		0, 2, 0, 0, // AnyPropertyType = 0
		&actualType, &actualFormat, &nitems, &bytesAfter, &prop,
	)
	if status != 0 /* Success */ || prop == nil || nitems < 1 {
		if prop != nil && w.xFree != nil {
			w.xFree(uintptr(unsafe.Pointer(prop)))
		}
		return false
	}
	// First CARDINAL is state (32-bit).
	state := *(*uint32)(unsafe.Pointer(prop))
	if w.xFree != nil {
		w.xFree(uintptr(unsafe.Pointer(prop)))
	}
	return state == xWMStateIconic || state == xWMStateWithdrawn
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
		xResizeWindow   func(dpy uintptr, win uintptr, w, h uint32) int
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
	purego.RegisterLibFunc(&xResizeWindow, lib, "XResizeWindow")

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
	// PropertyChangeMask: GNOME Iconify updates WM_STATE without UnmapNotify.
	xSelectInput(dpy, win, xStructureNotify|xExposureMask|xPropertyChangeMask)

	atomName := append([]byte("WM_DELETE_WINDOW"), 0)
	delAtom := xInternAtom(dpy, &atomName[0], 0)
	if delAtom != 0 {
		prot := delAtom
		xSetWMProtocols(dpy, win, &prot, 1)
	}
	wmStateName := append([]byte("WM_STATE"), 0)
	wmStateAtom := xInternAtom(dpy, &wmStateName[0], 0)

	// Register size-hint setter for LockSize (timed soaks keep 800x600).
	var xSetWMNormalHints func(dpy uintptr, win uintptr, hints *byte) int
	purego.RegisterLibFunc(&xSetWMNormalHints, lib, "XSetWMNormalHints")
	var xGetWindowProperty func(
		dpy uintptr, win uintptr, property uintptr,
		offset, length int64, delete int, reqType uintptr,
		actualType *uintptr, actualFormat *int32,
		nitems *uint64, bytesAfter *uint64, prop **byte,
	) int
	purego.RegisterLibFunc(&xGetWindowProperty, lib, "XGetWindowProperty")
	var xFree func(ptr uintptr) int
	purego.RegisterLibFunc(&xFree, lib, "XFree")
	// _NET_WM_PID so external soak drivers can find this window by process id.
	var xChangeProperty func(dpy, w, prop, typ uintptr, format, mode int, data *byte, nelements int) int
	purego.RegisterLibFunc(&xChangeProperty, lib, "XChangeProperty")
	pidAtomName := append([]byte("_NET_WM_PID"), 0)
	pidAtom := xInternAtom(dpy, &pidAtomName[0], 0)
	cardAtomName := append([]byte("CARDINAL"), 0)
	cardAtom := xInternAtom(dpy, &cardAtomName[0], 0)
	if pidAtom != 0 && cardAtom != 0 && xChangeProperty != nil {
		pid := uint32(os.Getpid())
		// PropModeReplace = 0
		xChangeProperty(dpy, win, pidAtom, cardAtom, 32, 0, (*byte)(unsafe.Pointer(&pid)), 1)
	}

	xMapWindow(dpy, win)
	xFlush(dpy)
	time.Sleep(50 * time.Millisecond)

	xw := &x11Win{
		lib: lib, Display: dpy, Window: win, wmDeleteAtom: delAtom,
		wmStateAtom: wmStateAtom,
		xPending:    xPending, xNextEvent: xNextEvent, xFlush: xFlush,
		xDestroyWindow: xDestroyWindow, xCloseDisplay: xCloseDisplay,
		xInternAtom: xInternAtom, xSetWMProtocols: xSetWMProtocols,
		xSetWMNormalHints:  xSetWMNormalHints,
		xResizeWindow:      xResizeWindow,
		xGetWindowProperty: xGetWindowProperty,
		xFree:              xFree,
	}
	return xw, nil
}
