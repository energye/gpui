//go:build linux

package platform

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// wl_seat binding — standard Wayland client input bootstrap.
//
// Correct order per the Wayland protocol:
//  1. registry.bind(wl_seat)
//  2. add capabilities listener (the compositor immediately emits
//     capabilities, indicating which input devices exist)
//  3. ONLY after capabilities arrives, request get_keyboard / get_pointer
//
// Requesting keyboard/pointer before capabilities is tolerated by most
// compositors but is not the documented behavior; doing it eagerly can break
// compositors whose input JS layer assumes capabilities was delivered first
// (observed: GNOME 42.9 gjs crash during the request dispatch).
//
// This binding therefore:
//   - binds the seat,
//   - installs the capabilities/name listener
//   - defers keyboard/pointer creation until capabilities is received

// wl_seat event opcodes.
const (
	wlSeatEvCapabilities = 0
	wlSeatEvName         = 1
)

// seatCapMask bits (compositor capabilities).
const (
	seatCapPointer  = 1
	seatCapKeyboard = 2
	seatCapTouch    = 4
)

// wlSeatState holds the bound seat + whether we have seen capabilities yet.
type wlSeatState struct {
	lib       *wlLib
	win       *wlWin
	seat      uintptr
	listener  [2]uintptr
	selfPtr   uintptr
	capsSeen  bool
	capsMask  uint32
	// pending device creation (called once after capabilities arrives)
	pendingKeys  bool
	pendingPtrs  bool
	pendingTI    bool // zwp_text_input_v3 also needs the bound seat
	pendingDD    bool // wl_data_device (clipboard + DnD) needs the bound seat
}

// bindSeat binds wl_seat and installs the capabilities/name listener.
// Returns nil when the seat is unavailable (silent degrade).
func (w *wlWin) bindSeat() *wlSeatState {
	if w == nil || w.lib == nil || w.seatName == 0 || w.lib.ifaceSeat == 0 {
		return nil
	}
	st := &wlSeatState{lib: w.lib, win: w}
	st.selfPtr = uintptr(unsafe.Pointer(st))
	st.seat = w.bind(w.registry, w.seatName, w.lib.ifaceSeat, 1)
	if st.seat == 0 {
		return nil
	}
	st.listener[wlSeatEvCapabilities] = purego.NewCallback(wlSeatCapCB)
	st.listener[wlSeatEvName] = purego.NewCallback(wlSeatNameCB)
	if w.lib.proxyAddListener(st.seat, uintptr(unsafe.Pointer(&st.listener[0])), st.selfPtr) != 0 {
		w.lib.proxyDestroy(st.seat)
		return nil
	}
	return st
}

func (st *wlSeatState) destroy() {
	if st == nil {
		return
	}
	if st.seat != 0 && st.lib != nil {
		st.lib.proxyDestroy(st.seat)
		st.seat = 0
	}
}

func seatFrom(data uintptr) *wlSeatState {
	if data == 0 {
		return nil
	}
	return (*wlSeatState)(unsafe.Pointer(data))
}

// wlSeatCapCB: capabilities(uint32). After this arrives we know which input
// devices exist and may request them.
func wlSeatCapCB(data, seat, caps uintptr) {
	st := seatFrom(data)
	if st == nil || st.win == nil {
		return
	}
	st.capsSeen = true
	st.capsMask = uint32(caps)
	// Now the protocol allows creating keyboard/pointer.
	wlSeatFlushPending(st)
}

// wlSeatNameCB: name(string) — v2+; informational only.
func wlSeatNameCB(data, seat, name uintptr) {}

// wlSeatFlushPending creates pending input devices now that capabilities
// arrived. Called from the dispatch callback (single-threaded with dispatch).
// Every seat-derived object is requested here — never before capabilities.
func wlSeatFlushPending(st *wlSeatState) {
	if st == nil || st.win == nil || !st.capsSeen {
		return
	}
	w := st.win
	if st.pendingKeys && w.kbd == nil {
		if st.capsMask&seatCapKeyboard != 0 && st.lib.ifaceKeyboard != 0 {
			w.kbd = w.bindKeyboard()
		}
		st.pendingKeys = false
	}
	if st.pendingPtrs && w.ptr == nil {
		if st.capsMask&seatCapPointer != 0 && st.lib.ifacePointer != 0 {
			w.ptr = w.bindPointer()
		}
		st.pendingPtrs = false
	}
	if st.pendingTI && w.ti == nil {
		if w.tiMgrName != 0 && st.lib.ifaceSeat != 0 {
			initTIInterfaces(st.lib.ifaceSurface, st.lib.ifaceSeat)
			w.ti = w.bindTextInput()
		}
		st.pendingTI = false
	}
	if st.pendingDD && w.dds == nil {
		if w.ddMgrName != 0 && st.lib.ifaceDataDevMgr != 0 {
			w.dds = w.bindDataDevice()
		}
		st.pendingDD = false
	}
	// Wake the loop so poll drains any queued events promptly.
	if h := w.hostForWake(); h != nil {
		h.WakeUp()
	}
}