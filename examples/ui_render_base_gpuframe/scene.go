// scene.go paints the gpuframe showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_gpuframe — BeginGPUFrame · damage flush · pending drain · session drop"

var (
	faceTitle text.Face
	faceLeg   text.Face
	faceCard  text.Face
	faceSmall text.Face
)

func ensureFaces() {
	faceTitle = wrkit.FaceAt(16)
	faceLeg = wrkit.FaceAt(12)
	faceCard = wrkit.FaceAt(13)
	faceSmall = wrkit.FaceAt(11)
}

// saturated probe anchors
var (
	colWave1 = [3]float64{0.86, 0.32, 0.20}
	colWave2 = [3]float64{0.20, 0.55, 0.86}
	colCellA = [3]float64{0.92, 0.80, 0.28}
	colCellB = [3]float64{0.14, 0.42, 0.30}
	colDotA  = [3]float64{0.55, 0.35, 0.75}
	colDotB  = [3]float64{0.30, 0.70, 0.50}
)

// gpuframe offscreen palette (cards A/B readbacks, 320×200 leg space)
const (
	gfW, gfH = 320, 200
)

var (
	gfBase  = [3]float64{0.28, 0.36, 0.52}
	gfBar   = [3]float64{0.14, 0.16, 0.26}
	gfFresh = [3]float64{0.85, 0.45, 0.16}
)

var gfRectNew = image.Rect(180, 100, 300, 180)

func fillRect(dc *render.Context, x, y, w, h float64, c [3]float64, a float64) {
	wrkit.DirectFillRect(dc, x, y, w, h, c, a)
}

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A BeginGPUFrame: 清帧态再画 v2",
		"B 损伤 flush 直调: Damage(Rects)",
		"C PendingGPU: 计数→排水→干净",
		"D Drop 会话: MemDig/层池统计",
		"probe: 计数/读回/会话重建",
		"pixel: 快照探针+抓屏对照",
		"golden: 顶栏+图例+四卡 逐位",
		"gate: -auto-only / RUN_SECONDS",
	}
	return func(dc *render.Context) {
		wrkit.PaintDirectChrome(dc, winTitle, lines, faceTitle, faceLeg)
	}
}

// paintCardA — BeginGPUFrame cross-frame redraw: v1 full + fresh patch, with
// the load-op toggle drawn as two rows (reset vs no-reset) of mini readbacks.
func paintCardA(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(0)
	wrkit.PaintDirectCardFrame(dc, 0, "A BeginGPUFrame 跨帧重绘", faceCard)
	thumb := func(name string, ix float64, label string) {
		img := imgs[name]
		if img == nil {
			return
		}
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16 + ix, Y: y + 40, DstWidth: 120, DstHeight: 75})
		wrkit.DirectDrawText(dc, label, x+16+ix, y+132, faceSmall, wrkit.DColTextDim)
	}
	thumb("reset", 0, "BeginGPUFrame→清帧态")
	thumb("noreset", 132, "不重置→LoadOpLoad 保留")
	thumb("fresh", 264, "v2 新内容基准")
	wrkit.DirectDrawText(dc, "Persistent dc 同视图跨帧：重置后旧帧让位，不重置保留", x+16, y+168, faceSmall, wrkit.DColTextDim)
}

// paintCardB — direct damage flush entries: view content readbacks.
func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B FlushGPUWithViewDamage(Rects) 直调", faceCard)
	thumb := func(name string, ix float64, label string) {
		img := imgs[name]
		if img == nil {
			return
		}
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16 + ix, Y: y + 40, DstWidth: 120, DstHeight: 75})
		wrkit.DirectDrawText(dc, label, x+16+ix, y+132, faceSmall, wrkit.DColTextDim)
	}
	thumb("single", 0, "单矩形损伤 flush")
	thumb("multi", 132, "双矩形损伤 flush")
	thumb("plain", 264, "无损伤 flush 基准")
	wrkit.DirectDrawText(dc, "flush 直调 + PresentFrame(nil) 读回；语义同 W1 损伤族", x+16, y+168, faceSmall, wrkit.DColTextDim)
}

// paintCardC — pending drain: dots grid + a drained cell strip.
func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C PendingGPUCount → DiscardPendingGPU", faceCard)
	for row := 0; row < 7; row++ {
		for col := 0; col < 13; col++ {
			c := colDotA
			if (row+col)%2 == 1 {
				c = colDotB
			}
			dc.SetRGBA(c[0], c[1], c[2], 1)
			dc.DrawRectangle(x+18+float64(col)*32, y+44+float64(row)*26, 6, 6)
			_ = dc.Fill()
		}
	}
	if img := imgs["drained"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 300, Y: y + 40, DstWidth: 120, DstHeight: 75})
		wrkit.DirectDrawText(dc, "排水后干净帧", x+300, y+132, faceSmall, wrkit.DColTextDim)
	}
	wrkit.DirectDrawText(dc, "排队>0 → Discard → 0 → 下帧零残留", x+16, y+cardStatusLabelY(), faceSmall, wrkit.DColTextDim)
}

func cardStatusLabelY() float64 { return wrkit.DCardH - 28 }

// paintCardD — session drop: verdict lines (values go to stderr/JSON, painted
// text stays deterministic for the bitwise golden) + rebuilt readback.
func paintCardD(dc *render.Context, imgs map[string]image.Image, verdicts []string) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D DropGPURenderContext + MemDig + 层池", faceCard)
	if img := imgs["rebuild"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 120, DstHeight: 75})
		wrkit.DirectDrawText(dc, "重建后重绘", x+16, y+132, faceSmall, wrkit.DColTextDim)
	}
	// verdicts 双列排版（15 行溢出卡底）
	const perCol = 8
	for i, ln := range verdicts {
		col := i / perCol
		row := i % perCol
		wrkit.DirectDrawText(dc, ln, x+150+float64(col)*148, y+52+float64(row)*22, faceSmall, wrkit.DColTextDim)
	}
}

// paintShowcase — final deterministic state (golden + pixel probes).
func paintShowcase(dc *render.Context, leg *gfLeg) {
	imgs := map[string]image.Image{}
	verdicts := []string{}
	if leg != nil {
		imgs = leg.imgs
		verdicts = leg.verdicts
	}
	paintChrome()(dc)
	paintCardA(dc, imgs)
	paintCardB(dc, imgs)
	paintCardC(dc, imgs)
	paintCardD(dc, imgs, verdicts)
}
