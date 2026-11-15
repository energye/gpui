// vtleg.go — offscreen direct legs for value-type predicates. Matrix, RRect,
// Paint/Stroke/Dash, RGBA and Vec2 entries are exercised with readback
// assertions; painted squares anchor the visible state. One GL-hub job.
package main

import (
	"fmt"
	"image"
	"math"
	"os"

	"github.com/energye/gpui/render"
)

const (
	vtW, vtH = 320, 240 // leg context size
)

type vtLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *vtLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "vtleg: %s\n", line)
}

func flushVT(dc *render.Context, view render.TextureView, leg *vtLeg, tag string) bool {
	if err := dc.PresentFrame(view, vtW, vtH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	return true
}

func clearVT(dc *render.Context) {
	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, vtW, vtH)
	_ = dc.Fill()
}

func nearV(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// ---- Card A: Matrix predicates ----
func runCardALeg(dc *render.Context, view render.TextureView, leg *vtLeg) {
	clearVT(dc)

	id := render.Identity()
	leg.probe("A.IsIdentity", id.IsIdentity(), "Identity() 自判")

	// 字段布局 [A B C; D E F]，C/F 为平移项（TransformPoint 含、TransformVector 不含）。
	tr := render.Translate(30, 0)
	leg.probe("A.IsTranslation", tr.IsTranslation() && tr.IsTranslationOnly(),
		"平移矩阵：IsTranslation 真（源码口径 IsTranslationOnly 同真）")
	trOnly := render.Translate(30, 40)
	leg.probe("A.IsTranslationOnly", trOnly.IsTranslationOnly(), "纯平移（2x2 段=恒等）")
	leg.probe("A.平移ScaleFactor", nearV(trOnly.ScaleFactor(), 1, 1e-12), "ScaleFactor=1")

	sc := render.Scale(2, 3)
	leg.probe("A.IsScaleOnly", sc.IsScaleOnly() && !sc.IsTranslationOnly(), "2,3 缩放")
	leg.probe("A.MaxScaleFactor", nearV(sc.MaxScaleFactor(), 3, 1e-12),
		fmt.Sprintf("max=%.1f", sc.MaxScaleFactor()))

	rot := render.Rotate(math.Pi / 6)
	scale, ok := rot.SimilarityScale()
	leg.probe("A.SimilarityScale", ok && nearV(scale, 1, 1e-9),
		fmt.Sprintf("30° 旋转 scale=%.4f ok=%v", scale, ok))

	// 非均匀缩放×旋转 = 非相似变换（ok=false 语义断言）；
	// 均匀缩放×旋转 = 相似变换 scale=2。
	rotNonUniform := rot.Multiply(sc)
	_, okNon := rotNonUniform.SimilarityScale()
	rotUniform := rot.Multiply(render.Scale(2, 2))
	scaleU, okU := rotUniform.SimilarityScale()
	leg.probe("A.相似变换", !okNon && okU && nearV(scaleU, 2, 1e-9) && !rotUniform.IsScaleOnly(),
		fmt.Sprintf("非均匀 ok=%v 均匀 scale=%.3f ok=%v", okNon, scaleU, okU))

	// TransformVector excludes translation.
	rotTr := render.Translate(100, 50).Multiply(rot)
	v := rotTr.TransformVector(render.Point{X: 1, Y: 0})
	leg.probe("A.TransformVector", nearV(v.X, math.Cos(math.Pi/6), 1e-9) && nearV(v.Y, math.Sin(math.Pi/6), 1e-9) &&
		rotTr.TransformPoint(render.Point{X: 1, Y: 0}).X != v.X,
		fmt.Sprintf("(1,0)→vec(%.4f,%.4f) point.x=%.4f（向量无平移）", v.X, v.Y,
			rotTr.TransformPoint(render.Point{X: 1, Y: 0}).X))

	// paint: identity square vs translated/scaled square
	dc.SetRGBA(colMatA[0], colMatA[1], colMatA[2], 1)
	dc.DrawRectangle(20, 40, 50, 50)
	_ = dc.Fill()
	dc.Push()
	dc.Transform(trOnly)
	dc.SetRGBA(colMatB[0], colMatB[1], colMatB[2], 1)
	dc.DrawRectangle(20, 40, 50, 50)
	_ = dc.Fill()
	dc.Pop()
	dc.Push()
	dc.Transform(sc)
	dc.SetRGBA(colMatA[0], colMatA[1], colMatA[2], 0.6)
	dc.DrawRectangle(10, 150, 20, 20)
	_ = dc.Fill()
	dc.Pop()
	dc.SetRGBA(1, 1, 1, 1)
	if !flushVT(dc, view, leg, "A") {
		return
	}
	leg.imgs["matid"] = dc.Image()
	leg.imgs["matscale"] = dc.Image()
	leg.imgs["matvec"] = dc.Image()
}

// ---- Card B: RRect type matrix ----
func runCardBLeg(dc *render.Context, view render.TextureView, leg *vtLeg) {
	clearVT(dc)

	rectRR := render.NewRRect(render.Rect{Min: render.Pt(10, 10), Max: render.Pt(110, 70)})
	leg.probe("B.IsRect", rectRR.IsRect(), "直角矩形")
	leg.probe("B.Type读回", rectRR.Type() == render.RRectRect, "typ=RRectRect")

	ovalRR := render.NewRRectXY(render.Rect{Min: render.Pt(120, 10), Max: render.Pt(220, 70)}, 50, 30)
	leg.probe("B.IsOval", ovalRR.IsOval(), "rX=半宽 rY=半高→椭圆")

	simpleRR := render.NewRRectXY(render.Rect{Min: render.Pt(10, 90), Max: render.Pt(90, 150)}, 10, 10)
	leg.probe("B.IsSimple", simpleRR.IsSimple() && simpleRR.IsSimpleCircular(),
		"统一圆角 → Simple+SimpleCircular")

	// nine-patch 判据（rrRadiiAreNinePatch）: ul.X==ll.X && ul.Y==ur.Y &&
	// ur.X==lr.X && ll.Y==lr.Y — 沿边配对相等。
	nineRR := render.NewRRectRadii(render.Rect{Min: render.Pt(110, 90), Max: render.Pt(210, 150)},
		[4]render.Point{
			render.Pt(20, 10), render.Pt(15, 10), // ul, ur
			render.Pt(15, 25), render.Pt(20, 25), // lr, ll
		})
	leg.probe("B.IsNinePatch", nineRR.IsNinePatch(),
		fmt.Sprintf("typ=%v（沿边配对）", nineRR.Type()))

	complexRR := render.NewRRectRadii(render.Rect{Min: render.Pt(230, 90), Max: render.Pt(310, 150)},
		[4]render.Point{
			render.Pt(30, 10), render.Pt(5, 5), render.Pt(20, 25), render.Pt(0, 0),
		})
	leg.probe("B.IsComplex", complexRR.IsComplex(), "四角各异 → Complex")

	emptyRR := render.RRect{}
	emptyRR.SetEmpty()
	leg.probe("B.SetEmpty", emptyRR.IsEmpty(), "空")

	// SetRect / SetRectXY / SetRectRadii mutators.
	var mut render.RRect
	mut.SetRect(render.Rect{Min: render.Pt(10, 170), Max: render.Pt(80, 220)})
	leg.probe("B.SetRect", mut.IsRect(), "SetRect→Rect")
	mut.SetRectXY(render.Rect{Min: render.Pt(10, 170), Max: render.Pt(80, 220)}, 8, 8)
	leg.probe("B.SetRectXY", mut.IsSimple() && nearV(mut.Radii(render.RRectCorner(0)).X, 8, 1e-9),
		"SetRectXY→Simple r=8")
	mut.SetRectRadii(render.Rect{Min: render.Pt(10, 170), Max: render.Pt(80, 220)},
		[4]render.Point{render.Pt(12, 8), render.Pt(8, 8), render.Pt(8, 12), render.Pt(12, 12)})
	leg.probe("B.SetRectRadii", mut.IsNinePatch(), "SetRectRadii→NinePatch")

	// oversize radii clamp: radius > half min dimension scales down.
	clamped := render.NewRRectXY(render.Rect{Min: render.Pt(240, 10), Max: render.Pt(310, 60)}, 60, 60)
	leg.probe("B.超大圆角缩夹", clamped.Radii(render.RRectCorner(0)).X <= 25.5,
		fmt.Sprintf("请求 r=60 → 实际 %.1f（≤半高 25）", clamped.Radii(render.RRectCorner(0)).X))
	leg.probe("B.缩夹后仍Simple", clamped.IsSimple(), "缩夹后判型不丢")

	// paint: three rrects
	for _, rr := range []render.RRect{rectRR, ovalRR, simpleRR, nineRR, complexRR} {
		dc.SetPath(render.BuildPath().RRect(rr).Build())
		dc.SetRGBA(colRRectA[0], colRRectA[1], colRRectA[2], 1)
		_ = dc.Fill()
	}
	dc.SetRGBA(1, 1, 1, 1)
	if !flushVT(dc, view, leg, "B") {
		return
	}
	leg.imgs["rrtype"] = dc.Image()
	leg.imgs["rrclamp"] = dc.Image()
}

// ---- Card C: Paint/Stroke/Dash effective chain ----
func runCardCLeg(dc *render.Context, view render.TextureView, leg *vtLeg) {
	clearVT(dc)

	paint := render.NewPaint()
	leg.probe("C.默认IsSolid", paint.IsSolid(), "无 brush → solid")
	leg.probe("C.默认宽度", nearV(paint.EffectiveLineWidth(), 1, 1e-12),
		fmt.Sprintf("=%.1f", paint.EffectiveLineWidth()))
	leg.probe("C.默认虚线", paint.EffectiveDash() == nil && !paint.IsDashed(), "无 dash")
	leg.probe("C.默认PathEffect", paint.EffectivePathEffect() == nil && !paint.HasPathEffect(), "无 effect")

	// GetStroke readback after SetStroke.
	st := render.Stroke{Width: 4, Cap: render.LineCapRound, Join: render.LineJoinBevel, MiterLimit: 7}
	paint.SetStroke(st)
	got := paint.GetStroke()
	leg.probe("C.GetStroke", got.Width == 4 && got.Cap == render.LineCapRound &&
		got.Join == render.LineJoinBevel && got.MiterLimit == 7,
		fmt.Sprintf("w=%.0f cap=%v join=%v miter=%.0f", got.Width, got.Cap, got.Join, got.MiterLimit))
	leg.probe("C.Effective宽度", nearV(paint.EffectiveLineWidth(), 4, 1e-12), "=设置值")

	// Stroke With* chain: immutable chaining readback.
	st2 := render.Stroke{Width: 2}.
		WithWidth(6).
		WithCap(render.LineCapSquare).
		WithJoin(render.LineJoinRound).
		WithMiterLimit(9).
		WithDashPattern(14, 6).
		WithDashOffset(5)
	leg.probe("C.Stroke.With链", st2.Width == 6 && st2.Cap == render.LineCapSquare &&
		st2.Join == render.LineJoinRound && st2.MiterLimit == 9 && st2.IsDashed() &&
		st2.Dash != nil && len(st2.Dash.Array) == 2 && st2.Dash.Offset == 5,
		fmt.Sprintf("w=%.0f dash=%v offset=%.0f", st2.Width, st2.Dash.Array, st2.Dash.Offset))

	// Dash math: pattern length / normalized offset / scale / clone.
	// 奇数元素模式按 Cairo/Skia 约定复制翻倍：[12,8,4] → 24×2=48。
	dash := render.NewDash(12, 8, 4)
	leg.probe("C.PatternLength", nearV(dash.PatternLength(), 48, 1e-9), "奇元素×2: (12+8+4)×2=48")
	even := render.NewDash(12, 8)
	leg.probe("C.偶元素长度", nearV(even.PatternLength(), 20, 1e-9), "12+8=20")
	doff := dash.WithOffset(30)
	leg.probe("C.WithOffset", nearV(doff.Offset, 30, 1e-9), "offset=30")
	leg.probe("C.NormalizedOffset", nearV(doff.NormalizedOffset(), 30, 1e-9),
		fmt.Sprintf("30 mod 48 = %.1f", doff.NormalizedOffset()))
	scaled := dash.Scale(2)
	leg.probe("C.Scale", len(scaled.Array) == 3 && scaled.Array[0] == 24 && scaled.Array[1] == 16 && scaled.Array[2] == 8,
		fmt.Sprintf("数组×2=%v offset=%v", scaled.Array, scaled.Offset))
	cl := dash.Clone()
	leg.probe("C.Clone", cl != dash && len(cl.Array) == 3 && cl.Array[0] == 12,
		"新实例同值")

	// paint: dashed line via dc dash API + solid anchor square
	dc.SetRGBA(colDash[0], colDash[1], colDash[2], 1)
	dc.SetLineWidth(6)
	dc.SetDash(14, 8)
	dc.SetDashOffset(5)
	dc.MoveTo(20, 50)
	dc.LineTo(300, 50)
	_ = dc.Stroke()
	dc.SetDash()
	dc.SetDashOffset(0)
	dc.SetLineWidth(1)
	dc.SetRGBA(colMatA[0], colMatA[1], colMatA[2], 1)
	dc.DrawRectangle(20, 100, 60, 40)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 1)
	if !flushVT(dc, view, leg, "C") {
		return
	}
	leg.imgs["painteff"] = dc.Image()
	leg.imgs["dashpaint"] = dc.Image()
}

// ---- Card D: RGBA premul roundtrip + Vec2 ----
func runCardDLeg(dc *render.Context, view render.TextureView, leg *vtLeg) {
	clearVT(dc)

	// Premultiply / Unpremultiply roundtrip (alpha < 1).
	c := render.RGBA2(0.8, 0.5, 0.3, 0.4)
	pm := c.Premultiply()
	leg.probe("D.Premultiply", nearV(pm.R, 0.8*0.4, 1e-12) && nearV(pm.G, 0.5*0.4, 1e-12) &&
		nearV(pm.B, 0.3*0.4, 1e-12) && nearV(pm.A, 0.4, 1e-12),
		fmt.Sprintf("RGB×a=(%.3f,%.3f,%.3f)", pm.R, pm.G, pm.B))
	up := pm.Unpremultiply()
	leg.probe("D.往返精确", nearV(up.R, 0.8, 1e-9) && nearV(up.G, 0.5, 1e-9) && nearV(up.B, 0.3, 1e-9) &&
		nearV(up.A, 0.4, 1e-9), "premul→unpremul 往返")

	zero := render.RGBA2(1, 1, 1, 0)
	leg.probe("D.零alpha", zero.Premultiply() == render.RGBA{}, "A=0 → 全 0")

	// Vec2 Approx / ToPoint / PointToVec2.
	v1 := render.V2(1.0, 2.0)
	v2 := render.V2(1.0+1e-9, 2.0-1e-9)
	leg.probe("D.Vec2Approx", v1.Approx(v2, 1e-6) && !v1.Approx(render.V2(1.1, 2.0), 1e-6),
		"1e-9 内真 0.1 偏差假")
	p := v1.ToPoint()
	leg.probe("D.ToPoint", p == render.Pt(1, 2), "(1,2)")
	back := render.PointToVec2(p)
	leg.probe("D.PointToVec2", back == v1, "往返")

	// paint: premul color card over black + vec arrows
	dc.SetFillBrush(render.BrushWithAlpha(render.SolidRGB(colPremul[0], colPremul[1], colPremul[2]), 0.4))
	dc.DrawRectangle(30, 40, 60, 40)
	_ = dc.Fill()
	dc.SetFillBrush(render.SolidRGB(colPremul[0], colPremul[1], colPremul[2]))
	dc.DrawRectangle(110, 40, 60, 40)
	_ = dc.Fill()
	dc.SetFillBrush(render.SolidRGB(1, 1, 1))
	// vec arrows: (1,0)→right, (0,1)→down, rotate 45°
	arrow := func(v render.Vec2, ox, oy float64) {
		dc.SetRGBA(colVec[0], colVec[1], colVec[2], 1)
		dc.SetLineWidth(2)
		dc.MoveTo(ox, oy)
		dc.LineTo(ox+v.X*30, oy+v.Y*30)
		_ = dc.Stroke()
	}
	arrow(render.V2(1, 0), 30, 130)
	arrow(render.V2(0, 1), 80, 130)
	arrow(render.V2(1, 0).Rotate(math.Pi/4), 130, 130)
	if !flushVT(dc, view, leg, "D") {
		return
	}
	leg.imgs["premul"] = dc.Image()
	leg.imgs["vec2"] = dc.Image()
}

// runVTLegs drives all four cards' offscreen legs. One GL-hub job.
func runVTLegs() *vtLeg {
	leg := &vtLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(vtW, vtH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(vtW, vtH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	runCardALeg(dc, view, leg)
	runCardBLeg(dc, view, leg)
	runCardCLeg(dc, view, leg)
	runCardDLeg(dc, view, leg)
	return leg
}
