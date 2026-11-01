//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package render

import (
	"image"
	"math"
	"testing"

	"github.com/energye/gpui/render/internal/clip"
)

// conicTestRGB samples the flushed context image (Image() flushes pending GPU
// draws and syncs them into the pixmap). Reading dc.pixmap directly would make
// these tests depend on whether a GPU accelerator happens to be registered.
func conicTestRGB(img image.Image, x, y int) (r, g, b float64) {
	r16, g16, b16, _ := img.At(x, y).RGBA()
	return float64(r16) / 65535, float64(g16) / 65535, float64(b16) / 65535
}

// fillQuarterDiskConicPath builds the quarter disk bounded by a conic arc:
// center at origin-ish (cx,cy), radius r, arc from (cx+r,cy) to (cx,cy-r)
// through the conic control (cx+r,cy-r) with weight sqrt(2)/2.
func fillQuarterDiskConicPath(cx, cy, r float64) *Path {
	p := NewPath()
	p.MoveTo(cx+r, cy)
	p.ConicTo(cx+r, cy-r, cx, cy-r, conicQuarterWeight)
	p.LineTo(cx, cy)
	p.Close()
	return p
}

func TestContextFillConicQuarterDisk(t *testing.T) {
	const size = 160
	dc := NewContext(size, size)
	defer func() { _ = dc.Close() }()

	dc.ClearWithColor(White)
	dc.SetRGB(1, 0, 0)
	dc.DrawPath(fillQuarterDiskConicPath(10, size-10, 100))
	dc.Fill()

	// Sample the bounding box of the quarter disk and compare the filled
	// fraction against the analytic area ratio.
	img := dc.Image()
	r := 100.0
	minX, minY := 10.0, float64(size)-10.0-r
	inside, total := 0, 0
	// Sample exactly the quarter-disk bounding box: x∈[cx,cx+r], y∈[cy−r,cy].
	for sy := minY + 0.5; sy < minY+r; sy += 1.0 {
		for sx := minX + 0.5; sx < minX+r; sx += 1.0 {
			total++
			pxR, pxG, _ := conicTestRGB(img, int(sx), int(sy))
			// Red fill (white background has R=1 too — require green low).
			if pxR > 0.5 && pxG < 0.5 {
				inside++
			}
		}
	}
	// Quarter disk area / bbox area = (π/4 r²) / r² = π/4.
	got := float64(inside) / float64(total)
	if math.Abs(got-math.Pi/4) > 0.02 {
		t.Errorf("filled fraction = %v, want π/4 = %v", got, math.Pi/4)
	}

	// Edge pins: a point just inside the arc must be filled, just outside white.
	// Arc point at 45°: (cx + r·cos45°, cy − r·sin45°) with cx=10, cy=size−10.
	cx, cy := 10.0, float64(size)-10.0
	c45, s45 := math.Cos(math.Pi/4), math.Sin(math.Pi/4)
	inX, inY := int(cx+(r-3)*c45), int(cy-(r-3)*s45)
	if pxR, pxG, _ := conicTestRGB(img, inX, inY); !(pxR > 0.9 && pxG < 0.1) {
		t.Errorf("inside-arc pixel (%d,%d) = r=%v g=%v, want filled red", inX, inY, pxR, pxG)
	}
	outX, outY := int(cx+(r+3)*c45), int(cy-(r+3)*s45)
	if outY >= 0 {
		if _, pxG, _ := conicTestRGB(img, outX, outY); pxG < 0.9 {
			t.Errorf("outside-arc pixel (%d,%d) g=%v, want white", outX, outY, pxG)
		}
	}
}

func TestContextClipConicPath(t *testing.T) {
	const size = 160
	dc := NewContext(size, size)
	defer func() { _ = dc.Close() }()

	dc.ClearWithColor(White)
	dc.DrawPath(fillQuarterDiskConicPath(10, size-10, 100))
	dc.Clip()
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(0, 0, size, size)
	dc.Fill()

	img := dc.Image()
	cx, cy := 10.0, float64(size)-10.0
	c45, s45 := math.Cos(math.Pi/4), math.Sin(math.Pi/4)
	inX, inY := int(cx+(100-3)*c45), int(cy-(100-3)*s45)
	if pxR, pxG, pxB := conicTestRGB(img, inX, inY); !(pxB > 0.9 && pxG < 0.1) {
		t.Errorf("inside-clip pixel (%d,%d) = r=%v g=%v b=%v, want blue", inX, inY, pxR, pxG, pxB)
	}
	outX, outY := int(cx+(100+3)*c45), int(cy-(100+3)*s45)
	if outY >= 0 {
		if _, pxG, _ := conicTestRGB(img, outX, outY); pxG < 0.9 {
			t.Errorf("outside-clip pixel (%d,%d) g=%v, want white", outX, outY, pxG)
		}
	}
}

func TestContextStrokeConicArc(t *testing.T) {
	const size = 160
	dc := NewContext(size, size)
	defer func() { _ = dc.Close() }()

	dc.ClearWithColor(White)
	dc.SetRGB(0, 0, 1)
	dc.SetLineWidth(4)
	// Quarter circle centered (10,150) r=120: start (130,150), end (10,30),
	// control at the tangent intersection (130,30).
	p := NewPath()
	p.MoveTo(130, 150)
	p.ConicTo(130, 30, 10, 30, conicQuarterWeight)
	dc.DrawPath(p)
	dc.Stroke()

	// A point on the arc at 45°: center + r·(cos45°, −sin45°) with y flipped
	// (arc sweeps upward from angle 0).
	img := dc.Image()
	mx, my := 10+120*c45s(), 150-120*c45s()
	found := false
	for dy := -3; dy <= 3 && !found; dy++ {
		for dx := -3; dx <= 3 && !found; dx++ {
			x, y := int(mx)+dx, int(my)+dy
			if x < 0 || y < 0 || x >= size || y >= size {
				continue
			}
			if pxR, _, pxB := conicTestRGB(img, x, y); pxB > 0.5 && pxR < 0.5 {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("no stroked pixel found near mid-arc (%v,%v)", mx, my)
	}
	// Far from the arc must stay white (white has B=1 — require R high).
	if pxR, _, _ := conicTestRGB(img, 5, 155); pxR < 0.9 {
		t.Errorf("far pixel r=%v, want white", pxR)
	}
}

func c45s() float64 { return math.Cos(math.Pi / 4) }

func TestPathWindingConic(t *testing.T) {
	p := fillQuarterDiskConicPath(10, 150, 100)
	c45 := math.Cos(math.Pi / 4)
	inside := Pt(10+50*c45, 150-50*c45)
	if p.Winding(inside) == 0 {
		t.Errorf("Winding(%v) = 0, want non-zero inside the conic quarter disk", inside)
	}
	outside := Pt(10+110*c45, 150-110*c45)
	if p.Winding(outside) != 0 {
		t.Errorf("Winding(%v) = %d, want 0 outside the conic quarter disk", outside, p.Winding(outside))
	}
}

func TestComputeMetricsConic(t *testing.T) {
	r := 80.0
	p := NewPath()
	p.MoveTo(r, 0)
	p.ConicTo(r, r, 0, r, conicQuarterWeight)
	p.Close()

	metrics := p.ComputeMetrics(0.01)
	if len(metrics) != 1 {
		t.Fatalf("metrics = %d contours, want 1", len(metrics))
	}
	// This quarter disk is bounded by the arc from (r,0) to (0,r) and the
	// closing chord between those two points: arc + chord, not two radii.
	want := math.Pi*r/2 + r*math.Sqrt2
	if got := metrics[0].Length(); math.Abs(got-want) > want*0.002 {
		t.Errorf("conic contour length = %v, want %v", got, want)
	}
}

func TestSoftwareRendererFillConic(t *testing.T) {
	r := NewSoftwareRenderer(160, 160)
	pm := NewPixmap(160, 160)
	pm.Clear(White)

	paint := NewPaint()
	paint.SetBrush(Solid(Red))
	if err := r.Fill(pm, fillQuarterDiskConicPath(10, 150, 100), paint); err != nil {
		t.Fatalf("Fill returned error: %v", err)
	}

	// 45° point at radius 97 is inside the quarter disk; radius 103 is outside.
	c45 := math.Cos(math.Pi / 4)
	in := pm.GetPixel(int(10+97*c45), int(150-97*c45))
	if in.R < 0.9 || in.G > 0.1 {
		t.Errorf("inside conic fill = %+v, want red", in)
	}
	out := pm.GetPixel(int(10+103*c45), int(150-103*c45))
	if out.G < 0.9 {
		t.Errorf("outside conic fill = %+v, want white", out)
	}
}

func TestSoftwareRendererStrokeConic(t *testing.T) {
	r := NewSoftwareRenderer(160, 160)
	pm := NewPixmap(160, 160)
	pm.Clear(White)

	paint := NewPaint()
	paint.SetBrush(Solid(Blue))
	paint.LineWidth = 4

	p := NewPath()
	p.MoveTo(130, 150)
	p.ConicTo(130, 30, 10, 30, conicQuarterWeight)
	if err := r.Stroke(pm, p, paint); err != nil {
		t.Fatalf("Stroke returned error: %v", err)
	}

	// 45° point on the r=120 arc must be painted; the center must stay white.
	c45 := math.Cos(math.Pi / 4)
	onArc := pm.GetPixel(int(10+120*c45), int(150-120*c45))
	if onArc.B < 0.5 {
		t.Errorf("on-arc conic stroke = %+v, want blue", onArc)
	}
	center := pm.GetPixel(10, 150)
	if center.G < 0.9 {
		t.Errorf("center of conic stroke = %+v, want white", center)
	}
}

func TestConvertPathToClipVerbsConic(t *testing.T) {
	verbs, _ := convertPathToClipVerbs(fillQuarterDiskConicPath(10, 150, 100))
	if len(verbs) < 2 {
		t.Fatalf("clip verbs = %d, want >=2", len(verbs))
	}
	for i, v := range verbs {
		// The clip package only has MoveTo..Close (0..4). A raw conic cast
		// would surface as 5 here and silently desync the mask rasterizer.
		if int(v) > int(clip.VerbClose) {
			t.Fatalf("clip verb[%d] = %d, want <=%d (conic not reduced)", i, v, clip.VerbClose)
		}
	}
}

func TestDashPathConicEmitsLinesOnly(t *testing.T) {
	p := NewPath()
	p.MoveTo(0, 0)
	p.ConicTo(100, 0, 100, 100, conicQuarterWeight)
	p.Close()
	dashed := dashPath(p, NewDash(6, 4))
	if dashed == p {
		t.Fatal("dashPath should return a new path for conic input")
	}
	dashed.Iterate(func(verb PathVerb, coords []float64) {
		if verb == ConicTo {
			t.Errorf("dashed path contains ConicTo")
		}
	})
	if dashed.NumVerbs() < 4 {
		t.Errorf("dashed path too short: %d verbs", dashed.NumVerbs())
	}
}

func TestPathLengthConicQuarterCircle(t *testing.T) {
	r := 80.0
	p := NewPath()
	p.MoveTo(r, 0)
	p.ConicTo(r, r, 0, r, conicQuarterWeight)
	got := p.Length(0.01)
	want := math.Pi * r / 2
	if math.Abs(got-want) > want*0.001 {
		t.Errorf("conic arc length = %v, want %v", got, want)
	}
}

func TestPathBoundingBoxConicArc(t *testing.T) {
	// Quarter arc from (r,0) to (0,r): tight bbox touches [0,r] on all sides.
	r := 40.0
	p := NewPath()
	p.MoveTo(r, 0)
	p.ConicTo(r, r, 0, r, conicQuarterWeight)
	b := p.BoundingBox()
	if math.Abs(b.Min.X-0) > 0.01 || math.Abs(b.Min.Y-0) > 0.01 ||
		math.Abs(float64(b.Max.X)-r) > 0.01 || math.Abs(float64(b.Max.Y)-r) > 0.01 {
		t.Errorf("conic arc bbox = %v, want [0,%v]×[0,%v]", b, r, r)
	}
}

func TestPathTrimConicStaysOnArc(t *testing.T) {
	r := 60.0
	p := NewPath()
	p.MoveTo(r, 0)
	p.ConicTo(r, r, 0, r, conicQuarterWeight)
	mid := p.Trim(0.4, 0.6)
	pts := mid.Flatten(0.01)
	if len(pts) < 2 {
		t.Fatal("trimmed path too short")
	}
	for _, pt := range pts {
		d := math.Hypot(pt.X, pt.Y)
		// Trim flattens internally at 0.25px, so 0.3 is the honest bound.
		if math.Abs(d-r) > 0.3 {
			t.Errorf("trimmed point (%v,%v) radius %v, want %v ±0.3", pt.X, pt.Y, d, r)
		}
	}
}

func TestPathConicReversal(t *testing.T) {
	p := NewPath()
	p.MoveTo(100, 0)
	p.ConicTo(100, 100, 0, 100, conicQuarterWeight)
	rev := p.Reversed()
	// Reversed path: same point set, opposite direction. Endpoints swapped.
	revPts := rev.Flatten(0.01)
	origPts := p.Flatten(0.01)
	if len(revPts) == 0 || len(origPts) == 0 {
		t.Fatal("flatten produced no points")
	}
	// First point of reversed ≈ last point of original.
	if math.Hypot(revPts[0].X-origPts[len(origPts)-1].X, revPts[0].Y-origPts[len(origPts)-1].Y) > 0.01 {
		t.Errorf("reversed start = (%v,%v), want (%v,%v)",
			revPts[0].X, revPts[0].Y, origPts[len(origPts)-1].X, origPts[len(origPts)-1].Y)
	}
	// The reversed curve must stay on the same circle.
	for _, pt := range revPts {
		if d := math.Hypot(pt.X, pt.Y); math.Abs(d-100) > 0.05 {
			t.Errorf("reversed point off arc: radius %v", d)
		}
	}
}

func TestExpandConicsToQuadsPreservesGeometry(t *testing.T) {
	p := NewPath()
	p.MoveTo(100, 0)
	p.ConicTo(100, 100, 0, 100, conicQuarterWeight)
	p.LineTo(0, 0)
	p.Close()
	if !p.HasConics() {
		t.Fatal("path should contain conics")
	}
	// Conic-free paths must pass through unchanged (same pointer).
	clean := NewPath()
	clean.MoveTo(0, 0)
	clean.LineTo(1, 1)
	if clean.ExpandConicsToQuads(0.1) != clean {
		t.Error("conic-free path must pass through unchanged")
	}

	q := p.ExpandConicsToQuads(0.05)
	if q.HasConics() {
		t.Error("expanded path still contains conics")
	}
	// Expanded quads must stay on the analytic arc.
	cur := Point{}
	q.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo, LineTo:
			cur = Pt(coords[0], coords[1])
		case QuadTo:
			qb := NewQuadBez(cur, Pt(coords[0], coords[1]), Pt(coords[2], coords[3]))
			for _, tt := range []float64{0.25, 0.5, 0.75} {
				pt := qb.Eval(tt)
				if d := math.Hypot(pt.X, pt.Y); math.Abs(d-100) > 0.11 {
					t.Errorf("expanded quad point (%v,%v) radius %v, want 100±0.11", pt.X, pt.Y, d)
				}
			}
			cur = qb.P2
		case Close:
		default:
			t.Fatalf("unexpected verb %v", verb)
		}
	})
	// Area equivalence: the expanded path encloses the same region.
	if math.Abs(p.Area()-q.Area()) > math.Max(1, 0.002*p.Area()) {
		t.Errorf("expanded area %v vs conic area %v", q.Area(), p.Area())
	}
}
