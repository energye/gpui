//go:build linux && !nouiplatform

package platform

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// WaylandHost is a minimal Wayland window Host (purego, no cgo).
// Ported from the exhost reference: xdg-shell + optional server-side decorations.
type WaylandHost struct {
	mu sync.Mutex

	lib *wlWaylandLib

	display  uintptr
	registry uintptr
	comp     uintptr
	surface  uintptr
	wmBase   uintptr
	xdgSurf  uintptr
	toplevel uintptr

	// registry bind targets
	compName, compVer uint32
	wmName, wmVer     uint32
	decoName          uint32
	decoMgr           uintptr
	decoTop           uintptr

	// seat (optional — no seat / zero caps is OK)
	seatName, seatVer uint32
	seat              uintptr
	pointer           uintptr
	keyboard          uintptr

	width, height int
	scale         float64
	title         string

	configured bool
	closed     bool
	resized    bool
	redraw     bool

	// input state (seat callbacks)
	ptrX, ptrY        float64
	modShift, modCtrl bool
	modAlt, modMeta   bool
	queue             []Event

	// pin listener callback tables + user data
	regListener  [2]uintptr
	wmListener   [1]uintptr
	xdgListener  [1]uintptr
	topListener  [2]uintptr
	seatListener [2]uintptr
	ptrListener  [12]uintptr // v7=9 events; pad for v8+ axis_value120 / relative
	keyListener  [8]uintptr  // v7=6; pad for future
	selfPtr      uintptr

	titlePin []byte
	appPin   []byte

	wake chan struct{}
	clip Clipboard
}

// WaylandOptions configures NewWaylandHost.
type WaylandOptions struct {
	Width, Height int
	Title         string
	Scale         float64
}

type wlWaylandLib struct {
	lib uintptr

	displayConnect         func(name *byte) uintptr
	displayDisconnect      func(d uintptr)
	displayDispatch        func(d uintptr) int
	displayDispatchPend    func(d uintptr) int
	displayFlush           func(d uintptr) int
	displayRoundtrip       func(d uintptr) int
	proxyAddListener       func(proxy uintptr, impl uintptr, data uintptr) int
	proxyMarshalArrayCtor  func(proxy uintptr, opcode uint32, args *wlArg, iface uintptr, version uint32) uintptr
	proxyMarshalArrayFlags func(proxy uintptr, opcode uint32, iface uintptr, version uint32, flags uint32, args *wlArg) uintptr
	proxyDestroy           func(proxy uintptr)
	proxyGetVersion        func(proxy uintptr) uint32
	displayGetFD           func(d uintptr) int
	displayPrepareRead     func(d uintptr) int
	displayReadEvents      func(d uintptr) int
	displayCancelRead      func(d uintptr) int

	ifaceCompositor uintptr
	ifaceSurface    uintptr
	ifaceRegistry   uintptr
	ifaceSeat       uintptr
	ifacePointer    uintptr
	ifaceKeyboard   uintptr
}

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

const (
	wlRegistryBind            = 0
	wlCompositorCreateSurface = 0
	wlSurfaceDestroy          = 0
	wlSurfaceCommit           = 6
	xdgWmBaseGetXdgSurface    = 2
	xdgWmBasePong             = 3
	xdgSurfaceGetToplevel     = 1
	xdgSurfaceAckConfigure    = 4
	xdgToplevelSetTitle       = 2
	xdgToplevelSetAppID       = 3
	xdgToplevelDestroy        = 0
	xdgSurfaceDestroy         = 0
	xdgWmBaseDestroy          = 0
	xdgDecoMgrGetDecoration   = 1
	xdgDecoSetMode            = 1
	xdgDecoModeServerSide     = 2 // SSD title bar when compositor supports it
)

// Keep C strings and message tables alive for the process lifetime.
var (
	wlXdgNames = struct {
		wmBase, surface, toplevel []byte
		decoMgr, decoTop          []byte
		mDestroy, mGetXdg, mPong  []byte
		mGetTop, mAck             []byte
		mSetTitle, mSetApp        []byte
		mGetDeco, mSetMode        []byte
		sEmpty, sNo, sU, sN, sS   []byte
		sNoTop                    []byte
		ePing, eCfg, eClose       []byte
		eCfgIia                   []byte
	}{
		wmBase:    append([]byte("xdg_wm_base"), 0),
		surface:   append([]byte("xdg_surface"), 0),
		toplevel:  append([]byte("xdg_toplevel"), 0),
		decoMgr:   append([]byte("zxdg_decoration_manager_v1"), 0),
		decoTop:   append([]byte("zxdg_toplevel_decoration_v1"), 0),
		mDestroy:  append([]byte("destroy"), 0),
		mGetXdg:   append([]byte("get_xdg_surface"), 0),
		mPong:     append([]byte("pong"), 0),
		mGetTop:   append([]byte("get_toplevel"), 0),
		mAck:      append([]byte("ack_configure"), 0),
		mSetTitle: append([]byte("set_title"), 0),
		mSetApp:   append([]byte("set_app_id"), 0),
		mGetDeco:  append([]byte("get_toplevel_decoration"), 0),
		mSetMode:  append([]byte("set_mode"), 0),
		sEmpty:    append([]byte(""), 0),
		sNo:       append([]byte("no"), 0),
		sU:        append([]byte("u"), 0),
		sN:        append([]byte("n"), 0),
		sS:        append([]byte("s"), 0),
		sNoTop:    append([]byte("no"), 0),
		ePing:     append([]byte("ping"), 0),
		eCfg:      append([]byte("configure"), 0),
		eClose:    append([]byte("close"), 0),
		eCfgIia:   append([]byte("iia"), 0),
	}

	wlIfaceXdgWmBase   wlInterfaceC
	wlIfaceXdgSurface  wlInterfaceC
	wlIfaceXdgToplevel wlInterfaceC
	wlIfaceDecoMgr     wlInterfaceC
	wlIfaceDecoTop     wlInterfaceC

	wlMsgWmBase    [4]wlMessageC
	wlMsgWmBaseEv  [1]wlMessageC
	wlMsgXdgSurf   [5]wlMessageC
	wlMsgXdgSurfEv [1]wlMessageC
	wlMsgTop       [4]wlMessageC
	wlMsgTopEv     [2]wlMessageC
	wlMsgDecoMgr   [2]wlMessageC
	wlMsgDecoTop   [3]wlMessageC
	wlMsgDecoTopEv [1]wlMessageC

	wlTypesXdgSurf [2]uintptr
	wlTypesTop     [1]uintptr
	wlTypesDeco    [2]uintptr

	wlMu    sync.Mutex
	wlByPtr = map[uintptr]*WaylandHost{}
)

func wlCstr(b []byte) uintptr { return uintptr(unsafe.Pointer(&b[0])) }

func initXDGInterfaces(ifaceSurface uintptr) {
	wlMsgWmBase[0] = wlMessageC{Name: wlCstr(wlXdgNames.mDestroy), Signature: wlCstr(wlXdgNames.sEmpty), Types: 0}
	wlMsgWmBase[1] = wlMessageC{Name: wlCstr(wlXdgNames.mDestroy), Signature: wlCstr(wlXdgNames.sN), Types: 0} // positioner stub
	wlTypesXdgSurf[0] = 0
	wlTypesXdgSurf[1] = ifaceSurface
	wlMsgWmBase[2] = wlMessageC{Name: wlCstr(wlXdgNames.mGetXdg), Signature: wlCstr(wlXdgNames.sNo), Types: uintptr(unsafe.Pointer(&wlTypesXdgSurf[0]))}
	wlMsgWmBase[3] = wlMessageC{Name: wlCstr(wlXdgNames.mPong), Signature: wlCstr(wlXdgNames.sU), Types: 0}
	wlMsgWmBaseEv[0] = wlMessageC{Name: wlCstr(wlXdgNames.ePing), Signature: wlCstr(wlXdgNames.sU), Types: 0}
	wlIfaceXdgWmBase = wlInterfaceC{
		Name: wlCstr(wlXdgNames.wmBase), Version: 2,
		MethodCount: 4, Methods: uintptr(unsafe.Pointer(&wlMsgWmBase[0])),
		EventCount: 1, Events: uintptr(unsafe.Pointer(&wlMsgWmBaseEv[0])),
	}

	wlTypesTop[0] = 0
	wlMsgXdgSurf[0] = wlMessageC{Name: wlCstr(wlXdgNames.mDestroy), Signature: wlCstr(wlXdgNames.sEmpty), Types: 0}
	wlMsgXdgSurf[1] = wlMessageC{Name: wlCstr(wlXdgNames.mGetTop), Signature: wlCstr(wlXdgNames.sN), Types: uintptr(unsafe.Pointer(&wlTypesTop[0]))}
	wlMsgXdgSurf[2] = wlMessageC{Name: wlCstr(wlXdgNames.mDestroy), Signature: wlCstr(wlXdgNames.sEmpty), Types: 0} // popup stub
	wlMsgXdgSurf[3] = wlMessageC{Name: wlCstr(wlXdgNames.mDestroy), Signature: wlCstr(wlXdgNames.sEmpty), Types: 0} // geometry stub
	wlMsgXdgSurf[4] = wlMessageC{Name: wlCstr(wlXdgNames.mAck), Signature: wlCstr(wlXdgNames.sU), Types: 0}
	wlMsgXdgSurfEv[0] = wlMessageC{Name: wlCstr(wlXdgNames.eCfg), Signature: wlCstr(wlXdgNames.sU), Types: 0}
	wlIfaceXdgSurface = wlInterfaceC{
		Name: wlCstr(wlXdgNames.surface), Version: 2,
		MethodCount: 5, Methods: uintptr(unsafe.Pointer(&wlMsgXdgSurf[0])),
		EventCount: 1, Events: uintptr(unsafe.Pointer(&wlMsgXdgSurfEv[0])),
	}
	wlTypesXdgSurf[0] = uintptr(unsafe.Pointer(&wlIfaceXdgSurface))

	wlMsgTop[0] = wlMessageC{Name: wlCstr(wlXdgNames.mDestroy), Signature: wlCstr(wlXdgNames.sEmpty), Types: 0}
	wlMsgTop[1] = wlMessageC{Name: wlCstr(wlXdgNames.mDestroy), Signature: wlCstr(wlXdgNames.sEmpty), Types: 0} // set_parent stub
	wlMsgTop[2] = wlMessageC{Name: wlCstr(wlXdgNames.mSetTitle), Signature: wlCstr(wlXdgNames.sS), Types: 0}
	wlMsgTop[3] = wlMessageC{Name: wlCstr(wlXdgNames.mSetApp), Signature: wlCstr(wlXdgNames.sS), Types: 0}
	wlMsgTopEv[0] = wlMessageC{Name: wlCstr(wlXdgNames.eCfg), Signature: wlCstr(wlXdgNames.eCfgIia), Types: 0}
	wlMsgTopEv[1] = wlMessageC{Name: wlCstr(wlXdgNames.eClose), Signature: wlCstr(wlXdgNames.sEmpty), Types: 0}
	wlIfaceXdgToplevel = wlInterfaceC{
		Name: wlCstr(wlXdgNames.toplevel), Version: 2,
		MethodCount: 4, Methods: uintptr(unsafe.Pointer(&wlMsgTop[0])),
		EventCount: 2, Events: uintptr(unsafe.Pointer(&wlMsgTopEv[0])),
	}
	wlTypesTop[0] = uintptr(unsafe.Pointer(&wlIfaceXdgToplevel))

	// zxdg_decoration_manager_v1 / zxdg_toplevel_decoration_v1 (SSD title bar).
	wlMsgDecoTop[0] = wlMessageC{Name: wlCstr(wlXdgNames.mDestroy), Signature: wlCstr(wlXdgNames.sEmpty), Types: 0}
	wlMsgDecoTop[1] = wlMessageC{Name: wlCstr(wlXdgNames.mSetMode), Signature: wlCstr(wlXdgNames.sU), Types: 0}
	wlMsgDecoTop[2] = wlMessageC{Name: wlCstr(wlXdgNames.mDestroy), Signature: wlCstr(wlXdgNames.sEmpty), Types: 0} // unset_mode stub
	wlMsgDecoTopEv[0] = wlMessageC{Name: wlCstr(wlXdgNames.eCfg), Signature: wlCstr(wlXdgNames.sU), Types: 0}
	wlIfaceDecoTop = wlInterfaceC{
		Name: wlCstr(wlXdgNames.decoTop), Version: 1,
		MethodCount: 3, Methods: uintptr(unsafe.Pointer(&wlMsgDecoTop[0])),
		EventCount: 1, Events: uintptr(unsafe.Pointer(&wlMsgDecoTopEv[0])),
	}
	wlTypesDeco[0] = uintptr(unsafe.Pointer(&wlIfaceDecoTop))
	wlTypesDeco[1] = uintptr(unsafe.Pointer(&wlIfaceXdgToplevel))
	wlMsgDecoMgr[0] = wlMessageC{Name: wlCstr(wlXdgNames.mDestroy), Signature: wlCstr(wlXdgNames.sEmpty), Types: 0}
	wlMsgDecoMgr[1] = wlMessageC{Name: wlCstr(wlXdgNames.mGetDeco), Signature: wlCstr(wlXdgNames.sNoTop), Types: uintptr(unsafe.Pointer(&wlTypesDeco[0]))}
	wlIfaceDecoMgr = wlInterfaceC{
		Name: wlCstr(wlXdgNames.decoMgr), Version: 1,
		MethodCount: 2, Methods: uintptr(unsafe.Pointer(&wlMsgDecoMgr[0])),
		EventCount: 0, Events: 0,
	}
}

func loadWaylandLib() (*wlWaylandLib, error) {
	lib, err := purego.Dlopen("libwayland-client.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libwayland-client.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		return nil, fmt.Errorf("platform/wayland: dlopen libwayland-client: %w", err)
	}
	l := &wlWaylandLib{lib: lib}
	purego.RegisterLibFunc(&l.displayConnect, lib, "wl_display_connect")
	purego.RegisterLibFunc(&l.displayDisconnect, lib, "wl_display_disconnect")
	purego.RegisterLibFunc(&l.displayDispatch, lib, "wl_display_dispatch")
	purego.RegisterLibFunc(&l.displayDispatchPend, lib, "wl_display_dispatch_pending")
	purego.RegisterLibFunc(&l.displayFlush, lib, "wl_display_flush")
	purego.RegisterLibFunc(&l.displayRoundtrip, lib, "wl_display_roundtrip")
	purego.RegisterLibFunc(&l.proxyAddListener, lib, "wl_proxy_add_listener")
	purego.RegisterLibFunc(&l.proxyMarshalArrayCtor, lib, "wl_proxy_marshal_array_constructor_versioned")
	purego.RegisterLibFunc(&l.proxyMarshalArrayFlags, lib, "wl_proxy_marshal_array_flags")
	purego.RegisterLibFunc(&l.proxyDestroy, lib, "wl_proxy_destroy")
	purego.RegisterLibFunc(&l.proxyGetVersion, lib, "wl_proxy_get_version")
	purego.RegisterLibFunc(&l.displayGetFD, lib, "wl_display_get_fd")
	purego.RegisterLibFunc(&l.displayPrepareRead, lib, "wl_display_prepare_read")
	purego.RegisterLibFunc(&l.displayReadEvents, lib, "wl_display_read_events")
	purego.RegisterLibFunc(&l.displayCancelRead, lib, "wl_display_cancel_read")

	for _, pair := range []struct {
		name string
		dst  *uintptr
		opt  bool // seat family optional (headless compositors)
	}{
		{"wl_compositor_interface", &l.ifaceCompositor, false},
		{"wl_surface_interface", &l.ifaceSurface, false},
		{"wl_registry_interface", &l.ifaceRegistry, false},
		{"wl_seat_interface", &l.ifaceSeat, true},
		{"wl_pointer_interface", &l.ifacePointer, true},
		{"wl_keyboard_interface", &l.ifaceKeyboard, true},
	} {
		p, err := purego.Dlsym(lib, pair.name)
		if err != nil || p == 0 {
			if pair.opt {
				continue
			}
			return nil, fmt.Errorf("platform/wayland: Dlsym %s: %v", pair.name, err)
		}
		*pair.dst = p
	}
	return l, nil
}

func (l *wlWaylandLib) displayGetRegistry(dpy uintptr) uintptr {
	args := []wlArg{argNewID()}
	ver := l.proxyGetVersion(dpy)
	if ver == 0 {
		ver = 1
	}
	return l.proxyMarshalArrayCtor(dpy, 1 /* WL_DISPLAY_GET_REGISTRY */, &args[0], l.ifaceRegistry, ver)
}

// NewWaylandHost opens a Wayland toplevel via xdg-shell (purego).
func NewWaylandHost(opts WaylandOptions) (*WaylandHost, error) {
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
	if !HasWaylandDisplay() {
		return nil, fmt.Errorf("platform/wayland: WAYLAND_DISPLAY not set")
	}
	lib, err := loadWaylandLib()
	if err != nil {
		return nil, err
	}
	initXDGInterfaces(lib.ifaceSurface)

	h := &WaylandHost{
		lib:    lib,
		width:  opts.Width,
		height: opts.Height,
		scale:  opts.Scale,
		title:  opts.Title,
		wake:   make(chan struct{}, 1),
		clip:   NewSystemClipboard(),
		redraw: true,
	}

	dpy := lib.displayConnect(nil)
	if dpy == 0 {
		return nil, fmt.Errorf("platform/wayland: wl_display_connect failed")
	}
	h.display = dpy

	h.selfPtr = uintptr(unsafe.Pointer(h))
	wlMu.Lock()
	wlByPtr[h.selfPtr] = h
	wlMu.Unlock()

	h.regListener[0] = purego.NewCallback(wlRegistryGlobal)
	h.regListener[1] = purego.NewCallback(wlRegistryGlobalRemove)

	reg := lib.displayGetRegistry(dpy)
	if reg == 0 {
		h.destroyNative()
		return nil, fmt.Errorf("platform/wayland: wl_display_get_registry failed")
	}
	h.registry = reg
	lib.proxyAddListener(reg, uintptr(unsafe.Pointer(&h.regListener[0])), h.selfPtr)
	lib.displayRoundtrip(dpy)

	if h.compName == 0 || h.wmName == 0 {
		h.destroyNative()
		return nil, fmt.Errorf("platform/wayland: missing wl_compositor or xdg_wm_base")
	}

	h.comp = h.bind(reg, h.compName, lib.ifaceCompositor, minU32(h.compVer, 4))
	if h.comp == 0 {
		h.destroyNative()
		return nil, fmt.Errorf("platform/wayland: bind wl_compositor failed")
	}

	h.wmBase = h.bind(reg, h.wmName, uintptr(unsafe.Pointer(&wlIfaceXdgWmBase)), minU32(h.wmVer, 2))
	if h.wmBase == 0 {
		h.destroyNative()
		return nil, fmt.Errorf("platform/wayland: bind xdg_wm_base failed")
	}
	h.wmListener[0] = purego.NewCallback(wlWmPing)
	lib.proxyAddListener(h.wmBase, uintptr(unsafe.Pointer(&h.wmListener[0])), h.selfPtr)

	h.surface = h.ctor(h.comp, wlCompositorCreateSurface, lib.ifaceSurface, 4)
	if h.surface == 0 {
		h.destroyNative()
		return nil, fmt.Errorf("platform/wayland: create wl_surface failed")
	}

	{
		args := []wlArg{argNewID(), argO(h.surface)}
		h.xdgSurf = lib.proxyMarshalArrayCtor(h.wmBase, xdgWmBaseGetXdgSurface, &args[0],
			uintptr(unsafe.Pointer(&wlIfaceXdgSurface)), 2)
	}
	if h.xdgSurf == 0 {
		h.destroyNative()
		return nil, fmt.Errorf("platform/wayland: get_xdg_surface failed")
	}
	h.xdgListener[0] = purego.NewCallback(wlXdgConfigure)
	lib.proxyAddListener(h.xdgSurf, uintptr(unsafe.Pointer(&h.xdgListener[0])), h.selfPtr)

	{
		args := []wlArg{argNewID()}
		h.toplevel = lib.proxyMarshalArrayCtor(h.xdgSurf, xdgSurfaceGetToplevel, &args[0],
			uintptr(unsafe.Pointer(&wlIfaceXdgToplevel)), 2)
	}
	if h.toplevel == 0 {
		h.destroyNative()
		return nil, fmt.Errorf("platform/wayland: get_toplevel failed")
	}
	h.topListener[0] = purego.NewCallback(wlTopConfigure)
	h.topListener[1] = purego.NewCallback(wlTopClose)
	lib.proxyAddListener(h.toplevel, uintptr(unsafe.Pointer(&h.topListener[0])), h.selfPtr)

	h.titlePin = append([]byte(opts.Title), 0)
	h.appPin = append([]byte("gpui"), 0)
	{
		args := []wlArg{argS(wlCstr(h.titlePin))}
		lib.proxyMarshalArrayFlags(h.toplevel, xdgToplevelSetTitle, 0, 0, 0, &args[0])
	}
	{
		args := []wlArg{argS(wlCstr(h.appPin))}
		lib.proxyMarshalArrayFlags(h.toplevel, xdgToplevelSetAppID, 0, 0, 0, &args[0])
	}

	// Request server-side decorations when the compositor supports it
	// (KDE/Sway/Hyprland; GNOME typically does not).
	if h.decoName != 0 {
		h.decoMgr = h.bind(reg, h.decoName, uintptr(unsafe.Pointer(&wlIfaceDecoMgr)), 1)
		if h.decoMgr != 0 {
			args := []wlArg{argNewID(), argO(h.toplevel)}
			h.decoTop = lib.proxyMarshalArrayCtor(h.decoMgr, xdgDecoMgrGetDecoration, &args[0],
				uintptr(unsafe.Pointer(&wlIfaceDecoTop)), 1)
			if h.decoTop != 0 {
				mode := []wlArg{argU(xdgDecoModeServerSide)}
				lib.proxyMarshalArrayFlags(h.decoTop, xdgDecoSetMode, 0, 0, 0, &mode[0])
			}
		}
	}

	// Seat input is optional: no global / zero capabilities still maps the window.
	h.setupSeat(reg)

	lib.proxyMarshalArrayFlags(h.surface, wlSurfaceCommit, 0, 0, 0, nil)

	deadline := time.Now().Add(3 * time.Second)
	for !h.configured && !h.closed && time.Now().Before(deadline) {
		if lib.displayDispatch(dpy) < 0 {
			break
		}
	}
	if !h.configured {
		h.destroyNative()
		return nil, fmt.Errorf("platform/wayland: xdg configure timeout")
	}
	runtime.KeepAlive(h)
	return h, nil
}

func (h *WaylandHost) bind(registry uintptr, name uint32, iface uintptr, version uint32) uintptr {
	if iface == 0 || h.lib == nil {
		return 0
	}
	iname := (*wlInterfaceC)(unsafe.Pointer(iface)).Name
	args := []wlArg{argU(name), argS(iname), argU(version), argNewID()}
	return h.lib.proxyMarshalArrayCtor(registry, wlRegistryBind, &args[0], iface, version)
}

func (h *WaylandHost) ctor(proxy uintptr, opcode uint32, iface uintptr, version uint32) uintptr {
	args := []wlArg{argNewID()}
	return h.lib.proxyMarshalArrayCtor(proxy, opcode, &args[0], iface, version)
}

func (h *WaylandHost) destroyNative() {
	if h == nil || h.lib == nil {
		return
	}
	lib := h.lib
	h.destroySeat()
	if h.decoTop != 0 {
		lib.proxyDestroy(h.decoTop)
		h.decoTop = 0
	}
	if h.decoMgr != 0 {
		lib.proxyDestroy(h.decoMgr)
		h.decoMgr = 0
	}
	if h.toplevel != 0 {
		lib.proxyMarshalArrayFlags(h.toplevel, xdgToplevelDestroy, 0, 0, 0, nil)
		lib.proxyDestroy(h.toplevel)
		h.toplevel = 0
	}
	if h.xdgSurf != 0 {
		lib.proxyMarshalArrayFlags(h.xdgSurf, xdgSurfaceDestroy, 0, 0, 0, nil)
		lib.proxyDestroy(h.xdgSurf)
		h.xdgSurf = 0
	}
	if h.surface != 0 {
		lib.proxyMarshalArrayFlags(h.surface, wlSurfaceDestroy, 0, 0, 0, nil)
		lib.proxyDestroy(h.surface)
		h.surface = 0
	}
	if h.wmBase != 0 {
		lib.proxyMarshalArrayFlags(h.wmBase, xdgWmBaseDestroy, 0, 0, 0, nil)
		lib.proxyDestroy(h.wmBase)
		h.wmBase = 0
	}
	if h.comp != 0 {
		lib.proxyDestroy(h.comp)
		h.comp = 0
	}
	if h.registry != 0 {
		lib.proxyDestroy(h.registry)
		h.registry = 0
	}
	if h.display != 0 {
		lib.displayDisconnect(h.display)
		h.display = 0
	}
	if h.selfPtr != 0 {
		wlMu.Lock()
		delete(wlByPtr, h.selfPtr)
		wlMu.Unlock()
		h.selfPtr = 0
	}
}

func wlWinFrom(data uintptr) *WaylandHost {
	wlMu.Lock()
	defer wlMu.Unlock()
	return wlByPtr[data]
}

func wlRegistryGlobal(data, registry, name, iface, version uintptr) {
	h := wlWinFrom(data)
	if h == nil || iface == 0 {
		return
	}
	s := wlGoString(iface)
	n, v := uint32(name), uint32(version)
	switch s {
	case "wl_compositor":
		h.compName, h.compVer = n, v
	case "xdg_wm_base":
		h.wmName, h.wmVer = n, v
	case "zxdg_decoration_manager_v1":
		h.decoName = n
		_ = v
	case "wl_seat":
		// First seat only (multi-seat later).
		if h.seatName == 0 {
			h.seatName, h.seatVer = n, v
		}
	}
	_ = registry
}

func wlRegistryGlobalRemove(data, registry, name uintptr) {}

func wlWmPing(data, wmBase, serial uintptr) {
	h := wlWinFrom(data)
	if h == nil || h.lib == nil {
		return
	}
	args := []wlArg{argU(uint32(serial))}
	h.lib.proxyMarshalArrayFlags(wmBase, xdgWmBasePong, 0, 0, 0, &args[0])
}

func wlXdgConfigure(data, xdgSurf, serial uintptr) {
	h := wlWinFrom(data)
	if h == nil || h.lib == nil {
		return
	}
	args := []wlArg{argU(uint32(serial))}
	h.lib.proxyMarshalArrayFlags(xdgSurf, xdgSurfaceAckConfigure, 0, 0, 0, &args[0])
	h.mu.Lock()
	h.configured = true
	h.mu.Unlock()
	if h.surface != 0 {
		h.lib.proxyMarshalArrayFlags(h.surface, wlSurfaceCommit, 0, 0, 0, nil)
	}
}

func wlTopConfigure(data, toplevel, width, height, states uintptr) {
	h := wlWinFrom(data)
	if h == nil {
		return
	}
	wi, hi := int32(width), int32(height)
	h.mu.Lock()
	if wi > 0 && hi > 0 {
		if h.width != int(wi) || h.height != int(hi) {
			h.width, h.height = int(wi), int(hi)
			h.resized = true
		}
	}
	h.mu.Unlock()
	_ = toplevel
	_ = states
}

func wlTopClose(data, toplevel uintptr) {
	h := wlWinFrom(data)
	if h != nil {
		h.mu.Lock()
		h.closed = true
		h.mu.Unlock()
	}
	_ = toplevel
}

func wlGoString(p uintptr) string {
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

// --- platform.Host ---

func (h *WaylandHost) Caps() Caps {
	// CapIME deferred (no xkbcommon / text-input protocol yet).
	// CapPointer / CapKeyboard advertised so UI routes work; actual devices may
	// appear later via seat capabilities (idle with no peripherals is OK).
	return CapWindow | CapPointer | CapKeyboard | CapTextInput |
		CapPresent | CapSurfaceLifecycle | CapClipboard
}

func (h *WaylandHost) Size() (int, int) {
	if h == nil {
		return 1, 1
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.width, h.height
}

func (h *WaylandHost) ScaleFactor() float64 {
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

func (h *WaylandHost) Display() uintptr {
	if h == nil {
		return 0
	}
	return h.display
}

func (h *WaylandHost) Window() uintptr {
	if h == nil {
		return 0
	}
	return h.surface
}

func (h *WaylandHost) Backend() DisplayBackend { return DisplayWayland }

func (h *WaylandHost) NativeSurface() NativeSurface {
	if h == nil {
		return NativeSurface{Kind: PlatformWayland}
	}
	return NativeSurface{
		Kind:    PlatformWayland,
		Display: h.display,
		Window:  h.surface,
	}
}

func (h *WaylandHost) Clipboard() Clipboard {
	if h == nil {
		return nil
	}
	if h.clip == nil {
		h.clip = NewSystemClipboard()
	}
	return h.clip
}

func (h *WaylandHost) PumpEvents() []Event { return h.WaitEvents(0) }

func (h *WaylandHost) WaitEvents(timeout time.Duration) []Event {
	if h == nil {
		return nil
	}
	if h.wake == nil {
		h.wake = make(chan struct{}, 1)
	}
	if evs := h.pollWith(0); len(evs) > 0 {
		return evs
	}
	if timeout == 0 {
		return h.pollWith(0)
	}
	// Timed / "block" via short fd waits so WakeUp and seat input both progress.
	if timeout < 0 {
		timeout = 16 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	const slice = 16 * time.Millisecond
	for {
		wait := slice
		left := time.Until(deadline)
		if left <= 0 {
			return h.pollWith(0)
		}
		if left < wait {
			wait = left
		}
		if evs := h.pollWith(wait); len(evs) > 0 {
			return evs
		}
		select {
		case <-h.wake:
			if evs := h.pollWith(0); len(evs) > 0 {
				return evs
			}
			return []Event{{Type: EventWake}}
		default:
		}
	}
}

func (h *WaylandHost) pollWith(fdWait time.Duration) []Event {
	if h.lib == nil || h.display == 0 {
		return nil
	}
	h.readWaylandFD(fdWait)
	if h.lib.displayDispatchPend != nil {
		h.lib.displayDispatchPend(h.display)
	}
	if h.lib.displayFlush != nil {
		h.lib.displayFlush(h.display)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	var out []Event
	if h.closed {
		out = append(out, Event{Type: EventClose})
	}
	if h.resized {
		h.resized = false
		out = append(out, Event{
			Type: EventResize, Width: h.width, Height: h.height, Scale: h.scale,
		})
	}
	if h.redraw {
		h.redraw = false
		out = append(out, Event{Type: EventRedraw})
	}
	if len(h.queue) > 0 {
		out = append(out, h.queue...)
		h.queue = nil
	}
	return out
}

// readWaylandFD implements the non-blocking Wayland read pattern so seat
// events arrive even when the app is idle (no prior flush traffic).
// fdWait=0 is pure non-blocking; >0 waits up to that duration on the display fd.
func (h *WaylandHost) readWaylandFD(fdWait time.Duration) {
	if h.lib == nil || h.display == 0 {
		return
	}
	// Always flush outbound first so compositor can reply.
	if h.lib.displayFlush != nil {
		h.lib.displayFlush(h.display)
	}
	// Drain already-queued messages without touching the fd.
	if h.lib.displayDispatchPend != nil {
		for h.lib.displayDispatchPend(h.display) > 0 {
		}
	}
	if h.lib.displayPrepareRead == nil || h.lib.displayGetFD == nil {
		return
	}
	// prepare_read: 0 = ready to poll fd; -1 = events already pending.
	if h.lib.displayPrepareRead(h.display) != 0 {
		if h.lib.displayDispatchPend != nil {
			h.lib.displayDispatchPend(h.display)
		}
		return
	}
	fd := h.lib.displayGetFD(h.display)
	if fd < 0 {
		if h.lib.displayCancelRead != nil {
			h.lib.displayCancelRead(h.display)
		}
		return
	}
	// select(2) — portable on linux amd64 without x/sys.
	var rset syscall.FdSet
	fdSet(&rset, fd)
	var tv *syscall.Timeval
	if fdWait > 0 {
		t := syscall.NsecToTimeval(fdWait.Nanoseconds())
		tv = &t
	} else {
		t := syscall.Timeval{} // zero = non-blocking
		tv = &t
	}
	n, err := syscall.Select(fd+1, &rset, nil, nil, tv)
	if n > 0 && err == nil && h.lib.displayReadEvents != nil {
		h.lib.displayReadEvents(h.display)
	} else if h.lib.displayCancelRead != nil {
		h.lib.displayCancelRead(h.display)
	}
}

func fdSet(set *syscall.FdSet, fd int) {
	if set == nil || fd < 0 {
		return
	}
	// FdSet.Bits is []int64 / [16]int64 depending on GOARCH; index by fd/64.
	idx := fd / 64
	if idx >= len(set.Bits) {
		return
	}
	set.Bits[idx] |= 1 << (uint(fd) % 64)
}

func (h *WaylandHost) WakeUp() {
	if h == nil {
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

func (h *WaylandHost) RequestRedraw() {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.redraw = true
	h.mu.Unlock()
	h.WakeUp()
}

func (h *WaylandHost) Close() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	h.closed = true
	h.mu.Unlock()
	h.destroyNative()
	return nil
}

var (
	_ Host            = (*WaylandHost)(nil)
	_ NativeHandles   = (*WaylandHost)(nil)
	_ SurfaceProvider = (*WaylandHost)(nil)
	_ BackendProvider = (*WaylandHost)(nil)
)
