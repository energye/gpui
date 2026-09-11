//go:build linux

package platform

import (
	"fmt"
	"os"
)

// waylandController implements WindowController for the Wayland backend via
// xdg_toplevel requests + configure-driven state reconciliation. Requests
// are marshalled through libwayland (purego, no CGO); protocol-impossible
// operations (Position/Focus/AlwaysOnTop/Decoration-toggle) return
// ErrUnsupported — the honesty contract of ENGINE_WINDOW_API.md §2.5.4.
// Show/Hide are real (hide = destroy the surface stack, show = recreate —
// §6.2 of ENGINE_WAYLAND_WINDOW_STANDARD.md).
//
// §2.5.5 thread contract: all methods may be called from any goroutine
// (libwayland proxy marshalling is internally locked during dispatch); the
// tracked state is guarded by wlWin.ctlMu.
type waylandController struct {
	h *wlHost
}

func (c *waylandController) win() *wlWin {
	if c == nil || c.h == nil {
		return nil
	}
	return c.h.win
}

// Title returns the last-applied title (tracked; xdg has no query).
func (c *waylandController) Title() string {
	w := c.win()
	if w == nil {
		return ""
	}
	w.ctlMu.Lock()
	defer w.ctlMu.Unlock()
	return w.title
}

func (c *waylandController) SetTitle(t string) {
	w := c.win()
	if w == nil || w.lib == nil {
		return
	}
	w.ctlMu.Lock()
	w.title = t
	w.ctlMu.Unlock()
	// The marshal copies the string synchronously; pin it anyway to match the
	// Create path and keep the pointer stable across dispatch. The tracked
	// title/pin survive a hide/show re-create (the new xdg_toplevel is
	// re-titled from titlePin in createSurfaceStack).
	w.titlePin = append([]byte(t), 0)
	if w.toplevel == 0 {
		return // hidden (surface stack detached) — re-applied on Show
	}
	args := []wlArg{argS(cstr(w.titlePin))}
	w.lib.proxyMarshalArrayFlags(w.toplevel, xdgToplevelSetTitle, 0, 0, 0, &args[0])
	w.lib.displayFlush(w.display)
	// The CSD title bar shows the same string (GTK parity).
	if w.csd != nil {
		w.csd.state.Title = t
		w.csd.repaintTitle()
	}
}

// Size returns the current client area (configure-delivered; SetSize updates
// it optimistically until the next configure reconciles).
func (c *waylandController) Size() (int, int) {
	w := c.win()
	if w == nil {
		return 0, 0
	}
	return w.width, w.height
}

// SetSize routes through the min==max clamp (§2.5.2): Wayland has no direct
// resize request, so the requested size becomes a hard constraint until
// SetMinSize/SetMaxSize re-open it or SetResizable(true) restores the user
// constraints. The compositor confirms with a configure (EventResize). The
// clamp also locks the CSD resize grips (fixed-size window shows no resize
// affordances — GTK4 parity).
func (c *waylandController) SetSize(wid, ht int) {
	w := c.win()
	if w == nil {
		return
	}
	if wid < 1 {
		wid = 1
	}
	if ht < 1 {
		ht = 1
	}
	w.top2i(xdgToplevelSetMinSize, wid, ht)
	w.top2i(xdgToplevelSetMaxSize, wid, ht)
	// Optimistic until configure; keeps Size() consistent for callers that
	// read back immediately (pfkit probe pattern).
	w.ctlMu.Lock()
	w.width, w.height = wid, ht
	w.locked = true
	w.ctlMu.Unlock()
	if w.csd != nil {
		w.csd.setLocked(true)
	}
}

func (c *waylandController) SetMinSize(wid, ht int) {
	w := c.win()
	if w == nil {
		return
	}
	w.ctlMu.Lock()
	w.minW, w.minH = wid, ht
	// Re-opening the constraint lifts the SetSize min==max lock.
	w.locked = false
	w.ctlMu.Unlock()
	if w.csd != nil {
		w.csd.setLocked(false)
	}
	w.top2i(xdgToplevelSetMinSize, wid, ht)
}

func (c *waylandController) SetMaxSize(wid, ht int) {
	w := c.win()
	if w == nil {
		return
	}
	w.ctlMu.Lock()
	w.maxW, w.maxH = wid, ht
	w.locked = false
	w.ctlMu.Unlock()
	if w.csd != nil {
		w.csd.setLocked(false)
	}
	w.top2i(xdgToplevelSetMaxSize, wid, ht)
}

// SetResizable locks (min==max=current size) or unlocks (restore user
// constraints) via xdg min/max requests — the Wayland equivalent of X11 size
// hints. Locking also disables the CSD resize grips.
func (c *waylandController) SetResizable(r bool) {
	w := c.win()
	if w == nil {
		return
	}
	w.ctlMu.Lock()
	if r == w.resizable {
		if r && w.locked {
			// A SetSize min==max clamp is in effect — SetResizable(true)
			// lifts it even when the flag did not change (§2.5.2 contract).
			w.locked = false
			w.ctlMu.Unlock()
			if w.csd != nil {
				w.csd.setLocked(false)
			}
			w.top2i(xdgToplevelSetMinSize, w.minW, w.minH)
			w.top2i(xdgToplevelSetMaxSize, w.maxW, w.maxH)
		} else {
			w.ctlMu.Unlock()
		}
		return
	}
	w.resizable = r
	var minW, minH, maxW, maxH int
	if !r {
		minW, minH, maxW, maxH = w.width, w.height, w.width, w.height
		w.locked = true
		w.ctlMu.Unlock()
	} else {
		minW, minH = w.minW, w.minH
		maxW, maxH = w.maxW, w.maxH
		w.locked = false
		w.ctlMu.Unlock()
	}
	if w.csd != nil {
		w.csd.setLocked(!r)
	}
	w.top2i(xdgToplevelSetMinSize, minW, minH)
	w.top2i(xdgToplevelSetMaxSize, maxW, maxH)
}

func (c *waylandController) IsResizable() bool {
	w := c.win()
	if w == nil {
		return false
	}
	w.ctlMu.Lock()
	defer w.ctlMu.Unlock()
	return w.resizable
}

// SetDecorations: the CSD is a creation-time choice (subsurfaces + shm
// buffers); rebuilding it at runtime is not cheap, so the toggle is
// unsupported (§2.3 Wayland column).
func (c *waylandController) SetDecorations(dec bool) error {
	return ErrUnsupported
}

func (c *waylandController) IsDecorated() bool {
	w := c.win()
	return w != nil && w.csd != nil
}

// SetIgnoreCursorEvents makes the window pointer-transparent via
// wl_surface.set_input_region: an empty wl_region = no input region; a NULL
// region restores the default (whole surface). Used for overlays.
func (c *waylandController) SetIgnoreCursorEvents(ignore bool) error {
	w := c.win()
	if w == nil || w.lib == nil || w.surface == 0 || w.lib.ifaceRegion == 0 {
		return ErrUnsupported
	}
	if !ignore {
		args := []wlArg{argO(0)}
		w.lib.proxyMarshalArrayFlags(w.surface, wlSurfaceSetInputRegion, 0, 0, 0, &args[0])
		w.lib.displayFlush(w.display)
		return nil
	}
	// Empty region: create + set, no add() calls → nothing is clickable.
	args := []wlArg{argNewID()}
	r := w.lib.proxyMarshalArrayCtor(w.comp, wlCompositorCreateRegion, &args[0], w.lib.ifaceRegion, 1)
	if r == 0 {
		return ErrUnsupported
	}
	sargs := []wlArg{argO(r)}
	w.lib.proxyMarshalArrayFlags(w.surface, wlSurfaceSetInputRegion, 0, 0, 0, &sargs[0])
	// The region is not needed after set_input_region copies the geometry.
	w.lib.proxyMarshalArrayFlags(r, wlRegionDestroy, 0, 0, 0, nil)
	w.lib.proxyDestroy(r)
	w.lib.displayFlush(w.display)
	return nil
}

// Position: xdg has no client-side positioning; the query answers ok=false
// (zero values ≠ "at origin", §2.5.3).
func (c *waylandController) Position() (int, int, bool) {
	return 0, 0, false
}

func (c *waylandController) SetPosition(x, y int) error {
	return ErrUnsupported
}

func (c *waylandController) Minimize() {
	w := c.win()
	if w == nil {
		return
	}
	w.ctlMu.Lock()
	mx, fl := w.maximized, w.fullscreen
	w.ctlMu.Unlock()
	w.setStateTriple(true, mx, fl) // optimistic until activated reverses it
	w.topNoArg(xdgToplevelSetMinimized)
}

// IsMinimized returns the optimistic tracked value (protocol has no
// minimized state; set_minimized → true, activated configure → false, §3.2).
func (c *waylandController) IsMinimized() bool {
	w := c.win()
	if w == nil {
		return false
	}
	w.ctlMu.Lock()
	defer w.ctlMu.Unlock()
	return w.minimized
}

func (c *waylandController) Maximize() {
	w := c.win()
	if w == nil {
		return
	}
	w.ctlMu.Lock()
	mn, fl := w.minimized, w.fullscreen
	w.ctlMu.Unlock()
	w.setStateTriple(mn, true, fl) // optimistic; configure reconciles
	w.topNoArg(xdgToplevelSetMaximized)
}

func (c *waylandController) Unmaximize() {
	w := c.win()
	if w == nil {
		return
	}
	w.ctlMu.Lock()
	mn, fl := w.minimized, w.fullscreen
	w.ctlMu.Unlock()
	w.setStateTriple(mn, false, fl)
	w.topNoArg(xdgToplevelUnsetMaxim)
}

func (c *waylandController) IsMaximized() bool {
	w := c.win()
	if w == nil {
		return false
	}
	w.ctlMu.Lock()
	defer w.ctlMu.Unlock()
	return w.maximized
}

func (c *waylandController) SetFullscreen(fs bool) {
	w := c.win()
	if w == nil {
		return
	}
	w.ctlMu.Lock()
	mn, mx := w.minimized, w.maximized
	w.ctlMu.Unlock()
	w.setStateTriple(mn, mx, fs)
	if fs {
		// NULL output = the compositor picks the current output (§2.4).
		w.top1o(xdgToplevelSetFullscreen, 0)
	} else {
		w.topNoArg(xdgToplevelUnsetFullscreen)
	}
}

func (c *waylandController) IsFullscreen() bool {
	w := c.win()
	if w == nil {
		return false
	}
	w.ctlMu.Lock()
	defer w.ctlMu.Unlock()
	return w.fullscreen
}

// Show/Hide: xdg has no map/unmap concept, so hiding destroys the surface
// stack (Hide → EventHidden{true} → embedder closes the GPU target and calls
// HiddenSurface.ApplyHiddenDetach — the window truly unmaps, GTK4
// gdk_wayland_window_hide parity) and Show re-creates it. Show may be called
// while the stack is gone; EventHidden{false} is then deferred until the
// re-created surface is re-mapped (§6.2).
func (c *waylandController) Show() error {
	w := c.win()
	if w == nil {
		return ErrUnsupported
	}
	w.showNative()
	return nil
}

func (c *waylandController) Hide() error {
	w := c.win()
	if w == nil {
		return ErrUnsupported
	}
	w.hideNative()
	return nil
}

func (c *waylandController) IsVisible() bool {
	w := c.win()
	if w == nil {
		return true
	}
	return !w.isHidden()
}

// Focus: xdg has no focus request; IsFocused reports the configure-delivered
// activated state (true value, §2.3 "查=activated").
func (c *waylandController) Focus() error {
	return ErrUnsupported
}

func (c *waylandController) IsFocused() bool {
	w := c.win()
	if w == nil {
		return false
	}
	return w.activated
}

func (c *waylandController) SetAlwaysOnTop(on bool) error {
	return ErrUnsupported
}

// RequestMove / RequestResize start a compositor-driven interactive
// move/resize (xdg_toplevel.move/resize). They require a valid input serial
// (the last pointer.enter) and a seat — without them the request would be
// silently dropped by the compositor, so an honest ErrUnsupported is
// returned instead of a fake success.
func (c *waylandController) RequestMove() error {
	w := c.win()
	if w == nil {
		return ErrUnsupported
	}
	seat, serial := w.grabInput()
	if seat == 0 || serial == 0 {
		return ErrUnsupported
	}
	args := []wlArg{argO(seat), argU(uint32(serial))}
	w.lib.proxyMarshalArrayFlags(w.toplevel, xdgToplevelMove, 0, 0, 0, &args[0])
	w.lib.displayFlush(w.display)
	return nil
}

func (c *waylandController) RequestResize(edge WindowEdge) error {
	w := c.win()
	if w == nil {
		return ErrUnsupported
	}
	e := wlEdgeOf(edge)
	if e == 0 {
		return ErrUnsupported // WindowEdgeNone / unknown
	}
	seat, serial := w.grabInput()
	if seat == 0 || serial == 0 {
		return ErrUnsupported
	}
	args := []wlArg{argO(seat), argU(uint32(serial)), argU(uint32(e))}
	w.lib.proxyMarshalArrayFlags(w.toplevel, xdgToplevelResize, 0, 0, 0, &args[0])
	w.lib.displayFlush(w.display)
	return nil
}

// grabInput returns the seat + last pointer.enter serial when the pointer is
// (or has been) over the window — the serial the compositor accepts for
// interactive move/resize. Zero serial = never entered (invalid request).
func (w *wlWin) grabInput() (uintptr, uintptr) {
	if w == nil || w.seat == 0 || w.ptr == nil {
		return 0, 0
	}
	return w.seat, w.ptr.enterSerial
}

// hideNative marks the window hidden, hides the CSD chrome and reports
// EventHidden{true} so the embedder stops frames, closes the GPU present
// target and then destroys the surface stack (HiddenSurface.ApplyHiddenDetach
// → the window truly unmaps, GTK parity). Idempotent.
func (w *wlWin) hideNative() {
	if w == nil || w.lib == nil {
		return
	}
	w.ctlMu.Lock()
	if w.hidden {
		w.ctlMu.Unlock()
		return
	}
	w.hidden = true
	w.ctlMu.Unlock()
	if w.csd != nil {
		w.csd.setVisible(false)
	}
	w.focusMu.Lock()
	w.focusEvents = append(w.focusEvents, Event{Type: EventHidden, Hidden: true})
	w.focusMu.Unlock()
}

// showNative flips the window back to visible. When the surface stack was
// detached (ApplyHiddenDetach), it re-creates the stack on the event thread's
// behalf WITHOUT dispatching (the pump is the only dispatcher): the first
// configure of the new toplevel is acked+committed by the pump (map), and
// poll then emits EventHidden{false} — which is when the embedder recreates
// the GPU present target on the NEW wl_surface. When the stack is still alive
// (chrome-only hide), the CSD is re-attached and EventHidden{false} fires
// immediately. Idempotent.
func (w *wlWin) showNative() {
	if w == nil || w.lib == nil {
		return
	}
	w.ctlMu.Lock()
	if !w.hidden {
		w.ctlMu.Unlock()
		return
	}
	w.hidden = false
	w.ctlMu.Unlock()
	if w.surface == 0 {
		// Recreate the whole stack (surface gone after ApplyHiddenDetach).
		if err := w.createSurfaceStack(true); err != nil {
			fmt.Fprintf(os.Stderr, "wayland: showNative: %v\n", err)
			w.ctlMu.Lock()
			w.hidden = true
			w.ctlMu.Unlock()
			return
		}
		w.applySurfaceConfig()
		w.ctlMu.Lock()
		w.recreated = true
		w.ctlMu.Unlock()
		// EventHidden{false} is deferred to poll, after the re-map configure.
		return
	}
	if w.csd != nil {
		w.csd.setVisible(true)
	}
	w.focusMu.Lock()
	w.focusEvents = append(w.focusEvents, Event{Type: EventHidden, Hidden: false})
	w.focusMu.Unlock()
}

// isHidden reports the tracked visibility state (Hide/Options.Visible=false).
func (w *wlWin) isHidden() bool {
	if w == nil {
		return false
	}
	w.ctlMu.Lock()
	defer w.ctlMu.Unlock()
	return w.hidden
}

// wlEdgeOf maps a cross-platform WindowEdge to the xdg_toplevel resize_edge
// enum (xdg-shell.xml authoritative): none=0 top=1 bottom=2 left=4
// top_left=5 bottom_left=6 right=8 top_right=9 bottom_right=10.
func wlEdgeOf(e WindowEdge) int {
	switch e {
	case WindowEdgeTop:
		return resizeTop
	case WindowEdgeBottom:
		return resizeBottom
	case WindowEdgeLeft:
		return resizeLeft
	case WindowEdgeTopLeft:
		return resizeTopLeft
	case WindowEdgeBottomLeft:
		return resizeBottomLeft
	case WindowEdgeRight:
		return resizeRight
	case WindowEdgeTopRight:
		return resizeTopRight
	case WindowEdgeBottomRight:
		return resizeBottomRight
	default:
		return 0
	}
}

// SetCursor records the active cursor and applies it immediately when the
// pointer is over the window (valid enter serial). Otherwise it is applied
// on the next enter/motion (CSD setCursor path). Best-effort: frameless
// windows without a wl_shm/cursor surface silently skip the visual swap.
func (c *waylandController) SetCursor(cur Cursor) {
	w := c.win()
	if w == nil {
		return
	}
	w.ctlMu.Lock()
	w.cursor = cur
	w.ctlMu.Unlock()
	if w.ptr == nil || w.ptr.surface == 0 || w.ptr.enterSerial == 0 || w.csd == nil {
		return
	}
	hit := w.csd.hitTest(w.ptr.surface, w.ptr.lastX, w.ptr.lastY)
	w.csd.setCursor(w.ptr.enterSerial, hit)
}

// cursorThemeName maps the cross-platform Cursor set to the X cursor theme
// name (standard xcursor names present in adwaita/breeze/…); "" = default.
func cursorThemeName(cur Cursor) string {
	switch cur {
	case CursorText:
		return "text"
	case CursorPointer:
		return "pointer"
	case CursorCrosshair:
		return "crosshair"
	case CursorWait:
		return "wait"
	case CursorResizeH:
		return "sb_h_double_arrow"
	case CursorResizeV:
		return "sb_v_double_arrow"
	case CursorResizeNE:
		return "top_right_corner"
	case CursorResizeNW:
		return "top_left_corner"
	default:
		return ""
	}
}