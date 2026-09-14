package progress_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/progress"
	"github.com/energye/gpui/ui/rendering"
)

// 脏局部：改一个进度条的配色，只脏它自己，不刷整棵树。
// 钩子是引擎现成的 PaintVisits（ui/rendering/paint_context.go），
// 用法抄 render_matrix_test.go：全量画一遍记个数，脏一个再画，
// 第二次必须远小于第一次，且布局次数不涨。
func TestProgress_PRD_PRG02_DirtyLocality(t *testing.T) {
	const vp = 240.0
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vp, vp

	me := progress.NewProgress(50)
	other := progress.NewProgress(20)
	me.Layout(rendering.Loose(vp, vp))
	other.Layout(rendering.Loose(vp, vp))
	// 两进度各占重绘边界内的轨子节点，中间夹 20 个静态块。
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

	// 只改一个进度条的配色：只标画脏，不碰布局。
	me.SetStrokeColor(render.RGBA{R: 1, G: 0, B: 0, A: 1})

	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{DC: dc, PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint after single progress dirty")
	}
	// 全树 20+ 静态加两进度，局部重画访问数必须远小于全量。
	if visits >= full/2 {
		t.Fatalf("PaintVisits=%d too high for single dirty progress (full=%d)", visits, full)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on color-only dirty: %d -> %d", layouts0, owner.LayoutCount)
	}
}
