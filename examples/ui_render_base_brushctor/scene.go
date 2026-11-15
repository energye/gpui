// scene.go paints the brushctor showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_brushctor — Solid constructors · standalone gradients · Painter/BrushFieldSeed"

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

// probe anchors
var (
	colHexCard = [3]float64{1.00, 0.50, 0.25} // SolidHex #FF8040
	colFromCol = [3]float64{0.20, 0.70, 0.90} // FromColor(标准库)
	colHSL     = [3]float64{1.00, 0.00, 0.00} // HSL(0,1,0.5)=红
	colH0      = [3]float64{0.90, 0.20, 0.20} // 渐变端色 A
	colH1      = [3]float64{0.20, 0.30, 0.90} // 渐变端色 B
	colStripeA = [3]float64{0.95, 0.85, 0.30} // 条纹亮带
	colAlpha   = [3]float64{0.40, 0.80, 0.55} // WithAlpha 主题色
)

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A Solid 族: RGB/Hex/FromColor/HSL",
		"B 渐变独立口: H/V/Linear/Radial/Sweep/Stripes",
		"C 包级口 vs Context 口同帧对照",
		"D WithAlpha 链 + BrushFieldSeed",
		"probe: ColorAt 数学+读回",
		"pixel: 快照探针+抓屏对照",
		"golden: 顶栏+图例+四卡 逐位",
		"gate: -auto-only / RUN_SECONDS",
	}
	return func(dc *render.Context) {
		wrkit.PaintDirectChrome(dc, winTitle, lines, faceTitle, faceLeg)
	}
}

func thumbImg(dc *render.Context, imgs map[string]image.Image, x, y, ix float64, name, label string) {
	if img := imgs[name]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16 + ix, Y: y + 40, DstWidth: 120, DstHeight: 90})
	}
	wrkit.DirectDrawText(dc, label, x+16+ix, y+146, faceSmall, wrkit.DColTextDim)
}

// paintCardA — solid constructor family.
func paintCardA(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(0)
	wrkit.PaintDirectCardFrame(dc, 0, "A SolidRGB/RGBA/Hex/FromColor/HSL + WithAlpha 链", faceCard)
	thumbImg(dc, imgs, x, y, 0, "solid", "五构造色卡")
	thumbImg(dc, imgs, x, y, 128, "alpha", "WithAlpha 三档")
	wrkit.DirectDrawText(dc, "HSL(0,1,0.5)=红 · Opaque/Transparent 读回", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

// paintCardB — standalone gradient constructors.
func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B Horizontal/Vertical/Linear/Radial/Sweep/Stripes", faceCard)
	thumbImg(dc, imgs, x, y, 0, "hgrad", "Horizontal")
	thumbImg(dc, imgs, x, y, 128, "vgrad", "Vertical")
	thumbImg(dc, imgs, x, y, 256, "lgrad", "Linear(斜向)")
	thumbImg(dc, imgs, x, y, 0, "rgrad", "Radial")
	thumbImg(dc, imgs, x, y, 128, "sweep", "Sweep")
	thumbImg(dc, imgs, x, y, 256, "stripes", "Stripes(斜纹)")
	wrkit.DirectDrawText(dc, "ColorAt 逐点数学探针全中（中点半混/r/2/周期）", x+16, y+330, faceSmall, wrkit.DColTextDim)
}

// paintCardC — package-level brush vs Context.SetRGBA parity.
func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C 包级口 vs Context 口 逐位一致", faceCard)
	thumbImg(dc, imgs, x, y, 0, "parityBrush", "SetFillBrush 帧")
	thumbImg(dc, imgs, x, y, 128, "paritySolid", "SetRGBA 帧")
	thumbImg(dc, imgs, x, y, 256, "parityDiff", "两帧 diff 图")
	wrkit.DirectDrawText(dc, "同几何两帧读回 diff=0 · PainterFromPaint 缓冲逐元素", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

// paintCardD — BrushFieldSeed semantics.
func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D BrushFieldSeed · GPU field cache key 语义", faceCard)
	if img := imgs["seed"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, "SolidBrush=0 · WithAlpha≠0 且随 alpha 变化互异", x+16, y+248, faceSmall, wrkit.DColTextDim)
	thumbImg(dc, imgs, x, y, 296-16, "alphapaint", "alpha 半色")
	wrkit.DirectDrawText(dc, "BrushWithAlpha(半色) 画面佐证", x+296, y+wrkit.DCardH-28, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *bcLeg) {
	imgs := map[string]image.Image{}
	if leg != nil {
		imgs = leg.imgs
	}
	paintChrome()(dc)
	paintCardA(dc, imgs)
	paintCardB(dc, imgs)
	paintCardC(dc, imgs)
	paintCardD(dc, imgs)
}
