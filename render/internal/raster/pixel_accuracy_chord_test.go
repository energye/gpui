package raster

// 批②红测：细吸附（acc=6）/稠密短边的覆盖正确性回归网。
//
// 历史（4e）：acc=6 下出现过「弦线」伪影，曾判为条带 1/4 步进漏检交叉。
// 本批逐步 trace 到具体边/行后，根因是两个引擎洞（不是切分粒度）：
//   ① 延迟边插入时 edgeIdx 被无条件回写拉低 → 下一行 first-pass 重复插边
//      → winding 失衡 → 整行假墨/少墨（弦线形态）；改单调高水位修。
//   ② 线边按子像素 LastY（1/4 网格）过早过期 → acc=6 丢失行首不足 1/4px
//      的真墨；改像素空间 LowerY 过期（acc=2 下数学恒等、逐位不变）。
// 修后 acc=6 是新默认质量档，严格优于/不劣于旧 acc=2 参考。
// 本文件对 32×32/像素数值积分真值断言覆盖误差；弦线（整行假墨，over8
// 300+/max255）必然爆界。

import (
	"math"
	"testing"
)

// buildCrossingFanPaths 构造交叉密集扇面：从顶点 (32,8) 到 y=56 底边均分
// 点的直线族（相邻线中途交叉），配底边闭合——单 path 多子路径，保证
// BuildFromPath 产生稠密短边群。返回 (path, oracle 线段集)。
func buildCrossingFanPaths(n int) (*testPath, [][4]float32) {
	var verbs []PathVerb
	var pts []float32
	var segs [][4]float32
	addSeg := func(x0, y0, x1, y1 float32) {
		if y0 != y1 {
			segs = append(segs, [4]float32{x0, y0, x1, y1})
		}
	}
	// n 个细三角形：每个三角形 = 顶点(32,8) + 底边两点，逐个闭合。
	// 相邻三角形的斜边在行 y∈(8,56) 内交叉。
	for i := 0; i < n; i++ {
		x0 := 6 + 52*float32(i)/float32(n)
		x1 := 6 + 52*float32(i+1)/float32(n)
		verbs = append(verbs, MoveTo, LineTo, LineTo, Close)
		pts = append(pts, 32, 8, x0, 56, x1, 56)
		addSeg(32, 8, x0, 56)
		addSeg(x0, 56, x1, 56)
		addSeg(x1, 56, 32, 8)
	}
	return &testPath{verbs: verbs, points: pts}, segs
}

// oracleFanCoverage 对线段集做 nonzero 32×32/像素 数值积分。
func oracleFanCoverage(segs [][4]float32, w, h int) []uint8 {
	out := make([]uint8, w*h)
	const n = 32
	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			hits := 0
			for j := 0; j < n; j++ {
				y := float64(py) + (float64(j)+0.5)/n
				for i := 0; i < n; i++ {
					x := float64(px) + (float64(i)+0.5)/n
					wv := 0.0
					for _, s := range segs {
						y0, y1 := float64(s[1]), float64(s[3])
						var lo, hi float64
						if y0 < y1 {
							lo, hi = y0, y1
						} else {
							lo, hi = y1, y0
						}
						if y < lo || y >= hi {
							continue
						}
						t := (y - float64(s[1])) / (float64(s[3]) - float64(s[1]))
						xi := float64(s[0]) + t*(float64(s[2])-float64(s[0]))
						if xi > x {
							if s[3] > s[1] {
								wv++
							} else {
								wv--
							}
						}
					}
					if wv != 0 {
						hits++
					}
				}
			}
			out[py*w+px] = uint8(math.Round(float64(hits) / float64(n*n) * 255))
		}
	}
	return out
}

func fillFanAtAccuracy(t *testing.T, tp *testPath, w, h int, accuracy int32) []uint8 {
	t.Helper()
	eb := NewEdgeBuilder(2)
	eb.SetFlattenCurves(true)
	if accuracy > 0 {
		eb.SetPixelAccuracy(accuracy)
	}
	eb.BuildFromPath(tp, IdentityTransform{})
	buf := make([]uint8, w*h)
	FillToBuffer(eb, w, h, FillRuleNonZero, buf)
	return buf
}

func cmpCoverage(got, want []uint8) (avg float64, maxv int, over8 int) {
	var sum float64
	for i := range got {
		d := int(got[i]) - int(want[i])
		if d < 0 {
			d = -d
		}
		if d > 8 {
			over8++
		}
		if d > maxv {
			maxv = d
		}
		sum += float64(d)
	}
	return sum / float64(len(got)), maxv, over8
}

// TestPixelAccuracyCrossingFan 锚定交叉密集场景（多条线同行内交叉）下两
// acc 的覆盖质量：与数值积分真值误差受界。弦线（整行假墨）必然爆界。
func TestPixelAccuracyCrossingFan(t *testing.T) {
	const w, h = 64, 64
	tp, segs := buildCrossingFanPaths(12)
	want := oracleFanCoverage(segs, w, h)

	for _, acc := range []int32{DefaultPixelAccuracy, FinePixelAccuracy} {
		acc := acc
		t.Run("acc"+itoa32(acc), func(t *testing.T) {
			got := fillFanAtAccuracy(t, tp, w, h, acc)
			avg, maxv, over8 := cmpCoverage(got, want)
			t.Logf("acc=%d: avg=%.3f max=%d over8=%d", acc, avg, maxv, over8)
			if avg > 3.0 {
				t.Errorf("acc=%d avg err %.3f exceeds 3.0 (0-255)", acc, avg)
			}
			if over8 > w*h/50 {
				t.Errorf("acc=%d over8=%d exceeds %d (2%% budget)", acc, over8, w*h/50)
			}
		})
	}
}

// buildDenseShortEdges 构造「端点分数 y 密集 + 行内多边」的短线段群
// （模拟 dash 描边展开产物）：端点 y 沿 1/64 网格密集分布、斜率各异使
// 相邻线段在行内交叉。这是 4e 弦线伪影的触发形态。
func buildDenseShortEdges(count int) (*testPath, [][4]float32) {
	var verbs []PathVerb
	var pts []float32
	var segs [][4]float32
	const halfW = 0.5 // 描边宽 1px 的半宽
	for i := 0; i < count; i++ {
		// 端点 y：1/64 粒度的分数分布（acc=6 下各不相同；acc=2 下大量
		// 同格——这正是两 acc 行为分化的来源）。
		y0 := 40.0 + float64(i)*0.7777
		y1 := y0 + 3.5 + float64(i%7)*0.109375 // 0.109375 = 7/64
		x0 := 6 + float64(i%13)*3.3
		x1 := x0 + 26 - float64(i%9)*2.0
		if y1 > 58 {
			y1 = 58 - float64(i%5)*0.53125
		}
		// 描边展开：线段法线偏移 ±halfW → 细四边形（dash 描边的真实产物）。
		dx, dy := x1-x0, y1-y0
		l := math.Sqrt(dx*dx + dy*dy)
		nx, ny := -dy/l*halfW, dx/l*halfW
		verbs = append(verbs, MoveTo, LineTo, LineTo, LineTo, Close)
		pts = append(pts,
			float32(x0+nx), float32(y0+ny),
			float32(x1+nx), float32(y1+ny),
			float32(x1-nx), float32(y1-ny),
			float32(x0-nx), float32(y0-ny))
		// oracle 线段：四边形的 4 条边。
		segs = append(segs,
			[4]float32{float32(x0 + nx), float32(y0 + ny), float32(x1 + nx), float32(y1 + ny)},
			[4]float32{float32(x1 + nx), float32(y1 + ny), float32(x1 - nx), float32(y1 - ny)},
			[4]float32{float32(x1 - nx), float32(y1 - ny), float32(x0 - nx), float32(y0 - ny)},
			[4]float32{float32(x0 - nx), float32(y0 - ny), float32(x0 + nx), float32(y0 + ny)})
	}
	return &testPath{verbs: verbs, points: pts}, segs
}

// TestPixelAccuracyDenseShortEdges 锚定稠密短边群（dash 描边展开形态）——
// 4e 弦线伪影的直接回归网。口径（本批拍板）：
//   - acc=6 是新默认质量档：严格锁 avg≤4 / over8≤4%（弦线必然爆界）；
//   - acc=2 是旧 Skia 逐位参考（1/4 吸附，over8 固有底≈170，本批改动前即
//     如此），不套新档预算，只作护栏：acc=6 不得劣于 acc=2。
func TestPixelAccuracyDenseShortEdges(t *testing.T) {
	const w, h = 64, 64
	tp, segs := buildDenseShortEdges(100)
	want := oracleFanCoverage(segs, w, h)

	avg2, max2, over82 := cmpCoverage(fillFanAtAccuracy(t, tp, w, h, DefaultPixelAccuracy), want)
	avg6, max6, over86 := cmpCoverage(fillFanAtAccuracy(t, tp, w, h, FinePixelAccuracy), want)
	t.Logf("acc=2 (legacy ref): avg=%.3f max=%d over8=%d", avg2, max2, over82)
	t.Logf("acc=6 (new default): avg=%.3f max=%d over8=%d", avg6, max6, over86)

	t.Run("acc6-strict", func(t *testing.T) {
		if avg6 > 4.0 {
			t.Errorf("acc=6 avg err %.3f exceeds 4.0 (0-255)", avg6)
		}
		if over86 > w*h/25 {
			t.Errorf("acc=6 over8=%d exceeds %d (4%% budget)", over86, w*h/25)
		}
	})

	t.Run("acc6-no-worse-than-acc2", func(t *testing.T) {
		if over86 > over82 {
			t.Errorf("acc=6 over8=%d worse than legacy acc=2 over8=%d", over86, over82)
		}
		if avg6 > avg2+0.05 {
			t.Errorf("acc=6 avg=%.3f materially worse than legacy acc=2 avg=%.3f", avg6, avg2)
		}
	})
}

func itoa32(v int32) string {
	if v == 0 {
		return "0"
	}
	digits := ""
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		digits = string(rune('0'+v%10)) + digits
		v /= 10
	}
	if neg {
		digits = "-" + digits
	}
	return digits
}
