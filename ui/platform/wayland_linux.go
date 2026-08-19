//go:build linux

package platform

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
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
	wlSurfaceFrame            = 3
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
	ifaceCallback   uintptr // wl_callback (wl_surface.frame return; frame-presented notice)
	// CSD (client-side decorations) interfaces.
	ifaceShm           uintptr
	ifaceShmPool       uintptr
	ifaceBuffer        uintptr
	ifaceSubcompositor uintptr
	ifaceSubsurface    uintptr
	ifaceOutput        uintptr // wl_output (xdg_toplevel.set_fullscreen arg)
	ifaceRegion        uintptr // wl_region (wl_surface.set_input_region)
	// Clipboard / DnD (wl_data_device_manager core protocol) interfaces.
	ifaceDataDevMgr uintptr // wl_data_device_manager
	ifaceDataDev    uintptr // wl_data_device
	ifaceDataSource uintptr // wl_data_source
	ifaceDataOffer  uintptr // wl_data_offer
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
		{"wl_callback_interface", &l.ifaceCallback},
		{"wl_shm_interface", &l.ifaceShm},
		{"wl_shm_pool_interface", &l.ifaceShmPool},
		{"wl_buffer_interface", &l.ifaceBuffer},
		{"wl_subcompositor_interface", &l.ifaceSubcompositor},
		{"wl_subsurface_interface", &l.ifaceSubsurface},
		{"wl_output_interface", &l.ifaceOutput},
		{"wl_region_interface", &l.ifaceRegion},
		{"wl_data_device_manager_interface", &l.ifaceDataDevMgr},
		{"wl_data_device_interface", &l.ifaceDataDev},
		{"wl_data_source_interface", &l.ifaceDataSource},
		{"wl_data_offer_interface", &l.ifaceDataOffer},
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
		mGetPopup, mSetGeom            []byte
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
		sNoo, sIiii, sNoTop            []byte
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
		mGetPopup:     append([]byte("get_popup"), 0),
		mSetGeom:      append([]byte("set_window_geometry"), 0),
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
		sNoo:          append([]byte("noo"), 0),
		sIiii:         append([]byte("iiii"), 0),
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
	// get_popup(2) and set_window_geometry(3) signatures MUST be exact:
	// libwayland computes the wire message size from the signature, so a
	// wrong/empty one desyncs the whole connection (the compositor then
	// misparses every later request, including the renderer's wgpu traffic).
	msgXdgSurf[2] = wlMessageC{Name: cstr(xdgNames.mGetPopup), Signature: cstr(xdgNames.sNoo), Types: 0}
	msgXdgSurf[3] = wlMessageC{Name: cstr(xdgNames.mSetGeom), Signature: cstr(xdgNames.sIiii), Types: 0}
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
	ddMgrName         uint32  // wl_data_device_manager global name (0 = absent)
	seat              uintptr // bound wl_seat proxy (via seatState)
	seatState         *wlSeatState
	// hostRef is set by waylandCreate so seat callbacks can wake the loop.
	hostRef *wlHost

	width, height int
	configured    bool
	closed        bool
	closeReq      bool // xdg close / CSD ✕ request (→ EventCloseRequested)
	resized       bool
	hidden        bool // Hide()/Options.Visible=false: surface stack detached (window unmapped)
	decorated     bool // Options.Decorations at create (re-applied on show re-create)
	recreated     bool // showNative rebuilt the stack; poll emits EventHidden{false} after the re-map configure
	// resizing mirrors the xdg_toplevel "resizing" configure state (3): the
	// compositor sets it while an interactive move/resize drag is in flight
	// and clears it when the drag ends. Read-only diagnostic for apps/tests
	// (event thread); the resize events themselves drive the relayout.
	resizing bool

	// Frame-presented notice (ENGINE_FRAME_PRESENT_STANDARD.md 块2): while
	// frameCB != 0 a wl_surface.frame request is in flight; the compositor
	// answers with wlFrameDone once the committed frame was shown. frameDone
	// is read by poll to emit EventFramePresented. RequestFrameNotify runs on
	// the raster thread, wlFrameDone on the event thread → atomics.
	frameCB   atomic.Uintptr // in-flight wl_callback proxy (0 = none)
	frameDone atomic.Bool    // a frame-presented notice arrived since last poll

	// Async window state (configure-driven).
	activated bool // EventFocus dedup
	suspended bool // EventOccluded dedup
	tiled     bool // xdg tiled state (5..8); informational (CSD)

	// ctlMu guards the WindowController-tracked state below. Controller
	// methods may be called from any goroutine (§2.5.5 thread contract);
	// wlTopConfigure reconciles the configure-driven fields on the event
	// thread.
	ctlMu      sync.Mutex
	title      string // tracked at Create/SetTitle
	minW, minH int    // user min constraint (0 = unconstrained)
	maxW, maxH int    // user max constraint (0 = unconstrained)
	resizable  bool   // SetResizable flag (false = min==max locked)
	locked     bool   // SetSize min==max clamp / SetResizable(false): size fixed
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
	// dds owns clipboard + external DnD (wl_data_device), nil when the
	// compositor lacks wl_data_device_manager (Clipboard() returns nil).
	dds *wlDataDeviceState
	// imeMu guards the pending IME event queue drained by poll.
	imeMu     sync.Mutex
	imeEvents []Event
	// keyMu guards the pending key event queue drained by poll.
	keyMu     sync.Mutex
	keyEvents []Event
	// ptrMu guards the pending pointer event queue drained by poll.
	ptrMu     sync.Mutex
	ptrEvents []Event
	// dndMu guards the pending drop event queue (wayland_clipboard_linux.go).
	dndMu     sync.Mutex
	dndEvents []Event

	regListener [2]uintptr
	wmListener  [1]uintptr
	xdgListener [1]uintptr
	topListener [2]uintptr
	decoListener [1]uintptr
	// frameListener is the wl_callback_listener (single done entry); created
	// once and re-attached per in-flight frame request (survives the
	// callback proxy itself).
	frameListener [1]uintptr
	selfPtr       uintptr

	titlePin []byte
	appPin   []byte

	// decoMode mirrors the zxdg_toplevel_decoration configure(mode) result:
	// 0=unknown/no global, 1=client_side, 2=server_side. Written on the
	// event thread; read before/at CSD creation (same dispatch goroutine).
	decoMode int

	// lastSerial is the most recent input-event serial (pointer button /
	// enter / keyboard key), stored atomically: the clipboard writer
	// (wl_data_device.set_selection) needs a fresh serial from any goroutine.
	lastSerial atomic.Uint32
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

	// Tracked window policy for the surface stack (min/max/resizable/
	// fullscreen/decorations). Create-time constraints are applied via
	// applySurfaceConfig after the first configure; the same tracked state
	// is re-asserted verbatim when Show re-creates the stack (§6.2).
	win.ctlMu.Lock()
	win.minW, win.minH = opts.MinWidth, opts.MinHeight
	win.maxW, win.maxH = opts.MaxWidth, opts.MaxHeight
	win.resizable = opts.Resizable
	win.fullscreen = opts.Fullscreen
	win.decorated = decorated
	win.ctlMu.Unlock()

	win.titlePin = append([]byte(title), 0)
	appID := opts.IconName
	if appID == "" {
		appID = "gpui.l1"
	}
	win.appPin = append([]byte(appID), 0)

	if err := win.createSurfaceStack(false); err != nil {
		win.destroyNative()
		return nil, err
	}
	runtime.KeepAlive(win)
	win.applySurfaceConfig()

	// Standard Wayland input bootstrap: bind wl_seat and WAIT for the
	// capabilities event before requesting keyboard/pointer. ALL seat-derived
	// objects (keyboard, pointer, text-input, data-device) are created only
	// after the seat callback fires (wlSeatFlushPending) — normal client
	// order.
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
			// The data device (clipboard + DnD) needs the bound seat but no
			// capability bit; it is created together with the other devices
			// once capabilities arrive.
			win.seatState.pendingDD = true
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

	// Options.Visible=false: open the window hidden (§9 二.2.1 隐藏窗口).
	if opts.Visible != nil && !*opts.Visible {
		win.hideNative()
	}

	host := &wlHost{win: win}
	win.hostRef = host
	ctl := &waylandController{h: host}
	return newWindow(host, PlatformWayland, imeFor(host), clipFor(host), ctl, host.destroy), nil
}

// createSurfaceStack builds the xdg surface stack (wl_surface → xdg_surface
// → xdg_toplevel) on the already-bound display/registry/compositor/wm_base,
// asserts the tracked title/app_id/decorations/maximized state and issues the
// first commit (xdg-shell only sends the first configure after a commit).
//
// async=false blocks until the first configure is acked+committed (Open path
// — single-threaded, before the event pump starts). async=true returns right
// after the commit (Show-after-hide path): the pump thread is the only
// dispatcher, so the non-pump goroutine never dispatches; wlXdgConfigure
// acks the first new configure and maps the window, and poll emits
// EventHidden{Hidden:false} once it observes the re-map (§6.2), which is when
// the embedder recreates the GPU present target on the NEW wl_surface.
func (w *wlWin) createSurfaceStack(async bool) error {
	if w == nil || w.lib == nil {
		return nil
	}
	lib := w.lib
	if w.comp == 0 || w.wmBase == 0 || w.registry == 0 {
		return fmt.Errorf("wayland: createSurfaceStack: missing compositor/wm_base")
	}
	w.configured = false
	w.closeReq = false

	w.surface = w.ctor(w.comp, wlCompositorCreateSurface, lib.ifaceSurface, 4)
	if w.surface == 0 {
		return fmt.Errorf("wayland: create wl_surface failed")
	}
	{
		args := []wlArg{argNewID(), argO(w.surface)}
		w.xdgSurf = lib.proxyMarshalArrayCtor(w.wmBase, xdgWmBaseGetXdgSurface, &args[0],
			uintptr(unsafe.Pointer(&ifaceXdgSurface)), 2)
	}
	if w.xdgSurf == 0 {
		return fmt.Errorf("wayland: get_xdg_surface failed")
	}
	if w.xdgListener[0] == 0 {
		w.xdgListener[0] = purego.NewCallback(wlXdgConfigure)
	}
	lib.proxyAddListener(w.xdgSurf, uintptr(unsafe.Pointer(&w.xdgListener[0])), w.selfPtr)

	{
		args := []wlArg{argNewID()}
		w.toplevel = lib.proxyMarshalArrayCtor(w.xdgSurf, xdgSurfaceGetToplevel, &args[0],
			uintptr(unsafe.Pointer(&ifaceXdgToplevel)), 2)
	}
	if w.toplevel == 0 {
		return fmt.Errorf("wayland: get_toplevel failed")
	}
	// The listener trampolines are created once (libwayland aborts when an
	// opcode's listener is NULL) and survive hide/show re-creates.
	if w.topListener[0] == 0 {
		w.topListener[0] = purego.NewCallback(wlTopConfigure)
	}
	if w.topListener[1] == 0 {
		w.topListener[1] = purego.NewCallback(wlTopClose)
	}
	lib.proxyAddListener(w.toplevel, uintptr(unsafe.Pointer(&w.topListener[0])), w.selfPtr)

	{
		args := []wlArg{argS(cstr(w.titlePin))}
		lib.proxyMarshalArrayFlags(w.toplevel, xdgToplevelSetTitle, 0, 0, 0, &args[0])
	}
	{
		args := []wlArg{argS(cstr(w.appPin))}
		lib.proxyMarshalArrayFlags(w.toplevel, xdgToplevelSetAppID, 0, 0, 0, &args[0])
	}

	// Decoration negotiation (zxdg_decoration_manager_v1): the client asks
	// for a preferred mode and the compositor answers with a configure(mode)
	// event telling which side actually draws the chrome. GNOME 42 has no
	// such global (decoName==0 → we always draw CSD). When it exists:
	//   - decorated: request server_side (compositor chrome preferred, GTK
	//     default); configure(server_side) hides our CSD, configure
	//     (client_side) keeps it (engine docs §9 一.3 装饰协商).
	//   - frameless: request none (no chrome at all).
	if w.decoName != 0 {
		w.decoMgr = w.bind(w.registry, w.decoName, uintptr(unsafe.Pointer(&ifaceDecoMgr)), 1)
		if w.decoMgr != 0 {
			args := []wlArg{argNewID(), argO(w.toplevel)}
			w.decoTop = lib.proxyMarshalArrayCtor(w.decoMgr, xdgDecoMgrGetDecoration, &args[0],
				uintptr(unsafe.Pointer(&ifaceDecoTop)), 1)
			if w.decoTop != 0 {
				// configure(mode) event: 0=none 1=client_side 2=server_side.
				if w.decoListener[0] == 0 {
					w.decoListener[0] = purego.NewCallback(wlDecoConfigure)
				}
				lib.proxyAddListener(w.decoTop, uintptr(unsafe.Pointer(&w.decoListener[0])), w.selfPtr)
				var mode uint32
				if w.decorated {
					mode = xdgDecoModeServerSide
				} else {
					mode = 0 // none — frameless has no chrome from either side
				}
				args := []wlArg{argU(mode)}
				lib.proxyMarshalArrayFlags(w.decoTop, xdgDecoSetMode, 0, 0, 0, &args[0])
			}
		}
	}

	// Maximized: set_maximized BEFORE the first commit so the compositor
	// never shows a non-maximized frame (main doc §2.4: 首 commit 前
	// set_maximized; the requested size from the first configure is then the
	// maximized size, which the CSD ignores when maximized).
	w.ctlMu.Lock()
	maximized := w.maximized
	w.ctlMu.Unlock()
	if maximized {
		lib.proxyMarshalArrayFlags(w.toplevel, xdgToplevelSetMaximized, 0, 0, 0, nil)
	}

	// First commit before any configure (empty commit is enough); the first
	// acked configure maps the window (wlXdgConfigure).
	lib.proxyMarshalArrayFlags(w.surface, wlSurfaceCommit, 0, 0, 0, nil)
	if !async {
		deadline := time.Now().Add(3 * time.Second)
		for !w.configured && !w.closed && time.Now().Before(deadline) {
			if lib.displayDispatch(w.display) < 0 {
				break
			}
		}
		if !w.configured {
			return fmt.Errorf("wayland: xdg configure timeout")
		}
	}
	return nil
}

// applySurfaceConfig re-applies the tracked window policy on the current
// toplevel after its first configure: creation-time size constraints (0 =
// unconstrained), the min==max fixed-size clamp (SetSize/SetResizable),
// fullscreen, and the CSD. Uses the tracked state, so a hide/show re-create
// re-asserts the same policy verbatim (§6.2).
func (w *wlWin) applySurfaceConfig() {
	if w == nil || w.lib == nil || w.toplevel == 0 {
		return
	}
	w.ctlMu.Lock()
	minW, minH, maxW, maxH := w.minW, w.minH, w.maxW, w.maxH
	resizable, locked := w.resizable, w.locked
	fullscreen := w.fullscreen
	decorated := w.decorated
	title := w.title
	w.ctlMu.Unlock()

	w.top2i(xdgToplevelSetMinSize, minW, minH)
	w.top2i(xdgToplevelSetMaxSize, maxW, maxH)
	if !resizable || locked {
		// Fixed-size window: min==max locks at the current size (§2.5.2
		// Wayland clamp contract; unlock via SetMinSize/SetMaxSize/SetResizable).
		w.top2i(xdgToplevelSetMinSize, w.width, w.height)
		w.top2i(xdgToplevelSetMaxSize, w.width, w.height)
	}
	if fullscreen {
		// set_fullscreen(NULL) → the compositor's current output (§2.4).
		w.top1o(xdgToplevelSetFullscreen, 0)
	}
	// Client-side decorations (CSD): GNOME provides no server-side chrome, so
	// draw a title bar + borders via wl_subsurface when requested (default
	// true). Silent degrade if the compositor lacks wl_shm/wl_subcompositor.
	if decorated {
		w.csd = w.initCSD(title)
	}
	// The deco configure(mode=server_side) may already have arrived during
	// the initial dispatch (before initCSD ran): the compositor draws the
	// chrome, so keep the CSD subsurfaces detached (hidden).
	if w.csd != nil && w.decoMode == xdgDecoModeServerSide {
		w.csd.setVisible(false)
	}
}

// destroySurfaceStack tears down the xdg surface stack (CSD subsurfaces +
// deco + toplevel + xdg_surface + wl_surface), leaving the display/registry/
// compositor/wm_base and seat-derived devices (keyboard/pointer/IME/
// data-device) alive. Destroying the wl_surface unmaps the window — xdg has
// no unmap request, so destroy + recreate on Show is the standard approach
// (GTK4 gdk_wayland_window_hide parity). Must run only after the GPU present
// target backing this wl_surface has been closed (embedder order, §6.2).
func (w *wlWin) destroySurfaceStack() {
	if w == nil || w.lib == nil {
		return
	}
	lib := w.lib
	if w.csd != nil {
		w.csd.destroy()
		w.csd = nil
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
	w.configured = false
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
	if w.dds != nil {
		w.dds.destroy()
		w.dds = nil
	}
	if w.kbd != nil {
		w.kbd.destroy()
		w.kbd = nil
	}
	if w.ptr != nil {
		w.ptr.destroy()
		w.ptr = nil
	}
	if w.seatState != nil {
		w.seatState.destroy()
		w.seatState = nil
		w.seat = 0
	}
	// Surface stack (CSD + deco + toplevel + xdg_surface + wl_surface).
	w.destroySurfaceStack()
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
	case "wl_data_device_manager":
		w.ddMgrName = n
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
	// Commit only on the FIRST configure (maps the window per xdg-shell:
	// the surface is mapped once a commit follows the first acked
	// configure). Later configures are acked WITHOUT committing — an empty
	// commit there would re-submit the stale pre-resize buffer, which
	// confuses the compositor's size tracking during state changes
	// (maximize/unmaximize restore gets aborted: it sees the window still
	// at the old size and snaps it back). The render layer commits at the
	// new size on its next present instead.
	if !w.configured {
		w.configured = true
		if w.surface != 0 {
			w.lib.proxyMarshalArrayFlags(w.surface, wlSurfaceCommit, 0, 0, 0, nil)
		}
	}
}

// wlFrameDone is the wl_callback done handler: the compositor has shown a
// committed content frame (ENGINE_FRAME_PRESENT_STANDARD.md 块2). The
// callback proxy is single-shot and dead after done; destroy it, clear the
// in-flight slot and surface the notice as EventFramePresented on the next
// poll. Event-thread (runs inside displayDispatchPend).
func wlFrameDone(data, cb, cbData uintptr) {
	w := winFrom(data)
	if w == nil || w.lib == nil {
		return
	}
	if w.frameCB.Load() == cb {
		w.frameCB.Store(0)
	}
	// wl_callback_destroy is an inline wrapper over wl_proxy_destroy in this
	// libwayland build (protocol request helpers are not exported).
	w.lib.proxyDestroy(cb)
	w.frameDone.Store(true)
	if h := w.hostRef; h != nil {
		h.WakeUp() // unblock WaitEvents so the notice is delivered promptly
	}
}

// RequestFrameNotify implements platform.FrameNotifier: asks the compositor
// to notify once the next committed content frame has been shown
// (wl_surface.frame + wl_callback listener). No-op while a request is in
// flight or before the content surface exists. Raster thread; libwayland
// marshals are internally locked (safe cross-thread).
func (h *wlHost) RequestFrameNotify() {
	if h == nil || h.win == nil || h.win.lib == nil {
		return
	}
	w := h.win
	if w.surface == 0 || w.frameCB.Load() != 0 {
		return
	}
	if w.frameListener[0] == 0 {
		w.frameListener[0] = purego.NewCallback(wlFrameDone)
	}
	args := []wlArg{argNewID()}
	cb := w.lib.proxyMarshalArrayCtor(w.surface, wlSurfaceFrame, &args[0], w.lib.ifaceCallback, 1)
	if cb == 0 {
		return
	}
	// wl_callback_add_listener is an inline wrapper over wl_proxy_add_listener
	// in this libwayland build (protocol request helpers are not exported).
	w.lib.proxyAddListener(cb, uintptr(unsafe.Pointer(&w.frameListener[0])), w.selfPtr)
	w.frameCB.Store(cb)
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
	w.tiled = states.tiled
	if states.activated {
		w.minimized = false
	}
	w.ctlMu.Unlock()
	// The resizing state flips only at drag start/end — not on every step —
	// so it is tracked outside the width/height dedup below.
	w.resizing = states.resizing
	if w.csd != nil {
		w.csd.setTiled(states.tiled)
	}
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
	if os.Getenv("GPUI_WL_TRACE_CFG") == "1" {
		fmt.Fprintf(os.Stderr, "WLDBG configure %dx%d states={max:%v full:%v resizing:%v act:%v tiled:%v susp:%v}\n",
			wi, hi, states.maximized, states.fullscreen, states.resizing, states.activated, states.tiled, states.suspended)
	}
	if wi > 0 && hi > 0 {
		// Maximized (not fullscreen): the compositor configures the FULL
		// work area, but the content must shrink by the title-bar height —
		// the bar (subsurface at (0,-32)) occupies the work area's top
		// strip, so total window (content + chrome) == work area and the
		// bar stays visible when maximized (§6). Fullscreen keeps the full
		// configure size (no chrome shown).
		if w.csd != nil && w.csd.visible && states.maximized && !states.fullscreen && hi > int32(csdTitleBarHeight) {
			hi -= int32(csdTitleBarHeight)
		}
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

// wlDecoConfigure: zxdg_toplevel_decoration_v1.configure(mode) — the
// compositor confirms which side draws the chrome (0=none, 1=client_side,
// 2=server_side). server_side → hide our CSD subsurfaces; client_side →
// show them. The mode is recorded even before CSD creation (the event can
// arrive during waylandCreate's initial dispatch, before initCSD runs).
func wlDecoConfigure(data, decoTop, mode uintptr) {
	w := winFrom(data)
	if w == nil {
		return
	}
	w.decoMode = int(mode)
	if w.csd != nil {
		w.csd.setVisible(mode != xdgDecoModeServerSide)
	}
	_ = decoTop
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
	// destroying gates WaitEvents against a concurrent Close from another
	// goroutine: Close sets it + writes the wake pipe, so a parked poll
	// wakes, sees the flag and returns instead of dispatching on a
	// destroyed wl_display (native SIGSEGV).
	destroying atomic.Int32
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
	// Gate + wake any WaitEvents parked in poll on another goroutine so it
	// returns nil instead of dispatching on the torn-down display. The wake
	// pipe stays open until after destroyNative (which disconnects the
	// display) — closing it first would turn the parked poll's wakeup into
	// POLLNVAL while the display is still referenced.
	h.destroying.Store(1)
	h.ensureWakePipe()
	if h.wakePipe[1] != 0 {
		_, _ = unix.Write(h.wakePipe[1], []byte{1})
	}
	h.win.destroyNative()
	if h.wakePipe[0] != 0 {
		_ = unix.Close(h.wakePipe[0])
		h.wakePipe[0] = 0
	}
	if h.wakePipe[1] != 0 {
		_ = unix.Close(h.wakePipe[1])
		h.wakePipe[1] = 0
	}
	h.win = nil
}

// ApplyHiddenDetach implements platform.HiddenSurface: called by the embedder
// AFTER the GPU present target (wgpu WSI surface backed by the content
// wl_surface) has been closed. Destroying the surface stack unmaps the window
// (xdg has no unmap request; GTK4 parity: hide = destroy, show = recreate).
// Seat-derived devices (keyboard/pointer/IME/data-device) survive.
func (h *wlHost) ApplyHiddenDetach() {
	if h == nil || h.win == nil {
		return
	}
	h.win.destroySurfaceStack()
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

// OnSurfaceResized implements platform.SurfacePresenter: the renderer's
// swapchain has just been reconfigured to logicalW×logicalH and the next
// commit carries a buffer of that size. Declare the xdg window geometry now
// so the compositor applies it together with the new-size buffer (a
// geometry larger than the current buffer would leave negative frame
// extents cached, corrupting maximize/unmaximize restore sizes). Called on
// the raster thread inside the present critical section — the marshal is
// queued without an explicit flush so it rides the present's own batch and
// never races the UI thread's wl_display_flush.
func (h *wlHost) OnSurfaceResized(logicalW, logicalH int) {
	if h == nil || h.win == nil {
		return
	}
	if h.win.csd != nil {
		h.win.csd.setGeometryNoFlush(logicalW, logicalH)
	}
}

// WaitVSync uses DRM vblank when available; scheduler falls back to software.
func (h *wlHost) WaitVSync() error { return WaitDRMVBlank() }

func (h *wlHost) WaitEvents(timeout time.Duration) []Event {
	if h == nil || h.win == nil || h.destroying.Load() != 0 {
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
			if h.destroying.Load() != 0 {
				return nil // Close from another goroutine — never touch the display
			}
			h.drainWake()
			w.lib.displayCancelRead(w.display)
			if evs := h.poll(); len(evs) > 0 {
				return evs
			}
			return []Event{{Type: EventWake}}
		}
		if pfds[0].Revents&unix.POLLIN != 0 {
			if h.destroying.Load() != 0 {
				return nil // Close from another goroutine — never touch the display
			}
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
			if h.destroying.Load() != 0 {
				return nil
			}
			w.lib.displayCancelRead(w.display)
			break
		}
		if h.destroying.Load() != 0 {
			return nil
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
	// Proxy destruction is deferred to the event thread (the only thread
	// that dispatches proxy events): drain the clipboard/DnD queue now.
	if w.dds != nil {
		w.dds.drainPendingDestroys()
	}
	// Window-level pointer model: an unconsumed surface leave = the pointer
	// left the window (internal content↔chrome crossings consume it via the
	// paired enter and surface as motion).
	if w.ptr != nil {
		w.ptr.resolveDeferredLeave()
	}
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
	// Compositor frame-presented notice (wl_surface.frame callback done).
	if w.frameDone.Swap(false) {
		out = append(out, Event{Type: EventFramePresented})
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
	// External file drop events queued by the wl_data_device callback.
	w.dndMu.Lock()
	if len(w.dndEvents) > 0 {
		out = append(out, w.dndEvents...)
		w.dndEvents = nil
	}
	w.dndMu.Unlock()
	// Show re-created the surface stack (async path): the re-map completes
	// when the first new configure is acked+committed here on the event
	// thread; only then does the embedder recreate the GPU present target
	// against the NEW wl_surface (§6.2).
	w.ctlMu.Lock()
	if w.recreated && w.configured {
		w.recreated = false
		w.ctlMu.Unlock()
		w.focusMu.Lock()
		w.focusEvents = append(w.focusEvents, Event{Type: EventHidden, Hidden: false})
		w.focusMu.Unlock()
	} else {
		w.ctlMu.Unlock()
	}
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
