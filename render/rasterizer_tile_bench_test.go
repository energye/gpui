package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
)

// 性能门禁：比较四种栅格模式（Analytic / SparseStrips / TileCompute / Auto）
// 在同一组形状上的单次填充耗时与分配量。用于 tile 栅格器修复前后对照，
// 以及后续改动不回归。
//
// 形状口径：
//   - Rect：整数对齐矩形（无 AA 边缘，最省）
//   - Diamond：分数坐标斜边（AA 边缘 + 分数行起止）
//   - BooleanDiff：布尔差集产物（生产真实复杂路径，输出碎片化）
//   - FrameClean：与 BooleanDiff 同形状（覆盖率逐位一致）的手写 8 条干净边，
//     用于把「碎片几何的固有成本」与「栅格器成本」分开计量
//   - ManyRects：64 个小矩形（多轮廓、多 coarse 条目）

func benchBuildRect(p *render.Path) {
	p.Rectangle(16, 16, 480, 480)
}

func benchBuildDiamond(p *render.Path) {
	p.MoveTo(256, 20.5)
	p.LineTo(492.25, 256.75)
	p.LineTo(256, 491.5)
	p.LineTo(20.75, 256.25)
	p.Close()
}

func benchBuildBooleanDiff(p *render.Path) {
	outer := render.NewPath()
	outer.Rectangle(16, 16, 480, 480)
	inner := render.NewPath()
	inner.Rectangle(96, 96, 320, 320)
	diff := outer.Op(inner, render.PathOpDifference)
	diff.Iterate(func(verb render.PathVerb, coords []float64) {
		switch verb {
		case render.MoveTo:
			p.MoveTo(coords[0], coords[1])
		case render.LineTo:
			p.LineTo(coords[0], coords[1])
		case render.Close:
			p.Close()
		}
	})
}

func benchBuildFrameClean(p *render.Path) {
	// 与 BooleanDiff 同形状（外框 16..496，孔 96..416），但手工书写为 8 条干净边。
	// 用于把「碎片几何的固有成本」与「栅格器成本」分开计量：两形状覆盖率逐位一致。
	p.MoveTo(16, 16)
	p.LineTo(496, 16)
	p.LineTo(496, 496)
	p.LineTo(16, 496)
	p.Close()
	// 内孔反向绕行（非零规则空洞）。
	p.MoveTo(96, 96)
	p.LineTo(96, 416)
	p.LineTo(416, 416)
	p.LineTo(416, 96)
	p.Close()
}

func benchBuildManyRects(p *render.Path) {
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			p.Rectangle(float64(16+col*60), float64(16+row*60), 44, 44)
		}
	}
}

func benchRasterFill(b *testing.B, mode render.RasterizerMode, w, h int, build func(p *render.Path)) {
	p := render.NewPath()
	build(p)
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.SetRasterizerMode(mode)
	dc.SetRGB(1, 0, 0)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := dc.FillPath(p); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRasterFill(b *testing.B) {
	const w, h = 512, 512
	shapes := []struct {
		name  string
		build func(p *render.Path)
	}{
		{"Rect", benchBuildRect},
		{"Diamond", benchBuildDiamond},
		{"BooleanDiff", benchBuildBooleanDiff},
		{"FrameClean", benchBuildFrameClean},
		{"ManyRects", benchBuildManyRects},
	}
	modes := []struct {
		name string
		mode render.RasterizerMode
	}{
		{"Analytic", render.RasterizerAnalytic},
		{"SparseStrips", render.RasterizerSparseStrips},
		{"TileCompute", render.RasterizerTileCompute},
		{"Auto", render.RasterizerAuto},
	}

	for _, shape := range shapes {
		for _, m := range modes {
			b.Run(shape.name+"/"+m.name, func(b *testing.B) {
				benchRasterFill(b, m.mode, w, h, shape.build)
			})
		}
	}
}

// BenchmarkRasterBackdropAlloc 单独度量 backdrop 记账的分配量（大画布，
// 暴露 cols*pixelRows 级分配）。
func BenchmarkRasterBackdropAlloc(b *testing.B) {
	const w, h = 1920, 1080
	p := render.NewPath()
	benchBuildBooleanDiff(p)
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.SetRasterizerMode(render.RasterizerSparseStrips)
	dc.SetRGB(1, 0, 0)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := dc.FillPath(p); err != nil {
			b.Fatal(err)
		}
	}
}
