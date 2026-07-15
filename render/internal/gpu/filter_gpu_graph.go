//go:build !nogpu

package gpu

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

// F.03: true multi-RT GPU image filter graph with texture ping-pong.
// Supported: Blur/BlurXY/Grayscale/Invert/ColorMatrix/DropShadow.

const filterGPUMaxPixels = 4 * 1024 * 1024
const filterGPUUniformSize = 128

const filterGPUGraphWGSL = `
struct Params {
    size: vec2<f32>,
    direction: vec2<f32>,
    mode: u32,           // 0=blur 1=gray 2=invert 3=matrix 4=shadowExtract 5=shadowComposite 6=copy
    radius: u32,
    offset: vec2<f32>,   // shadow offset in texels
    color: vec4<f32>,    // shadow color (straight RGBA 0-1)
    matrix: array<vec4<f32>, 5>, // 20 floats for 4x5 color matrix (CPU 0-255 space biases /255)
}

@group(0) @binding(0) var src_tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@group(0) @binding(2) var<uniform> p: Params;
@group(0) @binding(3) var aux_tex: texture_2d<f32>;

struct VSOut {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vi: u32) -> VSOut {
    var out: VSOut;
    let x = f32(i32(vi & 1u) * 4 - 1);
    let y = f32(i32(vi >> 1u) * 4 - 1);
    out.pos = vec4<f32>(x, y, 0.0, 1.0);
    out.uv = vec2<f32>((x + 1.0) * 0.5, (1.0 - y) * 0.5);
    return out;
}

fn sample_clamped(tex: texture_2d<f32>, uv: vec2<f32>) -> vec4<f32> {
    if (uv.x < 0.0 || uv.x > 1.0 || uv.y < 0.0 || uv.y > 1.0) {
        return vec4<f32>(0.0);
    }
    return textureSampleLevel(tex, samp, uv, 0.0);
}

@fragment
fn fs_main(in: VSOut) -> @location(0) vec4<f32> {
    if p.mode == 6u {
        return textureSampleLevel(src_tex, samp, in.uv, 0.0);
    }
    if p.mode == 4u {
        // Shadow extract: sample source alpha at -offset, colorize.
        let uv_s = in.uv - p.offset / max(p.size, vec2<f32>(1.0, 1.0));
        let a = sample_clamped(src_tex, uv_s).a;
        let sa = p.color.a * a;
        return vec4<f32>(p.color.rgb * sa, sa);
    }
    if p.mode == 5u {
        // Content (aux) over shadow (src).
        let shadow = textureSampleLevel(src_tex, samp, in.uv, 0.0);
        let orig = textureSampleLevel(aux_tex, samp, in.uv, 0.0);
        return orig + shadow * (1.0 - orig.a);
    }
    let c0 = textureSampleLevel(src_tex, samp, in.uv, 0.0);
    if p.mode == 1u {
        let y = 0.2126 * c0.r + 0.7152 * c0.g + 0.0722 * c0.b;
        return vec4<f32>(y, y, y, c0.a);
    }
    if p.mode == 2u {
        return vec4<f32>(1.0 - c0.r, 1.0 - c0.g, 1.0 - c0.b, c0.a);
    }
    if p.mode == 3u {
        // Color matrix in straight-alpha 0-1; matrix bias already /255.
        var r = 0.0;
        var g = 0.0;
        var b = 0.0;
        var a = c0.a;
        if (c0.a > 0.0001) {
            r = c0.r / c0.a;
            g = c0.g / c0.a;
            b = c0.b / c0.a;
        }
        // CPU applies matrix on 0-255 channels; convert: m_rgb * 255 -> scale coeffs for 0-1:
        // newR_255 = m0*r255 + m1*g255 + m2*b255 + m3*a255 + m4
        // newR_01 = newR_255/255 = m0*r + m1*g + m2*b + m3*a + m4/255
        let m0 = p.matrix[0];
        let m1 = p.matrix[1];
        let m2 = p.matrix[2];
        let m3 = p.matrix[3];
        let m4 = p.matrix[4];
        // packed as rows of 4: [m0 m1 m2 m3] [m4 m5 m6 m7] [m8 m9 m10 m11] [m12 m13 m14 m15] [m16 m17 m18 m19]
        // wait we pack as 5 vec4 sequential of 20 floats:
        // mat[0]=m0..m3, mat[1]=m4..m7, mat[2]=m8..m11, mat[3]=m12..m15, mat[4]=m16..m19
        let nr = m0.x*r + m0.y*g + m0.z*b + m0.w*a + m1.x;
        let ng = m1.y*r + m1.z*g + m1.w*b + m2.x*a + m2.y;
        let nb = m2.z*r + m2.w*g + m3.x*b + m3.y*a + m3.z;
        let na = m3.w*r + m4.x*g + m4.y*b + m4.z*a + m4.w;
        let aa = clamp(na, 0.0, 1.0);
        let rr = clamp(nr, 0.0, 1.0);
        let gg = clamp(ng, 0.0, 1.0);
        let bb = clamp(nb, 0.0, 1.0);
        return vec4<f32>(rr * aa, gg * aa, bb * aa, aa);
    }
    // Separable box blur
    let texel = p.direction / max(p.size, vec2<f32>(1.0, 1.0));
    let rad = i32(p.radius);
    var acc = vec4<f32>(0.0);
    var wsum = 0.0;
    for (var i = -8; i <= 8; i = i + 1) {
        if i < -rad || i > rad {
            continue;
        }
        let uv = in.uv + texel * f32(i);
        acc = acc + textureSampleLevel(src_tex, samp, uv, 0.0);
        wsum = wsum + 1.0;
    }
    return acc / max(wsum, 1.0);
}
`

type filterGPUCache struct {
	mu        sync.Mutex
	device    *webgpu.Device
	pipeline  *webgpu.RenderPipeline
	bgl       *webgpu.BindGroupLayout
	sampler   *webgpu.Sampler
	dummyTex  *webgpu.Texture
	dummyView *webgpu.TextureView
}

func (c *filterGPUCache) release() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.releaseUnlocked()
}

func (c *filterGPUCache) releaseUnlocked() {
	if c.pipeline != nil {
		c.pipeline.Release()
		c.pipeline = nil
	}
	if c.bgl != nil {
		c.bgl.Release()
		c.bgl = nil
	}
	if c.sampler != nil {
		c.sampler.Release()
		c.sampler = nil
	}
	if c.dummyView != nil {
		c.dummyView.Release()
		c.dummyView = nil
	}
	if c.dummyTex != nil {
		c.dummyTex.Release()
		c.dummyTex = nil
	}
	c.device = nil
}

func (c *filterGPUCache) ensure(device *webgpu.Device) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pipeline != nil && c.device == device {
		return nil
	}
	c.releaseUnlocked()
	c.device = device

	shader, err := device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "filter_gpu_graph",
		WGSL:  filterGPUGraphWGSL,
	})
	if err != nil {
		return err
	}
	defer shader.Release()

	bgl, err := device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "filter_gpu_bgl",
		Entries: []types.BindGroupLayoutEntry{
			{Binding: 0, Visibility: types.ShaderStageFragment, Texture: &types.TextureBindingLayout{
				SampleType: types.TextureSampleTypeFloat, ViewDimension: types.TextureViewDimension2D,
			}},
			{Binding: 1, Visibility: types.ShaderStageFragment, Sampler: &types.SamplerBindingLayout{
				Type: types.SamplerBindingTypeFiltering,
			}},
			{Binding: 2, Visibility: types.ShaderStageFragment, Buffer: &types.BufferBindingLayout{
				Type: types.BufferBindingTypeUniform, MinBindingSize: filterGPUUniformSize,
			}},
			{Binding: 3, Visibility: types.ShaderStageFragment, Texture: &types.TextureBindingLayout{
				SampleType: types.TextureSampleTypeFloat, ViewDimension: types.TextureViewDimension2D,
			}},
		},
	})
	if err != nil {
		return err
	}
	layout, err := device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label: "filter_gpu_pipe_layout", BindGroupLayouts: []*webgpu.BindGroupLayout{bgl},
	})
	if err != nil {
		bgl.Release()
		return err
	}
	defer layout.Release()

	replace := types.BlendState{
		Color: types.BlendComponent{SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorZero, Operation: types.BlendOperationAdd},
		Alpha: types.BlendComponent{SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorZero, Operation: types.BlendOperationAdd},
	}
	pipe, err := device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "filter_gpu_pipe",
		Layout: layout,
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
		bgl.Release()
		return err
	}
	samp, err := device.CreateSampler(&webgpu.SamplerDescriptor{
		Label:        "filter_gpu_samp",
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
		bgl.Release()
		return err
	}
	// 1x1 transparent dummy aux
	dtex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "filter_gpu_dummy",
		Size:          webgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA8Unorm,
		Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		samp.Release()
		pipe.Release()
		bgl.Release()
		return err
	}
	dview, err := device.CreateTextureView(dtex, &webgpu.TextureViewDescriptor{
		Label: "filter_gpu_dummy_view", Format: types.TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		dtex.Release()
		samp.Release()
		pipe.Release()
		bgl.Release()
		return err
	}

	c.pipeline = pipe
	c.bgl = bgl
	c.sampler = samp
	c.dummyTex = dtex
	c.dummyView = dview
	return nil
}

// runGPUFilterGraph executes multi-RT ping-pong filter nodes on GPU.
func runGPUFilterGraph(device *webgpu.Device, queue *webgpu.Queue, cache *filterGPUCache, src []byte, w, h int, nodes []render.ImageFilterNode) ([]byte, error) {
	if device == nil || queue == nil || cache == nil || w <= 0 || h <= 0 {
		return nil, fmt.Errorf("filter gpu: invalid args")
	}
	if w*h > filterGPUMaxPixels || len(src) < w*h*4 {
		return nil, fmt.Errorf("filter gpu: size")
	}
	if err := cache.ensure(device); err != nil {
		return nil, err
	}

	mkTex := func(label string, data []byte, usage types.TextureUsage) (*webgpu.Texture, *webgpu.TextureView, error) {
		tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
			Label:         label,
			Size:          webgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatRGBA8Unorm,
			Usage:  usage,
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
			bpr := alignTextureBytesPerRow(uint32(w * 4)) //nolint:gosec
			upload := data
			if int(bpr) != w*4 {
				padded := make([]byte, int(bpr)*h)
				for y := 0; y < h; y++ {
					copy(padded[y*int(bpr):y*int(bpr)+w*4], data[y*w*4:(y+1)*w*4])
				}
				upload = padded
			}
			if err := queue.WriteTexture(
				&webgpu.ImageCopyTexture{Texture: tex, MipLevel: 0},
				upload,
				&webgpu.ImageDataLayout{BytesPerRow: bpr, RowsPerImage: uint32(h)},           //nolint:gosec
				&webgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec
			); err != nil {
				view.Release()
				tex.Release()
				return nil, nil, err
			}
		}
		return tex, view, nil
	}

	usageRT := types.TextureUsageTextureBinding | types.TextureUsageRenderAttachment | types.TextureUsageCopySrc | types.TextureUsageCopyDst
	texA, viewA, err := mkTex("filter_rt_a", src, usageRT)
	if err != nil {
		return nil, err
	}
	defer texA.Release()
	defer viewA.Release()
	texB, viewB, err := mkTex("filter_rt_b", nil, usageRT)
	if err != nil {
		return nil, err
	}
	defer texB.Release()
	defer viewB.Release()
	texH, viewH, err := mkTex("filter_rt_hold", nil, usageRT)
	if err != nil {
		return nil, err
	}
	defer texH.Release()
	defer viewH.Release()

	curTex, curView := texA, viewA
	nxtTex, nxtView := texB, viewB

	type passArgs struct {
		mode                   uint32
		radius                 uint32
		dirX, dirY             float32
		offX, offY             float32
		colR, colG, colB, colA float32
		matrix                 [20]float32
		auxView                *webgpu.TextureView
		dstView                *webgpu.TextureView
		srcView                *webgpu.TextureView
	}

	doPass := func(a passArgs) error {
		ubuf := make([]byte, filterGPUUniformSize)
		putF32 := func(off int, v float32) {
			u := math.Float32bits(v)
			ubuf[off] = byte(u)
			ubuf[off+1] = byte(u >> 8)
			ubuf[off+2] = byte(u >> 16)
			ubuf[off+3] = byte(u >> 24)
		}
		putU32 := func(off int, v uint32) {
			ubuf[off] = byte(v)
			ubuf[off+1] = byte(v >> 8)
			ubuf[off+2] = byte(v >> 16)
			ubuf[off+3] = byte(v >> 24)
		}
		putF32(0, float32(w))
		putF32(4, float32(h))
		putF32(8, a.dirX)
		putF32(12, a.dirY)
		putU32(16, a.mode)
		putU32(20, a.radius)
		putF32(24, a.offX)
		putF32(28, a.offY)
		putF32(32, a.colR)
		putF32(36, a.colG)
		putF32(40, a.colB)
		putF32(44, a.colA)
		// matrix: 20 floats at offset 48; bias terms m4,m9,m14,m19 scaled /255
		for i := 0; i < 20; i++ {
			v := a.matrix[i]
			// bias columns: indices 4,9,14,19
			if i == 4 || i == 9 || i == 14 || i == 19 {
				v = v / 255.0
			}
			putF32(48+i*4, v)
		}

		uGPU, err := device.CreateBuffer(&webgpu.BufferDescriptor{
			Label: "filter_params", Size: filterGPUUniformSize,
			Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
		})
		if err != nil {
			return err
		}
		defer uGPU.Release()
		if err := queue.WriteBuffer(uGPU, 0, ubuf); err != nil {
			return err
		}

		cache.mu.Lock()
		pipe, bgl, samp, dummy := cache.pipeline, cache.bgl, cache.sampler, cache.dummyView
		cache.mu.Unlock()
		if pipe == nil || bgl == nil || samp == nil || dummy == nil {
			return fmt.Errorf("filter gpu pipeline missing")
		}
		srcV := a.srcView
		if srcV == nil {
			srcV = curView
		}
		dstV := a.dstView
		if dstV == nil {
			dstV = nxtView
		}
		auxV := a.auxView
		if auxV == nil {
			auxV = dummy
		}

		bg, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
			Label:  "filter_gpu_bg",
			Layout: bgl,
			Entries: []webgpu.BindGroupEntry{
				{Binding: 0, TextureView: srcV},
				{Binding: 1, Sampler: samp},
				{Binding: 2, Buffer: uGPU, Offset: 0, Size: filterGPUUniformSize},
				{Binding: 3, TextureView: auxV},
			},
		})
		if err != nil {
			return err
		}
		defer bg.Release()

		enc, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: "filter_gpu_enc"})
		if err != nil {
			return err
		}
		rp, err := enc.BeginRenderPass(&webgpu.RenderPassDescriptor{
			Label: "filter_gpu_pass",
			ColorAttachments: []webgpu.RenderPassColorAttachment{{
				View: dstV, LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
				ClearValue: types.Color{},
			}},
		})
		if err != nil {
			enc.DiscardEncoding()
			return err
		}
		rp.SetPipeline(pipe)
		rp.SetBindGroup(0, bg, nil)
		rp.Draw(3, 1, 0, 0)
		rp.End()
		cmd, err := enc.Finish()
		if err != nil {
			return err
		}
		if _, err := queue.Submit(cmd); err != nil {
			cmd.Release()
			return err
		}
		cmd.Release()
		return nil
	}

	swap := func() {
		curTex, nxtTex = nxtTex, curTex
		curView, nxtView = nxtView, curView
	}

	for i := range nodes {
		n := nodes[i]
		switch n.Kind {
		case render.ImageFilterGrayscale:
			if err := doPass(passArgs{mode: 1}); err != nil {
				return nil, err
			}
			swap()
		case render.ImageFilterInvert:
			if err := doPass(passArgs{mode: 2}); err != nil {
				return nil, err
			}
			swap()
		case render.ImageFilterBlur:
			r := uint32(math.Max(1, math.Min(8, math.Ceil(n.Radius))))
			if err := doPass(passArgs{mode: 0, radius: r, dirX: 1, dirY: 0}); err != nil {
				return nil, err
			}
			swap()
			if err := doPass(passArgs{mode: 0, radius: r, dirX: 0, dirY: 1}); err != nil {
				return nil, err
			}
			swap()
		case render.ImageFilterBlurXY:
			rx := uint32(math.Max(0, math.Min(8, math.Ceil(n.RadiusX))))
			ry := uint32(math.Max(0, math.Min(8, math.Ceil(n.RadiusY))))
			if rx > 0 {
				if err := doPass(passArgs{mode: 0, radius: rx, dirX: 1, dirY: 0}); err != nil {
					return nil, err
				}
				swap()
			}
			if ry > 0 {
				if err := doPass(passArgs{mode: 0, radius: ry, dirX: 0, dirY: 1}); err != nil {
					return nil, err
				}
				swap()
			}
		case render.ImageFilterColorMatrix:
			if err := doPass(passArgs{mode: 3, matrix: n.Matrix}); err != nil {
				return nil, err
			}
			swap()
		case render.ImageFilterDropShadow:
			// Snapshot content into hold.
			if err := doPass(passArgs{mode: 6, srcView: curView, dstView: viewH}); err != nil {
				return nil, err
			}
			// Extract shadow from hold into nxt.
			if err := doPass(passArgs{
				mode:    4,
				srcView: viewH,
				dstView: nxtView,
				offX:    float32(n.OffsetX),
				offY:    float32(n.OffsetY),
				colR:    float32(n.ShadowColor.R),
				colG:    float32(n.ShadowColor.G),
				colB:    float32(n.ShadowColor.B),
				colA:    float32(n.ShadowColor.A),
			}); err != nil {
				return nil, err
			}
			swap()
			// Blur shadow.
			if n.ShadowBlur > 0 {
				r := uint32(math.Max(1, math.Min(8, math.Ceil(n.ShadowBlur))))
				if err := doPass(passArgs{mode: 0, radius: r, dirX: 1, dirY: 0}); err != nil {
					return nil, err
				}
				swap()
				if err := doPass(passArgs{mode: 0, radius: r, dirX: 0, dirY: 1}); err != nil {
					return nil, err
				}
				swap()
			}
			// Composite hold (content) over shadow (cur).
			if err := doPass(passArgs{mode: 5, srcView: curView, auxView: viewH, dstView: nxtView}); err != nil {
				return nil, err
			}
			swap()
		default:
			return nil, fmt.Errorf("filter gpu unsupported node %v", n.Kind)
		}
	}

	// Readback curTex
	tightRow := uint32(w * 4) //nolint:gosec
	alignedRow := alignTextureBytesPerRow(tightRow)
	stagingSize := uint64(alignedRow) * uint64(h)
	staging, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "filter_gpu_readback", Size: stagingSize,
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, err
	}
	defer staging.Release()

	enc, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: "filter_gpu_read_enc"})
	if err != nil {
		return nil, err
	}
	enc.CopyTextureToBuffer(curTex, staging, []webgpu.BufferTextureCopy{{
		BufferLayout: webgpu.ImageDataLayout{BytesPerRow: alignedRow, RowsPerImage: uint32(h)}, //nolint:gosec
		TextureBase:  webgpu.ImageCopyTexture{Texture: curTex, MipLevel: 0, Aspect: types.TextureAspectAll},
		Size:         webgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec
	}})
	cmd, err := enc.Finish()
	if err != nil {
		return nil, err
	}
	if _, err := queue.Submit(cmd); err != nil {
		cmd.Release()
		return nil, err
	}
	cmd.Release()
	device.Poll(webgpu.PollWait)

	if err := staging.Map(context.Background(), webgpu.MapModeRead, 0, stagingSize); err != nil {
		return nil, err
	}
	mapped, err := staging.MappedRange(0, stagingSize)
	if err != nil {
		_ = staging.Unmap()
		return nil, err
	}
	srcMapped := mapped.Bytes()
	out := make([]byte, w*h*4)
	if alignedRow == tightRow {
		copy(out, srcMapped[:w*h*4])
	} else {
		for y := 0; y < h; y++ {
			copy(out[y*w*4:(y+1)*w*4], srcMapped[y*int(alignedRow):y*int(alignedRow)+w*4])
		}
	}
	mapped.Release()
	_ = staging.Unmap()
	return out, nil
}
