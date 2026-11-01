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
	"math"
	"testing"
)

func m4Near(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*math.Max(1, math.Abs(a)+math.Abs(b))
}

func TestMatrix4IdentityAndLift(t *testing.T) {
	id := Matrix4Identity()
	if !id.IsIdentity() {
		t.Fatal("Matrix4Identity reported non-identity")
	}
	m := Matrix{A: 2, B: -0.5, C: 11, D: 0.25, E: 3, F: -7}
	lifted := Matrix4FromMatrix(m)
	if got, ok := lifted.Affine(); !ok || got != m {
		t.Fatalf("Affine() = %+v ok=%v, want %+v", got, ok, m)
	}
	p := Pt(5, -3)
	got, w, ok := lifted.ProjectPoint(p.X, p.Y, 0)
	if !ok || w != 1 {
		t.Fatalf("lifted affine ProjectPoint ok=%v w=%v, want true/1", ok, w)
	}
	want := m.TransformPoint(p)
	if !m4Near(got.X, want.X) || !m4Near(got.Y, want.Y) {
		t.Fatalf("lifted affine screen = %+v, want %+v", got, want)
	}
	zv := Matrix4FromMatrix(m).TransformVec4([4]float64{1, 0, 4, 0})
	if zv[2] != 4 || zv[3] != 0 {
		t.Fatalf("z/w must pass through an affine lift, got %+v", zv)
	}
}

func TestMatrix4AffineRejectsProjective(t *testing.T) {
	m, ok := ProjectorMatrix4(100, Pt(300, 200), Pt(40, -10))
	if !ok {
		t.Fatal("ProjectorMatrix4 valid input rejected")
	}
	if _, ok := m.Affine(); ok {
		t.Fatal("projective matrix must not report an affine 2x3 block")
	}
}

func TestMatrix4MulMatches2x3ForAffineLifts(t *testing.T) {
	a := Matrix{A: 1.5, B: -0.25, C: 60, D: 0.1, E: 2.5, F: -12}
	b := Matrix{A: 0.75, B: 1.25, C: -5, D: -0.5, E: 1.1, F: 33}
	got, ok := Matrix4FromMatrix(a).Mul(Matrix4FromMatrix(b)).Affine()
	if !ok {
		t.Fatal("affine*affine lift lost its affine block")
	}
	want := a.Multiply(b)
	if got != want {
		t.Fatalf("lifted multiply = %+v, want %+v", got, want)
	}
}

func TestMatrix4InvertRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		m    Matrix4
	}{
		{"affine_lift", Matrix4FromMatrix(Matrix{A: 2, B: 0.5, C: 9, D: -0.25, E: 1.75, F: 4})},
		{"projector", func() Matrix4 {
			m, ok := ProjectorMatrix4(100, Pt(320, 240), Pt(50, -20))
			if !ok {
				t.Fatal("ProjectorMatrix4 rejected")
			}
			return m
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv, ok := tc.m.Invert()
			if !ok {
				t.Fatal("Invert rejected an invertible matrix")
			}
			id := tc.m.Mul(inv)
			for i := 0; i < 16; i++ {
				want := 0.0
				if i%5 == 0 {
					want = 1.0
				}
				if math.Abs(id.At(i/4, i%4)-want) > 1e-6 {
					t.Fatalf("M*M-1[%d][%d] = %v, want %v", i/4, i%4, id.At(i/4, i%4), want)
				}
			}
		})
	}
}

func TestMatrix4InvertSingularAndNonFinite(t *testing.T) {
	var zero Matrix4
	if _, ok := zero.Invert(); ok {
		t.Fatal("zero matrix inverted")
	}
	m := Matrix4Identity()
	m.m[3] = math.Inf(1)
	if _, ok := m.Invert(); ok {
		t.Fatal("matrix with Inf inverted")
	}
	singular := Matrix4{}
	singular.m[0], singular.m[5] = 1, 1
	// Row 3 duplicates row 2 through the w column to force det 0.
	singular.m[14], singular.m[15] = 0, 0
	if _, ok := singular.Invert(); ok {
		t.Fatal("matrix with zero w row inverted")
	}
}

// TestMatrix4ProjectorMatchesProjectorMath pins ProjectPoint against the
// engine/camera.Projector closed form:
// screen = center + (world - camera) * focal / (focal + depth), weight = focal + depth.
func TestMatrix4ProjectorMatchesProjectorMath(t *testing.T) {
	const focal = 100.0
	center := Pt(300, 200)
	camera := Pt(40, -10)
	m, ok := ProjectorMatrix4(focal, center, camera)
	if !ok {
		t.Fatal("ProjectorMatrix4 rejected valid inputs")
	}
	points := []struct {
		world Point
		depth float64
	}{
		{Pt(40, -10), 0},      // camera center: lands on center at any depth 0
		{Pt(140, -60), 37.5},  // arbitrary
		{Pt(-250, 512.25), 4}, // negative x, large y
		{Pt(77, 77), 999},     // far depth
	}
	for _, tc := range points {
		s, w, ok := m.ProjectPoint(tc.world.X, tc.world.Y, tc.depth)
		if !ok {
			t.Fatalf("ProjectPoint(%v, d=%v) rejected", tc.world, tc.depth)
		}
		scale := focal / (focal + tc.depth)
		want := Pt(center.X+(tc.world.X-camera.X)*scale, center.Y+(tc.world.Y-camera.Y)*scale)
		if !m4Near(s.X, want.X) || !m4Near(s.Y, want.Y) {
			t.Fatalf("screen = %v, want %v (d=%v)", s, want, tc.depth)
		}
		if !m4Near(w, focal+tc.depth) {
			t.Fatalf("weight = %v, want %v", w, focal+tc.depth)
		}
	}
}

func TestMatrix4ProjectorRejectsBadSetup(t *testing.T) {
	if _, ok := ProjectorMatrix4(0, Pt(0, 0), Pt(0, 0)); ok {
		t.Fatal("focal 0 accepted")
	}
	if _, ok := ProjectorMatrix4(-5, Pt(0, 0), Pt(0, 0)); ok {
		t.Fatal("negative focal accepted")
	}
	if _, ok := ProjectorMatrix4(math.NaN(), Pt(0, 0), Pt(0, 0)); ok {
		t.Fatal("NaN focal accepted")
	}
	if _, ok := ProjectorMatrix4(100, Pt(math.Inf(1), 0), Pt(0, 0)); ok {
		t.Fatal("Inf center accepted")
	}
	if _, ok := ProjectorMatrix4(100, Pt(0, 0), Pt(0, math.NaN())); ok {
		t.Fatal("NaN camera accepted")
	}
}

func TestMatrix4ProjectPointBehindEyeRejected(t *testing.T) {
	m, _ := ProjectorMatrix4(100, Pt(0, 0), Pt(0, 0))
	if _, _, ok := m.ProjectPoint(10, 10, -100); ok {
		t.Fatal("W == 0 accepted")
	}
	if _, _, ok := m.ProjectPoint(10, 10, -150); ok {
		t.Fatal("W < 0 accepted")
	}
	if _, _, ok := m.ProjectPoint(10, 10, math.NaN()); ok {
		t.Fatal("NaN depth accepted")
	}
	if _, _, ok := m.ProjectPoint(math.Inf(-1), 0, 10); ok {
		t.Fatal("Inf world coordinate accepted")
	}
}

func TestMatrix4ProjectQuadTracesProjectorContract(t *testing.T) {
	m, ok := ProjectorMatrix4(100, Pt(300, 200), Pt(150, 100))
	if !ok {
		t.Fatal("ProjectorMatrix4 rejected")
	}
	// A 200x100 world rect, far edge pushed deeper: the projected quad is a
	// trapezoid whose top edge must be shorter than the bottom edge.
	world := [4]Point{Pt(50, 50), Pt(250, 50), Pt(250, 150), Pt(50, 150)}
	depths := [4]float64{200, 200, 0, 0}
	screens, weights, ok := m.ProjectQuad(world, depths)
	if !ok {
		t.Fatal("ProjectQuad rejected a drawable quad")
	}
	for i := range world {
		w, okOne := func() (float64, bool) {
			_, w, ok := m.ProjectPoint(world[i].X, world[i].Y, depths[i])
			return w, ok
		}()
		if !okOne || !m4Near(weights[i], w) {
			t.Fatalf("weight[%d] = %v, want %v", i, weights[i], w)
		}
	}
	topLen := math.Hypot(screens[1].X-screens[0].X, screens[1].Y-screens[0].Y)
	botLen := math.Hypot(screens[2].X-screens[3].X, screens[2].Y-screens[3].Y)
	if !(topLen < botLen) {
		t.Fatalf("trapezoid not a trapezoid: top=%v bottom=%v", topLen, botLen)
	}
	// One corner behind the eye fails the whole quad.
	depths[2] = -100
	if _, _, ok := m.ProjectQuad(world, depths); ok {
		t.Fatal("quad with a behind-eye corner accepted")
	}
	world[3] = Pt(math.NaN(), 150)
	depths[2] = 0
	if _, _, ok := m.ProjectQuad(world, depths); ok {
		t.Fatal("quad with a NaN corner accepted")
	}
}

// TestMatrix4ProjectiveEdgeRatio pins the defining property of the homogeneous
// weights: at the screen-space midpoint of an edge whose corners have weights
// w0/w1, the perspective-correct uv is uv = w0/(w0+w1), NOT 0.5. This is the
// contract the CPU triSampler and the GPU vertex path must both reproduce.
func TestMatrix4ProjectiveEdgeRatio(t *testing.T) {
	m, ok := ProjectorMatrix4(100, Pt(0, 0), Pt(0, 0))
	if !ok {
		t.Fatal("ProjectorMatrix4 rejected")
	}
	// Edge along y=0: world x=0 at depth 0 (w=100), x=1 at depth 100 (w=200).
	s0, w0, ok := m.ProjectPoint(0, 0, 0)
	if !ok {
		t.Fatal("corner 0 rejected")
	}
	s1, w1, ok := m.ProjectPoint(1, 0, 100)
	if !ok {
		t.Fatal("corner 1 rejected")
	}
	mid := Pt((s0.X+s1.X)/2, 0)
	// Perspective-correct uv at the screen midpoint from the weights:
	// uv = (0.5*(0/w0) + 0.5*(1/w1)) / (0.5/w0 + 0.5/w1) = w0/(w0+w1).
	wantUV := w0 / (w0 + w1)
	if wantUV >= 0.5 {
		t.Fatalf("edge should shrink away from camera: uv=%v", wantUV)
	}
	// And the projected world u=wantUV lands exactly on mid:
	// screen of (wantUV,0,d(u)) where d(u) = 100*u (linear along the edge in
	// depth) — the homography maps the edge linearly in world space.
	su, _, ok := m.ProjectPoint(wantUV, 0, 100*wantUV)
	if !ok {
		t.Fatal("edge interior point rejected")
	}
	if !m4Near(su.X, mid.X) {
		t.Fatalf("projective edge: uv=%v maps to x=%v, want midpoint x=%v", wantUV, su.X, mid.X)
	}
}
