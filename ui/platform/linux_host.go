//go:build linux && !nouiplatform

package platform

import (
	"fmt"
	"os"
	"strconv"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Linux X11 thin adapter (M0). Real present/GPU stays in the app/example host
// loop via NativeHandles; this package only owns window + input.
//
// IME: CapIME is NOT set (XIM/XIC not wired). See ime.go for the formal
// degradation contract. KeyPress is decoded with XLookupString for Latin /
// special keys → EventKey + EventText.

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

	// Zero-flash live resize (device_lost_redraw / Skia / Flutter).
	xNone             = 0
	xNorthWestGravity = 1
	xWhenMapped       = 1
	xCWBackPixmap     = 1 << 0
	xCWBitGravity     = 1 << 4
	xCWWinGravity     = 1 << 5
	xCWBackingStore   = 1 << 6
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

	xPending          func(dpy uintptr) int
	xNextEvent        func(dpy uintptr, ev *byte) int
	xFlush            func(dpy uintptr) int
	xDestroyWindow    func(dpy uintptr, win uintptr) int
	xCloseDisplay     func(dpy uintptr) int
	xStoreName        func(dpy uintptr, win uintptr, name *byte) int
	xLookupString     func(ev *byte, buf *byte, bytes int, keysym *uintptr, status *int) int
	xCreateFontCursor func(dpy uintptr, shape uint) uintptr
	xDefineCursor     func(dpy, win, cursor uintptr) int
	xFreeCursor       func(dpy, cursor uintptr) int

	cursors       map[CursorKind]uintptr
	lastCursor    CursorKind
	hasLastCursor bool

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
		// HiDPI: respect common desktop scale env (matches browser DPR for AA).
		opts.Scale = 1
		if v := os.Getenv("GDK_SCALE"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
				opts.Scale = f
			}
		} else if v := os.Getenv("QT_SCALE_FACTOR"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
				opts.Scale = f
			}
		} else if v := os.Getenv("GPUI_SCALE"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
				opts.Scale = f
			}
		}
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
		xOpenDisplay      func(name *byte) uintptr
		xCloseDisplay     func(dpy uintptr) int
		xDefaultScreen    func(dpy uintptr) int
		xRootWindow       func(dpy uintptr, screen int) uintptr
		xCreateSimple     func(dpy uintptr, parent uintptr, x, y int, width, height, borderWidth uint, border, background uint64) uintptr
		xMapWindow        func(dpy uintptr, win uintptr) int
		xFlush            func(dpy uintptr) int
		xDestroyWindow    func(dpy uintptr, win uintptr) int
		xStoreName        func(dpy uintptr, win uintptr, name *byte) int
		xSelectInput      func(dpy uintptr, win uintptr, mask int64) int
		xPending          func(dpy uintptr) int
		xNextEvent        func(dpy uintptr, ev *byte) int
		xInternAtom       func(dpy uintptr, name *byte, onlyIfExists int) uintptr
		xSetWMProtocols   func(dpy uintptr, win uintptr, protocols *uintptr, count int) int
		xLookupString     func(ev *byte, buf *byte, bytes int, keysym *uintptr, status *int) int
		xSetBgPixmap      func(dpy, win, pixmap uintptr) int
		xChangeAttr       func(dpy, win uintptr, mask uint64, attrs unsafe.Pointer) int
		xCreateFontCursor func(dpy uintptr, shape uint) uintptr
		xDefineCursor     func(dpy, win, cursor uintptr) int
		xFreeCursor       func(dpy, cursor uintptr) int
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
	purego.RegisterLibFunc(&xLookupString, lib, "XLookupString")
	purego.RegisterLibFunc(&xCreateFontCursor, lib, "XCreateFontCursor")
	purego.RegisterLibFunc(&xDefineCursor, lib, "XDefineCursor")
	purego.RegisterLibFunc(&xFreeCursor, lib, "XFreeCursor")
	purego.RegisterLibFunc(&xSetBgPixmap, lib, "XSetWindowBackgroundPixmap")
	purego.RegisterLibFunc(&xChangeAttr, lib, "XChangeWindowAttributes")

	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		_ = purego.Dlclose(lib)
		return nil, fmt.Errorf("platform/linux: XOpenDisplay failed (DISPLAY=%q)", os.Getenv("DISPLAY"))
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	// Background 0 + None pixmap: X must not fill new regions solid mid-drag
	// (main cause of continuous flash). Aligns with device_lost_redraw/x11.go.
	win := xCreateSimple(dpy, root, 80, 60, uint(opts.Width), uint(opts.Height), 1, 0, 0)
	if win == 0 {
		xCloseDisplay(dpy)
		_ = purego.Dlclose(lib)
		return nil, fmt.Errorf("platform/linux: XCreateSimpleWindow failed")
	}
	// Zero-flash live resize (Skia/Flutter): no bg fill, NW gravity, backing store.
	xSetBgPixmap(dpy, win, uintptr(xNone))
	attrs := make([]byte, 128)
	*(*int32)(unsafe.Pointer(&attrs[32])) = int32(xNorthWestGravity)
	*(*int32)(unsafe.Pointer(&attrs[36])) = int32(xNorthWestGravity)
	*(*int32)(unsafe.Pointer(&attrs[40])) = int32(xWhenMapped)
	xChangeAttr(dpy, win, uint64(xCWBackPixmap|xCWBitGravity|xCWWinGravity|xCWBackingStore), unsafe.Pointer(&attrs[0]))

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
		xStoreName: xStoreName, xLookupString: xLookupString,
		xCreateFontCursor: xCreateFontCursor, xDefineCursor: xDefineCursor, xFreeCursor: xFreeCursor,
		cursors: make(map[CursorKind]uintptr),
	}
	return h, nil
}

// Caps implements Host.
//
// CapIME is intentionally NOT set: XIM/XIC composition is not wired in this thin
// X11 adapter. Composition events must not be assumed on the true window.
// See ime.go and docs/UI_FRAMEWORK_MAP.md §12.1 C1 / §12.3 W4.
// Latin/special keys emit EventKey/EventText via XLookupString.
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

// SetCursor implements CursorHost (X11 font cursors).
func (h *LinuxHost) SetCursor(kind CursorKind) {
	if h == nil || h.closed || h.display == 0 || h.window == 0 {
		return
	}
	if h.hasLastCursor && h.lastCursor == kind {
		return
	}
	if h.xCreateFontCursor == nil || h.xDefineCursor == nil {
		return
	}
	if h.cursors == nil {
		h.cursors = make(map[CursorKind]uintptr)
	}
	cur, ok := h.cursors[kind]
	if !ok || cur == 0 {
		shape := uint(68) // XC_left_ptr
		switch kind {
		case CursorPointer:
			shape = 60 // XC_hand2
		case CursorText:
			shape = 152 // XC_xterm
		case CursorNotAllowed:
			shape = 88 // XC_pirate (approx) / XC_X_cursor=0 better: 24 XC_circle
			shape = 24 // XC_circle often used as forbidden
		case CursorWait:
			shape = 150 // XC_watch
		case CursorMove:
			shape = 52 // XC_fleur
		case CursorCrosshair:
			shape = 34 // XC_crosshair
		default:
			shape = 68
		}
		cur = h.xCreateFontCursor(h.display, shape)
		if cur == 0 {
			return
		}
		h.cursors[kind] = cur
	}
	h.xDefineCursor(h.display, h.window, cur)
	if h.xFlush != nil {
		h.xFlush(h.display)
	}
	h.lastCursor = kind
	h.hasLastCursor = true
}

// Flush flushes the X connection.
func (h *LinuxHost) Flush() {
	if h != nil && h.xFlush != nil && h.display != 0 {
		h.xFlush(h.display)
	}
}

// PumpEvents implements Host (non-blocking).
func (h *LinuxHost) PumpEvents() []Event {
	return h.WaitEvents(0)
}

// WaitEvents implements Host (gogpu-aligned demand loop).
// timeout < 0 blocks on XNextEvent; 0 is non-blocking; >0 waits up to timeout.
func (h *LinuxHost) WaitEvents(timeout time.Duration) []Event {
	if h == nil || h.closed {
		return nil
	}
	// Drain any already-pending X events / queue first.
	h.drainPending()
	if len(h.queue) > 0 {
		return h.takeQueue()
	}
	if timeout == 0 {
		return nil
	}
	if timeout < 0 {
		var raw [192]byte
		if h.xNextEvent != nil && h.display != 0 {
			h.xNextEvent(h.display, &raw[0])
			h.handleRaw(&raw)
		}
		h.drainPending()
		return h.takeQueue()
	}
	// Timed wait: slice-poll so animation frames (~16ms) stay responsive.
	deadline := time.Now().Add(timeout)
	for {
		h.drainPending()
		if len(h.queue) > 0 {
			return h.takeQueue()
		}
		if h.closed || !time.Now().Before(deadline) {
			break
		}
		// Short sleep; remaining time may be less than 1ms.
		left := time.Until(deadline)
		if left > 2*time.Millisecond {
			time.Sleep(1 * time.Millisecond)
		} else if left > 0 {
			time.Sleep(left)
		} else {
			break
		}
	}
	h.drainPending()
	return h.takeQueue()
}

func (h *LinuxHost) drainPending() {
	for h.xPending != nil && h.display != 0 && h.xPending(h.display) > 0 {
		var raw [192]byte
		h.xNextEvent(h.display, &raw[0])
		h.handleRaw(&raw)
	}
}

func (h *LinuxHost) takeQueue() []Event {
	if len(h.queue) == 0 {
		return nil
	}
	out := h.queue
	h.queue = nil
	return out
}

// WakeUp implements Host. X11 wait is connection-driven; no-op until self-pipe.
// Same-thread RequestRedraw already enqueues events for the next WaitEvents slice.
func (h *LinuxHost) WakeUp() {}

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
	case xKeyPress, xKeyRelease:
		h.handleKey(raw, typ == xKeyPress)
	case xClientMessage:
		msgType := *(*uintptr)(unsafe.Pointer(&raw[40]))
		data0 := *(*uintptr)(unsafe.Pointer(&raw[56]))
		if h.wmDelete != 0 && (data0 == h.wmDelete || msgType == h.wmDelete) {
			h.queue = append(h.queue, Event{Type: EventClose})
		}
	}
}

// handleKey decodes XKeyEvent via XLookupString into EventKey and optional EventText.
// This is Latin/special-key path only — not CapIME composition (see Caps / ime.go).
func (h *LinuxHost) handleKey(raw *[192]byte, down bool) {
	var keysym uintptr
	var buf [32]byte
	n := 0
	if h.xLookupString != nil {
		n = h.xLookupString(&raw[0], &buf[0], len(buf), &keysym, nil)
	}
	key := keysymName(keysym)
	text := ""
	if n > 0 {
		s := string(buf[:n])
		if isPrintableText(s) {
			text = s
			if key == "" {
				key = s
			}
		}
	}
	if key == "" && text == "" {
		return
	}
	h.queue = append(h.queue, Event{Type: EventKey, Key: key, Text: text, Down: down})
	// Committed printable on key down (no CapIME): EventText for EditableText.
	if down && text != "" && !isControlKey(key) {
		h.queue = append(h.queue, Event{Type: EventText, Text: text})
	}
}

func isPrintableText(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func isControlKey(key string) bool {
	switch key {
	case "BackSpace", "Backspace", "Delete", "Tab", "Return", "Enter",
		"Left", "Right", "Up", "Down", "Home", "End", "Escape", "Esc":
		return true
	}
	return false
}

// keysymName maps common X11 keysyms to core-friendly key names.
func keysymName(keysym uintptr) string {
	switch keysym {
	case 0xff08: // XK_BackSpace
		return "Backspace"
	case 0xffff: // XK_Delete
		return "Delete"
	case 0xff09: // XK_Tab
		return "Tab"
	case 0xff0d: // XK_Return
		return "Enter"
	case 0xff1b: // XK_Escape
		return "Escape"
	case 0xff51: // XK_Left
		return "Left"
	case 0xff52: // XK_Up
		return "Up"
	case 0xff53: // XK_Right
		return "Right"
	case 0xff54: // XK_Down
		return "Down"
	case 0xff50: // XK_Home
		return "Home"
	case 0xff57: // XK_End
		return "End"
	case 0x20: // space
		return " "
	}
	if keysym >= 0x20 && keysym <= 0x7e {
		return string(rune(keysym))
	}
	return ""
}

// RequestRedraw implements Host (X11 expose is async; we just enqueue).
func (h *LinuxHost) RequestRedraw() {
	if h == nil || h.closed {
		return
	}
	h.queue = append(h.queue, Event{Type: EventRedraw})
	h.WakeUp()
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
