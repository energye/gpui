package rendering_test

import (
	"image"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

func TestSnapCoord_DeviceGrid(t *testing.T) {
	cases := []struct {
		v, scale, want float64
	}{
		{0.3, 1, 0},
		{0.6, 1, 1},
		{0.25, 2, 0.5},   // 0.25*2=0.5 → round=0? no: math.Round(0.5)=1 → 0.5
		{1.37, 2, 1.5},   // 2.74 → 3 → 1.5
		{0.1, 3, 0},      // 0.3 → 0
		{2.6, 3, 2.6667}, // 7.8 → 8 → 2.6667
	}
	for _, c := range cases {
		got := rendering.SnapCoord(c.v, c.scale)
		if math.Abs(got-c.want) > 0.0001 {
			t.Fatalf("SnapCoord(%v, %v)=%v want %v", c.v, c.scale, got, c.want)
		}
	}
	// scale <= 0 → identity DPR.
	if got := rendering.SnapCoord(0.5, 0); got != 1 {
		t.Fatalf("SnapCoord(0.5,0)=%v want 1 (default DPR 1)", got)
	}
}

func TestSnapLine_HairlineCenter(t *testing.T) {
	// Snapped 1px stroke centers sit on physical pixel centers (n+0.5), not
	// boundaries: DPR 2, y=10.4 → floor(20.8)+0.5=20.5 physical = 10.25 logical.
	got := rendering.SnapLine(10.4, 2)
	if math.Abs(got-10.25) > 0.0001 {
		t.Fatalf("SnapLine(10.4,2)=%v want 10.25", got)
	}
	// DPR 1: y=3.6 → floor(3.6)+0.5=3.5 logical.
	got = rendering.SnapLine(3.6, 1)
	if math.Abs(got-3.5) > 0.0001 {
		t.Fatalf("SnapLine(3.6,1)=%v want 3.5", got)
	}
	// Contract: SnapLine(v,s)*s is always half-integer; SnapCoord(v,s)*s is integral.
	for _, scale := range []float64{1, 1.5, 2} {
		for _, v := range []float64{0, 0.25, 1.3, 2.6, 10.4, 20.5, 99.9} {
			lineDev := rendering.SnapLine(v, scale) * scale
			if frac := lineDev - math.Floor(lineDev); math.Abs(frac-0.5) > 1e-9 {
				t.Fatalf("SnapLine(%v,%v)*scale=%v fractional=%v want 0.5", v, scale, lineDev, frac)
			}
			coordDev := rendering.SnapCoord(v, scale) * scale
			if math.Abs(coordDev-math.Round(coordDev)) > 1e-9 {
				t.Fatalf("SnapCoord(%v,%v)*scale=%v not integral", v, scale, coordDev)
			}
		}
	}
}

// TestSnapLine_OneDevicePixelCoverage locks the actual raster result: a 1px
// stroke centered with SnapLine must cover exactly one device row, not two
// half-covered rows. This is the regression for the old boundary-center bug.
func TestSnapLine_OneDevicePixelCoverage(t *testing.T) {
	for _, scale := range []float64{1, 2} {
		dc := render.NewContext(40, 24, render.WithDeviceScale(scale))
		dc.ClearWithColor(render.White)
		dc.SetRGB(0.3, 0.9, 0.6)
		dc.SetLineWidth(1 / scale)
		y := rendering.SnapLine(10.4, scale)
		dc.MoveTo(2, y)
		dc.LineTo(38, y)
		if err := dc.Stroke(); err != nil {
			t.Fatalf("scale %v: Stroke: %v", scale, err)
		}
		img := dc.Image()
		n := int(math.Floor(10.4 * scale))
		r, g, b, _ := rgba8(img, 20, n)
		if r > 80 || g < 226 || g > 232 || b < 150 || b > 156 {
			t.Fatalf("scale %v: center row %d rgba=%d,%d,%d want line 76,229,153", scale, n, r, g, b)
		}
		for _, row := range []int{n - 1, n + 1} {
			r, g, b, _ = rgba8(img, 20, row)
			if r < 252 || g < 252 || b < 252 {
				t.Fatalf("scale %v: adjacent row %d rgba=%d,%d,%d want background white (line split across pixels)", scale, row, r, g, b)
			}
		}
		_ = dc.Close()
	}
}

func rgba8(img image.Image, x, y int) (r, g, b, a uint8) {
	pr, pg, pb, pa := img.At(x, y).RGBA()
	return uint8(pr >> 8), uint8(pg >> 8), uint8(pb >> 8), uint8(pa >> 8)
}

func TestSnapRect_GridAlignedEdges(t *testing.T) {
	// DPR 2: rect edges must land on the device grid after snap.
	sx, sy, sw, sh := rendering.SnapRect(1.3, 2.6, 10.4, 5.2, 2)
	// x0=round(2.6)=3→1.5; x1=round(23.4)=23→11.5; sw=10
	if math.Abs(sx-1.5) > 0.0001 || math.Abs(sw-10) > 0.0001 {
		t.Fatalf("snapped rect x,w = %v,%v want 1.5,10", sx, sw)
	}
	// y0=round(5.2)=5→2.5; y1=round(15.6)=16→8; sh=5.5
	if math.Abs(sy-2.5) > 0.0001 || math.Abs(sh-5.5) > 0.0001 {
		t.Fatalf("snapped rect y,h = %v,%v want 2.5,5.5", sy, sh)
	}
}
