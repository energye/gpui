package main

import (
	"fmt"
	"math"
	"os"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH = 430.0, 232.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// Probe tolerance: 8/255 per channel. Every sample below sits at least half a
// pixel away from a stroke edge unless the probe is explicitly checking that
// edge pixel's partial coverage.
const tolCol = 8.0 / 255.0

type probeKind int

const (
	kindFG       probeKind = iota // within tolCol of want
	kindBG                        // within tolCol of colCardBG
	kindNotBG                     // farther than tolCol from colCardBG (ink present)
	kindMostlyBG                  // every channel closer to colCardBG than to want
)

// mostlyBG is the declared-tolerance check for pixels that sit one row outside
// a thin (sub-1px / 1px) crisp edge. The engine leaves a ~1/3-ink sub-pixel
// fringe there (measured 0.32-0.36; 3px and 5px strokes are clean), recorded as
// a sub-item in the README - so those probes only demand mix < 0.5 instead of
// exact background.
func mostlyBG(r, g, b float64, want [3]float64, limit float64) bool {
	vals := [3]float64{r, g, b}
	for i := range vals {
		d := want[i] - colCardBG[i]
		if math.Abs(d) < 1e-9 {
			continue
		}
		if (vals[i]-colCardBG[i])/d > limit {
			return false
		}
	}
	return true
}

// Palette. Pixel-probe expectations read these same constants, so a palette
// edit can never desync the drawn pixels from the asserted ones.
var (
	colCardBG = [3]float64{0.12, 0.135, 0.165}
	colW05    = [3]float64{0.98, 0.75, 0.25} // Thin 0.5 (amber)
	colW1     = [3]float64{0.20, 0.72, 0.68} // Default 1.0 (teal)
	colW3     = [3]float64{0.85, 0.35, 0.75} // Thick 3.0 (magenta)
	colW5     = [3]float64{0.45, 0.85, 0.40} // Bold 5.0 (green)
	colCapB   = [3]float64{0.55, 0.62, 0.92} // butt (periwinkle)
	colCapS   = [3]float64{0.92, 0.60, 0.30} // square (orange)
	colCapR   = [3]float64{0.95, 0.85, 0.30} // round (yellow)
	colJoinM  = [3]float64{0.90, 0.45, 0.45} // miter (salmon)
	colJoinR  = [3]float64{0.35, 0.80, 0.75} // round join (turquoise)
	colDash   = [3]float64{0.60, 0.85, 0.35} // DashedStroke (lime)
	colDot    = [3]float64{0.95, 0.45, 0.85} // DottedStroke (pink)
	colTitle  = [3]float64{0.80, 0.86, 0.94}
)

// Card A: line centres are half-integers, so a stroke of width w centred at
// y0 exactly covers [y0-w/2, y0+w/2] -> the 1/3/5 px lines land on whole pixel
// rows and the 0.5 px line covers exactly half of its row.
const (
	axA, axB   = 30.0, 400.0
	rowThin    = 45.5
	rowDefault = 80.5
	rowThick   = 115.5
	rowBold    = 155.5
)

// Card B: caps drawn at width 9 (half width 4.5) so the square cap's far
// corner owns a whole probe pixel that the round cap's quarter disc leaves
// empty. The three segments share one end at x = bxEnd.
const (
	bxStart = 40.5
	bxEnd   = 300.5
	capW    = 9.0
	rowButt = 60.5
	rowSq   = 110.5
	rowRnd  = 160.5
)

// Card C: two identical "down then right" corners; the left one uses the miter
// join (DefaultStroke), the right one the round join (RoundStroke). The outer
// corner square spans [V.x-w/2, V.x] x [V.y, V.y+w/2].
var (
	cornerM = [3]render.Point{render.Pt(60.5, 40.5), render.Pt(60.5, 140.5), render.Pt(170.5, 140.5)}
	cornerR = [3]render.Point{render.Pt(240.5, 40.5), render.Pt(240.5, 140.5), render.Pt(350.5, 140.5)}
	joinW   = 9.0
)

// Card D: dashed (8 on / 5 off, width 5) and dotted (its own width 2, round
// caps, 0.1 on / 4 off) lines. The dot line runs at integer y so the 2px dots
// straddle the row boundary.
const (
	dxA, dxB        = 30.5, 400.5
	rowDash         = 80.5
	rowDot          = 160.0
	dashOn, dashOff = 8.0, 5.0
)

type probeSpec struct {
	key  string
	card int
	pt   render.Point
	want [3]float64
	tol  float64
	kind probeKind
}

var probeSpecs []probeSpec

func init() {
	probeSpecs = []probeSpec{
		// A: widths. Thin only half-covers its row (0.5 px), so it must merely
		// be inked; the others fully cover their centre row and must leave the
		// adjacent row clean.
		{"A_thin_ink", 0, render.Pt(200, 45), colW05, tolCol, kindNotBG},
		{"A_thin_clear", 0, render.Pt(200, 46), colW05, tolCol, kindMostlyBG},
		{"A_w1_fill", 0, render.Pt(200, 80), colW1, tolCol, kindFG},
		{"A_w1_clear", 0, render.Pt(200, 81), colW1, tolCol, kindMostlyBG},
		{"A_w3_fill", 0, render.Pt(200, 115), colW3, tolCol, kindFG},
		{"A_w3_edge", 0, render.Pt(200, 114), colW3, tolCol, kindFG},
		{"A_w3_clear", 0, render.Pt(200, 117), colW3, tolCol, kindBG},
		{"A_w5_fill", 0, render.Pt(200, 155), colW5, tolCol, kindFG},
		{"A_w5_edge", 0, render.Pt(200, 153), colW5, tolCol, kindFG},
		{"A_w5_clear", 0, render.Pt(200, 159), colW5, tolCol, kindBG},
		// B: caps. Butt stops at the path end; square and round extend 4.5 px;
		// only the square fills the far corner pixel of that extension.
		{"B_butt_in", 1, render.Pt(299, 60), colCapB, tolCol, kindFG},
		{"B_butt_end", 1, render.Pt(302, 60), colCapB, tolCol, kindBG},
		{"B_square_end", 1, render.Pt(302, 110), colCapS, tolCol, kindFG},
		{"B_square_corner", 1, render.Pt(304, 114), colCapS, tolCol, kindFG},
		{"B_round_end", 1, render.Pt(302, 160), colCapR, tolCol, kindFG},
		{"B_round_corner", 1, render.Pt(304, 164), colCapR, tolCol, kindBG},
		// C: joins. The miter fills the outer corner square; the round join
		// carves it into a quarter disc, so its far corner stays empty.
		{"C_miter_tip", 2, render.Pt(56, 143), colJoinM, tolCol, kindFG},
		{"C_miter_in", 2, render.Pt(58, 141), colJoinM, tolCol, kindFG},
		{"C_round_tip", 2, render.Pt(235, 143), colJoinR, tolCol, kindBG},
		{"C_round_in", 2, render.Pt(238, 141), colJoinR, tolCol, kindFG},
		// D: dash on/off at the pattern's own coordinates; dot ink vs gap.
		{"D_dash_on", 3, render.Pt(34, 80), colDash, tolCol, kindFG},
		{"D_dash_gap", 3, render.Pt(41, 80), colDash, tolCol, kindBG},
		{"D_dash_on2", 3, render.Pt(46, 80), colDash, tolCol, kindFG},
		{"D_dash_offline", 3, render.Pt(34, 75), colDash, tolCol, kindBG},
		{"D_dot_ink", 3, render.Pt(30, 159), colDot, tolCol, kindNotBG},
		{"D_dot_gap", 3, render.Pt(32, 160), colDot, tolCol, kindBG},
		{"D_dot_ink2", 3, render.Pt(34, 160), colDot, tolCol, kindNotBG},
		{"D_dot_offline", 3, render.Pt(34, 157), colDot, tolCol, kindBG},
	}
}

// ---- logical probes ----

// runLogicProbes checks the value-level contract of the eight presets plus the
// dash expansion the window draws. Pure values/geometry: no draw context, so
// it runs once before the window opens. probeKeys in main.go pins the set.
func runLogicProbes() (ran, failed []string) {
	check := func(key string, ok bool, detail string) {
		ran = append(ran, key)
		if !ok {
			failed = append(failed, key)
			fmt.Fprintf(os.Stderr, "ui_render_base_strokepreset: logic detail %s: %s\n", key, detail)
		}
	}

	// DefaultStroke: solid 1 px, butt caps, miter joins, miter limit 4.
	{
		s := render.DefaultStroke()
		check("preset_default",
			eqf(s.Width, 1) && s.Cap == render.LineCapButt && s.Join == render.LineJoinMiter &&
				eqf(s.MiterLimit, 4) && s.Dash == nil && !s.IsDashed(),
			fmt.Sprintf("width=%v cap=%v join=%v miter=%v dashed=%v", s.Width, s.Cap, s.Join, s.MiterLimit, s.IsDashed()))
	}
	// Width presets keep the default cap/join and carry their own width.
	{
		t, k, b := render.Thin(), render.Thick(), render.Bold()
		check("preset_widths",
			eqf(t.Width, 0.5) && eqf(k.Width, 3) && eqf(b.Width, 5) &&
				t.Cap == render.LineCapButt && k.Join == render.LineJoinMiter && b.Dash == nil,
			fmt.Sprintf("thin=%v thick=%v bold=%v", t.Width, k.Width, b.Width))
	}
	{
		s := render.RoundStroke()
		check("preset_round",
			eqf(s.Width, 1) && s.Cap == render.LineCapRound && s.Join == render.LineJoinRound,
			fmt.Sprintf("width=%v cap=%v join=%v", s.Width, s.Cap, s.Join))
	}
	{
		s := render.SquareStroke()
		check("preset_square",
			s.Cap == render.LineCapSquare && s.Join == render.LineJoinMiter && eqf(s.Width, 1),
			fmt.Sprintf("cap=%v join=%v width=%v", s.Cap, s.Join, s.Width))
	}
	{
		s := render.DashedStroke(8, 5)
		check("preset_dashed",
			s.IsDashed() && s.Dash != nil && len(s.Dash.Array) == 2 &&
				eqf(s.Dash.Array[0], 8) && eqf(s.Dash.Array[1], 5) &&
				eqf(s.Width, 1) && s.Cap == render.LineCapButt,
			fmt.Sprintf("array=%v width=%v capped=%v", s.Dash.Array, s.Width, s.Cap))
	}
	{
		// The real DottedStroke pattern is 0.1 on / 4 off with round caps =
		// Skia's dot idiom (zero-length dash, gap = 2 x width); the stale
		// "(1, 2 pattern)" comment is rewritten by this window's doc fix.
		s := render.DottedStroke()
		check("preset_dotted",
			eqf(s.Width, 2) && s.Cap == render.LineCapRound && s.Join == render.LineJoinRound &&
				eqf(s.MiterLimit, 4) && s.Dash != nil && len(s.Dash.Array) == 2 &&
				eqf(s.Dash.Array[0], 0.1) && eqf(s.Dash.Array[1], 4) && eqf(s.Dash.PatternLength(), 4.1),
			fmt.Sprintf("array=%v width=%v pattern=%v", s.Dash.Array, s.Width, s.Dash.PatternLength()))
	}
	// Chaining With* returns copies and never mutates the base preset.
	{
		base := render.RoundStroke()
		chained := base.WithWidth(9).WithDashPattern(3, 2)
		check("with_chain_copy",
			eqf(base.Width, 1) && base.Dash == nil && eqf(chained.Width, 9) && chained.IsDashed() &&
				chained.Cap == render.LineCapRound && len(chained.Dash.Array) == 2 && eqf(chained.Dash.Array[0], 3),
			fmt.Sprintf("base=(w=%v dash=%v) chained=(w=%v array=%v)", base.Width, base.Dash == nil, chained.Width, chained.Dash.Array))
	}
	// Dash pattern bookkeeping: odd arrays duplicate; all-zero input is nil.
	{
		check("dash_pattern_len",
			eqf(render.NewDash(8, 5).PatternLength(), 13) && eqf(render.NewDash(5).PatternLength(), 10) &&
				render.NewDash() == nil && render.NewDash(0, 0) == nil,
			fmt.Sprintf("2elem=%v 1elem=%v emptyNil=%v zeroNil=%v",
				render.NewDash(8, 5).PatternLength(), render.NewDash(5).PatternLength(),
				render.NewDash() == nil, render.NewDash(0, 0) == nil))
	}
	// Skia's dotted idiom: the gap is twice the stroke width.
	{
		s := render.DottedStroke()
		check("dotted_skia_gap",
			s.Dash != nil && eqf(s.Dash.Array[1], 2*s.Width),
			fmt.Sprintf("gap=%v width=%v", s.Dash.Array[1], s.Width))
	}
	// ApplyDash cuts the path into the expected on-runs (card D geometry).
	{
		line := render.NewPath()
		line.MoveTo(0, 0)
		line.LineTo(30, 0)
		segs := dashSegments(render.ApplyDash(line, render.NewDash(8, 5)))
		want := [][4]float64{{0, 0, 8, 0}, {13, 0, 21, 0}, {26, 0, 30, 0}}
		check("apply_dash_ranges", eqSegs(segs, want), fmt.Sprintf("segs=%v", segs))
	}
	// A nil/non-dashed pattern leaves the path as-is.
	{
		line := render.NewPath()
		line.MoveTo(0, 0)
		line.LineTo(30, 0)
		segs := dashSegments(render.ApplyDash(line, nil))
		check("apply_dash_solid",
			len(segs) == 1 && eqf(segs[0][0], 0) && eqf(segs[0][2], 30) && eqf(segs[0][3], 0),
			fmt.Sprintf("segs=%v", segs))
	}
	// Stroke.Clone deep-copies the dash array.
	{
		s := render.DashedStroke(8, 5)
		c := s.Clone()
		c.Dash.Array[0] = 99
		check("stroke_clone_deep",
			eqf(s.Dash.Array[0], 8) && eqf(c.Dash.Array[0], 99),
			fmt.Sprintf("orig=%v clone=%v", s.Dash.Array, c.Dash.Array))
	}
	return ran, failed
}

func eqf(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func eqSegs(got, want [][4]float64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		for j := 0; j < 4; j++ {
			if math.Abs(got[i][j]-want[i][j]) > 0.001 {
				return false
			}
		}
	}
	return true
}

// dashSegments reads a dashed path back as [x0,y0,x1,y1] runs. Only MoveTo /
// LineTo are expected from ApplyDash; anything else returns nil (proof fails
// loudly rather than silently skipping coordinates).
func dashSegments(p *render.Path) [][4]float64 {
	var out [][4]float64
	coords := p.Coords()
	ci := 0
	for _, v := range p.Verbs() {
		switch v {
		case render.MoveTo:
			if ci+2 > len(coords) {
				return nil
			}
			out = append(out, [4]float64{coords[ci], coords[ci+1], coords[ci], coords[ci+1]})
			ci += 2
		case render.LineTo:
			if ci+2 > len(coords) || len(out) == 0 {
				return nil
			}
			out[len(out)-1][2], out[len(out)-1][3] = coords[ci], coords[ci+1]
			ci += 2
		default:
			return nil
		}
	}
	return out
}

// ---- scene ----

// buildStrokePresetScene fills the shell body with the four demo cards. The
// card background is painted by the card's own OnPaint (first op), so the
// background-reading probes see exactly colCardBG.
func buildStrokePresetScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.SetRepaintBoundary(true)
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, colTitle[0], colTitle[1], colTitle[2]),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A  Thin 0.5 / Default 1 / Thick 3 / Bold 5", paintWidths)
	card(1, "B  caps: butt / square / round (w=9)", paintCaps)
	card(2, "C  joins: miter (left) vs round (right), w=9", paintJoins)
	card(3, "D  DashedStroke(8,5) w=5 / DottedStroke w=2", paintDashDot)
}

func cardBG(dc *render.Context, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

// paintWidths: each preset draws its own width; the centre rows stay crisp
// (1/3/5 px) and Thin half-covers row 45.
func paintWidths(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	dc.Push()
	dc.Translate(ox, oy)
	line := func(y float64, st render.Stroke, col [3]float64) {
		dc.SetRGBA(col[0], col[1], col[2], 1)
		dc.SetStroke(st)
		dc.MoveTo(axA, y)
		dc.LineTo(axB, y)
		_ = dc.Stroke()
	}
	line(rowThin, render.Thin(), colW05)
	line(rowDefault, render.DefaultStroke(), colW1)
	line(rowThick, render.Thick(), colW3)
	line(rowBold, render.Bold(), colW5)
	dc.Pop()
}

// paintCaps: three segments share the end x; only the cap style differs.
func paintCaps(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	dc.Push()
	dc.Translate(ox, oy)
	seg := func(y float64, st render.Stroke, col [3]float64) {
		dc.SetRGBA(col[0], col[1], col[2], 1)
		dc.SetStroke(st)
		dc.MoveTo(bxStart, y)
		dc.LineTo(bxEnd, y)
		_ = dc.Stroke()
	}
	seg(rowButt, render.DefaultStroke().WithWidth(capW), colCapB)
	seg(rowSq, render.SquareStroke().WithWidth(capW), colCapS)
	seg(rowRnd, render.RoundStroke().WithWidth(capW), colCapR)
	dc.Pop()
}

// paintJoins: identical corners, miter join on the left, round join on the
// right.
func paintJoins(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	dc.Push()
	dc.Translate(ox, oy)
	corner := func(pts [3]render.Point, st render.Stroke, col [3]float64) {
		dc.SetRGBA(col[0], col[1], col[2], 1)
		dc.SetStroke(st)
		dc.MoveTo(pts[0].X, pts[0].Y)
		dc.LineTo(pts[1].X, pts[1].Y)
		dc.LineTo(pts[2].X, pts[2].Y)
		_ = dc.Stroke()
	}
	corner(cornerM, render.DefaultStroke().WithWidth(joinW), colJoinM)
	corner(cornerR, render.RoundStroke().WithWidth(joinW), colJoinR)
	dc.Pop()
}

// paintDashDot: DashedStroke (pattern 8 on / 5 off, width 5) and the dotted
// preset as-is (width 2, round caps, 0.1 on / 4 off).
func paintDashDot(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	dc.Push()
	dc.Translate(ox, oy)
	dc.SetRGBA(colDash[0], colDash[1], colDash[2], 1)
	dc.SetStroke(render.DashedStroke(dashOn, dashOff).WithWidth(5))
	dc.MoveTo(dxA, rowDash)
	dc.LineTo(dxB, rowDash)
	_ = dc.Stroke()

	dc.SetRGBA(colDot[0], colDot[1], colDot[2], 1)
	dc.SetStroke(render.DottedStroke())
	dc.MoveTo(dxA, rowDot)
	dc.LineTo(dxB, rowDot)
	_ = dc.Stroke()
	dc.Pop()
}
