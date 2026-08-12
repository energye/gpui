//go:build linux

package platform

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// wl_pointer binding via purego. Standard Wayland client input: the seat's
// pointer provides enter/leave/motion/button/axis events that drive mouse
// input (hit-test, gestures, clicks). No xkb dependency — button/axis map
// directly; coordinates are surface-local (buf.scale handling is a later
// refinement; GNOME Wayland scale 1 for now).
//
// Protocol (wl_pointer, stable v1 — event subset v1-v5 is enough):
//
//	requests: release(0), set_cursor(1)[serial,surface,hotspot,h,v], ...
//	events:   enter(0)[serial,surface,pos_fixed,surface_x_fixed,surface_y_fixed]
//	          leave(1)[serial,surface]
//	          motion(2)[time,pos_fixed,surface_x_fixed,surface_y_fixed]
//	          button(3)[serial,time,button,state]
//	          axis(4)[time,axis,value_fixed]
//	          frame(5) release(6) axis_source(7)...
//
// enter/motion coords are in surface-local coordinate space *before* the
// surface's scale is applied (wl_fixed). We treat them as logical px.

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
	// enterSerial: serial of the last pointer.enter (needed for set_cursor).
	enterSerial uintptr
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

func wlPtrEnterCB(data, ptr, serial, surface, sx, sy uintptr) {
	st := ptrFrom(data)
	if st == nil || st.win == nil {
		return
	}
	st.surface = surface
	st.enterSerial = serial
	// enter carries surface-local coords — record them so a press right after
	// enter (without any motion) hit-tests correctly (button has no coords).
	st.lastX = wlFixedToDouble(sx)
	st.lastY = wlFixedToDouble(sy)
	// Update hover state + cursor for the new region.
	if c := st.win.csd; c != nil {
		hit := c.onHover(st.surface, st.lastX, st.lastY)
		c.setCursor(serial, hit)
	}
	// Report enter to the upper layer (hover decision) alongside CSD use.
	st.win.pushPtr(Event{
		Type:    EventPointer,
		Pointer: PointerEnter,
		X:       st.lastX,
		Y:       st.lastY,
	})
}

func wlPtrLeaveCB(data, ptr, serial, surface uintptr) {
	st := ptrFrom(data)
	if st == nil || st.win == nil {
		return
	}
	// Leaving the window clears hover highlight + restores default cursor.
	if c := st.win.csd; c != nil {
		c.onHover(0, 0, 0)
		c.setCursor(serial, csdHit{})
	}
	st.surface = 0
	// Report leave to the upper layer (hover decision) alongside CSD use.
	st.win.pushPtr(Event{Type: EventPointer, Pointer: PointerLeave})
}

// wlPtrMotionCB: motion(time, surface_x, surface_y). Surface-local logical px.
func wlPtrMotionCB(data, ptr, time, sx, sy uintptr) {
	st := ptrFrom(data)
	if st == nil || st.win == nil {
		return
	}
	st.lastX = wlFixedToDouble(sx)
	st.lastY = wlFixedToDouble(sy)
	// Update hover state + resize cursor while moving.
	if c := st.win.csd; c != nil {
		hit := c.onHover(st.surface, st.lastX, st.lastY)
		c.setCursor(st.enterSerial, hit)
	}
	st.win.pushPtr(Event{
		Type:    EventPointer,
		Pointer: PointerMove,
		X:       wlFixedToDouble(sx),
		Y:       wlFixedToDouble(sy),
	})
}

// wlPtrButtonCB: button(serial, time, button, state). state 0=release, 1=press.
// wl_pointer buttons are evdev codes (0x110=BTN_LEFT, 0x111=MI DDLE, 0x112=RIGHT).
func wlPtrButtonCB(data, ptr, serial, time, button, state uintptr) {
	st := ptrFrom(data)
	if st == nil || st.win == nil {
		return
	}
	// Pointer press = the user clicked the window/input box. Re-commit the
	// text-input state here (GTK released_cb pattern): the compositor only
	// feeds key events to the IME engine once focus_in ran, and focus_in
	// requires a commit AFTER the engine (IBus) is ready — the initial
	// enable+commit at window creation is dropped (surface not focused yet)
	// and may race engine startup. Every click re-submits, so the engine
	// activates reliably.
	if state&0xff == 1 {
		st.win.refreshTextInput()
		// CSD chrome interaction (left button only); if consumed, skip the
		// normal pointer event (the chrome owns the press).
		if btn := int(button); btn == 0x110 { // BTN_LEFT
			if c := st.win.csd; c != nil && c.onButtonPress(st.win.seat, serial, c.hitTest(st.surface, st.lastX, st.lastY)) {
				return
			}
		}
	} else {
		if c := st.win.csd; c != nil {
			c.onButtonRelease()
		}
	}
	btn := int(button)
	// Map evdev button codes → 1/2/3 like platform convention.
	switch btn {
	case 0x110: // BTN_LEFT
		btn = 1
	case 0x111: // BTN_MIDDLE
		btn = 2
	case 0x112: // BTN_RIGHT
		btn = 3
	case 0x13d: // BTN_SIDE (thumb) → treat as middle-ish
		btn = 2
	default:
		btn = 1
	}
	k := PointerDown
	if state == 0 {
		k = PointerUp
	}
	st.win.pushPtr(Event{
		Type:    EventPointer,
		Pointer: k,
		Button:  btn,
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
