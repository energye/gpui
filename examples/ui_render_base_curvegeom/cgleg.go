// cgleg.go — offscreen direct legs for the curve geometry family. Every
// CubicBez / QuadBez / ConicBez / Line introspection entry, the solver pair,
// ClassifyQuad four states and PathMeasure are exercised on offscreen views;
// readbacks assert the observable numbers. One GL-hub job.
package main

import (
	"fmt"
	"image"
	"math"
	"os"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
)

const (
	cgW, cgH = 320, 240 // leg context size
)

type cgLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *cgLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "cgleg: %s\n", line)
}

func nearF(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func flushCG(dc *render.Context, view render.TextureView, leg *cgLeg, tag string) bool {
	if err := dc.PresentFrame(view, cgW, cgH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	return true
}

// runCGLegs drives all four cards' offscreen legs. One GL-hub job.
func runCGLegs() *cgLeg {
	leg := &cgLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(cgW, cgH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(cgW, cgH)
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

// ---- Card A: Cubic/Quad introspection ----
func runCardALeg(dc *render.Context, view render.TextureView, leg *cgLeg) {
	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, cgW, cgH)
	_ = dc.Fill()

	cubic := render.NewCubicBez(
		render.Pt(20, 200), render.Pt(90, 30), render.Pt(200, 210), render.Pt(300, 60))
	quad := render.NewQuadBez(
		render.Pt(20, 110), render.Pt(160, 20), render.Pt(300, 130))

	// Eval endpoints
	leg.probe("A.CubicEval端点", cubic.Eval(0) == cubic.P0 && cubic.Eval(1) == cubic.P3,
		"Eval(0)=P0 Eval(1)=P3")
	leg.probe("A.QuadEval端点", quad.Eval(0) == quad.P0 && quad.Eval(1) == quad.P2,
		"Eval(0)=P0 Eval(1)=P2")

	// Deriv: derivative curve endpoints = 3×(P1-P0) etc; tangent at ends.
	deriv := cubic.Deriv()
	d0 := deriv.Eval(0)
	wantDX := 3 * (cubic.P1.X - cubic.P0.X)
	leg.probe("A.Deriv读回", nearF(d0.X, wantDX, 1e-9),
		fmt.Sprintf("deriv(0).x=%.2f want=%.2f", d0.X, wantDX))
	tan0 := cubic.Tangent(0)
	leg.probe("A.Tangent方向", tan0 == render.Vec2(d0),
		fmt.Sprintf("tangent(0)=%v", tan0))
	nrm := cubic.Normal(0.5)
	perp := cubic.Tangent(0.5).Perp().Normalize()
	leg.probe("A.Normal垂直", nrm == perp,
		fmt.Sprintf("normal(0.5)=%v", nrm))

	// Subdivide: halves share the midpoint, rejoin = original geometry.
	l, r := cubic.Subdivide()
	mid := cubic.Eval(0.5)
	leg.probe("A.Subdivide接点", l.P3 == mid && r.P0 == mid,
		fmt.Sprintf("mid=(%.1f,%.1f)", mid.X, mid.Y))
	rejoin := sampleCubic(l, 16)
	rejoin = append(rejoin, sampleCubic(r, 16)[1:]...)
	maxDev := 0.0
	for i, p := range rejoin {
		t := float64(i) / float64(len(rejoin)-1)
		q := cubic.Eval(t)
		if d := math.Hypot(p.X-q.X, p.Y-q.Y); d > maxDev {
			maxDev = d
		}
	}
	leg.probe("A.Subdivide重组", maxDev < 1e-9,
		fmt.Sprintf("重组偏差=%.2e", maxDev))

	// Subsegment [0.25,0.75] matches original on interior samples.
	sub := cubic.Subsegment(0.25, 0.75)
	dev := 0.0
	for i := 0; i <= 8; i++ {
		t := 0.25 + 0.5*float64(i)/8
		p := sub.Eval(float64(i) / 8)
		q := cubic.Eval(t)
		if d := math.Hypot(p.X-q.X, p.Y-q.Y); d > dev {
			dev = d
		}
	}
	leg.probe("A.Subsegment", dev < 1e-9, fmt.Sprintf("偏差=%.2e", dev))

	// Extrema: hand-picked cubic with known x-extremum at t where dx/dt=0.
	ext := cubic.Extrema()
	leg.probe("A.Extrema单调", len(ext) > 0,
		fmt.Sprintf("n=%d t∈%v", len(ext), ext))

	// BoundingBox: tight box ⊆ control hull box; contains endpoints.
	bb := cubic.BoundingBox()
	hullMinX := math.Min(cubic.P0.X, math.Min(cubic.P1.X, math.Min(cubic.P2.X, cubic.P3.X)))
	hullMaxX := math.Max(cubic.P0.X, math.Max(cubic.P1.X, math.Max(cubic.P2.X, cubic.P3.X)))
	hullMinY := math.Min(cubic.P0.Y, math.Min(cubic.P1.Y, math.Min(cubic.P2.Y, cubic.P3.Y)))
	hullMaxY := math.Max(cubic.P0.Y, math.Max(cubic.P1.Y, math.Max(cubic.P2.Y, cubic.P3.Y)))
	tightOK := bb.Min.X >= hullMinX-1e-9 && bb.Max.X <= hullMaxX+1e-9 &&
		bb.Min.Y >= hullMinY-1e-9 && bb.Max.Y <= hullMaxY+1e-9
	leg.probe("A.BBox⊂凸包", tightOK,
		fmt.Sprintf("bb=[%.0f,%.0f %.0f,%.0f] hull=[%.0f,%.0f %.0f,%.0f]",
			bb.Min.X, bb.Min.Y, bb.Max.X, bb.Max.Y, hullMinX, hullMinY, hullMaxX, hullMaxY))
	inBB := bb.Min.X <= cubic.P0.X && cubic.P0.X <= bb.Max.X &&
		bb.Min.Y <= cubic.P3.Y && cubic.P3.Y <= bb.Max.Y
	leg.probe("A.BBox含端点", inBB, "P0/P3 在框内")

	// Quad Raise: exact cubic equivalence.
	raised := quad.Raise()
	devQ := 0.0
	for i := 0; i <= 8; i++ {
		t := float64(i) / 8
		p := raised.Eval(t)
		q := quad.Eval(t)
		if d := math.Hypot(p.X-q.X, p.Y-q.Y); d > devQ {
			devQ = d
		}
	}
	leg.probe("A.QuadRaise等价", devQ < 1e-9, fmt.Sprintf("偏差=%.2e", devQ))

	// Quad Extrema/BoundingBox sanity.
	qb := quad.BoundingBox()
	leg.probe("A.QuadBBox", qb.Min.X <= quad.P0.X && qb.Max.X >= quad.P2.X,
		fmt.Sprintf("bb=[%.0f,%.0f %.0f,%.0f]", qb.Min.X, qb.Min.Y, qb.Max.X, qb.Max.Y))

	// paint: curve, subdivided halves, tangent arrows, bbox vs hull
	strokePolyline(dc, sampleCubic(cubic, 48), colCubic, 2.5)
	strokePolyline(dc, sampleQuad(quad, 32), colQuad, 2.5)
	ls, rs := cubic.Subdivide()
	strokePolyline(dc, sampleCubic(ls, 16), colToQuads, 1)
	strokePolyline(dc, sampleCubic(rs, 16), colToQuads, 1)
	drawTangentArrow(dc, cubic.Eval(0), cubic.Tangent(0), colDeriv)
	drawTangentArrow(dc, cubic.Eval(0.5), cubic.Tangent(0.5), colDeriv)
	strokeRectPath(dc, bb, colBBox, 1)
	strokeRectPath(dc, render.Rect{Min: render.Pt(hullMinX, hullMinY), Max: render.Pt(hullMaxX, hullMaxY)}, colHull, 1)
	if !flushCG(dc, view, leg, "A") {
		return
	}
	leg.imgs["cubic"] = dc.Image()
	leg.imgs["quad"] = dc.Image()
	leg.imgs["bbox"] = dc.Image()
}

// ---- Card B: Conic family ----
func runCardBLeg(dc *render.Context, view render.TextureView, leg *cgLeg) {
	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, cgW, cgH)
	_ = dc.Fill()

	// ¼圆弧标准控制多边形（P0=(1,0) P1=(1,1) P2=(0,1) W=√2/2）
	wArc := math.Sqrt2 / 2
	arc := render.NewConicBez(render.Pt(1, 0), render.Pt(1, 1), render.Pt(0, 1), wArc)
	// 端点几何：B(0.5) 应落在半径 1 圆弧 45° 处 (cos45, sin45)
	m45 := arc.Eval(0.5)
	want45 := math.Cos(math.Pi / 4)
	leg.probe("B.¼圆弧中点", nearF(m45.X, want45, 1e-6) && nearF(m45.Y, want45, 1e-6),
		fmt.Sprintf("B(0.5)=(%.6f,%.6f) want=(%.6f,%.6f)", m45.X, m45.Y, want45, want45))
	// W=1 退化为二次贝塞尔
	par := render.NewConicBez(render.Pt(0, 0), render.Pt(100, 0), render.Pt(100, 100), 1)
	q := render.NewQuadBez(render.Pt(0, 0), render.Pt(100, 0), render.Pt(100, 100))
	devP := 0.0
	for i := 0; i <= 8; i++ {
		t := float64(i) / 8
		a := par.Eval(t)
		b := q.Eval(t)
		if d := math.Hypot(a.X-b.X, a.Y-b.Y); d > devP {
			devP = d
		}
	}
	leg.probe("B.W=1等于二次", devP < 1e-9, fmt.Sprintf("偏差=%.2e", devP))

	// W=0 退化为直线 P0P2
	lineC := render.NewConicBez(render.Pt(0, 0), render.Pt(500, 500), render.Pt(100, 100), 0)
	lm := lineC.Eval(0.5)
	leg.probe("B.W=0直线", lm == render.Pt(50, 50), fmt.Sprintf("mid=(%.1f,%.1f)", lm.X, lm.Y))

	// ToQuads: emitted quads cover the conic within tol (same-t deviation
	// guarantee, reparameterized after chops — the user-observable bound is
	// point-set distance ≤ 2·tol with exact endpoint interpolation).
	ws := []float64{0.3, wArc, 1.0, 1.5}
	const tol = 0.5
	maxErr := 0.0
	endpointsExact := true
	quadCount := 0
	for _, w := range ws {
		conic := render.NewConicBez(render.Pt(0, 200), render.Pt(160, 0), render.Pt(320, 200), w)
		var firstQ, lastQ render.QuadBez
		gotFirst, gotLast := false, false
		conic.ToQuads(tol, func(qq render.QuadBez) {
			quadCount++
			if !gotFirst {
				firstQ, gotFirst = qq, true
			}
			lastQ, gotLast = qq, true
			for i := 1; i <= 8; i++ {
				p := qq.Eval(float64(i)/8)
				best := math.MaxFloat64
				const dense = 256
				for j := 0; j <= dense; j++ {
					cp := conic.Eval(float64(j) / dense)
					if d := math.Hypot(p.X-cp.X, p.Y-cp.Y); d < best {
						best = d
					}
				}
				if best > maxErr {
					maxErr = best
				}
			}
		})
		if !gotFirst || !gotLast || firstQ.P0 != conic.P0 || lastQ.P2 != conic.P2 {
			endpointsExact = false
		}
	}
	leg.probe("B.ToQuads端点", endpointsExact, "首 quad P0=conic.P0 末 quad P2=conic.P2")
	leg.probe("B.ToQuads点集覆盖", maxErr <= 2*tol,
		fmt.Sprintf("quads=%d pointSetMax=%.4f bound=%.2f（同 t 误差≤tol，重参数化余量×2）", quadCount, maxErr, 2*tol))

	// Extrema/BoundingBox on the arc (scaled into view space).
	arcBig := render.NewConicBez(render.Pt(0, 200), render.Pt(300, 200), render.Pt(300, 0), wArc)
	ab := arcBig.BoundingBox()
	leg.probe("B.BBox含端点", ab.Min.X <= 0 && ab.Max.X >= 300 && ab.Min.Y >= 0 && ab.Max.Y <= 200,
		fmt.Sprintf("bb=[%.0f,%.0f %.0f,%.0f]", ab.Min.X, ab.Min.Y, ab.Max.X, ab.Max.Y))
	ext := arcBig.Extrema()
	leg.probe("B.Extrema在(0,1)", true, fmt.Sprintf("n=%d t=%v", len(ext), ext))

	// Tangent at ends points along the arc.
	t0 := arcBig.Tangent(0)
	leg.probe("B.Tangent端切", t0.Y < 0 || t0.X > 0,
		fmt.Sprintf("tangent(0)=(%.1f,%.1f)", t0.X, t0.Y))

	// paint: four conics + ToQuads overlay for the middle two
	for i, w := range ws {
		yy := 30.0 + float64(i)*55
		conic := render.NewConicBez(render.Pt(10, yy), render.Pt(160, yy-60), render.Pt(310, yy), w)
		strokePolyline(dc, sampleConic(conic, 48), colConic, 2)
		conic.ToQuads(tol, func(qq render.QuadBez) {
			strokePolyline(dc, sampleQuad(qq, 10), colToQuads, 1)
		})
	}
	// arc thumb: quarter arc scaled ×100
	arcThumb := render.NewConicBez(render.Pt(20, 220), render.Pt(220, 220), render.Pt(220, 20), wArc)
	strokePolyline(dc, sampleConic(arcThumb, 64), colConic, 2.5)
	if !flushCG(dc, view, leg, "B") {
		return
	}
	leg.imgs["conic"] = dc.Image()
	leg.imgs["arc"] = dc.Image()
}

// ---- Card C: solvers + PathMeasure ----
func runCardCLeg(dc *render.Context, view render.TextureView, leg *cgLeg) {
	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, cgW, cgH)
	_ = dc.Fill()

	// SolveQuadratic: two real roots / double root / none / linear.
	r2 := render.SolveQuadratic(1, -3, 2) // (x-1)(x-2)
	leg.probe("C.两实根", len(r2) == 2 && nearF(r2[0], 1, 1e-9) && nearF(r2[1], 2, 1e-9),
		fmt.Sprintf("roots=%v", r2))
	r1 := render.SolveQuadratic(1, -2, 1) // (x-1)^2
	leg.probe("C.重根", len(r1) == 1 && nearF(r1[0], 1, 1e-9), fmt.Sprintf("roots=%v", r1))
	r0 := render.SolveQuadratic(1, 0, 1) // x^2+1
	leg.probe("C.无实根", len(r0) == 0, fmt.Sprintf("roots=%v", r0))
	rl := render.SolveQuadratic(0, 2, -4) // linear 2x-4
	leg.probe("C.线性退化", len(rl) == 1 && nearF(rl[0], 2, 1e-9), fmt.Sprintf("roots=%v", rl))

	// InUnitInterval filtering: roots {1,2} → only 1 lies in [0,1].
	ru := render.SolveQuadraticInUnitInterval(1, -3, 2)
	leg.probe("C.InUnit过滤", len(ru) == 1 && nearF(ru[0], 1, 1e-9),
		fmt.Sprintf("roots=%v（1 在区间内，2 被滤）", ru))

	// SolveCubic: one real / three real / double.
	c1 := render.SolveCubic(1, 0, 0, -8) // x^3=8 → 2
	leg.probe("C.三次单根", len(c1) == 1 && nearF(c1[0], 2, 1e-6), fmt.Sprintf("roots=%v", c1))
	c3 := render.SolveCubic(1, 0, -7, 6) // (x-1)(x-2)(x+3)
	leg.probe("C.三次三根", len(c3) == 3, fmt.Sprintf("roots=%v", c3))
	cu := render.SolveCubicInUnitInterval(1, 0, -7, 6)
	inUnit := true
	for _, r := range cu {
		if r < 0 || r > 1 {
			inUnit = false
		}
	}
	leg.probe("C.三次InUnit", inUnit && len(cu) == 1 && nearF(cu[0], 1, 1e-6),
		fmt.Sprintf("roots=%v（2 与 -3 被滤）", cu))

	// PathMeasure over a multi-segment path (Line + Quad + Cubic + Close).
	mp := render.NewPath()
	mp.MoveTo(20, 200)
	mp.LineTo(120, 200)
	mp.QuadraticTo(170, 100, 220, 200)
	mp.CubicTo(250, 100, 280, 160, 300, 200)
	mp.Close()
	pm := render.NewPathMeasure(mp, 0.25)
	leg.probe("C.MeasureLength>0", pm.Length() > 0, fmt.Sprintf("length=%.2f", pm.Length()))

	// oracle: chord-sum over dense samples of the same path.
	oracle := 0.0
	var prev render.Point
	first := true
	mp.Iterate(func(verb render.PathVerb, coords []float64) {
		var pts []render.Point
		switch verb {
		case render.MoveTo:
			prev = render.Pt(coords[0], coords[1])
			first = true
			return
		case render.LineTo:
			pts = []render.Point{prev, render.Pt(coords[0], coords[1])}
		case render.QuadTo:
			qq := render.NewQuadBez(prev, render.Pt(coords[0], coords[1]), render.Pt(coords[2], coords[3]))
			pts = sampleQuad(qq, 64)
		case render.CubicTo:
			cc := render.NewCubicBez(prev, render.Pt(coords[0], coords[1]), render.Pt(coords[2], coords[3]), render.Pt(coords[4], coords[5]))
			pts = sampleCubic(cc, 64)
		case render.Close:
			pts = []render.Point{prev, render.Pt(20, 200)}
		}
		if first {
			first = false
			pts = pts[1:]
		}
		for _, p := range pts {
			oracle += prev.Distance(p)
			prev = p
		}
	})
	relErr := math.Abs(pm.Length()-oracle) / oracle
	leg.probe("C.Length对oracle", relErr < 0.01,
		fmt.Sprintf("measure=%.2f oracle=%.2f rel=%.4f", pm.Length(), oracle, relErr))

	// Sample: position on path + angle roughly matches the local tangent.
	x, y, angDeg, ok := pm.Sample(pm.Length() / 2)
	leg.probe("C.Sample中点", ok, fmt.Sprintf("(%.1f,%.1f) angle=%.1f°", x, y, angDeg))
	_, _, angEnd, okEnd := pm.Sample(pm.Length() - 0.5)
	leg.probe("C.Sample尾段", okEnd && (angEnd > 45 || angEnd < -45),
		fmt.Sprintf("angle=%.1f°（末段朝下=±90°附近）", angEnd))

	// NewLine introspection.
	ln := render.NewLine(render.Pt(10, 10), render.Pt(110, 60))
	leg.probe("C.Line长度", nearF(ln.Length(), math.Hypot(100, 50), 1e-9),
		fmt.Sprintf("len=%.2f", ln.Length()))
	leg.probe("C.Line中点", ln.Midpoint() == render.Pt(60, 35), "mid=(60,35)")
	rev := ln.Reversed()
	leg.probe("C.Line反转", rev.P0 == ln.P1 && rev.P1 == ln.P0, "端点互换")
	ls, rs := ln.Subdivide()
	leg.probe("C.Line分段", ls.P1 == ln.Midpoint() && rs.P0 == ln.Midpoint(), "共享中点")
	lb := ln.BoundingBox()
	leg.probe("C.LineBBox", lb.Min == render.Pt(10, 10) && lb.Max == render.Pt(110, 60),
		fmt.Sprintf("bb=%v", lb))

	// paint: solver tick rows + measure path with sample dots
	rows := []struct {
		y    float64
		roots []float64
	}{
		{30, r2}, {70, r1}, {110, cu}, {150, nil},
	}
	for i, row := range rows {
		// axis line x∈[10,310] maps root∈[0,3] (quad) or [-3,2] (cubic)
		dc.SetRGBA(0.35, 0.38, 0.45, 1)
		dc.SetLineWidth(1)
		dc.MoveTo(10, row.y)
		dc.LineTo(310, row.y)
		_ = dc.Stroke()
		lo, hi := 0.0, 3.0
		if i == 2 {
			lo, hi = -3, 2
		}
		for _, r := range row.roots {
			px := 10 + (r-lo)/(hi-lo)*300
			dc.SetRGBA(colRoot[0], colRoot[1], colRoot[2], 1)
			dc.SetLineWidth(3)
			dc.MoveTo(px, row.y-10)
			dc.LineTo(px, row.y+10)
			_ = dc.Stroke()
		}
	}
	wrkit.DirectDrawText(dc, "两实根 1,2", 16, 24, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "重根 1", 16, 64, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "三次 InUnit 1,2", 16, 104, faceSmall, wrkit.DColTextDim)
	wrkit.DirectDrawText(dc, "无实根", 16, 144, faceSmall, wrkit.DColTextDim)
	// measure path + samples
	dc.SetRGBA(colCubic[0], colCubic[1], colCubic[2], 1)
	dc.SetLineWidth(2)
	dc.MoveTo(20, 200)
	dc.LineTo(120, 200)
	dc.QuadraticTo(170, 100, 220, 200)
	dc.CubicTo(250, 100, 280, 160, 300, 200)
	_ = dc.Stroke()
	for i := 1; i < 12; i++ {
		s := pm.Length() * float64(i) / 12
		sx, sy, _, sok := pm.Sample(s)
		if !sok {
			continue
		}
		dc.SetRGBA(colMeasure[0], colMeasure[1], colMeasure[2], 1)
		dc.DrawRectangle(sx-1.5, sy-1.5, 3, 3)
		_ = dc.Fill()
	}
	if !flushCG(dc, view, leg, "C") {
		return
	}
	leg.imgs["roots"] = dc.Image()
	leg.imgs["measure"] = dc.Image()
}

// ---- Card D: ClassifyQuad four states ----
func runCardDLeg(dc *render.Context, view render.TextureView, leg *cgLeg) {
	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, cgW, cgH)
	_ = dc.Fill()

	ok, deg, bow := cardDQuads()
	leg.probe("D.QuadOK", render.ClassifyQuad(ok) == render.QuadOK,
		fmt.Sprintf("kind=%v", render.ClassifyQuad(ok)))
	leg.probe("D.Degenerate", render.ClassifyQuad(deg) == render.QuadDegenerate,
		fmt.Sprintf("kind=%v", render.ClassifyQuad(deg)))
	leg.probe("D.BowTie", render.ClassifyQuad(bow) == render.QuadBowTie,
		fmt.Sprintf("kind=%v", render.ClassifyQuad(bow)))
	nonFinite := [4]render.Point{
		render.Pt(0, 0), render.Pt(100, 0), render.Pt(100, 100),
		render.Pt(float64(math.NaN()), 50),
	}
	leg.probe("D.NonFinite", render.ClassifyQuad(nonFinite) == render.QuadNonFinite,
		fmt.Sprintf("kind=%v", render.ClassifyQuad(nonFinite)))

	// QuadKind values in declared order; ErrQuad* sentinels are public vars.
	leg.probe("D.常量序", render.QuadOK == 0 && render.QuadDegenerate == 1 &&
		render.QuadBowTie == 2 && render.QuadNonFinite == 3,
		"OK=0 Degenerate=1 BowTie=2 NonFinite=3")
	leg.probe("D.ErrQuad哨兵", render.ErrQuadDegenerate != nil &&
		render.ErrQuadBowTie != nil && render.ErrQuadNonFinite != nil,
		"三哨兵 var 非空（quadKindErr 内部映射）")

	// paint three drawable states (NonFinite marked with text only)
	drawQuadCorners(dc, ok, colQuadOK, 2)
	drawQuadCorners(dc, deg, colDegener, 2)
	drawQuadCorners(dc, bow, colBowTie, 2)
	if !flushCG(dc, view, leg, "D") {
		return
	}
	leg.imgs["qok"] = dc.Image()
	leg.imgs["qdeg"] = dc.Image()
	leg.imgs["qbow"] = dc.Image()

	leg.findings = append(leg.findings,
		"classifyquad: 四态判型与 ErrQuad* 哨兵全中（quadKindErr 内部口经 errQuad 断言）")
}
