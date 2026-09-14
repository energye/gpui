package border_beam_test

import (
	"testing"

	"github.com/energye/gpui/render"
	borderbeam "github.com/energye/gpui/ui/kit/border-beam"
	"github.com/energye/gpui/ui/rendering"
)

// Dirty locality: ticking one beam repaints only its own layer.
// Template: ui/kit/icon/dirty_test.go (root + self + 20 statics + sibling).
func TestBorderBeam_DirtyLocality_PRD_BB02(t *testing.T) {
	const vp = 200.0
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vp, vp

	me := borderbeam.NewBorderBeam(boxChild(60, 40))
	other := borderbeam.NewBorderBeam(boxChild(60, 40))
	root.AddChild(me.Node())
	for i := 0; i < 20; i++ {
		root.AddChild(rendering.NewRenderColorBox(4, 4, 0.3, 0.3, 0.3, 1))
	}
	root.AddChild(other.Node())

	owner := rendering.NewPipelineOwner(root)
	if !owner.FlushLayout(rendering.Size{Width: vp, Height: vp}, true) {
		t.Fatal("initial layout")
	}
	dc := render.NewContext(int(vp), int(vp))
	defer dc.Close()
	dc.BeginFrame()

	var visits int64
	owner.FlushPaint(&rendering.PaintContext{DC: dc, PaintVisits: &visits}, true)
	full := visits
	if full <= 0 {
		t.Fatal("full paint visited nothing")
	}
	layouts0 := owner.LayoutCount

	// Advance one beam only: paint dirt, no layout.
	me.Tick(0.5)

	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{DC: dc, PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint after single beam tick")
	}
	if visits >= full/2 {
		t.Fatalf("PaintVisits=%d too high for single dirty beam (full=%d)", visits, full)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on tick-only dirty: %d -> %d", layouts0, owner.LayoutCount)
	}
}
