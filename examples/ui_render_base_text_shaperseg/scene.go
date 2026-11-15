// scene.go paints the text_shaperseg showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_text_shaperseg — HbShaper/OwnShaper 生产对齐 · 分段 · HitTest 簇"

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
		"A 双 shaper: Hb/Own GID+X 一致",
		"B 分段: 混排/RTL/方向标记",
		"C HitTest: 簇命中/边界钳位",
		"D 实画佐证",
		"probe: 序列一致+腿读回",
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
	wrkit.PaintDirectCardFrame(dc, 0, "A HbShaper/OwnShaper 生产对齐", faceCard)
	wrkit.DirectDrawText(dc, "同文本同 face 逐字形 GID/X 一致 · ClearCache/RemoveSource", x+16, y+60, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "（纯契约读回卡）", x+16, y+wrkit.DCardH-40, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B SegmentText/RTL/BuiltinSegmenter 方向", faceCard)
	wrkit.DirectDrawText(dc, "ab中文cd→Latin/Han 分段 · RTL Hebrew · WithDirection 标记", x+16, y+60, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "（纯契约读回卡）", x+16, y+wrkit.DCardH-40, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C HitTestCluster(Edge) 边界语义 + Quantize", faceCard)
	wrkit.DirectDrawText(dc, "中点→第 4/5 簇 · 负 x→0 · 超出→钳位尾簇 · 10.75→(10,3)", x+16, y+60, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "（纯契约读回卡）", x+16, y+wrkit.DCardH-40, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D 实画佐证", faceCard)
	if img := imgs["seg"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, "真 face 实画", x+16, y+248, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *ssLeg) {
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
