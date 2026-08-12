//go:build linux

package platform

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// x11Controller implements WindowController for the X11 backend via Xlib +
// EWMH. All calls are marshalled through purego (no CGO); large/async state
// (maximized/fullscreen/focus) is tracked optimistically and reconciled with
// WM events in x11Host.drainX.
type x11Controller struct {
	h *x11Host
}

// x11CtlLib binds the Xlib entry points the controller needs (resolved once
// per process; XInitThreads guarantees thread-safety).
type x11CtlLib struct {
	openOnce sync.Once
	lib      uintptr

	moveWindow        func(dpy uintptr, w uintptr, x, y int) int
	resizeWindow      func(dpy uintptr, w uintptr, wd, ht uint) int
	mapWindow         func(dpy uintptr, w uintptr) int
	unmapWindow       func(dpy uintptr, w uintptr) int
	iconifyWindow     func(dpy uintptr, w uintptr, screen int) int
	raiseWindow       func(dpy uintptr, w uintptr) int
	setInputFocus     func(dpy uintptr, w uintptr, revertTo int, tm uintptr) int
	sendEvent         func(dpy uintptr, w uintptr, propagate int, mask int64, ev *byte) int
	internAtom        func(dpy uintptr, name *byte, onlyIf int) uintptr
	changeProperty    func(dpy uintptr, w, prop, typ uintptr, format, mode int, data unsafe.Pointer, n int) int
	createFontCursor  func(dpy uintptr, shape uint) uintptr
	defineCursor      func(dpy uintptr, w uintptr, cur uintptr) int
	undefineCursor    func(dpy uintptr, w uintptr) int
	freeCursor        func(dpy uintptr, cur uintptr) int
	translateCoords   func(dpy uintptr, src, dst uintptr, sx, sy int, dx, dy *int, child *uintptr) int
	setWMNormalHints  func(dpy uintptr, w uintptr, hints *xSizeHints) int
	rootWindow        func(dpy uintptr, screen int) uintptr
	defaultScreen     func(dpy uintptr) int
	getWindowProperty func(dpy uintptr, w, prop uintptr, longOffset, longLength int64, del int, reqType uintptr,
		actualType *uintptr, actualFormat *int, nitems, bytesAfter *uint64, propReturn **byte) int
	freeData func(ptr unsafe.Pointer) int
}

var ctlLib x11CtlLib

func (l *x11CtlLib) open() *x11CtlLib {
	l.openOnce.Do(func() {
		lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		}
		if err != nil {
			return
		}
		l.lib = lib
		purego.RegisterLibFunc(&l.moveWindow, lib, "XMoveWindow")
		purego.RegisterLibFunc(&l.resizeWindow, lib, "XResizeWindow")
		purego.RegisterLibFunc(&l.mapWindow, lib, "XMapWindow")
		purego.RegisterLibFunc(&l.unmapWindow, lib, "XUnmapWindow")
		purego.RegisterLibFunc(&l.iconifyWindow, lib, "XIconifyWindow")
		purego.RegisterLibFunc(&l.raiseWindow, lib, "XRaiseWindow")
		purego.RegisterLibFunc(&l.setInputFocus, lib, "XSetInputFocus")
		purego.RegisterLibFunc(&l.sendEvent, lib, "XSendEvent")
		purego.RegisterLibFunc(&l.internAtom, lib, "XInternAtom")
		purego.RegisterLibFunc(&l.changeProperty, lib, "XChangeProperty")
		purego.RegisterLibFunc(&l.createFontCursor, lib, "XCreateFontCursor")
		purego.RegisterLibFunc(&l.defineCursor, lib, "XDefineCursor")
		purego.RegisterLibFunc(&l.undefineCursor, lib, "XUndefineCursor")
		purego.RegisterLibFunc(&l.freeCursor, lib, "XFreeCursor")
		purego.RegisterLibFunc(&l.translateCoords, lib, "XTranslateCoordinates")
		purego.RegisterLibFunc(&l.setWMNormalHints, lib, "XSetWMNormalHints")
		purego.RegisterLibFunc(&l.rootWindow, lib, "XRootWindow")
		purego.RegisterLibFunc(&l.defaultScreen, lib, "XDefaultScreen")
		purego.RegisterLibFunc(&l.getWindowProperty, lib, "XGetWindowProperty")
		purego.RegisterLibFunc(&l.freeData, lib, "XFree")
	})
	return l
}

func (l *x11CtlLib) ok() bool { return l != nil && l.lib != 0 }

// x11HasRunningWM reports whether an EWMH window manager is managing this
// display (probe _NET_SUPPORTING_WM_CHECK on the root window). A WM takes
// over map/unmap/visibility timing from the client; tests that assert
// event-driven convergence (MapNotify/UnmapNotify) are only deterministic
// on bare X servers (Xvfb/CI), so they degrade honestly when a WM runs.
func x11HasRunningWM(dpy uintptr) bool {
	lib := ctlLib.open()
	if !lib.ok() || dpy == 0 || lib.rootWindow == nil || lib.getWindowProperty == nil {
		return false
	}
	name := append([]byte("_NET_SUPPORTING_WM_CHECK"), 0)
	a := lib.internAtom(dpy, &name[0], 1)
	if a == 0 {
		return false
	}
	root := lib.rootWindow(dpy, lib.defaultScreen(dpy))
	var (
		actualType   uintptr
		actualFormat int
		nitems       uint64
		bytesAfter   uint64
		data         *byte
	)
	st := lib.getWindowProperty(dpy, root, a, 0, 1, 0, 0,
		&actualType, &actualFormat, &nitems, &bytesAfter, &data)
	// XGetWindowProperty returns Status: 0 (Success) on success, negative on
	// error — NOT a boolean like XQueryExtension.
	if st == 0 && data != nil {
		if lib.freeData != nil {
			lib.freeData(unsafe.Pointer(data))
		}
		return true
	}
	return false
}

func (c *x11Controller) lib() *x11CtlLib {
	return ctlLib.open()
}

func (c *x11Controller) st() *x11State {
	if c == nil || c.h == nil {
		return nil
	}
	return c.h.st
}

func (c *x11Controller) flush() {
	if s := c.st(); s != nil && s.flush != nil {
		s.flush()
	}
}

// Title returns the last-applied title (not re-queried — X11 has no cheap
// round-trip-getter; the value is tracked at SetTitle/Create).
func (c *x11Controller) Title() string {
	s := c.st()
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.title
}

func (c *x11Controller) SetTitle(t string) {
	s := c.st()
	if s == nil {
		return
	}
	s.mu.Lock()
	s.title = t
	s.mu.Unlock()
	lib := c.lib()
	if !lib.ok() || lib.changeProperty == nil {
		return
	}
	tb := append([]byte(t), 0)
	// Legacy WM_NAME (XStoreName equivalent via property) + UTF-8 _NET_WM_NAME.
	if s.atUTF8 != 0 && s.atNetName != 0 {
		lib.changeProperty(s.display, s.window, s.atNetName, s.atUTF8, 8, 0,
			unsafe.Pointer(&tb[0]), len(t))
	}
	c.flush()
}

func (c *x11Controller) Size() (int, int) {
	s := c.st()
	if s == nil {
		return 0, 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w, s.h
}

func (c *x11Controller) SetSize(w, h int) {
	s := c.st()
	lib := c.lib()
	if s == nil || !lib.ok() || lib.resizeWindow == nil {
		return
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	lib.resizeWindow(s.display, s.window, uint(w), uint(h))
	s.mu.Lock()
	s.hints.Width, s.hints.Height = int32(w), int32(h)
	s.mu.Unlock()
	c.flush()
}

// applyHints re-sends the WM_NORMAL_HINTS with the current constraint state.
func (c *x11Controller) applyHints() {
	s := c.st()
	lib := c.lib()
	if s == nil || !lib.ok() || lib.setWMNormalHints == nil {
		return
	}
	s.mu.Lock()
	h := s.hints
	s.mu.Unlock()
	lib.setWMNormalHints(s.display, s.window, &h)
	c.flush()
}

func (c *x11Controller) SetMinSize(w, h int) {
	s := c.st()
	if s == nil {
		return
	}
	s.mu.Lock()
	s.userMinW, s.userMinH = w, h
	s.hints.MinWidth, s.hints.MinHeight = int32(w), int32(h)
	if w > 0 && h > 0 {
		s.hints.Flags |= pMinSize
	} else {
		s.hints.Flags &^= pMinSize
	}
	s.mu.Unlock()
	c.applyHints()
}

func (c *x11Controller) SetMaxSize(w, h int) {
	s := c.st()
	if s == nil {
		return
	}
	s.mu.Lock()
	s.userMaxW, s.userMaxH = w, h
	s.hints.MaxWidth, s.hints.MaxHeight = int32(w), int32(h)
	if w > 0 && h > 0 {
		s.hints.Flags |= pMaxSize
	} else {
		s.hints.Flags &^= pMaxSize
	}
	s.mu.Unlock()
	c.applyHints()
}

// SetResizable locks (min==max=current size) or unlocks (restore user
// constraints) the window via size hints. This matches the winit/GTK
// "resizable=false → fixed size" contract on X11.
func (c *x11Controller) SetResizable(r bool) {
	s := c.st()
	if s == nil {
		return
	}
	s.mu.Lock()
	if r == s.resizable {
		s.mu.Unlock()
		return
	}
	s.resizable = r
	if r {
		s.hints.MinWidth, s.hints.MinHeight = int32(s.userMinW), int32(s.userMinH)
		s.hints.MaxWidth, s.hints.MaxHeight = int32(s.userMaxW), int32(s.userMaxH)
		if s.userMinW > 0 && s.userMinH > 0 {
			s.hints.Flags |= pMinSize
		} else {
			s.hints.Flags &^= pMinSize
		}
		if s.userMaxW > 0 && s.userMaxH > 0 {
			s.hints.Flags |= pMaxSize
		} else {
			s.hints.Flags &^= pMaxSize
		}
	} else {
		s.hints.MinWidth, s.hints.MinHeight = int32(s.w), int32(s.h)
		s.hints.MaxWidth, s.hints.MaxHeight = int32(s.w), int32(s.h)
		s.hints.Flags |= pMinSize | pMaxSize
	}
	s.mu.Unlock()
	c.applyHints()
}

func (c *x11Controller) IsResizable() bool {
	s := c.st()
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resizable
}

// SetDecorations toggles _MOTIF_WM_HINTS (supported by GNOME/Mutter and most
// EWMH WMs). decorations=0 hides title bar + frame on next WM configure.
func (c *x11Controller) SetDecorations(dec bool) error {
	s := c.st()
	lib := c.lib()
	if s == nil || !lib.ok() || lib.changeProperty == nil || s.atMotifHints == 0 {
		return ErrUnsupported
	}
	s.mu.Lock()
	s.decorated = dec
	s.mu.Unlock()
	const (
		mwmHintsDecorations = 1 << 1
		mwmDecorAll         = 1
	)
	flags := int32(mwmHintsDecorations)
	val := int32(0) // both decorations & functions off when frameless
	if dec {
		val = mwmDecorAll
	}
	// _MOTIF_WM_HINTS { long flags; long functions; long decorations; long input_mode; long status }
	var data [5]int32
	data[0] = flags
	data[1] = val
	data[2] = val
	lib.changeProperty(s.display, s.window, s.atMotifHints, s.atMotifHints, 32, 0,
		unsafe.Pointer(&data[0]), 5)
	c.flush()
	return nil
}

func (c *x11Controller) IsDecorated() bool {
	s := c.st()
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.decorated
}

// SetIgnoreCursorEvents is not implemented on X11 yet (requires XShape input
// region); maps to ErrUnsupported until landed.
func (c *x11Controller) SetIgnoreCursorEvents(ignore bool) error {
	return ErrUnsupported
}

func (c *x11Controller) Position() (int, int, bool) {
	s := c.st()
	lib := c.lib()
	if s == nil || !lib.ok() || lib.translateCoords == nil || s.root == 0 {
		return 0, 0, false
	}
	var dx, dy int
	var child uintptr
	if lib.translateCoords(s.display, s.window, s.root, 0, 0, &dx, &dy, &child) == 0 {
		return 0, 0, false
	}
	return dx, dy, true
}

func (c *x11Controller) SetPosition(x, y int) error {
	s := c.st()
	lib := c.lib()
	if s == nil || !lib.ok() || lib.moveWindow == nil {
		return ErrUnsupported
	}
	lib.moveWindow(s.display, s.window, x, y)
	c.flush()
	return nil
}

func (c *x11Controller) Minimize() {
	s := c.st()
	lib := c.lib()
	if s == nil || !lib.ok() || lib.iconifyWindow == nil {
		return
	}
	s.mu.Lock()
	s.minimized = true
	s.mu.Unlock()
	lib.iconifyWindow(s.display, s.window, 0)
	c.flush()
}

// IsMinimized queries the window state. X11 has no cheap property round-trip;
// the value is tracked optimistically (IconifyWindow, UnmapNotify via
// visibility events) and reconciled with ConfigureNotify.
func (c *x11Controller) IsMinimized() bool {
	s := c.st()
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.minimized
}

// RequestMove starts an interactive window move (used by frameless windows
// claiming the WM drag gesture). xdnd/EWMH: _NET_WM_MOVERESIZE with
// direction 8 (sizeMove) — the WM drives the drag from the current pointer.
func (c *x11Controller) RequestMove() error {
	return c.requestMoveResize(requestMoveDir)
}

// RequestResize starts an interactive edge resize of the given edge.
// _NET_WM_MOVERESIZE directions: 0=sizeTopLeft, 1=sizeTop, 2=sizeTopRight,
// 3=sizeRight, 4=sizeBottomRight, 5=sizeBottom, 6=sizeBottomLeft, 7=sizeLeft.
func (c *x11Controller) RequestResize(edge WindowEdge) error {
	var d int
	switch edge {
	case WindowEdgeTopLeft:
		d = 0
	case WindowEdgeTop:
		d = 1
	case WindowEdgeTopRight:
		d = 2
	case WindowEdgeRight:
		d = 3
	case WindowEdgeBottomRight:
		d = 4
	case WindowEdgeBottom:
		d = 5
	case WindowEdgeBottomLeft:
		d = 6
	case WindowEdgeLeft:
		d = 7
	default:
		return ErrUnsupported // no direction
	}
	return c.requestMoveResize(d)
}

func (c *x11Controller) requestMoveResize(direction int) error {
	s := c.st()
	lib := c.lib()
	if s == nil || !lib.ok() || lib.sendEvent == nil || s.root == 0 {
		return ErrUnsupported
	}
	// The WM needs the pointer position; X11 origin of the drag is
	// _NET_WM_MOVERESIZE data.l[0..1] = x_root,y_root. Without a
	// XQueryPointer binding we send the window origin; the WM then uses the
	// current pointer. This satisfies the "start drag" contract; precise
	// pointer anchoring follows when XQueryPointer lands.
	var ev [96]byte
	*(*int32)(unsafe.Pointer(&ev[0])) = int32(xClientMessage) // type
	*(*int32)(unsafe.Pointer(&ev[16])) = 1                    // send_event=true
	*(*uintptr)(unsafe.Pointer(&ev[32])) = s.window           // destination = our window
	// message_type = _NET_WM_MOVERESIZE
	moveresize := c.netAtom("_NET_WM_MOVERESIZE")
	if moveresize == 0 {
		return ErrUnsupported
	}
	*(*uintptr)(unsafe.Pointer(&ev[40])) = moveresize
	*(*int32)(unsafe.Pointer(&ev[48])) = 32 // format
	// data.l[0..4]: x_root, y_root, direction, button, 0
	*(*int64)(unsafe.Pointer(&ev[56])) = 0 // x_root (WM uses current pointer)
	*(*int64)(unsafe.Pointer(&ev[64])) = 0 // y_root
	*(*int64)(unsafe.Pointer(&ev[72])) = int64(direction)
	*(*int64)(unsafe.Pointer(&ev[80])) = 0 // button
	const (
		substructureRedirect = 1 << 20
		substructureNotify   = 1 << 19
	)
	lib.sendEvent(s.display, s.root, 0, substructureRedirect|substructureNotify, &ev[0])
	c.flush()
	return nil
}

// EWMH _NET_WM_MOVERESIZE directions.
const (
	requestMoveDir = 8 // _NET_WM_MOVERESIZE_MOVE
)

// netAtom resolves a single EWMH atom (cheap; used by RequestMove/Resize).
func (c *x11Controller) netAtom(name string) uintptr {
	s := c.st()
	lib := c.lib()
	if s == nil || !lib.ok() || lib.internAtom == nil {
		return 0
	}
	b := append([]byte(name), 0)
	return lib.internAtom(s.display, &b[0], 0)
}

// sendNetState posts an EWMH _NET_WM_STATE client message to the root window.
func (c *x11Controller) sendNetState(state1, state2 uintptr, action int) {
	s := c.st()
	lib := c.lib()
	if s == nil || !lib.ok() || lib.sendEvent == nil || s.atNetState == 0 {
		return
	}
	var ev [96]byte
	*(*int32)(unsafe.Pointer(&ev[0])) = int32(xClientMessage) // type
	*(*int32)(unsafe.Pointer(&ev[16])) = 1                    // send_event=true
	*(*uintptr)(unsafe.Pointer(&ev[32])) = s.root             // window
	*(*uintptr)(unsafe.Pointer(&ev[40])) = s.atNetState       // message_type
	*(*int32)(unsafe.Pointer(&ev[48])) = 32                   // format
	// data.l[0..2] (long[5] @56)
	*(*int64)(unsafe.Pointer(&ev[56])) = int64(action)
	*(*int64)(unsafe.Pointer(&ev[64])) = int64(state1)
	*(*int64)(unsafe.Pointer(&ev[72])) = int64(state2)
	const (
		substructureRedirect = 1 << 20
		substructureNotify   = 1 << 19
	)
	lib.sendEvent(s.display, s.root, 0, substructureRedirect|substructureNotify, &ev[0])
	c.flush()
}

func (c *x11Controller) Maximize() {
	s := c.st()
	if s == nil {
		return
	}
	s.mu.Lock()
	s.maximized = true
	s.mu.Unlock()
	c.sendNetState(s.atMaxV, s.atMaxH, ewmhStateAdd)
}

func (c *x11Controller) Unmaximize() {
	s := c.st()
	if s == nil {
		return
	}
	s.mu.Lock()
	s.maximized = false
	s.mu.Unlock()
	c.sendNetState(s.atMaxV, s.atMaxH, ewmhStateRemove)
}

func (c *x11Controller) IsMaximized() bool {
	s := c.st()
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maximized
}

func (c *x11Controller) SetFullscreen(fs bool) {
	s := c.st()
	if s == nil {
		return
	}
	action := ewmhStateRemove
	if fs {
		action = ewmhStateAdd
	}
	s.mu.Lock()
	s.fullscreen = fs
	s.mu.Unlock()
	c.sendNetState(s.atFull, 0, action)
}

func (c *x11Controller) IsFullscreen() bool {
	s := c.st()
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fullscreen
}

func (c *x11Controller) Show() error {
	s := c.st()
	lib := c.lib()
	if s == nil || !lib.ok() || lib.mapWindow == nil {
		return ErrUnsupported
	}
	lib.mapWindow(s.display, s.window)
	c.flush()
	return nil
}

func (c *x11Controller) Hide() error {
	s := c.st()
	lib := c.lib()
	if s == nil || !lib.ok() || lib.unmapWindow == nil {
		return ErrUnsupported
	}
	lib.unmapWindow(s.display, s.window)
	c.flush()
	return nil
}

func (c *x11Controller) IsVisible() bool {
	s := c.st()
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.visible
}

func (c *x11Controller) Focus() error {
	s := c.st()
	lib := c.lib()
	if s == nil || !lib.ok() || lib.setInputFocus == nil {
		return ErrUnsupported
	}
	const (
		revertToParent = 2
		currentTime    = 0
	)
	lib.setInputFocus(s.display, s.window, revertToParent, currentTime)
	c.flush()
	return nil
}

func (c *x11Controller) IsFocused() bool {
	s := c.st()
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.focused
}

func (c *x11Controller) SetAlwaysOnTop(on bool) error {
	s := c.st()
	if s == nil {
		return ErrUnsupported
	}
	action := ewmhStateRemove
	if on {
		action = ewmhStateAdd
	}
	c.sendNetState(s.atAbove, 0, action)
	return nil
}

// xCursorShapes maps cross-platform Cursor to X cursor-font glyph numbers
// (cursorfont.h; used by XCreateFontCursor).
func (c *x11Controller) cursorShape(cur Cursor) (uint, bool) {
	switch cur {
	case CursorText:
		return 152, true // XC_xterm
	case CursorPointer:
		return 60, true // XC_hand2
	case CursorCrosshair:
		return 34, true // XC_crosshair
	case CursorWait:
		return 150, true // XC_watch
	case CursorResizeH:
		return 108, true // XC_sb_h_double_arrow
	case CursorResizeV:
		return 116, true // XC_sb_v_double_arrow
	case CursorResizeNE:
		return 136, true // XC_top_right_corner
	case CursorResizeNW:
		return 134, true // XC_top_left_corner
	default:
		return 68, true // XC_left_ptr
	}
}

var x11CursorMu sync.Mutex // guards per-window cursor lifetime

func (c *x11Controller) SetCursor(cur Cursor) {
	s := c.st()
	lib := c.lib()
	if s == nil || !lib.ok() || lib.createFontCursor == nil {
		return
	}
	shape, _ := c.cursorShape(cur)
	x11CursorMu.Lock()
	old := s.cursor
	s.cursor = 0
	if shape == 68 {
		lib.undefineCursor(s.display, s.window)
	} else {
		curp := lib.createFontCursor(s.display, shape)
		if curp != 0 {
			lib.defineCursor(s.display, s.window, curp)
			s.cursor = curp
		}
	}
	x11CursorMu.Unlock()
	if old != 0 {
		lib.freeCursor(s.display, old)
	}
	c.flush()
}

// --- EWMH state action constants ---

const (
	ewmhStateRemove = 0
	ewmhStateAdd    = 1
	ewmhStateToggle = 2
)

var _ = errors.New // keep errors import (ErrUnsupported path)

// x11CtlHelpers guards the purego registration once per process.
func init() { _ = ctlLib.open() }
