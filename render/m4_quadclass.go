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

// String 便于日志与单测打印。
func (k QuadKind) String() string {
	switch k {
	case QuadOK:
		return "ok"
	case QuadDegenerate:
		return "degenerate"
	case QuadBowTie:
		return "bowtie"
	case QuadNonFinite:
		return "nonfinite"
	default:
		return "unknown"
	}
}

// QuadDrawOptions 是 DrawImageQuadEx 的选项。
type QuadDrawOptions struct {
	// Interpolation 采样方式，零值按 Bilinear。
	Interpolation InterpolationMode
	// Opacity 0..1，零值按 1（沿用 DrawImageEx 惯例：0 表默认不透明）。
	Opacity float64
	// BlendMode 混合方式，R1 只支持 Normal，其余显式报错。
	BlendMode BlendMode
}

// R1 哨兵错：调用方用 errors.Is 判定，不许静默画错。
var (
	// ErrQuadDegenerate 退化四边形（零面积）。
	ErrQuadDegenerate = errors.New("render: degenerate quad (zero area)")
	// ErrQuadBowTie 自交四边形。
	ErrQuadBowTie = errors.New("render: self-intersecting quad")
	// ErrQuadNonFinite 非有限角点。
	ErrQuadNonFinite = errors.New("render: non-finite quad corner")
	// ErrQuadUnsupportedBlend R1 不支持的混合模式。
	ErrQuadUnsupportedBlend = errors.New("render: unsupported quad blend mode (R1 only Normal)")
	// ErrQuadUnsupportedInterp 不支持的采样方式。
	ErrQuadUnsupportedInterp = errors.New("render: unsupported quad interpolation")
)

// SetDither enables ordered dithering of soft fills after GPU resolve (P.09).
// When enabled, subsequent Image()/FlushGPU results for this context receive a
// Bayer 4x4 ordered dither on the pixmap (reduces gradient banding).
// quadHasNonFinite reports any non-finite corner.
func quadHasNonFinite(q [4]Point) bool {
	for i := 0; i < 4; i++ {
		if !isFinitePt(q[i]) {
			return true
		}
	}
	return false
}

// quadAreaSum is the twin-split area (GPU TL-TR-BL + TR-BR-BL), degenerate near zero.
// quadAreaSum is the twin-split area (GPU TL-TR-BL + TR-BR-BL), degenerate near zero.
func quadAreaSum(q [4]Point) float64 {
	return math.Abs(triArea(q[0], q[1], q[3])) + math.Abs(triArea(q[1], q[2], q[3]))
}

// quadBounds returns device-space AABB for damage and raster clamping.
// quadBounds returns device-space AABB for damage and raster clamping.
func quadBounds(dev [4]Point) (minX, minY, maxX, maxY float64) {
	minX = math.Min(math.Min(dev[0].X, dev[1].X), math.Min(dev[2].X, dev[3].X))
	minY = math.Min(math.Min(dev[0].Y, dev[1].Y), math.Min(dev[2].Y, dev[3].Y))
	maxX = math.Max(math.Max(dev[0].X, dev[1].X), math.Max(dev[2].X, dev[3].X))
	maxY = math.Max(math.Max(dev[0].Y, dev[1].Y), math.Max(dev[2].Y, dev[3].Y))
	return minX, minY, maxX, maxY
}

func quadKindErr(k QuadKind) error {
	switch k {
	case QuadDegenerate:
		return ErrQuadDegenerate
	case QuadBowTie:
		return ErrQuadBowTie
	case QuadNonFinite:
		return ErrQuadNonFinite
	default:
		return nil
	}
}

// ClassifyQuad classifies user-space corners without drawing.
// ClassifyQuad classifies user-space corners without drawing.
func ClassifyQuad(corners [4]Point) QuadKind {
	if quadHasNonFinite(corners) {
		return QuadNonFinite
	}
	if quadBowTie(corners) {
		return QuadBowTie
	}
	if quadAreaSum(corners) < 1e-9 {
		return QuadDegenerate
	}
	return QuadOK
}

func triArea(a, b, c Point) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (c.X-a.X)*(b.Y-a.Y)
}

// quadBowTie reports edge crossing of 0-1 vs 2-3 or 1-2 vs 3-0.
// quadBowTie reports edge crossing of 0-1 vs 2-3 or 1-2 vs 3-0.
func quadBowTie(q [4]Point) bool {
	if segsCross(q[0], q[1], q[2], q[3]) {
		return true
	}
	if segsCross(q[1], q[2], q[3], q[0]) {
		return true
	}
	return false
}

func orient(a, b, c Point) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

func segsCross(a, b, c, d Point) bool {
	o1 := orient(a, b, c)
	o2 := orient(a, b, d)
	o3 := orient(c, d, a)
	o4 := orient(c, d, b)
	// Shared endpoints do not count as crossing.
	if (a == c) || (a == d) || (b == c) || (b == d) {
		return false
	}
	return (o1*o2 < 0) && (o3*o4 < 0)
}
