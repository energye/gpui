//go:build linux

package platform

import "testing"

// TestXIMSymbolsResolve verifies the XIM entry points resolve from libX11.
// This is environment-dependent: skip (not fail) when X11 libs are absent.
func TestXIMSymbolsResolve(t *testing.T) {
	f := loadXIMFuncs()
	if f == nil {
		t.Skip("libX11 not available")
	}
	if f.xOpenIM == nil || f.xCreateIC == nil || f.xFilterEvent == nil || f.xutf8Lookup == nil {
		t.Fatalf("XIM funcs incomplete: %+v", f)
	}
}

// TestXimNamesAlive verifies the attribute-name C strings are NUL-terminated.
func TestXimNamesAlive(t *testing.T) {
	if string(ximNames.inputStyle) != "inputStyle\x00" {
		t.Fatalf("inputStyle = %q", ximNames.inputStyle)
	}
	if string(ximNames.clientWindow) != "clientWindow\x00" {
		t.Fatalf("clientWindow = %q", ximNames.clientWindow)
	}
}

// TestXimNilSafety verifies the XIM adapter degrades safely.
func TestXimNilSafety(t *testing.T) {
	var x *ximState
	x.close()
	x.setFocus(loadXIMFuncs())
	x.unsetFocus(loadXIMFuncs())
	if handled, committed := x.filter(loadXIMFuncs(), nil, 0); handled || committed != "" {
		t.Fatalf("nil xim filter = %v %q", handled, committed)
	}

	var im *x11Ime
	im.EnableIME(Rect{})
	im.SetComposing("", 0)
	im.Commit("")
	im.DisableIME() // must not panic

	// Host without XIM → nil capability.
	h := &x11Host{st: &x11State{}}
	if got := imeForX11(h); got != nil {
		t.Fatalf("host without xim should have nil IME, got %T", got)
	}
}

// TestPushIMEDrain mirrors the wayland text-input queue contract for X11.
func TestX11PushIMEDrain(t *testing.T) {
	h := &x11Host{st: &x11State{}} // non-nil st so WaitEvents proceeds
	h.pushIME(Event{Type: EventIME, IMEKind: 1, IMEText: "ni hao"})
	h.imeMu.Lock()
	n := len(h.imeEvents)
	h.imeMu.Unlock()
	if n != 1 {
		t.Fatalf("queued = %d", n)
	}
	// Drain via WaitEvents path (poll 0; no native events since st has no
	// pending func — the IME queue is drained regardless).
	evs := h.WaitEvents(0)
	found := false
	for _, ev := range evs {
		if ev.Type == EventIME && ev.IMEText == "ni hao" {
			found = true
		}
	}
	if !found {
		t.Fatalf("IME event not drained: %+v", evs)
	}
}
