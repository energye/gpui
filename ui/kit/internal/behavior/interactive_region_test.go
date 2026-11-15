//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package behavior_test

import (
	"testing"

	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/kit/internal/behavior"
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/rendering"
)

// buildRegionTree wires one region (fixed 100×40) under a root box and
// lays it out at offset (10, 10), giving hit testing real geometry.
func buildRegionTree(t *testing.T) (*behavior.InteractiveRegion, *rendering.RenderBox) {
	t.Helper()
	inner := rendering.NewRenderColorBox(100, 40, 0.2, 0.5, 0.9, 1)
	region := behavior.NewInteractiveRegion(behavior.InteractiveConfig{Focusable: true}, inner)
	region.FixedWidth, region.FixedHeight = 100, 40

	root := rendering.NewRenderBox(region)
	root.FixedWidth, root.FixedHeight = 300, 200
	_ = rendering.NewPipelineOwner(root)
	root.Layout(rendering.Tight(300, 200))
	region.SetOffset(rendering.Point{X: 10, Y: 10})
	return region, root
}

// routerWithTree builds an InputRouter whose hit test walks the real
// render tree — the standard chain, no window-side manual logic.
func routerWithTree(root rendering.RenderObject) *embedder.InputRouter {
	return embedder.NewInputRouter(func(x, y float64) (overlay.Band, rendering.RenderObject, *overlay.Entry) {
		return 0, root.HitTest(rendering.Point{X: x, Y: y}), nil
	}, nil)
}

// down/up helpers build window-space pointer events.
func evDown(id, x, y float64) input.Event {
	return input.Event{Kind: input.KindPointer, Pointer: input.PointerEvent{ID: int(id), X: x, Y: y, Kind: input.PointerDown}}
}
func evUp(id, x, y float64) input.Event {
	return input.Event{Kind: input.KindPointer, Pointer: input.PointerEvent{ID: int(id), X: x, Y: y, Kind: input.PointerUp}}
}
func evMove(id, x, y float64) input.Event {
	return input.Event{Kind: input.KindPointer, Pointer: input.PointerEvent{ID: int(id), X: x, Y: y, Kind: input.PointerMove}}
}

// TestRegion_AutoArenaTap: zero manual wiring — down on the region's
// bounds joins the arena, up within slop fires OnTap once.
func TestRegion_AutoArenaTap(t *testing.T) {
	region, root := buildRegionTree(t)
	taps := 0
	region.OnTap = func() { taps++ }

	r := routerWithTree(root)
	r.Route(evDown(1, 50, 30))
	if !region.States().Has(scope.StatePressed) {
		t.Fatal("down on region did not set pressed")
	}
	r.Route(evUp(1, 52, 32))
	if taps != 1 {
		t.Fatalf("tap fired %d times, want 1", taps)
	}
	if region.States().Has(scope.StatePressed) {
		t.Fatal("up did not clear pressed")
	}
}

// TestRegion_TapAndPanSamePathSingleWinner: the arena rule — tap and
// pan recognizers joining the same down must resolve to a single winner
// (Flutter GestureArena single-winner semantics): pan accepting rejects
// the tap, so a far drag never fires OnTap.
func TestRegion_TapAndPanSamePathSingleWinner(t *testing.T) {
	region, root := buildRegionTree(t)
	taps := 0
	region.OnTap = func() { taps++ }

	r := routerWithTree(root)
	r.Route(evDown(2, 50, 30))
	r.Route(evMove(2, 200, 30)) // far beyond tap slop
	r.Route(evUp(2, 200, 30))

	// Tap moved out of slop: must reject itself, not fire.
	if taps != 0 {
		t.Fatalf("tap fired after leaving slop, taps=%d", taps)
	}
}

// TestRegion_DisabledSwallows: disabled regions never join the arena and
// never fire taps.
func TestRegion_DisabledSwallows(t *testing.T) {
	inner := rendering.NewRenderColorBox(100, 40, 0.2, 0.5, 0.9, 1)
	region := behavior.NewInteractiveRegion(behavior.InteractiveConfig{Disabled: true}, inner)
	region.FixedWidth, region.FixedHeight = 100, 40
	taps := 0
	region.OnTap = func() { taps++ }

	root := rendering.NewRenderBox(region)
	root.FixedWidth, root.FixedHeight = 300, 200
	_ = rendering.NewPipelineOwner(root)
	root.Layout(rendering.Tight(300, 200))
	region.SetOffset(rendering.Point{X: 10, Y: 10})

	r := routerWithTree(root)
	r.Route(evDown(3, 50, 30))
	r.Route(evUp(3, 50, 30))
	if taps != 0 {
		t.Fatalf("disabled region fired tap %d times", taps)
	}
	if region.States().Has(scope.StatePressed) {
		t.Fatal("disabled region entered pressed state")
	}
}

// TestRegion_HoverFollowsMove: hover tracks in-bounds moves and enter/leave.
func TestRegion_HoverFollowsMove(t *testing.T) {
	region, root := buildRegionTree(t)
	r := routerWithTree(root)

	r.Route(evMove(4, 50, 30))
	if !region.States().Has(scope.StateHover) {
		t.Fatal("in-bounds move did not set hover")
	}
	r.Route(evMove(4, 250, 150))
	if region.States().Has(scope.StateHover) {
		t.Fatal("out-of-bounds move kept hover")
	}
}

// TestRegion_TapSlopReject: drag beyond slop between down and up rejects
// the tap (no click on drag).
func TestRegion_TapSlopReject(t *testing.T) {
	region, root := buildRegionTree(t)
	taps := 0
	region.OnTap = func() { taps++ }

	r := routerWithTree(root)
	r.Route(evDown(5, 50, 30))
	r.Route(evMove(5, 90, 30))
	r.Route(evUp(5, 90, 30))
	if taps != 0 {
		t.Fatalf("drag-up fired tap %d times", taps)
	}
}
