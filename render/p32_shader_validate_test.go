//go:build !nogpu

package render_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
)

// 深审 L2-P1-5（D10）钉：NewShaderEffect 入口必须做契约级校验（对齐 Flutter
// FragmentProgram.forShader 编译期拒绝）。旧状只到语法（Parse/Lower/Validate），
// 以下六类全部收进来，绘制时才按后端分叉炸（native fb+stderr、gles 画错、
// metal 各异）——「双端一致」不成立。

func d9Accepts(t *testing.T, name, wgsl string) {
	t.Helper()
	if _, err := render.NewShaderEffect(wgsl); err == nil {
		t.Fatalf("%s: entry must reject, got accept", name)
	} else {
		t.Logf("%s rejected: %v", name, err)
	}
}

func TestD10NoFragmentEntry(t *testing.T) {
	d9Accepts(t, "no-fs", `
@vertex fn vs_main() -> @builtin(position) vec4<f32> { return vec4<f32>(0.0); }
`)
}

func TestD10ComputeOnly(t *testing.T) {
	d9Accepts(t, "compute-only", `
@compute @workgroup_size(1) fn cs_main() { }
`)
}

func TestD10Vec3Return(t *testing.T) {
	d9Accepts(t, "vec3-return", `
@fragment fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec3<f32> {
	return vec3<f32>(1.0, 0.0, 0.0);
}
`)
}

func TestD10ScalarReturn(t *testing.T) {
	d9Accepts(t, "scalar-return", `
@fragment fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) f32 {
	return 1.0;
}
`)
}

func TestD10UniformOverBudget(t *testing.T) {
	// 17 个 vec4 = 288B > 256B 契约预算（规范 WGSL struct：成员逐行分号结尾）
	var b strings.Builder
	b.WriteString("struct Big {\n")
	for i := 0; i < 18; i++ {
		fmt.Fprintf(&b, "    m%d: vec4<f32>,\n", i)
	}
	b.WriteString("}\n")
	b.WriteString("@group(0) @binding(0) var<uniform> u: Big;\n")
	b.WriteString("@fragment fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> { return u.m0; }\n")
	d9Accepts(t, "uniform-288B", b.String())
}

func TestD10SamplerWithoutTexture(t *testing.T) {
	// 实测：sampler 声明在 binding(2) 而无 binding(1) 纹理，Parse/Lower/Validate
	// 三关全绿进缓存（textureSampleLevel(samp, samp, …) 这种非法形态前端也放行）。
	d9Accepts(t, "sampler-no-texture", `
@group(0) @binding(2) var samp: sampler;
@fragment fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> { return vec4<f32>(1.0); }
`)
}

func TestD10TextureOutsideContractSlot(t *testing.T) {
	// 纹理放在契约外的槽位（binding 3）而 sampler 在 2：入口必须按契约槽位收。
	d9Accepts(t, "tex-at-3", `
@group(0) @binding(3) var tex: texture_2d<f32>;
@group(0) @binding(2) var samp: sampler;
@fragment fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
	return textureSample(tex, samp, uv);
}
`)
}

func TestD10F64Uniform(t *testing.T) {
	d9Accepts(t, "f64", `
struct U { x: f64, pad: f32 }
@group(0) @binding(0) var<uniform> u: U;
@fragment fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
	return vec4<f32>(f32(u.x), 0.0, 0.0, 1.0);
}
`)
}

// 正例不受牵连：现有合法模块族必须继续过入口。
func TestD10ValidModulesStillAccepted(t *testing.T) {
	for i, src := range []string{
		`@fragment fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> { return vec4<f32>(1.0, 0.0, 0.0, 1.0); }`,
		`@fragment fn fs_main() -> @location(0) vec4<f32> { return vec4<f32>(1.0); }`,
		d8UniformWGSL, d8ImageWGSL, d9HalfRedWGSL,
	} {
		if _, err := render.NewShaderEffect(src); err != nil {
			t.Fatalf("valid module %d rejected: %v", i, err)
		}
	}
}
