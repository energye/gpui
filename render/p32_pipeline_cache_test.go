//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// 深审 L2-P1-3（D8）钉：shader 管线缓存的绑定接口必须由 shader 声明推导，
// 不能取「首绘实时值」永久缓存。旧状：BGL 的 MinBindingSize/纹理有无取首绘
// 那一刻的 len(uniforms)/Image()，之后 SetImage/换 uniform 尺寸 → bind group
// 与缓存 BGL 对不上 → 整帧回退 CPU（画漏=背景色）。

const d8UniformWGSL = `
struct U { a: vec4<f32>, b: vec4<f32> }
@group(0) @binding(0) var<uniform> u: U;
@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(0.0, u.a.x + u.b.x + 1.0, 0.0, 1.0);
}
`

const d8ImageWGSL = `
@group(0) @binding(1) var tex: texture_2d<f32>;
@group(0) @binding(2) var samp: sampler;
@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(tex, samp, uv);
}
`

func d8Effect(t *testing.T, wgsl string) *render.ShaderEffect {
	t.Helper()
	e, err := render.NewShaderEffect(wgsl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}

// d8Draw fills (4,4,24,24) with the shader paint and returns the center pixel
// R,G (0..255) plus the CPU-fallback count. White = draw was skipped
// (background); red = ambient-solid regression.
func d8Draw(t *testing.T, e *render.ShaderEffect) (int, int, int) {
	t.Helper()
	dc := render.NewContext(32, 32)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 0, 0) // ambient 红：若回退成「画环境色」也当场露馅
	dc.SetShader(e)
	dc.DrawRectangle(4, 4, 24, 24)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	r, g, _, _ := dc.Image().At(16, 16).RGBA()
	st := dc.RenderPathStats()
	return int(r >> 8), int(g >> 8), st.CPUFallbackOps
}

func d8Image(r, g, b uint8) *render.ImageBuf {
	img, err := render.NewImageBuf(4, 4, render.FormatRGBA8)
	if err != nil {
		panic(err)
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			_ = img.SetRGBA(x, y, r, g, b, 255)
		}
	}
	return img
}

// 首绘 uniform=32B 建缓存，二绘缩到 16B：bind 尺寸必须仍过布局校验。
func TestD8UniformShrinkAfterWarmup(t *testing.T) {
	requireFilterOpsGPU(t)
	e := d8Effect(t, d8UniformWGSL)
	if err := e.SetUniformBytes(make([]byte, 32)); err != nil {
		t.Fatalf("uniform 32B: %v", err)
	}
	r, g, fb := d8Draw(t, e)
	if g < 200 || r > 40 {
		t.Fatalf("warmup draw not green: (%d,%d) fb=%d", r, g, fb)
	}
	if err := e.SetUniformBytes(make([]byte, 16)); err != nil {
		t.Fatalf("uniform 16B: %v", err)
	}
	r, g, fb = d8Draw(t, e)
	if g < 200 || r > 40 || fb != 0 {
		t.Fatalf("after uniform 32->16 shrink: (%d,%d) fb=%d — 首绘毒化（BGL 取首绘实时 32B，二绘 bind 16B 校验失败回退）", r, g, fb)
	}
}

// shader 声明了纹理但首绘没绑图，之后 SetImage：管线布局必须已经带纹理槽。
func TestD8LateSetImage(t *testing.T) {
	requireFilterOpsGPU(t)
	e := d8Effect(t, d8ImageWGSL)
	r, g, fb := d8Draw(t, e)
	if r > 40 || g > 40 || fb != 0 {
		t.Fatalf("warmup draw (no image, placeholder) not black: (%d,%d) fb=%d", r, g, fb)
	}
	e.SetImage(d8Image(0, 255, 0))
	r, g, fb = d8Draw(t, e)
	if g < 200 || r > 40 || fb != 0 {
		t.Fatalf("after late SetImage: (%d,%d) fb=%d — 首绘毒化（BGL 无纹理槽，bind group 建不出来）", r, g, fb)
	}
}

// 反向：首绘绑了图，之后 SetImage(nil)：不许崩、不许画环境色，占位纹理=黑。
func TestD8SetImageNilAfterWarmup(t *testing.T) {
	requireFilterOpsGPU(t)
	e := d8Effect(t, d8ImageWGSL)
	e.SetImage(d8Image(0, 255, 0))
	r, g, fb := d8Draw(t, e)
	if g < 200 || r > 40 || fb != 0 {
		t.Fatalf("warmup draw (with image) not green: (%d,%d) fb=%d", r, g, fb)
	}
	e.SetImage(nil)
	r, g, fb = d8Draw(t, e)
	if r > 40 || g > 40 || fb != 0 {
		t.Fatalf("after SetImage(nil): (%d,%d) fb=%d — 首绘毒化（BGL 带纹理槽，bind 缺槽回退；期望占位黑）", r, g, fb)
	}
}

// 占位 1x1 黑纹理+sampler 是缓存级单例：反复缺图绘制不得逐帧涨显存账。
func TestD8PlaceholderNoLeak(t *testing.T) {
	requireFilterOpsGPU(t)
	e := d8Effect(t, d8ImageWGSL)
	d8Draw(t, e) // 首绘建占位 + 管线
	base := hal.VramLiveCount()
	for i := 0; i < 4; i++ {
		if r, g, fb := d8Draw(t, e); r > 40 || g > 40 || fb != 0 {
			t.Fatalf("draw %d not black: (%d,%d) fb=%d", i, r, g, fb)
		}
	}
	if after := hal.VramLiveCount(); after > base+4 {
		t.Fatalf("placeholder/draw path grows live handle count %d -> %d", base, after)
	}
}
