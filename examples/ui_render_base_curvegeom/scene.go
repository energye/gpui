// scene.go paints the curvegeom showcase on the shared direct-drive chrome.
package main

import (
	"image"
	"math"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const winTitle = "gpui ui_render_base_curvegeom — Cubic/Quad/Conic/Line introspection · solvers · ClassifyQuad · PathMeasure"

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
	colCubic   = [3]float64{0.95, 0.60, 0.15} // 卡 A cubic 曲线
	colQuad    = [3]float64{0.30, 0.70, 0.90} // 卡 A quad 曲线
	colDeriv   = [3]float64{0.85, 0.25, 0.45} // 卡 A 切向箭头
	colBBox    = [3]float64{0.55, 0.85, 0.45} // 卡 A 紧致框
	colHull    = [3]float64{0.45, 0.45, 0.55} // 卡 A 控制凸包框
	colConic   = [3]float64{0.60, 0.30, 0.85} // 卡 B conic 曲线
	colToQuads = [3]float64{0.90, 0.85, 0.30} // 卡 B ToQuads 折线
	colRoot    = [3]float64{0.20, 0.75, 0.65} // 卡 C solver 根刻度
	colMeasure = [3]float64{0.90, 0.50, 0.20} // 卡 C PathMeasure 采样点
	colQuadOK  = [3]float64{0.35, 0.80, 0.45} // 卡 D QuadOK
	colDegener = [3]float64{0.85, 0.70, 0.25} // 卡 D Degenerate
	colBowTie  = [3]float64{0.85, 0.30, 0.35} // 卡 D BowTie
)

func paintChrome() func(dc *render.Context) {
	lines := []string{
		"A Cubic/Quad: Eval/Deriv/Subdivide/BBox",
		"B Conic: W 四档 + ToQuads 叠画",
		"C Solver 根 + PathMeasure 采样",
		"D ClassifyQuad 四态判型",
		"probe: 数值读回+腿读回",
		"pixel: 快照探针+抓屏对照",
		"golden: 顶栏+图例+四卡 逐位",
		"gate: -auto-only / RUN_SECONDS",
	}
	return func(dc *render.Context) {
		wrkit.PaintDirectChrome(dc, winTitle, lines, faceTitle, faceLeg)
	}
}

// strokePolyline strokes a polyline through pts (device-space curve drawing).
func strokePolyline(dc *render.Context, pts []render.Point, c [3]float64, lw float64) {
	if len(pts) < 2 {
		return
	}
	dc.SetRGBA(c[0], c[1], c[2], 1)
	dc.SetLineWidth(lw)
	dc.MoveTo(pts[0].X, pts[0].Y)
	for _, p := range pts[1:] {
		dc.LineTo(p.X, p.Y)
	}
	_ = dc.Stroke()
}

// sampleCubic evaluates the cubic into a polyline.
func sampleCubic(c render.CubicBez, n int) []render.Point {
	pts := make([]render.Point, 0, n+1)
	for i := 0; i <= n; i++ {
		pts = append(pts, c.Eval(float64(i)/float64(n)))
	}
	return pts
}

// sampleQuad evaluates the quad into a polyline.
func sampleQuad(q render.QuadBez, n int) []render.Point {
	pts := make([]render.Point, 0, n+1)
	for i := 0; i <= n; i++ {
		pts = append(pts, q.Eval(float64(i)/float64(n)))
	}
	return pts
}

// sampleConic evaluates the conic into a polyline.
func sampleConic(c render.ConicBez, n int) []render.Point {
	pts := make([]render.Point, 0, n+1)
	for i := 0; i <= n; i++ {
		pts = append(pts, c.Eval(float64(i)/float64(n)))
	}
	return pts
}

// drawTangentArrow draws a short tangent arrow at p along dir.
func drawTangentArrow(dc *render.Context, p render.Point, dir render.Vec2, c [3]float64) {
	n := dir.Normalize()
	tip := render.Pt(p.X+n.X*18, p.Y+n.Y*18)
	perp := n.Perp()
	dc.SetRGBA(c[0], c[1], c[2], 1)
	dc.SetLineWidth(1.5)
	dc.MoveTo(p.X, p.Y)
	dc.LineTo(tip.X, tip.Y)
	_ = dc.Stroke()
	// arrowhead
	l := render.Pt(tip.X-n.X*6+perp.X*4, tip.Y-n.Y*6+perp.Y*4)
	r := render.Pt(tip.X-n.X*6-perp.X*4, tip.Y-n.Y*6-perp.Y*4)
	dc.MoveTo(l.X, l.Y)
	dc.LineTo(tip.X, tip.Y)
	dc.LineTo(r.X, r.Y)
	_ = dc.Stroke()
}

// strokeRect strokes an axis-aligned rect outline.
func strokeRectPath(dc *render.Context, r render.Rect, c [3]float64, lw float64) {
	dc.SetRGBA(c[0], c[1], c[2], 1)
	dc.SetLineWidth(lw)
	dc.MoveTo(r.Min.X, r.Min.Y)
	dc.LineTo(r.Max.X, r.Min.Y)
	dc.LineTo(r.Max.X, r.Max.Y)
	dc.LineTo(r.Min.X, r.Max.Y)
	dc.ClosePath()
	_ = dc.Stroke()
}

// paintCardA — Cubic/Quad introspection: polyline, deriv endpoints as tangent
// arrows, subdivide halves in distinct colors, tight bbox vs control hull.
func paintCardA(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(0)
	wrkit.PaintDirectCardFrame(dc, 0, "A Cubic/Quad · Deriv/Tangent/Subdivide/BBox", faceCard)
	thumb := func(name string, ix float64, label string) {
		if img := imgs[name]; img != nil {
			dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
				X: x + 16 + ix, Y: y + 40, DstWidth: 120, DstHeight: 90})
		}
		wrkit.DirectDrawText(dc, label, x+16+ix, y+146, faceSmall, wrkit.DColTextDim)
	}
	thumb("cubic", 0, "Cubic 内省")
	thumb("quad", 128, "Quad 内省")
	thumb("bbox", 256, "BBox/凸包")
	wrkit.DirectDrawText(dc, "Deriv 端点=切向箭头 · Subdivide 两半异色 · 紧致框⊂凸包框", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

// paintCardB — Conic family: four weights, ToQuads overlay.
func paintCardB(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(1)
	wrkit.PaintDirectCardFrame(dc, 1, "B ConicBez · W 四档 · ToQuads · Extrema/BBox", faceCard)
	if img := imgs["conic"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, "W=0.3 / 0.7071(¼圆弧) / 1(=二次) / 1.5", x+16, y+248, faceSmall, wrkit.DColTextDim)
	if img := imgs["arc"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 296, Y: y + 40, DstWidth: 120, DstHeight: 90})
	}
	wrkit.DirectDrawText(dc, "¼圆弧端点几何", x+296, y+146, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "ToQuads 折线叠画 ≤tol", x+296, y+wrkit.DCardH-28, faceSmall, wrkit.DColTextDim)
}

// paintCardC — solver roots as tick marks + PathMeasure sample points.
func paintCardC(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(2)
	wrkit.PaintDirectCardFrame(dc, 2, "C SolveQuad/Cubic · NewPathMeasure", faceCard)
	if img := imgs["roots"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16, Y: y + 40, DstWidth: 260, DstHeight: 195})
	}
	wrkit.DirectDrawText(dc, "两实根/重根/三实根/无实根 四档刻度", x+16, y+248, faceSmall, wrkit.DColTextDim)
	if img := imgs["measure"]; img != nil {
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 296, Y: y + 40, DstWidth: 120, DstHeight: 90})
	}
	wrkit.DirectDrawText(dc, "Sample 沿路点", x+296, y+146, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "Length vs 逐弦 oracle", x+296, y+wrkit.DCardH-28, faceSmall, wrkit.DColTextDim)
}

// paintCardD — ClassifyQuad four states (NonFinite marked, not drawn).
func paintCardD(dc *render.Context, imgs map[string]image.Image) {
	x, y := wrkit.DirectCardAt(3)
	wrkit.PaintDirectCardFrame(dc, 3, "D ClassifyQuad · OK/Degenerate/BowTie/NonFinite", faceCard)
	thumb := func(name string, ix float64, label string) {
		if img := imgs[name]; img != nil {
			dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
				X: x + 16 + ix, Y: y + 40, DstWidth: 120, DstHeight: 90})
		}
		wrkit.DirectDrawText(dc, label, x+16+ix, y+146, faceSmall, wrkit.DColTextDim)
	}
	thumb("qok", 0, "QuadOK")
	thumb("qdeg", 128, "Degenerate")
	thumb("qbow", 256, "BowTie")
	wrkit.DirectDrawText(dc, "NonFinite 不实画（NaN 角点判型读回）· ErrQuad* 哨兵 errors.Is 可判", x+16, y+180, faceSmall, wrkit.DColTextDim)
}

func paintShowcase(dc *render.Context, leg *cgLeg) {
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

// cardDQuads returns the four ClassifyQuad corner sets (leg + painter share).
func cardDQuads() (ok, deg, bow [4]render.Point) {
	ok = [4]render.Point{
		render.Pt(10, 10), render.Pt(100, 14), render.Pt(96, 100), render.Pt(14, 96),
	}
	deg = [4]render.Point{ // all four corners collinear on y=x → zero area
		render.Pt(10, 10), render.Pt(30, 30), render.Pt(50, 50), render.Pt(70, 70),
	}
	bow = [4]render.Point{ // self-intersecting bow tie
		render.Pt(10, 10), render.Pt(110, 100), render.Pt(110, 10), render.Pt(10, 100),
	}
	return ok, deg, bow
}

// drawQuadCorners strokes the quad outline through the four corners.
func drawQuadCorners(dc *render.Context, q [4]render.Point, c [3]float64, lw float64) {
	dc.SetRGBA(c[0], c[1], c[2], 1)
	dc.SetLineWidth(lw)
	dc.MoveTo(q[0].X, q[0].Y)
	dc.LineTo(q[1].X, q[1].Y)
	dc.LineTo(q[2].X, q[2].Y)
	dc.LineTo(q[3].X, q[3].Y)
	dc.ClosePath()
	_ = dc.Stroke()
}

var _ = math.Sqrt2 // keep math import when card art evolves
