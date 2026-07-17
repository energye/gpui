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

//go:embed shaders/convex.wgsl
var convexShaderSource string

// convexVertexStride is the byte stride per vertex in the convex render pipeline.
// Layout per vertex:
//
//	position (vec2<f32>) = 8 bytes  (location 0)
//	coverage (f32)       = 4 bytes  (location 1)
//	color    (vec4<f32>) = 16 bytes (location 2)
//
// Total = 28 bytes per vertex.
const convexVertexStride = 28

// convexAAExpand is the outward expansion distance in pixels for the
// anti-aliasing fringe around convex polygon edges. 0.75px gives a fuller
// one-pixel AA ramp (was 0.5) for diagonals/curves without bloating fills.
const convexAAExpand = 0.75

// ConvexDrawCommand holds the geometry and paint for a single convex polygon
// to be rendered via the convex fast-path renderer. Points must form a convex
// polygon (verified by IsConvex before queuing).
type ConvexDrawCommand struct {
	// Points are the convex polygon vertices in pixel coordinates,
	// after any curve flattening. The polygon is treated as closed
	// (last point connects to first).
	Points []render.Point

	// Color is the premultiplied RGBA fill color used when VertexColors
	// is empty (solid fill).
	Color [4]float32

	// VertexColors are optional per-vertex premultiplied RGBA colors.
	// When len(VertexColors) == len(Points), Gouraud shading is used;
	// the fan centroid uses the average of VertexColors unless
	// HasCentroidColor is set (P0-2 radial/sweep gradients).
	VertexColors [][4]float32

	// HasCentroidColor / CentroidColor override the fan hub color when
	// VertexColors is set. Radial and sweep brushes need ColorAt(centroid)
	// rather than the mean of boundary samples.
	HasCentroidColor bool
	CentroidColor    [4]float32

	// BlendMode selects the WebGPU blend state for this draw (B.02).
	// Zero value is BlendNormal (SourceOver).
	BlendMode render.BlendMode

	// SkipAA disables centroid-fan AA fringe expansion. Used by DrawMesh /
	// DrawVertices triangle lists (Skia drawVertices semantics): solid
	// triangles only — much cheaper for dense meshes.
	SkipAA bool

	// TriangleList means Points are independent triangles (groups of 3), not
	// one convex polygon. Used with SkipAA for dense DrawMesh batches so a
	// whole mesh is one command instead of N tri-commands.
	TriangleList bool
}

// ConvexRenderer renders convex polygons in a single draw call with per-edge
// analytic anti-aliasing. No stencil buffer is needed.
//
// This is Tier 2a in the GPU rendering hierarchy:
//
//	Tier 1:  SDF fragment shader (circles, rects, rrects)
//	Tier 2a: Convex fast-path (this) -- single draw, per-edge AA
//	Tier 2b: Stencil-then-cover -- arbitrary paths
//
// The algorithm fans from the polygon centroid, generating interior triangles
// with coverage=1.0 and AA fringe strips (0.5px outward expansion) with
// coverage ramping from 1.0 to 0.0 at the outermost edge.
//
// For the unified render pass (GPURenderSession), use pipelineWithStencil
// which includes a depth/stencil state that ignores the stencil buffer
// (Compare=Always, all ops=Keep, masks=0x00).
type ConvexRenderer struct {
	device      *webgpu.Device
	queue       *webgpu.Queue
	sampleCount uint32 // MSAA sample count (4 or 1), from GPUShared

	// GPU objects for the render pipeline.
	shader        *webgpu.ShaderModule
	uniformLayout *webgpu.BindGroupLayout
	pipeLayout    *webgpu.PipelineLayout
	pipeline      *webgpu.RenderPipeline

	// Session-compatible pipeline variant with depth/stencil state.
	// Used when this renderer participates in a unified render pass that
	// includes a stencil attachment (for stencil-then-cover paths).
	// The stencil test is Always/Keep (convex draws don't interact with stencil).
	pipelineWithStencil *webgpu.RenderPipeline

	// Depth-clipped pipeline variant (GPU-CLIP-003a). Same as pipelineWithStencil
	// but with DepthCompare=GreaterEqual to test against the depth clip buffer.
	pipelineWithDepthClip *webgpu.RenderPipeline

	// blendPipelinesWithStencil caches SourceOver-alternative cover pipelines
	// keyed by render.BlendMode (B.02 fixed-function Porter-Duff).
	blendPipelinesWithStencil map[render.BlendMode]*webgpu.RenderPipeline

	// Clip bind group layout for @group(1). Set by the session before
	// pipeline creation. When non-nil, included in the pipeline layout.
	clipBindLayout *webgpu.BindGroupLayout
	// defaultClipBindLayout is owned by this renderer and used only when a
	// standalone pipeline is created before the session supplies its layout.
	defaultClipBindLayout *webgpu.BindGroupLayout
	// pipeLayoutHasClip tracks whether the current pipeLayout was created
	// with clipBindLayout included. If clipBindLayout is set after the
	// layout was created, the pipeline must be recreated.
	pipeLayoutHasClip bool

	// maskBindLayout is @group(2) for L.06 full-surface R8 mask sampling.
	// Usually session-owned; maskLayoutOwned true only for standalone create.
	maskBindLayout  *webgpu.BindGroupLayout
	maskLayoutOwned bool
}

// SetClipBindLayout sets the bind group layout for the @group(1) RRect clip
// uniform. Must be called before ensurePipelineWithStencil.
func (cr *ConvexRenderer) SetClipBindLayout(layout *webgpu.BindGroupLayout) {
	cr.clipBindLayout = layout
}

// SetMaskBindLayout sets the shared @group(2) mask layout (session-owned).
func (cr *ConvexRenderer) SetMaskBindLayout(layout *webgpu.BindGroupLayout) {
	cr.maskBindLayout = layout
	cr.maskLayoutOwned = false
}

// MaskBindLayout returns the @group(2) layout for L.06 R8 mask sampling.
// Creates the pipeline base layouts if needed so the layout is available.
func (cr *ConvexRenderer) MaskBindLayout() *webgpu.BindGroupLayout {
	if cr.maskBindLayout == nil {
		_ = cr.ensurePipeline()
	}
	return cr.maskBindLayout
}

// NewConvexRenderer creates a new convex polygon renderer with the given
// device and queue. Pipelines are not created until ensurePipeline or
// ensurePipelineWithStencil is called.
func NewConvexRenderer(device *webgpu.Device, queue *webgpu.Queue, sampleCount uint32) *ConvexRenderer {
	return &ConvexRenderer{
		device:      device,
		queue:       queue,
		sampleCount: sampleCount,
	}
}

// Destroy releases all GPU resources held by the renderer. Safe to call
// multiple times or on a renderer with no allocated resources.
func (cr *ConvexRenderer) Destroy() {
	cr.destroyPipeline()
}

// ensurePipeline creates the shader, layouts, and standalone render pipeline
// if they don't already exist.
func (cr *ConvexRenderer) ensurePipeline() error {
	if cr.pipeline != nil {
		return nil
	}
	return cr.createPipeline()
}

// ensurePipelineWithStencil creates the session-compatible pipeline variant
// that includes a depth/stencil state. The convex pipeline ignores the stencil
// buffer (Compare=Always, all ops=Keep, write mask=0).
//
// The base pipeline (shader, layout) is created first if it doesn't exist.
func (cr *ConvexRenderer) ensurePipelineWithStencil() error { // Ensure base resources exist (shader, layouts).
	if cr.shader == nil || cr.uniformLayout == nil || cr.pipeLayout == nil {
		if err := cr.createPipeline(); err != nil {
			return err
		}
	}
	// If the pipeline layout was created without clip but clip is now set,
	// destroy and recreate so the layout includes @group(1). Without this,
	// SetBindGroup(1, clipBG) crashes on AMD/NVIDIA (Intel tolerates it).
	if cr.clipBindLayout != nil && !cr.pipeLayoutHasClip {
		cr.destroyPipeline()
		if err := cr.createPipeline(); err != nil {
			return err
		}
	}
	if cr.pipelineWithStencil != nil {
		return nil
	}

	premulBlend := types.BlendStatePremultiplied()
	pipeline, err := cr.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "convex_pipeline_with_stencil",
		Layout: cr.pipeLayout,
		Vertex: webgpu.VertexState{
			Module:     cr.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    convexVertexLayout(),
		},
		Fragment: &webgpu.FragmentState{
			Module:     cr.shader,
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
		Multisample:  multisampleState(cr.sampleCount),
	})
	if err != nil {
		return fmt.Errorf("create convex pipeline with stencil: %w", err)
	}
	cr.pipelineWithStencil = pipeline
	return nil
}

// RecordDraws records convex polygon draw commands into an existing render pass.
// The render pass is owned by GPURenderSession. This method uses the
// pipelineWithStencil variant because the session's render pass includes a
// depth/stencil attachment.
//
// The resources parameter holds pre-built vertex buffer, uniform buffer, and
// bind group for the current frame. This is a no-op if resources is nil.
// ensureDepthClipPipeline creates the depth-clipped pipeline variant if needed.
func (cr *ConvexRenderer) ensureDepthClipPipeline() error {
	if cr.pipelineWithDepthClip != nil {
		return nil
	}
	if cr.shader == nil || cr.pipeLayout == nil {
		if err := cr.ensurePipelineWithStencil(); err != nil {
			return err
		}
	}

	premulBlend := types.BlendStatePremultiplied()
	pipeline, err := cr.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "convex_pipeline_depth_clip",
		Layout: cr.pipeLayout,
		Vertex: webgpu.VertexState{
			Module:     cr.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    convexVertexLayout(),
		},
		Fragment: &webgpu.FragmentState{
			Module:     cr.shader,
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
		Multisample:  multisampleState(cr.sampleCount),
	})
	if err != nil {
		return fmt.Errorf("create convex pipeline with depth clip: %w", err)
	}
	cr.pipelineWithDepthClip = pipeline
	return nil
}

// RecordDraws records convex polygon draws into an existing render pass.
// When depthClipped is true (GPU-CLIP-003a), the depth-clipped pipeline
// variant is used to test fragments against the depth clip buffer.
func (cr *ConvexRenderer) RecordDraws(rp *webgpu.RenderPassEncoder, resources *convexFrameResources, clipBG *webgpu.BindGroup, maskBG *webgpu.BindGroup, depthClipped ...bool) {
	if resources == nil || resources.vertCount == 0 {
		return
	}
	useDepthClip := len(depthClipped) > 0 && depthClipped[0] && cr.pipelineWithDepthClip != nil
	rp.SetBindGroup(0, resources.bindGroup, nil)
	if clipBG != nil {
		rp.SetBindGroup(1, clipBG, nil)
	}
	if maskBG != nil {
		rp.SetBindGroup(2, maskBG, nil)
	}
	rp.SetVertexBuffer(0, resources.vertBuf, 0)

	ranges := resources.ranges
	if len(ranges) == 0 {
		ranges = []convexDrawRange{{
			firstVertex: resources.firstVertex,
			vertCount:   resources.vertCount,
			blendMode:   render.BlendNormal,
		}}
	}
	for _, rg := range ranges {
		if rg.vertCount == 0 {
			continue
		}
		pipe := cr.pipelineForBlend(rg.blendMode, useDepthClip)
		if pipe == nil {
			continue
		}
		rp.SetPipeline(pipe)
		rp.Draw(rg.vertCount, 1, rg.firstVertex, 0)
	}
}

// pipelineForBlend returns the stencil-pass-compatible pipeline for mode.
// Depth-clipped variants currently only exist for SourceOver; non-SO depth-clip
// falls back to the non-depth-clipped blend pipeline.
func (cr *ConvexRenderer) pipelineForBlend(mode render.BlendMode, depthClip bool) *webgpu.RenderPipeline {
	if mode == render.BlendNormal {
		if depthClip && cr.pipelineWithDepthClip != nil {
			return cr.pipelineWithDepthClip
		}
		return cr.pipelineWithStencil
	}
	if depthClip {
		// No depth-clip specialized non-SO pipelines yet.
		depthClip = false
	}
	if pipe, ok := cr.blendPipelinesWithStencil[mode]; ok && pipe != nil {
		return pipe
	}
	pipe, err := cr.createBlendPipelineWithStencil(mode)
	if err != nil {
		slogger().Warn("convex blend pipeline", "mode", mode, "err", err)
		return cr.pipelineWithStencil
	}
	return pipe
}

func (cr *ConvexRenderer) createBlendPipelineWithStencil(mode render.BlendMode) (*webgpu.RenderPipeline, error) {
	if cr.pipeLayout == nil || cr.shader == nil {
		if err := cr.createPipeline(); err != nil {
			return nil, err
		}
	}
	if err := cr.ensurePipelineWithStencil(); err != nil {
		return nil, err
	}
	bs, ok := gpuBlendStateForPaint(mode)
	if !ok {
		return nil, fmt.Errorf("unsupported convex blend mode %v", mode)
	}
	pipeline, err := cr.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  fmt.Sprintf("convex_pipeline_blend_%v", mode),
		Layout: cr.pipeLayout,
		Vertex: webgpu.VertexState{
			Module:     cr.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    convexVertexLayout(),
		},
		Fragment: &webgpu.FragmentState{
			Module:     cr.shader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{
					Format:    types.TextureFormatBGRA8Unorm,
					Blend:     &bs,
					WriteMask: types.ColorWriteMaskAll,
				},
			},
		},
		DepthStencil: stencilPassthroughDepthStencil(),
		Primitive:    triangleListPrimitive(),
		Multisample:  multisampleState(cr.sampleCount),
	})
	if err != nil {
		return nil, fmt.Errorf("create convex blend pipeline %v: %w", mode, err)
	}
	if cr.blendPipelinesWithStencil == nil {
		cr.blendPipelinesWithStencil = make(map[render.BlendMode]*webgpu.RenderPipeline)
	}
	cr.blendPipelinesWithStencil[mode] = pipeline
	return pipeline, nil
}

// createPipeline compiles the convex render shader and creates the render
// pipeline with premultiplied alpha blending and MSAA.
func (cr *ConvexRenderer) createPipeline() error {
	if convexShaderSource == "" {
		return fmt.Errorf("convex shader source is empty")
	}

	shader, err := cr.device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "convex_shader",
		WGSL:  convexShaderSource,
	})
	if err != nil {
		return fmt.Errorf("compile convex shader: %w", err)
	}
	cr.shader = shader

	uniformLayout, err := cr.device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "convex_uniform_layout",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
				Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: sdfRenderUniformSize},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("create convex uniform layout: %w", err)
	}
	cr.uniformLayout = uniformLayout

	clipLayout := cr.clipBindLayout
	hasClip := clipLayout != nil
	if clipLayout == nil {
		if cr.defaultClipBindLayout == nil {
			layout, err := createClipBindGroupLayout(cr.device, "convex_default_clip_layout")
			if err != nil {
				return fmt.Errorf("create default clip layout: %w", err)
			}
			cr.defaultClipBindLayout = layout
		}
		clipLayout = cr.defaultClipBindLayout
	}
	if cr.maskBindLayout == nil {
		layout, err := createMaskBindGroupLayout(cr.device, "convex_mask_layout")
		if err != nil {
			return fmt.Errorf("create mask layout: %w", err)
		}
		cr.maskBindLayout = layout
		cr.maskLayoutOwned = true
	}
	pipeLayout, err := cr.device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label:            "convex_pipe_layout",
		BindGroupLayouts: []*webgpu.BindGroupLayout{cr.uniformLayout, clipLayout, cr.maskBindLayout},
	})
	if err != nil {
		return fmt.Errorf("create convex pipeline layout: %w", err)
	}
	cr.pipeLayoutHasClip = hasClip
	cr.pipeLayout = pipeLayout

	premulBlend := types.BlendStatePremultiplied()
	pipeline, err := cr.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "convex_pipeline",
		Layout: cr.pipeLayout,
		Vertex: webgpu.VertexState{
			Module:     cr.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    convexVertexLayout(),
		},
		Fragment: &webgpu.FragmentState{
			Module:     cr.shader,
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
		Multisample: multisampleState(cr.sampleCount),
	})
	if err != nil {
		return fmt.Errorf("create convex pipeline: %w", err)
	}
	cr.pipeline = pipeline

	return nil
}

// destroyPipeline releases all pipeline resources in reverse creation order.
func (cr *ConvexRenderer) destroyPipeline() {
	if cr.device == nil {
		return
	}
	if cr.pipelineWithDepthClip != nil {
		cr.pipelineWithDepthClip.Release()
		cr.pipelineWithDepthClip = nil
	}
	if cr.pipelineWithStencil != nil {
		cr.pipelineWithStencil.Release()
		cr.pipelineWithStencil = nil
	}
	for mode, pipe := range cr.blendPipelinesWithStencil {
		if pipe != nil {
			pipe.Release()
		}
		delete(cr.blendPipelinesWithStencil, mode)
	}
	if cr.pipeline != nil {
		cr.pipeline.Release()
		cr.pipeline = nil
	}
	if cr.pipeLayout != nil {
		cr.pipeLayout.Release()
		cr.pipeLayout = nil
		cr.pipeLayoutHasClip = false
	}
	if cr.defaultClipBindLayout != nil {
		cr.defaultClipBindLayout.Release()
		cr.defaultClipBindLayout = nil
	}
	if cr.maskLayoutOwned && cr.maskBindLayout != nil {
		cr.maskBindLayout.Release()
	}
	cr.maskBindLayout = nil
	cr.maskLayoutOwned = false
	if cr.uniformLayout != nil {
		cr.uniformLayout.Release()
		cr.uniformLayout = nil
	}
	if cr.shader != nil {
		cr.shader.Release()
		cr.shader = nil
	}
}

// convexFrameResources holds per-frame GPU resources for convex rendering.
// convexDrawRange is a contiguous vertex sub-range sharing one blend mode.
type convexDrawRange struct {
	firstVertex uint32
	vertCount   uint32
	blendMode   render.BlendMode
}

type convexFrameResources struct {
	vertBuf     *webgpu.Buffer
	uniformBuf  *webgpu.Buffer
	bindGroup   *webgpu.BindGroup
	vertCount   uint32
	firstVertex uint32 // offset into shared vertex buffer (for scissor group sub-ranges)
	// ranges groups consecutive vertices by blend mode. When empty, a single
	// SourceOver draw of [firstVertex, vertCount) is used (legacy path).
	ranges []convexDrawRange
}

func (r *convexFrameResources) destroy() {
	if r.bindGroup != nil {
		r.bindGroup.Release()
	}
	if r.uniformBuf != nil {
		r.uniformBuf.Release()
	}
	if r.vertBuf != nil {
		r.vertBuf.Release()
	}
}

// convexVertexLayout returns the vertex buffer layout for the convex pipeline.
func convexVertexLayout() []types.VertexBufferLayout {
	return []types.VertexBufferLayout{
		{
			ArrayStride: convexVertexStride,
			StepMode:    types.VertexStepModeVertex,
			Attributes: []types.VertexAttribute{
				{Format: types.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},  // position
				{Format: types.VertexFormatFloat32, Offset: 8, ShaderLocation: 1},    // coverage
				{Format: types.VertexFormatFloat32x4, Offset: 12, ShaderLocation: 2}, // color
			},
		},
	}
}

// BuildConvexVertices generates vertex data for all convex polygon draw commands.
// For each polygon, interior fan triangles (coverage=1.0) are generated from
// the centroid, followed by AA fringe strips (coverage ramping 1.0 to 0.0)
// along each edge.
//
// Each polygon with N edges produces:
//   - N interior triangles (3N vertices)
//   - N AA fringe quads = 2N fringe triangles (6N vertices)
//   - Total: 9N vertices per polygon
func BuildConvexVertices(commands []ConvexDrawCommand) []byte {
	_, data := buildConvexVerticesReuse(commands, nil)
	return data
}

// buildConvexVerticesReuse generates vertex data into the provided staging
// buffer, growing it if necessary. Returns the (possibly reallocated) staging
// buffer and the slice of valid vertex data.
func buildConvexVerticesReuse(commands []ConvexDrawCommand, staging []byte) ([]byte, []byte) { //nolint:funlen // vertex generation loop is a single cohesive unit
	totalVerts := 0
	for i := range commands {
		totalVerts += convexCmdVertexCount(&commands[i])
	}
	if totalVerts == 0 {
		return staging, nil
	}

	needed := totalVerts * convexVertexStride
	if cap(staging) < needed {
		staging = make([]byte, needed)
	} else {
		staging = staging[:needed]
	}
	buf := staging
	offset := 0

	for i := range commands {
		cmd := &commands[i]
		n := len(cmd.Points)
		if n < 3 {
			continue
		}

		// Dense mesh / drawVertices: emit solid triangles without AA fringe.
		if cmd.TriangleList || cmd.SkipAA {
			useVC := len(cmd.VertexColors) == n
			color := cmd.Color
			if cmd.TriangleList {
				// Independent tris: walk groups of 3 (Skia drawVertices list).
				for j := 0; j+2 < n; j += 3 {
					c0, c1, c2 := color, color, color
					if useVC {
						c0, c1, c2 = cmd.VertexColors[j], cmd.VertexColors[j+1], cmd.VertexColors[j+2]
					}
					writeConvexVertex(buf[offset:], float32(cmd.Points[j].X), float32(cmd.Points[j].Y), 1.0, c0)
					offset += convexVertexStride
					writeConvexVertex(buf[offset:], float32(cmd.Points[j+1].X), float32(cmd.Points[j+1].Y), 1.0, c1)
					offset += convexVertexStride
					writeConvexVertex(buf[offset:], float32(cmd.Points[j+2].X), float32(cmd.Points[j+2].Y), 1.0, c2)
					offset += convexVertexStride
				}
				continue
			}
			if n == 3 {
				c0, c1, c2 := color, color, color
				if useVC {
					c0, c1, c2 = cmd.VertexColors[0], cmd.VertexColors[1], cmd.VertexColors[2]
				}
				writeConvexVertex(buf[offset:], float32(cmd.Points[0].X), float32(cmd.Points[0].Y), 1.0, c0)
				offset += convexVertexStride
				writeConvexVertex(buf[offset:], float32(cmd.Points[1].X), float32(cmd.Points[1].Y), 1.0, c1)
				offset += convexVertexStride
				writeConvexVertex(buf[offset:], float32(cmd.Points[2].X), float32(cmd.Points[2].Y), 1.0, c2)
				offset += convexVertexStride
				continue
			}
			// Fan without fringe for n>3.
			c0 := color
			if useVC {
				c0 = cmd.VertexColors[0]
			}
			for j := 1; j+1 < n; j++ {
				c1, c2 := color, color
				if useVC {
					c1 = cmd.VertexColors[j]
					c2 = cmd.VertexColors[j+1]
				}
				writeConvexVertex(buf[offset:], float32(cmd.Points[0].X), float32(cmd.Points[0].Y), 1.0, c0)
				offset += convexVertexStride
				writeConvexVertex(buf[offset:], float32(cmd.Points[j].X), float32(cmd.Points[j].Y), 1.0, c1)
				offset += convexVertexStride
				writeConvexVertex(buf[offset:], float32(cmd.Points[j+1].X), float32(cmd.Points[j+1].Y), 1.0, c2)
				offset += convexVertexStride
			}
			continue
		}

		// Compute centroid.
		var cx, cy float64
		for _, p := range cmd.Points {
			cx += p.X
			cy += p.Y
		}
		cx /= float64(n)
		cy /= float64(n)
		centroidX := float32(cx)
		centroidY := float32(cy)

		useVC := len(cmd.VertexColors) == n
		color := cmd.Color
		var cCentroid [4]float32
		if useVC {
			if cmd.HasCentroidColor {
				cCentroid = cmd.CentroidColor
			} else {
				var ar, ag, ab, aa float32
				for _, vc := range cmd.VertexColors {
					ar += vc[0]
					ag += vc[1]
					ab += vc[2]
					aa += vc[3]
				}
				inv := float32(1) / float32(n)
				cCentroid = [4]float32{ar * inv, ag * inv, ab * inv, aa * inv}
			}
		} else {
			cCentroid = color
		}
		// AA fringe uses a single color; prefer centroid average when Gouraud.
		fringeColor := color
		if useVC {
			fringeColor = cCentroid
		}

		for j := 0; j < n; j++ {
			v0 := cmd.Points[j]
			v1 := cmd.Points[(j+1)%n]

			v0x := float32(v0.X)
			v0y := float32(v0.Y)
			v1x := float32(v1.X)
			v1y := float32(v1.Y)

			c0, c1 := color, color
			if useVC {
				c0 = cmd.VertexColors[j]
				c1 = cmd.VertexColors[(j+1)%n]
			}

			// Interior fan triangle: centroid, v0, v1 (Gouraud when VertexColors set).
			writeConvexVertex(buf[offset:], centroidX, centroidY, 1.0, cCentroid)
			offset += convexVertexStride
			writeConvexVertex(buf[offset:], v0x, v0y, 1.0, c0)
			offset += convexVertexStride
			writeConvexVertex(buf[offset:], v1x, v1y, 1.0, c1)
			offset += convexVertexStride

			// AA fringe: outward expansion along edge normal.
			// Edge direction.
			edx := v1x - v0x
			edy := v1y - v0y
			edgeLen := float32(math.Sqrt(float64(edx*edx + edy*edy)))
			if edgeLen < 1e-8 {
				// Degenerate edge; emit degenerate fringe triangles.
				writeConvexVertex(buf[offset:], v0x, v0y, 1.0, fringeColor)
				offset += convexVertexStride
				writeConvexVertex(buf[offset:], v1x, v1y, 1.0, fringeColor)
				offset += convexVertexStride
				writeConvexVertex(buf[offset:], v0x, v0y, 0.0, fringeColor)
				offset += convexVertexStride

				writeConvexVertex(buf[offset:], v1x, v1y, 1.0, fringeColor)
				offset += convexVertexStride
				writeConvexVertex(buf[offset:], v1x, v1y, 0.0, fringeColor)
				offset += convexVertexStride
				writeConvexVertex(buf[offset:], v0x, v0y, 0.0, fringeColor)
				offset += convexVertexStride
				continue
			}

			// Outward normal (perpendicular to edge, pointing outward).
			// For a CCW polygon, the outward normal of edge (dx, dy) is (dy, -dx).
			// For CW, it would be (-dy, dx). We use the centroid to determine direction.
			nx := edy / edgeLen
			ny := -edx / edgeLen

			// Ensure normal points outward (away from centroid).
			// Midpoint of edge.
			midX := (v0x + v1x) * 0.5
			midY := (v0y + v1y) * 0.5
			// Vector from centroid to midpoint.
			toCentroidX := midX - centroidX
			toCentroidY := midY - centroidY
			// Dot product: if normal points toward centroid, flip it.
			if nx*toCentroidX+ny*toCentroidY < 0 {
				nx = -nx
				ny = -ny
			}

			// Expanded vertices (0.5px outward).
			expand := float32(convexAAExpand)
			v0ox := v0x + nx*expand
			v0oy := v0y + ny*expand
			v1ox := v1x + nx*expand
			v1oy := v1y + ny*expand

			// Fringe quad: two triangles.
			// Triangle 1: v0, v1, v0_outer.
			writeConvexVertex(buf[offset:], v0x, v0y, 1.0, fringeColor)
			offset += convexVertexStride
			writeConvexVertex(buf[offset:], v1x, v1y, 1.0, fringeColor)
			offset += convexVertexStride
			writeConvexVertex(buf[offset:], v0ox, v0oy, 0.0, fringeColor)
			offset += convexVertexStride

			// Triangle 2: v1, v1_outer, v0_outer.
			writeConvexVertex(buf[offset:], v1x, v1y, 1.0, fringeColor)
			offset += convexVertexStride
			writeConvexVertex(buf[offset:], v1ox, v1oy, 0.0, fringeColor)
			offset += convexVertexStride
			writeConvexVertex(buf[offset:], v0ox, v0oy, 0.0, fringeColor)
			offset += convexVertexStride
		}
	}

	return staging, buf[:offset]
}

// writeConvexVertex writes a single convex vertex into the buffer.
// Layout: position (vec2<f32>) + coverage (f32) + color (vec4<f32>) = 28 bytes.
func writeConvexVertex(buf []byte, px, py, coverage float32, color [4]float32) {
	binary.LittleEndian.PutUint32(buf[0:4], math.Float32bits(px))
	binary.LittleEndian.PutUint32(buf[4:8], math.Float32bits(py))
	binary.LittleEndian.PutUint32(buf[8:12], math.Float32bits(coverage))
	binary.LittleEndian.PutUint32(buf[12:16], math.Float32bits(color[0]))
	binary.LittleEndian.PutUint32(buf[16:20], math.Float32bits(color[1]))
	binary.LittleEndian.PutUint32(buf[20:24], math.Float32bits(color[2]))
	binary.LittleEndian.PutUint32(buf[24:28], math.Float32bits(color[3]))
}

// convexVertexCount returns the total vertex count for the given commands.

// buildConvexBlendRanges groups consecutive convex commands that share a blend
// mode into vertex ranges suitable for multi-draw with pipeline switches.
// baseFirstVertex is the absolute firstVertex of the first command in commands.
func buildConvexBlendRanges(commands []ConvexDrawCommand, baseFirstVertex uint32) []convexDrawRange {
	if len(commands) == 0 {
		return nil
	}
	var ranges []convexDrawRange
	var cur *convexDrawRange
	first := baseFirstVertex
	for i := range commands {
		n := convexVertexCount(commands[i : i+1])
		if n == 0 {
			continue
		}
		mode := commands[i].BlendMode
		if cur == nil || cur.blendMode != mode {
			ranges = append(ranges, convexDrawRange{
				firstVertex: first,
				vertCount:   n,
				blendMode:   mode,
			})
			cur = &ranges[len(ranges)-1]
		} else {
			cur.vertCount += n
		}
		first += n
	}
	return ranges
}

func convexCmdVertexCount(cmd *ConvexDrawCommand) int {
	n := len(cmd.Points)
	if n < 3 {
		return 0
	}
	if cmd.TriangleList {
		// Independent triangles: floor(n/3)*3 solid verts.
		return (n / 3) * 3
	}
	if cmd.SkipAA {
		// Single convex solid mesh: one triangle per 3 points, no fringe.
		// For n>3 solid path still fans without fringe: (n-2) tris * 3 verts.
		if n == 3 {
			return 3
		}
		return (n - 2) * 3
	}
	return n * 9
}

func convexVertexCount(commands []ConvexDrawCommand) uint32 {
	var total uint32
	for i := range commands {
		total += uint32(convexCmdVertexCount(&commands[i])) //nolint:gosec
	}
	return total
}
