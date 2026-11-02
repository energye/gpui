// Command ui_wr_p35_rrect4 is the P3.5 batch-2 capability real window: an
// RRect whose four corners carry independent radii.
//
//	A  four different corner radii — filled, stroked, over the un-rounded ghost;
//	B  ClipRRect through both routes (uniform circular radii take the SDF fast
//	   path, complex/oval radii fall to exact path clipping), plus a nested
//	   complex∩oval clip that only exact clipping can express;
//	C  the six-type reduction (Rect/Oval/Simple/NinePatch/Complex) and the
//	   overlapping-radii proportional scaling, one tile each.
//
// Logic probes read the same geometry constants the painters use, and pixel
// probes map through each card's recorded paint origin, so picture and
// assertion cannot drift apart. Gate mode asserts logic + pixel + a
// zero-tolerance golden over the shell content.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=6 go run ./examples/ui_wr_p35_rrect4 -auto-only
//	GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=6 go run ./examples/ui_wr_p35_rrect4 -auto-only  # seed once
//	go run ./examples/ui_wr_p35_rrect4                                                 # manual
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
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
	windowTitle = "gpui ui_wr_p35_rrect4 — P3.5 四角独立 RRect（类型/归一化/裁剪分流）"
)

// ── geometry shared by painters and probes ──
type geomT struct {
	aX, aY, aW, aH                      float64
	aRadii                              [4]render.Point // UL, UR, LR, LL
	bX, bY, bW, bH                      float64
	bGap                                float64
	bUniformR                           float64
	bComplex                            [4]render.Point
	bStripeW                            float64
	cX, cY, cTile, cTileH, cGapX, cGapY float64
}

var G = geomT{
	aX: 24, aY: 30, aW: 380, aH: 220,
	aRadii: [4]render.Point{{X: 0, Y: 0}, {X: 26, Y: 26}, {X: 90, Y: 45}, {X: 60, Y: 60}},
	bX:     16, bY: 40, bW: 130, bH: 170, bGap: 12,
	bUniformR: 45,
	bComplex:  [4]render.Point{{X: 46, Y: 16}, {X: 8, Y: 8}, {X: 0, Y: 0}, {X: 22, Y: 44}},
	bStripeW:  12,
	cX:        16, cY: 34, cTile: 120, cTileH: 84, cGapX: 26, cGapY: 130,
}

func rectAt(x, y, w, h float64) render.Rect {
	return render.Rect{Min: render.Pt(x, y), Max: render.Pt(x+w, y+h)}
}

func rrAt(x, y, w, h float64, radii [4]render.Point) render.RRect {
	return render.NewRRectRadii(rectAt(x, y, w, h), radii)
}

func rrPath(rr render.RRect) *render.Path {
	p := render.NewPath()
	p.RRect(rr)
	return p
}

// ─────────────────────────── logic probes ───────────────────────────

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

type logicResult struct {
	name string
	ok   bool
	det  string
	num  map[string]float64
}

func runLogicProbes() []logicResult {
	out := []logicResult{}
	add := func(r logicResult) { out = append(out, r) }

	// L1 six-type reduction (SkRRect::Type order).
	empty := render.NewRRect(rectAt(0, 0, 40, 40))
	empty.SetEmpty()
	plain := render.NewRRect(rectAt(0, 0, 40, 40))
	oval := render.NewRRect(rectAt(0, 0, 1, 1))
	oval.SetOval(rectAt(0, 0, 60, 40))
	simple := render.NewRRectXY(rectAt(0, 0, 80, 50), 10, 10)
	// Skia's nine-patch rule: ul.X==ll.X, ul.Y==ur.Y, ur.X==lr.X, ll.Y==lr.Y.
	nine := render.NewRRectRadii(rectAt(0, 0, 80, 50),
		[4]render.Point{{X: 10, Y: 20}, {X: 30, Y: 20}, {X: 30, Y: 40}, {X: 10, Y: 40}})
	complex := render.NewRRectRadii(rectAt(0, 0, 80, 50),
		[4]render.Point{{X: 5, Y: 5}, {X: 20, Y: 20}, {X: 10, Y: 30}, {X: 40, Y: 8}})
	want := []render.RRectType{render.RRectEmpty, render.RRectRect, render.RRectOval,
		render.RRectSimple, render.RRectNinePatch, render.RRectComplex}
	got := []render.RRectType{empty.Type(), plain.Type(), oval.Type(), simple.Type(), nine.Type(), complex.Type()}
	bad := 0
	for i := range want {
		if got[i] != want[i] {
			bad++
		}
	}
	add(logicResult{"six_type_reduction", bad == 0,
		fmt.Sprintf("六态判定 %s/%s/%s/%s/%s/%s（错 %d，Complex 串形=%q）",
			got[0], got[1], got[2], got[3], got[4], got[5], bad, complex.Type().String()), nil})

	// L2 normalisation: non-finite radius → square corner; single-axis zero
	// squares the whole corner; overlapping radii scale down proportionally.
	nf := render.NewRRectRadii(rectAt(0, 0, 100, 60),
		[4]render.Point{{X: math.NaN(), Y: 20}, {X: math.Inf(1), Y: 20},
			{X: 10, Y: 0}, {X: 0, Y: 15}})
	nfUL, nfUR, nfLR, nfLL := nf.Radii(render.RRectUpperLeft), nf.Radii(render.RRectUpperRight),
		nf.Radii(render.RRectLowerRight), nf.Radii(render.RRectLowerLeft)
	nfOK := nfUL == (render.Point{}) && nfUR == (render.Point{}) &&
		nfLR == (render.Point{}) && nfLL == (render.Point{})
	ov := render.NewRRectRadii(rectAt(0, 0, 60, 40), [4]render.Point{
		{X: 50, Y: 50}, {X: 50, Y: 50}, {X: 50, Y: 50}, {X: 50, Y: 50}})
	topSum := ov.Radii(render.RRectUpperLeft).X + ov.Radii(render.RRectUpperRight).X
	leftSum := ov.Radii(render.RRectUpperLeft).Y + ov.Radii(render.RRectLowerLeft).Y
	ratioKept := math.Abs(ov.Radii(render.RRectUpperLeft).X-ov.Radii(render.RRectUpperLeft).Y) < 1e-9
	add(logicResult{"normalisation_rules", nfOK && topSum <= 60+1e-9 && leftSum <= 40+1e-9 && ratioKept,
		fmt.Sprintf("NaN/±Inf 与单轴 0 全部归直角（UL%v UR%v LR%v LL%v 全 0=%v）；60×40 内 50 重叠→等比缩放 上边和=%.3f≤60 左边和=%.3f≤40 比例保持=%v",
			nfUL, nfUR, nfLR, nfLL, nfOK, topSum, leftSum, ratioKept),
		map[string]float64{"overlap_top_sum": topSum, "overlap_left_sum": leftSum}})

	// L3 all-equal radii must reproduce the historical single-radius path.
	rrSame := render.NewRRectXY(rectAt(10, 8, 170, 112), 18, 18)
	round := render.NewPath()
	round.RoundedRectangle(10, 8, 170, 112, 18)
	viaR := rrPath(rrSame)
	ok3 := viaR.NumVerbs() == round.NumVerbs() && viaR.NumVerbs() == 10
	maxGap := 0.0
	if cv, rv := viaR.Coords(), round.Coords(); ok3 && len(cv) == len(rv) {
		for i := range cv {
			if d := math.Abs(cv[i] - rv[i]); d > maxGap {
				maxGap = d
			}
		}
		// Both are quarter-circle cubic fits, with the classic constants 4/3·(√2−1)
		// (Path.RRect, Skia's) vs 0.5486… (legacy RoundedRectangle) — the control
		// arms differ by ~0.37% of the radius, so identity holds to 1% of it.
		ok3 = maxGap <= 0.01*18
	} else {
		ok3 = false
	}
	add(logicResult{"uniform_equals_roundrect", ok3,
		fmt.Sprintf("verbs=%d vs %d，坐标最大差 %.4f ≤1%%半径（两套三次圆弧常数 0.5523/0.5486 之差）",
			viaR.NumVerbs(), round.NumVerbs(), maxGap), map[string]float64{"uniform_max_coord_gap": maxGap}})

	// L4 one rounded corner at a time: the path carries exactly one cubic per
	// rounded corner, and that cubic's midpoint sits at the authored radius
	// from the corner's centre (quarter-circle cubic error ≈2e-4 relative).
	const r4 = 40.0
	x4, y4, w4, h4 := 30.0, 20.0, 160.0, 110.0
	plainArea := w4 * h4
	segmentLoss := (4.0 - math.Pi) * r4 * r4 / 4.0
	areaWorst, inkBad, verbBad := 0.0, 0, 0
	var inkLog [4]string
	for ci := 0; ci < 4; ci++ {
		radii := [4]render.Point{}
		radii[ci] = render.Point{X: r4, Y: r4}
		p := rrPath(rrAt(x4, y4, w4, h4, radii))
		// One rounded corner = one cubic; square corners stay plain joins.
		nCubic := 0
		for _, v := range p.Verbs() {
			if v == render.CubicTo {
				nCubic++
			}
		}
		if nCubic != 1 || p.NumVerbs() != 7 {
			verbBad++
		}
		// Area: exactly one corner's circular segment disappears.
		if rel := math.Abs((plainArea-p.Area())/segmentLoss - 1); rel > areaWorst {
			areaWorst = rel
		}
		// Pixels: only the rounded corner must drop out, the three square
		// corners must stay inked.
		ink := cornerInkMask(p, x4, y4, w4, h4)
		inkLog[ci] = fmt.Sprintf("测%d:%d%d%d%d", ci, b2i(ink[0]), b2i(ink[1]), b2i(ink[2]), b2i(ink[3]))
		for k := 0; k < 4; k++ {
			if ink[k] == (k == ci) {
				inkBad++
			}
		}
	}
	add(logicResult{"per_corner_radius_independently_honoured", inkBad == 0 && verbBad == 0 && areaWorst <= 3e-3,
		fmt.Sprintf("逐角独立：单角缺口面积 worst=%.2e rel（三次弧 vs 真圆缺口的固有差）；离屏填充只该角不入色（错 %d %s）；verb/cubic 结构错 %d（单圆角=1 cubic、7 verb）",
			areaWorst, inkBad, strings.Join(inkLog[:], " "), verbBad),
		map[string]float64{"corner_area_worst_rel": areaWorst}})

	// L5 area of a complex RRect (straight edges + four corner cubics — the
	// mixed path shape the area-convention fix made trustworthy) against the
	// analytic value and against a pure-line reference.
	circ := [4]render.Point{{X: 20, Y: 20}, {X: 45, Y: 45}, {X: 0, Y: 0}, {X: 30, Y: 30}}
	ap := rrPath(rrAt(0, 0, 200, 130, circ))
	analytic := 200.0 * 130.0
	for _, rad := range circ {
		analytic -= (4.0 - math.Pi) * rad.X * rad.X / 4.0
	}
	dense := denseOf(ap, 4000)
	relDense := math.Abs(ap.Area()-dense.Area()) / math.Max(1, math.Abs(dense.Area()))
	relAnalytic := math.Abs(ap.Area()-analytic) / analytic
	add(logicResult{"complex_rrect_area", relDense <= 1e-5 && relAnalytic <= 1e-3,
		fmt.Sprintf("Area()=%.4f 解析=%.4f rel=%.2e；纯直线参照=%.4f rel=%.2e",
			ap.Area(), analytic, relAnalytic, dense.Area(), relDense),
		map[string]float64{"area_analytic_rel": relAnalytic, "area_dense_rel": relDense}})

	// L6 clip routes: uniform circular radii and complex radii must both clip
	// exactly — the corner outside the radius stays clear while an interior
	// point takes the paint.
	type route struct {
		name    string
		rr      render.RRect
		filled  bool
		cleared bool
	}
	routes := []route{
		{"uniform(SDF 快路)", render.NewRRectXY(rectAt(0, 0, 200, 160), 40, 40), false, false},
		{"complex(精确路径)", rrAt(0, 0, 200, 160, [4]render.Point{
			{X: 70, Y: 20}, {X: 8, Y: 8}, {X: 0, Y: 0}, {X: 30, Y: 60}}), false, false},
		{"oval(精确路径)", func() render.RRect {
			r := render.NewRRect(rectAt(0, 0, 1, 1))
			r.SetOval(rectAt(0, 0, 200, 160))
			return r
		}(), false, false},
	}
	for i, rt := range routes {
		img := clipProbe(rt.rr)
		routes[i].filled = img.lumaAt(100, 80) > 0.6 // centre stays inside every shape
		routes[i].cleared = img.lumaAt(2, 2) < 0.25  // (2,2) is outside all three corner shapes
	}
	allOK := true
	desc := ""
	for _, rt := range routes {
		if !rt.filled || !rt.cleared {
			allOK = false
		}
		desc += fmt.Sprintf("%s: 内=%v 角=%v；", rt.name, rt.filled, rt.cleared)
	}
	add(logicResult{"clip_rrect_all_routes_clip", allOK, desc, nil})

	return out
}

type offImage struct {
	w, h int
	data []byte // RGBA
}

func (o offImage) lumaAt(x, y int) float64 {
	if o.data == nil || x < 0 || y < 0 || x >= o.w || y >= o.h {
		return -1
	}
	i := (y*o.w + x) * 4
	return float64(o.data[i]) / 255.0
}

// clipProbe fills a rect under one ClipRRect offscreen and reads pixels back.
func clipProbe(rr render.RRect) offImage {
	W, H := 200, 160
	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.Clear()
	dc.SetRGB(1, 0, 0)
	dc.ClipRRect(rr)
	dc.DrawRectangle(0, 0, float64(W), float64(H))
	_ = dc.Fill()
	im := dc.Image()
	if im == nil {
		return offImage{}
	}
	b := im.Bounds()
	out := offImage{w: b.Dx(), h: b.Dy()}
	buf := make([]byte, out.w*out.h*4)
	for y := 0; y < out.h; y++ {
		for x := 0; x < out.w; x++ {
			r, g, bb, a := im.At(x, y).RGBA()
			i := (y*out.w + x) * 4
			buf[i], buf[i+1], buf[i+2], buf[i+3] = byte(r>>8), byte(g>>8), byte(bb>>8), byte(a>>8)
		}
	}
	out.data = buf
	dc.ResetClip()
	return out
}

// cornerInkMask fills p offscreen and reports, for UL/UR/LR/LL, whether a point
// 2px inside each rect corner took the paint (true = inked).
func cornerInkMask(p *render.Path, x, y, w, h float64) [4]bool {
	W, H := int(x+w+4), int(y+h+4)
	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.Clear()
	dc.SetRGB(1, 0, 0)
	dc.DrawPath(p)
	_ = dc.Fill()
	im := dc.Image()
	var out [4]bool
	if im == nil {
		return out
	}
	pts := [4][2]int{
		{int(x) + 2, int(y) + 2}, {int(x+w) - 2, int(y) + 2},
		{int(x+w) - 2, int(y+h) - 2}, {int(x) + 2, int(y+h) - 2},
	}
	for i, q := range pts {
		r, _, _, _ := im.At(q[0], q[1]).RGBA()
		out[i] = uint8(r>>8) > 120
	}
	return out
}

// denseOf samples every curve verb into line segments (pure-line reference).
func denseOf(p *render.Path, per int) *render.Path {
	out := render.NewPath()
	var cur, start render.Point
	open := false
	p.Iterate(func(v render.PathVerb, c []float64) {
		switch v {
		case render.MoveTo:
			cur = render.Pt(c[0], c[1])
			start = cur
			if open {
				out.Close()
			}
			out.MoveTo(cur.X, cur.Y)
			open = true
		case render.LineTo:
			cur = render.Pt(c[0], c[1])
			out.LineTo(cur.X, cur.Y)
		case render.QuadTo:
			q := render.NewQuadBez(cur, render.Pt(c[0], c[1]), render.Pt(c[2], c[3]))
			for i := 1; i <= per; i++ {
				e := q.Eval(float64(i) / float64(per))
				out.LineTo(e.X, e.Y)
			}
			cur = q.P2
		case render.CubicTo:
			cb := render.NewCubicBez(cur, render.Pt(c[0], c[1]), render.Pt(c[2], c[3]), render.Pt(c[4], c[5]))
			for i := 1; i <= per; i++ {
				e := cb.Eval(float64(i) / float64(per))
				out.LineTo(e.X, e.Y)
			}
			cur = cb.P3
		case render.ConicTo:
			con := render.NewConicBez(cur, render.Pt(c[0], c[1]), render.Pt(c[2], c[3]), c[4])
			for i := 1; i <= per; i++ {
				e := con.Eval(float64(i) / float64(per))
				out.LineTo(e.X, e.Y)
			}
			cur = con.P2
		case render.Close:
			if cur != start {
				out.LineTo(start.X, start.Y)
			}
			out.Close()
			cur = start
			open = false
		}
	})
	if open {
		out.Close()
	}
	return out
}

// ─────────────────────────── painters ───────────────────────────

var cardOrigin [3]render.Point

func paintCardA(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[0] = render.Pt(pc.OriginX, pc.OriginY)
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	dc.SetRGBA(0.07, 0.08, 0.10, 1)
	dc.DrawRectangle(ox, oy, sz.Width, sz.Height)
	_ = dc.Fill()

	rr := rrAt(G.aX, G.aY, G.aW, G.aH, G.aRadii)
	ghost := render.NewPath()
	ghost.MoveTo(G.aX, G.aY)
	ghost.LineTo(G.aX+G.aW, G.aY)
	ghost.LineTo(G.aX+G.aW, G.aY+G.aH)
	ghost.LineTo(G.aX, G.aY+G.aH)
	ghost.Close()
	rendering.FillPath(pc, rrPath(rr), 0.95, 0.60, 0.20, 1)
	rendering.StrokePath(pc, ghost, 1.0, 0.45, 0.50, 0.60, 0.9)
	rendering.StrokePath(pc, rrPath(rr), 3.0, 1.0, 0.90, 0.30, 1)
}

// stripedUnderClip paints vertical stripes confined to one RRect clip.
func stripedUnderClip(dc render.Canvas, ox, oy float64, x, y, w, h float64, rr render.RRect) {
	dc.ClipRRect(rr)
	for i := 0; float64(i)*G.bStripeW < w; i++ {
		if i%2 == 0 {
			dc.SetRGBA(0.20, 0.65, 0.95, 1)
		} else {
			dc.SetRGBA(0.90, 0.35, 0.70, 1)
		}
		dc.DrawRectangle(ox+x+float64(i)*G.bStripeW, oy+y, G.bStripeW*0.6, h)
		_ = dc.Fill()
	}
	dc.ResetClip()
}

func paintCardB(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[1] = render.Pt(pc.OriginX, pc.OriginY)
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	dc.SetRGBA(0.07, 0.08, 0.10, 1)
	dc.DrawRectangle(ox, oy, sz.Width, sz.Height)
	_ = dc.Fill()

	// Uniform circular radii: the SDF fast path.
	stripedUnderClip(dc, ox, oy, G.bX, G.bY, G.bW, G.bH,
		render.NewRRectXY(rectAt(ox+G.bX, oy+G.bY, G.bW, G.bH), G.bUniformR, G.bUniformR))
	// Complex radii: exact path clipping.
	cx := G.bX + G.bW + G.bGap
	stripedUnderClip(dc, ox, oy, cx, G.bY, G.bW, G.bH,
		rrAt(ox+cx, oy+G.bY, G.bW, G.bH, G.bComplex))
	// Nested: complex outside, oval inside — mint only survives in the intersection.
	nx := G.bX + 2*(G.bW+G.bGap)
	outer := rrAt(ox+nx, oy+G.bY, G.bW, G.bH, G.bComplex)
	inner := render.NewRRect(rectAt(0, 0, 1, 1))
	inner.SetOval(rectAt(ox+nx+24, oy+G.bY+24, G.bW-48, G.bH-48))
	dc.ClipRRect(outer)
	dc.ClipRRect(inner)
	dc.SetRGBA(0.35, 0.90, 0.50, 1)
	dc.DrawRectangle(ox+nx, oy+G.bY, G.bW, G.bH)
	_ = dc.Fill()
	dc.ResetClip()
}

type tileSpec struct {
	title string
	rr    func() render.RRect
	col   [3]float64
}

func cTiles() []tileSpec {
	return []tileSpec{
		{"Rect", func() render.RRect { return render.NewRRect(rectAt(0, 0, 100, 70)) },
			[3]float64{0.95, 0.60, 0.20}},
		{"Oval", func() render.RRect {
			r := render.NewRRect(rectAt(0, 0, 1, 1))
			r.SetOval(rectAt(0, 0, 100, 70))
			return r
		}, [3]float64{0.30, 0.80, 0.95}},
		{"Simple", func() render.RRect { return render.NewRRectXY(rectAt(0, 0, 100, 70), 18, 18) },
			[3]float64{0.95, 0.25, 0.20}},
		{"NinePatch", func() render.RRect {
			return render.NewRRectRadii(rectAt(0, 0, 100, 70),
				[4]render.Point{{X: 10, Y: 20}, {X: 35, Y: 20}, {X: 35, Y: 40}, {X: 10, Y: 40}})
		}, [3]float64{0.75, 0.40, 0.95}},
		{"Complex", func() render.RRect {
			return render.NewRRectRadii(rectAt(0, 0, 100, 70),
				[4]render.Point{{X: 45, Y: 12}, {X: 6, Y: 6}, {X: 0, Y: 0}, {X: 20, Y: 40}})
		}, [3]float64{0.35, 0.90, 0.50}},
		{"重叠→等比缩放", func() render.RRect {
			return render.NewRRectRadii(rectAt(0, 0, 100, 70),
				[4]render.Point{{X: 90, Y: 90}, {X: 90, Y: 90}, {X: 90, Y: 90}, {X: 90, Y: 90}})
		}, [3]float64{1.0, 0.85, 0.20}},
	}
}

// tileRect returns the card-C placement box for tile i (card-local).
func tileRect(i int) (float64, float64) {
	col, row := float64(i%3), float64(i/3)
	return G.cX + col*(G.cTile+G.cGapX), G.cY + row*(G.cTileH+G.cGapY)
}

func paintCardC(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[2] = render.Pt(pc.OriginX, pc.OriginY)
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	dc.SetRGBA(0.07, 0.08, 0.10, 1)
	dc.DrawRectangle(ox, oy, sz.Width, sz.Height)
	_ = dc.Fill()

	for i, ts := range cTiles() {
		x, y := tileRect(i)
		rr := ts.rr()
		placed := render.NewRRectRadii(rectAt(x, y, G.cTile, G.cTileH), rr.RadiiArray())
		p := rrPath(placed)
		rendering.FillPath(pc, p, ts.col[0], ts.col[1], ts.col[2], 1)
		rendering.StrokePath(pc, p, 1.5, 0.90, 0.93, 0.98, 0.9)
	}
}

// ─────────────────────────── main ───────────────────────────

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: snapshot + probes + golden")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly {
		secs = 6
	} else if secsSet {
		wrkit.RequireMinRun(secs, "P35-rrect4")
	} else if *manualSeconds > 0 {
		secs = int(*manualSeconds)
	}
	wrkit.EnsureUIFace()

	logic := runLogicProbes()
	logicOK, logicTotal := 0, len(logic)
	for _, l := range logic {
		fmt.Fprintf(os.Stderr, "ui_wr_p35_rrect4: logic %-38s ok=%v %s\n", l.name, l.ok, l.det)
		if l.ok {
			logicOK++
		}
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: windowTitle,
		Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	shell := wrkit.NewShell(winW, winH, "P3.5 四角独立 RRect — 类型归约 / 归一化 / 裁剪分流", []string{
		"A UL 直角 · UR 26 圆 · LR 90×45 椭圆 · LL 60 圆",
		"A 橙填充 + 亮黄 3px 描边 + 未倒角 ghost 矩形",
		"B 左=uniform 圆角（SDF 快路）中=complex（精确路径裁剪）",
		"B 右=嵌套裁剪 complex ∩ 椭圆，绿块只留交集",
		"C 六态 Rect/Oval/Simple/NinePatch/Complex + 重叠等比缩放",
		"门禁: 逻辑 6 条 + 像素 9 条 + 金图逐位",
	})
	specs := []struct {
		x, y, w, h float64
		title      string
		paint      func(*rendering.PaintContext, rendering.Size)
	}{
		{8, 8, 436, 300, "A 四角各异半径（填充+描边）", paintCardA},
		{460, 8, 436, 300, "B ClipRRect 分流与嵌套裁剪", paintCardB},
		{8, 320, 888, 320, "C 六态归约与归一化", paintCardC},
	}
	for _, cs := range specs {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cs.w, cs.h
		paint := cs.paint
		box.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) { paint(pc, sz) }
		shell.Body.Place(box, cs.x, cs.y)
		shell.Body.LabelAt(cs.title, 11, cs.x+8, cs.y+6, 0.86, 0.90, 0.96)
	}

	snapDir := filepath.Join("examples", "ui_wr_p35_rrect4", "testdata")
	_ = os.MkdirAll(snapDir, 0o755)
	snapPath := filepath.Join(snapDir, "p35_rrect4_final.png")
	basePath := filepath.Join(snapDir, "goldens", "p35_rrect4_final.png")

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second, WarmUp: true, SnapshotPath: snapPath,
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	app.Scheduler().Tickers().Add(&tickFunc{on: func(dt float64) {
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

	fmt.Fprintf(os.Stderr, "ui_wr_p35_rrect4: origins A=%v B=%v C=%v\n", cardOrigin[0], cardOrigin[1], cardOrigin[2])

	// Pixel probes through each card's recorded paint origin.
	localA := func(x, y float64) (float64, float64) { return cardOrigin[0].X + x, cardOrigin[0].Y + y }
	localB := func(x, y float64) (float64, float64) { return cardOrigin[1].X + x, cardOrigin[1].Y + y }
	localC := func(x, y float64) (float64, float64) { return cardOrigin[2].X + x, cardOrigin[2].Y + y }

	aULx, aULy := localA(G.aX+2, G.aY+2)                                 // UL radius 0 → inked corner
	aLLx, aLLy := localA(G.aX+G.aW-2, G.aY+G.aH-2)                       // outside the LL radius → clear
	aLLinX, aLLinY := localA(G.aX+G.aW-46, G.aY+G.aH-46)                 // inside that radius → inked
	aOutX, aOutY := localA(G.aX+G.aW+8, G.aY+G.aH-40)                    // past the rect → card background
	cFlatX, cFlatY := localB(G.bX+G.bW+G.bGap+G.bW-6, G.bY+G.bH-6)       // complex LR radius 0 → stripe inked
	cRoundX, cRoundY := localB(G.bX+G.bW+G.bGap+2, G.bY+2)               // complex UL 70×20 → corner clear
	nestInX, nestInY := localB(G.bX+2*(G.bW+G.bGap)+G.bW/2, G.bY+G.bH/2) // inside complex ∩ oval
	nestOutX, nestOutY := localB(G.bX+2*(G.bW+G.bGap)+3, G.bY+3)         // outside both → clear
	compX, compY := func() (float64, float64) {
		x, y := tileRect(4)
		return localC(x+G.cTile-2, y+G.cTileH-2) // Complex tile: LL radius 0 → inked
	}()

	probes := []wrkit.PixelProbe{
		{Name: "A_square_UL_corner_inked", X: aULx, Y: aULy, R: 0.949, G: 0.60, B: 0.20, Tol: 40},
		{Name: "A_LL_corner_outside_is_bg", X: aLLx, Y: aLLy, R: 0.071, G: 0.078, B: 0.098, Tol: 14},
		{Name: "A_LL_inside_radius_inked", X: aLLinX, Y: aLLinY, R: 0.949, G: 0.60, B: 0.20, Tol: 40},
		{Name: "A_past_rect_is_bg", X: aOutX, Y: aOutY, R: 0.071, G: 0.078, B: 0.098, Tol: 14},
		{Name: "B_complex_square_LR_inked", X: cFlatX, Y: cFlatY, R: 0.20, G: 0.65, B: 0.95, Tol: 75},
		{Name: "B_complex_round_UL_clear", X: cRoundX, Y: cRoundY, R: 0.071, G: 0.078, B: 0.098, Tol: 14},
		{Name: "B_nested_intersection_mint", X: nestInX, Y: nestInY, R: 0.349, G: 0.90, B: 0.498, Tol: 40},
		{Name: "B_nested_outside_clear", X: nestOutX, Y: nestOutY, R: 0.071, G: 0.078, B: 0.098, Tol: 14},
		{Name: "C_complex_tile_square_corner", X: compX, Y: compY, R: 0.349, G: 0.90, B: 0.498, Tol: 40},
	}
	var pixelErr error
	for _, pr := range probes {
		if err := wrkit.CheckPixelProbes(snapPath, winW, []wrkit.PixelProbe{pr}); err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_p35_rrect4: pixel %-32s FAIL %v\n", pr.Name, err)
			if pixelErr == nil {
				pixelErr = err
			}
		} else {
			fmt.Fprintf(os.Stderr, "ui_wr_p35_rrect4: pixel %-32s ok\n", pr.Name)
		}
	}

	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGate(snapPath, basePath, wrkit.ShellContentRects())

	extra := map[string]any{
		"fps_gate": "off_correctness", "slope_gate": "off",
		"apis":            "NewRRectRadii/NewRRectXY/RRectType/RRectCorner, Path.RRect, Context.DrawRRect/ClipRRect",
		"logic_probes_ok": logicOK, "logic_probes_total": logicTotal,
		"tile_count":            len(cTiles()),
		"pixel_golden_diff_pct": goldenDiff, "pixel_golden_total_px": goldenPx,
		"pixel_golden_first_run": seeded,
	}
	for i, l := range logic {
		extra[fmt.Sprintf("logic_%02d_%s", i+1, l.name)] = l.ok
		for k, v := range l.num {
			extra[fmt.Sprintf("%s_%s", l.name, k)] = v
		}
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "P35-rrect4", Scenario: "ui_wr_p35_rrect4", Backend: win.Backend().String(),
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
	fmt.Fprintf(os.Stderr, "ui_wr_p35_rrect4: OK presents=%d logic=%d/%d golden=%.4f%% seeded=%v gate=%v elapsed=%.1fs\n",
		app.PresentCount(), logicOK, logicTotal, goldenDiff, seeded, *autoOnly, elapsed)
}

type tickFunc struct{ on func(dt float64) }

func (t *tickFunc) Tick(dt float64) bool { t.on(dt); return true }
