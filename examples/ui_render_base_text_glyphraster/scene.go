// scene.go paints the text_glyphraster showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_text_glyphraster — GlyphMaskRasterizer 全变体 · R8 掩码读回 · Aliased/LCD/Bold 对照"

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
		"A AA 档: Rasterize/Hinted/HintedBold/FT26",
		"B Aliased/Outline 二值档",
		"C LCD 三口 3× 横向",
		"D MaskResult 几何读回",
		"probe: R8 读回+腿读回",
		"pixel: 缩略不点探（AA）",
		"golden: 顶栏+图例+四卡 逐位",
		"gate: -auto-only / RUN_SECONDS",
	}
	return func(dc *render.Context) {
		wrkit.PaintDirectChrome(dc, winTitle, lines, faceTitle, faceLeg)
	}
}

func maskThumb(dc *render.Context, imgs map[string]image.Image, x, y, ix float64, name, label string) {
	if img := imgs[name]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16 + ix, Y: y + 40, DstWidth: 120, DstHeight: 90})
	}
	wrkit.DirectDrawText(dc, label, x+16+ix, y+146, faceSmall, wrkit.DColTextDim)
}

func paintCardA(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(0)
	wrkit.PaintDirectCardFrame(dc, 0, "A AA 档 · Rasterize/Hinted/HintedBold/HintedFT26", faceCard)
	maskThumb(dc, imgs, x, y, 0, "plain", "Rasterize")
	maskThumb(dc, imgs, x, y, 128, "hinted", "Hinted")
	maskThumb(dc, imgs, x, y, 256, "bold", "HintedBold")
	wrkit.DirectDrawText(dc, "R8 掩码读回：实心区 alpha>0 · Bold 笔画更宽", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B Aliased/Outline 二值档 · OutlineAliased", faceCard)
	maskThumb(dc, imgs, x, y, 0, "aliased", "Aliased")
	maskThumb(dc, imgs, x, y, 128, "aliasedbold", "AliasedBold")
	maskThumb(dc, imgs, x, y, 256, "outline", "Outline/Alias")
	wrkit.DirectDrawText(dc, "二值化：掩码值只有 0 或 255", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C LCD 三口 · RasterizeLCD/LCDBold/LCDOutline", faceCard)
	maskThumb(dc, imgs, x, y, 0, "lcd", "LCD")
	maskThumb(dc, imgs, x, y, 128, "lcdbold", "LCDBold")
	maskThumb(dc, imgs, x, y, 256, "lcdoutline", "LCDOutline")
	wrkit.DirectDrawText(dc, "3× 横向超采样：宽=3×逻辑宽（读回）", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D GlyphMaskResult 几何 · Width/Height/Bearing/Advance", faceCard)
	maskThumb(dc, imgs, x, y, 0, "geom", "几何读回")
	wrkit.DirectDrawText(dc, "掩码尺寸与 gid 字号匹配 · Bearing/Advance 一致性", x+16, y+146, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *grLeg) {
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
