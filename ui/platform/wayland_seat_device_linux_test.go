//go:build linux

package platform

import (
	"testing"
	"time"
	"unsafe"
)

func TestSeatCapsDiffAdded(t *testing.T) {
	evs := seatCapsToEvents(0, seatCapKeyboard|seatCapPointer, "seat0")
	if len(evs) != 2 {
		t.Fatalf("gained kbd+ptr = %+v, want 2", evs)
	}
	if evs[0].Type != EventDeviceAdded || evs[0].DeviceClass != DeviceKeyboard || evs[0].DeviceName != "seat0" {
		t.Fatalf("ev0 = %+v", evs[0])
	}
	if evs[1].Type != EventDeviceAdded || evs[1].DeviceClass != DeviceMouse {
		t.Fatalf("ev1 = %+v", evs[1])
	}
}

func TestSeatCapsDiffRemoved(t *testing.T) {
	old := uint32(seatCapKeyboard | seatCapPointer | seatCapTouch)
	evs := seatCapsToEvents(old, seatCapKeyboard|seatCapPointer, "seat0")
	if len(evs) != 1 || evs[0].Type != EventDeviceRemoved || evs[0].DeviceClass != DeviceTouch {
		t.Fatalf("touch lost = %+v", evs)
	}
}

func TestSeatCapsDiffMixed(t *testing.T) {
	evs := seatCapsToEvents(seatCapTouch, seatCapKeyboard, "seat0")
	if len(evs) != 2 {
		t.Fatalf("mixed = %+v, want 2", evs)
	}
	if evs[0].Type != EventDeviceAdded || evs[0].DeviceClass != DeviceKeyboard {
		t.Fatalf("ev0 = %+v", evs[0])
	}
	if evs[1].Type != EventDeviceRemoved || evs[1].DeviceClass != DeviceTouch {
		t.Fatalf("ev1 = %+v", evs[1])
	}
}

func TestSeatCapsDiffQuiet(t *testing.T) {
	if evs := seatCapsToEvents(seatCapKeyboard, seatCapKeyboard, "seat0"); len(evs) != 0 {
		t.Fatalf("same caps must stay quiet: %+v", evs)
	}
	if evs := seatCapsToEvents(0, 0, "seat0"); len(evs) != 0 {
		t.Fatalf("empty caps must stay quiet: %+v", evs)
	}
}

func TestSeatCapCBFirstSilent(t *testing.T) {
	w := &wlWin{}
	st := &wlSeatState{lib: &wlLib{}, win: w, seatName: "seat0"}
	st.selfPtr = uintptr(unsafe.Pointer(st))
	wlSeatCapCB(st.selfPtr, 0, uintptr(seatCapKeyboard|seatCapPointer))
	if !st.capsSeen {
		t.Fatal("first caps must set capsSeen")
	}
	w.devMu.Lock()
	n := len(w.devEvents)
	w.devMu.Unlock()
	if n != 0 {
		t.Fatalf("first caps must stay silent, got %d events", n)
	}
	// Second identical caps stays quiet.
	wlSeatCapCB(st.selfPtr, 0, uintptr(seatCapKeyboard|seatCapPointer))
	w.devMu.Lock()
	n = len(w.devEvents)
	w.devMu.Unlock()
	if n != 0 {
		t.Fatalf("duplicate caps must stay quiet, got %d", n)
	}
	// Gaining touch reports one Added with the seat name.
	wlSeatCapCB(st.selfPtr, 0, uintptr(seatCapKeyboard|seatCapPointer|seatCapTouch))
	w.devMu.Lock()
	defer w.devMu.Unlock()
	if len(w.devEvents) != 1 {
		t.Fatalf("touch gain = %+v, want 1", w.devEvents)
	}
	ev := w.devEvents[0]
	if ev.Type != EventDeviceAdded || ev.DeviceClass != DeviceTouch || ev.DeviceName != "seat0" {
		t.Fatalf("touch gain = %+v", ev)
	}
}

func TestSeatCapCBNilSafe(t *testing.T) {
	wlSeatCapCB(0, 0, 1)
	wlSeatNameCB(0, 0, 0)
	var st *wlSeatState
	_ = st
	w := &wlWin{}
	w.pushDev(Event{Type: EventDeviceAdded})
	w.devMu.Lock()
	n := len(w.devEvents)
	w.devMu.Unlock()
	if n != 1 {
		t.Fatalf("pushDev queued = %d, want 1", n)
	}
}

func TestWaylandSeatSteadyQuiet(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()
	h := win.Host()
	w := h.(*wlHost).win
	if w == nil || w.seatState == nil {
		t.Skipf("no wayland seat on this compositor")
	}
	if !w.seatState.capsSeen {
		t.Fatalf("seat caps never arrived")
	}
	soak := h.WaitEvents(400 * time.Millisecond)
	for _, e := range soak {
		if e.Type == EventDeviceAdded || e.Type == EventDeviceRemoved {
			t.Fatalf("spurious device event without hot-plug: %+v", e)
		}
	}
	t.Logf("seat caps=0x%x name=%q (live plug needs hardware)", w.seatState.capsMask, w.seatState.seatName)
}
