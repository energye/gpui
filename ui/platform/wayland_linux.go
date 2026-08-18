//go:build linux

package platform

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

// waylandBackend implements Backend for Wayland (wl_display* + wl_surface*).
// Ported from the verified exhost Wayland host, restructured into Create
// (window lifecycle) / wlHost (event pump) / capability probing.
type waylandBackend struct{}

func init() { Register(PlatformWayland, &waylandBackend{}) }

func (b *waylandBackend) Kind() PlatformKind { return PlatformWayland }

// Create opens a Wayland window (xdg_toplevel, optional CSD decorations).
func (b *waylandBackend) Create(opts Options) (*Window, error) {
	return waylandCreate(opts)
}

// Adopt for Wayland requires an existing wl_display* and wl_surface*; the
// backend would need to re-bind the registry and protocol objects to pump
// events, which is not safely reconstructible from raw handles alone. Return
// a clear error (interface reserved; implement with the embedding milestone).
func (b *waylandBackend) Adopt(ns NativeSurface) (*Window, error) {
	return nil, fmt.Errorf("wayland: Adopt not supported yet (embedding milestone); got display=%x surface=%x", ns.Display, ns.Window)
}

// --- Wayland protocol constants ---

const (
	wlRegistryBind            = 0
	wlCompositorCreateSurface = 0
	wlCompositorCreateRegion  = 1
	wlSurfaceDestroy          = 0
	wlSurfaceAttach           = 1
	wlSurfaceDamage           = 2
	wlSurfaceSetInputRegion   = 5
	wlSurfaceCommit           = 6
	wlRegionAdd               = 0
	wlRegionDestroy           = 1
	xdgWmBaseGetXdgSurface    = 2
	xdgWmBasePong             = 3
	xdgSurfaceGetToplevel     = 1
	xdgSurfaceAckConfigure    = 4
	xdgToplevelSetTitle       = 2
	xdgToplevelSetAppID       = 3
	xdgToplevelDestroy        = 0
	xdgSurfaceDestroy         = 0
	xdgWmBaseDestroy          = 0
	// xdg_toplevel state requests (xdg-shell.xml authoritative order):
	//   set_max_size(7) set_min_size(8) set_maximized(9) unset_maximized(10)
	//   set_fullscreen(11) unset_fullscreen(12) set_minimized(13)
	// (set_maximized/unset_maximized/set_minimized are declared in
	// wayland_csd_linux.go — shared by CSD and the controller.)
	xdgToplevelSetMaxSize      = 7
	xdgToplevelSetMinSize      = 8
	xdgToplevelSetFullscreen   = 11
	xdgToplevelUnsetFullscreen = 12
	// zxdg_decoration_manager_v1
	xdgDecoMgrGetDecoration = 1
	// zxdg_toplevel_decoration_v1
	xdgDecoSetMode = 1
	// decoration mode
	xdgDecoModeClientSide = 1
	xdgDecoModeServerSide = 2
)

// wl_argument is 8 bytes on amd64 (union).
type wlArg uint64

func argU(u uint32) wlArg  { return wlArg(u) }
func argS(p uintptr) wlArg { return wlArg(p) }
func argO(p uintptr) wlArg { return wlArg(p) }
func argNewID() wlArg      { return 0 }

type wlInterfaceC struct {
	Name        uintptr
	Version     int32
	MethodCount int32
	Methods     uintptr
	EventCount  int32
	_pad        int32
	Events      uintptr
}

type wlMessageC struct {
	Name      uintptr
	Signature uintptr
	Types     uintptr
}

type wlLib struct {
	lib uintptr

	displayConnect         func(name *byte) uintptr
	displayDisconnect      func(d uintptr)
	displayDispatch        func(d uintptr) int
	displayDispatchPend    func(d uintptr) int
	displayFlush           func(d uintptr) int
	displayRoundtrip       func(d uintptr) int
	displayGetFD           func(d uintptr) int
	displayPrepareRead     func(d uintptr) int
	displayReadEvents      func(d uintptr) int
	displayCancelRead      func(d uintptr) int
	proxyAddListener       func(proxy uintptr, impl uintptr, data uintptr) int
	proxyMarshalArrayCtor  func(proxy uintptr, opcode uint32, args *wlArg, iface uintptr, version uint32) uintptr
	proxyMarshalArrayFlags func(proxy uintptr, opcode uint32, iface uintptr, version uint32, flags uint32, args *wlArg) uintptr
	proxyDestroy           func(proxy uintptr)
	proxyGetVersion        func(proxy uintptr) uint32

	ifaceCompositor uintptr
	ifaceSurface    uintptr
	ifaceRegistry   uintptr
	ifaceSeat       uintptr
	ifaceKeyboard   uintptr
	ifacePointer    uintptr
	// CSD (client-side decorations) interfaces.
	ifaceShm           uintptr
	ifaceShmPool       uintptr
	ifaceBuffer        uintptr
	ifaceSubcompositor uintptr
	ifaceSubsurface    uintptr
	ifaceOutput        uintptr // wl_output (xdg_toplevel.set_fullscreen arg)
	ifaceRegion        uintptr // wl_region (wl_surface.set_input_region)
}

func loadWayland() (*wlLib, error) {
	lib, err := purego.Dlopen("libwayland-client.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libwayland-client.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		return nil, err
	}
	l := &wlLib{lib: lib}
	purego.RegisterLibFunc(&l.displayConnect, lib, "wl_display_connect")
	purego.RegisterLibFunc(&l.displayDisconnect, lib, "wl_display_disconnect")
	purego.RegisterLibFunc(&l.displayDispatch, lib, "wl_display_dispatch")
	purego.RegisterLibFunc(&l.displayDispatchPend, lib, "wl_display_dispatch_pending")
	purego.RegisterLibFunc(&l.displayFlush, lib, "wl_display_flush")
	purego.RegisterLibFunc(&l.displayRoundtrip, lib, "wl_display_roundtrip")
	purego.RegisterLibFunc(&l.displayGetFD, lib, "wl_display_get_fd")
	purego.RegisterLibFunc(&l.displayPrepareRead, lib, "wl_display_prepare_read")
	purego.RegisterLibFunc(&l.displayReadEvents, lib, "wl_display_read_events")
	purego.RegisterLibFunc(&l.displayCancelRead, lib, "wl_display_cancel_read")
	purego.RegisterLibFunc(&l.proxyAddListener, lib, "wl_proxy_add_listener")
	purego.RegisterLibFunc(&l.proxyMarshalArrayCtor, lib, "wl_proxy_marshal_array_constructor_versioned")
	purego.RegisterLibFunc(&l.proxyMarshalArrayFlags, lib, "wl_proxy_marshal_array_flags")
	purego.RegisterLibFunc(&l.proxyDestroy, lib, "wl_proxy_destroy")
	purego.RegisterLibFunc(&l.proxyGetVersion, lib, "wl_proxy_get_version")

	for _, pair := range []struct {
		name string
		dst  *uintptr
	}{
		{"wl_compositor_interface", &l.ifaceCompositor},
		{"wl_surface_interface", &l.ifaceSurface},
		{"wl_registry_interface", &l.ifaceRegistry},
		{"wl_seat_interface", &l.ifaceSeat},
		{"wl_keyboard_interface", &l.ifaceKeyboard},
		{"wl_pointer_interface", &l.ifacePointer},
		{"wl_shm_interface", &l.ifaceShm},
		{"wl_shm_pool_interface", &l.ifaceShmPool},
		{"wl_buffer_interface", &l.ifaceBuffer},
		{"wl_subcompositor_interface", &l.ifaceSubcompositor},
		{"wl_subsurface_interface", &l.ifaceSubsurface},
		{"wl_output_interface", &l.ifaceOutput},
		{"wl_region_interface", &l.ifaceRegion},
	} {
		p, err := purego.Dlsym(lib, pair.name)
		if err != nil || p == 0 {
			return nil, fmt.Errorf("Dlsym %s: %v", pair.name, err)
		}
		*pair.dst = p
	}
	return l, nil
}

// wl_display.get_registry is header-inline → opcode 1 + wl_registry_interface.
func (l *wlLib) displayGetRegistry(dpy uintptr) uintptr {
	args := []wlArg{argNewID()}
	ver := l.proxyGetVersion(dpy)
	if ver == 0 {
		ver = 1
	}
	return l.proxyMarshalArrayCtor(dpy, 1, &args[0], l.ifaceRegistry, ver)
}

// --- xdg-shell interfaces (defined in-process, not exported by libwayland) ---

var (
	xdgNames = struct {
		wmBase, surface, toplevel      []byte
		decoMgr, decoTop               []byte
		mDestroy, mGetXdg, mPong       []byte
		mGetTop, mAck                  []byte
		mSetTitle, mSetApp             []byte
		mSetParent, mShowMenu          []byte
		mMove, mResize                 []byte
		mSetMinSize, mSetMaxSize       []byte
		mSetMaxed, mUnsetMaxed         []byte
		mSetFull, mUnsetFull           []byte
		mSetMinimized                  []byte
		mGetDeco, mSetMode             []byte
		sEmpty, sNo, sU, sN, sS        []byte
		sO, sQo, sOu, sOuu, sOuii, sIi []byte
		sNoTop                         []byte
		ePing, eCfg, eClose            []byte
		eCfgIia                        []byte
	}{
		wmBase:        append([]byte("xdg_wm_base"), 0),
		surface:       append([]byte("xdg_surface"), 0),
		toplevel:      append([]byte("xdg_toplevel"), 0),
		decoMgr:       append([]byte("zxdg_decoration_manager_v1"), 0),
		decoTop:       append([]byte("zxdg_toplevel_decoration_v1"), 0),
		mDestroy:      append([]byte("destroy"), 0),
		mGetXdg:       append([]byte("get_xdg_surface"), 0),
		mPong:         append([]byte("pong"), 0),
		mGetTop:       append([]byte("get_toplevel"), 0),
		mAck:          append([]byte("ack_configure"), 0),
		mSetTitle:     append([]byte("set_title"), 0),
		mSetApp:       append([]byte("set_app_id"), 0),
		mGetDeco:      append([]byte("get_toplevel_decoration"), 0),
		mSetMode:      append([]byte("set_mode"), 0),
		mSetParent:    append([]byte("set_parent"), 0),
		mShowMenu:     append([]byte("show_window_menu"), 0),
		mMove:         append([]byte("move"), 0),
		mResize:       append([]byte("resize"), 0),
		mSetMinSize:   append([]byte("set_min_size"), 0),
		mSetMaxSize:   append([]byte("set_max_size"), 0),
		mSetMaxed:     append([]byte("set_maximized"), 0),
		mUnsetMaxed:   append([]byte("unset_maximized"), 0),
		mSetFull:      append([]byte("set_fullscreen"), 0),
		mUnsetFull:    append([]byte("unset_fullscreen"), 0),
		mSetMinimized: append([]byte("set_minimized"), 0),
		sEmpty:        append([]byte(""), 0),
		sNo:           append([]byte("no"), 0),
		sU:            append([]byte("u"), 0),
		sN:            append([]byte("n"), 0),
		sS:            append([]byte("s"), 0),
		sO:            append([]byte("o"), 0),
		sQo:           append([]byte("?o"), 0),
		sOu:           append([]byte("ou"), 0),
		sOuu:          append([]byte("ouu"), 0),
		sOuii:         append([]byte("ouii"), 0),
		sIi:           append([]byte("ii"), 0),
		sNoTop:        append([]byte("no"), 0),
		ePing:         append([]byte("ping"), 0),
		eCfg:          append([]byte("configure"), 0),
		eClose:        append([]byte("close"), 0),
		eCfgIia:       append([]byte("iia"), 0),
	}

	ifaceXdgWmBase   wlInterfaceC
	ifaceXdgSurface  wlInterfaceC
	ifaceXdgToplevel wlInterfaceC
	ifaceDecoMgr     wlInterfaceC
	ifaceDecoTop     wlInterfaceC

	msgWmBase    [4]wlMessageC
	msgWmBaseEv  [1]wlMessageC
	msgXdgSurf   [5]wlMessageC
	msgXdgSurfEv [1]wlMessageC
	msgTop       [14]wlMessageC
	msgTopEv     [2]wlMessageC
	msgDecoMgr   [2]wlMessageC
	msgDecoTop   [3]wlMessageC
	msgDecoTopEv [1]wlMessageC

	typesXdgSurf [2]uintptr
	typesTop     [1]uintptr
	typesTopO    [1]uintptr // set_parent: ?o → xdg_toplevel
	typesMenu    [4]uintptr // show_window_menu: o u i i
	typesMove    [2]uintptr // move: o u
	typesResize  [3]uintptr // resize: o u u
	typesFull    [1]uintptr // set_fullscreen: ?o → wl_output
	typesDeco    [2]uintptr
	typesEmpty   [1]uintptr
)

func cstr(b []byte) uintptr { return uintptr(unsafe.Pointer(&b[0])) }

func initXDGInterfaces(ifaceSurface, ifaceSeat, ifaceOutput uintptr) {
	seatIface := ifaceSeat
	outputIface := ifaceOutput
	msgWmBase[0] = wlMessageC{Name: cstr(xdgNames.mDestroy), Signature: cstr(xdgNames.sEmpty), Types: 0}
	msgWmBase[1] = wlMessageC{Name: cstr(xdgNames.mDestroy), Signature: cstr(xdgNames.sN), Types: 0}
	typesXdgSurf[0] = 0
	typesXdgSurf[1] = ifaceSurface
	msgWmBase[2] = wlMessageC{Name: cstr(xdgNames.mGetXdg), Signature: cstr(xdgNames.sNo), Types: uintptr(unsafe.Pointer(&typesXdgSurf[0]))}
	msgWmBase[3] = wlMessageC{Name: cstr(xdgNames.mPong), Signature: cstr(xdgNames.sU), Types: 0}
	msgWmBaseEv[0] = wlMessageC{Name: cstr(xdgNames.ePing), Signature: cstr(xdgNames.sU), Types: 0}

	ifaceXdgWmBase = wlInterfaceC{
		Name: cstr(xdgNames.wmBase), Version: 2,
		MethodCount: 4, Methods: uintptr(unsafe.Pointer(&msgWmBase[0])),
		EventCount: 1, Events: uintptr(unsafe.Pointer(&msgWmBaseEv[0])),
	}

	typesTop[0] = 0
	msgXdgSurf[0] = wlMessageC{Name: cstr(xdgNames.mDestroy), Signature: cstr(xdgNames.sEmpty), Types: 0}
	msgXdgSurf[1] = wlMessageC{Name: cstr(xdgNames.mGetTop), Signature: cstr(xdgNames.sN), Types: uintptr(unsafe.Pointer(&typesTop[0]))}
	msgXdgSurf[2] = wlMessageC{Name: cstr(xdgNames.mDestroy), Signature: cstr(xdgNames.sEmpty), Types: 0}
	msgXdgSurf[3] = wlMessageC{Name: cstr(xdgNames.mDestroy), Signature: cstr(xdgNames.sEmpty), Types: 0}
	msgXdgSurf[4] = wlMessageC{Name: cstr(xdgNames.mAck), Signature: cstr(xdgNames.sU), Types: 0}
	msgXdgSurfEv[0] = wlMessageC{Name: cstr(xdgNames.eCfg), Signature: cstr(xdgNames.sU), Types: 0}
	ifaceXdgSurface = wlInterfaceC{
		Name: cstr(xdgNames.surface), Version: 2,
		MethodCount: 5, Methods: uintptr(unsafe.Pointer(&msgXdgSurf[0])),
		EventCount: 1, Events: uintptr(unsafe.Pointer(&msgXdgSurfEv[0])),
	}
	typesXdgSurf[0] = uintptr(unsafe.Pointer(&ifaceXdgSurface))
	typesTop[0] = uintptr(unsafe.Pointer(&ifaceXdgToplevel))
	typesTopO[0] = uintptr(unsafe.Pointer(&ifaceXdgToplevel))
	typesMenu[0] = seatIface
	typesMove[0] = seatIface
	typesResize[0] = seatIface
	typesFull[0] = outputIface

	// xdg_toplevel requests (authoritative xdg-shell.xml order):
	//   destroy(0) set_parent(1) set_title(2) set_app_id(3)
	//   show_window_menu(4) move(5) resize(6) set_max_size(7)
	//   set_min_size(8) set_maximized(9) unset_maximized(10)
	//   set_fullscreen(11) unset_fullscreen(12) set_minimized(13)
	msgTop[0] = wlMessageC{Name: cstr(xdgNames.mDestroy), Signature: cstr(xdgNames.sEmpty), Types: 0}
	msgTop[1] = wlMessageC{Name: cstr(xdgNames.mSetParent), Signature: cstr(xdgNames.sQo), Types: uintptr(unsafe.Pointer(&typesTopO[0]))}
	msgTop[2] = wlMessageC{Name: cstr(xdgNames.mSetTitle), Signature: cstr(xdgNames.sS), Types: 0}
	msgTop[3] = wlMessageC{Name: cstr(xdgNames.mSetApp), Signature: cstr(xdgNames.sS), Types: 0}
	msgTop[4] = wlMessageC{Name: cstr(xdgNames.mShowMenu), Signature: cstr(xdgNames.sOuii), Types: uintptr(unsafe.Pointer(&typesMenu[0]))}
	msgTop[5] = wlMessageC{Name: cstr(xdgNames.mMove), Signature: cstr(xdgNames.sOu), Types: uintptr(unsafe.Pointer(&typesMove[0]))}
	msgTop[6] = wlMessageC{Name: cstr(xdgNames.mResize), Signature: cstr(xdgNames.sOuu), Types: uintptr(unsafe.Pointer(&typesResize[0]))}
	msgTop[7] = wlMessageC{Name: cstr(xdgNames.mSetMaxSize), Signature: cstr(xdgNames.sIi), Types: 0}
	msgTop[8] = wlMessageC{Name: cstr(xdgNames.mSetMinSize), Signature: cstr(xdgNames.sIi), Types: 0}
	msgTop[9] = wlMessageC{Name: cstr(xdgNames.mSetMaxed), Signature: cstr(xdgNames.sEmpty), Types: 0}
	msgTop[10] = wlMessageC{Name: cstr(xdgNames.mUnsetMaxed), Signature: cstr(xdgNames.sEmpty), Types: 0}
	msgTop[11] = wlMessageC{Name: cstr(xdgNames.mSetFull), Signature: cstr(xdgNames.sQo), Types: uintptr(unsafe.Pointer(&typesFull[0]))}
	msgTop[12] = wlMessageC{Name: cstr(xdgNames.mUnsetFull), Signature: cstr(xdgNames.sEmpty), Types: 0}
	msgTop[13] = wlMessageC{Name: cstr(xdgNames.mSetMinimized), Signature: cstr(xdgNames.sEmpty), Types: 0}
	msgTopEv[0] = wlMessageC{Name: cstr(xdgNames.eCfg), Signature: cstr(xdgNames.eCfgIia), Types: 0}
	msgTopEv[1] = wlMessageC{Name: cstr(xdgNames.eClose), Signature: cstr(xdgNames.sEmpty), Types: 0}
	ifaceXdgToplevel = wlInterfaceC{
		Name: cstr(xdgNames.toplevel), Version: 2,
		MethodCount: 14, Methods: uintptr(unsafe.Pointer(&msgTop[0])),
		EventCount: 2, Events: uintptr(unsafe.Pointer(&msgTopEv[0])),
	}
	typesTop[0] = uintptr(unsafe.Pointer(&ifaceXdgToplevel))

	msgDecoTop[0] = wlMessageC{Name: cstr(xdgNames.mDestroy), Signature: cstr(xdgNames.sEmpty), Types: 0}
	msgDecoTop[1] = wlMessageC{Name: cstr(xdgNames.mSetMode), Signature: cstr(xdgNames.sU), Types: 0}
	msgDecoTop[2] = wlMessageC{Name: cstr(xdgNames.mDestroy), Signature: cstr(xdgNames.sEmpty), Types: 0}
	msgDecoTopEv[0] = wlMessageC{Name: cstr(xdgNames.eCfg), Signature: cstr(xdgNames.sU), Types: 0}
	ifaceDecoTop = wlInterfaceC{
		Name: cstr(xdgNames.decoTop), Version: 1,
		MethodCount: 3, Methods: uintptr(unsafe.Pointer(&msgDecoTop[0])),
		EventCount: 1, Events: uintptr(unsafe.Pointer(&msgDecoTopEv[0])),
	}
	typesDeco[0] = uintptr(unsafe.Pointer(&ifaceDecoTop))
	typesDeco[1] = uintptr(unsafe.Pointer(&ifaceXdgToplevel))
	msgDecoMgr[0] = wlMessageC{Name: cstr(xdgNames.mDestroy), Signature: cstr(xdgNames.sEmpty), Types: 0}
	msgDecoMgr[1] = wlMessageC{Name: cstr(xdgNames.mGetDeco), Signature: cstr(xdgNames.sNoTop), Types: uintptr(unsafe.Pointer(&typesDeco[0]))}
	ifaceDecoMgr = wlInterfaceC{
		Name: cstr(xdgNames.decoMgr), Version: 1,
		MethodCount: 2, Methods: uintptr(unsafe.Pointer(&msgDecoMgr[0])),
		EventCount: 0, Events: 0,
	}
	_ = typesEmpty
}

// --- window state ---

type wlWin struct {
	lib *wlLib

	display  uintptr
	registry uintptr
	comp     uintptr
	surface  uintptr
	wmBase   uintptr
	xdgSurf  uintptr
	toplevel uintptr

	compName, compVer uint32
	wmName, wmVer     uint32
	decoName, decoVer uint32
	decoMgr           uintptr
	decoTop           uintptr
	tiMgrName         uint32  // zwp_text_input_manager_v3 global name (0 = absent)
	seatName          uint32  // wl_seat global name (0 = absent)
	shmName           uint32  // wl_shm global name (0 = absent)
	subcompName       uint32  // wl_subcompositor global name (0 = absent)
	seat              uintptr // bound wl_seat proxy (via seatState)
	seatState         *wlSeatState
	// hostRef is set by waylandCreate so seat callbacks can wake the loop.
	hostRef *wlHost

	width, height int
	configured    bool
	closed        bool
	closeReq      bool // xdg close / CSD ✕ request (→ EventCloseRequested)
	resized       bool
	// resizing mirrors the xdg_toplevel "resizing" configure state (3): the
	// compositor sets it while an interactive move/resize drag is in flight
	// and clears it when the drag ends. Read-only diagnostic for apps/tests
	// (event thread); the resize events themselves drive the relayout.
	resizing bool

	// Async window state (configure-driven).
	activated bool // EventFocus dedup
	suspended bool // EventOccluded dedup

	// ctlMu guards the WindowController-tracked state below. Controller
	// methods may be called from any goroutine (§2.5.5 thread contract);
	// wlTopConfigure reconciles the configure-driven fields on the event
	// thread.
	ctlMu      sync.Mutex
	title      string // tracked at Create/SetTitle
	minW, minH int    // user min constraint (0 = unconstrained)
	maxW, maxH int    // user max constraint (0 = unconstrained)
	resizable  bool   // SetResizable flag (false = min==max locked)
	minimized  bool   // optimistic: set_minimized → true; activated → false
	maximized  bool   // configure states (true value, not optimistic)
	fullscreen bool   // configure states (true value, not optimistic)
	cursor     Cursor // active cursor (applied on pointer enter/motion)

	// focusMu guards focusEvents (focus/occlusion) queued by callbacks on the
	// event thread and drained by poll.
	focusMu     sync.Mutex
	focusEvents []Event

	// text-input (IME) optional capability.
	ti *wlTIState
	// keyboard (wl_keyboard + xkb) optional capability.
	kbd *wlKeyboardState
	// pointer (wl_pointer) standard mouse input.
	ptr *wlPointerState
	// csd holds client-side decorations (title bar + borders), nil when
	// frameless or the compositor lacks wl_shm/wl_subcompositor.
	csd *wlCSD
	// imeMu guards the pending IME event queue drained by poll.
	imeMu     sync.Mutex
	imeEvents []Event
	// keyMu guards the pending key event queue drained by poll.
	keyMu     sync.Mutex
	keyEvents []Event
	// ptrMu guards the pending pointer event queue drained by poll.
	ptrMu     sync.Mutex
	ptrEvents []Event

	regListener [2]uintptr
	wmListener  [1]uintptr
	xdgListener [1]uintptr
	topListener [2]uintptr
	selfPtr     uintptr

	titlePin []byte
	appPin   []byte
}

var (
	wlMu    sync.Mutex
	wlByPtr = map[uintptr]*wlWin{}
)

func waylandCreate(opts Options) (*Window, error) {
	if !HasWaylandDisplay() {
		return nil, fmt.Errorf("wayland: WAYLAND_DISPLAY not set")
	}
	lib, err := loadWayland()
	if err != nil {
		return nil, err
	}
	initXDGInterfaces(lib.ifaceSurface, lib.ifaceSeat, lib.ifaceOutput)

	w, h := opts.Width, opts.Height
	if w < 1 {
		w = 640
	}
	if h < 1 {
		h = 480
	}
	title := opts.Title
	if title == "" {
		title = "gpui"
	}
	decorated := opts.Decorations // true = client-side decorations (GNOME has no SSD)

	win := &wlWin{
		lib:       lib,
		width:     w,
		height:    h,
		title:     title,
		resizable: true,
		cursor:    opts.Cursor,
	}
	if opts.Maximized {
		win.maximized = true
	}

	dpy := lib.displayConnect(nil)
	if dpy == 0 {
		return nil, fmt.Errorf("wayland: wl_display_connect failed")
	}
	win.display = dpy

	win.selfPtr = uintptr(unsafe.Pointer(win))
	wlMu.Lock()
	wlByPtr[win.selfPtr] = win
	wlMu.Unlock()

	win.regListener[0] = purego.NewCallback(wlRegistryGlobal)
	win.regListener[1] = purego.NewCallback(wlRegistryGlobalRemove)

	reg := lib.displayGetRegistry(dpy)
	if reg == 0 {
		win.destroyNative()
		return nil, fmt.Errorf("wayland: wl_display_get_registry failed")
	}
	win.registry = reg
	lib.proxyAddListener(reg, uintptr(unsafe.Pointer(&win.regListener[0])), win.selfPtr)
	lib.displayRoundtrip(dpy)

	if win.compName == 0 || win.wmName == 0 {
		win.destroyNative()
		return nil, fmt.Errorf("wayland: missing wl_compositor or xdg_wm_base global")
	}

	win.comp = win.bind(reg, win.compName, lib.ifaceCompositor, minU32(win.compVer, 4))
	if win.comp == 0 {
		win.destroyNative()
		return nil, fmt.Errorf("wayland: bind wl_compositor failed")
	}

	win.wmBase = win.bind(reg, win.wmName, uintptr(unsafe.Pointer(&ifaceXdgWmBase)), minU32(win.wmVer, 2))
	if win.wmBase == 0 {
		win.destroyNative()
		return nil, fmt.Errorf("wayland: bind xdg_wm_base failed")
	}
	win.wmListener[0] = purego.NewCallback(wlWmPing)
	lib.proxyAddListener(win.wmBase, uintptr(unsafe.Pointer(&win.wmListener[0])), win.selfPtr)

	win.surface = win.ctor(win.comp, wlCompositorCreateSurface, lib.ifaceSurface, 4)
	if win.surface == 0 {
		win.destroyNative()
		return nil, fmt.Errorf("wayland: create wl_surface failed")
	}

	{
		args := []wlArg{argNewID(), argO(win.surface)}
		win.xdgSurf = lib.proxyMarshalArrayCtor(win.wmBase, xdgWmBaseGetXdgSurface, &args[0],
			uintptr(unsafe.Pointer(&ifaceXdgSurface)), 2)
	}
	if win.xdgSurf == 0 {
		win.destroyNative()
		return nil, fmt.Errorf("wayland: get_xdg_surface failed")
	}
	win.xdgListener[0] = purego.NewCallback(wlXdgConfigure)
	lib.proxyAddListener(win.xdgSurf, uintptr(unsafe.Pointer(&win.xdgListener[0])), win.selfPtr)

	{
		args := []wlArg{argNewID()}
		win.toplevel = lib.proxyMarshalArrayCtor(win.xdgSurf, xdgSurfaceGetToplevel, &args[0],
			uintptr(unsafe.Pointer(&ifaceXdgToplevel)), 2)
	}
	if win.toplevel == 0 {
		win.destroyNative()
		return nil, fmt.Errorf("wayland: get_toplevel failed")
	}
	win.topListener[0] = purego.NewCallback(wlTopConfigure)
	win.topListener[1] = purego.NewCallback(wlTopClose)
	lib.proxyAddListener(win.toplevel, uintptr(unsafe.Pointer(&win.topListener[0])), win.selfPtr)

	win.titlePin = append([]byte(title), 0)
	win.appPin = append([]byte("gpui.l1"), 0)
	{
		args := []wlArg{argS(cstr(win.titlePin))}
		lib.proxyMarshalArrayFlags(win.toplevel, xdgToplevelSetTitle, 0, 0, 0, &args[0])
	}
	{
		args := []wlArg{argS(cstr(win.appPin))}
		lib.proxyMarshalArrayFlags(win.toplevel, xdgToplevelSetAppID, 0, 0, 0, &args[0])
	}

	if win.decoName != 0 {
		win.decoMgr = win.bind(reg, win.decoName, uintptr(unsafe.Pointer(&ifaceDecoMgr)), 1)
		if win.decoMgr != 0 {
			args := []wlArg{argNewID(), argO(win.toplevel)}
			win.decoTop = lib.proxyMarshalArrayCtor(win.decoMgr, xdgDecoMgrGetDecoration, &args[0],
				uintptr(unsafe.Pointer(&ifaceDecoTop)), 1)
			if win.decoTop != 0 {
				mode := []wlArg{argU(xdgDecoModeServerSide)}
				lib.proxyMarshalArrayFlags(win.decoTop, xdgDecoSetMode, 0, 0, 0, &mode[0])
			}
		}
	}

	// Options.Maximized: set_maximized BEFORE the first commit so the
	// compositor never shows a non-maximized frame (main doc §2.4: 首 commit
	// 前 set_maximized; the requested size from the first configure is then
	// the maximized size, which the CSD ignores when maximized).
	if win.maximized {
		lib.proxyMarshalArrayFlags(win.toplevel, xdgToplevelSetMaximized, 0, 0, 0, nil)
	}

	lib.proxyMarshalArrayFlags(win.surface, wlSurfaceCommit, 0, 0, 0, nil)

	deadline := time.Now().Add(3 * time.Second)
	for !win.configured && !win.closed && time.Now().Before(deadline) {
		if lib.displayDispatch(dpy) < 0 {
			break
		}
	}
	if !win.configured {
		win.destroyNative()
		return nil, fmt.Errorf("wayland: xdg configure timeout")
	}
	runtime.KeepAlive(win)

	// Creation-time size constraints (xdg has no creation hints; requests are
	// applied once the toplevel is configured). 0 = unconstrained (unlimited).
	win.top2i(xdgToplevelSetMinSize, opts.MinWidth, opts.MinHeight)
	win.top2i(xdgToplevelSetMaxSize, opts.MaxWidth, opts.MaxHeight)
	if !opts.Resizable {
		// Fixed-size window: min==max locks at the initial size (§2.5.2
		// Wayland clamp contract; unlock via SetMinSize/SetMaxSize/SetResizable).
		win.ctlMu.Lock()
		win.resizable = false
		win.ctlMu.Unlock()
		win.top2i(xdgToplevelSetMinSize, w, h)
		win.top2i(xdgToplevelSetMaxSize, w, h)
	}
	if opts.Fullscreen {
		win.ctlMu.Lock()
		win.fullscreen = true
		win.ctlMu.Unlock()
		// set_fullscreen(NULL) → the compositor's current output (§2.4).
		win.top1o(xdgToplevelSetFullscreen, 0)
	}
	// Options.Cursor is applied on the first pointer enter (set_cursor needs
	// an enter serial; win.cursor already holds the initial value).

	// Client-side decorations (CSD): GNOME provides no server-side chrome, so
	// draw a title bar + borders via wl_subsurface when requested (default
	// true). Silent degrade if the compositor lacks wl_shm/wl_subcompositor.
	if decorated {
		win.csd = win.initCSD(title)
	}

	// Standard Wayland input bootstrap: bind wl_seat and WAIT for the
	// capabilities event before requesting keyboard/pointer. ALL seat-derived
	// objects (keyboard, pointer, text-input) are created only after the seat
	// callback fires (wlSeatFlushPending) — this is the normal client order.
	//
	// Input is ON by default (standard Wayland client behavior; matches GTK/
	// Chromium). To opt out of one or all bindings set the flag to "0":
	//   GPUI_WL_KEYBOARD=0  GPUI_WL_POINTER=0  GPUI_WL_TEXTINPUT=0
	// or disable everything with GPUI_WL_NO_INPUT=1.
	seatEnabled := os.Getenv("GPUI_WL_NO_INPUT") != "1"
	if win.seatName != 0 && seatEnabled {
		win.seatState = win.bindSeat()
		if win.seatState != nil {
			win.seat = win.seatState.seat
			win.seatState.pendingKeys = os.Getenv("GPUI_WL_KEYBOARD") != "0"
			win.seatState.pendingPtrs = os.Getenv("GPUI_WL_POINTER") != "0"
			win.seatState.pendingTI = os.Getenv("GPUI_WL_TEXTINPUT") != "0"
			// Dispatch until seat.capabilities arrives so the deferred device
			// creation (wlSeatFlushPending) runs BEFORE the window is returned:
			// the IME capability (win.ti) must already exist when imeFor() is
			// evaluated below, otherwise Window.IME() reports nil.
			seatDeadline := time.Now().Add(2 * time.Second)
			for !win.seatState.capsSeen && time.Now().Before(seatDeadline) {
				if lib.displayDispatch(dpy) < 0 {
					break
				}
			}
		}
	}

	host := &wlHost{win: win}
	win.hostRef = host
	ctl := &waylandController{h: host}
	return newWindow(host, PlatformWayland, imeFor(host), nil, ctl, host.destroy), nil
}

// topNoArg marshals a no-argument xdg_toplevel request and flushes.
func (w *wlWin) topNoArg(op uint32) {
	if w == nil || w.lib == nil || w.toplevel == 0 {
		return
	}
	w.lib.proxyMarshalArrayFlags(w.toplevel, op, 0, 0, 0, nil)
	w.lib.displayFlush(w.display)
}

// top2i marshals an "ii" xdg_toplevel request (set_min_size/set_max_size; a
// negative or zero value is sent as-is, 0 = unconstrained per xdg semantics).
func (w *wlWin) top2i(op uint32, a, b int) {
	if w == nil || w.lib == nil || w.toplevel == 0 {
		return
	}
	if a < 0 {
		a = 0
	}
	if b < 0 {
		b = 0
	}
	args := []wlArg{argU(uint32(a)), argU(uint32(b))}
	w.lib.proxyMarshalArrayFlags(w.toplevel, op, 0, 0, 0, &args[0])
	w.lib.displayFlush(w.display)
}

// top1o marshals a "?o" xdg_toplevel request (set_fullscreen → wl_output,
// NULL = current output).
func (w *wlWin) top1o(op uint32, obj uintptr) {
	if w == nil || w.lib == nil || w.toplevel == 0 {
		return
	}
	args := []wlArg{argO(obj)}
	w.lib.proxyMarshalArrayFlags(w.toplevel, op, 0, 0, 0, &args[0])
	w.lib.displayFlush(w.display)
}

// activeCursor returns the controller-set cursor ("" free of side effects).
func (w *wlWin) activeCursor() Cursor {
	if w == nil {
		return CursorDefault
	}
	w.ctlMu.Lock()
	defer w.ctlMu.Unlock()
	return w.cursor
}

// imeFor returns the IME capability for a wayland host, or nil when the
// compositor does not support zwp_text_input_v3 (silent degrade).
func imeFor(h *wlHost) IME {
	if h == nil || h.win == nil || h.win.ti == nil {
		return nil
	}
	return &wlIme{h: h}
}

func (w *wlWin) bind(registry uintptr, name uint32, iface uintptr, version uint32) uintptr {
	if iface == 0 {
		return 0
	}
	iname := (*wlInterfaceC)(unsafe.Pointer(iface)).Name
	args := []wlArg{argU(name), argS(iname), argU(version), argNewID()}
	return w.lib.proxyMarshalArrayCtor(registry, wlRegistryBind, &args[0], iface, version)
}

func (w *wlWin) ctor(proxy uintptr, opcode uint32, iface uintptr, version uint32) uintptr {
	args := []wlArg{argNewID()}
	return w.lib.proxyMarshalArrayCtor(proxy, opcode, &args[0], iface, version)
}

func (w *wlWin) destroyNative() {
	if w == nil {
		return
	}
	lib := w.lib
	if lib == nil {
		return
	}
	if w.ti != nil {
		w.ti.destroy()
		w.ti = nil
	}
	if w.kbd != nil {
		w.kbd.destroy()
		w.kbd = nil
	}
	if w.ptr != nil {
		w.ptr.destroy()
		w.ptr = nil
	}
	if w.csd != nil {
		w.csd.destroy()
		w.csd = nil
	}
	if w.seatState != nil {
		w.seatState.destroy()
		w.seatState = nil
		w.seat = 0
	}
	if w.decoTop != 0 {
		lib.proxyDestroy(w.decoTop)
		w.decoTop = 0
	}
	if w.decoMgr != 0 {
		lib.proxyDestroy(w.decoMgr)
		w.decoMgr = 0
	}
	if w.toplevel != 0 {
		lib.proxyMarshalArrayFlags(w.toplevel, xdgToplevelDestroy, 0, 0, 0, nil)
		lib.proxyDestroy(w.toplevel)
		w.toplevel = 0
	}
	if w.xdgSurf != 0 {
		lib.proxyMarshalArrayFlags(w.xdgSurf, xdgSurfaceDestroy, 0, 0, 0, nil)
		lib.proxyDestroy(w.xdgSurf)
		w.xdgSurf = 0
	}
	if w.surface != 0 {
		lib.proxyMarshalArrayFlags(w.surface, wlSurfaceDestroy, 0, 0, 0, nil)
		lib.proxyDestroy(w.surface)
		w.surface = 0
	}
	if w.wmBase != 0 {
		lib.proxyMarshalArrayFlags(w.wmBase, xdgWmBaseDestroy, 0, 0, 0, nil)
		lib.proxyDestroy(w.wmBase)
		w.wmBase = 0
	}
	if w.comp != 0 {
		lib.proxyDestroy(w.comp)
		w.comp = 0
	}
	if w.registry != 0 {
		lib.proxyDestroy(w.registry)
		w.registry = 0
	}
	if w.display != 0 {
		lib.displayDisconnect(w.display)
		w.display = 0
	}
	if w.selfPtr != 0 {
		wlMu.Lock()
		delete(wlByPtr, w.selfPtr)
		wlMu.Unlock()
		w.selfPtr = 0
	}
}

func winFrom(data uintptr) *wlWin {
	wlMu.Lock()
	defer wlMu.Unlock()
	return wlByPtr[data]
}

func wlRegistryGlobal(data, registry, name, iface, version uintptr) {
	w := winFrom(data)
	if w == nil || iface == 0 {
		return
	}
	s := goString(iface)
	n, v := uint32(name), uint32(version)
	switch s {
	case "wl_compositor":
		w.compName, w.compVer = n, v
	case "xdg_wm_base":
		w.wmName, w.wmVer = n, v
	case "zxdg_decoration_manager_v1":
		w.decoName, w.decoVer = n, v
	case "zwp_text_input_manager_v3":
		w.tiMgrName = n
	case "wl_seat":
		w.seatName = n
	case "wl_shm":
		w.shmName = n
	case "wl_subcompositor":
		w.subcompName = n
	}
	_ = registry
}

func wlRegistryGlobalRemove(data, registry, name uintptr) {}

func wlWmPing(data, wmBase, serial uintptr) {
	w := winFrom(data)
	if w == nil || w.lib == nil {
		return
	}
	args := []wlArg{argU(uint32(serial))}
	w.lib.proxyMarshalArrayFlags(wmBase, xdgWmBasePong, 0, 0, 0, &args[0])
}

func wlXdgConfigure(data, xdgSurf, serial uintptr) {
	w := winFrom(data)
	if w == nil || w.lib == nil {
		return
	}
	args := []wlArg{argU(uint32(serial))}
	w.lib.proxyMarshalArrayFlags(xdgSurf, xdgSurfaceAckConfigure, 0, 0, 0, &args[0])
	w.configured = true
	if w.surface != 0 {
		w.lib.proxyMarshalArrayFlags(w.surface, wlSurfaceCommit, 0, 0, 0, nil)
	}
}

func wlTopConfigure(data, toplevel, width, height, statesArr uintptr) {
	w := winFrom(data)
	if w == nil {
		return
	}
	// xdg_toplevel.configure states is a wl_array of uint32 enum values
	// (1=maximized, 2=fullscreen, 3=resizing, 4=activated, 5..8=tiled,
	// 9=suspended), NOT a bitfield. Parse the array: struct wl_array
	// { size_t size; void *alloc; void *data; } — size@+0, data@+16 (amd64).
	states := wlStatesOf(statesArr)
	if w.csd != nil {
		w.csd.setMaximized(states.maximized)
		w.csd.setFullscreen(states.fullscreen)
		w.csd.setActivated(states.activated)
	}
	// Reconcile configure-driven state with the controller-tracked values
	// (configure is the true value; controller requests are optimistic until
	// the compositor confirms). Activated ⇒ the window was just restored from
	// minimize (protocol has no minimized state → optimistic tracking, §3.2).
	w.ctlMu.Lock()
	w.maximized = states.maximized
	w.fullscreen = states.fullscreen
	if states.activated {
		w.minimized = false
	}
	w.ctlMu.Unlock()
	// The resizing state flips only at drag start/end — not on every step —
	// so it is tracked outside the width/height dedup below.
	w.resizing = states.resizing
	// Report focus + occlusion state changes (values only when changed).
	if states.activated != w.activated {
		w.activated = states.activated
		w.focusEvents = append(w.focusEvents, Event{Type: EventFocus, Focused: states.activated})
	}
	if states.suspended != w.suspended {
		w.suspended = states.suspended
		w.focusEvents = append(w.focusEvents, Event{Type: EventOccluded, Occluded: states.suspended})
	}
	wi, hi := int32(width), int32(height)
	if wi > 0 && hi > 0 {
		if w.width != int(wi) || w.height != int(hi) {
			w.width, w.height = int(wi), int(hi)
			w.resized = true
		}
	}
	_ = toplevel
}

// wlToplevelStates is the decoded xdg_toplevel configure state set.
type wlToplevelStates struct {
	maximized  bool
	fullscreen bool
	resizing   bool
	activated  bool
	tiled      bool
	suspended  bool
}

// wlStatesOf decodes the wl_array (pointer to struct wl_array) into a
// wlToplevelStates. state enum (xdg-shell.xml): 1=maximized, 2=fullscreen,
// 3=resizing, 4=activated, 5..8=tiled, 9=suspended.
func wlStatesOf(arr uintptr) wlToplevelStates {
	var s wlToplevelStates
	if arr == 0 {
		return s
	}
	// struct wl_array: size (size_t, +0), alloc (void*, +8), data (void*, +16).
	size := *(*uint64)(unsafe.Pointer(arr))
	data := *(*uintptr)(unsafe.Pointer(arr + 16))
	if data == 0 {
		return s
	}
	for i := uint64(0); i+4 <= size; i += 4 {
		v := *(*uint32)(unsafe.Pointer(data + uintptr(i)))
		switch v {
		case 1:
			s.maximized = true
		case 2:
			s.fullscreen = true
		case 3:
			s.resizing = true
		case 4:
			s.activated = true
		case 5, 6, 7, 8:
			s.tiled = true
		case 9:
			s.suspended = true
		}
	}
	return s
}

func wlTopClose(data, toplevel uintptr) {
	w := winFrom(data)
	if w != nil {
		// xdg close = a request, not destruction: the app may veto it.
		// Only Window.Close() (→ destroyNative) actually ends the window.
		w.closeReq = true
	}
	_ = toplevel
}

func goString(p uintptr) string {
	if p == 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < 256; i++ {
		c := *(*byte)(unsafe.Pointer(p + uintptr(i)))
		if c == 0 {
			break
		}
		b.WriteByte(c)
	}
	return b.String()
}

func minU32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// --- Host (event pump) ---

type wlHost struct {
	win   *wlWin
	mu    sync.Mutex
	scale float64

	// displayFD is wl_display_get_fd (cached at first use).
	displayFD int
	// wakePipe[0]=read, wakePipe[1]=write — unblocks poll on WakeUp.
	wakePipe [2]int
}

func (h *wlHost) ensureWakePipe() {
	if h == nil {
		return
	}
	if h.wakePipe[0] != 0 || h.wakePipe[1] != 0 {
		return
	}
	p := [2]int{-1, -1}
	if err := unix.Pipe2(p[:], unix.O_NONBLOCK|unix.O_CLOEXEC); err == nil {
		h.wakePipe = p
	}
}

func (h *wlHost) drainWake() {
	if h == nil || h.wakePipe[0] == 0 {
		return
	}
	var buf [16]byte
	for {
		n, err := unix.Read(h.wakePipe[0], buf[:])
		if n <= 0 || err != nil {
			break
		}
	}
}

// hostForWake returns the event-pump host for this window (nil until
// waylandCreate installs it). Seat callbacks created by waylandCreate can
// only fire from a later dispatch, by which time hostRef is set, so the nil
// case is defensive only.
func (w *wlWin) hostForWake() *wlHost {
	if w == nil {
		return nil
	}
	return w.hostRef
}

func (h *wlHost) destroy() {
	if h == nil || h.win == nil {
		return
	}
	if h.wakePipe[0] != 0 {
		_ = unix.Close(h.wakePipe[0])
		h.wakePipe[0] = 0
	}
	if h.wakePipe[1] != 0 {
		_ = unix.Close(h.wakePipe[1])
		h.wakePipe[1] = 0
	}
	h.win.destroyNative()
	h.win = nil
}

func (h *wlHost) NativeSurface() NativeSurface {
	if h == nil || h.win == nil {
		return NativeSurface{Kind: PlatformWayland}
	}
	return NativeSurface{
		Kind:    PlatformWayland,
		Display: h.win.display,
		Window:  h.win.surface,
	}
}

func (h *wlHost) Size() (int, int) {
	if h == nil || h.win == nil {
		return 1, 1
	}
	return h.win.width, h.win.height
}

func (h *wlHost) ScaleFactor() float64 {
	if h == nil {
		return 1
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.scale <= 0 {
		return 1
	}
	return h.scale
}

// WaitVSync uses DRM vblank when available; scheduler falls back to software.
func (h *wlHost) WaitVSync() error { return WaitDRMVBlank() }

func (h *wlHost) WaitEvents(timeout time.Duration) []Event {
	if h == nil || h.win == nil {
		return nil
	}
	w := h.win
	if w.lib == nil || w.display == 0 {
		return nil
	}
	// Dispatch anything already buffered, then send pending requests.
	if evs := h.poll(); len(evs) > 0 {
		return evs
	}
	if timeout < 0 {
		timeout = 16 * time.Millisecond
	}
	if timeout == 0 {
		return h.poll()
	}
	h.ensureWakePipe()

	// Standard Wayland client wait protocol:
	//   wl_display_prepare_read → poll(display fd, wake pipe)
	//   → on fd readable: wl_display_read_events + dispatch_pending
	fd := h.displayFD
	if fd == 0 {
		fd = w.lib.displayGetFD(w.display)
		h.displayFD = fd
	}
	if fd <= 0 {
		// No fd (connection lost?) — fall back to timed poll.
		select {
		case <-time.After(timeout):
		}
		return h.poll()
	}

	deadline := time.Now().Add(timeout)
	for {
		left := time.Until(deadline)
		if left <= 0 {
			break
		}
		if left > 16*time.Millisecond {
			left = 16 * time.Millisecond
		}
		ms := int(left.Milliseconds())
		if ms < 1 {
			ms = 1
		}

		// Acquire the read lock; dispatch anything pending first.
		if w.lib.displayPrepareRead(w.display) != 0 {
			// Events already buffered; dispatch them and keep waiting.
			if evs := h.poll(); len(evs) > 0 {
				return evs
			}
			continue
		}
		// Flush our requests before blocking.
		w.lib.displayFlush(w.display)

		pfds := []unix.PollFd{
			{Fd: int32(fd), Events: unix.POLLIN},
			{Fd: int32(h.wakePipe[0]), Events: unix.POLLIN},
		}
		n, err := unix.Poll(pfds, ms)
		if err != nil {
			w.lib.displayCancelRead(w.display)
			if err == unix.EINTR {
				continue
			}
			break
		}
		if n == 0 {
			// Timeout.
			w.lib.displayCancelRead(w.display)
			break
		}
		if pfds[1].Revents&unix.POLLIN != 0 {
			// Woken up (thread wants us to re-poll / check quit).
			h.drainWake()
			w.lib.displayCancelRead(w.display)
			if evs := h.poll(); len(evs) > 0 {
				return evs
			}
			return []Event{{Type: EventWake}}
		}
		if pfds[0].Revents&unix.POLLIN != 0 {
			if w.lib.displayReadEvents(w.display) < 0 {
				w.lib.displayCancelRead(w.display)
				break
			}
			if evs := h.poll(); len(evs) > 0 {
				return evs
			}
			continue
		}
		if pfds[0].Revents&(unix.POLLHUP|unix.POLLERR) != 0 {
			w.lib.displayCancelRead(w.display)
			break
		}
		w.lib.displayCancelRead(w.display)
	}
	return h.poll()
}

func (h *wlHost) poll() []Event {
	w := h.win
	if w == nil || w.lib == nil || w.display == 0 {
		return nil
	}
	w.lib.displayDispatchPend(w.display)
	w.lib.displayFlush(w.display)
	var out []Event
	if w.closed {
		out = append(out, Event{Type: EventClose})
	}
	// CSD ✕ / xdg close → EventCloseRequested (interceptable); the window
	// stays alive unless the app calls Close(). Real destruction (surface
	// gone / Close()) reports EventClose separately.
	if w.closeReq {
		w.closeReq = false
		out = append(out, Event{Type: EventCloseRequested})
	} else if w.csd != nil && w.csd.closeRequested {
		w.csd.closeRequested = false
		out = append(out, Event{Type: EventCloseRequested})
	}
	if w.resized {
		w.resized = false
		// Keep CSD decoration surfaces sized to the new content area.
		if w.csd != nil {
			w.csd.resize(w.width, w.height)
		}
		out = append(out, Event{
			Type: EventResize, Width: w.width, Height: w.height, Scale: h.ScaleFactor(),
		})
	}
	// IME events queued by the text-input callbacks.
	w.imeMu.Lock()
	if len(w.imeEvents) > 0 {
		out = append(out, w.imeEvents...)
		w.imeEvents = nil
	}
	w.imeMu.Unlock()
	// Key events queued by the wl_keyboard callback.
	w.keyMu.Lock()
	if len(w.keyEvents) > 0 {
		out = append(out, w.keyEvents...)
		w.keyEvents = nil
	}
	w.keyMu.Unlock()
	// Pointer events queued by the wl_pointer callback.
	w.ptrMu.Lock()
	if len(w.ptrEvents) > 0 {
		out = append(out, w.ptrEvents...)
		w.ptrEvents = nil
	}
	w.ptrMu.Unlock()
	// Focus / occlusion events queued by wlTopConfigure.
	w.focusMu.Lock()
	if len(w.focusEvents) > 0 {
		out = append(out, w.focusEvents...)
		w.focusEvents = nil
	}
	w.focusMu.Unlock()
	return out
}

func (h *wlHost) WakeUp() {
	if h == nil {
		return
	}
	h.ensureWakePipe()
	if h.wakePipe[1] == 0 {
		return
	}
	// Non-blocking: one byte is enough to unblock poll.
	buf := []byte{1}
	_, _ = unix.Write(h.wakePipe[1], buf)
}
