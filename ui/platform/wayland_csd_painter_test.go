//go:build linux

package platform

import "testing"

// TestWaylandCSDNarrowPaint is a regression test for the tiny-window crash
// ("DBG t=7.8 EventResize 8x1" + `panic: runtime error: index out of range
// [-120]`): interactive resize can configure the window down to ~1px wide,
// and the title bar's right-aligned buttons then land at negative x. The
// painter must skip them (and putPxA guards negative coords) instead of
// indexing the buffer at a negative offset.
func TestWaylandCSDNarrowPaint(t *testing.T) {
	st := csdState{Focused: true, Title: "narrow window"}
	// Every width from the reported 8x1 crash up to just below the
	// button-cluster threshold, plus the exact 132 threshold.
	widths := []int{1, 2, 7, 8, 9, 31, 63, 100, 131, 132}
	for _, w := range widths {
		buf := make([]byte, w*32*4)
		paintTitleBar(buf, w, 32, st)
		// Border edges are painted with the same buffer semantics.
		edgeW, edgeH := 4, 36 // left/right border (ch+csdTitleBarHeight)
		edge := make([]byte, edgeW*edgeH*4)
		paintBorder(edge, edgeW, edgeH)
		edge2 := make([]byte, (w+8)*4*4)
		paintBorder(edge2, w+8, 4) // bottom border (cw+8) x 4
	}
	// putPxA negative/out-of-range coordinates must be no-ops.
	small := make([]byte, 8*8*4)
	for _, x := range []int{-124, -36, -1, 8, 9, 1000} {
		for _, y := range []int{-32, -1, 8, 1000} {
			putPxA(small, 8, x, y, 1, 2, 3, 0xFF)
		}
	}
}

// TestWaylandCSDNarrowButtonsNotDrawn: below 3*csdButtonW the buttons must
// not be painted (their left edge would be negative); at exactly the
// threshold they start. Guards the paintTitleBar early-return.
func TestWaylandCSDNarrowButtonsNotDrawn(t *testing.T) {
	st := csdState{Focused: true, Title: "t"}
	narrow := make([]byte, 131*32*4)
	// Buttons draw hover/press backgrounds into the last 3*44 columns of
	// the minimized state; with no buttons painted, the buffer pixels there
	// stay the plain background fill.
	paintTitleBar(narrow, 131, 32, st)
	// The whole first row must be the focus background color.
	for x := 100; x < 131; x++ {
		if got := narrow[x*4]; got != csdColorBgFocus[0] || narrow[x*4+3] != 0xFF {
			t.Fatalf("w=131 button area pixel x=%d not background", x)
		}
	}

	full := make([]byte, 132*32*4)
	paintTitleBar(full, 132, 32, st)
	// Glyphs draw centered at y=16; at w=132 the close glyph (cx=110)
	// must land in the last button column. Assert the row differs from
	// plain background vs the narrow case — a weak but useful smoke
	// signal that buttons actually painted at the threshold.
	sameness := true
	for x := 100; x < 132; x++ {
		if full[16*132*4+x*4] != csdColorBgFocus[0] {
			sameness = false
			break
		}
	}
	if sameness {
		t.Fatalf("w=132 painted nothing but background")
	}
}