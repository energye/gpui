//go:build !nogpu

package gpu

import (
	_ "embed"
	"fmt"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// Embedded WGSL shader sources for stencil-then-cover rendering.

//go:embed shaders/stencil_fill.wgsl
var stencilFillShaderSource string

//go:embed shaders/cover.wgsl
var coverShaderSource string

//go:embed shaders/cover_aa.wgsl
var coverAAShaderSource string

// stencilFillUniformSize is the byte size of the stencil fill uniform buffer.
// Layout: viewport (vec2<f32>) + padding (vec2<f32>) = 16 bytes.
const stencilFillUniformSize = 16

// coverUniformSize is the byte size of the cover pass uniform buffer.
// Layout: viewport (vec2<f32>) + padding (vec2<f32>) + color (vec4<f32>) = 32 bytes.
const coverUniformSize = 32

// vertexStride is the byte stride per vertex: 2 x float32 (x, y) = 8 bytes.
const vertexStride = 8

// aaVertexStride is the byte stride per vertex of the analytic-AA cover/band
// meshes: position (vec2<f32>) + signed edge distance (f32) = 12 bytes.
const aaVertexStride = 12

// aaVertexBufferLayout strips the fan/band AA meshes: pos at location 0,
// signed edge distance at location 1 (see shaders/cover_aa.wgsl).
func aaVertexBufferLayout() []types.VertexBufferLayout {
	return []types.VertexBufferLayout{
		{
			ArrayStride: aaVertexStride,
			StepMode:    types.VertexStepModeVertex,
			Attributes: []types.VertexAttribute{
				{Format: types.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},
				{Format: types.VertexFormatFloat32, Offset: 8, ShaderLocation: 1},
			},
		},
	}
}

// createPipelines compiles shaders and creates the stencil fill and cover
// render pipelines. Both pipelines share the same bind group layout (one
// uniform buffer at group(0) binding(0)) and vertex layout (float32x2 at
// location(0)).
//
// Two stencil fill pipeline variants are created:
//   - NonZero: front=IncrementWrap / back=DecrementWrap (winding number).
//   - EvenOdd: front=IncrementWrap / back=IncrementWrap with WriteMask=0x01
//     (parity via bit-0 toggle — equivalent to Invert but avoids the AMD D3D12
//     StencilOperationInvert driver bug on Radeon 890M and similar GPUs, #374).
//
// The cover pipeline reads the stencil buffer with NotEqual(0) and resets
// stencil values to zero via PassOp=Zero after writing the fill color.
// It is shared by both fill rules.
func (sr *StencilRenderer) createPipelines() error { //nolint:funlen // GPU pipeline descriptors are inherently verbose
	// Compile shaders.
	stencilShader, err := sr.device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "stencil_fill_shader",
		WGSL:  stencilFillShaderSource,
	})
	if err != nil {
		return fmt.Errorf("compile stencil fill shader: %w", err)
	}
	sr.stencilFillShader = stencilShader

	coverShader, err := sr.device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "cover_shader",
		WGSL:  coverShaderSource,
	})
	if err != nil {
		return fmt.Errorf("compile cover shader: %w", err)
	}
	sr.coverShader = coverShader

	// Create bind group layout shared by both pipelines.
	// One uniform buffer at group(0) binding(0), visible to vertex + fragment stages.
	// MinBindingSize left as 0 (None): fill is 16 bytes, cover is 32; a non-zero
	// min that matches only one of them forces late-size mismatches. Pipeline
	// switches after stencil re-bind group 0 explicitly in each Record* method.
	uniformLayout, err := sr.device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "stencil_cover_uniform_layout",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
				Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("create uniform bind group layout: %w", err)
	}
	sr.uniformLayout = uniformLayout

	// Create pipeline layouts.
	stencilPipeLayout, err := sr.device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label:            "stencil_fill_pipe_layout",
		BindGroupLayouts: []*webgpu.BindGroupLayout{sr.uniformLayout},
	})
	if err != nil {
		return fmt.Errorf("create stencil pipeline layout: %w", err)
	}
	sr.stencilPipeLayout = stencilPipeLayout

	clipLayout := sr.clipBindLayout
	hasClip := clipLayout != nil
	if clipLayout == nil {
		if sr.defaultClipBindLayout == nil {
			layout, err := createClipBindGroupLayout(sr.device, "stencil_default_clip_layout")
			if err != nil {
				return fmt.Errorf("create default clip layout: %w", err)
			}
			sr.defaultClipBindLayout = layout
		}
		clipLayout = sr.defaultClipBindLayout
	}
	if sr.maskBindLayout == nil {
		layout, err := createMaskBindGroupLayout(sr.device, "stencil_cover_mask_layout")
		if err != nil {
			return fmt.Errorf("create cover mask layout: %w", err)
		}
		sr.maskBindLayout = layout
		sr.maskLayoutOwned = true
	}
	coverPipeLayout, err := sr.device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label:            "cover_pipe_layout",
		BindGroupLayouts: []*webgpu.BindGroupLayout{sr.uniformLayout, clipLayout, sr.maskBindLayout},
	})
	if err != nil {
		return fmt.Errorf("create cover pipeline layout: %w", err)
	}
	sr.coverPipeLayout = coverPipeLayout
	sr.coverPipeLayoutHasClip = hasClip
	sr.coverPipeMaskLayout = sr.maskBindLayout

	// Shared vertex buffer layout: float32x2 position at location(0).
	vertexBufferLayout := []types.VertexBufferLayout{
		{
			ArrayStride: vertexStride,
			StepMode:    types.VertexStepModeVertex,
			Attributes: []types.VertexAttribute{
				{
					Format:         types.VertexFormatFloat32x2,
					Offset:         0,
					ShaderLocation: 0,
				},
			},
		},
	}

	// Shared multisample state: MSAA sample count from GPUShared (4x or 1x).
	multisample := multisampleState(sr.sampleCount)

	// Shared primitive state: triangle list, no culling.
	primitive := types.PrimitiveState{
		Topology: types.PrimitiveTopologyTriangleList,
		CullMode: types.CullModeNone,
	}

	// --- Stencil Fill Pipeline ---
	//
	// Non-zero fill rule: front faces increment, back faces decrement.
	// Color writes are suppressed (WriteMask=None) since this pass only
	// updates the stencil buffer. A dummy fragment shader is included for
	// backend compatibility.
	nonZeroStencilPipeline, err := sr.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "stencil_fill_pipeline",
		Layout: sr.stencilPipeLayout,
		Vertex: webgpu.VertexState{
			Module:     sr.stencilFillShader,
			EntryPoint: shaderEntryVS,
			Buffers:    vertexBufferLayout,
		},
		Fragment: &webgpu.FragmentState{
			Module:     sr.stencilFillShader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{
					Format:    types.TextureFormatBGRA8Unorm,
					WriteMask: types.ColorWriteMaskNone,
				},
			},
		},
		DepthStencil: &webgpu.DepthStencilState{
			Format:            types.TextureFormatDepth24PlusStencil8,
			DepthWriteEnabled: false,
			DepthCompare:      types.CompareFunctionAlways,
			StencilFront: webgpu.StencilFaceState{
				Compare:     types.CompareFunctionAlways,
				FailOp:      webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep,
				PassOp:      webgpu.StencilOperationIncrementWrap,
			},
			StencilBack: webgpu.StencilFaceState{
				Compare:     types.CompareFunctionAlways,
				FailOp:      webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep,
				PassOp:      webgpu.StencilOperationDecrementWrap,
			},
			StencilReadMask:  0xFF,
			StencilWriteMask: 0xFF,
		},
		Multisample: multisample,
		Primitive:   primitive,
	})
	if err != nil {
		return fmt.Errorf("create stencil fill pipeline: %w", err)
	}
	sr.nonZeroStencilPipeline = nonZeroStencilPipeline

	// --- Even-Odd Stencil Fill Pipeline ---
	//
	// Even-odd fill rule: both front and back faces toggle the stencil parity bit.
	// WriteMask=0x01 restricts writes to bit 0 only. IncrementWrap on a 1-bit field
	// is equivalent to XOR/Invert: 0→1→0→... This avoids StencilOperationInvert,
	// which has a driver bug on AMD Radeon 890M D3D12 (and similar GPUs) that
	// causes the stencil to not be toggled correctly, making strokes render as solid
	// fills (#374). Pixels inside an odd number of crossings have stencil=1 (inside),
	// pixels inside an even number have stencil=0 (outside). Same shader and layout
	// as the non-zero variant.
	evenOddStencilPipeline, err := sr.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "stencil_fill_even_odd_pipeline",
		Layout: sr.stencilPipeLayout,
		Vertex: webgpu.VertexState{
			Module:     sr.stencilFillShader,
			EntryPoint: shaderEntryVS,
			Buffers:    vertexBufferLayout,
		},
		Fragment: &webgpu.FragmentState{
			Module:     sr.stencilFillShader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{
					Format:    types.TextureFormatBGRA8Unorm,
					WriteMask: types.ColorWriteMaskNone,
				},
			},
		},
		DepthStencil: &webgpu.DepthStencilState{
			Format:            types.TextureFormatDepth24PlusStencil8,
			DepthWriteEnabled: false,
			DepthCompare:      types.CompareFunctionAlways,
			StencilFront: webgpu.StencilFaceState{
				Compare:     types.CompareFunctionAlways,
				FailOp:      webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep,
				PassOp:      webgpu.StencilOperationIncrementWrap,
			},
			StencilBack: webgpu.StencilFaceState{
				Compare:     types.CompareFunctionAlways,
				FailOp:      webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep,
				PassOp:      webgpu.StencilOperationIncrementWrap,
			},
			StencilReadMask:  0xFF,
			StencilWriteMask: 0x01,
		},
		Multisample: multisample,
		Primitive:   primitive,
	})
	if err != nil {
		return fmt.Errorf("create even-odd stencil fill pipeline: %w", err)
	}
	sr.evenOddStencilPipeline = evenOddStencilPipeline

	// --- Cover Pipeline ---
	//
	// Reads stencil buffer: only pixels with stencil != 0 pass the test.
	// PassOp=Zero resets stencil to 0 after coloring, clearing it for the
	// next path. Premultiplied alpha blending composites the fill color.
	premulBlend := types.BlendStatePremultiplied()
	nonZeroCoverPipeline, err := sr.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "cover_pipeline",
		Layout: sr.coverPipeLayout,
		Vertex: webgpu.VertexState{
			Module:     sr.coverShader,
			EntryPoint: shaderEntryVS,
			Buffers:    vertexBufferLayout,
		},
		Fragment: &webgpu.FragmentState{
			Module:     sr.coverShader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{
					Format:    types.TextureFormatBGRA8Unorm,
					Blend:     &premulBlend,
					WriteMask: types.ColorWriteMaskAll,
				},
			},
		},
		DepthStencil: &webgpu.DepthStencilState{
			Format:            types.TextureFormatDepth24PlusStencil8,
			DepthWriteEnabled: false,
			DepthCompare:      types.CompareFunctionAlways,
			StencilFront: webgpu.StencilFaceState{
				Compare:     types.CompareFunctionNotEqual,
				FailOp:      webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep,
				PassOp:      webgpu.StencilOperationZero,
			},
			StencilBack: webgpu.StencilFaceState{
				Compare:     types.CompareFunctionNotEqual,
				FailOp:      webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep,
				PassOp:      webgpu.StencilOperationZero,
			},
			StencilReadMask:  0xFF,
			StencilWriteMask: 0xFF,
		},
		Multisample: multisample,
		Primitive:   primitive,
	})
	if err != nil {
		return fmt.Errorf("create cover pipeline: %w", err)
	}
	sr.nonZeroCoverPipeline = nonZeroCoverPipeline

	if err := sr.createAABandPipelines(); err != nil {
		return err
	}

	return nil
}

// createAABandPipelines creates the sampleCount==1 analytic-AA fringe
// pipelines (Skia-style "fringe coverage"; see shaders/cover_aa.wgsl):
//
//   - aaBandPipeline (exterior half): SrcOver + stencil Equal(0), read-only —
//     drawn right after the stencil fill (before the binary cover) so pixels
//     just outside the boundary get partial coverage over the background.
//   - aaInnerBandPipeline (interior half): SrcOver + stencil NotEqual(0) with
//     PassOp=Zero — drawn BEFORE the binary cover so just-inside pixels blend
//     their true partial coverage over the still-intact background, then clear
//     their stencil so the cover skips them. (The previous post-cover Replace
//     scheme wrote premultiplied partial color with the background term lost,
//     producing dark speckles along curves/diagonals.)
//
// Non-SrcOver blend modes keep the binary cover (as do pattern/textured and
// depth-clip covers).
func (sr *StencilRenderer) createAABandPipelines() error {
	if sr.coverPipeLayout == nil || sr.coverShader == nil {
		return nil // base pipelines not created yet — retried via createPipelines
	}
	if sr.aaCoverShader == nil {
		sh, err := sr.device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
			Label: "cover_aa_shader",
			WGSL:  coverAAShaderSource,
		})
		if err != nil {
			return fmt.Errorf("compile cover_aa shader: %w", err)
		}
		sr.aaCoverShader = sh
	}
	multisample := multisampleState(sr.sampleCount)
	primitive := types.PrimitiveState{
		Topology: types.PrimitiveTopologyTriangleList,
		CullMode: types.CullModeNone,
	}
	premulBlend := types.BlendStatePremultiplied()
	layout := aaVertexBufferLayout()

	mk := func(label string, blend *types.BlendState, compare types.CompareFunction, passOp webgpu.StencilOperation, writeMask uint32) (*webgpu.RenderPipeline, error) {
		return sr.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
			Label:  label,
			Layout: sr.coverPipeLayout,
			Vertex: webgpu.VertexState{
				Module:     sr.aaCoverShader,
				EntryPoint: shaderEntryVS,
				Buffers:    layout,
			},
			Fragment: &webgpu.FragmentState{
				Module:     sr.aaCoverShader,
				EntryPoint: shaderEntryFS,
				Targets: []types.ColorTargetState{
					{
						Format:    types.TextureFormatBGRA8Unorm,
						Blend:     blend,
						WriteMask: types.ColorWriteMaskAll,
					},
				},
			},
			DepthStencil: &webgpu.DepthStencilState{
				Format:            types.TextureFormatDepth24PlusStencil8,
				DepthWriteEnabled: false,
				DepthCompare:      types.CompareFunctionAlways,
				StencilFront: webgpu.StencilFaceState{
					Compare:     compare,
					FailOp:      webgpu.StencilOperationKeep,
					DepthFailOp: webgpu.StencilOperationKeep,
					PassOp:      passOp,
				},
				StencilBack: webgpu.StencilFaceState{
					Compare:     compare,
					FailOp:      webgpu.StencilOperationKeep,
					DepthFailOp: webgpu.StencilOperationKeep,
					PassOp:      passOp,
				},
				StencilReadMask:  0xFF,
				StencilWriteMask: writeMask,
			},
			Multisample: multisample,
			Primitive:   primitive,
		})
	}

	band, err := mk("cover_aa_band_pipeline", &premulBlend, types.CompareFunctionEqual,
		webgpu.StencilOperationKeep, 0x00)
	if err != nil {
		return fmt.Errorf("create AA band pipeline: %w", err)
	}
	sr.aaBandPipeline = band

	inner, err := mk("cover_aa_inner_band_pipeline", &premulBlend, types.CompareFunctionNotEqual,
		webgpu.StencilOperationZero, 0xFF)
	if err != nil {
		band.Release()
		return fmt.Errorf("create AA inner band pipeline: %w", err)
	}
	sr.aaInnerBandPipeline = inner
	return nil
}

// ensureDepthClipPipelines creates the depth-clipped pipeline variants for
// GPU-CLIP-003a. These are identical to the normal pipelines except they use
// DepthCompare=GreaterEqual to restrict rendering to pixels where the depth
// clip geometry previously wrote Z=0.0.
//
// Stencil fill variants: same stencil operations but with depth test. This
// ensures the stencil buffer is only modified within the clip region.
//
// Cover variant: same stencil test + color write but with depth test. Only
// pixels inside the clip AND with non-zero stencil receive fill color.
//
// Created lazily on first use to avoid unnecessary GPU compilation.
func (sr *StencilRenderer) ensureDepthClipPipelines() error { //nolint:funlen // GPU pipeline descriptors are inherently verbose
	if sr.pipelineWithDepthClipNZ != nil {
		return nil // already created
	}
	if sr.stencilFillShader == nil || sr.stencilPipeLayout == nil {
		if err := sr.createPipelines(); err != nil {
			return err
		}
	}

	// Shared vertex buffer layout, primitive, multisample — same as base pipelines.
	vertexBufferLayout := []types.VertexBufferLayout{
		{
			ArrayStride: vertexStride,
			StepMode:    types.VertexStepModeVertex,
			Attributes: []types.VertexAttribute{
				{
					Format:         types.VertexFormatFloat32x2,
					Offset:         0,
					ShaderLocation: 0,
				},
			},
		},
	}
	multisample := multisampleState(sr.sampleCount)
	primitive := types.PrimitiveState{
		Topology: types.PrimitiveTopologyTriangleList,
		CullMode: types.CullModeNone,
	}

	// --- Non-zero stencil fill + depth clip ---
	nzPipeline, err := sr.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "stencil_fill_depth_clip_pipeline",
		Layout: sr.stencilPipeLayout,
		Vertex: webgpu.VertexState{
			Module:     sr.stencilFillShader,
			EntryPoint: shaderEntryVS,
			Buffers:    vertexBufferLayout,
		},
		Fragment: &webgpu.FragmentState{
			Module:     sr.stencilFillShader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{Format: types.TextureFormatBGRA8Unorm, WriteMask: types.ColorWriteMaskNone},
			},
		},
		DepthStencil: &webgpu.DepthStencilState{
			Format:            types.TextureFormatDepth24PlusStencil8,
			DepthWriteEnabled: false,
			DepthCompare:      types.CompareFunctionGreaterEqual,
			StencilFront: webgpu.StencilFaceState{
				Compare: types.CompareFunctionAlways, FailOp: webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep, PassOp: webgpu.StencilOperationIncrementWrap,
			},
			StencilBack: webgpu.StencilFaceState{
				Compare: types.CompareFunctionAlways, FailOp: webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep, PassOp: webgpu.StencilOperationDecrementWrap,
			},
			StencilReadMask:  0xFF,
			StencilWriteMask: 0xFF,
		},
		Multisample: multisample,
		Primitive:   primitive,
	})
	if err != nil {
		return fmt.Errorf("create stencil fill depth clip pipeline (NZ): %w", err)
	}
	sr.pipelineWithDepthClipNZ = nzPipeline

	// --- Even-odd stencil fill + depth clip ---
	// Same IncrementWrap+WriteMask=0x01 approach as the base EvenOdd pipeline
	// (avoids AMD D3D12 StencilOperationInvert bug, #374).
	eoPipeline, err := sr.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "stencil_fill_even_odd_depth_clip_pipeline",
		Layout: sr.stencilPipeLayout,
		Vertex: webgpu.VertexState{
			Module:     sr.stencilFillShader,
			EntryPoint: shaderEntryVS,
			Buffers:    vertexBufferLayout,
		},
		Fragment: &webgpu.FragmentState{
			Module:     sr.stencilFillShader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{Format: types.TextureFormatBGRA8Unorm, WriteMask: types.ColorWriteMaskNone},
			},
		},
		DepthStencil: &webgpu.DepthStencilState{
			Format:            types.TextureFormatDepth24PlusStencil8,
			DepthWriteEnabled: false,
			DepthCompare:      types.CompareFunctionGreaterEqual,
			StencilFront: webgpu.StencilFaceState{
				Compare: types.CompareFunctionAlways, FailOp: webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep, PassOp: webgpu.StencilOperationIncrementWrap,
			},
			StencilBack: webgpu.StencilFaceState{
				Compare: types.CompareFunctionAlways, FailOp: webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep, PassOp: webgpu.StencilOperationIncrementWrap,
			},
			StencilReadMask:  0xFF,
			StencilWriteMask: 0x01,
		},
		Multisample: multisample,
		Primitive:   primitive,
	})
	if err != nil {
		return fmt.Errorf("create stencil fill depth clip pipeline (EO): %w", err)
	}
	sr.pipelineWithDepthClipEO = eoPipeline

	// --- Cover pipeline + depth clip ---
	premulBlend := types.BlendStatePremultiplied()
	coverPipeline, err := sr.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "cover_depth_clip_pipeline",
		Layout: sr.coverPipeLayout,
		Vertex: webgpu.VertexState{
			Module:     sr.coverShader,
			EntryPoint: shaderEntryVS,
			Buffers:    vertexBufferLayout,
		},
		Fragment: &webgpu.FragmentState{
			Module:     sr.coverShader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{
					Format:    types.TextureFormatBGRA8Unorm,
					Blend:     &premulBlend,
					WriteMask: types.ColorWriteMaskAll,
				},
			},
		},
		DepthStencil: &webgpu.DepthStencilState{
			Format:            types.TextureFormatDepth24PlusStencil8,
			DepthWriteEnabled: false,
			DepthCompare:      types.CompareFunctionGreaterEqual,
			StencilFront: webgpu.StencilFaceState{
				Compare: types.CompareFunctionNotEqual, FailOp: webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep, PassOp: webgpu.StencilOperationZero,
			},
			StencilBack: webgpu.StencilFaceState{
				Compare: types.CompareFunctionNotEqual, FailOp: webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep, PassOp: webgpu.StencilOperationZero,
			},
			StencilReadMask:  0xFF,
			StencilWriteMask: 0xFF,
		},
		Multisample: multisample,
		Primitive:   primitive,
	})
	if err != nil {
		return fmt.Errorf("create cover depth clip pipeline: %w", err)
	}
	sr.pipelineWithDepthClipCover = coverPipeline

	return nil
}

// destroyPipelines releases all pipeline resources in reverse creation order.
// Safe to call on a renderer with no pipelines or with partially created pipelines.
func (sr *StencilRenderer) destroyPipelines() {
	if sr.device == nil {
		return
	}
	// Bump epoch first so any pooled bind groups are treated as stale even if
	// a concurrent path observes a half-destroyed renderer.
	sr.pipelineEpoch++
	sr.releaseNoMask()
	sr.coverPipeMaskLayout = nil
	// Analytic-AA fringe pipelines (sampleCount==1).
	if sr.aaBandPipeline != nil {
		sr.aaBandPipeline.Release()
		sr.aaBandPipeline = nil
	}
	if sr.aaInnerBandPipeline != nil {
		sr.aaInnerBandPipeline.Release()
		sr.aaInnerBandPipeline = nil
	}
	if sr.aaCoverShader != nil {
		sr.aaCoverShader.Release()
		sr.aaCoverShader = nil
	}
	if sr.texturedCoverPipeline != nil {
		sr.texturedCoverPipeline.Release()
		sr.texturedCoverPipeline = nil
	}
	if sr.texturedCoverPipeLay != nil {
		sr.texturedCoverPipeLay.Release()
		sr.texturedCoverPipeLay = nil
	}
	if sr.texturedCoverBGL0 != nil {
		sr.texturedCoverBGL0.Release()
		sr.texturedCoverBGL0 = nil
	}
	if sr.texturedCoverShader != nil {
		sr.texturedCoverShader.Release()
		sr.texturedCoverShader = nil
	}
	if sr.texturedCoverSampler != nil {
		sr.texturedCoverSampler.Release()
		sr.texturedCoverSampler = nil
	}
	if sr.patternCoverPipeline != nil {
		sr.patternCoverPipeline.Release()
		sr.patternCoverPipeline = nil
	}
	if sr.patternCoverPipeLay != nil {
		sr.patternCoverPipeLay.Release()
		sr.patternCoverPipeLay = nil
	}
	if sr.patternCoverBGL0 != nil {
		sr.patternCoverBGL0.Release()
		sr.patternCoverBGL0 = nil
	}
	if sr.patternCoverShader != nil {
		sr.patternCoverShader.Release()
		sr.patternCoverShader = nil
	}
	if sr.patternCoverSampler != nil {
		sr.patternCoverSampler.Release()
		sr.patternCoverSampler = nil
	}
	// Depth-clipped variants (GPU-CLIP-003a).
	if sr.pipelineWithDepthClipCover != nil {
		sr.pipelineWithDepthClipCover.Release()
		sr.pipelineWithDepthClipCover = nil
	}
	if sr.pipelineWithDepthClipEO != nil {
		sr.pipelineWithDepthClipEO.Release()
		sr.pipelineWithDepthClipEO = nil
	}
	if sr.pipelineWithDepthClipNZ != nil {
		sr.pipelineWithDepthClipNZ.Release()
		sr.pipelineWithDepthClipNZ = nil
	}
	// Cached cover pipelines for non-SourceOver blend modes (B.02).
	// Missing Release pinned Device across AutoRecover (cover_pipeline_blend_Plus).
	for mode, pipe := range sr.coverBlendPipelines {
		if pipe != nil {
			pipe.Release()
		}
		delete(sr.coverBlendPipelines, mode)
	}
	sr.coverBlendPipelines = nil
	// Base pipelines.
	if sr.nonZeroCoverPipeline != nil {
		sr.nonZeroCoverPipeline.Release()
		sr.nonZeroCoverPipeline = nil
	}
	if sr.evenOddStencilPipeline != nil {
		sr.evenOddStencilPipeline.Release()
		sr.evenOddStencilPipeline = nil
	}
	if sr.nonZeroStencilPipeline != nil {
		sr.nonZeroStencilPipeline.Release()
		sr.nonZeroStencilPipeline = nil
	}
	if sr.coverPipeLayout != nil {
		sr.coverPipeLayout.Release()
		sr.coverPipeLayout = nil
		sr.coverPipeLayoutHasClip = false
	}
	if sr.defaultClipBindLayout != nil {
		sr.defaultClipBindLayout.Release()
		sr.defaultClipBindLayout = nil
	}
	if sr.maskLayoutOwned && sr.maskBindLayout != nil {
		sr.maskBindLayout.Release()
	}
	sr.maskBindLayout = nil
	sr.maskLayoutOwned = false
	if sr.stencilPipeLayout != nil {
		sr.stencilPipeLayout.Release()
		sr.stencilPipeLayout = nil
	}
	if sr.uniformLayout != nil {
		sr.uniformLayout.Release()
		sr.uniformLayout = nil
	}
	if sr.coverShader != nil {
		sr.coverShader.Release()
		sr.coverShader = nil
	}
	if sr.stencilFillShader != nil {
		sr.stencilFillShader.Release()
		sr.stencilFillShader = nil
	}
}
