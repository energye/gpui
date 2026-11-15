// scene.go paints the svgfilter showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_svgfilter — Apply* 七口直调 · turbulence/lighting/convolve/dilate/erode/composite/colorfilter"

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
	colTurbNoise = [3]float64{0.45, 0.45, 0.55} // turbulence 灰噪（容差宽）
	colBase      = [3]float64{0.85, 0.35, 0.35} // 滤镜源色块
)

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A Turbulence: 生成器（seed 双跑一致）",
		"B Convolve: identity 核不改画面",
		"C Lighting: diffuse/specular",
		"D Dilate/Erode/Composite/ColorFilter",
		"probe: 数值读回+腿读回",
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

func paintCardA(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(0)
	wrkit.PaintDirectCardFrame(dc, 0, "A ApplySVGTurbulence · 生成器 · seed 确定性", faceCard)
	thumbImg(dc, imgs, x, y, 0, "turb", "fractalNoise")
	thumbImg(dc, imgs, x, y, 128, "turb2", "同 seed 二跑")
	thumbImg(dc, imgs, x, y, 256, "turb3", "turbulence 型")
	wrkit.DirectDrawText(dc, "同 seed 两缓冲逐位一致 · Offset 参数平移噪场", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B ApplySVGConvolveMatrix · identity 核不改画面", faceCard)
	thumbImg(dc, imgs, x, y, 0, "convid", "identity 核")
	thumbImg(dc, imgs, x, y, 128, "convbox", "box 模糊核")
	thumbImg(dc, imgs, x, y, 256, "convedge", "EdgeMode 三态")
	wrkit.DirectDrawText(dc, "identity 前后逐位 diff=0 · box 核红块变柔和", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C ApplySVGLighting · diffuse/specular × 光源", faceCard)
	thumbImg(dc, imgs, x, y, 0, "lightdiff", "diffuse distant")
	thumbImg(dc, imgs, x, y, 128, "lightspec", "specular point")
	wrkit.DirectDrawText(dc, "height map=alpha · diffuse 不透明 specular alpha=max(RGB)", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D Dilate/Erode · CompositeArithmetic · ColorFilter", faceCard)
	thumbImg(dc, imgs, x, y, 0, "morph", "dilate rx≠ry")
	thumbImg(dc, imgs, x, y, 128, "composite", "composite k1..k4")
	thumbImg(dc, imgs, x, y, 256, "colorfilter", "反转滤镜")
	wrkit.DirectDrawText(dc, "dilate 胀/erode 缩 · composite 公式逐点 · 反转=1-x", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *sfLeg) {
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
