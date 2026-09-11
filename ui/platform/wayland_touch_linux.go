//go:build linux

package platform

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// wl_touch binding — multi-touch slot input for P0 touch产出.
//
// wl_touch_interface has 7 events (v1-v3); the listener array MUST match
// event_count exactly or proxyAddListener reads out of bounds → wild pointer
// callbacks → SIGSEGV (same discipline as wl_pointer, see bindPointer).
// Binding is capabilities-gated (seatCapTouch) like keyboard/pointer: no
// touch device → never bound, zero behavior change.

// wl_touch event opcodes.
const (
	wlTouchDown        = 0
	wlTouchUp          = 1
	wlTouchMotion      = 2
	wlTouchCancel      = 3
	wlTouchShape       = 4
	wlTouchOrientation = 5
	wlTouchFrame       = 6
)

// wlTouchPoint tracks one live touch: down/motion carry surface + position,
// up/cancel carry neither, so the last app-space position is stamped onto
// the release (same pattern as the pointer leave stamp).
type wlTouchPoint struct {
	surface uintptr
	x, y    float64
}

type wlTouchState struct {
	lib   *wlLib
	win   *wlWin
	touch uintptr // wl_touch proxy

	listener [7]uintptr
	selfPtr  uintptr
	// points tracks live touches by compositor id.
	points map[uint32]wlTouchPoint
}

// ownsSurface reports whether the surface belongs to this window (content or
// CSD chrome). Shared by pointer and touch tracking.
func (w *wlWin) ownsSurface(surface uintptr) bool {
	if w == nil || surface == 0 {
		return false
	}
	if surface == w.surface {
		return true
	}
	if c := w.csd; c != nil {
		return surface == c.topSurface || surface == c.left.surf ||
			surface == c.right.surf || surface == c.bottom.surf
	}
	return false
}

// bindTouch creates a wl_touch from the seat and adds the listener.
// Returns nil when unavailable (silent degrade).
func (w *wlWin) bindTouch() *wlTouchState {
	if w == nil || w.lib == nil || w.seat == 0 || w.lib.ifaceTouch == 0 {
		return nil
	}
	st := &wlTouchState{lib: w.lib, win: w, points: make(map[uint32]wlTouchPoint)}
	st.selfPtr = uintptr(unsafe.Pointer(st))
	// seat.get_touch(new_id wl_touch) — opcode 2 on wl_seat.
	args := []wlArg{argNewID()}
	st.touch = w.lib.proxyMarshalArrayCtor(w.seat, wlSeatGetTouch, &args[0], w.lib.ifaceTouch, 1)
	if st.touch == 0 {
		return nil
	}
	st.listener[wlTouchDown] = purego.NewCallback(wlTouchDownCB)
	st.listener[wlTouchUp] = purego.NewCallback(wlTouchUpCB)
	st.listener[wlTouchMotion] = purego.NewCallback(wlTouchMotionCB)
	st.listener[wlTouchCancel] = purego.NewCallback(wlTouchCancelCB)
	st.listener[wlTouchShape] = purego.NewCallback(wlTouchShapeCB)
	st.listener[wlTouchOrientation] = purego.NewCallback(wlTouchOrientationCB)
	st.listener[wlTouchFrame] = purego.NewCallback(wlTouchFrameCB)
	if w.lib.proxyAddListener(st.touch, uintptr(unsafe.Pointer(&st.listener[0])), st.selfPtr) != 0 {
		w.lib.proxyDestroy(st.touch)
		return nil
	}
	return st
}

func (st *wlTouchState) destroy() {
	if st == nil {
		return
	}
	if st.touch != 0 && st.lib != nil {
		st.lib.proxyDestroy(st.touch)
		st.touch = 0
	}
}

func touchFrom(data uintptr) *wlTouchState {
	if data == 0 {
		return nil
	}
	return (*wlTouchState)(unsafe.Pointer(data))
}

// touchXY translates surface-local coords to content space (identity without
// CSD — mirrors wlPointerState.appXY).
func (st *wlTouchState) touchXY(surface uintptr, x, y float64) (float64, float64) {
	if st == nil || st.win == nil || st.win.csd == nil {
		return x, y
	}
	return st.win.csd.appCoords(surface, x, y)
}

// wlTouchDownCB: down(serial, time, surface, id, x, y). Surface-local logical px.
func wlTouchDownCB(data, touch, serial, time, surface, id, sx, sy uintptr) {
	st := touchFrom(data)
	if st == nil || st.win == nil {
		return
	}
	if !st.win.ownsSurface(surface) {
		return // foreign surface — nothing to track
	}
	x, y := st.touchXY(surface, wlFixedToDouble(sx), wlFixedToDouble(sy))
	st.points[uint32(id)] = wlTouchPoint{surface: surface, x: x, y: y}
	st.win.pushPtr(Event{Type: EventTouch, Pointer: PointerDown, TouchID: int(uint32(id)) + 1, X: x, Y: y})
}

// wlTouchMotionCB: motion(time, id, x, y). Surface comes from the tracked down.
func wlTouchMotionCB(data, touch, time, id, sx, sy uintptr) {
	st := touchFrom(data)
	if st == nil || st.win == nil {
		return
	}
	pt, ok := st.points[uint32(id)]
	if !ok {
		return // motion without down — protocol violation, ignore
	}
	x, y := st.touchXY(pt.surface, wlFixedToDouble(sx), wlFixedToDouble(sy))
	pt.x, pt.y = x, y
	st.points[uint32(id)] = pt
	st.win.pushPtr(Event{Type: EventTouch, Pointer: PointerMove, TouchID: int(uint32(id)) + 1, X: x, Y: y})
}

// wlTouchUpCB: up(serial, time, id). No position — stamp the last one.
func wlTouchUpCB(data, touch, serial, time, id uintptr) {
	st := touchFrom(data)
	if st == nil || st.win == nil {
		return
	}
	pt, ok := st.points[uint32(id)]
	if !ok {
		return
	}
	delete(st.points, uint32(id))
	st.win.pushPtr(Event{Type: EventTouch, Pointer: PointerUp, TouchID: int(uint32(id)) + 1, X: pt.x, Y: pt.y})
}

// wlTouchCancelCB: cancel(id). A system-aborted touch — never an Up.
func wlTouchCancelCB(data, touch, id uintptr) {
	st := touchFrom(data)
	if st == nil || st.win == nil {
		return
	}
	pt, ok := st.points[uint32(id)]
	if !ok {
		return
	}
	delete(st.points, uint32(id))
	st.win.pushPtr(Event{Type: EventTouch, Pointer: PointerCancel, TouchID: int(uint32(id)) + 1, X: pt.x, Y: pt.y})
}

func wlTouchShapeCB(data, touch, id, major, minor uintptr)      {}
func wlTouchOrientationCB(data, touch, id, orientation uintptr) {}
func wlTouchFrameCB(data, touch uintptr)                        {}
