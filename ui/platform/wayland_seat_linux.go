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
	lib      *wlLib
	win      *wlWin
	seat     uintptr
	listener [2]uintptr
	selfPtr  uintptr
	capsSeen bool
	capsMask uint32
	// seatName is the compositor seat name (wl_seat.name, usually "seat0").
	// Used as Event.DeviceName: the protocol reports capability bits only,
	// never per-device names.
	seatName string
	// pending device creation (called once after capabilities arrives)
	pendingKeys  bool
	pendingPtrs  bool
	pendingTouch bool // wl_touch, gated on the seat touch capability bit
	pendingTI    bool // zwp_text_input_v3 also needs the bound seat
	pendingDD    bool // wl_data_device (clipboard + DnD) needs the bound seat
	// want* remembers the opt-in after pending* clears, so a capability
	// appearing later (hot-plug) still binds its object.
	wantKeys  bool
	wantPtrs  bool
	wantTouch bool
}

// bindSeat binds wl_seat and installs the capabilities/name listener.
// Returns nil when the seat is unavailable (silent degrade).
func (w *wlWin) bindSeat() *wlSeatState {
	if w == nil || w.lib == nil || w.seatName == 0 || w.lib.ifaceSeat == 0 {
		return nil
	}
	st := &wlSeatState{lib: w.lib, win: w}
	st.selfPtr = uintptr(unsafe.Pointer(st))
	// Bind v2 when advertised to receive wl_seat.name (used as
	// Event.DeviceName); v1 has capabilities only. Higher versions add no
	// new seat events we listen to, so v2 is sufficient and safest.
	var ver uint32 = 1
	if w.seatVer >= 2 {
		ver = 2
	}
	st.seat = w.bind(w.registry, w.seatName, w.lib.ifaceSeat, ver)
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
// devices exist and may request them. The first packet only seeds the
// baseline (silent, no DeviceAdded): reporting it would spam 2-3 Added on
// every window open. Later packets diff against the baseline and report
// Added/Removed per changed capability bit.
func wlSeatCapCB(data, seat, caps uintptr) {
	st := seatFrom(data)
	if st == nil || st.win == nil {
		return
	}
	newCaps := uint32(caps)
	if !st.capsSeen {
		st.capsSeen = true
		st.capsMask = newCaps
		// Now the protocol allows creating keyboard/pointer.
		wlSeatFlushPending(st)
		return
	}
	old := st.capsMask
	if newCaps == old {
		return
	}
	st.capsMask = newCaps
	w := st.win
	for _, ev := range seatCapsToEvents(old, newCaps, st.seatName) {
		w.pushDev(ev)
	}
	// Late bind: a capability appearing after open still gets its object
	// (initial pending* already cleared). Removal keeps the stale proxy
	// (harmless, no events) to avoid destroy races on the event thread.
	if w.kbd == nil && st.wantKeys && newCaps&seatCapKeyboard != 0 && old&seatCapKeyboard == 0 {
		if st.lib != nil && st.lib.ifaceKeyboard != 0 {
			w.kbd = w.bindKeyboard()
		}
	}
	if w.ptr == nil && st.wantPtrs && newCaps&seatCapPointer != 0 && old&seatCapPointer == 0 {
		if st.lib != nil && st.lib.ifacePointer != 0 {
			w.ptr = w.bindPointer()
		}
	}
	if w.touch == nil && st.wantTouch && newCaps&seatCapTouch != 0 && old&seatCapTouch == 0 {
		if st.lib != nil && st.lib.ifaceTouch != 0 {
			w.touch = w.bindTouch()
		}
	}
	if h := w.hostForWake(); h != nil {
		h.WakeUp()
	}
}

// seatCapsToEvents diffs two seat capability masks into platform device
// events. Pure (no X/Wayland calls) so it is unit-testable. Order is fixed
// keyboard/mouse/touch for deterministic pumps.
func seatCapsToEvents(oldCaps, newCaps uint32, seatName string) []Event {
	var out []Event
	gained := newCaps &^ oldCaps
	lost := oldCaps &^ newCaps
	if gained&seatCapKeyboard != 0 {
		out = append(out, Event{Type: EventDeviceAdded, DeviceClass: DeviceKeyboard, DeviceName: seatName})
	}
	if gained&seatCapPointer != 0 {
		out = append(out, Event{Type: EventDeviceAdded, DeviceClass: DeviceMouse, DeviceName: seatName})
	}
	if gained&seatCapTouch != 0 {
		out = append(out, Event{Type: EventDeviceAdded, DeviceClass: DeviceTouch, DeviceName: seatName})
	}
	if lost&seatCapKeyboard != 0 {
		out = append(out, Event{Type: EventDeviceRemoved, DeviceClass: DeviceKeyboard, DeviceName: seatName})
	}
	if lost&seatCapPointer != 0 {
		out = append(out, Event{Type: EventDeviceRemoved, DeviceClass: DeviceMouse, DeviceName: seatName})
	}
	if lost&seatCapTouch != 0 {
		out = append(out, Event{Type: EventDeviceRemoved, DeviceClass: DeviceTouch, DeviceName: seatName})
	}
	return out
}

// wlSeatNameCB: name(string) — seat name (usually "seat0"), used as
// Event.DeviceName for hot-plug reports.
func wlSeatNameCB(data, seat, name uintptr) {
	st := seatFrom(data)
	if st == nil {
		return
	}
	st.seatName = goString(name)
}

// pushDev queues a seat hot-plug event for the next poll.
func (w *wlWin) pushDev(ev Event) {
	if w == nil {
		return
	}
	w.devMu.Lock()
	w.devEvents = append(w.devEvents, ev)
	w.devMu.Unlock()
}

// Device hot-plug (S6-P1 H 组 Wayland 侧): seat capabilities diff reports
// EventDeviceAdded/Removed (键/鼠/触; 笔走 tablet 协议二期, 见
// wayland_tablet_linux.go).

// wlSeatFlushPending creates pending input devices now that capabilities
// arrived. Called from the dispatch callback (single-threaded with dispatch).
// Every seat-derived object is requested here — never before capabilities.
func wlSeatFlushPending(st *wlSeatState) {
	if st == nil || st.win == nil || !st.capsSeen || st.lib == nil {
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
	if st.pendingTouch && w.touch == nil {
		if st.capsMask&seatCapTouch != 0 && st.lib.ifaceTouch != 0 {
			w.touch = w.bindTouch()
		}
		st.pendingTouch = false
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
