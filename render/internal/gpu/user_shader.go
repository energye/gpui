//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

//go:build !nogpu

package gpu

import (
	"fmt"
	"sync"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

// P3.2 user shader pass 1: evaluate a user WGSL fragment over the painted
// bounds into a premul RGBA offscreen RT (later composited by the coverage /
// R8-mask chain). The engine owns the vertex stage (fullscreen triangle with
// uv); the user module supplies `fn fs_main(@location(0) uv: vec2<f32>) ->
// vec4<f32>` (premultiplied output) plus, optionally, a @group(0) uniform
// block and one texture+sampler pair at bindings (1)/(2).

const userShaderVS = `
struct VSOut {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vi: u32) -> VSOut {
    var p = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>( 3.0, -1.0),
        vec2<f32>(-1.0,  3.0),
    );
    var uv = array<vec2<f32>, 3>(
        vec2<f32>(0.0, 1.0),
        vec2<f32>(2.0, 1.0),
        vec2<f32>(0.0, -1.0),
    );
    var o: VSOut;
    o.pos = vec4<f32>(p[vi], 0.0, 1.0);
    o.uv = uv[vi];
    return o;
}
`

type userShaderCache struct {
	mu          sync.Mutex
	initialized bool
	device      hal.Device
	// vs module is shared; the fs module is per-effect and owned by the
	// entry's pipeline record below.
	vsModule hal.ShaderModule
	entries  map[*render.ShaderEffect]*userShaderPipeline
}

type userShaderPipeline struct {
	module     hal.ShaderModule
	uniform    hal.Buffer
	uniformLen int
	bgl        hal.BindGroupLayout
	layout     hal.PipelineLayout
	pipeline   hal.RenderPipeline
	hasTex     bool
}

func (c *userShaderCache) ensureLocked(device hal.Device) error {
	if c.initialized && c.device == device {
		return nil
	}
	if c.initialized && c.device != device {
		c.destroyLocked()
	}
	vs, err := device.CreateShaderModule(&hal.ShaderModuleDescriptor{WGSL: userShaderVS})
	if err != nil {
		return fmt.Errorf("user shader: vs module: %w", err)
	}
	c.vsModule = vs
	c.device = device
	c.initialized = true
	c.entries = make(map[*render.ShaderEffect]*userShaderPipeline)
	return nil
}

func (c *userShaderCache) destroyLocked() {
	if c.vsModule != nil {
		c.vsModule.Destroy()
		c.vsModule = nil
	}
	for _, p := range c.entries {
		p.destroy()
	}
	c.entries = nil
	c.initialized = false
	c.device = nil
}

func (p *userShaderPipeline) destroy() {
	if p.pipeline != nil {
		p.pipeline.Destroy()
	}
	if p.layout != nil {
		p.layout.Destroy()
	}
	if p.bgl != nil {
		p.bgl.Destroy()
	}
	if p.uniform != nil {
		p.uniform.Destroy()
	}
	if p.module != nil {
		p.module.Destroy()
	}
}

// ensureEffect builds (or reuses) the pipeline for one effect. The uniform
// buffer is re-uploaded whenever its contents change; the bound image, if
// any, is uploaded once per distinct image.
func (rc *GPURenderContext) ensureUserShaderPipeline(device hal.Device, queue hal.Queue, effect *render.ShaderEffect) (*userShaderPipeline, error) {
	c := &rc.shared.userShader
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensureLocked(device); err != nil {
		return nil, err
	}
	if p, ok := c.entries[effect]; ok {
		return p, nil
	}

	uniforms := effect.Uniforms()
	hasTex := effect.Image() != nil

	// Bind group layout: uniform always; texture+sampler when bound.
	entries := []types.BindGroupLayoutEntry{
		{
			Binding:    0,
			Visibility: types.ShaderStageFragment,
			Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: uint64(maxInt(len(uniforms), 16))},
		},
	}
	if hasTex {
		entries = append(entries,
			types.BindGroupLayoutEntry{Binding: 1, Visibility: types.ShaderStageFragment, Texture: &types.TextureBindingLayout{SampleType: types.TextureSampleTypeFloat, ViewDimension: types.TextureViewDimension2D}},
			types.BindGroupLayoutEntry{Binding: 2, Visibility: types.ShaderStageFragment, Sampler: &types.SamplerBindingLayout{Type: types.SamplerBindingTypeFiltering}},
		)
	}
	bgl, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{Label: "user_shader_bgl", Entries: entries})
	if err != nil {
		return nil, fmt.Errorf("user shader: bgl: %w", err)
	}
	layout, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{Label: "user_shader_layout", BindGroupLayouts: []hal.BindGroupLayout{bgl}})
	if err != nil {
		bgl.Destroy()
		return nil, fmt.Errorf("user shader: layout: %w", err)
	}
	fs, err := device.CreateShaderModule(&hal.ShaderModuleDescriptor{WGSL: effect.WGSL()})
	if err != nil {
		layout.Destroy()
		bgl.Destroy()
		return nil, fmt.Errorf("user shader: fs module: %w", err)
	}
	sl := uint32(dualTexUniformSlotStride)
	layoutSize := maxInt(int(sl), (len(uniforms)+int(sl)-1)/int(sl)*int(sl))
	ubuf, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label: "user_shader_uniform",
		Size:  uint64(layoutSize),
		Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
	})
	if err != nil {
		fs.Destroy()
		layout.Destroy()
		bgl.Destroy()
		return nil, fmt.Errorf("user shader: uniform buffer: %w", err)
	}
	pipe, err := device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:       "user_shader_pipe",
		Layout:      layout,
		Vertex:      hal.VertexState{Module: c.vsModule, EntryPoint: "vs_main"},
		Primitive:   triangleListPrimitive(),
		Multisample: types.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &hal.FragmentState{
			Module:     fs,
			EntryPoint: "fs_main",
			Targets: []types.ColorTargetState{{
				Format:    types.TextureFormatBGRA8Unorm,
				WriteMask: types.ColorWriteMaskAll,
				Blend: &types.BlendState{
					Color: types.BlendComponent{Operation: types.BlendOperationAdd, SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorZero},
					Alpha: types.BlendComponent{Operation: types.BlendOperationAdd, SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorZero},
				},
			}},
		},
	})
	if err != nil {
		ubuf.Destroy()
		fs.Destroy()
		layout.Destroy()
		bgl.Destroy()
		return nil, fmt.Errorf("user shader: pipeline: %w", err)
	}
	p := &userShaderPipeline{module: fs, uniform: ubuf, uniformLen: len(uniforms), bgl: bgl, layout: layout, pipeline: pipe, hasTex: hasTex}
	c.entries[effect] = p
	return p, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// layoutBindGroupLayout exposes the pipeline's bind group layout for the
// per-draw bind group.
func (p *userShaderPipeline) layoutBindGroupLayout() hal.BindGroupLayout { return p.bgl }
