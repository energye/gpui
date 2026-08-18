//go:build linux

package platform

import (
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

// x11Backend implements Backend for Xlib (Display* + Window). It is split
// into three concerns: Create/Adopt (window lifecycle), x11Host (event pump),
// and capability probing (IME nil for now; XIM lands with the IME milestone).
type x11Backend struct{}

func init() { Register(PlatformX11, &x11Backend{}) }

func (b *x11Backend) Kind() PlatformKind { return PlatformX11 }

// Create opens an X11 window and returns it wired to an event pump.
func (b *x11Backend) Create(opts Options) (*Window, error) {
	return x11Create(opts)
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
		display:         ns.Display,
		window:          ns.Window,
		w:               640,
		h:               480,
		scale:           1,
		keycodeToKeysym: x11KeycodeToKeysym(lib),
	}
	if w, h, ok := x11GetGeometry(st); ok {
		st.w, st.h = w, h
	}
	h := &x11Host{st: st, lib: lib}
	// Atoms are resolved lazily by the controller (resolveAtoms on demand);
	// root stays 0 here → EWMH ops (RequestMove/state toggles) return
	// ErrUnsupported for adopted windows until atoms are probed.
	return newWindow(h, PlatformX11, nil, nil, &x11Controller{h: h}, h.destroy), nil
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
		xKeyPressMask | xKeyReleaseMask | xFocusChangeMask |
		xVisibilityChangeMask | xEnterWindowMask | xLeaveWindowMask
)

// XSizeHints flags (Xutil.h PMinSize/PMaxSize/PBaseSize/PSize).
const (
	pSize     = 1 << 3
	pMinSize  = 1 << 4
	pMaxSize  = 1 << 5
	pBaseSize = 1 << 8
)

// X event field offsets (linux amd64 Xlib layout — matches exhost verified).
const (
	xevTypeOff        = 0
	xevXOff           = 48 // XConfigureEvent x
	xevYOff           = 52 // XConfigureEvent y
	xevWidthOff       = 56 // XConfigureEvent
	xevHeightOff      = 60
	xevClientData0Off = 56 // XClientMessageEvent.data.l[0]
	xevPointerXOff    = 64
	xevPointerYOff    = 68
	xevButtonOff      = 84 // button (press/release) or keycode (key)
	xevKeycodeOff     = 84
	xevStateOff       = 88 // XVisibilityEvent.state
)

// X event codes + mask bits (X.h).
const (
	xFocusChangeMask      = 1 << 21
	xVisibilityChangeMask = 1 << 16
	xEnterWindowMask      = 1 << 4
	xLeaveWindowMask      = 1 << 5
	xFocusIn              = 9
	xFocusOut             = 10
	xVisibilityNotify     = 15
	xEnterNotify          = 7
	xLeaveNotify          = 8
	xMapNotify            = 19
	xUnmapNotify          = 18
	xReparentNotify       = 21
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
	keycodeToKeysym func(dpy uintptr, keycode uint, index int) uintptr
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
func x11Create(opts Options) (*Window, error) {
	w, h, title := opts.Width, opts.Height, opts.Title
	if w < 1 {
		w = 640
	}
	if h < 1 {
		h = 480
	}
	if !HasX11Display() {
		return nil, fmt.Errorf("x11: DISPLAY not set")
	}
	lib, err := x11OpenLib()
	if err != nil {
		return nil, err
	}
	var (
		xInitThreads      func() int
		xOpenDisplay      func(name *byte) uintptr
		xDefaultScreen    func(dpy uintptr) int
		xRootWindow       func(dpy uintptr, screen int) uintptr
		xCreateSimple     func(dpy uintptr, parent uintptr, x, y int, width, height, borderWidth uint, border, background uint64) uintptr
		xSetBgPixmap      func(dpy uintptr, win uintptr, pixmap uintptr) int
		xChangeAttr       func(dpy uintptr, win uintptr, valueMask uint64, attrs unsafe.Pointer) int
		xMapWindow        func(dpy uintptr, win uintptr) int
		xFlush            func(dpy uintptr) int
		xDestroyWindow    func(dpy uintptr, win uintptr) int
		xStoreName        func(dpy uintptr, win uintptr, name *byte) int
		xSelectInput      func(dpy uintptr, win uintptr, mask int64) int
		xPending          func(dpy uintptr) int
		xNextEvent        func(dpy uintptr, ev *byte) int
		xInternAtom       func(dpy uintptr, name *byte, onlyIfExists int) uintptr
		xSetWMProtocols   func(dpy uintptr, win uintptr, protocols *uintptr, count int) int
		xSetWMNormalHints func(dpy uintptr, win uintptr, hints *xSizeHints) int
		xSetClassHint     func(dpy uintptr, win uintptr, hint *xClassHint) int
		xChangeProperty   func(dpy uintptr, win uintptr, property, typ uintptr, format int, mode int, data *byte, nelements int) int
		xConnectionNumber func(dpy uintptr) int
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
	purego.RegisterLibFunc(&xConnectionNumber, lib.lib, "XConnectionNumber")
	xConnectionNumberFn = xConnectionNumber

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

	minW, minH := opts.MinWidth, opts.MinHeight
	if minW < 1 {
		minW = 160
	}
	if minH < 1 {
		minH = 120
	}
	maxW, maxH := opts.MaxWidth, opts.MaxHeight
	flags := int64(pSize | pMinSize | pBaseSize)
	hints := xSizeHints{
		Flags:      flags,
		Width:      int32(w),
		Height:     int32(h),
		MinWidth:   int32(minW),
		MinHeight:  int32(minH),
		BaseWidth:  int32(minW),
		BaseHeight: int32(minH),
	}
	if maxW > 0 && maxH > 0 {
		hints.Flags |= pMaxSize
		hints.MaxWidth, hints.MaxHeight = int32(maxW), int32(maxH)
	}
	// Resizable=false locks the window to its initial size (min==max).
	if !opts.Resizable {
		if minW != w || minH != h {
			hints.Flags |= pMaxSize
			hints.MaxWidth, hints.MaxHeight = int32(w), int32(h)
		}
	}
	xSetWMNormalHints(dpy, win, &hints)
	xSelectInput(dpy, win, xEventMask)

	delName := append([]byte("WM_DELETE_WINDOW"), 0)
	if wmDelete := xInternAtom(dpy, &delName[0], 0); wmDelete != 0 {
		protos := []uintptr{wmDelete}
		// _NET_WM_SYNC_REQUEST: interactive resize sync. Without it the
		// compositor stretches stale content while the mouse is held during
		// a resize drag and live frames are hidden until release.
		syncName := append([]byte("_NET_WM_SYNC_REQUEST"), 0)
		if atSync := xInternAtom(dpy, &syncName[0], 0); atSync != 0 {
			protos = append(protos, atSync)
			// Simple counter (mode 0): 64-bit value as two 32-bit words,
			// initially 0. Every painted frame after a sync request advances
			// it, which the compositor samples to stop stretching.
			counterName := append([]byte("_NET_WM_SYNC_REQUEST_COUNTER"), 0)
			if atCounter := xInternAtom(dpy, &counterName[0], 0); atCounter != 0 {
				cardName := append([]byte("CARDINAL"), 0)
				atCard := xInternAtom(dpy, &cardName[0], 0)
				var zero [2]uint32
				xChangeProperty(dpy, win, atCounter, atCard, 32, 0, (*byte)(unsafe.Pointer(&zero[0])), 2)
			}
		}
		xSetWMProtocols(dpy, win, &protos[0], len(protos))
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
	// Drain the map-time noise (map/configure/expose burst) so the app's
	// first WaitEvents does not start with a stale batch.
	time.Sleep(50 * time.Millisecond)
	var buf [256]byte
	for xPending(dpy) > 0 {
		xNextEvent(dpy, &buf[0])
	}

	st := &x11State{
		display:   dpy,
		window:    win,
		root:      root,
		screen:    screen,
		wmDelete:  0,
		w:         w,
		h:         h,
		scale:     1,
		pending:   func() int { return xPending(dpy) },
		nextEvent: func(ev *byte) int { return xNextEvent(dpy, ev) },
		flush:     func() { xFlush(dpy) },
		keycodeToKeysym: func(dpy2 uintptr, keycode uint, index int) uintptr {
			if lib.keycodeToKeysym == nil {
				return 0
			}
			return lib.keycodeToKeysym(dpy, keycode, index)
		},
		title:           title,
		decorated:       opts.Decorations,
		resizable:       opts.Resizable,
		visible:         opts.Visible == nil || *opts.Visible,
		xChangeProperty: xChangeProperty,
	}
	// The WM may resize the window at map time (maximize / fit the work
	// area), often animating the size over a few hundred ms. Wait until the
	// client geometry stabilizes (draining configure events meanwhile), then
	// use the ACTUAL size so the first frame and the initial EventResize
	// match the window — otherwise the content starts clipped/offset until
	// the first resize event (the Adopt path probes the same way). If the
	// WM changed the size, drainX emits an initial EventResize on the first
	// drain so the app re-lays-out at the real size.
	deadline := time.Now().Add(500 * time.Millisecond)
	var gw, gh int
	for time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		for xPending(dpy) > 0 {
			xNextEvent(dpy, &buf[0])
		}
		g, g2, ok := x11GetGeometry(st)
		if !ok || g <= 0 || g2 <= 0 {
			continue
		}
		if g == gw && g2 == gh {
			break // stable across two samples
		}
		gw, gh = g, g2
	}
	if gw > 0 && gh > 0 {
		if gw != w || gh != h {
			st.initResizePending = true
		}
		st.w, st.h = gw, gh
	}
	// Resolve EWMH atoms once; the controller and event pump share them.
	st.resolveAtoms(dpy)
	// Re-resolve wmDelete after drain (atom still valid).
	{
		delName := append([]byte("WM_DELETE_WINDOW"), 0)
		st.wmDelete = xInternAtom(dpy, &delName[0], 0)
	}
	host := &x11Host{st: st, lib: lib}
	// Optional IME capability via XIM (ibus/fcitx). Silent degrade when the
	// input method server is absent.
	host.xim = ximOpen(dpy, win)
	host.destroyFn = func() {
		if host.xim != nil {
			host.xim.close()
			host.xim = nil
		}
		xDestroyWindow(dpy, win)
		lib.closeDisplay(dpy)
	}
	ctl := &x11Controller{h: host}

	// Apply creation-time Options that cannot be expressed via XCreateSimple.
	// Order matters: position/map first, then size states so the WM sees a
	// consistent initial frame.
	if opts.Position != nil {
		ctl.SetPosition(opts.Position.X, opts.Position.Y)
	}
	if opts.Fullscreen {
		ctl.SetFullscreen(true)
	}
	if opts.Maximized {
		ctl.Maximize()
	}
	if opts.Cursor != CursorDefault {
		ctl.SetCursor(opts.Cursor)
	}
	if !opts.Decorations {
		ctl.SetDecorations(false)
	}
	visible := opts.Visible == nil || *opts.Visible
	if visible {
		xMapWindow(dpy, win)
	} else {
		ctl.Hide() // created mapped by default; unmap for Visible=false
	}
	xFlush(dpy)
	st.visible = visible
	st.resizable = opts.Resizable

	return newWindow(host, PlatformX11, imeForX11(host), nil, ctl, host.destroy), nil
}

// resolveAtoms resolves the EWMH atoms the controller and event pump share.
// Runs once with a valid display right after window creation.
func (st *x11State) resolveAtoms(dpy uintptr) {
	if st == nil || dpy == 0 {
		return
	}
	lib := ctlLib.open()
	if !lib.ok() || lib.internAtom == nil {
		return
	}
	atom := func(name string) uintptr {
		b := append([]byte(name), 0)
		return lib.internAtom(dpy, &b[0], 0)
	}
	st.atNetState = atom("_NET_WM_STATE")
	st.atMaxV = atom("_NET_WM_STATE_MAXIMIZED_VERT")
	st.atMaxH = atom("_NET_WM_STATE_MAXIMIZED_HORZ")
	st.atFull = atom("_NET_WM_STATE_FULLSCREEN")
	st.atAbove = atom("_NET_WM_STATE_ABOVE")
	st.atActive = atom("_NET_WM_STATE_ACTIVE")
	st.atNetName = atom("_NET_WM_NAME")
	st.atUTF8 = atom("UTF8_STRING")
	st.atMotifHints = atom("_MOTIF_WM_HINTS")
	st.atSyncReq = atom("_NET_WM_SYNC_REQUEST")
	st.atSyncCounter = atom("_NET_WM_SYNC_REQUEST_COUNTER")
	st.atCardinal = atom("CARDINAL")
}

// imeForX11 returns the XIM-based IME capability, or nil when no input
// method is available (silent degrade).
func imeForX11(h *x11Host) IME {
	if h == nil || h.xim == nil {
		return nil
	}
	return &x11Ime{h: h}
}

// --- window state ---

type x11State struct {
	display, window uintptr
	root            uintptr // root window (EWMH client-message target)
	screen          int     // default screen index
	wmDelete        uintptr
	mu              sync.Mutex
	w, h            int
	scale           float64
	pending         func() int
	nextEvent       func(ev *byte) int
	flush           func()
	keycodeToKeysym func(dpy uintptr, keycode uint, index int) uintptr

	// EWMH atoms (resolved at Create).
	atNetState, atMaxV, atMaxH uintptr
	atFull, atAbove, atActive  uintptr
	atNetName, atUTF8          uintptr
	atMotifHints               uintptr
	atSyncReq, atSyncCounter   uintptr // _NET_WM_SYNC_REQUEST + counter property
	atCardinal                 uintptr
	title                      string
	decorated                  bool

	// last reported screen position (EventMove dedup)
	posX, posY int
	posInit    bool

	// A WM-applied map-time resize (maximize / work-area fit) never reaches
	// the app as a ConfigureNotify after the startup drain (setSize reports
	// "no change" against the probed size). Deliver it once as an initial
	// EventResize so the first layout matches the actual window size
	// (Flutter/Skia deliver initial window metrics at startup).
	initResizePending bool

	// Async window state (updated by events + controller).
	maximized  bool
	fullscreen bool
	focused    bool
	visible    bool
	minimized  bool

	// Resizable / constraints bookkeeping for hints lock-restore.
	resizable          bool
	userMinW, userMinH int
	userMaxW, userMaxH int
	hints              xSizeHints // last-applied normal hints
	cursor             uintptr    // current X cursor (0 = default/undefined)

	// _NET_WM_SYNC_REQUEST (resize sync). syncCounter is the last advertised
	// frame counter; pendingSync is the latest request serial awaiting a
	// painted frame; syncDirty marks a property write owed to the X thread
	// (X calls must stay on the event pump thread, hence the flag).
	syncCounter uint64
	pendingSync uint64
	syncDirty   bool
	// xChangeProperty is bound at Create for the event thread flush.
	xChangeProperty func(dpy uintptr, win uintptr, property, typ uintptr, format int, mode int, data *byte, nelements int) int
}

// xConnectionNumberFn is bound at Create/Adopt from libX11. It is package-
// level because the event pump (WaitEvents) reads the X connection fd for the
// kernel poll loop, outside the window-creation closure that owns the binding.
var xConnectionNumberFn func(dpy uintptr) int

// x11Host implements Host for an X11 window (event pump). Destroying the
// window is the Window.Close callback.
type x11Host struct {
	st        *x11State
	lib       *x11Lib
	wake      chan struct{}
	destroyFn func()
	xim       *ximState

	// Kernel-poll plumbing: WaitEvents blocks on unix.Poll over the X
	// connection fd + a self-pipe (WakeUp writes it) instead of a
	// runtime-timer slice — matches the Flutter/Skia event-loop model and
	// removes the Go timer dependency that the frame heartbeat compensated
	// for. Lazily initialized (fallback to the timed path if unavailable).
	pollOnce sync.Once
	xfd      int
	wakeR    *os.File
	wakeW    *os.File

	// imeMu guards the pending IME event queue drained by WaitEvents.
	imeMu     sync.Mutex
	imeEvents []Event
}

// ximFocus activates/deactivates the XIM input context.
func (h *x11Host) ximFocus(on bool) {
	if h == nil || h.xim == nil {
		return
	}
	f := loadXIMFuncs()
	if f == nil {
		return
	}
	if on {
		h.xim.setFocus(f)
	} else {
		h.xim.unsetFocus(f)
	}
}

// pushIME queues an IME event for the next WaitEvents (thread-safe).
func (h *x11Host) pushIME(ev Event) {
	if h == nil {
		return
	}
	h.imeMu.Lock()
	h.imeEvents = append(h.imeEvents, ev)
	h.imeMu.Unlock()
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

// NotifyFrameDrawn advances the _NET_WM_SYNC_REQUEST counter after a frame
// has been submitted for presentation (FrameSync). The compositor samples
// the counter to stop stretching stale content during interactive resize
// drags; without this the visible content freezes at the pre-drag size
// until the mouse is released. The actual X property write is deferred to
// the event pump thread via syncDirty (Xlib is not thread-safe).
func (h *x11Host) NotifyFrameDrawn() {
	if h == nil || h.st == nil || h.st.atSyncCounter == 0 {
		return
	}
	st := h.st
	st.mu.Lock()
	next := st.syncCounter + 1
	if st.pendingSync != 0 && st.pendingSync+1 > next {
		next = st.pendingSync + 1
	}
	st.syncCounter = next
	st.pendingSync = 0
	st.syncDirty = true
	st.mu.Unlock()
	h.WakeUp()
}

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

// drain combines native event drain with the pending IME queue.
func (h *x11Host) drain() []Event {
	if h == nil {
		return nil
	}
	out := h.drainX()
	h.imeMu.Lock()
	if len(h.imeEvents) > 0 {
		out = append(out, h.imeEvents...)
		h.imeEvents = nil
	}
	h.imeMu.Unlock()
	return out
}

// WaitEvents drains X11 events (pointer/key/configure/close) plus queued IME
// events. Expose is always stripped: the GPU backend owns pixels; Expose is
// not architectural IDLE. Resize / input / close still flow.
//
// The wait loop blocks on unix.Poll over the X connection fd + a self-pipe
// (Flutter/Skia event-loop model: kernel-level wait, no runtime-timer
// dependency). Falls back to a timed slice if the poll fds are unavailable.
func (h *x11Host) WaitEvents(timeout time.Duration) []Event {
	if h == nil || h.st == nil {
		return nil
	}
	if h.wake == nil {
		h.wake = make(chan struct{}, 1)
	}
	if timeout == 0 {
		if evs := filterXNoise(h.drain()); len(evs) > 0 {
			return evs
		}
		if h.st.flush != nil {
			h.st.flush()
		}
		return filterXNoise(h.drain())
	}

	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	if h.ensurePollFds() {
		const pollSliceMs = 16
		for {
			if evs := filterXNoise(h.drain()); len(evs) > 0 {
				return evs
			}
			timeoutMs := pollSliceMs
			if !deadline.IsZero() {
				left := time.Until(deadline)
				if left <= 0 {
					if h.st.flush != nil {
						h.st.flush()
					}
					return filterXNoise(h.drain())
				}
				if leftMs := int(left / time.Millisecond); leftMs < timeoutMs {
					timeoutMs = leftMs
				}
			}
			fds := []unix.PollFd{
				{Fd: int32(h.xfd), Events: unix.POLLIN},
				{Fd: int32(h.wakeR.Fd()), Events: unix.POLLIN},
			}
			n, err := unix.Poll(fds, timeoutMs)
			if err != nil {
				if err == unix.EINTR {
					continue
				}
				time.Sleep(time.Duration(pollSliceMs) * time.Millisecond) // degraded safety
				continue
			}
			if n == 0 {
				// Poll slice elapsed (or deadline) → re-check deadline/drain.
				if h.st.flush != nil {
					h.st.flush()
				}
				continue
			}
			if fds[1].Revents&unix.POLLIN != 0 {
				h.drainWakePipe()
				if evs := filterXNoise(h.drain()); len(evs) > 0 {
					return evs
				}
				return []Event{{Type: EventWake}}
			}
			// X fd readable → the loop drains it on the next iteration.
		}
	}

	// Fallback (poll fds unavailable): timed slice on the runtime timer.
	const pollSlice = 16 * time.Millisecond
	for {
		if evs := filterXNoise(h.drain()); len(evs) > 0 {
			return evs
		}
		wait := pollSlice
		if !deadline.IsZero() {
			left := time.Until(deadline)
			if left <= 0 {
				if h.st.flush != nil {
					h.st.flush()
				}
				return filterXNoise(h.drain())
			}
			if left < wait {
				wait = left
			}
		}
		select {
		case <-h.wake:
			if evs := filterXNoise(h.drain()); len(evs) > 0 {
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

// ensurePollFds lazily initializes the kernel-poll plumbing: the X
// connection fd (XConnectionNumber) and a self-pipe for WakeUp. Returns true
// when both are usable.
func (h *x11Host) ensurePollFds() bool {
	if h == nil {
		return false
	}
	h.pollOnce.Do(func() {
		if h.st == nil || h.st.display == 0 || xConnectionNumberFn == nil {
			return
		}
		h.xfd = xConnectionNumberFn(h.st.display)
		if h.xfd <= 0 {
			// 0 would poll stdin — treat as unavailable and fall back.
			return
		}
		r, w, err := os.Pipe()
		if err != nil {
			return
		}
		h.wakeR, h.wakeW = r, w
	})
	return h.wakeR != nil && h.wakeW != nil && h.xfd > 0
}

// drainWakePipe consumes the self-pipe wake byte after poll reports it
// readable. Non-blocking + single read: a blocking Read would hang when a
// previous drain already consumed the byte (the wake is a boolean signal).
func (h *x11Host) drainWakePipe() {
	if h == nil || h.wakeR == nil {
		return
	}
	var buf [8]byte
	_ = unix.SetNonblock(int(h.wakeR.Fd()), true)
	_, _ = h.wakeR.Read(buf[:])
}

func (h *x11Host) WakeUp() {
	if h == nil {
		return
	}
	if h.wakeW != nil {
		// Kernel-poll path: write the self-pipe to wake unix.Poll.
		_, _ = h.wakeW.Write([]byte{1})
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
	// Flush any owed _NET_WM_SYNC_REQUEST counter write. X calls must stay
	// on the event pump thread; NotifyFrameDrawn only marks it dirty.
	if st.atSyncCounter != 0 {
		st.mu.Lock()
		dirty := st.syncDirty
		val := st.syncCounter
		st.syncDirty = false
		st.mu.Unlock()
		if dirty && h.st.xChangeProperty != nil {
			// format=32 data must be a long[] (8 bytes/elem); Xlib copies the
			// low 32 bits of each long. Never pass a packed uint32 array here.
			var v [2]int64
			v[0] = int64(val)
			v[1] = int64(val >> 32)
			h.st.xChangeProperty(st.display, st.window, st.atSyncCounter, st.atCardinal, 32, 0, (*byte)(unsafe.Pointer(&v[0])), 2)
		}
	}
	var out []Event
	// Startup: the WM may have applied a map-time resize (maximize /
	// work-area fit) whose ConfigureNotify was consumed by the Open drain.
	// setSize below reports "no change" (the probed size is already
	// st.w/st.h), so emit the initial size once — the app's first layout
	// must match the actual window.
	if st.initResizePending {
		st.initResizePending = false
		out = append(out, Event{Type: EventResize, Width: st.w, Height: st.h, Scale: h.ScaleFactor()})
	}
	var buf [256]byte
	for st.pending() > 0 {
		st.nextEvent(&buf[0])
		t := int(readI32(buf[:], xevTypeOff))
		switch t {
		case xConfigureNotify:
			nx := int(readI32(buf[:], xevXOff))
			ny := int(readI32(buf[:], xevYOff))
			nw := int(readI32(buf[:], xevWidthOff))
			nh := int(readI32(buf[:], xevHeightOff))
			moved := false
			st.mu.Lock()
			if !st.posInit || st.posX != nx || st.posY != ny {
				st.posX, st.posY, st.posInit = nx, ny, true
				moved = true
			}
			st.mu.Unlock()
			if nw > 0 && nh > 0 && h.setSize(nw, nh) {
				out = append(out, Event{
					Type: EventResize, Width: nw, Height: nh, Scale: h.ScaleFactor(),
				})
			}
			if moved {
				out = append(out, Event{Type: EventMove, MoveX: nx, MoveY: ny})
			}
		case xExpose:
			out = append(out, Event{Type: EventExpose})
		case xMapNotify:
			st.mu.Lock()
			st.visible = true
			st.minimized = false // remapped = restored from iconify
			st.mu.Unlock()
		case xUnmapNotify:
			st.mu.Lock()
			st.visible = false
			st.mu.Unlock()
		case xVisibilityNotify:
			// 0 = unobscured, 1 = partially, 2 = fully obscured.
			state := int(readI32(buf[:], xevStateOff))
			occluded := state == 2
			out = append(out, Event{Type: EventOccluded, Occluded: occluded})
		case xEnterNotify:
			out = append(out, Event{Type: EventPointer, Pointer: PointerEnter})
		case xLeaveNotify:
			out = append(out, Event{Type: EventPointer, Pointer: PointerLeave})
		case xButtonPress, xButtonRelease, xMotionNotify:
			if ev, ok := h.decodePointer(t, buf[:]); ok {
				out = append(out, ev)
			}
		case xKeyPress, xKeyRelease:
			// Route through the IME first (XIM). If the input method consumed
			// the key (composing), skip it as a plain key. If it committed
			// text, surface that as an IME event too — consumers split the
			// payloads: textinput inserts the commit, focus/keys use the
			// key event for shortcuts (a printable char still yields both).
			skipKey := false
			if h.xim != nil {
				handled, committed := h.xim.filter(loadXIMFuncs(), &buf[0], h.st.window)
				if handled {
					skipKey = true // IME is composing; not a plain key
				}
				if committed != "" {
					out = append(out, Event{
						Type: EventIME, IMEKind: 1, // commit
						IMEText: committed, IMEStart: -1, IMEEnd: -1,
					})
				}
			}
			if !skipKey {
				if ev, ok := h.decodeKey(t, buf[:]); ok {
					out = append(out, ev)
				}
			}
		case xClientMessage:
			data0 := readU64(buf[:], xevClientData0Off)
			switch {
			case st.wmDelete != 0 && uintptr(data0) == st.wmDelete:
				out = append(out, Event{Type: EventClose})
			case st.atSyncReq != 0 && uintptr(data0) == st.atSyncReq:
				// _NET_WM_SYNC_REQUEST: l[1] = new size, l[2] = serial (low
				// 32) + flags (high 32). Mode 0 (simple counter): the painted
				// frame must advance the counter past serial before the
				// compositor unstretches. Keep only the latest serial; a
				// frame in flight covers all prior requests at once. Surface
				// an EventResizeSync so the app schedules the promised frame
				// (Flutter/Skia: sync requests drive frame production).
				raw := readU64(buf[:], xevClientData0Off+16)
				if uint32(raw>>32) == 0 { // simple counter mode
					st.mu.Lock()
					st.pendingSync = uint64(uint32(raw))
					st.mu.Unlock()
					out = append(out, Event{Type: EventResizeSync})
				}
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
		ks := st.keycodeToKeysym(st.display, keycode, 0)
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

func x11KeycodeToKeysym(lib *x11Lib) func(dpy uintptr, keycode uint, index int) uintptr {
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
		xGetGeometry func(dpy, win uintptr, root *uintptr, x, y *int32, w, h *uint32, border *uint32, depth *uint32) int
	)
	purego.RegisterLibFunc(&xGetGeometry, lib.lib, "XGetGeometry")
	if xGetGeometry == nil {
		return 0, 0, false
	}
	var (
		rootRet uintptr
		x, y    int32
		wd, ht  uint32
		border  uint32
		depth   uint32
	)
	// XGetGeometry writes root_return unconditionally — a nil root pointer
	// dereferences address 0 in the X server client library.
	if xGetGeometry(st.display, st.window, &rootRet, &x, &y, &wd, &ht, &border, &depth) == 0 {
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
