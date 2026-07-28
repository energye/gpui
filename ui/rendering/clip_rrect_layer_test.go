package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// TestBuildLayerTree_ClipRRectLayer drives the shipped RO→BuildLayerTree path:
// a RenderClipRRect in the render tree must emit a real scene clip_rrect layer
// with expected bounds/radius and nested child structure (not only paint-time
// PushClipRRect or a hand-built LayerBuilder outside RO).
func TestBuildLayerTree_ClipRRectLayer(t *testing.T) {
	scene.ResetLayerIDGen()

	const (
		clipW, clipH = 100.0, 80.0
		radius       = 12.0
		placeX       = 15.0
		placeY       = 25.0
	)

	child := rendering.NewRenderColorBox(60, 50, 1, 0, 0, 1)
	cr := rendering.NewRenderClipRRect(child)
	cr.FixedWidth, cr.FixedHeight = clipW, clipH
	cr.SetRadius(radius)
	cr.SetRepaintBoundary(true)

	root := rendering.NewAbsoluteBox(200, 200)
	root.Place(cr, placeX, placeY)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 200}, true)

	b := rendering.BuildLayerTree(root)

	var (
		foundRRect *scene.ClipRRectLayer
		kinds      []string
		// nesting: after we see clip_rrect, a subsequent picture/offset child proves nesting
		sawChildUnderClip bool
		inClipDepth       int
	)
	var walkDepth func(l scene.Layer, depth int)
	walkDepth = func(l scene.Layer, depth int) {
		if l == nil {
			return
		}
		kinds = append(kinds, l.Kind())
		if l.Kind() == "clip_rrect" {
			if rr, ok := l.(*scene.ClipRRectLayer); ok {
				foundRRect = rr
			}
			inClipDepth = depth
			for _, ch := range l.Children() {
				sawChildUnderClip = true
				walkDepth(ch, depth+1)
			}
			return
		}
		for _, ch := range l.Children() {
			walkDepth(ch, depth+1)
		}
	}
	walkDepth(b.Root(), 0)

	if foundRRect == nil {
		t.Fatalf("expected clip_rrect in layer tree from RenderClipRRect RO, kinds=%v", kinds)
	}
	if foundRRect.Kind() != "clip_rrect" {
		t.Fatalf("kind=%q want clip_rrect", foundRRect.Kind())
	}
	if foundRRect.X != 0 || foundRRect.Y != 0 {
		t.Fatalf("local clip origin = (%v,%v) want (0,0) (offset lives on boundary/offset parent)", foundRRect.X, foundRRect.Y)
	}
	if foundRRect.W != clipW || foundRRect.H != clipH {
		t.Fatalf("clip size = %vx%v want %vx%v", foundRRect.W, foundRRect.H, clipW, clipH)
	}
	if foundRRect.Radius != radius {
		t.Fatalf("radius=%v want %v", foundRRect.Radius, radius)
	}
	if !sawChildUnderClip {
		t.Fatal("clip_rrect must nest children (color/picture/offset under it)")
	}
	if inClipDepth < 1 {
		t.Fatalf("clip_rrect depth=%d; expected nested under root/boundary", inClipDepth)
	}

	// Boundary at RO offset should exist for repaint-boundary ClipRRect.
	var foundBoundary bool
	scene.Walk(b.Root(), func(l scene.Layer) {
		if bl, ok := l.(*scene.BoundaryLayer); ok {
			if bl.DX == placeX && bl.DY == placeY {
				foundBoundary = true
			}
		}
	})
	if !foundBoundary {
		t.Fatalf("expected BoundaryLayer at place (%v,%v) for repaint-boundary ClipRRect; kinds=%v", placeX, placeY, kinds)
	}
}

// TestBuildLayerTree_ClipRRectLayer_NonBoundary still emits clip_rrect when the
// RO is not a repaint boundary (offset + clip stack).
func TestBuildLayerTree_ClipRRectLayer_NonBoundary(t *testing.T) {
	scene.ResetLayerIDGen()

	child := rendering.NewRenderColorBox(40, 40, 0, 1, 0, 1)
	cr := rendering.NewRenderClipRRect(child)
	cr.FixedWidth, cr.FixedHeight = 48, 48
	cr.SetRadius(8)

	root := rendering.NewRenderBox(cr)
	root.FixedWidth, root.FixedHeight = 100, 100
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)

	b := rendering.BuildLayerTree(root)
	var rr *scene.ClipRRectLayer
	scene.Walk(b.Root(), func(l scene.Layer) {
		if c, ok := l.(*scene.ClipRRectLayer); ok {
			rr = c
		}
	})
	if rr == nil {
		t.Fatal("expected clip_rrect from non-boundary RenderClipRRect")
	}
	if rr.W != 48 || rr.H != 48 || rr.Radius != 8 {
		t.Fatalf("got W=%v H=%v R=%v", rr.W, rr.H, rr.Radius)
	}
	if len(rr.Children()) == 0 {
		t.Fatal("expected nested child under clip_rrect")
	}
}

// TestRenderClipRRect_PaintClipsCorners drives paint through the RO (not only
// bare PaintContext): content outside the rrect arc must stay clear.
func TestRenderClipRRect_PaintClipsCorners(t *testing.T) {
	const W, H = 100, 100
	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	// Oversized red child; clip is 80×80 r=20 so corner void stays white.
	child := rendering.NewRenderColorBox(80, 80, 1, 0, 0, 1)
	cr := rendering.NewRenderClipRRect(child)
	cr.FixedWidth, cr.FixedHeight = 80, 80
	cr.SetRadius(20)

	root := rendering.NewAbsoluteBox(W, H)
	root.Place(cr, 10, 10)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: W, Height: H}, true)

	pc := rendering.NewPaintContext(dc, 1)
	owner.FlushPaint(pc, true)

	img := dc.Image()
	if img == nil {
		t.Fatal("nil image")
	}
	// Center of clip (absolute ~50,50) red.
	crR, crG, crB := sampleRGB(img.At(50, 50))
	if crR < 0xC000 || crG > 0x4000 || crB > 0x4000 {
		t.Fatalf("center (50,50)=#%04x%04x%04x want red", crR, crG, crB)
	}
	// Absolute clip origin (10,10); pixel (11,11) is in r=20 corner void.
	tr, tg, tb := sampleRGB(img.At(11, 11))
	if tg < 0xA000 || tb < 0xA000 {
		t.Fatalf("rrect corner void (11,11)=#%04x%04x%04x want near-white", tr, tg, tb)
	}
}

// TestRenderClipRRect_SetRadiusPaintOnly ensures radius change is paint-dirty only.
func TestRenderClipRRect_SetRadiusPaintOnly(t *testing.T) {
	child := rendering.NewRenderColorBox(40, 40, 0.2, 0.6, 0.9, 1)
	cr := rendering.NewRenderClipRRect(child)
	cr.FixedWidth, cr.FixedHeight = 40, 40
	root := rendering.NewAbsoluteBox(100, 100)
	root.Place(cr, 0, 0)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)
	var v int64
	owner.FlushPaint(&rendering.PaintContext{PaintVisits: &v}, true)

	cr.SetRadius(10)
	if cr.NeedsLayout() {
		t.Fatal("SetRadius should be paint-only")
	}
	if !cr.NeedsPaint() {
		t.Fatal("SetRadius should mark needs paint")
	}
}
