//go:build !nogpu

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// P3.7 批 6 ①：合批账的端到端钉。批 1a 的合并发生在 session 里，单测只能验算式；
// 这条钉的是「真画一遍，账本说出几条 draw、最长一段几张」——① 窗的探针用的就是
// 同一个口，所以窗里断言的语义先在这里立住（H12 的反面：夹一笔路径必须断链）。
package render_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

func p376PlateQuad(cx, cy, hw, hh, near, far float64) ([4]render.Point, [4]float64, bool) {
	m, ok := render.ProjectorMatrix4(520, render.Pt(cx, cy), render.Point{})
	if !ok {
		return [4]render.Point{}, [4]float64{}, false
	}
	world := [4]render.Point{
		{X: -hw, Y: -hh}, {X: hw, Y: -hh},
		{X: hw, Y: hh}, {X: -hw, Y: hh},
	}
	return m.ProjectQuad(world, [4]float64{far, far, near, near})
}

func p376QuadTexture(t *testing.T, w, h int) *render.ImageBuf {
	t.Helper()
	buf, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if err := buf.SetRGBA(x, y, 200, uint8(40+x*8), uint8(60+y*8), 255); err != nil {
				t.Fatalf("SetRGBA: %v", err)
			}
		}
	}
	return buf
}

func p376RequireGPU(t *testing.T) {
	t.Helper()
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
}

// 连续同透视 quad 必须合成一段；中间夹一笔描边就得断链（H12 的形态）。
func TestP376ImageBatchPerspectiveRunMerges(t *testing.T) {
	p376RequireGPU(t)
	const w, h = 320, 200
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ClearWithColor(render.RGBA{R: 0.05, G: 0.05, B: 0.08, A: 1})
	tex := p376QuadTexture(t, 16, 16)

	// 段一：4 张连续同透视 quad，中间只有图像命令。
	dc.ResetImageBatchStats()
	for i := 0; i < 4; i++ {
		corners, weights, ok := p376PlateQuad(float64(60+i*55), 90, 24, 20, 260, 420)
		if !ok {
			t.Fatalf("plate %d 投影被拒", i)
		}
		if err := dc.DrawImageQuad(tex, corners, render.QuadDrawOptions{
			Perspective: true, Weights: weights, Interpolation: render.InterpNearest, Opacity: 1,
		}); err != nil {
			t.Fatalf("DrawImageQuad %d: %v", i, err)
		}
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	merged := dc.ImageBatchStats()
	t.Logf("连续四张 → %+v", merged)
	if merged.PerspDraws != 1 || merged.MaxPerspRun != 4 {
		t.Fatalf("连续 4 张同透视 quad 没合成一段：draws=%d 最长=%d，want 1/4",
			merged.PerspDraws, merged.MaxPerspRun)
	}
	if merged.Draws != 1 {
		t.Errorf("整帧图像 draw = %d，want 1（这一帧只有图像层在动）", merged.Draws)
	}

	// 段二：同样 4 张，每张之间插一笔描边 ⇒ 路径层与图像层交错触发分段。
	dc.ResetImageBatchStats()
	for i := 0; i < 4; i++ {
		corners, weights, ok := p376PlateQuad(float64(60+i*55), 90, 24, 20, 260, 420)
		if !ok {
			t.Fatalf("plate %d 投影被拒", i)
		}
		if err := dc.DrawImageQuad(tex, corners, render.QuadDrawOptions{
			Perspective: true, Weights: weights, Interpolation: render.InterpNearest, Opacity: 1,
		}); err != nil {
			t.Fatalf("DrawImageQuad %d: %v", i, err)
		}
		dc.SetRGBA(1, 1, 1, 1)
		dc.SetLineWidth(2)
		dc.DrawRectangle(float64(20+i*10), 150, 30, 20)
		_ = dc.Stroke()
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU(带描边): %v", err)
	}
	split := dc.ImageBatchStats()
	t.Logf("夹描边四张 → %+v", split)
	if split.PerspDraws != 4 || split.MaxPerspRun != 1 {
		t.Fatalf("夹一笔路径后仍被合并：draws=%d 最长=%d，want 4/1 —— 这就是 H12 说的那条障碍",
			split.PerspDraws, split.MaxPerspRun)
	}

	// 复位交回旧值并清零（窗逐段计量的前提）。
	if prev := dc.ResetImageBatchStats(); prev.PerspDraws == 0 {
		t.Fatal("复位没交回旧账")
	}
	if after := dc.ImageBatchStats(); after != (render.ImageBatchStats{}) {
		t.Fatalf("复位后必须是零值账本：%+v", after)
	}
}

// 材质臂同样记账：批 3a 的合并段要能在窗里被断言成「op 数不随材质张数线性炸」。
func TestP376ImageBatchMaterialRunCounted(t *testing.T) {
	p376RequireGPU(t)
	const w, h = 200, 160
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ClearWithColor(render.RGBA{A: 1})
	tex := p376QuadTexture(t, 16, 16)

	// 恒等材质（用户 shader 直接输出精灵色）画三张连续 quad：材质臂一段或三次
	// 都算账，这里只断言「材质臂计数被记进账本」这一件事——臂级合并语义由
	// internal 算式钉与 ③ 材质窗各自负责。
	ident, err := render.NewShaderEffect(`
struct Uniforms { tint: vec4<f32> }
@group(0) @binding(0) var<uniform> u: Uniforms;
@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(uv.x, uv.y, u.tint.b, 1.0);
}
`)
	if err != nil {
		t.Skipf("shader effect 不受支持: %v", err)
	}
	defer ident.Close()
	for i := 0; i < 3; i++ {
		corners, _, ok := p376PlateQuad(float64(50+i*45), 80, 18, 16, 240, 380)
		if !ok {
			t.Fatal("plate 投影被拒")
		}
		if err := dc.DrawImageQuad(tex, corners, render.QuadDrawOptions{
			Interpolation: render.InterpNearest, Opacity: 1, Material: ident,
		}); err != nil {
			t.Fatalf("材质 quad %d: %v", i, err)
		}
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	st := dc.ImageBatchStats()
	t.Logf("三张材质 quad → %+v", st)
	if st.MaterialDraws == 0 {
		t.Fatalf("材质臂没进账本：%+v（③ 窗要靠它断言「op 数不随材质张数线性炸」）", st)
	}
	if float64(st.MaterialDraws) > float64(st.Quads)+1 {
		t.Errorf("材质 draw 数 %d 超过 quad 数 %d ⇒ op 数随张数线性炸", st.MaterialDraws, st.Quads)
	}
	if math.IsNaN(float64(st.MaxRun)) {
		t.Fatal("MaxRun NaN")
	}
}
