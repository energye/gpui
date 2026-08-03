package rendering

import (
	"testing"
)

// TestAlignBox_LayoutDrivenOffset verifies Flutter Align semantics: the child
// offset is derived from parent size × alignment, and re-computed when the
// parent constraints change (resize) — no imperative coordinates involved.
func TestAlignBox_LayoutDrivenOffset(t *testing.T) {
	hot := NewRenderColorBox(100, 100, 1, 0, 0, 1)
	align := NewRenderAlignBox(hot, 0.5, 0.5) // centered

	sz := align.Layout(Constraints{MinWidth: 0, MaxWidth: 900, MinHeight: 0, MaxHeight: 600})
	if sz.Width != 900 || sz.Height != 600 {
		t.Fatalf("align size = %v, want 900x600 (fills parent)", sz)
	}
	off := hot.Offset()
	if off.X != 400 || off.Y != 250 {
		t.Fatalf("centered offset = %v, want (400,250)", off)
	}

	// Parent resize → layout re-run with new constraints → offset follows.
	align.Layout(Constraints{MinWidth: 0, MaxWidth: 500, MinHeight: 0, MaxHeight: 300})
	off = hot.Offset()
	if off.X != 200 || off.Y != 100 {
		t.Fatalf("resized centered offset = %v, want (200,100)", off)
	}

	// Corner alignments.
	align.SetAlignment(0, 0)
	align.Layout(Constraints{MinWidth: 0, MaxWidth: 500, MinHeight: 0, MaxHeight: 300})
	if off = hot.Offset(); off.X != 0 || off.Y != 0 {
		t.Fatalf("top-left offset = %v, want (0,0)", off)
	}
	align.SetAlignment(1, 1)
	align.Layout(Constraints{MinWidth: 0, MaxWidth: 500, MinHeight: 0, MaxHeight: 300})
	if off = hot.Offset(); off.X != 400 || off.Y != 200 {
		t.Fatalf("bottom-right offset = %v, want (400,200)", off)
	}
}

// TestAlignBox_SetAlignmentDirties ensures alignment change triggers layout
// re-run (offset recompute) and child repaint.
func TestAlignBox_SetAlignmentDirties(t *testing.T) {
	hot := NewRenderColorBox(100, 100, 1, 0, 0, 1)
	align := NewRenderAlignBox(hot, 0.5, 0.5)
	align.Layout(Constraints{MinWidth: 0, MaxWidth: 900, MinHeight: 0, MaxHeight: 600})
	align.clearLayoutDirty()
	align.clearPaintDirty()
	hot.clearPaintDirty()

	align.SetAlignment(0.9, 0.5)
	if !align.NeedsLayout() {
		t.Fatal("SetAlignment must mark layout dirty")
	}
	if !hot.NeedsPaint() {
		t.Fatal("SetAlignment must mark child paint dirty (pixels move)")
	}
	align.Layout(Constraints{MinWidth: 0, MaxWidth: 900, MinHeight: 0, MaxHeight: 600})
	if off := hot.Offset(); off.X != 720 || off.Y != 250 {
		t.Fatalf("after SetAlignment offset = %v, want (720,250)", off)
	}
}
