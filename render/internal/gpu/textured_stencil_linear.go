//go:build !nogpu

package gpu

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"sync"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

// N1 textured stencil cover for linear / radial-simple / sweep (signed) / focal gradients:
// stencil-fill path + cover FS samples 1D ramp by gradient parameter t (device space).
// One GPU pass family. Prefer retain mode (dedicated BGRA result texture +
// QueueGPUTextureDraw, no CPU readback). Readback path kept as fallback.
// Does not demote solid/span/field/convex.

const texturedStencilFillWGSL = `
struct Uni {
    viewport: vec2<f32>,
    _pad: vec2<f32>,
}
@group(0) @binding(0) var<uniform> u: Uni;

@vertex
fn vs_main(@location(0) pos: vec2<f32>) -> @builtin(position) vec4<f32> {
    let ndc_x = pos.x / u.viewport.x * 2.0 - 1.0;
    let ndc_y = 1.0 - pos.y / u.viewport.y * 2.0;
    return vec4<f32>(ndc_x, ndc_y, 0.0, 1.0);
}

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(0.0);
}
`

const texturedStencilCoverLinearWGSL = `
// mode: 0=linear, 1=radial simple, 2=sweep (signed), 3=focal radial (same packing as linearRampMask)
struct CoverUni {
    viewport: vec2<f32>,
    _pad0: vec2<f32>,
    bounds_min: vec2<f32>,
    bounds_size: vec2<f32>,
    p0: vec2<f32>,
    p1: vec2<f32>,
    inv_len2: f32,
    t_min: f32,
    inv_span: f32,
    mode: f32,
}

@group(0) @binding(0) var<uniform> u: CoverUni;
@group(0) @binding(1) var ramp_tex: texture_2d<f32>;
@group(0) @binding(2) var ramp_samp: sampler;

struct VSOut {
    @builtin(position) pos: vec4<f32>,
}

@vertex
fn vs_main(@location(0) pos: vec2<f32>) -> VSOut {
    var o: VSOut;
    let ndc_x = pos.x / u.viewport.x * 2.0 - 1.0;
    let ndc_y = 1.0 - pos.y / u.viewport.y * 2.0;
    o.pos = vec4<f32>(ndc_x, ndc_y, 0.0, 1.0);
    return o;
}

fn gradient_t(px: f32, py: f32) -> f32 {
    if (u.mode < 0.5) {
        return ((px - u.p0.x) * u.p1.x + (py - u.p0.y) * u.p1.y) * u.inv_len2;
    }
    if (u.mode < 1.5) {
        let d = length(vec2<f32>(px, py) - u.p0);
        return (d - u.p1.x) * u.p1.y;
    }
    if (u.mode < 2.5) {
        // sweep (signed): p0=center, p1=(startAngle, invSweepRange); inv may be negative.
        // Wrap matches render.normalizeAngle / linear_ramp_mask mode 2.
        let v = vec2<f32>(px, py) - u.p0;
        if (dot(v, v) < 1e-12) {
            return u.t_min;
        }
        let angle = atan2(v.y, v.x);
        var rel = angle - u.p1.x;
        let two_pi = 6.283185307179586;
        if (u.p1.y >= 0.0) {
            // Positive sweep: [0, 2π)
            rel = rel - floor(rel / two_pi) * two_pi;
        } else {
            // Negative sweep: (-2π, 0]
            rel = rel - ceil(rel / two_pi) * two_pi;
        }
        return rel * u.p1.y;
    }
    // focal radial: p0=focus, p1=center, inv_len2=endRadius
    let focus = u.p0;
    let center = u.p1;
    let end_r = u.inv_len2;
    let d = vec2<f32>(px, py) - focus;
    let f = center - focus;
    let a = dot(d, d);
    if (a < 1e-12) {
        return 0.0;
    }
    let b = -2.0 * dot(d, f);
    let c = dot(f, f) - end_r * end_r;
    let disc = b * b - 4.0 * a * c;
    if (disc < 0.0) {
        return 1.0;
    }
    let sqrt_d = sqrt(disc);
    let t1 = (-b - sqrt_d) / (2.0 * a);
    let t2 = (-b + sqrt_d) / (2.0 * a);
    var t_ray = 0.0;
    let t1p = t1 > 0.0;
    let t2p = t2 > 0.0;
    if (t1p && t2p) {
        t_ray = min(t1, t2);
    } else if (t1p) {
        t_ray = t1;
    } else if (t2p) {
        t_ray = t2;
    } else {
        return 0.0;
    }
    let point_dist = sqrt(a);
    let intersect_dist = t_ray * point_dist;
    if (intersect_dist < 1e-12) {
        return 0.0;
    }
    return point_dist / intersect_dist;
}

@fragment
fn fs_main(in: VSOut) -> @location(0) vec4<f32> {
    let u0 = (in.pos.x + 0.5) / max(u.viewport.x, 1.0);
    let v0 = (in.pos.y + 0.5) / max(u.viewport.y, 1.0);
    let px = u.bounds_min.x + u0 * u.bounds_size.x;
    let py = u.bounds_min.y + v0 * u.bounds_size.y;
    let tt = gradient_t(px, py);
    var ru = (tt - u.t_min) * u.inv_span;
    ru = clamp(ru, 0.0, 1.0);
    return textureSampleLevel(ramp_tex, ramp_samp, vec2<f32>(ru, 0.5), 0.0);
}
`

// texturedStencilLinearParams packs projection for cover FS (mode matches linearRampMask).
// Mode 0 linear: start/d/invLen2; 1 radial: center/(startR,invRD); 2 sweep: center/(start,invRange) inv may be negative;
// 3 focal: focus/center/endRadius in invLen2.
type texturedStencilLinearParams struct {
	boundsMinX, boundsMinY float32
	boundsW, boundsH       float32
	startX, startY         float32
	dX, dY                 float32
	invLen2                float32
	tMin                   float32
	invSpan                float32
	mode                   float32
}

type texturedStencilLinearCache struct {
	mu          sync.Mutex
	device      *webgpu.Device
	sampleCount uint32

	fillShader   *webgpu.ShaderModule
	coverShader  *webgpu.ShaderModule
	fillBGL      *webgpu.BindGroupLayout
	coverBGL     *webgpu.BindGroupLayout
	fillPipeLay  *webgpu.PipelineLayout
	coverPipeLay *webgpu.PipelineLayout
	nzFillPipe   *webgpu.RenderPipeline
	eoFillPipe   *webgpu.RenderPipeline
	coverPipe    *webgpu.RenderPipeline
	sampler      *webgpu.Sampler

	tex textureSet
}

func (c *texturedStencilLinearCache) release() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.destroyPipelinesLocked()
	c.tex.destroyTextures()
	c.device = nil
}

func (c *texturedStencilLinearCache) destroyPipelinesLocked() {
	if c.coverPipe != nil {
		c.coverPipe.Release()
		c.coverPipe = nil
	}
	if c.nzFillPipe != nil {
		c.nzFillPipe.Release()
		c.nzFillPipe = nil
	}
	if c.eoFillPipe != nil {
		c.eoFillPipe.Release()
		c.eoFillPipe = nil
	}
	if c.fillPipeLay != nil {
		c.fillPipeLay.Release()
		c.fillPipeLay = nil
	}
	if c.coverPipeLay != nil {
		c.coverPipeLay.Release()
		c.coverPipeLay = nil
	}
	if c.fillBGL != nil {
		c.fillBGL.Release()
		c.fillBGL = nil
	}
	if c.coverBGL != nil {
		c.coverBGL.Release()
		c.coverBGL = nil
	}
	if c.fillShader != nil {
		c.fillShader.Release()
		c.fillShader = nil
	}
	if c.coverShader != nil {
		c.coverShader.Release()
		c.coverShader = nil
	}
	if c.sampler != nil {
		c.sampler.Release()
		c.sampler = nil
	}
}

func (c *texturedStencilLinearCache) ensure(device *webgpu.Device, sampleCount uint32) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if sampleCount == 0 {
		sampleCount = 4
	}
	if c.device != nil && (c.device != device || c.sampleCount != sampleCount) {
		c.destroyPipelinesLocked()
		c.tex.destroyTextures()
	}
	c.device = device
	c.sampleCount = sampleCount
	if c.coverPipe != nil {
		return nil
	}

	fillShader, err := device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "tex_stencil_fill", WGSL: texturedStencilFillWGSL,
	})
	if err != nil {
		return fmt.Errorf("tex stencil fill shader: %w", err)
	}
	coverShader, err := device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "tex_stencil_cover_linear", WGSL: texturedStencilCoverLinearWGSL,
	})
	if err != nil {
		fillShader.Release()
		return fmt.Errorf("tex stencil cover shader: %w", err)
	}

	fillBGL, err := device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "tex_stencil_fill_bgl",
		Entries: []types.BindGroupLayoutEntry{{
			Binding: 0, Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
			Buffer: &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: 16},
		}},
	})
	if err != nil {
		fillShader.Release()
		coverShader.Release()
		return err
	}
	coverBGL, err := device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "tex_stencil_cover_bgl",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding: 0, Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
				Buffer: &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: 64},
			},
			{
				Binding: 1, Visibility: types.ShaderStageFragment,
				Texture: &types.TextureBindingLayout{
					SampleType: types.TextureSampleTypeFloat, ViewDimension: types.TextureViewDimension2D,
				},
			},
			{
				Binding: 2, Visibility: types.ShaderStageFragment,
				Sampler: &types.SamplerBindingLayout{Type: types.SamplerBindingTypeFiltering},
			},
		},
	})
	if err != nil {
		fillBGL.Release()
		fillShader.Release()
		coverShader.Release()
		return err
	}
	fillLay, err := device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label: "tex_stencil_fill_lay", BindGroupLayouts: []*webgpu.BindGroupLayout{fillBGL},
	})
	if err != nil {
		coverBGL.Release()
		fillBGL.Release()
		fillShader.Release()
		coverShader.Release()
		return err
	}
	coverLay, err := device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label: "tex_stencil_cover_lay", BindGroupLayouts: []*webgpu.BindGroupLayout{coverBGL},
	})
	if err != nil {
		fillLay.Release()
		coverBGL.Release()
		fillBGL.Release()
		fillShader.Release()
		coverShader.Release()
		return err
	}

	vbl := []types.VertexBufferLayout{{
		ArrayStride: 8,
		StepMode:    types.VertexStepModeVertex,
		Attributes: []types.VertexAttribute{{
			Format: types.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0,
		}},
	}}
	ms := multisampleState(sampleCount)
	prim := triangleListPrimitive()

	makeFill := func(label string, evenOdd bool) (*webgpu.RenderPipeline, error) {
		frontPass := webgpu.StencilOperationIncrementWrap
		backPass := webgpu.StencilOperationDecrementWrap
		writeMask := uint32(0xFF)
		if evenOdd {
			backPass = webgpu.StencilOperationIncrementWrap
			writeMask = 0x01
		}
		return device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
			Label:  label,
			Layout: fillLay,
			Vertex: webgpu.VertexState{Module: fillShader, EntryPoint: shaderEntryVS, Buffers: vbl},
			Fragment: &webgpu.FragmentState{
				Module: fillShader, EntryPoint: shaderEntryFS,
				Targets: []types.ColorTargetState{{
					Format: types.TextureFormatBGRA8Unorm, WriteMask: types.ColorWriteMaskNone,
				}},
			},
			DepthStencil: &webgpu.DepthStencilState{
				Format: types.TextureFormatDepth24PlusStencil8, DepthWriteEnabled: false,
				DepthCompare: types.CompareFunctionAlways,
				StencilFront: webgpu.StencilFaceState{
					Compare: types.CompareFunctionAlways, FailOp: webgpu.StencilOperationKeep,
					DepthFailOp: webgpu.StencilOperationKeep, PassOp: frontPass,
				},
				StencilBack: webgpu.StencilFaceState{
					Compare: types.CompareFunctionAlways, FailOp: webgpu.StencilOperationKeep,
					DepthFailOp: webgpu.StencilOperationKeep, PassOp: backPass,
				},
				StencilReadMask: 0xFF, StencilWriteMask: writeMask,
			},
			Multisample: ms,
			Primitive:   prim,
		})
	}
	nz, err := makeFill("tex_stencil_fill_nz", false)
	if err != nil {
		coverLay.Release()
		fillLay.Release()
		coverBGL.Release()
		fillBGL.Release()
		fillShader.Release()
		coverShader.Release()
		return fmt.Errorf("nz fill pipe: %w", err)
	}
	eo, err := makeFill("tex_stencil_fill_eo", true)
	if err != nil {
		nz.Release()
		coverLay.Release()
		fillLay.Release()
		coverBGL.Release()
		fillBGL.Release()
		fillShader.Release()
		coverShader.Release()
		return fmt.Errorf("eo fill pipe: %w", err)
	}

	replace := types.BlendState{
		Color: types.BlendComponent{SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorZero, Operation: types.BlendOperationAdd},
		Alpha: types.BlendComponent{SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorZero, Operation: types.BlendOperationAdd},
	}
	cover, err := device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "tex_stencil_cover_linear_pipe",
		Layout: coverLay,
		Vertex: webgpu.VertexState{Module: coverShader, EntryPoint: shaderEntryVS, Buffers: vbl},
		Fragment: &webgpu.FragmentState{
			Module: coverShader, EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{{
				Format: types.TextureFormatBGRA8Unorm, Blend: &replace, WriteMask: types.ColorWriteMaskAll,
			}},
		},
		DepthStencil: &webgpu.DepthStencilState{
			Format: types.TextureFormatDepth24PlusStencil8, DepthWriteEnabled: false,
			DepthCompare: types.CompareFunctionAlways,
			StencilFront: webgpu.StencilFaceState{
				Compare: types.CompareFunctionNotEqual, FailOp: webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep, PassOp: webgpu.StencilOperationZero,
			},
			StencilBack: webgpu.StencilFaceState{
				Compare: types.CompareFunctionNotEqual, FailOp: webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep, PassOp: webgpu.StencilOperationZero,
			},
			StencilReadMask: 0xFF, StencilWriteMask: 0xFF,
		},
		Multisample: ms,
		Primitive:   prim,
	})
	if err != nil {
		eo.Release()
		nz.Release()
		coverLay.Release()
		fillLay.Release()
		coverBGL.Release()
		fillBGL.Release()
		fillShader.Release()
		coverShader.Release()
		return fmt.Errorf("cover pipe: %w", err)
	}
	samp, err := device.CreateSampler(&webgpu.SamplerDescriptor{
		Label:        "tex_stencil_ramp_samp",
		AddressModeU: types.AddressModeClampToEdge, AddressModeV: types.AddressModeClampToEdge,
		AddressModeW: types.AddressModeClampToEdge,
		MagFilter:    types.FilterModeLinear, MinFilter: types.FilterModeLinear,
		MipmapFilter: types.MipmapFilterModeNearest, Anisotropy: 1,
	})
	if err != nil {
		cover.Release()
		eo.Release()
		nz.Release()
		coverLay.Release()
		fillLay.Release()
		coverBGL.Release()
		fillBGL.Release()
		fillShader.Release()
		coverShader.Release()
		return err
	}

	c.fillShader = fillShader
	c.coverShader = coverShader
	c.fillBGL = fillBGL
	c.coverBGL = coverBGL
	c.fillPipeLay = fillLay
	c.coverPipeLay = coverLay
	c.nzFillPipe = nz
	c.eoFillPipe = eo
	c.coverPipe = cover
	c.sampler = samp
	return nil
}

func encodeTexturedStencilCoverUniform(w, h uint32, p texturedStencilLinearParams) []byte {
	// viewport(2)+pad(2)+bounds(4)+start(2)+d(2)+inv_len2+t_min+inv_span+pad = 16 floats
	buf := make([]byte, 64)
	put := func(i int, v float32) {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	put(0, float32(w))
	put(1, float32(h))
	put(2, 0)
	put(3, 0)
	put(4, p.boundsMinX)
	put(5, p.boundsMinY)
	put(6, p.boundsW)
	put(7, p.boundsH)
	put(8, p.startX)
	put(9, p.startY)
	put(10, p.dX)
	put(11, p.dY)
	put(12, p.invLen2)
	put(13, p.tMin)
	put(14, p.invSpan)
	put(15, p.mode)
	return buf
}

// texturedStencilCoverLinear stencil-fills localPath and covers with 1D ramp.
// Returns premul RGBA (readback). Prefer texturedStencilCoverLinearRetain.
func texturedStencilCoverLinear(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *texturedStencilLinearCache,
	localPath *render.Path,
	fillRule render.FillRule,
	nw, nh int,
	ramp []byte,
	n int,
	params texturedStencilLinearParams,
	sampleCount uint32,
) ([]byte, error) {
	px, tex, view, err := texturedStencilCoverLinearEx(device, queue, cache, localPath, fillRule, nw, nh, ramp, n, params, sampleCount, false)
	if tex != nil {
		view.Release()
		tex.Release()
	}
	return px, err
}

// texturedStencilCoverLinearRetain is the no-readback path: result stays on GPU
// as BGRA TextureBinding for QueueGPUTextureDraw. Caller owns tex/view.
func texturedStencilCoverLinearRetain(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *texturedStencilLinearCache,
	localPath *render.Path,
	fillRule render.FillRule,
	nw, nh int,
	ramp []byte,
	n int,
	params texturedStencilLinearParams,
	sampleCount uint32,
) (*webgpu.Texture, *webgpu.TextureView, error) {
	px, tex, view, err := texturedStencilCoverLinearEx(device, queue, cache, localPath, fillRule, nw, nh, ramp, n, params, sampleCount, true)
	if err != nil {
		return nil, nil, err
	}
	if px == nil && tex == nil {
		return nil, nil, nil // empty / no ink
	}
	return tex, view, nil
}

// texturedStencilCoverLinearEx implements cover. retain=true keeps a dedicated
// result texture (TextureBinding); retain=false maps readback and frees it.
func texturedStencilCoverLinearEx(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *texturedStencilLinearCache,
	localPath *render.Path,
	fillRule render.FillRule,
	nw, nh int,
	ramp []byte,
	n int,
	params texturedStencilLinearParams,
	sampleCount uint32,
	retain bool,
) ([]byte, *webgpu.Texture, *webgpu.TextureView, error) {
	if device == nil || queue == nil || cache == nil || localPath == nil || n < 1 || nw <= 0 || nh <= 0 {
		return nil, nil, nil, fmt.Errorf("tex stencil: bad args")
	}
	if len(ramp) < n*4 {
		return nil, nil, nil, fmt.Errorf("tex stencil: ramp size")
	}
	if err := cache.ensure(device, sampleCount); err != nil {
		return nil, nil, nil, err
	}

	tess := NewFanTessellator()
	tess.TessellatePath(localPath)
	fan := tess.Vertices()
	if len(fan) == 0 {
		return nil, nil, nil, nil
	}
	cover := tess.CoverQuad()

	w, h := uint32(nw), uint32(nh) //nolint:gosec
	cache.mu.Lock()
	if err := cache.tex.ensureTextures(device, w, h, "tex_stencil_lin", cache.sampleCount); err != nil {
		cache.mu.Unlock()
		return nil, nil, nil, err
	}
	msaaView := cache.tex.msaaView
	stencilView := cache.tex.stencilView
	nzPipe, eoPipe, coverPipe := cache.nzFillPipe, cache.eoFillPipe, cache.coverPipe
	fillBGL, coverBGL, sampler := cache.fillBGL, cache.coverBGL, cache.sampler
	cache.mu.Unlock()

	// Dedicated result texture (TextureBinding) so we can QueueGPUTextureDraw
	// without sampling the shared resolve target (which is reused per frame).
	outTex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "tex_stencil_lin_result",
		Size:          webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatBGRA8Unorm,
		Usage:  types.TextureUsageRenderAttachment | types.TextureUsageCopySrc | types.TextureUsageTextureBinding,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	outView, err := device.CreateTextureView(outTex, &webgpu.TextureViewDescriptor{
		Label: "tex_stencil_lin_result_view", Format: types.TextureFormatBGRA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		outTex.Release()
		return nil, nil, nil, err
	}
	releaseOut := func() {
		outView.Release()
		outTex.Release()
	}

	// Upload geometry.
	fanBytes := float32SliceToBytes(fan)
	fanBuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "tex_stencil_fan", Size: uint64(len(fanBytes)),
		Usage: types.BufferUsageVertex | types.BufferUsageCopyDst,
	})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	defer fanBuf.Release()
	if err := queue.WriteBuffer(fanBuf, 0, fanBytes); err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	cqBytes := float32SliceToBytes(cover[:])
	coverBuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "tex_stencil_cover_quad", Size: uint64(len(cqBytes)),
		Usage: types.BufferUsageVertex | types.BufferUsageCopyDst,
	})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	defer coverBuf.Release()
	if err := queue.WriteBuffer(coverBuf, 0, cqBytes); err != nil {
		releaseOut()
		return nil, nil, nil, err
	}

	// Fill uniform (viewport).
	fillUni := make([]byte, 16)
	binary.LittleEndian.PutUint32(fillUni[0:], math.Float32bits(float32(w)))
	binary.LittleEndian.PutUint32(fillUni[4:], math.Float32bits(float32(h)))
	fillUBuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "tex_stencil_fill_uni", Size: 16,
		Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
	})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	defer fillUBuf.Release()
	if err := queue.WriteBuffer(fillUBuf, 0, fillUni); err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	fillBG, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label: "tex_stencil_fill_bg", Layout: fillBGL,
		Entries: []webgpu.BindGroupEntry{{Binding: 0, Buffer: fillUBuf, Offset: 0, Size: 16}},
	})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	defer fillBG.Release()

	// Ramp texture.
	rampTex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "tex_stencil_ramp",
		Size:          webgpu.Extent3D{Width: uint32(n), Height: 1, DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA8Unorm,
		Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	defer rampTex.Release()
	rampView, err := device.CreateTextureView(rampTex, &webgpu.TextureViewDescriptor{
		Label: "tex_stencil_ramp_view", Format: types.TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	defer rampView.Release()
	rbpr := alignTextureBytesPerRow(uint32(n * 4)) //nolint:gosec
	rampUp := ramp
	if rbpr != uint32(n*4) { //nolint:gosec
		padded := make([]byte, int(rbpr))
		copy(padded, ramp[:n*4])
		rampUp = padded
	}
	if err := queue.WriteTexture(
		&webgpu.ImageCopyTexture{Texture: rampTex, MipLevel: 0},
		rampUp,
		&webgpu.ImageDataLayout{BytesPerRow: rbpr, RowsPerImage: 1},
		&webgpu.Extent3D{Width: uint32(n), Height: 1, DepthOrArrayLayers: 1}, //nolint:gosec
	); err != nil {
		releaseOut()
		return nil, nil, nil, err
	}

	coverUni := encodeTexturedStencilCoverUniform(w, h, params)
	coverUBuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "tex_stencil_cover_uni", Size: uint64(len(coverUni)),
		Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
	})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	defer coverUBuf.Release()
	if err := queue.WriteBuffer(coverUBuf, 0, coverUni); err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	coverBG, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label: "tex_stencil_cover_bg", Layout: coverBGL,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, Buffer: coverUBuf, Offset: 0, Size: uint64(len(coverUni))},
			{Binding: 1, TextureView: rampView},
			{Binding: 2, Sampler: sampler},
		},
	})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	defer coverBG.Release()

	enc, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: "tex_stencil_enc"})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	rp, err := enc.BeginRenderPass(&webgpu.RenderPassDescriptor{
		Label: "tex_stencil_pass",
		ColorAttachments: []webgpu.RenderPassColorAttachment{{
			View: msaaView, ResolveTarget: outView,
			LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
			ClearValue: types.Color{R: 0, G: 0, B: 0, A: 0},
		}},
		DepthStencilAttachment: &webgpu.RenderPassDepthStencilAttachment{
			View:        stencilView,
			DepthLoadOp: types.LoadOpClear, DepthStoreOp: types.StoreOpDiscard, DepthClearValue: 1,
			StencilLoadOp: types.LoadOpClear, StencilStoreOp: types.StoreOpDiscard, StencilClearValue: 0,
		},
	})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	fillPipe := nzPipe
	if fillRule == render.FillRuleEvenOdd {
		fillPipe = eoPipe
	}
	rp.SetPipeline(fillPipe)
	rp.SetBindGroup(0, fillBG, nil)
	rp.SetVertexBuffer(0, fanBuf, 0)
	rp.Draw(uint32(len(fan)/2), 1, 0, 0) //nolint:gosec

	rp.SetPipeline(coverPipe)
	rp.SetBindGroup(0, coverBG, nil)
	rp.SetVertexBuffer(0, coverBuf, 0)
	rp.SetStencilReference(0)
	rp.Draw(6, 1, 0, 0)
	rp.End()

	if retain {
		// Resolve complete; transition for sampling in session GPUTextureDraw.
		enc.TransitionTextures([]webgpu.TextureBarrier{{
			Texture: outTex,
			Usage: webgpu.TextureUsageTransition{
				OldUsage: types.TextureUsageRenderAttachment,
				NewUsage: types.TextureUsageTextureBinding,
			},
		}})
		cmd, err := enc.Finish()
		if err != nil {
			releaseOut()
			return nil, nil, nil, err
		}
		defer cmd.Release()
		if _, err := queue.Submit(cmd); err != nil {
			releaseOut()
			return nil, nil, nil, err
		}
		// Opaque retain: caller releases after Flush consumes the view.
		return nil, outTex, outView, nil
	}

	// Readback path (legacy GPU* bootstrap).
	enc.TransitionTextures([]webgpu.TextureBarrier{{
		Texture: outTex,
		Usage: webgpu.TextureUsageTransition{
			OldUsage: types.TextureUsageRenderAttachment,
			NewUsage: types.TextureUsageCopySrc,
		},
	}})

	bytesPerRow := w * 4
	aligned := alignTextureBytesPerRow(bytesPerRow)
	stagingSize := uint64(aligned) * uint64(h)
	staging, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "tex_stencil_readback", Size: stagingSize,
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	defer staging.Release()
	enc.CopyTextureToBuffer(outTex, staging, []webgpu.BufferTextureCopy{{
		BufferLayout: webgpu.ImageDataLayout{BytesPerRow: aligned, RowsPerImage: h},
		TextureBase:  webgpu.ImageCopyTexture{Texture: outTex, MipLevel: 0, Aspect: types.TextureAspectAll},
		Size:         webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
	}})
	cmd, err := enc.Finish()
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	defer cmd.Release()
	if _, err := queue.Submit(cmd); err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	device.Poll(webgpu.PollWait)
	if err := staging.Map(context.Background(), webgpu.MapModeRead, 0, stagingSize); err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	mapped, err := staging.MappedRange(0, stagingSize)
	if err != nil {
		_ = staging.Unmap()
		releaseOut()
		return nil, nil, nil, err
	}
	src := mapped.Bytes()
	// BGRA resolve → premul RGBA out.
	out := make([]byte, nw*nh*4)
	for y := 0; y < nh; y++ {
		row := src[y*int(aligned):]
		for x := 0; x < nw; x++ {
			si := x * 4
			di := (y*nw + x) * 4
			out[di+0] = row[si+2]
			out[di+1] = row[si+1]
			out[di+2] = row[si+0]
			out[di+3] = row[si+3]
		}
	}
	mapped.Release()
	_ = staging.Unmap()
	releaseOut()

	any := false
	for i := 3; i < len(out); i += 4 {
		if out[i] != 0 {
			any = true
			break
		}
	}
	if !any {
		return nil, nil, nil, nil
	}
	return out, nil, nil, nil
}
