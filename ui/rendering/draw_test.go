package rendering_test

import (
	"image/color"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

func sampleAt(img interface{ At(x, y int) color.Color }, x, y int) (r, g, b uint32) {
	rr, gg, bb, _ := img.At(x, y).RGBA()
	return rr, gg, bb
}

func TestDraw_FillRect_OriginAware(t *testing.T) {
	dc := render.NewContext(80, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1).WithOrigin(10, 10)
	rendering.FillRect(pc, 5, 5, 20, 20, 1, 0, 0, 1)

	img := dc.Image()
	// local (5,5) → abs (15,15)
	r, g, b := sampleAt(img, 15, 15)
	if r < 0xC000 || g > 0x4000 || b > 0x4000 {
		t.Fatalf("fill center #%04x%04x%04x want red", r, g, b)
	}
	// outside
	r, g, b = sampleAt(img, 2, 2)
	if r < 0xC000 || g < 0xC000 || b < 0xC000 {
		t.Fatalf("outside #%04x%04x%04x want white", r, g, b)
	}
}

func TestDraw_StrokeRect_NotFullyFilled(t *testing.T) {
	dc := render.NewContext(100, 100)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1)
	rendering.StrokeRect(pc, 20, 20, 40, 40, 3, 0, 0, 1, 1)

	img := dc.Image()
	// Interior should stay near-white (stroke only).
	r, g, b := sampleAt(img, 40, 40)
	if r < 0xA000 || g < 0xA000 || b < 0xA000 {
		t.Fatalf("interior #%04x%04x%04x want near-white (stroke, not fill)", r, g, b)
	}
	// Edge pixel should pick up blue stroke (allow AA).
	er, eg, eb := sampleAt(img, 20, 40)
	if eb < 0x4000 && er > 0xE000 && eg > 0xE000 {
		t.Fatalf("edge #%04x%04x%04x want some blue from stroke", er, eg, eb)
	}
}

func TestDraw_FillCircle_CenterRed(t *testing.T) {
	dc := render.NewContext(80, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1)
	rendering.FillCircle(pc, 40, 40, 15, 1, 0, 0, 1)

	img := dc.Image()
	r, g, b := sampleAt(img, 40, 40)
	if r < 0xC000 || g > 0x4000 || b > 0x4000 {
		t.Fatalf("circle center #%04x%04x%04x want red", r, g, b)
	}
	// Far corner white
	r, g, b = sampleAt(img, 2, 2)
	if r < 0xC000 || g < 0xC000 || b < 0xC000 {
		t.Fatalf("corner #%04x%04x%04x want white", r, g, b)
	}
}

func TestDraw_FillOval_And_StrokeArc(t *testing.T) {
	dc := render.NewContext(120, 120)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1)
	rendering.FillOval(pc, 20, 40, 80, 40, 0, 0.6, 0, 1)
	rendering.StrokeArc(pc, 60, 60, 30, -math.Pi/2, math.Pi/2, 3, 0, 0, 1, 1)

	img := dc.Image()
	r, g, b := sampleAt(img, 60, 60)
	// Oval center should be green-ish
	if g < 0x6000 {
		t.Fatalf("oval center #%04x%04x%04x want green component", r, g, b)
	}
}

func TestDraw_FillPath_Triangle(t *testing.T) {
	dc := render.NewContext(100, 100)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1).WithOrigin(10, 10)
	p := rendering.NewPath()
	p.MoveTo(40, 5)
	p.LineTo(70, 60)
	p.LineTo(10, 60)
	p.Close()
	rendering.FillPath(pc, p, 0, 0, 1, 1)

	img := dc.Image()
	// centroid roughly abs (10+40, 10+40) = (50,50)
	r, g, b := sampleAt(img, 50, 45)
	if b < 0x8000 {
		t.Fatalf("path fill #%04x%04x%04x want blue-ish", r, g, b)
	}
}

func TestDraw_FillLinearGradient_Runs(t *testing.T) {
	dc := render.NewContext(64, 32)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1)
	rendering.FillLinearGradient(pc, 0, 0, 64, 32, 0, 0, 64, 0,
		1, 0, 0, 1,
		0, 0, 1, 1,
	)
	img := dc.Image()
	// Left more red, right more blue (allow AA/interp).
	lr, _, lb := sampleAt(img, 4, 16)
	rr, _, rb := sampleAt(img, 60, 16)
	if lr <= lb {
		t.Fatalf("left should be redder than blue: R=%04x B=%04x", lr, lb)
	}
	if rb <= rr {
		t.Fatalf("right should be bluer than red: R=%04x B=%04x", rr, rb)
	}
}

func TestDraw_FillRadialAndSweep_NoPanic(t *testing.T) {
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1)
	rendering.FillRadialGradient(pc, 0, 0, 64, 64, 32, 32, 2, 30,
		1, 1, 0, 1, 0.1, 0.1, 0.2, 1)
	rendering.FillSweepGradient(pc, 0, 0, 64, 64, 32, 32, 0,
		1, 0, 0, 1, 0, 0, 1, 1)
	if dc.Image() == nil {
		t.Fatal("nil image")
	}
}

func TestDraw_SetStrokeStyle_RoundCap(t *testing.T) {
	dc := render.NewContext(40, 40)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1)
	rendering.SetStrokeStyle(pc, 4, render.LineCapRound, render.LineJoinRound)
	rendering.StrokeLine(pc, 5, 20, 35, 20, 4, 0, 0, 0, 1)
	if dc.Image() == nil {
		t.Fatal("nil image")
	}
}

// TestDraw_DrawPoints_OriginAware: points land at Abs positions.
func TestDraw_DrawPoints_OriginAware(t *testing.T) {
	dc := render.NewContext(60, 60)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1).WithOrigin(10, 10)
	// Local (20,20) → abs (30,30)
	rendering.DrawPoints(pc, []float64{20, 20}, 4, 1, 0, 0, 1)
	img := dc.Image()
	r, g, b := sampleAt(img, 30, 30)
	if r < 0xC000 || g > 0x4000 {
		t.Fatalf("point (30,30)=#%04x%04x%04x want red", r, g, b)
	}
	or, og, ob := sampleAt(img, 5, 5)
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("far (5,5) want white got #%04x%04x%04x", or, og, ob)
	}
}

// TestDraw_DrawVertices_Triangle fills a solid-color triangle.
func TestDraw_DrawVertices_Triangle(t *testing.T) {
	dc := render.NewContext(80, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1).WithOrigin(5, 5)
	// Local triangle covering center; solid blue via DC fill color path (no per-vert colors).
	pos := []rendering.Point{
		{X: 10, Y: 10},
		{X: 60, Y: 10},
		{X: 35, Y: 55},
	}
	pc.DC.SetRGBA(0, 0, 1, 1)
	rendering.DrawVertices(pc, pos, nil, rendering.VertexModeTriangles)
	img := dc.Image()
	// Centroid-ish abs ~(5+35, 5+25)=(40,30)
	r, g, b := sampleAt(img, 40, 30)
	if b < 0x8000 {
		t.Fatalf("triangle interior #%04x%04x%04x want blue-ish", r, g, b)
	}
}

// TestDraw_FillDRRect_RingHasHole: outer red ring, inner hole stays near-white.
func TestDraw_FillDRRect_RingHasHole(t *testing.T) {
	dc := render.NewContext(100, 100)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1)
	rendering.FillDRRect(pc,
		10, 10, 80, 80, 8,
		30, 30, 40, 40, 4,
		1, 0, 0, 1,
	)
	img := dc.Image()
	// Outer band (15,50) should be red.
	rr, rg, rb := sampleAt(img, 15, 50)
	if rr < 0xA000 {
		t.Fatalf("outer ring (15,50)=#%04x%04x%04x want red", rr, rg, rb)
	}
	// Hole center (50,50) near-white.
	hr, hg, hb := sampleAt(img, 50, 50)
	if hr < 0xA000 || hg < 0xA000 || hb < 0xA000 {
		t.Fatalf("DRRect hole (50,50)=#%04x%04x%04x want near-white", hr, hg, hb)
	}
}

// TestDraw_PathAddHelpers_BuildAndFill: PathAddRect/RRect produce fillable geometry.
func TestDraw_PathAddHelpers_BuildAndFill(t *testing.T) {
	dc := render.NewContext(60, 60)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1)
	p := rendering.NewPath()
	rendering.PathAddRect(p, 10, 10, 20, 20)
	rendering.FillPath(pc, p, 0, 0.7, 0, 1)
	img := dc.Image()
	r, g, b := sampleAt(img, 20, 20)
	if g < 0x8000 {
		t.Fatalf("PathAddRect fill #%04x%04x%04x want green", r, g, b)
	}
}
