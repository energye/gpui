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

//go:embed shaders/depth_clip.wgsl
var depthClipShaderSource string

// depthClipUniformSize is the byte size of the depth clip uniform buffer.
// Layout: viewport (vec2<f32>) + pad (vec2<f32>) = 16 bytes.
// Same layout as stencil_fill.wgsl uniforms for consistency.
const depthClipUniformSize = 16

// DepthClipPipeline manages the GPU pipeline for depth-based arbitrary path
// clipping (GPU-CLIP-003a). Uses a stencil-then-cover-to-depth algorithm for
// correct non-convex path clipping within a single render pass.
//
// This follows the Skia Ganesh pattern: the stencil buffer determines the
// winding number of the clip path, then a cover quad writes depth only where
// the stencil test passes (inside the clip). This ensures correct clipping
// for arbitrary paths including stars, bezier shapes, and self-intersecting
// paths where simple fan tessellation + direct depth write would be wrong.
//
// Depth model:
//   - DepthClearValue = 1.0 (existing, unchanged)
//   - Clip path writes Z = 0.0 where geometry exists → depth buffer = 0.0
//   - Where clip absent, depth buffer remains 1.0 (clear value)
//   - Content pipelines use DepthCompare=GreaterEqual, fragment Z = 0.0
//   - Where clip drawn:     0.0 >= 0.0 → PASS
//   - Where clip NOT drawn: 0.0 >= 1.0 → FAIL
//
// Algorithm (two-phase, same render pass, BEFORE content):
//
//	Phase 1 — Stencil fill (winding number):
//	  Fan-tessellated clip path → stencil IncrementWrap/DecrementWrap.
//	  After this: stencil != 0 inside clip, stencil == 0 outside.
//	  DepthWriteEnabled=false, ColorWriteMask=None.
//
//	Phase 2 — Cover quad → depth write:
//	  Bounding box quad covering the clip region.
//	  StencilCompare=NotEqual(0) → only passes inside clip.
//	  DepthCompare=Always, DepthWriteEnabled=true → writes Z=0.0.
//	  StencilPassOp=Zero → resets stencil for Tier 2b reuse.
//	  ColorWriteMask=None.
//
// After both phases:
//   - Depth buffer = 0.0 where clip path covers (correct winding)
//   - Depth buffer = 1.0 (clear) outside clip path
//   - Stencil buffer = 0 everywhere (clean for Tier 2b)
//
// Nested clips:
//
//	All clip levels write Z=0.0. The intersection of nested clips happens
//	GEOMETRICALLY — clip 2's path only covers pixels where clip 1 already
//	wrote 0.0. Content at any depth tests GreaterEqual(0.0, buffer): passes
//	where ANY clip wrote 0.0, fails where no clip touched. This is the
//	simplest correct nested model.
//
//	Clip restore (pop) limitation: within a single ScissorGroup, depth writes
//	cannot be "undone" without redrawing. For v1, each ScissorGroup has at
//	most ONE ClipPath. Nested clips from the scene graph create nested groups
//	or use the Context CPU clip stack. This matches other renderers (SDF,
//	convex, text) which each have one depth clip state per group.
//
// Architecture:
//
//	ScissorGroup.ClipPath → FanTessellator → stencil fill + cover-to-depth
//	  Phase 1: reuses stencil fill pipeline (IncrWrap/DecrWrap, no depth write)
//	  Phase 2: depthCoverPipeline (stencil NotEqual, depth write, stencil zero)
type DepthClipPipeline struct {
	device      *webgpu.Device
	queue       *webgpu.Queue
	sampleCount uint32 // MSAA sample count (4 or 1), from GPUShared

	// shader is the vertex/fragment shader for the cover-to-depth pass.
	// Reuses the same depth_clip.wgsl (vertex: NDC transform, Z=0.0; fragment: no-op).
	shader *webgpu.ShaderModule

	// uniformBGL is the bind group layout for the uniform buffer (@group(0) @binding(0)).
	uniformBGL *webgpu.BindGroupLayout

	// pipeLayout is the pipeline layout for the stencil fill phase (uniform only).
	pipeLayout *webgpu.PipelineLayout

	// stencilFillPipeline performs Phase 1: fan triangles → stencil buffer.
	// Non-zero winding: front IncrementWrap, back DecrementWrap.
	// DepthWriteEnabled=false, ColorWriteMask=None.
	stencilFillPipeline *webgpu.RenderPipeline

	// depthCoverPipeline performs Phase 2: cover quad → depth write.
	// StencilCompare=NotEqual(0), DepthCompare=Always, DepthWriteEnabled=true.
	// StencilPassOp=Zero (cleanup for Tier 2b). ColorWriteMask=None.
	depthCoverPipeline *webgpu.RenderPipeline

	tessellator  *FanTessellator
	uniformBuf   *webgpu.Buffer
	bindGroup    *webgpu.BindGroup
	vertBuf      *webgpu.Buffer
	vertBufCap   uint64
	coverBuf     *webgpu.Buffer // vertex buffer for cover quad (6 vertices)
	coverBufCap  uint64         // capacity of cover buffer in bytes
	vertexStaged []byte         // CPU staging buffer for vertex data
}

// NewDepthClipPipeline creates a new depth clip pipeline for the given device.
// The pipelines are not created until ensurePipeline() is called.
func NewDepthClipPipeline(device *webgpu.Device, queue *webgpu.Queue, sampleCount uint32) *DepthClipPipeline {
	return &DepthClipPipeline{
		device:      device,
		queue:       queue,
		sampleCount: sampleCount,
		tessellator: NewFanTessellator(),
	}
}

// ensurePipeline compiles shaders and creates both GPU pipelines if not
// already created:
//   - stencilFillPipeline: fan triangles → stencil IncrWrap/DecrWrap (Phase 1)
//   - depthCoverPipeline: cover quad → depth write where stencil!=0 (Phase 2)
func (p *DepthClipPipeline) ensurePipeline() error { //nolint:funlen // GPU pipeline descriptors are inherently verbose
	if p.depthCoverPipeline != nil {
		return nil
	}

	// Compile shader (shared by both pipelines — same vertex transform, same no-op fragment).
	shader, err := p.device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "depth_clip_shader",
		WGSL:  depthClipShaderSource,
	})
	if err != nil {
		return fmt.Errorf("compile depth clip shader: %w", err)
	}
	p.shader = shader

	// Bind group layout: one uniform buffer at group(0) binding(0).
	bgl, err := p.device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "depth_clip_uniform_layout",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
				Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("create depth clip bind group layout: %w", err)
	}
	p.uniformBGL = bgl

	// Pipeline layout: just the uniform bind group (no clip @group(1) needed
	// for the clip pipeline itself -- it IS the clip).
	pipeLayout, err := p.device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label:            "depth_clip_pipe_layout",
		BindGroupLayouts: []*webgpu.BindGroupLayout{p.uniformBGL},
	})
	if err != nil {
		return fmt.Errorf("create depth clip pipeline layout: %w", err)
	}
	p.pipeLayout = pipeLayout

	// Shared vertex buffer layout: float32x2 position at location(0).
	vertexBufLayout := []types.VertexBufferLayout{
		{
			ArrayStride: vertexStride, // 8 bytes (2 x float32)
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
	multisample := multisampleState(p.sampleCount)
	primitive := types.PrimitiveState{
		Topology: types.PrimitiveTopologyTriangleList,
		CullMode: types.CullModeNone,
	}

	// --- Phase 1 pipeline: Stencil fill (non-zero winding) ---
	//
	// Fan-tessellated clip path writes to stencil buffer only.
	// Front faces: IncrementWrap, Back faces: DecrementWrap.
	// After pass: stencil != 0 inside clip, stencil == 0 outside.
	// No depth write, no color write.
	stencilFillPipeline, err := p.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "depth_clip_stencil_fill_pipeline",
		Layout: p.pipeLayout,
		Vertex: webgpu.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    vertexBufLayout,
		},
		Fragment: &webgpu.FragmentState{
			Module:     p.shader,
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
			DepthWriteEnabled: false,                       // don't write depth in Phase 1
			DepthCompare:      types.CompareFunctionAlways, // pass all depth tests
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
		return fmt.Errorf("create depth clip stencil fill pipeline: %w", err)
	}
	p.stencilFillPipeline = stencilFillPipeline

	// --- Phase 2 pipeline: Cover quad → depth write ---
	//
	// Draws a bounding box quad. Only pixels where stencil != 0 (inside clip)
	// pass the stencil test. Those pixels get depth = 0.0 written.
	// StencilPassOp=Zero resets stencil to 0, cleaning up for Tier 2b.
	// No color output.
	depthCoverPipeline, err := p.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "depth_clip_cover_pipeline",
		Layout: p.pipeLayout,
		Vertex: webgpu.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    vertexBufLayout,
		},
		Fragment: &webgpu.FragmentState{
			Module:     p.shader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{
					Format:    types.TextureFormatBGRA8Unorm,
					WriteMask: types.ColorWriteMaskNone, // no color output
				},
			},
		},
		DepthStencil: &webgpu.DepthStencilState{
			Format:            types.TextureFormatDepth24PlusStencil8,
			DepthWriteEnabled: true,                        // write depth Z=0.0
			DepthCompare:      types.CompareFunctionAlways, // always pass depth test
			StencilFront: webgpu.StencilFaceState{
				Compare:     types.CompareFunctionNotEqual, // only where stencil != 0
				FailOp:      webgpu.StencilOperationKeep,   // outside clip: keep stencil (already 0)
				DepthFailOp: webgpu.StencilOperationKeep,   // depth always passes, never hit
				PassOp:      webgpu.StencilOperationZero,   // reset stencil after use
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
		return fmt.Errorf("create depth clip cover pipeline: %w", err)
	}
	p.depthCoverPipeline = depthCoverPipeline

	return nil
}

// DepthClipResources holds per-frame resources for one depth clip draw.
// Contains both the fan tessellation vertices (Phase 1: stencil fill) and
// the cover quad vertices (Phase 2: depth write).
type DepthClipResources struct {
	vertBuf    *webgpu.Buffer    // fan triangle vertices for stencil fill
	coverBuf   *webgpu.Buffer    // bounding box quad vertices for cover pass
	bindGroup  *webgpu.BindGroup // uniform bind group (viewport)
	vertCount  uint32            // number of fan vertices (Phase 1)
	coverCount uint32            // number of cover quad vertices (Phase 2, always 6)
	owned      bool              // if true, vertBuf and coverBuf are per-call (must be released)
}

// Release frees per-call GPU buffers if owned.
func (r *DepthClipResources) Release() {
	if r == nil || !r.owned {
		return
	}
	if r.vertBuf != nil {
		r.vertBuf.Release()
		r.vertBuf = nil
	}
	if r.coverBuf != nil {
		r.coverBuf.Release()
		r.coverBuf = nil
	}
}

// BuildClipResources tessellates the clip path and uploads vertices + uniforms
// to the GPU. Returns resources needed for RecordDraw, or nil if the path is
// empty (no clip to draw).
//
// Produces two vertex buffers:
//   - Fan triangles for Phase 1 (stencil fill) — determines winding inside clip
//   - Cover quad for Phase 2 (depth write) — bounding box of the clip path
func (p *DepthClipPipeline) BuildClipResources(
	clipPath *render.Path,
	w, h uint32,
) (*DepthClipResources, error) {
	// Tessellate clip path into fan triangles.
	p.tessellator.Reset()
	vertCount := p.tessellator.TessellatePath(clipPath)
	if vertCount == 0 {
		return nil, nil //nolint:nilnil // empty clip path, nothing to draw
	}

	// Upload fan vertices (Phase 1: stencil fill).
	if err := p.uploadFanVertices(); err != nil {
		return nil, err
	}

	// Upload cover quad vertices (Phase 2: depth write via stencil test).
	if err := p.uploadCoverQuad(); err != nil {
		return nil, err
	}

	// Upload uniforms (viewport dimensions).
	if err := p.uploadUniforms(w, h); err != nil {
		return nil, err
	}

	// Ensure bind group (recreated if uniform buffer changed).
	if p.bindGroup == nil {
		bg, err := p.device.CreateBindGroup(&webgpu.BindGroupDescriptor{
			Label:  "depth_clip_bind",
			Layout: p.uniformBGL,
			Entries: []webgpu.BindGroupEntry{
				{Binding: 0, Buffer: p.uniformBuf, Offset: 0, Size: depthClipUniformSize},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("create depth clip bind group: %w", err)
		}
		p.bindGroup = bg
	}

	// Create per-call vertex buffers so multiple groups don't overwrite each other.
	// The pipeline-level buffers (p.vertBuf, p.coverBuf) are staging — copy to owned buffers.
	ownedVertBuf, err := p.device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "depth_clip_fan_owned",
		Size:  uint64(len(p.tessellator.Vertices())) * 4, //nolint:gosec // bounded
		Usage: types.BufferUsageVertex | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create owned fan buffer: %w", err)
	}
	fanData := make([]byte, len(p.tessellator.Vertices())*4)
	for i, v := range p.tessellator.Vertices() {
		binary.LittleEndian.PutUint32(fanData[i*4:], math.Float32bits(v))
	}
	if wErr := p.queue.WriteBuffer(ownedVertBuf, 0, fanData); wErr != nil {
		ownedVertBuf.Release()
		return nil, fmt.Errorf("write owned fan buffer: %w", wErr)
	}

	coverQuad := p.tessellator.CoverQuad()
	ownedCoverBuf, err := p.device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "depth_clip_cover_owned",
		Size:  12 * 4,
		Usage: types.BufferUsageVertex | types.BufferUsageCopyDst,
	})
	if err != nil {
		ownedVertBuf.Release()
		return nil, fmt.Errorf("create owned cover buffer: %w", err)
	}
	coverData := make([]byte, 12*4)
	for i, v := range coverQuad {
		binary.LittleEndian.PutUint32(coverData[i*4:], math.Float32bits(v))
	}
	if wErr := p.queue.WriteBuffer(ownedCoverBuf, 0, coverData); wErr != nil {
		ownedVertBuf.Release()
		ownedCoverBuf.Release()
		return nil, fmt.Errorf("write owned cover buffer: %w", wErr)
	}

	return &DepthClipResources{
		vertBuf:    ownedVertBuf,
		coverBuf:   ownedCoverBuf,
		bindGroup:  p.bindGroup,
		vertCount:  uint32(vertCount), //nolint:gosec // bounded by tessellator
		coverCount: 6,
		owned:      true, // mark for cleanup
	}, nil
}

// uploadFanVertices ensures the fan vertex buffer is large enough and uploads
// the tessellated fan triangle data to the GPU.
func (p *DepthClipPipeline) uploadFanVertices() error {
	verts := p.tessellator.Vertices()
	vertBytes := uint64(len(verts)) * 4 //nolint:gosec // len(verts) bounded by tessellator

	// Ensure fan vertex buffer (grow-only).
	if p.vertBuf == nil || p.vertBufCap < vertBytes {
		if p.vertBuf != nil {
			p.vertBuf.Release()
		}
		newCap := vertBytes
		if newCap < 4096 {
			newCap = 4096 // minimum 4KB
		}
		buf, err := p.device.CreateBuffer(&webgpu.BufferDescriptor{
			Label: "depth_clip_vert",
			Size:  newCap,
			Usage: types.BufferUsageVertex | types.BufferUsageCopyDst,
		})
		if err != nil {
			return fmt.Errorf("create depth clip vertex buffer: %w", err)
		}
		p.vertBuf = buf
		p.vertBufCap = newCap
	}

	// Stage and upload fan vertex data.
	if uint64(cap(p.vertexStaged)) < vertBytes {
		p.vertexStaged = make([]byte, vertBytes)
	}
	staging := p.vertexStaged[:vertBytes]
	for i, v := range verts {
		binary.LittleEndian.PutUint32(staging[i*4:], math.Float32bits(v))
	}
	if err := p.queue.WriteBuffer(p.vertBuf, 0, staging); err != nil {
		return fmt.Errorf("write depth clip vertices: %w", err)
	}
	return nil
}

// uploadCoverQuad ensures the cover vertex buffer exists and uploads the
// bounding box quad vertices (from tessellator bounds) to the GPU.
func (p *DepthClipPipeline) uploadCoverQuad() error {
	coverQuad := p.tessellator.CoverQuad() // [12]float32: 6 vertices (2 triangles)
	const coverBytes = 12 * 4              // 12 float32 values

	// Ensure cover vertex buffer.
	if p.coverBuf == nil || p.coverBufCap < coverBytes {
		if p.coverBuf != nil {
			p.coverBuf.Release()
		}
		buf, err := p.device.CreateBuffer(&webgpu.BufferDescriptor{
			Label: "depth_clip_cover_vert",
			Size:  coverBytes,
			Usage: types.BufferUsageVertex | types.BufferUsageCopyDst,
		})
		if err != nil {
			return fmt.Errorf("create depth clip cover buffer: %w", err)
		}
		p.coverBuf = buf
		p.coverBufCap = coverBytes
	}

	// Upload cover quad vertices.
	coverData := make([]byte, coverBytes)
	for i, v := range coverQuad {
		binary.LittleEndian.PutUint32(coverData[i*4:], math.Float32bits(v))
	}
	if err := p.queue.WriteBuffer(p.coverBuf, 0, coverData); err != nil {
		return fmt.Errorf("write depth clip cover vertices: %w", err)
	}
	return nil
}

// uploadUniforms ensures the uniform buffer exists and writes viewport
// dimensions to it.
func (p *DepthClipPipeline) uploadUniforms(w, h uint32) error {
	if p.uniformBuf == nil {
		buf, err := p.device.CreateBuffer(&webgpu.BufferDescriptor{
			Label: "depth_clip_uniform",
			Size:  depthClipUniformSize,
			Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
		})
		if err != nil {
			return fmt.Errorf("create depth clip uniform buffer: %w", err)
		}
		p.uniformBuf = buf
	}

	uniformData := make([]byte, depthClipUniformSize)
	binary.LittleEndian.PutUint32(uniformData[0:4], math.Float32bits(float32(w)))
	binary.LittleEndian.PutUint32(uniformData[4:8], math.Float32bits(float32(h)))
	// bytes 8..15 remain zero (padding).
	if err := p.queue.WriteBuffer(p.uniformBuf, 0, uniformData); err != nil {
		return fmt.Errorf("write depth clip uniforms: %w", err)
	}
	return nil
}

// RecordDraw records the two-phase depth clip draw commands into a render pass.
// This must be called BEFORE any content draws in the group, so the depth
// buffer is populated before content pipelines test against it.
//
// Phase 1: Stencil fill — fan triangles write winding number to stencil buffer.
//
//	After this phase: stencil != 0 inside clip, stencil == 0 outside.
//
// Phase 2: Cover quad — writes depth Z=0.0 only where stencil != 0 (inside clip).
//
//	Also resets stencil to 0 (PassOp=Zero) so Tier 2b stencil is clean.
//
// This correctly clips arbitrary non-convex paths (stars, bezier shapes, etc.)
// because the stencil buffer determines interior via winding number, not just
// triangle coverage.
func (p *DepthClipPipeline) RecordDraw(rp *webgpu.RenderPassEncoder, res *DepthClipResources) {
	if res == nil || res.vertCount == 0 {
		return
	}

	// Phase 1: Stencil fill — fan triangles determine winding inside clip path.
	// Front faces IncrementWrap, back faces DecrementWrap. No depth write, no color.
	clearPassBindGroups(rp)
	rp.SetPipeline(p.stencilFillPipeline)
	rp.SetBindGroup(0, res.bindGroup, nil)
	rp.SetVertexBuffer(0, res.vertBuf, 0)
	rp.SetStencilReference(0)
	rp.Draw(res.vertCount, 1, 0, 0)

	// Phase 2: Cover quad — write depth where stencil != 0, reset stencil to 0.
	// Only pixels inside the clip path (stencil != 0) receive depth Z=0.0.
	// StencilPassOp=Zero cleans up stencil for subsequent Tier 2b rendering.
	// Fill/cover share the same group-0 layout; clear is cheap insurance.
	clearPassBindGroups(rp)
	rp.SetPipeline(p.depthCoverPipeline)
	rp.SetBindGroup(0, res.bindGroup, nil)
	rp.SetVertexBuffer(0, res.coverBuf, 0)
	rp.SetStencilReference(0)
	rp.Draw(res.coverCount, 1, 0, 0)
}

// Destroy releases all GPU resources held by the depth clip pipeline.
func (p *DepthClipPipeline) Destroy() {
	if p.bindGroup != nil {
		p.bindGroup.Release()
		p.bindGroup = nil
	}
	if p.uniformBuf != nil {
		p.uniformBuf.Release()
		p.uniformBuf = nil
	}
	if p.coverBuf != nil {
		p.coverBuf.Release()
		p.coverBuf = nil
		p.coverBufCap = 0
	}
	if p.vertBuf != nil {
		p.vertBuf.Release()
		p.vertBuf = nil
		p.vertBufCap = 0
	}
	if p.depthCoverPipeline != nil {
		p.depthCoverPipeline.Release()
		p.depthCoverPipeline = nil
	}
	if p.stencilFillPipeline != nil {
		p.stencilFillPipeline.Release()
		p.stencilFillPipeline = nil
	}
	if p.pipeLayout != nil {
		p.pipeLayout.Release()
		p.pipeLayout = nil
	}
	if p.uniformBGL != nil {
		p.uniformBGL.Release()
		p.uniformBGL = nil
	}
	if p.shader != nil {
		p.shader.Release()
		p.shader = nil
	}
}
