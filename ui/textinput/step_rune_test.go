package textinput

import "testing"

// stepRuneUTF16 is the single rune stepper behind password moves and rune
// fallbacks: astral runes move by 2, everything else by 1, clamped into
// range. Locks the convergence (extend paths used to split astral runes
// with plain cur±1).
func TestStepRuneUTF16(t *testing.T) {
	const s = "a𐀀b" // utf16 units: a=1, 𐀀=2, b=1 → len 4
	if n := utf16Len(s); n != 4 {
		t.Fatalf("fixture len=%d want 4", n)
	}
	for _, tc := range []struct {
		cur, delta, want int
	}{
		{0, 1, 1}, {1, 1, 3}, {3, 1, 4}, {4, 1, 4},
		{4, -1, 3}, {3, -1, 1}, {1, -1, 0}, {0, -1, 0},
	} {
		if got := stepRuneUTF16(s, tc.cur, tc.delta); got != tc.want {
			t.Fatalf("step(%d,%+d)=%d want %d", tc.cur, tc.delta, got, tc.want)
		}
	}
	// Parity with the collapsed cursor moves over mixed text.
	e := New()
	mustSetText(t, e, s)
	e.SetSelection(TextRange{Base: 0, Extent: 0})
	for _, want := range []int{1, 3, 4, 4} {
		e.MoveCursorForward()
		if e.SelectionRange().Extent != want {
			t.Fatalf("forward extent=%d want %d", e.SelectionRange().Extent, want)
		}
	}
	for _, want := range []int{3, 1, 0, 0} {
		e.MoveCursorBack()
		if e.SelectionRange().Extent != want {
			t.Fatalf("back extent=%d want %d", e.SelectionRange().Extent, want)
		}
	}
}
