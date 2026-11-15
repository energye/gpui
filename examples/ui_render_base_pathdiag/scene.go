// scene.go paints the pathdiag showcase on the shared direct-drive chrome.
package main

import (
	"fmt"
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_pathdiag — PathBuilder shapes · DetectShape kinds · path queries · Append"

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
	colCircle  = [3]float64{0.30, 0.70, 0.90} // builder 圆
	colStar    = [3]float64{0.95, 0.75, 0.25} // builder 星
	colRRect   = [3]float64{0.35, 0.80, 0.50} // builder 圆角矩形
	colPoly    = [3]float64{0.85, 0.45, 0.30} // builder 多边形
	colRectD   = [3]float64{0.55, 0.45, 0.85} // 判定矩阵 rect
	colArcD    = [3]float64{0.30, 0.85, 0.70} // 判定矩阵 arc
	colQuery   = [3]float64{0.90, 0.60, 0.20} // path 查询
	colReversed = [3]float64{0.80, 0.30, 0.45} // 反转对照
	colAppend  = [3]float64{0.45, 0.65, 0.30} // Append 拼接
)

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A Builder: Star/RoundRect/Circle",
		"B DetectShape: 六路判定+参数",
		"C 查询: CurrentPoint/Reversed/Flatten",
		"D Append 拼接 + Build 别名",
		"probe: 判定读回+腿读回",
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

// paintCardA — builder shapes.
func paintCardA(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(0)
	wrkit.PaintDirectCardFrame(dc, 0, "A PathBuilder · Star/RoundRect/Polygon/Circle/Ellipse", faceCard)
	thumbImg(dc, imgs, x, y, 0, "circle", "Circle")
	thumbImg(dc, imgs, x, y, 128, "star", "Star")
	thumbImg(dc, imgs, x, y, 256, "shapes", "RoundRect/Polygon")
	wrkit.DirectDrawText(dc, "DetectShape 读回：圆→ShapeCircle 星→ShapeUnknown", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

// paintCardB — DetectShape decision matrix with parameter readback.
func paintCardB(dc *render.Context, imgs map[string]image.Image, leg *pdLeg) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B DetectShape · Shape* 判定 + 参数逐字段", faceCard)
	if img := imgs["detect"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, "rect/rrect/arc/ellipse/circle/unknown 六路", x+16, y+248, faceSmall, wrkit.DColTextDim)
	if leg != nil {
		wrkit.DirectDrawText(dc, fmt.Sprintf("arc a0=%.2f a1=%.2f rrect r=%.0f",
			leg.arcA0, leg.arcA1, leg.rrectR), x+296, y+70, faceSmall, wrkit.DColTextDim)
	}
	if img := imgs["detectzoom"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 296, Y: y + 40, DstWidth: 120, DstHeight: 90})
	}
	wrkit.DirectDrawText(dc, "RRect 判定放大", x+296, y+146, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "Angle0/1 恢复 DrawArc 弧窗", x+296, y+wrkit.DCardH-28, faceSmall, wrkit.DColTextDim)
}

// paintCardC — path queries: current point lifecycle, reversed, flatten.
func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C CurrentPoint · Reversed · FlattenCallback", faceCard)
	if img := imgs["query"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, "原路径+Reversed 叠画（箭头标走向）", x+16, y+248, faceSmall, wrkit.DColTextDim)
	thumbImg(dc, imgs, x, y, 296-16, "flatten", "Flatten 点")
	wrkit.DirectDrawText(dc, "tol=0.1 采样点 vs 真曲线 ≤0.1", x+296, y+wrkit.DCardH-28, faceSmall, wrkit.DColTextDim)
}

// paintCardD — Append concatenation + Build alias.
func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D Path.Append · Build()==Path()", faceCard)
	if img := imgs["append"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, "两子路一笔填充（verb 数=a+b）", x+16, y+248, faceSmall, wrkit.DColTextDim)
	thumbImg(dc, imgs, x, y, 296-16, "alias", "Build 别名")
	wrkit.DirectDrawText(dc, "Build()==Path() 同指针", x+296, y+wrkit.DCardH-28, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *pdLeg) {
	imgs := map[string]image.Image{}
	if leg != nil {
		imgs = leg.imgs
	}
	paintChrome()(dc)
	paintCardA(dc, imgs)
	paintCardB(dc, imgs, leg)
	paintCardC(dc, imgs)
	paintCardD(dc, imgs)
}
