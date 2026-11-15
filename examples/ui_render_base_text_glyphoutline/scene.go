// scene.go paints the text_glyphoutline showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_text_glyphoutline — OutlineExtractor · GlyfContours · Embolden · Filtered/Multi 委托面"

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
	colOutlineA = [3]float64{0.35, 0.75, 0.90}
	colOutlineB = [3]float64{0.90, 0.70, 0.30}
)

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A OutlineExtractor: Extract×3+Embolden",
		"B GlyfContours: 解析点云",
		"C FilteredFace 委托读回",
		"D MultiFace 委托读回",
		"probe: 段数/点数/委托读回",
		"pixel: 缩略不点探（细线）",
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
	wrkit.PaintDirectCardFrame(dc, 0, "A OutlineExtractor · Extract×3 + EmboldenOutline 外扩", faceCard)
	thumbImg(dc, imgs, x, y, 0, "outline", "'A' 轮廓段")
	wrkit.DirectDrawText(dc, "段数读回 · hinted 同段数 · bold 外扩段数保持", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B ParseGlyfContours · NumContours/ContourPoints", faceCard)
	thumbImg(dc, imgs, x, y, 0, "contours", "glyf 点云")
	wrkit.DirectDrawText(dc, "源字体直解析：轮廓数+点数>0（点云缩放显示）", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C FilteredFace · AppendGlyphs/Features/Embolden 委托", faceCard)
	wrkit.DirectDrawText(dc, "过滤面只收 UnicodeRange 内字符——委托读回=源 face 值", x+16, y+60, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "AppendGlyphs n=2 · Features/Embolden 同源值", x+16, y+90, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "（纯契约读回卡，无像素画面）", x+16, y+wrkit.DCardH-40, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D MultiFace · AppendGlyphs/Features/WithWeight/Embolden 委托", faceCard)
	wrkit.DirectDrawText(dc, "多 face 链：单 face 构造——委托读回=源 face 值", x+16, y+60, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "WithWeight 返回同指针（链式）· Embolden 真值登记", x+16, y+90, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "（纯契约读回卡，无像素画面）", x+16, y+wrkit.DCardH-40, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *goLeg) {
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
