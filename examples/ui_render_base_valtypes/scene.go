// scene.go paints the valtypes showcase on the shared direct-drive chrome.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_valtypes — Matrix/RRect/Paint/Stroke/Dash/RGBA/Vec2 predicates"

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
	colMatA   = [3]float64{0.35, 0.70, 0.90} // 恒等方块
	colMatB   = [3]float64{0.90, 0.55, 0.20} // 平移/缩放方块
	colRRectA = [3]float64{0.30, 0.80, 0.55} // 判型 rrect
	colDash   = [3]float64{0.85, 0.75, 0.30} // dash 线
	colPremul = [3]float64{0.60, 0.30, 0.80} // premul 色卡
	colVec    = [3]float64{0.90, 0.40, 0.35} // vec 箭头
)

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A Matrix: Is*/Scale*/TransformVector",
		"B RRect 判型: SetRect* 全矩阵",
		"C Paint/Stroke/Dash Effective 链",
		"D RGBA Premul 往返 + Vec2",
		"probe: 谓词读回+腿读回",
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

// paintCardA — matrix predicates with transformed squares.
func paintCardA(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(0)
	wrkit.PaintDirectCardFrame(dc, 0, "A Matrix · Is*/Scale*/Similarity/Max/TransformVector", faceCard)
	thumbImg(dc, imgs, x, y, 0, "matid", "恒等/平移")
	thumbImg(dc, imgs, x, y, 128, "matscale", "缩放/旋转")
	thumbImg(dc, imgs, x, y, 256, "matvec", "TransformVector")
	wrkit.DirectDrawText(dc, "TransformVector 不含平移项 · SimilarityScale 旋转矩阵 ok", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

// paintCardB — RRect type matrix.
func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B RRect · SetRect/XY/Radii/SetEmpty · Is* 全判型", faceCard)
	thumbImg(dc, imgs, x, y, 0, "rrtype", "判型矩阵")
	thumbImg(dc, imgs, x, y, 128, "rrclamp", "超大圆角缩夹")
	wrkit.DirectDrawText(dc, "Rect/Oval/Simple/NinePatch/Complex 五型读回", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

// paintCardC — Paint/Stroke/Dash effective chain.
func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C Paint.IsSolid/GetStroke/Effective* · Stroke.With* · Dash", faceCard)
	thumbImg(dc, imgs, x, y, 0, "painteff", "Effective 链")
	thumbImg(dc, imgs, x, y, 128, "dashpaint", "Dash 线")
	wrkit.DirectDrawText(dc, "默认值+设置值全链读回 · NormalizedOffset 归一", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

// paintCardD — RGBA premul roundtrip + Vec2.
func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D RGBA.Premultiply/Unpremultiply · Vec2.Approx/ToPoint", faceCard)
	thumbImg(dc, imgs, x, y, 0, "premul", "Premul 色卡")
	thumbImg(dc, imgs, x, y, 128, "vec2", "Vec2 箭头")
	wrkit.DirectDrawText(dc, "alpha<255 往返精确 · ToPoint/PointToVec2 往返", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *vtLeg) {
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
