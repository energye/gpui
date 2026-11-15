// scene.go paints the scene_sceneshapes showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_scene_sceneshapes — scene 14 构造 × ToPath/BuildPathInto/Bounds + Renderer 实画"

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
		"A 基础形: Rect/RRect/Circle/Ellipse/Line",
		"B 多边形: Polygon/Regular/Star/Arc/Pie",
		"C Composite/Transform 组合",
		"D scene.Renderer CPU 三形状实画",
		"probe: ToPath verb+Bounds+腿读回",
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
	wrkit.PaintDirectCardFrame(dc, 0, "A 基础形 6 构造 × ToPath/BuildPathInto", faceCard)
	wrkit.DirectDrawText(dc, "verb 数下限+Bounds 非空读回 · pooled 路径=ToPath", x+16, y+60, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B 多边形族 5 构造", faceCard)
	wrkit.DirectDrawText(dc, "Polygon/RegularPolygon(6 边)/Star(5 角 11 verb)/Arc/Pie", x+16, y+60, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C CompositeShape/TransformShape 组合", faceCard)
	wrkit.DirectDrawText(dc, "拼接 verb 累加 · 变换 bounds 放大（40×2=80）", x+16, y+60, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D scene.Renderer CPU 渲染（WithWorkers(1)）", faceCard)
	if img := imgs["shapes"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, "Fill 圆+Fill 圆角矩形+Stroke 星形 实画", x+16, y+248, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *ssLeg2) {
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
