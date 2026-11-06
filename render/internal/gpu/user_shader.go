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
// bounds into a premul RGBA offscreen RT (un-premultiplied into the straight
// data the image-draw composite chain expects, then carried through the
// coverage / R8-mask chain). The engine owns the vertex stage (fullscreen
// triangle with uv); the user module supplies `fn fs_main(@location(0) uv:
// vec2<f32>) -> vec4<f32>` (premultiplied output) plus, optionally, a
// @group(0) uniform block and one texture+sampler pair at bindings (1)/(2).

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

// shaderUniformMinBindingSize lays out binding(0) at the contract's full
// uniform budget (the engine's 256B slot stride), independent of the live
// uniform length.
const shaderUniformMinBindingSize = uint64(render.ShaderUniformBudget)

type userShaderCache struct {
	mu          sync.Mutex
	initialized bool
	device      hal.Device
	// vs module is shared; the fs module is per-effect and owned by the
	// entry's pipeline record below.
	vsModule hal.ShaderModule
	entries  map[*render.ShaderEffect]*userShaderPipeline
	// rejected = H23 负面缓存（填充腿）：这台设备拒过的模块记一次，之后每帧直接
	// 当「不可用」回退 CPU，不再重打设备、不再每帧刷一整段 stderr。
	rejected map[*render.ShaderEffect]error
	// black 1x1 texture+sampler stands in for an unbound declared image slot
	// (placeholder-binding pattern, as in Skia/Impeller and Dawn's default
	// bindings): the bind group always matches the layout derived from the
	// shader module, whatever SetImage state the draw happens to have.
	blackTex  hal.Texture
	blackView hal.TextureView
	blackSamp hal.Sampler
}

type userShaderPipeline struct {
	module      hal.ShaderModule
	uniform     hal.Buffer
	uniformSize int // full buffer size bound at binding(0) (layout-derived)
	bgl         hal.BindGroupLayout
	layout      hal.PipelineLayout
	pipeline    hal.RenderPipeline
	hasTex      bool
	// texturePairs is the declared pair count (batch 3c); the bind group must
	// feed every declared pair (slot 0 = SetImage/fill slot).
	texturePairs int
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
	c.rejected = make(map[*render.ShaderEffect]error)
	return nil
}

func (c *userShaderCache) destroyLocked() {
	if c.vsModule != nil {
		c.vsModule.Destroy()
		c.vsModule = nil
	}
	if c.blackView != nil {
		c.blackView.Destroy()
		c.blackView = nil
	}
	if c.blackTex != nil {
		c.blackTex.Destroy()
		c.blackTex = nil
	}
	if c.blackSamp != nil {
		c.blackSamp.Destroy()
		c.blackSamp = nil
	}
	for _, p := range c.entries {
		p.destroy()
	}
	c.entries = nil
	c.rejected = nil
	c.initialized = false
	c.device = nil
}

// ensureBlackBindings lazily creates the 1x1 black texture + filtering sampler
// backing the placeholder image slot (cache mutex held).
func (c *userShaderCache) ensureBlackBindings(device hal.Device, queue hal.Queue) error {
	if c.blackTex != nil && c.blackSamp != nil {
		return nil
	}
	view := c.blackView
	tex := c.blackTex
	if tex == nil {
		var err error
		tex, view, err = dualTexCreateTex(device, queue, "user_shader_black", 1, 1,
			[]byte{0, 0, 0, 255}, types.TextureUsageTextureBinding|types.TextureUsageCopyDst)
		if err != nil {
			return fmt.Errorf("user shader: black texture: %w", err)
		}
	}
	samp := c.blackSamp
	if samp == nil {
		var err error
		samp, err = device.CreateSampler(&hal.SamplerDescriptor{
			MagFilter: types.FilterModeLinear, MinFilter: types.FilterModeLinear,
		})
		if err != nil {
			tex.Destroy()
			view.Destroy()
			return fmt.Errorf("user shader: black sampler: %w", err)
		}
	}
	c.blackTex, c.blackView, c.blackSamp = tex, view, samp
	return nil
}

// evictEffect drops the pipeline cache of one closed effect (L2-P0-1:
// render.ShaderEffect.Close must not leave the fs module, uniform buffer,
// bind group layout, pipeline layout, or pipeline on the device ledger).
func (c *userShaderCache) evictEffect(effect *render.ShaderEffect) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if p, ok := c.entries[effect]; ok {
		delete(c.entries, effect)
		delete(c.rejected, effect)
		p.destroy()
	}
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

// ensureEffect builds (or reuses) the pipeline for one effect. The bind
// layout comes from the effect's declared shader interface (module-derived,
// immutable); uniform contents and the bound image are re-uploaded per pass.
func (rc *GPURenderContext) ensureUserShaderPipeline(device hal.Device, queue hal.Queue, effect *render.ShaderEffect) (*userShaderPipeline, error) {
	c := &rc.shared.userShader
	c.mu.Lock()
	defer c.mu.Unlock()
	if effect == nil || effect.Closed() {
		return nil, fmt.Errorf("%w: user shader ensure after Close", render.ErrShaderClosed)
	}
	if err := c.ensureLocked(device); err != nil {
		return nil, err
	}
	if p, ok := c.entries[effect]; ok {
		return p, nil
	}
	// H23 负面缓存：这台设备拒过这枚模块 ⇒ 零设备调用，直接把具名错还给调用方
	// （调用方据此走普通 CPU 回退，不再每帧重打设备、不再每帧刷 stderr）。
	if err, bad := c.rejected[effect]; bad {
		return nil, err
	}

	uniforms := effect.Uniforms()
	// L2-P1-3: the bind layout is derived from the shader module's declared
	// bindings, never from first-draw runtime state. The uniform slot is laid
	// out at max(256B default budget, declared uniform size) (batch 3c, D4
	// 预算放开)，and the declared texture+sampler pairs ride bindings
	// (1,2), (3,4), … — so later SetImageAt/SetImageAt(nil) or uniform-size
	// changes can never outdate the cached layout.
	bind := effect.ShaderBinding()
	hasTex := bind.TexturePairs > 0

	uniMin := uint64(shaderUniformMinBindingSize)
	if bind.UniformBytes > int(shaderUniformMinBindingSize) {
		uniMin = uint64(bind.UniformBytes)
	}
	entries := []types.BindGroupLayoutEntry{
		{
			Binding:    0,
			Visibility: types.ShaderStageFragment,
			Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: uniMin},
		},
	}
	for k := 0; k < bind.TexturePairs; k++ {
		entries = append(entries,
			types.BindGroupLayoutEntry{Binding: uint32(2*k + 1), Visibility: types.ShaderStageFragment, Texture: &types.TextureBindingLayout{SampleType: types.TextureSampleTypeFloat, ViewDimension: types.TextureViewDimension2D}},
			types.BindGroupLayoutEntry{Binding: uint32(2*k + 2), Visibility: types.ShaderStageFragment, Sampler: &types.SamplerBindingLayout{Type: types.SamplerBindingTypeFiltering}},
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
	// 与材质腿同款：wgpu 的 shader 校验是排队的，模块创建必须包在验证错误域里，
	// pop 回来才算它自己的账（H23）。
	device.PushErrorScope(hal.ErrorFilterValidation)
	fs, err := device.CreateShaderModule(&hal.ShaderModuleDescriptor{WGSL: effect.ComposedFillWGSL()})
	if scoped := device.PopErrorScope(); scoped != nil && err == nil {
		err = scoped
	}
	if err != nil {
		reject := fmt.Errorf("%w: user shader fs module: %w", render.ErrShaderBackendCompile, err)
		layout.Destroy()
		bgl.Destroy()
		c.rejected[effect] = reject
		return nil, reject
	}
	sl := uint32(dualTexUniformSlotStride)
	layoutSize := maxInt(maxInt(int(sl), bind.UniformBytes), (len(uniforms)+int(sl)-1)/int(sl)*int(sl))
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
		reject := fmt.Errorf("%w: user shader pipeline: %w", render.ErrShaderBackendCompile, err)
		ubuf.Destroy()
		fs.Destroy()
		layout.Destroy()
		bgl.Destroy()
		c.rejected[effect] = reject
		return nil, reject
	}
	p := &userShaderPipeline{module: fs, uniform: ubuf, uniformSize: layoutSize, bgl: bgl, layout: layout, pipeline: pipe, hasTex: hasTex, texturePairs: bind.TexturePairs}
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
