// scene.go paints the text_textsys showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_text_textsys — FontResolver 链 · Load/LoadMultiFace · Script/Direction 枚举 · FontFeature 哈希"

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
		"C Resolver 链: SetDefault/SetPaths→Load",
		"C 枚举: Script/Direction/Feature 哈希",
		"D Source 内省: WeightClass/IsVariable",
		"probe: 链读回+腿读回",
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
	wrkit.PaintDirectCardFrame(dc, 0, "A 进程级设置 → DefaultFontResolver 反映", faceCard)
	wrkit.DirectDrawText(dc, "SetDefaultFontPath/SetSystemFontPaths → Candidates 首元素=设置值", x+16, y+60, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "（纯契约读回卡）", x+16, y+wrkit.DCardH-40, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B Load/LoadMultiFace/LoadDefaultFace 全链出真 face", faceCard)
	wrkit.DirectDrawText(dc, "UI+CJK 链（testdata 两字体）出 MultiFace · LoadDefaultFace 名字=设置值", x+16, y+60, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "（纯契约读回卡）", x+16, y+wrkit.DCardH-40, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C Script/Direction 枚举 · FontFeature 哈希顺序无关", faceCard)
	wrkit.DirectDrawText(dc, "Latin 简单 / Han 复杂整形 / LTR 横 TTB 竖 / tnum+liga XOR 哈希", x+16, y+60, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "（纯契约读回卡）", x+16, y+wrkit.DCardH-40, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D resolver 实画佐证 · Source 内省", faceCard)
	if img := imgs["resolver"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, "WeightClass>0 · goregular IsVariable=false · 无轴", x+16, y+248, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *tsLeg) {
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
