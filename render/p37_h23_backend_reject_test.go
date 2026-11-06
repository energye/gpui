//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// H23（P3.7 批 6 立）：一枚「前端放行、后端 naga 拒收」的用户材质模块，挂在
// 图像口/quad 口上时必须与填充腿同形——具名 + 计数 + 整笔拒画，绝不掀掉整帧。
// 判据与方案 docs/render_gpu_converge/evidence/P3.7/batch6/h23-assessment.md。
package render_test

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
)

// h23BadWGSL：uniform 地址空间要求数组 stride 是 16 的倍数，`array<f32,64>` 的
// stride=4 ⇒ 前端 NewShaderEffect 放行、naga 在 CreateShaderModule 拒（本机实测，
// 证据 batch6/gap3-backend-reject-probe.txt）。
const h23BadWGSL = `
struct BadAlign {
    vals: array<f32, 64>,
}
@group(0) @binding(0) var<uniform> u: BadAlign;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let v = u.vals[0];
    return vec4<f32>(uv.x, uv.y, 0.5, 1.0) + vec4<f32>(v, v, v, 0.0);
}`

// h23Solid 纯色源图（对照腿与材质腿共用）。
func h23Solid(t *testing.T, w, h int, r, g, b uint8) *render.ImageBuf {
	t.Helper()
	buf, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if err := buf.SetRGBA(x, y, r, g, b, 255); err != nil {
				t.Fatal(err)
			}
		}
	}
	return buf
}

// h23Near：与「同一张图上从没被任何笔画过的裸底像素」比对（自校准，不猜量化）。
func h23Near(t *testing.T, out *render.ImageBuf, label string, x, y, bareX, bareY int) {
	t.Helper()
	br, bg, bb, _ := out.GetRGBA(bareX, bareY)
	r, g, b, _ := out.GetRGBA(x, y)
	if abs3(int(r)-int(br)) > 1 || abs3(int(g)-int(bg)) > 1 || abs3(int(b)-int(bb)) > 1 {
		t.Errorf("%s：(%d,%d)=(%d,%d,%d) 与裸底 (%d,%d,%d) 不符（差>1）", label, x, y, r, g, b, br, bg, bb)
	}
}

func abs3(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func h23Setup(t *testing.T) {
	t.Helper()
	requireNativeGPU(t)
	os.Unsetenv("GOGPU_RENDER_MODE")
}

func h23BadEffect(t *testing.T) *render.ShaderEffect {
	t.Helper()
	e, err := render.NewShaderEffect(h23BadWGSL)
	if err != nil {
		t.Fatalf("前置事实不成立：本机前端应当放行这枚模块（后端才拒），got %v", err)
	}
	if err := e.SetUniformBytes(make([]byte, 256)); err != nil {
		t.Fatalf("SetUniformBytes: %v", err)
	}
	return e
}

// 钉 1：图像口——后端拒收不许掀帧，必须「计数 + 该笔不落墨」，同帧其余内容照常。
func TestH23ImageLegDoesNotBreakFrame(t *testing.T) {
	h23Setup(t)
	mat := h23BadEffect(t)
	defer func() { _ = mat.Close() }()
	src := h23Solid(t, 32, 32, 210, 40, 40)
	defer src.Dispose()

	dc := render.NewContext(200, 200)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.RGBA{R: 0.25, G: 0.25, B: 0.25, A: 1})
	// 对照：同帧一张无材质图，必须照常出图（帧没被掀的证据）。
	dc.DrawImageEx(src, render.DrawImageOptions{X: 10, Y: 10, DstWidth: 60, DstHeight: 60})
	dc.DrawImageEx(src, render.DrawImageOptions{X: 120, Y: 120, DstWidth: 60, DstHeight: 60, Material: mat})
	before := dc.RenderPathStats()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("H23 现形：图像口后端拒收把整帧 flush 掀了 → %v", err)
	}
	after := dc.RenderPathStats()
	if after.ShaderBackendFails-before.ShaderBackendFails < 1 {
		t.Errorf("拒收未计数：ShaderBackendFails %d→%d", before.ShaderBackendFails, after.ShaderBackendFails)
	}
	if after.MaterialSkips-before.MaterialSkips < 1 {
		t.Errorf("拒画未计 MaterialSkips：%d→%d", before.MaterialSkips, after.MaterialSkips)
	}
	var out *render.ImageBuf
	if !dc.ExportImageBuf(&out) || out == nil {
		t.Fatal("ExportImageBuf 失败")
	}
	defer out.Dispose()
	if r, g, b, _ := out.GetRGBA(40, 40); r < 150 || g > 100 || b > 100 {
		t.Errorf("对照腿（无材质）没画出：(40,40)=(%d,%d,%d) want≈源图 (210,40,40)", r, g, b)
	}
	h23Near(t, out, "材质腿落墨了（拒而不画的契约破了）", 150, 150, 5, 190)
}

// 钉 2：quad 口——同一条路，同一种形态。
func TestH23QuadLegDoesNotBreakFrame(t *testing.T) {
	h23Setup(t)
	mat := h23BadEffect(t)
	defer func() { _ = mat.Close() }()
	src := h23Solid(t, 32, 32, 60, 90, 210)
	defer src.Dispose()

	dc := render.NewContext(200, 200)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.RGBA{R: 0.25, G: 0.25, B: 0.25, A: 1})
	dc.DrawImageEx(src, render.DrawImageOptions{X: 10, Y: 10, DstWidth: 60, DstHeight: 60})
	if err := dc.DrawImageQuad(src, [4]render.Point{
		{X: 120, Y: 120}, {X: 180, Y: 120}, {X: 180, Y: 180}, {X: 120, Y: 180},
	}, render.QuadDrawOptions{Material: mat}); err != nil {
		t.Fatalf("DrawImageQuad 入队期就报错（本洞是构建期，不是校验期）：%v", err)
	}
	before := dc.RenderPathStats()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("H23 现形：quad 口后端拒收把整帧 flush 掀了 → %v", err)
	}
	after := dc.RenderPathStats()
	if after.ShaderBackendFails-before.ShaderBackendFails < 1 {
		t.Errorf("拒收未计数：ShaderBackendFails %d→%d", before.ShaderBackendFails, after.ShaderBackendFails)
	}
	var out *render.ImageBuf
	if !dc.ExportImageBuf(&out) || out == nil {
		t.Fatal("ExportImageBuf 失败")
	}
	defer out.Dispose()
	h23Near(t, out, "quad 口材质腿应整笔拒画", 150, 150, 5, 190)
}

// 钉 3：负面缓存的公开可观察面——计数**一帧一次**，第二帧起入口就拒（不再每帧
// 重新尝试编译），且第二帧的 DrawImageQuad 必须给回可辨识的具名错误。
func TestH23RejectionIsRememberedPerFrame(t *testing.T) {
	h23Setup(t)
	mat := h23BadEffect(t)
	defer func() { _ = mat.Close() }()
	src := h23Solid(t, 32, 32, 40, 200, 40)
	defer src.Dispose()

	dc := render.NewContext(200, 200)
	defer func() { _ = dc.Close() }()
	frame := func() (fails, skips int) {
		dc.ClearWithColor(render.RGBA{R: 0.25, G: 0.25, B: 0.25, A: 1})
		_ = dc.DrawImageQuad(src, [4]render.Point{
			{X: 120, Y: 120}, {X: 180, Y: 120}, {X: 180, Y: 180}, {X: 120, Y: 180},
		}, render.QuadDrawOptions{Material: mat})
		b := dc.RenderPathStats()
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("flush 被掀：%v", err)
		}
		a := dc.RenderPathStats()
		return a.ShaderBackendFails - b.ShaderBackendFails, a.MaterialSkips - b.MaterialSkips
	}
	f1, s1 := frame()
	if f1 < 1 {
		t.Fatalf("首帧未计数：fails=%d skips=%d", f1, s1)
	}
	f2, s2 := frame()
	if f2 != 0 {
		t.Errorf("第二帧又去编译了一次（无负面缓存）：fails 帧1=%d 帧2=%d", f1, f2)
	}
	if s2 < 1 {
		t.Errorf("第二帧入口没按已知不可用拒画：skips 帧1=%d 帧2=%d", s1, s2)
	}
}

// 钉 4：填充腿现状不许被这次修复改动（本来就是对的形态：具名 + 计数 + CPU 回退，
// 区内不落墨）。
func TestH23FillLegUnchanged(t *testing.T) {
	h23Setup(t)
	mat := h23BadEffect(t)
	defer func() { _ = mat.Close() }()

	dc := render.NewContext(200, 200)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.RGBA{R: 0.25, G: 0.25, B: 0.25, A: 1})
	before := dc.RenderPathStats()
	dc.SetRGB(1, 0, 0)
	dc.SetShader(mat)
	dc.DrawRectangle(80, 80, 60, 60)
	if err := dc.Fill(); err != nil {
		t.Logf("填充腿 Fill 返回（现状允许带具名错）：%v", err)
	}
	dc.SetShader(nil)
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("填充腿不该掀帧：%v", err)
	}
	after := dc.RenderPathStats()
	if after.ShaderBackendFails-before.ShaderBackendFails < 1 {
		t.Errorf("填充腿计数回退：ShaderBackendFails %d→%d", before.ShaderBackendFails, after.ShaderBackendFails)
	}
	if after.CPUFallbackOps-before.CPUFallbackOps < 1 {
		t.Errorf("填充腿回退计数回退：CPUFallbackOps %d→%d", before.CPUFallbackOps, after.CPUFallbackOps)
	}
	var out *render.ImageBuf
	if !dc.ExportImageBuf(&out) || out == nil {
		t.Fatal("ExportImageBuf 失败")
	}
	defer out.Dispose()
	h23Near(t, out, "填充腿画出了别的东西", 110, 110, 5, 190)

	// 第二帧（H23 负面缓存）：设备不再碰 ⇒ ShaderBackendFails 零增量，而这一帧的
	// 像素确实仍来自 CPU 腿 ⇒ CPUFallbackOps 照记。
	b2 := dc.RenderPathStats()
	dc.ClearWithColor(render.RGBA{R: 0.25, G: 0.25, B: 0.25, A: 1})
	dc.SetRGB(1, 0, 0)
	dc.SetShader(mat)
	dc.DrawRectangle(80, 80, 60, 60)
	_ = dc.Fill()
	dc.SetShader(nil)
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("第二帧填充腿被掀：%v", err)
	}
	a2 := dc.RenderPathStats()
	if a2.ShaderBackendFails-b2.ShaderBackendFails != 0 {
		t.Errorf("填充腿重复帧又打了一次设备编译：fails %d→%d（want 不变）",
			b2.ShaderBackendFails, a2.ShaderBackendFails)
	}
	if a2.CPUFallbackOps-b2.CPUFallbackOps < 1 {
		t.Errorf("填充腿重复帧没记 CPU 回退：cpu_fb %d→%d", b2.CPUFallbackOps, a2.CPUFallbackOps)
	}
}
