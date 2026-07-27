//go:build linux && !nouiplatform

package platform

import (
	"fmt"
	"os"
	"strconv"
	"sync"
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
	xKeyPress        = 2
	xKeyRelease      = 3
	xButtonPress     = 4
	xButtonRelease   = 5
	xMotionNotify    = 6
	xFocusIn         = 9
	xFocusOut        = 10
	xExpose          = 12
	xConfigureNotify = 22
	xClientMessage   = 33

	xKeyPressMask        = int64(1 << 0)
	xKeyReleaseMask      = int64(1 << 1)
	xButtonPressMask     = int64(1 << 2)
	xButtonReleaseMask   = int64(1 << 3)
	xPointerMotionMask   = int64(1 << 6)
	xExposureMask        = int64(1 << 15)
	xStructureNotifyMask = int64(1 << 17)
	xFocusChangeMask     = int64(1 << 21)

	// Zero-flash live resize (device_lost_redraw / Skia / Flutter).
	// Modifier bits: see modifiers.go ParseModifierState (X.h Shift/Ctrl/Mod1/Mod4).
	xNone             = 0
	xNorthWestGravity = 1
	xWhenMapped       = 1
	xCWBackPixmap     = 1 << 0
	xCWBitGravity     = 1 << 4
	xCWWinGravity     = 1 << 5
	xCWBackingStore   = 1 << 6
)

// LinuxHost is a minimal X11 window Host.
//
// When Wayland is detected, NewLinuxHost returns a LinuxHost wrapper that
// delegates to a WaylandHost (wlBackend). The struct is shared for both
// backends so callers always use the *LinuxHost type.
//
// Threading: the UI app runs WaitEvents/XNextEvent on the main goroutine and
// may call Flush/SetCursor from the render thread after Present. Xlib requires
// XInitThreads before XOpenDisplay; xmu serializes all Xlib entry points.
type LinuxHost struct {
	lib     uintptr
	display uintptr
	window  uintptr
	screen  int
	width   int
	height  int
	scale   float64
	title   string

	xmu sync.Mutex // all Xlib calls (display + window)

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

	// System text clipboard (xclip/xsel + memory fallback). CapClipboard always set.
	clip Clipboard

	// wlBackend is set when NewLinuxHost selected Wayland.
	// When non-nil, the host delegates all lifecycle/event methods to this backend.
	wlBackend *WaylandHost

	// backend is the selected display backend (X11 or Wayland).
	backend DisplayBackend
}

// LinuxOptions configures NewLinuxHost.
type LinuxOptions struct {
	Width, Height int
	Title         string
	Scale         float64
	// Backend overrides detection when not DisplayAuto.
	// Also honored via GPUI_DISPLAY when left at DisplayAuto.
	Backend DisplayBackend
}

// NewLinuxHost opens a native Linux window (X11 or Wayland).
//
// Selection order matches the exhost reference:
//
//	GPUI_DISPLAY / opts.Backend → try preferred → fall back to the other when available
//
// Close order for callers: stop GPU present before Host.Close.
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

	want := opts.Backend
	if want == DisplayAuto {
		want = DetectDisplayBackend()
	}

	var errs []error
	tryX11 := HasX11Display() && (want == DisplayX11 || want == DisplayAuto || want == DisplayWayland)
	tryWayland := HasWaylandDisplay() && (want == DisplayWayland || want == DisplayAuto)
	if want == DisplayX11 {
		tryWayland = false
	}

	// Prefer X11 first for normal WM decorations when requested/detected as x11.
	if tryX11 && want != DisplayWayland {
		h, err := newX11Host(opts)
		if err == nil {
			return h, nil
		}
		errs = append(errs, fmt.Errorf("x11: %w", err))
	}
	if tryWayland {
		h, err := newWaylandLinuxHost(opts)
		if err == nil {
			return h, nil
		}
		errs = append(errs, fmt.Errorf("wayland: %w", err))
	}
	// Wayland-forced but failed: try X11 if available and not already tried.
	if tryX11 && want == DisplayWayland {
		h, err := newX11Host(opts)
		if err == nil {
			return h, nil
		}
		errs = append(errs, fmt.Errorf("x11: %w", err))
	}
	if len(errs) == 0 {
		return nil, fmt.Errorf("platform/linux: no display (set WAYLAND_DISPLAY and/or DISPLAY, or GPUI_DISPLAY=x11|wayland)")
	}
	return nil, fmt.Errorf("platform/linux: open failed: %v", errs)
}

// newWaylandLinuxHost wraps WaylandHost as *LinuxHost so callers keep one type.
func newWaylandLinuxHost(opts LinuxOptions) (*LinuxHost, error) {
	wlHost, err := NewWaylandHost(WaylandOptions{
		Width:  opts.Width,
		Height: opts.Height,
		Title:  opts.Title,
		Scale:  opts.Scale,
	})
	if err != nil {
		return nil, err
	}
	return &LinuxHost{
		display:   wlHost.Display(),
		window:    wlHost.Window(),
		width:     opts.Width,
		height:    opts.Height,
		scale:     opts.Scale,
		title:     opts.Title,
		wlBackend: wlHost,
		backend:   DisplayWayland,
		clip:      NewSystemClipboard(),
	}, nil
}

// newX11Host opens a simple X11 window. Requires DISPLAY.
func newX11Host(opts LinuxOptions) (*LinuxHost, error) {
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

	// XSizeHints / XClassHint subsets (Xutil.h) for normal decorated top-level.
	type xSizeHints struct {
		Flags                                          int64
		X, Y                                           int32
		Width, Height                                  int32
		MinWidth, MinHeight                            int32
		MaxWidth, MaxHeight                            int32
		WidthInc, HeightInc                            int32
		MinAspectN, MinAspectD, MaxAspectN, MaxAspectD int32
		BaseWidth, BaseHeight                          int32
		WinGravity                                     int32
	}
	type xClassHint struct {
		ResName, ResClass *byte
	}

	var (
		xInitThreads      func() int
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
		xSetWMNormalHints func(dpy uintptr, win uintptr, hints *xSizeHints) int
		xSetClassHint     func(dpy uintptr, win uintptr, hint *xClassHint) int
		xChangeProperty   func(dpy uintptr, win uintptr, property, typ uintptr, format int, mode int, data *byte, nelements int) int
	)
	// MUST be first Xlib call: render thread may XFlush while main drains events.
	purego.RegisterLibFunc(&xInitThreads, lib, "XInitThreads")
	if xInitThreads != nil {
		_ = xInitThreads()
	}
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
	purego.RegisterLibFunc(&xSetWMNormalHints, lib, "XSetWMNormalHints")
	purego.RegisterLibFunc(&xSetClassHint, lib, "XSetClassHint")
	purego.RegisterLibFunc(&xChangeProperty, lib, "XChangeProperty")

	dpy := xOpenDisplay(nil)
	if dpy == 0 {
		_ = purego.Dlclose(lib)
		return nil, fmt.Errorf("platform/linux: XOpenDisplay failed (DISPLAY=%q)", os.Getenv("DISPLAY"))
	}
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	// borderWidth=0: border drawn by WM title bar, not a client black rim.
	// Background None pixmap: X must not fill new regions solid mid-drag.
	win := xCreateSimple(dpy, root, 80, 60, uint(opts.Width), uint(opts.Height), 0, 0, 0)
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

	// UTF-8 title for modern WMs (GNOME/KDE title bar text).
	utf8AtomName := append([]byte("UTF8_STRING"), 0)
	netName := append([]byte("_NET_WM_NAME"), 0)
	atomUTF8 := xInternAtom(dpy, &utf8AtomName[0], 0)
	atomNetName := xInternAtom(dpy, &netName[0], 0)
	if atomUTF8 != 0 && atomNetName != 0 {
		xChangeProperty(dpy, win, atomNetName, atomUTF8, 8, 0, &name[0], len(opts.Title))
	}

	// WM_CLASS — required by many WMs for normal frame / taskbar grouping.
	resName := append([]byte("gpui"), 0)
	resClass := append([]byte("gpui"), 0)
	ch := xClassHint{ResName: &resName[0], ResClass: &resClass[0]}
	xSetClassHint(dpy, win, &ch)

	// Size hints so the WM maps us as a normal resizable top-level.
	const (
		pSize     = 1 << 3
		pMinSize  = 1 << 4
		pBaseSize = 1 << 8
	)
	hints := xSizeHints{
		Flags:      pSize | pMinSize | pBaseSize,
		Width:      int32(opts.Width),
		Height:     int32(opts.Height),
		MinWidth:   160,
		MinHeight:  120,
		BaseWidth:  160,
		BaseHeight: 120,
	}
	xSetWMNormalHints(dpy, win, &hints)

	// _NET_WM_WINDOW_TYPE_NORMAL — ask for standard decorated frame.
	typeName := append([]byte("_NET_WM_WINDOW_TYPE"), 0)
	normalName := append([]byte("_NET_WM_WINDOW_TYPE_NORMAL"), 0)
	atomType := xInternAtom(dpy, &typeName[0], 0)
	atomNormal := xInternAtom(dpy, &normalName[0], 0)
	atomAtomName := append([]byte("ATOM"), 0)
	atomAtom := xInternAtom(dpy, &atomAtomName[0], 0)
	if atomType != 0 && atomNormal != 0 && atomAtom != 0 {
		var v uint64 = uint64(atomNormal)
		xChangeProperty(dpy, win, atomType, atomAtom, 32, 0, (*byte)(unsafe.Pointer(&v)), 1)
	}

	mask := xStructureNotifyMask | xExposureMask |
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

	// Drain map noise so first WaitEvents sees real input/resize.
	var buf [256]byte
	for xPending(dpy) > 0 {
		xNextEvent(dpy, &buf[0])
	}

	h := &LinuxHost{
		lib: lib, display: dpy, window: win, screen: screen,
		width: opts.Width, height: opts.Height, scale: opts.Scale, title: opts.Title,
		wmDelete: delAtom,
		xPending: xPending, xNextEvent: xNextEvent, xFlush: xFlush,
		xDestroyWindow: xDestroyWindow, xCloseDisplay: xCloseDisplay,
		xStoreName: xStoreName, xLookupString: xLookupString,
		xCreateFontCursor: xCreateFontCursor, xDefineCursor: xDefineCursor, xFreeCursor: xFreeCursor,
		cursors: make(map[CursorKind]uintptr),
		clip:    NewSystemClipboard(),
		backend: DisplayX11,
	}
	return h, nil
}

// Caps implements Host.
//
// CapIME is intentionally NOT set: XIM/XIC composition is not wired in this thin
// X11 adapter. Composition events must not be assumed on the true window.
// See ime.go and docs/UI_FRAMEWORK_MAP.md §12.1 C1 / §12.3 W4.
// Latin/special keys emit EventKey/EventText via XLookupString.
// CapClipboard is set; backend is OS clipboard (xclip/xsel) + memory fallback.
func (h *LinuxHost) Caps() Caps {
	if h.wlBackend != nil {
		return h.wlBackend.Caps()
	}
	return CapWindow | CapPointer | CapKeyboard | CapTextInput | CapPresent | CapSurfaceLifecycle | CapCursor | CapClipboard
}

// Clipboard implements ClipboardProvider.
func (h *LinuxHost) Clipboard() Clipboard {
	if h == nil {
		return nil
	}
	if h.clip == nil {
		h.clip = NewSystemClipboard()
	}
	return h.clip
}

// Size implements Host.
func (h *LinuxHost) Size() (int, int) {
	if h == nil {
		return 0, 0
	}
	if h.wlBackend != nil {
		return h.wlBackend.Size()
	}
	h.xmu.Lock()
	defer h.xmu.Unlock()
	return h.width, h.height
}

// ScaleFactor implements Host.
func (h *LinuxHost) ScaleFactor() float64 {
	if h.wlBackend != nil {
		return h.wlBackend.ScaleFactor()
	}
	if h.scale <= 0 {
		return 1
	}
	return h.scale
}

// Display implements NativeHandles.
func (h *LinuxHost) Display() uintptr {
	if h == nil {
		return 0
	}
	if h.wlBackend != nil {
		return h.wlBackend.Display()
	}
	return h.display
}

// Window implements NativeHandles.
func (h *LinuxHost) Window() uintptr {
	if h == nil {
		return 0
	}
	if h.wlBackend != nil {
		return h.wlBackend.Window()
	}
	return h.window
}

// Backend reports the selected display backend (X11 or Wayland).
func (h *LinuxHost) Backend() DisplayBackend {
	if h == nil {
		return DisplayAuto
	}
	if h.backend != DisplayAuto {
		return h.backend
	}
	if h.wlBackend != nil {
		return DisplayWayland
	}
	return DisplayX11
}

// NativeSurface implements SurfaceProvider for wgpu CreateSurface.
func (h *LinuxHost) NativeSurface() NativeSurface {
	if h == nil {
		return NativeSurface{}
	}
	if h.wlBackend != nil {
		return h.wlBackend.NativeSurface()
	}
	return NativeSurface{
		Kind:    PlatformX11,
		Display: h.display,
		Window:  h.window,
	}
}

// Screen returns the X11 screen index (0 on Wayland).
func (h *LinuxHost) Screen() int {
	if h == nil || h.wlBackend != nil {
		return 0
	}
	return h.screen
}

// SetCursor implements CursorHost (X11 font cursors).
func (h *LinuxHost) SetCursor(kind CursorKind) {
	if h == nil {
		return
	}
	h.xmu.Lock()
	defer h.xmu.Unlock()
	if h.closed || h.display == 0 || h.window == 0 {
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
// Safe to call from the render thread after Present (XInitThreads + xmu).
func (h *LinuxHost) Flush() {
	if h == nil {
		return
	}
	if h.wlBackend != nil {
		return // Wayland host handles flush internally
	}
	if h.xFlush == nil {
		return
	}
	h.xmu.Lock()
	defer h.xmu.Unlock()
	if h.closed || h.display == 0 {
		return
	}
	h.xFlush(h.display)
}

// PumpEvents implements Host (non-blocking).
func (h *LinuxHost) PumpEvents() []Event {
	if h.wlBackend != nil {
		return h.wlBackend.PumpEvents()
	}
	return h.WaitEvents(0)
}

// WaitEvents implements Host (gogpu-aligned demand loop).
// timeout < 0 blocks on XNextEvent; 0 is non-blocking; >0 waits up to timeout.
//
// All Xlib entry points run under xmu (with XInitThreads) so render-thread
// Flush/SetCursor cannot race the event pump (resize storms used to abort in xcb).
func (h *LinuxHost) WaitEvents(timeout time.Duration) []Event {
	if h == nil {
		return nil
	}
	if h.wlBackend != nil {
		return h.wlBackend.WaitEvents(timeout)
	}
	h.xmu.Lock()
	if h.closed {
		h.xmu.Unlock()
		return nil
	}
	// Drain any already-pending X events / queue first.
	h.drainPendingLocked()
	if len(h.queue) > 0 {
		out := h.takeQueueLocked()
		h.xmu.Unlock()
		return out
	}
	if timeout == 0 {
		h.xmu.Unlock()
		return nil
	}
	if timeout < 0 {
		var raw [192]byte
		if h.xNextEvent != nil && h.display != 0 {
			// Blocking wait while holding xmu: Present/Flush only runs after
			// WaitEvents returns (main thread serializes the app loop).
			h.xNextEvent(h.display, &raw[0])
			h.handleRaw(&raw)
		}
		h.drainPendingLocked()
		out := h.takeQueueLocked()
		h.xmu.Unlock()
		return out
	}
	// Timed wait: release lock during sleep so Flush from a concurrent present
	// hop cannot wedge if the architecture ever overlaps (defensive).
	deadline := time.Now().Add(timeout)
	for {
		h.drainPendingLocked()
		if len(h.queue) > 0 {
			out := h.takeQueueLocked()
			h.xmu.Unlock()
			return out
		}
		closed := h.closed
		h.xmu.Unlock()
		if closed || !time.Now().Before(deadline) {
			h.xmu.Lock()
			break
		}
		left := time.Until(deadline)
		if left <= 0 {
			h.xmu.Lock()
			break
		}
		// One sleep for the remainder (cap 16ms). Avoid 1ms busy slices that
		// keep ANIMATING mode hot even when a single Spin only paints occasionally.
		chunk := left
		if chunk > 16*time.Millisecond {
			chunk = 16 * time.Millisecond
		}
		time.Sleep(chunk)
		h.xmu.Lock()
		if h.closed {
			break
		}
	}
	h.drainPendingLocked()
	out := h.takeQueueLocked()
	h.xmu.Unlock()
	return out
}

// drainPendingLocked requires h.xmu held.
func (h *LinuxHost) drainPendingLocked() {
	for h.xPending != nil && h.display != 0 && !h.closed && h.xPending(h.display) > 0 {
		var raw [192]byte
		h.xNextEvent(h.display, &raw[0])
		h.handleRaw(&raw)
	}
}

// takeQueueLocked requires h.xmu held.
func (h *LinuxHost) takeQueueLocked() []Event {
	if len(h.queue) == 0 {
		return nil
	}
	out := h.queue
	h.queue = nil
	return out
}

// WakeUp implements Host.
func (h *LinuxHost) WakeUp() {
	if h.wlBackend != nil {
		h.wlBackend.WakeUp()
		return
	}
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
		// XButtonEvent LP64: x@64 y@68 state@80 button@84
		x := float64(*(*int32)(unsafe.Pointer(&raw[64])))
		y := float64(*(*int32)(unsafe.Pointer(&raw[68])))
		state := *(*uint32)(unsafe.Pointer(&raw[80]))
		btnN := int(*(*uint32)(unsafe.Pointer(&raw[84])))
		sh, ctrl, alt, meta := modsFromXState(state)
		// Classic X11 mouse wheel: Button4/5 vertical, Button6/7 horizontal.
		// Only press generates a step (release is ignored).
		if btnN >= 4 && btnN <= 7 {
			if typ == xButtonPress {
				const step = 48.0 // ~3 lines at 16px
				dx, dy := 0.0, 0.0
				switch btnN {
				case 4: // up
					dy = -step
				case 5: // down
					dy = step
				case 6: // left
					dx = -step
				case 7: // right
					dx = step
				}
				h.queue = append(h.queue, Event{
					Type: EventScroll, X: x, Y: y, ScrollDX: dx, ScrollDY: dy,
					Shift: sh, Ctrl: ctrl, Alt: alt, Meta: meta,
				})
			}
			break
		}
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
		h.queue = append(h.queue, Event{
			Type: EventPointer, Pointer: kind, X: x, Y: y, Button: btn,
			Shift: sh, Ctrl: ctrl, Alt: alt, Meta: meta,
		})
	case xMotionNotify:
		// XMotionEvent LP64: x@64 y@68 state@80
		x := float64(*(*int32)(unsafe.Pointer(&raw[64])))
		y := float64(*(*int32)(unsafe.Pointer(&raw[68])))
		state := *(*uint32)(unsafe.Pointer(&raw[80]))
		sh, ctrl, alt, meta := modsFromXState(state)
		h.queue = append(h.queue, Event{
			Type: EventPointer, Pointer: PointerMove, X: x, Y: y,
			Shift: sh, Ctrl: ctrl, Alt: alt, Meta: meta,
		})
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
// Modifiers are taken from XKeyEvent.state (Shift/Ctrl/Alt/Meta).
func (h *LinuxHost) handleKey(raw *[192]byte, down bool) {
	// XKeyEvent LP64: state@80 keycode@84 (same prefix as XButtonEvent)
	state := *(*uint32)(unsafe.Pointer(&raw[80]))
	sh, ctrl, alt, meta := modsFromXState(state)

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
	h.queue = append(h.queue, Event{
		Type: EventKey, Key: key, Text: text, Down: down,
		Shift: sh, Ctrl: ctrl, Alt: alt, Meta: meta,
	})
	// Committed printable on key down (no CapIME): EventText for EditableText.
	if down && text != "" && !isControlKey(key) {
		h.queue = append(h.queue, Event{Type: EventText, Text: text})
	}
}

// modsFromXState maps X11 state mask bits to unified Event modifiers.
func modsFromXState(state uint32) (shift, ctrl, alt, meta bool) {
	return ParseModifierState(state)
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
	if h == nil {
		return
	}
	if h.wlBackend != nil {
		h.wlBackend.RequestRedraw()
		return
	}
	h.xmu.Lock()
	defer h.xmu.Unlock()
	if h.closed {
		return
	}
	h.queue = append(h.queue, Event{Type: EventRedraw})
	// WakeUp is a no-op on X11; next WaitEvents drain will see the queue.
}

// Close implements Host.
func (h *LinuxHost) Close() error {
	if h == nil {
		return nil
	}
	if h.wlBackend != nil {
		return h.wlBackend.Close()
	}
	h.xmu.Lock()
	defer h.xmu.Unlock()
	if h.closed {
		return nil
	}
	h.closed = true
	if h.xFreeCursor != nil {
		for _, cur := range h.cursors {
			if cur != 0 {
				h.xFreeCursor(h.display, cur)
			}
		}
		h.cursors = nil
	}
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
	_ Host            = (*LinuxHost)(nil)
	_ NativeHandles   = (*LinuxHost)(nil)
	_ SurfaceProvider = (*LinuxHost)(nil)
	_ BackendProvider = (*LinuxHost)(nil)
)
