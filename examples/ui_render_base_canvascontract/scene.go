// scene.go paints the canvascontract showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_canvascontract — render.Canvas 双实现同像素 · live Context vs Recorder 回放"

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
	colSquare  = [3]float64{0.35, 0.70, 0.90} // 状态/形状方块
	colShape2  = [3]float64{0.90, 0.55, 0.20} // 圆/椭圆
	colImageT  = [3]float64{0.45, 0.80, 0.55} // 图像缩略主色
	colLayer   = [3]float64{0.60, 0.35, 0.80} // 层混合块
	colText    = [3]float64{0.92, 0.92, 0.95} // 文本色
)

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A 状态+变换: Push/Pop/Transform 族",
		"B 路径+形状: Fill/Stroke/Preserve/Shapes",
		"C 图像+文本: DrawImage(Ex)/DrawString/Measure",
		"D 层+裁剪+滤镜: PushLayer/Clip/ApplyBlur",
		"probe: 双实现逐位 diff=0",
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
	wrkit.PaintDirectCardFrame(dc, 0, "A 状态+变换 · live（左）/回放（中）/diff（右）", faceCard)
	thumbImg(dc, imgs, x, y, 0, "stateLive", "live 直画")
	thumbImg(dc, imgs, x, y, 128, "stateReplay", "Recorder 回放")
	thumbImg(dc, imgs, x, y, 256, "stateDiff", "两帧 diff")
	wrkit.DirectDrawText(dc, "GetTransform/SetTransform 读回+Push/Pop 隔离", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B 路径+形状 · 同脚本双实现", faceCard)
	thumbImg(dc, imgs, x, y, 0, "pathLive", "live")
	thumbImg(dc, imgs, x, y, 128, "pathReplay", "回放")
	thumbImg(dc, imgs, x, y, 256, "pathDiff", "diff")
	wrkit.DirectDrawText(dc, "Fill/Stroke/Preserve + Draw*Shape 全族", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C 图像+文本 · DrawImage(Ex)/DrawString/Measure", faceCard)
	thumbImg(dc, imgs, x, y, 0, "imgLive", "live")
	thumbImg(dc, imgs, x, y, 128, "imgReplay", "回放")
	thumbImg(dc, imgs, x, y, 256, "imgDiff", "diff")
	wrkit.DirectDrawText(dc, "MeasureString 双实现同值 · TextureRef 登记见 findings", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D 层+裁剪+滤镜 · PushLayer/ClipRect/ApplyBlur", faceCard)
	thumbImg(dc, imgs, x, y, 0, "layerLive", "live")
	thumbImg(dc, imgs, x, y, 128, "layerReplay", "回放")
	thumbImg(dc, imgs, x, y, 256, "layerDiff", "diff")
	wrkit.DirectDrawText(dc, "层混合+矩形剪裁+blur 双实现同像素", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *ccLeg) {
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
