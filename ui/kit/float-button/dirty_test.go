package float_button_test

import (
	"testing"

	"github.com/energye/gpui/render"
	float_button "github.com/energye/gpui/ui/kit/float-button"
	"github.com/energye/gpui/ui/rendering"
)

// Paint locality: dirty one button, only it repaints (FB-06 edge evidence).
// Hook is the engine PaintVisits counter; pattern copies icon/dirty_test.go:
// full paint counts visits, single paint-only dirty repaints far fewer,
// and Layout stays flat.
func TestFloatButton_PRD_FB06_DirtyLocality(t *testing.T) {
	const vp = 200.0
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vp, vp

	me := float_button.NewFloatButton()
	me.SetType(float_button.ButtonTypePrimary)
	other := float_button.NewFloatButton()
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

	// Paint-only dirty: hover recolors, edge stays 40x40, no relayout.
	me.SetHover(true)

	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{DC: dc, PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint after single button dirty")
	}
	if visits >= full/2 {
		t.Fatalf("PaintVisits=%d too high for single dirty button (full=%d)", visits, full)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on hover-only dirty: %d -> %d", layouts0, owner.LayoutCount)
	}
}
