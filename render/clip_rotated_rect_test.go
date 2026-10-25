package render

import (
	"fmt"
	"math"
	"testing"
)

// TestClipRectRotatedPrecise pins the B1 fix: with a rotated CTM, ClipRect
// must clip to the transformed QUADRILATERAL (Skia clips rect via the exact
// transformed quad), not the AABB of two transformed corners — pre-fix the
// four corner regions inside the AABB but outside the quad stayed visible.
//
// Oracle: rect (100..200, 100..200) rotated 45° about (150,150) becomes a
// diamond with vertices (150, 150±70.71) and (150±70.71, 150). The point
// (110,110) lies inside the corner AABB (≈(79,79)-(221,221)) but outside the
// diamond — it must NOT be painted.
//
// Engine save/restore: Push records the clip-stack depth and Pop pops the
// clip back to it (Skia save/clip/restore semantics), so the clipped fill
// happens INSIDE the pushed layer.
func TestClipRectRotatedPrecise(t *testing.T) {
	c := NewContext(300, 300)
	c.ClearWithColor(RGBA{R: 0.05, G: 0.05, B: 0.08, A: 1})

	c.Push()
	c.RotateAbout(math.Pi/4, 150, 150)
	c.ClipRectOp(100, 100, 100, 100, ClipOpIntersect)
	c.SetRGBA(1, 0, 0, 1)
	c.DrawRectangle(0, 0, 300, 300)
	if err := c.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	c.Pop()
	img := c.Image()

	at := func(x, y int) (r, g, b uint8) {
		r32, g32, b32, _ := img.At(x, y).RGBA()
		return uint8(r32 >> 8), uint8(g32 >> 8), uint8(b32 >> 8)
	}
	red := func(x, y int) bool {
		r, g, b := at(x, y)
		return r > 200 && g < 60 && b < 60
	}
	// Inside the diamond center: must be red.
	if !red(150, 150) {
		t.Fatalf("diamond center (150,150) not clipped-in (got %v)", fmt.Sprint(at(150, 150)))
	}
	// Corner leaks (inside corner AABB, outside the rotated quad): pre-fix
	// these are red.
	for _, p := range [][2]int{{110, 110}, {190, 110}, {110, 190}, {190, 190}} {
		if red(p[0], p[1]) {
			t.Fatalf("corner point (%d,%d) painted (%v) — ClipRect used the corner AABB instead of the rotated quad", p[0], p[1], fmt.Sprint(at(p[0], p[1])))
		}
	}
}
