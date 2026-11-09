package render

// SDF 覆盖公式对真值（解析面积）的红测（调查批 4f）。
// 背景：窗口进程（import render/gpu）的圆/圆角矩形经 SDFAccelerator 走
// SDF 单点采样 + 0.75px smoothstep；验证环境（conformance/resvg 对比）走
// AnalyticFiller 解析面积覆盖。showcase.svg 1024×640 实测两者差 32.3%。
// 本测试把 SDF 覆盖公式的边缘质量对 64×64/像素 数值积分真值量化。

import (
	"math"
	"testing"
)

// circleOracleArea 解析计算像素 (px,py)（1×1 方块）与圆 (cx,cy,r) 的相交面积。
// 用高密度子采样数值积分（256×256/px），独立于 SDF 公式。
func circleOracleArea(px, py, cx, cy, r float64) float64 {
	const n = 256
	hits := 0
	step := 1.0 / n
	for j := 0; j < n; j++ {
		y := py + (float64(j)+0.5)*step
		dy2 := (y - cy) * (y - cy)
		if dy2 > r*r {
			continue
		}
		half := math.Sqrt(r*r - dy2)
		// 本子采样行与圆相交的 x 区间 [cx-half, cx+half]
		x0 := cx - half
		x1 := cx + half
		// 与像素 x 区间 [px, px+1] 求交
		if x0 < px {
			x0 = px
		}
		if x1 > px+1 {
			x1 = px + 1
		}
		if x1 > x0 {
			hits += int((x1 - x0) / step)
		}
	}
	return float64(hits) / float64(n*n)
}

// oraclePointInside 通用点测试（椭圆/rrect 的 256×256/px 数值积分用）。
func oracleAreaGeneric(t *testing.T, name string, inside func(x, y float64) bool, px, py int) float64 {
	t.Helper()
	const n = 128
	hits := 0
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			x := float64(px) + (float64(i)+0.5)/n
			y := float64(py) + (float64(j)+0.5)/n
			if inside(x, y) {
				hits++
			}
		}
	}
	return float64(hits) / float64(n*n)
}

func TestSDFFilledEllipseCoverageVsTruth(t *testing.T) {
	cases := []struct {
		name             string
		cx, cy, rx, ry   float64
	}{
		{"fractional", 32.5, 32.25, 20.5, 14.5},
		{"wide-flat", 32, 32, 24, 10.25},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			inside := func(x, y float64) bool {
				dx, dy := (x-tc.cx)/tc.rx, (y-tc.cy)/tc.ry
				return dx*dx+dy*dy <= 1
			}
			var sumErr float64
			var maxErr float64
			n := 0
			for py := int(tc.cy - tc.ry - 2); py <= int(tc.cy+tc.ry+2); py++ {
				for px := int(tc.cx - tc.rx - 2); px <= int(tc.cx+tc.rx+2); px++ {
					truth := oracleAreaGeneric(t, tc.name, inside, px, py)
					got := SDFFilledEllipseCoverage(float64(px)+0.5, float64(py)+0.5, tc.cx, tc.cy, tc.rx, tc.ry)
					d := math.Abs(got - truth) * 255
					if truth > 0 && truth < 1 {
						sumErr += d
						n++
						if d > maxErr {
							maxErr = d
						}
					} else if d > 5 {
						sumErr += d
						n++
						if d > maxErr {
							maxErr = d
						}
					}
				}
			}
			avg := sumErr / float64(n)
			t.Logf("%s: edge-px=%d avg=%.3f max=%.1f", tc.name, n, avg, maxErr)
			if avg > 2.0 {
				t.Errorf("%s: ellipse avg %.3f exceeds 2.0", tc.name, avg)
			}
			if maxErr > 8.0 {
				t.Errorf("%s: ellipse max %.1f exceeds 8.0", tc.name, maxErr)
			}
		})
	}
}

func TestSDFFilledRRectCoverageVsTruth(t *testing.T) {
	cases := []struct {
		name                          string
		cx, cy, hw, hh, cr            float64
	}{
		{"rounded-card", 32.5, 32.25, 20.5, 14.5, 5.5},
		{"nearly-circle", 32, 32, 12, 12, 12},
		{"tight-corners", 32.5, 32.5, 18.5, 10.5, 2.5},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			inside := func(x, y float64) bool {
				return sdfRRect(x, y, tc.cx, tc.cy, tc.hw, tc.hh, tc.cr) <= 0
			}
			var sumErr float64
			var maxErr float64
			n := 0
			for py := int(tc.cy - tc.hh - 2); py <= int(tc.cy+tc.hh+2); py++ {
				for px := int(tc.cx - tc.hw - 2); px <= int(tc.cx+tc.hw+2); px++ {
					truth := oracleAreaGeneric(t, tc.name, inside, px, py)
					got := SDFFilledRRectCoverage(float64(px)+0.5, float64(py)+0.5, tc.cx, tc.cy, tc.hw, tc.hh, tc.cr)
					d := math.Abs(got - truth) * 255
					if truth > 0 && truth < 1 {
						sumErr += d
						n++
						if d > maxErr {
							maxErr = d
						}
					} else if d > 5 {
						sumErr += d
						n++
						if d > maxErr {
							maxErr = d
						}
					}
				}
			}
			avg := sumErr / float64(n)
			t.Logf("%s: edge-px=%d avg=%.3f max=%.1f", tc.name, n, avg, maxErr)
			if avg > 2.0 {
				t.Errorf("%s: rrect avg %.3f exceeds 2.0", tc.name, avg)
			}
			if maxErr > 8.0 {
				t.Errorf("%s: rrect max %.1f exceeds 8.0", tc.name, maxErr)
			}
		})
	}
}

func TestSDFFilledCircleCoverageVsTruth(t *testing.T) {
	cases := []struct {
		name         string
		cx, cy, r    float64
	}{
		// 半径带分数、圆心带分数（showcase badge circle r=11、虚线圆 r=46、
		// mask 圆 r=52 的典型形态）。
		{"fractional-center-r", 32.5, 32.5, 20.5},
		{"integral-center", 32, 32, 20},
		{"small-badge", 16.5, 16.25, 11.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sumErr float64
			var maxErr float64
			n := 0
			// 只扫边缘带（r±2px 的环带），统计边缘像素的覆盖误差。
			minX := int(math.Floor(tc.cx - tc.r - 2))
			maxX := int(math.Ceil(tc.cx + tc.r + 2))
			minY := int(math.Floor(tc.cy - tc.r - 2))
			maxY := int(math.Ceil(tc.cy + tc.r + 2))
			for py := minY; py <= maxY; py++ {
				for px := minX; px <= maxX; px++ {
					truth := circleOracleArea(float64(px), float64(py), tc.cx, tc.cy, tc.r)
					got := SDFFilledCircleCoverage(float64(px)+0.5, float64(py)+0.5, tc.cx, tc.cy, tc.r)
					d := math.Abs(got - truth)
					// 只计边缘过渡像素（0<truth<1）
					if truth > 0 && truth < 1 {
						sumErr += d * 255
						n++
						if d*255 > maxErr {
							maxErr = d * 255
						}
					} else if d > 0.02 { // 平坦区大错（越界墨迹/漏墨）
						sumErr += d * 255
						n++
						if d*255 > maxErr {
							maxErr = d * 255
						}
					}
				}
			}
			avg := sumErr / float64(n)
			t.Logf("%s: edge-px=%d avg=%.3f max=%.1f (0-255 scale)", tc.name, n, avg, maxErr)
			// 解析级质量下界（对齐 AnalyticFiller 的 0.0-0.3 量级）：
			// avg ≤ 2/255 且 max ≤ 8/255。现状单点采样+smoothstep(0.75)
			// 实测 avg≈8/max≈19（红），解析行积分修复后应收紧到本界。
			if avg > 2.0 {
				t.Errorf("%s: SDF coverage avg err %.3f exceeds 2.0 (0-255)", tc.name, avg)
			}
			if maxErr > 8.0 {
				t.Errorf("%s: SDF coverage max err %.1f exceeds 8.0 (0-255)", tc.name, maxErr)
			}
		})
	}
}
