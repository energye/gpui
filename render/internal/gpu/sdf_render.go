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
	"sync"
	"unsafe"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

//go:embed shaders/sdf_render.wgsl
var sdfRenderShaderSource string

// sdfRenderVertexStride is the byte stride per vertex in the SDF render pipeline.
// Layout per vertex:
//
//	position (vec2<f32>) = 8 bytes (location 0)
//	local (vec2<f32>) = 8 bytes (location 1)
//	shape_kind (f32) = 4 bytes (location 2)
//	param1 (f32) = 4 bytes (location 3)
//	param2 (f32) = 4 bytes (location 4)
//	param3 (f32) = 4 bytes (location 5)
//	half_stroke (f32) = 4 bytes (location 6)
//	is_stroked (f32) = 4 bytes (location 7)
//	color (vec4<f32>) = 16 bytes (location 8)
//
// Total = 56 bytes per vertex.
const sdfRenderVertexStride = 64

// sdfRenderUniformSize is the byte size of the SDF render uniform buffer.
// Layout: viewport (vec2<f32>) + padding (vec2<f32>) = 16 bytes.
const sdfRenderUniformSize = 16

// sdfRenderAAMargin is the anti-aliasing padding in pixels added to each side
// of the bounding quad. This ensures the smoothstep transition zone at shape
// edges is fully covered.
const sdfRenderAAMargin = 1.5

// SDFRenderPipeline manages GPU resources for SDF-based shape rendering
// via a vertex+fragment render pipeline. Instead of a compute shader that
// writes to a storage buffer, this approach draws bounding quads with a
// fragment shader that evaluates the SDF per pixel.
//
// The pipeline uses the same MSAA+resolve texture pattern as StencilRenderer.
// For unified rendering via GPURenderSession, pipelineWithStencil is used
// when the render pass includes a depth/stencil attachment.
type SDFRenderPipeline struct {
	device      hal.Device
	queue       hal.Queue
	sampleCount uint32 // MSAA sample count (4 or 1), from GPUShared

	// mu serializes pipeline lifecycle (create/destroy) on this shared
	// pipeline across windows — same contract as StencilRenderer.mu.
	// Record/draw stays lock-free; handles are immutable once built.
	mu sync.Mutex

	// GPU objects for the render pipeline.
	shader        hal.ShaderModule
	uniformLayout hal.BindGroupLayout
	pipeLayout    hal.PipelineLayout
	pipeline      hal.RenderPipeline

	// Session-compatible pipeline variant with depth/stencil state.
	// This is used when the SDF pipeline participates in a unified render
	// pass that includes a stencil attachment (for stencil-then-cover paths).
	// The stencil test is Always/Keep (SDF shapes don't interact with stencil).
	pipelineWithStencil hal.RenderPipeline

	// Depth-clipped pipeline variant (GPU-CLIP-003a). Same as pipelineWithStencil
	// but with DepthCompare=GreaterEqual to test against the depth clip buffer.
	// Created on demand when a ScissorGroup has ClipPath set.
	pipelineWithDepthClip hal.RenderPipeline

	// Set by the session before
	// pipeline creation. When non-nil, included in the pipeline layout.
	clipBindLayout hal.BindGroupLayout
	// defaultClipBindLayout is owned by this pipeline and used only when a
	// standalone pipeline is created before the session supplies its layout.
	defaultClipBindLayout hal.BindGroupLayout
	// pipeLayoutHasClip tracks whether the current pipeLayout was created
	// with clipBindLayout included. If clipBindLayout is set after the
	// layout was created, the pipeline must be recreated.
	pipeLayoutHasClip bool

	// Usually set by
	// the session (not owned). maskLayoutOwned is true only for standalone
	// pipelines that create their own layout.
	maskBindLayout  hal.BindGroupLayout
	maskLayoutOwned bool

	// MSAA and resolve textures for offscreen rendering (standalone mode).
	// When used via GPURenderSession, these are nil -- the session owns textures.
	msaaTex     hal.Texture
	msaaView    hal.TextureView
	resolveTex  hal.Texture
	resolveView hal.TextureView

	width, height uint32
}

// uniform. Layouts baked into shared pipelines are owned by GPUShared (or by
// the session for session-owned pipelines) and must not be destroyed here.
func (p *SDFRenderPipeline) SetClipBindLayout(layout hal.BindGroupLayout) {
	p.clipBindLayout = layout
}

func (p *SDFRenderPipeline) SetMaskBindLayout(layout hal.BindGroupLayout) {
	p.maskBindLayout = layout
	p.maskLayoutOwned = false
}

// EnsureSharedBase builds the base SDF pipelines once for the shared
// pipeline. Same contract as StencilRenderer.EnsureSharedBase: sessions call
// it instead of bespoke create, present pipelines are left untouched.
func (p *SDFRenderPipeline) EnsureSharedBase() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pipelineWithStencil != nil {
		return nil
	}
	return p.ensurePipelineWithStencil()
}

// NewSDFRenderPipeline creates a new SDF render pipeline with the given device
// and queue. The render pipeline and textures are not created until
// ensureReady is called with the desired dimensions.
func NewSDFRenderPipeline(device hal.Device, queue hal.Queue, sampleCount uint32) *SDFRenderPipeline {
	return &SDFRenderPipeline{
		device:      device,
		queue:       queue,
		sampleCount: sampleCount,
	}
}

// Destroy releases all GPU resources held by the pipeline. Safe to call
// multiple times or on a pipeline with no allocated resources.
func (p *SDFRenderPipeline) Destroy() {
	p.destroyPipeline()
	p.destroyTextures()
}

// RenderShapes renders detected SDF shapes via the fragment shader pipeline.
// Each shape is expanded into a bounding quad; the fragment shader evaluates
// the SDF per pixel for smooth anti-aliased coverage.
//
// The shapes are rendered in a single render pass. The MSAA color attachment
// resolves to a single-sample texture, which is then copied to a staging
// buffer for CPU readback into target.Data.
//
// Returns nil for empty shape slices. Returns ErrFallbackToCPU if the shader
// source is empty (build-time issue).
func (p *SDFRenderPipeline) RenderShapes(target render.GPURenderTarget, shapes []SDFRenderShape) error {
	if len(shapes) == 0 {
		return nil
	}

	w, h := uint32(target.Width), uint32(target.Height) //nolint:gosec // dimensions always fit uint32
	if err := p.ensureReady(w, h); err != nil {
		return err
	}

	// Build vertex data for all shapes (6 vertices per quad = 2 triangles).
	vertexData := buildSDFRenderVertices(shapes, w, h)
	vertexCount := uint32(len(shapes) * 6) //nolint:gosec // shape count fits uint32

	// Create per-frame GPU resources.
	vertBuf, err := p.createAndUploadBuffer("sdf_render_verts", vertexData,
		types.BufferUsageVertex|types.BufferUsageCopyDst)
	if err != nil {
		return fmt.Errorf("create vertex buffer: %w", err)
	}
	defer vertBuf.Destroy()

	uniformData := makeSDFRenderUniform(w, h, true)
	uniformBuf, err := p.createAndUploadBuffer("sdf_render_uniform", uniformData,
		types.BufferUsageUniform|types.BufferUsageCopyDst)
	if err != nil {
		return fmt.Errorf("create uniform buffer: %w", err)
	}
	defer uniformBuf.Destroy()

	bindGroup, err := p.device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "sdf_render_bind",
		Layout: p.uniformLayout,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, Buffer: uniformBuf, Offset: 0, Size: sdfRenderUniformSize},
		},
	})
	if err != nil {
		return fmt.Errorf("create bind group: %w", err)
	}
	defer bindGroup.Destroy()

	// Encode render pass + readback.
	return p.encodeAndReadback(w, h, vertBuf, vertexCount, bindGroup, target)
}

// ensureReady creates textures and the pipeline if needed.
func (p *SDFRenderPipeline) ensureReady(w, h uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.ensureTextures(w, h); err != nil {
		return fmt.Errorf("ensure textures: %w", err)
	}
	if p.pipeline == nil {
		if err := p.createPipeline(); err != nil {
			return fmt.Errorf("create pipeline: %w", err)
		}
	}
	return nil
}

// ensureTextures creates or recreates MSAA and resolve textures if the
// requested dimensions differ from the current size.
func (p *SDFRenderPipeline) ensureTextures(w, h uint32) error {
	if p.width == w && p.height == h && p.msaaTex != nil {
		return nil
	}
	p.destroyTextures()

	size := hal.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1}

	// MSAA color texture (BGRA8Unorm, sample count from GPUShared).
	msaaTex, err := p.device.CreateTexture(&hal.TextureDescriptor{
		Label:         "sdf_render_msaa",
		Size:          size,
		MipLevelCount: 1,
		SampleCount:   p.sampleCount,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatBGRA8Unorm,
		Usage:         types.TextureUsageRenderAttachment,
	})
	if err != nil {
		return fmt.Errorf("create MSAA texture: %w", err)
	}
	p.msaaTex = msaaTex

	msaaView, err := p.device.CreateTextureView(msaaTex, &hal.TextureViewDescriptor{
		Label:         "sdf_render_msaa_view",
		Format:        types.TextureFormatBGRA8Unorm,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		p.destroyTextures()
		return fmt.Errorf("create MSAA view: %w", err)
	}
	p.msaaView = msaaView

	// Single-sample resolve target (CopySrc for readback).
	resolveTex, err := p.device.CreateTexture(&hal.TextureDescriptor{
		Label:         "sdf_render_resolve",
		Size:          size,
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatBGRA8Unorm,
		Usage:         types.TextureUsageRenderAttachment | types.TextureUsageCopySrc,
	})
	if err != nil {
		p.destroyTextures()
		return fmt.Errorf("create resolve texture: %w", err)
	}
	p.resolveTex = resolveTex

	resolveView, err := p.device.CreateTextureView(resolveTex, &hal.TextureViewDescriptor{
		Label:         "sdf_render_resolve_view",
		Format:        types.TextureFormatBGRA8Unorm,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		p.destroyTextures()
		return fmt.Errorf("create resolve view: %w", err)
	}
	p.resolveView = resolveView

	p.width = w
	p.height = h
	return nil
}

// destroyTextures releases all texture resources and resets dimensions.
func (p *SDFRenderPipeline) destroyTextures() {
	if p.resolveView != nil {
		p.resolveView.Destroy()
		p.resolveView = nil
	}
	if p.resolveTex != nil {
		p.resolveTex.Destroy()
		p.resolveTex = nil
	}
	if p.msaaView != nil {
		p.msaaView.Destroy()
		p.msaaView = nil
	}
	if p.msaaTex != nil {
		p.msaaTex.Destroy()
		p.msaaTex = nil
	}
	p.width = 0
	p.height = 0
}

// createPipeline compiles the SDF render shader and creates the render
// pipeline with premultiplied alpha blending and MSAA.
func (p *SDFRenderPipeline) createPipeline() error {
	if sdfRenderShaderSource == "" {
		return fmt.Errorf("sdf_render shader source is empty")
	}

	shader, err := p.device.CreateShaderModule(&hal.ShaderModuleDescriptor{
		Label: "sdf_render_shader",
		WGSL:  sdfRenderShaderSource,
	})
	if err != nil {
		return fmt.Errorf("compile sdf_render shader: %w", err)
	}
	p.shader = shader

	uniformLayout, err := p.device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "sdf_render_uniform_layout",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
				Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: sdfRenderUniformSize},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("create uniform layout: %w", err)
	}
	p.uniformLayout = uniformLayout

	clipLayout := p.clipBindLayout
	hasClip := clipLayout != nil
	if clipLayout == nil {
		// Same rule as stencil: shared pipelines only ever use the shared
		// layouts pinned by GPUShared. Session-owned pipelines (effect
		// offscreens, standalone paths) keep the owned fallback below.
		if p.defaultClipBindLayout == nil {
			layout, err := createClipBindGroupLayout(p.device, "sdf_render_default_clip_layout")
			if err != nil {
				return fmt.Errorf("create default clip layout: %w", err)
			}
			p.defaultClipBindLayout = layout
		}
		clipLayout = p.defaultClipBindLayout
	}
	if p.maskBindLayout == nil {
		return fmt.Errorf("sdf pipeline: no mask layout (shared layouts must be pinned by GPUShared first)")
	}
	pipeLayout, err := p.device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "sdf_render_pipe_layout",
		BindGroupLayouts: []hal.BindGroupLayout{p.uniformLayout, clipLayout, p.maskBindLayout},
	})
	if err != nil {
		return fmt.Errorf("create pipeline layout: %w", err)
	}
	p.pipeLayoutHasClip = hasClip
	p.pipeLayout = pipeLayout

	premulBlend := types.BlendStatePremultiplied()
	pipeline, err := p.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "sdf_render_pipeline",
		Layout: p.pipeLayout,
		Vertex: hal.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    sdfRenderVertexLayout(),
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
		Primitive:   triangleListPrimitive(),
		Multisample: multisampleState(p.sampleCount),
	})
	if err != nil {
		return fmt.Errorf("create render pipeline: %w", err)
	}
	p.pipeline = pipeline

	return nil
}

// ensurePipelineWithStencil creates the session-compatible pipeline variant
// that includes a depth/stencil state. This pipeline is used when the SDF
// shapes are rendered in a unified render pass alongside stencil-then-cover
// paths. The SDF pipeline ignores the stencil buffer (Compare=Always, all
// ops=Keep, write mask=0).
//
// The base pipeline (shader, layout, bind group layout) is created first
// if it doesn't exist.
func (p *SDFRenderPipeline) ensurePipelineWithStencil() error { // Ensure base resources exist (shader, layouts).
	if p.shader == nil || p.uniformLayout == nil || p.pipeLayout == nil {
		if err := p.createPipeline(); err != nil {
			return err
		}
	}
	// Without this,
	// SetBindGroup(1, clipBG) crashes on AMD/NVIDIA (Intel tolerates it).
	if p.clipBindLayout != nil && !p.pipeLayoutHasClip {
		p.destroyPipeline()
		if err := p.createPipeline(); err != nil {
			return err
		}
	}
	if p.pipelineWithStencil != nil {
		return nil
	}

	premulBlend := types.BlendStatePremultiplied()
	pipeline, err := p.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "sdf_render_pipeline_with_stencil",
		Layout: p.pipeLayout,
		Vertex: hal.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    sdfRenderVertexLayout(),
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
		return fmt.Errorf("create SDF pipeline with stencil: %w", err)
	}
	p.pipelineWithStencil = pipeline
	return nil
}

// ensureDepthClipPipeline creates the depth-clipped pipeline variant if not
// already created. This variant uses DepthCompare=GreaterEqual for depth-based
// arbitrary path clipping (GPU-CLIP-003a).
func (p *SDFRenderPipeline) ensureDepthClipPipeline() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pipelineWithDepthClip != nil {
		return nil
	}
	// Base resources (shader, layout) must exist.
	if p.shader == nil || p.pipeLayout == nil {
		if err := p.ensurePipelineWithStencil(); err != nil {
			return err
		}
	}

	premulBlend := types.BlendStatePremultiplied()
	pipeline, err := p.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "sdf_render_pipeline_depth_clip",
		Layout: p.pipeLayout,
		Vertex: hal.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    sdfRenderVertexLayout(),
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
		return fmt.Errorf("create SDF pipeline with depth clip: %w", err)
	}
	p.pipelineWithDepthClip = pipeline
	return nil
}

// RecordDraws records SDF shape draws into an existing render pass.
// The render pass is owned by GPURenderSession. This method uses the
// pipelineWithStencil variant because the session's render pass includes
// a depth/stencil attachment.
//
// When depthClipped is true (GPU-CLIP-003a), the depth-clipped pipeline
// variant is used instead, which tests fragments against the depth clip buffer.
//
// ledger carries the pass bind-dedup state explicitly: the surface pass
// passes its own ledger (shared across its groups), offscreen record passes
// pass nil and always take the full bind path. The ledger used to live on
// this shared pipeline object — every window overwrote it with its own,
// which is a cross-window data race with wrong-skip fallout. Explicit
// threading keeps shared state immutable.
//
// The resources parameter holds pre-built vertex buffer, uniform buffer,
// and bind group for the current frame.
func (p *SDFRenderPipeline) RecordDraws(rp hal.RenderPassEncoder, resources *sdfFrameResources, clipBG hal.BindGroup, maskBG hal.BindGroup, ledger *PassBindLedger, depthClipped ...bool) {
	if resources == nil || resources.vertCount == 0 {
		return
	}
	if resources.bindGroup == nil {
		// Avoid Draw with a stale group-0 bind group from a prior pipeline
		// (e.g. stencil_fill_bind) — that panics wgpu validation on submit.
		return
	}
	useDepthClip := len(depthClipped) > 0 && depthClipped[0] && p.pipelineWithDepthClip != nil
	pipe := p.pipelineWithStencil
	if useDepthClip {
		pipe = p.pipelineWithDepthClip
	}
	if pipe == nil {
		return
	}
	// Ledger dedup: identical bound set → Draw only, else full bind path.
	if ledger != nil && ledger.skipBind(rp, pipe, resources.bindGroup, clipBG, maskBG, resources.vertBuf) {
		rp.Draw(resources.vertCount, 1, resources.firstVertex, 0)
		return
	}
	// Order: SetPipeline then bind groups. Always rebind group 0 after any
	// prior stencil/image/text draw in the same pass.
	rp.SetPipeline(pipe)
	rp.SetBindGroup(0, resources.bindGroup, nil)
	if clipBG != nil {
		rp.SetBindGroup(1, clipBG, nil)
	}
	if maskBG != nil {
		rp.SetBindGroup(2, maskBG, nil)
	}
	rp.SetVertexBuffer(0, resources.vertBuf, 0)
	rp.Draw(resources.vertCount, 1, resources.firstVertex, 0)
	if ledger != nil {
		ledger.noteBind(rp, pipe, resources.bindGroup, clipBG, maskBG, resources.vertBuf)
	}
}

// destroyPipeline releases all pipeline resources in reverse creation order.
func (p *SDFRenderPipeline) destroyPipeline() {
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
	if p.pipeline != nil {
		p.pipeline.Destroy()
		p.pipeline = nil
	}
	if p.pipeLayout != nil {
		p.pipeLayout.Destroy()
		p.pipeLayout = nil
		p.pipeLayoutHasClip = false
	}
	if p.defaultClipBindLayout != nil {
		p.defaultClipBindLayout.Destroy()
		p.defaultClipBindLayout = nil
	}
	if p.maskLayoutOwned && p.maskBindLayout != nil {
		p.maskBindLayout.Destroy()
	}
	p.maskBindLayout = nil
	p.maskLayoutOwned = false
	if p.uniformLayout != nil {
		p.uniformLayout.Destroy()
		p.uniformLayout = nil
	}
	if p.shader != nil {
		p.shader.Destroy()
		p.shader = nil
	}
}

// encodeAndReadback encodes the SDF render pass, copies the resolve texture
// to a staging buffer, submits, waits, and reads back pixels.
func (p *SDFRenderPipeline) encodeAndReadback(
	w, h uint32, vertBuf hal.Buffer, vertexCount uint32,
	bindGroup hal.BindGroup, target render.GPURenderTarget,
) error {
	encoder, err := p.device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{
		Label: "sdf_render_encoder",
	})
	if err != nil {
		return fmt.Errorf("create command encoder: %w", err)
	}
	// Render pass with MSAA resolve (only when sampleCount>1).
	colorAtt := hal.RenderPassColorAttachment{
		View:       p.resolveView,
		LoadOp:     types.LoadOpClear,
		StoreOp:    types.StoreOpStore,
		ClearValue: types.Color{R: 0, G: 0, B: 0, A: 0},
	}
	if p.sampleCount > 1 && p.msaaView != nil {
		colorAtt = hal.RenderPassColorAttachment{
			View:          p.msaaView,
			ResolveTarget: p.resolveView,
			LoadOp:        types.LoadOpClear,
			StoreOp:       types.StoreOpStore,
			ClearValue:    types.Color{R: 0, G: 0, B: 0, A: 0},
		}
	}
	rpDesc := &hal.RenderPassDescriptor{
		Label: "sdf_render_pass",
		ColorAttachments: []hal.RenderPassColorAttachment{
			colorAtt,
		},
	}
	rp, rpErr := encoder.BeginRenderPass(rpDesc)
	if rpErr != nil {
		return fmt.Errorf("begin render pass: %w", rpErr)
	}
	rp.SetPipeline(p.pipeline)
	rp.SetBindGroup(0, bindGroup, nil)
	rp.SetVertexBuffer(0, vertBuf, 0)
	rp.Draw(vertexCount, 1, 0, 0)
	_ = rp.End()

	// VK-LAYOUT-001: After MSAA resolve the texture is in
	// COLOR_ATTACHMENT_OPTIMAL layout. CopyTextureToBuffer requires
	// TRANSFER_SRC_OPTIMAL. Insert an explicit barrier to transition.
	// This is a no-op on Metal, GLES, software, and native backends.
	encoder.TransitionTextures([]hal.TextureBarrier{{
		Texture: p.resolveTex,
		Usage: hal.TextureUsageTransition{
			OldUsage: types.TextureUsageRenderAttachment,
			NewUsage: types.TextureUsageCopySrc,
		},
	}})

	// Copy resolve texture to staging buffer for readback.
	pixelBufSize := uint64(w) * uint64(h) * 4
	stagingBuf, err := p.device.CreateBuffer(&hal.BufferDescriptor{
		Label: "sdf_render_staging",
		Size:  pixelBufSize,
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		encoder.DiscardEncoding()
		return fmt.Errorf("create staging buffer: %w", err)
	}
	defer stagingBuf.Destroy()

	encoder.CopyTextureToBuffer(p.resolveTex, stagingBuf, []hal.BufferTextureCopy{{
		BufferLayout: hal.ImageDataLayout{Offset: 0, BytesPerRow: w * 4, RowsPerImage: h},
		TextureBase:  hal.ImageCopyTexture{Texture: p.resolveTex, MipLevel: 0},
		Size:         hal.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
	}})

	cmdBuf, err := encoder.Finish()
	if err != nil {
		return fmt.Errorf("end encoding: %w", err)
	}
	// Must Release after Submit: unreleased CBs pin device VRAM across recover.
	defer cmdBuf.Destroy()

	// Submit (auto-polls pending maps at tail).
	if _, err := p.queue.Submit(cmdBuf); err != nil {
		return fmt.Errorf("submit: %w", err)
	}

	// Map staging buffer; blocks until GPU completes via submission tracking.
	mapping, err := p.device.MapBuffer(stagingBuf, 0, pixelBufSize)
	if err != nil {
		return fmt.Errorf("map staging: %w", err)
	}
	readback := make([]byte, pixelBufSize)
	copy(readback, unsafe.Slice((*byte)(mapping.Ptr), pixelBufSize)) //nolint:gosec // hal.BufferMapping opaque pointer
	if err := p.device.UnmapBuffer(stagingBuf); err != nil {
		slogger().Warn("unmap failed", "err", err)
	}

	compositeBGRAOverRGBA(readback, target.Data, target.Width*target.Height)
	return nil
}

// createAndUploadBuffer creates a GPU buffer and uploads data.
func (p *SDFRenderPipeline) createAndUploadBuffer(label string, data []byte, usage types.BufferUsage) (hal.Buffer, error) {
	buf, err := p.device.CreateBuffer(&hal.BufferDescriptor{
		Label: label,
		Size:  uint64(len(data)),
		Usage: usage,
	})
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", label, err)
	}
	if err := p.queue.WriteBuffer(buf, 0, data); err != nil {
		buf.Destroy()
		return nil, fmt.Errorf("write %s: %w", label, err)
	}
	return buf, nil
}

// Size returns the current texture dimensions.
func (p *SDFRenderPipeline) Size() (uint32, uint32) {
	return p.width, p.height
}

// SDFRenderShape holds the parameters for a single shape to be rendered
// via the SDF render pipeline.
type SDFRenderShape struct {
	Kind       uint32  // 0 = circle/ellipse, 1 = rrect, 2 = annular arc sector
	CenterX    float32 // Shape center X in pixel coordinates.
	CenterY    float32 // Shape center Y in pixel coordinates.
	Param1     float32 // radius_x (circle/ellipse/arc) or half_width (rrect).
	Param2     float32 // radius_y (circle/ellipse/arc) or half_height (rrect).
	Param3     float32 // corner_radius (rrect only, 0 for circle/arc).
	HalfStroke float32 // Half stroke width (0 for filled shapes).
	IsStroked  float32 // 1.0 for stroked, 0.0 for filled.
	ColorR     float32 // Premultiplied red.
	ColorG     float32 // Premultiplied green.
	ColorB     float32 // Premultiplied blue.
	ColorA     float32 // Premultiplied alpha.
	// Angle0/Angle1 delimit the annular arc sector (kind 2): parameter-space
	// start/end angles in radians; Angle0 < Angle1 with sweep < 2π.
	Angle0 float32
	Angle1 float32
}

// DetectedShapeToRenderShape converts a render.DetectedShape and paint into an
// SDFRenderShape ready for the render pipeline.
func DetectedShapeToRenderShape(shape render.DetectedShape, paint *render.Paint, stroked bool) (SDFRenderShape, bool) {
	var rs SDFRenderShape
	switch shape.Kind {
	case render.ShapeCircle, render.ShapeEllipse:
		rs.Kind = 0
		rs.Param1 = float32(shape.RadiusX)
		rs.Param2 = float32(shape.RadiusY)
	case render.ShapeRect, render.ShapeRRect:
		rs.Kind = 1
		rs.Param1 = float32(shape.Width / 2)
		rs.Param2 = float32(shape.Height / 2)
		rs.Param3 = float32(shape.CornerRadius)
	case render.ShapeArc:
		// Annular arc sector: only meaningful for strokes (a filled open arc
		// implicitly closes across its chord, which kind 2 cannot express).
		if !stroked {
			return rs, false
		}
		rs.Kind = 2
		rs.Param1 = float32(shape.RadiusX)
		rs.Param2 = float32(shape.RadiusY)
		rs.Angle0 = float32(shape.Angle0)
		rs.Angle1 = float32(shape.Angle1)
	default:
		return rs, false
	}

	rs.CenterX = float32(shape.CenterX)
	rs.CenterY = float32(shape.CenterY)

	if stroked {
		rs.HalfStroke = float32(effectiveStrokeWidth(paint) / 2)
		rs.IsStroked = 1.0
	}

	color := getColorFromPaint(paint)
	// Premultiply color for GPU blending.
	rs.ColorR = float32(color.R * color.A)
	rs.ColorG = float32(color.G * color.A)
	rs.ColorB = float32(color.B * color.A)
	rs.ColorA = float32(color.A)

	return rs, true
}

// sdfRenderVertexLayout returns the vertex buffer layout for the SDF render pipeline.
func sdfRenderVertexLayout() []types.VertexBufferLayout {
	return []types.VertexBufferLayout{
		{
			ArrayStride: sdfRenderVertexStride,
			StepMode:    types.VertexStepModeVertex,
			Attributes: []types.VertexAttribute{
				{Format: types.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},  // position
				{Format: types.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1},  // local
				{Format: types.VertexFormatFloat32, Offset: 16, ShaderLocation: 2},   // shape_kind
				{Format: types.VertexFormatFloat32, Offset: 20, ShaderLocation: 3},   // param1
				{Format: types.VertexFormatFloat32, Offset: 24, ShaderLocation: 4},   // param2
				{Format: types.VertexFormatFloat32, Offset: 28, ShaderLocation: 5},   // param3
				{Format: types.VertexFormatFloat32, Offset: 32, ShaderLocation: 6},   // half_stroke
				{Format: types.VertexFormatFloat32, Offset: 36, ShaderLocation: 7},   // is_stroked
				{Format: types.VertexFormatFloat32x4, Offset: 40, ShaderLocation: 8}, // color
				{Format: types.VertexFormatFloat32, Offset: 56, ShaderLocation: 9},   // angle0 (kind 2)
				{Format: types.VertexFormatFloat32, Offset: 60, ShaderLocation: 10},  // angle1 (kind 2)
			},
		},
	}
}

// buildSDFRenderVertices generates vertex data for all shapes. Each shape
// produces 6 vertices (2 triangles forming a screen-aligned quad).
// The quad covers the shape's bounding box plus an AA margin.
func buildSDFRenderVertices(shapes []SDFRenderShape, w, h uint32) []byte {
	_, data := buildSDFRenderVerticesReuse(shapes, w, h, nil)
	return data
}

// buildSDFRenderVerticesReuse generates vertex data for all shapes into the
// provided staging buffer, growing it if necessary. Returns the (possibly
// reallocated) staging buffer and the slice of valid vertex data.
func buildSDFRenderVerticesReuse(shapes []SDFRenderShape, _, _ uint32, staging []byte) ([]byte, []byte) {
	needed := len(shapes) * 6 * sdfRenderVertexStride
	if cap(staging) < needed {
		staging = make([]byte, needed)
	} else {
		staging = staging[:needed]
	}
	buf := staging
	offset := 0

	for i := range shapes {
		s := &shapes[i]

		// Compute bounding box half-extents.
		// For circles/ellipses, Param1/2 are radii.
		// For rrects, Param1/2 are half dimensions.
		// Both produce the same bounding box calculation.
		halfW := s.Param1
		halfH := s.Param2
		// Expand for stroke + AA margin.
		halfW += s.HalfStroke + sdfRenderAAMargin
		halfH += s.HalfStroke + sdfRenderAAMargin

		// Quad corners in pixel coordinates and local (shape-relative) coordinates.
		// Triangle 1: TL, TR, BL
		// Triangle 2: TR, BR, BL
		type corner struct {
			px, py float32 // pixel position
			lx, ly float32 // local offset from center
		}
		tl := corner{s.CenterX - halfW, s.CenterY - halfH, -halfW, -halfH}
		tr := corner{s.CenterX + halfW, s.CenterY - halfH, halfW, -halfH}
		bl := corner{s.CenterX - halfW, s.CenterY + halfH, -halfW, halfH}
		br := corner{s.CenterX + halfW, s.CenterY + halfH, halfW, halfH}

		corners := [6]corner{tl, tr, bl, tr, br, bl}

		for _, c := range corners {
			writeSDFRenderVertex(buf[offset:], c.px, c.py, c.lx, c.ly, s)
			offset += sdfRenderVertexStride
		}
	}

	return staging, buf
}

// writeSDFRenderVertex writes a single vertex into the buffer at the current position.
func writeSDFRenderVertex(buf []byte, px, py, lx, ly float32, s *SDFRenderShape) {
	binary.LittleEndian.PutUint32(buf[0:4], math.Float32bits(px))
	binary.LittleEndian.PutUint32(buf[4:8], math.Float32bits(py))
	binary.LittleEndian.PutUint32(buf[8:12], math.Float32bits(lx))
	binary.LittleEndian.PutUint32(buf[12:16], math.Float32bits(ly))
	binary.LittleEndian.PutUint32(buf[16:20], math.Float32bits(float32(s.Kind)))
	binary.LittleEndian.PutUint32(buf[20:24], math.Float32bits(s.Param1))
	binary.LittleEndian.PutUint32(buf[24:28], math.Float32bits(s.Param2))
	binary.LittleEndian.PutUint32(buf[28:32], math.Float32bits(s.Param3))
	binary.LittleEndian.PutUint32(buf[32:36], math.Float32bits(s.HalfStroke))
	binary.LittleEndian.PutUint32(buf[36:40], math.Float32bits(s.IsStroked))
	binary.LittleEndian.PutUint32(buf[40:44], math.Float32bits(s.ColorR))
	binary.LittleEndian.PutUint32(buf[44:48], math.Float32bits(s.ColorG))
	binary.LittleEndian.PutUint32(buf[48:52], math.Float32bits(s.ColorB))
	binary.LittleEndian.PutUint32(buf[52:56], math.Float32bits(s.ColorA))
	binary.LittleEndian.PutUint32(buf[56:60], math.Float32bits(s.Angle0))
	binary.LittleEndian.PutUint32(buf[60:64], math.Float32bits(s.Angle1))
}

// makeSDFRenderUniform creates the 16-byte uniform buffer.
// Layout: viewport (vec2<f32>) + anti_alias (u32) + padding (u32).
func makeSDFRenderUniform(w, h uint32, antiAlias bool) []byte {
	return makeSDFRenderUniformInto(nil, w, h, antiAlias)
}

// makeSDFRenderUniformInto reuses buf when possible.
func makeSDFRenderUniformInto(buf []byte, w, h uint32, antiAlias bool) []byte {
	if cap(buf) < sdfRenderUniformSize {
		buf = make([]byte, sdfRenderUniformSize)
	} else {
		buf = buf[:sdfRenderUniformSize]
	}
	binary.LittleEndian.PutUint32(buf[0:4], math.Float32bits(float32(w)))
	binary.LittleEndian.PutUint32(buf[4:8], math.Float32bits(float32(h)))
	aa := uint32(1)
	if !antiAlias {
		aa = 0
	}
	binary.LittleEndian.PutUint32(buf[8:12], aa)
	// Pad remainder of fixed uniform size without a full-buffer clear loop.
	for i := 12; i < len(buf); i++ {
		buf[i] = 0
	}
	// Intentionally no slogger here: Into is a hot path.
	return buf
}
