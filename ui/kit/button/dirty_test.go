package button_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/button"
	"github.com/energye/gpui/ui/rendering"
)

// 脏局部：只 hover 一个按钮，只脏它自己，不刷整棵树。
// 钩子是引擎现成的 PaintVisits（ui/rendering/paint_context.go），
// 用法抄 render_matrix_test.go：全量画一遍记个数，脏一个再画，
// 第二次必须远小于第一次，且布局次数不涨。
func TestButton_DirtyLocality(t *testing.T) {
	const vp = 200.0
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vp, vp

	me := button.NewButton("确定")
	other := button.NewButton("取消")
	// 两按钮各占一个重绘边界，中间夹 20 个静态块。
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

	// 只 hover 自己：只标画脏，不碰布局。
	sz := me.LaidOut()
	me.PointerMove(sz.Width/2, sz.Height/2)
	if !me.Hovered() {
		t.Fatal("pointer inside must hover")
	}

	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{DC: dc, PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint after single button dirty")
	}
	// 全树 20+ 静态加两按钮，局部重画访问数必须远小于全量。
	if visits >= full/2 {
		t.Fatalf("PaintVisits=%d too high for single dirty button (full=%d)", visits, full)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on hover-only dirty: %d -> %d", layouts0, owner.LayoutCount)
	}
}
