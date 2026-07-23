package platform

import "testing"

func TestParseModifierState(t *testing.T) {
	sh, ctrl, alt, meta := ParseModifierState(1 | 4 | 8 | 64)
	if !sh || !ctrl || !alt || !meta {
		t.Fatalf("got sh=%v ctrl=%v alt=%v meta=%v", sh, ctrl, alt, meta)
	}
	sh, ctrl, alt, meta = ParseModifierState(0)
	if sh || ctrl || alt || meta {
		t.Fatal("zero")
	}
}
