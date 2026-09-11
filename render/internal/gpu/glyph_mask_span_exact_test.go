//go:build !nogpu

package gpu

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// TestShapedSubmitSpanExact guards the integer-grid contract the shaped
// path chose (see layoutGlyphs): absolute rounding keeps the run span
// identical to the caret table (no cumulative drift, no suffix overlap),
// so equal fractional advances alternate between floor and ceil pitch.
// This test locks that choice: gaps must stay within {floor,ceil} of the
// shaped pitch AND the total span must match (an advance-rounded grid
// would pass the gaps but drift the span ~0.3px/glyph and fail below).
// Even ink on repeats needs near-integer advances from script-appropriate
// fallback, not a different rounding here.
func TestShapedSubmitEvenSpacing(t *testing.T) {
	face := glyphMaskTestFont(t, 16)
	eng := NewGlyphMaskEngine()
	base := text.Shape("ffffffff", face)
	if len(base) == 0 {
		t.Skip("no f shaping")
	}
	const pitch = 10.288
	sg := append([]text.ShapedGlyph(nil), base...)
	for i := range sg {
		sg[i].X = float64(i) * pitch
		sg[i].XAdvance = pitch
	}
	b, err := eng.LayoutShapedGlyphs(face, sg, 100.0, 20, render.RGBA{A: 1}, render.Identity(), 1.0, false)
	if err != nil {
		t.Fatalf("LayoutShapedGlyphs: %v", err)
	}
	if len(b.Quads) != len(sg) {
		t.Fatalf("quads=%d want %d", len(b.Quads), len(sg))
	}
	lo, hi := math.Floor(pitch), math.Ceil(pitch)
	for i := 1; i < len(b.Quads); i++ {
		gap := float64(b.Quads[i].X0 - b.Quads[i-1].X0)
		if gap < lo-0.01 || gap > hi+0.01 {
			t.Fatalf("gap%d=%.2f outside {%.0f,%.0f} (placement broke)", i, gap, lo, hi)
		}
	}
	span := float64(b.Quads[len(b.Quads)-1].X0 - b.Quads[0].X0)
	if math.Abs(span-float64(len(sg)-1)*pitch) > 1.0 {
		t.Fatalf("span=%.2f want %.2f (cumulative drift)", span, float64(len(sg)-1)*pitch)
	}
}
