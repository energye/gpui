//go:build !nogpu

package gpu

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/energye/gpui/gpu/hal"
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
	shader   hal.ShaderModule
	bgl      hal.BindGroupLayout
	pipeLay  hal.PipelineLayout
	pipeline hal.RenderPipeline
	sampler  hal.Sampler
}

func (c *maskR8Cache) release() {
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

func (c *maskR8Cache) ensure(device *webgpu.Device) error {
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
		Label: "mask_r8_modulate",
		WGSL:  maskR8ModulateWGSL,
	})
	if err != nil {
		return fmt.Errorf("mask r8 shader: %w", err)
	}
	bgl, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
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
		shader.Destroy()
		return fmt.Errorf("mask r8 bgl: %w", err)
	}
	pipeLay, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label: "mask_r8_pipe_layout", BindGroupLayouts: []hal.BindGroupLayout{bgl},
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
		Label:  "mask_r8_modulate_pipe",
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
		return fmt.Errorf("mask r8 pipeline: %w", err)
	}
	samp, err := device.CreateSampler(&hal.SamplerDescriptor{
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
		pipe.Destroy()
		pipeLay.Destroy()
		bgl.Destroy()
		shader.Destroy()
		return err
	}
	c.shader, c.bgl, c.pipeLay, c.pipeline, c.sampler = shader, bgl, pipeLay, pipe, samp
	return nil
}

// maskR8Modulate multiplies premul RGBA source by R8 mask on GPU.
// srcRGBA is bw*bh*4, maskR8 is bw*bh (tight). Returns premul RGBA result.
func maskR8Modulate(
	device *webgpu.Device,
	queue hal.Queue,
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

	mkRGBA := func(label string, data []byte, usage types.TextureUsage) (hal.Texture, hal.TextureView, error) {
		tex, err := device.CreateTexture(&hal.TextureDescriptor{
			Label:         label,
			Size:          hal.Extent3D{Width: uint32(bw), Height: uint32(bh), DepthOrArrayLayers: 1}, //nolint:gosec
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatRGBA8Unorm, Usage: usage,
		})
		if err != nil {
			return nil, nil, err
		}
		view, err := device.CreateTextureView(tex, &hal.TextureViewDescriptor{
			Label: label + "_view", Format: types.TextureFormatRGBA8Unorm,
			Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
		})
		if err != nil {
			tex.Destroy()
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
				&hal.ImageCopyTexture{Texture: tex, MipLevel: 0},
				upload,
				&hal.ImageDataLayout{BytesPerRow: aligned, RowsPerImage: uint32(bh)},        //nolint:gosec
				&hal.Extent3D{Width: uint32(bw), Height: uint32(bh), DepthOrArrayLayers: 1}, //nolint:gosec
			); err != nil {
				view.Destroy()
				tex.Destroy()
				return nil, nil, err
			}
		}
		return tex, view, nil
	}

	mkR8 := func(label string, data []byte) (hal.Texture, hal.TextureView, error) {
		tex, err := device.CreateTexture(&hal.TextureDescriptor{
			Label:         label,
			Size:          hal.Extent3D{Width: uint32(bw), Height: uint32(bh), DepthOrArrayLayers: 1}, //nolint:gosec
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatR8Unorm,
			Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
		})
		if err != nil {
			return nil, nil, err
		}
		view, err := device.CreateTextureView(tex, &hal.TextureViewDescriptor{
			Label: label + "_view", Format: types.TextureFormatR8Unorm,
			Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
		})
		if err != nil {
			tex.Destroy()
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
			&hal.ImageCopyTexture{Texture: tex, MipLevel: 0},
			upload,
			&hal.ImageDataLayout{BytesPerRow: aligned, RowsPerImage: uint32(bh)},        //nolint:gosec
			&hal.Extent3D{Width: uint32(bw), Height: uint32(bh), DepthOrArrayLayers: 1}, //nolint:gosec
		); err != nil {
			view.Destroy()
			tex.Destroy()
			return nil, nil, err
		}
		return tex, view, nil
	}

	srcTex, srcView, err := mkRGBA("mask_r8_src", srcRGBA, types.TextureUsageTextureBinding|types.TextureUsageCopyDst)
	if err != nil {
		return nil, fmt.Errorf("mask r8 src: %w", err)
	}
	defer srcView.Destroy()
	defer srcTex.Destroy()

	maskTex, maskView, err := mkR8("mask_r8_mask", maskR8)
	if err != nil {
		return nil, fmt.Errorf("mask r8 mask: %w", err)
	}
	defer maskView.Destroy()
	defer maskTex.Destroy()

	outTex, outView, err := mkRGBA("mask_r8_out", nil,
		types.TextureUsageRenderAttachment|types.TextureUsageCopySrc|types.TextureUsageTextureBinding)
	if err != nil {
		return nil, fmt.Errorf("mask r8 out: %w", err)
	}
	defer outView.Destroy()
	defer outTex.Destroy()

	cache.mu.Lock()
	bgl, pipeline, sampler := cache.bgl, cache.pipeline, cache.sampler
	cache.mu.Unlock()

	bg, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "mask_r8_bg",
		Layout: bgl,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, TextureView: srcView},
			{Binding: 1, TextureView: maskView},
			{Binding: 2, Sampler: sampler},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("mask r8 bg: %w", err)
	}
	defer bg.Destroy()

	enc, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "mask_r8_enc"})
	if err != nil {
		return nil, err
	}
	rp, err := enc.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "mask_r8_pass",
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

	tightRow := uint32(bw * 4) //nolint:gosec
	alignedRow := alignTextureBytesPerRow(tightRow)
	stagingSize := uint64(alignedRow) * uint64(bh)
	staging, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label: "mask_r8_readback", Size: stagingSize,
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, err
	}
	defer staging.Destroy()

	enc.CopyTextureToBuffer(outTex, staging, []hal.BufferTextureCopy{{
		BufferLayout: hal.ImageDataLayout{BytesPerRow: alignedRow, RowsPerImage: uint32(bh)}, //nolint:gosec
		TextureBase:  hal.ImageCopyTexture{Texture: outTex, MipLevel: 0, Aspect: types.TextureAspectAll},
		Size:         hal.Extent3D{Width: uint32(bw), Height: uint32(bh), DepthOrArrayLayers: 1}, //nolint:gosec
	}})
	cmd, err := enc.Finish()
	if err != nil {
		return nil, err
	}
	defer cmd.Release()
	if _, err := queue.Submit(cmd); err != nil {
		return nil, err
	}
	device.Poll(hal.PollWait)
	mapping, err := device.MapBuffer(staging, 0, stagingSize)
	if err != nil {
		return nil, err
	}
	src := unsafe.Slice((*byte)(mapping.Ptr), stagingSize) //nolint:gosec // hal.BufferMapping opaque pointer
	out := make([]byte, needRGBA)
	if alignedRow == tightRow {
		copy(out, src[:needRGBA])
	} else {
		for y := 0; y < bh; y++ {
			copy(out[y*bw*4:(y+1)*bw*4], src[y*int(alignedRow):y*int(alignedRow)+bw*4])
		}
	}
	_ = device.UnmapBuffer(staging)
	return out, nil
}
