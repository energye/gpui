package render

import "testing"

// TestOffscreenPassSkipsAncestorClipMask is a regression guard for the record
// pass of a retained layer: it must rasterize the FULL picture. The ancestor
// mask/difference clip lives in surface coordinates while the pass draws at
// record-local coordinates, so promoting or baking it there zeroes every fill
// inside the layer (observed as an invisible text caret and selection, whose
// small RTs sampled the surface mask at (0..2, 0..20)).
func TestOffscreenPassSkipsAncestorClipMask(t *testing.T) {
	dc := NewContext(64, 64)
	defer dc.Close()

	dc.ClipRect(0, 0, 64, 64)
	dc.ClipRectOp(16, 16, 32, 32, ClipOpDifference)
	if !dc.maskClipPromotionAllowed() {
		t.Fatal("mask clip must be promotable outside an offscreen pass")
	}

	dc.offscreenPassDepth = 1
	if dc.maskClipPromotionAllowed() {
		t.Fatal("ancestor mask clip must not be promoted into an offscreen record pass")
	}

	m := NewMask(64, 64)
	m.Set(1, 1, 255)
	dc.SetMask(m)
	dc.applyMaskToPaint()
	if dc.paint.MaskCoverage != nil {
		t.Fatal("ancestor mask coverage must not be baked into an offscreen record pass")
	}

	dc.offscreenPassDepth = 0
	dc.applyMaskToPaint()
	if dc.paint.MaskCoverage == nil {
		t.Fatal("mask coverage must still apply outside an offscreen pass")
	}
}
