// scene.go paints the accelcontract showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_accelcontract — GPUAccelerator 契约直驱 · 能力面/裁剪/掩码/文本加速/CoverageFiller"

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
	colSDF   = [3]float64{0.30, 0.70, 0.90} // SDF 形状色
	colClip  = [3]float64{0.90, 0.60, 0.25} // 剪裁块色
	colMask  = [3]float64{0.40, 0.80, 0.50} // 掩码块色
	colText  = [3]float64{0.92, 0.92, 0.95} // 文本色（近白，掩码字形密度探针用）
)

const ds = 120.0 / 320.0 // D 卡缩略比例

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A 核心契约: CanAccelerate/FillShape/Flush",
		"B 可选能力面: Direct/Adapter/Compute/MSAA/Stats",
		"C 硬件剪裁+掩码: Scissor/RRect SDF/MaskTexture",
		"D 文本加速+CoverageFiller 注入",
		"probe: 接口读回+腿读回",
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
	wrkit.PaintDirectCardFrame(dc, 0, "A 核心契约 · Name/CanAccelerate/FillShape/StrokeShape/Flush", faceCard)
	thumbImg(dc, imgs, x, y, 0, "sdf", "FillShape SDF")
	thumbImg(dc, imgs, x, y, 128, "sdfstroke", "StrokeShape")
	wrkit.DirectDrawText(dc, "CanAccelerate 八 op 读回 · Flush 幂等", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B 可选能力面 · 逐接口类型断言读回", faceCard)
	thumbImg(dc, imgs, x, y, 0, "caps", "能力读回")
	wrkit.DirectDrawText(dc, "Direct/Adapter/Compute/Pipeline/MSAA/Frame/SceneStats", x+16, y+146, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "真设备各值如实登记，不判死", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C 硬件剪裁+掩码 · Scissor / RRect SDF / MaskTexture", faceCard)
	thumbImg(dc, imgs, x, y, 0, "scissor", "SetClipRect")
	thumbImg(dc, imgs, x, y, 128, "rrectclip", "SetClipRRect")
	thumbImg(dc, imgs, x, y, 256, "masktex", "SetMaskTexture")
	wrkit.DirectDrawText(dc, "块外被剪 · 圆角剪对照 · 半掩码 alpha 减半", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D 文本加速 · DrawGlyphMaskText/Aliased/Shaped · CoverageFiller", faceCard)
	if img := imgs["glyphmask"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, "真 face 直调三口读回字形像素", x+16, y+248, faceSmall, wrkit.DColTextDim)
	thumbImg(dc, imgs, x, y, 296-16, "coverage", "FillCoverage")
	wrkit.DirectDrawText(dc, "注入迷你 filler 覆盖值对照", x+296, y+wrkit.DCardH-28, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *acLeg) {
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
