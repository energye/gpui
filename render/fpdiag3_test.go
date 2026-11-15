//go:build !nogpu

package render_test

// fpdiag3 — gpuframe 窗 B 腿（双矩形损伤 flush）诊断：vector fill 走 MSAA
// grouped 路径（RenderFrameGrouped → encodeSubmitSurfaceGrouped），两块损伤
// 矩形都在 damage 内、各自组剪刀内，rect2 中心 (70,135) 应落地 gfFresh。
// 实测 (71,92,133)=gfBase——rect2 内容未落地。本探针逐段复刻 B 腿并打印
// 读回，区分：单矩形是否落地 / 双矩形 union 是否落地 / 双矩形各自落地。

import (
	"fmt"
	"image"
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

func TestFPDiag3_GPUFrameMultiDamage(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 320, 200
	gfBase := [3]float64{0.28, 0.36, 0.52}
	gfBar := [3]float64{0.14, 0.16, 0.26}
	gfFresh := [3]float64{0.85, 0.45, 0.16}
	rectNew := image.Rect(180, 100, 300, 180)
	rect2 := image.Rect(20, 100, 120, 170)

	sample := func(dc *render.Context, label string, pts map[string][2]int) {
		img := dc.Image()
		for n, p := range pts {
			r, g, b, _ := img.At(p[0], p[1]).RGBA()
			fmt.Fprintf(os.Stderr, "fpdiag3 %-24s %s(%d,%d) = (%3d,%3d,%3d)\n",
				label+":", n, p[0], p[1], r>>8, g>>8, b>>8)
		}
	}

	dc := render.NewContextWithScale(w, h, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(w, h)
	if view.IsNil() {
		t.Skip("no offscreen")
	}
	defer rel()

	// v1: base + two bars (y 20-34, y 168-182).
	dc.BeginGPUFrame()
	p1FillRect(dc, 0, 0, w, h, gfBase, 1)
	p1FillRect(dc, 0, 20, w, 12, gfBar, 1)
	p1FillRect(dc, 0, 168, w, 12, gfBar, 1)
	if err := dc.PresentFrame(view, w, h, nil); err != nil {
		t.Fatalf("v1: %v", err)
	}
	sample(dc, "v1", map[string][2]int{"rect2心(70,135)": {70, 135}})

	// leg A reset: BeginGPUFrame + patch + PresentFrame (same view).
	dc.BeginGPUFrame()
	p1FillRect(dc, 180, 100, 120, 80, gfFresh, 1)
	if err := dc.PresentFrame(view, w, h, nil); err != nil {
		t.Fatalf("A.reset: %v", err)
	}
	sample(dc, "A-reset", map[string][2]int{
		"patch(240,140)": {240, 140},
		"陈帧点(76,60)":    {76, 60},
	})

	// leg A restore + noreset.
	dc.BeginGPUFrame()
	p1FillRect(dc, 0, 0, w, h, gfBase, 1)
	p1FillRect(dc, 0, 20, w, 12, gfBar, 1)
	p1FillRect(dc, 0, 168, w, 12, gfBar, 1)
	if err := dc.PresentFrame(view, w, h, nil); err != nil {
		t.Fatalf("A.restore: %v", err)
	}
	p1FillRect(dc, 180, 100, 120, 80, gfFresh, 1)
	if err := dc.PresentFrame(view, w, h, nil); err != nil {
		t.Fatalf("A.noreset: %v", err)
	}
	sample(dc, "A-noreset", map[string][2]int{"patch(240,140)": {240, 140}})

	// leg B single: patch rectNew, flush with single damage.
	p1FillRect(dc, 180, 100, 120, 80, gfFresh, 1)
	if err := dc.FlushGPUWithViewDamage(view, w, h, rectNew); err != nil {
		t.Fatalf("single flush: %v", err)
	}
	if err := dc.PresentFrame(view, w, h, nil); err != nil {
		t.Fatalf("single present: %v", err)
	}
	sample(dc, "B-single", map[string][2]int{"patch(240,140)": {240, 140}})

	// control: what does the VIEW itself hold (bypass pixmap readback)?
	sample(dc, "B-single-again", map[string][2]int{"patch(240,140)": {240, 140}})

	// leg B multi A: second patch rect2, flush with [rectNew, rect2].
	p1FillRect(dc, 20, 100, 100, 70, gfFresh, 1)
	if err := dc.FlushGPUWithViewDamageRects(view, w, h, []image.Rectangle{rectNew, rect2}); err != nil {
		t.Fatalf("multi flush: %v", err)
	}
	if err := dc.PresentFrame(view, w, h, nil); err != nil {
		t.Fatalf("multi present: %v", err)
	}
	sample(dc, "B-multi", map[string][2]int{
		"rect2心(70,135)":  {70, 135},
		"rectNew心(240,140)": {240, 140},
	})

	// leg B multi B: fresh view, same two patches in ONE BeginGPUFrame, multi damage.
	view2, rel2 := dc.CreateOffscreenTexture(w, h)
	defer rel2()
	dc.BeginGPUFrame()
	p1FillRect(dc, 180, 100, 120, 80, gfFresh, 1)
	p1FillRect(dc, 20, 100, 100, 70, gfFresh, 1)
	if err := dc.FlushGPUWithViewDamageRects(view2, w, h, []image.Rectangle{rectNew, rect2}); err != nil {
		t.Fatalf("multi2 flush: %v", err)
	}
	if err := dc.PresentFrame(view2, w, h, nil); err != nil {
		t.Fatalf("multi2 present: %v", err)
	}
	sample(dc, "B-multi2-fresh", map[string][2]int{
		"rect2心(70,135)":  {70, 135},
		"rectNew心(240,140)": {240, 140},
	})

	// leg C replica: fresh viewD, queue 3 fills → discard → marker fill → present.
	view3, rel3 := dc.CreateOffscreenTexture(w, h)
	defer rel3()
	dc.BeginGPUFrame()
	p1FillRect(dc, 10, 10, 60, 40, gfFresh, 1)
	p1FillRect(dc, 80, 10, 60, 40, gfFresh, 1)
	p1FillRect(dc, 150, 10, 60, 40, gfFresh, 1)
	dc.DiscardPendingGPU()
	p1FillRect(dc, 120, 80, 80, 60, gfFresh, 1)
	if err := dc.PresentFrame(view3, w, h, nil); err != nil {
		t.Fatalf("C flush: %v", err)
	}
	sample(dc, "C-drained", map[string][2]int{
		"marker(160,110)": {160, 110},
		"弃排区(40,30)":    {40, 30},
		"弃排区(110,30)":   {110, 30},
		"远角(300,190)":    {300, 190},
	})
}
