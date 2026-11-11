//go:build !nogpu

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gpu

import (
	"fmt"
	"math"
	"sync"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// M5 HDR 全链 P3（evidence/M5/assessment-design.md §3 G4）：
// float 面（RGBA16Float，线性 premul/直色）→ 8bit 的 tonemap 出口。
//
// 曲线与 render/hdr.FloatTarget.Encode 同一组数字（S66 冻结）：
//
//	曝光(0,16] → reinhard x/(1+x) → sRGB encode → 8bit 量化 floor(x*255+0.5)。
//
// 生产呈现（P4）与 CPU 金图/对拍共用这一条，禁止第二套曲线。
//
// 语义：float 面按「线性直色」解释（与 hdr.FloatTarget 一致），输出也是直色；
// 不透明内容 alpha=1 时直色=premul，P4 若接半透明层再单独定型。
const tonemapMaxExposure = 16.0

const tonemapWGSL = `
struct ToneParams {
    exposure: f32,
    _pad0: f32,
    _pad1: f32,
    _pad2: f32,
};
@group(0) @binding(0) var src_tex: texture_2d<f32>;
@group(0) @binding(1) var src_samp: sampler;
@group(0) @binding(2) var<uniform> params: ToneParams;

struct VSOut {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
};

@vertex fn vs(@builtin(vertex_index) vi: u32) -> VSOut {
    var p = array<vec2<f32>, 3>(vec2<f32>(-1.0, -1.0), vec2<f32>(3.0, -1.0), vec2<f32>(-1.0, 3.0));
    var out: VSOut;
    out.pos = vec4<f32>(p[vi], 0.0, 1.0);
    out.uv = vec2<f32>((p[vi].x + 1.0) * 0.5, 1.0 - (p[vi].y + 1.0) * 0.5);
    return out;
}

fn reinhard(x: f32) -> f32 {
    return x / (1.0 + x);
}

fn srgb_encode(x: f32) -> f32 {
    if (x <= 0.0031308) {
        return 12.92 * x;
    }
    return 1.055 * pow(x, 1.0 / 2.4) - 0.055;
}

fn quant8(x: f32) -> f32 {
    let c = clamp(x, 0.0, 1.0);
    return floor(c * 255.0 + 0.5) / 255.0;
}

@fragment fn fs(in: VSOut) -> @location(0) vec4<f32> {
    let c = textureSampleLevel(src_tex, src_samp, in.uv, 0.0);
    let e = params.exposure;
    let r = srgb_encode(reinhard(max(c.r * e, 0.0)));
    let g = srgb_encode(reinhard(max(c.g * e, 0.0)));
    let b = srgb_encode(reinhard(max(c.b * e, 0.0)));
    let a = clamp(c.a, 0.0, 1.0);
    return vec4<f32>(quant8(r), quant8(g), quant8(b), quant8(a));
}`

// tonemapPipeline owns the fullscreen tonemap pass. Created lazily by the
// session in P4; P3 drives it directly from tests.
type tonemapPipeline struct {
	device hal.Device
	queue  hal.Queue

	shader   hal.ShaderModule
	bgl      hal.BindGroupLayout
	layout   hal.PipelineLayout
	pipeline hal.RenderPipeline
	sampler  hal.Sampler
	uniform  hal.Buffer

	mu sync.Mutex // Run serializes the shared uniform buffer
}

// newTonemapPipeline builds the pass for the given 8-bit target format
// (zero = historical BGRA8).
func newTonemapPipeline(device hal.Device, queue hal.Queue, colorFormat types.TextureFormat) (*tonemapPipeline, error) {
	if device == nil || queue == nil {
		return nil, fmt.Errorf("tonemap: nil device/queue")
	}
	p := &tonemapPipeline{device: device, queue: queue}
	var err error
	if p.shader, err = device.CreateShaderModule(&hal.ShaderModuleDescriptor{Label: "tonemap_shader", WGSL: tonemapWGSL}); err != nil {
		return nil, fmt.Errorf("tonemap: shader: %w", err)
	}
	if p.bgl, err = device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "tonemap_bgl",
		Entries: []types.BindGroupLayoutEntry{
			{Binding: 0, Visibility: types.ShaderStageFragment, Texture: &types.TextureBindingLayout{SampleType: types.TextureSampleTypeFloat, ViewDimension: types.TextureViewDimension2D}},
			{Binding: 1, Visibility: types.ShaderStageFragment, Sampler: &types.SamplerBindingLayout{Type: types.SamplerBindingTypeFiltering}},
			{Binding: 2, Visibility: types.ShaderStageFragment, Buffer: &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform}},
		},
	}); err != nil {
		p.Destroy()
		return nil, fmt.Errorf("tonemap: bind group layout: %w", err)
	}
	if p.layout, err = device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{Label: "tonemap_pipe_layout", BindGroupLayouts: []hal.BindGroupLayout{p.bgl}}); err != nil {
		p.Destroy()
		return nil, fmt.Errorf("tonemap: pipeline layout: %w", err)
	}
	if p.pipeline, err = device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "tonemap_pipeline",
		Layout: p.layout,
		Vertex: hal.VertexState{Module: p.shader, EntryPoint: "vs"},
		Fragment: &hal.FragmentState{
			Module: p.shader, EntryPoint: "fs",
			Targets: []types.ColorTargetState{{
				Format:    colorFormatOr(colorFormat),
				WriteMask: types.ColorWriteMaskAll,
			}},
		},
		Primitive:   triangleListPrimitive(),
		Multisample: multisampleState(1),
	}); err != nil {
		p.Destroy()
		return nil, fmt.Errorf("tonemap: render pipeline: %w", err)
	}
	if p.sampler, err = device.CreateSampler(&hal.SamplerDescriptor{
		Label: "tonemap_sampler", MagFilter: types.FilterModeLinear, MinFilter: types.FilterModeLinear,
		AddressModeU: types.AddressModeClampToEdge, AddressModeV: types.AddressModeClampToEdge,
	}); err != nil {
		p.Destroy()
		return nil, fmt.Errorf("tonemap: sampler: %w", err)
	}
	if p.uniform, err = device.CreateBuffer(&hal.BufferDescriptor{
		Label: "tonemap_params", Size: 16,
		Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
	}); err != nil {
		p.Destroy()
		return nil, fmt.Errorf("tonemap: params buffer: %w", err)
	}
	return p, nil
}

// Run tonemaps src (RGBA16Float, linear) into dst (8-bit, colorFormat) with the
// given exposure and submits it. dst must be the same size as src.
func (p *tonemapPipeline) Run(src, dst hal.TextureView, exposure float64) error {
	if p == nil || p.pipeline == nil {
		return fmt.Errorf("tonemap: pipeline not built")
	}
	if src == nil || dst == nil {
		return fmt.Errorf("tonemap: nil view")
	}
	if math.IsNaN(exposure) || math.IsInf(exposure, 0) || exposure <= 0 || exposure > tonemapMaxExposure {
		return fmt.Errorf("tonemap: bad exposure %v", exposure)
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	var params [16]byte
	bits := math.Float32bits(float32(exposure))
	params[0], params[1], params[2], params[3] = byte(bits), byte(bits>>8), byte(bits>>16), byte(bits>>24)
	if err := p.queue.WriteBuffer(p.uniform, 0, params[:]); err != nil {
		return fmt.Errorf("tonemap: write params: %w", err)
	}
	bg, err := p.device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label: "tonemap_bg", Layout: p.bgl,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, TextureView: src},
			{Binding: 1, Sampler: p.sampler},
			{Binding: 2, Buffer: p.uniform, Size: 16},
		},
	})
	if err != nil {
		return fmt.Errorf("tonemap: bind group: %w", err)
	}
	defer bg.Destroy()

	enc, err := p.device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "tonemap_enc"})
	if err != nil {
		return fmt.Errorf("tonemap: encoder: %w", err)
	}
	rp, err := enc.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "tonemap_pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View: dst, LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
			ClearValue: types.Color{R: 0, G: 0, B: 0, A: 0},
		}},
	})
	if err != nil {
		return fmt.Errorf("tonemap: begin pass: %w", err)
	}
	rp.SetPipeline(p.pipeline)
	rp.SetBindGroup(0, bg, nil)
	rp.Draw(3, 1, 0, 0)
	if err := rp.End(); err != nil {
		return fmt.Errorf("tonemap: end pass: %w", err)
	}
	cmd, err := enc.Finish()
	if err != nil {
		return fmt.Errorf("tonemap: finish: %w", err)
	}
	defer p.device.FreeCommandBuffer(cmd)
	if _, err := p.queue.Submit(cmd); err != nil {
		return fmt.Errorf("tonemap: submit: %w", err)
	}
	return nil
}

// Destroy releases the pass resources.
func (p *tonemapPipeline) Destroy() {
	if p == nil {
		return
	}
	if p.uniform != nil {
		p.uniform.Destroy()
		p.uniform = nil
	}
	if p.sampler != nil {
		p.sampler.Destroy()
		p.sampler = nil
	}
	if p.pipeline != nil {
		p.pipeline.Destroy()
		p.pipeline = nil
	}
	if p.layout != nil {
		p.layout.Destroy()
		p.layout = nil
	}
	if p.bgl != nil {
		p.bgl.Destroy()
		p.bgl = nil
	}
	if p.shader != nil {
		p.shader.Destroy()
		p.shader = nil
	}
}
