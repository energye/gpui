package render_test

// P3.7 批 3a 契约钉：fs_main 声明前缀（uv/tint/hom_w）+ 引擎模板合成。
// 判据源：batch3a-design.md §2.1/§2.2（拍板 A 模板合成 + T2 声明即接管）。
// 反向鉴别纪律：打断必须「仍在编译」；同名锚点限定窗口。

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render"
)

const p37AUvOnly = `@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(uv.x, uv.y, 0.0, 1.0);
}`

const p37AFullPrefix = `@fragment
fn fs_main(@location(0) uv: vec2<f32>, @location(1) tint: vec4<f32>, @location(2) hom_w: f32) -> @location(0) vec4<f32> {
    return vec4<f32>(uv.x * tint.r, uv.y * tint.g, hom_w * 0.5, tint.a);
}`

const p37AZeroParam = `@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(0.25, 0.5, 0.75, 1.0);
}`

// 合成腿钉：合法前缀放行 + 组合产物带引擎段 + 用户入口手术到位。
func TestP37AContractAcceptsDeclaredPrefixes(t *testing.T) {
	for name, wgsl := range map[string]string{
		"uv_only":    p37AUvOnly,
		"full_pre":   p37AFullPrefix,
		"zero_param": p37AZeroParam,
	} {
		e, err := render.NewShaderEffect(wgsl)
		if err != nil {
			t.Fatalf("%s: NewShaderEffect: %v", name, err)
		}
		defer e.Close()
		quad := e.ComposedQuadWGSL()
		fill := e.ComposedFillWGSL()
		if quad == "" || fill == "" {
			t.Fatalf("%s: composed variants empty", name)
		}
		if quad == wgsl || fill == wgsl {
			t.Fatalf("%s: composed module equals the raw source — 组合没发生（引擎段缺失）", name)
		}
		// 引擎段锚点（quad）：顶点模块、裁剪覆盖、输出段、用户函数调用点。
		for _, marker := range []string{"fn vs_main(", "gpui_eng_clip_cov", "gpui_eng_draw", "gpui_user_fs_main("} {
			if !strings.Contains(quad, marker) {
				t.Fatalf("%s: composed quad missing engine marker %q", name, marker)
			}
		}
		// 手术锚点：原入口声明不再以 @fragment 形态出现（T2 输出段接管）。
		if strings.Contains(quad, "@fragment\nfn fs_main(@location(0) uv") {
			t.Fatalf("%s: raw user entry survived the rewrite", name)
		}
		// fill 变体不引引擎裁剪（coverage 走 mask 链）。
		if strings.Contains(fill, "gpui_eng_clip") {
			t.Fatalf("%s: fill variant must not declare engine clip bindings", name)
		}
	}
}

// 全前缀的调用面：quad 包装按声明传三参（tint 由用户接管 ⇒ 输出段不再乘 tint）。
func TestP37AFullPrefixTintTakeover(t *testing.T) {
	e, err := render.NewShaderEffect(p37AFullPrefix)
	if err != nil {
		t.Fatalf("NewShaderEffect: %v", err)
	}
	defer e.Close()
	quad := e.ComposedQuadWGSL()
	if !strings.Contains(quad, "gpui_user_fs_main(in.tex_coord, in.tint, in.hom_w)") {
		t.Fatalf("full prefix call site missing — got:\n%s", quad)
	}
	// T2：用户声明 tint ⇒ 引擎输出段不再乘 in.tint（声明即接管）。
	if strings.Contains(quad, "* in.tint") {
		t.Fatalf("engine multiplied tint after a tint-declaring fragment — 双乘脚枪（T2 违约）")
	}
}

// uv-only 的调用面：引擎后乘 tint（灯光染色走到画面上的既有语义）。
func TestP37AUvOnlyEngineTint(t *testing.T) {
	e, err := render.NewShaderEffect(p37AUvOnly)
	if err != nil {
		t.Fatalf("NewShaderEffect: %v", err)
	}
	defer e.Close()
	if !strings.Contains(e.ComposedQuadWGSL(), "gpui_user_fs_main(in.tex_coord)") {
		t.Fatalf("uv-only call site must pass only uv")
	}
	if !strings.Contains(e.ComposedQuadWGSL(), "return user * opacity * in.tint;") {
		t.Fatalf("engine must post-multiply tint for a fragment that did not declare it")
	}
}

// 拒绝面：乱序 / 类型不符 / 超前缀 / 缺 @location / 位置错。
func TestP37AContractRejectsBadPrefixes(t *testing.T) {
	cases := map[string]string{
		"swapped": `@fragment
fn fs_main(@location(0) uv: vec2<f32>, @location(1) hom_w: f32) -> @location(0) vec4<f32> {
    return vec4<f32>(uv.x, uv.y, hom_w, 1.0);
}`,
		"bad_type": `@fragment
fn fs_main(@location(0) uv: vec3<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(uv, 1.0);
}`,
		"too_many": `@fragment
fn fs_main(@location(0) uv: vec2<f32>, @location(1) tint: vec4<f32>, @location(2) hom_w: f32, @location(3) extra: f32) -> @location(0) vec4<f32> {
    return vec4<f32>(uv, 1.0);
}`,
		"no_location": `@fragment
fn fs_main(uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(uv, 1.0);
}`,
		"wrong_slot": `@fragment
fn fs_main(@location(1) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(uv, 1.0);
}`,
	}
	for name, wgsl := range cases {
		e, err := render.NewShaderEffect(wgsl)
		if err == nil {
			e.Close()
			t.Fatalf("%s: expected rejection, got an effect", name)
		}
		if !strings.Contains(err.Error(), "contract") {
			t.Fatalf("%s: rejection must be a named contract error, got: %v", name, err)
		}
	}
}
