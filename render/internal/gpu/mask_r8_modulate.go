//go:build !nogpu

package gpu

import (
	"context"
	"fmt"
	"sync"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// L.06: modulate premul source by an R8 mask texture in a fragment shader.
// Geometry may be staged; mask application is true R8 GPU sampling (not CPU bake).
const maskR8ModulateWGSL = `
struct VSOut {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@group(0) @binding(0) var src_tex: texture_2d<f32>;
@group(0) @binding(1) var mask_tex: texture_2d<f32>;
@group(0) @binding(2) var samp: sampler;

@vertex
fn vs_main(@builtin(vertex_index) vi: u32) -> VSOut {
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

@fragment
fn fs_main(in: VSOut) -> @location(0) vec4<f32> {
    let s = textureSample(src_tex, samp, in.uv);
    let m = textureSample(mask_tex, samp, in.uv).r;
    // Premul source modulated by mask alpha.
    return s * m;
}
`

type maskR8Cache struct {
	mu       sync.Mutex
	device   *webgpu.Device
	shader   *webgpu.ShaderModule
	bgl      *webgpu.BindGroupLayout
	pipeLay  *webgpu.PipelineLayout
	pipeline *webgpu.RenderPipeline
	sampler  *webgpu.Sampler
}

func (c *maskR8Cache) release() {
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

func (c *maskR8Cache) ensure(device *webgpu.Device) error {
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
		Label: "mask_r8_modulate",
		WGSL:  maskR8ModulateWGSL,
	})
	if err != nil {
		return fmt.Errorf("mask r8 shader: %w", err)
	}
	bgl, err := device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "mask_r8_bgl",
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
		},
	})
	if err != nil {
		shader.Release()
		return fmt.Errorf("mask r8 bgl: %w", err)
	}
	pipeLay, err := device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label: "mask_r8_pipe_layout", BindGroupLayouts: []*webgpu.BindGroupLayout{bgl},
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
		Label:  "mask_r8_modulate_pipe",
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
		return fmt.Errorf("mask r8 pipeline: %w", err)
	}
	samp, err := device.CreateSampler(&webgpu.SamplerDescriptor{
		Label:        "mask_r8_samp",
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
		return err
	}
	c.shader, c.bgl, c.pipeLay, c.pipeline, c.sampler = shader, bgl, pipeLay, pipe, samp
	return nil
}

// maskR8Modulate multiplies premul RGBA source by R8 mask on GPU.
// srcRGBA is bw*bh*4, maskR8 is bw*bh (tight). Returns premul RGBA result.
func maskR8Modulate(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *maskR8Cache,
	srcRGBA, maskR8 []byte,
	bw, bh int,
) ([]byte, error) {
	if device == nil || queue == nil || cache == nil {
		return nil, fmt.Errorf("mask r8: nil device/queue/cache")
	}
	needRGBA := bw * bh * 4
	needR8 := bw * bh
	if bw <= 0 || bh <= 0 || len(srcRGBA) < needRGBA || len(maskR8) < needR8 {
		return nil, fmt.Errorf("mask r8: bad sizes")
	}
	if err := cache.ensure(device); err != nil {
		return nil, err
	}

	mkRGBA := func(label string, data []byte, usage types.TextureUsage) (*webgpu.Texture, *webgpu.TextureView, error) {
		tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
			Label:         label,
			Size:          webgpu.Extent3D{Width: uint32(bw), Height: uint32(bh), DepthOrArrayLayers: 1}, //nolint:gosec
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatRGBA8Unorm, Usage: usage,
		})
		if err != nil {
			return nil, nil, err
		}
		view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
			Label: label + "_view", Format: types.TextureFormatRGBA8Unorm,
			Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
		})
		if err != nil {
			tex.Release()
			return nil, nil, err
		}
		if data != nil {
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
				&webgpu.ImageDataLayout{BytesPerRow: aligned, RowsPerImage: uint32(bh)},        //nolint:gosec
				&webgpu.Extent3D{Width: uint32(bw), Height: uint32(bh), DepthOrArrayLayers: 1}, //nolint:gosec
			); err != nil {
				view.Release()
				tex.Release()
				return nil, nil, err
			}
		}
		return tex, view, nil
	}

	mkR8 := func(label string, data []byte) (*webgpu.Texture, *webgpu.TextureView, error) {
		tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
			Label:         label,
			Size:          webgpu.Extent3D{Width: uint32(bw), Height: uint32(bh), DepthOrArrayLayers: 1}, //nolint:gosec
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatR8Unorm,
			Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
		})
		if err != nil {
			return nil, nil, err
		}
		view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
			Label: label + "_view", Format: types.TextureFormatR8Unorm,
			Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
		})
		if err != nil {
			tex.Release()
			return nil, nil, err
		}
		tight := uint32(bw) //nolint:gosec
		aligned := alignTextureBytesPerRow(tight)
		upload := data
		if aligned != tight && bh > 1 {
			padded := make([]byte, int(aligned)*bh)
			for y := 0; y < bh; y++ {
				copy(padded[y*int(aligned):y*int(aligned)+bw], data[y*bw:(y+1)*bw])
			}
			upload = padded
		} else if aligned != tight && bh == 1 {
			// single row still needs aligned BytesPerRow for some backends
			padded := make([]byte, int(aligned))
			copy(padded, data[:bw])
			upload = padded
		}
		if err := queue.WriteTexture(
			&webgpu.ImageCopyTexture{Texture: tex, MipLevel: 0},
			upload,
			&webgpu.ImageDataLayout{BytesPerRow: aligned, RowsPerImage: uint32(bh)},        //nolint:gosec
			&webgpu.Extent3D{Width: uint32(bw), Height: uint32(bh), DepthOrArrayLayers: 1}, //nolint:gosec
		); err != nil {
			view.Release()
			tex.Release()
			return nil, nil, err
		}
		return tex, view, nil
	}

	srcTex, srcView, err := mkRGBA("mask_r8_src", srcRGBA, types.TextureUsageTextureBinding|types.TextureUsageCopyDst)
	if err != nil {
		return nil, fmt.Errorf("mask r8 src: %w", err)
	}
	defer srcView.Release()
	defer srcTex.Release()

	maskTex, maskView, err := mkR8("mask_r8_mask", maskR8)
	if err != nil {
		return nil, fmt.Errorf("mask r8 mask: %w", err)
	}
	defer maskView.Release()
	defer maskTex.Release()

	outTex, outView, err := mkRGBA("mask_r8_out", nil,
		types.TextureUsageRenderAttachment|types.TextureUsageCopySrc|types.TextureUsageTextureBinding)
	if err != nil {
		return nil, fmt.Errorf("mask r8 out: %w", err)
	}
	defer outView.Release()
	defer outTex.Release()

	cache.mu.Lock()
	bgl, pipeline, sampler := cache.bgl, cache.pipeline, cache.sampler
	cache.mu.Unlock()

	bg, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label:  "mask_r8_bg",
		Layout: bgl,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, TextureView: srcView},
			{Binding: 1, TextureView: maskView},
			{Binding: 2, Sampler: sampler},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("mask r8 bg: %w", err)
	}
	defer bg.Release()

	enc, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: "mask_r8_enc"})
	if err != nil {
		return nil, err
	}
	rp, err := enc.BeginRenderPass(&webgpu.RenderPassDescriptor{
		Label: "mask_r8_pass",
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

	tightRow := uint32(bw * 4) //nolint:gosec
	alignedRow := alignTextureBytesPerRow(tightRow)
	stagingSize := uint64(alignedRow) * uint64(bh)
	staging, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "mask_r8_readback", Size: stagingSize,
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, err
	}
	defer staging.Release()

	enc.CopyTextureToBuffer(outTex, staging, []webgpu.BufferTextureCopy{{
		BufferLayout: webgpu.ImageDataLayout{BytesPerRow: alignedRow, RowsPerImage: uint32(bh)}, //nolint:gosec
		TextureBase:  webgpu.ImageCopyTexture{Texture: outTex, MipLevel: 0, Aspect: types.TextureAspectAll},
		Size:         webgpu.Extent3D{Width: uint32(bw), Height: uint32(bh), DepthOrArrayLayers: 1}, //nolint:gosec
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
	out := make([]byte, needRGBA)
	if alignedRow == tightRow {
		copy(out, src[:needRGBA])
	} else {
		for y := 0; y < bh; y++ {
			copy(out[y*bw*4:(y+1)*bw*4], src[y*int(alignedRow):y*int(alignedRow)+bw*4])
		}
	}
	mapped.Release()
	_ = staging.Unmap()
	return out, nil
}
