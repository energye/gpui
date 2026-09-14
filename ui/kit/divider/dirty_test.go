package divider_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/divider"
	"github.com/energye/gpui/ui/rendering"
)

// 脏局部：改一条分割线，只脏它自己，不刷整棵树。
// 钩子是引擎现成的 PaintVisits（ui/rendering/paint_context.go），
// 用法抄 render_matrix_test.go：全量画一遍记个数，脏一个再画，
// 第二次必须远小于第一次，且布局次数不涨。
func TestDivider_PRD_DIV13_DirtyLocality(t *testing.T) {
	const vp = 200.0
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vp, vp

	me := divider.NewDivider()
	me.Layout(rendering.Loose(vp, 40))
	other := divider.NewDivider()
	other.Layout(rendering.Loose(vp, 40))
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

	// 只改线色：只标画脏，不碰布局。
	me.SetStyle(divider.Style{Border: render.RGBA{R: 1, G: 0, B: 0, A: 1}})

	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{DC: dc, PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint after single divider dirty")
	}
	if visits >= full/2 {
		t.Fatalf("PaintVisits=%d too high for single dirty divider (full=%d)", visits, full)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on color-only dirty: %d -> %d", layouts0, owner.LayoutCount)
	}
}
