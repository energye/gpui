// scene.go paints the masksrc showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_masksrc — mask construction · AsMask path capture · ambient filter · plane stats"

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

var (
	colMaskA = [3]float64{0.85, 0.30, 0.25}
	colMaskB = [3]float64{0.25, 0.70, 0.85}
	colMaskC = [3]float64{0.30, 0.80, 0.45}
	colAsM   = [3]float64{0.85, 0.55, 0.20}
	colSoft  = [3]float64{0.60, 0.35, 0.80}
)

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A 构造三口: Alpha/Data/Luminance",
		"B AsMask: 当前路径捕获",
		"C 环境滤镜: SetMaskFilter 软边",
		"D MaskPlaneStats 计数链",
		"probe: 构造形状/软边/计数",
		"pixel: 快照探针+抓屏对照",
		"golden: 顶栏+图例+四卡 逐位",
		"gate: -auto-only / RUN_SECONDS",
	}
	return func(dc *render.Context) {
		wrkit.PaintDirectChrome(dc, winTitle, lines, faceTitle, faceLeg)
	}
}

func thumbAt(dc *render.Context, img image.Image, ox, oy float64, label string, lx, ly float64) {
	if img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: ox, Y: oy, DstWidth: 120, DstHeight: 90})
	}
	wrkit.DirectDrawText(dc, label, lx, ly, faceSmall, wrkit.DColTextDim)
}

func paintCardA(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(0)
	wrkit.PaintDirectCardFrame(dc, 0, "A 构造三口 NewMaskFromAlpha/Data/Luminance", faceCard)
	thumbAt(dc, imgs["alpha"], x+16, y+40, "Alpha 通道", x+16, y+146)
	thumbAt(dc, imgs["data"], x+16+128, y+40, "Data 字节", x+16+128, y+146)
	thumbAt(dc, imgs["lum"], x+16+256, y+40, "Luminance 亮度", x+16+256, y+146)
	wrkit.DirectDrawText(dc, "三种来源构造→SetMask→显示即构造形状", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B Context.AsMask 路径捕获", faceCard)
	thumbAt(dc, imgs["asmask"], x+16, y+40, "路径→掩码→显示", x+16, y+146)
	wrkit.DirectDrawText(dc, "路径未 Fill 时 AsMask 栅格化捕获（Fill 后=空掩码陷阱）", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C SetMaskFilter 环境模糊半径", faceCard)
	thumbAt(dc, imgs["soft"], x+16, y+40, "radius=6 软边", x+16, y+146)
	thumbAt(dc, imgs["hard"], x+16+128, y+40, "radius=0 硬边", x+16+128, y+146)
	wrkit.DirectDrawText(dc, "软边沿中间值探针（硬边沿两侧值探针）", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D MaskPlaneStats 计数链", faceCard)
	thumbAt(dc, imgs["stats"], x+16, y+40, "掩码绘制帧", x+16, y+146)
	for i, ln := range dLinesC {
		wrkit.DirectDrawText(dc, ln, x+150, y+52+float64(i)*22, faceSmall, wrkit.DColTextDim)
	}
}

// dLinesC — card D verdict lines (no dynamic values: golden stability).
var dLinesC = []string{
	"首绘: planes/bytes 增",
	"同掩码再绘: hits 增",
	"清掩码: 计数如实",
	"统计读回=真实路径",
}

func paintShowcase(dc *render.Context, leg *msLeg) {
	imgs := map[string]image.Image{}
	if leg != nil {
		imgs = leg.imgs
	}
	if leg != nil {
		dLinesC = leg.verdicts
	}
	paintChrome()(dc)
	paintCardA(dc, imgs)
	paintCardB(dc, imgs)
	paintCardC(dc, imgs)
	paintCardD(dc, imgs)
}
