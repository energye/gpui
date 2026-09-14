package masonry_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit/masonry"
	"github.com/energye/gpui/ui/rendering"
)

// Dirty locality: change one masonry (paint-only), prove only its path repaints.
// Template copied from ui/kit/icon/dirty_test.go: full paint count, dirty one,
// partial must be far smaller with no layout work.
func TestMasonry_PRD_MAS22_DirtyLocality(t *testing.T) {
	const vp = 240.0
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vp, vp

	me := masonry.NewMasonry(
		masonry.MasonryItem{Key: "a", Column: -1, Height: 20, Content: rendering.NewRenderColorBox(40, 20, 0.3, 0.4, 0.8, 1)},
		masonry.MasonryItem{Key: "b", Column: -1, Height: 20, Content: rendering.NewRenderColorBox(40, 20, 0.3, 0.4, 0.8, 1)},
	)
	me.SetColumns(2)
	other := masonry.NewMasonry(
		masonry.MasonryItem{Key: "c", Column: -1, Height: 20, Content: rendering.NewRenderColorBox(40, 20, 0.8, 0.3, 0.3, 1)},
	)
	other.SetColumns(1)
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

	// Paint-only change on the masonry: naming must not touch layout.
	me.SetAriaLabel("masonry-locality")

	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint after single masonry dirty")
	}
	if visits >= full/2 {
		t.Fatalf("PaintVisits=%d too high for single dirty masonry (full=%d)", visits, full)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on aria-only dirty: %d -> %d", layouts0, owner.LayoutCount)
	}
}
