//go:build !nogpu

package gpu

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
)

// TestRasterizeTransformMaskRotatedRect verifies the kTransformedMask raster
// produces full ink for a simple rotated rectangle (the same geometry the
// render_text_transform cells use): a rotated 160x80 rect at 45° must fill a
// large fraction of its bbox.
func TestRasterizeTransformMaskRotatedRect(t *testing.T) {
	p := render.NewPath()
	cx, cy := 200.0, 150.0
	ang := math.Pi / 4
	var pts [][2]float64
	for _, sx := range []float64{-1, 1} {
		for _, sy := range []float64{-1, 1} {
			x := cx + sx*80*math.Cos(ang) - sy*40*math.Sin(ang)
			y := cy + sx*80*math.Sin(ang) + sy*40*math.Cos(ang)
			pts = append(pts, [2]float64{x, y})
		}
	}
	p.MoveTo(pts[0][0], pts[0][1])
	p.LineTo(pts[2][0], pts[2][1])
	p.LineTo(pts[3][0], pts[3][1])
	p.LineTo(pts[1][0], pts[1][1])
	p.Close()

	mask, bx, by, bw, bh, err := rasterizeTransformMask(p)
	if err != nil {
		t.Fatalf("rasterize: %v", err)
	}
	ink := 0
	for _, v := range mask {
		if v != 0 {
			ink++
		}
	}
	area := bw * bh
	t.Logf("bbox=(%d,%d) %dx%d ink=%d (%.0f%% of area; ~44%% expected for rotated 160x80)",
		bx, by, bw, bh, ink, 100*float64(ink)/float64(area))
	if ink < 40*area/100 {
		dumpMaskAscii(t, mask, bw, bh)
		t.Fatalf("ink too low: %d/%d", ink, area)
	}
}

// TestRasterizeTransformMaskAxisRect sanity-checks the rasterer on a plain
// axis-aligned rectangle (control for the rotated case).
func TestRasterizeTransformMaskAxisRect(t *testing.T) {
	p := render.NewPath()
	p.MoveTo(100, 100)
	p.LineTo(260, 100)
	p.LineTo(260, 180)
	p.LineTo(100, 180)
	p.Close()

	mask, bx, by, bw, bh, err := rasterizeTransformMask(p)
	if err != nil {
		t.Fatalf("rasterize: %v", err)
	}
	ink := 0
	for _, v := range mask {
		if v != 0 {
			ink++
		}
	}
	area := bw * bh
	t.Logf("axis bbox=(%d,%d) %dx%d ink=%d (%.0f%%)", bx, by, bw, bh, ink, 100*float64(ink)/float64(area))
	if ink < area/2 {
		dumpMaskAscii(t, mask, bw, bh)
		t.Fatalf("axis ink too low: %d/%d", ink, area)
	}
}

func dumpMaskAscii(t *testing.T, mask []byte, w, h int) {
	t.Helper()
	for gy := 0; gy < h; gy += 6 {
		row := ""
		for gx := 0; gx < w; gx += 6 {
			ink := false
			for y := gy; y < gy+6 && y < h; y++ {
				for x := gx; x < gx+6 && x < w; x++ {
					if mask[y*w+x] != 0 {
						ink = true
					}
				}
			}
			if ink {
				row += "#"
			} else {
				row += "."
			}
		}
		t.Logf("  y=%3d: %s", gy, row)
	}
}