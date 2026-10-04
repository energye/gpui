package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// TestRenderText_SpacingMeasureEqualsLayout locks single-source deviation 0:
// the spaced measure path and the spaced layout share one builder, and the
// bump is exact on 1:1 shaped text.
func TestRenderText_SpacingMeasureEqualsLayout(t *testing.T) {
	face := loadTestFace(t, 16)
	for _, tc := range []struct {
		text string
		ls   float64
		want float64 // exact bump over unspaced (1 gap)
	}{
		{"AB", 6, 6},
		{"鹈鹕", 8, 8},
	} {
		plain := rendering.NewRenderText(tc.text)
		plain.SetFace(face)
		plain.SetFontSize(16)
		_ = plain.Layout(rendering.Loose(400, 80))
		plainW := spacedLineWidth(t, plain)

		spaced := rendering.NewRenderText(tc.text)
		spaced.SetFace(face)
		spaced.SetFontSize(16)
		spaced.SetLetterSpacing(tc.ls)
		_ = spaced.Layout(rendering.Loose(400, 80))
		spacedW := spacedLineWidth(t, spaced)

		if d := spacedW - plainW; d != tc.want {
			t.Fatalf("%q bump=%v want %v", tc.text, d, tc.want)
		}
		if m := spaced.MeasureWidth(tc.text); m != spacedW {
			t.Fatalf("%q measure=%v layout=%v want equal", tc.text, m, spacedW)
		}
	}
}

// TestRenderText_SpacingPaintShiftsInk: spaced paint puts ink further right.
func TestRenderText_SpacingPaintShiftsInk(t *testing.T) {
	face := loadTestFace(t, 16)
	rightmost := func(ls float64) int {
		txt := rendering.NewRenderText("AB")
		txt.SetFace(face)
		txt.SetFontSize(16)
		txt.R, txt.G, txt.B, txt.A = 0, 0, 0, 1
		if ls != 0 {
			txt.SetLetterSpacing(ls)
		}
		_ = txt.Layout(rendering.Loose(400, 80))
		dc := render.NewContext(200, 80)
		defer dc.Close()
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		txt.Paint(rendering.NewPaintContext(dc, 1))
		img := dc.Image()
		edge := -1
		for x := 0; x < 200; x++ {
			for y := 0; y < 80; y++ {
				r, g, b := sampleRGB(img.At(x, y))
				if r < 0x4000 && g < 0x4000 && b < 0x4000 {
					edge = x
					break
				}
			}
		}
		return edge
	}
	plain, spaced := rightmost(0), rightmost(10)
	if plain < 0 || spaced < 0 {
		t.Fatal("no ink painted in one path")
	}
	if spaced <= plain {
		t.Fatalf("spaced edge=%d plain edge=%d want spaced further right", spaced, plain)
	}
}

// TestRenderText_SpacingInvalidatesParentCache: text rides parent boundaries;
// changing spacing must invalidate the parent picture (fingerprint), not
// replay stale unspaced pixels.
func TestRenderText_SpacingInvalidatesParentCache(t *testing.T) {
	face := loadTestFace(t, 16)
	cache := rendering.NewBoundaryCache()
	root := rendering.NewAbsoluteBox(200, 60)
	root.SetRepaintBoundary(true)
	child := rendering.NewRenderText("AB")
	child.SetFace(face)
	child.SetFontSize(16)
	root.Place(child, 10, 10)
	root.Layout(rendering.Tight(200, 60))

	paint := func() {
		dc := render.NewContext(200, 60)
		defer dc.Close()
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		pc := rendering.NewPaintContext(dc, 1)
		pc.BoundaryCache, pc.UseBoundaryCache = cache, true
		cache.BeginFrame()
		root.Paint(pc)
	}
	paint()
	rr, skip, _, _ := cache.FrameCounts()
	if rr != 1 || skip != 0 || !cache.HasValid(root) {
		t.Fatalf("first paint rr=%d skip=%d valid=%v want 1/0/true", rr, skip, cache.HasValid(root))
	}
	paint()
	if rr, skip, _, _ := cache.FrameCounts(); rr != 0 || skip != 1 {
		t.Fatalf("clean repaint rr=%d skip=%d want 0/1", rr, skip)
	}
	child.SetLetterSpacing(6)
	paint()
	if rr, _, _, _ := cache.FrameCounts(); rr != 1 {
		t.Fatalf("spacing change must re-record, rr=%d", rr)
	}
	if !cache.HasValid(root) {
		t.Fatal("parent must hold the re-recorded spaced picture")
	}
}

func spacedLineWidth(t *testing.T, txt *rendering.RenderText) float64 {
	t.Helper()
	lay := txt.TextLayout()
	if lay == nil || lay.LineCount() == 0 {
		t.Fatal("no layout")
	}
	_, _, w, _, ok := lay.Line(0)
	if !ok {
		t.Fatal("no line 0")
	}
	return w
}
