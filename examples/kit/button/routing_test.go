package main

import (
	"testing"

	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/rendering"
)

// TestLiveRouter_PressTracking locks the R3-3 routing gate: window-level
// hit-test routing (at), press-ID tracking across move (down on A, release
// elsewhere fires nothing), same-spot release fires once, and sticky hover
// (one holder at a time, empty space clears). These paths were previously
// exercised only by human hands in the manual phase.
func TestLiveRouter_PressTracking(t *testing.T) {
	bw := buildItems()
	vp := rendering.NewRenderViewport(nil)
	lw := &liveWindow{bw: bw, vp: vp, fmgr: focus.NewManager(), pressID: -1, hoverID: -1}

	// Two plain click-kind instances far apart (no switch/loading side effects).
	var a, b int = -1, -1
	for i := range bw.items {
		it := &bw.items[i]
		if it.b == nil || it.w <= 0 || it.h <= 0 || it.kind != "click" {
			continue
		}
		if a < 0 {
			a = i
		} else {
			b = i
			break
		}
	}
	if a < 0 || b < 0 {
		t.Fatal("need two click-kind instances")
	}
	center := func(i int) (float64, float64) {
		it := &bw.items[i]
		return it.x + it.w/2, it.y + it.h/2 + viewportTop
	}
	ax, ay := center(a)

	// at() resolves the instance under the point.
	if got, _, _ := lw.at(ax, ay); got != a {
		t.Fatalf("at(A)=%d want %d", got, a)
	}
	if got, _, _ := lw.at(-10, -10); got != -1 {
		t.Fatalf("at(empty)=%d want -1", got)
	}

	// Down on A tracks pressID; release elsewhere fires nothing on A.
	firedA := 0
	bw.items[a].b.OnClick = func() { firedA++ }
	lw.handleBladePress(ax, ay, true)
	if lw.pressID != a {
		t.Fatalf("pressID=%d want %d after down on A", lw.pressID, a)
	}
	bx, by := center(b)
	lw.handleBladePress(bx, by, false)
	if firedA != 0 {
		t.Fatalf("move-out release fired %d want 0", firedA)
	}
	if lw.pressID != -1 {
		t.Fatalf("pressID=%d want -1 after release", lw.pressID)
	}

	// Same-spot release fires exactly once.
	firedA = 0
	lw.handleBladePress(ax, ay, true)
	lw.handleBladePress(ax, ay, false)
	if firedA != 1 {
		t.Fatalf("same-spot release fired %d want 1", firedA)
	}
}

// TestLiveRouter_StickyHover locks one-holder hover + empty-space clear.
func TestLiveRouter_StickyHover(t *testing.T) {
	bw := buildItems()
	vp := rendering.NewRenderViewport(nil)
	lw := &liveWindow{bw: bw, vp: vp, fmgr: focus.NewManager(), pressID: -1, hoverID: -1}

	var a, b int = -1, -1
	for i := range bw.items {
		it := &bw.items[i]
		if it.b == nil || it.w <= 0 || it.h <= 0 || it.kind != "click" {
			continue
		}
		if a < 0 {
			a = i
		} else {
			b = i
			break
		}
	}
	if a < 0 || b < 0 {
		t.Fatal("need two click-kind instances")
	}
	center := func(i int) (float64, float64) {
		it := &bw.items[i]
		return it.x + it.w/2, it.y + it.h/2 + viewportTop
	}
	ax, ay := center(a)
	bx, by := center(b)

	lw.handleBladeHover(ax, ay)
	if lw.hoverID != a || !bw.items[a].b.Hovered() {
		t.Fatalf("hover A: id=%d hovered=%v", lw.hoverID, bw.items[a].b.Hovered())
	}
	lw.handleBladeHover(bx, by)
	if lw.hoverID != b {
		t.Fatalf("hover B: id=%d want %d", lw.hoverID, b)
	}
	if bw.items[a].b.Hovered() {
		t.Fatal("A must unhover when B takes hover (sticky)")
	}
	lw.handleBladeHover(-10, -10)
	if lw.hoverID != -1 || bw.items[b].b.Hovered() {
		t.Fatal("empty space must clear hover")
	}
}
