// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//---------------------------------------------------------------------------
//
// P6 阶段 0 客观门禁（damage 几何）：damage 矩形必须**保守覆盖**实际绘制
// 区域的每一个角点。这是 Flutter `ComputeDamage` → `buffer_damage` 的语义
// 底线：少报 1px 就会在持久合成面剪刀下留下残影（P1 spin 拖尾根因）。
//
// 判据是逐角点的几何包含断言，不是 mean / 统计近似。

package scene

import (
	"image"
	"math"
	"testing"

	"github.com/energye/gpui/render"
)

// transformedCorners 返回矩形 b 四角在 dc 当前 CTM 下的 surface-logical
// 坐标（与 transformBounds 同口径：TransformPoint / deviceScale）。
func transformedCorners(dc *render.Context, b image.Rectangle) []struct{ x, y float64 } {
	scale := dc.DeviceScale()
	if scale <= 0 {
		scale = 1
	}
	pts := []struct{ x, y float64 }{
		{float64(b.Min.X), float64(b.Min.Y)},
		{float64(b.Max.X), float64(b.Min.Y)},
		{float64(b.Max.X), float64(b.Max.Y)},
		{float64(b.Min.X), float64(b.Max.Y)},
	}
	out := make([]struct{ x, y float64 }, 0, 4)
	for _, p := range pts {
		tx, ty := dc.TransformPoint(p.x, p.y)
		out = append(out, struct{ x, y float64 }{tx / scale, ty / scale})
	}
	return out
}

// assertCoversAllCorners 断言 got 保守覆盖 corners 的每一个角点：min 向下
// 取整、max 向上取整。任一漏点即 FAIL（damage 少报 = 残影）。
func assertCoversAllCorners(t *testing.T, got image.Rectangle, corners []struct{ x, y float64 }) {
	t.Helper()
	for i, c := range corners {
		cx, cy := int(math.Floor(c.x)), int(math.Ceil(c.y))
		fx, fy := int(math.Ceil(c.x)), int(math.Ceil(c.y))
		_ = cy
		// min 边界用 floor（向左上扩），max 边界用 ceil（向右下扩）
		if cx < got.Min.X || fx > got.Max.X || int(math.Floor(c.y)) < got.Min.Y || fy > got.Max.Y {
			t.Errorf("corner %d at (%.3f,%.3f) not covered by damage %v (want min≤(%.3f,%.3f), max≥(%.3f,%.3f))",
				i, c.x, c.y, got, c.x, c.y, c.x, c.y)
		}
	}
}

// TestDamageBoundsCoverRotatedQuad：旋转 cached 层的 damage 必须覆盖绘制
// quad 的四角。P0 用收紧的 e.bounds + int() 截断会漏掉角落 1px（spin 拖尾），
// 这个逐点断言会直接抓到；保守 floor/ceil 后转绿。
func TestDamageBoundsCoverRotatedQuad(t *testing.T) {
	for _, ang := range []float64{math.Pi / 6, math.Pi / 4, math.Pi / 3, 0.7} {
		dc := render.NewContext(200, 200)
		dc.Push()
		dc.Translate(100, 100)
		dc.Rotate(ang)
		// 模拟一张 36×36 的图标纹理 quad（含 1px AA 外扩由调用方给出，
		// 这里只验变换本身是否保守覆盖四角）。
		b := image.Rect(-18, -18, 18, 18)
		corners := transformedCorners(dc, b)
		got := transformBounds(dc, b)
		dc.Pop()
		dc.Close()
		assertCoversAllCorners(t, got, corners)
	}
}

// TestDamageBoundsCeilOnFarEdge：专门钉死 max 边界的 ceil 行为。构造一个
// 变换后坐标为非整数（含 .5+ 小数）的场景：int() 截断会少算 1px，ceil
// 才保守。这是 P1 spin 残影的第二处根因。
func TestDamageBoundsCeilOnFarEdge(t *testing.T) {
	dc := render.NewContext(200, 200)
	dc.Push()
	dc.Translate(37.5, 21.25)
	dc.Scale(1.3, 0.9)
	b := image.Rect(3, 7, 41, 53)
	corners := transformedCorners(dc, b)
	got := transformBounds(dc, b)
	dc.Pop()
	dc.Close()

	// 至少有一个角点变换后带小数，否则本用例失去意义（假绿防护）。
	fractional := false
	for _, c := range corners {
		if math.Abs(c.x-math.Floor(c.x)) > 0.05 || math.Abs(c.y-math.Floor(c.y)) > 0.05 {
			fractional = true
		}
	}
	if !fractional {
		t.Fatalf("test setup invalid: no fractional corner, cannot exercise ceil")
	}
	assertCoversAllCorners(t, got, corners)
}
