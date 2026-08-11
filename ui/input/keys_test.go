package input

import "testing"

// TestKeyTableStable locks the logical key numbering so serialized key codes
// stay comparable across versions. Appending new keys is fine; renumbering
// existing ones is a breaking change.
func TestKeyTableStable(t *testing.T) {
	// KeyNone = 0; letters follow.
	if KeyNone != 0 {
		t.Fatalf("KeyNone = %d, want 0", KeyNone)
	}
	// Digits come right after Z.
	if Key0 != KeyZ+1 || Key9 != Key0+9 {
		t.Fatalf("digit range broken: Z=%d 0=%d 9=%d", KeyZ, Key0, Key9)
	}
	// F-keys after digits.
	if KeyF1 != Key9+1 {
		t.Fatalf("F1 = %d, want %d", KeyF1, Key9+1)
	}
	// Navigation block order is not asserted numerically (append-friendly),
	// but these must exist and be distinct.
	seen := map[Key]bool{}
	for _, k := range []Key{
		KeyEnter, KeyTab, KeySpace, KeyBackspace, KeyDelete, KeyEscape,
		KeyHome, KeyEnd, KeyPageUp, KeyPageDown, KeyInsert,
		KeyArrowUp, KeyArrowDown, KeyArrowLeft, KeyArrowRight,
		KeyShift, KeyControl, KeyAlt, KeyMeta,
		KeyPad0, KeyPadEnter, KeyIMECompose, KeyIMECandidateSelect,
	} {
		if k == KeyNone {
			t.Fatalf("%s must not be KeyNone", k)
		}
		if seen[k] {
			t.Fatalf("duplicate key %s", k)
		}
		seen[k] = true
	}
}

func TestKeyString(t *testing.T) {
	cases := map[Key]string{
		KeyA: "a", KeyZ: "z", Key0: "0", Key9: "9",
		KeyEnter: "enter", KeySpace: "space", KeyArrowUp: "arrowup",
		KeyPadAdd: "padadd", KeyIMECandidateNext: "imecandnext",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("%s.String() = %q, want %q", k, got, want)
		}
	}
	if KeyUnknown := Key(9999).String(); KeyUnknown != "unknown" {
		t.Fatalf("unknown key string = %q", KeyUnknown)
	}
}