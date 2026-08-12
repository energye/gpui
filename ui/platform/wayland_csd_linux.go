//go:build linux

package platform

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

// Client-side decorations (CSD) for GNOME Wayland: GNOME/mutter does NOT
// provide server-side decorations (zxdg_decoration_manager_v1 server-side is
// a KDE feature), so a frameless xdg_toplevel shows no title bar. GTK and
// most Wayland toolkits draw their own chrome. We do the same using four
// wl_subsurfaces (title bar + left/right/bottom borders) rendered into
// wl_shm buffers — the standard approach (same as gogpu/GTK CSD).
//
// Layout (parent = content surface):
//
//	┌──────────────────────────────────────┐  top (title bar) subsurface
//	└┬────────────────────────────────────┬┘
//	 left │         content surface        │ right  ← side borders
//	 └────┴────────────────────────────────┴────┘
//	              bottom border
//
// Pointer interaction: dragging the title bar calls xdg_toplevel.move(seat,
// serial); the close button emits a close request. All drawn with a minimal
// ARGB8888 painter (no external deps).

// CSD geometry (logical px).
const (
	csdTitleBarH = 32
	csdBorderW   = 4 // resize grip area — wide enough to click
	csdCloseW    = 44
	csdPadLeft   = 10
)

// wl_shm / wl_shm_pool / wl_buffer / wl_subcompositor / wl_subsurface
// request opcodes (wayland.xml authoritative).
const (
	wlShmCreatePool        = 0 // create_pool(id: new_id, fd: fd, size: int)
	wlShmPoolCreateBuffer  = 0 // create_buffer(id: new_id, offset, width, height, stride: int, format: uint)
	wlSubcompGetSubsurface = 1 // get_subsurface(id: new_id, surface, parent)
	wlSubsurfaceDestroy    = 0
	wlSubsurfaceSetPos     = 1 // set_position(x: int, y: int)
	wlSubsurfaceSetSync    = 4 // set_sync()
	wlSubsurfaceSetDesync  = 5 // set_desync()
	// xdg_toplevel requests (xdg-shell.xml authoritative order):
	//   destroy(0) set_parent(1) set_title(2) set_app_id(3)
	//   show_window_menu(4) move(5) resize(6) set_max_size(7)
	//   set_min_size(8) set_maximized(9) unset_maximized(10)
	//   set_fullscreen(11) unset_fullscreen(12) set_minimized(13)
	xdgToplevelMove        = 5
	xdgToplevelResize      = 6
	xdgToplevelSetMaximized = 9
	xdgToplevelUnsetMaxim   = 10
	xdgToplevelSetMinimized = 13
)

// xdg_toplevel resize edges (xdg-shell.xml resize_edge enum).
const (
	resizeNone       = 0
	resizeTop        = 1
	resizeBottom     = 2
	resizeLeft       = 4
	resizeTopLeft    = 5
	resizeBottomLeft = 6
	resizeRight      = 8
	resizeTopRight   = 9
	resizeBottomRight = 10
)

// wl_shm_format ARGB8888 = 0.
const wlShmFormatARGB8888 = 0

// wlCSD manages the client-side decoration subsurfaces for one window.
type wlCSD struct {
	win *wlWin

	shm     uintptr // wl_shm proxy
	subcomp uintptr // wl_subcompositor proxy

	top    *csdSurface
	left   *csdSurface
	right  *csdSurface
	bottom *csdSurface

	title string
	// topSurfaceID lets pointer events know the pointer is over the title bar.
	topSurface uintptr
	// closeRequested is set by a click on the close button (drained in poll).
	closeRequested bool
	// maximized tracks xdg_toplevel state so the maximize button can draw
	// either "maximize" or "restore" (updated from wlTopConfigure states).
	maximized bool
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
	csd := &wlCSD{win: w, title: title}
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
	// Top: title bar spanning content width + both borders.
	if csd.top, err = csd.newSurface(cw+2*csdBorderW, csdTitleBarH, -csdBorderW, -csdTitleBarH); err != nil {
		csd.destroy()
		return nil
	}
	csd.topSurface = csd.top.surf
	// Left / right / bottom borders.
	if csd.left, err = csd.newSurface(csdBorderW, ch, -csdBorderW, 0); err != nil {
		csd.destroy()
		return nil
	}
	if csd.right, err = csd.newSurface(csdBorderW, ch, cw, 0); err != nil {
		csd.destroy()
		return nil
	}
	if csd.bottom, err = csd.newSurface(cw+2*csdBorderW, csdBorderW, -csdBorderW, ch); err != nil {
		csd.destroy()
		return nil
	}

	csd.paint()
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
	// set_desync — decoration commits take effect immediately, independent of
	// the parent (content) surface's commit cadence (the wgpu renderer owns
	// the parent commit). Sync mode would defer decoration updates until the
	// next content frame, causing stale/offset decoration after resize.
	lib.proxyMarshalArrayFlags(s.sub, wlSubsurfaceSetDesync, 0, 0, 0, nil)

	// shm buffer: memfd → pool → buffer.
	size := w * h * 4
	fd, err := memfdCreate("gpui-csd")
	if err != nil {
		s.destroy()
		return nil, err
	}
	s.fd = fd
	if err := syscall.Ftruncate(fd, int64(size)); err != nil {
		s.destroy()
		return nil, err
	}
	buf, err := syscall.Mmap(fd, 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		s.destroy()
		return nil, err
	}
	s.data = buf

	// wl_shm.create_pool(new_id, fd, size)
	pargs := []wlArg{argNewID(), argO(uintptr(fd)), argU(uint32(size))}
	s.pool = lib.proxyMarshalArrayCtor(c.shm, wlShmCreatePool, &pargs[0], lib.ifaceShmPool, 1)
	if s.pool == 0 {
		s.destroy()
		return nil, fmt.Errorf("csd: create_pool failed")
	}
	// create_buffer(new_id, offset, width, height, stride, format)
	bargs := []wlArg{argNewID(), argU(0), argU(uint32(w)), argU(uint32(h)), argU(uint32(w * 4)), argU(wlShmFormatARGB8888)}
	s.buffer = lib.proxyMarshalArrayCtor(s.pool, wlShmPoolCreateBuffer, &bargs[0], lib.ifaceBuffer, 1)
	if s.buffer == 0 {
		s.destroy()
		return nil, fmt.Errorf("csd: create_buffer failed")
	}
	return s, nil
}

// commit attaches the buffer, damages the full area and commits the
// subsurface (sync mode → applied with the next parent commit).
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
	if c.top != nil {
		paintTitleBar(c.top.data, c.top.w, c.top.h, c.title, c.maximized)
		c.top.commit(lib)
	}
	if c.left != nil {
		paintLeftBorder(c.left.data, c.left.w, c.left.h)
		c.left.commit(lib)
	}
	if c.right != nil {
		paintRightBorder(c.right.data, c.right.w, c.right.h)
		c.right.commit(lib)
	}
	if c.bottom != nil {
		paintBottomBorder(c.bottom.data, c.bottom.w, c.bottom.h)
		c.bottom.commit(lib)
	}
	lib.displayFlush(c.win.display)
}

// resize resizes the four decoration surfaces to a new content size (called
// from poll when wlTopConfigure delivered a new toplevel size). Only the shm
// buffers are recreated — the wl_surface / wl_subsurface objects stay alive
// so an active interactive resize (pointer grab) is not disrupted.
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
	// Top: title bar spanning content width + both borders.
	if c.top != nil {
		c.resizeSurface(c.top, cw+2*csdBorderW, csdTitleBarH, -csdBorderW, -csdTitleBarH)
		c.topSurface = c.top.surf
	}
	if c.left != nil {
		c.resizeSurface(c.left, csdBorderW, ch, -csdBorderW, 0)
	}
	if c.right != nil {
		c.resizeSurface(c.right, csdBorderW, ch, cw, 0)
	}
	if c.bottom != nil {
		c.resizeSurface(c.bottom, cw+2*csdBorderW, csdBorderW, -csdBorderW, ch)
	}
	c.paint()
}

// resizeSurface frees the old shm buffer/pool and creates a new buffer at the
// given size/position, keeping the wl_surface and wl_subsurface proxies.
func (c *wlCSD) resizeSurface(s *csdSurface, w, h, x, y int) {
	if s == nil || c == nil || c.win == nil || c.win.lib == nil {
		return
	}
	lib := c.win.lib
	// Detach old buffer before destroying it.
	lib.proxyMarshalArrayFlags(s.surf, wlSurfaceAttach, 0, 0, 0, nil)
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

	s.w, s.h = w, h
	size := w * h * 4
	fd, err := memfdCreate("gpui-csd")
	if err != nil {
		return
	}
	s.fd = fd
	if err := syscall.Ftruncate(fd, int64(size)); err != nil {
		return
	}
	buf, err := syscall.Mmap(fd, 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return
	}
	s.data = buf
	pargs := []wlArg{argNewID(), argO(uintptr(fd)), argU(uint32(size))}
	s.pool = lib.proxyMarshalArrayCtor(c.shm, wlShmCreatePool, &pargs[0], lib.ifaceShmPool, 1)
	if s.pool == 0 {
		return
	}
	bargs := []wlArg{argNewID(), argU(0), argU(uint32(w)), argU(uint32(h)), argU(uint32(w * 4)), argU(wlShmFormatARGB8888)}
	s.buffer = lib.proxyMarshalArrayCtor(s.pool, wlShmPoolCreateBuffer, &bargs[0], lib.ifaceBuffer, 1)
}

// destroySurfaces frees the four decoration surfaces but keeps wl_shm and
// wl_subcompositor alive (used by destroy; resize keeps objects alive).
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

// destroy tears down all decoration subsurfaces and frees buffers.
func (c *wlCSD) destroy() {
	if c == nil {
		return
	}
	c.destroySurfaces()
	lib := c.win.lib
	if lib != nil {
		if c.subcomp != 0 {
			lib.proxyDestroy(c.subcomp)
			c.subcomp = 0
		}
		if c.shm != 0 {
			lib.proxyDestroy(c.shm)
			c.shm = 0
		}
	}
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

// csdCorner is the resize-grip corner size in px.
const csdCorner = 8

// hitTest maps a pointer press on a decoration surface to an action.
// surface is the wl_surface the compositor reported (top/left/right/bottom
// decoration subsurface), (x,y) is surface-local logical px.
func (c *wlCSD) hitTest(surface uintptr, x, y float64) csdHit {
	if c == nil {
		return csdHit{}
	}
	switch surface {
	case c.topSurface:
		w := c.top.w
		// Buttons win over the top-right corner resize (GTK pattern).
		if x >= float64(w-csdCloseW) {
			return csdHit{act: csdActClose}
		}
		if x >= float64(w-2*csdCloseW) {
			return csdHit{act: csdActMaximize}
		}
		if x >= float64(w-3*csdCloseW) {
			return csdHit{act: csdActMinimize}
		}
		// Top edge / corners (non-button area).
		if y < csdCorner {
			switch {
			case x < csdCorner:
				return csdHit{act: csdActResize, edge: resizeTopLeft}
			case x >= float64(w-csdCorner):
				return csdHit{act: csdActResize, edge: resizeTopRight}
			default:
				return csdHit{act: csdActResize, edge: resizeTop}
			}
		}
		return csdHit{act: csdActMove}
	case c.left.surf:
		switch {
		case y < csdCorner:
			return csdHit{act: csdActResize, edge: resizeTopLeft}
		case y >= float64(c.left.h-csdCorner):
			return csdHit{act: csdActResize, edge: resizeBottomLeft}
		default:
			return csdHit{act: csdActResize, edge: resizeLeft}
		}
	case c.right.surf:
		switch {
		case y < csdCorner:
			return csdHit{act: csdActResize, edge: resizeTopRight}
		case y >= float64(c.right.h-csdCorner):
			return csdHit{act: csdActResize, edge: resizeBottomRight}
		default:
			return csdHit{act: csdActResize, edge: resizeRight}
		}
	case c.bottom.surf:
		switch {
		case x < csdCorner:
			return csdHit{act: csdActResize, edge: resizeBottomLeft}
		case x >= float64(c.bottom.w-csdCorner):
			return csdHit{act: csdActResize, edge: resizeBottomRight}
		default:
			return csdHit{act: csdActResize, edge: resizeBottom}
		}
	}
	return csdHit{}
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
	if c.maximized {
		op = xdgToplevelUnsetMaxim
	}
	c.win.lib.proxyMarshalArrayFlags(c.win.toplevel, op, 0, 0, 0, nil)
	c.win.lib.displayFlush(c.win.display)
}

// setMaximized updates the maximize icon (from xdg_toplevel.configure states)
// and repaints the title bar.
func (c *wlCSD) setMaximized(v bool) {
	if c == nil || c.maximized == v {
		return
	}
	c.maximized = v
	if c.top != nil && c.win != nil && c.win.lib != nil {
		paintTitleBar(c.top.data, c.top.w, c.top.h, c.title, c.maximized)
		c.top.commit(c.win.lib)
		c.win.lib.displayFlush(c.win.display)
	}
}

// --- minimal ARGB8888 painter (no external deps) ---

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

func fillRect(buf []byte, stride, x0, y0, w, h int, r, g, b byte) {
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			putPx(buf, stride, x, y, r, g, b)
		}
	}
}

// clearRect sets a region fully transparent.
func clearRect(buf []byte, stride, x0, y0, w, h int) {
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			putPxA(buf, stride, x, y, 0, 0, 0, 0x00)
		}
	}
}

// paintLeftBorder draws the visible line on the OUTER (left) edge; the rest
// stays transparent. The subsurface is wider than the line purely as a
// resize hit area.
func paintLeftBorder(buf []byte, w, h int) {
	clearRect(buf, w, 0, 0, w, h)
	for y := 0; y < h; y++ {
		putPx(buf, w, 0, y, 0x2B, 0x2D, 0x30)
	}
}

// paintRightBorder draws the visible line on the OUTER (right) edge.
func paintRightBorder(buf []byte, w, h int) {
	clearRect(buf, w, 0, 0, w, h)
	for y := 0; y < h; y++ {
		putPx(buf, w, w-1, y, 0x2B, 0x2D, 0x30)
	}
}

// paintBottomBorder draws the visible line on the OUTER (bottom) edge.
func paintBottomBorder(buf []byte, w, h int) {
	clearRect(buf, w, 0, 0, w, h)
	for x := 0; x < w; x++ {
		putPx(buf, w, x, h-1, 0x2B, 0x2D, 0x30)
	}
}

func paintTitleBar(buf []byte, w, h int, title string, maximized bool) {
	fillRect(buf, w, 0, 0, w, h, 0x2B, 0x2D, 0x30)

	// Buttons right-to-left: [min] [max] [close], each csdCloseW wide.
	minX := w - 3*csdCloseW
	maxX := w - 2*csdCloseW
	closeX := w - csdCloseW

	// Close: red square + white X.
	fillRect(buf, w, closeX, 0, csdCloseW, h, 0xC4, 0x2B, 0x1C)
	cx := closeX + csdCloseW/2
	cy := h / 2
	for i := -5; i <= 5; i++ {
		putPx(buf, w, cx+i, cy+i, 0xFF, 0xFF, 0xFF)
		putPx(buf, w, cx+i, cy-i, 0xFF, 0xFF, 0xFF)
		putPx(buf, w, cx+i+1, cy+i, 0xFF, 0xFF, 0xFF)
		putPx(buf, w, cx+i+1, cy-i, 0xFF, 0xFF, 0xFF)
	}

	// Maximize / restore: square outline.
	drawMaximizeIcon(buf, w, maxX, 0, csdCloseW, h, 0xDF, 0xE1, 0xE5, maximized)

	// Minimize: horizontal line.
	drawMinimizeIcon(buf, w, minX, 0, csdCloseW, h, 0xDF, 0xE1, 0xE5)

	// Title text: simple 5x7 uppercase-ish glyphs (ASCII letters only).
	drawTitle(buf, w, h, title)
}

// drawMinimizeIcon draws a horizontal line centered in the button area.
func drawMinimizeIcon(buf []byte, stride, bx, by, bw, bh int, r, g, b byte) {
	cx := bx + bw/2
	cy := by + bh/2
	for x := cx - 6; x <= cx+6; x++ {
		putPx(buf, stride, x, cy, r, g, b)
	}
}

// drawMaximizeIcon draws a square outline (or two overlapping squares when
// maximized → restore icon).
func drawMaximizeIcon(buf []byte, stride, bx, by, bw, bh int, r, g, b byte, maximized bool) {
	cx := bx + bw/2
	cy := by + bh/2
	if maximized {
		// Restore icon: two overlapping squares.
		for x := cx - 6; x <= cx+3; x++ {
			putPx(buf, stride, x, cy-5, r, g, b)
			putPx(buf, stride, x, cy+4, r, g, b)
		}
		for y := cy - 5; y <= cy+4; y++ {
			putPx(buf, stride, cx-6, y, r, g, b)
			putPx(buf, stride, cx+3, y, r, g, b)
		}
		return
	}
	// Maximize icon: single square outline.
	for x := cx - 6; x <= cx+6; x++ {
		putPx(buf, stride, x, cy-5, r, g, b)
		putPx(buf, stride, x, cy+5, r, g, b)
	}
	for y := cy - 5; y <= cy+5; y++ {
		putPx(buf, stride, cx-6, y, r, g, b)
		putPx(buf, stride, cx+6, y, r, g, b)
	}
}

// drawTitle renders ASCII title using a minimal 5x7 bitmap font.
func drawTitle(buf []byte, stride, h int, title string) {
	x := csdPadLeft
	y0 := (h - 7) / 2
	maxX := stride - 3*csdCloseW - csdPadLeft
	for _, r := range title {
		if x+5 > maxX {
			break
		}
		var g [7]byte
		if r >= 'A' && r <= 'Z' {
			g = font5x7[r-'A']
		} else if r >= 'a' && r <= 'z' {
			g = font5x7[r-'a']
		} else if r >= '0' && r <= '9' {
			g = font5x7[r-'0'+26]
		} else {
			g = font5x7[36] // '?' placeholder
		}
		for row := 0; row < 7; row++ {
			for col := 0; col < 5; col++ {
				if g[row]&(0x10>>uint(col)) != 0 {
					putPx(buf, stride, x+col, y0+row, 0xDF, 0xE1, 0xE5)
				}
			}
		}
		x += 6
	}
}

// font5x7: A-Z, 0-9, '?' — 37 glyphs (5x7, MSB = leftmost).
var font5x7 = [37][7]byte{
	// A..Z
	{0x0E, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11}, // A
	{0x1E, 0x11, 0x11, 0x1E, 0x11, 0x11, 0x1E}, // B
	{0x0E, 0x11, 0x10, 0x10, 0x10, 0x11, 0x0E}, // C
	{0x1E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1E}, // D
	{0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x1F}, // E
	{0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x10}, // F
	{0x0E, 0x11, 0x10, 0x17, 0x11, 0x11, 0x0F}, // G
	{0x11, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11}, // H
	{0x0E, 0x04, 0x04, 0x04, 0x04, 0x04, 0x0E}, // I
	{0x07, 0x02, 0x02, 0x02, 0x02, 0x12, 0x0C}, // J
	{0x11, 0x12, 0x14, 0x18, 0x14, 0x12, 0x11}, // K
	{0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x1F}, // L
	{0x11, 0x1B, 0x15, 0x15, 0x11, 0x11, 0x11}, // M
	{0x11, 0x19, 0x15, 0x13, 0x11, 0x11, 0x11}, // N
	{0x0E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E}, // O
	{0x1E, 0x11, 0x11, 0x1E, 0x10, 0x10, 0x10}, // P
	{0x0E, 0x11, 0x11, 0x11, 0x15, 0x12, 0x0D}, // Q
	{0x1E, 0x11, 0x11, 0x1E, 0x14, 0x12, 0x11}, // R
	{0x0F, 0x10, 0x10, 0x0E, 0x01, 0x01, 0x1E}, // S
	{0x1F, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04}, // T
	{0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E}, // U
	{0x11, 0x11, 0x11, 0x11, 0x0A, 0x0A, 0x04}, // V
	{0x11, 0x11, 0x11, 0x15, 0x15, 0x15, 0x0A}, // W
	{0x11, 0x11, 0x0A, 0x04, 0x0A, 0x11, 0x11}, // X
	{0x11, 0x11, 0x0A, 0x04, 0x04, 0x04, 0x04}, // Y
	{0x1F, 0x01, 0x02, 0x04, 0x08, 0x10, 0x1F}, // Z
	// 0..9
	{0x0E, 0x11, 0x13, 0x15, 0x19, 0x11, 0x0E}, // 0
	{0x04, 0x0C, 0x04, 0x04, 0x04, 0x04, 0x0E}, // 1
	{0x0E, 0x11, 0x01, 0x02, 0x04, 0x08, 0x1F}, // 2
	{0x1F, 0x02, 0x04, 0x02, 0x01, 0x11, 0x0E}, // 3
	{0x02, 0x06, 0x0A, 0x12, 0x1F, 0x02, 0x02}, // 4
	{0x1F, 0x10, 0x1E, 0x01, 0x01, 0x11, 0x0E}, // 5
	{0x06, 0x08, 0x10, 0x1E, 0x11, 0x11, 0x0E}, // 6
	{0x1F, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08}, // 7
	{0x0E, 0x11, 0x11, 0x0E, 0x11, 0x11, 0x0E}, // 8
	{0x0E, 0x11, 0x11, 0x0F, 0x01, 0x02, 0x0C}, // 9
	// '?'
	{0x0E, 0x11, 0x01, 0x02, 0x04, 0x00, 0x04}, // ?
}

// memfdCreate is a small wrapper (avoids importing unix just for this).
func memfdCreate(name string) (int, error) {
	return memfdCreateImpl(name)
}

// memfdCreateImpl calls memfd_create(2) via golang.org/x/sys/unix (portable
// across arches; raw syscall numbers differ per architecture).
func memfdCreateImpl(name string) (int, error) {
	fd, err := unix.MemfdCreate(name, unix.MFD_CLOEXEC)
	if err != nil {
		return -1, err
	}
	return fd, nil
}

