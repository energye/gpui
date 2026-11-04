//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// 深审 L2-P1-4（D9）钉：ShaderEffect 输出=预乘 alpha（Flutter 官方契约，拍板
// 2026-11-03）。旧状文档写「straight、引擎预乘」与实现（mask 链按 premul 直接
// 合成）互相矛盾，且无一条 A<1 的行为钉。此文件把拍板语义钉成可执行契约。

const d9HalfRedWGSL = `
@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(0.5, 0.0, 0.0, 0.5); // premul: rgb 已随 a=0.5 缩过
}
`

func d9Pixel(t *testing.T, wgsl string) (int, int, int, int) {
	t.Helper()
	e, err := render.NewShaderEffect(wgsl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	defer e.Close()
	dc := render.NewContext(32, 32)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 0, 0) // ambient 红：回退成实心色当场露馅
	dc.SetShader(e)
	dc.DrawRectangle(4, 4, 24, 24)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	r, g, b, a := dc.Image().At(16, 16).RGBA()
	return int(r >> 8), int(g >> 8), int(b >> 8), int(a >> 8)
}

// 契约腿钉：premul (0.5,0,0,0.5) 是 50% 纯红（premul 语义 rgb 已随 a 缩过），
// premul src-over 白底：r = 0.5 + 1×(1−0.5) = 1.0 → 255，g/b = 0 + 0.5 → 128，
// 即 (255,128,128)——与其自身注释「等价于 50% 纯红合成」一致的数学值。
// 2026-11-05 修正（探针 P3，evidence/P3.7/batch3a/probe-p3-premul.txt）：旧断言
// r=128±3 实际锁的是填充腿 premul 反交洞的回绕值（uint8(255+127)=382 mod 256=126，
// 被容差吞掉）；fillShaderAsImage 已改为交 premul（与图像腿契约/兄弟函数对齐），
// 本钉同步锁住正确语义。
func TestD9PremulOutputContract(t *testing.T) {
	requireFilterOpsGPU(t)
	r, g, b, a := d9Pixel(t, d9HalfRedWGSL)
	if d := r - 255; d < -3 || d > 3 {
		t.Fatalf("premul contract R=%d want 255±3 got (%d,%d,%d,%d) — 50%% premul 红合成白底必须全红（回绕洞回归？）", r, r, g, b, a)
	}
	if d := g - 128; d < -3 || d > 3 {
		t.Fatalf("G=%d want 128±3（白底 50%% 合成）got (%d,%d,%d,%d)", g, r, g, b, a)
	}
	if d := b - 128; d < -3 || d > 3 {
		t.Fatalf("B=%d want 128±3 got (%d,%d,%d,%d)", b, r, g, b, a)
	}
	if d := a - 255; d < -3 || d > 3 {
		t.Fatalf("A=%d want 255±3（dst 不透明 src-over 结果 alpha 饱和，直话）got (%d,%d,%d,%d)", a, r, g, b, a)
	}
}

// CPU-only：shader 绘制静默跳过、不计 cpu_fallback（契约 GPU-only 的 A<1 侧）。
func TestD9CPUModeSkips(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	e, err := render.NewShaderEffect(d9HalfRedWGSL)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	defer e.Close()
	dc := render.NewContext(16, 16)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 0, 0)
	dc.SetShader(e)
	dc.DrawRectangle(2, 2, 12, 12)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	pm := dc.PixmapForTest()
	data := pm.Data()
	for i := 0; i < len(data); i += 4 {
		if data[i] != 255 || data[i+1] != 255 || data[i+2] != 255 {
			t.Fatalf("CPU shader draw painted at %d: (%d,%d,%d,%d) — must skip", i, data[i], data[i+1], data[i+2], data[i+3])
		}
	}
	if st := dc.RenderPathStats(); st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fallback_ops=%d — GPU-only skip must not count as fallback", st.CPUFallbackOps)
	}
}
