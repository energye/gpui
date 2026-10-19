//go:build !nogpu

package render_test

import (
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// Stroke damage must cover the stroked outline, not just the path bounds:
// half the line width plus caps/joins paint outside the path. Damage-tight
// layer composites blit only damage+pad, so under-reporting clipped a thick
// stroke's outer edge (2026-10-19, found while closing the text stroke/path
// window).
func TestStrokeDamageCoversLineWidth(t *testing.T) {
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.SetDamageTracking(true)
	dc.ResetFrameDamage()
	dc.SetRGB(1, 0, 0)
	dc.SetLineWidth(12)
	dc.DrawRectangle(20, 20, 24, 24)
	_ = dc.Stroke()

	// Path bounds are 20..44; width 12 paints 14..50.
	damage := dc.FrameDamageUnion()
	if damage.Min.X > 14 || damage.Min.Y > 14 || damage.Max.X < 50 || damage.Max.Y < 50 {
		t.Fatalf("stroke damage = %v, want to cover painted bounds 14..50", damage)
	}
	if damage.Min.X < 13 || damage.Min.Y < 13 || damage.Max.X > 51 || damage.Max.Y > 51 {
		t.Fatalf("stroke damage = %v, want a tight-ish bound (over-report is a bug too)", damage)
	}

	// Pixel proof: an isolated layer composites only its damage union.
	dc2 := render.NewContext(64, 64)
	defer dc2.Close()
	dc2.SetDamageTracking(true)
	dc2.ResetFrameDamage()
	dc2.PushLayerIsolated(1)
	dc2.SetRGB(1, 1, 1)
	dc2.SetLineWidth(12)
	dc2.DrawRectangle(20, 20, 24, 24)
	_ = dc2.Stroke()
	dc2.PopLayer()
	img := dc2.Image()
	for _, pt := range [][2]int{{14, 32}, {32, 14}, {49, 32}, {32, 49}} {
		if r, g, b, a := img.At(pt[0], pt[1]).RGBA(); r < 0xc000 || g < 0xc000 || b < 0xc000 || a < 0xc000 {
			t.Fatalf("pixel(%d,%d) = %d,%d,%d,%d, want opaque white (stroke outer edge clipped by damage composite)",
				pt[0], pt[1], r>>8, g>>8, b>>8, a>>8)
		}
	}
	// Nothing paints beyond the stroke's outer edge.
	for _, pt := range [][2]int{{8, 32}, {32, 8}, {55, 32}, {32, 55}} {
		if _, _, _, a := img.At(pt[0], pt[1]).RGBA(); a != 0 {
			t.Fatalf("pixel(%d,%d) alpha = %d, want transparent (painted outside the stroke)", pt[0], pt[1], a>>8)
		}
	}
}

// The outline cache key must keep fractional ppem sizes distinct. The old
// int16 key truncated 20.8 to 20, so the second face reused the first face's
// outline (2026-10-19).
func TestTextOutlineCacheKeepsFractionalSizes(t *testing.T) {
	fontPath := filepath.Join("text", "testdata", "ahem.ttf")
	src, err := text.NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("test font unavailable (%v)", err)
	}
	outlineMaxX := func(size float64) float64 {
		dc := render.NewContext(200, 100)
		defer dc.Close()
		dc.SetFont(src.Face(size))
		p := dc.TextPath("A", 10, 60)
		if p == nil {
			t.Fatalf("TextPath returned nil at size %v", size)
		}
		maxX := -1e9
		p.Iterate(func(_ render.PathVerb, coords []float64) {
			for i := 0; i+1 < len(coords); i += 2 {
				if coords[i] > maxX {
					maxX = coords[i]
				}
			}
		})
		return maxX
	}

	// Ahem 'A' is a filled square of one em: 10 + size.
	small := outlineMaxX(20.2)
	large := outlineMaxX(20.8)
	if want := 30.2; small < want-0.05 || small > want+0.05 {
		t.Fatalf("outline maxX at 20.2 = %.4f, want %.4f", small, want)
	}
	if want := 30.8; large < want-0.05 || large > want+0.05 {
		t.Fatalf("outline maxX at 20.8 = %.4f, want %.4f (fractional size shared a cache entry)", large, want)
	}
}
