// gfleg.go — the offscreen direct-drive legs for the GPU frame family.
// Every Context.BeginGPUFrame / FlushGPUWithViewDamage(Rects) /
// DiscardPendingGPU / PendingGPUCount / DropGPURenderContext / MemDigCmdBufs /
// LayerPoolStats / ResetLayerPoolStats entry is called directly on offscreen
// views; readbacks assert the observable state. Runs on the GL hub.
package main

import (
	"fmt"
	"image"
	"os"

	"github.com/energye/gpui/render"
)

type gfLeg struct {
	imgs     map[string]image.Image
	dLines   []string // 全量明细（含值，stderr/JSON 用）
	verdicts []string // 判定行（无值——卡 D 画面用，黄金图稳定）
	findings []string
	logicOK  int
	logicBad []string
}

func (l *gfLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	verdict := name
	if ok {
		verdict += ": OK"
		l.logicOK++
	} else {
		line = "FAIL " + line
		verdict += ": FAIL"
		l.logicBad = append(l.logicBad, name)
	}
	l.dLines = append(l.dLines, line)
	l.verdicts = append(l.verdicts, verdict)
	fmt.Fprintf(os.Stderr, "gfleg: %s\n", line)
}

func drawGFV1(dc *render.Context) {
	dc.BeginGPUFrame()
	fillRect(dc, 0, 0, gfW, gfH, gfBase, 1)
	fillRect(dc, 0, 20, gfW, 12, gfBar, 1)
	fillRect(dc, 0, 168, gfW, 12, gfBar, 1)
}

// runGFLegs drives all four cards' offscreen legs. One GL-hub job.
func runGFLegs() *gfLeg {
	leg := &gfLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(gfW, gfH, 1)
	defer dc.Close()

	// ---- Card A: BeginGPUFrame cross-frame redraw (same view across frames) ----
	viewA, relA := dc.CreateOffscreenTexture(gfW, gfH)
	if viewA.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	drawGFV1(dc)
	if err := dc.PresentFrame(viewA, gfW, gfH, nil); err != nil {
		leg.probe("A.v1 flush", false, err.Error())
		return leg
	}
	// reset leg: BeginGPUFrame → draw only the fresh patch → flush
	dc.BeginGPUFrame()
	fillRect(dc, float64(gfRectNew.Min.X), float64(gfRectNew.Min.Y),
		float64(gfRectNew.Dx()), float64(gfRectNew.Dy()), gfFresh, 1)
	if err := dc.PresentFrame(viewA, gfW, gfH, nil); err != nil {
		leg.probe("A.reset flush", false, err.Error())
		return leg
	}
	leg.imgs["reset"] = dc.Image()
	leg.checkAA("A.BeginGPUFrame清帧态", leg.imgs["reset"], gfFresh, true)

	// no-reset leg: redraw v1 (restore), then WITHOUT BeginGPUFrame draw the
	// patch — LoadOpLoad preserves the stale frame outside the patch.
	drawGFV1(dc)
	if err := dc.PresentFrame(viewA, gfW, gfH, nil); err != nil {
		leg.probe("A.restore flush", false, err.Error())
		return leg
	}
	fillRect(dc, float64(gfRectNew.Min.X), float64(gfRectNew.Min.Y),
		float64(gfRectNew.Dx()), float64(gfRectNew.Dy()), gfFresh, 1)
	if err := dc.PresentFrame(viewA, gfW, gfH, nil); err != nil {
		leg.probe("A.noreset flush", false, err.Error())
		return leg
	}
	leg.imgs["noreset"] = dc.Image()
	leg.checkAA("A.不重置保留", leg.imgs["noreset"], gfFresh, false)
	relA()

	// fresh baseline (v2 full content)
	viewB, relB := dc.CreateOffscreenTexture(gfW, gfH)
	drawGFV1(dc)
	fillRect(dc, float64(gfRectNew.Min.X), float64(gfRectNew.Min.Y),
		float64(gfRectNew.Dx()), float64(gfRectNew.Dy()), gfFresh, 1)
	if err := dc.PresentFrame(viewB, gfW, gfH, nil); err != nil {
		leg.probe("A.fresh flush", false, err.Error())
		return leg
	}
	leg.imgs["fresh"] = dc.Image()
	relB()

	// ---- Card B: direct damage flush entries ----
	viewC, relC := dc.CreateOffscreenTexture(gfW, gfH)
	drawGFV1(dc)
	if err := dc.PresentFrame(viewC, gfW, gfH, nil); err != nil {
		leg.probe("B.v1 flush", false, err.Error())
		return leg
	}
	// single-rect damage flush: draw the patch, flush with the damage rect,
	// then mark the view readable via a no-op nil-present.
	fillRect(dc, float64(gfRectNew.Min.X), float64(gfRectNew.Min.Y),
		float64(gfRectNew.Dx()), float64(gfRectNew.Dy()), gfFresh, 1)
	if err := dc.FlushGPUWithViewDamage(viewC, gfW, gfH, gfRectNew); err != nil {
		leg.probe("B.FlushGPUWithViewDamage", false, err.Error())
		return leg
	}
	if err := dc.PresentFrame(viewC, gfW, gfH, nil); err != nil {
		leg.probe("B.mark readback", false, err.Error())
		return leg
	}
	leg.imgs["single"] = dc.Image()
	leg.checkLanded("B.单矩形损伤flush", leg.imgs["single"], gfRectNew)

	// two fresh patch rects, multi-rect damage flush
	rect2 := image.Rect(20, 100, 120, 170)
	fillRect(dc, float64(rect2.Min.X), float64(rect2.Min.Y),
		float64(rect2.Dx()), float64(rect2.Dy()), gfFresh, 1)
	if err := dc.FlushGPUWithViewDamageRects(viewC, gfW, gfH,
		[]image.Rectangle{gfRectNew, rect2}); err != nil {
		leg.probe("B.FlushGPUWithViewDamageRects", false, err.Error())
		return leg
	}
	if err := dc.PresentFrame(viewC, gfW, gfH, nil); err != nil {
		leg.probe("B.mark readback2", false, err.Error())
		return leg
	}
	leg.imgs["multi"] = dc.Image()
	leg.checkLanded("B.双矩形损伤flush", leg.imgs["multi"], rect2)

	// plain full flush baseline
	drawGFV1(dc)
	if err := dc.PresentFrame(viewC, gfW, gfH, nil); err != nil {
		leg.probe("B.plain flush", false, err.Error())
		return leg
	}
	leg.imgs["plain"] = dc.Image()
	leg.checkColorAt("B.无损伤基准", leg.imgs["plain"], 160, 26, gfBar)
	relC()

	// ---- Card C: pending count + drain ----
	viewD, relD := dc.CreateOffscreenTexture(gfW, gfH)
	dc.BeginGPUFrame()
	fillRect(dc, 10, 10, 60, 40, gfFresh, 1)
	fillRect(dc, 80, 10, 60, 40, gfFresh, 1)
	fillRect(dc, 150, 10, 60, 40, gfFresh, 1)
	pending := dc.PendingGPUCount()
	leg.probe("C.PendingGPUCount", pending > 0, fmt.Sprintf("pending=%d", pending))
	dc.DiscardPendingGPU()
	after := dc.PendingGPUCount()
	leg.probe("C.DiscardPendingGPU", after == 0, fmt.Sprintf("after=%d", after))
	// drained frame: only the marker lands — the discarded fills are gone.
	fillRect(dc, 120, 80, 80, 60, gfFresh, 1)
	if err := dc.PresentFrame(viewD, gfW, gfH, nil); err != nil {
		leg.probe("C.drained flush", false, err.Error())
		return leg
	}
	leg.imgs["drained"] = dc.Image()
	leg.checkLanded("C.排水零残留", leg.imgs["drained"], image.Rect(120, 80, 200, 140))
	// the discarded fills must NOT appear
	leg.checkAbsent("C.弃排内容不在", leg.imgs["drained"], image.Rect(10, 10, 70, 50))
	relD()

	// ---- Card D: session drop + MemDig + layer pool stats ----
	prev, usedSurface := dc.MemDigCmdBufs()
	leg.probe("D.MemDigCmdBufs", prev >= 0, fmt.Sprintf("prev=%d usedSurface=%v", prev, usedSurface))

	// release session-bound views before dropping the session
	relB()
	// Drop → 重建探针（干净状态下的会话重建主探针）→ 层池探针（CPU 路径：
	// GPU 会话的层 RT 不经 pixmap 池；CPU 层与 GPU 重建交错时的读回异常见
	// findings——不在门禁内交错）。
	dc.DropGPURenderContext()
	prev0, _ := dc.MemDigCmdBufs()
	leg.probe("D.Drop→会话清", prev0 == -1, fmt.Sprintf("prev=%d", prev0))

	leg.rebuildAfterDrop(dc)

	dc.ResetLayerPoolStats()
	for i := 0; i < 3; i++ {
		// PushLayerIsolated = 真 saveLayer（PushLayer(BlendNormal) 走 F1
		// opacity-group 快路径，不取池——计数不动）。
		dc.PushLayerIsolated(1.0)
		fillRect(dc, 0, 0, 50, 50, gfFresh, 1)
		dc.PopLayer()
	}
	g, p, h, m := dc.LayerPoolStats()
	leg.probe("D.LayerPoolStats", g > 0 && p > 0, fmt.Sprintf("gets=%d puts=%d hits=%d misses=%d", g, p, h, m))
	dc.ResetLayerPoolStats()
	g2, p2, h2, m2 := dc.LayerPoolStats()
	leg.probe("D.ResetLayerPoolStats", g2 == 0 && p2 == 0 && h2 == 0 && m2 == 0,
		fmt.Sprintf("reset→ gets=%d puts=%d hits=%d misses=%d", g2, p2, h2, m2))
	return leg
}

// rebuildAfterDrop pins the session rebuild: next use recreates a clean session.
func (leg *gfLeg) rebuildAfterDrop(dc *render.Context) {
	view, rel := dc.CreateOffscreenTexture(gfW, gfH)
	if view.IsNil() {
		leg.probe("D.重建视图", false, "CreateOffscreenTexture nil")
		return
	}
	defer rel()
	drawGFV1(dc)
	fillRect(dc, float64(gfRectNew.Min.X), float64(gfRectNew.Min.Y),
		float64(gfRectNew.Dx()), float64(gfRectNew.Dy()), gfFresh, 1)
	if err := dc.PresentFrame(view, gfW, gfH, nil); err != nil {
		leg.probe("D.重建重绘", false, err.Error())
		return
	}
	leg.imgs["rebuild"] = dc.Image()
	if dbg := os.Getenv("WR_GF_DBG"); dbg == "1" && leg.imgs["rebuild"] != nil {
		img := leg.imgs["rebuild"]
		for _, pt := range [][2]int{{76, 60}, {160, 100}, {240, 140}, {5, 5}, {160, 26}} {
			got := sampleAt2(img, pt[0], pt[1])
			fmt.Fprintf(os.Stderr, "gfleg dbg rebuild(%d,%d) = (%.0f,%.0f,%.0f)\n",
				pt[0], pt[1], got[0], got[1], got[2])
		}
	}
	leg.checkColorAt("D.重建后重绘", leg.imgs["rebuild"], 240, 140, gfFresh)
	prev2, _ := dc.MemDigCmdBufs()
	leg.probe("D.重建会话恢复", prev2 >= 0, fmt.Sprintf("prev=%d", prev2))
}

// checkAA asserts the load-op toggle semantics at the patch + a stale point.
// reset=true expects the stale point CLEARED (not v1); reset=false expects v1 kept.
func (leg *gfLeg) checkAA(name string, img image.Image, wantAtPatch [3]float64, reset bool) {
	if img == nil {
		leg.probe(name, false, "readback nil")
		return
	}
	bad := ""
	if got := sampleAt2(img, 240, 140); !near2(got, wantAtPatch) {
		bad += fmt.Sprintf(" 补丁点 got=(%.0f,%.0f,%.0f)", got[0], got[1], got[2])
	}
	gotStale := sampleAt2(img, 76, 60)
	if reset {
		if near2(gotStale, gfBase) {
			bad += " 清帧态失效（陈帧仍在）"
		}
	} else if !near2(gotStale, gfBase) {
		bad += fmt.Sprintf(" 保留失效 got=(%.0f,%.0f,%.0f)", gotStale[0], gotStale[1], gotStale[2])
	}
	if bad == "" {
		leg.probe(name, true, "读回全中")
	} else {
		leg.probe(name, false, bad)
	}
}

// checkLanded asserts the patch rect center equals the fresh color.
func (leg *gfLeg) checkLanded(name string, img image.Image, r image.Rectangle) {
	cx := (r.Min.X + r.Max.X) / 2
	cy := (r.Min.Y + r.Max.Y) / 2
	leg.checkColorAt(name, img, cx, cy, gfFresh)
}

// checkColorAt asserts one point equals want (±8/channel).
func (leg *gfLeg) checkColorAt(name string, img image.Image, x, y int, want [3]float64) {
	if img == nil {
		leg.probe(name, false, "readback nil")
		return
	}
	got := sampleAt2(img, x, y)
	if near2(got, want) {
		leg.probe(name, true, fmt.Sprintf("点(%d,%d)=(%.0f,%.0f,%.0f)", x, y, got[0], got[1], got[2]))
	} else {
		leg.probe(name, false, fmt.Sprintf("点(%d,%d) got=(%.0f,%.0f,%.0f) want=(%.0f,%.0f,%.0f)",
			x, y, got[0], got[1], got[2], want[0]*255, want[1]*255, want[2]*255))
	}
}

// checkAbsent asserts a rect center does NOT hold the given content.
func (leg *gfLeg) checkAbsent(name string, img image.Image, r image.Rectangle) {
	if img == nil {
		leg.probe(name, false, "readback nil")
		return
	}
	cx := (r.Min.X + r.Max.X) / 2
	cy := (r.Min.Y + r.Max.Y) / 2
	got := sampleAt2(img, cx, cy)
	if !near2(got, gfFresh) {
		leg.probe(name, true, fmt.Sprintf("弃排区(%d,%d) 无内容", cx, cy))
	} else {
		leg.probe(name, false, fmt.Sprintf("弃排区(%d,%d) 仍见被弃内容", cx, cy))
	}
}

func sampleAt2(img image.Image, x, y int) [3]float64 {
	r, g, b, _ := img.At(x, y).RGBA()
	return [3]float64{float64(r >> 8), float64(g >> 8), float64(b >> 8)}
}

func near2(got, want [3]float64) bool {
	d := func(a, b float64) float64 {
		if a < b {
			return b - a
		}
		return a - b
	}
	return d(got[0], want[0]*255) <= 8 && d(got[1], want[1]*255) <= 8 && d(got[2], want[2]*255) <= 8
}
