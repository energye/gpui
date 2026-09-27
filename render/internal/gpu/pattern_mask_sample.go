//go:build !nogpu

package gpu

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"unsafe"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// N2 textured pattern cover: sample ImagePattern source texture by inverse
// affine map from device position, multiply by R8 coverage — on GPU.
// Removes O(pixels) ColorAt field expand for non-rect paths. Still GPU*
// (coverage + blit); rect native tile path stays first and is not demoted.
const patternMaskSampleWGSL = `
struct Params {
    bounds_min: vec2<f32>,
    bounds_size: vec2<f32>,
    inv_row0: vec4<f32>, // A, B, C, _
    inv_row1: vec4<f32>, // D, E, F, _
    pat_size: vec2<f32>,
    opacity: f32,
    clamp_mode: f32, // >0.5 => transparent OOB (ColorAt clamp)
}

struct VSOut {
    @builtin(position) pos: vec4<f32>,
}

@group(0) @binding(0) var pat_tex: texture_2d<f32>;
@group(0) @binding(1) var mask_tex: texture_2d<f32>;
@group(0) @binding(2) var pat_samp: sampler;
@group(0) @binding(3) var<uniform> p: Params;

@vertex
fn vs_main(@builtin(vertex_index) vi: u32) -> VSOut {
    var pts = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>( 3.0, -1.0),
        vec2<f32>(-1.0,  3.0),
    );
    var o: VSOut;
    o.pos = vec4<f32>(pts[vi], 0.0, 1.0);
    return o;
}

fn wrap_unit(v: f32, size: f32) -> f32 {
    // Map to [0, size).
    let s = max(size, 1.0);
    var r = v - floor(v / s) * s;
    // Guard float edge.
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
    let xy = vec2<i32>(i32(in.pos.x), i32(in.pos.y));
    let m = textureLoad(mask_tex, xy, 0).r;
    if (m < 1.0 / 255.0) {
        return vec4<f32>(0.0);
    }
    let dims = vec2<f32>(textureDimensions(mask_tex));
    let u = (in.pos.x + 0.5) / max(dims.x, 1.0);
    let v = (in.pos.y + 0.5) / max(dims.y, 1.0);
    let px = p.bounds_min.x + u * p.bounds_size.x;
    let py = p.bounds_min.y + v * p.bounds_size.y;

    // inverse affine: image = M * device
    let ix = p.inv_row0.x * px + p.inv_row0.y * py + p.inv_row0.z;
    let iy = p.inv_row1.x * px + p.inv_row1.y * py + p.inv_row1.z;

    var sx: f32;
    var sy: f32;
    if (p.clamp_mode > 0.5) {
        // ColorAt clamp: OOB → transparent (not edge clamp).
        if (ix < 0.0 || iy < 0.0 || ix >= p.pat_size.x || iy >= p.pat_size.y) {
            return vec4<f32>(0.0);
        }
        sx = ix;
        sy = iy;
    } else {
        sx = wrap_unit(ix, p.pat_size.x);
        sy = wrap_unit(iy, p.pat_size.y);
    }

    // Nearest-ish: sample pixel centers (matches int(lx) for positive coords).
    let fx = floor(sx);
    let fy = floor(sy);
    let uv = (vec2<f32>(fx, fy) + vec2<f32>(0.5, 0.5)) / max(p.pat_size, vec2<f32>(1.0, 1.0));
    let color = textureSampleLevel(pat_tex, pat_samp, uv, 0.0);
    // Premul source * opacity * coverage.
    return color * p.opacity * m;
}
`

type patternMaskSampleParams struct {
	boundsMinX, boundsMinY float32
	boundsW, boundsH       float32
	invA, invB, invC       float32
	invD, invE, invF       float32
	patW, patH             float32
	opacity                float32
	clampMode              float32
}

type patternMaskSampleCache struct {
	mu       sync.Mutex
	device   hal.Device
	shader   hal.ShaderModule
	bgl      hal.BindGroupLayout
	pipeLay  hal.PipelineLayout
	pipeline hal.RenderPipeline
	sampler  hal.Sampler
}

func (c *patternMaskSampleCache) release() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pipeline != nil {
		c.pipeline.Destroy()
		c.pipeline = nil
	}
	if c.pipeLay != nil {
		c.pipeLay.Destroy()
		c.pipeLay = nil
	}
	if c.bgl != nil {
		c.bgl.Destroy()
		c.bgl = nil
	}
	if c.shader != nil {
		c.shader.Destroy()
		c.shader = nil
	}
	if c.sampler != nil {
		c.sampler.Destroy()
		c.sampler = nil
	}
	c.device = nil
}

func (c *patternMaskSampleCache) ensure(device hal.Device) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.device != nil && c.device != device {
		if c.pipeline != nil {
			c.pipeline.Destroy()
		}
		if c.pipeLay != nil {
			c.pipeLay.Destroy()
		}
		if c.bgl != nil {
			c.bgl.Destroy()
		}
		if c.shader != nil {
			c.shader.Destroy()
		}
		if c.sampler != nil {
			c.sampler.Destroy()
		}
		c.pipeline, c.pipeLay, c.bgl, c.shader, c.sampler = nil, nil, nil, nil, nil
	}
	c.device = device
	if c.pipeline != nil {
		return nil
	}
	shader, err := device.CreateShaderModule(&hal.ShaderModuleDescriptor{
		Label: "pattern_mask_sample",
		WGSL:  patternMaskSampleWGSL,
	})
	if err != nil {
		return fmt.Errorf("pattern mask shader: %w", err)
	}
	bgl, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "pattern_mask_bgl",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding: 0, Visibility: types.ShaderStageFragment,
				Texture: &types.TextureBindingLayout{
					SampleType: types.TextureSampleTypeFloat, ViewDimension: types.TextureViewDimension2D,
				},
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
			{
				Binding: 3, Visibility: types.ShaderStageFragment,
				Buffer: &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: 64},
			},
		},
	})
	if err != nil {
		shader.Destroy()
		return fmt.Errorf("pattern mask bgl: %w", err)
	}
	pipeLay, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label: "pattern_mask_pipe_layout", BindGroupLayouts: []hal.BindGroupLayout{bgl},
	})
	if err != nil {
		bgl.Destroy()
		shader.Destroy()
		return err
	}
	replace := types.BlendState{
		Color: types.BlendComponent{SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorZero, Operation: types.BlendOperationAdd},
		Alpha: types.BlendComponent{SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorZero, Operation: types.BlendOperationAdd},
	}
	pipe, err := device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "pattern_mask_sample_pipe",
		Layout: pipeLay,
		Vertex: hal.VertexState{Module: shader, EntryPoint: "vs_main"},
		Fragment: &hal.FragmentState{
			Module: shader, EntryPoint: "fs_main",
			Targets: []types.ColorTargetState{{
				Format: types.TextureFormatRGBA8Unorm, Blend: &replace, WriteMask: types.ColorWriteMaskAll,
			}},
		},
		Primitive:   triangleListPrimitive(),
		Multisample: types.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
	})
	if err != nil {
		pipeLay.Destroy()
		bgl.Destroy()
		shader.Destroy()
		return fmt.Errorf("pattern mask pipeline: %w", err)
	}
	samp, err := device.CreateSampler(&hal.SamplerDescriptor{
		Label:        "pattern_mask_samp",
		AddressModeU: types.AddressModeClampToEdge,
		AddressModeV: types.AddressModeClampToEdge,
		AddressModeW: types.AddressModeClampToEdge,
		MagFilter:    types.FilterModeNearest,
		MinFilter:    types.FilterModeNearest,
		MipmapFilter: types.MipmapFilterModeNearest,
		Anisotropy:   1,
	})
	if err != nil {
		pipe.Destroy()
		pipeLay.Destroy()
		bgl.Destroy()
		shader.Destroy()
		return fmt.Errorf("pattern mask sampler: %w", err)
	}
	c.shader = shader
	c.bgl = bgl
	c.pipeLay = pipeLay
	c.pipeline = pipe
	c.sampler = samp
	return nil
}

func encodePatternMaskSampleUniform(p patternMaskSampleParams) []byte {
	buf := make([]byte, 64)
	put := func(i int, v float32) {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	put(0, p.boundsMinX)
	put(1, p.boundsMinY)
	put(2, p.boundsW)
	put(3, p.boundsH)
	put(4, p.invA)
	put(5, p.invB)
	put(6, p.invC)
	put(7, 0)
	put(8, p.invD)
	put(9, p.invE)
	put(10, p.invF)
	put(11, 0)
	put(12, p.patW)
	put(13, p.patH)
	put(14, p.opacity)
	put(15, p.clampMode)
	return buf
}

// patternMaskSampleExpand samples pattern tile by inverse map × R8 coverage on GPU.
// tile is srcW*srcH*4 premul RGBA; maskR8 is nw*nh.
func patternMaskSampleExpand(
	device hal.Device,
	queue hal.Queue,
	cache *patternMaskSampleCache,
	tile []byte,
	srcW, srcH int,
	maskR8 []byte,
	nw, nh int,
	params patternMaskSampleParams,
) ([]byte, error) {
	if device == nil || queue == nil || cache == nil || srcW < 1 || srcH < 1 || nw <= 0 || nh <= 0 {
		return nil, fmt.Errorf("pattern mask: bad args")
	}
	needTile := srcW * srcH * 4
	needMask := nw * nh
	needOut := nw * nh * 4
	if len(tile) < needTile || len(maskR8) < needMask {
		return nil, fmt.Errorf("pattern mask: buffer size")
	}
	if err := cache.ensure(device); err != nil {
		return nil, err
	}

	// Pattern texture.
	patTex, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "pattern_mask_pat",
		Size:          hal.Extent3D{Width: uint32(srcW), Height: uint32(srcH), DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA8Unorm,
		Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("pattern tex: %w", err)
	}
	defer patTex.Destroy()
	patView, err := device.CreateTextureView(patTex, &hal.TextureViewDescriptor{
		Label: "pattern_mask_pat_view", Format: types.TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		return nil, err
	}
	defer patView.Destroy()
	tightPat := uint32(srcW * 4) //nolint:gosec
	alignedPat := alignTextureBytesPerRow(tightPat)
	patUpload := tile
	if alignedPat != tightPat {
		padded := make([]byte, int(alignedPat)*srcH)
		for y := 0; y < srcH; y++ {
			copy(padded[y*int(alignedPat):y*int(alignedPat)+srcW*4], tile[y*srcW*4:(y+1)*srcW*4])
		}
		patUpload = padded
	}
	if err := queue.WriteTexture(
		&hal.ImageCopyTexture{Texture: patTex, MipLevel: 0},
		patUpload,
		&hal.ImageDataLayout{BytesPerRow: alignedPat, RowsPerImage: uint32(srcH)},       //nolint:gosec
		&hal.Extent3D{Width: uint32(srcW), Height: uint32(srcH), DepthOrArrayLayers: 1}, //nolint:gosec
	); err != nil {
		return nil, fmt.Errorf("pattern upload: %w", err)
	}

	// Coverage R8.
	maskTex, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "pattern_mask_cov",
		Size:          hal.Extent3D{Width: uint32(nw), Height: uint32(nh), DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatR8Unorm,
		Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("pattern mask tex: %w", err)
	}
	defer maskTex.Destroy()
	maskView, err := device.CreateTextureView(maskTex, &hal.TextureViewDescriptor{
		Label: "pattern_mask_cov_view", Format: types.TextureFormatR8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		return nil, err
	}
	defer maskView.Destroy()
	maskTight := uint32(nw) //nolint:gosec
	maskAligned := alignTextureBytesPerRow(maskTight)
	maskUpload := maskR8
	if maskAligned != maskTight {
		padded := make([]byte, int(maskAligned)*nh)
		for y := 0; y < nh; y++ {
			copy(padded[y*int(maskAligned):y*int(maskAligned)+nw], maskR8[y*nw:(y+1)*nw])
		}
		maskUpload = padded
	}
	if err := queue.WriteTexture(
		&hal.ImageCopyTexture{Texture: maskTex, MipLevel: 0},
		maskUpload,
		&hal.ImageDataLayout{BytesPerRow: maskAligned, RowsPerImage: uint32(nh)},    //nolint:gosec
		&hal.Extent3D{Width: uint32(nw), Height: uint32(nh), DepthOrArrayLayers: 1}, //nolint:gosec
	); err != nil {
		return nil, fmt.Errorf("pattern mask upload: %w", err)
	}

	outTex, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "pattern_mask_out",
		Size:          hal.Extent3D{Width: uint32(nw), Height: uint32(nh), DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA8Unorm,
		Usage:  types.TextureUsageRenderAttachment | types.TextureUsageCopySrc | types.TextureUsageTextureBinding,
	})
	if err != nil {
		return nil, fmt.Errorf("pattern out: %w", err)
	}
	defer outTex.Destroy()
	outView, err := device.CreateTextureView(outTex, &hal.TextureViewDescriptor{
		Label: "pattern_mask_out_view", Format: types.TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		return nil, err
	}
	defer outView.Destroy()

	uData := encodePatternMaskSampleUniform(params)
	uBuf, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label: "pattern_mask_uniform", Size: uint64(len(uData)),
		Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, err
	}
	defer uBuf.Destroy()
	if err := queue.WriteBuffer(uBuf, 0, uData); err != nil {
		return nil, err
	}

	cache.mu.Lock()
	bgl, pipeline, sampler := cache.bgl, cache.pipeline, cache.sampler
	cache.mu.Unlock()

	bg, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "pattern_mask_bg",
		Layout: bgl,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, TextureView: patView},
			{Binding: 1, TextureView: maskView},
			{Binding: 2, Sampler: sampler},
			{Binding: 3, Buffer: uBuf, Offset: 0, Size: uint64(len(uData))},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("pattern mask bg: %w", err)
	}
	defer bg.Destroy()

	enc, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "pattern_mask_enc"})
	if err != nil {
		return nil, err
	}
	rp, err := enc.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "pattern_mask_pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View: outView, LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
			ClearValue: types.Color{R: 0, G: 0, B: 0, A: 0},
		}},
	})
	if err != nil {
		return nil, err
	}
	rp.SetPipeline(pipeline)
	rp.SetBindGroup(0, bg, nil)
	rp.Draw(3, 1, 0, 0)
	rp.End()

	tightRow := uint32(nw * 4) //nolint:gosec
	alignedRow := alignTextureBytesPerRow(tightRow)
	stagingSize := uint64(alignedRow) * uint64(nh)
	staging, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label: "pattern_mask_readback", Size: stagingSize,
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, err
	}
	defer staging.Destroy()

	enc.CopyTextureToBuffer(outTex, staging, []hal.BufferTextureCopy{{
		BufferLayout: hal.ImageDataLayout{BytesPerRow: alignedRow, RowsPerImage: uint32(nh)}, //nolint:gosec
		TextureBase:  hal.ImageCopyTexture{Texture: outTex, MipLevel: 0, Aspect: types.TextureAspectAll},
		Size:         hal.Extent3D{Width: uint32(nw), Height: uint32(nh), DepthOrArrayLayers: 1}, //nolint:gosec
	}})
	cmd, err := enc.Finish()
	if err != nil {
		return nil, err
	}
	defer device.FreeCommandBuffer(cmd)
	if _, err := queue.Submit(cmd); err != nil {
		return nil, err
	}
	device.Poll(hal.PollWait)
	mapping, err := device.MapBuffer(staging, 0, stagingSize)
	if err != nil {
		return nil, err
	}
	src := unsafe.Slice((*byte)(mapping.Ptr), stagingSize) //nolint:gosec // hal.BufferMapping opaque pointer
	out := make([]byte, needOut)
	if alignedRow == tightRow {
		copy(out, src[:needOut])
	} else {
		for y := 0; y < nh; y++ {
			copy(out[y*nw*4:(y+1)*nw*4], src[y*int(alignedRow):y*int(alignedRow)+nw*4])
		}
	}
	_ = device.UnmapBuffer(staging)
	return out, nil
}
