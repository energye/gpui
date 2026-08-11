//go:build linux

package platform

import "testing"

// TestTextInputInterfaceTable verifies the in-process zwp_text_input_v3
// interface tables have the expected method/event counts and opcodes.
func TestTextInputInterfaceTable(t *testing.T) {
	initTIInterfaces(0x1234, 0x5678) // ifaceSurface/ifaceSeat placeholders

	if ifaceTiMgr.MethodCount != 2 || ifaceTiMgr.EventCount != 0 {
		t.Fatalf("tiMgr counts = %d/%d", ifaceTiMgr.MethodCount, ifaceTiMgr.EventCount)
	}
	if ifaceTi.MethodCount != 8 || ifaceTi.EventCount != 6 {
		t.Fatalf("ti counts = %d/%d", ifaceTi.MethodCount, ifaceTi.EventCount)
	}
	// Opcode 0 = destroy; opcode 1 = get_text_input (protocol order).
	if name := goString(msgTiMgr[0].Name); name != "destroy" {
		t.Fatalf("tiMgr[0] = %q", name)
	}
	if name := goString(msgTiMgr[1].Name); name != "get_text_input" {
		t.Fatalf("tiMgr[1] = %q", name)
	}
	if typesTiMgr[1] != 0x5678 {
		t.Fatalf("get_text_input seat type = %x, want ifaceSeat", typesTiMgr[1])
	}
	if name := goString(msgTi[tiEnable].Name); name != "enable" {
		t.Fatalf("ti[enable] = %q", name)
	}
	if name := goString(msgTiEv[tiEvPreeditString].Name); name != "preedit_string" {
		t.Fatalf("tiEv[preedit] = %q", name)
	}
	if name := goString(msgTiEv[tiEvCommitString].Name); name != "commit_string" {
		t.Fatalf("tiEv[commit] = %q", name)
	}
}

// TestPushIMEDrain verifies the pending IME queue survives push → drain.
func TestPushIMEDrain(t *testing.T) {
	w := &wlWin{}
	w.pushIME(Event{Type: EventIME, IMEKind: 1, IMEText: "a"})
	w.pushIME(Event{Type: EventIME, IMEKind: 0, IMEText: "ni"})

	w.imeMu.Lock()
	n := len(w.imeEvents)
	w.imeMu.Unlock()
	if n != 2 {
		t.Fatalf("queued = %d, want 2", n)
	}

	// Drain via the poll path fields (poll itself needs a live display, so
	// mirror its drain logic here).
	w.imeMu.Lock()
	evs := append([]Event(nil), w.imeEvents...)
	w.imeEvents = nil
	w.imeMu.Unlock()
	if len(evs) != 2 || evs[0].IMEText != "a" || evs[1].IMEText != "ni" {
		t.Fatalf("drained = %+v", evs)
	}
}

// TestWlImeNilSafety verifies the IME capability degrades safely.
func TestWlImeNilSafety(t *testing.T) {
	var im *wlIme
	im.EnableIME(Rect{})
	im.SetComposing("", 0)
	im.Commit("")
	im.DisableIME() // must not panic

	// Host with no text-input state → nil capability.
	h := &wlHost{win: &wlWin{}}
	if got := imeFor(h); got != nil {
		t.Fatalf("host without ti should have nil IME, got %T", got)
	}
}
