package primitive_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// nestedScrollTree builds outer(inner) scroll viewports for F7 tests.
func nestedScrollTree(t *testing.T) (inner, outer *primitive.ScrollViewport, tree *core.Tree) {
	t.Helper()
	innerBody := primitive.NewBox()
	innerBody.Width, innerBody.Height = 80, 400
	inner = primitive.NewScrollViewport(innerBody)
	inner.Width, inner.Height = 100, 100
	inner.SetScroll(0, 0)

	outerBody := primitive.NewBox(inner)
	outerBody.Width, outerBody.Height = 120, 500
	outer = primitive.NewScrollViewport(outerBody)
	outer.Width, outer.Height = 120, 120
	outer.SetScroll(0, 0)

	root := primitive.NewBox(outer)
	root.Width, root.Height = 120, 120
	tree = core.NewTree(root)
	tree.Layout(core.Size{Width: 120, Height: 120})
	return inner, outer, tree
}

// Nested scroll: wheel on inner viewport scrolls inner only when it can absorb.
func TestNestedScrollWheelHitsInner(t *testing.T) {
	inner, outer, tree := nestedScrollTree(t)

	tree.DispatchScroll(&core.ScrollEvent{X: 50, Y: 50, DY: 40})
	if inner.ScrollY <= 0 {
		t.Fatalf("inner ScrollY=%v want >0 after wheel", inner.ScrollY)
	}
	if outer.ScrollY != 0 {
		t.Fatalf("outer ScrollY=%v want 0 (inner should handle)", outer.ScrollY)
	}
}

// Wheel on outer-only region scrolls outer (not zero effect).
func TestNestedScrollWheelOnOuter(t *testing.T) {
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

// F7: at top edge, further upward wheel chains to outer.
func TestNestedScrollChainAtTopEdge(t *testing.T) {
	inner, outer, tree := nestedScrollTree(t)
	// Inner at top (ScrollY=0). Wheel "up" (negative DY) cannot move inner → chain.
	// First give outer some room by not scrolling it yet; outer should receive chain.
	// Note: sign convention ScrollY += DY*step; negative DY tries to decrease ScrollY.
	ev := &core.ScrollEvent{X: 50, Y: 50, DY: -40}
	tree.DispatchScroll(ev)
	if inner.ScrollY != 0 {
		t.Fatalf("inner should stay at top, got %v", inner.ScrollY)
	}
	// Outer may or may not move on negative DY at its own top (also 0).
	// Reset: scroll outer down first, then chain again.
	outer.SetScroll(0, 50)
	inner.SetScroll(0, 0)
	// Re-layout not required for SetScroll paint path.
	tree.DispatchScroll(&core.ScrollEvent{X: 50, Y: 50, DY: -40})
	if inner.ScrollY != 0 {
		t.Fatalf("inner still at top want 0 got %v", inner.ScrollY)
	}
	if outer.ScrollY >= 50 {
		t.Fatalf("outer should chain-scroll up from 50, got %v", outer.ScrollY)
	}
}

// F7: at bottom edge, further downward wheel chains to outer.
func TestNestedScrollChainAtBottomEdge(t *testing.T) {
	inner, outer, tree := nestedScrollTree(t)
	_, maxY := inner.MaxScroll()
	if maxY <= 0 {
		t.Fatalf("inner maxY=%v want >0", maxY)
	}
	inner.SetScroll(0, maxY)
	outer.SetScroll(0, 0)

	tree.DispatchScroll(&core.ScrollEvent{X: 50, Y: 50, DY: 40})
	if inner.ScrollY != maxY {
		// clamp may float; allow small epsilon
		if inner.ScrollY < maxY-0.5 || inner.ScrollY > maxY+0.5 {
			t.Fatalf("inner should stay at bottom max=%v got %v", maxY, inner.ScrollY)
		}
	}
	if outer.ScrollY <= 0 {
		t.Fatalf("outer should chain-scroll down, got %v", outer.ScrollY)
	}
}

// F7: TrapWheel keeps event even at edge (no chain).
func TestNestedScrollTrapWheelNoChain(t *testing.T) {
	inner, outer, tree := nestedScrollTree(t)
	inner.TrapWheel = true
	_, maxY := inner.MaxScroll()
	if maxY <= 0 {
		t.Fatal("need overflow")
	}
	// At bottom edge; further down would chain without trap.
	inner.SetScroll(0, maxY)
	outer.SetScroll(0, 0)

	tree.DispatchScroll(&core.ScrollEvent{X: 50, Y: 50, DY: 40})
	if outer.ScrollY != 0 {
		t.Fatalf("TrapWheel must not chain: outer=%v want 0", outer.ScrollY)
	}
	// Control: without trap, same setup chains.
	inner.TrapWheel = false
	inner.SetScroll(0, maxY)
	outer.SetScroll(0, 0)
	tree.DispatchScroll(&core.ScrollEvent{X: 50, Y: 50, DY: 40})
	if outer.ScrollY <= 0 {
		t.Fatalf("without TrapWheel should chain, outer=%v", outer.ScrollY)
	}
}

// F7: mid-range scroll does not chain.
func TestNestedScrollMidRangeNoChain(t *testing.T) {
	inner, outer, tree := nestedScrollTree(t)
	_, maxY := inner.MaxScroll()
	if maxY < 80 {
		t.Fatalf("need room to scroll, maxY=%v", maxY)
	}
	inner.SetScroll(0, 40)
	outer.SetScroll(0, 0)

	before := inner.ScrollY
	tree.DispatchScroll(&core.ScrollEvent{X: 50, Y: 50, DY: 20})
	if inner.ScrollY <= before {
		t.Fatalf("inner should scroll down from %v, got %v", before, inner.ScrollY)
	}
	if outer.ScrollY != 0 {
		t.Fatalf("outer must stay 0 when inner absorbs, got %v", outer.ScrollY)
	}
}

// F7: no-overflow inner always chains.
func TestNestedScrollNoOverflowChains(t *testing.T) {
	// Inner content fits entirely — no overflow.
	innerBody := primitive.NewBox()
	innerBody.Width, innerBody.Height = 80, 50
	inner := primitive.NewScrollViewport(innerBody)
	inner.Width, inner.Height = 100, 100

	// tall outer content under/with inner
	pad := primitive.NewBox()
	pad.Width, pad.Height = 120, 400
	col := primitive.Column(inner, pad)
	outer := primitive.NewScrollViewport(col)
	outer.Width, outer.Height = 120, 120

	root := primitive.NewBox(outer)
	root.Width, root.Height = 120, 120
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 120, Height: 120})

	// Hit center of inner (top of outer content).
	tree.DispatchScroll(&core.ScrollEvent{X: 50, Y: 40, DY: 30})
	if outer.ScrollY <= 0 {
		t.Fatalf("no-overflow inner should chain to outer, outer.ScrollY=%v", outer.ScrollY)
	}
}
