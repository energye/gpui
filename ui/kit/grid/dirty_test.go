package grid_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit/grid"
	"github.com/energye/gpui/ui/rendering"
)

// Dirty locality: dirty one row, siblings and statics stay clean.
// Full paint counts visits, single-row dirty repaints far fewer,
// and paint-only dirty never runs layout.
func TestGrid_DirtyLocality(t *testing.T) {
	const vp = 1200.0
	mkRow := func() *grid.Row {
		a := grid.NewCol(rendering.NewRenderColorBox(100, 20, 0.2, 0.3, 0.4, 1))
		b := grid.NewCol(rendering.NewRenderColorBox(100, 20, 0.4, 0.3, 0.2, 1))
		a.SetSpan(12)
		b.SetSpan(12)
		r := grid.NewRow(a, b)
		r.Layout(rendering.Tight(vp, 200))
		return r
	}
	me := mkRow()
	other := mkRow()
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vp, 400
	root.AddChild(me.Node())
	for i := 0; i < 20; i++ {
		root.AddChild(rendering.NewRenderColorBox(4, 4, 0.3, 0.3, 0.3, 1))
	}
	root.AddChild(other.Node())

	owner := rendering.NewPipelineOwner(root)
	if !owner.FlushLayout(rendering.Size{Width: vp, Height: 400}, true) {
		t.Fatal("initial layout")
	}
	var visits int64
	owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, true)
	full := visits
	if full <= 0 {
		t.Fatal("full paint visited nothing")
	}
	layouts0 := owner.LayoutCount

	me.Node().MarkNeedsPaint()
	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint after single row dirty")
	}
	if visits >= full/2 {
		t.Fatalf("PaintVisits=%d too high for single dirty row (full=%d)", visits, full)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on paint-only dirty: %d -> %d", layouts0, owner.LayoutCount)
	}
}
