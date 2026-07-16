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
)

// N1 textured param cover: sample 1D premul ramp by gradient parameter t
// (linear / radial-simple / sweep), multiply by R8 coverage — on GPU.
// Removes O(pixels) CPU field expand. Still GPU* (coverage may be stenciled
// offscreen; blit via QueueImageDraw); not solid stencil cover of a solid color.
const linearRampMaskWGSL = `
// mode: 0=linear, 1=radial simple, 2=sweep+, 3=focal radial
struct Params {
    bounds_min: vec2<f32>,
    bounds_size: vec2<f32>,
    // linear: start; radial/sweep: center
    p0: vec2<f32>,
    // linear: d=end-start; radial: (startR, invRadiusDiff); sweep: (startAngle, invSweepRange)
    p1: vec2<f32>,
    // linear: inv_len2; else unused
    inv_len2: f32,
    t_min: f32,
    inv_span: f32,
    mode: f32,
}

struct VSOut {
    @builtin(position) pos: vec4<f32>,
}

@group(0) @binding(0) var ramp_tex: texture_2d<f32>;
@group(0) @binding(1) var mask_tex: texture_2d<f32>;
@group(0) @binding(2) var ramp_samp: sampler;
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

fn gradient_t(px: f32, py: f32) -> f32 {
    if (p.mode < 0.5) {
        // linear
        return ((px - p.p0.x) * p.p1.x + (py - p.p0.y) * p.p1.y) * p.inv_len2;
    }
    if (p.mode < 1.5) {
        // radial simple: p0=center, p1=(startR, invRadiusDiff)
        let d = length(vec2<f32>(px, py) - p.p0);
        return (d - p.p1.x) * p.p1.y;
    }
    if (p.mode < 2.5) {
        // sweep+: p0=center, p1=(startAngle, invSweepRange)
        let v = vec2<f32>(px, py) - p.p0;
        if (dot(v, v) < 1e-12) {
            return p.t_min;
        }
        let angle = atan2(v.y, v.x);
        var rel = angle - p.p1.x;
        let two_pi = 6.283185307179586;
        rel = rel - floor(rel / two_pi) * two_pi;
        return rel * p.p1.y;
    }
    // mode 3: focal radial. p0=focus, p1=center, inv_len2=endRadius
    // Matches render.RadialGradientBrush.computeTFocal.
    let focus = p.p0;
    let center = p.p1;
    let end_r = p.inv_len2;
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
    let tt = gradient_t(px, py);
    var ru = (tt - p.t_min) * p.inv_span;
    ru = clamp(ru, 0.0, 1.0);
    let color = textureSampleLevel(ramp_tex, ramp_samp, vec2<f32>(ru, 0.5), 0.0);
    return color * m;
}
`

// linearRampMaskParams carries device-space projection for the GPU expand.
// Mode: 0=linear, 1=radial simple, 2=sweep+, 3=focal radial.
type linearRampMaskParams struct {
	boundsMinX, boundsMinY float32
	boundsW, boundsH       float32
	// linear: start; radial/sweep: center
	startX, startY float32
	// linear: d; radial: (startR, invRadiusDiff); sweep: (startAngle, invSweepRange)
	dX, dY  float32
	invLen2 float32 // linear only
	tMin    float32
	invSpan float32
	mode    float32
}

type linearRampMaskCache struct {
	mu       sync.Mutex
	device   *webgpu.Device
	shader   *webgpu.ShaderModule
	bgl      *webgpu.BindGroupLayout
	pipeLay  *webgpu.PipelineLayout
	pipeline *webgpu.RenderPipeline
	sampler  *webgpu.Sampler
}

func (c *linearRampMaskCache) release() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pipeline != nil {
		c.pipeline.Release()
		c.pipeline = nil
	}
	if c.pipeLay != nil {
		c.pipeLay.Release()
		c.pipeLay = nil
	}
	if c.bgl != nil {
		c.bgl.Release()
		c.bgl = nil
	}
	if c.shader != nil {
		c.shader.Release()
		c.shader = nil
	}
	if c.sampler != nil {
		c.sampler.Release()
		c.sampler = nil
	}
	c.device = nil
}

func (c *linearRampMaskCache) ensure(device *webgpu.Device) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.device != nil && c.device != device {
		if c.pipeline != nil {
			c.pipeline.Release()
		}
		if c.pipeLay != nil {
			c.pipeLay.Release()
		}
		if c.bgl != nil {
			c.bgl.Release()
		}
		if c.shader != nil {
			c.shader.Release()
		}
		if c.sampler != nil {
			c.sampler.Release()
		}
		c.pipeline, c.pipeLay, c.bgl, c.shader, c.sampler = nil, nil, nil, nil, nil
	}
	c.device = device
	if c.pipeline != nil {
		return nil
	}
	shader, err := device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "linear_ramp_mask",
		WGSL:  linearRampMaskWGSL,
	})
	if err != nil {
		return fmt.Errorf("linear ramp mask shader: %w", err)
	}
	bgl, err := device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "linear_ramp_mask_bgl",
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
		shader.Release()
		return fmt.Errorf("linear ramp mask bgl: %w", err)
	}
	pipeLay, err := device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label: "linear_ramp_mask_pipe_layout", BindGroupLayouts: []*webgpu.BindGroupLayout{bgl},
	})
	if err != nil {
		bgl.Release()
		shader.Release()
		return err
	}
	replace := types.BlendState{
		Color: types.BlendComponent{SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorZero, Operation: types.BlendOperationAdd},
		Alpha: types.BlendComponent{SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorZero, Operation: types.BlendOperationAdd},
	}
	pipe, err := device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "linear_ramp_mask_pipe",
		Layout: pipeLay,
		Vertex: webgpu.VertexState{Module: shader, EntryPoint: "vs_main"},
		Fragment: &webgpu.FragmentState{
			Module: shader, EntryPoint: "fs_main",
			Targets: []types.ColorTargetState{{
				Format: types.TextureFormatRGBA8Unorm, Blend: &replace, WriteMask: types.ColorWriteMaskAll,
			}},
		},
		Primitive:   triangleListPrimitive(),
		Multisample: types.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
	})
	if err != nil {
		pipeLay.Release()
		bgl.Release()
		shader.Release()
		return fmt.Errorf("linear ramp mask pipeline: %w", err)
	}
	// Linear along ramp; mask uses textureLoad (no filter).
	samp, err := device.CreateSampler(&webgpu.SamplerDescriptor{
		Label:        "linear_ramp_mask_samp",
		AddressModeU: types.AddressModeClampToEdge,
		AddressModeV: types.AddressModeClampToEdge,
		AddressModeW: types.AddressModeClampToEdge,
		MagFilter:    types.FilterModeLinear,
		MinFilter:    types.FilterModeLinear,
		MipmapFilter: types.MipmapFilterModeNearest,
		Anisotropy:   1,
	})
	if err != nil {
		pipe.Release()
		pipeLay.Release()
		bgl.Release()
		shader.Release()
		return fmt.Errorf("linear ramp mask sampler: %w", err)
	}
	c.shader = shader
	c.bgl = bgl
	c.pipeLay = pipeLay
	c.pipeline = pipe
	c.sampler = samp
	return nil
}

func encodeLinearRampMaskUniform(p linearRampMaskParams) []byte {
	// 12 × f32 = 48 bytes (+16 pad) = 64, std140-friendly.
	buf := make([]byte, 64)
	put := func(i int, v float32) {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	put(0, p.boundsMinX)
	put(1, p.boundsMinY)
	put(2, p.boundsW)
	put(3, p.boundsH)
	put(4, p.startX)
	put(5, p.startY)
	put(6, p.dX)
	put(7, p.dY)
	put(8, p.invLen2)
	put(9, p.tMin)
	put(10, p.invSpan)
	put(11, p.mode)
	return buf
}

// linearRampMaskExpand samples 1D premul ramp by projected t and multiplies by
// R8 coverage on GPU. ramp is n*4 premul RGBA; maskR8 is nw*nh.
func linearRampMaskExpand(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *linearRampMaskCache,
	ramp []byte,
	n int,
	maskR8 []byte,
	nw, nh int,
	params linearRampMaskParams,
) ([]byte, error) {
	if device == nil || queue == nil || cache == nil || n < 1 || nw <= 0 || nh <= 0 {
		return nil, fmt.Errorf("linear ramp mask: bad args")
	}
	needRamp := n * 4
	needMask := nw * nh
	needOut := nw * nh * 4
	if len(ramp) < needRamp || len(maskR8) < needMask {
		return nil, fmt.Errorf("linear ramp mask: buffer size")
	}
	if err := cache.ensure(device); err != nil {
		return nil, err
	}

	// Ramp texture: n×1 RGBA8.
	rampTex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "linear_ramp_tex",
		Size:          webgpu.Extent3D{Width: uint32(n), Height: 1, DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA8Unorm,
		Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("linear ramp tex: %w", err)
	}
	defer rampTex.Release()
	rampView, err := device.CreateTextureView(rampTex, &webgpu.TextureViewDescriptor{
		Label: "linear_ramp_view", Format: types.TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		return nil, err
	}
	defer rampView.Release()
	rampBPR := alignTextureBytesPerRow(uint32(n * 4)) //nolint:gosec
	rampUpload := ramp
	if rampBPR != uint32(n*4) { //nolint:gosec
		padded := make([]byte, int(rampBPR))
		copy(padded, ramp[:needRamp])
		rampUpload = padded
	}
	if err := queue.WriteTexture(
		&webgpu.ImageCopyTexture{Texture: rampTex, MipLevel: 0},
		rampUpload,
		&webgpu.ImageDataLayout{BytesPerRow: rampBPR, RowsPerImage: 1},
		&webgpu.Extent3D{Width: uint32(n), Height: 1, DepthOrArrayLayers: 1}, //nolint:gosec
	); err != nil {
		return nil, fmt.Errorf("linear ramp upload: %w", err)
	}

	// Coverage R8.
	maskTex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "linear_ramp_mask_tex",
		Size:          webgpu.Extent3D{Width: uint32(nw), Height: uint32(nh), DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatR8Unorm,
		Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("linear ramp mask tex: %w", err)
	}
	defer maskTex.Release()
	maskView, err := device.CreateTextureView(maskTex, &webgpu.TextureViewDescriptor{
		Label: "linear_ramp_mask_view", Format: types.TextureFormatR8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		return nil, err
	}
	defer maskView.Release()
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
		&webgpu.ImageCopyTexture{Texture: maskTex, MipLevel: 0},
		maskUpload,
		&webgpu.ImageDataLayout{BytesPerRow: maskAligned, RowsPerImage: uint32(nh)},    //nolint:gosec
		&webgpu.Extent3D{Width: uint32(nw), Height: uint32(nh), DepthOrArrayLayers: 1}, //nolint:gosec
	); err != nil {
		return nil, fmt.Errorf("linear ramp mask upload: %w", err)
	}

	// Output RT.
	outTex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "linear_ramp_out",
		Size:          webgpu.Extent3D{Width: uint32(nw), Height: uint32(nh), DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA8Unorm,
		Usage:  types.TextureUsageRenderAttachment | types.TextureUsageCopySrc | types.TextureUsageTextureBinding,
	})
	if err != nil {
		return nil, fmt.Errorf("linear ramp out: %w", err)
	}
	defer outTex.Release()
	outView, err := device.CreateTextureView(outTex, &webgpu.TextureViewDescriptor{
		Label: "linear_ramp_out_view", Format: types.TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		return nil, err
	}
	defer outView.Release()

	// Uniform.
	uData := encodeLinearRampMaskUniform(params)
	uBuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "linear_ramp_uniform", Size: uint64(len(uData)),
		Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, err
	}
	defer uBuf.Release()
	if err := queue.WriteBuffer(uBuf, 0, uData); err != nil {
		return nil, err
	}

	cache.mu.Lock()
	bgl, pipeline, sampler := cache.bgl, cache.pipeline, cache.sampler
	cache.mu.Unlock()

	bg, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label:  "linear_ramp_mask_bg",
		Layout: bgl,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, TextureView: rampView},
			{Binding: 1, TextureView: maskView},
			{Binding: 2, Sampler: sampler},
			{Binding: 3, Buffer: uBuf, Offset: 0, Size: uint64(len(uData))},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("linear ramp bg: %w", err)
	}
	defer bg.Release()

	enc, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: "linear_ramp_enc"})
	if err != nil {
		return nil, err
	}
	rp, err := enc.BeginRenderPass(&webgpu.RenderPassDescriptor{
		Label: "linear_ramp_pass",
		ColorAttachments: []webgpu.RenderPassColorAttachment{{
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
	staging, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "linear_ramp_readback", Size: stagingSize,
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, err
	}
	defer staging.Release()

	enc.CopyTextureToBuffer(outTex, staging, []webgpu.BufferTextureCopy{{
		BufferLayout: webgpu.ImageDataLayout{BytesPerRow: alignedRow, RowsPerImage: uint32(nh)}, //nolint:gosec
		TextureBase:  webgpu.ImageCopyTexture{Texture: outTex, MipLevel: 0, Aspect: types.TextureAspectAll},
		Size:         webgpu.Extent3D{Width: uint32(nw), Height: uint32(nh), DepthOrArrayLayers: 1}, //nolint:gosec
	}})
	cmd, err := enc.Finish()
	if err != nil {
		return nil, err
	}
	defer cmd.Release()
	if _, err := queue.Submit(cmd); err != nil {
		return nil, err
	}
	device.Poll(webgpu.PollWait)
	if err := staging.Map(context.Background(), webgpu.MapModeRead, 0, stagingSize); err != nil {
		return nil, err
	}
	mapped, err := staging.MappedRange(0, stagingSize)
	if err != nil {
		_ = staging.Unmap()
		return nil, err
	}
	src := mapped.Bytes()
	out := make([]byte, needOut)
	if alignedRow == tightRow {
		copy(out, src[:needOut])
	} else {
		for y := 0; y < nh; y++ {
			copy(out[y*nw*4:(y+1)*nw*4], src[y*int(alignedRow):y*int(alignedRow)+nw*4])
		}
	}
	mapped.Release()
	_ = staging.Unmap()
	return out, nil
}
