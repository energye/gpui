//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package gpu

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

//go:embed shaders/textured_quad.wgsl
var texturedQuadShaderSource string

//go:embed shaders/textured_quad_bicubic.wgsl
var texturedQuadBicubicShaderSource string

// imageVertexStride is the byte stride per vertex in the textured quad pipeline.
// Layout per vertex:
//
//	position (vec2<f32>) = 8 bytes (location 0)
//	tex_coord (vec2<f32>) = 8 bytes (location 1)
//	tint (vec4<f32>) = 16 bytes (location 2, premultiplied straight)
//
// Total = 32 bytes per vertex.
const imageVertexStride = 32

// imageUniformSize is the byte size of the image uniform buffer.
// Layout:
//
//	transform (mat4x4<f32>) = 64 bytes
//	opacity (f32) = 4 bytes
//	_pad (vec3<f32>) = 12 bytes
//
// Total = 80 bytes.
const imageUniformSize = 80

// imageUniformSlotStride is the bytes reserved per draw uniform in a shared
// slab buffer. Must be a multiple of minUniformBufferOffsetAlignment
// (default 256) so BindGroup entries may use non-zero Offset.
const imageUniformSlotStride = 256

// ImageDrawCommand holds everything needed to render one image as a textured
// quad on the GPU. Populated by context_image.go and consumed by the render
// session. All coordinates are in device pixels (post-CTM).
type ImageDrawCommand struct {
	// Image pixel data (premultiplied RGBA, row-major).
	PixelData    []byte
	GenerationID uint64 // Pixmap.GenerationID() — GPU cache key (ADR-014)
	ImgWidth     int
	ImgHeight    int
	ImgStride    int

	// Destination axis-aligned bounds in device pixels.
	// For rotated/skewed images this is the AABB of the transformed quad.
	DstX, DstY float32
	DstW, DstH float32

	// Destination quad corners in device pixels after CTM:
	// TL, TR, BR, BL. Vertex emission always uses these corners so rotation
	// and non-uniform scale are preserved (axis-aligned is the special case
	// where corners form a rectangle).
	TLX, TLY float32
	TRX, TRY float32
	BRX, BRY float32
	BLX, BLY float32

	// Tint multiplies straight source texels (R4 AtlasSprite.Tint, zero
	// struct = identity white). Stored premultiplied (R*A, G*A, B*A, A) so
	// the shader's texel*tint keeps premultiplied-alpha blending correct.
	// Zero struct (TintR/G/B/A all 0) means identity, matching
	// atlasTintIsIdentity: old callers that never touch these fields keep
	// (1,1,1,1) on the wire bit-identically.
	TintR, TintG, TintB, TintA float32

	Opacity        float32
	ViewportWidth  uint32
	ViewportHeight uint32

	// Source UV rectangle (normalized 0..1 within the image).
	// For full-image draws: u0=0, v0=0, u1=1, v1=1.
	U0, V0, U1, V1 float32

	// Filter selects texture sampling (I.03). false = Linear (default), true = Nearest.
	Nearest bool

	// Bicubic selects GPU 4x4 bicubic convolution (I.03, textured_quad_bicubic.wgsl).
	// Mutually exclusive with Nearest: Bicubic wins when both are set.
	Bicubic bool

	// FilterMipmapLinear selects the between-level blend (trilinear) for
	// this picture. False (zero) keeps
	// the historic nearest-level select: old callers that never touch this
	// field render bit-identically.
	FilterMipmapLinear bool

	// FilterAniso caps anisotropic filtering for this picture, 1..16.
	// 0 means 1 (historic default, off): old callers that never touch this
	// field render bit-identically. Set via SetPerImageFilter (new branch).
	FilterAniso uint16

	// ContentDirty: GenerationID is stable but pixel bytes changed (ExportImageBuf
	// reuse). ImageCache re-uploads into the existing GPU texture in place.
	ContentDirty bool
}

// TexturedQuadPipeline manages GPU resources for image rendering (Tier 3).
// Each image draw is a textured quad: 6 vertices (2 triangles) with UV mapping.
// The fragment shader samples the image texture with bilinear filtering and
// applies opacity as a uniform multiplier.
//
// Architecture:
//
//	GPURenderSession owns persistent vertex/index buffers (if needed)
//	TexturedQuadPipeline owns shader, layout, pipeline, sampler
//	ImageCache (on GPUShared) owns per-image GPU textures
//	Bind groups are created per-batch (uniform + texture + sampler)
type TexturedQuadPipeline struct {
	device      hal.Device
	queue       hal.Queue
	sampleCount uint32 // MSAA sample count (4 or 1), from GPUShared

	// GPU objects for the render pipeline.
	shader        hal.ShaderModule
	uniformLayout hal.BindGroupLayout
	pipeLayout    hal.PipelineLayout

	// Bicubic sampling shader (textured_quad_bicubic.wgsl) — 16-tap cubic
	// convolution, same bind group layout as the linear shader.
	bicubicShader hal.ShaderModule

	// Session-compatible pipeline variant with depth/stencil state.
	// Used when images participate in a unified render pass that includes
	// a stencil attachment. Stencil test is Always/Keep (images do not
	// interact with stencil).
	pipelineWithStencil hal.RenderPipeline

	// Depth-clipped pipeline variant (GPU-CLIP-003a). Same as pipelineWithStencil
	// but with DepthCompare=GreaterEqual to test against the depth clip buffer.
	pipelineWithDepthClip hal.RenderPipeline

	// Bicubic sampling variants (I.03): stencil + depth-clip, same depth/stencil
	// state as their linear counterparts, different fragment shader.
	pipelineWithBicubic          hal.RenderPipeline
	pipelineWithBicubicDepthClip hal.RenderPipeline

	// Non-MSAA blit pipeline for compositor fast path (ADR-016).
	// SampleCount=1, no depth/stencil — used when the frame contains
	// only textured quads (base layer + overlays) with no vector shapes.
	blitPipeline      hal.RenderPipeline
	blitLayout        hal.PipelineLayout
	blitLayoutHasClip bool

	// Default sampler for image textures (bilinear filtering, clamp-to-edge).
	sampler hal.Sampler
	// Nearest-neighbor sampler (I.03).
	nearestSampler hal.Sampler

	// filterSamplers caches R3 per-picture samplers by effective key.
	// Default keys reuse sampler/nearestSampler above (never cached here);
	// only mipmap-linear or aniso>1 keys allocate. Lazily created by
	// SamplerForFilter, released by destroyPipeline.
	filterSamplers map[ImageSamplerKey]hal.Sampler

	// Set by the session before ensurePipelineWithStencil.
	clipBindLayout    hal.BindGroupLayout
	pipeLayoutHasClip bool
}

// NewTexturedQuadPipeline creates a new textured quad pipeline.
func NewTexturedQuadPipeline(device hal.Device, queue hal.Queue, sampleCount uint32) *TexturedQuadPipeline {
	return &TexturedQuadPipeline{
		device:      device,
		queue:       queue,
		sampleCount: sampleCount,
	}
}

// Must be called before ensurePipelineWithStencil.
func (p *TexturedQuadPipeline) SetClipBindLayout(layout hal.BindGroupLayout) {
	p.clipBindLayout = layout
}

// Destroy releases all GPU resources held by the pipeline.
func (p *TexturedQuadPipeline) Destroy() {
	p.destroyPipeline()
}

// ensurePipelineWithStencil creates the pipeline variant that includes
// depth/stencil state (for unified render pass with stencil-then-cover).
func (p *TexturedQuadPipeline) ensurePipelineWithStencil() error {
	if err := p.ensureBase(); err != nil {
		return err
	}
	// If the pipeline layout was created without clip but clip is now set,
	// destroy and recreate.
	if p.clipBindLayout != nil && !p.pipeLayoutHasClip {
		p.destroyPipeline()
		if err := p.ensureBase(); err != nil {
			return err
		}
	}
	if p.pipelineWithStencil != nil {
		return nil
	}

	premulBlend := types.BlendStatePremultiplied()
	pipeline, err := p.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "textured_quad_pipeline_with_stencil",
		Layout: p.pipeLayout,
		Vertex: hal.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    imageVertexLayout(),
		},
		Fragment: &hal.FragmentState{
			Module:     p.shader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{
					Format:    types.TextureFormatBGRA8Unorm,
					Blend:     &premulBlend,
					WriteMask: types.ColorWriteMaskAll,
				},
			},
		},
		DepthStencil: stencilPassthroughDepthStencil(),
		Primitive:    triangleListPrimitive(),
		Multisample:  multisampleState(p.sampleCount),
	})
	if err != nil {
		return fmt.Errorf("create textured quad pipeline with stencil: %w", err)
	}
	p.pipelineWithStencil = pipeline
	return nil
}

// ensureDepthClipPipeline creates the depth-clipped pipeline variant if needed.
func (p *TexturedQuadPipeline) ensureDepthClipPipeline() error {
	if p.pipelineWithDepthClip != nil {
		return nil
	}
	if err := p.ensurePipelineWithStencil(); err != nil {
		return err
	}

	premulBlend := types.BlendStatePremultiplied()
	pipeline, err := p.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "textured_quad_pipeline_depth_clip",
		Layout: p.pipeLayout,
		Vertex: hal.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    imageVertexLayout(),
		},
		Fragment: &hal.FragmentState{
			Module:     p.shader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{
					Format:    types.TextureFormatBGRA8Unorm,
					Blend:     &premulBlend,
					WriteMask: types.ColorWriteMaskAll,
				},
			},
		},
		DepthStencil: depthClipDepthStencil(),
		Primitive:    triangleListPrimitive(),
		Multisample:  multisampleState(p.sampleCount),
	})
	if err != nil {
		return fmt.Errorf("create textured quad pipeline with depth clip: %w", err)
	}
	p.pipelineWithDepthClip = pipeline
	return nil
}

// ensureBicubicPipelines creates the bicubic sampling pipeline variants
// (I.03): stencil + depth-clip, sharing the linear pipelines' bind group
// layout (uniform + texture + sampler + clip). Only the fragment shader
// differs (16-tap cubic convolution). Lazily compiled once.
func (p *TexturedQuadPipeline) ensureBicubicPipelines() error {
	if p.pipelineWithBicubic != nil {
		return nil
	}
	if err := p.ensurePipelineWithStencil(); err != nil {
		return err
	}
	if p.bicubicShader == nil {
		shader, err := p.device.CreateShaderModule(&hal.ShaderModuleDescriptor{
			Label: "textured_quad_bicubic_shader",
			WGSL:  texturedQuadBicubicShaderSource,
		})
		if err != nil {
			return fmt.Errorf("compile textured quad bicubic shader: %w", err)
		}
		p.bicubicShader = shader
	}

	premulBlend := types.BlendStatePremultiplied()
	stencilPipe, err := p.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "textured_quad_pipeline_bicubic",
		Layout: p.pipeLayout,
		Vertex: hal.VertexState{
			Module:     p.bicubicShader,
			EntryPoint: shaderEntryVS,
			Buffers:    imageVertexLayout(),
		},
		Fragment: &hal.FragmentState{
			Module:     p.bicubicShader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{
					Format:    types.TextureFormatBGRA8Unorm,
					Blend:     &premulBlend,
					WriteMask: types.ColorWriteMaskAll,
				},
			},
		},
		DepthStencil: stencilPassthroughDepthStencil(),
		Primitive:    triangleListPrimitive(),
		Multisample:  multisampleState(p.sampleCount),
	})
	if err != nil {
		return fmt.Errorf("create textured quad bicubic pipeline: %w", err)
	}
	p.pipelineWithBicubic = stencilPipe

	// Depth-clip bicubic variant (GPU-CLIP-003a interaction: bicubic content
	// must honor arbitrary path clipping like any other content tier).
	depthPipe, err := p.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "textured_quad_pipeline_bicubic_depth_clip",
		Layout: p.pipeLayout,
		Vertex: hal.VertexState{
			Module:     p.bicubicShader,
			EntryPoint: shaderEntryVS,
			Buffers:    imageVertexLayout(),
		},
		Fragment: &hal.FragmentState{
			Module:     p.bicubicShader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{
					Format:    types.TextureFormatBGRA8Unorm,
					Blend:     &premulBlend,
					WriteMask: types.ColorWriteMaskAll,
				},
			},
		},
		DepthStencil: depthClipDepthStencil(),
		Primitive:    triangleListPrimitive(),
		Multisample:  multisampleState(p.sampleCount),
	})
	if err != nil {
		p.pipelineWithBicubic.Destroy()
		p.pipelineWithBicubic = nil
		return fmt.Errorf("create textured quad bicubic depth clip pipeline: %w", err)
	}
	p.pipelineWithBicubicDepthClip = depthPipe
	return nil
}

// pipelineForDraw returns the render pipeline for a draw call, honoring
// bicubic sampling and the depth-clip variant (GPU-CLIP-003a).
func (p *TexturedQuadPipeline) pipelineForDraw(dc imageDrawCall, useDepthClip bool) hal.RenderPipeline {
	if dc.bicubic {
		if useDepthClip && p.pipelineWithBicubicDepthClip != nil {
			return p.pipelineWithBicubicDepthClip
		}
		if p.pipelineWithBicubic != nil {
			return p.pipelineWithBicubic
		}
	}
	if useDepthClip && p.pipelineWithDepthClip != nil {
		return p.pipelineWithDepthClip
	}
	return p.pipelineWithStencil
}

// ensureBlitPipeline creates the non-MSAA pipeline variant for compositor
// fast path (ADR-016). SampleCount=1, no depth/stencil attachment.
func (p *TexturedQuadPipeline) ensureBlitPipeline() error {
	if err := p.ensureBase(); err != nil {
		return err
	}
	wantClip := p.clipBindLayout != nil
	if p.blitPipeline != nil && p.blitLayoutHasClip == wantClip {
		return nil
	}
	if p.blitPipeline != nil {
		p.blitPipeline.Destroy()
		p.blitPipeline = nil
	}
	if p.blitLayout != nil {
		p.blitLayout.Destroy()
		p.blitLayout = nil
	}

	// clip group when the session wired one. Clipped subtrees are cacheable
	// again since C8, so compositor blits can carry a rounded clip — the old
	// single-group layout silently dropped it (sharp corners on screen).
	layouts := []hal.BindGroupLayout{p.uniformLayout}
	if wantClip {
		layouts = append(layouts, p.clipBindLayout)
	}
	layout, err := p.device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "textured_quad_blit_layout",
		BindGroupLayouts: layouts,
	})
	if err != nil {
		return fmt.Errorf("create blit pipeline layout: %w", err)
	}
	p.blitLayout = layout
	p.blitLayoutHasClip = wantClip

	premulBlend := types.BlendStatePremultiplied()
	pipeline, err := p.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "textured_quad_blit_pipeline",
		Layout: p.blitLayout,
		Vertex: hal.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    imageVertexLayout(),
		},
		Fragment: &hal.FragmentState{
			Module:     p.shader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{
					Format:    types.TextureFormatBGRA8Unorm,
					Blend:     &premulBlend,
					WriteMask: types.ColorWriteMaskAll,
				},
			},
		},
		Primitive: triangleListPrimitive(),
		Multisample: types.MultisampleState{
			Count: 1,
			Mask:  0xFFFFFFFF,
		},
	})
	if err != nil {
		return fmt.Errorf("create textured quad blit pipeline: %w", err)
	}
	p.blitPipeline = pipeline
	return nil
}

// RecordBlitDraws records draw calls using the non-MSAA blit pipeline.
// Used for compositor fast path when no vector shapes need MSAA.
func (p *TexturedQuadPipeline) RecordBlitDraws(rp hal.RenderPassEncoder, res *imageFrameResources, clipBG hal.BindGroup) {
	if p.blitPipeline == nil || res == nil {
		return
	}
	rp.SetPipeline(p.blitPipeline)
	if p.blitLayoutHasClip && clipBG != nil {
		rp.SetBindGroup(1, clipBG, nil)
	}
	rp.SetVertexBuffer(0, res.vertBuf, 0)
	for _, dc := range res.drawCalls {
		rp.SetBindGroup(0, dc.bindGroup, nil)
		rp.Draw(imageDrawVertexCount(dc), 1, dc.firstVertex, 0)
	}
}

// ensureBase creates the shader, sampler, bind group layout, and pipeline layout
// if they don't exist yet.
func (p *TexturedQuadPipeline) ensureBase() error {
	if p.shader != nil && p.uniformLayout != nil && p.pipeLayout != nil && p.sampler != nil && p.nearestSampler != nil {
		return nil
	}

	if texturedQuadShaderSource == "" {
		return fmt.Errorf("textured_quad shader source is empty")
	}

	// Shader module.
	shader, err := p.device.CreateShaderModule(&hal.ShaderModuleDescriptor{
		Label: "textured_quad_shader",
		WGSL:  texturedQuadShaderSource,
	})
	if err != nil {
		return fmt.Errorf("compile textured_quad shader: %w", err)
	}
	p.shader = shader

	// Samplers: bilinear default + nearest (I.03), clamp-to-edge.
	sampler, err := p.device.CreateSampler(&hal.SamplerDescriptor{
		Label:        "image_sampler_linear",
		AddressModeU: types.AddressModeClampToEdge,
		AddressModeV: types.AddressModeClampToEdge,
		AddressModeW: types.AddressModeClampToEdge,
		MagFilter:    types.FilterModeLinear,
		MinFilter:    types.FilterModeLinear,
		MipmapFilter: types.MipmapFilterModeNearest,
	})
	if err != nil {
		return fmt.Errorf("create image sampler: %w", err)
	}
	p.sampler = sampler
	nearest, err := p.device.CreateSampler(&hal.SamplerDescriptor{
		Label:        "image_sampler_nearest",
		AddressModeU: types.AddressModeClampToEdge,
		AddressModeV: types.AddressModeClampToEdge,
		AddressModeW: types.AddressModeClampToEdge,
		MagFilter:    types.FilterModeNearest,
		MinFilter:    types.FilterModeNearest,
		MipmapFilter: types.MipmapFilterModeNearest,
	})
	if err != nil {
		return fmt.Errorf("create nearest image sampler: %w", err)
	}
	p.nearestSampler = nearest
	if p.filterSamplers == nil {
		p.filterSamplers = map[ImageSamplerKey]hal.Sampler{}
	}

	// Bind group layout: uniform + texture + sampler.
	uniformLayout, err := p.device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "textured_quad_bind_layout",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
				Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: imageUniformSize},
			},
			{
				Binding:    1,
				Visibility: types.ShaderStageFragment,
				Texture: &types.TextureBindingLayout{
					SampleType:    types.TextureSampleTypeFloat,
					ViewDimension: types.TextureViewDimension2D,
				},
			},
			{
				Binding:    2,
				Visibility: types.ShaderStageFragment,
				Sampler:    &types.SamplerBindingLayout{Type: types.SamplerBindingTypeFiltering},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("create textured_quad bind layout: %w", err)
	}
	p.uniformLayout = uniformLayout

	// Pipeline layout.
	bgLayouts := []hal.BindGroupLayout{p.uniformLayout}
	hasClip := p.clipBindLayout != nil
	if hasClip {
		bgLayouts = append(bgLayouts, p.clipBindLayout)
	}
	pipeLayout, err := p.device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "textured_quad_pipe_layout",
		BindGroupLayouts: bgLayouts,
	})
	if err != nil {
		return fmt.Errorf("create textured_quad pipe layout: %w", err)
	}
	p.pipeLayoutHasClip = hasClip
	p.pipeLayout = pipeLayout

	return nil
}

// RecordDraws records image draw commands into an existing render pass.
// Each draw call renders one textured quad with its own bind group (texture + uniform).
// When depthClipped is true (GPU-CLIP-003a), the depth-clipped pipeline
// variant is used to test fragments against the depth clip buffer.
func (p *TexturedQuadPipeline) RecordDraws(rp hal.RenderPassEncoder, res *imageFrameResources, clipBG hal.BindGroup, depthClipped ...bool) {
	useDepthClip := len(depthClipped) > 0 && depthClipped[0] && p.pipelineWithDepthClip != nil
	// Clear prior bind groups before pipeline switch (incompatible group-0 layouts).
	clearPassBindGroups(rp)
	if clipBG != nil {
		rp.SetBindGroup(1, clipBG, nil)
	}
	rp.SetVertexBuffer(0, res.vertBuf, 0)
	var lastPipe hal.RenderPipeline
	for _, dc := range res.drawCalls {
		pipe := p.pipelineForDraw(dc, useDepthClip)
		if pipe == nil {
			continue
		}
		if pipe != lastPipe {
			clearPassBindGroups(rp)
			rp.SetPipeline(pipe)
			if clipBG != nil {
				rp.SetBindGroup(1, clipBG, nil)
			}
			lastPipe = pipe
		}
		rp.SetBindGroup(0, dc.bindGroup, nil)
		rp.Draw(imageDrawVertexCount(dc), 1, dc.firstVertex, 0)
	}
}

// destroyPipeline releases all pipeline resources.
func (p *TexturedQuadPipeline) destroyPipeline() {
	if p.device == nil {
		return
	}
	if p.pipelineWithDepthClip != nil {
		p.pipelineWithDepthClip.Destroy()
		p.pipelineWithDepthClip = nil
	}
	if p.pipelineWithStencil != nil {
		p.pipelineWithStencil.Destroy()
		p.pipelineWithStencil = nil
	}
	if p.blitPipeline != nil {
		p.blitPipeline.Destroy()
		p.blitPipeline = nil
	}
	if p.blitLayout != nil {
		p.blitLayout.Destroy()
		p.blitLayout = nil
	}
	if p.pipeLayout != nil {
		p.pipeLayout.Destroy()
		p.pipeLayout = nil
		p.pipeLayoutHasClip = false
	}
	if p.uniformLayout != nil {
		p.uniformLayout.Destroy()
		p.uniformLayout = nil
	}
	if p.sampler != nil {
		p.sampler.Destroy()
		p.sampler = nil
	}
	if p.filterSamplers != nil {
		for key, s := range p.filterSamplers {
			if s != nil {
				s.Destroy()
			}
			delete(p.filterSamplers, key)
		}
	}
	if p.nearestSampler != nil {
		p.nearestSampler.Destroy()
		p.nearestSampler = nil
	}
	if p.shader != nil {
		p.shader.Destroy()
		p.shader = nil
	}
}

// imageFrameResources holds pre-built GPU resources for image rendering in
// a single frame. Created by the render session's buildImageResources.
type imageFrameResources struct {
	vertBuf   hal.Buffer
	drawCalls []imageDrawCall
}

// imageDrawCall holds per-image (or multi-quad batch) draw parameters within a frame.
type imageDrawCall struct {
	bindGroup   hal.BindGroup
	firstVertex uint32
	vertexCount uint32 // 0 means 6 (single quad, backward compatible)
	bicubic     bool   // uses bicubic pipeline variant (I.03), no merge with linear
}

// imageDrawVertexCount returns the vertex count for a draw call.
func imageDrawVertexCount(dc imageDrawCall) uint32 {
	if dc.vertexCount == 0 {
		return 6
	}
	return dc.vertexCount
}

// imageFilterAnisoMin/Max bound the per-picture anisotropy cap.
// 1 is off (historic behaviour), 16 is the device cap.
const (
	imageFilterAnisoMin = 1
	imageFilterAnisoMax = 16
)

// ImageSamplerKey names one picture's effective sampling choice.
// It is the merge and cache key: two pictures share a bind group and a
// sampler exactly when their keys are equal.
type ImageSamplerKey struct {
	Nearest      bool
	Bicubic      bool
	MipmapLinear bool
	Aniso        uint16 // normalized 1..16
}

// NormalizeImageFilterAniso pins a to [1, 16]; 0 means the historic
// default 1, so zero-value commands keep historic behaviour.
func NormalizeImageFilterAniso(a uint16) uint16 {
	if a == 0 {
		return imageFilterAnisoMin
	}
	if a > imageFilterAnisoMax {
		return imageFilterAnisoMax
	}
	return a
}

// EffectiveImageSamplerKey folds raw per-picture switches to the sampler
// that will actually be used. Nearest and bicubic pictures sample with
// the nearest sampler (bicubic taps are manual), so their mipmap/aniso
// switches are masked: such pictures always merge with each other.
func EffectiveImageSamplerKey(nearest, bicubic, mipmapLinear bool, aniso uint16) ImageSamplerKey {
	if bicubic || nearest {
		return ImageSamplerKey{Nearest: nearest, Bicubic: bicubic}
	}
	return ImageSamplerKey{MipmapLinear: mipmapLinear, Aniso: NormalizeImageFilterAniso(aniso)}
}

// FilterKey returns cmd's effective sampling key. Nil-safe: a nil command
// reports the zero key (callers check nil first).
func (c *ImageDrawCommand) FilterKey() ImageSamplerKey {
	if c == nil {
		return ImageSamplerKey{}
	}
	return EffectiveImageSamplerKey(c.Nearest, c.Bicubic, c.FilterMipmapLinear, c.FilterAniso)
}

// SetPerImageFilter records this picture's R3 sampling switch (new
// branch): mipmapLinear selects the between-level blend, maxAniso caps
// anisotropy (clamped to [1, 16], 0 means 1). The old Nearest/Bicubic
// fields stay untouched. Nil-safe no-op.
func (c *ImageDrawCommand) SetPerImageFilter(mipmapLinear bool, maxAniso uint16) {
	if c == nil {
		return
	}
	c.FilterMipmapLinear = mipmapLinear
	c.FilterAniso = NormalizeImageFilterAniso(maxAniso)
}

// SamplerDescriptorForImageFilter maps a key to its sampler descriptor
// (pure: no device needed). The default linear key reproduces the
// historic linear sampler (Linear/Linear/Nearest, anisotropy off) and
// the nearest/bicubic keys reproduce the historic nearest sampler, so
// old pictures keep their exact sampling. Contract frozen in
// game/tex/testdata/mipmap_cases.json ("sampler_contract").
func SamplerDescriptorForImageFilter(key ImageSamplerKey) hal.SamplerDescriptor {
	if key.Bicubic || key.Nearest {
		return hal.SamplerDescriptor{
			Label:        "image_sampler_r3_nearest",
			AddressModeU: types.AddressModeClampToEdge,
			AddressModeV: types.AddressModeClampToEdge,
			AddressModeW: types.AddressModeClampToEdge,
			MagFilter:    types.FilterModeNearest,
			MinFilter:    types.FilterModeNearest,
			MipmapFilter: types.MipmapFilterModeNearest,
			Anisotropy:   imageFilterAnisoMin,
		}
	}
	mip := types.MipmapFilterModeNearest
	label := "image_sampler_r3_linear"
	if key.MipmapLinear {
		mip = types.MipmapFilterModeLinear
		label = "image_sampler_r3_mipmap_linear"
	}
	aniso := NormalizeImageFilterAniso(key.Aniso)
	if aniso > imageFilterAnisoMin {
		label += "_aniso"
	}
	return hal.SamplerDescriptor{
		Label:        label,
		AddressModeU: types.AddressModeClampToEdge,
		AddressModeV: types.AddressModeClampToEdge,
		AddressModeW: types.AddressModeClampToEdge,
		MagFilter:    types.FilterModeLinear,
		MinFilter:    types.FilterModeLinear,
		MipmapFilter: mip,
		Anisotropy:   aniso,
	}
}

// canMergeImageDraw reports whether two image commands may share one bind group
// and multi-quad draw (same GPU texture key + sampling + opacity + viewport).
// Tint intentionally does NOT block the merge: tint rides per-vertex, so
// neighbors with different tints share one bind group and one multi-quad
// Draw (1000 same-texture particles stay one commit). GPUTextureDraw keeps
// the stricter rule (no tint channel there).
func canMergeImageDraw(a, b *ImageDrawCommand) bool {
	if a == nil || b == nil {
		return false
	}
	if a.GenerationID == 0 || a.GenerationID != b.GenerationID {
		return false
	}
	if a.Nearest != b.Nearest || a.Bicubic != b.Bicubic || a.Opacity != b.Opacity {
		return false
	}
	// R3 per-picture switch: different effective samplers must not share
	// one bind group. Zero-value commands share the default key, so old
	// callers merge exactly as before.
	if a.FilterKey() != b.FilterKey() {
		return false
	}
	if a.ViewportWidth != b.ViewportWidth || a.ViewportHeight != b.ViewportHeight {
		return false
	}
	if a.ImgWidth != b.ImgWidth || a.ImgHeight != b.ImgHeight {
		return false
	}
	return true
}

// canMergeGPUTextureDraw reports whether two GPU-to-GPU texture overlays may share
// one bind group + multi-quad Draw. Same texture view pointer, opacity, and
// viewport; never merge across scissor seals (caller enforces batchSeal).
func canMergeGPUTextureDraw(a, b *GPUTextureDrawCommand) bool {
	if a == nil || b == nil {
		return false
	}
	if a.View.IsNil() || b.View.IsNil() {
		return false
	}
	if !a.View.Equals(b.View) {
		return false
	}
	if a.Opacity != b.Opacity {
		return false
	}
	if a.ViewportWidth != b.ViewportWidth || a.ViewportHeight != b.ViewportHeight {
		return false
	}
	return true
}

// imageVertexLayout returns the vertex buffer layout for the textured quad pipeline.
func imageVertexLayout() []types.VertexBufferLayout {
	return []types.VertexBufferLayout{
		{
			ArrayStride: imageVertexStride,
			StepMode:    types.VertexStepModeVertex,
			Attributes: []types.VertexAttribute{
				{Format: types.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},  // position
				{Format: types.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1},  // tex_coord
				{Format: types.VertexFormatFloat32x4, Offset: 16, ShaderLocation: 2}, // tint (premul)
			},
		},
	}
}

// buildImageVertices generates vertex data for a single image quad.
// Returns 6 vertices (2 triangles: TL, TR, BL, TR, BR, BL).
// Corners may form a rotated/skewed parallelogram after CTM.
func buildImageVertices(cmd *ImageDrawCommand) []byte {
	const vertsPerQuad = 6
	buf := make([]byte, vertsPerQuad*imageVertexStride)
	buildImageVerticesInto(buf, cmd)
	return buf
}

// buildImageVerticesInto writes one image quad (6 verts) into dst.
// dst must have length >= 6*imageVertexStride.
func buildImageVerticesInto(dst []byte, cmd *ImageDrawCommand) {
	const vertsPerQuad = 6
	need := vertsPerQuad * imageVertexStride
	if len(dst) < need {
		panic("buildImageVerticesInto: dst too small")
	}

	// Prefer explicit CTM-transformed corners. Fall back to axis-aligned
	// DstX/Y/W/H when corners were not populated (legacy callers / texture overlays).
	x0, y0 := cmd.TLX, cmd.TLY
	x1, y1 := cmd.TRX, cmd.TRY
	x2, y2 := cmd.BRX, cmd.BRY
	x3, y3 := cmd.BLX, cmd.BLY
	if x0 == 0 && y0 == 0 && x1 == 0 && y1 == 0 && x2 == 0 && y2 == 0 && x3 == 0 && y3 == 0 &&
		(cmd.DstW != 0 || cmd.DstH != 0 || cmd.DstX != 0 || cmd.DstY != 0) {
		x0, y0 = cmd.DstX, cmd.DstY
		x1, y1 = cmd.DstX+cmd.DstW, cmd.DstY
		x2, y2 = cmd.DstX+cmd.DstW, cmd.DstY+cmd.DstH
		x3, y3 = cmd.DstX, cmd.DstY+cmd.DstH
	}

	// UV coordinates.
	u0, v0, u1, v1 := cmd.U0, cmd.V0, cmd.U1, cmd.V1

	// Per-quad tint (premultiplied straight); zero struct = identity white.
	tr, tg, tb, ta := cmd.TintR, cmd.TintG, cmd.TintB, cmd.TintA
	if tr == 0 && tg == 0 && tb == 0 && ta == 0 {
		tr, tg, tb, ta = 1, 1, 1, 1
	}

	// Triangle 1: TL, TR, BL
	// Triangle 2: TR, BR, BL
	verts := [6][8]float32{
		{x0, y0, u0, v0, tr, tg, tb, ta}, // TL
		{x1, y1, u1, v0, tr, tg, tb, ta}, // TR
		{x3, y3, u0, v1, tr, tg, tb, ta}, // BL
		{x1, y1, u1, v0, tr, tg, tb, ta}, // TR
		{x2, y2, u1, v1, tr, tg, tb, ta}, // BR
		{x3, y3, u0, v1, tr, tg, tb, ta}, // BL
	}

	offset := 0
	for _, v := range verts {
		binary.LittleEndian.PutUint32(dst[offset:], math.Float32bits(v[0]))
		binary.LittleEndian.PutUint32(dst[offset+4:], math.Float32bits(v[1]))
		binary.LittleEndian.PutUint32(dst[offset+8:], math.Float32bits(v[2]))
		binary.LittleEndian.PutUint32(dst[offset+12:], math.Float32bits(v[3]))
		binary.LittleEndian.PutUint32(dst[offset+16:], math.Float32bits(v[4]))
		binary.LittleEndian.PutUint32(dst[offset+20:], math.Float32bits(v[5]))
		binary.LittleEndian.PutUint32(dst[offset+24:], math.Float32bits(v[6]))
		binary.LittleEndian.PutUint32(dst[offset+28:], math.Float32bits(v[7]))
		offset += imageVertexStride
	}
}

// makeImageUniform creates the uniform buffer data for an image draw.
// Contains an orthographic projection matrix and opacity.
func makeImageUniform(viewportW, viewportH uint32, opacity float32) []byte {
	return makeImageUniformInto(nil, viewportW, viewportH, opacity)
}

// makeImageUniformInto writes image uniforms into buf (reused when cap >= imageUniformSize).
func makeImageUniformInto(buf []byte, viewportW, viewportH uint32, opacity float32) []byte {
	if cap(buf) < int(imageUniformSize) {
		buf = make([]byte, imageUniformSize)
	} else {
		buf = buf[:imageUniformSize]
		clear(buf)
	}
	putImageUniform(buf, viewportW, viewportH, opacity)
	return buf
}

// putImageUniform writes one image uniform block at the start of dst.
// dst must have length >= imageUniformSize. Bytes beyond the 80-byte payload
// are left untouched (important for 256-byte slab slots).
func putImageUniform(dst []byte, viewportW, viewportH uint32, opacity float32) {
	if len(dst) < int(imageUniformSize) {
		panic("putImageUniform: dst too small")
	}
	// Clear only the payload (slot padding may already be zero from slab alloc).
	for i := 0; i < int(imageUniformSize); i++ {
		dst[i] = 0
	}
	w := float32(viewportW)
	h := float32(viewportH)
	binary.LittleEndian.PutUint32(dst[0:], math.Float32bits(2.0/w))
	binary.LittleEndian.PutUint32(dst[20:], math.Float32bits(-2.0/h))
	binary.LittleEndian.PutUint32(dst[40:], math.Float32bits(1.0))
	binary.LittleEndian.PutUint32(dst[48:], math.Float32bits(-1.0))
	binary.LittleEndian.PutUint32(dst[52:], math.Float32bits(1.0))
	binary.LittleEndian.PutUint32(dst[60:], math.Float32bits(1.0))
	binary.LittleEndian.PutUint32(dst[64:], math.Float32bits(opacity))
}

// SamplerFor returns the sampler for the command filter mode (I.03).
// Bicubic convolution taps samples manually with the nearest sampler
// (hardware filtering must not re-interpolate between taps).
// Historic behaviour is frozen: R3 keys that reproduce the two historic
// samplers return the same pointers; use SamplerForFilter for the full
// per-picture switch.
func (p *TexturedQuadPipeline) SamplerFor(nearest bool) hal.Sampler {
	if nearest && p.nearestSampler != nil {
		return p.nearestSampler
	}
	return p.sampler
}

// SamplerForFilter returns the sampler for one picture's full R3 switch
// Default keys return the two historic samplers (same pointers as
// SamplerFor); mipmap-linear or aniso>1 keys lazily create cached
// samplers. Never crashes: nil pipeline, missing device, or a failed
// allocation all fall back to the historic sampler.
func (p *TexturedQuadPipeline) SamplerForFilter(key ImageSamplerKey) hal.Sampler {
	if p == nil {
		return nil
	}
	key.Aniso = NormalizeImageFilterAniso(key.Aniso)
	if key.Bicubic || key.Nearest {
		if p.nearestSampler != nil {
			return p.nearestSampler
		}
		return p.sampler
	}
	if !key.MipmapLinear && key.Aniso <= imageFilterAnisoMin {
		return p.sampler
	}
	if p.device == nil {
		return p.sampler
	}
	if s, ok := p.filterSamplers[key]; ok && s != nil {
		return s
	}
	desc := SamplerDescriptorForImageFilter(key)
	s, err := p.device.CreateSampler(&desc)
	if err != nil {
		return p.sampler
	}
	if p.filterSamplers == nil {
		p.filterSamplers = map[ImageSamplerKey]hal.Sampler{}
	}
	p.filterSamplers[key] = s
	return s
}
