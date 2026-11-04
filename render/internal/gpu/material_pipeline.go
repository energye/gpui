//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2024
//
//----------------------------------------

//go:build !nogpu

package gpu

// P3.7 批 3a：真·quad 片元材质腿（D8 拍板）。一次绘制、无读回、可合批：
// 命令携带 *render.ShaderEffect（内部 ImageDrawCommand.Material，公开入口归
// 3b），绘制走组合模块（render 侧 NewShaderEffect 期已组合并重验）——
// 引擎顶点（36B：pos/uv/tint/hom_w，位置按 hom_w 缩放出真 clip-space w）+
// 用户函数 + 引擎输出段（opacity × RRect SDF 覆盖；tint 按 T2 由组合期定）。
//
// 绑定布局（组合模块声明派生，L2-P1-3 原则；跨腿共用同一 bgl 对象 ⇒ 绑定组
// 在 stencil/depth-clip 两腿间有效，同标准腿共享 uniformLayout 的形态）：
//   @group(0) = 用户原样：binding(0) uniform（恒布 256B 预算槽，未声明时绑
//               全零占位）、binding(1)/(2) 纹理对 = 精灵自己的纹理 + 命令采样
//               器（Godot TEXTURE/UV 同语义；SetImage 的图在材质路无槽可占，
//               多纹理归 3c）。
//   @group(1) = 引擎逐绘制块（transform + opacity，字节同 ImageUniforms，
//               slab 步长 256 对齐 WebGPU 绑定偏移）。
//   @group(2) = 标准裁剪绑定组（与 textured_quad 管线同一个 clipBindLayout
//               对象 ⇒ noClipBindGroup/裁剪组绑定组可直接复用）。
//
// 批 3a 口径：材质命令仿射（hom_w=1）+ Normal 混合；Material+Perspective 与
// 非 Normal 组合在 buildImageResources 具名拒绝（扩展归 3b）。CPU 求值器
// 不存在（P3.2 契约）——材质命令不进任何 CPU 腿，公开入口（3b）在队列前
// 拒绝，写进契约不假装支持。

import (
	"fmt"
	"sync"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

// materialEngineUniformSize is the engine per-draw block: transform (mat4x4)
// + opacity_pad (vec4) — byte-identical to the standard ImageUniforms block,
// so putImageUniform packs it too. Slab stride stays 256 (WebGPU uniform
// binding offsets align to 256, same convention as the image slab).
const materialEngineUniformSize = imageUniformSize

// materialEffectEntry is one effect's GPU material state: shared bind group
// layouts (identical across legs), the composed module, the user uniform
// slot, and one pipeline per clip leg.
type materialEffectEntry struct {
	module    hal.ShaderModule
	uniform   hal.Buffer // user uniform block, full 256B budget slot
	uniSize   int
	hasTex    bool
	pipelines [2]hal.RenderPipeline // [0]=stencil (default leg), [1]=depth-clip
}

func (e *materialEffectEntry) destroy() {
	for _, p := range e.pipelines {
		if p != nil {
			p.Destroy()
		}
	}
	if e.module != nil {
		e.module.Destroy()
	}
	if e.uniform != nil {
		e.uniform.Destroy()
	}
}

// materialCache holds per-effect material state plus the global bind group
// layouts. Layouts derive from the composed module shape (immutable at
// NewShaderEffect time), never from per-draw state (L2-P1-3). Closed effects
// evict through the RegisterShaderEffectEvictor hook, same as the fill
// pass-1 cache.
type materialCache struct {
	mu          sync.Mutex
	initialized bool
	device      hal.Device
	entries     map[*render.ShaderEffect]*materialEffectEntry
	// bgl0 has two shapes (user declares the texture pair or not); bgl1 is
	// the engine per-draw block. All material pipelines reference these
	// objects, so per-slot bind groups stay valid across legs.
	bgl0NoTex hal.BindGroupLayout
	bgl0Tex   hal.BindGroupLayout
	bgl1      hal.BindGroupLayout
	// dummyUniform backs the always-laid-out user binding(0) for modules
	// that declare no uniform block.
	dummyUniform hal.Buffer
}

func (c *materialCache) ensureLocked(device hal.Device) error {
	if c.initialized && c.device == device {
		return nil
	}
	if c.initialized && c.device != device {
		c.destroyLocked()
	}
	c.device = device
	c.initialized = true
	c.entries = make(map[*render.ShaderEffect]*materialEffectEntry)
	return nil
}

func (c *materialCache) destroyLocked() {
	for _, e := range c.entries {
		e.destroy()
	}
	c.entries = nil
	for _, bgl := range []hal.BindGroupLayout{c.bgl0NoTex, c.bgl0Tex, c.bgl1} {
		if bgl != nil {
			bgl.Destroy()
		}
	}
	c.bgl0NoTex, c.bgl0Tex, c.bgl1 = nil, nil, nil
	if c.dummyUniform != nil {
		c.dummyUniform.Destroy()
		c.dummyUniform = nil
	}
	c.initialized = false
	c.device = nil
}

// evictEffect drops one closed effect's state.
func (c *materialCache) evictEffect(effect *render.ShaderEffect) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[effect]; ok {
		delete(c.entries, effect)
		e.destroy()
	}
}

func (c *materialCache) userBGL(hasTex bool) hal.BindGroupLayout {
	if hasTex {
		return c.bgl0Tex
	}
	return c.bgl0NoTex
}

// ensureLayoutsLocked creates the shared bind group layouts once.
func (c *materialCache) ensureLayoutsLocked(device hal.Device) error {
	if c.bgl1 != nil {
		return nil
	}
	uniSize := int(render.ShaderUniformBudget)
	base := []types.BindGroupLayoutEntry{{
		Binding:    0,
		Visibility: types.ShaderStageFragment,
		Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: uint64(uniSize)},
	}}
	tex := append(append([]types.BindGroupLayoutEntry{}, base...),
		types.BindGroupLayoutEntry{Binding: 1, Visibility: types.ShaderStageFragment, Texture: &types.TextureBindingLayout{SampleType: types.TextureSampleTypeFloat, ViewDimension: types.TextureViewDimension2D}},
		types.BindGroupLayoutEntry{Binding: 2, Visibility: types.ShaderStageFragment, Sampler: &types.SamplerBindingLayout{Type: types.SamplerBindingTypeFiltering}},
	)
	bgl0NoTex, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{Label: "material_user_bgl", Entries: base})
	if err != nil {
		return fmt.Errorf("material: user bgl: %w", err)
	}
	bgl0Tex, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{Label: "material_user_tex_bgl", Entries: tex})
	if err != nil {
		bgl0NoTex.Destroy()
		return fmt.Errorf("material: user tex bgl: %w", err)
	}
	bgl1, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{Label: "material_engine_bgl",
		Entries: []types.BindGroupLayoutEntry{{
			Binding:    0,
			Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
			Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: materialEngineUniformSize},
		}}})
	if err != nil {
		bgl0Tex.Destroy()
		bgl0NoTex.Destroy()
		return fmt.Errorf("material: engine bgl: %w", err)
	}
	c.bgl0NoTex, c.bgl0Tex, c.bgl1 = bgl0NoTex, bgl0Tex, bgl1
	return nil
}

// ensureMaterialPipeline builds (or reuses) one effect's material state and
// returns it; the pipeline for the requested leg is created on first use.
// The cache is session-owned (like imageCache): standalone sessions (tests)
// never touch a GPUShared. clipBindLayout is the session's standard clip
// layout object — the material pipeline layout references the SAME object so
// the per-group clip bind group (noClipBindGroup / scissor-group clip) binds
// at @group(2) unchanged.
func (s *GPURenderSession) ensureMaterialPipeline(effect *render.ShaderEffect, depthClip bool) (*materialEffectEntry, error) {
	c := &s.userMaterial
	c.mu.Lock()
	defer c.mu.Unlock()
	if effect == nil || effect.Closed() {
		return nil, fmt.Errorf("material: effect closed or nil")
	}
	if s.clipBindLayout == nil {
		return nil, fmt.Errorf("material: clip bind layout not ready")
	}
	device := s.device
	if err := c.ensureLocked(device); err != nil {
		return nil, err
	}
	if err := c.ensureLayoutsLocked(device); err != nil {
		return nil, err
	}

	entry, ok := c.entries[effect]
	if !ok {
		if c.dummyUniform == nil {
			dummy, err := device.CreateBuffer(&hal.BufferDescriptor{
				Label: "material_user_uniform_empty",
				Size:  uint64(render.ShaderUniformBudget),
				Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
			})
			if err != nil {
				return nil, fmt.Errorf("material: empty uniform buffer: %w", err)
			}
			c.dummyUniform = dummy
		}
		module, err := device.CreateShaderModule(&hal.ShaderModuleDescriptor{WGSL: effect.ComposedQuadWGSL()})
		if err != nil {
			return nil, fmt.Errorf("material: module: %w", err)
		}
		ubuf, err := device.CreateBuffer(&hal.BufferDescriptor{
			Label: "material_user_uniform",
			Size:  uint64(render.ShaderUniformBudget),
			Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
		})
		if err != nil {
			module.Destroy()
			return nil, fmt.Errorf("material: uniform buffer: %w", err)
		}
		entry = &materialEffectEntry{
			module:  module,
			uniform: ubuf,
			uniSize: int(render.ShaderUniformBudget),
			hasTex:  effect.ShaderBinding().UsesImage,
		}
		c.entries[effect] = entry
	}

	leg := 0
	if depthClip {
		leg = 1
	}
	if entry.pipelines[leg] != nil {
		return entry, nil
	}
	layout, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "material_layout",
		BindGroupLayouts: []hal.BindGroupLayout{c.userBGL(entry.hasTex), c.bgl1, s.clipBindLayout},
	})
	if err != nil {
		return nil, fmt.Errorf("material: layout: %w", err)
	}
	label := "material_pipeline"
	dsState := stencilPassthroughDepthStencil()
	if depthClip {
		label = "material_pipeline_depth_clip"
		dsState = depthClipDepthStencil()
	}
	pipe, err := device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  label,
		Layout: layout,
		Vertex: hal.VertexState{
			Module:     entry.module,
			EntryPoint: "vs_main",
			Buffers:    perspVertexLayout(),
		},
		Primitive:    triangleListPrimitive(),
		DepthStencil: dsState,
		Multisample:  types.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &hal.FragmentState{
			Module:     entry.module,
			EntryPoint: "fs_main",
			Targets: []types.ColorTargetState{{
				Format:    types.TextureFormatBGRA8Unorm,
				WriteMask: types.ColorWriteMaskAll,
				Blend:     premulBlendStatePtr(),
			}},
		},
	})
	if err != nil {
		layout.Destroy()
		return nil, fmt.Errorf("material: pipeline: %w", err)
	}
	entry.pipelines[leg] = pipe
	return entry, nil
}

// uploadMaterialUniforms writes the effect's uniform block into its 256B
// slot. Called per buildImageResources containing material slots (the effect
// holds the uniform state; no public per-draw override exists in 3a).
func uploadMaterialUniforms(queue hal.Queue, effect *render.ShaderEffect, buf hal.Buffer) error {
	uniforms := effect.Uniforms()
	if len(uniforms) == 0 {
		return nil
	}
	return queue.WriteBuffer(buf, 0, uniforms)
}

// premulBlendStatePtr returns the standard premultiplied blend state — the
// material leg shares the image leg's compositing exactly.
func premulBlendStatePtr() *types.BlendState {
	bs := types.BlendStatePremultiplied()
	return &bs
}
