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

// N2 textured stencil cover for ImagePattern (non-rect):
// stencil-fill path + cover FS samples pattern texture by inverse affine UV.
// Prefer retain (no CPU readback) + QueueGPUTextureDraw. Readback kept as
// fallback. Rect native tile path stays first and is not demoted.

const texturedStencilCoverPatternWGSL = `
struct CoverUni {
    viewport: vec2<f32>,
    _pad0: vec2<f32>,
    bounds_min: vec2<f32>,
    bounds_size: vec2<f32>,
    inv_row0: vec4<f32>, // A, B, C, _
    inv_row1: vec4<f32>, // D, E, F, _
    pat_size: vec2<f32>,
    opacity: f32,
    clamp_mode: f32, // >0.5 => transparent OOB
}

@group(0) @binding(0) var<uniform> u: CoverUni;
@group(0) @binding(1) var pat_tex: texture_2d<f32>;
@group(0) @binding(2) var pat_samp: sampler;

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

fn wrap_unit(v: f32, size: f32) -> f32 {
    let s = max(size, 1.0);
    var r = v - floor(v / s) * s;
    if (r >= s) {
        r = 0.0;
    }
    if (r < 0.0) {
        r = r + s;
    }
    return r;
}

@fragment
fn fs_main(in: VSOut) -> @location(0) vec4<f32> {
    let u0 = in.pos.x / max(u.viewport.x, 1.0);
    let v0 = in.pos.y / max(u.viewport.y, 1.0);
    let px = u.bounds_min.x + u0 * u.bounds_size.x;
    let py = u.bounds_min.y + v0 * u.bounds_size.y;

    let ix = u.inv_row0.x * px + u.inv_row0.y * py + u.inv_row0.z;
    let iy = u.inv_row1.x * px + u.inv_row1.y * py + u.inv_row1.z;

    var sx: f32;
    var sy: f32;
    if (u.clamp_mode > 0.5) {
        if (ix < 0.0 || iy < 0.0 || ix >= u.pat_size.x || iy >= u.pat_size.y) {
            return vec4<f32>(0.0);
        }
        sx = ix;
        sy = iy;
    } else {
        sx = wrap_unit(ix, u.pat_size.x);
        sy = wrap_unit(iy, u.pat_size.y);
    }

    let fx = floor(sx);
    let fy = floor(sy);
    let uv = (vec2<f32>(fx, fy) + vec2<f32>(0.5, 0.5)) / max(u.pat_size, vec2<f32>(1.0, 1.0));
    let color = textureSampleLevel(pat_tex, pat_samp, uv, 0.0);
    return color * u.opacity;
}
`

type texturedStencilPatternParams struct {
	boundsMinX, boundsMinY float32
	boundsW, boundsH       float32
	invA, invB, invC       float32
	invD, invE, invF       float32
	patW, patH             float32
	opacity                float32
	clampMode              float32
}

type texturedStencilPatternCache struct {
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

func (c *texturedStencilPatternCache) release() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.destroyPipelinesLocked()
	c.tex.destroyTextures()
	c.device = nil
}

func (c *texturedStencilPatternCache) destroyPipelinesLocked() {
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

func (c *texturedStencilPatternCache) ensure(device *webgpu.Device, sampleCount uint32) error {
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
		Label: "tex_stencil_pat_fill", WGSL: texturedStencilFillWGSL,
	})
	if err != nil {
		return fmt.Errorf("tex stencil pat fill shader: %w", err)
	}
	coverShader, err := device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "tex_stencil_pat_cover", WGSL: texturedStencilCoverPatternWGSL,
	})
	if err != nil {
		fillShader.Release()
		return fmt.Errorf("tex stencil pat cover shader: %w", err)
	}

	fillBGL, err := device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "tex_stencil_pat_fill_bgl",
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
		Label: "tex_stencil_pat_cover_bgl",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding: 0, Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
				Buffer: &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: 80},
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
		Label: "tex_stencil_pat_fill_lay", BindGroupLayouts: []*webgpu.BindGroupLayout{fillBGL},
	})
	if err != nil {
		coverBGL.Release()
		fillBGL.Release()
		fillShader.Release()
		coverShader.Release()
		return err
	}
	coverLay, err := device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label: "tex_stencil_pat_cover_lay", BindGroupLayouts: []*webgpu.BindGroupLayout{coverBGL},
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
	nz, err := makeFill("tex_stencil_pat_fill_nz", false)
	if err != nil {
		coverLay.Release()
		fillLay.Release()
		coverBGL.Release()
		fillBGL.Release()
		fillShader.Release()
		coverShader.Release()
		return fmt.Errorf("nz fill pipe: %w", err)
	}
	eo, err := makeFill("tex_stencil_pat_fill_eo", true)
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
		Color: types.BlendComponent{
			SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorZero, Operation: types.BlendOperationAdd,
		},
		Alpha: types.BlendComponent{
			SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorZero, Operation: types.BlendOperationAdd,
		},
	}
	cover, err := device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "tex_stencil_pat_cover",
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
		Label:        "tex_stencil_pat_samp",
		AddressModeU: types.AddressModeClampToEdge, AddressModeV: types.AddressModeClampToEdge,
		AddressModeW: types.AddressModeClampToEdge,
		MagFilter:    types.FilterModeNearest, MinFilter: types.FilterModeNearest,
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

func encodeTexturedStencilPatternUniform(w, h uint32, p texturedStencilPatternParams) []byte {
	// 20 floats = 80 bytes
	buf := make([]byte, 80)
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
	put(8, p.invA)
	put(9, p.invB)
	put(10, p.invC)
	put(11, 0)
	put(12, p.invD)
	put(13, p.invE)
	put(14, p.invF)
	put(15, 0)
	put(16, p.patW)
	put(17, p.patH)
	put(18, p.opacity)
	put(19, p.clampMode)
	return buf
}

// texturedStencilCoverPattern stencil-fills localPath and covers with pattern
// sampling. Returns premul RGBA (readback). Prefer Retain for no-readback path.
func texturedStencilCoverPattern(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *texturedStencilPatternCache,
	localPath *render.Path,
	fillRule render.FillRule,
	nw, nh int,
	tile []byte,
	srcW, srcH int,
	params texturedStencilPatternParams,
	sampleCount uint32,
) ([]byte, error) {
	px, tex, view, err := texturedStencilCoverPatternEx(device, queue, cache, localPath, fillRule, nw, nh, tile, srcW, srcH, params, sampleCount, false)
	if tex != nil {
		view.Release()
		tex.Release()
	}
	return px, err
}

// texturedStencilCoverPatternRetain keeps the cover result on GPU for
// QueueGPUTextureDraw. Caller owns tex/view until after Flush.
func texturedStencilCoverPatternRetain(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *texturedStencilPatternCache,
	localPath *render.Path,
	fillRule render.FillRule,
	nw, nh int,
	tile []byte,
	srcW, srcH int,
	params texturedStencilPatternParams,
	sampleCount uint32,
) (*webgpu.Texture, *webgpu.TextureView, error) {
	px, tex, view, err := texturedStencilCoverPatternEx(device, queue, cache, localPath, fillRule, nw, nh, tile, srcW, srcH, params, sampleCount, true)
	if err != nil {
		return nil, nil, err
	}
	if px == nil && tex == nil {
		return nil, nil, nil
	}
	return tex, view, nil
}

func texturedStencilCoverPatternEx(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *texturedStencilPatternCache,
	localPath *render.Path,
	fillRule render.FillRule,
	nw, nh int,
	tile []byte,
	srcW, srcH int,
	params texturedStencilPatternParams,
	sampleCount uint32,
	retain bool,
) ([]byte, *webgpu.Texture, *webgpu.TextureView, error) {
	if device == nil || queue == nil || cache == nil || localPath == nil || nw <= 0 || nh <= 0 {
		return nil, nil, nil, fmt.Errorf("tex stencil pat: bad args")
	}
	if srcW <= 0 || srcH <= 0 || len(tile) < srcW*srcH*4 {
		return nil, nil, nil, fmt.Errorf("tex stencil pat: tile size")
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
	if err := cache.tex.ensureTextures(device, w, h, "tex_stencil_pat", cache.sampleCount); err != nil {
		cache.mu.Unlock()
		return nil, nil, nil, err
	}
	msaaView := cache.tex.msaaView
	stencilView := cache.tex.stencilView
	nzPipe, eoPipe, coverPipe := cache.nzFillPipe, cache.eoFillPipe, cache.coverPipe
	fillBGL, coverBGL, sampler := cache.fillBGL, cache.coverBGL, cache.sampler
	cache.mu.Unlock()

	outTex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "tex_stencil_pat_result",
		Size:          webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatBGRA8Unorm,
		Usage:  types.TextureUsageRenderAttachment | types.TextureUsageCopySrc | types.TextureUsageTextureBinding,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	outView, err := device.CreateTextureView(outTex, &webgpu.TextureViewDescriptor{
		Label: "tex_stencil_pat_result_view", Format: types.TextureFormatBGRA8Unorm,
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

	fanBytes := float32SliceToBytes(fan)
	fanBuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "tex_stencil_pat_fan", Size: uint64(len(fanBytes)),
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
		Label: "tex_stencil_pat_cover_quad", Size: uint64(len(cqBytes)),
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

	fillUni := make([]byte, 16)
	binary.LittleEndian.PutUint32(fillUni[0:], math.Float32bits(float32(w)))
	binary.LittleEndian.PutUint32(fillUni[4:], math.Float32bits(float32(h)))
	fillUBuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "tex_stencil_pat_fill_uni", Size: 16,
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
		Label: "tex_stencil_pat_fill_bg", Layout: fillBGL,
		Entries: []webgpu.BindGroupEntry{{Binding: 0, Buffer: fillUBuf, Offset: 0, Size: 16}},
	})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	defer fillBG.Release()

	patTex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "tex_stencil_pat_src",
		Size:          webgpu.Extent3D{Width: uint32(srcW), Height: uint32(srcH), DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA8Unorm,
		Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	defer patTex.Release()
	patView, err := device.CreateTextureView(patTex, &webgpu.TextureViewDescriptor{
		Label: "tex_stencil_pat_src_view", Format: types.TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	defer patView.Release()
	patBPR := alignTextureBytesPerRow(uint32(srcW * 4)) //nolint:gosec
	patUp := tile
	if patBPR != uint32(srcW*4) { //nolint:gosec
		padded := make([]byte, int(patBPR)*srcH)
		rowBytes := srcW * 4
		for y := 0; y < srcH; y++ {
			copy(padded[y*int(patBPR):y*int(patBPR)+rowBytes], tile[y*rowBytes:(y+1)*rowBytes])
		}
		patUp = padded
	}
	if err := queue.WriteTexture(
		&webgpu.ImageCopyTexture{Texture: patTex, MipLevel: 0},
		patUp,
		&webgpu.ImageDataLayout{BytesPerRow: patBPR, RowsPerImage: uint32(srcH)},           //nolint:gosec
		&webgpu.Extent3D{Width: uint32(srcW), Height: uint32(srcH), DepthOrArrayLayers: 1}, //nolint:gosec
	); err != nil {
		releaseOut()
		return nil, nil, nil, err
	}

	coverUni := encodeTexturedStencilPatternUniform(w, h, params)
	coverUBuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "tex_stencil_pat_cover_uni", Size: uint64(len(coverUni)),
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
		Label: "tex_stencil_pat_cover_bg", Layout: coverBGL,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, Buffer: coverUBuf, Offset: 0, Size: uint64(len(coverUni))},
			{Binding: 1, TextureView: patView},
			{Binding: 2, Sampler: sampler},
		},
	})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	defer coverBG.Release()

	enc, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: "tex_stencil_pat_enc"})
	if err != nil {
		releaseOut()
		return nil, nil, nil, err
	}
	rp, err := enc.BeginRenderPass(&webgpu.RenderPassDescriptor{
		Label: "tex_stencil_pat_pass",
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
		return nil, outTex, outView, nil
	}

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
		Label: "tex_stencil_pat_readback", Size: stagingSize,
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
