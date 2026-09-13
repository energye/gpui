//go:build linux

package platform

import (
	"testing"
	"time"
	"unsafe"
)

// Modifier-only reporting (S6 §4.4 B 组): the Wayland keyboard derives the
// effective Shift/Control/Alt(Mod1)/Meta(Mod4) state from xkb after every
// key/modifiers event and emits EventModifiersChanged on change, leading
// the Key itself (mirrors X11). Plain typing stays quiet.

func TestWLTrackModsEmitsOnce(t *testing.T) {
	st := &wlKeyboardState{}
	if _, ok := st.wlTrackMods(false, false, false, false); ok {
		t.Fatal("initial empty must stay quiet")
	}
	mev, ok := st.wlTrackMods(true, false, false, false)
	if !ok || mev.Type != EventModifiersChanged || !mev.ModShift {
		t.Fatalf("shift change = %+v ok=%v", mev, ok)
	}
	if _, ok := st.wlTrackMods(true, false, false, false); ok {
		t.Fatal("repeat same state must stay quiet")
	}
	mev, ok = st.wlTrackMods(false, false, false, false)
	if !ok || mev.Type != EventModifiersChanged || mev.ModShift {
		t.Fatalf("shift release = %+v ok=%v", mev, ok)
	}
}

func TestWLTrackModsNilSafe(t *testing.T) {
	var st *wlKeyboardState
	if _, ok := st.wlTrackMods(true, false, false, false); ok {
		t.Fatal("nil state must stay quiet")
	}
}

func TestWLModsFromXKBNilSafe(t *testing.T) {
	if s, c, a, m := wlModsFromXKB(nil, 0x1234); s || c || a || m {
		t.Fatal("nil xkb must read released")
	}
	if s, c, a, m := wlModsFromXKB(&xkbFuncs{}, 0x1234); s || c || a || m {
		t.Fatal("missing binding must read released")
	}
	if s, c, a, m := wlModsFromXKB(loadXKB(), 0); s || c || a || m {
		t.Fatal("null xkb state must read released")
	}
}

func TestWLModifiersCBNilSafe(t *testing.T) {
	// Nil userdata or missing xkb degrades silently, never panics.
	wlKbModifiersCB(0, 0, 0, 0, 0, 0, 0)
	st := &wlKeyboardState{}
	wlKbModifiersCB(uintptr(unsafe.Pointer(st)), 0, 0, 0, 0, 0, 0)
	st.xkb = &xkbFuncs{}
	wlKbModifiersCB(uintptr(unsafe.Pointer(st)), 0, 0, 0, 0, 0, 0)
}

// evdev keycodes (linux/input-event-codes.h): Wayland key events carry
// these; the callback adds the xkb +8 offset internally.
const (
	evdevLeftShift = 42
	evdevKeyA      = 30
)

// TestWaylandModifiersChangedPump drives the real key path against the
// compositor's own keymap: a Shift press must queue ModifiersChanged ahead
// of its Key, plain 'a' stays quiet, release clears. Needs a compositor
// with a keyboard seat; otherwise Skip with reason (never silent green).
func TestWaylandModifiersChangedPump(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()
	w := win.Host().(*wlHost).win
	if w == nil {
		t.Skipf("no wayland window")
	}
	// The seat binds the keyboard after capabilities; the keymap arrives
	// async on the event thread. Wait for a usable keyboard state.
	var st *wlKeyboardState
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st = w.kbd
		if st != nil && st.state != 0 && st.xkb != nil && st.xkb.modNameIsActive != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
		_ = win.Host().WaitEvents(50 * time.Millisecond)
	}
	if st == nil {
		t.Skipf("wayland modifiers test skipped: compositor gave no keyboard seat")
	}
	if st.state == 0 || st.xkb == nil || st.xkb.modNameIsActive == nil {
		t.Skipf("wayland modifiers test skipped: no keymap/modifier query from compositor")
	}
	st.modMu.Lock()
	st.modShift, st.modControl, st.modAlt, st.modMeta = false, false, false, false
	st.modMu.Unlock()
	drainKeys(w)

	// Shift press: modifier leads its key.
	wlKbKeyCB(st.selfPtr, 0, 0x8000, 0, evdevLeftShift, 1)
	got := collectKeys(w, 2*time.Second, func(ev []Event) bool {
		return hasKeyType(ev, EventModifiersChanged) != nil
	})
	mev := hasKeyType(got, EventModifiersChanged)
	if mev == nil {
		t.Fatalf("shift press produced no ModifiersChanged; saw %v", keyEvTypes(got))
	}
	if !mev.ModShift {
		t.Fatalf("modifiers = %+v, want shift", *mev)
	}
	if firstKeyIdx(got) < modifierIdx(got) {
		t.Fatalf("ModifiersChanged must lead its Key; saw %v", keyEvTypes(got))
	}
	st.cancelRepeat()

	// Plain 'a' with Shift still held: state unchanged, stays quiet.
	drainKeys(w)
	wlKbKeyCB(st.selfPtr, 0, 0x8001, 0, evdevKeyA, 1)
	wlKbKeyCB(st.selfPtr, 0, 0x8002, 0, evdevKeyA, 0)
	st.cancelRepeat()
	soak := collectKeys(w, 400*time.Millisecond, nil)
	for _, e := range soak {
		if e.Type == EventModifiersChanged {
			t.Fatalf("steady state emitted %+v", e)
		}
	}

	// Shift release clears.
	drainKeys(w)
	wlKbKeyCB(st.selfPtr, 0, 0x8003, 0, evdevLeftShift, 0)
	got = collectKeys(w, 2*time.Second, func(ev []Event) bool {
		for _, e := range ev {
			if e.Type == EventModifiersChanged && !e.ModShift {
				return true
			}
		}
		return false
	})
	found := false
	for _, e := range got {
		if e.Type == EventModifiersChanged && !e.ModShift {
			found = true
		}
	}
	if !found {
		t.Fatalf("shift release produced no empty ModifiersChanged; saw %v", keyEvTypes(got))
	}
	st.cancelRepeat()
}

func drainKeys(w *wlWin) {
	w.keyMu.Lock()
	w.keyEvents = nil
	w.keyMu.Unlock()
}

func collectKeys(w *wlWin, timeout time.Duration, until func([]Event) bool) []Event {
	deadline := time.Now().Add(timeout)
	var out []Event
	for time.Now().Before(deadline) {
		w.keyMu.Lock()
		if len(w.keyEvents) > 0 {
			out = append(out, w.keyEvents...)
			w.keyEvents = nil
		}
		w.keyMu.Unlock()
		if until != nil && until(out) {
			return out
		}
		if until == nil {
			// Fixed soak: keep draining for the full window.
			time.Sleep(50 * time.Millisecond)
			continue
		}
		time.Sleep(10 * time.Millisecond)
	}
	return out
}

func hasKeyType(evs []Event, typ EventType) *Event {
	for i := range evs {
		if evs[i].Type == typ {
			return &evs[i]
		}
	}
	return nil
}

func keyEvTypes(evs []Event) []string {
	out := make([]string, 0, len(evs))
	for _, e := range evs {
		out = append(out, e.Type.String())
	}
	return out
}

func modifierIdx(evs []Event) int {
	for i, e := range evs {
		if e.Type == EventModifiersChanged {
			return i
		}
	}
	return len(evs)
}

func firstKeyIdx(evs []Event) int {
	for i, e := range evs {
		if e.Type == EventKey {
			return i
		}
	}
	return len(evs)
}
