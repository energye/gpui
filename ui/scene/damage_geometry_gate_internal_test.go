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
// 坐标（与 transformBounds 同口径：TransformPoint 已是逻辑坐标，c.matrix
// 只含用户矩阵，设备缩放由 trackDamage 在写 damage 时乘上）。
func transformedCorners(dc *render.Context, b image.Rectangle) []struct{ x, y float64 } {
	pts := []struct{ x, y float64 }{
		{float64(b.Min.X), float64(b.Min.Y)},
		{float64(b.Max.X), float64(b.Min.Y)},
		{float64(b.Max.X), float64(b.Max.Y)},
		{float64(b.Min.X), float64(b.Max.Y)},
	}
	out := make([]struct{ x, y float64 }, 0, 4)
	for _, p := range pts {
		tx, ty := dc.TransformPoint(p.x, p.y)
		out = append(out, struct{ x, y float64 }{tx, ty})
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

// TestTransformBoundsDeviceScaleLogical：transformBounds 的输出必须是
// surface-logical 坐标（TrackDamageRect / damage union 的域），设备缩放
// 只在 trackDamage 落盘时乘。c.matrix 只装用户矩阵（SetDeviceScale 明确
// 不碰它），所以 DeviceScale≠1 时再除一次 scale 会把 damage 缩成 1/scale
// ——高 DPI 下欠报，持久合成面会留残影。
func TestTransformBoundsDeviceScaleLogical(t *testing.T) {
	for _, scale := range []float64{1.5, 2.0, 3.0} {
		dc := render.NewContext(400, 300, render.WithDeviceScale(scale))
		b := image.Rect(10, 20, 100, 80)

		if got, want := transformBounds(dc, b), b; got != want {
			t.Errorf("scale=%.1f identity CTM: transformBounds=%v, want logical %v", scale, got, want)
		}

		dc.Push()
		dc.Translate(30, 40)
		got := transformBounds(dc, b)
		want := image.Rect(40, 60, 130, 120)
		dc.Pop()
		if got != want {
			t.Errorf("scale=%.1f translate(30,40): transformBounds=%v, want logical %v", scale, got, want)
		}
		dc.Close()
	}
}

// TestTransformBoundsToFrameDamagePhysical：端到端坐标链
// transformBounds（逻辑）→ TrackDamageRect（×scale 落盘）→ FrameDamage
// 必须是物理像素，正好是逻辑矩形的 scale 倍。高 DPI 下 final=逻辑尺寸
// 就是欠报（持久面按它剪裁 = 少刷 1/scale 的区域）。
func TestTransformBoundsToFrameDamagePhysical(t *testing.T) {
	dc := render.NewContext(400, 300, render.WithDeviceScale(2.0))
	defer dc.Close()
	dc.BeginFrame()
	b := image.Rect(10, 20, 100, 80)
	dc.TrackDamageRect(transformBounds(dc, b))
	rects := dc.FrameDamage()
	if len(rects) != 1 {
		t.Fatalf("FrameDamage=%v, want exactly one rect", rects)
	}
	if want := image.Rect(20, 40, 200, 160); rects[0] != want {
		t.Fatalf("physical damage=%v, want %v (logical %v × scale 2)", rects[0], want, b)
	}
}

// TestCompositeCullKeepsLogicalOverlapAtDeviceScale：cull 比较的两边
// （图层 footprint 与 damage union）都是逻辑坐标。高 DPI 下 footprint 被
// 误缩 1/scale 会与真实脏区错位，把该重画的子树整棵跳过 → 残影。
func TestCompositeCullKeepsLogicalOverlapAtDeviceScale(t *testing.T) {
	dc := render.NewContext(400, 300, render.WithDeviceScale(2.0))
	defer dc.Close()
	// 逻辑坐标：clip 覆盖 (0,0)-(80,40)，脏区 (60,20)-(100,60) 与之相交。
	l := &ClipRectLayer{X: 0, Y: 0, W: 80, H: 40}
	union := image.Rect(60, 20, 100, 60)
	if compositeCullable(l, dc, union, nil, nil) {
		t.Fatalf("layer footprint %v overlaps union %v but was culled at deviceScale=2",
			image.Rect(0, 0, 80, 40), union)
	}
}
