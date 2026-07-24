package render

import (
	"math"
	"testing"
)

// TestStrokeJoin_OpenPolylineSolid is a regression for icon check / chevron elbows:
// expand+EvenOdd punched white holes at inner-pivot V-joins of open polylines.
// Stroke outlines must fill with NonZero so the join ink stays solid.
func TestStrokeJoin_OpenPolylineSolid(t *testing.T) {
	// PaintLocalCheck proportions at 64px, width 4 (same as UI icons scaled up).
	const w, h, lw = 64.0, 64.0, 4.0
	dc := NewContext(int(w), int(h))
	dc.ClearWithColor(White)
	dc.SetAntiAlias(true)
	dc.SetRGBA(0, 0, 0, 1)
	dc.SetStroke(DefaultStroke().WithWidth(lw).WithCap(LineCapRound).WithJoin(LineJoinRound))
	dc.MoveTo(w*0.22, h*0.50)
	dc.LineTo(w*0.42, h*0.70)
	dc.LineTo(w*0.78, h*0.30)
	if err := dc.Stroke(); err != nil {
		t.Fatal(err)
	}

	jx, jy := int(math.Round(w*0.42)), int(math.Round(h*0.70))
	// Sample only inside the stroke disk (half-width + AA fringe).
	rMax := int(math.Ceil(lw/2 + 1.5))
	dark, total := 0, 0
	for dy := -rMax; dy <= rMax; dy++ {
		for dx := -rMax; dx <= rMax; dx++ {
			if dx*dx+dy*dy > rMax*rMax {
				continue
			}
			total++
			rr, gg, bb, _ := dc.Image().At(jx+dx, jy+dy).RGBA()
			if (rr+gg+bb)/3 < 0x9000 {
				dark++
			}
		}
	}
	if total == 0 || float64(dark)/float64(total) < 0.45 {
		t.Fatalf("open polyline join too sparse: dark=%d/%d — EvenOdd-style hole?", dark, total)
	}
	// Exact join center must not be a pure white hole.
	r, g, b, _ := dc.Image().At(jx, jy).RGBA()
	if (r+g+b)/3 > 0xC000 {
		t.Fatalf("join center is white hole: RGB=%d,%d,%d", r>>8, g>>8, b>>8)
	}
}

// TestStrokeJoin_ClosedRingHollow ensures NonZero still leaves the interior of a
// closed stroke ring empty (reverse inner contour winding).
func TestStrokeJoin_ClosedRingHollow(t *testing.T) {
	dc := NewContext(100, 100)
	dc.ClearWithColor(White)
	dc.SetAntiAlias(false)
	dc.SetRGBA(0, 0, 0, 1)
	dc.SetStroke(DefaultStroke().WithWidth(8).WithCap(LineCapButt).WithJoin(LineJoinMiter))
	dc.MoveTo(20, 20)
	dc.LineTo(80, 20)
	dc.LineTo(80, 80)
	dc.LineTo(20, 80)
	dc.ClosePath()
	if err := dc.Stroke(); err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := dc.Image().At(50, 50).RGBA()
	if (r+g+b)/3 < 0xC000 {
		t.Fatalf("closed stroke ring center should be hollow/white, got RGB=%d,%d,%d", r>>8, g>>8, b>>8)
	}
	// Top edge mid should be inked.
	edgeDark := 0
	for y := 16; y <= 24; y++ {
		rr, gg, bb, _ := dc.Image().At(50, y).RGBA()
		if (rr+gg+bb)/3 < 0x8000 {
			edgeDark++
		}
	}
	if edgeDark < 4 {
		t.Fatalf("closed stroke ring edge not inked: edgeDark=%d", edgeDark)
	}
}

// TestStrokeJoin_CheckMarkUI mirrors ui/core.PaintLocalCheck proportions.
func TestStrokeJoin_CheckMarkUI(t *testing.T) {
	const size = 32.0
	lw := size * 0.125
	if lw < 1.6 {
		lw = 1.6
	}
	if lw > 2.5 {
		lw = 2.5
	}
	dc := NewContext(int(size)+8, int(size)+8)
	dc.ClearWithColor(White)
	dc.SetAntiAlias(true)
	dc.SetRGBA(0, 0, 0, 1)
	dc.SetStroke(DefaultStroke().WithWidth(lw).WithCap(LineCapRound).WithJoin(LineJoinRound))
	// Same points as PaintLocalCheck
	ox, oy := 4.0, 4.0
	dc.MoveTo(ox+size*0.22, oy+size*0.50)
	dc.LineTo(ox+size*0.42, oy+size*0.70)
	dc.LineTo(ox+size*0.78, oy+size*0.30)
	_ = dc.Stroke()

	jx := int(math.Round(ox + size*0.42))
	jy := int(math.Round(oy + size*0.70))
	r := int(math.Ceil(lw))
	dark := 0
	total := 0
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy > r*r {
				continue
			}
			total++
			rr, gg, bb, _ := dc.Image().At(jx+dx, jy+dy).RGBA()
			if (rr+gg+bb)/3 < 0x9000 {
				dark++
			}
		}
	}
	if total == 0 || float64(dark)/float64(total) < 0.45 {
		t.Fatalf("UI check elbow coverage dark=%d/%d — join hole regression?", dark, total)
	}
}
