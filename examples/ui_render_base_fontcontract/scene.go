// scene.go paints the fontcontract showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_fontcontract — Face/ParsedFont 契约 · FontResolver · FontFeature · ColorFont 能力面"

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
	colFaceText = [3]float64{0.92, 0.92, 0.95} // 真字体画面主色
)

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A Face 12 契约口: With* 缔造读回",
		"B ParsedFont: NumGlyphs/UPM/GlyphAdvance",
		"C FontResolver: SetFile/Candidates/Load",
		"D ColorFont 能力面 + Feature 哈希",
		"probe: 契约读回+腿读回",
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
	wrkit.PaintDirectCardFrame(dc, 0, "A Face 契约 · Metrics/Advance/Glyphs/With* 读回", faceCard)
	thumbImg(dc, imgs, x, y, 0, "face", "契约读回")
	thumbImg(dc, imgs, x, y, 128, "faceopts", "With* 三档")
	wrkit.DirectDrawText(dc, "Language/Features/Hinting/Weight=设置值 · RuneAdvance 快路径", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B ParsedFont 内省 · NumGlyphs/UnitsPerEm/GlyphAdvance", faceCard)
	thumbImg(dc, imgs, x, y, 0, "parsed", "内省读回")
	wrkit.DirectDrawText(dc, "NumGlyphs>0 · UPM 典型 1000/2048 · 与 Face.Glyphs 一致", x+16, y+146, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C FontResolver · SetFontFile/Candidates/Load · SystemFontsByFamily", faceCard)
	thumbImg(dc, imgs, x, y, 0, "resolver", "解析链")
	wrkit.DirectDrawText(dc, "testdata 字体指路→Load 出 Face · Candidates=设置值", x+16, y+146, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D ColorFont 能力面 · NewFontFeature/Hash · 真字体画面佐证", faceCard)
	if img := imgs["facepaint"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, "真 face DrawString · Feature 哈希顺序无关", x+16, y+248, faceSmall, wrkit.DColTextDim)
	thumbImg(dc, imgs, x, y, 296-16, "colorfont", "ColorFont 读回")
	wrkit.DirectDrawText(dc, "非彩色字体→false 如实登记", x+296, y+wrkit.DCardH-28, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *fcLeg) {
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
