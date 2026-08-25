package rendering_test

import (
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// TestRenderOpacity_PaintCompositesGroup drives the shipped RO paint path:
// a RenderOpacity(0.5) red box over white must blend to a mid red (group
// opacity), not stay fully opaque.
func TestRenderOpacity_PaintCompositesGroup(t *testing.T) {
	dc := render.NewContext(80, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	box := rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1)
	op := rendering.NewRenderOpacity(0.5, box)
	root := rendering.NewAbsoluteBox(80, 80)
	root.Place(op, 20, 20)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 80, Height: 80}, true)
	owner.FlushPaint(rendering.NewPaintContext(dc, 1), true)

	img := dc.Image()
	c := color.RGBAModel.Convert(img.At(40, 40)).(color.RGBA)
	// 0.5 red over white: white contributes R=255, so blended red is (255,128,128).
	if c.R != 255 || c.G != 128 || c.B != 128 {
		t.Fatalf("expected blended red #ff8080 at (40,40), got #%02x%02x%02x", c.R, c.G, c.B)
	}
}

// TestRenderOpacity_ZeroSkipsSubtree: op=0 paints nothing — the region stays
// backdrop-colored and the child records no paint visit.
func TestRenderOpacity_ZeroSkipsSubtree(t *testing.T) {
	dc := render.NewContext(80, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	visits := int64(0)
	box := rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1)
	op := rendering.NewRenderOpacity(0, box)
	root := rendering.NewAbsoluteBox(80, 80)
	root.Place(op, 20, 20)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 80, Height: 80}, true)
	pcZero := rendering.NewPaintContext(dc, 1)
	pcZero.PaintVisits = &visits
	owner.FlushPaint(pcZero, true)

	img := dc.Image()
	c := color.RGBAModel.Convert(img.At(40, 40)).(color.RGBA)
	if c.R != 255 || c.G != 255 || c.B != 255 {
		t.Fatalf("op=0 must leave backdrop untouched, got #%02x%02x%02x", c.R, c.G, c.B)
	}
}

// TestRenderOpacity_SetOpacityPaintOnly: changing opacity dirties paint but
// not layout; a repaint picks up the new value without relayout.
func TestRenderOpacity_SetOpacityPaintOnly(t *testing.T) {
	op := rendering.NewRenderOpacity(1, rendering.NewRenderColorBox(10, 10, 1, 1, 1, 1))
	root := rendering.NewAbsoluteBox(50, 50)
	root.Place(op, 5, 5)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 50, Height: 50}, true)

	op.SetOpacity(0.25)
	if !op.NeedsPaint() {
		t.Fatal("SetOpacity must mark needs-paint")
	}
	if op.NeedsLayout() {
		t.Fatal("SetOpacity must NOT mark needs-layout")
	}
	if got := op.OpacityParams(); got != 0.25 {
		t.Fatalf("OpacityParams=%v want 0.25", got)
	}
}

// TestRenderOpacity_HitTestBlindToOpacity: a faded subtree still receives
// hits (Flutter semantics); outside the laid-out box misses.
func TestRenderOpacity_HitTestBlindToOpacity(t *testing.T) {
	child := rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1)
	op := rendering.NewRenderOpacity(0.2, child)
	root := rendering.NewAbsoluteBox(80, 80)
	root.Place(op, 20, 20)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 80, Height: 80}, true)

	if hit := root.HitTest(rendering.Point{X: 30, Y: 30}); hit == nil {
		t.Fatal("faded subtree must still hit inside its bounds")
	} else if hit != child && hit != op {
		t.Fatalf("hit=%T want child or opacity node", hit)
	}
	if hit := root.HitTest(rendering.Point{X: 5, Y: 5}); hit == nil {
		t.Fatal("point over the root container itself should hit the root")
	} else if hit == child {
		t.Fatalf("faded child must not be hit outside its bounds, got %T", hit)
	}
}

// TestBuildLayerTree_OpacityLayer drives the shipped RO→BuildLayerTree path:
// a RenderOpacity in the render tree must emit a real scene opacity layer with
// the clamped value and nested children (not only paint-time SaveLayer).
func TestBuildLayerTree_OpacityLayer(t *testing.T) {
	scene.ResetLayerIDGen()

	const (
		opVal  = 0.4
		placeX = 12.0
		placeY = 18.0
	)

	child := rendering.NewRenderColorBox(30, 24, 1, 0, 0, 1)
	op := rendering.NewRenderOpacity(opVal, child)
	op.SetRepaintBoundary(true)

	root := rendering.NewAbsoluteBox(120, 120)
	root.Place(op, placeX, placeY)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 120, Height: 120}, true)

	b := rendering.BuildLayerTree(root)

	var (
		foundOp    *scene.OpacityLayer
		foundBound bool
		kinds      []string
	)
	scene.Walk(b.Root(), func(l scene.Layer) {
		kinds = append(kinds, l.Kind())
		if ol, ok := l.(*scene.OpacityLayer); ok && foundOp == nil {
			foundOp = ol
		}
		if bl, ok := l.(*scene.BoundaryLayer); ok {
			if bl.DX == placeX && bl.DY == placeY {
				foundBound = true
			}
		}
	})

	if foundOp == nil {
		t.Fatalf("expected opacity layer from RenderOpacity RO, kinds=%v", kinds)
	}
	if foundOp.Opacity != opVal {
		t.Fatalf("layer opacity=%v want %v", foundOp.Opacity, opVal)
	}
	if len(foundOp.Children()) == 0 {
		t.Fatal("opacity layer must nest children")
	}
	if !foundBound {
		t.Fatalf("expected BoundaryLayer at (%v,%v) for repaint-boundary RenderOpacity; kinds=%v", placeX, placeY, kinds)
	}
}

// TestBuildLayerTree_OpacityLayer_NonBoundary: without repaint-boundary the
// node emits offset+opacity stack; children record pictures under it.
func TestBuildLayerTree_OpacityLayer_NonBoundary(t *testing.T) {
	scene.ResetLayerIDGen()

	child := rendering.NewRenderColorBox(30, 24, 0, 1, 0, 1)
	op := rendering.NewRenderOpacity(0.6, child)
	root := rendering.NewAbsoluteBox(120, 120)
	root.Place(op, 8, 9)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 120, Height: 120}, true)

	b := rendering.BuildLayerTree(root)
	var foundOp *scene.OpacityLayer
	scene.Walk(b.Root(), func(l scene.Layer) {
		if ol, ok := l.(*scene.OpacityLayer); ok && foundOp == nil {
			foundOp = ol
		}
	})
	if foundOp == nil {
		t.Fatal("non-boundary RenderOpacity must still emit an opacity layer")
	}
	if foundOp.Opacity != 0.6 {
		t.Fatalf("opacity=%v want 0.6", foundOp.Opacity)
	}
	sawPicture := false
	scene.Walk(foundOp, func(l scene.Layer) {
		if l.Kind() == "picture" {
			sawPicture = true
		}
	})
	if !sawPicture {
		t.Fatal("child content must be recorded as a picture under the opacity layer")
	}
}

// TestBuildLayerTree_OpacityIdentityOmitted: op=1 must not push an opacity
// layer (builder skips identity) so steady-state layer trees stay unchanged.
func TestBuildLayerTree_OpacityIdentityOmitted(t *testing.T) {
	scene.ResetLayerIDGen()

	op := rendering.NewRenderOpacity(1, rendering.NewRenderColorBox(20, 20, 0, 0, 1, 1))
	root := rendering.NewAbsoluteBox(60, 60)
	root.Place(op, 4, 4)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 60, Height: 60}, true)

	b := rendering.BuildLayerTree(root)
	scene.Walk(b.Root(), func(l scene.Layer) {
		if _, ok := l.(*scene.OpacityLayer); ok {
			t.Fatal("op=1 must omit the opacity layer")
		}
	})
}
