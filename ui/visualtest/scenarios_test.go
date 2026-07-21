package visualtest_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/visualtest"
)

// paintRoundRectFillStroke draws the fixed first-period chrome sample:
// 64×64 white canvas, centered 48×48 r=6 fill + 1px stroke.
func paintRoundRectFillStroke(pc *core.PaintContext) {
	// 8px margin → 48×48 box centered on 64×64
	const (
		x, y   = 8.0, 8.0
		w, h   = 48.0, 48.0
		radius = 6.0
		border = 1.0
	)
	fill := render.RGBA{R: 0.90, G: 0.92, B: 0.96, A: 1}   // light blue-gray fill
	stroke := render.RGBA{R: 0.20, G: 0.35, B: 0.70, A: 1} // solid border
	pc.FillLocalRoundRect(x, y, w, h, radius, fill)
	pc.StrokeLocalRoundRect(x, y, w, h, radius, border, stroke)
}

func TestScenario_RoundRectFillStroke(t *testing.T) {
	img := visualtest.Capture(64, 64, paintRoundRectFillStroke)
	if img == nil {
		t.Fatal("nil capture")
	}
	b := img.Bounds()
	if b.Dx() != 64 || b.Dy() != 64 {
		t.Fatalf("size %dx%d want 64x64", b.Dx(), b.Dy())
	}
	// Must paint non-white ink (not empty chrome).
	if countDarkish(img, 250) < 100 {
		t.Fatalf("expected rounded-rect ink, got nearly blank canvas")
	}
	visualtest.AssertScenario(t, "roundrect_fill_stroke", img, visualtest.DefaultCompare)
}

// TestRoundRect_BrokenInsetDiffers proves track-2 catches stroke inset/radius damage:
// stroking the outer path without inset must not match the approved baseline.
func TestRoundRect_BrokenInsetDiffers(t *testing.T) {
	baseline, err := visualtest.LoadPNG(visualtest.BaselinePath("roundrect_fill_stroke"))
	if err != nil {
		t.Skipf("baseline missing (run UPDATE_VISUAL=1 first): %v", err)
	}

	// Intentionally wrong: stroke centered on outer path (no inset, full radius).
	broken := visualtest.Capture(64, 64, func(pc *core.PaintContext) {
		const x, y, w, h, radius, border = 8.0, 8.0, 48.0, 48.0, 6.0, 1.0
		fill := render.RGBA{R: 0.90, G: 0.92, B: 0.96, A: 1}
		stroke := render.RGBA{R: 0.20, G: 0.35, B: 0.70, A: 1}
		pc.FillLocalRoundRect(x, y, w, h, radius, fill)
		// Bypass PaintContext stroke: raw outer path → wrong chrome.
		if pc.DC == nil {
			return
		}
		pc.DC.SetRGBA(stroke.R, stroke.G, stroke.B, stroke.A)
		pc.DC.SetLineWidth(border)
		pc.DC.DrawRoundedRectangle(pc.Origin.X+x, pc.Origin.Y+y, w, h, radius)
		_ = pc.DC.Stroke()
	})

	_, err = visualtest.CompareImages(baseline, broken, visualtest.DefaultCompare)
	if err == nil {
		t.Fatal("expected visual mismatch when stroke inset is removed; mechanism ineffective")
	}
	t.Logf("mechanism ok: broken inset differs: %v", err)
}

// TestCompare_ToleranceExactMatch is a unit check for the compare helper.
func TestCompare_ToleranceExactMatch(t *testing.T) {
	img := visualtest.Capture(8, 8, func(pc *core.PaintContext) {
		pc.FillLocalRect(0, 0, 8, 8, render.RGBA{R: 1, G: 0, B: 0, A: 1})
	})
	stats, err := visualtest.CompareImages(img, img, visualtest.DefaultCompare)
	if err != nil {
		t.Fatal(err)
	}
	if stats.DiffPixels != 0 {
		t.Fatalf("self-diff pixels=%d", stats.DiffPixels)
	}
}

func countDarkish(img image.Image, thr uint32) int {
	b := img.Bounds()
	n := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			if r>>8 < thr || g>>8 < thr || bl>>8 < thr {
				n++
			}
		}
	}
	return n
}
