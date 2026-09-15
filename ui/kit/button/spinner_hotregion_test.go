package button_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/button"
	"github.com/energye/gpui/ui/rendering"
)

// 转圈热区：稳态转一格只脏转圈小层，按钮底边字保持干净，整窗只重画小块。
// 转速仍跟显示器走（Tick 每帧推进相位），省的是每帧重画整颗按钮的量。
func TestButton_SpinnerHotRegion(t *testing.T) {
	const vp = 200.0
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vp, vp

	spinner := button.NewButton("Loading")
	spinner.SetType(button.ButtonPrimary)
	other := button.NewButton("取消")
	root.AddChild(spinner.Node())
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

	// 进入加载态（过渡帧整颗与转圈都脏一次），再全量画一遍落稳态。
	spinner.SetLoading(true)
	owner.FlushPaint(&rendering.PaintContext{DC: dc}, true)

	// 稳态转一格：按钮自己必须保持干净，只有子树里转圈那层脏。
	spinner.Tick(1.0 / 60)
	if spinner.Node().NeedsPaint() {
		t.Fatal("spinner tick must not dirty the button chrome (parent stays clean)")
	}
	if !rendering.SubtreeNeedsPaint(spinner.Node()) {
		t.Fatal("spinner tick must dirty the spinner hot region")
	}

	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{DC: dc, PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint after spinner tick")
	}
	// 稳态帧只重画转圈小层：访问数必须远小于全量，且不跑布局。
	if visits >= full/2 {
		t.Fatalf("PaintVisits=%d too high for spinner-only frame (full=%d)", visits, full)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on spinner tick: %d -> %d", layouts0, owner.LayoutCount)
	}

	// 关掉加载：转圈层要重录清空，不能留下残影（残留即脏标记丢失）。
	spinner.SetLoading(false)
	if !rendering.SubtreeNeedsPaint(spinner.Node()) {
		t.Fatal("loading-off must dirty the spinner layer so the ring clears")
	}
}
