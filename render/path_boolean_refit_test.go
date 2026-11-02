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
	"math"
	"testing"

	"github.com/ctessum/go.clipper"
)

// TestBooleanFlattenPointSequenceUnchanged pins the traced flatteners against
// the untraced ones the Clipper polygons used before batch 5: same points in
// the same order (so intersection placement cannot shift), and each point equal
// to the curve evaluated at the recorded parameter.
func TestBooleanFlattenPointSequenceUnchanged(t *testing.T) {
	quadCases := []QuadBez{
		NewQuadBez(Pt(0, 0), Pt(50, -30), Pt(120, 40)),
		NewQuadBez(Pt(3, 7), Pt(9, 7), Pt(30, 7)),  // flat: one point
		NewQuadBez(Pt(0, 0), Pt(1, 200), Pt(2, 0)), // tight: deep recursion
	}
	for i, q := range quadCases {
		var plain, traced []Point
		var ts []float64
		flattenQuad(q.P0, q.P1, q.P2, pathBooleanFlattenTolerance, func(pt Point) { plain = append(plain, pt) })
		flattenQuadT(q, pathBooleanFlattenTolerance, func(pt Point, t float64) {
			traced = append(traced, pt)
			ts = append(ts, t)
		})
		if len(plain) != len(traced) {
			t.Fatalf("quad %d sample count %d vs %d", i, len(plain), len(traced))
		}
		for j := range plain {
			if plain[j].Sub(traced[j]).Length() > 1e-12 {
				t.Fatalf("quad %d sample %d differs: %v vs %v", i, j, plain[j], traced[j])
			}
			if d := q.Eval(ts[j]).Sub(traced[j]).Length(); d > 1e-12 {
				t.Fatalf("quad %d parameter %v does not match its point (off %v)", i, ts[j], d)
			}
		}
		if len(ts) == 0 || ts[len(ts)-1] != 1 || !sortedAscending(ts) {
			t.Fatalf("quad %d parameters not ascending to 1: %v", i, ts)
		}
	}

	cubicCases := []CubicBez{
		NewCubicBez(Pt(0, 0), Pt(20, 60), Pt(100, -60), Pt(140, 20)),
		NewCubicBez(Pt(0, 0), Pt(1, 0.5), Pt(2, 0.5), Pt(3, 0)),
		NewCubicBez(Pt(10, 10), Pt(1000, 10), Pt(10, 900), Pt(40, 40)),
	}
	for i, c := range cubicCases {
		var plain, traced []Point
		var ts []float64
		flattenCubic(c.P0, c.P1, c.P2, c.P3, pathBooleanFlattenTolerance, func(pt Point) { plain = append(plain, pt) })
		flattenCubicT(c, pathBooleanFlattenTolerance, func(pt Point, t float64) {
			traced = append(traced, pt)
			ts = append(ts, t)
		})
		if len(plain) != len(traced) {
			t.Fatalf("cubic %d sample count %d vs %d", i, len(plain), len(traced))
		}
		for j := range plain {
			if plain[j].Sub(traced[j]).Length() > 1e-12 {
				t.Fatalf("cubic %d sample %d differs", i, j)
			}
			if d := c.Eval(ts[j]).Sub(traced[j]).Length(); d > 1e-12 {
				t.Fatalf("cubic %d parameter %v does not match its point (off %v)", i, ts[j], d)
			}
		}
		if len(ts) == 0 || ts[len(ts)-1] != 1 || !sortedAscending(ts) {
			t.Fatalf("cubic %d parameters not ascending to 1: %v", i, ts)
		}
	}
}

// TestBooleanConicFlattenSamplesStayOnConic: the conic flattener is the one
// place batch 5 changes the sample placement (parent-parameter splits instead of
// quad degradation), so pin its accuracy class: monotone parameters, points on
// the exact conic, deviation from the chord within tolerance.
func TestBooleanConicFlattenSamplesStayOnConic(t *testing.T) {
	for _, w := range []float64{0.3, math.Sqrt2 / 2, 0.9, 1.4} {
		c := NewConicBez(Pt(0, 0), Pt(60, -40), Pt(110, 25), w)
		var ts []float64
		var pts []Point
		flattenConicT(c, pathBooleanFlattenTolerance, func(pt Point, t float64) {
			pts = append(pts, pt)
			ts = append(ts, t)
		})
		if len(pts) < 2 || ts[len(ts)-1] != 1 || !sortedAscending(ts) {
			t.Fatalf("w=%v conic samples bad: n=%d ts=%v", w, len(pts), ts)
		}
		for j, p := range pts {
			if d := c.Eval(ts[j]).Sub(p).Length(); d > 1e-9 {
				t.Fatalf("w=%v sample %v off the conic by %v", w, j, d)
			}
		}
		// Curve-to-chord deviation must stay inside the tolerance class the
		// untraced flattener promises (<= 2*tolerance for a degraded conic).
		worst := 0.0
		for j := 1; j < len(pts); j++ {
			a, b := pts[j-1], pts[j]
			for k := 1; k < 16; k++ {
				tt := ts[j-1] + (ts[j]-ts[j-1])*float64(k)/16
				if d := pointSegDist(c.Eval(tt), a, b); d > worst {
					worst = d
				}
			}
		}
		if worst > 2*pathBooleanFlattenTolerance {
			t.Fatalf("w=%v polyline deviates %v beyond tolerance", w, worst)
		}
	}
}

// TestBooleanTraceRecordsRunsAndGrid checks the sample bookkeeping the refit
// walk depends on: one recorded sample per distinct grid vertex, lookups keyed
// by that vertex, deduped coincident samples dropped.
func TestBooleanTraceRecordsRunsAndGrid(t *testing.T) {
	p := NewPath()
	p.MoveTo(0, 0)
	p.LineTo(40, 0)
	p.QuadraticTo(60, 20, 40, 40)
	p.CubicTo(20, 60, -20, 40, -10, 10)
	p.Close()

	paths, tr := pathToClipperPathsTraced(p)
	if len(paths) != 1 {
		t.Fatalf("expected 1 contour, got %d", len(paths))
	}
	if len(tr.runs) != 2 {
		t.Fatalf("expected 2 curve runs (quad+cubic), got %d", len(tr.runs))
	}
	total := 0
	for _, r := range tr.runs {
		if len(r.ts) != len(r.gp) {
			t.Fatalf("run sample/parameter misalignment: %d vs %d", len(r.ts), len(r.gp))
		}
		if r.ts[len(r.ts)-1] != 1 {
			t.Fatalf("run parameter list does not end at 1: %v", r.ts)
		}
		for i := 1; i < len(r.gp); i++ {
			if r.gp[i] == r.gp[i-1] {
				t.Fatal("two samples recorded on one grid vertex")
			}
		}
		total += len(r.gp)
	}
	// Every recorded sample must be findable by its grid vertex, and the polygon
	// must contain it exactly once.
	for ri, r := range tr.runs {
		for si, gp := range r.gp {
			hits := tr.lookup(&clipper.IntPoint{X: gp.x, Y: gp.y})
			own := 0
			for _, h := range hits {
				if h.run == ri && h.sample == si {
					own++
				}
			}
			if own != 1 {
				t.Fatalf("sample (%d,%d) found %d times", ri, si, own)
			}
		}
	}
	// Straight vertices stay untraced: the move point, the line point and the
	// closing edge back to the start — three. Curve samples that share a vertex
	// (a run's t=0 start on the previous segment's end) do not add one.
	distinct := map[boolGP]bool{}
	for _, r := range tr.runs {
		for _, gp := range r.gp {
			distinct[gp] = true
		}
	}
	inPoly := map[boolGP]int{}
	for _, v := range paths[0] {
		inPoly[boolGP{x: v.X, y: v.Y}]++
	}
	for gp := range distinct {
		if inPoly[gp] == 0 {
			t.Fatalf("recorded sample %v is not a polygon vertex", gp)
		}
	}
	straightOnly := 0
	for _, v := range paths[0] {
		if !distinct[boolGP{x: v.X, y: v.Y}] {
			straightOnly++
		}
	}
	if straightOnly != 2 { // the move point and the closing edge back to it
		t.Fatalf("straight-only vertices = %d, want 2", straightOnly)
	}
	// The t=0 start of each run after the first must be recorded on the shared
	// vertex: that is what lets an untouched contour come back seam-for-seam.
	for ri, r := range tr.runs {
		if r.ts[0] != 0 {
			t.Fatalf("run %d has no t=0 start sample (ts[0]=%v)", ri, r.ts[0])
		}
	}
}

// pointSegDist is the distance from p to segment [a,b].
func pointSegDist(p, a, b Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	if dx == 0 && dy == 0 {
		return math.Hypot(p.X-a.X, p.Y-a.Y)
	}
	tt := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / (dx*dx + dy*dy)
	if tt < 0 {
		tt = 0
	} else if tt > 1 {
		tt = 1
	}
	return math.Hypot(p.X-(a.X+tt*dx), p.Y-(a.Y+tt*dy))
}

func sortedAscending(ts []float64) bool {
	for i := 1; i < len(ts); i++ {
		if ts[i] <= ts[i-1] {
			return false
		}
	}
	return true
}
