//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package filter

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
)

// P3.4 batch-1 value pins: morphology erode/dilate duality, offset exact
// translation, alpha-threshold binarization — all in the premul domain.

func newTestPixmap(t *testing.T, w, h int) *render.Pixmap {
	t.Helper()
	return render.NewPixmap(w, h)
}

func fillRect(pm *render.Pixmap, x0, y0, x1, y1 int, c render.RGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			pm.SetPixel(x, y, c)
		}
	}
}

func fullRect(pm *render.Pixmap) scene.Rect {
	return scene.Rect{MinX: 0, MinY: 0, MaxX: float32(pm.Width()), MaxY: float32(pm.Height())}
}

func pxAt(pm *render.Pixmap, x, y int) [4]uint8 {
	d := pm.Data()
	i := (y*pm.Width() + x) * 4
	return [4]uint8{d[i], d[i+1], d[i+2], d[i+3]}
}

// TestErodeDilateDuality pins the min/max duality: dilate(erode(img)) ≥ img
// (opening) and erode shrinks the bright square by exactly the radius.
func TestErodeDilateDuality(t *testing.T) {
	src := newTestPixmap(t, 32, 32)
	src.Clear(render.RGBA{})
	fillRect(src, 8, 8, 24, 24, render.RGBA{R: 1, A: 1}) // opaque white square

	er := newTestPixmap(t, 32, 32)
	er.Clear(render.RGBA{})
	NewErodeFilter(2, 2).Apply(src, er, fullRect(er))
	// Erode shrinks: the corner of the square moves inward by r.
	if px := pxAt(er, 9, 9); px[0] != 0 {
		t.Fatalf("erode at (9,9)=%v, want transparent (corner consumed)", px)
	}
	if px := pxAt(er, 16, 16); px[0] != 255 {
		t.Fatalf("erode at center=%v, want opaque white", px)
	}

	di := newTestPixmap(t, 32, 32)
	di.Clear(render.RGBA{})
	NewDilateFilter(2, 2).Apply(src, di, fullRect(di))
	if px := pxAt(di, 6, 6); px[0] != 255 {
		t.Fatalf("dilate at (6,6)=%v, want opaque (square grown)", px)
	}
	if px := pxAt(di, 5, 5); px[0] != 0 {
		t.Fatalf("dilate at (5,5)=%v, want untouched", px)
	}

	// Duality: dilate(erode) ⊆ original ⊆ erode(dilate).
	opened := newTestPixmap(t, 32, 32)
	opened.Clear(render.RGBA{})
	NewDilateFilter(2, 2).Apply(er, opened, fullRect(opened))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			if openedPx := pxAt(opened, x, y)[0]; openedPx > srcPx(src, x, y)[0] {
				t.Fatalf("opening exceeds original at (%d,%d)", x, y)
			}
		}
	}
}

func srcPx(pm *render.Pixmap, x, y int) [4]uint8 { return pxAt(pm, x, y) }

// TestOffsetExactTranslation pins the whole-pixel shift.
func TestOffsetExactTranslation(t *testing.T) {
	src := newTestPixmap(t, 16, 16)
	src.Clear(render.RGBA{})
	fillRect(src, 2, 2, 6, 6, render.RGBA{R: 1, A: 1})
	dst := newTestPixmap(t, 16, 16)
	dst.Clear(render.RGBA{})
	NewOffsetFilter(4, 3).Apply(src, dst, fullRect(dst))
	if px := pxAt(dst, 7, 6); px[0] != 255 {
		t.Fatalf("offset at (7,6)=%v, want opaque (2+4,2+3)", px)
	}
	if px := pxAt(dst, 3, 3); px[0] != 0 {
		t.Fatalf("offset origin=%v, want vacated", px)
	}
	if px := pxAt(dst, 12, 12); px[0] != 0 {
		t.Fatalf("offset uncovered=%v, want transparent", px)
	}
}

// TestAlphaThresholdBinarization pins the hard alpha cutoff.
func TestAlphaThresholdBinarization(t *testing.T) {
	src := newTestPixmap(t, 8, 8)
	src.Clear(render.RGBA{})
	src.SetPixel(0, 0, render.RGBA{R: 1, A: 0.3}) // below 0.5
	src.SetPixel(1, 0, render.RGBA{R: 1, A: 0.7}) // above
	dst := newTestPixmap(t, 8, 8)
	dst.Clear(render.RGBA{})
	NewAlphaThresholdFilter(128).Apply(src, dst, fullRect(dst))
	if px := pxAt(dst, 0, 0); px[3] != 0 {
		t.Fatalf("below-threshold alpha=%d, want 0", px[3])
	}
	if px := pxAt(dst, 1, 0); px[3] == 0 {
		t.Fatal("above-threshold pixel was zeroed")
	}
}
