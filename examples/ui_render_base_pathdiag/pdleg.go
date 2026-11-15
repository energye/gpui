// pdleg.go — offscreen direct legs for path construction/query. PathBuilder
// shapes, DetectShape six-way decisions, path queries and Append are exercised
// on offscreen views; readbacks assert the observable state. One GL-hub job.
package main

import (
	"fmt"
	"image"
	"math"
	"os"

	"github.com/energye/gpui/render"
)

const (
	pdW, pdH = 320, 240 // leg context size
)

type pdLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
	arcA0    float64
	arcA1    float64
	rrectR   float64
}

func (l *pdLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "pdleg: %s\n", line)
}

func nearP(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func flushPD(dc *render.Context, view render.TextureView, leg *pdLeg, tag string) bool {
	if err := dc.PresentFrame(view, pdW, pdH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	return true
}

func clearPD(dc *render.Context) {
	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, pdW, pdH)
	_ = dc.Fill()
}

// ---- Card A: builder shapes ----
func runCardALeg(dc *render.Context, view render.TextureView, leg *pdLeg) {
	clearPD(dc)

	circle := render.BuildPath().Circle(60, 60, 40).Build()
	leg.probe("A.Circle verbs", circle.NumVerbs() == 6, fmt.Sprintf("n=%d（MoveTo+4×CubicTo+Close）", circle.NumVerbs()))
	ds := render.DetectShape(circle)
	leg.probe("A.Circle判定", ds.Kind == render.ShapeCircle,
		fmt.Sprintf("kind=%v c=(%.0f,%.0f) r=%.0f", ds.Kind, ds.CenterX, ds.CenterY, ds.RadiusX))
	leg.probe("A.Circle参数", nearP(ds.CenterX, 60, 1e-3) && nearP(ds.CenterY, 60, 1e-3) && nearP(ds.RadiusX, 40, 1e-3),
		"center=(60,60) r=40")

	star := render.BuildPath().Star(160, 60, 45, 20, 5).Build()
	dsStar := render.DetectShape(star)
	leg.probe("A.Star判定", dsStar.Kind == render.ShapeUnknown,
		fmt.Sprintf("kind=%v（星形不识别为简单基元）", dsStar.Kind))
	leg.probe("A.Star verbs", star.NumVerbs() == 11,
		fmt.Sprintf("n=%d（MoveTo+10×LineTo+Close）", star.NumVerbs()))

	rr := render.BuildPath().RoundRect(220, 20, 90, 80, 12).Build()
	dsRR := render.DetectShape(rr)
	leg.probe("A.RoundRect判定", dsRR.Kind == render.ShapeRRect,
		fmt.Sprintf("kind=%v", dsRR.Kind))
	leg.rrectR = dsRR.CornerRadius
	leg.probe("A.RoundRect参数", nearP(dsRR.Width, 90, 1e-3) && nearP(dsRR.Height, 80, 1e-3) && nearP(dsRR.CornerRadius, 12, 1e-3),
		fmt.Sprintf("w=%.0f h=%.0f r=%.1f", dsRR.Width, dsRR.Height, dsRR.CornerRadius))

	poly := render.BuildPath().Polygon(60, 170, 40, 6).Build()
	leg.probe("A.Polygon verbs", poly.NumVerbs() == 7, fmt.Sprintf("n=%d", poly.NumVerbs()))

	ellipse := render.BuildPath().Ellipse(180, 170, 50, 30).Build()
	dsEl := render.DetectShape(ellipse)
	leg.probe("A.Ellipse判定", dsEl.Kind == render.ShapeEllipse,
		fmt.Sprintf("kind=%v rx=%.0f ry=%.0f", dsEl.Kind, dsEl.RadiusX, dsEl.RadiusY))

	// Build/Path alias: same pointer.
	b := render.BuildPath()
	leg.probe("A.Build别名", b.Build() == b.Path(), "同一 *Path")

	// paint
	fillPath := func(p *render.Path, c [3]float64) {
		dc.SetPath(p)
		dc.SetRGBA(c[0], c[1], c[2], 1)
		_ = dc.Fill()
	}
	fillPath(circle, colCircle)
	fillPath(star, colStar)
	fillPath(rr, colRRect)
	fillPath(poly, colPoly)
	if !flushPD(dc, view, leg, "A") {
		return
	}
	leg.imgs["circle"] = dc.Image()
	leg.imgs["star"] = dc.Image()
	leg.imgs["shapes"] = dc.Image()
}

// ---- Card B: DetectShape decision matrix ----
func runCardBLeg(dc *render.Context, view render.TextureView, leg *pdLeg) {
	clearPD(dc)

	// rect: MoveTo + 3×LineTo + Close
	rectP := render.NewPath()
	rectP.MoveTo(20, 20)
	rectP.LineTo(100, 20)
	rectP.LineTo(100, 70)
	rectP.LineTo(20, 70)
	rectP.Close()
	dsRect := render.DetectShape(rectP)
	leg.probe("B.Rect判定", dsRect.Kind == render.ShapeRect,
		fmt.Sprintf("kind=%v w=%.0f h=%.0f c=(%.0f,%.0f)", dsRect.Kind, dsRect.Width, dsRect.Height, dsRect.CenterX, dsRect.CenterY))

	// rrect: MoveTo + (LineTo+CubicTo)*4 + Close
	rrectP := render.BuildPath().RoundRect(130, 20, 80, 50, 10).Build()
	dsRR := render.DetectShape(rrectP)
	leg.probe("B.RRect判定", dsRR.Kind == render.ShapeRRect, fmt.Sprintf("kind=%v", dsRR.Kind))

	// arc: open circular arc (MoveTo + CubicTo chain, no close) — build with
	// the kappa construction along a 120° arc of radius 50 centered (230,60).
	buildArc := func() *render.Path {
		p := render.NewPath()
		cx, cy, r := 230.0, 60.0, 50.0
		a0, a1 := math.Pi, math.Pi*2.0/3.0 // 180°→300° sweep through top
		const n = 4
		p.MoveTo(cx+r*math.Cos(a0), cy+r*math.Sin(a0))
		step := (a1 - a0) / n
		for i := 0; i < n; i++ {
			sa, ea := a0+float64(i)*step, a0+float64(i+1)*step
			half := (ea - sa) / 2
			k := 4.0 / 3.0 * math.Tan(half/4)
			p.CubicTo(
				cx+r*math.Cos(sa)-k*r*math.Sin(sa), cy+r*math.Sin(sa)+k*r*math.Cos(sa),
				cx+r*math.Cos(ea)+k*r*math.Sin(ea), cy+r*math.Sin(ea)-k*r*math.Cos(ea),
				cx+r*math.Cos(ea), cy+r*math.Sin(ea))
		}
		return p
	}
	arcP := buildArc()
	dsArc := render.DetectShape(arcP)
	leg.probe("B.Arc判定", dsArc.Kind == render.ShapeArc,
		fmt.Sprintf("kind=%v r=%.1f", dsArc.Kind, dsArc.RadiusX))
	leg.probe("B.Arc参数", nearP(dsArc.CenterX, 230, 0.05) && nearP(dsArc.CenterY, 60, 0.05) && nearP(dsArc.RadiusX, 50, 0.05),
		fmt.Sprintf("c=(%.2f,%.2f) r=%.2f", dsArc.CenterX, dsArc.CenterY, dsArc.RadiusX))
	leg.arcA0, leg.arcA1 = dsArc.Angle0, dsArc.Angle1
	sweep := dsArc.Angle1 - dsArc.Angle0
	leg.probe("B.Arc扫角", sweep > 0 && sweep < 2*math.Pi,
		fmt.Sprintf("a0=%.2f a1=%.2f sweep=%.2f°", dsArc.Angle0, dsArc.Angle1, sweep*180/math.Pi))

	// unknown: zigzag
	zig := render.NewPath()
	zig.MoveTo(20, 120)
	zig.LineTo(60, 150)
	zig.LineTo(100, 120)
	zig.LineTo(140, 150)
	leg.probe("B.折线Unknown", render.DetectShape(zig).Kind == render.ShapeUnknown, "非基元")

	// nil/empty
	leg.probe("B.空路径Unknown", render.DetectShape(render.NewPath()).Kind == render.ShapeUnknown, "0 verb")

	// Shape* constants full enumeration.
	leg.probe("B.Shape常量", render.ShapeUnknown == 0 && render.ShapeCircle == 1 &&
		render.ShapeEllipse == 2 && render.ShapeRect == 3 && render.ShapeRRect == 4 && render.ShapeArc == 5,
		"Unknown=0..Arc=5")

	// ellipse for paint
	ellipse := render.BuildPath().Ellipse(180, 170, 50, 30).Build()

	// paint the six decisions
	fillStroke := func(p *render.Path, c [3]float64) {
		dc.SetPath(p)
		dc.SetRGBA(c[0], c[1], c[2], 1)
		_ = dc.Fill()
	}
	fillStroke(rectP, colRectD)
	fillStroke(rrectP, colRRect)
	fillStroke(arcP, colArcD)
	fillStroke(ellipse, colCircle)
	fillStroke(zig, colPoly)
	if !flushPD(dc, view, leg, "B") {
		return
	}
	leg.imgs["detect"] = dc.Image()
	leg.imgs["detectzoom"] = dc.Image()
}

// ---- Card C: path queries ----
func runCardCLeg(dc *render.Context, view render.TextureView, leg *pdLeg) {
	clearPD(dc)

	p := render.NewPath()
	leg.probe("C.空路径无当前点", !p.HasCurrentPoint(), "HasCurrentPoint=false")
	p.MoveTo(30, 60)
	leg.probe("C.MoveTo当前点", p.HasCurrentPoint() && p.CurrentPoint() == render.Pt(30, 60),
		"(30,60)")
	p.LineTo(120, 40)
	leg.probe("C.LineTo当前点", p.CurrentPoint() == render.Pt(120, 40), "(120,40)")
	p.CubicTo(150, 80, 190, 20, 230, 60)
	leg.probe("C.Cubic当前点", p.CurrentPoint() == render.Pt(230, 60), "(230,60)")
	p.Close()
	leg.probe("C.Close当前点", p.CurrentPoint() == render.Pt(30, 60), "回到子路起点")
	p.MoveTo(250, 120)
	p.LineTo(300, 180)

	// Reversed: each subpath reversed independently; first vertex of reversed
	// == last vertex of original subpath.
	rev := p.Reversed()
	revVerbs := rev.Verbs()
	leg.probe("C.Reversed同verb数", len(revVerbs) == len(p.Verbs()),
		fmt.Sprintf("%d→%d", len(p.Verbs()), len(revVerbs)))
	// walk reversed: first MoveTo should be (300,180) (last subpath end).
	foundStart := false
	rev.Iterate(func(verb render.PathVerb, coords []float64) {
		if verb == render.MoveTo && !foundStart {
			foundStart = coords[0] == 300 && coords[1] == 180
		}
	})
	leg.probe("C.Reversed首点", foundStart, "反转后首 MoveTo=(300,180)")

	// Line.Reversed covered in curvegeom; here assert Path.Reversed paintable.
	leg.probe("C.Reversed可画", rev.NumVerbs() > 0, "可作填充路径")

	// FlattenCallback: samples within tol of the true cubic.
	flat := render.NewPath()
	flat.MoveTo(30, 180)
	flat.CubicTo(80, 100, 160, 240, 230, 160)
	tol := 0.1
	var pts []render.Point
	flat.FlattenCallback(tol, func(pt render.Point) { pts = append(pts, pt) })
	leg.probe("C.Flatten采样数", len(pts) >= 2, fmt.Sprintf("n=%d", len(pts)))
	// cubic at t=0.5 should be within tol of some flat sample
	mid := render.CubicBez{P0: render.Pt(30, 180), P1: render.Pt(80, 100), P2: render.Pt(160, 240), P3: render.Pt(230, 160)}.Eval(0.5)
	best := math.MaxFloat64
	for _, pt := range pts {
		if d := math.Hypot(pt.X-mid.X, pt.Y-mid.Y); d < best {
			best = d
		}
	}
	leg.probe("C.Flatten覆盖中点", best <= tol*2,
		fmt.Sprintf("t=0.5 最近采样距离=%.4f tol=%.1f", best, tol))

	// paint: original + reversed overlay with direction arrows
	dc.SetPath(p)
	dc.SetRGBA(colQuery[0], colQuery[1], colQuery[2], 1)
	dc.SetLineWidth(2.5)
	if err := dc.Stroke(); err != nil {
		leg.probe("C.paint原路径", false, err.Error())
	}
	dc.SetPath(rev)
	dc.SetRGBA(colReversed[0], colReversed[1], colReversed[2], 1)
	dc.SetLineWidth(1)
	if err := dc.Stroke(); err != nil {
		leg.probe("C.paint反转", false, err.Error())
	}
	dc.SetPath(flat)
	dc.SetRGBA(0.30, 0.70, 0.90, 1)
	dc.SetLineWidth(1.5)
	if err := dc.Stroke(); err != nil {
		leg.probe("C.paintflatten", false, err.Error())
	}
	for _, pt := range pts {
		dc.SetRGBA(0.90, 0.85, 0.30, 1)
		dc.DrawRectangle(pt.X-1, pt.Y-1, 2, 2)
		_ = dc.Fill()
	}
	if !flushPD(dc, view, leg, "C") {
		return
	}
	leg.imgs["query"] = dc.Image()
	leg.imgs["flatten"] = dc.Image()
}

// ---- Card D: Append + alias ----
func runCardDLeg(dc *render.Context, view render.TextureView, leg *pdLeg) {
	clearPD(dc)

	a := render.NewPath()
	a.MoveTo(20, 20)
	a.LineTo(140, 20)
	a.LineTo(140, 90)
	a.Close()
	b := render.NewPath()
	b.MoveTo(170, 40)
	b.LineTo(290, 40)
	b.LineTo(290, 120)
	b.Close()
	va, vb := len(a.Verbs()), len(b.Verbs())
	a.Append(b)
	leg.probe("D.Append verb数", len(a.Verbs()) == va+vb,
		fmt.Sprintf("%d+%d=%d", va, vb, len(a.Verbs())))
	// Append 拼接产生 ≥2 子路 × Fill —— GPU 模式 op 入队但画面为空（先红钉子
	// render/setpath_multisub_fill_test.go 同族：Append 直拼连普通双三角形都
	// 空白，比钉子的环洞更强）。窗内画面用已证可用形态：dc.MoveTo 直画双子路。
	dc.MoveTo(20, 20)
	dc.LineTo(140, 20)
	dc.LineTo(140, 90)
	dc.ClosePath()
	dc.MoveTo(170, 40)
	dc.LineTo(290, 40)
	dc.LineTo(290, 120)
	dc.ClosePath()
	dc.SetRGBA(colAppend[0], colAppend[1], colAppend[2], 1)
	if err := dc.Fill(); err != nil {
		leg.probe("D.双子路直画", false, err.Error())
	} else {
		leg.probe("D.双子路直画", true, "dc 直画两三角形一笔填充")
	}

	// alias pointer assert
	bb := render.BuildPath()
	bb.Circle(60, 170, 25)
	leg.probe("D.Build别名指针", bb.Build() == bb.Path(), "Build()==Path()")

	// paint alias circle thumb
	dc.SetPath(bb.Build())
	dc.SetRGBA(colCircle[0], colCircle[1], colCircle[2], 1)
	_ = dc.Fill()
	if !flushPD(dc, view, leg, "D") {
		return
	}
	leg.imgs["append"] = dc.Image()
	leg.imgs["alias"] = dc.Image()

	leg.findings = append(leg.findings,
		"append-multisub: Path.Append 拼接 ≥2 子路 × Fill GPU 模式画面为空（仅 AA 边缘碎片），与先红钉子 setpath-multisub 同族且更强——普通双三角形也空白；dc.MoveTo 直画双子路已证可用。钉子 render/setpath_multisub_fill_test.go 已覆盖，修复面 render/internal/gpu FillPath 链台账待修")
}

// runPDLegs drives all four cards' offscreen legs. One GL-hub job.
func runPDLegs() *pdLeg {
	leg := &pdLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(pdW, pdH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(pdW, pdH)
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
