//go:build !nogpu

package gpu

import (
	"context"
	"fmt"
	"math"
	"sync"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

// F.03: true multi-RT GPU image filter graph with texture ping-pong.
// Resources (A/B/hold RTs, uniform, staging, publish slots) are pooled on
// filterGPUCache so continuous glow/effect frames do not alloc/free VRAM or
// staging buffers every ApplyBlur.

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
    // Separable Gaussian blur (sigma in offset.x; radius = half-kernel).
    // Matches CPU CachedGaussianKernel: half = ceil(3*sigma), weight = exp(-x^2/(2s^2)).
    let texel = p.direction / max(p.size, vec2<f32>(1.0, 1.0));
    let rad = i32(p.radius);
    let sigma = max(p.offset.x, 0.01);
    let two_sigma_sq = 2.0 * sigma * sigma;
    var acc = vec4<f32>(0.0);
    var wsum = 0.0;
    for (var i = -24; i <= 24; i = i + 1) {
        if i < -rad || i > rad {
            continue;
        }
        let fi = f32(i);
        let w = exp(-(fi * fi) / two_sigma_sq);
        let uv = in.uv + texel * fi;
        acc = acc + textureSampleLevel(src_tex, samp, uv, 0.0) * w;
        wsum = wsum + w;
    }
    return acc / max(wsum, 1e-6);
}
`

type filterPublishSlot struct {
	tex  *webgpu.Texture
	view *webgpu.TextureView
	w, h int
}

type filterGPUCache struct {
	runMu     sync.Mutex // serializes full graph runs (pooled RTs)
	mu        sync.Mutex
	device    *webgpu.Device
	pipeline  *webgpu.RenderPipeline
	bgl       *webgpu.BindGroupLayout
	sampler   *webgpu.Sampler
	dummyTex  *webgpu.Texture
	dummyView *webgpu.TextureView

	// Pooled ping-pong RTs for continuous effect frames.
	poolW, poolH        int
	texA, texB, texH    *webgpu.Texture
	viewA, viewB, viewH *webgpu.TextureView

	// Reused uniform + CPU staging/readback buffers.
	uniform    *webgpu.Buffer
	uniformCPU []byte
	staging    *webgpu.Buffer
	stagingCap uint64
	outScratch []byte
	uploadPad  []byte

	// Published result textures (owned until caller Release).
	publishFree []filterPublishSlot

	// Reused per-pass uniform buffers (written via WriteBuffer before encode).
	passUniforms []*webgpu.Buffer

	// Per-run scratch (no per-frame make).
	passBGScratch   []*webgpu.BindGroup
	passURefScratch []*webgpu.Buffer

	// Stable bind-group cache for continuous effect frames (glow).
	// Keyed by view/uniform pointers; cleared when pool RTs are rebuilt.
	bgCache map[filterBGKey]*webgpu.BindGroup
}

type filterBGKey struct {
	src, dst, aux, ubuf uintptr
}

func (c *filterGPUCache) release() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.releaseUnlocked()
}

func (c *filterGPUCache) releasePoolUnlocked() {
	c.clearBGCacheUnlocked()
	if c.viewA != nil {
		c.viewA.Release()
		c.viewA = nil
	}
	if c.texA != nil {
		c.texA.Release()
		c.texA = nil
	}
	if c.viewB != nil {
		c.viewB.Release()
		c.viewB = nil
	}
	if c.texB != nil {
		c.texB.Release()
		c.texB = nil
	}
	if c.viewH != nil {
		c.viewH.Release()
		c.viewH = nil
	}
	if c.texH != nil {
		c.texH.Release()
		c.texH = nil
	}
	c.poolW, c.poolH = 0, 0
}

func (c *filterGPUCache) releaseUnlocked() {
	c.releasePoolUnlocked()
	if c.uniform != nil {
		c.uniform.Release()
		c.uniform = nil
	}
	if c.staging != nil {
		c.staging.Release()
		c.staging = nil
		c.stagingCap = 0
	}
	for i := range c.publishFree {
		if c.publishFree[i].view != nil {
			c.publishFree[i].view.Release()
		}
		if c.publishFree[i].tex != nil {
			c.publishFree[i].tex.Release()
		}
	}
	c.publishFree = nil
	for _, b := range c.passUniforms {
		if b != nil {
			b.Release()
		}
	}
	c.passUniforms = nil
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
	c.outScratch = nil
	c.uploadPad = nil
	c.uniformCPU = nil
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

	ubuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "filter_params_pooled", Size: filterGPUUniformSize,
		Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
	})
	if err != nil {
		dview.Release()
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
	c.uniform = ubuf
	c.uniformCPU = make([]byte, filterGPUUniformSize)
	return nil
}

func (c *filterGPUCache) ensurePool(device *webgpu.Device, w, h int) error {
	if c.texA != nil && c.poolW == w && c.poolH == h && c.device == device {
		return nil
	}
	c.releasePoolUnlocked()
	usageRT := types.TextureUsageTextureBinding | types.TextureUsageRenderAttachment | types.TextureUsageCopySrc | types.TextureUsageCopyDst
	mk := func(label string) (*webgpu.Texture, *webgpu.TextureView, error) {
		tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
			Label:         label,
			Size:          webgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatRGBA8Unorm,
			Usage:  usageRT,
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
		return tex, view, nil
	}
	var err error
	c.texA, c.viewA, err = mk("filter_rt_a")
	if err != nil {
		return err
	}
	c.texB, c.viewB, err = mk("filter_rt_b")
	if err != nil {
		c.releasePoolUnlocked()
		return err
	}
	c.texH, c.viewH, err = mk("filter_rt_hold")
	if err != nil {
		c.releasePoolUnlocked()
		return err
	}
	c.poolW, c.poolH = w, h
	return nil
}

func (c *filterGPUCache) ensureStaging(device *webgpu.Device, size uint64) error {
	if c.staging != nil && c.stagingCap >= size && c.device == device {
		return nil
	}
	if c.staging != nil {
		c.staging.Release()
		c.staging = nil
		c.stagingCap = 0
	}
	// Cap slightly above request to absorb minor size jitter.
	cap := size
	if cap < 64*1024 {
		cap = 64 * 1024
	}
	stg, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "filter_gpu_readback_pooled", Size: cap,
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		return err
	}
	c.staging = stg
	c.stagingCap = cap
	return nil
}

func (c *filterGPUCache) clearBGCacheUnlocked() {
	for k, bg := range c.bgCache {
		if bg != nil {
			bg.Release()
		}
		delete(c.bgCache, k)
	}
	if c.bgCache == nil {
		c.bgCache = make(map[filterBGKey]*webgpu.BindGroup)
	}
}

func (c *filterGPUCache) bindGroup(device *webgpu.Device, bgl *webgpu.BindGroupLayout, samp *webgpu.Sampler,
	src, dst, aux *webgpu.TextureView, ubuf *webgpu.Buffer,
) (*webgpu.BindGroup, error) {
	if device == nil || bgl == nil || samp == nil || src == nil || dst == nil || aux == nil || ubuf == nil {
		return nil, fmt.Errorf("filter bg: nil arg")
	}
	key := filterBGKey{
		src:  uintptr(unsafe.Pointer(src)),
		dst:  uintptr(unsafe.Pointer(dst)), // dst not in BG, but partition cache by dest intent
		aux:  uintptr(unsafe.Pointer(aux)),
		ubuf: uintptr(unsafe.Pointer(ubuf)),
	}
	// dst is render attachment, not bind-group entry — key should be src/aux/ubuf only.
	key.dst = 0
	c.mu.Lock()
	if c.bgCache == nil {
		c.bgCache = make(map[filterBGKey]*webgpu.BindGroup)
	}
	if bg := c.bgCache[key]; bg != nil {
		c.mu.Unlock()
		return bg, nil
	}
	c.mu.Unlock()

	bg, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label:  "filter_gpu_bg_cached",
		Layout: bgl,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, TextureView: src},
			{Binding: 1, Sampler: samp},
			{Binding: 2, Buffer: ubuf, Offset: 0, Size: filterGPUUniformSize},
			{Binding: 3, TextureView: aux},
		},
	})
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	// Another runner may have filled the slot; keep one, release duplicate.
	if prev := c.bgCache[key]; prev != nil {
		c.mu.Unlock()
		bg.Release()
		return prev, nil
	}
	c.bgCache[key] = bg
	c.mu.Unlock()
	return bg, nil
}

func (c *filterGPUCache) acquirePassUniform(device *webgpu.Device, idx int) (*webgpu.Buffer, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if idx < len(c.passUniforms) && c.passUniforms[idx] != nil && c.device == device {
		return c.passUniforms[idx], nil
	}
	for len(c.passUniforms) <= idx {
		c.passUniforms = append(c.passUniforms, nil)
	}
	if c.passUniforms[idx] != nil {
		c.passUniforms[idx].Release()
		c.passUniforms[idx] = nil
	}
	b, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "filter_params_pass_pooled", Size: filterGPUUniformSize,
		Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, err
	}
	c.passUniforms[idx] = b
	return b, nil
}

// promotePoolResultToPublish zero-copy publishes a finished A/B pool RT by
// swapping it out of the pool. Replacement prefers the publish free-list so
// steady-state glow frames do not CopyTextureToTexture or allocate VRAM.
// Safe to call after encode and before Submit: the command buffer retains the
// promoted texture; the pool receives a recycled/new RT for the next graph.
func (c *filterGPUCache) promotePoolResultToPublish(device *webgpu.Device, tex *webgpu.Texture, view *webgpu.TextureView, w, h int) (filterPublishSlot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if device == nil || tex == nil || view == nil || c.poolW != w || c.poolH != h {
		return filterPublishSlot{}, fmt.Errorf("filter gpu promote: pool mismatch")
	}
	if c.texA != tex && c.texB != tex {
		return filterPublishSlot{}, fmt.Errorf("filter gpu promote: tex not in pool")
	}

	// Recycle a free publish slot as the new pool RT (steady-state: no alloc).
	var replTex *webgpu.Texture
	var replView *webgpu.TextureView
	for i := range c.publishFree {
		s := c.publishFree[i]
		if s.w == w && s.h == h && s.tex != nil && s.view != nil {
			c.publishFree = append(c.publishFree[:i], c.publishFree[i+1:]...)
			replTex, replView = s.tex, s.view
			break
		}
	}
	if replTex == nil {
		usageRT := types.TextureUsageTextureBinding | types.TextureUsageRenderAttachment | types.TextureUsageCopySrc | types.TextureUsageCopyDst
		texNew, err := device.CreateTexture(&webgpu.TextureDescriptor{
			Label:         "filter_rt_repl",
			Size:          webgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatRGBA8Unorm,
			Usage:  usageRT,
		})
		if err != nil {
			return filterPublishSlot{}, err
		}
		viewNew, err := device.CreateTextureView(texNew, &webgpu.TextureViewDescriptor{
			Label: "filter_rt_repl_view", Format: types.TextureFormatRGBA8Unorm,
			Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
		})
		if err != nil {
			texNew.Release()
			return filterPublishSlot{}, err
		}
		replTex, replView = texNew, viewNew
	}

	// Keep bgCache: promoted views leave the pool; recycled slots retain the
	// same TextureView pointers so subsequent frames still hit the cache.
	// Full clear only happens on pool rebuild (ensurePool / releasePool).

	if c.texA == tex {
		c.texA, c.viewA = replTex, replView
	} else {
		c.texB, c.viewB = replTex, replView
	}
	return filterPublishSlot{tex: tex, view: view, w: w, h: h}, nil
}

// detachPoolResult is kept for callers that need explicit ownership transfer.
// Prefer promotePoolResultToPublish for continuous effect publish (glow).
func (c *filterGPUCache) detachPoolResult(device *webgpu.Device, tex *webgpu.Texture, view *webgpu.TextureView, w, h int) (filterPublishSlot, error) {
	return c.promotePoolResultToPublish(device, tex, view, w, h)
}

func (c *filterGPUCache) acquirePublish(device *webgpu.Device, w, h int) (filterPublishSlot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.publishFree {
		s := c.publishFree[i]
		if s.w == w && s.h == h && s.tex != nil && s.view != nil {
			c.publishFree = append(c.publishFree[:i], c.publishFree[i+1:]...)
			return s, nil
		}
	}
	usage := types.TextureUsageTextureBinding | types.TextureUsageCopyDst | types.TextureUsageCopySrc | types.TextureUsageRenderAttachment
	tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "filter_publish",
		Size:          webgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA8Unorm,
		Usage:  usage,
	})
	if err != nil {
		return filterPublishSlot{}, err
	}
	view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
		Label: "filter_publish_view", Format: types.TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		tex.Release()
		return filterPublishSlot{}, err
	}
	return filterPublishSlot{tex: tex, view: view, w: w, h: h}, nil
}

func (c *filterGPUCache) releasePublish(slot filterPublishSlot) {
	if slot.tex == nil || slot.view == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// Keep a small free list (cap 4) to absorb glow RT cadence without VRAM creep.
	if len(c.publishFree) < 4 {
		c.publishFree = append(c.publishFree, slot)
		return
	}
	slot.view.Release()
	slot.tex.Release()
}

// runGPUFilterGraph executes multi-RT ping-pong filter nodes on GPU and readbacks.
func runGPUFilterGraph(device *webgpu.Device, queue *webgpu.Queue, cache *filterGPUCache, src []byte, w, h int, nodes []render.ImageFilterNode) ([]byte, error) {
	out, _, _, err := runGPUFilterGraphEx(device, queue, cache, src, nil, w, h, nodes, true)
	return out, err
}

// runGPUFilterGraphGPUOnly runs the filter graph and publishes a GPU texture view
// for zero-copy compositing (DrawGPUTexture). No CPU Map/readback.
// Caller must invoke release when the view is no longer sampled.
func runGPUFilterGraphGPUOnly(device *webgpu.Device, queue *webgpu.Queue, cache *filterGPUCache, src []byte, w, h int, nodes []render.ImageFilterNode) (gpucontext.TextureView, func(), error) {
	_, view, release, err := runGPUFilterGraphEx(device, queue, cache, src, nil, w, h, nodes, false)
	return view, release, err
}

// runGPUFilterGraphFromView seeds the graph from an existing GPU texture view
// (no CPU upload). Source may be BGRA offscreen; first copy pass samples into
// the RGBA pool (WebGPU returns BGRA samples in RGBA order).
func runGPUFilterGraphFromView(device *webgpu.Device, queue *webgpu.Queue, cache *filterGPUCache, srcView gpucontext.TextureView, w, h int, nodes []render.ImageFilterNode) (gpucontext.TextureView, func(), error) {
	if srcView.IsNil() {
		return gpucontext.TextureView{}, nil, fmt.Errorf("filter gpu: nil src view")
	}
	wgpuView := (*webgpu.TextureView)(srcView.Pointer())
	_, view, release, err := runGPUFilterGraphEx(device, queue, cache, nil, wgpuView, w, h, nodes, false)
	return view, release, err
}

func runGPUFilterGraphEx(
	device *webgpu.Device, queue *webgpu.Queue, cache *filterGPUCache,
	src []byte, srcView *webgpu.TextureView, w, h int, nodes []render.ImageFilterNode, wantPixels bool,
) (out []byte, pubView gpucontext.TextureView, pubRelease func(), err error) {
	if device == nil || queue == nil || cache == nil || w <= 0 || h <= 0 {
		return nil, gpucontext.TextureView{}, nil, fmt.Errorf("filter gpu: invalid args")
	}
	if w*h > filterGPUMaxPixels {
		return nil, gpucontext.TextureView{}, nil, fmt.Errorf("filter gpu: size")
	}
	if srcView == nil && len(src) < w*h*4 {
		return nil, gpucontext.TextureView{}, nil, fmt.Errorf("filter gpu: size")
	}
	cache.runMu.Lock()
	defer cache.runMu.Unlock()
	if err := cache.ensure(device); err != nil {
		return nil, gpucontext.TextureView{}, nil, err
	}

	cache.mu.Lock()
	if err := cache.ensurePool(device, w, h); err != nil {
		cache.mu.Unlock()
		return nil, gpucontext.TextureView{}, nil, err
	}
	texA, viewA := cache.texA, cache.viewA
	texB, viewB := cache.texB, cache.viewB
	texH, viewH := cache.texH, cache.viewH
	uGPU := cache.uniform
	ubuf := cache.uniformCPU
	pipe, bgl, samp, dummy := cache.pipeline, cache.bgl, cache.sampler, cache.dummyView
	cache.mu.Unlock()

	if texA == nil || viewA == nil || texB == nil || viewB == nil || texH == nil || viewH == nil ||
		uGPU == nil || len(ubuf) < filterGPUUniformSize || pipe == nil || bgl == nil || samp == nil || dummy == nil {
		return nil, gpucontext.TextureView{}, nil, fmt.Errorf("filter gpu pool missing")
	}

	curTex, curView := texA, viewA
	nxtTex, nxtView := texB, viewB

	// Seed content into A: either CPU upload or copy-sample from GPU view.
	if srcView != nil {
		// mode 6 copy: sample srcView → A (BGRA sources sample as RGBA colors).
		// Implemented after doPass is defined — placeholder replaced below.
	} else {
		bpr := alignTextureBytesPerRow(uint32(w * 4)) //nolint:gosec
		upload := src
		if int(bpr) != w*4 {
			need := int(bpr) * h
			cache.mu.Lock()
			if cap(cache.uploadPad) < need {
				cache.uploadPad = make([]byte, need)
			}
			cache.uploadPad = cache.uploadPad[:need]
			for y := 0; y < h; y++ {
				copy(cache.uploadPad[y*int(bpr):y*int(bpr)+w*4], src[y*w*4:(y+1)*w*4])
			}
			upload = cache.uploadPad
			cache.mu.Unlock()
		}
		if err := queue.WriteTexture(
			&webgpu.ImageCopyTexture{Texture: texA, MipLevel: 0},
			upload,
			&webgpu.ImageDataLayout{BytesPerRow: bpr, RowsPerImage: uint32(h)},           //nolint:gosec
			&webgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec
		); err != nil {
			return nil, gpucontext.TextureView{}, nil, err
		}
	}

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

	// One command encoder for all passes + publish copy (single queue submit).
	// Uniforms cannot be rewritten mid-encoder safely for concurrent binds, so
	// each pass gets a pooled uniform buffer written before encode.
	cache.mu.Lock()
	if cap(cache.passURefScratch) < 8 {
		cache.passURefScratch = make([]*webgpu.Buffer, 0, 8)
	}
	passUBufs := cache.passURefScratch[:0]
	cache.mu.Unlock()
	enc, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: "filter_gpu_batch_enc"})
	if err != nil {
		return nil, gpucontext.TextureView{}, nil, err
	}
	cleanupPasses := func() {
		// uniforms pooled; bind groups cached on filterGPUCache
		cache.mu.Lock()
		cache.passURefScratch = passUBufs[:0]
		cache.mu.Unlock()
	}

	doPass := func(a passArgs) error {
		clear(ubuf)
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
		for i := 0; i < 20; i++ {
			v := a.matrix[i]
			if i == 4 || i == 9 || i == 14 || i == 19 {
				v = v / 255.0
			}
			putF32(48+i*4, v)
		}
		// Per-pass uniform from pool (alive until Submit via passUBufs hold).
		uPass, err := cache.acquirePassUniform(device, len(passUBufs))
		if err != nil {
			return err
		}
		passUBufs = append(passUBufs, uPass)
		if err := queue.WriteBuffer(uPass, 0, ubuf); err != nil {
			return err
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

		bg, err := cache.bindGroup(device, bgl, samp, srcV, dstV, auxV, uPass)
		if err != nil {
			return err
		}
		// Cached bind groups are owned by filterGPUCache — do not Release in cleanup.

		rp, err := enc.BeginRenderPass(&webgpu.RenderPassDescriptor{
			Label: "filter_gpu_pass",
			ColorAttachments: []webgpu.RenderPassColorAttachment{{
				View: dstV, LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
				ClearValue: types.Color{},
			}},
		})
		if err != nil {
			return err
		}
		rp.SetPipeline(pipe)
		rp.SetBindGroup(0, bg, nil)
		rp.Draw(3, 1, 0, 0)
		rp.End()
		return nil
	}

	// External GPU source without mandatory copy pass.
	// First write goes into A; then A/B ping-pong as usual.
	fromExternal := srcView != nil
	if fromExternal {
		curTex, curView = nil, srcView
		nxtTex, nxtView = texA, viewA
	}

	swap := func() {
		if fromExternal && curTex == nil {
			// First result is in nxt (A). Next destination must be B.
			curTex, curView = nxtTex, nxtView
			nxtTex, nxtView = texB, viewB
			fromExternal = false
			return
		}
		curTex, nxtTex = nxtTex, curTex
		curView, nxtView = nxtView, curView
	}

	for i := range nodes {
		n := nodes[i]
		switch n.Kind {
		case render.ImageFilterGrayscale:
			if err := doPass(passArgs{mode: 1}); err != nil {
				enc.DiscardEncoding()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
		case render.ImageFilterInvert:
			if err := doPass(passArgs{mode: 2}); err != nil {
				enc.DiscardEncoding()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
		case render.ImageFilterBlur:
			sigma := math.Max(0.01, n.Radius)
			r := uint32(math.Max(1, math.Min(24, math.Ceil(sigma*3))))
			if err := doPass(passArgs{mode: 0, radius: r, dirX: 1, dirY: 0, offX: float32(sigma)}); err != nil {
				enc.DiscardEncoding()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
			if err := doPass(passArgs{mode: 0, radius: r, dirX: 0, dirY: 1, offX: float32(sigma)}); err != nil {
				enc.DiscardEncoding()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
		case render.ImageFilterBlurXY:
			sx := math.Max(0, n.RadiusX)
			sy := math.Max(0, n.RadiusY)
			if sx > 0 {
				rx := uint32(math.Max(1, math.Min(24, math.Ceil(sx*3))))
				if err := doPass(passArgs{mode: 0, radius: rx, dirX: 1, dirY: 0, offX: float32(sx)}); err != nil {
					return nil, gpucontext.TextureView{}, nil, err
				}
				swap()
			}
			if sy > 0 {
				ry := uint32(math.Max(1, math.Min(24, math.Ceil(sy*3))))
				if err := doPass(passArgs{mode: 0, radius: ry, dirX: 0, dirY: 1, offX: float32(sy)}); err != nil {
					return nil, gpucontext.TextureView{}, nil, err
				}
				swap()
			}
		case render.ImageFilterColorMatrix:
			if err := doPass(passArgs{mode: 3, matrix: n.Matrix}); err != nil {
				enc.DiscardEncoding()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
		case render.ImageFilterDropShadow:
			if err := doPass(passArgs{mode: 6, srcView: curView, dstView: viewH}); err != nil {
				return nil, gpucontext.TextureView{}, nil, err
			}
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
				enc.DiscardEncoding()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
			if n.ShadowBlur > 0 {
				sigma := math.Max(0.01, n.ShadowBlur)
				r := uint32(math.Max(1, math.Min(24, math.Ceil(sigma*3))))
				if err := doPass(passArgs{mode: 0, radius: r, dirX: 1, dirY: 0, offX: float32(sigma)}); err != nil {
					return nil, gpucontext.TextureView{}, nil, err
				}
				swap()
				if err := doPass(passArgs{mode: 0, radius: r, dirX: 0, dirY: 1, offX: float32(sigma)}); err != nil {
					return nil, gpucontext.TextureView{}, nil, err
				}
				swap()
			}
			if err := doPass(passArgs{mode: 5, srcView: curView, auxView: viewH, dstView: nxtView}); err != nil {
				enc.DiscardEncoding()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
		default:
			enc.DiscardEncoding()
			cleanupPasses()
			return nil, gpucontext.TextureView{}, nil, fmt.Errorf("filter gpu unsupported node %v", n.Kind)
		}
	}

	// Finish graph: zero-copy promote of A/B result into a publish slot when
	// possible (steady-state glow). Fall back to publish-slot copy only when
	// the result is not a pool RT (should be rare after real filter passes).
	var slot filterPublishSlot
	if !wantPixels {
		if curTex != nil {
			slot, err = cache.promotePoolResultToPublish(device, curTex, curView, w, h)
		} else {
			err = fmt.Errorf("filter gpu publish: nil result tex")
		}
		if err != nil {
			// Fallback: acquire publish RT + GPU copy (keeps correctness).
			slot, err = cache.acquirePublish(device, w, h)
			if err != nil {
				enc.DiscardEncoding()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			if curTex != nil {
				enc.CopyTextureToTexture(curTex, slot.tex, []webgpu.TextureCopy{{
					Source: webgpu.ImageCopyTexture{
						Texture: curTex, MipLevel: 0,
						Origin: webgpu.Origin3D{}, Aspect: types.TextureAspectAll,
					},
					Destination: webgpu.ImageCopyTexture{
						Texture: slot.tex, MipLevel: 0,
						Origin: webgpu.Origin3D{}, Aspect: types.TextureAspectAll,
					},
					Size: webgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec
				}})
			} else if curView != nil {
				// External-only: sample-copy into publish via mode-6 pass.
				if err := doPass(passArgs{mode: 6, srcView: curView, dstView: slot.view}); err != nil {
					cache.releasePublish(slot)
					enc.DiscardEncoding()
					cleanupPasses()
					return nil, gpucontext.TextureView{}, nil, err
				}
			} else {
				cache.releasePublish(slot)
				enc.DiscardEncoding()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, fmt.Errorf("filter gpu publish: no source")
			}
		}
	}
	cmd, err := enc.Finish()
	if err != nil {
		if slot.tex != nil {
			cache.releasePublish(slot)
		}
		cleanupPasses()
		return nil, gpucontext.TextureView{}, nil, err
	}
	if _, err := queue.Submit(cmd); err != nil {
		cmd.Release()
		if slot.tex != nil {
			cache.releasePublish(slot)
		}
		cleanupPasses()
		return nil, gpucontext.TextureView{}, nil, err
	}
	cmd.Release()
	cleanupPasses()

	if !wantPixels {
		view := gpucontext.NewTextureView(unsafe.Pointer(slot.view)) //nolint:gosec
		release := func() { cache.releasePublish(slot) }
		return nil, view, release, nil
	}

	// Pixel path: readback from curTex with pooled staging (no publish slot).
	tightRow := uint32(w * 4) //nolint:gosec
	alignedRow := alignTextureBytesPerRow(tightRow)
	stagingSize := uint64(alignedRow) * uint64(h)
	cache.mu.Lock()
	if err := cache.ensureStaging(device, stagingSize); err != nil {
		cache.mu.Unlock()
		return nil, gpucontext.TextureView{}, nil, err
	}
	staging := cache.staging
	cache.mu.Unlock()

	enc2, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: "filter_gpu_read_enc"})
	if err != nil {
		return nil, gpucontext.TextureView{}, nil, err
	}
	enc2.CopyTextureToBuffer(curTex, staging, []webgpu.BufferTextureCopy{{
		BufferLayout: webgpu.ImageDataLayout{BytesPerRow: alignedRow, RowsPerImage: uint32(h)}, //nolint:gosec
		TextureBase:  webgpu.ImageCopyTexture{Texture: curTex, MipLevel: 0, Aspect: types.TextureAspectAll},
		Size:         webgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec
	}})
	cmd2, err := enc2.Finish()
	if err != nil {
		return nil, gpucontext.TextureView{}, nil, err
	}
	if _, err := queue.Submit(cmd2); err != nil {
		cmd2.Release()
		return nil, gpucontext.TextureView{}, nil, err
	}
	cmd2.Release()
	device.Poll(webgpu.PollWait)

	if err := staging.Map(context.Background(), webgpu.MapModeRead, 0, stagingSize); err != nil {
		return nil, gpucontext.TextureView{}, nil, err
	}
	mapped, err := staging.MappedRange(0, stagingSize)
	if err != nil {
		_ = staging.Unmap()
		return nil, gpucontext.TextureView{}, nil, err
	}
	srcMapped := mapped.Bytes()
	needOut := w * h * 4
	cache.mu.Lock()
	if cap(cache.outScratch) < needOut {
		cache.outScratch = make([]byte, needOut)
	}
	cache.outScratch = cache.outScratch[:needOut]
	out = cache.outScratch
	if alignedRow == tightRow {
		copy(out, srcMapped[:needOut])
	} else {
		for y := 0; y < h; y++ {
			copy(out[y*w*4:(y+1)*w*4], srcMapped[y*int(alignedRow):y*int(alignedRow)+w*4])
		}
	}
	cache.mu.Unlock()
	mapped.Release()
	_ = staging.Unmap()
	return out, gpucontext.TextureView{}, nil, nil
}
