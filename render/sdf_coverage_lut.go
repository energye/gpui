//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package render

import (
	"math"
	"sync"
)

// SDF 覆盖线（evidence/SDF/coverage-line.md，2026-11-11 路线 A）：
// GPU 侧 sdf_render.wgsl 的 AA 覆盖从 0.75px smoothstep 换成「解析真值预计算
// 查表」。三组实验结论支撑本设计（同文档 2026-11-11 段）：
//
//  1. 现状 1.5px hermite 带对解析真值 avg≈3.1/max≈18.9（/255）——smoothstep
//     的带宽与形状都与真覆盖分布不匹配。
//  2. 像素中心距离 d 固定时，2D 真覆盖随位置散布 ≈11/255，**对半径无关**
//     ——任何 d→coverage 单变量模型（线性/查表）的 max 误差下限 ≈5.5/255。
//  3. 取每个 d 的真值**中位数**即为单变量模型的最优映射：理论 max≈5.5/255
//     < 8 的验收门；中位数无解析式，必须数值预计算——这就是本表。
//
// 散布对形状/半径无关 ⇒ 一张 1024 项表经距离归一化后圆/椭圆/圆角矩形通吃；
// 线性插值由硬件采样完成。表域 [-1,1]（2px 覆盖带，外侧饱和）。

// sdfCoverageLUTRadius 是预计算的标定半径。实验证明散布对半径无关（3..64
// 的固定 d 散布 10.95..11.20/255），取中等曲率 r=16。
const sdfCoverageLUTRadius = 16.0

// SDFCoverageLUTDomain 是查表域：SDF 距离 [-1,1]（2px 覆盖带），shader 侧
// 以 (dist+1)/2 归一化到 [0,1] 采样，clamp-to-edge 完成外侧饱和。
const SDFCoverageLUTDomain = 2.0

var (
	sdfCoverageLUTOnce sync.Once
	sdfCoverageLUT     [1024]uint8
)

// SDFCoverageLUT1024 returns the precomputed normalized-distance → pixel-
// coverage table (1024 entries, midrange-of-analytic mapping). Each entry is
// the center of the exact 1×1-pixel coverage range over a full circle sweep
// at that signed distance, computed with the row-integration truth
// (exactCirclePixelArea). Midrange minimizes the MAX error (spread/2 ≈ 5.5,
// the single-variable model's floor); the median only minimizes mean error
// and leaves a one-sided tail. Thread-safe; computed once.
func SDFCoverageLUT1024() []uint8 {
	sdfCoverageLUTOnce.Do(func() {
		const (
			sweepSteps = 720 // full-circle angles per entry
			radius     = sdfCoverageLUTRadius
		)
		// Working buffers reused per entry (exactCirclePixelArea is pure).
		vals := make([]float64, sweepSteps)
		// Circle center offset keeps every sampled pixel block positive.
		const cc = sdfCoverageLUTRadius + 2.0
		for i := 0; i < 1024; i++ {
			d := -1.0 + 2.0*float64(i)/1023.0
			// Pixel center sits at signed distance d from the circle edge
			// (radius+d from the circle center), swept over all angles.
			n := 0
			for k := 0; k < sweepSteps; k++ {
				th := 2 * math.Pi * float64(k) / sweepSteps
				pxC := cc + (radius+d)*math.Cos(th)
				pyC := cc + (radius+d)*math.Sin(th)
				vals[n] = exactCirclePixelArea(pxC-0.5, pyC-0.5, cc, cc, radius)
				n++
			}
			// Midrange, not median: the gate minimizes MAX error — the optimal
			// constant for a fixed d is the center of the true-coverage range
			// ((min+max)/2, error ≤ spread/2 ≈ 5.5/255); the median only
			// minimizes mean error and leaves a one-sided tail up to ~9.
			var tMin, tMax float64
			for a := 0; a < n; a++ {
				if a == 0 || vals[a] < tMin {
					tMin = vals[a]
				}
				if a == 0 || vals[a] > tMax {
					tMax = vals[a]
				}
			}
			mid := (tMin + tMax) / 2
			if mid < 0 {
				mid = 0
			}
			if mid > 1 {
				mid = 1
			}
			sdfCoverageLUT[i] = uint8(mid*255.0 + 0.5)
		}
	})
	return sdfCoverageLUT[:]
}

// sdfCoverageLUTSample is the CPU mirror of the shader-side lookup,
// including the hardware linear interpolation between entries (the GPU
// samples a 1024×1 RGBA8 texture with a filtering sampler).
func sdfCoverageLUTSample(d float64) float64 {
	lut := SDFCoverageLUT1024()
	x := (d + 1.0) / 2.0 * 1023.0
	if x < 0 {
		x = 0
	}
	if x > 1023 {
		x = 1023
	}
	i0 := int(x)
	if i0 > 1022 {
		i0 = 1022
	}
	f := x - float64(i0)
	v := float64(lut[i0])*(1-f) + float64(lut[i0+1])*f
	return v / 255.0
}
