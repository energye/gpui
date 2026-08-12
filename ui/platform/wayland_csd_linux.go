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
	csdBorderW   = 1
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
	// xdg_toplevel move(seat, serial) — opcode 5 (xdg-shell.xml).
	xdgToplevelMove = 5
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
	// set_sync — commits are atomic with the parent surface.
	lib.proxyMarshalArrayFlags(s.sub, wlSubsurfaceSetSync, 0, 0, 0, nil)

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
		paintTitleBar(c.top.data, c.top.w, c.top.h, c.title)
		c.top.commit(lib)
	}
	for _, s := range []*csdSurface{c.left, c.right, c.bottom} {
		if s != nil {
			paintBorder(s.data, s.w, s.h)
			s.commit(lib)
		}
	}
	lib.displayFlush(c.win.display)
}

// destroy tears down all decoration subsurfaces and frees buffers.
func (c *wlCSD) destroy() {
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
	c.top, c.left, c.right, c.bottom = nil, nil, nil, nil
}

// --- pointer interaction (called from wl_pointer callbacks) ---

// pointerInTitleBar reports whether the pointer is over our title bar
// subsurface (surface is the wl_surface the compositor reported).
func (c *wlCSD) pointerInTitleBar(surface uintptr) bool {
	return c != nil && c.top != nil && surface == c.top.surf
}

// titleBarHitClose reports whether (x,y) in title-bar coords hits the close
// button (right edge).
func titleBarHitClose(x float64, w int) bool {
	return x >= float64(w-csdCloseW)
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

// --- minimal ARGB8888 painter (no external deps) ---

// putPx writes one pixel (BGRA in memory for ARGB8888 little-endian).
func putPx(buf []byte, stride, x, y int, r, g, b byte) {
	off := (y*stride + x) * 4
	if off+3 >= len(buf) {
		return
	}
	buf[off] = b
	buf[off+1] = g
	buf[off+2] = r
	buf[off+3] = 0xFF
}

func fillRect(buf []byte, stride, x0, y0, w, h int, r, g, b byte) {
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			putPx(buf, stride, x, y, r, g, b)
		}
	}
}

func paintBorder(buf []byte, w, h int) {
	fillRect(buf, w, 0, 0, w, h, 0x2B, 0x2D, 0x30)
}

func paintTitleBar(buf []byte, w, h int, title string) {
	fillRect(buf, w, 0, 0, w, h, 0x2B, 0x2D, 0x30)
	// Close button: red square + white X.
	closeX := w - csdCloseW
	fillRect(buf, w, closeX, 0, csdCloseW, h, 0xC4, 0x2B, 0x1C)
	cx := closeX + csdCloseW/2
	cy := h / 2
	for i := -5; i <= 5; i++ {
		putPx(buf, w, cx+i, cy+i, 0xFF, 0xFF, 0xFF)
		putPx(buf, w, cx+i, cy-i, 0xFF, 0xFF, 0xFF)
		putPx(buf, w, cx+i+1, cy+i, 0xFF, 0xFF, 0xFF)
		putPx(buf, w, cx+i+1, cy-i, 0xFF, 0xFF, 0xFF)
	}
	// Title text: simple 5x7 uppercase-ish glyphs (ASCII letters only).
	drawTitle(buf, w, h, title)
}

// drawTitle renders ASCII title using a minimal 5x7 bitmap font.
func drawTitle(buf []byte, stride, h int, title string) {
	x := csdPadLeft
	y0 := (h - 7) / 2
	for _, r := range title {
		if x+5 > stride-csdCloseW {
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

