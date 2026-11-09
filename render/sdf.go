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
	"sort"
)

// sdfAntialiasWidth controls the smoothstep transition width in pixels.
// A value of 0.7 produces smooth anti-aliasing at standard DPI.
const sdfAntialiasWidth = 0.75 // match GPU sdf_render.wgsl aa_hw

// SDFFilledCircleCoverage computes anti-aliased coverage for a filled circle
// at the 1×1 pixel block whose top-left corner is (px, py).
//
// 批 4f 根修：旧实现是像素中心单点 SDF 采样 + 0.75px smoothstep——切线区
// （弧顶/弧底）的真实解析过渡是 2-3px 长渐变，被固定窄带宽陡化成阶梯
// （对真值边缘 avg≈8/255、max≈19/255）。现实现按行解析积分圆与像素方块
// 的精确相交面积（circular-segment 闭式解），对真值 avg≤0.5/max≤2，
// 与 AnalyticFiller 的解析覆盖同级（调查批 4f，红测
// TestSDFFilledCircleCoverageVsTruth 锁 avg≤2/max≤8）。
//
// Parameters:
//   - px, py: pixel block top-left corner (pixel spans [px,px+1]×[py,py+1])
//   - cx, cy: circle center
//   - radius: circle radius
//
// Returns a coverage value in [0, 1] where 1 means fully inside.
func SDFFilledCircleCoverage(px, py, cx, cy, radius float64) float64 {
	// (px,py) 是像素中心（历史 API 语义）；exactCirclePixelArea 吃方块左上角。
	return exactCirclePixelArea(px-0.5, py-0.5, cx, cy, radius)
}

// sdfWAntideriv is the antiderivative of w(t)=√(r²−t²) on |t|≤r:
// W(t) = (t·√(r²−t²) + r²·asin(t/r)) / 2.
func sdfWAntideriv(t, r float64) float64 {
	t = math.Max(-r, math.Min(r, t))
	return (t*math.Sqrt(r*r-t*t) + r*r*math.Asin(t/r)) / 2
}

// sdfWHalfWidth evaluates w(t)=√(r²−t²); t outside [-r,r] yields 0.
func sdfWHalfWidth(t, r float64) float64 {
	if t <= -r || t >= r {
		return 0
	}
	return math.Sqrt(r*r - t*t)
}

// exactCirclePixelArea integrates the exact intersection area of the 1×1
// pixel block [px,px+1]×[py,py+1] with the circle (cx,cy,r).
func exactCirclePixelArea(px, py, cx, cy, r float64) float64 {
	return exactCircleRectArea(px, py, px+1, py+1, cx, cy, r)
}

// exactCircleRectArea integrates the exact intersection area of the axis-
// aligned rectangle [rx0,rx1]×[ry0,ry1] with the circle (cx,cy,r).
//
// 行 [ry0,ry1] 上圆的水平截面是 x∈[cx−w, cx+w]（w=√(r²−t²)，t=y−cy）。
// 与 x 区间的交长 = clamp(cx+w,rx0,rx1) − clamp(cx−w,rx0,rx1)。
// 以 u=rx0−cx、宽度 W=rx1−rx0 重写：右交 = clamp(w−u,0,W)，左交 =
// clamp(−u−w,0,W)。两者的分段边界都在 w(t)=k 的等高线上（k=u+W、u、
// −u−W、−u），即 t=±√(r²−k²)；每个子区间用原函数 W(t) 解析积分，无采样
// 近似（调查批 4f：替换单点 SDF+smoothstep 的陡化阶梯）。
func exactCircleRectArea(rx0, ry0, rx1, ry1, cx, cy, r float64) float64 {
	if r <= 0 || rx1 <= rx0 || ry1 <= ry0 {
		return 0
	}
	rectW := rx1 - rx0
	// 快速路径：矩形四角到圆心距离全 ≤ r → 完全在圆内（凸性）。
	corners := [4][2]float64{{rx0, ry0}, {rx1, ry0}, {rx0, ry1}, {rx1, ry1}}
	allIn := true
	for _, c := range corners {
		dx, dy := c[0]-cx, c[1]-cy
		if dx*dx+dy*dy > r*r {
			allIn = false
			break
		}
	}
	if allIn {
		return rectW * (ry1 - ry0)
	}
	// 快速路径：圆心到矩形最近点距离 ≥ r → 完全在外。
	ncx := math.Max(rx0, math.Min(cx, rx1))
	ncy := math.Max(ry0, math.Min(cy, ry1))
	if math.Hypot(cx-ncx, cy-ncy) >= r {
		return 0
	}

	u := rx0 - cx               // 矩形左边相对圆心
	ty0 := math.Max(ry0-cy, -r) // 行 ∩ 圆的存在区间（t 空间）
	ty1 := math.Min(ry1-cy, r)
	if ty0 >= ty1 {
		return 0
	}

	// 收集分段点：w(t)=k 的 t=±√(r²−k²)（k>0 时才有解）。k 取 R+ 的形态
	// 边界 w=u+W（R+=W ⇔ w≥u+W）、w=u（R+=0 ⇔ w≤u）与 R− 的形态边界
	// w=−u−W、w=−u，再叠圆极点 ±r。
	cuts := make([]float64, 0, 10)
	for _, k := range [4]float64{u + rectW, u, -u - rectW, -u} {
		if k > 0 && k < r {
			s := math.Sqrt(r*r - k*k)
			cuts = append(cuts, -s, s)
		} else if k == 0 {
			cuts = append(cuts, -r, r)
		}
	}
	cuts = append(cuts, ty0, ty1)
	// 排序 + 去重 + clip。
	sort.Float64s(cuts)
	uniq := make([]float64, 0, len(cuts))
	for _, v := range cuts {
		if v < ty0-1e-12 || v > ty1+1e-12 {
			continue
		}
		if len(uniq) == 0 || v > uniq[len(uniq)-1]+1e-12 {
			uniq = append(uniq, math.Max(ty0, math.Min(ty1, v)))
		}
	}
	if len(uniq) < 2 {
		return 0
	}

	// 原函数分段积分：cuts 已把行切成 R+/R− 形态恒定的子区间，每个子区间
	// 按中点值判定形态，用 W(t) 或线性 t 精确积分。
	area := 0.0
	for i := 0; i+1 < len(uniq); i++ {
		ta, tb := uniq[i], uniq[i+1]
		if tb <= ta {
			continue
		}
		tm := (ta + tb) / 2
		wm := sdfWHalfWidth(tm, r)
		// R+：g(t)=w−u 的 clamp(0,W)。形态：wm−u≥W → 全 W；wm−u≤0 → 全 0；
		// 否则 ∫(w−u)dt = (W(tb)−W(ta)) − u·(tb−ta)。
		var rp, rn float64
		switch {
		case wm-u >= rectW:
			rp = rectW * (tb - ta)
		case wm-u > 0:
			rp = (sdfWAntideriv(tb, r) - sdfWAntideriv(ta, r)) - u*(tb-ta)
		}
		// R−：g(t)=−u−w 的 clamp(0,W)。形态：−u−wm≥W → 全 W；≤0 → 全 0；
		// 否则 ∫(−u−w)dt = −u·(tb−ta) − (W(tb)−W(ta))。
		switch {
		case -u-wm >= rectW:
			rn = rectW * (tb - ta)
		case -u-wm > 0:
			rn = -u*(tb-ta) - (sdfWAntideriv(tb, r) - sdfWAntideriv(ta, r))
		}
		area += rp - rn
	}
	if area < 0 {
		area = 0
	}
	return area
}

// exactEllipsePixelArea integrates the exact intersection area of the 1×1
// pixel block at pixel-center (px,py) with the ellipse (cx,cy,rx,ry):
// affine-map to unit-circle space (X=(x−cx)/rx, Y=(y−cy)/ry), integrate the
// circle against the mapped rectangle, scale by rx·ry.
func exactEllipsePixelArea(px, py, cx, cy, rx, ry float64) float64 {
	if rx <= 0 || ry <= 0 {
		return 0
	}
	rx0 := (px - 0.5 - cx) / rx
	rx1 := (px + 0.5 - cx) / rx
	ry0 := (py - 0.5 - cy) / ry
	ry1 := (py + 0.5 - cy) / ry
	return exactCircleRectArea(rx0, ry0, rx1, ry1, 0, 0, 1) * rx * ry
}

// exactRRectPixelArea integrates the exact intersection area of the 1×1
// pixel block at pixel-center (px,py) with the rounded rectangle centered
// (cx,cy), half extents (halfW,halfH), corner radius cr.
//
// 行积分：rrect 的水平截面半宽 W(dy) 分两段——直边区（|dy|≤hh−cr）W=hw
// 常数；角区（hh−cr<|dy|<hh）W=(hw−cr)+√(cr²−u²)（u=|dy|−(hh−cr)，原函数
// 同圆）。把行按 {−hh,−(hh−cr),hh−cr,hh} 与 clamp 分界切开逐段解析积分。
func exactRRectPixelArea(px, py, cx, cy, hw, hh, cr float64) float64 {
	if hw <= 0 || hh <= 0 {
		return 0
	}
	if cr <= 0 {
		// 退化成矩形：精确平凡。
		ix0 := math.Max(px-0.5, cx-hw)
		ix1 := math.Min(px+0.5, cx+hw)
		iy0 := math.Max(py-0.5, cy-hh)
		iy1 := math.Min(py+0.5, cy+hh)
		if ix1 <= ix0 || iy1 <= iy0 {
			return 0
		}
		return (ix1 - ix0) * (iy1 - iy0)
	}
	cr = math.Min(cr, math.Min(hw, hh))

	rx0, rx1 := px-0.5, px+0.5
	ry0, ry1 := py-0.5, py+0.5
	// 快速路径：四角全内 → 满覆盖。
	corners := [4][2]float64{{rx0, ry0}, {rx1, ry0}, {rx0, ry1}, {rx1, ry1}}
	allIn := true
	for _, c := range corners {
		if sdfRRect(c[0], c[1], cx, cy, hw, hh, cr) > 0 {
			allIn = false
			break
		}
	}
	if allIn {
		return 1
	}
	// 快速路径：最近点在外 → 0。
	ncx := math.Max(rx0, math.Min(cx, rx1))
	ncy := math.Max(ry0, math.Min(cy, ry1))
	if sdfRRect(ncx, ncy, cx, cy, hw, hh, cr) >= 0 {
		return 0
	}

	// 行分段：y 空间的结构分界。
	straightDy := hh - cr // 直边区 |dy| ≤ straightDy
	var ys []float64
	ys = append(ys, ry0, ry1)
	for _, b := range []float64{cy - hh, cy - straightDy, cy + straightDy, cy + hh} {
		if b > ry0 && b < ry1 {
			ys = append(ys, b)
		}
	}
	sort.Float64s(ys)

	area := 0.0
	for i := 0; i+1 < len(ys); i++ {
		ya, yb := ys[i], ys[i+1]
		if yb <= ya {
			continue
		}
		ym := (ya + yb) / 2
		dym := ym - cy
		ady := math.Abs(dym)
		if ady >= hh {
			continue // 该子行在形状外（y 空间结构保证不发生，防御）
		}
		if ady <= straightDy {
			// 直边区：W=hw 常数。截面积 = clamp 区间长 × (yb−ya)。
			ix0 := math.Max(rx0, cx-hw)
			ix1 := math.Min(rx1, cx+hw)
			if ix1 > ix0 {
				area += (ix1 - ix0) * (yb - ya)
			}
			continue
		}
		// 角区（上/下）：W(dy) = arcBase + √(cr²−u²)，u=|dy|−straightDy。
		arcBase := hw - cr
		// u 空间的行区间（沿 dy 递增方向）。上角 u 随 dy 增；下角 u 随 dy 减。
		var ua, ub float64
		upper := dym > 0
		if upper {
			ua, ub = ya-cy-straightDy, yb-cy-straightDy
		} else {
			ua, ub = -(yb - cy) - straightDy, -(ya - cy) - straightDy
		}
		// clamp 分界（W=k ⇔ u=±√(cr²−(k−arcBase)²)）。四个形态边界：
		//   R+：W = rx1−cx（上 clamp）、W = rx0−cx（下 clamp）；
		//   R−：W = cx−rx0（−(rx0−cx)−W=0）、W = cx−rx1（=rectW）。
		//   W 的范围 [arcBase, arcBase+cr]。
		kU := rx1 - cx
		kL := rx0 - cx
		cutU := func(k float64, out []float64) []float64 {
			d := k - arcBase
			if d > 0 && d < cr {
				s := math.Sqrt(cr*cr - d*d)
				out = append(out, -s, s)
			} else if d == 0 {
				out = append(out, -cr, cr)
			}
			return out
		}
		ucuts := make([]float64, 0, 10)
		for _, k := range [4]float64{kU, kL, cx - rx0, cx - rx1} {
			ucuts = cutU(k, ucuts)
		}
		ucuts = append(ucuts, ua, ub)
		sort.Float64s(ucuts)
		uuniq := make([]float64, 0, len(ucuts))
		for _, v := range ucuts {
			if v < ua-1e-12 || v > ub+1e-12 {
				continue
			}
			if len(uuniq) == 0 || v > uuniq[len(uuniq)-1]+1e-12 {
				uuniq = append(uuniq, math.Max(ua, math.Min(ub, v)))
			}
		}
		// u 空间行进长度系数：上角 du=+dy、下角 du=−dy → |du/dy|=1，段长一致。
		for j := 0; j+1 < len(uuniq); j++ {
			ua2, ub2 := uuniq[j], uuniq[j+1]
			if ub2 <= ua2 {
				continue
			}
			um := (ua2 + ub2) / 2
			wm := arcBase + sdfWHalfWidth(um, cr)
			// R+：clamp(cx+W,rx0,rx1)−rx0 = clamp(W−(rx0−cx),0,rectW)。
			// 用 W 对 u 的原函数：∫W du = arcBase·len + (Wcr(ub2)−Wcr(ua2))。
			wInt := func(a, b float64) float64 {
				return arcBase*(b-a) + (sdfWAntideriv(b, cr) - sdfWAntideriv(a, cr))
			}
			rectW := rx1 - rx0
			var rp, rn float64
			switch {
			case wm-(rx0-cx) >= rectW:
				rp = rectW * (ub2 - ua2)
			case wm-(rx0-cx) > 0:
				rp = wInt(ua2, ub2) - (rx0 - cx) * (ub2 - ua2)
			}
			// R−：clamp(cx−W,rx0,rx1)−rx0 = clamp(−(rx0−cx)−W,0,rectW)。
			switch {
			case -(rx0-cx)-wm >= rectW:
				rn = rectW * (ub2 - ua2)
			case -(rx0-cx)-wm > 0:
				rn = -(rx0-cx)*(ub2-ua2) - wInt(ua2, ub2)
			}
			area += rp - rn
		}
	}
	if area < 0 {
		area = 0
	}
	return area
}

// SDFCircleCoverage computes anti-aliased coverage for a stroked circle
// using a signed distance field approach.
//
// Parameters:
//   - px, py: pixel center coordinates
//   - cx, cy: circle center
//   - radius: circle radius (to center of stroke)
//   - halfStrokeWidth: half the stroke width
//
// Returns a coverage value in [0, 1] where 1 means fully inside the stroke.
func SDFCircleCoverage(px, py, cx, cy, radius, halfStrokeWidth float64) float64 {
	dist := math.Hypot(px-cx, py-cy)
	sdf := math.Abs(dist-radius) - halfStrokeWidth
	return smoothstepCoverage(sdf)
}

// SDFFilledRRectCoverage computes anti-aliased coverage for a filled rounded
// rectangle at the 1×1 pixel block whose center is (px, py).
//
// 批 4f 根修：与圆同因（单点 SDF+smoothstep 陡化），改解析行积分——直边区
// 常数截面、角区圆弧原函数精确积分。
//
// Parameters:
//   - px, py: pixel center coordinates
//   - cx, cy: rectangle center
//   - halfW, halfH: half-width and half-height of the rectangle
//   - cornerRadius: radius of the rounded corners
//
// Returns a coverage value in [0, 1] where 1 means fully inside.
func SDFFilledRRectCoverage(px, py, cx, cy, halfW, halfH, cornerRadius float64) float64 {
	return exactRRectPixelArea(px, py, cx, cy, halfW, halfH, cornerRadius)
}

// SDFFilledEllipseCoverage computes anti-aliased coverage for a filled
// ellipse at the 1×1 pixel block whose center is (px, py).
//
// 批 4f 根修：旧伪距离 (len−1)·min_r + smoothstep 在长轴/短轴的过渡带
// 宽度不一致且切线区陡化；改仿射到单位圆空间做精确行积分，面积按 rx·ry
// 还原。
func SDFFilledEllipseCoverage(px, py, cx, cy, rx, ry float64) float64 {
	return exactEllipsePixelArea(px, py, cx, cy, rx, ry)
}

// SDFRRectCoverage computes anti-aliased coverage for a stroked rounded
// rectangle using a signed distance field approach.
//
// Parameters:
//   - px, py: pixel center coordinates
//   - cx, cy: rectangle center
//   - halfW, halfH: half-width and half-height of the rectangle
//   - cornerRadius: radius of the rounded corners
//   - halfStrokeWidth: half the stroke width
//
// Returns a coverage value in [0, 1] where 1 means fully inside the stroke.
func SDFRRectCoverage(px, py, cx, cy, halfW, halfH, cornerRadius, halfStrokeWidth float64) float64 {
	dist := sdfRRect(px, py, cx, cy, halfW, halfH, cornerRadius)
	sdf := math.Abs(dist) - halfStrokeWidth
	return smoothstepCoverage(sdf)
}

// sdfRRect computes the signed distance from a point to a rounded rectangle.
// Negative values are inside, positive values are outside.
func sdfRRect(px, py, cx, cy, halfW, halfH, cornerRadius float64) float64 {
	// Translate to center and use symmetry (work in first quadrant).
	dx := math.Abs(px-cx) - halfW + cornerRadius
	dy := math.Abs(py-cy) - halfH + cornerRadius

	// Outside the corner region: max(dx, dy) gives the distance to the edge.
	// Inside the corner region: the Euclidean distance to the corner circle.
	outside := math.Sqrt(math.Max(dx, 0)*math.Max(dx, 0) + math.Max(dy, 0)*math.Max(dy, 0))
	inside := math.Min(math.Max(dx, dy), 0)

	return outside + inside - cornerRadius
}

// smoothstepCoverage converts a signed distance to an anti-aliased coverage
// value using a Hermite smoothstep function.
//
// sdf < -afwidth => 1.0 (fully inside)
// sdf > +afwidth => 0.0 (fully outside)
// Otherwise => smooth transition
func smoothstepCoverage(sdf float64) float64 {
	if sdf >= sdfAntialiasWidth {
		return 0
	}
	if sdf <= -sdfAntialiasWidth {
		return 1
	}
	t := (sdf + sdfAntialiasWidth) / (2 * sdfAntialiasWidth)
	// Hermite smoothstep: 3t^2 - 2t^3
	return 1 - (t * t * (3 - 2*t))
}
