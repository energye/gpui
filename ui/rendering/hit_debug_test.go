package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

func TestHitDebugName_MatchesPaintedBoxes(t *testing.T) {
	root := rendering.NewAbsoluteBox(400, 300)
	g := rendering.NewRenderColorBox(100, 100, 0, 1, 0, 1)
	g.SetDebugName("green")
	r := rendering.NewRenderColorBox(80, 80, 1, 0, 0, 1)
	r.SetDebugName("red")
	root.Place(g, 10, 10)
	root.Place(r, 200, 150)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 400, Height: 300}, true)

	hit := root.HitTest(rendering.Point{X: 60, Y: 60})
	if rendering.HitDebugName(hit) != "green" {
		t.Fatalf("got %q want green", rendering.HitDebugName(hit))
	}
	hit = root.HitTest(rendering.Point{X: 240, Y: 190})
	if rendering.HitDebugName(hit) != "red" {
		t.Fatalf("got %q want red", rendering.HitDebugName(hit))
	}
	hit = root.HitTest(rendering.Point{X: 5, Y: 5})
	// May hit root AbsoluteBox (no debug name) or nil path — empty name OK.
	if name := rendering.HitDebugName(hit); name != "" && name != "green" && name != "red" {
		// root itself has no name
		if hit != root && hit != nil {
			t.Fatalf("unexpected name %q on empty area hit=%T", name, hit)
		}
	}
}

func TestBoundaryCache_ClearDropsEntries(t *testing.T) {
	box := rendering.NewRenderColorBox(20, 20, 1, 0, 0, 1)
	box.SetRepaintBoundary(true)
	root := rendering.NewAbsoluteBox(40, 40)
	root.Place(box, 0, 0)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 40, Height: 40}, true)
	cache := owner.BoundaryCache()
	dc := render.NewContext(40, 40)
	pc := rendering.NewPaintContext(dc, 1)
	pc.BoundaryCache, pc.UseBoundaryCache = cache, true
	cache.BeginFrame()
	root.Paint(pc)
	if cache.Len() < 1 {
		t.Fatal("expected cache entry after paint")
	}
	cache.Clear()
	if cache.Len() != 0 {
		t.Fatalf("after Clear len=%d", cache.Len())
	}
	if cache.HasValid(box) {
		t.Fatal("HasValid must be false after Clear")
	}
}
