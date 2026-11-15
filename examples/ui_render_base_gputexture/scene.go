// scene.go paints the gputexture showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_gputexture — DrawGPUTexture family · texture ref · source registry · video paths"

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
	colSrcTL = [3]float64{0.85, 0.25, 0.25}
	colSrcTR = [3]float64{0.25, 0.75, 0.35}
	colSrcBL = [3]float64{0.30, 0.40, 0.85}
	colSrcBR = [3]float64{0.90, 0.70, 0.20}
	// 已知底色（透明度混合探针用）
	colBlendBG = [3]float64{0.12, 0.30, 0.40}
)

const (
	gtW, gtH = 160, 120 // 直绘源纹理尺寸
)

func fillRect(dc *render.Context, x, y, w, h float64, c [3]float64, a float64) {
	wrkit.DirectFillRect(dc, x, y, w, h, c, a)
}

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A GPU 纹理直绘: 不透明/透明度/UV",
		"B Base blit + TextureRef 生命周期",
		"C 纹理源注册: 自实现 PaintTexture",
		"D 视频路径: Frame/Slot/Bridge/平面",
		"probe: 注册计数/解析/混合数学",
		"pixel: 快照探针+抓屏对照",
		"golden: 顶栏+图例+四卡 逐位",
		"gate: -auto-only / RUN_SECONDS",
	}
	return func(dc *render.Context) {
		wrkit.PaintDirectChrome(dc, winTitle, lines, faceTitle, faceLeg)
	}
}

// thumb120 scales a 160×120 readback into a 120×90 thumb (s=0.75).
const thumbS = 120.0 / 160.0

func drawThumb(dc *render.Context, img image.Image, ox, oy float64) {
	if img == nil {
		return
	}
	dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
		X: ox, Y: oy, DstWidth: 120, DstHeight: 90})
}

func paintCardA(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(0)
	wrkit.PaintDirectCardFrame(dc, 0, "A GPU 纹理直绘 DrawGPUTexture 族", faceCard)
	labels := []string{"不透明", "透明度 0.5", "UV 子区"}
	for i, name := range []string{"opaque", "opacity", "uv"} {
		ix := float64(i) * 128
		drawThumb(dc, imgs[name], x+16+ix, y+40)
		wrkit.DirectDrawText(dc, labels[i], x+16+ix, y+146, faceSmall, wrkit.DColTextDim)
	}
	wrkit.DirectDrawText(dc, "透明度混合=底×0.5+源×0.5（逐像素数学探针）", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B DrawGPUTextureBase + TextureRef", faceCard)
	drawThumb(dc, imgs["base"], x+16, y+40)
	wrkit.DirectDrawText(dc, "Base 无剪裁", x+16, y+146, faceSmall, wrkit.DColTextDim)
	drawThumb(dc, imgs["ref"], x+16+128, y+40)
	wrkit.DirectDrawText(dc, "Ref 直绘", x+16+128, y+146, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "register→解析✓→直绘→release→解析✗→跳画", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C 纹理源注册 TextureSource 契约", faceCard)
	drawThumb(dc, imgs["source"], x+16, y+40)
	wrkit.DirectDrawText(dc, "自实现源 PaintTexture 直调", x+16, y+146, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "register→取回→直画→注销→取回✗", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintCardD(dc *render.Context, imgs map[string]image.Image, dLines []string) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D 视频路径 DrawVideoFrame/Slot/Bridge/Planes", faceCard)
	drawThumb(dc, imgs["vframe"], x+16, y+40)
	wrkit.DirectDrawText(dc, "ImageBuf 通用路", x+16, y+146, faceSmall, wrkit.DColTextDim)
	drawThumb(dc, imgs["vslot"], x+16+128, y+40)
	wrkit.DirectDrawText(dc, "Slot 零上传路", x+16+128, y+146, faceSmall, wrkit.DColTextDim)
	drawThumb(dc, imgs["vbridge"], x+16+256, y+40)
	wrkit.DirectDrawText(dc, "Bridge 路由", x+16+256, y+146, faceSmall, wrkit.DColTextDim)
	// 只画尾部 7 行（明细全量在 stderr/JSON；避免溢出卡底）
	start := 0
	if len(dLines) > 7 {
		start = len(dLines) - 7
	}
	for i, ln := range dLines[start:] {
		wrkit.DirectDrawText(dc, ln, x+16, y+178+float64(i)*20, faceSmall, wrkit.DColTextDim)
	}
}

func paintShowcase(dc *render.Context, leg *gtLeg) {
	imgs := map[string]image.Image{}
	dLines := []string{"offscreen 腿待执行"}
	if leg != nil {
		imgs = leg.imgs
		dLines = leg.dLines
	}
	paintChrome()(dc)
	paintCardA(dc, imgs)
	paintCardB(dc, imgs)
	paintCardC(dc, imgs)
	paintCardD(dc, imgs, dLines)
}
