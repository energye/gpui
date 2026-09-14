package splitter_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/splitter"
	"github.com/energye/gpui/ui/rendering"
)

// 脏局部：改一条分隔条，只脏它自己，不刷整棵树。
// 钩子是引擎现成的 PaintVisits（ui/rendering/paint_context.go），
// 用法抄 render_matrix_test.go：全量画一遍记个数，脏一个再画，
// 第二次必须远小于第一次，且布局次数不涨。
func TestSplitter_DirtyLocality(t *testing.T) {
	const vp = 400.0
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vp, vp

	me := splitter.NewSplitter(
		splitter.NewSplitterPanel(rendering.NewRenderColorBox(20, 20, 0.9, 0.2, 0.2, 1)),
		splitter.NewSplitterPanel(rendering.NewRenderColorBox(20, 20, 0.2, 0.4, 0.9, 1)),
	)
	me.SetWidth(120)
	me.SetHeight(60)
	other := splitter.NewSplitter(
		splitter.NewSplitterPanel(rendering.NewRenderColorBox(20, 20, 0.2, 0.9, 0.2, 1)),
		splitter.NewSplitterPanel(rendering.NewRenderColorBox(20, 20, 0.9, 0.9, 0.2, 1)),
	)
	other.SetWidth(120)
	other.SetHeight(60)
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

	// 只改一条分隔条的悬停：只标画脏，不碰布局。
	me.SetBarHover(0, true)

	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{DC: dc, PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint after single bar dirty")
	}
	if visits >= full/2 {
		t.Fatalf("PaintVisits=%d too high for single dirty bar (full=%d)", visits, full)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on hover-only dirty: %d -> %d", layouts0, owner.LayoutCount)
	}
}
