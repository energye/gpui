package main

import (
	"fmt"
	"image"
	"math"
	"os"

	"github.com/energye/gpui/render"
)

// ---------------------------------------------------------------------------
// Value-level contract probes (no rendering device)
// ---------------------------------------------------------------------------

type probeResult struct {
	Key    string
	OK     bool
	Detail string
}

func boolRect(x, y, w, h float64) *render.Path {
	p := render.NewPath()
	p.Rectangle(x, y, w, h)
	return p
}

func boolContours(p *render.Path) [][]render.Point {
	if p == nil {
		return nil
	}
	var out [][]render.Point
	var cur []render.Point
	p.Iterate(func(v render.PathVerb, c []float64) {
		switch v {
		case render.MoveTo:
			cur = []render.Point{render.Pt(c[0], c[1])}
		case render.LineTo:
			cur = append(cur, render.Pt(c[0], c[1]))
		case render.QuadTo:
			cur = append(cur, render.Pt(c[2], c[3]))
		case render.CubicTo:
			cur = append(cur, render.Pt(c[4], c[5]))
		case render.Close:
			if len(cur) > 0 {
				out = append(out, cur)
				cur = nil
			}
		}
	})
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

func boolInside(p *render.Path, x, y float64) bool {
	return p != nil && p.Winding(render.Pt(x, y)) != 0
}

func boolHasPoint(pts []render.Point, x, y, tol float64) bool {
	for _, p := range pts {
		if math.Abs(p.X-x) <= tol && math.Abs(p.Y-y) <= tol {
			return true
		}
	}
	return false
}

func boolHasVertices(contours [][]render.Point, want [][2]float64) bool {
	tol := 1.0 / 512
	for _, w := range want {
		found := false
		for _, c := range contours {
			if boolHasPoint(c, w[0], w[1], tol) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func boolFloatBounds(p *render.Path) (minX, minY, maxX, maxY float64) {
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for _, c := range boolContours(p) {
		for _, pt := range c {
			minX = math.Min(minX, pt.X)
			minY = math.Min(minY, pt.Y)
			maxX = math.Max(maxX, pt.X)
			maxY = math.Max(maxY, pt.Y)
		}
	}
	return
}

func runLogicProbes() (probes, failed []string) {
	add := func(key string, ok bool, detail string) {
		probes = append(probes, key)
		if !ok {
			failed = append(failed, key)
		}
		status := "ok"
		if !ok {
			status = "FAIL"
		}
		fmt.Fprintf(os.Stderr, "ui_render_base_pathbool: logic %-30s %s: %s\n", key, status, detail)
	}

	// 1. Difference of two rects produces exactly two contours (frame + hole),
	// 10 verbs, and the hole stays empty.
	{
		res := boolRect(16, 16, 480, 480).Op(boolRect(96, 96, 320, 320), render.PathOpDifference)
		cs := boolContours(res)
		ok := len(cs) == 2 && res.NumVerbs() == 10 && !res.HasCurves() &&
			len(cs[0]) == 4 && len(cs[1]) == 4 &&
			boolInside(res, 20, 20) && !boolInside(res, 200, 200) && boolInside(res, 480, 480)
		add("pathbool_diff_contours", ok,
			fmt.Sprintf("contours=%d verbs=%d curves=%v", len(cs), res.NumVerbs(), res.HasCurves()))
	}

	// 2. Fractional input/output must stay exact to sub-pixel precision; not
	// quantized to the integer raster grid.
	{
		res := boolRect(16.25, 16.25, 480, 480).Op(boolRect(96.5, 96.5, 320, 320), render.PathOpDifference)
		cs := boolContours(res)
		want := [][2]float64{
			{16.25, 16.25}, {496.25, 16.25}, {496.25, 496.25}, {16.25, 496.25},
			{96.5, 96.5}, {416.5, 96.5}, {416.5, 416.5}, {96.5, 416.5},
		}
		ok := len(cs) == 2 && boolHasVertices(cs, want)
		add("pathbool_diff_fractional", ok, fmt.Sprintf("contours=%d exact_vertices=%v", len(cs), ok))
	}

	// 3. Union of overlapping rects is one contour, with the overlap filled
	// once and no internal seam.
	{
		res := boolRect(0, 0, 100, 100).Op(boolRect(50, 0, 100, 100), render.PathOpUnion)
		cs := boolContours(res)
		ok := len(cs) == 1 && res.NumVerbs() == 5 && !res.HasCurves() &&
			boolInside(res, 25, 50) && boolInside(res, 75, 50) && boolInside(res, 125, 50) &&
			!boolInside(res, 160, 50)
		add("pathbool_union_overlap", ok,
			fmt.Sprintf("contours=%d verbs=%d overlap_inside=%v", len(cs), res.NumVerbs(), boolInside(res, 75, 50)))
	}

	// 4. Fractional diamond intersect rect keeps the exact analytic vertices
	// (same fixture as render/path_boolean_test.go).
	{
		dia := render.NewPath()
		dia.MoveTo(10.5, 0.25)
		dia.LineTo(20.75, 10.5)
		dia.LineTo(10.5, 20.25)
		dia.LineTo(0.25, 10.5)
		dia.Close()
		res := dia.Op(boolRect(5, 5, 10, 10), render.PathOpIntersect)
		cs := boolContours(res)
		want := [][2]float64{{5, 5.75}, {5.75, 5}, {15, 5}, {15, 15}, {5, 15}}
		ok := len(cs) == 1 && len(cs[0]) == 5 && boolHasVertices(cs, want) &&
			boolInside(res, 10, 10) && !boolInside(res, 2, 10)
		add("pathbool_intersect_exact", ok,
			fmt.Sprintf("contours=%d verts=%d exact=%v", len(cs), len(cs[0]), ok))
	}

	// 5. XOR keeps the two non-overlapping lobes and removes the overlap.
	{
		res := boolRect(0, 0, 100, 100).Op(boolRect(40, 0, 100, 100), render.PathOpXor)
		cs := boolContours(res)
		ok := len(cs) == 2 && boolInside(res, 10, 50) && boolInside(res, 120, 50) &&
			!boolInside(res, 50, 50) && !boolInside(res, 70, 50)
		add("pathbool_xor_lobes", ok,
			fmt.Sprintf("contours=%d left=%v overlap=%v right=%v", len(cs),
				boolInside(res, 10, 50), boolInside(res, 50, 50), boolInside(res, 120, 50)))
	}

	// 6. Geometry beyond 2048 px is preserved exactly (no silent clip/tile cap).
	{
		res := boolRect(3000, 16, 480, 480).Op(boolRect(3080, 96, 320, 320), render.PathOpDifference)
		minX, minY, maxX, maxY := boolFloatBounds(res)
		ok := len(boolContours(res)) == 2 && res.NumVerbs() == 10 &&
			minX == 3000 && minY == 16 && maxX == 3480 && maxY == 496 &&
			boolInside(res, 3005, 20) && !boolInside(res, 3200, 200) && boolInside(res, 3475, 490)
		add("pathbool_far_no_2048", ok,
			fmt.Sprintf("bounds=(%.0f,%.0f)-(%.0f,%.0f) contours=%d", minX, minY, maxX, maxY, len(boolContours(res))))
	}

	// 7. Output winding must satisfy NonZero and EvenOdd identically: the
	// engine's boolean output is usable under both fill rules.
	{
		frame := boolRect(4, 4, 36, 36).Op(boolRect(12, 12, 20, 20), render.PathOpDifference)
		frame = frame.Op(boolRect(18, 18, 8, 8), render.PathOpUnion)
		mismatch := 0
		for x := 4.5; x < 40; x += 1.5 {
			for y := 4.5; y < 40; y += 1.5 {
				w := frame.Winding(render.Pt(x, y))
				if (w != 0) != (w%2 != 0) {
					mismatch++
				}
			}
		}
		ok := mismatch == 0 && boolInside(frame, 5.5, 5.5) &&
			!boolInside(frame, 13.5, 13.5) && boolInside(frame, 20.5, 20.5)
		add("pathbool_fill_rule_agreement", ok,
			fmt.Sprintf("mismatch=%d band=%v hole=%v island=%v", mismatch,
				boolInside(frame, 5.5, 5.5), boolInside(frame, 13.5, 13.5), boolInside(frame, 20.5, 20.5)))
	}

	// 8. Curves are flattened to a polygonal output (documented scope limit),
	// and the flattened boundary stays inside the analytic tolerance.
	{
		circle := render.NewPath()
		circle.Circle(0, 0, 20)
		res := circle.Op(boolRect(-25, -10, 50, 20), render.PathOpIntersect)
		maxR := 0.0
		for _, c := range boolContours(res) {
			for _, pt := range c {
				maxR = math.Max(maxR, math.Hypot(pt.X, pt.Y))
			}
		}
		ok := !res.HasCurves() && res.NumVerbs() > 10 && maxR <= 20.2 &&
			boolInside(res, 0, 0) && !boolInside(res, 0, 15)
		add("pathbool_curves_flattened", ok,
			fmt.Sprintf("verbs=%d curves=%v max_radius=%.3f", res.NumVerbs(), res.HasCurves(), maxR))
	}

	// 9. Empty operand identities: empty ∪ rect = rect; rect − rect = empty.
	{
		empty := render.NewPath()
		u := empty.Op(boolRect(10, 10, 20, 20), render.PathOpUnion)
		d := boolRect(10, 10, 20, 20).Op(boolRect(10, 10, 20, 20), render.PathOpDifference)
		ok := len(boolContours(u)) == 1 && boolInside(u, 15, 15) && d.NumVerbs() == 0
		add("pathbool_empty_identity", ok,
			fmt.Sprintf("union_verbs=%d diff_verbs=%d", u.NumVerbs(), d.NumVerbs()))
	}

	// 10. Self-intersecting bowtie is split into its two filled lobes; the
	// crossing gaps stay empty.
	{
		bow := render.NewPath()
		bow.MoveTo(0, 0)
		bow.LineTo(10, 10)
		bow.LineTo(10, 0)
		bow.LineTo(0, 10)
		bow.Close()
		res := bow.Op(boolRect(-50, -50, 200, 200), render.PathOpIntersect)
		ok := boolInside(res, 1, 2) && boolInside(res, 9, 2) &&
			boolInside(res, 1, 8) && boolInside(res, 9, 8) &&
			!boolInside(res, 5, 1) && !boolInside(res, 5, 9)
		add("pathbool_bowtie_lobes", ok,
			fmt.Sprintf("contours=%d lobes=%v,%v,%v,%v", len(boolContours(res)),
				boolInside(res, 1, 2), boolInside(res, 9, 2), boolInside(res, 1, 8), boolInside(res, 9, 8)))
	}

	return probes, failed
}

// ---------------------------------------------------------------------------
// Pixel probes (GPU snapshot)
// ---------------------------------------------------------------------------

type band struct {
	ch       int
	min, max float64
}

func b8(ch int, lo, hi byte) band {
	return band{ch: ch, min: float64(lo) / 255, max: float64(hi) / 255}
}

type probeSpec struct {
	key, desc string
	card      int
	pt        render.Point
	want      func(ox, oy, dpr float64, devPX, devPY int) (r, g, b float64)
	bands     []band
}

func blendRGB(bg, fill [3]float64, cov float64) (r, g, b float64) {
	f := func(i int) float64 { return bg[i]*(1-cov) + fill[i]*cov }
	return f(0), f(1), f(2)
}

const tolSolid = 8.0 / 255

var probeSpecs []probeSpec

func bandsOf(c [3]float64) []band {
	pad := 0.07
	return []band{
		{ch: 0, min: math.Max(0, c[0]-pad), max: math.Min(1, c[0]+pad)},
		{ch: 1, min: math.Max(0, c[1]-pad), max: math.Min(1, c[1]+pad)},
		{ch: 2, min: math.Max(0, c[2]-pad), max: math.Min(1, c[2]+pad)},
	}
}

func init() {
	solid := func(c [3]float64) func(float64, float64, float64, int, int) (float64, float64, float64) {
		return func(float64, float64, float64, int, int) (float64, float64, float64) {
			return c[0], c[1], c[2]
		}
	}
	add := func(key, desc string, card int, pt render.Point,
		want func(float64, float64, float64, int, int) (float64, float64, float64), bands ...band) {
		probeSpecs = append(probeSpecs, probeSpec{key, desc, card, pt, want, bands})
	}

	// A: difference frame + hole, same path under NonZero (left) / EvenOdd
	// (right). The fractional hole's left edge is sampled analytically.
	add("a_left_band", "A left NonZero frame band = teal", 0, render.Pt(20, 20),
		solid(colFillA), bandsOf(colFillA)...)
	add("a_left_hole", "A left NonZero hole = card BG", 0, render.Pt(94, 74),
		solid(colCardBG), bandsOf(colCardBG)...)
	add("a_right_band", "A right EvenOdd frame band = teal", 0, render.Pt(238, 20),
		solid(colFillA), bandsOf(colFillA)...)
	add("a_right_hole", "A right EvenOdd hole = card BG", 0, render.Pt(312, 74),
		solid(colCardBG), bandsOf(colCardBG)...)
	add("a_frac_inside", "A fractional hole interior = card BG", 0, render.Pt(45, 74),
		solid(colCardBG), bandsOf(colCardBG)...)
	add("a_outside", "A outside frames = card BG", 0, render.Pt(200, 170),
		solid(colCardBG), bandsOf(colCardBG)...)

	// B: union of two overlapping circles; alpha fill makes a double-fill seam
	// visible, so the overlap sample must be exactly one source-over blend.
	union := func() [3]float64 {
		r, g, b := blendRGB(colCardBG, colFillB, unionAlpha)
		return [3]float64{r, g, b}
	}()
	add("b_left", "B union left lobe = one alpha blend", 1, render.Pt(110, 96),
		solid(union), bandsOf(union)...)
	add("b_overlap", "B union overlap = still one alpha blend (no seam)", 1, render.Pt(183, 96),
		solid(union), bandsOf(union)...)
	add("b_right", "B union right lobe = one alpha blend", 1, render.Pt(286, 96),
		solid(union), bandsOf(union)...)
	add("b_outside", "B outside union = card BG", 1, render.Pt(40, 96),
		solid(colCardBG), bandsOf(colCardBG)...)

	// C: diamond ∩ circle, opaque fill; source outlines are drawn on top but
	// the probes stay well inside/outside both inputs.
	add("c_center", "C intersection center = purple", 2, render.Pt(203, 96),
		solid(colFillC), bandsOf(colFillC)...)
	add("c_left", "C intersection left = purple", 2, render.Pt(150, 96),
		solid(colFillC), bandsOf(colFillC)...)
	add("c_diamond_only", "C diamond-only region = card BG", 2, render.Pt(285, 96),
		solid(colCardBG), bandsOf(colCardBG)...)
	add("c_circle_only", "C circle-only region = card BG", 2, render.Pt(203, 158),
		solid(colCardBG), bandsOf(colCardBG)...)

	// D: XOR at x≈3000, translated into the card after the operation. The
	// overlap (center) is empty; both crescents are filled.
	add("d_left", "D far-XOR left crescent = green", 3, render.Pt(120, 96),
		solid(colFillD), bandsOf(colFillD)...)
	add("d_overlap", "D far-XOR overlap = card BG", 3, render.Pt(203, 96),
		solid(colCardBG), bandsOf(colCardBG)...)
	add("d_right", "D far-XOR right crescent = green", 3, render.Pt(286, 96),
		solid(colFillD), bandsOf(colFillD)...)
	add("d_outside", "D far-XOR outside = card BG", 3, render.Pt(50, 96),
		solid(colCardBG), bandsOf(colCardBG)...)
}

type pixelProbeResult struct {
	Key    string
	OK     bool
	Detail string
}

func runPixelProbes(img image.Image, bx, by, dpr float64) []pixelProbeResult {
	out := make([]pixelProbeResult, 0, len(probeSpecs))
	for _, s := range probeSpecs {
		ca := cardAt[s.card]
		ox, oy := bx+ca[0]+contentX0, by+ca[1]+contentY0
		px, py := int((ox+s.pt.X)*dpr), int((oy+s.pt.Y)*dpr)
		wr, wg, wb := s.want(ox, oy, dpr, px, py)
		r, g, b, valid := readPixel(img, px, py)
		ok := valid && nearC(r, g, b, wr, wg, wb, tolSolid) && bandsOK(r, g, b, s.bands)
		out = append(out, pixelProbeResult{
			Key: s.key, OK: ok,
			Detail: fmt.Sprintf("%-52s @(%4d,%4d) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f)",
				s.desc, px, py, r, g, b, wr, wg, wb),
		})
	}
	return out
}

func readPixel(img image.Image, px, py int) (r, g, b float64, valid bool) {
	if img == nil {
		return 0, 0, 0, false
	}
	if px < img.Bounds().Min.X || px >= img.Bounds().Max.X ||
		py < img.Bounds().Min.Y || py >= img.Bounds().Max.Y {
		return 0, 0, 0, false
	}
	rr, gg, bb, _ := img.At(px, py).RGBA()
	return float64(rr>>8) / 255, float64(gg>>8) / 255, float64(bb>>8) / 255, true
}

func nearC(r, g, b, wr, wg, wb, tol float64) bool {
	return math.Abs(r-wr) <= tol && math.Abs(g-wg) <= tol && math.Abs(b-wb) <= tol
}

func bandsOK(r, g, b float64, bands []band) bool {
	ch := [3]float64{r, g, b}
	for _, bd := range bands {
		if ch[bd.ch] < bd.min || ch[bd.ch] > bd.max {
			return false
		}
	}
	return true
}
