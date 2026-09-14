package watermark_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/watermark"
	"github.com/energye/gpui/ui/rendering"
)

// 脏局部：改水印配置，只脏水印自己，不刷整棵树。
// 钩子是引擎现成的 PaintVisits（ui/rendering/paint_context.go），
// 用法抄 render_matrix_test.go：全量画一遍记个数，脏一个再画，
// 第二次必须远小于第一次，且布局次数不涨。
func TestWatermark_PRD_WMDirtyLocality(t *testing.T) {
	const vp = 320.0
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vp, vp

	me := watermark.NewWatermark(sizedBox(vp, 120))
	me.SetContent("Ant Design")
	other := watermark.NewWatermark(sizedBox(vp, 120))
	other.SetContent("Other")
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

	// 只改一个水印的旋转：只标画脏，不碰布局。
	me.SetRotate(-15)

	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{DC: dc, PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint after single watermark dirty")
	}
	if visits >= full/2 {
		t.Fatalf("PaintVisits=%d too high for single dirty watermark (full=%d)", visits, full)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on rotate-only dirty: %d -> %d", layouts0, owner.LayoutCount)
	}
}
