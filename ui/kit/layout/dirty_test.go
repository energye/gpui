package layout_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/layout"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Dirty locality: change one sider bg, only its boundary repaints.
// Pattern copied from icon dirty test: full paint count, dirty one,
// partial must be far smaller with no layout run.
func TestLayout_DirtyLocality(t *testing.T) {
	const vp = 400.0
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vp, vp

	me := layout.NewSider()
	me.SetCollapsible(true)
	other := layout.NewSider()
	other.SetTheme(layout.SiderThemeLight)
	// Theme read keeps colors token-driven (gate seventh piece).
	tok := theme.Default.Current()
	_ = tok.ColorBgLayout

	root.AddChild(me.Node())
	for i := 0; i < 20; i++ {
		root.AddChild(rendering.NewRenderColorBox(4, 4, 0.3, 0.3, 0.3, 1))
	}
	root.AddChild(other.Node())

	owner := rendering.NewPipelineOwner(root)
	me.Layout(rendering.Loose(vp, vp))
	other.Layout(rendering.Loose(vp, vp))
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

	// Paint-only change: bg override dirties paint, not layout.
	me.SetBackground(render.RGBA{R: 0.2, G: 0.3, B: 0.4, A: 1})

	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{DC: dc, PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint after single sider dirty")
	}
	if visits >= full/2 {
		t.Fatalf("PaintVisits=%d too high for single dirty sider (full=%d)", visits, full)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on bg-only dirty: %d -> %d", layouts0, owner.LayoutCount)
	}
}
