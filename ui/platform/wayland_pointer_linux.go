//go:build linux

package platform

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/ebitengine/purego"
)

// ptrDbg enables pointer-event diagnostics (GPUI_WL_DBG_PTR=1): every
// enter/motion/leave prints its target surface and surface-local coords, so
// compositor↔CSD interaction can be audited on a live window without a
// second input device. The compositor decides which surface the pointer
// events target; a mismatch (e.g. events on "content" while the pointer is
// over the title bar) shows up here instantly.
var ptrDbg = os.Getenv("GPUI_WL_DBG_PTR") == "1"

// ptrSurfName classifies a wl_surface for diagnostics.
func ptrSurfName(w *wlWin, surface uintptr) string {
	if surface == 0 {
		return "none"
	}
	if w == nil {
		return "?"
	}
	if surface == w.surface {
		return "content"
	}
	if c := w.csd; c != nil {
		switch surface {
		case c.topSurface:
			return "top"
		case c.left.surf:
			return "left"
		case c.right.surf:
			return "right"
		case c.bottom.surf:
			return "bottom"
		}
	}
	return "foreign"
}

// wl_pointer binding via purego. Standard Wayland client input: the seat's
// pointer provides enter/leave/motion/button/axis events that drive mouse
// input (hit-test, gestures, clicks). No xkb dependency — button/axis map
// directly; coordinates are surface-local (buf.scale handling is a later
// refinement; GNOME Wayland scale 1 for now).
//
// Protocol (wl_pointer, stable v1 — event subset v1-v5 is enough):
//
//	requests: set_cursor(0)[serial,surface,hotspot,h,v], release(1), ...
//	events:   enter(0)[serial,surface,pos_fixed,surface_x_fixed,surface_y_fixed]
//	          leave(1)[serial,surface]
//	          motion(2)[time,pos_fixed,surface_x_fixed,surface_y_fixed]
//	          button(3)[serial,time,button,state]
//	          axis(4)[time,axis,value_fixed]
//	          frame(5) release(6) axis_source(7)...
//
// enter/motion coords are in surface-local coordinate space *before* the
// surface's scale is applied (wl_fixed). We treat them as logical px.
//
// Window-level pointer model (GTK4 parity): the app sees ONE continuous
// coordinate space over the whole window — content + CSD chrome. Pointer
// coordinates from the decoration subsurfaces are translated into content
// coordinates (negative y over the title bar); enter/leave are reported only
// when the pointer crosses the WINDOW boundary, and internal
// content↔chrome crossings arrive as motion.

// wl_pointer requests (wayland.xml authoritative): set_cursor(0) since 1,
// release(1) since 3.
const (
	wlPtrRelease   = 1
	wlPtrSetCursor = 0

	wlPtrEnter        = 0
	wlPtrLeave        = 1
	wlPtrMotion       = 2
	wlPtrButton       = 3
	wlPtrAxis         = 4
	wlPtrFrame        = 5
	wlPtrAxisSource   = 6
	wlPtrAxisStop     = 7
	wlPtrAxisDiscrete = 8
)

// wlPointerState holds the bound wl_pointer proxy + listener table.
// wl_pointer_interface has 9 events (v1-v7); the listener array MUST match
// event_count exactly or proxyAddListener reads out of bounds → wild pointer
// callbacks → SIGSEGV (system crash).
type wlPointerState struct {
	lib *wlLib
	win *wlWin
	ptr uintptr // wl_pointer proxy

	listener [9]uintptr
	selfPtr  uintptr
	// surface tracks the wl_surface under the pointer (set on enter, cleared
	// on leave) so CSD title-bar / close-button interaction can be routed.
	surface uintptr
	// lastX/lastY: last surface-local pointer position (for CSD hit-testing).
	lastX, lastY float64
	// lastAppX/lastAppY: last content-space position (stamped onto Leave —
	// wl_pointer.leave carries no coordinates of its own).
	lastAppX, lastAppY float64
	// enterSerial: serial of the last pointer.enter (needed for set_cursor).
	enterSerial uintptr

	// Window-level pointer state (whole window = content + CSD chrome).
	// inWindow is true while the pointer is logically inside the window.
	inWindow bool
	// lastLeaveOurs: the last pointer event was a leave of one of our
	// surfaces. It is consumed by the next enter on one of our surfaces
	// (internal content↔chrome crossing → motion) or resolved in poll()
	// (resolveDeferredLeave) as a window leave when nothing follows.
	lastLeaveOurs bool
	// lastLeaveSerial is the serial of the deferred leave (cursor restore).
	lastLeaveSerial uintptr
}

// bindPointer creates a wl_pointer from the seat and adds the listener.
// Returns nil when unavailable (silent degrade).
func (w *wlWin) bindPointer() *wlPointerState {
	if w == nil || w.lib == nil || w.seat == 0 || w.lib.ifacePointer == 0 {
		return nil
	}
	st := &wlPointerState{lib: w.lib, win: w}
	st.selfPtr = uintptr(unsafe.Pointer(st))
	// seat.get_pointer(new_id wl_pointer) — opcode 1 on wl_seat.
	args := []wlArg{argNewID()}
	st.ptr = w.lib.proxyMarshalArrayCtor(w.seat, wlSeatGetPointer, &args[0], w.lib.ifacePointer, 1)
	if st.ptr == 0 {
		return nil
	}
	// zwp_cursor_shape_device_v1 (compositor-rendered cursor): attach when
	// the manager global was advertised; nil → callers fall back to the
	// wl_cursor_theme path.
	w.cursors = w.bindCursors(st.ptr)
	// All 9 wl_pointer events must have a slot.
	st.listener[wlPtrEnter] = purego.NewCallback(wlPtrEnterCB)
	st.listener[wlPtrLeave] = purego.NewCallback(wlPtrLeaveCB)
	st.listener[wlPtrMotion] = purego.NewCallback(wlPtrMotionCB)
	st.listener[wlPtrButton] = purego.NewCallback(wlPtrButtonCB)
	st.listener[wlPtrAxis] = purego.NewCallback(wlPtrAxisCB)
	st.listener[wlPtrFrame] = purego.NewCallback(wlPtrFrameCB)
	st.listener[wlPtrAxisSource] = purego.NewCallback(wlPtrAxisSourceCB)
	st.listener[wlPtrAxisStop] = purego.NewCallback(wlPtrAxisStopCB)
	st.listener[wlPtrAxisDiscrete] = purego.NewCallback(wlPtrAxisDiscreteCB)
	if w.lib.proxyAddListener(st.ptr, uintptr(unsafe.Pointer(&st.listener[0])), st.selfPtr) != 0 {
		w.lib.proxyDestroy(st.ptr)
		return nil
	}
	return st
}

func (st *wlPointerState) destroy() {
	if st == nil {
		return
	}
	if st.win != nil && st.win.cursors != nil {
		st.win.cursors.destroy()
		st.win.cursors = nil
	}
	if st.ptr != 0 && st.lib != nil {
		st.lib.proxyDestroy(st.ptr)
		st.ptr = 0
	}
}

func ptrFrom(data uintptr) *wlPointerState {
	if data == 0 {
		return nil
	}
	return (*wlPointerState)(unsafe.Pointer(data))
}

// wlFixedToDouble converts wl_fixed (24.8 signed) to float64.
func wlFixedToDouble(f uintptr) float64 {
	return float64(int32(f)) / 256.0
}

// isOurs reports whether the surface belongs to this window (content or CSD
// chrome) — window-level pointer tracking treats them as one surface.
func (st *wlPointerState) isOurs(surface uintptr) bool {
	if st == nil {
		return false
	}
	return st.win.ownsSurface(surface)
}

// isChrome reports whether the surface is a CSD decoration subsurface
// (title bar / borders) — chrome presses are consumed, never forwarded.
func (st *wlPointerState) isChrome(surface uintptr) bool {
	if st == nil || st.win == nil || surface == 0 || st.win.csd == nil {
		return false
	}
	c := st.win.csd
	return surface == c.topSurface || surface == c.left.surf ||
		surface == c.right.surf || surface == c.bottom.surf
}

// appXY translates surface-local coords to the content (toplevel) space for
// app-facing events (identity for the content surface).
func (st *wlPointerState) appXY(surface uintptr, x, y float64) (float64, float64) {
	if st == nil || st.win == nil || st.win.csd == nil {
		return x, y
	}
	return st.win.csd.appCoords(surface, x, y)
}


// applyCursor routes the cursor update: the zwp_cursor_shape_v1 device when
// bound (compositor-rendered; no client image traffic), else the CSD
// wl_cursor_theme path. hit==csdHit{} restores the default arrow.
func (st *wlPointerState) applyCursor(serial uintptr, hit csdHit) {
	if st == nil || st.win == nil {
		return
	}
	if c := st.win.cursors; c != nil && c.dev != 0 {
		c.setCursor(serial, hit)
		return
	}
	if c := st.win.csd; c != nil {
		c.setCursor(serial, hit)
	}
}

// leaveWindow pushes the window-level PointerLeave and clears chrome state
// (hover highlight + cursor).
//go:noinline // state here is mutated through uintptr self-pointers, which
// defeat the optimizer's alias tracking: inlining into a cached-view caller
// risks stale field reads, so every invocation reloads from memory.
func (st *wlPointerState) leaveWindow(serial uintptr) {
	if st == nil || st.win == nil {
		return
	}
	st.inWindow = false
	if c := st.win.csd; c != nil {
		c.onHover(0, 0, 0)
		c.setCursor(serial, csdHit{})
	}
	st.win.pushPtr(Event{Type: EventPointer, Pointer: PointerLeave, X: st.lastAppX, Y: st.lastAppY})
}

// resolveDeferredLeave flushes an unconsumed leave of one of our surfaces as
// a window-level PointerLeave. Called from poll() right after dispatch: an
// internal content↔chrome crossing consumes lastLeaveOurs via its paired
// enter; anything left over means the pointer left the window (possibly with
// no enter at all — pointer over no surface).
//go:noinline // same uintptr-aliasing reason as leaveWindow.
func (st *wlPointerState) resolveDeferredLeave() {
	if st == nil || !st.lastLeaveOurs {
		return
	}
	st.lastLeaveOurs = false
	if st.inWindow {
		st.leaveWindow(st.lastLeaveSerial)
	}
}

func wlPtrEnterCB(data, ptr, serial, surface, sx, sy uintptr) {
	st := ptrFrom(data)
	if st == nil || st.win == nil {
		return
	}
	x := wlFixedToDouble(sx)
	y := wlFixedToDouble(sy)
	if ptrDbg {
		fmt.Fprintf(os.Stderr, "PTR enter surf=%s x=%.1f y=%.1f inWindow=%v lastLeave=%v\n",
			ptrSurfName(st.win, surface), x, y, st.inWindow, st.lastLeaveOurs)
	}
	st.win.lastSerial.Store(uint32(serial))
	ours := st.isOurs(surface)
	if ours && st.lastLeaveOurs {
		// Internal content↔chrome crossing: motion (translated coords), not
		// a window enter/leave pair.
		st.lastLeaveOurs = false
		st.surface = surface
		st.enterSerial = serial
		st.lastX, st.lastY = x, y
		hit := csdHit{}
		if c := st.win.csd; c != nil {
			hit = c.onHover(surface, x, y)
		}
		st.applyCursor(serial, hit)
		ax, ay := st.appXY(surface, x, y)
		st.lastAppX, st.lastAppY = ax, ay
		st.win.pushPtr(Event{Type: EventPointer, Pointer: PointerMove, X: ax, Y: ay})
		return
	}
	if !ours {
		// Entered another window: the preceding leave was a window leave.
		if st.inWindow {
			st.leaveWindow(serial)
		}
		st.lastLeaveOurs = false
		st.surface = 0
		return
	}
	// Entered the window from outside.
	st.inWindow = true
	st.lastLeaveOurs = false
	st.surface = surface
	st.enterSerial = serial
	st.lastX, st.lastY = x, y
	hit := csdHit{}
	if c := st.win.csd; c != nil {
		hit = c.onHover(surface, x, y)
	}
	st.applyCursor(serial, hit)
	ax, ay := st.appXY(surface, x, y)
	st.lastAppX, st.lastAppY = ax, ay
	st.win.pushPtr(Event{
		Type:    EventPointer,
		Pointer: PointerEnter,
		X:       ax,
		Y:       ay,
	})
}

//go:noinline // direct-Go-callers must observe callback writes (see leaveWindow).
func wlPtrLeaveCB(data, ptr, serial, surface uintptr) {
	st := ptrFrom(data)
	if st == nil || st.win == nil {
		return
	}
	if !st.isOurs(surface) {
		if ptrDbg {
			fmt.Fprintf(os.Stderr, "PTR leave surf=%s (foreign)\n", ptrSurfName(st.win, surface))
		}
		return // leave of a foreign surface — nothing to track
	}
	if ptrDbg {
		fmt.Fprintf(os.Stderr, "PTR leave surf=%s\n", ptrSurfName(st.win, surface))
	}
	// Defer the decision: an internal crossing is followed by enter(ours)
	// (consumed there); a window leave is resolved by the next enter
	// (foreign) or by poll() (resolveDeferredLeave).
	st.lastLeaveOurs = true
	st.lastLeaveSerial = serial
	st.surface = 0
	_ = ptr
}

// wlPtrMotionCB: motion(time, surface_x, surface_y). Surface-local logical px.
func wlPtrMotionCB(data, ptr, time, sx, sy uintptr) {
	st := ptrFrom(data)
	if st == nil || st.win == nil {
		return
	}
	st.lastX = wlFixedToDouble(sx)
	st.lastY = wlFixedToDouble(sy)
	if ptrDbg {
		fmt.Fprintf(os.Stderr, "PTR motion surf=%s x=%.1f y=%.1f\n", ptrSurfName(st.win, st.surface), st.lastX, st.lastY)
	}
	// Update hover state + resize cursor while moving (surface-local coords;
	// the CSD hit-test is per-surface).
	if c := st.win.csd; c != nil {
		hit := c.onHover(st.surface, st.lastX, st.lastY)
		c.setCursor(st.enterSerial, hit)
	}
	ax, ay := st.appXY(st.surface, st.lastX, st.lastY)
	st.lastAppX, st.lastAppY = ax, ay
	st.win.pushPtr(Event{
		Type:    EventPointer,
		Pointer: PointerMove,
		X:       ax,
		Y:       ay,
	})
}

// wlPtrButtonCB: button(serial, time, button, state). state 0=release, 1=press.
// wl_pointer buttons are evdev codes (0x110=BTN_LEFT, 0x111=BUTTON MIDDLE,
// 0x112=BTN_RIGHT, 0x13d/0x13e=BTN_SIDE/EXTRA).
func wlPtrButtonCB(data, ptr, serial, time, button, state uintptr) {
	st := ptrFrom(data)
	if st == nil || st.win == nil {
		return
	}
	st.win.lastSerial.Store(uint32(serial))
	// Pointer press = the user clicked the window/input box. Re-commit the
	// text-input state here (GTK released_cb pattern): the compositor only
	// feeds key events to the IME engine once focus_in ran, and focus_in
	// requires a commit AFTER the engine (IBus) is ready — the initial
	// enable+commit at window creation is dropped (surface not focused yet)
	// and may race engine startup. Every click re-submits, so the engine
	// activates reliably.
	if state&0xff == 1 {
		st.win.refreshTextInput()
	}
	btn := int(button)
	pressed := state&0xff == 1

	// Chrome (title bar / borders) owns every press — nothing reaches the
	// content layer: left = caption drag / buttons / resize grips, right =
	// caption window menu, middle = nothing (GTK4 event-separation model).
	if st.isChrome(st.surface) {
		if pressed {
			switch btn {
			case 0x110: // BTN_LEFT
				if c := st.win.csd; c != nil {
					c.onButtonPress(st.win.seat, serial, c.hitTest(st.surface, st.lastX, st.lastY))
				}
			case 0x112: // BTN_RIGHT
				if c := st.win.csd; c != nil {
					c.onRightPress(st.win.seat, serial, st.surface, st.lastX, st.lastY)
				}
			}
		} else if btn == 0x110 {
			if c := st.win.csd; c != nil {
				c.onButtonRelease(st.surface, st.lastX, st.lastY)
			}
		}
		return
	}

	// Content surface: the CSD only consumes invisible chrome actions
	// (nothing remains with a CSD title bar — content never resizes), so
	// presses/releases forward to the content layer.
	if c := st.win.csd; c != nil && pressed {
		hit := c.hitTest(st.surface, st.lastX, st.lastY)
		if hit.act != csdActNone {
			c.onButtonPress(st.win.seat, serial, hit)
			return
		}
	}
	if c := st.win.csd; c != nil && !pressed {
		c.onButtonRelease(st.surface, st.lastX, st.lastY)
	}

	// Map evdev button codes → 1/2/3 like platform convention (X11 button
	// numbers); side buttons map to 8/9 (GTK parity).
	switch btn {
	case 0x110: // BTN_LEFT
		btn = 1
	case 0x111: // BTN_MIDDLE
		btn = 2
	case 0x112: // BTN_RIGHT
		btn = 3
	case 0x13d: // BTN_SIDE
		btn = 8
	case 0x13e: // BTN_EXTRA
		btn = 9
	default:
		btn = 1
	}
	k := PointerDown
	if !pressed {
		k = PointerUp
	}
	ax, ay := st.appXY(st.surface, st.lastX, st.lastY)
	st.win.pushPtr(Event{
		Type:    EventPointer,
		Pointer: k,
		Button:  btn,
		X:       ax,
		Y:       ay,
	})
}

// wlPtrAxisCB: axis(time, axis, value_fixed). axis 0=vertical, 1=horizontal.
// We do not wait for frame; apply per-axis directly (sufficient for wheel).
func wlPtrAxisCB(data, ptr, time, axis, value uintptr) {
	st := ptrFrom(data)
	if st == nil || st.win == nil {
		return
	}
	v := wlFixedToDouble(value)
	ev := Event{Type: EventPointer, Pointer: PointerScroll}
	if axis == 0 {
		ev.ScrollY = v
	} else {
		ev.ScrollX = v
	}
	st.win.pushPtr(ev)
}

func wlPtrFrameCB(data, ptr uintptr)                        {}
func wlPtrAxisSourceCB(data, ptr, source uintptr)           {}
func wlPtrAxisStopCB(data, ptr, time, axis uintptr)         {}
func wlPtrAxisDiscreteCB(data, ptr, axis, discrete uintptr) {}

// pushPtr queues a pointer event for the next poll (thread-safe).
func (w *wlWin) pushPtr(ev Event) {
	if w == nil {
		return
	}
	w.ptrMu.Lock()
	w.ptrEvents = append(w.ptrEvents, ev)
	w.ptrMu.Unlock()
}
