package render

import "testing"

// TestStrokeSubpixelWidthCoverage pins Skia/Flutter hairline semantics:
// only a zero-width stroke is promoted to one device pixel. A positive
// sub-pixel width keeps its coverage (0.5px ≈ 50% alpha), so Thin() no
// longer renders as a fully opaque 1px line.
func TestStrokeSubpixelWidthCoverage(t *testing.T) {
	alphaAt := func(width, y float64) uint8 {
		pm := NewPixmap(32, 32)
		r := NewSoftwareRenderer(32, 32)
		p := NewPath()
		p.MoveTo(4, y)
		p.LineTo(28, y)
		paint := NewPaint()
		paint.SetBrush(SolidRGBA(1, 0, 0, 1))
		paint.LineWidth = width
		if err := r.Stroke(pm, p, paint); err != nil {
			t.Fatalf("stroke width %v: %v", width, err)
		}
		return pm.Data()[(10*32+16)*4+3]
	}

	// Center on y=10.5: the 1px band covers row 10 fully, the 0.5px band
	// covers half of row 10.
	if got := alphaAt(1.0, 10.5); got != 255 {
		t.Fatalf("1px stroke alpha=%d, want 255", got)
	}
	if got := alphaAt(0.5, 10.5); got < 120 || got > 136 {
		t.Fatalf("0.5px stroke alpha=%d, want ~128 (sub-pixel coverage)", got)
	}
	// 0.1px is below the 4x edge-builder quantum, so the thin-stroke
	// normalisation renders it as a 1px line at 10% alpha instead of
	// letting it quantise to zero.
	if got := alphaAt(0.1, 10.5); got < 20 || got > 32 {
		t.Fatalf("0.1px stroke alpha=%d, want ~26 (10%% coverage)", got)
	}
	// Width 0 is the Flutter hairline and must stay a visible 1px line.
	if got := alphaAt(0, 10.5); got != 255 {
		t.Fatalf("hairline alpha=%d, want 255", got)
	}
}
