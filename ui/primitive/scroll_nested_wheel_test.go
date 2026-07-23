package primitive_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Nested scroll: wheel on inner viewport scrolls inner only (DispatchScroll
// hits nearest ScrollHandler and stops when Handled).
func TestNestedScrollWheelHitsInner(t *testing.T) {
	innerBody := primitive.NewBox()
	innerBody.Width, innerBody.Height = 80, 400
	inner := primitive.NewScrollViewport(innerBody)
	inner.Width, inner.Height = 100, 100
	inner.SetScroll(0, 0)

	// Place inner at origin inside outer content.
	outerBody := primitive.NewBox(inner)
	outerBody.Width, outerBody.Height = 120, 500
	outer := primitive.NewScrollViewport(outerBody)
	outer.Width, outer.Height = 120, 120
	outer.SetScroll(0, 0)

	root := primitive.NewBox(outer)
	root.Width, root.Height = 120, 120
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 120, Height: 120})

	// Wheel at center of window → should hit inner first.
	tree.DispatchScroll(&core.ScrollEvent{X: 50, Y: 50, DY: 40})
	if inner.ScrollY <= 0 {
		t.Fatalf("inner ScrollY=%v want >0 after wheel", inner.ScrollY)
	}
	// Outer should not have consumed the same event if inner handled it.
	if outer.ScrollY != 0 {
		t.Fatalf("outer ScrollY=%v want 0 (inner should handle)", outer.ScrollY)
	}
}

// Wheel on outer-only region scrolls outer (not zero effect).
func TestNestedScrollWheelOnOuter(t *testing.T) {
	// Only outer scrollable tall content — no nested viewport.
	tall := primitive.NewBox()
	tall.Width, tall.Height = 100, 400
	outer := primitive.NewScrollViewport(tall)
	outer.Width, outer.Height = 100, 100
	root := primitive.NewBox(outer)
	root.Width, root.Height = 100, 100
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 100, Height: 100})

	tree.DispatchScroll(&core.ScrollEvent{X: 40, Y: 40, DY: 30})
	if outer.ScrollY <= 0 {
		t.Fatalf("outer ScrollY=%v want >0", outer.ScrollY)
	}
}
