//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package render_test

import (
	"image"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// P3.8 批 2 回归钉：GPU 径向 focal 渐变必须与 CPU ColorAt 逐点一致。
// 洞史：session cover 的 cover_textured_linear.wgsl focal 尾部曾直接
// return t_ray（缺 pointDist/intersectDist 换算），GPU 渲出的 focal 渐变
// 与 CPU 求值反相——P3.8 批 2 真窗 ui_wr_p38_svg_paint 卡 2 抓出，修复后
// 本钉沿同一 session-cover 链路对拍（GPU 加速器注册后的 render.Context）。
func TestP38RadialFocalGPUMatchesCPU(t *testing.T) {
	g := render.NewRadialGradientBrush(172, 30, 0, 150)
	g.SetFocus(110, 30)
	g.AddColorStop(0, render.RGBA{R: 1, A: 1})
	g.AddColorStop(1, render.RGBA{A: 1})

	dc := render.NewContext(344, 60)
	dc.SetFillBrush(g)
	dc.DrawRectangle(0, 0, 344, 60)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	img, ok := dc.Image().(*image.RGBA)
	if !ok {
		t.Fatal("image type")
	}
	bad := 0
	for _, x := range []int{0, 40, 80, 110, 120, 172, 220, 260, 300, 343} {
		c := img.RGBAAt(x, 30)
		// GPU samples the pixel CENTER; the CPU parity probe must use the
		// same point (the exact focal pixel t=0 is a measure-zero
		// degenerate — transparent per the tiny-skia mask — while the
		// pixel covering it evaluates at its center, t>0).
		cpu := g.ColorAt(float64(x)+0.5, 30.5)
		want := uint8(math.Round(math.Min(1, math.Max(0, cpu.R)) * 255))
		diff := int(c.R) - int(want)
		if diff < 0 {
			diff = -diff
		}
		if diff > 3 {
			bad++
			t.Errorf("radial focal GPU(%d,30) R=%d want≈%d (CPU %.3f)", x, c.R, want, cpu.R)
		}
	}
	if bad == 0 {
		t.Logf("radial focal GPU==CPU at 10 probes (max |diff|<=3)")
	}
}
