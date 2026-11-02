// Command ui_wr_p35_patheffect is the P3.5 batch-4 capability real window:
// PathEffect mounted on Stroke. Five constructors, the two combinators, the
// three 1D styles, the cache-identity contract and the stamp-density governor —
//
//	A  one zigzag source, three effects: dash (the same engine Stroke.Dash
//	   uses, so the two entries cannot drift), corner (sharp vertices cut to
//	   arcs), discrete (chord jitter inside the declared deviation). The grey
//	   line under each row is the un-effected source.
//	B  the three Path1D styles on the same circle — Translate keeps the
//	   prototype's orientation, Rotate turns it with the tangent, Morph maps the
//	   prototype's x along the arc and y along the normal — plus Path2D's grid.
//	C  Sum (both effects on one source, unioned, never folded) and Compose
//	   (outer(inner(src)) — the two orders are visibly different geometries),
//	   plus the density governor beside a control at a sane advance: the
//	   pathological advance refuses the whole path, so its framed slot stays
//	   empty. That refusal costs the full governor budget (≈0.1s), so it is
//	   asserted once by logic probe 7 instead of every frame; the slot is still
//	   probed empty.
//
// Every stroke goes through the mounted entry (Stroke.WithPathEffect), and the
// pixel probes read points taken from the effect's own output path, so an
// assertion can never drift from the picture.
//
// Gate mode asserts 10 logic probes (numbers land in the JSON report), semantic
// pixel probes, and a zero-tolerance golden over the shell content.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=6 go run ./examples/ui_wr_p35_patheffect -auto-only                       # gate
//	GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=6 go run ./examples/ui_wr_p35_patheffect -auto-only  # seed once
//	go run ./examples/ui_wr_p35_patheffect                                                 # manual, stays open
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
	winW, winH   = 1200, 800
	closeSeconds = 6
	windowTitle  = "gpui ui_wr_p35_patheffect — P3.5 PathEffect 五构造器与合成"

	metricTol = 0.25 // the tolerance the 1D/2D walkers use internally
)

const (
	cardAX, cardAY, cardAW, cardAH = 8.0, 8.0, 436.0, 300.0
	cardBX, cardBY, cardBW, cardBH = 460.0, 8.0, 436.0, 300.0
	cardCX, cardCY, cardCW, cardCH = 8.0, 320.0, 888.0, 320.0
)

var cardBG = [3]float64{0.07, 0.08, 0.10}

type geomT struct {
	// Card A: one zigzag shape, three rows.
	aX0, aSpan, aAmp float64
	aRows            [3]float64
	aDash            [2]float64
	aCornerR         float64
	aSeg, aDev       float64
	// Card B: three 1D panels on a circle, one 2D panel on a rectangle.
	bPanelX          [4]float64
	bCY, bSrc        float64
	bAdvance, bPhase float64
	bProto           float64
	b2DX, b2DY       float64
	// Card C: sum, compose in both orders, density governor.
	cZigSpan, cZigAmp float64
	cSumX, cSumY      float64
	cCompX, cCompY1   float64
	cCompY2           float64
	cGovX, cGovY      float64
}

var G = geomT{
	aX0: 24, aSpan: 388, aAmp: 26,
	aRows:    [3]float64{84, 164, 244},
	aDash:    [2]float64{22, 18},
	aCornerR: 18,
	aSeg:     16, aDev: 7,
	bPanelX: [4]float64{58, 162, 266, 372},
	bCY:     150, bSrc: 44,
	bAdvance: 24, bPhase: 6, bProto: 9,
	b2DX: 20, b2DY: 20,
	cZigSpan: 240, cZigAmp: 28,
	cSumX: 24, cSumY: 170,
	cCompX: 300, cCompY1: 100, cCompY2: 240,
	cGovX: 596, cGovY: 170,
}

// ─────────────────────────── sources, prototypes, effects ───────────────────────────

// zig builds the shared source shape: a polyline of n segments across span/amp.
func zig(x0, y, span, amp float64, n int) *render.Path {
	p := render.NewPath()
	step := span / float64(n)
	p.MoveTo(x0, y)
	for i := 1; i <= n; i++ {
		dy := 0.0
		if i%2 == 1 {
			dy = -amp
		}
		p.LineTo(x0+step*float64(i), y+dy)
	}
	return p
}

// steps builds a square wave — every turn is a right angle, which is what a
// corner-radius probe needs (the cut of an acute turn scales with 1/sin(θ/2)).
func steps(x0, y, run, rise float64, n int) *render.Path {
	p := render.NewPath()
	p.MoveTo(x0, y)
	cy := y
	for i := 0; i < n; i++ {
		x := x0 + run*float64(i+1)
		p.LineTo(x, cy)
		if i%2 == 0 {
			cy -= rise
		} else {
			cy += rise
		}
		p.LineTo(x, cy)
	}
	return p
}

func protoTri(size float64) *render.Path {
	p := render.NewPath()
	p.MoveTo(-size, size*0.6)
	p.LineTo(size, size*0.6)
	p.LineTo(0, -size*0.9)
	p.Close()
	return p
}

func protoBar(w, h float64) *render.Path {
	p := render.NewPath()
	p.MoveTo(-w, -h)
	p.LineTo(w, -h)
	p.LineTo(w, h)
	p.LineTo(-w, h)
	p.Close()
	return p
}

func circle(cx, cy, r float64) *render.Path {
	p := render.NewPath()
	p.Circle(cx, cy, r)
	return p
}

func rectPath(x, y, w, h float64) *render.Path {
	p := render.NewPath()
	p.MoveTo(x, y)
	p.LineTo(x+w, y)
	p.LineTo(x+w, y+h)
	p.LineTo(x, y+h)
	p.Close()
	return p
}

// effectSet holds every effect the window draws. Built once and shared by the
// painters, the probes and the logic probes.
type effectSet struct {
	dash, corner, discrete render.PathEffect
	oneD                   [3]render.PathEffect // translate / rotate / morph
	twoD                   render.PathEffect
	halfCorner             render.PathEffect
	sum                    render.PathEffect
	compAB, compBA         render.PathEffect
	pathological           render.PathEffect
}

var E effectSet

func buildEffects() {
	E.dash = render.NewDashPathEffect(render.NewDash(G.aDash[0], G.aDash[1]))
	E.corner = render.NewCornerPathEffect(G.aCornerR)
	E.discrete = render.NewDiscretePathEffect(G.aSeg, G.aDev)
	E.halfCorner = render.NewCornerPathEffect(G.aCornerR / 2)

	tri := protoTri(G.bProto)
	bar := protoBar(G.bProto*1.4, G.bProto*0.45)
	E.oneD[0] = render.NewPath1DPathEffect(tri, render.Path1DTranslate, G.bAdvance, G.bPhase)
	E.oneD[1] = render.NewPath1DPathEffect(tri, render.Path1DRotate, G.bAdvance, G.bPhase)
	E.oneD[2] = render.NewPath1DPathEffect(bar, render.Path1DMorph, G.bAdvance, G.bPhase)
	E.twoD = render.NewPath2DPathEffect(protoTri(G.bProto*0.8), G.b2DX, G.b2DY)

	E.sum = render.SumPathEffects(E.dash, E.corner)
	E.compAB = render.ComposePathEffects(E.corner, E.discrete)
	E.compBA = render.ComposePathEffects(E.discrete, E.corner)
	// advance below the ULP of the walking distance: d += advance never
	// advances, so the density governor must refuse the whole path.
	E.pathological = render.NewPath1DPathEffect(protoTri(4), render.Path1DTranslate, 1e-12, 0)
}

// ─────────────────────────── the scene, as data ───────────────────────────

// layer is one stroke of the scene: a card-local source, an effect mounted on
// the stroke, and the colour it is painted with. The painters draw this list and
// the probes derive their points from it, so picture and assertions share one
// source of truth.
type layer struct {
	card     int
	name     string // empty = decoration, never probed
	src      *render.Path
	eff      render.PathEffect
	width    float64
	col      [3]float64
	alpha    float64
	probeGap bool // also probe a point where the effect leaves no ink
	lazy     bool // assert the effect off the frame loop (the density governor
	// costs ~0.1s per pass, so mounting it every frame would tank the frame rate)
}

var scene []layer

var (
	colGhost = [3]float64{0.28, 0.31, 0.38}
	colDash  = [3]float64{0.95, 0.35, 0.25}
	colCorr  = [3]float64{0.25, 0.85, 0.45}
	colDisc  = [3]float64{0.30, 0.70, 0.95}
	colTrans = [3]float64{0.95, 0.80, 0.25}
	colRot   = [3]float64{0.45, 0.90, 0.95}
	colMorph = [3]float64{0.90, 0.45, 0.90}
	col2D    = [3]float64{0.35, 0.95, 0.65}
	colSum   = [3]float64{0.98, 0.62, 0.20}
	colComp1 = [3]float64{0.35, 0.80, 0.98}
	colComp2 = [3]float64{0.98, 0.45, 0.70}
	colFrame = [3]float64{0.42, 0.47, 0.57}
)

func ghost(card int, src *render.Path) {
	// 3px wide: a hairline antialiases to a fractional pixel, which would make a
	// "only the ghost is here" probe read a blend instead of the ghost colour.
	scene = append(scene, layer{card: card, src: src, width: 3, col: colGhost, alpha: 0.55})
}

func buildScene() {
	scene = nil
	// Card A — dash / corner / discrete on the same source shape.
	for i, y := range G.aRows {
		src := zig(G.aX0, y, G.aSpan, G.aAmp, 6)
		if i == 1 {
			// Corner rounding is measured on right angles.
			src = steps(G.aX0, y+22, 96, 44, 3)
		}
		ghost(0, src)
		l := layer{card: 0, src: src, width: 6, alpha: 1}
		switch i {
		case 0:
			l.name, l.eff, l.col, l.probeGap = "A_dash", E.dash, colDash, true
			l.width = 7
		case 1:
			l.name, l.eff, l.col = "A_corner", E.corner, colCorr
			l.width = 5
		case 2:
			l.name, l.eff, l.col = "A_discrete", E.discrete, colDisc
			l.width = 4
		}
		scene = append(scene, l)
	}

	// Card B — the three 1D styles plus the 2D grid.
	for i := 0; i < 3; i++ {
		src := circle(G.bPanelX[i], G.bCY, G.bSrc)
		ghost(1, src)
		names := [3]string{"B_1D_translate", "B_1D_rotate", "B_1D_morph"}
		cols := [3][3]float64{colTrans, colRot, colMorph}
		scene = append(scene, layer{card: 1, name: names[i], src: src, eff: E.oneD[i],
			width: 2, col: cols[i], alpha: 1, probeGap: true})
	}
	gridSrc := rectPath(G.bPanelX[3]-G.bSrc, G.bCY-G.bSrc, 2*G.bSrc, 2*G.bSrc)
	ghost(1, gridSrc)
	scene = append(scene, layer{card: 1, name: "B_2D_grid", src: gridSrc, eff: E.twoD,
		width: 2, col: col2D, alpha: 1, probeGap: true})

	// Card C — sum, compose in both orders, and the refused density slot.
	sumSrc := zig(G.cSumX, G.cSumY, G.cZigSpan, G.cZigAmp, 5)
	ghost(2, sumSrc)
	scene = append(scene, layer{card: 2, name: "C_sum", src: sumSrc, eff: E.sum,
		width: 4, col: colSum, alpha: 1})
	comp1 := zig(G.cCompX, G.cCompY1, G.cZigSpan*0.85, G.cZigAmp, 5)
	scene = append(scene, layer{card: 2, name: "C_compose_corner_of_discrete", src: comp1, eff: E.compAB,
		width: 4, col: colComp1, alpha: 1})
	comp2 := zig(G.cCompX, G.cCompY2, G.cZigSpan*0.85, G.cZigAmp, 5)
	scene = append(scene, layer{card: 2, name: "C_compose_discrete_of_corner", src: comp2, eff: E.compBA,
		width: 4, col: colComp2, alpha: 1})
	// Control: the same prototype at a sane advance stamps normally…
	ctrl := circle(G.cGovX+40, G.cGovY-58, 34)
	scene = append(scene, layer{card: 2, name: "C_governor_control_advance24", src: ctrl,
		eff:   render.NewPath1DPathEffect(protoTri(5), render.Path1DTranslate, 24, 0),
		width: 2, col: colTrans, alpha: 1})
	// …while the pathological advance is refused outright: no geometry at all.
	// The refusal costs the full governor budget, so logic probe 7 asserts it off
	// the frame loop (lazy) and the framed slot here is probed empty.
	refused := circle(G.cGovX+40, G.cGovY+34, 34)
	scene = append(scene, layer{card: 2, name: "C_density_governor_refused", src: refused,
		eff: E.pathological, width: 3, col: colSum, alpha: 1, lazy: true})
	scene = append(scene, layer{card: 2, src: rectPath(G.cGovX-6, G.cGovY-4, 92, 76),
		width: 1.0, col: colFrame, alpha: 0.9})
}

// ─────────────────────────── painters ───────────────────────────

var cardOrigin [3]render.Point

func cardBase(pc *rendering.PaintContext, sz rendering.Size) (float64, float64) {
	dc := pc.DC
	dc.SetRGBA(cardBG[0], cardBG[1], cardBG[2], 1)
	dc.DrawRectangle(pc.OriginX, pc.OriginY, sz.Width, sz.Height)
	_ = dc.Fill()
	return pc.OriginX, pc.OriginY
}

// strokeLayer mounts the effect on the stroke itself (never pre-filters the
// path), so the raster is the engine's effect+stroke pipeline. Raw Canvas calls
// take window coordinates, hence the origin-shifted source.
func strokeLayer(dc render.Canvas, ox, oy float64, l layer) {
	if l.lazy {
		return // asserted by logic probe 7; a refused effect paints nothing anyway
	}
	st := render.DefaultStroke().WithWidth(l.width).
		WithCap(render.LineCapRound).WithJoin(render.LineJoinRound)
	if l.eff != nil {
		st = st.WithPathEffect(l.eff)
	}
	dc.SetStroke(st)
	dc.SetRGBA(l.col[0], l.col[1], l.col[2], l.alpha)
	dc.DrawPath(l.src.Transform(render.Translate(ox, oy)))
	if err := dc.Stroke(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_p35_patheffect: stroke %q failed: %v\n", l.name, err)
	}
}

func makePainter(idx int) func(*rendering.PaintContext, rendering.Size) {
	return func(pc *rendering.PaintContext, sz rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		cardOrigin[idx] = render.Pt(pc.OriginX, pc.OriginY)
		ox, oy := cardBase(pc, sz)
		for _, l := range scene {
			if l.card != idx {
				continue
			}
			strokeLayer(pc.DC, ox, oy, l)
		}
	}
}

// ─────────────────────────── pixel probes ───────────────────────────

func flatPts(p *render.Path) []render.Point {
	if p == nil {
		return nil
	}
	return p.Flatten(0.4)
}

func distToAny(pts []render.Point, q render.Point) float64 {
	best := math.Inf(1)
	for _, p := range pts {
		if d := math.Hypot(p.X-q.X, p.Y-q.Y); d < best {
			best = d
		}
	}
	return best
}

// contour is one sub-path's vertices plus whether it closes.
type contour struct {
	pts    []render.Point
	closed bool
}

// vertexContours splits a path into contours. The closing edge is only listed
// when the contour really closes, so a phantom chord between an open path's last
// and first vertex can never be mistaken for geometry.
func vertexContours(p *render.Path) []contour {
	var out []contour
	for k := 1; ; k++ {
		pts, closed := contourPtsClosed(p, k)
		if len(pts) == 0 {
			return out
		}
		out = append(out, contour{pts: pts, closed: closed})
	}
}

// edges lists every drawn segment of a contour (the closing edge when closed).
func (c contour) edges() [][2]render.Point {
	var out [][2]render.Point
	for i := 1; i < len(c.pts); i++ {
		out = append(out, [2]render.Point{c.pts[i-1], c.pts[i]})
	}
	if c.closed && len(c.pts) > 2 {
		out = append(out, [2]render.Point{c.pts[len(c.pts)-1], c.pts[0]})
	}
	return out
}

// distToSegs is the distance from q to the nearest of the given segments.
func distToSegs(segs [][2]render.Point, q render.Point) float64 {
	best := math.Inf(1)
	for _, e := range segs {
		if d := pointSegDist(q, e[0], e[1]); d < best {
			best = d
		}
	}
	return best
}

// filteredOf applies the layer's effect exactly as the mounted stroke does;
// ok=false is the engine's refusal (the density-governor case), which is itself
// something to probe.
func filteredOf(l layer) (*render.Path, bool) {
	if l.eff == nil {
		return l.src, true
	}
	return l.eff.FilterPath(l.src)
}

func emptyProbe(name string, x, y float64) wrkit.PixelProbe {
	return wrkit.PixelProbe{Name: name, X: x, Y: y, R: cardBG[0], G: cardBG[1], B: cardBG[2], Tol: 8}
}

// allSegments returns every drawn segment of a path: straight edges as they are,
// curves subdivided to a couple of pixels, closing edges only for closed
// contours — so a distance measured against it is the distance to real geometry,
// never to a chord across an open path's endpoints.
func allSegments(p *render.Path) [][2]render.Point {
	if p == nil {
		return nil
	}
	var out [][2]render.Point
	var chain []render.Point
	var first render.Point
	flush := func() {
		for i := 1; i < len(chain); i++ {
			out = append(out, [2]render.Point{chain[i-1], chain[i]})
		}
		chain = nil
	}
	p.Iterate(func(v render.PathVerb, c []float64) {
		switch v {
		case render.MoveTo:
			flush()
			first = render.Pt(c[0], c[1])
			chain = append(chain, first)
		case render.LineTo:
			chain = append(chain, render.Pt(c[0], c[1]))
		case render.QuadTo:
			q := render.NewQuadBez(lastOf(chain), render.Pt(c[0], c[1]), render.Pt(c[2], c[3]))
			n := curveSteps(q.P0.Sub(q.P2).Length())
			for i := 1; i <= n; i++ {
				chain = append(chain, q.Eval(float64(i)/float64(n)))
			}
		case render.CubicTo:
			cb := render.NewCubicBez(lastOf(chain), render.Pt(c[0], c[1]), render.Pt(c[2], c[3]), render.Pt(c[4], c[5]))
			n := curveSteps(cb.P0.Sub(cb.P3).Length())
			for i := 1; i <= n; i++ {
				chain = append(chain, cb.Eval(float64(i)/float64(n)))
			}
		case render.Close:
			if len(chain) > 1 {
				chain = append(chain, first)
			}
		}
	})
	flush()
	return out
}

func lastOf(chain []render.Point) render.Point {
	if len(chain) == 0 {
		return render.Point{}
	}
	return chain[len(chain)-1]
}

// curveSteps chops a curve of the given span into chords of about 2px.
func curveSteps(span float64) int {
	n := int(math.Ceil(span / 2))
	if n < 2 {
		n = 2
	}
	return n
}

// buildProbes derives every pixel probe from the scene: an ink point on a real
// edge of the effect's own output, an optional gap point on the source where the
// effect left no geometry, and for a refused effect an empty-slot point.
func buildProbes() []wrkit.PixelProbe {
	segs := map[string][][2]render.Point{}
	perCard := map[int][][][2]render.Point{}
	all := map[int][][2]render.Point{}
	for _, l := range scene {
		f, ok := filteredOf(l)
		if !ok || f == nil {
			continue
		}
		se := allSegments(f)
		all[l.card] = append(all[l.card], se...)
		if l.name == "" {
			continue
		}
		segs[l.name] = se
		perCard[l.card] = append(perCard[l.card], se)
	}

	probes := []wrkit.PixelProbe{}
	for _, l := range scene {
		if l.name == "" {
			continue
		}
		ox, oy := cardOrigin[l.card].X, cardOrigin[l.card].Y
		w, h := cardW(l.card), cardH(l.card)
		mine := segs[l.name]
		if len(mine) == 0 {
			// Refused geometry: the slot must stay bare at the source's centre.
			b := l.src.Bounds()
			cx := float64(b.Min.X+b.Max.X) / 2
			cy := float64(b.Min.Y+b.Max.Y) / 2
			probes = append(probes, emptyProbe(l.name+"_slot_empty", ox+cx, oy+cy))
			continue
		}
		var neigh [][2]render.Point
		for _, set := range perCard[l.card] {
			if len(set) > 0 && set[0] == mine[0] {
				continue // this layer's own geometry
			}
			neigh = append(neigh, set...)
		}
		if ink, ok := pickInkPoint(l, mine, neigh); ok {
			// The ink probe asserts the hue, not the exact coverage: a stroke
			// centre line that falls between pixel centres antialiases across the
			// sampled pixel. Every layer's colour is ≥ 110 8-bit steps from the
			// backdrop and the ghost in at least two channels, and the spatial
			// precision lives in the gap/empty probes.
			probes = append(probes, wrkit.PixelProbe{
				Name: l.name + "_ink", X: ox + ink.X, Y: oy + ink.Y,
				R: l.col[0], G: l.col[1], B: l.col[2], Tol: 90,
			})
		}
		if l.probeGap {
			// Where the effect removed geometry only the ghost stays: the blend
			// of the un-effected source line over the card backdrop. Clearance is
			// measured from the effect's centre line, so it must exceed this
			// stroke's half-width plus a pixel or two of antialiasing.
			if gp, ok := pickGapPoint(l.src, mine, neigh, l.width/2+4); ok {
				probes = append(probes, wrkit.PixelProbe{
					Name: l.name + "_gap_ghost_only", X: ox + gp.X, Y: oy + gp.Y,
					R: ghostBlend[0], G: ghostBlend[1], B: ghostBlend[2], Tol: 18,
				})
			}
		}
		if ep, ok := pickEmptyPoint(w, h, all[l.card]); ok {
			probes = append(probes, emptyProbe(l.name+"_panel_empty", ox+ep.X, oy+ep.Y))
		}
	}
	return probes
}

func cardW(idx int) float64 {
	if idx == 2 {
		return cardCW
	}
	return cardAW
}

func cardH(idx int) float64 {
	if idx == 2 {
		return cardCH
	}
	return cardAH
}

// ghostBlend is the colour of the un-effected ghost line over the card backdrop
// — what a probe reads wherever an effect removed geometry.
var ghostBlend = [3]float64{
	0.55*colGhost[0] + 0.45*cardBG[0],
	0.55*colGhost[1] + 0.45*cardBG[1],
	0.55*colGhost[2] + 0.45*cardBG[2],
}

// pickInkPoint returns the midpoint of the longest drawn edge that is near
// axis-aligned (strict pass first, so the pixel is mostly stroke rather than
// antialiased fringe) and clear of every other painted layer, so the sampled
// colour belongs to this layer. The hue, not the exact coverage, is the claim:
// every layer's colour is ≥ 110 8-bit steps from the backdrop and the ghost in
// at least two channels, and the spatial precision lives in the gap/empty probes.
func pickInkPoint(l layer, mine, neigh [][2]render.Point) (render.Point, bool) {
	clearance := l.width/2 + 5
	best, out, found := 0.0, render.Point{}, false
	ratio := func(dx, dy float64) float64 {
		lo, hi := math.Min(dx, dy), math.Max(dx, dy)
		if lo <= 0 {
			return math.Inf(1)
		}
		return hi / lo
	}
	for _, strict := range []bool{true, false} {
		best, found = 0.0, false
		for _, e := range mine {
			dx, dy := math.Abs(e[1].X-e[0].X), math.Abs(e[1].Y-e[0].Y)
			length := math.Hypot(dx, dy)
			minLen, minRatio := 8.0, 2.0
			if strict {
				minLen, minRatio = 10.0, 5.0
			}
			if length < minLen || length <= best || ratio(dx, dy) < minRatio {
				continue
			}
			m := render.Pt((e[0].X+e[1].X)/2, (e[0].Y+e[1].Y)/2)
			if len(neigh) > 0 && distToSegs(neigh, m) < clearance {
				continue
			}
			best, out, found = length, m, true
		}
		if found {
			return out, true
		}
	}
	return out, found
}

// centreOnPixel reports whether a near-axis edge's centre line falls inside the
// sampled pixel, so the pixel reads the stroke's full colour instead of a blend.
func centreOnPixel(m render.Point, e [2]render.Point) bool {
	if math.Abs(e[1].X-e[0].X) >= math.Abs(e[1].Y-e[0].Y) {
		frac := math.Abs(m.Y - math.Trunc(m.Y))
		return frac >= 0.15 && frac <= 0.85
	}
	frac := math.Abs(m.X - math.Trunc(m.X))
	return frac >= 0.15 && frac <= 0.85
}

// pickGapPoint finds a point of the source the effect removed — the dash gap,
// the space between two stamps — far from the effect's own geometry and from
// every other painted layer.
func pickGapPoint(src *render.Path, mine, neigh [][2]render.Point, clearance float64) (render.Point, bool) {
	for _, e := range allSegments(src) {
		length := math.Hypot(e[1].X-e[0].X, e[1].Y-e[0].Y)
		if length < 2*clearance {
			continue // too short a source edge to hold a real gap
		}
		// Walk the edge in ~3px steps: a gap is somewhere along it, not
		// necessarily at its midpoint.
		for t := 0.5; t <= 1; t += 3 / length {
			p := render.Pt(e[0].X+(e[1].X-e[0].X)*t, e[0].Y+(e[1].Y-e[0].Y)*t)
			if distToSegs(mine, p) < clearance {
				continue // still under this effect's geometry
			}
			if len(neigh) > 0 && distToSegs(neigh, p) < clearance {
				continue
			}
			return p, true
		}
	}
	return render.Point{}, false
}

// pickEmptyPoint scans a coarse grid inside the card and returns the first point
// no geometry comes near (labels live above y=28, so the scan starts below).
func pickEmptyPoint(w, h float64, all [][2]render.Point) (render.Point, bool) {
	for y := 40.0; y < h-24; y += 6 {
		for x := 20.0; x < w-20; x += 6 {
			p := render.Pt(x, y)
			if len(all) == 0 || distToSegs(all, p) >= 12 {
				return p, true
			}
		}
	}
	return render.Point{}, false
}

// ─────────────────────────── logic probes ───────────────────────────

type logicResult struct {
	name string
	ok   bool
	det  string
	num  map[string]float64
}

func pathCoordsEqual(a, b *render.Path) bool {
	if a == nil || b == nil {
		return a == b
	}
	va, vb := a.Verbs(), b.Verbs()
	ca, cb := a.Coords(), b.Coords()
	if len(va) != len(vb) || len(ca) != len(cb) {
		return false
	}
	for i := range va {
		if va[i] != vb[i] {
			return false
		}
	}
	for i := range ca {
		if ca[i] != cb[i] {
			return false
		}
	}
	return true
}

func verbTotal(p *render.Path) int {
	if p == nil {
		return 0
	}
	return p.NumVerbs()
}

func moveToCount(p *render.Path) int {
	if p == nil {
		return 0
	}
	n := 0
	for _, v := range p.Verbs() {
		if v == render.MoveTo {
			n++
		}
	}
	return n
}

func runLogicProbes() []logicResult {
	out := []logicResult{}
	add := func(r logicResult) { out = append(out, r) }
	src := zig(G.aX0, G.aRows[0], G.aSpan, G.aAmp, 6)
	circ := circle(G.bPanelX[0], G.bCY, G.bSrc)
	offset := normalizePhase(G.bPhase, G.bAdvance)

	// L1 the dash effect is the same engine Stroke.Dash uses: one dasher, so the
	// convenience field and the effect can never drift apart.
	dashed, okDash := E.dash.FilterPath(src)
	want := render.ApplyDash(src, render.NewDash(G.aDash[0], G.aDash[1]))
	ok1 := okDash && pathCoordsEqual(dashed, want) && verbTotal(dashed) == verbTotal(want)
	add(logicResult{"dash_effect_equals_apply_dash", ok1,
		fmt.Sprintf("effect verbs=%d ApplyDash verbs=%d, verbs+coords identical=%v",
			verbTotal(dashed), verbTotal(want), ok1),
		map[string]float64{"dash_verbs": float64(verbTotal(dashed))}})

	// L2 corner: radius ≤ 0 means no effect, and on a right-angle source every
	// vertex is pulled in by exactly r(√2−1) along the bisector while the output
	// stays inside the source's neighbourhood.
	cornerSrc := steps(G.aX0, G.aRows[1]+22, 96, 44, 3)
	nilRadius := render.NewCornerPathEffect(0) == nil && render.NewCornerPathEffect(-3) == nil
	corn, okCorn := E.corner.FilterPath(cornerSrc)
	cSamples := corn.Flatten(0.05)
	// A right-angle cut leaves the vertex at 0.35r…0.42r from the rounded output
	// (the exact value depends on the arc's chord construction), the same at every
	// corner, and half the radius halves the cut.
	dists := []float64{}
	for _, v := range cornerVertices(cornerSrc) {
		dists = append(dists, distToAny(cSamples, v))
	}
	minD, maxD := math.Inf(1), 0.0
	for _, d := range dists {
		if d < minD {
			minD = d
		}
		if d > maxD {
			maxD = d
		}
	}
	drift := 0.0
	for _, p := range cSamples {
		if dd := distToPolyline(cornerSrc, p); dd > drift {
			drift = dd
		}
	}
	half, okHalf := E.halfCorner.FilterPath(cornerSrc)
	halfMin := 0.0
	if okHalf {
		hs := half.Flatten(0.05)
		halfMin = math.Inf(1)
		for _, v := range cornerVertices(cornerSrc) {
			if d := distToAny(hs, v); d < halfMin {
				halfMin = d
			}
		}
	}
	ok2 := nilRadius && okCorn && verbTotal(corn) > verbTotal(cornerSrc) &&
		minD >= G.aCornerR*0.30 && maxD <= G.aCornerR*0.45 &&
		maxD-minD <= G.aCornerR*0.05 && drift <= G.aCornerR &&
		okHalf && math.Abs(halfMin-minD/2) <= G.aCornerR*0.06
	add(logicResult{"corner_effect_cuts_right_angles", ok2,
		fmt.Sprintf("radius≤0 → nil=%v; verbs %d→%d; vertex-to-arc distance %.2f..%.2fpx (r=%.0f → 0.30r..0.45r, spread %.2f); halving r gives %.2f (want %.2f); max drift off the source=%.2fpx",
			nilRadius, verbTotal(cornerSrc), verbTotal(corn), minD, maxD, G.aCornerR, maxD-minD,
			halfMin, minD/2, drift),
		map[string]float64{"corner_verbs": float64(verbTotal(corn)), "cut_min": minD, "cut_max": maxD,
			"cut_half_radius": halfMin, "corner_drift": drift}})

	// L3 discrete jitters within its declared deviation, deterministically.
	d1, okD1 := E.discrete.FilterPath(src)
	d2, okD2 := E.discrete.FilterPath(src)
	maxDev := 0.0
	for _, p := range flatPts(d1) {
		if d := distToPolyline(src, p); d > maxDev {
			maxDev = d
		}
	}
	ok3 := okD1 && okD2 && pathCoordsEqual(d1, d2) && maxDev > 0.5 && maxDev <= G.aDev*1.4
	add(logicResult{"discrete_within_deviation_deterministic", ok3,
		fmt.Sprintf("worst departure=%.3fpx (deviation=%.1f, cap 1.4x), repeat FilterPath identical=%v",
			maxDev, G.aDev, pathCoordsEqual(d1, d2)),
		map[string]float64{"discrete_max_dev": maxDev}})

	// L4 translate stamping walks [offset, length) by advance and anchors the
	// prototype on the sample point (Skia's loop bounds, no end-point stamp).
	length := circ.TotalLength(metricTol)
	wantStamps := 0
	for d := offset; d < length; d += G.bAdvance {
		wantStamps++
	}
	stamped, okS := E.oneD[0].FilterPath(circ)
	anchor, _ := circ.PositionAt(offset, metricTol)
	// The first stamp's centroid must sit on the sample point the walker chose:
	// the prototype is a triangle whose centroid is its own anchor offset.
	triCent := centroidOf(contourPts(stamped, 1))
	protoCent := centroidOf(contourPts(protoTri(G.bProto), 1))
	cent := render.Pt(triCent.X-protoCent.X, triCent.Y-protoCent.Y)
	ok4 := okS && wantStamps > 3 && moveToCount(stamped) == wantStamps &&
		verbTotal(stamped) >= 3*wantStamps &&
		math.Hypot(cent.X-anchor.X, cent.Y-anchor.Y) <= 0.6
	add(logicResult{"one_d_translate_count_and_anchor", ok4,
		fmt.Sprintf("length=%.2f offset=%.2f advance=%.1f → %d stamps (verbs=%d, 4 per triangle), first-stamp centroid Δ=%.3fpx",
			length, offset, G.bAdvance, wantStamps, verbTotal(stamped),
			math.Hypot(cent.X-anchor.X, cent.Y-anchor.Y)),
		map[string]float64{"stamps": float64(wantStamps), "anchor_err": math.Hypot(cent.X-anchor.X, cent.Y-anchor.Y)}})

	// L5 rotate turns the prototype with the tangent; translate must not follow.
	rotated, okR := E.oneD[1].FilterPath(circ)
	d2nd := offset + G.bAdvance
	_, tan, okTan := circ.TangentAt(d2nd, metricTol)
	tanAng := math.Atan2(tan.Y, tan.X)
	rotAng := firstContourAngle(rotated, 1)
	traAng := firstContourAngle(stamped, 1)
	ok5 := okR && okTan && verbTotal(rotated) == verbTotal(stamped) &&
		math.Abs(angDiff(rotAng, tanAng)) <= 0.05 && math.Abs(angDiff(traAng, tanAng)) > 0.05
	add(logicResult{"one_d_rotate_follows_tangent", ok5,
		fmt.Sprintf("stamp#2 at d=%.2f: tangent=%.3f rad, rotate edge=%.3f (Δ=%.3f), translate edge=%.3f (Δ=%.3f)",
			d2nd, tanAng, rotAng, angDiff(rotAng, tanAng), traAng, angDiff(traAng, tanAng)),
		map[string]float64{"rotate_angle_err": math.Abs(angDiff(rotAng, tanAng)), "translate_angle_err": math.Abs(angDiff(traAng, tanAng))}})

	// L6 morph maps the prototype's x along the arc and y along the normal:
	// rebuild the second stamp's four corners analytically and demand that each
	// of them is present in the engine's output.
	morphed, okM := E.oneD[2].FilterPath(circ)
	mf := flatPts(morphed)
	bw, bh := G.bProto*1.4, G.bProto*0.45
	maxMapErr := 0.0
	for _, c := range [][2]float64{{-bw, -bh}, {bw, -bh}, {bw, bh}, {-bw, bh}} {
		pos, tk, ok := circ.TangentAt(d2nd+c[0], metricTol)
		if !ok {
			maxMapErr = math.Inf(1)
			break
		}
		exp := render.Pt(pos.X-tk.Y*c[1], pos.Y+tk.X*c[1])
		if d := distToAny(mf, exp); d > maxMapErr {
			maxMapErr = d
		}
	}
	ok6 := okM && maxMapErr <= 0.8
	add(logicResult{"one_d_morph_arc_and_normal", ok6,
		fmt.Sprintf("second stamp's corners reproduce x-along-arc / y-along-normal mapping to ≤%.3fpx", maxMapErr),
		map[string]float64{"morph_err": maxMapErr}})

	// L7 phase follows the PostScript reduction, and a pathological advance is
	// refused by the density governor instead of spinning the walker forever.
	negPath, okNeg := render.NewPath1DPathEffect(protoTri(G.bProto), render.Path1DTranslate,
		G.bAdvance, -G.bAdvance*0.25).FilterPath(circ)
	posPath, okPos := render.NewPath1DPathEffect(protoTri(G.bProto), render.Path1DTranslate,
		G.bAdvance, G.bAdvance*0.75).FilterPath(circ)
	phaseSame := okNeg && okPos && pathCoordsEqual(negPath, posPath)
	tGov := time.Now()
	refused, okRefused := E.pathological.FilterPath(circ)
	govDur := time.Since(tGov).Seconds()
	nilGuards := render.NewPath1DPathEffect(protoTri(5), render.Path1DTranslate, 0, 0) == nil &&
		render.NewPath1DPathEffect(nil, render.Path1DTranslate, 10, 0) == nil &&
		render.NewPath1DPathEffect(protoTri(5), render.Path1DTranslate, 10, math.NaN()) == nil &&
		render.NewCornerPathEffect(math.NaN()) == nil &&
		render.NewCornerPathEffect(-3) == nil &&
		render.NewDiscretePathEffect(10, math.Inf(1)) == nil &&
		render.NewDashPathEffect(render.NewDash(0)) == nil &&
		render.NewPath2DPathEffect(protoTri(4), 0, 5) == nil
	ok7 := phaseSame && !okRefused && refused == nil && govDur < 1.0 && nilGuards
	add(logicResult{"phase_rule_and_density_governor", ok7,
		fmt.Sprintf("phase=-0.25a ≡ +0.75a: %v; pathological refused=%v in %.4fs (governor 100000); degenerate constructors return nil: %v",
			phaseSame, !okRefused, govDur, nilGuards),
		map[string]float64{"governor_seconds": govDur}})

	// L8 Sum unions both effects on one source; Compose is outer(inner(src)) and
	// the two orders are different geometries.
	sumP, okSum := E.sum.FilterPath(src)
	aP, okA := E.dash.FilterPath(src)
	bP, okB := E.corner.FilterPath(src)
	inner, okInner := E.discrete.FilterPath(src)
	manual, okManual := E.corner.FilterPath(inner)
	cAB, okAB := E.compAB.FilterPath(src)
	cBA, okBA := E.compBA.FilterPath(src)
	ok8 := okSum && okA && okB && okInner && okManual && okAB && okBA &&
		verbTotal(sumP) == verbTotal(aP)+verbTotal(bP) && containsAll(sumP, aP) && containsAll(sumP, bP) &&
		pathCoordsEqual(cAB, manual) && !pathCoordsEqual(cAB, cBA)
	add(logicResult{"sum_unions_compose_orders", ok8,
		fmt.Sprintf("sum verbs=%d == dash %d + corner %d and contains both; compose(corner,discrete) ≡ manual %v; reversed differs (%d vs %d verbs)",
			verbTotal(sumP), verbTotal(aP), verbTotal(bP), pathCoordsEqual(cAB, manual), verbTotal(cAB), verbTotal(cBA)),
		map[string]float64{"sum_verbs": float64(verbTotal(sumP)), "compose_ab": float64(verbTotal(cAB)),
			"compose_ba": float64(verbTotal(cBA))}})

	// L9 cache identity: built-ins key on parameters, a value-shaped custom
	// effect reports 0 (unkeyable), a reference-shaped one keys on the instance.
	h1 := render.PathEffectHash(render.NewDashPathEffect(render.NewDash(G.aDash[0], G.aDash[1])))
	h2 := render.PathEffectHash(render.NewDashPathEffect(render.NewDash(G.aDash[0], G.aDash[1])))
	h3 := render.PathEffectHash(render.NewDashPathEffect(render.NewDash(G.aDash[0], G.aDash[1]+3)))
	hCorner := render.PathEffectHash(E.corner)
	hScaled := render.PathEffectHash(render.ScalePathEffect(E.corner, 2))
	hSame := render.PathEffectHash(render.ScalePathEffect(E.corner, 1))
	var customVal render.PathEffect = valueEffect{amp: 3}
	customA, customB := &valueEffect{amp: 4}, &valueEffect{amp: 4}
	ok9 := h1 != 0 && h1 == h2 && h1 != h3 && hScaled != hCorner && hSame == hCorner &&
		render.PathEffectHash(customVal) == 0 && render.PathEffectHash(nil) == 0 &&
		render.PathEffectHash(customA) != 0 && render.PathEffectHash(customA) == render.PathEffectHash(customA) &&
		render.PathEffectHash(customA) != render.PathEffectHash(customB)
	add(logicResult{"effect_hash_identity", ok9,
		fmt.Sprintf("equal params share a key (%016x) and differing ones do not; scale folds into the key; value custom → 0, pointer custom → per-instance (%016x vs %016x)",
			h1, render.PathEffectHash(customA), render.PathEffectHash(customB)),
		map[string]float64{"hash_stability_checks": 6}})

	// L10 the mount drives the raster, and it wins over the convenience Dash
	// field (Skia's single effect slot).
	plainSt := render.DefaultStroke().WithWidth(6).WithJoin(render.LineJoinRound)
	effSt := plainSt.WithPathEffect(E.corner)
	bothSt := plainSt.WithDash(render.NewDash(G.aDash[0], G.aDash[1])).WithPathEffect(E.corner)
	plainInk := strokeInk(src, plainSt)
	effInk := strokeInk(src, effSt)
	bothInk := strokeInk(src, bothSt)
	dashInk := strokeInk(src, plainSt.WithDash(render.NewDash(G.aDash[0], G.aDash[1])))
	ok10 := plainInk > 0 && effInk > 0 && math.Abs(float64(effInk-bothInk)) <= 2 &&
		math.Abs(float64(plainInk-effInk)) > 20 && dashInk != effInk &&
		effSt.HasPathEffect() && !plainSt.HasPathEffect()
	add(logicResult{"stroke_mount_drives_raster_wins_over_dash", ok10,
		fmt.Sprintf("ink px: plain=%d corner-effect=%d effect+dash=%d (must equal the effect) dash-only=%d; HasPathEffect mounts=%v",
			plainInk, effInk, bothInk, dashInk, effSt.HasPathEffect()),
		map[string]float64{"ink_plain": float64(plainInk), "ink_effect": float64(effInk), "ink_effect_plus_dash": float64(bothInk)}})

	return out
}

// ─────────────────────────── logic helpers ───────────────────────────

// cornerVertices are the interior vertices of a polyline path (the right angles
// the corner effect must cut).
func cornerVertices(p *render.Path) []render.Point {
	pts := flatPts(p)
	if len(pts) < 3 {
		return nil
	}
	return pts[1 : len(pts)-1]
}

// normalizePhase mirrors the documented PostScript reduction so the probes can
// predict where the first stamp lands.
func normalizePhase(phase, advance float64) float64 {
	if phase < 0 {
		phase = -phase
		if phase > advance {
			phase = math.Mod(phase, advance)
		}
	} else {
		if phase > advance {
			phase = math.Mod(phase, advance)
		}
		phase = advance - phase
	}
	if phase >= advance {
		phase = 0
	}
	return phase
}

// contourPtsClosed returns the n-th contour's vertices (1-based) and whether it
// carries a Close verb.
func contourPtsClosed(p *render.Path, n int) ([]render.Point, bool) {
	if p == nil {
		return nil, false
	}
	out := []render.Point{}
	cur, want, closed := 1, n == 1, false
	p.Iterate(func(v render.PathVerb, c []float64) {
		if v == render.MoveTo {
			want = cur == n
			cur++
		}
		if !want {
			return
		}
		switch v {
		case render.MoveTo, render.LineTo:
			out = append(out, render.Pt(c[0], c[1]))
		case render.QuadTo:
			out = append(out, render.Pt(c[2], c[3]))
		case render.CubicTo:
			out = append(out, render.Pt(c[4], c[5]))
		case render.Close:
			closed = true
		}
	})
	if len(out) == 0 {
		return nil, false
	}
	return out, closed
}

// contourPts is the vertex chain of the n-th contour (1-based).
func contourPts(p *render.Path, n int) []render.Point {
	pts, _ := contourPtsClosed(p, n)
	return pts
}

func centroidOf(pts []render.Point) render.Point {
	var sum render.Point
	for _, p := range pts {
		sum.X += p.X
		sum.Y += p.Y
	}
	if len(pts) == 0 {
		return render.Point{}
	}
	return render.Pt(sum.X/float64(len(pts)), sum.Y/float64(len(pts)))
}

// firstContourAngle is the direction of the leading edge of the n-th contour
// (1-based): MoveTo opens a contour, its first LineTo gives the edge.
func firstContourAngle(p *render.Path, n int) float64 {
	if p == nil {
		return 0
	}
	cur := 0
	onTarget, got := false, false
	var first render.Point
	angle := 0.0
	p.Iterate(func(v render.PathVerb, c []float64) {
		if got {
			return
		}
		switch v {
		case render.MoveTo:
			onTarget = cur == n
			cur++
			if onTarget {
				first = render.Pt(c[0], c[1])
			}
		case render.LineTo:
			if onTarget {
				angle = math.Atan2(c[1]-first.Y, c[0]-first.X)
				got = true
			}
		}
	})
	if !got {
		return 0
	}
	return angle
}

func angDiff(a, b float64) float64 {
	d := a - b
	for d > math.Pi {
		d -= 2 * math.Pi
	}
	for d < -math.Pi {
		d += 2 * math.Pi
	}
	return d
}

// distToPolyline measures the distance from q to the source's flattened
// polyline (segments, not just vertices).
func distToPolyline(p *render.Path, q render.Point) float64 {
	pts := flatPts(p)
	best := math.Inf(1)
	for i := 1; i < len(pts); i++ {
		if d := pointSegDist(q, pts[i-1], pts[i]); d < best {
			best = d
		}
	}
	return best
}

func pointSegDist(q, a, b render.Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	if dx == 0 && dy == 0 {
		return math.Hypot(q.X-a.X, q.Y-a.Y)
	}
	t := ((q.X-a.X)*dx + (q.Y-a.Y)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return math.Hypot(q.X-(a.X+t*dx), q.Y-(a.Y+t*dy))
}

// containsAll reports whether dst carries every coordinate pair of src: the sum
// effect appends both filtered copies of the same source.
func containsAll(dst, src *render.Path) bool {
	if dst == nil || src == nil {
		return false
	}
	dc, sc := dst.Coords(), src.Coords()
	if len(dc) < len(sc) {
		return false
	}
	used := make([]bool, len(dc)/2)
	for i := 0; i+1 < len(sc); i += 2 {
		hit := false
		for j := 0; j+1 < len(dc); j += 2 {
			k := j / 2
			if used[k] || dc[j] != sc[i] || dc[j+1] != sc[i+1] {
				continue
			}
			used[k] = true
			hit = true
			break
		}
		if !hit {
			return false
		}
	}
	return true
}

// valueEffect is a custom, value-shaped effect: no cache can key its identity,
// so PathEffectHash reports 0 and callers must apply it directly.
type valueEffect struct{ amp float64 }

func (v valueEffect) FilterPath(src *render.Path) (*render.Path, bool) {
	if src == nil || src.NumVerbs() == 0 {
		return nil, false
	}
	return src.Transform(render.Scale(1+v.amp/100, 1-v.amp/100)), true
}

// strokeInk rasterizes the source with the given stroke on the CPU path and
// counts ink pixels — a crude but exact-enough measure of "the mount drives the
// raster".
func strokeInk(src *render.Path, st render.Stroke) int {
	dc := render.NewContext(440, 320)
	defer dc.Close()
	dc.ClearWithColor(render.RGBA{R: 0, G: 0, B: 0, A: 1})
	dc.SetStroke(st)
	dc.SetRGBA(1, 1, 1, 1)
	dc.DrawPath(src)
	if err := dc.Stroke(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_p35_patheffect: strokeInk: %v\n", err)
		return -1
	}
	_ = dc.FlushGPU()
	img := dc.Image()
	ink := 0
	for y := 0; y < 320; y++ {
		for x := 0; x < 440; x++ {
			r, _, _, _ := img.At(x, y).RGBA()
			if r>>8 > 40 {
				ink++
			}
		}
	}
	return ink
}

// ─────────────────────────── main ───────────────────────────

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: snapshot + probes + golden")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until closed)")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly {
		secs = closeSeconds
	} else if secsSet {
		wrkit.RequireMinRun(secs, "P35-patheffect")
	} else if *manualSeconds > 0 {
		secs = int(*manualSeconds)
	}
	wrkit.EnsureUIFace()
	buildEffects()
	buildScene()

	logic := runLogicProbes()
	logicOK, logicTotal := 0, len(logic)
	for _, l := range logic {
		fmt.Fprintf(os.Stderr, "ui_wr_p35_patheffect: logic %-42s ok=%v %s\n", l.name, l.ok, l.det)
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

	shell := wrkit.NewShell(winW, winH, "P3.5 PathEffect — 五构造器 / 1D 三形态 / Sum·Compose / 密度护栏", []string{
		"A 红=Dash 效果（灰=未加效果原线）",
		"A 绿=Corner r=18：尖角切弧",
		"A 蓝=Discrete 抖动（deviation 内）",
		"B 黄=1D Translate 青=1D Rotate 紫=1D Morph",
		"B 绿=2D 网格铺源包围盒",
		"C 橙=Sum(dash,corner)：两效果并集",
		"C 青=Compose(corner,discrete)，品红=反序",
		"C 上圆=正常 advance 对照，下框=病态 advance",
		"C 病态整条拒绝：框内无几何（金标签）",
		"探针点全部取自效果输出路径本身",
		"门禁: 逻辑 10 条 + 像素 23 条 + 金图逐位",
	})

	type cardSpec struct {
		x, y, w, h float64
		title      string
		paint      func(*rendering.PaintContext, rendering.Size)
	}
	specs := []cardSpec{
		{cardAX, cardAY, cardAW, cardAH, "A 构造器 1-3：dash / corner / discrete", makePainter(0)},
		{cardBX, cardBY, cardBW, cardBH, "B 1D 三形态 + 2D 网格", makePainter(1)},
		{cardCX, cardCY, cardCW, cardCH, "C Sum vs Compose（顺序可辨）+ 密度护栏", makePainter(2)},
	}
	for _, cs := range specs {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cs.w, cs.h
		box.OnPaint = cs.paint
		shell.Body.Place(box, cs.x, cs.y)
		shell.Body.LabelAt(cs.title, 11, cs.x+8, cs.y+6, 0.86, 0.90, 0.96)
	}

	snapDir := filepath.Join("examples", "ui_wr_p35_patheffect", "testdata")
	_ = os.MkdirAll(snapDir, 0o755)
	snapPath := filepath.Join(snapDir, "p35_patheffect_final.png")
	basePath := filepath.Join(snapDir, "goldens", "p35_patheffect_final.png")

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

	probes := buildProbes()
	failedPixels := 0
	var firstPixelErr error
	for _, pr := range probes {
		if err := wrkit.CheckPixelProbes(snapPath, winW, []wrkit.PixelProbe{pr}); err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_p35_patheffect: pixel %-46s FAIL %v\n", pr.Name, err)
			failedPixels++
			if firstPixelErr == nil {
				firstPixelErr = err
			}
		} else {
			fmt.Fprintf(os.Stderr, "ui_wr_p35_patheffect: pixel %-46s ok\n", pr.Name)
		}
	}

	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGate(snapPath, basePath, wrkit.ShellContentRects())

	extra := map[string]any{
		"fps_gate": "off_correctness", "slope_gate": "off",
		"apis":            "PathEffect/Stroke.WithPathEffect/NewDash·Corner·Discrete·Path1D·Path2D/Sum·Compose/PathEffectHash/ScalePathEffect",
		"logic_probes_ok": logicOK, "logic_probes_total": logicTotal,
		"pixel_probes_total": len(probes), "pixel_probes_failed": failedPixels,
		"dash": G.aDash, "corner_radius": G.aCornerR, "discrete": [2]float64{G.aSeg, G.aDev},
		"advance": G.bAdvance, "phase": G.bPhase, "stamp_advance_is_user_space": true,
		"pixel_golden_diff_pct": goldenDiff, "pixel_golden_total_px": goldenPx,
		"pixel_golden_first_run": seeded,
	}
	for i, l := range logic {
		extra[fmt.Sprintf("logic_%02d_%s", i+1, l.name)] = l.ok
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "P35-patheffect", Scenario: "ui_wr_p35_patheffect", Backend: win.Backend().String(),
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
	if len(probes) < 12 {
		fail("only %d pixel probes built", len(probes))
	}
	if failedPixels != 0 {
		fail("pixel probes: %d/%d failed, first: %v", failedPixels, len(probes), firstPixelErr)
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
	fmt.Fprintf(os.Stderr, "ui_wr_p35_patheffect: OK presents=%d logic=%d/%d pixel=%d/%d golden=%.4f%% seeded=%v gate=%v elapsed=%.1fs\n",
		app.PresentCount(), logicOK, logicTotal, len(probes)-failedPixels, len(probes),
		goldenDiff, seeded, *autoOnly, elapsed)
}

type tickFunc struct{ on func(dt float64) }

func (t *tickFunc) Tick(dt float64) bool { t.on(dt); return true }
