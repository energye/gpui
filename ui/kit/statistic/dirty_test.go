package statistic_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit/statistic"
	"github.com/energye/gpui/ui/rendering"
)

// Dirty locality: recolor one statistic, siblings stay clean.
// Full paint visits the whole tree; the partial pass must stay far below
// half of full, and layout must not rerun on a paint-only change.
func TestStatistic_PRD_DirtyLocality(t *testing.T) {
	const vp = 200.0
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vp, vp

	me := statistic.NewStatistic()
	me.SetTitle("me")
	me.SetValue(112893)
	other := statistic.NewStatistic()
	other.SetTitle("other")
	other.SetValue(93)
	root.AddChild(me.Node())
	for i := 0; i < 20; i++ {
		root.AddChild(rendering.NewRenderColorBox(4, 4, 0.3, 0.3, 0.3, 1))
	}
	root.AddChild(other.Node())

	owner := rendering.NewPipelineOwner(root)
	if !owner.FlushLayout(rendering.Size{Width: vp, Height: vp}, true) {
		t.Fatal("initial layout")
	}
	var visits int64
	owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, true)
	full := visits
	if full <= 0 {
		t.Fatal("full paint visited nothing")
	}
	layouts0 := owner.LayoutCount

	me.SetContentStyle(statistic.ColorStyle(me.ContentColor()))

	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint after single statistic dirty")
	}
	if visits >= full/2 {
		t.Fatalf("PaintVisits=%d too high for single dirty statistic (full=%d)", visits, full)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on color-only dirty: %d -> %d", layouts0, owner.LayoutCount)
	}
}
