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
	"hash/fnv"
	"math"
	"sync"
	"time"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/clip"
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
//   - Where clip drawn: 0.0 >= 0.0 → PASS
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
//	or use the Context CPU clip stack.
//
// Architecture:
//
//	ScissorGroup.ClipPath → FanTessellator → stencil fill + cover-to-depth
//	  Phase 1: reuses stencil fill pipeline (IncrWrap/DecrWrap, no depth write)
//	  Phase 2: depthCoverPipeline (stencil NotEqual, depth write, stencil zero)
type DepthClipPipeline struct {
	device      hal.Device
	queue       hal.Queue
	sampleCount uint32 // MSAA sample count (4 or 1), from GPUShared

	// shader is the vertex/fragment shader for the cover-to-depth pass.
	// Reuses the same depth_clip.wgsl (vertex: NDC transform, Z=0.0; fragment: no-op).
	shader hal.ShaderModule

	uniformBGL hal.BindGroupLayout

	// pipeLayout is the pipeline layout for the stencil fill phase (uniform only).
	pipeLayout hal.PipelineLayout

	// stencilFillPipeline performs Phase 1: fan triangles → stencil buffer.
	// Non-zero winding: front IncrementWrap, back DecrementWrap.
	// DepthWriteEnabled=false, ColorWriteMask=None.
	stencilFillPipeline hal.RenderPipeline

	// stencilExpandPipeline (sampleCount==1 only) performs the geometric
	// stencil expansion half of GPU-CLIP-003a-AA: it paints the analytic-AA
	// exterior band mesh (bandVerts, stride-12 x,y,d) with stencil Replace(1)
	// so the depth-test pass region covers the same AA fringe that the R8
	// coverage mask modulates. Without it, the binary fan-triangle stencil
	// edge is tighter than the mask fringe: boundary pixels are rejected by
	// the depth test before the mask can fade them, leaving aliased edges.
	// DepthWriteEnabled=false, ColorWriteMask=None.
	stencilExpandPipeline hal.RenderPipeline

	// depthCoverPipeline performs Phase 2: cover quad → depth write.
	// StencilCompare=NotEqual(0), DepthCompare=Always, DepthWriteEnabled=true.
	// StencilPassOp=Zero (cleanup for Tier 2b). ColorWriteMask=None.
	depthCoverPipeline hal.RenderPipeline

	tessellator  *FanTessellator
	uniformBuf   hal.Buffer
	uniformW     uint32
	uniformH     uint32
	bindGroup    hal.BindGroup
	vertBuf      hal.Buffer
	vertBufCap   uint64
	coverBuf     hal.Buffer // vertex buffer for cover quad (6 vertices)
	coverBufCap  uint64     // capacity of cover buffer in bytes
	vertexStaged []byte     // CPU staging buffer for vertex data

	// geomCache shares one tessellated fan/cover/band vertex set across all
	// groups that clip to the same path in a frame. R7 splits one viewport
	// clip into ~23 scissor groups; before this cache each group tessellated
	// the path and uploaded its own buffers (BuildClipResources was ~4.3s of
	// a 60s run). Entries are released after frame encoding via
	// releaseFrameGeom, so they cannot outlive an in-flight command buffer.
	geomMu    sync.Mutex
	geomCache map[uint64]*depthClipGeom

	// maskCache reuses the full-frame R8 coverage mask for identical clip
	// paths. A scene can split one clip into many scissor groups (R7: ~23
	// groups sharing the 620x656 viewport clip). Without reuse, each group
	// rasterized and uploaded its own full-frame mask every frame, turning
	// one visual clip into ~140ms of raster work per frame. Entries are
	// read-only after upload and shared by groupResources; DepthClipResources
	// marks them non-owned so Release does not destroy a shared mask.
	maskCacheMu    sync.Mutex
	maskCache      []*depthMaskEntry
	maskCacheBytes int
}

// depthMaskEntry caches one uploaded coverage mask. All fields are owned by
// the pipeline's maskCache; group resources borrow them until Release.
type depthMaskEntry struct {
	key      uint64
	tex      hal.Texture
	view     hal.TextureView
	bg       hal.BindGroup
	bytes    int
	lastUsed time.Time
}

// depthClipGeom is the frame-local tessellation for one clip path. All groups
// using the same path borrow these buffers; the pipeline owns them until
// releaseFrameGeom runs after the frame's command buffer has been submitted.
type depthClipGeom struct {
	vertBuf    hal.Buffer
	coverBuf   hal.Buffer
	bandBuf    hal.Buffer
	vertCount  uint32
	coverCount uint32
	bandCount  uint32
}

var (
	// maskCacheTTL bounds how long an unused mask may keep GPU memory. It is
	// far longer than the raster pipeline depth, so an in-flight frame can
	// never reference an entry we evict.
	maskCacheTTL = render.DefaultBudget(render.BudgetClipMask).TTL
	// maskCacheMaxBytes/maxEntries bound the cache; 16 MiB covers ~17
	// full-HD R8 masks. R7's shared viewport clip needs one entry.
	// P2.4b: values come from the declaration table (render/budget.go).
	maskCacheMaxBytes   = render.DefaultBudget(render.BudgetClipMask).MaxBytes
	maskCacheMaxEntries = render.DefaultBudget(render.BudgetClipMask).MaxEntries
)

// NewDepthClipPipeline creates a new depth clip pipeline for the given device.
// The pipelines are not created until ensurePipeline() is called.
func NewDepthClipPipeline(device hal.Device, queue hal.Queue, sampleCount uint32) *DepthClipPipeline {
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
	shader, err := p.device.CreateShaderModule(&hal.ShaderModuleDescriptor{
		Label: "depth_clip_shader",
		WGSL:  depthClipShaderSource,
	})
	if err != nil {
		return fmt.Errorf("compile depth clip shader: %w", err)
	}
	p.shader = shader

	// Bind group layout: one uniform buffer at group(0) binding(0).
	bgl, err := p.device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
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

	pipeLayout, err := p.device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "depth_clip_pipe_layout",
		BindGroupLayouts: []hal.BindGroupLayout{p.uniformBGL},
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
	stencilFillPipeline, err := p.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "depth_clip_stencil_fill_pipeline",
		Layout: p.pipeLayout,
		Vertex: hal.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    vertexBufLayout,
		},
		Fragment: &hal.FragmentState{
			Module:     p.shader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{
					Format:    types.TextureFormatBGRA8Unorm,
					WriteMask: types.ColorWriteMaskNone,
				},
			},
		},
		DepthStencil: &hal.DepthStencilState{
			Format:            types.TextureFormatDepth24PlusStencil8,
			DepthWriteEnabled: false,                       // don't write depth in Phase 1
			DepthCompare:      types.CompareFunctionAlways, // pass all depth tests
			StencilFront: hal.StencilFaceState{
				Compare:     types.CompareFunctionAlways,
				FailOp:      types.StencilOperationKeep,
				DepthFailOp: types.StencilOperationKeep,
				PassOp:      types.StencilOperationIncrementWrap,
			},
			StencilBack: hal.StencilFaceState{
				Compare:     types.CompareFunctionAlways,
				FailOp:      types.StencilOperationKeep,
				DepthFailOp: types.StencilOperationKeep,
				PassOp:      types.StencilOperationDecrementWrap,
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

	// --- sampleCount==1 stencil-expansion pipeline (GPU-CLIP-003a-AA) ---
	//
	// The binary fan stencil edge is tighter than the R8 coverage mask's AA
	// fringe, so edge pixels get rejected by the depth test before the mask
	// can fade them. This pipeline paints the exterior band mesh (stride-12
	// x,y,d triples; d ignored — shader reads x,y only) with stencil
	// Replace(1), widening the depth-pass region to exactly the mask fringe.
	// Only created when sampleCount==1; guard in RecordDraw.
	aaVertexBufLayout := []types.VertexBufferLayout{
		{
			ArrayStride: aaVertexStride, // 12 bytes (x,y + signed edge dist)
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
	if p.sampleCount == 1 {
		stencilExpandPipeline, err := p.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
			Label:  "depth_clip_stencil_expand_pipeline",
			Layout: p.pipeLayout,
			Vertex: hal.VertexState{
				Module:     p.shader,
				EntryPoint: shaderEntryVS,
				Buffers:    aaVertexBufLayout,
			},
			Fragment: &hal.FragmentState{
				Module:     p.shader,
				EntryPoint: shaderEntryFS,
				Targets: []types.ColorTargetState{
					{
						Format:    types.TextureFormatBGRA8Unorm,
						WriteMask: types.ColorWriteMaskNone,
					},
				},
			},
			DepthStencil: &hal.DepthStencilState{
				Format:            types.TextureFormatDepth24PlusStencil8,
				DepthWriteEnabled: false,
				DepthCompare:      types.CompareFunctionAlways,
				StencilFront: hal.StencilFaceState{
					Compare:     types.CompareFunctionAlways,
					FailOp:      types.StencilOperationKeep,
					DepthFailOp: types.StencilOperationKeep,
					PassOp:      types.StencilOperationReplace,
				},
				StencilBack: hal.StencilFaceState{
					Compare:     types.CompareFunctionAlways,
					FailOp:      types.StencilOperationKeep,
					DepthFailOp: types.StencilOperationKeep,
					PassOp:      types.StencilOperationReplace,
				},
				StencilReadMask:  0xFF,
				StencilWriteMask: 0xFF,
			},
			Multisample: multisample,
			Primitive:   primitive,
		})
		if err != nil {
			return fmt.Errorf("create depth clip stencil expand pipeline: %w", err)
		}
		p.stencilExpandPipeline = stencilExpandPipeline
	}

	// --- Phase 2 pipeline: Cover quad → depth write ---
	//
	// Draws a bounding box quad. Only pixels where stencil != 0 (inside clip)
	// pass the stencil test. Those pixels get depth = 0.0 written.
	// StencilPassOp=Zero resets stencil to 0, cleaning up for Tier 2b.
	// No color output.
	depthCoverPipeline, err := p.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "depth_clip_cover_pipeline",
		Layout: p.pipeLayout,
		Vertex: hal.VertexState{
			Module:     p.shader,
			EntryPoint: shaderEntryVS,
			Buffers:    vertexBufLayout,
		},
		Fragment: &hal.FragmentState{
			Module:     p.shader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{
					Format:    types.TextureFormatBGRA8Unorm,
					WriteMask: types.ColorWriteMaskNone, // no color output
				},
			},
		},
		DepthStencil: &hal.DepthStencilState{
			Format:            types.TextureFormatDepth24PlusStencil8,
			DepthWriteEnabled: true,                        // write depth Z=0.0
			DepthCompare:      types.CompareFunctionAlways, // always pass depth test
			StencilFront: hal.StencilFaceState{
				Compare:     types.CompareFunctionNotEqual, // only where stencil != 0
				FailOp:      types.StencilOperationKeep,    // outside clip: keep stencil (already 0)
				DepthFailOp: types.StencilOperationKeep,    // depth always passes, never hit
				PassOp:      types.StencilOperationZero,    // reset stencil after use
			},
			StencilBack: hal.StencilFaceState{
				Compare:     types.CompareFunctionNotEqual,
				FailOp:      types.StencilOperationKeep,
				DepthFailOp: types.StencilOperationKeep,
				PassOp:      types.StencilOperationZero,
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
	vertBuf    hal.Buffer    // fan triangle vertices for stencil fill
	coverBuf   hal.Buffer    // bounding box quad vertices for cover pass
	bindGroup  hal.BindGroup // uniform bind group (viewport)
	vertCount  uint32        // number of fan vertices (Phase 1)
	coverCount uint32        // number of cover quad vertices (Phase 2, always 6)
	owned      bool          // if true, mask fields must be released
	// geomShared means vertBuf/coverBuf/bandBuf are borrowed from
	// DepthClipPipeline.geomCache. Release must not destroy them; the
	// pipeline drops the whole frame cache after encoding.
	geomShared bool

	// maskTex / maskView / maskBG hold the analytic-AA coverage mask for the
	// clip path when rendering at sampleCount==1 (Skia kCoverage / Flutter
	// ClipMask pattern). The stencil+depth phases are binary at 1x — a pixel
	// either passes the depth test or not — so the clip boundary itself
	// aliases. The R8 mask (full-frame coverage, 0..255 with an AA fringe
	// coverage. Nil at sampleCount>1 (MSAA already provides sub-sample edge
	// coverage through the depth test).
	maskTex  hal.Texture
	maskView hal.TextureView
	maskBG   hal.BindGroup
	// maskShared means the texture/view/bind group are borrowed from
	// DepthClipPipeline.maskCache. Release must not destroy them; the cache
	// owns their lifetime.
	maskShared bool

	// bandBuf / bandCount hold the analytic-AA exterior fringe mesh
	// (stride-12 x,y,d triples) for the stencil-expansion pass at
	// sampleCount==1: the binary fan stencil edge is tighter than the mask
	// fringe, so we paint this band with stencil Replace(1) to widen the
	// depth-pass region to exactly the mask's AA band. Nil when sampleCount>1.
	bandBuf   hal.Buffer
	bandCount uint32
}

// Release frees per-call GPU buffers if owned.
func (r *DepthClipResources) Release() {
	if r == nil || !r.owned {
		return
	}
	if !r.geomShared {
		if r.vertBuf != nil {
			r.vertBuf.Destroy()
		}
		if r.coverBuf != nil {
			r.coverBuf.Destroy()
		}
		if r.bandBuf != nil {
			r.bandBuf.Destroy()
		}
	}
	r.vertBuf = nil
	r.coverBuf = nil
	r.bandBuf = nil
	r.bandCount = 0
	if r.maskBG != nil {
		if !r.maskShared {
			r.maskBG.Destroy()
		}
		r.maskBG = nil
	}
	if r.maskView != nil {
		if !r.maskShared {
			r.maskView.Destroy()
		}
		r.maskView = nil
	}
	if r.maskTex != nil {
		if !r.maskShared {
			r.maskTex.Destroy()
		}
		r.maskTex = nil
	}
	r.maskShared = false
}

// clipPathCoverageMask rasterizes the clip path into a full-frame R8 coverage
// mask using the same analytic-AA scanline algorithm as the CPU reference
// (render/internal/clip.MaskClipper). fillRule selects how contour overlaps
// resolve (winding/even-odd). Pixels inside the path are 255, outside
// are 0, and the boundary band carries a smooth coverage gradient — this is
// what the depth test alone cannot express at sampleCount==1.
//
// Returns a w*h byte slice (row-major) or nil when the path is empty.
func clipPathCoverageMask(clipPath *render.Path, fillRule render.FillRule, w, h int) []byte {
	if clipPath == nil || clipPath.NumVerbs() == 0 {
		return nil
	}
	bb := clipPath.Bounds()
	if bb.Empty() {
		return nil
	}

	// Bounds expand 1px around the path bbox so the AA fringe (±0.5px,
	// MaskClipper's 4x Y-supersampling) is fully captured. MaskClipper
	// rasterizes in local coordinates offset by the bounds origin.
	const pad = 1.0
	rect := clip.Rect{
		X: float64(bb.Min.X) - pad,
		Y: float64(bb.Min.Y) - pad,
		W: float64(bb.Dx()) + 2*pad,
		H: float64(bb.Dy()) + 2*pad,
	}

	// render.PathVerb and clip.PathVerb share the same byte values; convert
	// so the CPU AA rasterizer can be reused directly.
	verbs := clipPath.Verbs()
	clipVerbs := make([]clip.PathVerb, len(verbs))
	for i, v := range verbs {
		clipVerbs[i] = clip.PathVerb(v)
	}

	mc, err := clip.NewMaskClipper(clipVerbs, clipPath.Coords(), rect, true, clip.FillRule(fillRule))
	if err != nil {
		return nil
	}
	m := mc.Mask()
	mw, mh := m.Width(), m.Height()
	if mw <= 0 || mh <= 0 {
		return nil
	}

	mask := make([]byte, w*h) // outside bbox → 0 coverage (depth test rejects anyway)
	offX := int(rect.X)
	offY := int(rect.Y)
	// The mask is a local Gray8 buffer; read its backing bytes once instead of
	// calling GetRGBA per pixel. GetRGBA takes an RWMutex for every pixel via
	// PixelBytes, which turned a 1200x800 full-frame mask into ~1M lock/unlock
	// pairs (R7 measured 1fps; the raster thread was stuck here). The buffer
	// cannot be Disposed concurrently because MaskClipper is local to this
	// call, so the bulk read is safe and byte-identical to GetRGBA for Gray8.
	src := m.Data()
	stride := m.Stride()
	if src == nil || stride < mw || len(src) < (mh-1)*stride+mw {
		return nil
	}
	// Clip the mask bbox to the destination and copy whole row spans. R7's
	// clip is fully inside the surface, so this is one memmove per row rather
	// than a bounds-checked byte loop over 640x658 pixels.
	x0, x1 := max(0, offX), min(w, offX+mw)
	y0, y1 := max(0, offY), min(h, offY+mh)
	for y := y0; y < y1; y++ {
		srcRow := (y - offY) * stride
		dstRow := y * w
		copy(mask[dstRow+x0:dstRow+x1], src[srcRow+x0-offX:srcRow+x1-offX])
	}
	return mask
}

// BuildClipMask ensures the analytic-AA coverage mask for the clip path is
// uploaded as a full-frame R8 texture + bind group on res. The mask carries
// the clip's fill rule so even-odd holes are carved out of the winding
// stencil region. This is the sampleCount==1 half of GPU-CLIP-003a-AA: the
// binary stencil/depth gate cannot express fractional edge coverage, so
// content fragments multiply by this mask.
//
// Identical clip paths share one cached mask texture/bind group. A scene can
// split a single visual clip into many scissor groups; rasterizing and
// uploading a fresh 620x656 mask for each group cost R7 ~23 masks/frame and
// capped it at 8fps. Skia's clip mask cache has the same shape: the clip
// element owns the mask, all draws under it borrow it.
//
// Skipped when sampleCount>1 (MSAA sub-sample coverage already softens the
// depth-test boundary) or when any required resource is nil.
func (p *DepthClipPipeline) BuildClipMask(
	res *DepthClipResources,
	clipPath *render.Path,
	fillRule render.FillRule,
	w, h uint32,
	maskLayout hal.BindGroupLayout,
	sampler hal.Sampler,
	uniformOn hal.Buffer,
) error {
	if res == nil || clipPath == nil || p.sampleCount > 1 {
		return nil
	}
	if maskLayout == nil || sampler == nil || uniformOn == nil {
		return nil
	}

	key := clipPathCoverageMaskKey(clipPath, fillRule, int(w), int(h))
	now := time.Now()

	p.maskCacheMu.Lock()
	if entry := p.lookupMaskLocked(key, now); entry != nil {
		p.installMaskLocked(res, entry)
		p.maskCacheMu.Unlock()
		return nil
	}
	p.maskCacheMu.Unlock()

	mask := clipPathCoverageMask(clipPath, fillRule, int(w), int(h))
	if mask == nil {
		return nil
	}
	tex, view, bg, err := p.uploadCoverageMask(mask, w, h, maskLayout, sampler, uniformOn)
	if err != nil {
		return err
	}
	entry := &depthMaskEntry{
		key: key, tex: tex, view: view, bg: bg,
		bytes: int(w) * int(h), lastUsed: now,
	}

	p.maskCacheMu.Lock()
	// Re-check: a concurrent build may have inserted the same key while we
	// rasterized. Reuse it and drop our duplicate upload.
	if existing := p.lookupMaskLocked(key, now); existing != nil {
		p.installMaskLocked(res, existing)
		p.maskCacheMu.Unlock()
		bg.Destroy()
		view.Destroy()
		tex.Destroy()
		return nil
	}
	p.pruneMaskCacheLocked(now)
	p.addMaskLocked(entry)
	p.installMaskLocked(res, entry)
	p.maskCacheMu.Unlock()
	return nil
}

// uploadCoverageMask creates the full-frame R8 mask texture, uploads the
// coverage bytes (row-padded to the backend alignment) and builds the mask
// bind group. Ownership transfers to the caller; on error all partial
// resources are destroyed.
func (p *DepthClipPipeline) uploadCoverageMask(
	mask []byte,
	w, h uint32,
	maskLayout hal.BindGroupLayout,
	sampler hal.Sampler,
	uniformOn hal.Buffer,
) (hal.Texture, hal.TextureView, hal.BindGroup, error) {
	tight := uint32(w) //nolint:gosec // bounded by caller
	aligned := alignTextureBytesPerRow(tight)
	upload := mask
	if aligned != tight {
		padded := make([]byte, int(aligned)*int(h))
		for y := 0; y < int(h); y++ {
			copy(padded[y*int(aligned):y*int(aligned)+int(tight)], mask[y*int(w):(y+1)*int(w)])
		}
		upload = padded
	}

	tex, err := p.device.CreateTexture(&hal.TextureDescriptor{
		Label:         "depth_clip_mask",
		Size:          hal.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatR8Unorm,
		Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create depth clip mask texture: %w", err)
	}
	view, err := p.device.CreateTextureView(tex, &hal.TextureViewDescriptor{
		Label: "depth_clip_mask_view", Format: types.TextureFormatR8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		tex.Destroy()
		return nil, nil, nil, fmt.Errorf("create depth clip mask view: %w", err)
	}
	if err := p.queue.WriteTexture(
		&hal.ImageCopyTexture{Texture: tex, MipLevel: 0},
		upload,
		&hal.ImageDataLayout{BytesPerRow: aligned, RowsPerImage: h}, //nolint:gosec
		&hal.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},   //nolint:gosec
	); err != nil {
		view.Destroy()
		tex.Destroy()
		return nil, nil, nil, fmt.Errorf("upload depth clip mask: %w", err)
	}

	bg, err := p.device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "depth_clip_mask_bg",
		Layout: maskLayout,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, TextureView: view},
			{Binding: 1, Sampler: sampler},
			{Binding: 2, Buffer: uniformOn, Offset: 0, Size: maskParamsSize},
		},
	})
	if err != nil {
		view.Destroy()
		tex.Destroy()
		return nil, nil, nil, fmt.Errorf("create depth clip mask bind group: %w", err)
	}
	return tex, view, bg, nil
}

// installMaskLocked points res at a cached mask. Existing non-shared mask
// resources on res are destroyed; shared ones belong to the cache.
// maskCacheMu must be held.
func (p *DepthClipPipeline) installMaskLocked(res *DepthClipResources, entry *depthMaskEntry) {
	if res == nil || entry == nil {
		return
	}
	if res.maskBG != nil {
		if !res.maskShared {
			res.maskBG.Destroy()
		}
		res.maskBG = nil
	}
	if res.maskView != nil {
		if !res.maskShared {
			res.maskView.Destroy()
		}
		res.maskView = nil
	}
	if res.maskTex != nil {
		if !res.maskShared {
			res.maskTex.Destroy()
		}
		res.maskTex = nil
	}
	res.maskTex, res.maskView, res.maskBG = entry.tex, entry.view, entry.bg
	res.maskShared = true
}

// lookupMaskLocked returns the cached mask for key and marks it recently
// used (LRU). maskCacheMu must be held.
func (p *DepthClipPipeline) lookupMaskLocked(key uint64, now time.Time) *depthMaskEntry {
	for i, entry := range p.maskCache {
		if entry.key != key {
			continue
		}
		entry.lastUsed = now
		if i != len(p.maskCache)-1 {
			copy(p.maskCache[i:], p.maskCache[i+1:])
			p.maskCache[len(p.maskCache)-1] = entry
		}
		return entry
	}
	return nil
}

// pruneMaskCacheLocked drops masks unused beyond maskCacheTTL. Capacity is
// enforced after addMaskLocked so the freshly added entry survives.
// maskCacheMu must be held.
func (p *DepthClipPipeline) pruneMaskCacheLocked(now time.Time) {
	if len(p.maskCache) == 0 {
		return
	}
	keep := p.maskCache[:0]
	for _, entry := range p.maskCache {
		if now.Sub(entry.lastUsed) > maskCacheTTL {
			p.maskCacheBytes -= entry.bytes
			p.destroyMaskLocked(entry)
			continue
		}
		keep = append(keep, entry)
	}
	p.maskCache = keep
}

// addMaskLocked appends entry and evicts least-recently-used masks until the
// cache is within its count/byte budget. The last entry is never evicted.
// maskCacheMu must be held.
func (p *DepthClipPipeline) addMaskLocked(entry *depthMaskEntry) {
	p.maskCache = append(p.maskCache, entry)
	p.maskCacheBytes += entry.bytes
	for (len(p.maskCache) > maskCacheMaxEntries || p.maskCacheBytes > int(maskCacheMaxBytes)) && len(p.maskCache) > 1 {
		victim := p.maskCache[0]
		copy(p.maskCache, p.maskCache[1:])
		p.maskCache[len(p.maskCache)-1] = nil
		p.maskCache = p.maskCache[:len(p.maskCache)-1]
		p.maskCacheBytes -= victim.bytes
		p.destroyMaskLocked(victim)
	}
}

// destroyMaskLocked releases one cached mask. maskCacheMu must be held.
func (p *DepthClipPipeline) destroyMaskLocked(entry *depthMaskEntry) {
	if entry == nil {
		return
	}
	if entry.bg != nil {
		entry.bg.Destroy()
		entry.bg = nil
	}
	if entry.view != nil {
		entry.view.Destroy()
		entry.view = nil
	}
	if entry.tex != nil {
		entry.tex.Destroy()
		entry.tex = nil
	}
}

// clipPathCoverageMaskKey hashes everything that changes mask pixels: path
// verbs/coords, fill rule and surface dimensions. FNV-1a is cheap relative to
// rasterizing, and a 64-bit collision is not a practical correctness risk for
// an internal cache.
func clipPathCoverageMaskKey(clipPath *render.Path, fillRule render.FillRule, w, h int) uint64 {
	hsh := fnv.New64a()
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(fillRule))
	hsh.Write(buf[:])
	binary.LittleEndian.PutUint64(buf[:], uint64(uint32(w))<<32|uint64(uint32(h))) //nolint:gosec
	hsh.Write(buf[:])
	verbs := clipPath.Verbs()
	binary.LittleEndian.PutUint64(buf[:], uint64(len(verbs)))
	hsh.Write(buf[:])
	for _, v := range verbs {
		hsh.Write([]byte{byte(v)})
	}
	coords := clipPath.Coords()
	binary.LittleEndian.PutUint64(buf[:], uint64(len(coords)))
	hsh.Write(buf[:])
	for _, c := range coords {
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(c))
		hsh.Write(buf[:])
	}
	return hsh.Sum64()
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
	if clipPath == nil || clipPath.NumVerbs() == 0 {
		return nil, nil //nolint:nilnil // empty clip path, nothing to draw
	}

	// Geometry depends only on the path and the fixed sample count. R7 pushes
	// the same viewport clip around every picture it replays, producing ~23
	// scissor groups per frame; sharing the tessellation drops 22 redundant
	// tessellate+upload passes per frame.
	key := clipPathCoverageMaskKey(clipPath, render.FillRuleNonZero, 0, 0)
	p.geomMu.Lock()
	geom := p.geomCache[key]
	p.geomMu.Unlock()
	if geom == nil {
		built, err := p.buildClipGeom(clipPath)
		if err != nil {
			return nil, err
		}
		if built == nil {
			return nil, nil //nolint:nilnil // empty clip path
		}
		p.geomMu.Lock()
		if existing := p.geomCache[key]; existing != nil {
			p.geomMu.Unlock()
			destroyDepthClipGeom(built)
			geom = existing
		} else {
			if p.geomCache == nil {
				p.geomCache = make(map[uint64]*depthClipGeom)
			}
			p.geomCache[key] = built
			p.geomMu.Unlock()
			geom = built
		}
	}

	// Upload uniforms (viewport dimensions). Skip the cgo write when the
	// viewport is unchanged; the bind group points at the same buffer.
	if err := p.uploadUniforms(w, h); err != nil {
		return nil, err
	}

	// Ensure bind group (uniform buffer is fixed-size for the pipeline life).
	if p.bindGroup == nil {
		bg, err := p.device.CreateBindGroup(&hal.BindGroupDescriptor{
			Label:  "depth_clip_bind",
			Layout: p.uniformBGL,
			Entries: []hal.BindGroupEntry{
				{Binding: 0, Buffer: p.uniformBuf, Offset: 0, Size: depthClipUniformSize},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("create depth clip bind group: %w", err)
		}
		p.bindGroup = bg
	}

	return &DepthClipResources{
		vertBuf:    geom.vertBuf,
		coverBuf:   geom.coverBuf,
		bindGroup:  p.bindGroup,
		vertCount:  geom.vertCount,
		coverCount: geom.coverCount,
		bandBuf:    geom.bandBuf,
		bandCount:  geom.bandCount,
		owned:      true,
		geomShared: true,
	}, nil
}

// buildClipGeom tessellates one clip path into the frame-local geometry cache.
func (p *DepthClipPipeline) buildClipGeom(clipPath *render.Path) (*depthClipGeom, error) {
	// Tessellate clip path into fan triangles.
	p.tessellator.Reset()
	vertCount := p.tessellator.TessellatePath(clipPath)
	if vertCount == 0 {
		return nil, nil //nolint:nilnil // empty clip path, nothing to draw
	}

	// sampleCount==1: also tessellate the analytic-AA exterior fringe band
	// (stride-12 x,y,d). Drives the stencil-expansion pass (RecordDraw Phase
	// 1.5) so the depth-pass region covers the same edge band the R8 coverage
	// mask modulates — without it the binary fan stencil rejects boundary
	// pixels before the mask can fade them (aliased edges).
	var bandCount int
	if p.sampleCount == 1 {
		bandCount = p.tessellator.TessellateAA(clipPath)
	}

	// Upload fan vertices (Phase 1: stencil fill) into a shared frame buffer.
	verts := p.tessellator.Vertices()
	vertBuf, err := p.device.CreateBuffer(&hal.BufferDescriptor{
		Label: "depth_clip_fan_shared",
		Size:  uint64(len(verts)) * 4, //nolint:gosec // bounded by tessellator
		Usage: types.BufferUsageVertex | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create shared fan buffer: %w", err)
	}
	fanData := make([]byte, len(verts)*4)
	for i, v := range verts {
		binary.LittleEndian.PutUint32(fanData[i*4:], math.Float32bits(v))
	}
	if wErr := p.queue.WriteBuffer(vertBuf, 0, fanData); wErr != nil {
		vertBuf.Destroy()
		return nil, fmt.Errorf("write shared fan buffer: %w", wErr)
	}

	// Cover quad vertices (Phase 2: depth write via stencil test).
	coverQuad := p.tessellator.CoverQuad()
	coverBuf, err := p.device.CreateBuffer(&hal.BufferDescriptor{
		Label: "depth_clip_cover_shared",
		Size:  12 * 4,
		Usage: types.BufferUsageVertex | types.BufferUsageCopyDst,
	})
	if err != nil {
		vertBuf.Destroy()
		return nil, fmt.Errorf("create shared cover buffer: %w", err)
	}
	coverData := make([]byte, 12*4)
	for i, v := range coverQuad {
		binary.LittleEndian.PutUint32(coverData[i*4:], math.Float32bits(v))
	}
	if wErr := p.queue.WriteBuffer(coverBuf, 0, coverData); wErr != nil {
		vertBuf.Destroy()
		coverBuf.Destroy()
		return nil, fmt.Errorf("write shared cover buffer: %w", wErr)
	}

	geom := &depthClipGeom{
		vertBuf:    vertBuf,
		coverBuf:   coverBuf,
		vertCount:  uint32(vertCount), //nolint:gosec // bounded by tessellator
		coverCount: 6,
	}
	if bandCount > 0 {
		band := p.tessellator.BandVerts()
		bandBuf, err := p.device.CreateBuffer(&hal.BufferDescriptor{
			Label: "depth_clip_band_shared",
			Size:  uint64(len(band)) * 4, //nolint:gosec // bounded by tessellator
			Usage: types.BufferUsageVertex | types.BufferUsageCopyDst,
		})
		if err != nil {
			destroyDepthClipGeom(geom)
			return nil, fmt.Errorf("create shared band buffer: %w", err)
		}
		bandData := make([]byte, len(band)*4)
		for i, v := range band {
			binary.LittleEndian.PutUint32(bandData[i*4:], math.Float32bits(v))
		}
		if wErr := p.queue.WriteBuffer(bandBuf, 0, bandData); wErr != nil {
			bandBuf.Destroy()
			destroyDepthClipGeom(geom)
			return nil, fmt.Errorf("write shared band buffer: %w", wErr)
		}
		geom.bandBuf = bandBuf
		geom.bandCount = uint32(bandCount) //nolint:gosec // bounded by tessellator
	}
	return geom, nil
}

func destroyDepthClipGeom(geom *depthClipGeom) {
	if geom == nil {
		return
	}
	if geom.vertBuf != nil {
		geom.vertBuf.Destroy()
	}
	if geom.coverBuf != nil {
		geom.coverBuf.Destroy()
	}
	if geom.bandBuf != nil {
		geom.bandBuf.Destroy()
	}
}

// releaseFrameGeom drops the frame-local tessellation cache after the frame's
// command buffer has been submitted. Group resources are released first.
func (p *DepthClipPipeline) releaseFrameGeom() {
	p.geomMu.Lock()
	cache := p.geomCache
	p.geomCache = nil
	p.geomMu.Unlock()
	for _, geom := range cache {
		destroyDepthClipGeom(geom)
	}
}

// uploadFanVertices ensures the fan vertex buffer is large enough and uploads
// the tessellated fan triangle data to the GPU.
func (p *DepthClipPipeline) uploadFanVertices() error {
	verts := p.tessellator.Vertices()
	vertBytes := uint64(len(verts)) * 4 //nolint:gosec // len(verts) bounded by tessellator

	// Ensure fan vertex buffer (grow-only).
	if p.vertBuf == nil || p.vertBufCap < vertBytes {
		if p.vertBuf != nil {
			p.vertBuf.Destroy()
		}
		newCap := vertBytes
		if newCap < 4096 {
			newCap = 4096 // minimum 4KB
		}
		buf, err := p.device.CreateBuffer(&hal.BufferDescriptor{
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
			p.coverBuf.Destroy()
		}
		buf, err := p.device.CreateBuffer(&hal.BufferDescriptor{
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
		buf, err := p.device.CreateBuffer(&hal.BufferDescriptor{
			Label: "depth_clip_uniform",
			Size:  depthClipUniformSize,
			Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
		})
		if err != nil {
			return fmt.Errorf("create depth clip uniform buffer: %w", err)
		}
		p.uniformBuf = buf
		p.uniformW, p.uniformH = 0, 0
	}
	if p.uniformW == w && p.uniformH == h {
		return nil
	}

	uniformData := make([]byte, depthClipUniformSize)
	binary.LittleEndian.PutUint32(uniformData[0:4], math.Float32bits(float32(w)))
	binary.LittleEndian.PutUint32(uniformData[4:8], math.Float32bits(float32(h)))
	// bytes 8..15 remain zero (padding).
	if err := p.queue.WriteBuffer(p.uniformBuf, 0, uniformData); err != nil {
		return fmt.Errorf("write depth clip uniforms: %w", err)
	}
	p.uniformW, p.uniformH = w, h
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
func (p *DepthClipPipeline) RecordDraw(rp hal.RenderPassEncoder, res *DepthClipResources) {
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

	// Phase 1.5 (sampleCount==1, GPU-CLIP-003a-AA): stencil expansion — paint
	// the analytic-AA exterior fringe band with stencil Replace(1). The
	// binary fan stencil edge is tighter than the R8 coverage mask's fringe;
	// without this pass boundary pixels fail the depth test before the mask
	// can fade them, leaving aliased edges. Phase 2's cover then writes
	if res.bandCount > 0 && p.stencilExpandPipeline != nil {
		clearPassBindGroups(rp)
		rp.SetPipeline(p.stencilExpandPipeline)
		rp.SetBindGroup(0, res.bindGroup, nil)
		rp.SetVertexBuffer(0, res.bandBuf, 0)
		rp.SetStencilReference(1)
		rp.Draw(res.bandCount, 1, 0, 0)
	}

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
	p.releaseFrameGeom()
	p.maskCacheMu.Lock()
	for _, entry := range p.maskCache {
		p.destroyMaskLocked(entry)
	}
	p.maskCache = nil
	p.maskCacheBytes = 0
	p.maskCacheMu.Unlock()

	if p.bindGroup != nil {
		p.bindGroup.Destroy()
		p.bindGroup = nil
	}
	if p.uniformBuf != nil {
		p.uniformBuf.Destroy()
		p.uniformBuf = nil
	}
	if p.coverBuf != nil {
		p.coverBuf.Destroy()
		p.coverBuf = nil
		p.coverBufCap = 0
	}
	if p.vertBuf != nil {
		p.vertBuf.Destroy()
		p.vertBuf = nil
		p.vertBufCap = 0
	}
	if p.depthCoverPipeline != nil {
		p.depthCoverPipeline.Destroy()
		p.depthCoverPipeline = nil
	}
	if p.stencilFillPipeline != nil {
		p.stencilFillPipeline.Destroy()
		p.stencilFillPipeline = nil
	}
	if p.stencilExpandPipeline != nil {
		p.stencilExpandPipeline.Destroy()
		p.stencilExpandPipeline = nil
	}
	if p.pipeLayout != nil {
		p.pipeLayout.Destroy()
		p.pipeLayout = nil
	}
	if p.uniformBGL != nil {
		p.uniformBGL.Destroy()
		p.uniformBGL = nil
	}
	if p.shader != nil {
		p.shader.Destroy()
		p.shader = nil
	}
}
