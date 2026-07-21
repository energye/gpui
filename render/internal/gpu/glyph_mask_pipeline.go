//go:build !nogpu

package gpu

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

// Embedded glyph mask shader sources.
//
//go:embed shaders/glyph_mask.wgsl
var glyphMaskShaderSource string

//go:embed shaders/glyph_mask_lcd.wgsl
var glyphMaskLCDShaderSource string

// glyphMaskVertexStride is the byte stride per vertex in the glyph mask pipeline.
// Layout per vertex (matches MSDF text pipeline for Intel Vulkan driver compat):
//
//	position  (vec2<f32>) =  8 bytes  (location 0)
//	tex_coord (vec2<f32>) =  8 bytes  (location 1)
//
// Total = 16 bytes per vertex.
// Color is passed via per-batch uniform buffer (not per-vertex).
const glyphMaskVertexStride = 16

// glyphMaskUniformSize is the byte size of the glyph mask uniform buffer.
// Layout (grayscale):
//
//	transform (mat4x4<f32>) = 64 bytes
//	color     (vec4<f32>)   = 16 bytes
//
// Total = 80 bytes.
const glyphMaskUniformSize = 80

// glyphMaskLCDUniformSize is the byte size of the LCD uniform buffer.
// Layout:
//
//	transform  (mat4x4<f32>) = 64 bytes
//	color      (vec4<f32>)   = 16 bytes
//	atlas_size (vec2<f32>)   =  8 bytes
//	_pad       (vec2<f32>)   =  8 bytes
//
// Total = 96 bytes.
const glyphMaskLCDUniformSize = 96

// GlyphMaskPipeline manages GPU resources for alpha mask text rendering
// (Tier 6). Each text run is rendered as a set of textured quads using
// indexed drawing. The fragment shader samples a single-channel (R8) alpha
// atlas and multiplies by the text color for premultiplied output.
//
// The pipeline uses the same MSAA+depth/stencil texture pattern as
// MSDFTextPipeline for unified render pass integration. A pipelineWithStencil
// variant is provided when the render pass includes a depth/stencil
// attachment (stencil test is Always/Keep -- text does not interact with stencil).
//
// Architecture:
//
//	GPURenderSession owns persistent buffers (vertex, index, uniform)
//	GlyphMaskPipeline owns shader, layout, pipeline, sampler
//	bind groups are created per atlas texture (uniform + texture + sampler)
type GlyphMaskPipeline struct {
	device      *webgpu.Device
	queue       *webgpu.Queue
	sampleCount uint32 // MSAA sample count (4 or 1), from GPUShared

	// GPU objects for the render pipeline.
	shader        *webgpu.ShaderModule
	uniformLayout *webgpu.BindGroupLayout
	pipeLayout    *webgpu.PipelineLayout
	pipeline      *webgpu.RenderPipeline

	// Session-compatible pipeline variant with depth/stencil state.
	// Used when text participates in a unified render pass that includes
	// a stencil attachment (for stencil-then-cover paths).
	// Stencil test is Always/Keep (text does not interact with stencil).
	pipelineWithStencil *webgpu.RenderPipeline

	// Depth-clipped pipeline variant (GPU-CLIP-003a). Same as pipelineWithStencil
	// but with DepthCompare=GreaterEqual to test against the depth clip buffer.
	// Created on demand when a ScissorGroup has ClipPath set.
	pipelineWithDepthClip *webgpu.RenderPipeline

	// Default sampler for R8 atlas textures (linear filtering for smooth
	// alpha interpolation at subpixel positions).
	sampler *webgpu.Sampler

	// LCD pipeline: separate shader + pipeline for ClearType rendering.
	// Uses a different uniform struct (96 bytes with atlas_size) and a
	// different fragment shader (per-channel alpha compositing).
	// This avoids the Intel Vulkan null pipeline handle bug caused by
	// adding is_lcd to the grayscale uniform struct.
	lcdShader        *webgpu.ShaderModule
	lcdUniformLayout *webgpu.BindGroupLayout
	lcdPipeLayout    *webgpu.PipelineLayout
	// Two-pass LCD (true per-channel ClearType without dual-source blending):
	// 1) darken: out = dst * (1 - cov_rgb)   blend Zero / OneMinusSrc
	// 2) add:    out = dst + color * cov_rgb blend One / One
	lcdPipelineDarken *webgpu.RenderPipeline
	lcdPipelineAdd    *webgpu.RenderPipeline
	// lcdPipelineWithStencil is kept as an alias of lcdPipelineAdd for older checks.
	lcdPipelineWithStencil *webgpu.RenderPipeline

	// clipBindLayout is the shared @group(1) bind group layout for RRect clip.
	// Set by the session before ensurePipelineWithStencil.
	clipBindLayout *webgpu.BindGroupLayout
	// pipeLayoutHasClip tracks whether the current pipeLayout was created
	// with clipBindLayout included. If clipBindLayout is set after the
	// layout was created, the pipeline must be recreated.
	pipeLayoutHasClip bool
	// lcdPipeLayoutHasClip tracks the same for the LCD pipeline layout.
	lcdPipeLayoutHasClip bool
}

// NewGlyphMaskPipeline creates a new glyph mask pipeline with the given device
// and queue. The render pipeline and GPU objects are not created until
// ensurePipelineWithStencil is called.
func NewGlyphMaskPipeline(device *webgpu.Device, queue *webgpu.Queue, sampleCount uint32) *GlyphMaskPipeline {
	return &GlyphMaskPipeline{
		device:      device,
		queue:       queue,
		sampleCount: sampleCount,
	}
}

// SetClipBindLayout sets the bind group layout for the @group(1) RRect clip
// uniform. Must be called before ensurePipelineWithStencil. The layout is
// owned by the session and must not be destroyed by the pipeline.
func (p *GlyphMaskPipeline) SetClipBindLayout(layout *webgpu.BindGroupLayout) {
	p.clipBindLayout = layout
}

// Destroy releases all GPU resources held by the pipeline. Safe to call
// multiple times or on a pipeline with no allocated resources.
func (p *GlyphMaskPipeline) Destroy() {
	p.destroyLCDPipeline()
	p.destroyPipeline()
	if p.sampler != nil {
		p.sampler.Release()
		p.sampler = nil
	}
}

// ensureSharedResources compiles the shader and creates the bind group layout,
// pipeline layout, and sampler. These are shared between the base and stencil
// pipeline variants. Separated from pipeline creation to allow the stencil
// variant to be created even if the base (non-stencil) pipeline fails on
// some Intel drivers.
func (p *GlyphMaskPipeline) ensureSharedResources() error {
	if p.shader != nil && p.uniformLayout != nil && p.pipeLayout != nil && p.sampler != nil {
		return nil
	}

	if glyphMaskShaderSource == "" {
		return fmt.Errorf("glyph_mask shader source is empty")
	}

	// destroyPipeline clears shader/layouts but intentionally keeps sampler
	// (shared across stencil/clip rebuilds). Only create missing pieces so we
	// never overwrite a live GPU object (sampler leak → AutoRecover OOM).
	if p.shader == nil {
		shader, err := p.device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
			Label: "glyph_mask_shader",
			WGSL:  glyphMaskShaderSource,
		})
		if err != nil {
			return fmt.Errorf("compile glyph_mask shader: %w", err)
		}
		p.shader = shader
	}

	// Bind group layout:
	//   Binding 0: GlyphMaskUniforms (uniform buffer, vertex+fragment)
	//   Binding 1: R8 atlas texture (texture_2d, fragment)
	//   Binding 2: Sampler (fragment)
	if p.uniformLayout == nil {
		uniformLayout, err := p.device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
			Label: "glyph_mask_uniform_layout",
			Entries: []types.BindGroupLayoutEntry{
				{
					Binding:    0,
					Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
					Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: glyphMaskUniformSize},
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
			return fmt.Errorf("create glyph_mask uniform layout: %w", err)
		}
		p.uniformLayout = uniformLayout
	}

	if p.pipeLayout == nil {
		glyphBGLayouts := []*webgpu.BindGroupLayout{p.uniformLayout}
		hasClip := p.clipBindLayout != nil
		if hasClip {
			glyphBGLayouts = append(glyphBGLayouts, p.clipBindLayout)
		}
		pipeLayout, err := p.device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
			Label:            "glyph_mask_pipe_layout",
			BindGroupLayouts: glyphBGLayouts,
		})
		if err != nil {
			return fmt.Errorf("create glyph_mask pipeline layout: %w", err)
		}
		p.pipeLayout = pipeLayout
		p.pipeLayoutHasClip = hasClip
	}

	// Create sampler for R8 atlas textures.
	// Nearest filtering: glyph masks are CPU-rasterized at exact device pixel size
	// with subpixel hinting. Linear filtering would blur the already-hinted bitmaps.
	if p.sampler == nil {
		sampler, err := p.device.CreateSampler(&webgpu.SamplerDescriptor{
			Label:        "glyph_mask_sampler",
			AddressModeU: types.AddressModeClampToEdge,
			AddressModeV: types.AddressModeClampToEdge,
			AddressModeW: types.AddressModeClampToEdge,
			MagFilter:    types.FilterModeNearest,
			MinFilter:    types.FilterModeNearest,
			MipmapFilter: types.MipmapFilterModeNearest,
		})
		if err != nil {
			return fmt.Errorf("create glyph_mask sampler: %w", err)
		}
		p.sampler = sampler
	}

	return nil
}

// ensurePipelineWithStencil creates the session-compatible pipeline variant
// that includes a depth/stencil state. This pipeline is used when text is
// rendered in a unified render pass alongside stencil-then-cover paths.
// The stencil test is Always/Keep (text does not interact with stencil).
//
// The base pipeline (shader, layout, sampler) is created first if it
// doesn't exist.
func (p *GlyphMaskPipeline) ensurePipelineWithStencil() error {
	// Ensure shared resources exist (shader, layouts, sampler).
	if err := p.ensureSharedResources(); err != nil {
		return err
	}
	// If the pipeline layout was created without clip but clip is now set,
	// destroy and recreate so the layout includes @group(1). Without this,
	// SetBindGroup(1, clipBG) crashes on AMD/NVIDIA (Intel tolerates it).
	if p.clipBindLayout != nil && !p.pipeLayoutHasClip {
		p.destroyPipeline()
		if err := p.ensureSharedResources(); err != nil {
			return err
		}
	}
	if p.pipelineWithStencil != nil {
		return nil
	}

	premulBlend := types.BlendStatePremultiplied()
	pipeline, err := p.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "glyph_mask_pipeline_with_stencil",
		Layout: p.pipeLayout,
		Vertex: webgpu.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    glyphMaskVertexLayout(),
		},
		Fragment: &webgpu.FragmentState{
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
		return fmt.Errorf("create glyph mask pipeline with stencil: %w", err)
	}
	p.pipelineWithStencil = pipeline
	return nil
}

// ensureDepthClipPipeline creates the depth-clipped pipeline variant if needed.
// This variant uses DepthCompare=GreaterEqual for depth-based arbitrary path
// clipping (GPU-CLIP-003a).
func (p *GlyphMaskPipeline) ensureDepthClipPipeline() error {
	if p.pipelineWithDepthClip != nil {
		return nil
	}
	if err := p.ensurePipelineWithStencil(); err != nil {
		return err
	}

	premulBlend := types.BlendStatePremultiplied()
	pipeline, err := p.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "glyph_mask_pipeline_depth_clip",
		Layout: p.pipeLayout,
		Vertex: webgpu.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    glyphMaskVertexLayout(),
		},
		Fragment: &webgpu.FragmentState{
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
		return fmt.Errorf("create glyph mask pipeline with depth clip: %w", err)
	}
	p.pipelineWithDepthClip = pipeline
	return nil
}

// RecordDraws records glyph mask draw commands into an existing render pass.
// The render pass is owned by GPURenderSession. This method uses the
// pipelineWithStencil variant because the session's render pass includes
// a depth/stencil attachment.
//
// When depthClipped is true (GPU-CLIP-003a), the depth-clipped pipeline
// variant is used to test fragments against the depth clip buffer.
//
// The resources parameter holds pre-built vertex/index buffers, uniform buffer,
// and bind group for the current frame. If isLCD is true and the LCD pipeline
// is available, the LCD pipeline is used for per-channel alpha compositing.
func (p *GlyphMaskPipeline) RecordDraws(rp *webgpu.RenderPassEncoder, resources *glyphMaskFrameResources, clipBG *webgpu.BindGroup, depthClipped ...bool) {
	if resources == nil || len(resources.drawCalls) == 0 {
		return
	}

	useDepthClip := len(depthClipped) > 0 && depthClipped[0] && p.pipelineWithDepthClip != nil

	rp.SetVertexBuffer(0, resources.vertBuf, 0)
	rp.SetIndexBuffer(resources.idxBuf, types.IndexFormatUint16, 0)

	drawOne := func(pipeline *webgpu.RenderPipeline, dc glyphMaskDrawCall) {
		if pipeline == nil || dc.indexCount == 0 || dc.bindGroup == nil {
			return
		}
		// Clear prior bind groups before pipeline switch (LCD two-pass and
		// grayscale share different uniform min_binding_size layouts).
		clearPassBindGroups(rp)
		rp.SetPipeline(pipeline)
		if clipBG != nil {
			rp.SetBindGroup(1, clipBG, nil)
		}
		rp.SetBindGroup(0, dc.bindGroup, nil)
		rp.DrawIndexed(dc.indexCount, 1, dc.indexOffset, 0, 0)
	}

	// Per-draw pipeline selection. Never force LCD pipeline for grayscale draws
	// in a mixed frame (bind group layouts differ: 80 vs 96 byte uniforms).
	for _, dc := range resources.drawCalls {
		if dc.indexCount == 0 {
			continue
		}
		switch {
		case useDepthClip:
			// Depth-clip path currently only has grayscale stencil variant.
			drawOne(p.pipelineWithDepthClip, dc)
		case dc.isLCD && p.lcdPipelineDarken != nil && p.lcdPipelineAdd != nil:
			// Two-pass LCD ClearType (no dual-source blend required).
			drawOne(p.lcdPipelineDarken, dc)
			drawOne(p.lcdPipelineAdd, dc)
		case dc.isLCD && p.lcdPipelineWithStencil != nil:
			drawOne(p.lcdPipelineWithStencil, dc)
		default:
			drawOne(p.pipelineWithStencil, dc)
		}
	}
}

// ensureLCDPipelineWithStencil creates the LCD pipeline variant for ClearType
// rendering. Uses a separate shader (glyph_mask_lcd.wgsl) with a different
// uniform struct (96 bytes: includes atlas_size for texel stepping).
//
// This is a separate pipeline from the grayscale one (Skia pattern) to avoid
// the Intel Vulkan null pipeline handle bug that occurs when adding fields to
// the grayscale uniform struct.
func (p *GlyphMaskPipeline) ensureLCDPipelineWithStencil() error {
	// Ensure base resources exist (sampler is shared).
	if err := p.ensureSharedResources(); err != nil {
		return err
	}

	// If the LCD pipeline layout was created without clip but clip is now set,
	// destroy and recreate.
	if p.clipBindLayout != nil && !p.lcdPipeLayoutHasClip {
		p.destroyLCDPipeline()
	}
	if p.lcdPipelineDarken != nil && p.lcdPipelineAdd != nil {
		return nil
	}

	if glyphMaskLCDShaderSource == "" {
		return fmt.Errorf("glyph_mask_lcd shader source is empty")
	}

	lcdShader, err := p.device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "glyph_mask_lcd_shader",
		WGSL:  glyphMaskLCDShaderSource,
	})
	if err != nil {
		return fmt.Errorf("compile glyph_mask_lcd shader: %w", err)
	}
	p.lcdShader = lcdShader

	// LCD bind group layout: same bindings as grayscale (uniform, texture, sampler)
	// but with a larger uniform buffer (96 bytes instead of 80).
	lcdUniformLayout, err := p.device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "glyph_mask_lcd_uniform_layout",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
				Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: glyphMaskLCDUniformSize},
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
		return fmt.Errorf("create glyph_mask_lcd uniform layout: %w", err)
	}
	p.lcdUniformLayout = lcdUniformLayout

	lcdBGLayouts := []*webgpu.BindGroupLayout{p.lcdUniformLayout}
	hasClip := p.clipBindLayout != nil
	if hasClip {
		lcdBGLayouts = append(lcdBGLayouts, p.clipBindLayout)
	}
	lcdPipeLayout, err := p.device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label:            "glyph_mask_lcd_pipe_layout",
		BindGroupLayouts: lcdBGLayouts,
	})
	if err != nil {
		return fmt.Errorf("create glyph_mask_lcd pipeline layout: %w", err)
	}
	p.lcdPipeLayout = lcdPipeLayout
	p.lcdPipeLayoutHasClip = hasClip

	// Two-pass LCD blends (Skia/DirectWrite ClearType without dual-source).
	darkenBlend := types.BlendState{
		Color: types.BlendComponent{
			SrcFactor: types.BlendFactorZero,
			DstFactor: types.BlendFactorOneMinusSrc,
			Operation: types.BlendOperationAdd,
		},
		Alpha: types.BlendComponent{
			SrcFactor: types.BlendFactorZero,
			DstFactor: types.BlendFactorOneMinusSrc,
			Operation: types.BlendOperationAdd,
		},
	}
	addBlend := types.BlendState{
		Color: types.BlendComponent{
			SrcFactor: types.BlendFactorOne,
			DstFactor: types.BlendFactorOne,
			Operation: types.BlendOperationAdd,
		},
		Alpha: types.BlendComponent{
			SrcFactor: types.BlendFactorOne,
			DstFactor: types.BlendFactorOne,
			Operation: types.BlendOperationAdd,
		},
	}

	mkLCD := func(label, entry string, blend types.BlendState) (*webgpu.RenderPipeline, error) {
		b := blend
		return p.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
			Label:  label,
			Layout: p.lcdPipeLayout,
			Vertex: webgpu.VertexState{
				Module:     p.lcdShader,
				EntryPoint: shaderEntryVS,
				Buffers:    glyphMaskVertexLayout(),
			},
			Fragment: &webgpu.FragmentState{
				Module:     p.lcdShader,
				EntryPoint: entry,
				Targets: []types.ColorTargetState{
					{
						Format:    types.TextureFormatBGRA8Unorm,
						Blend:     &b,
						WriteMask: types.ColorWriteMaskAll,
					},
				},
			},
			DepthStencil: stencilPassthroughDepthStencil(),
			Primitive:    triangleListPrimitive(),
			Multisample:  multisampleState(p.sampleCount),
		})
	}

	darkenPipe, err := mkLCD("glyph_mask_lcd_darken", "fs_darken", darkenBlend)
	if err != nil {
		return fmt.Errorf("create glyph mask LCD darken pipeline: %w", err)
	}
	addPipe, err := mkLCD("glyph_mask_lcd_add", "fs_add", addBlend)
	if err != nil {
		darkenPipe.Release()
		return fmt.Errorf("create glyph mask LCD add pipeline: %w", err)
	}
	p.lcdPipelineDarken = darkenPipe
	p.lcdPipelineAdd = addPipe
	p.lcdPipelineWithStencil = addPipe
	return nil
}

// destroyPipeline releases all pipeline resources in reverse creation order.
func (p *GlyphMaskPipeline) destroyPipeline() {
	if p.device == nil {
		return
	}
	if p.pipelineWithDepthClip != nil {
		p.pipelineWithDepthClip.Release()
		p.pipelineWithDepthClip = nil
	}
	if p.pipelineWithStencil != nil {
		p.pipelineWithStencil.Release()
		p.pipelineWithStencil = nil
	}
	if p.pipeline != nil {
		p.pipeline.Release()
		p.pipeline = nil
	}
	if p.pipeLayout != nil {
		p.pipeLayout.Release()
		p.pipeLayout = nil
		p.pipeLayoutHasClip = false
	}
	if p.uniformLayout != nil {
		p.uniformLayout.Release()
		p.uniformLayout = nil
	}
	if p.shader != nil {
		p.shader.Release()
		p.shader = nil
	}
}

// destroyLCDPipeline releases LCD pipeline resources in reverse creation order.
func (p *GlyphMaskPipeline) destroyLCDPipeline() {
	if p.device == nil {
		return
	}
	// darken and add are distinct pipelines; withStencil may alias add.
	darken := p.lcdPipelineDarken
	add := p.lcdPipelineAdd
	stencil := p.lcdPipelineWithStencil
	p.lcdPipelineDarken = nil
	p.lcdPipelineAdd = nil
	p.lcdPipelineWithStencil = nil
	if darken != nil {
		darken.Release()
	}
	if add != nil && add != darken {
		add.Release()
	}
	if stencil != nil && stencil != darken && stencil != add {
		stencil.Release()
	}
	if p.lcdPipeLayout != nil {
		p.lcdPipeLayout.Release()
		p.lcdPipeLayout = nil
		p.lcdPipeLayoutHasClip = false
	}
	if p.lcdUniformLayout != nil {
		p.lcdUniformLayout.Release()
		p.lcdUniformLayout = nil
	}
	if p.lcdShader != nil {
		p.lcdShader.Release()
		p.lcdShader = nil
	}
}

// ---- Per-frame GPU resources ----

// glyphMaskDrawCall represents a single draw call within a glyph mask batch.
type glyphMaskDrawCall struct {
	indexOffset uint32 // first index in the shared index buffer
	indexCount  uint32 // number of indices for this draw
	bindGroup   *webgpu.BindGroup
	// isLCD selects LCD vs grayscale pipeline for THIS draw only.
	// Mixed LCD/grayscale batches in one frame must not share a frame-level pipeline
	// (LCD BGL minBindingSize=96 vs grayscale=80 → wgpu validation abort).
	isLCD bool
}

// glyphMaskFrameResources holds per-frame GPU resources for glyph mask rendering.
type glyphMaskFrameResources struct {
	vertBuf   *webgpu.Buffer
	idxBuf    *webgpu.Buffer
	drawCalls []glyphMaskDrawCall
	// isLCD is retained for diagnostics; RecordDraws uses per-drawCall isLCD.
	isLCD bool
}

// ---- Vertex layout ----

// glyphMaskVertexLayout returns the vertex buffer layout for the glyph mask pipeline.
// Matches VertexInput in glyph_mask.wgsl:
//
//	location 0: position  (vec2<f32>)
//	location 1: tex_coord (vec2<f32>)
//
// Color and is_lcd are in the uniform buffer (per-batch, not per-vertex).
// This matches the MSDF text pipeline layout and avoids Intel Vulkan driver
// issues with >2 vertex attributes.
func glyphMaskVertexLayout() []types.VertexBufferLayout {
	return []types.VertexBufferLayout{
		{
			ArrayStride: glyphMaskVertexStride,
			StepMode:    types.VertexStepModeVertex,
			Attributes: []types.VertexAttribute{
				{Format: types.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0}, // position
				{Format: types.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1}, // tex_coord
			},
		},
	}
}

// ---- Data types for GlyphMaskPipeline ----

// GlyphMaskQuad represents a single glyph quad for alpha mask rendering.
// Each glyph is rendered as a textured quad with position and UV.
type GlyphMaskQuad struct {
	// Position of quad corners in screen/local space.
	X0, Y0, X1, Y1 float32

	// UV coordinates in R8 atlas [0, 1].
	// For LCD glyphs, UVs span the 3x-wide region in the atlas.
	U0, V0, U1, V1 float32

	// Page is the glyph-mask atlas page index for these UVs (CPU metadata only;
	// not uploaded as a vertex attribute). Required when MaxAtlases > 1 —
	// hardcoding AtlasPageIndex=0 samples the wrong R8 texture after page0 fills.
	Page int
}

// GlyphMaskBatch represents a batch of glyph mask quads with shared
// rendering parameters. Multiple batches may use different atlas pages,
// transforms, or colors.
type GlyphMaskBatch struct {
	// Quads is the list of glyph quads to render.
	Quads []GlyphMaskQuad

	// Transform is the 2D affine transform for this batch.
	Transform render.Matrix

	// Color is the text color (RGBA, premultiplied alpha) for this batch.
	// All glyphs in a batch share the same color (set per DrawString call).
	Color [4]float32

	// IsLCD indicates this batch uses LCD subpixel rendering.
	// When true, the LCD pipeline is used with per-channel alpha compositing.
	// The atlas region contains 3 R8 texels per logical pixel (R, G, B coverage).
	IsLCD bool

	// AtlasWidth and AtlasHeight are the dimensions of the R8 atlas texture
	// in texels. Used by the LCD fragment shader to compute the texel step
	// for sampling adjacent R, G, B coverage texels.
	AtlasWidth  float32
	AtlasHeight float32

	// AtlasPageIndex identifies which atlas page (R8 texture) to use.
	AtlasPageIndex int
}

// CanMerge reports whether other can be merged into this batch.
// Batches are mergeable when they share the same visual properties
// (transform, color, LCD mode, atlas page) — only the quad list differs.
// This enables draw call coalescing: multiple DrawString calls with the
// same style produce a single GPU draw call (ADR-031).
func (b *GlyphMaskBatch) CanMerge(other GlyphMaskBatch) bool {
	return b.Transform == other.Transform &&
		b.Color == other.Color &&
		b.IsLCD == other.IsLCD &&
		b.AtlasPageIndex == other.AtlasPageIndex
}

// SplitGlyphMaskBatchByPage expands a batch whose quads may reference multiple
// atlas pages into one batch per consecutive page run. Draw order is preserved.
// Single-page batches (the common case) return a one-element slice with
// AtlasPageIndex set from the quads.
func SplitGlyphMaskBatchByPage(batch GlyphMaskBatch) []GlyphMaskBatch {
	if len(batch.Quads) == 0 {
		return nil
	}
	// Legacy path: Page unset on all quads → trust batch.AtlasPageIndex.
	allZeroPage := true
	for i := range batch.Quads {
		if batch.Quads[i].Page != 0 {
			allZeroPage = false
			break
		}
	}
	if allZeroPage {
		out := batch
		if out.AtlasPageIndex < 0 {
			out.AtlasPageIndex = 0
		}
		return []GlyphMaskBatch{out}
	}

	out := make([]GlyphMaskBatch, 0, 2)
	start := 0
	page := batch.Quads[0].Page
	for i := 1; i <= len(batch.Quads); i++ {
		if i < len(batch.Quads) && batch.Quads[i].Page == page {
			continue
		}
		out = append(out, GlyphMaskBatch{
			Quads:          batch.Quads[start:i],
			Transform:      batch.Transform,
			Color:          batch.Color,
			IsLCD:          batch.IsLCD,
			AtlasWidth:     batch.AtlasWidth,
			AtlasHeight:    batch.AtlasHeight,
			AtlasPageIndex: page,
		})
		if i < len(batch.Quads) {
			start = i
			page = batch.Quads[i].Page
		}
	}
	return out
}

// ---- Vertex/index/uniform data builders ----

// buildGlyphMaskVertexDataInto serializes GlyphMaskQuad slices into raw vertex
// bytes suitable for GPU upload. Each quad produces 4 vertices x 16 bytes = 64 bytes.
// When data has enough capacity it is reused (zero-alloc hot path).
func buildGlyphMaskVertexDataInto(data []byte, quads []GlyphMaskQuad) []byte {
	if len(quads) == 0 {
		return nil
	}
	need := len(quads) * 4 * glyphMaskVertexStride
	if cap(data) < need {
		data = make([]byte, need)
	} else {
		data = data[:need]
	}
	off := 0
	for _, q := range quads {
		// Vertex 0: top-left
		writeGlyphMaskVertex(data[off:], q.X0, q.Y0, q.U0, q.V0)
		off += glyphMaskVertexStride
		// Vertex 1: top-right
		writeGlyphMaskVertex(data[off:], q.X1, q.Y0, q.U1, q.V0)
		off += glyphMaskVertexStride
		// Vertex 2: bottom-right
		writeGlyphMaskVertex(data[off:], q.X1, q.Y1, q.U1, q.V1)
		off += glyphMaskVertexStride
		// Vertex 3: bottom-left
		writeGlyphMaskVertex(data[off:], q.X0, q.Y1, q.U0, q.V1)
		off += glyphMaskVertexStride
	}
	return data
}

// writeGlyphMaskVertex writes a single glyph mask vertex into buf.
// Only position and texcoord are per-vertex; color/isLCD are per-batch uniform.
func writeGlyphMaskVertex(buf []byte, x, y, u, v float32) {
	binary.LittleEndian.PutUint32(buf[0:4], math.Float32bits(x))
	binary.LittleEndian.PutUint32(buf[4:8], math.Float32bits(y))
	binary.LittleEndian.PutUint32(buf[8:12], math.Float32bits(u))
	binary.LittleEndian.PutUint32(buf[12:16], math.Float32bits(v))
}

// buildGlyphMaskIndexDataInto serializes quad indices into raw bytes for GPU upload.
// Uses the same index pattern as MSDF text: 0,1,2, 2,3,0 per quad.
func buildGlyphMaskIndexDataInto(data []byte, numQuads int) []byte {
	if numQuads <= 0 {
		return nil
	}
	need := numQuads * 6 * 2
	if cap(data) < need {
		data = make([]byte, need)
	} else {
		data = data[:need]
	}
	for i := 0; i < numQuads; i++ {
		base := i * 12          // 6 indices * 2 bytes
		vertex := uint16(i * 4) //nolint:gosec // bounded by MaxQuadCapacity
		binary.LittleEndian.PutUint16(data[base+0:], vertex+0)
		binary.LittleEndian.PutUint16(data[base+2:], vertex+1)
		binary.LittleEndian.PutUint16(data[base+4:], vertex+2)
		binary.LittleEndian.PutUint16(data[base+6:], vertex+2)
		binary.LittleEndian.PutUint16(data[base+8:], vertex+3)
		binary.LittleEndian.PutUint16(data[base+10:], vertex+0)
	}
	return data
}

// makeGlyphMaskUniformInto creates the 80-byte uniform buffer for a glyph mask batch.
// The uniform contains the transform matrix and text color.
func makeGlyphMaskUniformInto(buf []byte, transform render.Matrix, color [4]float32) []byte {
	if cap(buf) < int(glyphMaskUniformSize) {
		buf = make([]byte, glyphMaskUniformSize)
	} else {
		buf = buf[:glyphMaskUniformSize]
		for i := range buf {
			buf[i] = 0
		}
	}
	off := 0

	// Transform: WGSL mat4x4<f32> is stored COLUMN-MAJOR in memory.
	// Column-major storage for WGSL:
	//   col0=[A,D,0,0]  col1=[B,E,0,0]  col2=[0,0,1,0]  col3=[C,F,0,1]
	t := [16]float32{
		float32(transform.A), float32(transform.D), 0, 0, // column 0
		float32(transform.B), float32(transform.E), 0, 0, // column 1
		0, 0, 1, 0, // column 2
		float32(transform.C), float32(transform.F), 0, 1, // column 3
	}
	for _, v := range t {
		binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(v))
		off += 4
	}

	// Color (vec4<f32>): premultiplied RGBA.
	for i := range 4 {
		binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(color[i]))
		off += 4
	}

	return buf
}

// makeGlyphMaskLCDUniform creates the 96-byte uniform buffer for an LCD batch.
// The uniform contains the transform matrix, text color, and atlas dimensions
// (needed by the LCD fragment shader to compute the texel step for sampling
// adjacent R, G, B coverage texels).
func makeGlyphMaskLCDUniform(transform render.Matrix, color [4]float32, atlasW, atlasH float32) []byte {
	return makeGlyphMaskLCDUniformInto(nil, transform, color, atlasW, atlasH)
}

// makeGlyphMaskLCDUniformInto is the R7.1 zero-realloc form of makeGlyphMaskLCDUniform.
func makeGlyphMaskLCDUniformInto(buf []byte, transform render.Matrix, color [4]float32, atlasW, atlasH float32) []byte {
	if cap(buf) < int(glyphMaskLCDUniformSize) {
		buf = make([]byte, glyphMaskLCDUniformSize)
	} else {
		buf = buf[:glyphMaskLCDUniformSize]
		clear(buf)
	}
	off := 0

	// Transform (mat4x4<f32>, column-major).
	t := [16]float32{
		float32(transform.A), float32(transform.D), 0, 0,
		float32(transform.B), float32(transform.E), 0, 0,
		0, 0, 1, 0,
		float32(transform.C), float32(transform.F), 0, 1,
	}
	for _, v := range t {
		binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(v))
		off += 4
	}

	// Color (vec4<f32>): premultiplied RGBA.
	for i := range 4 {
		binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(color[i]))
		off += 4
	}

	// Atlas size (vec2<f32>).
	binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(atlasW))
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(atlasH))
	off += 4

	// Padding (vec2<f32>): align to 16-byte boundary.
	binary.LittleEndian.PutUint32(buf[off:], 0)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 0)

	return buf
}
