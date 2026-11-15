// scene.go paints the scene_sceneenc showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_scene_sceneenc — Encoding.Encode×13 + Iterator 回读 · 三池语义 · 值类型"

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
		"A Encode×13: Transform/Path/Fill/Stroke/Clip",
		"B 三池: Encoding/Scene/Path Get/Put",
		"C 值类型: Affine/EmptyRect/UnionPoint",
		"D 编码场景实画",
		"probe: Iterator 计数+腿读回",
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
	wrkit.PaintDirectCardFrame(dc, 0, "A Encode×13 直调 + Iterator 计数回读", faceCard)
	wrkit.DirectDrawText(dc, "11 条编码 Iterator 回读 11 · Reset 复位", x+16, y+60, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B EncodingPool/ScenePool/PathPool/DefaultPool", faceCard)
	wrkit.DirectDrawText(dc, "Get/Put/Warmup 语义 · GetEncoding 便捷口", x+16, y+60, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C AffineFromMatrix/NewAffine/EmptyRect/UnionPoint/DefaultStrokeStyle", faceCard)
	wrkit.DirectDrawText(dc, "平移项映射 C/F · 空矩形反向界 · stroke 默认 w=1 miter=10", x+16, y+60, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D 编码场景 Renderer 实画", faceCard)
	if img := imgs["enc"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, "FillRect 实画读回", x+16, y+248, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *seLeg) {
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
