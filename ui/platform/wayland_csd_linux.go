//go:build linux

package platform

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

// Client-side decorations (CSD) for GNOME Wayland — a complete standard
// window chrome drawn by the client (GNOME 42.9 has NO server-side
// decorations; zxdg_decoration_manager_v1 server-side is a KDE feature).
//
// Layout (parent = content surface, positions in content-local px):
//
//	               top (title bar) subsurface
//	┌──────────────────────────────────────┐
//	│  left │   content surface    │ right │   ← border subsurfaces
//	└──────────────────────────────────────┘
//	               bottom subsurface
//
// Four wl_subsurfaces (top/left/right/bottom) render the chrome; the content
// surface is owned by the wgpu/render layer. All decoration surfaces use
// set_desync so their commits apply immediately (independent of the parent's
// commit cadence, fixing stale decoration after resize).
//
// Interaction (GTK CSD parity):
//   - title-bar caption drag       → xdg_toplevel.move(seat, serial)
//   - double-click caption         → toggle maximize
//   - minimize / maximize/restore  / close buttons (hover + press states)
//   - 8-direction edge/corner      → xdg_toplevel.resize(seat, serial, edge),
//     hot zone on borders AND on the content edges (invisible 8px)
//   - system cursor theme          → resize cursors via wl_cursor_theme
//
// See docs/ENGINE_WAYLAND_WINDOW_STANDARD.md (design source of truth).

// wl_shm / wl_shm_pool / wl_buffer / wl_subcompositor / wl_subsurface
// request opcodes (wayland.xml authoritative).
const (
	wlShmCreatePool        = 0 // create_pool(id: new_id, fd, size)
	wlShmPoolCreateBuffer  = 0 // create_buffer(id: new_id, offset, w, h, stride, format)
	wlSubcompGetSubsurface = 1 // get_subsurface(id: new_id, surface, parent)
	wlSubsurfaceDestroy    = 0
	wlSubsurfaceSetPos     = 1 // set_position(x, y)
	wlSubsurfaceSetSync    = 4 // set_sync()
	wlSubsurfaceSetDesync  = 5 // set_desync()

	// xdg_toplevel requests (xdg-shell.xml authoritative order):
	//   destroy(0) set_parent(1) set_title(2) set_app_id(3)
	//   show_window_menu(4) move(5) resize(6) set_max_size(7)
	//   set_min_size(8) set_maximized(9) unset_maximized(10)
	//   set_fullscreen(11) unset_fullscreen(12) set_minimized(13)
	xdgToplevelShowMenu    = 4
	xdgToplevelMove        = 5
	xdgToplevelResize      = 6
	xdgToplevelSetMaximized = 9
	xdgToplevelUnsetMaxim  = 10
	xdgToplevelSetMinimized = 13
)

// xdg_surface requests (xdg-shell.xml): destroy(0) get_toplevel(1)
// get_popup(2) set_window_geometry(3) ack_configure(4).
const xdgSurfaceSetWindowGeometry = 3

// xdg_toplevel resize edges (xdg-shell.xml resize_edge enum).
const (
	resizeNone        = 0
	resizeTop         = 1
	resizeBottom      = 2
	resizeLeft        = 4
	resizeTopLeft     = 5
	resizeBottomLeft  = 6
	resizeRight       = 8
	resizeTopRight    = 9
	resizeBottomRight = 10
)

// wl_shm_format ARGB8888 = 0.
const wlShmFormatARGB8888 = 0

// wlCSD manages the decoration subsurfaces for one window.
type wlCSD struct {
	win *wlWin

	shm     uintptr // wl_shm proxy
	subcomp uintptr // wl_subcompositor proxy

	top, left, right, bottom *csdSurface

	state csdState // title/focus/maximize + button hover/press

	// topSurface lets pointer events know the pointer is over the title bar.
	topSurface uintptr
	// closeRequested is drained in poll() → EventClose.
	closeRequested bool

	// lastCaptionClick tracks double-click-on-caption (maximize toggle).
	lastCaptionClick time.Time

	// armedButton is the title-bar control button pressed but not yet
	// released (GTK4 release-inside semantics: the action fires only when
	// the pointer is still over the same button on release; dragging off
	// cancels). csdActNone = no armed button.
	armedButton int

	// dumpW/dumpH track the last dumped top-surface size (GPUI_WL_DUMP_CSD
	// appearance verification dumps only when the decoration changes size).
	dumpW, dumpH int

	// Cursor state (system cursor theme via libwayland-cursor).
	cursorSurf  uintptr // cursor wl_surface
	cursorTheme uintptr // wl_cursor_theme*
	curName     string  // last set cursor name (dedupe set_cursor)
}

// csdSurface is one decoration subsurface + its shm buffer.
type csdSurface struct {
	surf   uintptr // wl_surface
	sub    uintptr // wl_subsurface
	pool   uintptr // wl_shm_pool
	buffer uintptr // wl_buffer
	data   []byte  // mmap'd pixels (ARGB8888, stride = w*4)
	fd     int
	w, h   int
}

// initCSD binds wl_shm + wl_subcompositor and creates the four decoration
// subsurfaces. Called from waylandCreate when decorated is true. Returns nil
// (silent degrade) when the compositor lacks wl_shm/wl_subcompositor.
func (w *wlWin) initCSD(title string) *wlCSD {
	if w == nil || w.lib == nil || w.surface == 0 {
		return nil
	}
	lib := w.lib
	if w.shmName == 0 || w.subcompName == 0 {
		return nil
	}
	csd := &wlCSD{win: w}
	csd.state.Title = title
	csd.state.Focused = true
	csd.state.TitleAlignment = 0 // center
	csd.shm = w.bind(w.registry, w.shmName, lib.ifaceShm, 1)
	csd.subcomp = w.bind(w.registry, w.subcompName, lib.ifaceSubcompositor, 1)
	if csd.shm == 0 || csd.subcomp == 0 {
		csd.destroy()
		return nil
	}

	cw, ch := w.width, w.height
	if cw < 1 {
		cw = 640
	}
	if ch < 1 {
		ch = 480
	}

	var err error
	// Top (title bar): spans content width + both borders, above the content.
	if csd.top, err = csd.newSurface(cw+2*csdBorderThick, csdTitleBarHeight, -csdBorderThick, -csdTitleBarHeight); err != nil {
		csd.destroy()
		return nil
	}
	csd.topSurface = csd.top.surf
	// Left / right / bottom borders.
	if csd.left, err = csd.newSurface(csdBorderThick, ch, -csdBorderThick, 0); err != nil {
		csd.destroy()
		return nil
	}
	if csd.right, err = csd.newSurface(csdBorderThick, ch, cw, 0); err != nil {
		csd.destroy()
		return nil
	}
	if csd.bottom, err = csd.newSurface(cw+2*csdBorderThick, csdBorderThick, -csdBorderThick, ch); err != nil {
		csd.destroy()
		return nil
	}

	csd.paint()
	csd.setGeometry(cw, ch)
	return csd
}

// newSurface creates a subsurface with a shm buffer at (x,y) relative to the
// parent (content) surface.
func (c *wlCSD) newSurface(w, h, x, y int) (*csdSurface, error) {
	lib := c.win.lib
	s := &csdSurface{w: w, h: h}

	s.surf = c.win.ctor(c.win.comp, wlCompositorCreateSurface, lib.ifaceSurface, 4)
	if s.surf == 0 {
		return nil, fmt.Errorf("csd: create surface failed")
	}
	// subcompositor.get_subsurface(new_id, surface, parent)
	args := []wlArg{argNewID(), argO(s.surf), argO(c.win.surface)}
	s.sub = lib.proxyMarshalArrayCtor(c.subcomp, wlSubcompGetSubsurface, &args[0], lib.ifaceSubsurface, 1)
	if s.sub == 0 {
		s.destroy()
		return nil, fmt.Errorf("csd: get_subsurface failed")
	}
	// set_position(x, y)
	pos := []wlArg{argU(uint32(int32(x))), argU(uint32(int32(y)))}
	lib.proxyMarshalArrayFlags(s.sub, wlSubsurfaceSetPos, 0, 0, 0, &pos[0])
	// set_desync — decoration commits apply immediately, not on parent commit.
	lib.proxyMarshalArrayFlags(s.sub, wlSubsurfaceSetDesync, 0, 0, 0, nil)

	if err := c.createBufferWith(s, w, h); err != nil {
		s.destroy()
		return nil, err
	}
	return s, nil
}

// createBufferWith allocates a fresh memfd/shm-pool/buffer for w×h and maps
// it (needs the wl_shm proxy from the owning wlCSD).
func (c *wlCSD) createBufferWith(s *csdSurface, w, h int) error {
	lib := c.win.lib
	s.w, s.h = w, h
	size := w * h * 4
	fd, err := memfdCreate("gpui-csd")
	if err != nil {
		return err
	}
	s.fd = fd
	if err := syscall.Ftruncate(fd, int64(size)); err != nil {
		s.destroy()
		return err
	}
	buf, err := syscall.Mmap(fd, 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		s.destroy()
		return err
	}
	s.data = buf

	pargs := []wlArg{argNewID(), argO(uintptr(fd)), argU(uint32(size))}
	s.pool = lib.proxyMarshalArrayCtor(c.shm, wlShmCreatePool, &pargs[0], lib.ifaceShmPool, 1)
	if s.pool == 0 {
		return fmt.Errorf("csd: create_pool failed")
	}
	bargs := []wlArg{argNewID(), argU(0), argU(uint32(w)), argU(uint32(h)), argU(uint32(w * 4)), argU(wlShmFormatARGB8888)}
	s.buffer = lib.proxyMarshalArrayCtor(s.pool, wlShmPoolCreateBuffer, &bargs[0], lib.ifaceBuffer, 1)
	if s.buffer == 0 {
		return fmt.Errorf("csd: create_buffer failed")
	}
	return nil
}

// commit attaches the buffer, damages the full area and commits the
// subsurface (desync → applies immediately).
func (s *csdSurface) commit(lib *wlLib) {
	if s == nil || s.surf == 0 || s.buffer == 0 {
		return
	}
	args := []wlArg{argO(s.buffer), argU(0), argU(0)}
	lib.proxyMarshalArrayFlags(s.surf, wlSurfaceAttach, 0, 0, 0, &args[0])
	dam := []wlArg{argU(0), argU(0), argU(uint32(s.w)), argU(uint32(s.h))}
	lib.proxyMarshalArrayFlags(s.surf, wlSurfaceDamage, 0, 0, 0, &dam[0])
	lib.proxyMarshalArrayFlags(s.surf, wlSurfaceCommit, 0, 0, 0, nil)
}

func (s *csdSurface) destroy() {
	if s == nil {
		return
	}
	if s.data != nil {
		syscall.Munmap(s.data)
		s.data = nil
	}
	if s.fd > 0 {
		syscall.Close(s.fd)
		s.fd = 0
	}
	s.buffer, s.pool, s.sub, s.surf = 0, 0, 0, 0
}

// paint draws the title bar + borders and commits them.
func (c *wlCSD) paint() {
	if c == nil || c.win == nil || c.win.lib == nil {
		return
	}
	lib := c.win.lib
	outline := !c.state.Maximized && !c.state.Fullscreen
	if c.top != nil {
		paintTitleBar(c.top.data, c.top.w, c.top.h, c.state)
		c.top.commit(lib)
	}
	if c.left != nil {
		paintBorder(c.left.data, c.left.w, c.left.h, csdEdgeLeft, outline)
		c.left.commit(lib)
	}
	if c.right != nil {
		paintBorder(c.right.data, c.right.w, c.right.h, csdEdgeRight, outline)
		c.right.commit(lib)
	}
	if c.bottom != nil {
		paintBorder(c.bottom.data, c.bottom.w, c.bottom.h, csdEdgeBottom, outline)
		c.bottom.commit(lib)
	}
	lib.displayFlush(c.win.display)
	c.dumpPNGs()
}

// repaintTitle repaints only the title bar (button hover/press / focus /
// maximize changes) without touching content or borders.
func (c *wlCSD) repaintTitle() {
	if c == nil || c.win == nil || c.win.lib == nil || c.top == nil {
		return
	}
	paintTitleBar(c.top.data, c.top.w, c.top.h, c.state)
	c.top.commit(c.win.lib)
	c.win.lib.displayFlush(c.win.display)
	c.dumpPNGs()
}

// dumpPNGs writes the decoration surfaces to PNG files for appearance
// verification when GPUI_WL_DUMP_CSD=<dir> is set. Only the title bar is
// dumped (it carries all the visible chrome); the borders are transparent
// strips. Dumps once per size change, not per repaint.
func (c *wlCSD) dumpPNGs() {
	if c == nil || c.top == nil || c.top.data == nil {
		return
	}
	dir := os.Getenv("GPUI_WL_DUMP_CSD")
	if dir == "" {
		return
	}
	if c.top.w == c.dumpW && c.top.h == c.dumpH {
		return
	}
	c.dumpW, c.dumpH = c.top.w, c.top.h
	_ = os.MkdirAll(dir, 0o755)
	img := image.NewRGBA(image.Rect(0, 0, c.top.w, c.top.h))
	for y := 0; y < c.top.h; y++ {
		for x := 0; x < c.top.w; x++ {
			o := (y*c.top.w + x) * 4
			img.Pix[(y*c.top.w+x)*4+0] = c.top.data[o+2] // R
			img.Pix[(y*c.top.w+x)*4+1] = c.top.data[o+1] // G
			img.Pix[(y*c.top.w+x)*4+2] = c.top.data[o+0] // B
			img.Pix[(y*c.top.w+x)*4+3] = c.top.data[o+3] // A
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("csd_title_%dx%d.png", c.top.w, c.top.h)), buf.Bytes(), 0o644)
}

// resize resizes the four decoration surfaces to a new content size (called
// from poll when wlTopConfigure delivered a new toplevel size) and updates
// the content dims used for edge-zone hit testing. Only the shm buffers are
// recreated — the wl_surface / wl_subsurface objects stay alive so an active
// interactive resize (pointer grab) is not disrupted.
func (c *wlCSD) resize(cw, ch int) {
	if c == nil || c.win == nil || c.win.lib == nil {
		return
	}
	if cw < 1 {
		cw = 640
	}
	if ch < 1 {
		ch = 480
	}
	if c.top != nil {
		c.resizeSurface(c.top, cw+2*csdBorderThick, csdTitleBarHeight, -csdBorderThick, -csdTitleBarHeight)
		c.topSurface = c.top.surf
	}
	if c.left != nil {
		c.resizeSurface(c.left, csdBorderThick, ch, -csdBorderThick, 0)
	}
	if c.right != nil {
		c.resizeSurface(c.right, csdBorderThick, ch, cw, 0)
	}
	if c.bottom != nil {
		c.resizeSurface(c.bottom, cw+2*csdBorderThick, csdBorderThick, -csdBorderThick, ch)
	}
	// The xdg window geometry is NOT declared here (from the configure
	// event): the compositor applies a declared geometry at the next
	// commit, and the renderer's buffer has not caught up to the new size
	// yet (the swapchain reconfigures asynchronously) — declaring it now
	// would make mutter cache negative frame extents (geometry > surface),
	// which corrupt the maximize/unmaximize restore size. The geometry is
	// declared by the renderer's swapchain-resize hook (OnSurfaceResized →
	// setGeometryNoFlush) in the same wire batch as the new-size buffer.
	c.paint()
}

// setVisible shows (true) or hides (false) the four decoration subsurfaces.
// Hiding detaches each buffer (attach NULL + commit, desync → applies
// immediately): the window then has no chrome — used by server-side
// decoration negotiation (deco mode = server_side) and by window Hide().
// Showing re-attaches the still-live shm buffers. Idempotent.
func (c *wlCSD) setVisible(v bool) {
	if c == nil || c.win == nil || c.win.lib == nil {
		return
	}
	lib := c.win.lib
	surfs := []*csdSurface{c.top, c.left, c.right, c.bottom}
	for _, s := range surfs {
		if s == nil {
			continue
		}
		if v {
			s.commit(lib)
		} else {
			detach := []wlArg{argO(0), argU(0), argU(0)}
			lib.proxyMarshalArrayFlags(s.surf, wlSurfaceAttach, 0, 0, 0, &detach[0])
			lib.proxyMarshalArrayFlags(s.surf, wlSurfaceCommit, 0, 0, 0, nil)
		}
	}
	lib.displayFlush(c.win.display)
}

// setGeometry declares the xdg window geometry = the content area
// (xdg_surface.set_window_geometry, GTK4 parity). With the chrome drawn in
// subsurfaces OUTSIDE the content surface, the geometry is the content rect
// itself: the compositor then treats the window (placement, maximize restore
// saved_rect, configure sizes) as the content area only, and the configure
// the client receives is exactly the content size — GTK reports the same
// content-size semantics (a GTK 400x400 window has geometry 400x437
// including its title bar; the app still sees 400x400). Without this call
// the compositor defaults the geometry to the bounding box of surface +
// subsurfaces (content + chrome), which inflates every size the compositor
// tracks and breaks maximize/unmaximize restore (the window restores to a
// garbage size and snaps back).
func (c *wlCSD) setGeometry(cw, ch int) {
	c.marshalGeometry(cw, ch, true)
}

// setGeometryNoFlush declares the geometry without flushing the display:
// called from the renderer's swapchain-resize hook (OnSurfaceResized) on
// the raster thread inside the present critical section, the request rides
// the present's own wire batch (geometry → attach → commit) so the
// compositor applies it together with a buffer of the same size. Flushing
// here would race the UI thread's wl_display_flush in poll().
func (c *wlCSD) setGeometryNoFlush(cw, ch int) {
	c.marshalGeometry(cw, ch, false)
}

// marshalGeometry queues xdg_surface.set_window_geometry(0,0,cw,ch) and
// optionally flushes. See wlCSD.setGeometry for the contract.
func (c *wlCSD) marshalGeometry(cw, ch int, flush bool) {
	if c == nil || c.win == nil || c.win.lib == nil || c.win.xdgSurf == 0 {
		return
	}
	// mutter rejects a zero-size geometry (width/height == 0 → warning).
	if cw < 1 {
		cw = 1
	}
	if ch < 1 {
		ch = 1
	}
	args := []wlArg{argU(0), argU(0), argU(uint32(cw)), argU(uint32(ch))}
	c.win.lib.proxyMarshalArrayFlags(c.win.xdgSurf, xdgSurfaceSetWindowGeometry, 0, 0, 0, &args[0])
	if flush {
		c.win.lib.displayFlush(c.win.display)
	}
}

// resizeSurface frees the old shm buffer/pool and creates a new buffer at the
// given size/position, keeping the wl_surface and wl_subsurface proxies.
func (c *wlCSD) resizeSurface(s *csdSurface, w, h, x, y int) {
	if s == nil || c == nil || c.win == nil || c.win.lib == nil {
		return
	}
	lib := c.win.lib
	// Detach old buffer (attach NULL + commit) before destroying it.
	// wl_surface.attach signature is "oii" (buffer, x, y) — MUST pass the
	// three args; passing nil makes libwayland read out of bounds → SIGSEGV.
	detach := []wlArg{argO(0), argU(0), argU(0)}
	lib.proxyMarshalArrayFlags(s.surf, wlSurfaceAttach, 0, 0, 0, &detach[0])
	lib.proxyMarshalArrayFlags(s.surf, wlSurfaceCommit, 0, 0, 0, nil)
	if s.buffer != 0 {
		lib.proxyDestroy(s.buffer)
		s.buffer = 0
	}
	if s.pool != 0 {
		lib.proxyDestroy(s.pool)
		s.pool = 0
	}
	if s.data != nil {
		syscall.Munmap(s.data)
		s.data = nil
	}
	if s.fd > 0 {
		syscall.Close(s.fd)
		s.fd = 0
	}

	// Update position on the (still-alive) subsurface.
	pos := []wlArg{argU(uint32(int32(x))), argU(uint32(int32(y)))}
	lib.proxyMarshalArrayFlags(s.sub, wlSubsurfaceSetPos, 0, 0, 0, &pos[0])
	_ = c.createBufferWith(s, w, h)
}

// destroySurfaces frees the four decoration surfaces but keeps wl_shm and
// wl_subcompositor alive (used by destroy).
func (c *wlCSD) destroySurfaces() {
	if c == nil {
		return
	}
	lib := c.win.lib
	for _, s := range []*csdSurface{c.top, c.left, c.right, c.bottom} {
		if s == nil {
			continue
		}
		if lib != nil {
			if s.buffer != 0 {
				lib.proxyDestroy(s.buffer)
				s.buffer = 0
			}
			if s.pool != 0 {
				lib.proxyDestroy(s.pool)
				s.pool = 0
			}
			if s.sub != 0 {
				lib.proxyDestroy(s.sub)
				s.sub = 0
			}
			if s.surf != 0 {
				lib.proxyDestroy(s.surf)
				s.surf = 0
			}
		}
		s.destroy()
	}
	c.top, c.left, c.right, c.bottom = nil, nil, nil, nil
	c.topSurface = 0
}

// destroy tears down all decoration subsurfaces, cursor and frees buffers.
func (c *wlCSD) destroy() {
	if c == nil {
		return
	}
	c.destroySurfaces()
	lib := c.win.lib
	if lib != nil {
		if c.cursorSurf != 0 {
			lib.proxyDestroy(c.cursorSurf)
			c.cursorSurf = 0
		}
		if c.subcomp != 0 {
			lib.proxyDestroy(c.subcomp)
			c.subcomp = 0
		}
		if c.shm != 0 {
			lib.proxyDestroy(c.shm)
			c.shm = 0
		}
	}
	c.cursorTheme = 0
}

// --- window state (from xdg_toplevel.configure states) ---

// xdg states (xdg-shell.xml xdg_toplevel_state enum): 1=maximized,
// 2=fullscreen, 3=resizing, 4=activated, 5..8=tiled, 9=suspended.

// setActivated updates focused state (title bar color).
func (c *wlCSD) setActivated(v bool) {
	if c == nil || c.state.Focused == v {
		return
	}
	c.state.Focused = v
	c.repaintTitle()
}

// setMaximized updates maximize flag + icon.
func (c *wlCSD) setMaximized(v bool) {
	if c == nil || c.state.Maximized == v {
		return
	}
	c.state.Maximized = v
	c.paint()
}

// setFullscreen updates fullscreen flag (no visible borders when fullscreen).
func (c *wlCSD) setFullscreen(v bool) {
	if c == nil || c.state.Fullscreen == v {
		return
	}
	c.state.Fullscreen = v
	c.paint()
}

// setLocked enables/disables resize grips + cursors (SetSize min==max clamp /
// SetResizable(false)); the buttons and caption keep working.
func (c *wlCSD) setLocked(v bool) {
	if c == nil || c.state.Locked == v {
		return
	}
	c.state.Locked = v
	// No repaint needed (grips are invisible); cursor updates on next motion.
}

// setTiled records the xdg tiled state (informational for now).
func (c *wlCSD) setTiled(v bool) {
	if c == nil || c.state.Tiled == v {
		return
	}
	c.state.Tiled = v
}

// --- pointer interaction (called from wl_pointer callbacks) ---

// csdHit describes what a press at (x,y) on the given decoration surface does.
type csdHit struct {
	act  int // csdAct*
	edge int // resize edge (csdActResize)
}

// csd actions.
const (
	csdActNone = iota
	csdActMove
	csdActClose
	csdActMinimize
	csdActMaximize
	csdActResize
)

// hitTest maps a pointer press on a decoration surface to an action.
// surface is the wl_surface the compositor reported (title-bar subsurface vs
// content surface); (x,y) is surface-local logical px.
//
// Priority: buttons (right of title bar) > resize grips > caption drag.
// Resize grips are disabled when the window is maximized/fullscreen (nothing
// to resize) or size-locked (SetSize min==max / SetResizable(false)) —
// GTK4 parity: a fixed-size window shows no resize affordances.
func (c *wlCSD) hitTest(surface uintptr, x, y float64) csdHit {
	if c == nil {
		return csdHit{}
	}
	noResize := c.state.Maximized || c.state.Fullscreen || c.state.Locked
	switch surface {
	case c.topSurface:
		wTop := c.top.w
		// Buttons win over resize (GTK pattern).
		closeX := float64(wTop - csdButtonW)
		maxX := closeX - csdButtonW
		minX := maxX - csdButtonW
		switch {
		case x >= closeX:
			return csdHit{act: csdActClose}
		case x >= maxX:
			return csdHit{act: csdActMaximize}
		case x >= minX:
			return csdHit{act: csdActMinimize}
		}
		// Resize zone: top edge + the left/right flanks (the title bar spans
		// the full window width, so its edge strips are the left/right resize
		// grips — winit/sctk CSD layout). Disabled when maximized/fullscreen/
		// size-locked.
		if !noResize {
			switch {
			case y < csdCornerGrip && x < csdCornerGrip:
				return csdHit{act: csdActResize, edge: resizeTopLeft}
			case y < csdCornerGrip && x >= float64(wTop-csdCornerGrip):
				return csdHit{act: csdActResize, edge: resizeTopRight}
			case x <= float64(csdBorderThick):
				return csdHit{act: csdActResize, edge: resizeLeft}
			case x >= float64(wTop-csdBorderThick):
				return csdHit{act: csdActResize, edge: resizeRight}
			case y < csdCornerGrip:
				return csdHit{act: csdActResize, edge: resizeTop}
			}
		}
		return csdHit{act: csdActMove}
	case c.left.surf:
		if noResize {
			return csdHit{}
		}
		switch {
		case y < csdCornerGrip:
			return csdHit{act: csdActResize, edge: resizeTopLeft}
		case y >= float64(c.left.h-csdCornerGrip):
			return csdHit{act: csdActResize, edge: resizeBottomLeft}
		default:
			return csdHit{act: csdActResize, edge: resizeLeft}
		}
	case c.right.surf:
		if noResize {
			return csdHit{}
		}
		switch {
		case y < csdCornerGrip:
			return csdHit{act: csdActResize, edge: resizeTopRight}
		case y >= float64(c.right.h-csdCornerGrip):
			return csdHit{act: csdActResize, edge: resizeBottomRight}
		default:
			return csdHit{act: csdActResize, edge: resizeRight}
		}
	case c.bottom.surf:
		if noResize {
			return csdHit{}
		}
		switch {
		case x < csdCornerGrip:
			return csdHit{act: csdActResize, edge: resizeBottomLeft}
		case x >= float64(c.bottom.w-csdCornerGrip):
			return csdHit{act: csdActResize, edge: resizeBottomRight}
		default:
			return csdHit{act: csdActResize, edge: resizeBottom}
		}
	case c.win.surface:
		// Content never participates in window resize when a CSD title bar
		// exists (user requirement: 有标题栏时内容区不参与窗口调整; GTK4 CSD
		// resize handles live on the chrome, not the content).
		return csdHit{}
	}
	return csdHit{}
}

// stateW returns current content w/h (for hot-zone tests).
func (c *wlCSD) stateW() (int, int) {
	if c == nil || c.win == nil {
		return 640, 480
	}
	cw, ch := c.win.width, c.win.height
	if cw < 1 {
		cw = 640
	}
	if ch < 1 {
		ch = 480
	}
	return cw, ch
}

// appCoords translates decoration-surface-local pointer coords into the
// content (toplevel) surface coordinate space — GTK4 parity: the app sees
// one continuous coordinate space over the whole window, with negative y
// over the title bar (the chrome surfaces sit outside the content).
func (c *wlCSD) appCoords(surface uintptr, x, y float64) (float64, float64) {
	if c == nil || surface == 0 {
		return x, y
	}
	switch surface {
	case c.topSurface:
		return x - csdBorderThick, y - csdTitleBarHeight
	case c.left.surf:
		return x - csdBorderThick, y
	case c.right.surf:
		cw, _ := c.stateW()
		return x + float64(cw), y
	case c.bottom.surf:
		_, ch := c.stateW()
		return x - csdBorderThick, y + float64(ch)
	}
	return x, y
}

// buttonHitFor maps a title-bar hit to button state indices (for hover press).
func (c *wlCSD) buttonHitFor(hit csdHit) *csdButtonState {
	switch hit.act {
	case csdActClose:
		return &c.state.Close
	case csdActMaximize:
		return &c.state.Maximize
	case csdActMinimize:
		return &c.state.Minimize
	}
	return nil
}

// onHover updates button hover states + cursor when the pointer moves.
// Called from wlPtrMotionCB; returns the resulting cursor name ("" = default).
func (c *wlCSD) onHover(surface uintptr, x, y float64) csdHit {
	if c == nil {
		return csdHit{}
	}
	hit := c.hitTest(surface, x, y)
	// Clear all hover, then set for the current hit.
	c.state.Close.Hovered = false
	c.state.Maximize.Hovered = false
	c.state.Minimize.Hovered = false
	if surface == c.topSurface {
		if b := c.buttonHitFor(hit); b != nil {
			b.Hovered = true
		}
	}
	c.repaintTitle()
	return hit
}

// onButtonPress handles a left-button press on the chrome. Returns whether
// the event was consumed (true = do not forward to content).
//
// GTK4 semantics: control buttons (close/minimize/maximize) arm on press
// (pressed state shown) and fire on release-inside (onButtonRelease); the
// caption drag and resize grips act on press (a drag starts immediately).
func (c *wlCSD) onButtonPress(seat, serial uintptr, hit csdHit) bool {
	if c == nil {
		return false
	}
	// Content presses (act none) are NOT chrome: forward to the content layer.
	if hit.act == csdActNone {
		return false
	}
	if b := c.buttonHitFor(hit); b != nil {
		b.Pressed = true
		c.armedButton = hit.act
		c.repaintTitle()
	}
	switch hit.act {
	case csdActClose, csdActMinimize, csdActMaximize:
		// Armed; fires on release-inside.
	case csdActMove:
		// Double-click on caption → toggle maximize (GTK behavior).
		now := time.Now()
		if !c.lastCaptionClick.IsZero() && now.Sub(c.lastCaptionClick) < 400*time.Millisecond {
			c.lastCaptionClick = time.Time{}
			c.toggleMaximize()
		} else {
			c.lastCaptionClick = now
			c.requestMove(seat, serial)
		}
	case csdActResize:
		c.requestResize(seat, serial, hit.edge)
	}
	return true
}

// onButtonRelease fires an armed control-button action when the pointer is
// still over the same button at release (GTK4 release-inside; dragging off
// cancels). surface/pos are the pointer's current surface + position.
func (c *wlCSD) onButtonRelease(surface uintptr, x, y float64) {
	if c == nil || c.armedButton == csdActNone {
		return
	}
	act := c.armedButton
	c.armedButton = csdActNone
	if b := c.buttonHitFor(csdHit{act: act}); b != nil {
		b.Pressed = false
	}
	c.repaintTitle()
	// Fire only when the release is still inside the same button.
	hit := c.hitTest(surface, x, y)
	if hit.act != act {
		return
	}
	switch act {
	case csdActClose:
		c.closeRequested = true
	case csdActMinimize:
		c.requestMinimize()
	case csdActMaximize:
		c.toggleMaximize()
	}
}

// onRightPress handles a right-button press on the chrome: the caption (and
// only the caption, not the buttons) opens the window menu via
// xdg_toplevel.show_window_menu — the standard Wayland window menu (GTK3/
// winit parity; the compositor renders restore/move/resize/minimize/maximize/
// close). Any chrome right-press is consumed (the bar owns its presses);
// the menu only fires on the caption.
func (c *wlCSD) onRightPress(seat, serial uintptr, surface uintptr, x, y float64) bool {
	if c == nil || surface != c.topSurface {
		return false
	}
	hit := c.hitTest(surface, x, y)
	if hit.act == csdActMove {
		c.showWindowMenu(seat, serial, x, y)
	}
	return true
}

// showWindowMenu asks the compositor to show the window menu at (x,y) in
// surface-local coordinates of the toplevel (content) surface — the CSD
// surfaces sit outside the content, so the coordinates are translated.
func (c *wlCSD) showWindowMenu(seat, serial uintptr, x, y float64) {
	if c == nil || c.win == nil || c.win.lib == nil || c.win.toplevel == 0 || seat == 0 || serial == 0 {
		return
	}
	// top surface is offset (-border, -titleBarHeight) from the content.
	cx := int32(x) + csdBorderThick
	cy := int32(y) + csdTitleBarHeight
	args := []wlArg{argO(seat), argU(uint32(serial)), argU(uint32(cx)), argU(uint32(cy))}
	c.win.lib.proxyMarshalArrayFlags(c.win.toplevel, xdgToplevelShowMenu, 0, 0, 0, &args[0])
	c.win.lib.displayFlush(c.win.display)
}

// requestMove starts an interactive xdg_toplevel.move(seat, serial).
func (c *wlCSD) requestMove(seat, serial uintptr) {
	if c == nil || c.win == nil || c.win.lib == nil || c.win.toplevel == 0 || seat == 0 {
		return
	}
	args := []wlArg{argO(seat), argU(uint32(serial))}
	c.win.lib.proxyMarshalArrayFlags(c.win.toplevel, xdgToplevelMove, 0, 0, 0, &args[0])
	c.win.lib.displayFlush(c.win.display)
}

// requestResize starts an interactive xdg_toplevel.resize(seat, serial, edge).
func (c *wlCSD) requestResize(seat, serial uintptr, edge int) {
	if c == nil || c.win == nil || c.win.lib == nil || c.win.toplevel == 0 || seat == 0 || edge == resizeNone {
		return
	}
	args := []wlArg{argO(seat), argU(uint32(serial)), argU(uint32(edge))}
	c.win.lib.proxyMarshalArrayFlags(c.win.toplevel, xdgToplevelResize, 0, 0, 0, &args[0])
	c.win.lib.displayFlush(c.win.display)
}

// requestMinimize calls xdg_toplevel.set_minimized.
func (c *wlCSD) requestMinimize() {
	if c == nil || c.win == nil || c.win.lib == nil || c.win.toplevel == 0 {
		return
	}
	c.win.lib.proxyMarshalArrayFlags(c.win.toplevel, xdgToplevelSetMinimized, 0, 0, 0, nil)
	c.win.lib.displayFlush(c.win.display)
}

// toggleMaximize calls xdg_toplevel.set/unset_maximized based on current state.
func (c *wlCSD) toggleMaximize() {
	if c == nil || c.win == nil || c.win.lib == nil || c.win.toplevel == 0 {
		return
	}
	op := uint32(xdgToplevelSetMaximized)
	if c.state.Maximized {
		op = xdgToplevelUnsetMaxim
	}
	c.win.lib.proxyMarshalArrayFlags(c.win.toplevel, op, 0, 0, 0, nil)
	c.win.lib.displayFlush(c.win.display)
}

// --- resize cursor (system cursor theme via wl_pointer.set_cursor) ---

// cursorNameForEdge returns the X cursor name for a resize edge.
func cursorNameForEdge(edge int) string {
	switch edge {
	case resizeTop, resizeBottom:
		return "sb_v_double_arrow"
	case resizeLeft, resizeRight:
		return "sb_h_double_arrow"
	case resizeTopLeft, resizeBottomRight:
		return "top_left_corner"
	case resizeTopRight, resizeBottomLeft:
		return "top_right_corner"
	}
	return ""
}

// setCursor updates the pointer cursor for the current hit region.
// wl_pointer.set_cursor (opcode 1, signature "ouii": serial, surface,
// hotspot_x, hotspot_y). Called on pointer enter/motion with enter serial.
//
// The compositor default cursor is left untouched until we have a real
// replacement (resize edge or controller cursor): sending set_cursor with an
// empty surface (or a failed image apply) makes the pointer invisible.
func (c *wlCSD) setCursor(serial uintptr, hit csdHit) {
	if c == nil || c.win == nil || c.win.lib == nil || c.win.ptr == nil {
		return
	}
	name := ""
	if hit.act == csdActResize {
		name = cursorNameForEdge(hit.edge)
	}
	if name == "" && c.win != nil {
		// No resize-edge cursor: apply the controller-set active cursor
		// (SetCursor / Options.Cursor), "" = default.
		name = cursorThemeName(c.win.activeCursor())
	}
	if name == "" {
		// Back to the compositor default: only when a custom cursor is
		// currently showing; set_cursor(NULL) restores the default theme.
		// Never set one otherwise (the default cursor is already active).
		if c.curName != "" {
			c.curName = ""
			args := []wlArg{argU(uint32(serial)), argO(0), argU(0), argU(0)}
			c.win.lib.proxyMarshalArrayFlags(c.win.ptr.ptr, wlPtrSetCursor, 0, 0, 0, &args[0])
			c.win.lib.displayFlush(c.win.display)
		}
		return
	}
	if name == c.curName {
		return
	}
	// Attach the image FIRST; only on success hand the surface to the
	// compositor (an empty surface would hide the pointer).
	if !c.applyCursorImage(name) {
		return
	}
	c.curName = name
	args := []wlArg{argU(uint32(serial)), argO(c.cursorSurf), argU(0), argU(0)}
	c.win.lib.proxyMarshalArrayFlags(c.win.ptr.ptr, wlPtrSetCursor, 0, 0, 0, &args[0])
	c.win.lib.displayFlush(c.win.display)
}

// ensureCursorSurface loads the cursor theme once and returns the cursor
// surface (created lazily). Returns 0 until BOTH the surface and the theme
// are ready — a cursor surface without an attached image would make the
// pointer invisible (empty surface), so setCursor must never be given one.
func (c *wlCSD) ensureCursorSurface() uintptr {
	if c == nil || c.win == nil || c.win.lib == nil || c.shm == 0 {
		return 0
	}
	if c.cursorSurf != 0 && c.cursorTheme != 0 {
		return c.cursorSurf
	}
	lib := c.win.lib
	if c.cursorSurf == 0 {
		c.cursorSurf = c.win.ctor(c.win.comp, wlCompositorCreateSurface, lib.ifaceSurface, 4)
		if c.cursorSurf == 0 {
			return 0
		}
	}
	if c.cursorTheme == 0 {
		l := loadCursorLib()
		if l == nil {
			return 0
		}
		var name *byte
		c.cursorTheme = l.themeLoad(name, 24, c.shm)
		if c.cursorTheme == 0 {
			return 0
		}
	}
	return c.cursorSurf
}

// applyCursorImage loads the cursor image for name into cursorSurf and
// returns whether an image was actually attached. False when the theme,
// cursor name or image is unavailable — the caller must NOT set_cursor with
// an empty surface (the pointer would disappear).
func (c *wlCSD) applyCursorImage(name string) bool {
	if c == nil || c.win == nil || c.win.lib == nil || c.cursorSurf == 0 || c.cursorTheme == 0 {
		return false
	}
	l := loadCursorLib()
	if l == nil {
		return false
	}
	nb := append([]byte(name), 0)
	cur := l.themeGetCur(c.cursorTheme, &nb[0])
	if cur == 0 {
		return false
	}
	// struct wl_cursor { unsigned image_count; wl_cursor_image **images;
	// char *name; } — images is an ARRAY of pointers (wayland-cursor.h),
	// so dereference twice: images@8 → images[0] → wl_cursor_image.
	imgArr := *(*uintptr)(unsafe.Pointer(cur + 8))
	if imgArr == 0 {
		return false
	}
	imgPtr := *(*uintptr)(unsafe.Pointer(imgArr))
	if imgPtr == 0 {
		return false
	}
	img := (*wlCursorImageC)(unsafe.Pointer(imgPtr))
	if img.Buffer == 0 {
		return false
	}
	args := []wlArg{argO(img.Buffer), argU(0), argU(0)}
	c.win.lib.proxyMarshalArrayFlags(c.cursorSurf, wlSurfaceAttach, 0, 0, 0, &args[0])
	c.win.lib.proxyMarshalArrayFlags(c.cursorSurf, wlSurfaceCommit, 0, 0, 0, nil)
	c.win.lib.displayFlush(c.win.display)
	return true
}

// wl_cursor_theme (libwayland-cursor.so.0) — system cursor theme.
var (
	cursorLibOnce sync.Once
	cursorLib     *wlCursorLib
)

type wlCursorLib struct {
	themeLoad   func(name *byte, size int, shm uintptr) uintptr
	themeGetCur func(theme uintptr, name *byte) uintptr
}

func loadCursorLib() *wlCursorLib {
	cursorLibOnce.Do(func() {
		lib, err := purego.Dlopen("libwayland-cursor.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			lib, err = purego.Dlopen("libwayland-cursor.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		}
		if err != nil {
			return
		}
		l := &wlCursorLib{}
		purego.RegisterLibFunc(&l.themeLoad, lib, "wl_cursor_theme_load")
		purego.RegisterLibFunc(&l.themeGetCur, lib, "wl_cursor_theme_get_cursor")
		if l.themeLoad == nil || l.themeGetCur == nil {
			return
		}
		cursorLib = l
	})
	return cursorLib
}

// wl_cursor_image layout (amd64): width,height,hotspot_x,hotspot_y,delay
// (uint32 each) then wl_buffer* (uintptr, 8-aligned).
type wlCursorImageC struct {
	Width  uint32
	Height uint32
	HotX   uint32
	HotY   uint32
	Delay  uint32
	_      uint32 // pad
	Buffer uintptr
}

// --- shared low-level pixel helpers ---

// putPx writes one pixel (BGRA in memory for ARGB8888 little-endian), opaque.
func putPx(buf []byte, stride, x, y int, r, g, b byte) {
	putPxA(buf, stride, x, y, r, g, b, 0xFF)
}

// putPxA writes one pixel with explicit alpha (0x00 = fully transparent).
func putPxA(buf []byte, stride, x, y int, r, g, b, a byte) {
	off := (y*stride + x) * 4
	if off+3 >= len(buf) {
		return
	}
	buf[off] = b
	buf[off+1] = g
	buf[off+2] = r
	buf[off+3] = a
}

func fillRect(buf []byte, stride, x0, y0, w, h int, c [4]byte) {
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			putPxA(buf, stride, x, y, c[2], c[1], c[0], c[3])
		}
	}
}

func clearRect(buf []byte, stride, x0, y0, w, h int) {
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			putPxA(buf, stride, x, y, 0, 0, 0, 0x00)
		}
	}
}

// memfdCreate is a small wrapper (avoids importing unix just for this).
func memfdCreate(name string) (int, error) {
	fd, err := unix.MemfdCreate(name, unix.MFD_CLOEXEC)
	if err != nil {
		return -1, err
	}
	return fd, nil
}
