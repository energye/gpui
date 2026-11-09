//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package render

import "testing"

// TestAcc6StrokeNoSingleRowInnerFill 钉住批②挂起项「acc=6 单行内填」。
//
// 几何：showcase 的蓝虚线圆/实线圆在 570×357 烘焙后的设备坐标
// （cx=540*570/1024, cy=520*s+(357-640*s)/2, r=46*s, width=6*s）。
// 现象：acc=6 时圆内某一行（虚线 y=284/289、实线 y=281）被整条填满；
// acc=2 与 resvg 都是左右两段笔画。根因是描边轮廓的 inner edge 比 outer
// edge 晚一个 1/64 子条带开始，全局 winding 在中间那段没有闭合，filler
// 把 outer-left 与 outer-right 配成一条横跨圆内的梯形。
//
// 断言不看 mean：直接检查圆内中段（x=290..310）的 alpha，任何一行出现
// 整段墨都算红；同时保留 acc=2 作为逐位护栏（生产默认档）。
func TestAcc6StrokeNoSingleRowInnerFill(t *testing.T) {
	const scale = 570.0 / 1024.0
	cx, cy := 540*scale, 520*scale+(357.0-640*scale)/2
	rad, width := 46*scale, 6*scale

	cases := []struct {
		name string
		dash *Dash
		rows []int
	}{
		{"dashed_round_cap", NewDash(30*scale, 14*scale), []int{284, 289}},
		{"solid_round_cap", nil, []int{281}},
	}
	for _, tc := range cases {
		for _, acc := range []int32{2, 6} {
			r := NewSoftwareRenderer(570, 357)
			r.edgeBuilder.SetPixelAccuracy(acc)
			pm := NewPixmap(570, 357)
			pm.Clear(Transparent)
			p := NewPath()
			p.Circle(cx, cy, rad)
			paint := NewPaint()
			paint.SetBrush(SolidRGBA(0, 0, 1, 1))
			paint.SetStroke(Stroke{Width: width, Cap: LineCapRound, Join: LineJoinRound, MiterLimit: 4, Dash: tc.dash})
			paint.TransformScale = 1
			if err := r.Stroke(pm, p, paint); err != nil {
				t.Fatalf("%s acc=%d: %v", tc.name, acc, err)
			}
			for _, y := range tc.rows {
				// 圆心水平弦中段必须没有墨；acc=6 单行内填会给出 alpha≈40。
				for x := 290; x <= 310; x++ {
					if a := pm.Data()[(y*570+x)*4+3]; a > 8 {
						t.Fatalf("%s acc=%d row=%d: inner fill alpha=%d at x=%d (want <=8); "+
							"acc=6 stroke inner edge must not merge the left/right bands",
							tc.name, acc, y, a, x)
					}
				}
				// 左右两段笔画仍然要有墨，防止「整行都没画」的假绿。
				if a := pm.Data()[(y*570+275)*4+3]; a < 32 {
					t.Fatalf("%s acc=%d row=%d: left band alpha=%d at x=275 (want >=32)", tc.name, acc, y, a)
				}
				if a := pm.Data()[(y*570+325)*4+3]; a < 32 {
					t.Fatalf("%s acc=%d row=%d: right band alpha=%d at x=325 (want >=32)", tc.name, acc, y, a)
				}
			}
		}
	}
}
