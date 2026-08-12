//go:build linux

package platform

import (
	"fmt"
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
	xdgToplevelMove         = 5
	xdgToplevelResize       = 6
	xdgToplevelSetMaximized = 9
	xdgToplevelUnsetMaxim   = 10
	xdgToplevelSetMinimized = 13
)

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
	if c.top != nil {
		paintTitleBar(c.top.data, c.top.w, c.top.h, c.state)
		c.top.commit(lib)
	}
	if c.left != nil {
		paintBorder(c.left.data, c.left.w, c.left.h, csdEdgeLeft)
		c.left.commit(lib)
	}
	if c.right != nil {
		paintBorder(c.right.data, c.right.w, c.right.h, csdEdgeRight)
		c.right.commit(lib)
	}
	if c.bottom != nil {
		paintBorder(c.bottom.data, c.bottom.w, c.bottom.h, csdEdgeBottom)
		c.bottom.commit(lib)
	}
	lib.displayFlush(c.win.display)
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
	c.paint()
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
	c.repaintTitle()
}

// setFullscreen updates fullscreen flag (no visible borders when fullscreen).
func (c *wlCSD) setFullscreen(v bool) {
	if c == nil || c.state.Fullscreen == v {
		return
	}
	c.state.Fullscreen = v
	c.paint()
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
func (c *wlCSD) hitTest(surface uintptr, x, y float64) csdHit {
	if c == nil {
		return csdHit{}
	}
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
		// Resize top edge/corners (non-button area of the title bar).
		if y < csdCornerGrip {
			switch {
			case x < csdCornerGrip:
				return csdHit{act: csdActResize, edge: resizeTopLeft}
			case x >= float64(wTop-csdCornerGrip):
				return csdHit{act: csdActResize, edge: resizeTopRight}
			default:
				return csdHit{act: csdActResize, edge: resizeTop}
			}
		}
		return csdHit{act: csdActMove}
	case c.left.surf:
		switch {
		case y < csdCornerGrip:
			return csdHit{act: csdActResize, edge: resizeTopLeft}
		case y >= float64(c.left.h-csdCornerGrip):
			return csdHit{act: csdActResize, edge: resizeBottomLeft}
		default:
			return csdHit{act: csdActResize, edge: resizeLeft}
		}
	case c.right.surf:
		switch {
		case y < csdCornerGrip:
			return csdHit{act: csdActResize, edge: resizeTopRight}
		case y >= float64(c.right.h-csdCornerGrip):
			return csdHit{act: csdActResize, edge: resizeBottomRight}
		default:
			return csdHit{act: csdActResize, edge: resizeRight}
		}
	case c.bottom.surf:
		switch {
		case x < csdCornerGrip:
			return csdHit{act: csdActResize, edge: resizeBottomLeft}
		case x >= float64(c.bottom.w-csdCornerGrip):
			return csdHit{act: csdActResize, edge: resizeBottomRight}
		default:
			return csdHit{act: csdActResize, edge: resizeBottom}
		}
	case c.win.surface:
		// Content-surface edge hot zone (standard CSD: invisible resize
		// border inside the content too). (x,y) is content-local.
		if c.state.Fullscreen || c.state.Maximized {
			return csdHit{}
		}
		cw, ch := c.stateW()
		nearL := x < csdCornerGrip
		nearR := x >= float64(cw-csdCornerGrip)
		nearT := y < csdCornerGrip
		nearB := y >= float64(ch-csdCornerGrip)
		switch {
		case nearL && nearT:
			return csdHit{act: csdActResize, edge: resizeTopLeft}
		case nearR && nearT:
			return csdHit{act: csdActResize, edge: resizeTopRight}
		case nearL && nearB:
			return csdHit{act: csdActResize, edge: resizeBottomLeft}
		case nearR && nearB:
			return csdHit{act: csdActResize, edge: resizeBottomRight}
		case nearL:
			return csdHit{act: csdActResize, edge: resizeLeft}
		case nearR:
			return csdHit{act: csdActResize, edge: resizeRight}
		case nearT:
			return csdHit{act: csdActResize, edge: resizeTop}
		case nearB:
			return csdHit{act: csdActResize, edge: resizeBottom}
		}
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

// onButtonPress handles a button press on the chrome. Returns whether the
// event was consumed (true = do not forward to content).
func (c *wlCSD) onButtonPress(seat, serial uintptr, hit csdHit) bool {
	if c == nil {
		return false
	}
	if b := c.buttonHitFor(hit); b != nil {
		b.Pressed = true
		c.repaintTitle()
	}
	switch hit.act {
	case csdActClose:
		c.closeRequested = true
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
	case csdActMinimize:
		c.requestMinimize()
	case csdActMaximize:
		c.toggleMaximize()
	}
	return true
}

// onButtonRelease clears pressed states after a chrome press.
func (c *wlCSD) onButtonRelease() {
	if c == nil {
		return
	}
	if c.state.Close.Pressed || c.state.Maximize.Pressed || c.state.Minimize.Pressed {
		c.state.Close.Pressed = false
		c.state.Maximize.Pressed = false
		c.state.Minimize.Pressed = false
		c.repaintTitle()
	}
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
		// Restore default cursor: set_cursor(serial, NULL, 0, 0).
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
	surf := c.ensureCursorSurface()
	if surf == 0 {
		return
	}
	c.applyCursorImage(name)
	if c.curName == "" {
		c.curName = name // even if image failed, avoid re-loop
	}
	args := []wlArg{argU(uint32(serial)), argO(surf), argU(0), argU(0)}
	c.win.lib.proxyMarshalArrayFlags(c.win.ptr.ptr, wlPtrSetCursor, 0, 0, 0, &args[0])
	c.win.lib.displayFlush(c.win.display)
}

// ensureCursorSurface loads the cursor theme once and returns the cursor
// surface (created lazily).
func (c *wlCSD) ensureCursorSurface() uintptr {
	if c == nil || c.win == nil || c.win.lib == nil || c.shm == 0 {
		return 0
	}
	if c.cursorSurf != 0 && c.cursorTheme != 0 {
		return c.cursorSurf
	}
	lib := c.win.lib
	l := loadCursorLib()
	if l == nil {
		return 0
	}
	if c.cursorSurf == 0 {
		c.cursorSurf = c.win.ctor(c.win.comp, wlCompositorCreateSurface, lib.ifaceSurface, 4)
		if c.cursorSurf == 0 {
			return 0
		}
	}
	if c.cursorTheme == 0 {
		var name *byte
		c.cursorTheme = l.themeLoad(name, 24, c.shm)
	}
	return c.cursorSurf
}

// applyCursorImage loads the cursor image for name into cursorSurf.
func (c *wlCSD) applyCursorImage(name string) {
	if c == nil || c.win == nil || c.win.lib == nil || c.cursorSurf == 0 || c.cursorTheme == 0 {
		return
	}
	l := loadCursorLib()
	if l == nil {
		return
	}
	nb := append([]byte(name), 0)
	cur := l.themeGetCur(c.cursorTheme, &nb[0])
	if cur == 0 {
		return
	}
	// wl_cursor { unsigned image_count; wl_cursor_image **images; char *name; }
	// images pointer at offset 8 (after u32 count + pad).
	imgPtr := *(*uintptr)(unsafe.Pointer(cur + 8))
	if imgPtr == 0 {
		return
	}
	img := (*wlCursorImageC)(unsafe.Pointer(imgPtr))
	if img.Buffer == 0 {
		return
	}
	args := []wlArg{argO(img.Buffer), argU(0), argU(0)}
	c.win.lib.proxyMarshalArrayFlags(c.cursorSurf, wlSurfaceAttach, 0, 0, 0, &args[0])
	c.win.lib.proxyMarshalArrayFlags(c.cursorSurf, wlSurfaceCommit, 0, 0, 0, nil)
	c.win.lib.displayFlush(c.win.display)
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