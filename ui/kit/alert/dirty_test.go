package alert_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/alert"
	"github.com/energye/gpui/ui/rendering"
)

// Template copied from ui/kit/icon/dirty_test.go: root holds the alert,
// 20 static blocks and one sibling. Full paint counts visits, dirtying only
// the alert must repaint far less than half and never relayout.
func TestAlert_DirtyLocality(t *testing.T) {
	const vp = 240.0
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vp, vp

	me := alert.NewAlert("dirty me")
	me.SetShowIcon(true)
	other := alert.NewAlert("sibling")
	other.SetShowIcon(true)
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

	// Paint-only change: type swaps colors, size is unchanged.
	me.SetType(alert.AlertError)

	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{DC: dc, PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint after single alert dirty")
	}
	if visits >= full/2 {
		t.Fatalf("PaintVisits=%d too high for single dirty alert (full=%d)", visits, full)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on color-only dirty: %d -> %d", layouts0, owner.LayoutCount)
	}
}
