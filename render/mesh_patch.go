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
	"errors"
	"math"
)

// MeshPatchStyle selects the surface basis of one MeshPatch.
//
// The two styles are the two standard colored-patch families:
//   - MeshPatchCoons is Skia's SkCanvas::drawPatch: four cubic edges sharing
//     four corners, 12 control points.
//   - MeshPatchBicubic is the PDF tensor-product patch (shading type 7): a
//     4x4 Bezier control lattice, 16 control points.
type MeshPatchStyle uint8

const (
	// MeshPatchCoons is the 12-control-point Coons patch of
	// SkCanvas::drawPatch (Skia src/utils/SkPatchUtils).
	MeshPatchCoons MeshPatchStyle = iota
	// MeshPatchBicubic is the 16-control-point tensor-product (bicubic)
	// patch of PDF shading type 7.
	MeshPatchBicubic
)

// MeshPatch is one colored Bezier patch.
//
// Points are user-space control points; the current CTM/fold is applied when
// the patch is drawn. Colors are ordered clockwise from the top-left corner:
// Colors[0]=TopLeft, Colors[1]=TopRight, Colors[2]=BottomRight,
// Colors[3]=BottomLeft. The same four roles are used by Skia's drawPatch.
//
// MeshPatchCoons uses exactly 12 points in Skia's order:
//
//	0=TL, 1=top1, 2=top2, 3=TR,
//	4=right1, 5=right2, 6=BR,
//	7=bottom2, 8=bottom1, 9=BL,
//	10=left2, 11=left1
//
// MeshPatchBicubic uses 16 points in row-major order (u across, v down):
//
//	 0  1  2  3
//	 4  5  6  7
//	 8  9 10 11
//	12 13 14 15
//
// The four corners are 0=TL, 3=TR, 12=BL, 15=BR. A patch grid (2x2 or 3x3
// patches) is built by the caller: adjacent patches share the same corner
// positions/colors; Skia has no multi-patch API either.
type MeshPatch struct {
	Style  MeshPatchStyle
	Points []Point // 12 (Coons) or 16 (Bicubic)
	Colors [4]RGBA // TL, TR, BR, BL
}

// ErrMeshPatchShape reports an unknown style or a control-point count that
// does not match the style (12 Coons / 16 Bicubic).
var ErrMeshPatchShape = errors.New("render: mesh patch needs 12 (Coons) or 16 (Bicubic) points")

const (
	meshPatchPartitionSize = 10
	meshPatchMinLOD        = 8
	meshPatchMaxLOD        = 200
	meshPatchMaxVertices   = 10000
	meshPatchMaxIndices    = 60000
)

// meshPatchPointCount returns the required control-point count for style.
func meshPatchPointCount(style MeshPatchStyle) (int, bool) {
	switch style {
	case MeshPatchCoons:
		return 12, true
	case MeshPatchBicubic:
		return 16, true
	default:
		return 0, false
	}
}

// meshPatchLOD mirrors SkPatchUtils::GetLevelOfDetail: the subdivision count
// per axis is the longest device-space boundary cubic's polyline length / 10,
// with a floor of 8. The same 10000-vertex / 60000-index cap as Skia is
// applied so the result always fits Mesh.Indices (uint16).
func meshPatchLOD(patch MeshPatch, m Matrix) (int, int, bool) {
	n, ok := meshPatchPointCount(patch.Style)
	if !ok || len(patch.Points) != n {
		return 0, 0, false
	}
	var top, bottom, left, right [4]Point
	switch patch.Style {
	case MeshPatchCoons:
		top = [4]Point{patch.Points[0], patch.Points[1], patch.Points[2], patch.Points[3]}
		right = [4]Point{patch.Points[3], patch.Points[4], patch.Points[5], patch.Points[6]}
		bottom = [4]Point{patch.Points[9], patch.Points[8], patch.Points[7], patch.Points[6]}
		left = [4]Point{patch.Points[0], patch.Points[11], patch.Points[10], patch.Points[9]}
	case MeshPatchBicubic:
		top = [4]Point{patch.Points[0], patch.Points[1], patch.Points[2], patch.Points[3]}
		right = [4]Point{patch.Points[3], patch.Points[7], patch.Points[11], patch.Points[15]}
		bottom = [4]Point{patch.Points[12], patch.Points[13], patch.Points[14], patch.Points[15]}
		left = [4]Point{patch.Points[0], patch.Points[4], patch.Points[8], patch.Points[12]}
	}

	mapCubic := func(pts [4]Point) (float64, bool) {
		for i, p := range pts {
			if !isFinitePt(p) {
				return 0, false
			}
			pts[i] = m.TransformPoint(p)
			if !isFinitePt(pts[i]) {
				return 0, false
			}
		}
		return meshPatchArcLength(pts), true
	}
	topLen, okTop := mapCubic(top)
	bottomLen, okBottom := mapCubic(bottom)
	leftLen, okLeft := mapCubic(left)
	rightLen, okRight := mapCubic(right)
	if !okTop || !okBottom || !okLeft || !okRight {
		return 0, 0, false
	}
	if !isFiniteF64(topLen) || !isFiniteF64(bottomLen) || !isFiniteF64(leftLen) || !isFiniteF64(rightLen) {
		return 0, 0, false
	}

	lodX := int(math.Max(topLen, bottomLen) / meshPatchPartitionSize)
	lodY := int(math.Max(leftLen, rightLen) / meshPatchPartitionSize)
	if lodX < meshPatchMinLOD {
		lodX = meshPatchMinLOD
	}
	if lodY < meshPatchMinLOD {
		lodY = meshPatchMinLOD
	}
	if (lodX+1)*(lodY+1) > meshPatchMaxVertices || lodX > meshPatchMaxLOD || lodY > meshPatchMaxLOD {
		sum := lodX + lodY
		if sum <= 0 {
			return 0, 0, false
		}
		wx := float64(lodX) / float64(sum)
		wy := float64(lodY) / float64(sum)
		lodX = int(math.Floor(wx * meshPatchMaxLOD))
		lodY = int(math.Floor(wy * meshPatchMaxLOD))
		if lodX < 1 {
			lodX = 1
		}
		if lodY < 1 {
			lodY = 1
		}
	}
	if lodX < 1 || lodY < 1 || lodX*lodY*6 > meshPatchMaxIndices {
		return 0, 0, false
	}
	return lodX, lodY, true
}

// meshPatchArcLength is the Skia approx_arc_length polyline approximation.
func meshPatchArcLength(pts [4]Point) float64 {
	l := 0.0
	for i := 0; i < 3; i++ {
		l += math.Hypot(pts[i+1].X-pts[i].X, pts[i+1].Y-pts[i].Y)
	}
	return l
}

// meshPatchSurfacePoint evaluates the patch surface at (u,v), u/v in [0,1].
// Coons uses SkPatchUtils' s0+s1-s2 form; Bicubic uses the 4x4 tensor
// product of cubic Bernstein basis functions.
func meshPatchSurfacePoint(patch MeshPatch, u, v float64) (Point, bool) {
	n, ok := meshPatchPointCount(patch.Style)
	if !ok || len(patch.Points) != n {
		return Point{}, false
	}
	if math.IsNaN(u) || math.IsNaN(v) || math.IsInf(u, 0) || math.IsInf(v, 0) {
		return Point{}, false
	}
	u = clamp01(u)
	v = clamp01(v)

	var p Point
	switch patch.Style {
	case MeshPatchCoons:
		top := cubicPoint([4]Point{patch.Points[0], patch.Points[1], patch.Points[2], patch.Points[3]}, u)
		bottom := cubicPoint([4]Point{patch.Points[9], patch.Points[8], patch.Points[7], patch.Points[6]}, u)
		left := cubicPoint([4]Point{patch.Points[0], patch.Points[11], patch.Points[10], patch.Points[9]}, v)
		right := cubicPoint([4]Point{patch.Points[3], patch.Points[4], patch.Points[5], patch.Points[6]}, v)
		s0 := lerpPoint(top, bottom, v)
		s1 := lerpPoint(left, right, u)
		s2 := bilerpPoint(patch.Points[0], patch.Points[3], patch.Points[9], patch.Points[6], u, v)
		p = Point{X: s0.X + s1.X - s2.X, Y: s0.Y + s1.Y - s2.Y}
	case MeshPatchBicubic:
		bu := bernstein3(u)
		bv := bernstein3(v)
		for i := 0; i < 4; i++ {
			for j := 0; j < 4; j++ {
				w := bv[i] * bu[j]
				q := patch.Points[i*4+j]
				p.X += w * q.X
				p.Y += w * q.Y
			}
		}
	}
	if !isFinitePt(p) {
		return Point{}, false
	}
	return p, true
}

// tessellateMeshPatch turns one patch into the colored vertex mesh Skia's
// SkDevice::drawPatch feeds to drawVertices. Positions stay in user space.
func tessellateMeshPatch(patch MeshPatch, lodX, lodY int) (Mesh, bool) {
	if lodX < 1 || lodY < 1 || lodX > meshPatchMaxLOD || lodY > meshPatchMaxLOD {
		return Mesh{}, false
	}
	if _, ok := meshPatchPointCount(patch.Style); !ok {
		return Mesh{}, false
	}
	if lodX*lodY*6 > meshPatchMaxIndices {
		return Mesh{}, false
	}
	vertexCount := (lodX + 1) * (lodY + 1)
	if vertexCount > 65536 {
		return Mesh{}, false
	}
	positions := make([]Point, vertexCount)
	colors := make([]RGBA, vertexCount)
	indices := make([]uint16, lodX*lodY*6)
	stride := lodY + 1

	for x := 0; x <= lodX; x++ {
		u := float64(x) / float64(lodX)
		for y := 0; y <= lodY; y++ {
			v := float64(y) / float64(lodY)
			pos, ok := meshPatchSurfacePoint(patch, u, v)
			if !ok {
				return Mesh{}, false
			}
			di := x*stride + y
			positions[di] = pos
			colors[di] = meshPatchColorAt(patch, u, v)
			if x < lodX && y < lodY {
				i := 6 * (x*lodY + y)
				indices[i+0] = uint16(di)
				indices[i+1] = uint16(x*stride + y + 1)
				indices[i+2] = uint16((x+1)*stride + y + 1)
				indices[i+3] = indices[i]
				indices[i+4] = indices[i+2]
				indices[i+5] = uint16((x+1)*stride + y)
			}
		}
	}
	return Mesh{Positions: positions, Colors: colors, Indices: indices}, true
}

// meshPatchColorAt bilerps the four corner colors in premultiplied space,
// matching SkPatchUtils::MakeVertices (colors are converted to premul before
// bilerp and back to straight for drawVertices).
func meshPatchColorAt(patch MeshPatch, u, v float64) RGBA {
	pm := func(c RGBA) RGBA {
		a := clamp01(c.A)
		return RGBA{R: clamp01(c.R) * a, G: clamp01(c.G) * a, B: clamp01(c.B) * a, A: a}
	}
	tl := pm(patch.Colors[0])
	tr := pm(patch.Colors[1])
	br := pm(patch.Colors[2])
	bl := pm(patch.Colors[3])
	top := lerpRGBA(tl, tr, u)
	bottom := lerpRGBA(bl, br, u)
	out := lerpRGBA(top, bottom, v)
	if out.A <= 0 {
		return RGBA{}
	}
	return RGBA{
		R: clamp01(out.R / out.A),
		G: clamp01(out.G / out.A),
		B: clamp01(out.B / out.A),
		A: clamp01(out.A),
	}
}

// DrawMeshPatch draws one colored Bezier patch through the existing colored
// vertex-mesh path (GPU indexed mesh or CPU true Gouraud). Invalid or
// non-finite patches are skipped; use DrawMeshPatchEx to get the reason.
func (c *Context) DrawMeshPatch(patch MeshPatch) {
	_, _ = c.DrawMeshPatchEx(patch, VertDrawOptions{})
}

// DrawMeshPatchEx validates then draws one patch, returning the same
// degradation/result shape as DrawMeshEx.
func (c *Context) DrawMeshPatchEx(patch MeshPatch, _ VertDrawOptions) (VertDrawResult, error) {
	if c == nil {
		return VertDrawResult{Skipped: true}, nil
	}
	want, ok := meshPatchPointCount(patch.Style)
	if !ok || len(patch.Points) != want {
		return VertDrawResult{}, ErrMeshPatchShape
	}
	for _, p := range patch.Points {
		if !isFinitePt(p) {
			return VertDrawResult{}, ErrVertsNonFinite
		}
	}
	for _, col := range patch.Colors {
		if !isFiniteRGBA(col) {
			return VertDrawResult{}, ErrVertsNonFinite
		}
	}
	lodX, lodY, ok := meshPatchLOD(patch, c.totalMatrix())
	if !ok {
		return VertDrawResult{Skipped: true}, nil
	}
	mesh, ok := tessellateMeshPatch(patch, lodX, lodY)
	if !ok {
		return VertDrawResult{Skipped: true}, nil
	}
	useGPU := !CPUOnlyMode() && c.gpuCtxOps() != nil
	c.DrawMesh(mesh)
	return VertDrawResult{
		Degraded:  !useGPU,
		Reason:    mapDegradedReason(!useGPU),
		Triangles: len(mesh.Indices) / 3,
	}, nil
}

func cubicPoint(p [4]Point, t float64) Point {
	b := bernstein3(t)
	return Point{
		X: b[0]*p[0].X + b[1]*p[1].X + b[2]*p[2].X + b[3]*p[3].X,
		Y: b[0]*p[0].Y + b[1]*p[1].Y + b[2]*p[2].Y + b[3]*p[3].Y,
	}
}

func bernstein3(t float64) [4]float64 {
	mt := 1 - t
	return [4]float64{mt * mt * mt, 3 * t * mt * mt, 3 * t * t * mt, t * t * t}
}

func lerpPoint(a, b Point, t float64) Point {
	return Point{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t}
}

func lerpRGBA(a, b RGBA, t float64) RGBA {
	return RGBA{
		R: a.R + (b.R-a.R)*t,
		G: a.G + (b.G-a.G)*t,
		B: a.B + (b.B-a.B)*t,
		A: a.A + (b.A-a.A)*t,
	}
}

// bilerpPoint takes the four corners in TL, TR, BL, BR order, which is the
// natural pair layout for the Coons s2 term.
func bilerpPoint(tl, tr, bl, br Point, u, v float64) Point {
	top := lerpPoint(tl, tr, u)
	bottom := lerpPoint(bl, br, u)
	return lerpPoint(top, bottom, v)
}

func isFiniteF64(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
