// Command ui_wr_p35_conic is the P3.5 batch-1 capability real window: the true
// rational-quadratic (conic) path verb. Three views of the same geometry —
//
//	A  exact circular arc (ConicTo, w=cos(θ/2)) vs its ExpandConicsToQuads
//	   degradation vs a cubic approximation, drawn on the same centre;
//	B  the quad-chain radial error magnified ×400 around the exact arc, so the
//	   interpolation points where the approximation touches the circle are
//	   visible and the max bulge is measurable;
//	C  the weight family (w=0 degenerate chord … w>1 overshoot) plus a
//	   non-uniform affine image, which must keep the weight and map pointwise.
//
// Gate mode freezes the sweep marker and asserts logic probes (numbers printed
// into the report), semantic pixel probes, and a zero-tolerance golden over the
// shell content (legend + body).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=6 go run ./examples/ui_wr_p35_conic -auto-only          # gate
//	GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=6 go run ./examples/ui_wr_p35_conic -auto-only  # seed once
//	go run ./examples/ui_wr_p35_conic                                    # manual, stays open
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH  = 1200, 800
	windowTitle = "gpui ui_wr_p35_conic — P3.5 conic 真 verb（精确弧/降二次/权重族）"
)

// ── single source of truth for geometry: drawing and probes both read this ──
type geomT struct {
	// Card A (body-local origin added by the panel placement).
	aCx, aCy, aR float64 // arc centre + radius inside card A
	aTheta       float64 // sweep = 90°
	// Card B magnifier.
	bCx, bCy, bR float64
	bMag         float64 // radial error magnification
	bCoarseTol   float64 // degradation tolerance used by the magnifier
	aFineTol     float64 // card A / logic probes: fine degradation tolerance
	// Card C weight family + affine sample.
	cBaseX, cBaseY float64
	cStep          float64
	cHalf          float64 // half chord length for the fixed P0/P2 pair
	// Affine demo arc (card C right side) — probes read the same numbers.
	affineCx, affineCy, affineR float64
	affineSX, affineSY          float64
}

// centeredScale scales about (px,py): T(p)·S(k)·T(−p).
func centeredScale(px, py, sx, sy float64) render.Matrix {
	return render.Translate(px, py).Multiply(render.Scale(sx, sy)).Multiply(render.Translate(-px, -py))
}

var G = geomT{
	aCx: 120, aCy: 250, aR: 200, aTheta: math.Pi / 2,
	bCx: 210, bCy: 250, bR: 180, bMag: 20, bCoarseTol: 1.0, aFineTol: 0.1,
	cBaseX: 60, cBaseY: 250, cStep: 150, cHalf: 60,
	affineCx: 780, affineCy: 230, affineR: 90, affineSX: 1.8, affineSY: 0.6,
}

// conicArc builds an exact circular arc of `theta` around (cx,cy) radius r from
// angle a0, as MoveTo + ConicTo with w=cos(theta/2). Skia semantics: the weight
// of a circular arc is cos(half angle) and the control point sits on the angle
// bisector at r/cos(theta/2) from the centre.
func conicArc(cx, cy, r, a0, theta float64) (*render.Path, render.ConicBez) {
	a1 := a0 + theta
	p0 := render.Pt(cx+r*math.Cos(a0), cy+r*math.Sin(a0))
	p2 := render.Pt(cx+r*math.Cos(a1), cy+r*math.Sin(a1))
	// control = centre + ((p0-c)+(p2-c)) / (2 cos²(theta/2))
	k := 2 * math.Cos(theta/2) * math.Cos(theta/2)
	ctrl := render.Pt(cx+((p0.X-cx)+(p2.X-cx))/k, cy+((p0.Y-cy)+(p2.Y-cy))/k)
	con := render.NewConicBez(p0, ctrl, p2, math.Cos(theta/2))
	p := render.NewPath()
	p.MoveTo(p0.X, p0.Y)
	p.ConicTo(ctrl.X, ctrl.Y, p2.X, p2.Y, con.W)
	return p, con
}

// familyConic returns the i-th weight-family segment exactly as paintCardC
// draws it (card-local coordinates), so a probe can be derived from the same
// numbers the picture uses.
func familyConic(i int) (render.ConicBez, [3]float64) {
	weights := []struct {
		w   float64
		col [3]float64
	}{
		{0, [3]float64{0.55, 0.58, 0.66}},
		{0.35, [3]float64{0.95, 0.55, 0.20}},
		{math.Cos(math.Pi / 4), [3]float64{0.95, 0.25, 0.20}},
		{1.0, [3]float64{0.30, 0.80, 0.95}},
		{1.35, [3]float64{0.75, 0.40, 0.95}},
	}
	x := G.cBaseX + float64(i)*G.cStep
	y := G.cBaseY
	p0 := render.Pt(x-G.cHalf, y+G.cHalf*0.6)
	ctrl := render.Pt(x, y-G.cHalf*1.2)
	p2 := render.Pt(x+G.cHalf, y+G.cHalf*0.6)
	return render.NewConicBez(p0, ctrl, p2, weights[i].w), weights[i].col
}

// magnifierChain walks the coarse quad degradation of `pA` and returns the
// points the magnifier draws (card B, card-local): each flattened sample pushed
// radially by MAG·signedError. The painter draws exactly these vertices and the
// probes read them back, so the picture and the assertion cannot drift apart.
func magnifierChain(pA *render.Path, cx, cy, r float64) []render.Point {
	bad := pA.ExpandConicsToQuads(G.bCoarseTol)
	var pts []render.Point
	var cur render.Point
	bad.Iterate(func(v render.PathVerb, c []float64) {
		switch v {
		case render.MoveTo:
			cur = render.Pt(c[0], c[1])
		case render.QuadTo:
			q := render.NewQuadBez(cur, render.Pt(c[0], c[1]), render.Pt(c[2], c[3]))
			for i := 0; i <= 48; i++ {
				pt := q.Eval(float64(i) / 48)
				e := radialErr(cx, cy, r, pt)
				dx, dy := pt.X-cx, pt.Y-cy
				l := math.Hypot(dx, dy)
				if l == 0 {
					continue
				}
				pts = append(pts, render.Pt(cx+dx/l*(r+G.bMag*e), cy+dy/l*(r+G.bMag*e)))
			}
			cur = q.P2
		}
	})
	return pts
}

// radialErr is |p−centre| − r (signed: + outside the circle).
func radialErr(cx, cy, r float64, p render.Point) float64 {
	return math.Hypot(p.X-cx, p.Y-cy) - r
}

// quadChainMaxErr flattens the conic path's degraded quad chain and returns the
// worst radial error plus the parameter where it happens.
func quadChainMaxErr(p *render.Path, cx, cy, r float64) (float64, float64, int) {
	worst, worstT, nQuad := 0.0, 0.0, 0
	var cur render.Point
	p.ExpandConicsToQuads(0.1).Iterate(func(v render.PathVerb, c []float64) {
		switch v {
		case render.MoveTo, render.LineTo:
			cur = render.Pt(c[0], c[1])
		case render.QuadTo:
			nQuad++
			q := render.NewQuadBez(cur, render.Pt(c[0], c[1]), render.Pt(c[2], c[3]))
			for i := 0; i <= 64; i++ {
				t := float64(i) / 64
				e := math.Abs(radialErr(cx, cy, r, q.Eval(t)))
				if e > worst {
					worst, worstT = e, t
				}
			}
			cur = q.P2
		}
	})
	return worst, worstT, nQuad
}

// ─────────────────────────── logic probes ───────────────────────────

type logicResult struct {
	name string
	ok   bool
	det  string
	num  map[string]float64
}

func runLogicProbes() []logicResult {
	out := []logicResult{}
	add := func(r logicResult) { out = append(out, r) }

	// L1 exactness of the authored circular arc: sample the conic itself.
	pA, conA := conicArc(G.aCx, G.aCy, G.aR, -G.aTheta, G.aTheta)
	maxErr := 0.0
	for i := 0; i <= 40; i++ {
		t := float64(i) / 40
		if e := math.Abs(radialErr(G.aCx, G.aCy, G.aR, conA.Eval(t))); e > maxErr {
			maxErr = e
		}
	}
	add(logicResult{"conic_arc_is_exact_circle", maxErr <= 1e-9,
		fmt.Sprintf("max radial err=%.3e over 41 samples (w=%.10f)", maxErr, conA.W),
		map[string]float64{"max_radial_err": maxErr}})

	// L2 the quad degradation is an approximation inside the declared tolerance.
	worst, worstT, nQuad := quadChainMaxErr(pA, G.aCx, G.aCy, G.aR)
	ok2 := worst > 1e-6 && worst <= 2*G.aFineTol
	add(logicResult{"expand_conics_to_quads_within_tolerance", ok2,
		fmt.Sprintf("quad chain n=%d worst bulge=%.4fpx at t=%.3f (tol=%v, 2·tol cap)", nQuad, worst, worstT, G.aFineTol),
		map[string]float64{"quad_worst_err": worst, "quad_count": float64(nQuad)}})

	// L3 the verb really is stored as a conic (round-trip through the path).
	verbs := pA.Verbs()
	coords := pA.Coords()
	ok3 := len(verbs) == 2 && verbs[1] == render.ConicTo && len(coords) >= 5
	wStored := 0.0
	if ok3 {
		wStored = coords[len(coords)-1]
	}
	add(logicResult{"conic_verb_roundtrip", ok3 && math.Abs(wStored-conA.W) < 1e-12,
		fmt.Sprintf("verbs=%d last coord w=%.10f vs authored %.10f", len(verbs), wStored, conA.W),
		map[string]float64{"stored_w": wStored}})

	ok4 := true
	pA2, _ := conicArc(0, 0, 1, 0, math.Pi/2)
	add2 := ok4 && pA2.HasConics() && !pA2.ExpandConicsToQuads(0.1).HasConics()
	add(logicResult{"has_conics_and_degrades_clean", add2,
		"HasConics true; ExpandConicsToQuads → conic-free", nil})

	// L4 quarter-disk area against the analytic πr²/4 (conic area is exact).
	disk := render.NewPath()
	disk.MoveTo(G.aCx, G.aCy)
	disk.LineTo(conA.P0.X, conA.P0.Y)
	disk.ConicTo(conA.P1.X, conA.P1.Y, conA.P2.X, conA.P2.Y, conA.W)
	disk.Close()
	wantArea := math.Pi * G.aR * G.aR / 4
	gotArea := disk.Area()
	relArea := math.Abs(gotArea-wantArea) / wantArea
	// conicArea integrates its ToQuads(1e-3) decomposition exactly, so it
	// carries that decomposition's budget (≈1e-8 relative here) instead of
	// machine precision. Mixed line+curve paths used to be inflated by a whole
	// ½[x·y] per curve run — pinned in pathops_area_convention_test.go.
	add(logicResult{"quarter_disk_area_exact", relArea <= 1e-6,
		fmt.Sprintf("Area()=%.6f vs πr²/4=%.6f (rel=%.3e)", gotArea, wantArea, relArea),
		map[string]float64{"area_rel_err": relArea}})

	// L5 Chop / Subsegment consistency on the authored conic.
	l, r0 := conA.Chop()
	mid := conA.Eval(0.5)
	subFull := conA.Subsegment(0, 1)
	ok5 := l.P2.Sub(mid).Length() <= 1e-9 && r0.P0.Sub(mid).Length() <= 1e-9 &&
		subFull.P0.Sub(conA.P0).Length() <= 1e-12 && subFull.P2.Sub(conA.P2).Length() <= 1e-12 &&
		math.Abs(subFull.W-conA.W) <= 1e-12
	add(logicResult{"chop_subsegment_consistent", ok5,
		fmt.Sprintf("chop joint |Δ|=%.3e/%.3e, Subsegment(0,1) identity w Δ=%.3e",
			l.P2.Sub(mid).Length(), r0.P0.Sub(mid).Length(), subFull.W-conA.W), nil})

	// L6 non-uniform affine keeps the weight and maps pointwise (batch-1 rule).
	m := centeredScale(G.aCx, G.aCy, G.affineSX, G.affineSY)
	tAff := pA.Transform(m)
	wAff, maxAffErr := 0.0, 0.0
	var curA render.Point
	tAff.Iterate(func(v render.PathVerb, c []float64) {
		if v == render.ConicTo {
			wAff = c[4]
			for i := 0; i <= 24; i++ {
				t := float64(i) / 24
				want := m.TransformPoint(conA.Eval(t))
				// Rebuild the same conic piece from the transformed verbs.
				got := render.NewConicBez(curA, render.Pt(c[0], c[1]), render.Pt(c[2], c[3]), c[4]).Eval(t)
				if e := want.Sub(got).Length(); e > maxAffErr {
					maxAffErr = e
				}
			}
			curA = render.Pt(c[2], c[3])
		} else if v == render.MoveTo || v == render.LineTo {
			curA = render.Pt(c[0], c[1])
		}
	})
	ok6 := math.Abs(wAff-conA.W) <= 1e-12 && maxAffErr <= 1e-9
	add(logicResult{"affine_maps_conic_keeps_weight", ok6,
		fmt.Sprintf("w %.10f→%.10f, pointwise max |Δ|=%.3e under Scale(%.1f,%.1f)",
			conA.W, wAff, maxAffErr, G.affineSX, G.affineSY),
		map[string]float64{"affine_pointwise_err": maxAffErr}})

	// L7 w<=0 degenerates to a straight segment (Skia semantics).
	deg := render.NewPath()
	deg.MoveTo(0, 0)
	deg.ConicTo(50, -40, 100, 0, 0)
	onChord := true
	var curD render.Point
	deg.Iterate(func(v render.PathVerb, c []float64) {
		switch v {
		case render.MoveTo:
			curD = render.Pt(c[0], c[1])
		case render.ConicTo:
			end := render.Pt(c[2], c[3])
			for i := 0; i <= 16; i++ {
				t := float64(i) / 16
				q := render.NewQuadBez(curD, render.Pt(c[0], c[1]), end)
				pt := q.Eval(t)
				if math.Abs(pt.Y) > 1e-9 {
					onChord = false
				}
			}
			curD = end
		}
	})
	// Path.Flatten of the degenerate conic must be the chord endpoints only.
	flat := deg.Flatten(0.01)
	add(logicResult{"weight_le_zero_degenerates_to_line", onChord && len(flat) >= 2,
		fmt.Sprintf("w=0 samples stay on chord (max |y| ≤1e-9), flatten=%d pts", len(flat)), nil})

	return out
}

// ─────────────────────────── scene ───────────────────────────

var sweepPhase float64 // animation; frozen by gate mode

// cardOrigin[i] captures pc.OriginX/Y the painter actually received, so probes
// map local geometry through the same origin the picture was drawn with.
var cardOrigin [3]render.Point

func paintCardA(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[0] = render.Pt(pc.OriginX, pc.OriginY)
	ox, oy := pc.OriginX, pc.OriginY
	dc := pc.DC
	dc.SetRGBA(0.07, 0.08, 0.10, 1)
	dc.DrawRectangle(ox, oy, sz.Width, sz.Height)
	_ = dc.Fill()

	// Paths stay card-local: rendering.FillPath/StrokePath add the box origin.
	// The two approximation chains go down first and the exact conic arc last:
	// they coincide within 1e-3 px, so the overlap must read as the exact arc.
	pA, _ := conicArc(G.aCx, G.aCy, G.aR, -G.aTheta, G.aTheta)
	rendering.StrokePath(pc, pA.ExpandConicsToQuads(G.aFineTol), 1.5, 0.25, 0.90, 0.40, 1)
	cub := render.NewPath()
	k := 0.5522847498307936 * G.aR
	cub.MoveTo(G.aCx+G.aR, G.aCy)
	cub.CubicTo(G.aCx+G.aR, G.aCy-k, G.aCx+k, G.aCy-G.aR, G.aCx, G.aCy-G.aR)
	rendering.StrokePath(pc, cub, 1.5, 0.30, 0.55, 0.95, 1)
	rendering.StrokePath(pc, pA, 3.0, 0.95, 0.25, 0.20, 1)

	// Radial guides + centre tick, so the reader sees what a radius means here.
	guide := render.NewPath()
	guide.MoveTo(G.aCx, G.aCy)
	guide.LineTo(G.aCx+G.aR, G.aCy)
	guide.MoveTo(G.aCx, G.aCy)
	guide.LineTo(G.aCx, G.aCy-G.aR)
	rendering.StrokePath(pc, guide, 1.0, 0.45, 0.50, 0.60, 0.6)
	dot := render.NewPath()
	dot.Circle(G.aCx, G.aCy, 3)
	rendering.FillPath(pc, dot, 0.90, 0.92, 0.98, 1)

	// Sweep marker along the exact arc (gate mode freezes it).
	_, con := conicArc(G.aCx, G.aCy, G.aR, -G.aTheta, G.aTheta)
	mk := render.NewPath()
	mk.Circle(con.Eval(sweepPhase).X, con.Eval(sweepPhase).Y, 6)
	rendering.FillPath(pc, mk, 1, 0.85, 0.20, 1)
}

func paintCardB(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[1] = render.Pt(pc.OriginX, pc.OriginY)
	ox, oy, dc := pc.OriginX, pc.OriginY, pc.DC
	dc.SetRGBA(0.07, 0.08, 0.10, 1)
	dc.DrawRectangle(ox, oy, sz.Width, sz.Height)
	_ = dc.Fill()

	pA, con := conicArc(G.bCx, G.bCy, G.bR, -G.aTheta, G.aTheta)
	// Reference circle (the exact arc) in red.
	rendering.StrokePath(pc, pA, 2.0, 0.95, 0.25, 0.20, 1)

	// Magnified radial error of the coarse quad chain, as one polyline.
	bandPts := magnifierChain(pA, G.bCx, G.bCy, G.bR)
	if len(bandPts) > 1 {
		band := render.NewPath()
		band.MoveTo(bandPts[0].X, bandPts[0].Y)
		for _, p := range bandPts[1:] {
			band.LineTo(p.X, p.Y)
		}
		rendering.StrokePath(pc, band, 2.0, 0.25, 0.90, 0.40, 1)
	}

	// Touch points where the approximation is exact (θ endpoints + quad midpoints).
	touch := render.NewPath()
	for _, t := range []float64{0, 0.5, 1} {
		pt := con.Eval(t)
		touch.Circle(pt.X, pt.Y, 4)
	}
	rendering.FillPath(pc, touch, 1, 0.85, 0.20, 1)
}

func paintCardC(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[2] = render.Pt(pc.OriginX, pc.OriginY)
	ox, oy, dc := pc.OriginX, pc.OriginY, pc.DC
	dc.SetRGBA(0.07, 0.08, 0.10, 1)
	dc.DrawRectangle(ox, oy, sz.Width, sz.Height)
	_ = dc.Fill()

	// Weight family on a fixed chord: P0/P2 fixed, control fixed, w swept.
	weights := []struct {
		w     float64
		label string
		col   [3]float64
	}{
		{0, "w=0 退化弦", [3]float64{0.55, 0.58, 0.66}},
		{0.35, "w=0.35", [3]float64{0.95, 0.55, 0.20}},
		{math.Cos(math.Pi / 4), "w=cos45° 精确圆弧", [3]float64{0.95, 0.25, 0.20}},
		{1.0, "w=1 抛物段", [3]float64{0.30, 0.80, 0.95}},
		{1.35, "w=1.35 过冲", [3]float64{0.75, 0.40, 0.95}},
	}
	for i, wf := range weights {
		x := G.cBaseX + float64(i)*G.cStep
		y := G.cBaseY
		p := render.NewPath()
		p.MoveTo(x-G.cHalf, y+G.cHalf*0.6)
		p.ConicTo(x, y-G.cHalf*1.2, x+G.cHalf, y+G.cHalf*0.6, wf.w)
		rendering.StrokePath(pc, p, 2.5, wf.col[0], wf.col[1], wf.col[2], 1)
		// Chord ghost, so the bulge/degenerate relation is readable.
		ch := render.NewPath()
		ch.MoveTo(x-G.cHalf, y+G.cHalf*0.6)
		ch.LineTo(x+G.cHalf, y+G.cHalf*0.6)
		rendering.StrokePath(pc, ch, 1.0, 0.30, 0.33, 0.40, 0.8)

	}

	// Non-uniform affine image of the exact arc: must stay one conic, weight kept.
	pA, con := conicArc(G.affineCx, G.affineCy, G.affineR, -math.Pi/2, math.Pi/2)
	rendering.StrokePath(pc, pA, 1.5, 0.40, 0.45, 0.55, 0.85)
	// The affine image about the arc centre: same weight, pointwise mapped.
	m := centeredScale(G.affineCx, G.affineCy, G.affineSX, G.affineSY)
	rendering.StrokePath(pc, pA.Transform(m), 3.0, 0.25, 0.90, 0.40, 1)
	mk := render.NewPath()
	mk.Circle(con.Eval(0.5).X, con.Eval(0.5).Y, 5)
	rendering.FillPath(pc, mk, 1, 0.85, 0.20, 1)
}

// ─────────────────────────── main ───────────────────────────

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze sweep, snapshot + probes + golden")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until closed)")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly {
		secs = 6
	} else if secsSet {
		wrkit.RequireMinRun(secs, "P35-conic")
	} else if *manualSeconds > 0 {
		secs = int(*manualSeconds)
	}
	wrkit.EnsureUIFace()
	if *autoOnly {
		sweepPhase = 0.35 // deterministic snapshot
	}

	logic := runLogicProbes()
	logicOK, logicTotal := 0, len(logic)
	for _, l := range logic {
		fmt.Fprintf(os.Stderr, "ui_wr_p35_conic: logic %-34s ok=%v %s\n", l.name, l.ok, l.det)
		if l.ok {
			logicOK++
		}
	}
	logicFail := logicTotal - logicOK

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: windowTitle,
		Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "P3.5 conic 真 verb — 精确弧 / 降二次 / 权重族 / 仿射保权", []string{
		"A 红=conic 精确弧(3px) 绿=降二次链 蓝=立方近似",
		"B 绿=quad 链径向误差 ×400 放大，红=精确弧基准",
		"B 黄点=近似链与圆重合处（端点+抛物中点）",
		"C 权重族：w=0 退化弦 … w=1.35 过冲",
		"C 右：非均匀 Scale 后的同一 conic（权重不变）",
		"黄点在 A/B 上沿弧扫掠（人工模式下动）",
		"门禁: 逻辑 7 条 + 像素 6 条 + 金图逐位",
	})

	// Body layout: A and B side by side on top, C full width below.
	type cardSpec struct {
		x, y, w, h float64
		title      string
		paint      func(*rendering.PaintContext, rendering.Size)
	}
	specs := []cardSpec{
		{8, 8, 436, 300, "A 精确弧 vs 降二次 vs 立方", paintCardA},
		{460, 8, 436, 300, "B 径向误差 ×400 放大镜", paintCardB},
		{8, 320, 888, 320, "C 权重族 + 非均匀仿射（保权、逐点映射）", paintCardC},
	}
	for _, cs := range specs {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cs.w, cs.h
		paint := cs.paint
		box.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
			// Cards paint in body-local coordinates; the panel origin is added
			// by the placement, so probe coordinates stay window-logical.
			paint(pc, sz)
		}
		shell.Body.Place(box, cs.x, cs.y)
		shell.Body.LabelAt(cs.title, 11, cs.x+8, cs.y+6, 0.86, 0.90, 0.96)
	}

	snapDir := filepath.Join("examples", "ui_wr_p35_conic", "testdata")
	_ = os.MkdirAll(snapDir, 0o755)
	snapPath := filepath.Join(snapDir, "p35_conic_final.png")
	basePath := filepath.Join(snapDir, "goldens", "p35_conic_final.png")

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second, WarmUp: true, SnapshotPath: snapPath,
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	var t float64
	app.Scheduler().Tickers().Add(&tickFunc{on: func(dt float64) {
		if !*autoOnly {
			t += dt
			sweepPhase = 0.5 + 0.45*math.Sin(t*0.8)
		}
		shell.NoteHUDTick(dt)
		app.ScheduleFrame()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)
	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	app.Close()
	win.Close()
	elapsed := time.Since(t0).Seconds()

	// Pixel probes in window-logical coordinates: body panel origin + card
	// placement + the same geometry constants the painters use.
	// Map a card-local point into window coordinates through the origin that
	// card's painter actually received.
	localA := func(x, y float64) (float64, float64) { return cardOrigin[0].X + x, cardOrigin[0].Y + y }
	localB := func(x, y float64) (float64, float64) { return cardOrigin[1].X + x, cardOrigin[1].Y + y }
	localC := func(x, y float64) (float64, float64) { return cardOrigin[2].X + x, cardOrigin[2].Y + y }

	// Card A: the red exact arc is the topmost 3px stroke, the approximation
	// chains are 1.5px under it, so ±1.0 px off the arc is red-only ink and
	// +8 px is bare background — the arc position pinned to better than a pixel.
	ax, ay := localA(G.aCx, G.aCy)
	c45 := -math.Pi / 4
	mkRing := func(dr float64) (float64, float64) {
		return ax + (G.aR+dr)*math.Cos(c45), ay + (G.aR+dr)*math.Sin(c45)
	}
	redOutX, redOutY := mkRing(1.0)
	redInX, redInY := mkRing(-1.0)
	bgX, bgY := mkRing(8.0)

	// Card B: pick the magnified band vertex nearest the arc's own mid point.
	_, bcon := conicArc(G.bCx, G.bCy, G.bR, -G.aTheta, G.aTheta)
	bandPath, _ := conicArc(G.bCx, G.bCy, G.bR, -G.aTheta, G.aTheta)
	band := magnifierChain(bandPath, G.bCx, G.bCy, G.bR)
	// Sample the band away from the interpolation points (0, ½, 1), where the
	// degradation is exact and the magnified chain collapses onto the arc.
	bandPt, midPt := bcon.Eval(0.3), bcon.Eval(0.3)
	if len(band) > 0 {
		wantAng := math.Atan2(midPt.Y-G.bCy, midPt.X-G.bCx)
		bestD := math.Inf(1)
		for _, q := range band {
			d := math.Abs(math.Atan2(q.Y-G.bCy, q.X-G.bCx) - wantAng)
			if d < bestD {
				bestD, bandPt = d, q
			}
		}
	}
	bandX, bandY := localB(bandPt.X, bandPt.Y)
	touchX, touchY := localB(bcon.Eval(0.5).X, bcon.Eval(0.5).Y)

	// Card C: every family apex is the same ConicBez the painter strokes.
	famAt := func(i int) (float64, float64, [3]float64) {
		con, col := familyConic(i)
		x, y := localC(con.Eval(0.5).X, con.Eval(0.5).Y)
		return x, y, col
	}
	w035x, w035y, w035c := famAt(1)
	wExx, wExy, wExc := famAt(2)
	wPax, wPay, wPac := famAt(3)
	wOvx, wOvy, wOvc := famAt(4)
	_, acon := conicArc(G.affineCx, G.affineCy, G.affineR, -math.Pi/2, math.Pi/2)
	amatrix := centeredScale(G.affineCx, G.affineCy, G.affineSX, G.affineSY)
	amap := amatrix.TransformPoint(acon.Eval(0.5))
	affX, affY := localC(amap.X, amap.Y)

	probes := []wrkit.PixelProbe{
		{Name: "A_red_ink_just_outside_arc", X: redOutX, Y: redOutY, R: 0.949, G: 0.251, B: 0.200, Tol: 60},
		{Name: "A_red_ink_just_inside_arc", X: redInX, Y: redInY, R: 0.949, G: 0.251, B: 0.200, Tol: 60},
		{Name: "A_no_ink_outside_arc", X: bgX, Y: bgY, R: 0.071, G: 0.078, B: 0.098, Tol: 12},
		{Name: "B_magnifier_band_green", X: bandX, Y: bandY, R: 0.251, G: 0.902, B: 0.400, Tol: 70},
		// The chain touches the exact arc at its interpolation point: the yellow
		// marker drawn there is what the magnified band must pass under.
		{Name: "B_touch_point_marker", X: touchX, Y: touchY, R: 1.0, G: 0.851, B: 0.200, Tol: 60},
		{Name: "C_w035_apex", X: w035x, Y: w035y, R: w035c[0], G: w035c[1], B: w035c[2], Tol: 70},
		{Name: "C_wcos45_apex", X: wExx, Y: wExy, R: wExc[0], G: wExc[1], B: wExc[2], Tol: 70},
		{Name: "C_w1_apex", X: wPax, Y: wPay, R: wPac[0], G: wPac[1], B: wPac[2], Tol: 70},
		{Name: "C_w135_apex", X: wOvx, Y: wOvy, R: wOvc[0], G: wOvc[1], B: wOvc[2], Tol: 70},
		{Name: "C_affine_apex_mint", X: affX, Y: affY, R: 0.251, G: 0.902, B: 0.400, Tol: 70},
	}
	var pixelErr error
	for _, pr := range probes {
		if err := wrkit.CheckPixelProbes(snapPath, winW, []wrkit.PixelProbe{pr}); err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_p35_conic: pixel %-32s FAIL %v\n", pr.Name, err)
			if pixelErr == nil {
				pixelErr = err
			}
		} else {
			fmt.Fprintf(os.Stderr, "ui_wr_p35_conic: pixel %-32s ok\n", pr.Name)
		}
	}

	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGate(snapPath, basePath, wrkit.ShellContentRects())

	extra := map[string]any{
		"fps_gate": "off_correctness", "slope_gate": "off",
		"apis":            "Path.ConicTo/HasConics/ExpandConicsToQuads, ConicBez.Eval/Chop/Subsegment, Path.Transform 保权",
		"logic_probes_ok": logicOK, "logic_probes_total": logicTotal, "logic_failed": logicFail,
		"pixel_golden_diff_pct": goldenDiff, "pixel_golden_total_px": goldenPx,
		"pixel_golden_first_run": seeded,
		"quad_tol":               G.aFineTol,
		"radial_err_exact_arc":   logic[0].num["max_radial_err"],
		"quad_worst_bulge":       logic[1].num["quad_worst_err"],
		"area_rel_err":           logic[3].num["area_rel_err"],
		"affine_pointwise_err":   logic[5].num["affine_pointwise_err"],
	}
	for i, l := range logic {
		extra[fmt.Sprintf("logic_%02d_%s", i+1, l.name)] = l.ok
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "P35-conic", Scenario: "ui_wr_p35_conic", Backend: win.Backend().String(),
		Snap: app.Metrics().Snapshot(), PresentCount: app.PresentCount(), ElapsedSec: elapsed,
		SurfaceAreaPx: winW * winH, Warmup: true, Extra: extra,
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		if *autoOnly {
			os.Exit(1)
		}
	}
	if logicOK != logicTotal {
		fail("logic probes %d/%d ok", logicOK, logicTotal)
	}
	if pixelErr != nil {
		fail("pixel probes: %v", pixelErr)
	}
	if goldenErr != nil {
		fail("golden: %v", goldenErr)
	}
	if !seeded && goldenDiff != 0 {
		fail("golden diff %.4f%% want 0 over %d px", goldenDiff, goldenPx)
	}
	if *autoOnly {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1, RequireFullPaintPolicy: true}); err != nil {
			fail("%v", err)
		}
	}
	fmt.Fprintf(os.Stderr, "ui_wr_p35_conic: OK presents=%d logic=%d/%d golden=%.4f%% seeded=%v gate=%v elapsed=%.1fs\n",
		app.PresentCount(), logicOK, logicTotal, goldenDiff, seeded, *autoOnly, elapsed)
}

type tickFunc struct{ on func(dt float64) }

func (t *tickFunc) Tick(dt float64) bool { t.on(dt); return true }
