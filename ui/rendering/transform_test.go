package rendering_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

func TestRenderTransform_SetRotationPaintOnly(t *testing.T) {
	child := rendering.NewRenderColorBox(40, 40, 0.2, 0.6, 0.9, 1)
	tr := rendering.NewRenderTransform(child)
	tr.FixedWidth, tr.FixedHeight = 40, 40
	tr.SetRepaintBoundary(true)
	root := rendering.NewAbsoluteBox(200, 200)
	root.Place(tr, 20, 20)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 200}, true)
	var v int64
	owner.FlushPaint(&rendering.PaintContext{PaintVisits: &v}, true)

	layoutsBefore := 0
	// rotation must not mark layout dirty on transform
	tr.SetRotation(math.Pi / 6)
	if tr.NeedsLayout() {
		t.Fatal("SetRotation should be paint-only")
	}
	_ = layoutsBefore
	v = 0
	owner.FlushPaint(&rendering.PaintContext{PaintVisits: &v}, false)
	if v < 1 {
		t.Fatal("expected paint visits after rotation")
	}
}

func TestBuildLayerTree_TransformLayer(t *testing.T) {
	scene.ResetLayerIDGen()
	child := rendering.NewRenderColorBox(20, 20, 1, 0, 0, 1)
	tr := rendering.NewRenderTransform(child)
	tr.FixedWidth, tr.FixedHeight = 20, 20
	tr.SetRotation(0.3)
	tr.SetScale(1.2, 1.2)
	tr.SetRepaintBoundary(true)
	root := rendering.NewRenderBox(tr)
	root.FixedWidth, root.FixedHeight = 100, 100
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)

	b := rendering.BuildLayerTree(root)
	var kinds []string
	scene.Walk(b.Root(), func(l scene.Layer) {
		kinds = append(kinds, l.Kind())
	})
	found := false
	for _, k := range kinds {
		if k == "transform" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected transform in layer tree, kinds=%v", kinds)
	}
}
