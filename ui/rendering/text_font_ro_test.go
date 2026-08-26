package rendering_test

import (
	"math"
	"testing"
	"unicode/utf8"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// loadTestFace loads a real system UI face or skips when none is available.
func loadTestFace(t *testing.T, points float64) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, desc, err := rendering.TryLoadDefaultFace(points)
	if err != nil || face == nil {
		t.Skipf("no system face for RO font tests: %v", err)
	}
	t.Log("face:", desc, "size:", face.Size())
	return face
}

// TestRenderText_LayoutUsesFaceMeasure drives shipped Layout with a real Face:
// measured width must come from Face.Measure, not the rune×ApproxCharW heuristic.
func TestRenderText_LayoutUsesFaceMeasure(t *testing.T) {
	face := loadTestFace(t, 16)
	const s = "Hello gpui WWW"
	txt := rendering.NewRenderText(s)
	txt.FontSize = 16
	txt.ApproxCharW = 0.55 // deliberate heuristic that differs from real advances
	txt.SetFace(face)

	sz := txt.Layout(rendering.Loose(800, 100))
	heu := float64(utf8.RuneCountInString(s)) * 16 * 0.55
	realW, _ := text.Measure(s, face)
	if math.Abs(sz.Width-realW) > 1.0 {
		t.Fatalf("layout width=%.2f want face measure ≈%.2f (heuristic would be %.2f)", sz.Width, realW, heu)
	}
	// Prove we did not fall through to heuristic (DejaVu "WWW" is wider than 0.55em avg).
	if math.Abs(sz.Width-heu) < 0.5 {
		t.Fatalf("layout width=%.2f equals heuristic %.2f — face path not used", sz.Width, heu)
	}
}

// TestRenderText_SetFontSize_RescalesFaceMeasure: same Face source, different
// FontSize via SetFontSize must change laid-out width (face re-derived at size).
func TestRenderText_SetFontSize_RescalesFaceMeasure(t *testing.T) {
	face := loadTestFace(t, 12)
	const s = "Size scale WWW"
	txt := rendering.NewRenderText(s)
	txt.SetFace(face)
	txt.SetFontSize(12)
	sz12 := txt.Layout(rendering.Loose(800, 100))

	txt.SetFontSize(24)
	// Force relayout after size change.
	txt.MarkNeedsLayout()
	sz24 := txt.Layout(rendering.Loose(800, 100))

	if sz24.Width <= sz12.Width*1.4 {
		t.Fatalf("24pt width=%.2f should be substantially larger than 12pt %.2f", sz24.Width, sz12.Width)
	}
	ratio := sz24.Width / sz12.Width
	if ratio < 1.7 || ratio > 2.4 {
		t.Fatalf("size ratio=%.2f want ~2.0 (12→24pt)", ratio)
	}
	if sz24.Height <= sz12.Height {
		t.Fatalf("24pt height=%.2f should exceed 12pt %.2f", sz24.Height, sz12.Height)
	}
}

// TestRenderText_PaintSetsFaceOnDC drives shipped Paint: after paint the DC
// font must be the size-synced face (DrawString requires SetFont).
func TestRenderText_PaintSetsFaceOnDC(t *testing.T) {
	face := loadTestFace(t, 14)
	txt := rendering.NewRenderText("Paint face")
	txt.SetFace(face)
	txt.SetFontSize(20) // different from face load size → must re-derive
	txt.R, txt.G, txt.B, txt.A = 1, 0, 0, 1
	_ = txt.Layout(rendering.Loose(400, 80))

	dc := render.NewContext(200, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	if dc.Font() != nil {
		t.Fatal("DC font should start nil")
	}
	pc := rendering.NewPaintContext(dc, 1)
	txt.Paint(pc)

	got := dc.Font()
	if got == nil {
		t.Fatal("Paint must SetFont on DC (DrawString no-ops without face)")
	}
	// Size should track FontSize (20), not the original load size (14).
	if d := math.Abs(got.Size() - 20); d > 0.5 {
		t.Fatalf("DC face size=%.2f want ~20 (FontSize), loaded was 14", got.Size())
	}

	// Non-trivial paint: some red ink near baseline region.
	img := dc.Image()
	if img == nil {
		t.Fatal("nil image")
	}
	found := false
	for y := 0; y < 80 && !found; y++ {
		for x := 0; x < 200; x++ {
			r, g, b := sampleRGB(img.At(x, y))
			if r > 0x8000 && g < 0x6000 && b < 0x6000 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("expected red text pixels after Paint with face (face not applied or draw failed)")
	}
}

// TestRenderText_NoFaceStillHeuristic keeps the no-face path green.
func TestRenderText_NoFaceStillHeuristic(t *testing.T) {
	const s = "AB"
	txt := rendering.NewRenderText(s)
	txt.FontSize = 10
	txt.ApproxCharW = 1.0
	// No SetFace
	sz := txt.Layout(rendering.Loose(400, 40))
	want := float64(utf8.RuneCountInString(s)) * 10 * 1.0
	if math.Abs(sz.Width-want) > 0.1 {
		t.Fatalf("no-face width=%.2f want heuristic %.2f", sz.Width, want)
	}
}

// TestRenderText_GlyphInkBounds pins the ink-bounds accessor used for caret
// gap centering: ink box of a visible glyph must be non-empty and sit inside
// its advance box; whitespace has an empty outline (MinX==MaxX).
func TestRenderText_GlyphInkBounds(t *testing.T) {
	face := loadTestFace(t, 20)
	txt := rendering.NewRenderText("")
	txt.SetFace(face)
	txt.SetFontSize(20)

	d, dok := txt.GlyphInkBounds('d')
	if !dok || d.MinX >= d.MaxX {
		t.Fatalf("'d' ink bounds=%v ok=%v — want non-empty outline", d, dok)
	}
	l, lok := txt.GlyphInkBounds('l')
	if !lok || l.MinX >= l.MaxX {
		t.Fatalf("'l' ink bounds=%v ok=%v — want non-empty outline", l, lok)
	}
	if l.MinX <= 0 || l.MinX > d.MaxX {
		t.Fatalf("'l' ink-left=%.2f should be a positive left-side bearing within 'd' advance (%.2f)", l.MinX, d.MaxX)
	}
	sp, sok := txt.GlyphInkBounds(' ')
	if !sok {
		t.Skipf("face yields no glyph for space")
	}
	if sp.MinX != sp.MaxX {
		t.Fatalf("space outline must be empty (MinX==MaxX), got [%v]", sp)
	}
	if _, ok := txt.GlyphInkBounds(0x01); ok {
		t.Fatal("control char must report ok=false")
	}
}
