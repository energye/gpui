package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
)

// nestedPair builds parent/child viewports with Parent handoff wired.
//
//	parentVP (100×200) content height 500 → maxScrollY=300
//	childVP nested content height 400 → maxScrollY set explicitly
func nestedPair(t *testing.T) (parentVP, childVP *rendering.RenderViewport, childSC *rendering.Scrollable) {
	t.Helper()
	const W, parentH = 100.0, 200.0
	const childContentH, parentContentH = 400.0, 500.0

	childContent := rendering.NewRenderColorBox(W, childContentH, 0.2, 0.4, 0.6, 1)
	childVP = rendering.NewRenderViewport(childContent)

	parentContent := rendering.NewRenderBox(childVP)
	parentContent.FixedWidth = W
	parentContent.FixedHeight = parentContentH

	parentVP = rendering.NewRenderViewport(parentContent)
	parentSC := rendering.NewScrollable(parentVP)
	childSC = rendering.NewScrollable(childVP)
	childSC.SetParent(parentSC)
	childSC.TouchSlop = 1
	parentSC.TouchSlop = 1

	owner := rendering.NewPipelineOwner(parentVP)
	owner.FlushLayout(rendering.Size{Width: W, Height: parentH}, true)

	// Deterministic clamps after layout.
	ch := childVP.Size().Height
	if ch < 1 {
		ch = 100
	}
	childVP.SetMaxScrollY(childContentH - ch)
	parentVP.SetMaxScrollY(parentContentH - parentH)
	return parentVP, childVP, childSC
}

func drag(sc *rendering.Scrollable, fromY float64, deltas []float64) {
	sc.HandlePointer(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, Y: fromY})
	y := fromY
	for _, d := range deltas {
		y += d
		sc.HandlePointer(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerMove, Y: y})
	}
	sc.HandlePointer(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerUp, Y: y})
}

func nDeltas(n int, step float64) []float64 {
	d := make([]float64, n)
	for i := range d {
		d[i] = step
	}
	return d
}

func TestNested_ChildNotAtEdge_ParentUnchanged(t *testing.T) {
	parentVP, childVP, childSC := nestedPair(t)
	childVP.SetScrollOffset(0, 50)
	parentBefore := parentVP.ScrollOffset().Y

	// Finger up → child scrollY increases; not at bottom yet → parent unchanged.
	drag(childSC, 100, nDeltas(50, -2))

	if parentVP.ScrollOffset().Y != parentBefore {
		t.Fatalf("parent scrollY %v → %v (child not at edge)", parentBefore, parentVP.ScrollOffset().Y)
	}
	if childVP.ScrollOffset().Y <= 50 {
		t.Fatalf("child scrollY should increase, got %v", childVP.ScrollOffset().Y)
	}
}

func TestNested_ChildAtTop_ContinuePull_ParentScrolls(t *testing.T) {
	parentVP, childVP, childSC := nestedPair(t)
	// Parent mid-scroll so residual decrease is absorbable.
	parentVP.SetScrollOffset(0, 80)
	childVP.SetScrollOffset(0, 0)

	parentBefore := parentVP.ScrollOffset().Y
	// Finger down at child top → residual to parent (scrollY decreases).
	drag(childSC, 50, nDeltas(50, 2))

	if childVP.ScrollOffset().Y > 1e-6 {
		t.Fatalf("child should remain at top, got %v", childVP.ScrollOffset().Y)
	}
	if parentVP.ScrollOffset().Y >= parentBefore {
		t.Fatalf("parent scrollY should decrease from %v, got %v", parentBefore, parentVP.ScrollOffset().Y)
	}
}

func TestNested_ChildAtBottom_ContinuePush_ParentScrolls(t *testing.T) {
	parentVP, childVP, childSC := nestedPair(t)
	maxC := childVP.MaxScrollY()
	if maxC < 1 {
		t.Fatalf("child maxScrollY=%v", maxC)
	}
	childVP.SetScrollOffset(0, maxC)
	parentVP.SetScrollOffset(0, 0)
	parentBefore := parentVP.ScrollOffset().Y

	// Finger up at child bottom → residual increases parent scrollY.
	drag(childSC, 100, nDeltas(50, -2))

	if parentVP.ScrollOffset().Y <= parentBefore {
		t.Fatalf("parent scrollY should increase from %v, got %v", parentBefore, parentVP.ScrollOffset().Y)
	}
	if childVP.ScrollOffset().Y < maxC-1e-3 {
		t.Fatalf("child should stay near max %v, got %v", maxC, childVP.ScrollOffset().Y)
	}
}

func TestNested_NoDoubleDelta(t *testing.T) {
	parentVP, childVP, childSC := nestedPair(t)
	childVP.SetScrollOffset(0, 20)
	parentVP.SetScrollOffset(0, 100)
	// Intended dScrollY=-50: child absorbs -20 (20→0), residual -30 → parent 100-30=70.
	childSC.ApplyScrollDeltaForTest(0, -50)
	if childVP.ScrollOffset().Y > 1e-6 {
		t.Fatalf("child Y=%v want 0", childVP.ScrollOffset().Y)
	}
	if got := parentVP.ScrollOffset().Y; got < 69.5 || got > 70.5 {
		t.Fatalf("parent Y=%v want ~70 (absorbed part not double-applied)", got)
	}
}

func TestNested_WheelBubble(t *testing.T) {
	parentVP, childVP, childSC := nestedPair(t)
	childVP.SetScrollOffset(0, 0)
	parentVP.SetScrollOffset(0, 50)
	// Wheel down (ScrollY+) moves content in scroll space: child at top
	// consumes the whole delta; parent stays put.
	childSC.HandlePointer(platform.Event{
		Type: platform.EventPointer, Pointer: platform.PointerScroll, ScrollY: 40,
	})
	if got := childVP.ScrollOffset().Y; got < 39.5 || got > 40.5 {
		t.Fatalf("child Y=%v want ~40", got)
	}
	if got := parentVP.ScrollOffset().Y; got != 50 {
		t.Fatalf("parent Y=%v want 50 (child not at edge)", got)
	}
}

func TestNested_WheelBubble_AtBottom(t *testing.T) {
	parentVP, childVP, childSC := nestedPair(t)
	maxC := childVP.MaxScrollY()
	childVP.SetScrollOffset(0, maxC)
	parentVP.SetScrollOffset(0, 50)
	// Child at bottom: wheel down residual bubbles to parent.
	childSC.HandlePointer(platform.Event{
		Type: platform.EventPointer, Pointer: platform.PointerScroll, ScrollY: 40,
	})
	if got := childVP.ScrollOffset().Y; got != maxC {
		t.Fatalf("child Y=%v want max %v", got, maxC)
	}
	if got := parentVP.ScrollOffset().Y; got < 89.5 || got > 90.5 {
		t.Fatalf("parent Y=%v want ~90", got)
	}
}

func TestWheel_NotchStepAndDirection(t *testing.T) {
	parentVP, childVP, childSC := nestedPair(t)
	_ = parentVP
	childVP.SetScrollOffset(0, 100)
	// One discrete notch (exactly ±1, X11 buttons 4/5) = one 48px step.
	childSC.HandlePointer(platform.Event{
		Type: platform.EventPointer, Pointer: platform.PointerScroll, ScrollY: 1,
	})
	if got := childVP.ScrollOffset().Y; got != 148 {
		t.Fatalf("notch down Y=%v want 148", got)
	}
	childSC.HandlePointer(platform.Event{
		Type: platform.EventPointer, Pointer: platform.PointerScroll, ScrollY: -1,
	})
	if got := childVP.ScrollOffset().Y; got != 100 {
		t.Fatalf("notch up Y=%v want 100", got)
	}
	// Continuous valuators (Wayland/trackpad) pass through unscaled.
	childSC.HandlePointer(platform.Event{
		Type: platform.EventPointer, Pointer: platform.PointerScroll, ScrollY: 12.5,
	})
	if got := childVP.ScrollOffset().Y; got != 112.5 {
		t.Fatalf("continuous Y=%v want 112.5", got)
	}
}

func TestNested_TransferDisabled(t *testing.T) {
	parentVP, childVP, childSC := nestedPair(t)
	childSC.TransferAtEdge = false
	childVP.SetScrollOffset(0, 0)
	parentVP.SetScrollOffset(0, 80)
	childSC.ApplyScrollDeltaForTest(0, -30)
	if parentVP.ScrollOffset().Y != 80 {
		t.Fatalf("parent should not move when TransferAtEdge=false, got %v", parentVP.ScrollOffset().Y)
	}
}
