//go:build linux && !nouiplatform

package platform

import "testing"

func TestWlFixedToFloat(t *testing.T) {
	// 256 → 1.0; 128 → 0.5; negative
	if g := wlFixedToFloat(256); g != 1.0 {
		t.Fatalf("256 → %v", g)
	}
	if g := wlFixedToFloat(128); g != 0.5 {
		t.Fatalf("128 → %v", g)
	}
	neg := uint32(0xFFFFFF00) // int32(-256) as wl_fixed bits
	if g := wlFixedToFloat(uintptr(neg)); g != -1.0 {
		t.Fatalf("-256 → %v", g)
	}
}

func TestWlKeyNameBasics(t *testing.T) {
	cases := []struct {
		code      uint32
		shift     bool
		key, text string
	}{
		{keyTab, false, "Tab", ""},
		{keyEnter, false, "Enter", ""},
		{keySpace, false, " ", " "},
		{keyA, false, "a", "a"},
		{keyA, true, "A", "A"},
		{key1, false, "1", "1"},
		{key1, true, "!", "!"},
		{keyEsc, false, "Escape", ""},
		{keyLeft, false, "Left", ""},
	}
	for _, c := range cases {
		k, tx := wlKeyName(c.code, c.shift)
		if k != c.key || tx != c.text {
			t.Fatalf("code=%d shift=%v → (%q,%q) want (%q,%q)",
				c.code, c.shift, k, tx, c.key, c.text)
		}
	}
}

func TestSeatCapBits(t *testing.T) {
	// Documented wayland.xml values — keep stable for capability handling.
	if wlSeatCapPointer != 1 || wlSeatCapKeyboard != 2 {
		t.Fatalf("cap bits drift: ptr=%d key=%d", wlSeatCapPointer, wlSeatCapKeyboard)
	}
}
