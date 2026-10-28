//go:build !nogpu

package render_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// alphaAt returns the 8-bit alpha at (x,y).
func alphaAt(img image.Image, x, y int) uint8 {
	_, _, _, a := img.At(x, y).RGBA()
	return uint8(a >> 8)
}

// TestLayerRT_NestedIsolatedStartsTransparent pins the offscreen clear color:
// a fresh layer RT must start transparent (Skia saveLayer base), not opaque
// black. The first composite blit into a nested isolated layer used to clear
// with the surface's opaque black, turning the whole parent layer opaque as
// soon as the child composited (nested isolation). Regression guard for the
// blit-only compositor clear value (offscreen targets clear a=0).
func TestLayerRT_NestedIsolatedStartsTransparent(t *testing.T) {
	requireNativeGPU(t)

	const size = 100
	dc := render.NewContext(size, size)
	defer dc.Close()

	dc.PushLayerIsolated(1.0)
	dc.PushLayerIsolated(1.0)
	dc.SetRGBA(1, 0, 0, 1)
	dc.DrawRectangle(80, 80, 10, 10)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	dc.PopLayer()
	dc.PopLayer()

	img := dc.Image()
	if a := alphaAt(img, 85, 85); a != 255 {
		t.Errorf("content pixel alpha = %d, want 255", a)
	}
	for _, p := range [][2]int{{5, 5}, {95, 5}, {5, 95}, {95, 95}} {
		if a := alphaAt(img, p[0], p[1]); a != 0 {
			t.Errorf("nested isolated layer: alpha at (%d,%d) = %d, want 0 (layer RT must start transparent)", p[0], p[1], a)
		}
	}
}

// TestLayerRT_NestedMaskedStartsTransparent pins the same offscreen clear for
// masked layers: inner mask keeps the top half, outer mask keeps the left half,
// so only the top-left quadrant may carry content.
func TestLayerRT_NestedMaskedStartsTransparent(t *testing.T) {
	requireNativeGPU(t)

	const size = 100
	dc := render.NewContext(size, size)
	defer dc.Close()

	outer := render.NewMask(size, size) // left half
	for y := 0; y < size; y++ {
		for x := 0; x < size/2; x++ {
			outer.Set(x, y, 255)
		}
	}
	inner := render.NewMask(size, size) // top half
	for y := 0; y < size/2; y++ {
		for x := 0; x < size; x++ {
			inner.Set(x, y, 255)
		}
	}
	dc.PushMaskLayer(outer)
	dc.PushMaskLayer(inner)
	dc.SetRGBA(1, 0, 0, 1)
	dc.DrawRectangle(0, 0, size, size)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	dc.PopLayer()
	dc.PopLayer()

	img := dc.Image()
	if a := alphaAt(img, 25, 25); a != 255 {
		t.Errorf("top-left (both masks allow) alpha = %d, want 255", a)
	}
	for _, p := range [][2]int{{75, 25}, {25, 75}, {75, 75}} {
		if a := alphaAt(img, p[0], p[1]); a != 0 {
			t.Errorf("nested masked layer: alpha at (%d,%d) = %d, want 0", p[0], p[1], a)
		}
	}
}
