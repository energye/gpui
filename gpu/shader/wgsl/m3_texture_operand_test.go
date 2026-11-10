package wgsl

// M3 (D10 疑点转正，2026-11-11)：WGSL 前端按内建签名验 texture 调用操作数类型。
// 上游 naga 口径：texture/sampler 操作数类型错位（如 sampler 当 texture 传）在
// 编译期拒绝，绝不进 codegen——此前 `textureSampleLevel(samp, samp, …)` 三关全绿
// 进 IR，绘制期 GLSL 编译才炸（evidence/P2.6/m3-dissection.md）。
// 可解析（模块级绑定）必检；不可解析（函数参数/局部变量纹理）跳过不误伤。

import "testing"

const m3SamplerOnly = `
@group(0) @binding(0) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
	return textureSampleLevel(samp, samp, uv, 0.0);
}
`

const m3SamplerAsTexture = `
@group(0) @binding(1) var tex: texture_2d<f32>;
@group(0) @binding(0) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
	return textureSampleLevel(samp, samp, uv, 0.0);
}
`

const m3TextureAsSampler = `
@group(0) @binding(1) var tex: texture_2d<f32>;
@group(0) @binding(0) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
	return textureSampleLevel(tex, tex, uv, 0.0);
}
`

const m3SamplerInTextureLoad = `
@group(0) @binding(0) var samp: sampler;

@fragment
fn fs_main() -> @location(0) vec4<f32> {
	return textureLoad(samp, vec2<i32>(0, 0), 0i);
}
`

const m3LegalSample = `
@group(0) @binding(1) var tex: texture_2d<f32>;
@group(0) @binding(0) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
	return textureSampleLevel(tex, samp, uv, 0.0);
}
`

const m3TextureParam = `
@group(0) @binding(0) var samp: sampler;

fn sampleAt(t: texture_2d<f32>, s: sampler, uv: vec2<f32>) -> vec4<f32> {
	return textureSampleLevel(t, s, uv, 0.0);
}

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
	return sampleAt(textureParam(), samp, uv);
}

fn textureParam() -> texture_2d<f32> {
	var local: texture_2d<f32>;
	return local;
}
`

func lowerOrDie(t *testing.T, name, src string) error {
	t.Helper()
	tokens, err := NewLexer(src).Tokenize()
	if err != nil {
		t.Fatalf("%s: tokenize: %v", name, err)
	}
	ast, err := NewParser(tokens).Parse()
	if err != nil {
		t.Fatalf("%s: parse: %v", name, err)
	}
	_, lowerErr := Lower(ast)
	return lowerErr
}

func TestM3_SamplerAsTextureRejected(t *testing.T) {
	for _, name := range []string{"sampler_only", "texture_declared"} {
		src := m3SamplerOnly
		if name == "texture_declared" {
			src = m3SamplerAsTexture
		}
		err := lowerOrDie(t, name, src)
		if err == nil {
			t.Fatalf("%s: textureSampleLevel(samp, samp, …) must be rejected at lower time (M3)", name)
		}
	}
}

func TestM3_TextureAsSamplerRejected(t *testing.T) {
	if err := lowerOrDie(t, "texture_as_sampler", m3TextureAsSampler); err == nil {
		t.Fatal("textureSampleLevel(tex, tex, …) must be rejected at lower time (M3)")
	}
}

func TestM3_SamplerInTextureLoadRejected(t *testing.T) {
	if err := lowerOrDie(t, "sampler_in_textureLoad", m3SamplerInTextureLoad); err == nil {
		t.Fatal("textureLoad(samp, …) must be rejected at lower time (M3)")
	}
}

func TestM3_LegalFormsStillAccepted(t *testing.T) {
	if err := lowerOrDie(t, "legal_sample", m3LegalSample); err != nil {
		t.Fatalf("legal textureSampleLevel(tex, samp, …) must still lower: %v", err)
	}
}

func TestM3_TextureParamNotFalseRejected(t *testing.T) {
	if err := lowerOrDie(t, "texture_param", m3TextureParam); err != nil {
		t.Fatalf("texture passed through function params must not be falsely rejected: %v", err)
	}
}
