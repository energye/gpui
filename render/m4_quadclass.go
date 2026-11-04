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

// QuadDrawOptions 是 DrawImageQuad 的选项。
type QuadDrawOptions struct {
	// Interpolation 采样方式，零值按 Bilinear。
	Interpolation InterpolationMode
	// Opacity 0..1，零值按 1（沿用 DrawImageEx 惯例：0 表默认不透明）。
	Opacity float64
	// BlendMode 混合方式，R1 只支持 Normal，其余显式报错。
	BlendMode BlendMode
	// Perspective 打开透视校正贴图（P3.5 批 3）：UV 按 Weights 做 1/w 有理插值，
	// 与 Matrix4.ProjectQuad 返回的齐次权重配对（Skia 透视贴图 / GPU clip-space
	// 真 w 同款语义）。零值保持既有仿射双三角行为逐位不变。
	Perspective bool
	// Mipmap 是缩样档位（P3.6 批 2，与 DrawImageOptions.Mipmap 同语义）：
	// 零值 MipmapNone 保持既有单级路径逐位不变。
	Mipmap MipmapMode
	// Weights 是四角齐次权重 w（TL、TR、BR、BL 顺序，恒 >0 有限）。
	// 同权等价于仿射；梯形（深度不均）下决定 UV 沿边的透视落位。
	// Perspective 为真时逐项校验，非法返回 ErrQuadBadWeights。
	Weights [4]float64
	// Tint 逐顶点染色，与 AtlasSprite.Tint 同口径：零结构体=白不透明=不染色
	// （存量逐位不变），分量乘源纹理素（straight 语义，CPU 与 GPU 同一式）。
	// P3.7 批 1b：透视腿此前写死白，2.5D「按深度投影 + 被灯光染色」两件事互斥。
	Tint RGBA
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
	// ErrQuadBadWeights 透视权重非法（非有限或 <=0）。
	ErrQuadBadWeights = errors.New("render: quad perspective weights must be finite and > 0")
	// ErrQuadPerspectiveBicubic 透视模式不支持 bicubic（采样核基于仿射 UV 步距）。
	ErrQuadPerspectiveBicubic = errors.New("render: perspective quad does not support bicubic interpolation")
	// ErrQuadBadTint 染色分量非有限（NaN/Inf）。零结构体是合法的「不染色」。
	ErrQuadBadTint = errors.New("render: quad tint components must be finite")
	// ErrQuadBehindEye 投影层下任一角落在眼后（w ≤ 0），整 quad 拒绝
	//（P3.7 批 1d；与 Matrix4.ProjectQuad/p35「眼后整块拒绝」同语义）。
	ErrQuadBehindEye = errors.New("render: quad corner behind the eye under projection")
)

// quadTintFinite 报告染色四分量全有限。零结构体（白不透明）合法。
func quadTintFinite(t RGBA) bool {
	for _, v := range [4]float64{t.R, t.G, t.B, t.A} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

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
