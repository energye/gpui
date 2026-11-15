// scene.go paints the stylectx showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_stylectx — Context style direct verbs · dash phase · pipeline/rasterizer modes · path verbs"

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
	colHex    = [3]float64{1.0, 0.5, 0.25}    // #FF8040
	colStroke = [3]float64{0.20, 0.80, 0.55}  // 描边直画
	colDashA  = [3]float64{0.90, 0.85, 0.30}  // 虚线甲（相位 0）
	colDashB  = [3]float64{0.35, 0.55, 0.90}  // 虚线乙（相位 13）
	colModeA  = [3]float64{0.85, 0.30, 0.45}  // RenderPass 档
	colModeB  = [3]float64{0.30, 0.45, 0.85}  // Compute/Analytic 档
	colCPU    = [3]float64{0.95, 0.60, 0.15}  // FillRectCPU 直写
	colPath   = [3]float64{0.60, 0.30, 0.85}  // SetPath/NewSubPath
	colRRect  = [3]float64{0.20, 0.75, 0.65}  // 圆角族
	colInvY   = [3]float64{0.85, 0.20, 0.60}  // InvertY 镜像
)

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A 样式快捷: Hex/Brush/GetStroke 读回",
		"B 虚线相位: SetDashOffset 错相",
		"C 模式: Pipeline/Rasterizer 切换",
		"D 路径直控: SetPath/SubPath/RRect",
		"probe: 读回/相位/模式/镜像",
		"pixel: 快照探针+抓屏对照",
		"golden: 顶栏+图例+四卡 逐位",
		"gate: -auto-only / RUN_SECONDS",
	}
	return func(dc *render.Context) {
		wrkit.PaintDirectChrome(dc, winTitle, lines, faceTitle, faceLeg)
	}
}

// paintCardA — hex shortcut + brush getters + stroke readback.
func paintCardA(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(0)
	wrkit.PaintDirectCardFrame(dc, 0, "A SetHexColor · FillBrush/StrokeBrush · GetStroke", faceCard)
	thumb := func(name string, ix float64, label string) {
		if img := imgs[name]; img != nil {
			dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
				X: x + 16 + ix, Y: y + 40, DstWidth: 120, DstHeight: 90})
		}
		wrkit.DirectDrawText(dc, label, x+16+ix, y+146, faceSmall, wrkit.DColTextDim)
	}
	thumb("hex", 0, "SetHexColor 直画")
	thumb("brush", 128, "StrokeBrush 直画")
	wrkit.DirectDrawText(dc, "GetStroke 读回=设置值（宽/帽/虚线全链）", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

// paintCardB — dash phase: same [16,10] pattern, offsets 0 vs 13.
func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B SetDashOffset 虚线相位", faceCard)
	thumb := func(name string, ix float64, label string) {
		if img := imgs[name]; img != nil {
			dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
				X: x + 16 + ix, Y: y + 40, DstWidth: 120, DstHeight: 90})
		}
		wrkit.DirectDrawText(dc, label, x+16+ix, y+146, faceSmall, wrkit.DColTextDim)
	}
	thumb("phase0", 0, "offset=0")
	thumb("phase13", 128, "offset=13")
	wrkit.DirectDrawText(dc, "同图样不同相位：起笔位置错开（像素探针逐点验）", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

// paintCardC — pipeline / rasterizer mode toggles with pixel parity.
func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C PipelineMode · RasterizerMode 切换", faceCard)
	thumb := func(name string, ix float64, label string) {
		if img := imgs[name]; img != nil {
			dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
				X: x + 16 + ix, Y: y + 40, DstWidth: 120, DstHeight: 90})
		}
		wrkit.DirectDrawText(dc, label, x+16+ix, y+146, faceSmall, wrkit.DColTextDim)
	}
	thumb("rpass", 0, "RenderPass 档")
	thumb("compute", 128, "Compute 档")
	thumb("analytic", 256, "Analytic 档")
	wrkit.DirectDrawText(dc, "三档同场景像素正确（模式读回=设置值）", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

// paintCardD — path verbs: SetPath/NewSubPath/DrawRRect/DrawRoundedRectangleXY/
// FillRectCPU/InvertY — paths thumb (260×195) + cpufill thumb (120×90).
func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D SetPath · NewSubPath · RRect · FillRectCPU · InvertY", faceCard)
	if img := imgs["paths"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, "SetPath+环洞/椭圆角/InvertY", x+16, y+248, faceSmall, wrkit.DColTextDim)
	if img := imgs["cpufill"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 296, Y: y + 40, DstWidth: 120, DstHeight: 90})
	}
	wrkit.DirectDrawText(dc, "FillRectCPU 直写（干净上下文）", x+296, y+146, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "多子路×SetPath 洞见 findings（钉子已落）", x+16, y+wrkit.DCardH-28, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *scLeg) {
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
