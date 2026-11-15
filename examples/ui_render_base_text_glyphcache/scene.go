// scene.go paints the text_glyphcache showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_text_glyphcache — GlyphCache/SubpixelCache 缓存统计 · GlyphMaskAtlas 页账本"

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
		"A GlyphCache: 命中/Stats/Maintain",
		"B Subpixel: 档位 4/10/None+槽位缓存",
		"C Atlas: GetOrRasterize 快路径+脏页",
		"D Atlas 台账: Entry/Memory/Page/PutLCD",
		"probe: 缓存读回+腿读回",
		"pixel: 缩略不点探",
		"golden: 顶栏+图例+四卡 逐位",
		"gate: -auto-only / RUN_SECONDS",
	}
	return func(dc *render.Context) {
		wrkit.PaintDirectChrome(dc, winTitle, lines, faceTitle, faceLeg)
	}
}

func thumbImg(dc *render.Context, imgs map[string]image.Image, x, y float64, name, label string) {
	if img := imgs[name]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, label, x+16, y+248, faceSmall, wrkit.DColTextDim)
}

func paintCardA(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(0)
	wrkit.PaintDirectCardFrame(dc, 0, "A GlyphCache · GetOrCreate 同指针/Stats/Tick 语义", faceCard)
	wrkit.DirectDrawText(dc, "命中同指针 · hit/miss/evict/ins 计数 · Maintain 无错", x+16, y+60, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "（纯契约读回卡）", x+16, y+wrkit.DCardH-40, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B SubpixelCache · 档位语义+槽位缓存+SetConfig", faceCard)
	wrkit.DirectDrawText(dc, "Subpixel4=×4 / Subpixel10=×10 / None 禁用", x+16, y+60, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "槽位同指针命中 · SetConfig/Maintain 无错", x+16, y+90, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "（纯契约读回卡）", x+16, y+wrkit.DCardH-40, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C GlyphMaskAtlas · GetOrRasterize 缓存快路径+脏页", faceCard)
	wrkit.DirectDrawText(dc, "二次取同 region（不重栅格）· 脏页出现→MarkClean 清空", x+16, y+60, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "EntryCount/MemoryUsage/PageBytes(0)/PageR8Data 读回", x+16, y+90, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "（纯契约读回卡）", x+16, y+wrkit.DCardH-40, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D Atlas 页面缩略 · PutLCD R8 3×宽 · UnderPressure", faceCard)
	thumbImg(dc, imgs, x, y, "atlas", "页0 R8 缩略")
	wrkit.DirectDrawText(dc, "PutLCD region.Width=逻辑宽×3 · 压页=false 如实值", x+16, y+wrkit.DCardH-40, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *gcLeg) {
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
