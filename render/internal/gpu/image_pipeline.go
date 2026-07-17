//go:build !nogpu

package gpu

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

//go:embed shaders/textured_quad.wgsl
var texturedQuadShaderSource string

// imageVertexStride is the byte stride per vertex in the textured quad pipeline.
// Layout per vertex:
//
//	position  (vec2<f32>) =  8 bytes  (location 0)
//	tex_coord (vec2<f32>) =  8 bytes  (location 1)
//
// Total = 16 bytes per vertex.
const imageVertexStride = 16

// imageUniformSize is the byte size of the image uniform buffer.
// Layout:
//
//	transform (mat4x4<f32>) = 64 bytes
//	opacity   (f32)         =  4 bytes
//	_pad      (vec3<f32>)   = 12 bytes
//
// Total = 80 bytes.
const imageUniformSize = 80

// imageUniformSlotStride is the bytes reserved per draw uniform in a shared
// slab buffer (opt29). Must be a multiple of minUniformBufferOffsetAlignment
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

	Opacity        float32
	ViewportWidth  uint32
	ViewportHeight uint32

	// Source UV rectangle (normalized 0..1 within the image).
	// For full-image draws: u0=0, v0=0, u1=1, v1=1.
	U0, V0, U1, V1 float32

	// Filter selects texture sampling (I.03). false = Linear (default), true = Nearest.
	Nearest bool

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
	device      *webgpu.Device
	queue       *webgpu.Queue
	sampleCount uint32 // MSAA sample count (4 or 1), from GPUShared

	// GPU objects for the render pipeline.
	shader        *webgpu.ShaderModule
	uniformLayout *webgpu.BindGroupLayout
	pipeLayout    *webgpu.PipelineLayout

	// Session-compatible pipeline variant with depth/stencil state.
	// Used when images participate in a unified render pass that includes
	// a stencil attachment. Stencil test is Always/Keep (images do not
	// interact with stencil).
	pipelineWithStencil *webgpu.RenderPipeline

	// Depth-clipped pipeline variant (GPU-CLIP-003a). Same as pipelineWithStencil
	// but with DepthCompare=GreaterEqual to test against the depth clip buffer.
	pipelineWithDepthClip *webgpu.RenderPipeline

	// Non-MSAA blit pipeline for compositor fast path (ADR-016).
	// SampleCount=1, no depth/stencil — used when the frame contains
	// only textured quads (base layer + overlays) with no vector shapes.
	blitPipeline *webgpu.RenderPipeline
	blitLayout   *webgpu.PipelineLayout // single bind group, no clip

	// Default sampler for image textures (bilinear filtering, clamp-to-edge).
	sampler *webgpu.Sampler
	// Nearest-neighbor sampler (I.03).
	nearestSampler *webgpu.Sampler

	// clipBindLayout is the shared @group(1) bind group layout for RRect clip.
	// Set by the session before ensurePipelineWithStencil.
	clipBindLayout    *webgpu.BindGroupLayout
	pipeLayoutHasClip bool
}

// NewTexturedQuadPipeline creates a new textured quad pipeline.
func NewTexturedQuadPipeline(device *webgpu.Device, queue *webgpu.Queue, sampleCount uint32) *TexturedQuadPipeline {
	return &TexturedQuadPipeline{
		device:      device,
		queue:       queue,
		sampleCount: sampleCount,
	}
}

// SetClipBindLayout sets the bind group layout for the @group(1) RRect clip
// uniform. Must be called before ensurePipelineWithStencil.
func (p *TexturedQuadPipeline) SetClipBindLayout(layout *webgpu.BindGroupLayout) {
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
	pipeline, err := p.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "textured_quad_pipeline_with_stencil",
		Layout: p.pipeLayout,
		Vertex: webgpu.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    imageVertexLayout(),
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
	pipeline, err := p.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "textured_quad_pipeline_depth_clip",
		Layout: p.pipeLayout,
		Vertex: webgpu.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    imageVertexLayout(),
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
		return fmt.Errorf("create textured quad pipeline with depth clip: %w", err)
	}
	p.pipelineWithDepthClip = pipeline
	return nil
}

// ensureBlitPipeline creates the non-MSAA pipeline variant for compositor
// fast path (ADR-016). SampleCount=1, no depth/stencil attachment.
func (p *TexturedQuadPipeline) ensureBlitPipeline() error {
	if err := p.ensureBase(); err != nil {
		return err
	}
	if p.blitPipeline != nil {
		return nil
	}

	// Blit pipeline uses a single-bind-group layout (no clip group).
	// The regular pipeLayout may have 2 bind groups (texture + clip),
	// and RecordBlitDraws only sets group 0 — leaving group 1 undefined
	// would cause GPU validation errors.
	if p.blitLayout == nil {
		layout, err := p.device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
			Label:            "textured_quad_blit_layout",
			BindGroupLayouts: []*webgpu.BindGroupLayout{p.uniformLayout},
		})
		if err != nil {
			return fmt.Errorf("create blit pipeline layout: %w", err)
		}
		p.blitLayout = layout
	}

	premulBlend := types.BlendStatePremultiplied()
	pipeline, err := p.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "textured_quad_blit_pipeline",
		Layout: p.blitLayout,
		Vertex: webgpu.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    imageVertexLayout(),
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
func (p *TexturedQuadPipeline) RecordBlitDraws(rp *webgpu.RenderPassEncoder, res *imageFrameResources) {
	if p.blitPipeline == nil || res == nil {
		return
	}
	rp.SetPipeline(p.blitPipeline)
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
	shader, err := p.device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "textured_quad_shader",
		WGSL:  texturedQuadShaderSource,
	})
	if err != nil {
		return fmt.Errorf("compile textured_quad shader: %w", err)
	}
	p.shader = shader

	// Samplers: bilinear default + nearest (I.03), clamp-to-edge.
	sampler, err := p.device.CreateSampler(&webgpu.SamplerDescriptor{
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
	nearest, err := p.device.CreateSampler(&webgpu.SamplerDescriptor{
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

	// Bind group layout: uniform + texture + sampler.
	uniformLayout, err := p.device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
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
	bgLayouts := []*webgpu.BindGroupLayout{p.uniformLayout}
	hasClip := p.clipBindLayout != nil
	if hasClip {
		bgLayouts = append(bgLayouts, p.clipBindLayout)
	}
	pipeLayout, err := p.device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
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
func (p *TexturedQuadPipeline) RecordDraws(rp *webgpu.RenderPassEncoder, res *imageFrameResources, clipBG *webgpu.BindGroup, depthClipped ...bool) {
	useDepthClip := len(depthClipped) > 0 && depthClipped[0] && p.pipelineWithDepthClip != nil
	if useDepthClip {
		rp.SetPipeline(p.pipelineWithDepthClip)
	} else {
		rp.SetPipeline(p.pipelineWithStencil)
	}
	if clipBG != nil {
		rp.SetBindGroup(1, clipBG, nil)
	}
	rp.SetVertexBuffer(0, res.vertBuf, 0)
	for _, dc := range res.drawCalls {
		rp.SetBindGroup(0, dc.bindGroup, nil)
		// S4.1: vertexCount may cover multiple quads sharing one bind group.
		rp.Draw(imageDrawVertexCount(dc), 1, dc.firstVertex, 0)
	}
}

// destroyPipeline releases all pipeline resources.
func (p *TexturedQuadPipeline) destroyPipeline() {
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
	if p.blitPipeline != nil {
		p.blitPipeline.Release()
		p.blitPipeline = nil
	}
	if p.blitLayout != nil {
		p.blitLayout.Release()
		p.blitLayout = nil
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
	if p.sampler != nil {
		p.sampler.Release()
		p.sampler = nil
	}
	if p.nearestSampler != nil {
		p.nearestSampler.Release()
		p.nearestSampler = nil
	}
	if p.shader != nil {
		p.shader.Release()
		p.shader = nil
	}
}

// imageFrameResources holds pre-built GPU resources for image rendering in
// a single frame. Created by the render session's buildImageResources.
type imageFrameResources struct {
	vertBuf   *webgpu.Buffer
	drawCalls []imageDrawCall
}

// imageDrawCall holds per-image (or multi-quad batch) draw parameters within a frame.
// S4.1: consecutive quads with identical texture/opacity/filter share one bind
// group and one Draw(vertexCount) spanning vertexCount/6 quads.
type imageDrawCall struct {
	bindGroup   *webgpu.BindGroup
	firstVertex uint32
	vertexCount uint32 // 0 means 6 (single quad, backward compatible)
}

// imageDrawVertexCount returns the vertex count for a draw call.
func imageDrawVertexCount(dc imageDrawCall) uint32 {
	if dc.vertexCount == 0 {
		return 6
	}
	return dc.vertexCount
}

// canMergeImageDraw reports whether two image commands may share one bind group
// and multi-quad draw (same GPU texture key + sampling + opacity + viewport).
func canMergeImageDraw(a, b *ImageDrawCommand) bool {
	if a == nil || b == nil {
		return false
	}
	if a.GenerationID == 0 || a.GenerationID != b.GenerationID {
		return false
	}
	if a.Nearest != b.Nearest || a.Opacity != b.Opacity {
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
// one bind group + multi-quad Draw (S6.3). Same texture view pointer, opacity, and
// viewport; never merge across scissor seals (caller enforces batchSeal).
func canMergeGPUTextureDraw(a, b *GPUTextureDrawCommand) bool {
	if a == nil || b == nil {
		return false
	}
	if a.View.IsNil() || b.View.IsNil() {
		return false
	}
	if a.View.Pointer() != b.View.Pointer() {
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
				{Format: types.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0}, // position
				{Format: types.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1}, // tex_coord
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

	// Triangle 1: TL, TR, BL
	// Triangle 2: TR, BR, BL
	verts := [6][4]float32{
		{x0, y0, u0, v0}, // TL
		{x1, y1, u1, v0}, // TR
		{x3, y3, u0, v1}, // BL
		{x1, y1, u1, v0}, // TR
		{x2, y2, u1, v1}, // BR
		{x3, y3, u0, v1}, // BL
	}

	offset := 0
	for _, v := range verts {
		binary.LittleEndian.PutUint32(dst[offset:], math.Float32bits(v[0]))
		binary.LittleEndian.PutUint32(dst[offset+4:], math.Float32bits(v[1]))
		binary.LittleEndian.PutUint32(dst[offset+8:], math.Float32bits(v[2]))
		binary.LittleEndian.PutUint32(dst[offset+12:], math.Float32bits(v[3]))
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
func (p *TexturedQuadPipeline) SamplerFor(nearest bool) *webgpu.Sampler {
	if nearest && p.nearestSampler != nil {
		return p.nearestSampler
	}
	return p.sampler
}
