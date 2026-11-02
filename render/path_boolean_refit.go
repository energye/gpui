//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"github.com/ctessum/go.clipper"
)

// ---------------------------------------------------------------------------
// Boolean curve provenance + refit (P3.5 批 5，D6 混合法).
//
// Clipper still decides the topology: the input curves are flattened onto its
// integer grid exactly as before, so intersection placement is unchanged. What
// changes is what comes back — every flattened sample records which input curve
// segment it came from and at what parameter, and a solution contour's run of
// consecutive samples of one segment is rebuilt as that segment's exact
// Subsegment. Straight edges and the intersection vertices Clipper inserted
// stay verbatim.
//
// Quad and cubic samples track their parameter exactly (a restricted range
// subdivided at its midpoint lands on the parent curve's midpoint parameter —
// pinned by TestBooleanFlattenPointSequenceUnchanged). Conics are sampled by
// adaptive splits on their own parameter: the untraced flattener degrades them
// to quads first and cannot report a parent parameter, so this is the one place
// the polygon changes (same tolerance, conics only).
// ---------------------------------------------------------------------------

// boolGP is a polygon vertex on Clipper's integer grid — the key curve samples
// are recorded under.
type boolGP struct{ x, y clipper.CInt }

// boolCurveRun is one input curve segment plus the flattened samples it emitted.
type boolCurveRun struct {
	kind PathVerb // QuadTo, CubicTo or ConicTo
	p0   Point    // segment start
	p2   Point    // segment end
	c1   Point    // quad/conic control, cubic control 1
	c2   Point    // cubic control 2 (zero for quad/conic)
	w    float64  // conic weight (1 for quad/cubic)
	ts   []float64
	gp   []boolGP
}

// boolRef points at one recorded sample.
type boolRef struct {
	run    int
	sample int
}

// boolTrace records the flattened curve samples of one operand path.
type boolTrace struct {
	runs []boolCurveRun
	byGP map[boolGP][]boolRef
}

func newBoolTrace() *boolTrace {
	return &boolTrace{byGP: make(map[boolGP][]boolRef)}
}

func (t *boolTrace) newRun(kind PathVerb, p0, p2, c1, c2 Point, w float64) int {
	t.runs = append(t.runs, boolCurveRun{kind: kind, p0: p0, p2: p2, c1: c1, c2: c2, w: w})
	return len(t.runs) - 1
}

// addSample records one curve sample. Samples whose grid vertex repeats the
// previous one are not recorded: the polygon carries a single vertex there, and
// keeping samples 1:1 with vertices is what lets the walk chain by ±1.
func (t *boolTrace) addSample(run int, tParam float64, gp boolGP, fresh bool) {
	if !fresh {
		return
	}
	r := &t.runs[run]
	r.ts = append(r.ts, tParam)
	r.gp = append(r.gp, gp)
	t.byGP[gp] = append(t.byGP[gp], boolRef{run: run, sample: len(r.gp) - 1})
}

// recordSharedStart attaches a fresh run's t=0 endpoint to the vertex the
// previous verb already emitted (curve runs meet at a shared endpoint without
// adding a polygon vertex). Without it the boundary between two authored
// segments would come back as a chord; with it the walk can carry straight on
// into the next run from that same vertex, so an untouched contour reproduces
// exactly.
func (t *boolTrace) recordSharedStart(run int, gp boolGP, hasVertex bool) {
	if !hasVertex {
		return
	}
	r := &t.runs[run]
	r.ts = append(r.ts, 0)
	r.gp = append(r.gp, gp)
	t.byGP[gp] = append(t.byGP[gp], boolRef{run: run, sample: 0})
}

// lookup returns the samples that flattened onto this grid vertex.
func (t *boolTrace) lookup(p *clipper.IntPoint) []boolRef {
	if t == nil || len(t.runs) == 0 {
		return nil
	}
	return t.byGP[boolGP{x: p.X, y: p.Y}]
}

func (r *boolCurveRun) quad() QuadBez   { return NewQuadBez(r.p0, r.c1, r.p2) }
func (r *boolCurveRun) cubic() CubicBez { return NewCubicBez(r.p0, r.c1, r.c2, r.p2) }
func (r *boolCurveRun) conic() ConicBez { return NewConicBez(r.p0, r.c1, r.p2, r.w) }

// ---------------------------------------------------------------------------
// Parameter-tracked flattening. The quad and cubic point sequences are
// identical to pathops_bbox.go's flatteners (same flatness test, same
// recursion); only the parameter is carried along.
// ---------------------------------------------------------------------------

func flattenQuadT(q QuadBez, tolerance float64, emit func(pt Point, t float64)) {
	flattenQuadRecursiveT(q, tolerance*tolerance, emit, 0, 1, 0)
}

func flattenQuadRecursiveT(q QuadBez, toleranceSq float64, emit func(Point, float64), t0, t1 float64, depth int) {
	if depth > 10 {
		emit(q.P2, t1)
		return
	}
	mid := q.P0.Lerp(q.P2, 0.5)
	dist := q.P1.Sub(mid)
	if dist.LengthSquared() <= toleranceSq {
		emit(q.P2, t1)
		return
	}
	tm := 0.5*t0 + 0.5*t1
	q1, q2 := q.Subdivide()
	flattenQuadRecursiveT(q1, toleranceSq, emit, t0, tm, depth+1)
	flattenQuadRecursiveT(q2, toleranceSq, emit, tm, t1, depth+1)
}

func flattenCubicT(c CubicBez, tolerance float64, emit func(pt Point, t float64)) {
	flattenCubicRecursiveT(c, tolerance*tolerance, emit, 0, 1, 0)
}

func flattenCubicRecursiveT(c CubicBez, toleranceSq float64, emit func(Point, float64), t0, t1 float64, depth int) {
	if depth > 10 {
		emit(c.P3, t1)
		return
	}
	if cubicFlatness(c) <= toleranceSq*16 { // same metric scale as flattenCubicRecursive
		emit(c.P3, t1)
		return
	}
	tm := 0.5*t0 + 0.5*t1
	c1, c2 := c.Subdivide()
	flattenCubicRecursiveT(c1, toleranceSq, emit, t0, tm, depth+1)
	flattenCubicRecursiveT(c2, toleranceSq, emit, tm, t1, depth+1)
}

// flattenConicT splits the conic on its own parameter, emitting exact conic
// points with their parameter. A span is accepted only when both error sources
// are bounded: the conic-to-quad approximation, and the quad's own deviation
// from its chord (the same flatness budget the untraced flatteners use, so the
// polygon keeps the documented tolerance).
func flattenConicT(c ConicBez, tolerance float64, emit func(pt Point, t float64)) {
	flattenConicRecursiveT(c, tolerance, emit, 0, 1, 0)
}

func flattenConicRecursiveT(c ConicBez, tolerance float64, emit func(Point, float64), t0, t1 float64, depth int) {
	const maxDepth = 18
	if depth >= maxDepth {
		emit(c.Eval(t1), t1)
		return
	}
	sub := c.Subsegment(t0, t1)
	q := sub.quadApprox()
	chord := q.P1.Sub(q.P0.Lerp(q.P2, 0.5)).Length()
	if sub.quadError(q) <= tolerance*0.5 && chord <= tolerance*0.5 {
		emit(c.Eval(t1), t1)
		return
	}
	tm := 0.5*t0 + 0.5*t1
	flattenConicRecursiveT(c, tolerance, emit, t0, tm, depth+1)
	flattenConicRecursiveT(c, tolerance, emit, tm, t1, depth+1)
}

// ---------------------------------------------------------------------------
// Traced Clipper input
// ---------------------------------------------------------------------------

// pathToClipperPathsTraced flattens p into closed Clipper contours and records
// each curve sample's segment id and parameter. Contours with fewer than three
// distinct points carry no region and are dropped (unchanged rule).
func pathToClipperPathsTraced(p *Path) (clipper.Paths, *boolTrace) {
	trace := newBoolTrace()
	var out clipper.Paths
	var cur clipper.Path
	var current, start Point

	closeContour := func() {
		if len(cur) >= 3 {
			out = append(out, cur)
		}
		cur = nil
	}
	// appendVertex adds a grid vertex unless it repeats the previous one — the
	// same dedup the untraced builder applies, keeping polygons identical. It
	// reports whether the vertex is new.
	appendVertex := func(x, y float64) (boolGP, bool) {
		gp := boolGP{x: pathBooleanCoord(x), y: pathBooleanCoord(y)}
		if n := len(cur); n > 0 && cur[n-1].X == gp.x && cur[n-1].Y == gp.y {
			return gp, false
		}
		pt := clipper.IntPoint{X: gp.x, Y: gp.y}
		cur = append(cur, &pt)
		return gp, true
	}
	lastVertex := func() (boolGP, bool) {
		if n := len(cur); n > 0 {
			return boolGP{x: cur[n-1].X, y: cur[n-1].Y}, true
		}
		return boolGP{}, false
	}

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			closeContour()
			current = Pt(coords[0], coords[1])
			start = current
			appendVertex(coords[0], coords[1])
		case LineTo:
			current = Pt(coords[0], coords[1])
			appendVertex(coords[0], coords[1])
		case QuadTo:
			ctrl, end := Pt(coords[0], coords[1]), Pt(coords[2], coords[3])
			run := trace.newRun(QuadTo, current, end, ctrl, Point{}, 1)
			gp0, has0 := lastVertex()
			trace.recordSharedStart(run, gp0, has0)
			flattenQuadT(NewQuadBez(current, ctrl, end), pathBooleanFlattenTolerance,
				func(pt Point, t float64) {
					gp, fresh := appendVertex(pt.X, pt.Y)
					trace.addSample(run, t, gp, fresh)
				})
			current = end
		case CubicTo:
			c1, c2 := Pt(coords[0], coords[1]), Pt(coords[2], coords[3])
			end := Pt(coords[4], coords[5])
			run := trace.newRun(CubicTo, current, end, c1, c2, 1)
			gp0, has0 := lastVertex()
			trace.recordSharedStart(run, gp0, has0)
			flattenCubicT(NewCubicBez(current, c1, c2, end), pathBooleanFlattenTolerance,
				func(pt Point, t float64) {
					gp, fresh := appendVertex(pt.X, pt.Y)
					trace.addSample(run, t, gp, fresh)
				})
			current = end
		case ConicTo:
			ctrl, end := Pt(coords[0], coords[1]), Pt(coords[2], coords[3])
			w := coords[4]
			if w == 1 { // A unit-weight conic is its own quadratic.
				run := trace.newRun(QuadTo, current, end, ctrl, Point{}, 1)
				gp0, has0 := lastVertex()
				trace.recordSharedStart(run, gp0, has0)
				flattenQuadT(NewQuadBez(current, ctrl, end), pathBooleanFlattenTolerance,
					func(pt Point, t float64) {
						gp, fresh := appendVertex(pt.X, pt.Y)
						trace.addSample(run, t, gp, fresh)
					})
				current = end
				return
			}
			run := trace.newRun(ConicTo, current, end, ctrl, Point{}, w)
			gp0, has0 := lastVertex()
			trace.recordSharedStart(run, gp0, has0)
			flattenConicT(NewConicBez(current, ctrl, end, w), pathBooleanFlattenTolerance,
				func(pt Point, t float64) {
					gp, fresh := appendVertex(pt.X, pt.Y)
					trace.addSample(run, t, gp, fresh)
				})
			current = end
		case Close:
			if current != start {
				appendVertex(start.X, start.Y)
			}
			current = start
		}
	})
	closeContour()
	return out, trace
}

// ---------------------------------------------------------------------------
// Refit
// ---------------------------------------------------------------------------

// solutionRef is a sample lookup result, tagged with the operand it came from.
type solutionRef struct {
	tr     *boolTrace
	run    int
	sample int
}

// curveSpan is a chain of `edges` output edges covered by one curve run, from
// the parameter of the chain's first vertex to that of its last. `reversed`
// marks a contour that walks the run backwards (Clipper is free to flip contour
// orientation).
type curveSpan struct {
	run      *boolCurveRun
	t0, t1   float64
	reversed bool
	edges    int
}

// refitClipperPaths rebuilds solution contours: runs of consecutive samples of
// one input curve segment become that segment's exact subsegment, everything
// else stays the polyline Clipper emitted.
func refitClipperPaths(paths clipper.Paths, ta, tb *boolTrace) *Path {
	out := NewPath()
	for _, c := range paths {
		if len(c) < 3 {
			continue
		}
		refitContour(out, c, ta, tb)
	}
	return out
}

// refitContour emits one closed contour of the solution.
func refitContour(out *Path, c clipper.Path, ta, tb *boolTrace) {
	n := len(c)
	refs := make([][]solutionRef, n)
	for i, p := range c {
		for _, r := range ta.lookup(p) {
			refs[i] = append(refs[i], solutionRef{tr: ta, run: r.run, sample: r.sample})
		}
		for _, r := range tb.lookup(p) {
			refs[i] = append(refs[i], solutionRef{tr: tb, run: r.run, sample: r.sample})
		}
	}

	// Start where the walk can chain cleanly: an untraced vertex (a corner or an
	// intersection Clipper inserted) breaks no run, and a fully traced contour
	// starts at a run boundary (an authored segment seam) so an untouched
	// contour comes back verb-for-verb instead of split at whatever vertex
	// Clipper chose to rotate it to.
	start := -1
	for i := 0; i < n; i++ {
		if len(refs[i]) == 0 {
			start = i
			break
		}
	}
	if start < 0 {
		for i := 0; i < n; i++ {
			if isRunBoundary(refs[i]) {
				start = i
				break
			}
		}
	}
	if start < 0 {
		start = 0
	}
	sp := ptPath(c[start])
	out.MoveTo(sp.X, sp.Y)

	// Walk the ring once. A chain may end on the start vertex (the closing edge
	// is then a curve, not a chord), which is what lets an untouched contour
	// reproduce its authored segments seam-for-seam.
	emitted, prev := 0, start
	for emitted <= n {
		span, ok := matchCurveSpan(refs, n, prev, n-emitted)
		if ok {
			emitCurveSpan(out, span)
			emitted += span.edges
			prev = (prev + span.edges) % n
			if prev == start {
				break
			}
			continue
		}
		next := (prev + 1) % n
		if next == start {
			break
		}
		lp := ptPath(c[next])
		out.LineTo(lp.X, lp.Y)
		emitted++
		prev = next
	}
	out.Close()
}

// emitCurveSpan writes the span's exact subsegment. A reversed span emits the
// same curve flipped: control points swap pairwise, a conic swaps endpoints and
// keeps its weight.
func emitCurveSpan(out *Path, s curveSpan) {
	lo, hi := s.t0, s.t1
	if hi < lo {
		lo, hi = hi, lo
	}
	switch s.run.kind {
	case QuadTo:
		sub := s.run.quad().Subsegment(lo, hi)
		if s.reversed {
			out.QuadraticTo(sub.P1.X, sub.P1.Y, sub.P0.X, sub.P0.Y)
		} else {
			out.QuadraticTo(sub.P1.X, sub.P1.Y, sub.P2.X, sub.P2.Y)
		}
	case CubicTo:
		sub := s.run.cubic().Subsegment(lo, hi)
		if s.reversed {
			out.CubicTo(sub.P2.X, sub.P2.Y, sub.P1.X, sub.P1.Y, sub.P0.X, sub.P0.Y)
		} else {
			out.CubicTo(sub.P1.X, sub.P1.Y, sub.P2.X, sub.P2.Y, sub.P3.X, sub.P3.Y)
		}
	case ConicTo:
		sub := s.run.conic().Subsegment(lo, hi)
		if s.reversed {
			out.ConicTo(sub.P1.X, sub.P1.Y, sub.P0.X, sub.P0.Y, sub.W)
		} else {
			out.ConicTo(sub.P1.X, sub.P1.Y, sub.P2.X, sub.P2.Y, sub.W)
		}
	}
}

// matchCurveSpan finds the longest chain of consecutive samples of a single run
// starting at vertex `prev` and walking forward through the contour. Only
// recorded samples join a chain, so the chain's endpoints are always exact
// curve points and the intersection vertices Clipper created stay as corners.
// Chains shorter than two edges stay polyline: one chord is what the flattened
// polygon already had.
func matchCurveSpan(refs [][]solutionRef, n, prev, maxEdges int) (curveSpan, bool) {
	if maxEdges < 2 || len(refs[prev]) == 0 {
		return curveSpan{}, false
	}
	for _, r0 := range refs[prev] {
		step := 0
		for _, r1 := range refs[(prev+1)%n] {
			if r1.tr != r0.tr || r1.run != r0.run {
				continue
			}
			switch r1.sample - r0.sample {
			case 1:
				step = 1
			case -1:
				step = -1
			}
			if step != 0 {
				break
			}
		}
		if step == 0 {
			continue
		}
		run := &r0.tr.runs[r0.run]
		last, edges := r0.sample+step, 1
		for edges < maxEdges {
			want := r0.sample + step*(edges+1)
			if want < 0 || want >= len(run.ts) {
				break
			}
			matched := false
			for _, r := range refs[(prev+edges+1)%n] {
				if r.tr == r0.tr && r.run == r0.run && r.sample == want {
					matched = true
					break
				}
			}
			if !matched {
				break
			}
			last, edges = want, edges+1
		}
		// Chains of a single chord stay polyline — that is what the flattened
		// polygon already had.
		if edges < 2 {
			return curveSpan{}, false
		}
		return curveSpan{
			run:      run,
			t0:       run.ts[r0.sample],
			t1:       run.ts[last],
			reversed: step < 0,
			edges:    edges,
		}, true
	}
	return curveSpan{}, false
}

// isRunBoundary reports whether this vertex is the first or last sample of one
// of its runs — an authored seam between curve segments.
func isRunBoundary(refs []solutionRef) bool {
	for _, r := range refs {
		run := &r.tr.runs[r.run]
		if r.sample == 0 || r.sample == len(run.ts)-1 {
			return true
		}
	}
	return false
}

// ptPath converts a Clipper grid vertex back to path units.
func ptPath(p *clipper.IntPoint) Point {
	return Pt(float64(p.X)*pathBooleanInvScale, float64(p.Y)*pathBooleanInvScale)
}
