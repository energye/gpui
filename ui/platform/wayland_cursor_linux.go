//go:build linux

package platform

import (
	"fmt"
	"os"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// cursor-shape-v1 shape enum values (wayland.xml cursor_shape_v1.enum shape).
const (
	csShapeDefault     uint32 = 0
	csShapeContextMenu uint32 = 1
	csShapeCrosshair   uint32 = 3
	csShapeText        uint32 = 5
	csShapePointer     uint32 = 9
	csShapeWait        uint32 = 12
	csShapeEWResize    uint32 = 43
	csShapeNSResize    uint32 = 44
	csShapeNwseRes     uint32 = 45
	csShapeNeswRes     uint32 = 46
	csShapeMove        uint32 = 108
)

// Standard pointer cursor, GTK/sctk style:
//
//  1. Preferred: zwp_cursor_shape_v1 — the compositor renders the cursor
//     from its own theme; the client only sends one enum per enter/motion
//     (GNOME/mutter supports it since 42). No surface, no shm, no theme
//     loading, no per-image hotspot handling.
//  2. Fallback (compositor without the protocol): wl_cursor_theme from
//     libwayland-cursor with xcursor standard cursor names, attached to a
//     dedicated cursor wl_surface.
//
// The strategy resolves every CSD hit region to a shape enum; the fallback
// maps the same enum to a theme name.
type wlCursors struct {
	lib *wlLib
	win *wlWin

	mgr uintptr // zwp_cursor_shape_manager_v1 proxy
	dev uintptr // zwp_cursor_shape_device_v1 proxy (0 = unavailable)

	// Fallback state (only used when dev == 0).
	theme struct {
		lib  uintptr  // dlopen handle of libwayland-cursor.so.0
		c    uintptr  // wl_cursor_theme*
		surf uintptr  // cursor wl_surface
		once sync.Once
	}
}

// bindCursors attaches the cursor strategy to the bound wl_pointer. Returns
// a non-nil object even when neither strategy is available (setCursor then
// no-ops, leaving the compositor default visible).
func (w *wlWin) bindCursors(ptr uintptr) *wlCursors {
	if w == nil || w.lib == nil || ptr == 0 {
		return nil
	}
	c := &wlCursors{lib: w.lib, win: w}
	if w.csMgrName != 0 && ifaceCursorShapeMgr.Name != 0 {
		ver := uint32(1)
		if w.csMgrVer < ver {
			ver = w.csMgrVer
		}
		c.mgr = w.bind(w.registry, w.csMgrName, uintptr(unsafe.Pointer(&ifaceCursorShapeMgr)), ver)
	}
	if c.mgr != 0 {
		args := []wlArg{argNewID(), argO(ptr)}
		c.dev = w.lib.proxyMarshalArrayCtor(c.mgr, 1, &args[0], uintptr(unsafe.Pointer(&ifaceCursorShapeDevice)), 1)
	}
	return c
}

func (c *wlCursors) destroy() {
	if c == nil {
		return
	}
	if c.dev != 0 {
		c.lib.proxyDestroy(c.dev)
		c.dev = 0
	}
	if c.mgr != 0 {
		c.lib.proxyDestroy(c.mgr)
		c.mgr = 0
	}
	if c.theme.surf != 0 {
		c.lib.proxyDestroy(c.theme.surf)
		c.theme.surf = 0
	}
	c.theme.c = 0
}

// setCursor applies the cursor for the current hit region (called on every
// pointer enter/motion with the enter serial).
func (c *wlCursors) setCursor(serial uintptr, hit csdHit) {
	if c == nil || c.lib == nil || serial == 0 {
		return
	}
	if c.dev != 0 {
		shape := hitCursorShape(hit, c.win.activeCursor())
		csLog("set_shape dev=%d serial=%d shape=%d act=%d edge=%d", c.dev, serial, shape, hit.act, hit.edge)
		args := []wlArg{argU(uint32(serial)), argU(shape)}
		c.lib.proxyMarshalArrayFlags(c.dev, 1, uintptr(unsafe.Pointer(&ifaceCursorShapeDevice)), 1, 0, &args[0])
		c.lib.displayFlush(c.win.display)
		return
	}
	shape := hitCursorShape(hit, c.win.activeCursor())
	csLog("theme set_cursor serial=%d shape=%d act=%d edge=%d", serial, shape, hit.act, hit.edge)
	c.applyTheme(serial, shape)
}

// csLog prints cursor-path diagnostics when GPUI_WL_CURSOR_LOG=1 (default
// off; does not change behaviour).
func csLog(format string, args ...any) {
	if os.Getenv("GPUI_WL_CURSOR_LOG") == "1" {
		fmt.Fprintf(os.Stderr, "[wl-cursor] "+format+"\n", args...)
	}
}

// hitCursorShape maps a CSD hit + controller cursor to the standard shape
// enum (cursor-shape-v1 values; the fallback theme uses the same mapping
// via standard xcursor names).
func hitCursorShape(hit csdHit, cur Cursor) uint32 {
	if hit.act == csdActResize {
		switch hit.edge {
		case resizeLeft, resizeRight:
			return csShapeEWResize
		case resizeTop, resizeBottom:
			return csShapeNSResize
		case resizeTopLeft, resizeBottomRight:
			return csShapeNwseRes
		case resizeTopRight, resizeBottomLeft:
			return csShapeNeswRes
		}
	}
	switch cur {
	case CursorText:
		return csShapeText
	case CursorCrosshair:
		return csShapeCrosshair
	case CursorWait:
		return csShapeWait
	case CursorResizeH:
		return csShapeEWResize
	case CursorResizeV:
		return csShapeNSResize
	case CursorResizeNE:
		return csShapeNeswRes
	case CursorResizeNW:
		return csShapeNwseRes
	case CursorPointer:
		return csShapePointer
	default:
		return csShapeDefault
	}
}

// themeNamesFor maps a shape enum to candidate xcursor theme names, newest
// standard name first, X11 legacy names as fallback (older themes only ship
// sb_h_double_arrow etc.).
func themeNamesFor(shape uint32) []string {
	switch shape {
	case csShapeText:
		return []string{"text", "xterm"}
	case csShapeCrosshair:
		return []string{"crosshair", "cross"}
	case csShapeWait:
		return []string{"wait", "watch"}
	case csShapeEWResize:
		return []string{"ew-resize", "sb_h_double_arrow"}
	case csShapeNSResize:
		return []string{"ns-resize", "sb_v_double_arrow"}
	case csShapeNeswRes:
		return []string{"nesw-resize", "top_right_corner"}
	case csShapeNwseRes:
		return []string{"nwse-resize", "top_left_corner"}
	case csShapePointer:
		return []string{"pointer", "hand2", "hand"}
	case csShapeMove:
		return []string{"move", "fleur"}
	default:
		return []string{"left_ptr", "default", "arrow"}
	}
}

// applyTheme is the wl_cursor_theme fallback: load the theme once, fetch the
// cursor image for the shape name and set_cursor with it. Failures are
// silent — the compositor keeps its default cursor.
func (c *wlCursors) applyTheme(serial uintptr, shape uint32) {
	if c == nil || c.win == nil || c.win.lib == nil {
		csLog("applyTheme skipped: nil state")
		return
	}
	c.theme.once.Do(func() {
		l := loadCursorLibT()
		if l == nil {
			csLog("libwayland-cursor load FAILED — cursor stays compositor default")
			return
		}
		if c.theme.surf == 0 {
			c.theme.surf = c.win.ctor(c.win.comp, wlCompositorCreateSurface, c.win.lib.ifaceSurface, 4)
			if c.theme.surf == 0 {
				csLog("cursor surface create FAILED")
				return
			}
		}
		scale := 1.0
		if c.win.hostRef != nil {
			scale = c.win.hostRef.ScaleFactor()
		}
		if scale < 1 {
			scale = 1
		}
		var name *byte
		shm := c.win.cursorShm()
		c.theme.c = l.themeLoad(name, int(24*scale), shm)
		csLog("theme load size=%d shm=%d theme=%d", int(24*scale), shm, c.theme.c)
	})
	if c.theme.c == 0 || c.theme.surf == 0 {
		return
	}
	l := loadCursorLibT()
	var cur uintptr
	var used string
	for _, name := range themeNamesFor(shape) {
		nb := append([]byte(name), 0)
		cur = l.themeGetCur(c.theme.c, &nb[0])
		csLog("get_cursor theme=%d name=%q cur=%d", c.theme.c, name, cur)
		if cur != 0 {
			used = name
			break
		}
	}
	if cur == 0 {
		csLog("no cursor image found for shape %d — compositor default stays", shape)
		return
	}
	_ = used
	// struct wl_cursor { unsigned image_count; wl_cursor_image **images;
	// char *name; } (amd64: images at offset 8). Use the first image.
	imgArr := *(*uintptr)(unsafe.Pointer(cur + 8))
	if imgArr == 0 {
		csLog("cursor images array is NULL")
		return
	}
	imgPtr := *(*uintptr)(unsafe.Pointer(imgArr))
	if imgPtr == 0 {
		csLog("cursor images[0] is NULL")
		return
	}
	img := (*wlCursorImageFull)(unsafe.Pointer(imgPtr))
	// Prefer the library's own accessor (layout-proof); fall back to the
	// struct read only when the symbol is unavailable.
	buf := uintptr(0)
	if l.getBuffer != nil {
		buf = l.getBuffer(imgPtr)
	} else if img.Buffer != 0 {
		buf = img.Buffer
	}
	csLog("image=%d w=%d h=%d hot=(%d,%d) buffer=%d", imgPtr, img.Width, img.Height, img.HotX, img.HotY, buf)
	if buf == 0 {
		csLog("cursor image buffer is NULL")
		return
	}
	args := []wlArg{argO(buf), argU(0), argU(0)}
	c.win.lib.proxyMarshalArrayFlags(c.theme.surf, wlSurfaceAttach, 0, 0, 0, &args[0])
	c.win.lib.proxyMarshalArrayFlags(c.theme.surf, wlSurfaceCommit, 0, 0, 0, nil)
	c.win.lib.displayFlush(c.win.display)
	set := []wlArg{argU(uint32(serial)), argO(c.theme.surf), argU(img.HotX), argU(img.HotY)}
	c.win.lib.proxyMarshalArrayFlags(c.win.ptr.ptr, wlPtrSetCursor, 0, 0, 0, &set[0])
	c.win.lib.displayFlush(c.win.display)
}

// wl_cursor_theme (libwayland-cursor.so.0) loading, cached process-wide.
var (
	cursorLibOnceT sync.Once
	cursorLibTval  *wlCursorLibT
)

type wlCursorLibT struct {
	themeLoad   func(name *byte, size int, shm uintptr) uintptr
	themeGetCur func(theme uintptr, name *byte) uintptr
	getBuffer   func(img uintptr) uintptr
}

func loadCursorLibT() *wlCursorLibT {
	cursorLibOnceT.Do(func() {
		lib, err := purego.Dlopen("libwayland-cursor.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			lib, err = purego.Dlopen("libwayland-cursor.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		}
		if err != nil {
			return
		}
		l := &wlCursorLibT{}
		purego.RegisterLibFunc(&l.themeLoad, lib, "wl_cursor_theme_load")
		purego.RegisterLibFunc(&l.themeGetCur, lib, "wl_cursor_theme_get_cursor")
		// Optional accessor (libwayland-cursor >= 1.0); the struct read
		// below is the fallback.
		if p, err := purego.Dlsym(lib, "wl_cursor_image_get_buffer"); err == nil && p != 0 {
			purego.RegisterLibFunc(&l.getBuffer, lib, "wl_cursor_image_get_buffer")
		}
		if l.themeLoad == nil || l.themeGetCur == nil {
			return
		}
		cursorLibTval = l
	})
	return cursorLibTval
}

// wl_cursor_image layout (amd64): width,height,hotspot_x,hotspot_y,delay
// (uint32 each) then wl_buffer* (uintptr, 8-aligned). Local mirror: the
// shared wlCursorImageC in wayland_csd_linux.go omits the trailing buffer
// pointer this fallback needs (theme_get_cursor → buffer handoff).
type wlCursorImageFull struct {
	Width  uint32
	Height uint32
	HotX   uint32
	HotY   uint32
	Delay  uint32
	_      uint32 // pad
	Buffer uintptr
}