// scene.go paints the pixmapops showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_pixmapops — pixel writes · span fills · encode round-trip · genID watcher · ink crop"

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

var (
	colPixA  = [3]float64{0.85, 0.30, 0.20}
	colPixB  = [3]float64{0.20, 0.70, 0.45}
	colSpan  = [3]float64{0.30, 0.45, 0.85}
	colBlend = [3]float64{0.92, 0.78, 0.25}
	colCrop  = [3]float64{0.55, 0.25, 0.75}
)

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A 像素直写: SetPixel/Premul/WritePixels",
		"B Span 填充: FillSpan/Blend 数学",
		"C 编码往返: PNG 逐位/JPEG 容差",
		"D GenID 监视 + 墨迹裁剪",
		"probe: 像素精确/混合数学/计数",
		"pixel: 快照探针+抓屏对照",
		"golden: 顶栏+图例+四卡 逐位",
		"gate: -auto-only / RUN_SECONDS",
	}
	return func(dc *render.Context) {
		wrkit.PaintDirectChrome(dc, winTitle, lines, faceTitle, faceLeg)
	}
}

func thumbAt(dc *render.Context, img image.Image, ox, oy, w, h float64, label string, lx, ly float64) {
	if img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: ox, Y: oy, DstWidth: w, DstHeight: h})
	}
	wrkit.DirectDrawText(dc, label, lx, ly, faceSmall, wrkit.DColTextDim)
}

func paintCardA(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(0)
	wrkit.PaintDirectCardFrame(dc, 0, "A 像素直写 SetPixel/Premul/WritePixels", faceCard)
	thumbAt(dc, imgs["setpix"], x+16, y+40, 120, 90, "SetPixel×2", x+16, y+146)
	thumbAt(dc, imgs["premul"], x+16+128, y+40, 120, 90, "SetPixelPremul", x+16+128, y+146)
	thumbAt(dc, imgs["writepix"], x+16+256, y+40, 120, 90, "WritePixels", x+16+256, y+146)
	wrkit.DirectDrawText(dc, "预乘/直写/上传三口逐像素精确断言", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B FillSpan / FillSpanBlend", faceCard)
	thumbAt(dc, imgs["span"], x+16, y+40, 120, 90, "FillSpan 实心", x+16, y+146)
	thumbAt(dc, imgs["spanblend"], x+16+128, y+40, 120, 90, "FillSpanBlend 混合", x+16+128, y+146)
	wrkit.DirectDrawText(dc, "混合=底×(1-α)+色×α（逐像素数学探针）", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C EncodePNG/JPEG 往返 + PixelWidth/Height", faceCard)
	thumbAt(dc, imgs["png"], x+16, y+40, 120, 90, "PNG 逐位往返", x+16, y+146)
	thumbAt(dc, imgs["jpeg"], x+16+128, y+40, 120, 90, "JPEG 容差往返", x+16+128, y+146)
	wrkit.DirectDrawText(dc, "PNG 解码逐位=源；JPEG 有损（容差声明）；尺寸读回", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D GenerationID + AddGenIDWatcher + CropImageToInk", faceCard)
	thumbAt(dc, imgs["crop"], x+16, y+40, 120, 90, "墨迹裁剪", x+16, y+146)
	thumbAt(dc, imgs["cropdraw"], x+16+128, y+40, 120, 90, "裁剪结果直画", x+16+128, y+146)
	wrkit.DirectDrawText(dc, "GenID 变更触发监视器（logic 探针）", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *pmLeg) {
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
