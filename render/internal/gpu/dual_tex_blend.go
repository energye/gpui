//go:build !nogpu

package gpu

import (
	"context"
	"fmt"
	"sync"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

// dual-tex advanced blend shader: sample dest + src, write composited premul RGBA.
// Mode codes: 1=Multiply, 2=Screen, 3=Overlay (match render.Blend* iota).
const dualTexBlendWGSL = `
struct VSOut {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

struct Params {
    mode: u32,
    _pad0: u32,
    _pad1: u32,
    _pad2: u32,
}

@group(0) @binding(0) var dst_tex: texture_2d<f32>;
@group(0) @binding(1) var src_tex: texture_2d<f32>;
@group(0) @binding(2) var samp: sampler;
@group(0) @binding(3) var<uniform> params: Params;

@vertex
fn vs_main(@builtin(vertex_index) vi: u32) -> VSOut {
    // Fullscreen triangle
    var p = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>( 3.0, -1.0),
        vec2<f32>(-1.0,  3.0),
    );
    var uv = array<vec2<f32>, 3>(
        vec2<f32>(0.0, 1.0),
        vec2<f32>(2.0, 1.0),
        vec2<f32>(0.0, -1.0),
    );
    var o: VSOut;
    o.pos = vec4<f32>(p[vi], 0.0, 1.0);
    o.uv = uv[vi];
    return o;
}

fn unpremul(c: vec4<f32>) -> vec4<f32> {
    if (c.a <= 0.0001) {
        return vec4<f32>(0.0, 0.0, 0.0, 0.0);
    }
    return vec4<f32>(clamp(c.rgb / c.a, vec3<f32>(0.0), vec3<f32>(1.0)), c.a);
}

fn blend_channel_multiply(cb: f32, cs: f32) -> f32 { return cb * cs; }
fn blend_channel_screen(cb: f32, cs: f32) -> f32 { return cb + cs - cb * cs; }
fn blend_channel_overlay(cb: f32, cs: f32) -> f32 {
    if (cb <= 0.5) {
        return 2.0 * cb * cs;
    }
    return 1.0 - 2.0 * (1.0 - cb) * (1.0 - cs);
}

fn blend_fn(mode: u32, cb: vec3<f32>, cs: vec3<f32>) -> vec3<f32> {
    if (mode == 2u) {
        return vec3<f32>(
            blend_channel_screen(cb.r, cs.r),
            blend_channel_screen(cb.g, cs.g),
            blend_channel_screen(cb.b, cs.b),
        );
    }
    if (mode == 3u) {
        return vec3<f32>(
            blend_channel_overlay(cb.r, cs.r),
            blend_channel_overlay(cb.g, cs.g),
            blend_channel_overlay(cb.b, cs.b),
        );
    }
    // Multiply (1) default
    return vec3<f32>(
        blend_channel_multiply(cb.r, cs.r),
        blend_channel_multiply(cb.g, cs.g),
        blend_channel_multiply(cb.b, cs.b),
    );
}

// W3C Compositing: backdrop b, source s; advanced color B(Cb,Cs)
// Co = (1-αb)*Cs + (1-αs)*Cb + αb*αs*B(Cb,Cs)
// αo = αs + αb*(1-αs)
@fragment
fn fs_main(in: VSOut) -> @location(0) vec4<f32> {
    let dp = textureSample(dst_tex, samp, in.uv);
    let sp = textureSample(src_tex, samp, in.uv);
    let d = unpremul(dp);
    let s = unpremul(sp);
    let ab = d.a;
    let as_ = s.a;
    let bcol = blend_fn(params.mode, d.rgb, s.rgb);
    let co = (1.0 - ab) * s.rgb + (1.0 - as_) * d.rgb + ab * as_ * bcol;
    let ao = as_ + ab * (1.0 - as_);
    // Premultiply output for later SourceOver blit of opaque interiors.
    return vec4<f32>(co * ao, ao);
}
`

// dualTexBlendCache holds reusable GPU objects for dual-texture advanced blend.
type dualTexBlendCache struct {
	mu       sync.Mutex
	device   *webgpu.Device
	shader   *webgpu.ShaderModule
	bgl      *webgpu.BindGroupLayout
	pipeLay  *webgpu.PipelineLayout
	pipeline *webgpu.RenderPipeline
	sampler  *webgpu.Sampler
	uniform  *webgpu.Buffer
}

func (c *dualTexBlendCache) release() {
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
	if c.uniform != nil {
		c.uniform.Release()
		c.uniform = nil
	}
	c.device = nil
}

func (c *dualTexBlendCache) ensure(device *webgpu.Device) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.device != nil && c.device != device {
		// Device changed — drop cache.
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
		if c.uniform != nil {
			c.uniform.Release()
		}
		c.pipeline, c.pipeLay, c.bgl, c.shader, c.sampler, c.uniform = nil, nil, nil, nil, nil, nil
	}
	c.device = device
	if c.pipeline != nil {
		return nil
	}
	shader, err := device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "dual_tex_advanced_blend",
		WGSL:  dualTexBlendWGSL,
	})
	if err != nil {
		return fmt.Errorf("dual-tex blend shader: %w", err)
	}
	bgl, err := device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "dual_tex_blend_bgl",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: types.ShaderStageFragment,
				Texture: &types.TextureBindingLayout{
					SampleType:    types.TextureSampleTypeFloat,
					ViewDimension: types.TextureViewDimension2D,
				},
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
			{
				Binding:    3,
				Visibility: types.ShaderStageFragment,
				Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: 16},
			},
		},
	})
	if err != nil {
		shader.Release()
		return fmt.Errorf("dual-tex blend bgl: %w", err)
	}
	pipeLay, err := device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label:            "dual_tex_blend_pipe_layout",
		BindGroupLayouts: []*webgpu.BindGroupLayout{bgl},
	})
	if err != nil {
		bgl.Release()
		shader.Release()
		return fmt.Errorf("dual-tex blend pipe layout: %w", err)
	}
	// Replace blend: write fully composited result.
	replace := types.BlendState{
		Color: types.BlendComponent{
			SrcFactor: types.BlendFactorOne,
			DstFactor: types.BlendFactorZero,
			Operation: types.BlendOperationAdd,
		},
		Alpha: types.BlendComponent{
			SrcFactor: types.BlendFactorOne,
			DstFactor: types.BlendFactorZero,
			Operation: types.BlendOperationAdd,
		},
	}
	pipe, err := device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "dual_tex_advanced_blend_pipe",
		Layout: pipeLay,
		Vertex: webgpu.VertexState{
			Module:     shader,
			EntryPoint: "vs_main",
		},
		Fragment: &webgpu.FragmentState{
			Module:     shader,
			EntryPoint: "fs_main",
			Targets: []types.ColorTargetState{{
				Format:    types.TextureFormatRGBA8Unorm,
				Blend:     &replace,
				WriteMask: types.ColorWriteMaskAll,
			}},
		},
		Primitive:   triangleListPrimitive(),
		Multisample: types.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
	})
	if err != nil {
		pipeLay.Release()
		bgl.Release()
		shader.Release()
		return fmt.Errorf("dual-tex blend pipeline: %w", err)
	}
	samp, err := device.CreateSampler(&webgpu.SamplerDescriptor{
		Label:        "dual_tex_blend_samp",
		AddressModeU: types.AddressModeClampToEdge,
		AddressModeV: types.AddressModeClampToEdge,
		AddressModeW: types.AddressModeClampToEdge,
		MagFilter:    types.FilterModeNearest,
		MinFilter:    types.FilterModeNearest,
		MipmapFilter: types.MipmapFilterModeNearest,
		Anisotropy:   1,
	})
	if err != nil {
		pipe.Release()
		pipeLay.Release()
		bgl.Release()
		shader.Release()
		return fmt.Errorf("dual-tex blend sampler: %w", err)
	}
	uni, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "dual_tex_blend_uniform",
		Size:  16,
		Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
	})
	if err != nil {
		samp.Release()
		pipe.Release()
		pipeLay.Release()
		bgl.Release()
		shader.Release()
		return fmt.Errorf("dual-tex blend uniform: %w", err)
	}
	c.shader = shader
	c.bgl = bgl
	c.pipeLay = pipeLay
	c.pipeline = pipe
	c.sampler = samp
	c.uniform = uni
	return nil
}

// dualTexAdvancedBlend composites src over dst using GPU dual-texture sampling.
// dstRGBA/srcRGBA are tight premul RGBA8 (bw*bh*4). mode is Multiply/Screen/Overlay.
func dualTexAdvancedBlend(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *dualTexBlendCache,
	dstRGBA, srcRGBA []byte,
	bw, bh int,
	mode render.BlendMode,
) ([]byte, error) {
	if device == nil || queue == nil || cache == nil {
		return nil, fmt.Errorf("dual-tex blend: nil device/queue/cache")
	}
	if bw <= 0 || bh <= 0 {
		return nil, fmt.Errorf("dual-tex blend: empty size")
	}
	need := bw * bh * 4
	if len(dstRGBA) < need || len(srcRGBA) < need {
		return nil, fmt.Errorf("dual-tex blend: short buffers")
	}
	if err := cache.ensure(device); err != nil {
		return nil, err
	}

	modeU := uint32(1)
	switch mode {
	case render.BlendScreen:
		modeU = 2
	case render.BlendOverlay:
		modeU = 3
	default:
		modeU = 1 // Multiply
	}
	// Uniform: mode + pad (16 bytes)
	uniData := make([]byte, 16)
	uniData[0] = byte(modeU)
	uniData[1] = byte(modeU >> 8)
	uniData[2] = byte(modeU >> 16)
	uniData[3] = byte(modeU >> 24)
	if err := queue.WriteBuffer(cache.uniform, 0, uniData); err != nil {
		return nil, fmt.Errorf("dual-tex uniform write: %w", err)
	}

	mkTex := func(label string, data []byte, usage types.TextureUsage) (*webgpu.Texture, *webgpu.TextureView, error) {
		tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
			Label: label,
			Size: webgpu.Extent3D{
				Width:              uint32(bw), //nolint:gosec
				Height:             uint32(bh), //nolint:gosec
				DepthOrArrayLayers: 1,
			},
			MipLevelCount: 1,
			SampleCount:   1,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatRGBA8Unorm,
			Usage:         usage,
		})
		if err != nil {
			return nil, nil, err
		}
		view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
			Label:         label + "_view",
			Format:        types.TextureFormatRGBA8Unorm,
			Dimension:     types.TextureViewDimension2D,
			Aspect:        types.TextureAspectAll,
			MipLevelCount: 1,
		})
		if err != nil {
			tex.Release()
			return nil, nil, err
		}
		if data != nil {
			// Align pitch to 256 for multi-row uploads.
			tight := uint32(bw * 4) //nolint:gosec
			aligned := alignTextureBytesPerRow(tight)
			upload := data
			if aligned != tight && bh > 1 {
				padded := make([]byte, int(aligned)*bh)
				for y := 0; y < bh; y++ {
					copy(padded[y*int(aligned):y*int(aligned)+bw*4], data[y*bw*4:(y+1)*bw*4])
				}
				upload = padded
			}
			if err := queue.WriteTexture(
				&webgpu.ImageCopyTexture{Texture: tex, MipLevel: 0},
				upload,
				&webgpu.ImageDataLayout{
					Offset:       0,
					BytesPerRow:  aligned,
					RowsPerImage: uint32(bh), //nolint:gosec
				},
				&webgpu.Extent3D{Width: uint32(bw), Height: uint32(bh), DepthOrArrayLayers: 1}, //nolint:gosec
			); err != nil {
				view.Release()
				tex.Release()
				return nil, nil, err
			}
		}
		return tex, view, nil
	}

	dstTex, dstView, err := mkTex("dual_tex_dst", dstRGBA,
		types.TextureUsageTextureBinding|types.TextureUsageCopyDst)
	if err != nil {
		return nil, fmt.Errorf("dual-tex dst tex: %w", err)
	}
	defer dstView.Release()
	defer dstTex.Release()

	srcTex, srcView, err := mkTex("dual_tex_src", srcRGBA,
		types.TextureUsageTextureBinding|types.TextureUsageCopyDst)
	if err != nil {
		return nil, fmt.Errorf("dual-tex src tex: %w", err)
	}
	defer srcView.Release()
	defer srcTex.Release()

	outTex, outView, err := mkTex("dual_tex_out", nil,
		types.TextureUsageRenderAttachment|types.TextureUsageCopySrc|types.TextureUsageTextureBinding)
	if err != nil {
		return nil, fmt.Errorf("dual-tex out tex: %w", err)
	}
	defer outView.Release()
	defer outTex.Release()

	cache.mu.Lock()
	bgl := cache.bgl
	pipeline := cache.pipeline
	sampler := cache.sampler
	uniform := cache.uniform
	cache.mu.Unlock()

	bg, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label:  "dual_tex_blend_bg",
		Layout: bgl,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, TextureView: dstView},
			{Binding: 1, TextureView: srcView},
			{Binding: 2, Sampler: sampler},
			{Binding: 3, Buffer: uniform, Offset: 0, Size: 16},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dual-tex bind group: %w", err)
	}
	defer bg.Release()

	enc, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: "dual_tex_blend_enc"})
	if err != nil {
		return nil, fmt.Errorf("dual-tex encoder: %w", err)
	}
	rp, err := enc.BeginRenderPass(&webgpu.RenderPassDescriptor{
		Label: "dual_tex_blend_pass",
		ColorAttachments: []webgpu.RenderPassColorAttachment{{
			View:       outView,
			LoadOp:     types.LoadOpClear,
			StoreOp:    types.StoreOpStore,
			ClearValue: types.Color{R: 0, G: 0, B: 0, A: 0},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("dual-tex begin pass: %w", err)
	}
	rp.SetPipeline(pipeline)
	rp.SetBindGroup(0, bg, nil)
	rp.Draw(3, 1, 0, 0)
	rp.End()

	// Readback staging
	tightRow := uint32(bw * 4) //nolint:gosec
	alignedRow := alignTextureBytesPerRow(tightRow)
	stagingSize := uint64(alignedRow) * uint64(bh)
	staging, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "dual_tex_readback",
		Size:  stagingSize,
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("dual-tex staging: %w", err)
	}
	defer staging.Release()

	enc.CopyTextureToBuffer(outTex, staging, []webgpu.BufferTextureCopy{{
		BufferLayout: webgpu.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  alignedRow,
			RowsPerImage: uint32(bh), //nolint:gosec
		},
		TextureBase: webgpu.ImageCopyTexture{
			Texture: outTex, MipLevel: 0, Origin: webgpu.Origin3D{}, Aspect: types.TextureAspectAll,
		},
		Size: webgpu.Extent3D{Width: uint32(bw), Height: uint32(bh), DepthOrArrayLayers: 1}, //nolint:gosec
	}})

	cmd, err := enc.Finish()
	if err != nil {
		return nil, fmt.Errorf("dual-tex finish: %w", err)
	}
	defer cmd.Release()
	if _, err := queue.Submit(cmd); err != nil {
		return nil, fmt.Errorf("dual-tex submit: %w", err)
	}
	// Wait for GPU before map (matches texture readback smoke tests).
	device.Poll(webgpu.PollWait)

	if err := staging.Map(context.Background(), webgpu.MapModeRead, 0, stagingSize); err != nil {
		return nil, fmt.Errorf("dual-tex map: %w", err)
	}
	mapped, err := staging.MappedRange(0, stagingSize)
	if err != nil {
		_ = staging.Unmap()
		return nil, fmt.Errorf("dual-tex mapped range: %w", err)
	}
	src := mapped.Bytes()
	out := make([]byte, need)
	if alignedRow == tightRow {
		copy(out, src[:need])
	} else {
		for y := 0; y < bh; y++ {
			copy(out[y*bw*4:(y+1)*bw*4], src[y*int(alignedRow):y*int(alignedRow)+bw*4])
		}
	}
	mapped.Release()
	_ = staging.Unmap()
	return out, nil
}
