// scene.go paints the text_glyphrender showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_text_glyphrender — TextRenderer/GlyphRenderer 直调出轮廓 · GlyphRunBuilder 命令面"

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
		"A TextRenderer: ShapeAndRender(At)",
		"B GlyphRenderer: RenderGlyph(s)/Run/Layout",
		"C RenderParams: WithColor/Opacity",
		"D GlyphRunBuilder: Build/Transformed/Clear",
		"probe: 轮廓数+腿读回",
		"pixel: 缩略不点探（细线）",
		"golden: 顶栏+图例+四卡 逐位",
		"gate: -auto-only / RUN_SECONDS",
	}
	return func(dc *render.Context) {
		wrkit.PaintDirectChrome(dc, winTitle, lines, faceTitle, faceLeg)
	}
}

func paintCardA(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(0)
	wrkit.PaintDirectCardFrame(dc, 0, "A TextRenderer · SetDefault* + ShapeAndRender(At)", faceCard)
	wrkit.DirectDrawText(dc, "Render→6 轮廓 · At 42→5 轮廓（轮廓描边实画）", x+16, y+60, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "GlyphRenderer() 内建非 nil", x+16, y+90, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B GlyphRenderer · RenderGlyph/Glyphs/Run/Layout + SetCache", faceCard)
	wrkit.DirectDrawText(dc, "单字形段数>0 · 四口轮廓数一致 · 私有缓存注入", x+16, y+60, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "（契约读回+实画佐证）", x+16, y+90, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C RenderParams · WithColor/WithOpacity 链", faceCard)
	wrkit.DirectDrawText(dc, "不可变链：返回新 params（opacity=0.5 读回）", x+16, y+60, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "（纯契约读回卡）", x+16, y+wrkit.DCardH-40, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D GlyphRunBuilder · Add*/Build/BuildTransformed/Instances/Clear", faceCard)
	wrkit.DirectDrawText(dc, "AddGlyph+ShapedGlyph(s)+ShapedRun→6 实例=6 命令", x+16, y+60, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "BuildTransformed 携带仿射 · Clear 清空", x+16, y+90, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *grlLeg) {
	imgs := map[string]image.Image{}
	if leg != nil {
		imgs = leg.imgs
	}
	paintChrome()(dc)
	paintCardA(dc, imgs)
	paintCardB(dc, imgs)
	paintCardC(dc, imgs)
	paintCardD(dc, imgs)
	_ = imgs
}
