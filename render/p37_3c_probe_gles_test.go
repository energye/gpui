//go:build !nogpu

package render_test

// P3.7 批 3c 前置探针（design.md §4.3「必做前置」）：binding 3+ 多纹理对与大
// uniform 块经 gles naga→GLSL 翻译链的真编译实测。契约扩展（多对纹理/预算
// 放开）动代码之前，先证后端真吃得了——结论落
// evidence/P3.7/batch3c/probe-gles-bindings.txt，缺证据不动手。

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	_ "github.com/energye/gpui/gpu/gwgpu/gles"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

const p37CMultiTexWGSL = `
struct Uniforms {
    mix_level: f32,
    _pad0: vec3<f32>,
}
@group(0) @binding(0) var<uniform> u: Uniforms;
@group(0) @binding(1) var p37c_tex0: texture_2d<f32>;
@group(0) @binding(2) var p37c_samp0: sampler;
@group(0) @binding(3) var p37c_tex1: texture_2d<f32>;
@group(0) @binding(4) var p37c_samp1: sampler;
@group(0) @binding(5) var p37c_tex2: texture_2d<f32>;
@group(0) @binding(6) var p37c_samp2: sampler;

struct VSOut {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vi: u32) -> VSOut {
    var p = array<vec2<f32>, 6>(
        vec2<f32>(0.0, 0.0), vec2<f32>(32.0, 0.0), vec2<f32>(32.0, 32.0),
        vec2<f32>(0.0, 0.0), vec2<f32>(32.0, 32.0), vec2<f32>(0.0, 32.0));
    var o: VSOut;
    let q = p[vi];
    o.pos = vec4<f32>(q.x / 16.0 - 1.0, 1.0 - q.y / 16.0, 0.0, 1.0);
    o.uv = q / 32.0;
    return o;
}

@fragment
fn fs_main(in: VSOut) -> @location(0) vec4<f32> {
    let a = textureSample(p37c_tex0, p37c_samp0, in.uv);
    let b = textureSample(p37c_tex1, p37c_samp1, in.uv);
    let c = textureSample(p37c_tex2, p37c_samp2, in.uv);
    return mix(a, (b + c) * 0.5, u.mix_level);
}`

const p37CBigUniformWGSL = `
struct BigUniforms {
    // 272B：超旧 256B 契约预算（16B 对齐）
    coeffs: array<vec4<f32>, 17>,
}
@group(0) @binding(0) var<uniform> u: BigUniforms;

struct VSOut {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vi: u32) -> VSOut {
    var p = array<vec2<f32>, 6>(
        vec2<f32>(0.0, 0.0), vec2<f32>(32.0, 0.0), vec2<f32>(32.0, 32.0),
        vec2<f32>(0.0, 0.0), vec2<f32>(32.0, 32.0), vec2<f32>(0.0, 32.0));
    var o: VSOut;
    let q = p[vi];
    o.pos = vec4<f32>(q.x / 16.0 - 1.0, 1.0 - q.y / 16.0, 0.0, 1.0);
    o.uv = q / 32.0;
    return o;
}

@fragment
fn fs_main(in: VSOut) -> @location(0) vec4<f32> {
    let k = u.coeffs[0].x + u.coeffs[16].w;
    return vec4<f32>(0.25 + k * 0.0, 0.5, 0.75, 1.0);
}`

// p37CGLDevice 引导一台真 gles 设备（照 conformance setupGLDevice 同款，
// 但不接管 accelerator——探针直接用 hal.Device）。
func p37CGLDevice(t *testing.T) hal.Device {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY: pure-Go GL needs an X11 context-provider window")
	}
	be, ok := hal.GetBackend(gputypes.BackendGL)
	if !ok || be == nil {
		t.Skip("BackendGL not registered")
	}
	inst, err := be.CreateInstance(&hal.InstanceDescriptor{Backends: gputypes.BackendsPrimary})
	if err != nil {
		t.Skipf("GL CreateInstance: %v", err)
	}
	t.Cleanup(inst.Release)
	xw := memOpenX11(t, 64, 64)
	t.Cleanup(xw.close)
	surf, err := inst.CreateSurface(hal.SurfaceTarget{
		Kind:          hal.SurfaceTargetXlibWindow,
		DisplayHandle: xw.display,
		WindowHandle:  xw.window,
	})
	if err != nil {
		t.Skipf("GL CreateSurface(Xlib): %v", err)
	}
	t.Cleanup(surf.Destroy)
	ad, err := inst.RequestAdapter(&hal.RequestAdapterOptions{
		PowerPreference:   gputypes.PowerPreferenceHighPerformance,
		CompatibleSurface: surf,
	})
	if err != nil {
		t.Skipf("GL RequestAdapter: %v", err)
	}
	t.Cleanup(ad.Release)
	dev, err := ad.RequestDevice(render.DeviceDescriptor("p37c-probe"))
	if err != nil {
		t.Skipf("GL RequestDevice: %v", err)
	}
	t.Cleanup(dev.Release)
	return dev
}

// TestP37CProbeGLESMultiTextureBindings gles 真编译：binding 3+/4+/5+ 三纹理对
// + 大 uniform（272B > 旧 256B 预算）。CreateRenderPipeline 内即 naga→GLSL
// 翻译 + GL 编译，任一步拒绝即探针红。
func TestP37CProbeGLESMultiTextureBindings(t *testing.T) {
	dev := p37CGLDevice(t)

	// limits 读数进证据（预算放开的上限依据）。
	l := dev.Limits()
	t.Logf("gles limits: MaxSampledTexturesPerShaderStage=%d MaxSamplersPerShaderStage=%d MaxBindingsPerBindGroup=%d MaxUniformBufferBindingSize=%d MinUniformBufferOffsetAlignment=%d",
		l.MaxSampledTexturesPerShaderStage, l.MaxSamplersPerShaderStage, l.MaxBindingsPerBindGroup, l.MaxUniformBufferBindingSize, l.MinUniformBufferOffsetAlignment)

	mod, err := dev.CreateShaderModule(&hal.ShaderModuleDescriptor{Label: "p37c_multi_tex", WGSL: p37CMultiTexWGSL})
	if err != nil {
		t.Fatalf("gles CreateShaderModule (bindings 0..6): %v", err)
	}
	defer mod.Destroy()

	uniSize := uint64(16)
	entries := []gputypes.BindGroupLayoutEntry{
		{Binding: 0, Visibility: gputypes.ShaderStageFragment, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: uniSize}},
		{Binding: 1, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D}},
		{Binding: 2, Visibility: gputypes.ShaderStageFragment, Sampler: &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering}},
		{Binding: 3, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D}},
		{Binding: 4, Visibility: gputypes.ShaderStageFragment, Sampler: &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering}},
		{Binding: 5, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D}},
		{Binding: 6, Visibility: gputypes.ShaderStageFragment, Sampler: &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering}},
	}
	bgl, err := dev.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{Label: "p37c_bgl", Entries: entries})
	if err != nil {
		t.Fatalf("gles CreateBindGroupLayout (bindings 0..6): %v", err)
	}
	defer bgl.Destroy()
	layout, err := dev.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{Label: "p37c_layout", BindGroupLayouts: []hal.BindGroupLayout{bgl}})
	if err != nil {
		t.Fatalf("gles CreatePipelineLayout: %v", err)
	}
	defer layout.Destroy()
	bs := gputypes.BlendStatePremultiplied()
	pipe, err := dev.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:       "p37c_multi_tex_pipe",
		Layout:      layout,
		Vertex:      hal.VertexState{Module: mod, EntryPoint: "vs_main"},
		Primitive:   gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &hal.FragmentState{
			Module:     mod,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format:    gputypes.TextureFormatBGRA8Unorm,
				WriteMask: gputypes.ColorWriteMaskAll,
				Blend:     &bs,
			}},
		},
	})
	if err != nil {
		t.Fatalf("gles CreateRenderPipeline (naga→GLSL 翻译 + 编译, bindings 0..6): %v", err)
	}
	pipe.Destroy()

	// 大 uniform（272B > 旧 256B 契约预算）。
	mod2, err := dev.CreateShaderModule(&hal.ShaderModuleDescriptor{Label: "p37c_big_uniform", WGSL: p37CBigUniformWGSL})
	if err != nil {
		t.Fatalf("gles CreateShaderModule (272B uniform): %v", err)
	}
	defer mod2.Destroy()
	bgl2, err := dev.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{Label: "p37c_bgl2",
		Entries: []gputypes.BindGroupLayoutEntry{
			{Binding: 0, Visibility: gputypes.ShaderStageFragment, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: 272}},
		}})
	if err != nil {
		t.Fatalf("gles CreateBindGroupLayout (272B uniform): %v", err)
	}
	defer bgl2.Destroy()
	layout2, err := dev.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{Label: "p37c_layout2", BindGroupLayouts: []hal.BindGroupLayout{bgl2}})
	if err != nil {
		t.Fatalf("gles CreatePipelineLayout (272B): %v", err)
	}
	defer layout2.Destroy()
	pipe2, err := dev.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:       "p37c_big_uniform_pipe",
		Layout:      layout2,
		Vertex:      hal.VertexState{Module: mod2, EntryPoint: "vs_main"},
		Primitive:   gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &hal.FragmentState{
			Module:     mod2,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format:    gputypes.TextureFormatBGRA8Unorm,
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		t.Fatalf("gles CreateRenderPipeline (272B uniform): %v", err)
	}
	pipe2.Destroy()
	t.Logf("probe gles: bindings 0..6 三纹理对 + 272B uniform 全链真编译 OK")
}

var _ = fmt.Sprintf
