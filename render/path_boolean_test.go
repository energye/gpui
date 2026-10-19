package render_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
)

// 布尔几何重写（Clipper 扫描线）的值级验收：
//   - 输出不再碎片化（方框减孔 = 2 轮廓 / 10 verbs，而不是 800 轮廓 / 4000 verbs）
//   - 交点坐标为精确几何值（1/1024px 量化），不再量化到像素网格
//   - 无 2048px 静默截断
//   - 区域语义 = 非零环绕：重叠子路径=并集面积、反向内环=挖孔、自相交=两瓣都填
//   - 输出在 NonZero 与 EvenOdd 两种填充规则下渲染一致
//   - 曲线按 0.1px 容差展平，顶点落在解析边界容差内

func boolRect(x, y, w, h float64) *render.Path {
	p := render.NewPath()
	p.Rectangle(x, y, w, h)
	return p
}

// boolRectReversed 手写反向矩形（与 Path.Rectangle 绕向相反），用于构造挖孔环。
func boolRectReversed(x, y, w, h float64) *render.Path {
	p := render.NewPath()
	p.MoveTo(x, y)
	p.LineTo(x, y+h)
	p.LineTo(x+w, y+h)
	p.LineTo(x+w, y)
	p.Close()
	return p
}

func boolInside(p *render.Path, x, y float64) bool {
	return p.Winding(render.Pt(x, y)) != 0
}

// boolContours 收集路径轮廓（MoveTo→LineTo*→Close）。
func boolContours(p *render.Path) [][]render.Point {
	var out [][]render.Point
	var cur []render.Point
	p.Iterate(func(v render.PathVerb, c []float64) {
		switch v {
		case render.MoveTo:
			cur = []render.Point{render.Pt(c[0], c[1])}
		case render.LineTo:
			cur = append(cur, render.Pt(c[0], c[1]))
		case render.Close:
			if len(cur) > 0 {
				out = append(out, cur)
				cur = nil
			}
		}
	})
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

func boolVertexSet(p *render.Path) []render.Point {
	var pts []render.Point
	for _, c := range boolContours(p) {
		pts = append(pts, c...)
	}
	return pts
}

func boolHasPoint(pts []render.Point, x, y, tol float64) bool {
	for _, p := range pts {
		if math.Abs(p.X-x) <= tol && math.Abs(p.Y-y) <= tol {
			return true
		}
	}
	return false
}

func TestPathBooleanDifferenceNoFragmentation(t *testing.T) {
	outer := boolRect(16, 16, 480, 480) // x 16..496
	inner := boolRect(96, 96, 320, 320) // x 96..416
	res := outer.Op(inner, render.PathOpDifference)

	contours := boolContours(res)
	if len(contours) != 2 {
		t.Fatalf("frame difference should yield 2 contours, got %d", len(contours))
	}
	if got := res.NumVerbs(); got != 10 {
		t.Fatalf("frame difference should yield 10 verbs (2 contours), got %d", got)
	}
	if res.HasCurves() {
		t.Fatalf("boolean output should be polygonal, got curves")
	}

	pts := boolVertexSet(res)
	if len(pts) != 8 {
		t.Fatalf("frame difference should yield 8 vertices, got %d", len(pts))
	}
	// 精确几何：整数坐标必须逐位相等（经 1/1024 网格往返无损）
	for _, want := range [][2]float64{
		{16, 16}, {496, 16}, {496, 496}, {16, 496},
		{96, 96}, {416, 96}, {416, 416}, {96, 416},
	} {
		if !boolHasPoint(pts, want[0], want[1], 0) {
			t.Fatalf("missing exact vertex (%v,%v); got %v", want[0], want[1], pts)
		}
	}

	for _, in := range [][2]float64{{50, 50}, {250, 30}, {60, 256}, {450, 256}, {250, 450}} {
		if !boolInside(res, in[0], in[1]) {
			t.Errorf("point (%v,%v) should be inside the frame", in[0], in[1])
		}
	}
	for _, out := range [][2]float64{{256, 256}, {100, 100}, {410, 410}} {
		if boolInside(res, out[0], out[1]) {
			t.Errorf("point (%v,%v) should be the punched hole", out[0], out[1])
		}
	}
}

// 旧实现（逐像素采样）在区域 >2048px 时静默截断，孔洞与右/下部分丢失。
func TestPathBooleanDifferenceBeyondLegacyCap(t *testing.T) {
	outer := boolRect(0, 0, 3000, 3000)
	inner := boolRect(1000, 1000, 1000, 1000)
	res := outer.Op(inner, render.PathOpDifference)

	if got := len(boolContours(res)); got != 2 {
		t.Fatalf("large frame should yield 2 contours, got %d", got)
	}
	if !boolInside(res, 500, 500) {
		t.Error("(500,500) should be inside the frame")
	}
	if boolInside(res, 1500, 1500) {
		t.Error("(1500,1500) should be the punched hole")
	}
	if !boolInside(res, 2990, 10) || !boolInside(res, 10, 2990) || !boolInside(res, 2990, 2990) {
		t.Error("region beyond 2048px should still be filled (no silent truncation)")
	}
}

func TestPathBooleanExactIntersections(t *testing.T) {
	// 分数坐标菱形 ∩ 方框：交点是精确几何值，不是像素量化值。
	dia := render.NewPath()
	dia.MoveTo(10.5, 0.25)
	dia.LineTo(20.75, 10.5)
	dia.LineTo(10.5, 20.25)
	dia.LineTo(0.25, 10.5)
	dia.Close()
	clip := boolRect(5, 5, 10, 10) // 5..15

	res := dia.Op(clip, render.PathOpIntersect)
	contours := boolContours(res)
	if len(contours) != 1 || len(contours[0]) != 5 {
		t.Fatalf("expected a single 5-vertex polygon, got %d contours: %v", len(contours), contours)
	}
	for _, want := range [][2]float64{{5, 5.75}, {5.75, 5}, {15, 5}, {15, 15}, {5, 15}} {
		if !boolHasPoint(contours[0], want[0], want[1], 1.0/512) {
			t.Errorf("missing exact intersection vertex (%v,%v); got %v", want[0], want[1], contours[0])
		}
	}
}

func TestPathBooleanRegionSemantics(t *testing.T) {
	big := boolRect(-50, -50, 200, 200) // 用于强制走布尔管线（单路径归一化）

	t.Run("overlapping subpaths fill under non-zero", func(t *testing.T) {
		p := render.NewPath()
		p.Rectangle(0, 0, 10, 10)
		p.Rectangle(5, 5, 10, 10)
		if p.Winding(render.Pt(7, 7)) == 0 {
			t.Fatal("fixture: overlap should wind twice (non-zero inside)")
		}
		res := p.Op(big, render.PathOpIntersect)
		if got := len(boolContours(res)); got != 1 {
			t.Fatalf("overlapping rects should merge into 1 contour, got %d", got)
		}
		for _, in := range [][2]float64{{2, 2}, {7, 7}, {12, 12}} {
			if !boolInside(res, in[0], in[1]) {
				t.Errorf("(%v,%v) should be inside the merged region", in[0], in[1])
			}
		}
		if boolInside(res, 14, 1) {
			t.Error("(14,1) should be outside")
		}
	})

	t.Run("reversed inner contour keeps the hole", func(t *testing.T) {
		ann := render.NewPath()
		ann.Rectangle(0, 0, 20, 20)
		ann.Append(boolRectReversed(5, 5, 10, 10))
		if ann.Winding(render.Pt(10, 10)) != 0 {
			t.Fatal("fixture: reversed inner rect should punch a hole")
		}
		res := ann.Op(big, render.PathOpIntersect)
		if !boolInside(res, 2, 2) || !boolInside(res, 17, 17) {
			t.Error("annulus ring should be filled")
		}
		if boolInside(res, 10, 10) {
			t.Error("annulus hole should stay empty (reversed contour semantics)")
		}
	})

	t.Run("concentric same-direction contours stay solid", func(t *testing.T) {
		p := render.NewPath()
		p.Rectangle(0, 0, 20, 20)
		p.Rectangle(5, 5, 10, 10)
		res := p.Op(big, render.PathOpIntersect)
		if !boolInside(res, 2, 2) || !boolInside(res, 10, 10) {
			t.Error("concentric same-direction contours should fill the whole disk")
		}
	})

	t.Run("self-intersecting bowtie fills both lobes", func(t *testing.T) {
		bow := render.NewPath()
		bow.MoveTo(0, 0)
		bow.LineTo(10, 10)
		bow.LineTo(10, 0)
		bow.LineTo(0, 10)
		bow.Close()
		res := bow.Op(big, render.PathOpIntersect)
		for _, in := range [][2]float64{{1, 2}, {9, 2}, {1, 8}, {9, 8}} {
			if !boolInside(res, in[0], in[1]) {
				t.Errorf("bowtie lobe (%v,%v) should be filled", in[0], in[1])
			}
		}
		for _, out := range [][2]float64{{5, 1}, {5, 9}} {
			if boolInside(res, out[0], out[1]) {
				t.Errorf("(%v,%v) should be outside the bowtie", out[0], out[1])
			}
		}
	})

	t.Run("xor excludes the overlap", func(t *testing.T) {
		a := boolRect(0, 0, 10, 10)
		b := boolRect(5, 5, 10, 10)
		res := a.Op(b, render.PathOpXor)
		if !boolInside(res, 2, 2) || !boolInside(res, 12, 12) {
			t.Error("xor should keep the non-overlapping parts")
		}
		if boolInside(res, 7, 7) {
			t.Error("xor should exclude the overlap")
		}
	})

	t.Run("curve after close starts from subpath start", func(t *testing.T) {
		// Close 后紧跟曲线：曲线起点应回到子路径起点（镜像 Path.Close 语义）
		a := render.NewPath()
		a.MoveTo(0, 0)
		a.LineTo(10, 0)
		a.Close()
		a.QuadraticTo(15, 5, 10, 10)
		a.Close()

		// 等价写法：显式回到起点后再画同一曲线
		b := render.NewPath()
		b.MoveTo(0, 0)
		b.LineTo(10, 0)
		b.LineTo(0, 0)
		b.QuadraticTo(15, 5, 10, 10)
		b.Close()

		big := boolRect(-50, -50, 200, 200)
		ra := a.Op(big, render.PathOpIntersect)
		rb := b.Op(big, render.PathOpIntersect)
		if ra.NumVerbs() == 0 {
			t.Fatal("fixture produced empty region")
		}
		diff := 0
		for x := -1.0; x <= 16.0; x += 0.5 {
			for y := -1.0; y <= 11.0; y += 0.5 {
				if boolInside(ra, x, y) != boolInside(rb, x, y) {
					diff++
				}
			}
		}
		if diff != 0 {
			t.Fatalf("close-then-curve path differs from explicit equivalent at %d probe points", diff)
		}
	})
}

// 输出方向必须同时满足两种填充规则：NonZero 靠轮廓反向抵消，EvenOdd 靠奇偶。
func TestPathBooleanFillRuleInvariance(t *testing.T) {
	outer := boolRect(4, 4, 36, 36)
	inner := boolRect(12, 12, 20, 20)
	// 再嵌一层岛，保证嵌套 ≥3 层也正确
	island := boolRect(18, 18, 8, 8)
	frame := outer.Op(inner, render.PathOpDifference)
	frame = frame.Op(island, render.PathOpUnion)

	draw := func(rule render.FillRule) *render.Context {
		dc := render.NewContext(44, 44)
		dc.ClearWithColor(render.White)
		dc.SetFillRule(rule)
		dc.SetRGB(1, 0, 0)
		frame.Iterate(func(v render.PathVerb, c []float64) {
			switch v {
			case render.MoveTo:
				dc.MoveTo(c[0], c[1])
			case render.LineTo:
				dc.LineTo(c[0], c[1])
			case render.Close:
				dc.ClosePath()
			}
		})
		if err := dc.Fill(); err != nil {
			t.Fatalf("Fill: %v", err)
		}
		return dc
	}

	nz := draw(render.FillRuleNonZero)
	defer nz.Close()
	eo := draw(render.FillRuleEvenOdd)
	defer eo.Close()

	imgNZ, imgEO := nz.Image(), eo.Image()
	diff := 0
	for y := 0; y < 44; y++ {
		for x := 0; x < 44; x++ {
			r1, g1, b1, a1 := imgNZ.At(x, y).RGBA()
			r2, g2, b2, a2 := imgEO.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				diff++
				if diff <= 3 {
					t.Errorf("pixel (%d,%d) differs: nonzero=%v,%v,%v,%v evenodd=%v,%v,%v,%v",
						x, y, r1, g1, b1, a1, r2, g2, b2, a2)
				}
			}
		}
	}
	if diff > 0 {
		t.Errorf("nonzero vs even-odd rendering differs in %d pixels", diff)
	}

	// 岛/孔/带三处区域取样复核：外框 4..40，孔 12..32，岛 18..26
	for _, tc := range []struct {
		x, y float64
		in   bool
	}{
		{6, 6, true},    // 外带
		{34, 34, true},  // 外带（右下）
		{15, 15, false}, // 孔环（孔内、岛外）
		{22, 22, true},  // 岛
		{2, 2, false},   // 外框之外
		{38, 20, true},  // 外带（右）
	} {
		got := boolInside(frame, tc.x, tc.y)
		if got != tc.in {
			t.Errorf("filigree probe (%v,%v): inside=%v want %v", tc.x, tc.y, got, tc.in)
		}
	}
}

func TestPathBooleanCurvesFlattened(t *testing.T) {
	circle := render.NewPath()
	circle.Circle(0, 0, 20)
	clip := boolRect(0, -25, 25, 50) // x>=0 半平面矩形
	res := circle.Op(clip, render.PathOpUnion)

	for _, in := range [][2]float64{{-15, 0}, {18, 18}, {2, 19}, {24, 0}} {
		if !boolInside(res, in[0], in[1]) {
			t.Errorf("(%v,%v) should be inside circle∪rect", in[0], in[1])
		}
	}
	for _, out := range [][2]float64{{-21, 0}, {-15, 15}, {-24, -24}, {26, 0}} {
		if boolInside(res, out[0], out[1]) {
			t.Errorf("(%v,%v) should be outside circle∪rect", out[0], out[1])
		}
	}

	// 顶点必须落在解析边界容差内：圆弧（|p|=20）或矩形边界（x∈{0,25} 或 y∈{±25}）。
	const tol = 0.15
	for _, p := range res.Flatten(0.001) {
		dCircle := math.Abs(math.Hypot(p.X, p.Y) - 20)
		dRect := math.Min(math.Min(math.Abs(p.X-0), math.Abs(p.X-25)), math.Min(math.Abs(p.Y-25), math.Abs(p.Y+25)))
		if dCircle > tol && dRect > tol {
			t.Errorf("vertex (%v,%v) off both boundaries: dCircle=%v dRect=%v", p.X, p.Y, dCircle, dRect)
		}
	}

	if n := res.NumVerbs(); n > 200 {
		t.Errorf("circle∪rect should stay compact, got %d verbs", n)
	}
}

func TestPathBooleanDegenerateInputs(t *testing.T) {
	rect := boolRect(0, 0, 10, 10)

	t.Run("nil and empty operands", func(t *testing.T) {
		empty := render.NewPath()
		ops := []render.PathBooleanOp{render.PathOpUnion, render.PathOpIntersect, render.PathOpDifference, render.PathOpXor}
		for _, op := range ops {
			if got := render.BooleanPath(nil, nil, op); got.NumVerbs() != 0 {
				t.Errorf("op %d nil/nil should be empty, got %d verbs", op, got.NumVerbs())
			}
			if got := render.BooleanPath(nil, rect, op); op == render.PathOpUnion || op == render.PathOpXor {
				if got.NumVerbs() != rect.NumVerbs() {
					t.Errorf("op %d nil/rect should clone rect, got %d verbs", op, got.NumVerbs())
				}
			} else if got.NumVerbs() != 0 {
				t.Errorf("op %d nil/rect should be empty, got %d verbs", op, got.NumVerbs())
			}
			if got := render.BooleanPath(empty, rect, op); op == render.PathOpUnion || op == render.PathOpXor {
				if got.NumVerbs() != rect.NumVerbs() {
					t.Errorf("op %d empty/rect should clone rect, got %d verbs", op, got.NumVerbs())
				}
			} else if got.NumVerbs() != 0 {
				t.Errorf("op %d empty/rect should be empty, got %d verbs", op, got.NumVerbs())
			}
		}
	})

	t.Run("line-only path has no area", func(t *testing.T) {
		line := render.NewPath()
		line.MoveTo(0, 0)
		line.LineTo(10, 0)
		res := line.Op(rect, render.PathOpIntersect)
		for _, p := range [][2]float64{{5, 0}, {5, 5}, {0, 0}} {
			if boolInside(res, p[0], p[1]) {
				t.Errorf("zero-area line should not fill (%v,%v)", p[0], p[1])
			}
		}
		res2 := line.Op(rect, render.PathOpUnion)
		if !boolInside(res2, 5, 5) {
			t.Error("line ∪ rect should keep the rect region")
		}
	})

	t.Run("spike retrace keeps triangle region", func(t *testing.T) {
		spike := render.NewPath()
		spike.MoveTo(0, 0)
		spike.LineTo(10, 0)
		spike.LineTo(5, 0)
		spike.LineTo(5, 5)
		spike.Close()
		res := spike.Op(boolRect(-20, -20, 60, 60), render.PathOpIntersect)
		if !boolInside(res, 4, 1) {
			t.Error("(4,1) should be inside the surviving triangle")
		}
		if boolInside(res, 1, 4) {
			t.Error("(1,4) should be outside the surviving triangle")
		}
	})

	t.Run("nan coordinates do not corrupt output", func(t *testing.T) {
		p := render.NewPath()
		p.MoveTo(math.NaN(), 0)
		p.LineTo(10, 0)
		p.LineTo(10, 10)
		p.LineTo(0, math.NaN())
		p.Close()
		res := p.Op(boolRect(0, 0, 20, 20), render.PathOpIntersect)
		for i, v := range res.Coords() {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("coord %d is not finite: %v", i, v)
			}
		}
	})

	t.Run("huge coordinates are clamped", func(t *testing.T) {
		// 远超钳位上限：不崩、坐标有限（几何被钳到边界，区域收缩）
		huge := boolRect(1e7, 1e7, 100, 100)
		res := huge.Op(boolRect(1e7, 1e7, 10, 10), render.PathOpDifference)
		for i, v := range res.Coords() {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("coord %d is not finite: %v", i, v)
			}
			if math.Abs(v) > 1.0e6+1 {
				t.Fatalf("coord %d exceeds clamp bound: %v", i, v)
			}
		}

		// 钳位范围内的大坐标：区域与孔都保留
		far := boolRect(1e5, 1e5, 100, 100)
		hole := boolRect(1e5+20, 1e5+20, 10, 10)
		res2 := far.Op(hole, render.PathOpDifference)
		if got := len(boolContours(res2)); got != 2 {
			t.Fatalf("far frame should keep 2 contours, got %d", got)
		}
		if !boolInside(res2, 1e5+5, 1e5+5) || boolInside(res2, 1e5+25, 1e5+25) {
			t.Error("far frame should keep band filled and hole empty")
		}
	})

	t.Run("lone close verb does not panic", func(t *testing.T) {
		p := render.NewPath()
		p.Close() // 首个 verb 即 Close（isEmpty 为 false）
		res := p.Op(rect, render.PathOpUnion)
		if !boolInside(res, 5, 5) {
			t.Error("close-only path ∪ rect should keep the rect region")
		}
		if got := p.Op(rect, render.PathOpIntersect).NumVerbs(); got != 0 {
			t.Errorf("close-only path ∩ rect should be empty, got %d verbs", got)
		}
	})

	t.Run("self operand difference is empty", func(t *testing.T) {
		res := rect.Op(rect, render.PathOpDifference)
		if res.NumVerbs() != 0 {
			t.Fatalf("rect - rect should be empty, got %d verbs", res.NumVerbs())
		}
	})
}

func BenchmarkBooleanPathFrameDifference(b *testing.B) {
	outer := boolRect(16, 16, 480, 480)
	inner := boolRect(96, 96, 320, 320)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		res := outer.Op(inner, render.PathOpDifference)
		if res.NumVerbs() == 0 {
			b.Fatal("empty result")
		}
	}
}

func BenchmarkBooleanPathCirclesUnion(b *testing.B) {
	a := render.NewPath()
	a.Circle(100, 100, 80)
	c := render.NewPath()
	c.Circle(150, 100, 80)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		res := a.Op(c, render.PathOpUnion)
		if res.NumVerbs() == 0 {
			b.Fatal("empty result")
		}
	}
}
