//go:build linux

package platform

import (
	"testing"
	"time"
	"unsafe"
)

// TestXKBResolve verifies libxkbcommon entry points resolve.
func TestXKBResolve(t *testing.T) {
	f := loadXKB()
	if f == nil {
		t.Skip("libxkbcommon not available")
	}
	if f.contextNew == nil || f.keymapNewFrom == nil || f.stateNew == nil ||
		f.stateKeySym == nil || f.keysymToUTF8 == nil {
		t.Fatalf("xkb funcs incomplete: %+v", f)
	}
}

// TestDecodeRune covers the minimal UTF-8 decoder used by the key callback.
func TestDecodeRune(t *testing.T) {
	cases := []struct {
		in   []byte
		want rune
		sz   int
	}{
		{[]byte("a"), 'a', 1},
		{[]byte("z"), 'z', 1},
		{[]byte{0xC3, 0xA9}, 'é', 2},             // é
		{[]byte{0xE4, 0xBD, 0xA0}, '你', 3},       // 你
		{[]byte{0xF0, 0x9F, 0x98, 0x80}, '😀', 4}, // emoji
		{[]byte{}, 0, 0},
	}
	for _, c := range cases {
		r, sz := decodeRune(c.in)
		if r != c.want || sz != c.sz {
			t.Errorf("decodeRune(%q) = (%c,%d), want (%c,%d)", c.in, r, sz, c.want, c.sz)
		}
	}
}

// TestKeyboardNilSafety verifies keyboard state degrades safely.
func TestKeyboardNilSafety(t *testing.T) {
	var st *wlKeyboardState
	st.destroy()
	var w *wlWin
	w.pushKey(Event{Type: EventKey, KeyCode: 1}) // nil receiver: no-op
	w = &wlWin{}
	w.pushKey(Event{Type: EventKey, KeyCode: 1})
	w.keyMu.Lock()
	n := len(w.keyEvents)
	w.keyMu.Unlock()
	if n != 1 {
		t.Fatalf("pushKey did not queue: %d", n)
	}
}

// TestPushKeyDrain mirrors the IME queue contract for keyboard events.
func TestPushKeyDrain(t *testing.T) {
	w := &wlWin{}
	w.pushKey(Event{Type: EventKey, KeyCode: 0xff61, Rune: 'a', Pressed: true})
	w.keyMu.Lock()
	n := len(w.keyEvents)
	w.keyMu.Unlock()
	if n != 1 {
		t.Fatalf("queued = %d", n)
	}
	// Drain mirror (poll needs a live display; drain logic is shared).
	w.keyMu.Lock()
	evs := append([]Event(nil), w.keyEvents...)
	w.keyEvents = nil
	w.keyMu.Unlock()
	if len(evs) != 1 || evs[0].Rune != 'a' {
		t.Fatalf("drained = %+v", evs)
	}
}

// TestWLKeyRepeatInfoParsing verifies repeat_info fields are stored.
func TestWLKeyRepeatInfoParsing(t *testing.T) {
	st := &wlKeyboardState{win: &wlWin{}}
	wlKbRepeatCB(uintptr(unsafe.Pointer(st)), 0, 30, 250)
	st.repeatMu.Lock()
	rate, delay := st.repRate, st.repDelayMs
	st.repeatMu.Unlock()
	if rate != 30 || delay != 250 {
		t.Fatalf("rate/delay = %d/%d, want 30/250", rate, delay)
	}
}

// drainKeyEvents swaps out the queued key events.
func drainKeyEvents(w *wlWin) []Event {
	w.keyMu.Lock()
	evs := append([]Event(nil), w.keyEvents...)
	w.keyEvents = nil
	w.keyMu.Unlock()
	return evs
}

// TestWLKeyRepeatFiresAndCancels drives the client-side repeat machinery:
// held key fires repeatedly; an unrelated release keeps it going; the
// repeating key's release stops it.
func TestWLKeyRepeatFiresAndCancels(t *testing.T) {
	w := &wlWin{}
	st := &wlKeyboardState{win: w, repRate: 200, repDelayMs: 20}

	st.armRepeat(38, 'a') // hold 'a'
	time.Sleep(150 * time.Millisecond)
	if n := len(drainKeyEvents(w)); n == 0 {
		t.Fatal("held key produced no repeat events")
	}

	st.cancelRepeatIf(39) // unrelated key released: repeat must continue
	time.Sleep(60 * time.Millisecond)
	if n := len(drainKeyEvents(w)); n == 0 {
		t.Fatal("unrelated release stopped the repeat")
	}

	st.cancelRepeatIf(38) // the repeating key itself released
	// A timer fire racing the cancel may have pushed one final event before
	// the cancel landed — discard in-flight events, then assert QUIESCENCE
	// (no pushes happen after heldKC=0).
	drainKeyEvents(w)
	time.Sleep(80 * time.Millisecond)
	if n := len(drainKeyEvents(w)); n != 0 {
		t.Fatalf("cancel did not stop repeats: %d more events", n)
	}
}

// TestWLKeyRepeatModifierNotArmed verifies modifiers and unknown keysyms
// never start repetition (holding Shift must not spam Shift).
func TestWLKeyRepeatModifierNotArmed(t *testing.T) {
	if !isModifierKeysym(0xffe1) || isModifierKeysym('a') {
		t.Fatal("modifier keysym table wrong")
	}
	w := &wlWin{}
	st := &wlKeyboardState{win: w, repRate: 200, repDelayMs: 10}
	st.armRepeat(50, 0xffe1) // Shift
	st.armRepeat(51, 0)      // unrecognized
	time.Sleep(60 * time.Millisecond)
	if n := len(drainKeyEvents(w)); n != 0 {
		t.Fatalf("modifier/unknown keysym repeated: %d events", n)
	}
}
