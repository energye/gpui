package flex_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit/flex"
	"github.com/energye/gpui/ui/rendering"
)

// Dirty locality: change one flex (paint-only), prove only its path repaints.
// Template copied from ui/kit/icon/dirty_test.go: full paint count, dirty one,
// partial must be far smaller with no layout work.
func TestFlex_PRD_FLX22_DirtyLocality(t *testing.T) {
	const vp = 240.0
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vp, vp

	me := flex.NewFlex(
		rendering.NewRenderColorBox(40, 20, 0.3, 0.4, 0.8, 1),
		rendering.NewRenderColorBox(40, 20, 0.3, 0.4, 0.8, 1),
	)
	other := flex.NewFlex(
		rendering.NewRenderColorBox(40, 20, 0.8, 0.3, 0.3, 1),
	)
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

	// Paint-only change on the flex: naming must not touch layout.
	me.SetAriaLabel("flex-locality")

	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint after single flex dirty")
	}
	if visits >= full/2 {
		t.Fatalf("PaintVisits=%d too high for single dirty flex (full=%d)", visits, full)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on aria-only dirty: %d -> %d", layouts0, owner.LayoutCount)
	}
}
