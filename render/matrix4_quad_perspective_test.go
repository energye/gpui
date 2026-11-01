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
	"bytes"
	"errors"
	"image"
	"math"
	"testing"
)

// perspQuadCase builds the canonical P3.5 batch 3 projective quad: a world
// rect whose left corners sit at depth 100 and right corners at depth 0, run
// through the Projector-aligned matrix (focal 100, center/camera origin).
// The projected screens form an axis-aligned device rectangle
// (TL(-80,-60) TR(160,-60) BR(160,60) BL(-80,60)) with weights 200/100/100/200
// — the screen shape stays rectangular while the UV mapping is a genuine
// homography. Closed-form oracle for any device pixel: perspUV below.
func perspQuadCase(t *testing.T) (corners [4]Point, weights [4]float64) {
	t.Helper()
	m, ok := ProjectorMatrix4(100, Pt(0, 0), Pt(0, 0))
	if !ok {
		t.Fatal("ProjectorMatrix4 rejected canonical setup")
	}
	world := [4]Point{Pt(-160, -120), Pt(160, -60), Pt(160, 60), Pt(-160, 120)}
	depths := [4]float64{100, 0, 0, 100}
	screens, ws, ok := m.ProjectQuad(world, depths)
	if !ok {
		t.Fatal("ProjectQuad rejected canonical depths")
	}
	want := [4]Point{Pt(-80, -60), Pt(160, -60), Pt(160, 60), Pt(-80, 60)}
	for i := range want {
		if math.Abs(screens[i].X-want[i].X) > 1e-9 || math.Abs(screens[i].Y-want[i].Y) > 1e-9 {
			t.Fatalf("screen[%d] = %v, want %v", i, screens[i], want[i])
		}
	}
	if ws != [4]float64{200, 100, 100, 200} {
		t.Fatalf("weights = %v, want [200 100 100 200]", ws)
	}
	return screens, ws
}

// devQuad corners translated to device pixels so probes land on integer
// centers: TL(0.5,0.5) TR(160.5,0.5) BR(160.5,120.5) BL(0.5,120.5).
func devQuad(corners [4]Point) [4]Point {
	var out [4]Point
	for i, p := range corners {
		out[i] = Pt(p.X+80.5, p.Y+60.5)
	}
	return out
}

// bandImage builds a width×1 straight RGBA strip: u < boundary/width samples
// red, the rest blue, fully opaque. Nearest sampling: pixel = floor(u*width).
func bandImage(t *testing.T, width, boundary int) *ImageBuf {
	t.Helper()
	img, err := NewImageBuf(width, 1, FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	for x := 0; x < width; x++ {
		var r, g, b uint8 = 255, 0, 0
		if x >= boundary {
			r, g, b = 0, 0, 255
		}
		if err := img.SetRGBA(x, 0, r, g, b, 255); err != nil {
			t.Fatalf("SetRGBA: %v", err)
		}
	}
	return img
}

// perspUV is the independent oracle: planar screen barycentrics λ on the GPU
// twin split (TL,TR,BL / TR,BR,BL), then projective uv = Σ λᵢ·uvᵢ/wᵢ / Σ λᵢ/wᵢ
// — the same formula the rasterizer (CPU triSampler / clip-space w hardware)
// must reproduce. Derived straight from the homogeneous-edge-ratio property,
// not from the sampler implementation.
func perspUV(px, py float64, corners [4]Point, weights [4]float64) (u, v float64, inQuad bool) {
	tris := [][3]int{{0, 1, 3}, {1, 2, 3}}
	uvs := [][3][2]float64{
		{{0, 0}, {1, 0}, {0, 1}},
		{{1, 0}, {1, 1}, {0, 1}},
	}
	for ti, tri := range tris {
		p0, p1, p2 := corners[tri[0]], corners[tri[1]], corners[tri[2]]
		den := (p1.Y-p2.Y)*(p0.X-p2.X) + (p2.X-p1.X)*(p0.Y-p2.Y)
		if math.Abs(den) < 1e-12 {
			continue
		}
		dx, dy := px-p2.X, py-p2.Y
		l0 := ((p1.Y-p2.Y)*dx + (p2.X-p1.X)*dy) / den
		l1 := ((p2.Y-p0.Y)*dx + (p0.X-p2.X)*dy) / den
		l2 := 1 - l0 - l1
		const eps = -1e-9
		if l0 < eps || l1 < eps || l2 < eps {
			continue
		}
		ls := [3]float64{
			l0 / weights[tri[0]],
			l1 / weights[tri[1]],
			l2 / weights[tri[2]],
		}
		sum := ls[0] + ls[1] + ls[2]
		if sum == 0 {
			continue
		}
		uv := uvs[ti]
		return (ls[0]*uv[0][0] + ls[1]*uv[1][0] + ls[2]*uv[2][0]) / sum,
			(ls[0]*uv[0][1] + ls[1]*uv[1][1] + ls[2]*uv[2][1]) / sum, true
	}
	return 0, 0, false
}

// TestQuadPerspectiveWeightsValidation pins the sentinel behavior: invalid
// weights are rejected without touching pixels, affine mode ignores Weights
// entirely, and bicubic has no perspective pipeline.
func TestQuadPerspectiveWeightsValidation(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	img := bandImage(t, 20, 16)
	defer img.Dispose()
	corners, weights := perspQuadCase(t)

	dc := NewContext(161, 121)
	defer dc.Close()
	dc.ClearWithColor(White)
	dc.Translate(80.5, 60.5)

	for _, w := range [][4]float64{
		{0, 1, 1, 1},
		{-1, 1, 1, 1},
		{1, 1, 1, 0},
		{1, 1, 1, math.NaN()},
		{1, 1, 1, math.Inf(1)},
	} {
		if err := dc.DrawImageQuad(img, corners, QuadDrawOptions{
			Perspective: true, Weights: w, Interpolation: InterpNearest, Opacity: 1,
		}); !errors.Is(err, ErrQuadBadWeights) {
			t.Fatalf("weights %v err = %v, want ErrQuadBadWeights", w, err)
		}
	}
	// Rejected draws leave the canvas untouched.
	if c := dc.pixmap.GetPixel(120, 5); c.R < 0.99 && c.B < 0.99 {
		t.Fatalf("invalid-weight draw polluted the canvas: %+v", c)
	}
	// Bicubic + perspective: no pipeline variant, sentinel error.
	if err := dc.DrawImageQuad(img, corners, QuadDrawOptions{
		Perspective: true, Weights: weights, Interpolation: InterpBicubic,
	}); !errors.Is(err, ErrQuadPerspectiveBicubic) {
		t.Fatalf("perspective bicubic err = %v, want ErrQuadPerspectiveBicubic", err)
	}
	// Affine mode ignores Weights entirely (no validation, draws normally).
	if err := dc.DrawImageQuad(img, corners, QuadDrawOptions{
		Perspective: false, Weights: [4]float64{-5, 0, math.NaN(), math.Inf(1)},
	}); err != nil {
		t.Fatalf("affine draw with junk weights: %v", err)
	}
}

// TestQuadPerspectiveBandPixels is the core pixel pin: every probe follows the
// closed-form homography, NOT the affine planar interpolation. The
// discriminating pixel (120,5): projective u = 6/7 samples blue while the
// affine twin would give u = 0.75 and sample red — dropping the 1/w terms
// flips it (negative-control target).
func TestQuadPerspectiveBandPixels(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	img := bandImage(t, 20, 16) // u < 0.8 red, u >= 0.8 blue
	defer img.Dispose()
	userCorners, weights := perspQuadCase(t)
	devCorners := devQuad(userCorners)

	dc := NewContext(241, 121)
	defer dc.Close()
	dc.ClearWithColor(White)
	if err := dc.DrawImageQuad(img, devCorners, QuadDrawOptions{
		Perspective: true, Weights: weights, Interpolation: InterpNearest, Opacity: 1,
	}); err != nil {
		t.Fatalf("DrawImageQuad perspective: %v", err)
	}
	_ = dc.FlushGPU()

	// Device corners span x[0.5,240.5] y[0.5,120.5]; the discriminating probe
	// (180,5): projective u = 6/7 (blue) vs affine u = 0.75 (red).
	probes := []struct{ px, py int }{
		{180, 5}, {60, 5}, {120, 60}, {180, 90}, {10, 60},
	}
	for _, p := range probes {
		cx := float64(p.px) + 0.5
		cy := float64(p.py) + 0.5
		u, v, in := perspUV(cx, cy, devCorners, weights)
		if !in {
			t.Fatalf("probe %v outside quad per oracle", p)
		}
		wantR, wantB := 1.0, 0.0
		if u >= 0.8 {
			wantR, wantB = 0.0, 1.0
		}
		got := dc.pixmap.GetPixel(p.px, p.py)
		_ = v
		if nearF(got.R, wantR) && nearF(got.B, wantB) && nearF(got.A, 1) {
			continue
		}
		t.Fatalf("probe (%d,%d) oracle u=%v want rgb=(%v,0,%v) got %+v",
			p.px, p.py, u, wantR, wantB, got)
	}

	// Sanity: the oracle really is a homography — the discriminating probe
	// sits between the projective and affine u values.
	if u, _, _ := perspUV(180.5, 5.5, devCorners, weights); u < 0.8 {
		t.Fatalf("oracle lost the homography: u = %v, want >= 0.8", u)
	}
	if aff := uAffine(180.5, 5.5, devCorners); aff >= 0.8 {
		t.Fatalf("affine twin at the same pixel is also blue (%v): pin no longer discriminates", aff)
	}
}

// uAffine computes the plain planar barycentric u (the uncorrected value) at
// a device pixel of the canonical rectangle case.
func uAffine(px, py float64, corners [4]Point) float64 {
	// Canonical case is axis-aligned, but compute generally: reuse the
	// oracle with all weights equal.
	even := [4]float64{1, 1, 1, 1}
	u, _, _ := perspUV(px, py, corners, even)
	return u
}

func nearF(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= 1.5/255.0
}

// TestQuadPerspectiveEqualWeightsMatchAffine pins the degenerate case: equal
// weights reduce the rational interpolation to the affine planar one,
// pixel-for-pixel on the CPU path.
func TestQuadPerspectiveEqualWeightsMatchAffine(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	img := bandImage(t, 20, 16)
	defer img.Dispose()
	userCorners, _ := perspQuadCase(t)
	devCorners := devQuad(userCorners)

	draw := func(persp bool, w [4]float64) []byte {
		dc := NewContext(161, 121)
		defer dc.Close()
		dc.ClearWithColor(White)
		if err := dc.DrawImageQuad(img, devCorners, QuadDrawOptions{
			Perspective: persp, Weights: w, Interpolation: InterpBilinear, Opacity: 1,
		}); err != nil {
			t.Fatalf("draw persp=%v: %v", persp, err)
		}
		_ = dc.FlushGPU()
		return append([]byte(nil), dc.pixmap.Data()...)
	}
	affine := draw(false, [4]float64{})
	even := draw(true, [4]float64{200, 200, 200, 200})
	if !bytes.Equal(affine, even) {
		t.Fatal("equal weights must reduce to the affine twin bit-for-bit")
	}
}

// TestQuadPerspectiveDamageBounds pins that the perspective path tracks the
// same device-space AABB as the affine path (the corners are shared; only the
// UV interpolation differs).
func TestQuadPerspectiveDamageBounds(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	img := bandImage(t, 20, 16)
	defer img.Dispose()
	userCorners, weights := perspQuadCase(t)
	devCorners := devQuad(userCorners)

	damageAfter := func(persp bool) image.Rectangle {
		dc := NewContext(322, 242)
		defer dc.Close()
		dc.SetDamageTracking(true)
		dc.ResetFrameDamage()
		if err := dc.DrawImageQuad(img, devCorners, QuadDrawOptions{
			Perspective: persp, Weights: weights, Interpolation: InterpNearest, Opacity: 1,
		}); err != nil {
			t.Fatalf("draw persp=%v: %v", persp, err)
		}
		return dc.FrameDamageUnion()
	}
	perspUnion := damageAfter(true)
	affineUnion := damageAfter(false)
	if perspUnion != affineUnion {
		t.Fatalf("damage union differs: persp=%v affine=%v", perspUnion, affineUnion)
	}
	want := image.Rect(0, 0, 241, 121)
	if perspUnion != want {
		t.Fatalf("damage union = %v, want %v", perspUnion, want)
	}
}
