// scene.go paints the blendmodes showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_blendmodes — 12 非平凡混合模式逐值 CPU 公式探针 · scene.Blend* 枚举"

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

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A 可分离: Darken/Lighten/Burn/Dodge",
		"B 叠加: Overlay/Hard/Soft/Excl/Diff",
		"C HSL 族: Hue/Saturation/Color/Luminosity",
		"D scene.Blend* 12 值 + Multiply 实画",
		"probe: CPU 公式 ±2 数学探针",
		"pixel: 缩略不点探",
		"golden: 顶栏+图例+四卡 逐位",
		"gate: -auto-only / RUN_SECONDS",
	}
	return func(dc *render.Context) {
		wrkit.PaintDirectChrome(dc, winTitle, lines, faceTitle, faceLeg)
	}
}

func paintCardA(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(0)
	wrkit.PaintDirectCardFrame(dc, 0, "A 可分离族 · Darken/Lighten/ColorBurn/ColorDodge", faceCard)
	wrkit.DirectDrawText(dc, "min/max/(1-d)/s 分档/… 公式逐像素 ±2", x+16, y+60, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B 叠加族 · Overlay/HardLight/SoftLight/Exclusion/Difference", faceCard)
	wrkit.DirectDrawText(dc, "Overlay 对称位/SoftLight W3C D(s) 近似/绝对差", x+16, y+60, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C HSL 非分离族 · Hue/Saturation/Color/Luminosity", faceCard)
	wrkit.DirectDrawText(dc, "setSat/setLum 组合（W3C compositing-1 语义）", x+16, y+60, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D scene.Blend* 12 常量 + Multiply 实画 + SetBlendMode 读回", faceCard)
	if img := imgs["grid"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, "scene Multiply 0.9×0.5=0.45 读回", x+16, y+248, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *bmLeg) {
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
