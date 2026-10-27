//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"image"
	"math"
	"testing"
)

// Gradient / pattern brushes are defined in user space, but the rasterizer
// samples them at device coordinates (paths are baked by deviceSpacePath).
// These tests pin that brush geometry follows the CTM: the same drawing at a
// translated / rotated / scaled CTM must land where the geometry lands, and
// non-similarity transforms must keep sampling exact through the inverse.

const gradientCTMTol = 0.02

func gradientCTMContext(t *testing.T, w, h int, opts ...ContextOption) *Context {
	t.Helper()
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	dc := NewContext(w, h, opts...)
	t.Cleanup(func() { _ = dc.Close() })
	return dc
}

// gradientCTMLinear is the reference brush: black at x=0 to red at x=100,
// padded so everything past 100 stays red.
func gradientCTMLinear() *LinearGradientBrush {
	g := NewLinearGradientBrush(0, 0, 100, 0)
	g.AddColorStop(0, RGBA{R: 0, G: 0, B: 0, A: 1})
	g.AddColorStop(1, RGBA{R: 1, G: 0, B: 0, A: 1})
	return g
}

func gradientCTMFill(t *testing.T, dc *Context, x, y, w, h float64) {
	t.Helper()
	dc.DrawRectangle(x, y, w, h)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
}

func gradientCTMPixel(t *testing.T, dc *Context, x, y int) RGBA {
	t.Helper()
	return dc.pixmap.GetPixel(x, y)
}

func gradientCTMEqual(t *testing.T, got, want RGBA, tol float64, what string) {
	t.Helper()
	if math.Abs(got.R-want.R) > tol || math.Abs(got.G-want.G) > tol ||
		math.Abs(got.B-want.B) > tol || math.Abs(got.A-want.A) > tol {
		t.Errorf("%s = (%.4f,%.4f,%.4f,%.4f), want (%.4f,%.4f,%.4f,%.4f) ±%.3f",
			what, got.R, got.G, got.B, got.A, want.R, want.G, want.B, want.A, tol)
	}
}

func TestGradientFollowsCTM_Translate(t *testing.T) {
	const w, h = 220, 100
	base := gradientCTMContext(t, w, h)
	base.SetFillBrush(gradientCTMLinear())
	gradientCTMFill(t, base, 0, 0, 100, h)

	moved := gradientCTMContext(t, w, h)
	moved.Translate(100, 0)
	moved.SetFillBrush(gradientCTMLinear())
	gradientCTMFill(t, moved, 0, 0, 100, h)

	for _, x := range []int{1, 25, 50, 75, 98} {
		gradientCTMEqual(t, gradientCTMPixel(t, moved, x+100, 50), gradientCTMPixel(t, base, x, 50),
			gradientCTMTol, "translated gradient")
	}
	// The left of the translated rect must stay untouched (content moved with
	// the CTM, not stretched from the origin).
	gradientCTMEqual(t, gradientCTMPixel(t, moved, 50, 50), RGBA{}, 1e-9, "left of translated rect")
}

func TestGradientFollowsCTM_Rotate90(t *testing.T) {
	const w, h = 100, 100
	base := gradientCTMContext(t, w, h)
	base.SetFillBrush(gradientCTMLinear())
	gradientCTMFill(t, base, 0, 0, 100, 100)

	// Translate(100,0) * Rotate(90°) maps user (x,y) -> device (100-y, x), so
	// the horizontal gradient becomes vertical (start at the top).
	rot := gradientCTMContext(t, w, h)
	rot.Translate(100, 0)
	rot.Rotate(math.Pi / 2)
	rot.SetFillBrush(gradientCTMLinear())
	gradientCTMFill(t, rot, 0, 0, 100, 100)

	for _, x := range []int{1, 25, 50, 75, 98} {
		gradientCTMEqual(t, gradientCTMPixel(t, rot, 50, x), gradientCTMPixel(t, base, x, 50),
			gradientCTMTol, "rotated gradient")
	}
}

func TestGradientFollowsCTM_UniformScale(t *testing.T) {
	const w, h = 220, 220
	base := gradientCTMContext(t, w, h)
	base.SetFillBrush(gradientCTMLinear())
	gradientCTMFill(t, base, 0, 0, 100, 100)

	scaled := gradientCTMContext(t, w, h)
	scaled.Scale(2, 2)
	scaled.SetFillBrush(gradientCTMLinear())
	gradientCTMFill(t, scaled, 0, 0, 100, 100)

	for _, x := range []int{1, 25, 50, 75, 98} {
		gradientCTMEqual(t, gradientCTMPixel(t, scaled, 2*x, 100), gradientCTMPixel(t, base, x, 50),
			gradientCTMTol, "uniform-scaled gradient")
	}
}

// TestGradientFollowsCTM_NonUniformScaleOracle pins the inverse-sampling
// fallback: a non-uniform scale distorts the gradient's iso-lines, which no
// gradient geometry can express, so each device pixel must sample the base
// brush at the un-transformed user coordinate.
func TestGradientFollowsCTM_NonUniformScaleOracle(t *testing.T) {
	const w, h = 200, 100
	dc := gradientCTMContext(t, w, h)
	brush := NewLinearGradientBrush(0, 0, 100, 100) // diagonal: distorted by Scale(2,1)
	brush.AddColorStop(0, RGBA{R: 0, G: 0, B: 0, A: 1})
	brush.AddColorStop(1, RGBA{R: 0, G: 0, B: 1, A: 1})

	dc.Scale(2, 1)
	dc.SetFillBrush(brush)
	gradientCTMFill(t, dc, 0, 0, 100, 100)

	inv := Scale(2, 1).Invert()
	for _, x := range []int{10, 60, 100, 150, 190} {
		y := 40
		u := inv.TransformPoint(Pt(float64(x)+0.5, float64(y)+0.5))
		want := brush.ColorAt(u.X, u.Y)
		got := gradientCTMPixel(t, dc, x, y)
		if math.Abs(got.B-want.B) > 0.03 || math.Abs(got.R-want.R) > 0.03 {
			t.Errorf("non-uniform pixel (%d,%d) = (%.4f,%.4f,%.4f), want (%.4f,%.4f,%.4f)",
				x, y, got.R, got.G, got.B, want.R, want.G, want.B)
		}
	}
}

// TestGradientFollowsCTM_DeviceScale pins the HiDPI half of the same bug: a
// gradient at identity CTM with deviceScale=2 used to stop halfway across the
// physical surface because only the path was baked (brush ignored the scale).
func TestGradientFollowsCTM_DeviceScale(t *testing.T) {
	const w, h = 200, 200
	base := gradientCTMContext(t, w, h)
	big := NewLinearGradientBrush(0, 0, 200, 0)
	big.AddColorStop(0, RGBA{A: 1})
	big.AddColorStop(1, RGBA{R: 1, A: 1})
	base.SetFillBrush(big)
	gradientCTMFill(t, base, 0, 0, 200, 200)

	dc := gradientCTMContext(t, 100, 100, WithDeviceScale(2))
	dc.SetFillBrush(gradientCTMLinear())
	gradientCTMFill(t, dc, 0, 0, 100, 100)

	for _, x := range []int{1, 25, 50, 75, 98} {
		gradientCTMEqual(t, gradientCTMPixel(t, dc, 2*x, 100), gradientCTMPixel(t, base, 2*x, 100),
			gradientCTMTol, "device-scaled gradient")
	}
	// The physical surface is 200x200; the gradient must cover all of it (it
	// used to stop at device x=100 with the rest left empty).
	if got := gradientCTMPixel(t, dc, 150, 100); got.A < 0.99 {
		t.Errorf("dpr mid alpha = %.3f, want opaque (gradient must cover the whole surface)", got.A)
	}
}

func TestGradientFollowsCTM_RestoresState(t *testing.T) {
	dc := gradientCTMContext(t, 120, 120)
	brush := gradientCTMLinear()
	dc.SetFillBrush(brush)
	dc.Translate(30, 20)
	dc.SetAntiAlias(false)

	ctmBefore := dc.GetTransform()
	patternBefore := dc.paint.Pattern

	gradientCTMFill(t, dc, 0, 0, 50, 50)

	if dc.FillBrush() != Brush(brush) {
		t.Errorf("brush not restored after fill")
	}
	if got := dc.GetTransform(); got != ctmBefore {
		t.Errorf("CTM changed by fill: %v, want %v", got, ctmBefore)
	}
	if dc.paint.Pattern != patternBefore {
		t.Errorf("pattern not restored after fill")
	}
	if dc.AntiAlias() {
		t.Errorf("anti-aliasing state changed by fill")
	}
}

func TestGradientFollowsCTM_Stroke(t *testing.T) {
	const w, h = 220, 100
	base := gradientCTMContext(t, w, h)
	base.SetLineWidth(8)
	base.SetFillBrush(gradientCTMLinear())
	base.MoveTo(0, 50)
	base.LineTo(100, 50)
	if err := base.Stroke(); err != nil {
		t.Fatalf("Stroke: %v", err)
	}

	moved := gradientCTMContext(t, w, h)
	moved.Translate(100, 0)
	moved.SetLineWidth(8)
	moved.SetFillBrush(gradientCTMLinear())
	moved.MoveTo(0, 50)
	moved.LineTo(100, 50)
	if err := moved.Stroke(); err != nil {
		t.Fatalf("Stroke: %v", err)
	}

	for _, x := range []int{2, 25, 50, 75, 98} {
		gradientCTMEqual(t, gradientCTMPixel(t, moved, x+100, 50), gradientCTMPixel(t, base, x, 50),
			gradientCTMTol, "translated gradient stroke")
	}
}

func gradientCTMChecker() *ImageBuf {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	set := func(x, y int, c RGBA) {
		i := img.PixOffset(x, y)
		img.Pix[i+0] = uint8(c.R * 255)
		img.Pix[i+1] = uint8(c.G * 255)
		img.Pix[i+2] = uint8(c.B * 255)
		img.Pix[i+3] = 255
	}
	set(0, 0, RGBA{R: 1, A: 1})
	set(1, 0, RGBA{G: 1, A: 1})
	set(0, 1, RGBA{B: 1, A: 1})
	set(1, 1, RGBA{R: 1, G: 1, A: 1})
	return ImageBufFromImage(img)
}

func TestImagePatternFollowsCTM(t *testing.T) {
	const w, h = 220, 40
	src := gradientCTMChecker()

	base := gradientCTMContext(t, w, h)
	base.SetFillPattern(NewImagePattern(src, 0, 0, 2, 2))
	gradientCTMFill(t, base, 0, 0, 8, 8)

	moved := gradientCTMContext(t, w, h)
	moved.Translate(100, 0)
	moved.SetFillPattern(NewImagePattern(src, 0, 0, 2, 2))
	gradientCTMFill(t, moved, 0, 0, 8, 8)

	for _, p := range [][2]int{{1, 1}, {2, 1}, {3, 3}, {6, 6}} {
		gradientCTMEqual(t, gradientCTMPixel(t, moved, p[0]+100, p[1]), gradientCTMPixel(t, base, p[0], p[1]),
			gradientCTMTol, "translated image pattern")
	}
	if got := gradientCTMPixel(t, moved, 50, 4); got.A != 0 {
		t.Errorf("pixel left of translated pattern = %+v, want transparent", got)
	}
}

func TestMatrixSimilarityScale(t *testing.T) {
	cases := []struct {
		name  string
		m     Matrix
		scale float64
		ok    bool
	}{
		{"identity", Identity(), 1, true},
		{"translate", Translate(10, -4), 1, true},
		{"uniform scale", Scale(3, 3), 3, true},
		{"rotate", Rotate(math.Pi / 3), 1, true},
		{"rotate+scale", Rotate(0.7).Multiply(Scale(2, 2)), 2, true},
		{"mirror", Scale(-1, 1), 1, true},
		{"non-uniform scale", Scale(2, 3), 0, false},
		{"skew", Matrix{A: 1, B: 0.5, D: 0, E: 1}, 0, false},
		{"degenerate", Matrix{}, 0, false},
	}
	for _, tc := range cases {
		scale, ok := tc.m.SimilarityScale()
		if ok != tc.ok || (ok && math.Abs(scale-tc.scale) > 1e-9) {
			t.Errorf("%s: SimilarityScale() = (%v,%v), want (%v,%v)", tc.name, scale, ok, tc.scale, tc.ok)
		}
	}
}

// TestBrushFieldSeed_DistinctWrappers pins the GPU field-cache key: different
// wrapped brushes must not share a cached ColorAt field.
func TestBrushFieldSeed_DistinctWrappers(t *testing.T) {
	if BrushFieldSeed(SolidBrush{Color: White}) != 0 {
		t.Errorf("solid brush needs no wrapper seed")
	}
	g := gradientCTMLinear()
	a := deviceSpaceBrush{base: g, inv: Translate(1, 0).Invert()}
	a.seed = mixBrushSeedMatrix(brushContentSeed(g), a.inv)
	b := deviceSpaceBrush{base: g, inv: Translate(2, 0).Invert()}
	b.seed = mixBrushSeedMatrix(brushContentSeed(g), b.inv)
	if sa, sb := BrushFieldSeed(a), BrushFieldSeed(b); sa == 0 || sa == sb {
		t.Errorf("device-space seeds collide: %d vs %d", sa, sb)
	}

	op := opacityMulBrush{base: g, mul: 0.5}
	if BrushFieldSeed(op) == 0 {
		t.Errorf("F1 wrapper needs a seed")
	}
	if BrushFieldSeed(opacityMulBrush{base: g, mul: 0.5}) != BrushFieldSeed(op) {
		t.Errorf("F1 seed must be stable for identical wrappers")
	}
	if BrushFieldSeed(opacityMulBrush{base: g, mul: 0.25}) == BrushFieldSeed(op) {
		t.Errorf("F1 seeds must track the opacity multiplier")
	}
}
