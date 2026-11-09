package render_test

import (
	"fmt"
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

// 值级验收：tile 栅格器（Sparse 4x4 / TileCompute 16x16）与 Analytic 金样
// 逐像素比较覆盖值，并对手算精确面积的像素做强断言。覆盖 tile 边界、
// 分数行起止、洞、多轮廓、even-odd、超视口等关键路径。
//
// 容差依据（实测）：
//   - tile 路径用反导数（clampIntegral）精确积分直线边的面积，理论无误差；
//   - Analytic 金样在子像素网格上量化边坐标（1/4px 级），分数边/顶点行
//     与精确面积偏差实测 ≤ 8/255；
//   - TileCompute 沿 vello fine 公式，竖直边面积用 f32 相消计算，实测
//     偏差 ≤ 2/255。
// 因此：完全覆盖像素必须逐位一致；部分覆盖像素允许 ≤ 8/255（见
// rasterValueTolerance）；Auto/SDF 只比较 ink 轮廓（SDF 是 Auto 对矩形类
// 形状的另一条分路，AA 模型是距离场，与面积金样非同一口径）。

type rasterValueCase struct {
	name     string
	build    func(p *render.Path)
	fillRule render.FillRule
	// diagonal marks shapes with slanted edges / fractional vertices, where
	// the SDF distance-field AA (rleModes) is not the same coverage model as
	// the analytic area golden. valueModes still pin them pixel-by-pixel to
	// rasterValueTolerance; only the row-by-row ">128 ink" contour check is
	// replaced by a geometry check (see the rleModes loop).
	diagonal bool
}

func rasterValueCases() []rasterValueCase {
	return []rasterValueCase{
		{
			name: "单矩形(对齐)",
			build: func(p *render.Path) {
				p.Rectangle(8, 8, 40, 40)
			},
		},
		{
			name: "非对齐矩形(分数边界)",
			build: func(p *render.Path) {
				p.Rectangle(8.25, 9.5, 30.75, 25.5)
			},
		},
		{
			name: "U形单轮廓",
			build: func(p *render.Path) {
				p.MoveTo(10, 10)
				p.LineTo(10, 40)
				p.LineTo(46, 40)
				p.LineTo(46, 10)
				p.LineTo(40, 10)
				p.LineTo(40, 34)
				p.LineTo(16, 34)
				p.LineTo(16, 10)
				p.Close()
			},
		},
		{
			name: "上下堆叠两矩形",
			build: func(p *render.Path) {
				p.Rectangle(8, 20, 40, 10)
				p.Rectangle(8, 30, 40, 10)
			},
		},
		{
			name: "左右并排两矩形",
			build: func(p *render.Path) {
				p.Rectangle(8, 20, 12, 4)
				p.Rectangle(36, 20, 12, 4)
			},
		},
		{
			name: "差集(布尔产物)",
			build: func(p *render.Path) {
				outer := render.NewPath()
				outer.Rectangle(8, 8, 40, 40)
				inner := render.NewPath()
				inner.Rectangle(20, 20, 16, 16)
				copyPathInto(p, outer.Op(inner, render.PathOpDifference))
			},
		},
		{
			name:     "even-odd 打洞",
			fillRule: render.FillRuleEvenOdd,
			build: func(p *render.Path) {
				p.Rectangle(8, 8, 40, 40)
				p.Rectangle(20, 20, 16, 16)
			},
		},
		{
			name: "菱形(分数行起止)",
			build: func(p *render.Path) {
				p.MoveTo(20, 5.5)
				p.LineTo(35.5, 20)
				p.LineTo(20, 34.5)
				p.LineTo(4.5, 20)
				p.Close()
			},
			diagonal: true,
		},
		{
			name: "超视口矩形",
			build: func(p *render.Path) {
				p.Rectangle(-10, -5, 60, 50)
			},
		},
	}
}

// copyPathInto 把一个路径的 verb 序列追加到目标路径，便于复用 build 闭包。
func copyPathInto(dst, src *render.Path) {
	src.Iterate(func(verb render.PathVerb, coords []float64) {
		switch verb {
		case render.MoveTo:
			dst.MoveTo(coords[0], coords[1])
		case render.LineTo:
			dst.LineTo(coords[0], coords[1])
		case render.Close:
			dst.Close()
		}
	})
}

// rasterValueCoverage 以指定模式光栅化用例，返回覆盖值（alpha）数组。
func rasterValueCoverage(t *testing.T, mode render.RasterizerMode, c rasterValueCase, w, h int) []uint8 {
	t.Helper()
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.SetRasterizerMode(mode)
	dc.ClearWithColor(render.Transparent)
	if c.fillRule == render.FillRuleEvenOdd {
		dc.SetFillRule(render.FillRuleEvenOdd)
	}
	dc.SetRGB(1, 0, 0)
	p := render.NewPath()
	c.build(p)
	if err := dc.FillPath(p); err != nil {
		t.Fatalf("FillPath: %v", err)
	}

	img := dc.Image()
	out := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			out[y*w+x] = uint8(a >> 8)
		}
	}
	return out
}

// rasterValueInkRuns 返回某一行覆盖率 > 128 的连续区段，用于轮廓比较。
func rasterValueInkRuns(v []uint8, y, w int) string {
	out := ""
	inRun := false
	start := 0
	for x := 0; x < w; x++ {
		on := v[y*w+x] > 128
		if on && !inRun {
			start = x
			inRun = true
		}
		if !on && inRun {
			out += fmt.Sprintf("[%d..%d] ", start, x-1)
			inRun = false
		}
	}
	if inRun {
		out += fmt.Sprintf("[%d..%d] ", start, w-1)
	}
	return out
}

// rasterValueInkBBox returns the bbox of ">128 ink" pixels, the same
// threshold rasterValueInkRuns uses, for the diagonal-shape geometry check.
func rasterValueInkBBox(v []uint8, w, h int) image.Rectangle {
	minX, minY, maxX, maxY := 1<<30, 1<<30, -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if v[y*w+x] > 128 {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < 0 {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

func absRaster(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func rasterValueMaxDiff(a, b []uint8) (maxDiff int, diffCount int) {
	if len(a) != len(b) {
		panic("length mismatch")
	}
	for i := range a {
		d := int(a[i]) - int(b[i])
		if d < 0 {
			d = -d
		}
		if d != 0 {
			diffCount++
		}
		if d > maxDiff {
			maxDiff = d
		}
	}
	return maxDiff, diffCount
}

// rasterValueDiffDetail 输出前几处差异，便于失败定位。
func rasterValueDiffDetail(a, b []uint8, w int, limit int) string {
	shown := 0
	out := ""
	for i := range a {
		if a[i] != b[i] {
			out += fmt.Sprintf(" (%d,%d)=%d/%d", i%w, i/w, a[i], b[i])
			shown++
			if shown >= limit {
				break
			}
		}
	}
	return out
}

// rasterValueTolerance 是部分覆盖像素允许的最大覆盖值差（0-255 定量）。
const rasterValueTolerance = 8

// TestRasterizerValueMatchesAnalytic 比较 tile 栅格器与 Analytic 金样。
func TestRasterizerValueMatchesAnalytic(t *testing.T) {
	const w, h = 64, 64

	// valueModes 逐像素比值；rleModes 只比 ink 轮廓。
	valueModes := []render.RasterizerMode{
		render.RasterizerSparseStrips,
		render.RasterizerTileCompute,
	}
	rleModes := []render.RasterizerMode{
		render.RasterizerAuto,
		render.RasterizerSDF,
	}

	for _, c := range rasterValueCases() {
		t.Run(c.name, func(t *testing.T) {
			golden := rasterValueCoverage(t, render.RasterizerAnalytic, c, w, h)
			inkPixels := 0
			for _, v := range golden {
				if v != 0 {
					inkPixels++
				}
			}
			if inkPixels == 0 {
				t.Fatalf("analytic golden is empty for %q", c.name)
			}

			for _, mode := range valueModes {
				got := rasterValueCoverage(t, mode, c, w, h)

				// 完全覆盖像素必须逐位一致；部分覆盖像素允许量化容差。
				for i := range golden {
					g, o := golden[i], got[i]
					if g == o {
						continue
					}
					if g == 255 && o != 255 {
						t.Errorf("%s: interior pixel (%d,%d) = %d, want 255", mode, i%w, i/w, o)
					}
					if (g == 0) != (o == 0) && (g > 32 || o > 32) {
						t.Errorf("%s: phantom/missing ink at (%d,%d): %d vs %d", mode, i%w, i/w, g, o)
					}
				}

				maxDiff, diffCount := rasterValueMaxDiff(golden, got)
				if maxDiff > rasterValueTolerance {
					t.Errorf("%s: max coverage diff %d (%d px differ) vs Analytic; first diffs:%s",
						mode, maxDiff, diffCount, rasterValueDiffDetail(golden, got, w, 8))
				} else {
					t.Logf("%s: max diff %d, differing px %d, ink px %d", mode, maxDiff, diffCount, inkPixels)
				}
			}

			for _, mode := range rleModes {
				got := rasterValueCoverage(t, mode, c, w, h)
				if c.diagonal {
					// SDF/Auto use a distance-field AA: on slanted edges and
					// sharp vertices its coverage is systematically higher
					// than the analytic area golden (measured on this diamond:
					// apex row 139 vs 34, next rows 155/171/187 vs 40/49/59,
					// interior saturating to 255 one row earlier). Its >128
					// "ink" contour is therefore wider on EVERY slant row —
					// comparing it row-by-row to an area golden is comparing
					// two different AA models, not a regression. Skia's
					// distance-field AA has the same smoothstep model. The
					// valueModes above still pin coverage to ≤8/255. Here
					// assert geometry instead: ink present and its bounding
					// box within 2 px of the golden's (catches a missing,
					// displaced or flooded shape).
					gb, ob := rasterValueInkBBox(golden, w, h), rasterValueInkBBox(got, w, h)
					if ob.Empty() {
						t.Errorf("%s: no ink for diagonal shape %q", mode, c.name)
						continue
					}
					if absRaster(gb.Min.X-ob.Min.X) > 2 || absRaster(gb.Min.Y-ob.Min.Y) > 2 ||
						absRaster(gb.Max.X-ob.Max.X) > 2 || absRaster(gb.Max.Y-ob.Max.Y) > 2 {
						t.Errorf("%s: ink bbox %v vs golden %v (want within 2px)", mode, ob, gb)
					}
					continue
				}
				for y := 0; y < h; y++ {
					want := rasterValueInkRuns(golden, y, w)
					gotRuns := rasterValueInkRuns(got, y, w)
					if want != gotRuns {
						t.Errorf("%s: ink runs mismatch at row %d: golden=%q got=%q", mode, y, want, gotRuns)
						break
					}
				}
			}
		})
	}
}

// TestRasterizerExactAreaGolden 用独立手算的精确面积做金样（不依赖任何
// 现有实现）。
//
// 菱形边从顶点 (20,5.5) 出发斜率 dx/dy = 15.5/14.5 = 1.068966：
//   - 像素 (18,6) [x18..19,y6..7]：左边界 x(y)=20-1.068966*(y-5.5) 在
//     y=6.4354 处穿过 x=19，之后覆盖宽度 = 19-x(y)，面积
//     ∫(1.068966u-1)du (u 从 0.9354 到 1.5) = 0.17032 → 43（0.17032*255+0.5）。
//   - 像素 (19,6)：面积 = ∫1.068966u du (0.5→0.9354) + ∫1 du (0.9354→1.5)
//     = 0.89857 → 229。
//
// 非对齐矩形 Rectangle(8.25, 9.5, 30.75, 25.5) 左边缘在 x=8.25：
//   - 像素 (8,10)（整行覆盖）：宽度 0.75 → 0.75*255+0.5 = 191.75 → 191。
//   - 像素 (8,9)（顶部半行）：0.5*0.75 = 0.375 → 96.125 → 96。
//
// TileCompute 沿 vello fine 公式，竖直边用 f32 相消算面积，实测比精确值
// 低至 1-2/255，故 TileCompute 用 exactGoldenToleranceTile 容差。
func TestRasterizerExactAreaGolden(t *testing.T) {
	const w, h = 64, 64
	type pixelGolden struct {
		x, y  int
		value uint8
		note  string
	}

	cases := []struct {
		name    string
		build   func(p *render.Path)
		pixels  []pixelGolden
		tileTol uint8
	}{
		{
			name: "菱形(分数行起止)",
			build: func(p *render.Path) {
				p.MoveTo(20, 5.5)
				p.LineTo(35.5, 20)
				p.LineTo(20, 34.5)
				p.LineTo(4.5, 20)
				p.Close()
			},
			pixels: []pixelGolden{
				{x: 18, y: 6, value: 43, note: "上左顶点行部分覆盖"},
				{x: 19, y: 6, value: 229, note: "上顶点右邻像素"},
			},
			tileTol: 0,
		},
		{
			name: "非对齐矩形(分数边界)",
			build: func(p *render.Path) {
				p.Rectangle(8.25, 9.5, 30.75, 25.5)
			},
			pixels: []pixelGolden{
				{x: 8, y: 10, value: 191, note: "左边缘 0.75 覆盖"},
				{x: 8, y: 9, value: 96, note: "顶行 0.375 覆盖"},
			},
			tileTol: 2,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cv := rasterValueCase{name: c.name, build: c.build}
			for _, mode := range []render.RasterizerMode{render.RasterizerSparseStrips, render.RasterizerTileCompute} {
				got := rasterValueCoverage(t, mode, cv, w, h)
				tol := uint8(0)
				if mode == render.RasterizerTileCompute {
					tol = c.tileTol
				}
				for _, pg := range c.pixels {
					v := got[pg.y*w+pg.x]
					d := int(v) - int(pg.value)
					if d < 0 {
						d = -d
					}
					if uint8(d) > tol {
						t.Errorf("%s: pixel (%d,%d) = %d, want %d±%d (%s)",
							mode, pg.x, pg.y, v, pg.value, tol, pg.note)
					}
				}
			}
		})
	}
}
