//go:build linux

package platform

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// waylandBackend implements Backend for Wayland (wl_display* + wl_surface*).
// Ported from the verified exhost Wayland host, restructured into Create
// (window lifecycle) / wlHost (event pump) / capability probing.
type waylandBackend struct{}

func init() { Register(PlatformWayland, &waylandBackend{}) }

func (b *waylandBackend) Kind() PlatformKind { return PlatformWayland }

// Create opens a Wayland window (xdg_toplevel, optional SSD decorations).
func (b *waylandBackend) Create(opts Options) (*Window, error) {
	return waylandCreate(opts.Width, opts.Height, opts.Title)
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

	ifaceXdgWmBase   wlInterfaceC
	ifaceXdgSurface  wlInterfaceC
	ifaceXdgToplevel wlInterfaceC
	ifaceDecoMgr     wlInterfaceC
	ifaceDecoTop     wlInterfaceC

	msgWmBase    [4]wlMessageC
	msgWmBaseEv  [1]wlMessageC
	msgXdgSurf   [5]wlMessageC
	msgXdgSurfEv [1]wlMessageC
	msgTop       [4]wlMessageC
	msgTopEv     [2]wlMessageC
	msgDecoMgr   [2]wlMessageC
	msgDecoTop   [3]wlMessageC
	msgDecoTopEv [1]wlMessageC

	typesXdgSurf [2]uintptr
	typesTop     [1]uintptr
	typesDeco    [2]uintptr
	typesEmpty   [1]uintptr
)

func cstr(b []byte) uintptr { return uintptr(unsafe.Pointer(&b[0])) }

func initXDGInterfaces(ifaceSurface uintptr) {
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

	msgTop[0] = wlMessageC{Name: cstr(xdgNames.mDestroy), Signature: cstr(xdgNames.sEmpty), Types: 0}
	msgTop[1] = wlMessageC{Name: cstr(xdgNames.mDestroy), Signature: cstr(xdgNames.sEmpty), Types: 0}
	msgTop[2] = wlMessageC{Name: cstr(xdgNames.mSetTitle), Signature: cstr(xdgNames.sS), Types: 0}
	msgTop[3] = wlMessageC{Name: cstr(xdgNames.mSetApp), Signature: cstr(xdgNames.sS), Types: 0}
	msgTopEv[0] = wlMessageC{Name: cstr(xdgNames.eCfg), Signature: cstr(xdgNames.eCfgIia), Types: 0}
	msgTopEv[1] = wlMessageC{Name: cstr(xdgNames.eClose), Signature: cstr(xdgNames.sEmpty), Types: 0}
	ifaceXdgToplevel = wlInterfaceC{
		Name: cstr(xdgNames.toplevel), Version: 2,
		MethodCount: 4, Methods: uintptr(unsafe.Pointer(&msgTop[0])),
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
	tiMgrName         uint32 // zwp_text_input_manager_v3 global name (0 = absent)
	seatName          uint32 // wl_seat global name (0 = absent)
	seat              uintptr // bound wl_seat proxy
	seatListener      [2]uintptr // capabilities(0) name(1)
	seatCaps          uint32 // last seat capabilities bitmask (0/1/2 = ptr/kbd/touch)

	width, height int
	configured    bool
	closed        bool
	resized       bool

	// text-input (IME) optional capability.
	ti *wlTIState
	// keyboard (wl_keyboard + xkb) optional capability.
	kbd *wlKeyboardState
	// pointer (wl_pointer) standard mouse input.
	ptr *wlPointerState
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

func waylandCreate(w, h int, title string) (*Window, error) {
	if !HasWaylandDisplay() {
		return nil, fmt.Errorf("wayland: WAYLAND_DISPLAY not set")
	}
	lib, err := loadWayland()
	if err != nil {
		return nil, err
	}
	initXDGInterfaces(lib.ifaceSurface)

	win := &wlWin{lib: lib, width: w, height: h}
	if win.width < 1 {
		win.width = 640
	}
	if win.height < 1 {
		win.height = 480
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

	// Standard Wayland input: bind wl_seat unconditionally (keyboard + pointer
	// are the base input channel; IME rides on keyboard focus).
	if win.seatName != 0 {
		win.seat = win.bind(win.registry, win.seatName, lib.ifaceSeat, 1)
		// Seat listener (2 events: capabilities, name) — required so the seat
		// proxy can dispatch; without it events accumulate and capabilities
		// (keyboard/pointer/touch availability) are never observed.
		win.seatListener[0] = purego.NewCallback(wlSeatCapabilitiesCB)
		win.seatListener[1] = purego.NewCallback(wlSeatNameCB)
		if lib.proxyAddListener(win.seat, uintptr(unsafe.Pointer(&win.seatListener[0])), win.selfPtr) != 0 {
			lib.proxyDestroy(win.seat)
			win.seat = 0
		}
	}
	// Keyboard (wl_keyboard + xkb) — standard, drives plain text + IME focus.
	if win.seat != 0 && lib.ifaceKeyboard != 0 {
		win.kbd = win.bindKeyboard()
	}
	// Pointer (wl_pointer) — standard mouse events.
	if win.seat != 0 && lib.ifacePointer != 0 {
		win.ptr = win.bindPointer()
	}
	// Optional IME capability: zwp_text_input_v3 when advertised.
	if win.seat != 0 && win.tiMgrName != 0 {
		initTIInterfaces(lib.ifaceSurface, lib.ifaceSeat)
		win.ti = win.bindTextInput()
	}

	host := &wlHost{win: win}
	return newWindow(host, PlatformWayland, imeFor(host), nil, host.destroy), nil
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
	if w.seat != 0 {
		lib.proxyDestroy(w.seat)
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
	}
	_ = registry
}

func wlRegistryGlobalRemove(data, registry, name uintptr) {}

// wlSeatCapabilitiesCB handles seat.capabilities(caps): bit 0 = pointer,
// bit 1 = keyboard, bit 2 = touch. Recorded for diagnostics; the keyboard/
// pointer proxies are bound unconditionally when the interfaces exist.
func wlSeatCapabilitiesCB(data, seat, caps uintptr) {
	w := winFrom(data)
	if w == nil {
		return
	}
	w.seatCaps = uint32(caps)
}

// wlSeatNameCB handles seat.name(name) (v2+); informational.
func wlSeatNameCB(data, seat, name uintptr) {}

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

func wlTopConfigure(data, toplevel, width, height, states uintptr) {
	w := winFrom(data)
	if w == nil {
		return
	}
	wi, hi := int32(width), int32(height)
	if wi > 0 && hi > 0 {
		if w.width != int(wi) || w.height != int(hi) {
			w.width, w.height = int(wi), int(hi)
			w.resized = true
		}
	}
	_ = toplevel
	_ = states
}

func wlTopClose(data, toplevel uintptr) {
	w := winFrom(data)
	if w != nil {
		w.closed = true
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
	wake  chan struct{}
	mu    sync.Mutex
	scale float64
}

func (h *wlHost) destroy() {
	if h == nil || h.win == nil {
		return
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
	if h.wake == nil {
		h.wake = make(chan struct{}, 1)
	}
	if evs := h.poll(); len(evs) > 0 {
		return evs
	}
	if timeout < 0 {
		timeout = 16 * time.Millisecond
	}
	if timeout == 0 {
		return h.poll()
	}
	select {
	case <-h.wake:
		if evs := h.poll(); len(evs) > 0 {
			return evs
		}
		return []Event{{Type: EventWake}}
	case <-time.After(timeout):
		return h.poll()
	}
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
	if w.resized {
		w.resized = false
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
	return out
}

func (h *wlHost) WakeUp() {
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
