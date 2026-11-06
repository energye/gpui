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

// materialEffectEntry is one effect's GPU material state: a module-derived
// bind group layout (batch 3c: declared texture pairs at bindings (1,2),
// (3,4), … — layouts can no longer be one of two global shapes), the composed
// module, the user uniform slot (sized to the declared block), the SetImageAt
// slot textures (pairs 1+; pair 0 is the drawn sprite itself), and one
// pipeline per clip leg.
type materialEffectEntry struct {
	module       hal.ShaderModule
	bgl0         hal.BindGroupLayout // module-derived (uniform + declared pairs)
	uniform      hal.Buffer          // user uniform block, sized per contract
	uniMin       uint64              // binding(0) MinBindingSize (max(256, declared))
	uniSize      int
	texturePairs int
	pipelines    [2]hal.RenderPipeline // [0]=stencil (default leg), [1]=depth-clip
	// slotTex holds the uploaded GPU textures for SetImageAt slots 1+ (pair 0
	// is the command's sprite, fed live from the image cache). Re-uploaded
	// when the bound image's generation changes; destroyed with the entry.
	slotTex []*materialSlotTexture
	// slotTexGen mirrors the bound image generation per slot (0 = needs upload).
	slotTexGen []uint64
}

type materialSlotTexture struct {
	tex  hal.Texture
	view hal.TextureView
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
	if e.bgl0 != nil {
		e.bgl0.Destroy()
	}
	if e.uniform != nil {
		e.uniform.Destroy()
	}
	for _, st := range e.slotTex {
		if st != nil {
			if st.view != nil {
				st.view.Destroy()
			}
			if st.tex != nil {
				st.tex.Destroy()
			}
		}
	}
}

// materialCache holds per-effect material state plus the global bind group
// layouts. Layouts derive from the composed module shape (immutable at
// NewShaderEffect time), never from per-draw state (L2-P1-3). Closed effects
// evict through the RegisterShaderEffectEvictor hook, same as the fill
// pass-1 cache.
// materialKey is the per-variant cache key (材质×bicubic 纹素契约 2026-11-07)：
// 纹素声明的效果在 bicubic 档用内嵌 16-tap 核模块（第二形状），其余档共享
// plain 模块——两变体各自持模块/管线/ uniform 缓冲，bgl0 同形各自建。
type materialKey struct {
	effect  *render.ShaderEffect
	bicubic bool
}

type materialCache struct {
	mu          sync.Mutex
	initialized bool
	device      hal.Device
	entries     map[materialKey]*materialEffectEntry
	// rejected 是 H23 的负面缓存：后端（naga）拒收的模块在这里记一次，之后每一帧
	// 各口直接按「已知不可用」拒画，不再重复尝试编译（重复尝试既不改变结果，还把
	// p95 帧间隔从 23.4ms 拖到 51.1ms，并每帧刷一整段 stderr）。键含 bicubic 位
	// （材质×bicubic 纹素契约 2026-11-07）：plain 模块被拒不代表核变体被拒。
	rejected map[materialKey]error
	// bgl1 is the engine per-draw block (shared). bgl0 lives per effect
	// (batch 3c: declared pairs are module-derived, so the layout is no
	// longer one of two global shapes).
	bgl1 hal.BindGroupLayout
	// black bindings back unbound declared slots (Dawn default-bindings 同款).
	blackTex  hal.Texture
	blackView hal.TextureView
	blackSamp hal.Sampler
	// linear sampler for SetImageAt slots 1+ (fill-path SetImage convention).
	linearSamp hal.Sampler
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
	c.entries = make(map[materialKey]*materialEffectEntry)
	c.rejected = make(map[materialKey]error)
	return nil
}

func (c *materialCache) destroyLocked() {
	for _, e := range c.entries {
		e.destroy()
	}
	c.entries = nil
	c.rejected = nil
	if c.bgl1 != nil {
		c.bgl1.Destroy()
		c.bgl1 = nil
	}
	for _, r := range []hal.Texture{c.blackTex} {
		if r != nil {
			r.Destroy()
		}
	}
	if c.blackView != nil {
		c.blackView.Destroy()
		c.blackView = nil
	}
	if c.blackSamp != nil {
		c.blackSamp.Destroy()
		c.blackSamp = nil
	}
	if c.linearSamp != nil {
		c.linearSamp.Destroy()
		c.linearSamp = nil
	}
	c.initialized = false
	c.device = nil
}

// evictEffect drops one closed effect's state (both variants).
func (c *materialCache) evictEffect(effect *render.ShaderEffect) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, bic := range []bool{false, true} {
		key := materialKey{effect: effect, bicubic: bic}
		if e, ok := c.entries[key]; ok {
			delete(c.entries, key)
			e.destroy()
		}
		// 关闭即撤判：同一个地址将来可能被另一枚 effect 复用（H23 负面缓存随生命周期走）。
		delete(c.rejected, key)
	}
}

// ensureBlackLocked lazily creates the 1×1 black texture backing unbound
// declared slots (Dawn default-bindings 同款; cache mutex held).
func (c *materialCache) ensureBlackLocked(device hal.Device, queue hal.Queue) error {
	if c.blackTex != nil && c.blackSamp != nil {
		return nil
	}
	tex, view, err := dualTexCreateTex(device, queue, "material_black", 1, 1,
		[]byte{0, 0, 0, 255}, types.TextureUsageTextureBinding|types.TextureUsageCopyDst)
	if err != nil {
		return fmt.Errorf("material: black texture: %w", err)
	}
	samp, err := device.CreateSampler(&hal.SamplerDescriptor{MagFilter: types.FilterModeLinear, MinFilter: types.FilterModeLinear})
	if err != nil {
		tex.Destroy()
		view.Destroy()
		return fmt.Errorf("material: black sampler: %w", err)
	}
	c.blackTex, c.blackView, c.blackSamp = tex, view, samp
	return nil
}

// ensureLinearLocked lazily creates the fixed linear sampler for SetImageAt
// slots 1+ (fill-path SetImage convention; cache mutex held).
func (c *materialCache) ensureLinearLocked(device hal.Device) error {
	if c.linearSamp != nil {
		return nil
	}
	samp, err := device.CreateSampler(&hal.SamplerDescriptor{MagFilter: types.FilterModeLinear, MinFilter: types.FilterModeLinear})
	if err != nil {
		return fmt.Errorf("material: linear sampler: %w", err)
	}
	c.linearSamp = samp
	return nil
}

// ensureLayoutsLocked creates the shared engine block layout once (bgl0 is
// per-effect, module-derived — batch 3c).
func (c *materialCache) ensureLayoutsLocked(device hal.Device) error {
	if c.bgl1 != nil {
		return nil
	}
	bgl1, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{Label: "material_engine_bgl",
		Entries: []types.BindGroupLayoutEntry{{
			Binding:    0,
			Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
			Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: materialEngineUniformSize},
		}}})
	if err != nil {
		return fmt.Errorf("material: engine bgl: %w", err)
	}
	c.bgl1 = bgl1
	return nil
}

// materialUserBGL0 derives the per-effect user layout from the declared bind
// interface (batch 3c, L2-P1-3): uniform@0 sized max(256, UniformBytes) +
// texture@2k+1 / sampler@2k+2 per declared pair.
func materialUserBGL0(device hal.Device, bind render.ShaderInterface) (hal.BindGroupLayout, error) {
	uniMin := uint64(render.ShaderUniformBudget)
	if bind.UniformBytes > int(render.ShaderUniformBudget) {
		uniMin = uint64(bind.UniformBytes)
	}
	entries := []types.BindGroupLayoutEntry{{
		Binding:    0,
		Visibility: types.ShaderStageFragment,
		Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: uniMin},
	}}
	for k := 0; k < bind.TexturePairs; k++ {
		entries = append(entries,
			types.BindGroupLayoutEntry{Binding: uint32(2*k + 1), Visibility: types.ShaderStageFragment, Texture: &types.TextureBindingLayout{SampleType: types.TextureSampleTypeFloat, ViewDimension: types.TextureViewDimension2D}},
			types.BindGroupLayoutEntry{Binding: uint32(2*k + 2), Visibility: types.ShaderStageFragment, Sampler: &types.SamplerBindingLayout{Type: types.SamplerBindingTypeFiltering}},
		)
	}
	bgl0, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{Label: "material_user_bgl", Entries: entries})
	if err != nil {
		return nil, fmt.Errorf("material: user bgl: %w", err)
	}
	return bgl0, nil
}

// ensureMaterialPipeline builds (or reuses) one effect's material state and
// noteMaterialRefusal 记一笔「这笔被后端拒收 ⇒ 整笔不画」（每笔一次 skips）。
// ShaderBackendFails 不在这里加——它由 rejectMaterialEffect 在**首次**登记时记，
// 重复帧走缓存只加 skips。
func (s *GPURenderSession) noteMaterialRefusal() { s.matRefusalSkips++ }

// rejectMaterialEffect 登记一次后端拒收（调用者持 c.mu）：写负面缓存 + 记一次
// ShaderBackendFails，返回要上抛的具名错。
func (s *GPURenderSession) rejectMaterialEffect(c *materialCache, key materialKey, stage string, err error) error {
	reject := fmt.Errorf("%w: material %s: %w", render.ErrShaderBackendCompile, stage, err)
	if _, seen := c.rejected[key]; !seen {
		s.matRefusalFails++
	}
	c.rejected[key] = reject
	return reject
}

// MaterialRejectAttempts 返回本会话真正打到设备的材质模块创建次数（含成功）。
// 负面缓存的验收面：同一枚被拒的模块，N 帧之后这里必须还是 1。
func (s *GPURenderSession) MaterialRejectAttempts() int { return s.matRejectAttempts }

// TakeMaterialRefusals 取走并清零两个计数（Context 在 RenderPathStats 处折算进
// 自己的 pathStats）。
func (s *GPURenderSession) TakeMaterialRefusals() (skips, freshFails int) {
	skips, freshFails = s.matRefusalSkips, s.matRefusalFails
	s.matRefusalSkips, s.matRefusalFails = 0, 0
	return skips, freshFails
}

// returns it; the pipeline for the requested leg is created on first use.
// The cache is session-owned (like imageCache): standalone sessions (tests)
// never touch a GPUShared. clipBindLayout is the session's standard clip
// layout object — the material pipeline layout references the SAME object so
// the per-group clip bind group (noClipBindGroup / scissor-group clip) binds
// at @group(2) unchanged.
func (s *GPURenderSession) ensureMaterialPipeline(effect *render.ShaderEffect, bicubic, depthClip bool) (*materialEffectEntry, error) {
	c := &s.userMaterial
	c.mu.Lock()
	defer c.mu.Unlock()
	if effect == nil || effect.Closed() {
		return nil, fmt.Errorf("%w: material ensure after Close", render.ErrShaderClosed)
	}
	key := materialKey{effect: effect, bicubic: bicubic}
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
	// H23 负面缓存：这台设备已经拒过这枚模块 ⇒ 零设备调用，直接把具名错还给调用方。
	if err, bad := c.rejected[key]; bad {
		return nil, err
	}

	entry, ok := c.entries[key]
	if !ok {
		bind := effect.ShaderBinding()
		bgl0, err := materialUserBGL0(device, bind)
		if err != nil {
			return nil, err
		}
		uniSize := int(render.ShaderUniformBudget)
		if bind.UniformBytes > uniSize {
			uniSize = bind.UniformBytes
		}
		s.matRejectAttempts++
		device.PushErrorScope(hal.ErrorFilterValidation)
		module, err := device.CreateShaderModule(&hal.ShaderModuleDescriptor{WGSL: effect.ComposedQuadWGSLFor(bicubic)})
		// H23：wgpu-native 的 shader 校验是**排队**的——CreateShaderModule 自己返回
		// nil，错误落在后面任意一个设备调用身上（实测先后命中过 CreateBuffer，
		// 且换顺序也只是换个受害者）。所以模块创建必须包在验证错误域里，pop 回来
		// 才算它自己的账；域同时把错误就地吃掉，不再污染下一次调用（实测 pop 之后
		// 建缓冲返回 nil）。gles/metal 的域是 stub（拿不到也算拒不了：它们的前端
		// 根本不收这种模块，见 ③ 臂 G 的两端实测）。
		if scoped := device.PopErrorScope(); scoped != nil && err == nil {
			err = scoped
		}
		if err != nil {
			bgl0.Destroy()
			return nil, s.rejectMaterialEffect(c, key, "module", err)
		}
		ubuf, err := device.CreateBuffer(&hal.BufferDescriptor{
			Label: "material_user_uniform",
			Size:  uint64(uniSize),
			Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
		})
		if err != nil {
			module.Destroy()
			bgl0.Destroy()
			return nil, fmt.Errorf("material: uniform buffer: %w", err)
		}
		entry = &materialEffectEntry{
			module:       module,
			bgl0:         bgl0,
			uniform:      ubuf,
			uniMin:       uint64(max(render.ShaderUniformBudget, bind.UniformBytes)),
			uniSize:      uniSize,
			texturePairs: bind.TexturePairs,
		}
		if bind.TexturePairs > 1 {
			entry.slotTex = make([]*materialSlotTexture, bind.TexturePairs-1)
			entry.slotTexGen = make([]uint64, bind.TexturePairs-1)
		}
		c.entries[key] = entry
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
		BindGroupLayouts: []hal.BindGroupLayout{entry.bgl0, c.bgl1, s.clipBindLayout},
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
		return nil, s.rejectMaterialEffect(c, key, "pipeline", err)
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

// ensureMaterialSlotTextures uploads the effect's SetImageAt images (slots 1+)
// into the entry's per-slot GPU textures (batch 3c). gen-keyed: a re-bound
// image re-uploads once; nil slots fall to the black placeholder at bind
// time. Called per material slot build (idempotent across slots of one
// effect). Per-frame upload pooling = registered follow-up (same family as
// the fill-path scratch pooling).
func (s *GPURenderSession) ensureMaterialSlotTextures(entry *materialEffectEntry, effect *render.ShaderEffect) error {
	if entry.texturePairs <= 1 {
		return nil
	}
	c := &s.userMaterial
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := 1; k < entry.texturePairs; k++ {
		idx := k - 1
		img := effect.ImageAt(k)
		var gen uint64
		if img != nil {
			gen = img.GenerationID()
		}
		if gen == entry.slotTexGen[idx] && (gen == 0 || entry.slotTex[idx] != nil) {
			continue
		}
		if entry.slotTex[idx] != nil {
			if entry.slotTex[idx].view != nil {
				entry.slotTex[idx].view.Destroy()
			}
			if entry.slotTex[idx].tex != nil {
				entry.slotTex[idx].tex.Destroy()
			}
			entry.slotTex[idx] = nil
		}
		entry.slotTexGen[idx] = gen
		if img == nil || gen == 0 {
			continue
		}
		tex, view, err := dualTexCreateTex(s.device, s.queue, fmt.Sprintf("material_slot_tex_%d", k), img.Width(), img.Height(), img.PremultipliedData(),
			types.TextureUsageTextureBinding|types.TextureUsageCopyDst)
		if err != nil {
			return fmt.Errorf("material: slot %d texture: %w", k, err)
		}
		entry.slotTex[idx] = &materialSlotTexture{tex: tex, view: view}
	}
	return nil
}
