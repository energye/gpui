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

import "math"

// Matrix4 is a 4x4 matrix in row-major order. It is the render-layer analogue
// of SkM44: the 2x3 affine Matrix stays untouched for everyday transforms, and
// Matrix4 exists to carry a real homogeneous w so perspective-correct image
// quads (P3.5 batch 3) have a single math source shared by the CPU and GPU.
//
// m[i*4+j] is row i, column j. Transforms act on column vectors (x, y, z, w)
// read right-to-left: result = m * v.
type Matrix4 struct {
	m [16]float64
}

// Matrix4Identity returns the 4x4 identity matrix.
func Matrix4Identity() Matrix4 {
	var r Matrix4
	r.m[0] = 1
	r.m[5] = 1
	r.m[10] = 1
	r.m[15] = 1
	return r
}

// Matrix4FromMatrix lifts the 2x3 affine matrix into the 4x4 space
// (translation rides the w column: x' = A·(x,y) + t·w; z and w pass through).
// The inverse of Affine.
func Matrix4FromMatrix(m Matrix) Matrix4 {
	var r Matrix4
	r.m[0] = m.A
	r.m[1] = m.B
	r.m[3] = m.C
	r.m[4] = m.D
	r.m[5] = m.E
	r.m[7] = m.F
	r.m[10] = 1
	r.m[15] = 1
	return r
}

// Matrix4Homography builds the 2D ground-plane homography: the affine part m
// maps (x,y) positionally while the homogeneous row (wx, wy, w1) makes the
// perspective divisor vary linearly across the plane —
// screen = m·(x,y) / (wx·x + wy·y + w1)（W 行在行主序布局里是 m[12..15]，见
// ProjectorMatrix4 的 W ← d + focal 同位）。This is the projection-layer form of
// 「按深度缩放」 for flat-world 2.5D content (P3.7 batch 1d): a ground plane
// seen from above is exactly this map, and w > 0 everywhere on the drawable
// half-plane. 非有限分量不装坏矩阵（fail-closed 同 ProjectorMatrix4）。
func Matrix4Homography(m Matrix, wx, wy, w1 float64) Matrix4 {
	r := Matrix4FromMatrix(m)
	if wx != wx || wy != wy || w1 != w1 { // NaN guard
		return r
	}
	r.m[12] = wx
	r.m[13] = wy
	r.m[15] = w1
	return r
}

// At returns the raw element at row, col (0-based). Exposed so the GPU vertex
// path can read the same numbers the CPU uses; prefer the typed methods for
// ordinary work.
func (m Matrix4) At(row, col int) float64 {
	if row < 0 || row > 3 || col < 0 || col > 3 {
		return 0
	}
	return m.m[row*4+col]
}

// Mul multiplies two matrices (m * other).
func (m Matrix4) Mul(other Matrix4) Matrix4 {
	var r Matrix4
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			sum := 0.0
			for k := 0; k < 4; k++ {
				sum += m.m[i*4+k] * other.m[k*4+j]
			}
			r.m[i*4+j] = sum
		}
	}
	return r
}

// TransformVec4 applies the matrix to a homogeneous vector.
func (m Matrix4) TransformVec4(v [4]float64) [4]float64 {
	var r [4]float64
	for i := 0; i < 4; i++ {
		r[i] = m.m[i*4]*v[0] + m.m[i*4+1]*v[1] + m.m[i*4+2]*v[2] + m.m[i*4+3]*v[3]
	}
	return r
}

// IsIdentity reports whether m is exactly the 4x4 identity.
func (m Matrix4) IsIdentity() bool {
	return m == Matrix4Identity()
}

// Invert returns the inverse matrix and reports whether it is invertible.
// It uses the general 4x4 adjugate/cofactor method; a near-zero determinant or
// any non-finite input returns the identity with ok=false, so callers can fall
// back instead of propagating NaN through the draw path.
func (m Matrix4) Invert() (Matrix4, bool) {
	a := m.m
	// 2x2 sub-determinants (fast 4x4 inverse, same layout as GLM).
	s0 := a[0]*a[5] - a[1]*a[4]
	s1 := a[0]*a[6] - a[2]*a[4]
	s2 := a[0]*a[7] - a[3]*a[4]
	s3 := a[1]*a[6] - a[2]*a[5]
	s4 := a[1]*a[7] - a[3]*a[5]
	s5 := a[2]*a[7] - a[3]*a[6]
	c0 := a[8]*a[13] - a[9]*a[12]
	c1 := a[8]*a[14] - a[10]*a[12]
	c2 := a[8]*a[15] - a[11]*a[12]
	c3 := a[9]*a[14] - a[10]*a[13]
	c4 := a[9]*a[15] - a[11]*a[13]
	c5 := a[10]*a[15] - a[11]*a[14]

	det := s0*c5 - s1*c4 + s2*c3 + s3*c2 - s4*c1 + s5*c0
	if math.IsNaN(det) || math.IsInf(det, 0) || math.Abs(det) < 1e-12 {
		return Matrix4Identity(), false
	}
	inv := 1.0 / det
	var r Matrix4
	b := &r.m
	b[0] = (a[5]*c5 - a[6]*c4 + a[7]*c3) * inv
	b[1] = (-a[1]*c5 + a[2]*c4 - a[3]*c3) * inv
	b[2] = (a[13]*s5 - a[14]*s4 + a[15]*s3) * inv
	b[3] = (-a[9]*s5 + a[10]*s4 - a[11]*s3) * inv
	b[4] = (-a[4]*c5 + a[6]*c2 - a[7]*c1) * inv
	b[5] = (a[0]*c5 - a[2]*c2 + a[3]*c1) * inv
	b[6] = (-a[12]*s5 + a[14]*s2 - a[15]*s1) * inv
	b[7] = (a[8]*s5 - a[10]*s2 + a[11]*s1) * inv
	b[8] = (a[4]*c4 - a[5]*c2 + a[7]*c0) * inv
	b[9] = (-a[0]*c4 + a[1]*c2 - a[3]*c0) * inv
	b[10] = (a[12]*s4 - a[13]*s2 + a[15]*s0) * inv
	b[11] = (-a[8]*s4 + a[9]*s2 - a[11]*s0) * inv
	b[12] = (-a[4]*c3 + a[5]*c1 - a[6]*c0) * inv
	b[13] = (a[0]*c3 - a[1]*c1 + a[2]*c0) * inv
	b[14] = (-a[12]*s3 + a[13]*s1 - a[14]*s0) * inv
	b[15] = (a[8]*s3 - a[9]*s1 + a[10]*s0) * inv

	for _, v := range b {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return Matrix4Identity(), false
		}
	}
	return r, true
}

// ProjectorMatrix4 builds the homogeneous 4x4 that matches
// engine/camera.Projector: a world point (x, y) at depth d maps to
// center + (world - camera) * focal/(focal+d), with the divisor carried in the
// w row (W = d + focal). Non-positive focal, NaN/Inf inputs, or any non-finite
// result are rejected with ok=false and the identity, mirroring NewProjector.
//
// This is the P3.5 batch 3 "perspective utility" the 2.5D module library feeds
// to the perspective image-quad draw entry.
func ProjectorMatrix4(focal float64, center, camera Point) (Matrix4, bool) {
	if !matrix4Finite(focal) || focal <= 0 {
		return Matrix4Identity(), false
	}
	if !matrix4FinitePoint(center) || !matrix4FinitePoint(camera) {
		return Matrix4Identity(), false
	}
	var r Matrix4
	r.m[0] = focal                         // X <- f*x
	r.m[2] = center.X                      // X <- cx*d
	r.m[3] = focal * (center.X - camera.X) // X <- f*center - f*camera (const)
	r.m[5] = focal                         // Y <- f*y
	r.m[6] = center.Y                      // Y <- cy*d
	r.m[7] = focal * (center.Y - camera.Y)
	r.m[10] = 1     // Z <- d (pass depth through the third coordinate)
	r.m[14] = 1     // W <- d
	r.m[15] = focal // W <- d + focal
	for _, v := range r.m {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return Matrix4Identity(), false
		}
	}
	return r, true
}

// ProjectPoint applies the matrix's projective map to a world point at depth,
// returning the screen point and the homogeneous weight (the corner's real w).
// ok=false when the transformed vector is non-finite or W <= 0 (behind the
// eye), matching Projector.Project.
func (m Matrix4) ProjectPoint(x, y, depth float64) (Point, float64, bool) {
	v := m.TransformVec4([4]float64{x, y, depth, 1})
	w := v[3]
	if !matrix4Finite(w) || w <= 0 {
		return Point{}, 0, false
	}
	pt := Point{X: v[0] / w, Y: v[1] / w}
	if !matrix4FinitePoint(pt) {
		return Point{}, 0, false
	}
	return pt, w, true
}

// ProjectQuad maps four corners plus per-corner depths through the matrix.
// Any undrawable corner fails the whole quad (ok=false) so the caller never
// draws half a trapezoid — the same contract as Projector.ProjectQuad. The
// returned weights are the corners' homogeneous w, which is exactly what the
// perspective image-quad path needs for perspective-correct UV interpolation.
func (m Matrix4) ProjectQuad(corners [4]Point, depths [4]float64) ([4]Point, [4]float64, bool) {
	var screens [4]Point
	var weights [4]float64
	for i := 0; i < 4; i++ {
		if !matrix4FinitePoint(corners[i]) {
			return [4]Point{}, [4]float64{}, false
		}
		s, w, ok := m.ProjectPoint(corners[i].X, corners[i].Y, depths[i])
		if !ok {
			return [4]Point{}, [4]float64{}, false
		}
		screens[i] = s
		weights[i] = w
	}
	return screens, weights, true
}

// Affine returns the 2x3 affine sub-block. ok=false unless the matrix is a
// plain lifted affine (translation in the w column, z and w pass through), so
// a projective map is never silently flattened into a plain transform.
func (m Matrix4) Affine() (Matrix, bool) {
	if m.m[2] != 0 || m.m[6] != 0 ||
		m.m[8] != 0 || m.m[9] != 0 || m.m[10] != 1 || m.m[11] != 0 ||
		m.m[12] != 0 || m.m[13] != 0 || m.m[14] != 0 || m.m[15] != 1 {
		return Identity(), false
	}
	return Matrix{
		A: m.m[0], B: m.m[1], C: m.m[3],
		D: m.m[4], E: m.m[5], F: m.m[7],
	}, true
}

func matrix4Finite(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0)
}

func matrix4FinitePoint(p Point) bool {
	return matrix4Finite(p.X) && matrix4Finite(p.Y)
}
