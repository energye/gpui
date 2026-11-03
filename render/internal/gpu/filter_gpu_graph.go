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
	"fmt"
	"math"
	"sync"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/gpu/res"
)

// F.03: true multi-RT GPU image filter graph with texture ping-pong.
// Resources (A/B/hold RTs, uniform, staging, publish slots) are pooled on
// filterGPUCache so continuous glow/effect frames do not alloc/free VRAM or
// staging buffers every ApplyBlur.

const filterGPUMaxPixels = 4 * 1024 * 1024
const filterGPUUniformSize = 192

// per-pass Params live in one slab buffer. Slot stride must be a multiple
// of WebGPU minUniformBufferOffsetAlignment (default 256); payload is 128B.
const filterPassUniformSlotStride = 256
const filterPassUniformSlabMinSlots = 16

// Static encoder descriptors.
var (
	filterGPUBatchEncoderDesc = &hal.CommandEncoderDescriptor{Label: "filter_gpu_batch_enc"}
	filterGPUReadEncoderDesc  = &hal.CommandEncoderDescriptor{Label: "filter_gpu_read_enc"}
	filterSeedMeshEncoderDesc = &hal.CommandEncoderDescriptor{Label: "filter_seed_mesh_enc"}
)

const filterGPUGraphWGSL = `
struct Params {
    size: vec2<f32>,
    direction: vec2<f32>,
    mode: u32,           // 0=blur 1=gray 2=invert 3=matrix 4=shadowExtract 5=shadowComposite 6=copy
    radius: u32,
    offset: vec2<f32>,   // shadow offset in texels
    color: vec4<f32>,    // shadow color (straight RGBA 0-1)
    matrix: array<vec4<f32>, 5>, // 20 floats for 4x5 color matrix (CPU 0-255 space biases /255)
    // P3.4 extension slots (128..192):
    rect0: vec4<f32>, // lighting: light pos (x, y, z, kd)
    rect1: vec4<f32>, // lighting: light color (r, g, b, ks)
    rect2: vec4<f32>, // lighting: (shininess, surfaceScale, -, -) / tile srcRect x0,y0
    rect3: vec4<f32>, // tile srcRect x1, y1 (+ spares)
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
    if p.mode == 9u {
        // Offset (P3.4): whole-pixel translation via uv shift; uncovered
        // area stays transparent. dst(x) samples src(x - offset) — content
        // moves right/down, matching the CPU offsetApply.
        let uv_o = in.uv - vec2<f32>(p.offset) / max(p.size, vec2<f32>(1.0, 1.0));
        return sample_clamped(src_tex, uv_o);
    }
    if p.mode == 10u {
        // AlphaThreshold (P3.4): zero alpha below the threshold. The
        // threshold rides p.color.a (0-1).
        var c0 = textureSampleLevel(src_tex, samp, in.uv, 0.0);
        if (c0.a < p.color.a) {
            return vec4<f32>(0.0);
        }
        return c0;
    }
    if p.mode == 11u {
        // Point diffuse lighting (P3.4, Skia LightingImageFilter): the source
        // ALPHA channel is the height map (Skia semantics); the surface
        // normal comes from central differences scaled by surfaceScale.
        let px = 1.0 / max(p.size, vec2<f32>(1.0, 1.0));
        let hL = textureSampleLevel(src_tex, samp, in.uv + vec2<f32>(-px.x, 0.0), 0.0).a;
        let hR = textureSampleLevel(src_tex, samp, in.uv + vec2<f32>(px.x, 0.0), 0.0).a;
        let hT = textureSampleLevel(src_tex, samp, in.uv + vec2<f32>(0.0, -px.y), 0.0).a;
        let hB = textureSampleLevel(src_tex, samp, in.uv + vec2<f32>(0.0, px.y), 0.0).a;
        let ss = p.rect2.y;
        let n = normalize(vec3<f32>(-(hR - hL) * ss, -(hB - hT) * ss, 1.0));
        let lx = p.rect0.x - in.uv.x * p.size.x;
        let ly = p.rect0.y - in.uv.y * p.size.y;
        let lz0 = p.rect0.z;
        var lz = lz0;
        if (lz0 >= 0.0 && lz0 < 0.001) {
            // Zero-division guard only; negative z stays (light below the
            // plane → kd≤0), matching the CPU reference (P1-9).
            lz = 0.001;
        }
        let l = normalize(vec3<f32>(lx, ly, lz));
        let kd = max(dot(n, l), 0.0);
        let src0 = textureSampleLevel(src_tex, samp, in.uv, 0.0);
        let lit = p.rect1.rgb * p.rect0.w * kd;
        // Lit color replaces RGB (straight-domain math on premul storage is
        // alpha-safe here because the light result is scaled by src alpha at
        // composite time); keep the source alpha.
        let a = src0.a;
        return vec4<f32>(lit * a, a);
    }
    if p.mode == 12u {
        // Tile (P3.4, Skia ImageFilters::Tile): repeat p.rect2/rect3's
        // src-rect region of the source across the whole output.
        let x0 = p.rect2.z;
        let y0 = p.rect2.w;
        let x1 = p.rect3.x;
        let y1 = p.rect3.y;
        let w0 = max(x1 - x0, 0.0001);
        let h0 = max(y1 - y0, 0.0001);
        let u = in.uv.x * p.size.x;
        let v = in.uv.y * p.size.y;
        // Pixel-space origin plus the wrapped offset, normalized once —
        // x0 must not be added to the normalized uv (ClampToEdge smear,
        // P1-8; CPU reference lighting.go TileFilter.Apply does x0+mod).
        let tuv = vec2<f32>((x0 + (u % w0)) / p.size.x,
                            (y0 + (v % h0)) / p.size.y);
        return textureSampleLevel(src_tex, samp, tuv, 0.0);
    }
    if p.mode == 7u || p.mode == 8u {
        // Morphology (P3.4): erode=min / dilate=max over a (2r+1)^2 box.
        var best = vec4<f32>(-1.0);
        let r = i32(p.radius);
        let px = 1.0 / max(p.size, vec2<f32>(1.0, 1.0));
        for (var dy = -r; dy <= r; dy++) {
            for (var dx = -r; dx <= r; dx++) {
                let uv_m = in.uv + vec2<f32>(f32(dx), f32(dy)) * px;
                let c1 = sample_clamped(src_tex, uv_m);
                if (best.r < -0.5) {
                    best = c1;
                } else if (p.mode == 7u) {
                    best = min(best, c1);
                } else {
                    best = max(best, c1);
                }
            }
        }
        return best;
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
        // Invert in straight-alpha space, then store premultiplied:
        //   straight' = 1 - straight = 1 - c0.rgb / c0.a
        //   premul'   = c0.a * straight' = c0.a - c0.rgb
        // Skia/Flutter color filters run on unpremultiplied colors; doing
        // 1-c0.rgb on premultiplied storage breaks alpha (channels > alpha).
        return vec4<f32>(c0.a - c0.r, c0.a - c0.g, c0.a - c0.b, c0.a);
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
    // Matches CPU CachedGaussianKernel exactly: half = ceil(3*sigma),
    // weight = exp(-x^2/(2s^2)). The loop bound is the uniform radius —
    // dynamic like Impeller's blur kernel, no static clamp (a fixed ±24
    // window truncated large-sigma kernels and forked from the CPU
    // reference, P1-7).
    let texel = p.direction / max(p.size, vec2<f32>(1.0, 1.0));
    let rad = i32(p.radius);
    let sigma = max(p.offset.x, 0.01);
    let two_sigma_sq = 2.0 * sigma * sigma;
    var acc = vec4<f32>(0.0);
    var wsum = 0.0;
    for (var i = -rad; i <= rad; i = i + 1) {
        let fi = f32(i);
        let w = exp(-(fi * fi) / two_sigma_sq);
        let uv = in.uv + texel * fi;
        acc = acc + textureSampleLevel(src_tex, samp, uv, 0.0) * w;
        wsum = wsum + w;
    }
    return acc / max(wsum, 1e-6);
}
`

// filterPublishSlot is one graph result handed to the compositor: the RT plus
// the strong cache ref keeping it out of the pool until the caller releases it.
type filterPublishSlot struct {
	tex  hal.Texture
	view hal.TextureView
	ref  res.Ref
}

type filterGPUCache struct {
	runMu     sync.Mutex // serializes full graph runs (pooled RTs)
	mu        sync.Mutex
	device    hal.Device
	pipeline  hal.RenderPipeline
	bgl       hal.BindGroupLayout
	sampler   hal.Sampler
	dummyTex  hal.Texture
	dummyView hal.TextureView

	// P2.4d: the graph's RGBA8 render targets (ping-pong A/B, hold, and
	// published results) are one device-level res.Cache keyed by size, so
	// windows sharing a device share same-size RTs, the domain has a byte
	// budget, and an entry still referenced by an in-flight submit is never
	// lent out again. resOwner/gate come from GPUShared; without them (bare
	// cache in tests) RTs are created and destroyed directly.
	rtCache       *res.Cache
	rtCacheDevice hal.Device
	resOwner      res.Owner
	gate          *directSubmit

	// Reused uniform + CPU staging/readback buffers.
	uniform    hal.Buffer
	uniformCPU []byte
	staging    hal.Buffer
	stagingCap uint64
	outScratch []byte
	uploadPad  []byte

	// single slab for all pass uniforms (stride filterPassUniformSlotStride).
	// Each pass bind group uses a fixed Offset into the slab; one WriteBuffer
	// covers all packed slots after encode, before Submit.
	passUniformSlab    hal.Buffer
	passUniformSlabCap uint64
	passUniformScratch []byte
	// lastPassUniform* — test/pprof diagnostics (updated each graph run).
	lastPassUniformSlots int
	lastPassUniformWB    int // WriteBuffer calls for pass uniforms this run (0 or 1)
	lastGraphFinishes    int
	lastUsedSharedEnc    bool

	// Stable bind-group cache for continuous effect frames (glow).
	// Keyed by view/uniform pointer + slab offset; cleared when pool/slab rebuilds.
	bgCache map[filterBGKey]hal.BindGroup
	// rtDropGen is the RT cache's destroy counter observed the last time
	// bgCache was validated. Pooled RT views can be destroyed by budget trim /
	// purge without going through a pool rebuild, and a Go allocator happily
	// reuses the freed pointer for the next texture — which would make an old
	// bind group key alias a different texture. Bumped whenever that counter
	// moves, so stale keys can never be hit.
	rtDropGen uint64

	// reuse filter-pass RP descriptor (no per-pass ColorAttachments alloc).
	filterPassRPDesc   hal.RenderPassDescriptor
	filterPassColorAtt [1]hal.RenderPassColorAttachment
	filterPassRPInited bool
}

type filterBGKey struct {
	src, aux hal.TextureView
	ubuf     hal.Buffer
	offset   uint64
}

func (c *filterGPUCache) release() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.releaseUnlocked()
}

// releasePoolUnlocked drains the RT domain. Checked-out RTs (a published
// result the compositor still holds, or a graph run in progress) are left to
// their owners' release path; the cache itself is dropped with the device.
func (c *filterGPUCache) releasePoolUnlocked() {
	c.clearBGCacheUnlocked()
	c.rtCache.ReleaseAll()
	c.rtCache = nil
	c.rtCacheDevice = nil
}

func (c *filterGPUCache) releaseUnlocked() {
	c.releasePoolUnlocked()
	if c.uniform != nil {
		c.uniform.Destroy()
		c.uniform = nil
	}
	if c.staging != nil {
		c.staging.Destroy()
		c.staging = nil
		c.stagingCap = 0
	}
	if c.passUniformSlab != nil {
		c.passUniformSlab.Destroy()
		c.passUniformSlab = nil
		c.passUniformSlabCap = 0
	}
	c.passUniformScratch = nil
	c.lastPassUniformSlots = 0
	c.lastPassUniformWB = 0
	c.lastGraphFinishes = 0
	c.lastUsedSharedEnc = false
	if c.pipeline != nil {
		c.pipeline.Destroy()
		c.pipeline = nil
	}
	if c.bgl != nil {
		c.bgl.Destroy()
		c.bgl = nil
	}
	if c.sampler != nil {
		c.sampler.Destroy()
		c.sampler = nil
	}
	if c.dummyView != nil {
		c.dummyView.Destroy()
		c.dummyView = nil
	}
	if c.dummyTex != nil {
		c.dummyTex.Destroy()
		c.dummyTex = nil
	}
	c.device = nil
	c.outScratch = nil
	c.uploadPad = nil
	c.uniformCPU = nil
}

// filterPassRenderPassDesc fills a reused RenderPassDescriptor for one filter
// full-screen pass. Warm path is 0-alloc.
func (c *filterGPUCache) filterPassRenderPassDesc(dst hal.TextureView) *hal.RenderPassDescriptor {
	if c == nil {
		return nil
	}
	if !c.filterPassRPInited {
		c.filterPassRPDesc.Label = "filter_gpu_pass"
		c.filterPassRPDesc.ColorAttachments = c.filterPassColorAtt[:]
		c.filterPassRPInited = true
	}
	c.filterPassColorAtt[0] = hal.RenderPassColorAttachment{
		View:       dst,
		LoadOp:     types.LoadOpClear,
		StoreOp:    types.StoreOpStore,
		ClearValue: types.Color{},
	}
	return &c.filterPassRPDesc
}

func (c *filterGPUCache) ensure(device hal.Device) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pipeline != nil && c.device == device {
		return nil
	}
	c.releaseUnlocked()
	c.device = device

	shader, err := device.CreateShaderModule(&hal.ShaderModuleDescriptor{
		Label: "filter_gpu_graph",
		WGSL:  filterGPUGraphWGSL,
	})
	if err != nil {
		return err
	}
	defer shader.Destroy()

	bgl, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
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
	layout, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label: "filter_gpu_pipe_layout", BindGroupLayouts: []hal.BindGroupLayout{bgl},
	})
	if err != nil {
		bgl.Destroy()
		return err
	}
	defer layout.Destroy()

	replace := types.BlendState{
		Color: types.BlendComponent{SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorZero, Operation: types.BlendOperationAdd},
		Alpha: types.BlendComponent{SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorZero, Operation: types.BlendOperationAdd},
	}
	pipe, err := device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "filter_gpu_pipe",
		Layout: layout,
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
		bgl.Destroy()
		return err
	}
	samp, err := device.CreateSampler(&hal.SamplerDescriptor{
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
		pipe.Destroy()
		bgl.Destroy()
		return err
	}
	// 1x1 transparent dummy aux
	dtex, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "filter_gpu_dummy",
		Size:          hal.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA8Unorm,
		Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		samp.Destroy()
		pipe.Destroy()
		bgl.Destroy()
		return err
	}
	dview, err := device.CreateTextureView(dtex, &hal.TextureViewDescriptor{
		Label: "filter_gpu_dummy_view", Format: types.TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		dtex.Destroy()
		samp.Destroy()
		pipe.Destroy()
		bgl.Destroy()
		return err
	}

	ubuf, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label: "filter_params_pooled", Size: filterGPUUniformSize,
		Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
	})
	if err != nil {
		dview.Destroy()
		dtex.Destroy()
		samp.Destroy()
		pipe.Destroy()
		bgl.Destroy()
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

// filterRTUsage is the usage mask every graph render target needs: sampled as
// a source, written as an attachment, and copied to/from (publish copy,
// readback).
func filterRTUsage() types.TextureUsage {
	return types.TextureUsageTextureBinding | types.TextureUsageRenderAttachment |
		types.TextureUsageCopySrc | types.TextureUsageCopyDst
}

// filterRTKey is the RT cache key for one w×h RGBA8 target.
func filterRTKey(w, h int) res.CacheKey {
	return res.CacheKey{
		W:       uint32(w), //nolint:gosec // filterGPUMaxPixels caps the size
		H:       uint32(h), //nolint:gosec
		Format:  uint32(types.TextureFormatRGBA8Unorm),
		Samples: 1,
		Usage:   uint64(filterRTUsage()),
		Role:    res.RoleFilterRT,
	}
}

// filterRTNative is one pooled graph render target plus its only view.
type filterRTNative struct {
	tex  hal.Texture
	view hal.TextureView
}

// Release destroys the pair; the registry calls it exactly once, when no
// usage ref and no in-flight submit reference the target.
func (n *filterRTNative) Release() {
	if n == nil {
		return
	}
	if n.view != nil {
		n.view.Destroy()
		n.view = nil
	}
	if n.tex != nil {
		n.tex.Destroy()
		n.tex = nil
	}
}

// bindRes attaches the device-level registry owner and submission gate. Called
// by GPUShared every time it hands out this cache.
func (c *filterGPUCache) bindRes(owner res.Owner, gate *directSubmit) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resOwner = owner
	c.gate = gate
}

// gateOf returns the device-level submission gate (nil in bare-cache mode,
// where submits stay plain Queue.Submit).
func (c *filterGPUCache) gateOf() *directSubmit {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gate
}

// rtCacheLocked returns the RT cache for device, rebuilding it when the device
// changed (the create closure captures the device). A cache without a bound
// registry owner stays nil and callers keep the direct path. Caller holds c.mu;
// no native allocation happens here.
func (c *filterGPUCache) rtCacheLocked(device hal.Device) *res.Cache {
	if c.resOwner.IsNil() || device == nil {
		return nil
	}
	if c.rtCache != nil && c.rtCacheDevice == device {
		return c.rtCache
	}
	c.rtCache.ReleaseAll()
	budget := render.DefaultBudget(render.BudgetFilterGraph)
	cache := res.NewCache(c.resOwner, func(key res.CacheKey) (res.Native, error) {
		tex, err := device.CreateTexture(&hal.TextureDescriptor{
			Label:         "filter_rt",
			Size:          hal.Extent3D{Width: key.W, Height: key.H, DepthOrArrayLayers: 1},
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatRGBA8Unorm,
			Usage:  filterRTUsage(),
		})
		if err != nil {
			return nil, err
		}
		view, err := device.CreateTextureView(tex, &hal.TextureViewDescriptor{
			Label: "filter_rt_view", Format: types.TextureFormatRGBA8Unorm,
			Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
		})
		if err != nil {
			tex.Destroy()
			return nil, err
		}
		return &filterRTNative{tex: tex, view: view}, nil
	}, res.CacheOptions{
		Name:      "filter-graph",
		Budget:    uint64(budget.MaxBytes),
		MaxPerKey: budget.MaxEntries,
		SizeFn:    func(key res.CacheKey) uint64 { return uint64(key.W) * uint64(key.H) * 4 },
	})
	c.rtCache = cache
	c.rtCacheDevice = device
	c.rtDropGen = rtDrops(cache)
	return cache
}

// rtDrops is the RT cache's destroy counter (evictions + purges): every way a
// pooled view can disappear moves it, so it doubles as the bind-group cache's
// invalidation generation.
func rtDrops(cache *res.Cache) uint64 {
	if cache == nil {
		return 0
	}
	st := cache.Stats()
	return st.Evictions + st.Purged
}

// stats snapshots the RT domain for the pool probe (render.GPUPoolStats). It
// never builds the cache: an unused domain reports an empty snapshot, not a
// freshly created pool.
func (c *filterGPUCache) stats() res.CacheStats {
	if c == nil {
		return res.CacheStats{}
	}
	c.mu.Lock()
	cache := c.rtCache
	c.mu.Unlock()
	return cache.Stats()
}

// acquireRT takes one w×h graph render target. The returned Ref is the strong
// cache reference the caller must hand back to recycleRT; a nil Ref means the
// target came from outside the cache (no registry bound) and the caller must
// destroy it.
func (c *filterGPUCache) acquireRT(device hal.Device, w, h int) (hal.Texture, hal.TextureView, res.Ref, error) {
	if w <= 0 || h <= 0 {
		return nil, nil, res.Ref{}, fmt.Errorf("filter gpu rt: bad size %dx%d", w, h)
	}
	c.mu.Lock()
	cache := c.rtCacheLocked(device)
	owner := c.resOwner
	c.mu.Unlock()
	if cache != nil {
		ref, err := cache.Acquire(filterRTKey(w, h))
		if err != nil {
			return nil, nil, res.Ref{}, err
		}
		native, ok := owner.Registry().NativeOf(ref).(*filterRTNative)
		if !ok || native == nil {
			cache.ReleaseRetire(ref)
			return nil, nil, res.Ref{}, fmt.Errorf("filter gpu rt: unexpected native for %dx%d", w, h)
		}
		return native.tex, native.view, ref, nil
	}
	tex, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "filter_rt",
		Size:          hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA8Unorm,
		Usage:  filterRTUsage(),
	})
	if err != nil {
		return nil, nil, res.Ref{}, err
	}
	view, err := device.CreateTextureView(tex, &hal.TextureViewDescriptor{
		Label: "filter_rt_view", Format: types.TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		tex.Destroy()
		return nil, nil, res.Ref{}, err
	}
	return tex, view, res.Ref{}, nil
}

// recycleRT returns a target to the pool, or destroys it when it was created
// outside the cache. track (may be nil) marks the ref in flight for the
// submission that touched it: the session's own submission when the graph ran
// into a shared encoder, nothing for paths that waited on the GPU. Without it,
// re-pooling could lend the target to another window while the GPU is still
// writing it.
func (c *filterGPUCache) recycleRT(ref res.Ref, track func(res.Ref)) {
	if ref.IsNil() {
		return
	}
	c.mu.Lock()
	cache := c.rtCache
	c.mu.Unlock()
	if cache == nil {
		return
	}
	if track != nil {
		track(ref)
	}
	cache.Release(ref)
}

// rtRefs collects the non-nil refs of a run (gate input).
func rtRefs(refs ...res.Ref) []res.Ref {
	out := make([]res.Ref, 0, len(refs))
	for _, r := range refs {
		if !r.IsNil() {
			out = append(out, r)
		}
	}
	return out
}

// releasePooledVRAM drops every idle render target while keeping checked-out
// ones and the pipelines; returns the number of entries freed. Call under VRAM
// pressure before a critical CreateTexture (glyph atlas, etc.).
func (c *filterGPUCache) releasePooledVRAM() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	cache := c.rtCache
	c.mu.Unlock()
	n, _ := cache.PurgeIdle()
	return n
}

func (c *filterGPUCache) ensureStaging(device hal.Device, size uint64) error {
	if c.staging != nil && c.stagingCap >= size && c.device == device {
		return nil
	}
	if c.staging != nil {
		c.staging.Destroy()
		c.staging = nil
		c.stagingCap = 0
	}
	// Cap slightly above request to absorb minor size jitter.
	cap := size
	if cap < 64*1024 {
		cap = 64 * 1024
	}
	stg, err := device.CreateBuffer(&hal.BufferDescriptor{
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
			bg.Destroy()
		}
		delete(c.bgCache, k)
	}
	if c.bgCache == nil {
		c.bgCache = make(map[filterBGKey]hal.BindGroup)
	}
}

func (c *filterGPUCache) bindGroup(device hal.Device, bgl hal.BindGroupLayout, samp hal.Sampler,
	src, dst, aux hal.TextureView, ubuf hal.Buffer, offset uint64,
) (hal.BindGroup, error) {
	if device == nil || bgl == nil || samp == nil || src == nil || dst == nil || aux == nil || ubuf == nil {
		return nil, fmt.Errorf("filter bg: nil arg")
	}
	key := filterBGKey{
		src:    src,
		aux:    aux,
		ubuf:   ubuf,
		offset: offset,
	}
	// dst is render attachment, not bind-group entry — key is src/aux/ubuf/offset.
	c.mu.Lock()
	if c.bgCache == nil {
		c.bgCache = make(map[filterBGKey]hal.BindGroup)
	}
	if bg := c.bgCache[key]; bg != nil {
		c.mu.Unlock()
		return bg, nil
	}
	c.mu.Unlock()

	bg, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "filter_gpu_bg_cached",
		Layout: bgl,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, TextureView: src},
			{Binding: 1, Sampler: samp},
			{Binding: 2, Buffer: ubuf, Offset: offset, Size: filterGPUUniformSize},
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
		bg.Destroy()
		return prev, nil
	}
	c.bgCache[key] = bg
	c.mu.Unlock()
	return bg, nil
}

// Recreating the slab clears the bind-group cache (entries pin the old buffer).
func (c *filterGPUCache) ensurePassUniformSlab(device hal.Device, nSlots int) (hal.Buffer, error) {
	if device == nil {
		return nil, fmt.Errorf("filter pass uniform slab: nil device")
	}
	if nSlots < 1 {
		nSlots = 1
	}
	need := uint64(nSlots) * filterPassUniformSlotStride
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.passUniformSlab != nil && c.device == device && c.passUniformSlabCap >= need {
		return c.passUniformSlab, nil
	}
	alloc := need
	minBytes := uint64(filterPassUniformSlabMinSlots) * filterPassUniformSlotStride
	if alloc < minBytes {
		alloc = minBytes
	}
	// Grow with headroom so continuous glow frames rarely reallocate.
	if alloc < c.passUniformSlabCap*2 && c.passUniformSlabCap*2 > alloc {
		alloc = c.passUniformSlabCap * 2
	}
	b, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label: "filter_params_pass_slab", Size: alloc,
		Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, err
	}
	if c.passUniformSlab != nil {
		c.passUniformSlab.Destroy()
		c.passUniformSlab = nil
	}
	c.passUniformSlab = b
	c.passUniformSlabCap = alloc
	c.clearBGCacheUnlocked()
	return b, nil
}

// acquirePublish takes a w×h target for a result that must not be one of the
// run's own ping-pong/hold targets (an external-only source). Same pool as the
// graph RTs, so steady-state glow frames neither allocate VRAM nor copy: the
// pool hands back what the previous frame released.
func (c *filterGPUCache) acquirePublish(device hal.Device, w, h int) (filterPublishSlot, error) {
	tex, view, ref, err := c.acquireRT(device, w, h)
	if err != nil {
		return filterPublishSlot{}, err
	}
	return filterPublishSlot{tex: tex, view: view, ref: ref}, nil
}

// releasePublish returns a published result to the pool (destroying it when it
// predates the cache). track marks the submission that still samples the
// result, so it cannot be re-lent or drained before the GPU is done with it;
// nil track matches the pre-P2.4d contract (the strong ref protected it while
// published, and the pool takes it back as soon as that ref is gone).
func (c *filterGPUCache) releasePublish(slot filterPublishSlot, track func(res.Ref)) {
	if slot.tex == nil || slot.view == nil {
		return
	}
	if slot.ref.IsNil() {
		slot.view.Destroy()
		slot.tex.Destroy()
		return
	}
	c.recycleRT(slot.ref, track)
}

// runGPUFilterGraph executes multi-RT ping-pong filter nodes on GPU and readbacks.
func runGPUFilterGraph(device hal.Device, queue hal.Queue, cache *filterGPUCache, src []byte, w, h int, nodes []render.ImageFilterNode) ([]byte, error) {
	out, _, _, err := runGPUFilterGraphEx(device, queue, cache, src, nil, w, h, nodes, true, nil, nil, nil)
	return out, err
}

// runGPUFilterGraphGPUOnly runs the filter graph and publishes a GPU texture view
// for zero-copy compositing (DrawGPUTexture). No CPU Map/readback.
// Caller must invoke release when the view is no longer sampled.
func runGPUFilterGraphGPUOnly(device hal.Device, queue hal.Queue, cache *filterGPUCache, src []byte, w, h int, nodes []render.ImageFilterNode) (gpucontext.TextureView, func(), error) {
	_, view, release, err := runGPUFilterGraphEx(device, queue, cache, src, nil, w, h, nodes, false, nil, nil, nil)
	return view, release, err
}

// runGPUFilterGraphFromView seeds the graph from an existing GPU texture view
// (no CPU upload). Source may be BGRA offscreen; first copy pass samples into
// the RGBA pool.
func runGPUFilterGraphFromView(device hal.Device, queue hal.Queue, cache *filterGPUCache, srcView gpucontext.TextureView, w, h int, nodes []render.ImageFilterNode) (gpucontext.TextureView, func(), error) {
	return runGPUFilterGraphFromViewWithLeading(device, queue, cache, srcView, w, h, nodes, nil)
}

// runGPUFilterGraphFromViewWithLeading is FromView with optional leading command
// buffers submitted in the same Queue.Submit (mesh seed + filter = one submit).
// Leading CBs are Released after submit (success or failure). On filter encode
// failure before Submit, leading CBs are NOT released — caller may submit them
// alone for recovery.
func runGPUFilterGraphFromViewWithLeading(device hal.Device, queue hal.Queue, cache *filterGPUCache, srcView gpucontext.TextureView, w, h int, nodes []render.ImageFilterNode, leading []hal.CommandBuffer) (gpucontext.TextureView, func(), error) {
	if srcView.IsNil() {
		return gpucontext.TextureView{}, nil, fmt.Errorf("filter gpu: nil src view")
	}
	halView := unpackView(srcView)
	_, view, release, err := runGPUFilterGraphEx(device, queue, cache, nil, halView, w, h, nodes, false, leading, nil, nil)
	return view, release, err
}

// runGPUFilterGraphFromViewIntoEncoder continues the filter graph on an open
// encoder that already contains the mesh-seed render passes. One Finish
// covers seed+filter. On failure the encoder is left open so the caller can
// Finish+Submit seed-only recovery (mesh draws are only applied if submitted).
func runGPUFilterGraphFromViewIntoEncoder(device hal.Device, queue hal.Queue, cache *filterGPUCache, srcView gpucontext.TextureView, w, h int, nodes []render.ImageFilterNode, sharedEnc hal.CommandEncoder, track func(res.Ref)) (gpucontext.TextureView, func(), error) {
	if srcView.IsNil() {
		return gpucontext.TextureView{}, nil, fmt.Errorf("filter gpu: nil src view")
	}
	if sharedEnc == nil {
		return gpucontext.TextureView{}, nil, fmt.Errorf("filter gpu: nil shared encoder")
	}
	halView := unpackView(srcView)
	_, view, release, err := runGPUFilterGraphEx(device, queue, cache, nil, halView, w, h, nodes, false, nil, sharedEnc, track)
	return view, release, err
}

func runGPUFilterGraphEx(
	device hal.Device, queue hal.Queue, cache *filterGPUCache,
	src []byte, srcView hal.TextureView, w, h int, nodes []render.ImageFilterNode, wantPixels bool,
	leading []hal.CommandBuffer, sharedEnc hal.CommandEncoder, track func(res.Ref),
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
	uGPU := cache.uniform
	ubuf := cache.uniformCPU
	pipe, bgl, samp, dummy := cache.pipeline, cache.bgl, cache.sampler, cache.dummyView
	// Bind groups are keyed by view pointers; a pooled target destroyed since
	// the last validation could have had its pointer reused by the next
	// texture, so re-validate the whole map when the drop counter moves.
	if rtDrops(cache.rtCache) != cache.rtDropGen {
		cache.clearBGCacheUnlocked()
		cache.rtDropGen = rtDrops(cache.rtCache)
	}
	cache.mu.Unlock()

	// Three targets per run: ping-pong A/B plus the shadow-hold input. The
	// refs travel with the views through the swap; the one that ends up
	// published leaves the run with the slot, the rest go back to the pool.
	var pubRef res.Ref
	var slot filterPublishSlot
	var submitted bool
	// giveBack returns one run target (or a discarded publish slot). A ref
	// that reached the published slot stays with it. Once the graph's own
	// Submit succeeded, the run's refs are covered by the device submission
	// gate until the fence resolves, so pooling them needs no tracker; in the
	// bare-cache path (no registry) the run owns its targets outright.
	giveBack := func(r res.Ref, tex hal.Texture, view hal.TextureView) {
		if r.IsNil() {
			if tex != nil && tex != slot.tex {
				if view != nil {
					view.Destroy()
				}
				tex.Destroy()
			}
			return
		}
		if r == pubRef {
			return
		}
		if !submitted && sharedEnc != nil {
			// Recovery hand-off: the caller may still Finish+Submit the open
			// encoder, which references this target. Hold the ref — the old
			// fixed pool left the targets alive in exactly this case, and the
			// registry owner frees them at device teardown.
			return
		}
		cache.recycleRT(r, nil)
	}
	texA, viewA, refA, errA := cache.acquireRT(device, w, h)
	if errA != nil {
		return nil, gpucontext.TextureView{}, nil, errA
	}
	texB, viewB, refB, errB := cache.acquireRT(device, w, h)
	if errB != nil {
		giveBack(refA, texA, viewA)
		return nil, gpucontext.TextureView{}, nil, errB
	}
	texH, viewH, refH, errH := cache.acquireRT(device, w, h)
	if errH != nil {
		giveBack(refA, texA, viewA)
		giveBack(refB, texB, viewB)
		return nil, gpucontext.TextureView{}, nil, errH
	}
	defer func() {
		giveBack(refA, texA, viewA)
		giveBack(refB, texB, viewB)
		giveBack(refH, texH, viewH)
	}()
	// dropSlot discards a publish slot taken on the fallback copy path (error
	// returns only — the success path hands it to the caller).
	dropSlot := func() {
		if slot.tex == nil {
			return
		}
		giveBack(slot.ref, slot.tex, slot.view)
		slot = filterPublishSlot{}
	}

	if texA == nil || viewA == nil || texB == nil || viewB == nil || texH == nil || viewH == nil ||
		uGPU == nil || len(ubuf) < filterGPUUniformSize || pipe == nil || bgl == nil || samp == nil || dummy == nil {
		return nil, gpucontext.TextureView{}, nil, fmt.Errorf("filter gpu pool missing")
	}

	curTex, curView, curRef := texA, viewA, refA
	nxtTex, nxtView, nxtRef := texB, viewB, refB
	gate := cache.gateOf()

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
			&hal.ImageCopyTexture{Texture: texA, MipLevel: 0},
			upload,
			&hal.ImageDataLayout{BytesPerRow: bpr, RowsPerImage: uint32(h)},           //nolint:gosec
			&hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec
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
		lightX, lightY, lightZ float32
		lightKD, lightKS       float32
		lightR, lightG, lightB float32
		shininess, surfScale   float32
		tileX0, tileY0         float32
		tileX1, tileY1         float32
		auxView                hal.TextureView
		dstView                hal.TextureView
		srcView                hal.TextureView
	}

	// One command encoder for all passes + publish copy (single queue submit).
	// pass uniforms share one slab (distinct offsets). Pack into CPU
	// scratch during encode; single WriteBuffer before Finish/Submit so each
	// bind range has distinct live content (cannot rewrite one uniform mid-CB).
	estSlots := len(nodes)*4 + 2
	if estSlots < filterPassUniformSlabMinSlots {
		estSlots = filterPassUniformSlabMinSlots
	}
	slab, err := cache.ensurePassUniformSlab(device, estSlots)
	if err != nil {
		return nil, gpucontext.TextureView{}, nil, err
	}
	cache.mu.Lock()
	if cap(cache.passUniformScratch) < estSlots*filterPassUniformSlotStride {
		cache.passUniformScratch = make([]byte, estSlots*filterPassUniformSlotStride)
	}
	passScratch := cache.passUniformScratch
	cache.lastGraphFinishes = 0
	cache.lastUsedSharedEnc = sharedEnc != nil
	cache.mu.Unlock()
	nPassSlots := 0
	var enc hal.CommandEncoder
	ownsEncoder := false
	if sharedEnc != nil {
		enc = sharedEnc
	} else {
		enc, err = device.CreateCommandEncoder(filterGPUBatchEncoderDesc)
		if err != nil {
			return nil, gpucontext.TextureView{}, nil, err
		}
		ownsEncoder = true
	}
	// discardOnErr only owns the encoder we created; sharedEnc is left open on
	// failure so FlushAndFilter can Finish+Submit mesh seed for recovery.
	discardOnErr := func() {
		if ownsEncoder && enc != nil {
			enc.DiscardEncoding()
		}
	}
	cleanupPasses := func() {
		// slab + bind groups owned by filterGPUCache
	}

	flushPassUniforms := func() error {
		if nPassSlots == 0 {
			cache.mu.Lock()
			cache.lastPassUniformSlots = 0
			cache.lastPassUniformWB = 0
			cache.mu.Unlock()
			return nil
		}
		needBytes := nPassSlots * filterPassUniformSlotStride
		if cap(passScratch) < needBytes {
			return fmt.Errorf("filter pass uniform scratch short: cap=%d need=%d", cap(passScratch), needBytes)
		}
		// Ensure slab covers packed slots (rare growth if estimate was low).
		if uint64(needBytes) > 0 {
			s2, e2 := cache.ensurePassUniformSlab(device, nPassSlots)
			if e2 != nil {
				return e2
			}
			if s2 != slab {
				// Slab reallocated after bind groups were built — not recoverable
				// in this run (BGs pin old buffer). Estimate should prevent this.
				return fmt.Errorf("filter pass uniform slab grew mid-run (slots=%d)", nPassSlots)
			}
		}
		if err := queue.WriteBuffer(slab, 0, passScratch[:needBytes]); err != nil {
			return err
		}
		cache.mu.Lock()
		cache.lastPassUniformSlots = nPassSlots
		cache.lastPassUniformWB = 1
		cache.mu.Unlock()
		return nil
	}

	doPass := func(a passArgs) error {
		slot := nPassSlots
		nPassSlots++
		if slot >= estSlots {
			// Grow scratch; slab was pre-sized with headroom via ensure min/growth.
			needSlots := slot + 1
			if needSlots < filterPassUniformSlabMinSlots {
				needSlots = filterPassUniformSlabMinSlots
			}
			s2, e2 := cache.ensurePassUniformSlab(device, needSlots)
			if e2 != nil {
				return e2
			}
			if s2 != slab {
				return fmt.Errorf("filter pass uniform slab grew after bind groups (slot=%d)", slot)
			}
			needBytes := needSlots * filterPassUniformSlotStride
			if cap(passScratch) < needBytes {
				n := make([]byte, needBytes)
				copy(n, passScratch[:slot*filterPassUniformSlotStride])
				passScratch = n
				cache.mu.Lock()
				cache.passUniformScratch = n
				cache.mu.Unlock()
			}
			estSlots = needSlots
		}
		off := slot * filterPassUniformSlotStride
		slotBuf := passScratch[off : off+filterGPUUniformSize]
		// all 128B Params fields written below (no clear needed).
		// Direct LE store (same bits as prior byte-wise pack).
		putF32 := func(o int, v float32) {
			*(*uint32)(unsafe.Pointer(&slotBuf[o])) = math.Float32bits(v) //nolint:gosec
		}
		putU32 := func(o int, v uint32) {
			*(*uint32)(unsafe.Pointer(&slotBuf[o])) = v //nolint:gosec
		}
		putF32(128, a.lightX)
		putF32(132, a.lightY)
		putF32(136, a.lightZ)
		putF32(140, a.lightKD)
		putF32(144, a.lightR)
		putF32(148, a.lightG)
		putF32(152, a.lightB)
		putF32(156, a.lightKS)
		putF32(160, a.shininess)
		putF32(164, a.surfScale)
		putF32(168, a.tileX0)
		putF32(172, a.tileY0)
		putF32(176, a.tileX1)
		putF32(180, a.tileY1)
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

		bg, err := cache.bindGroup(device, bgl, samp, srcV, dstV, auxV, slab, uint64(off))
		if err != nil {
			return err
		}
		// Cached bind groups are owned by filterGPUCache — do not Release in cleanup.

		rp, err := enc.BeginRenderPass(cache.filterPassRenderPassDesc(dstV))
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
		curTex, curView, curRef = nil, srcView, res.Ref{}
		nxtTex, nxtView, nxtRef = texA, viewA, refA
	}

	swap := func() {
		if fromExternal && curTex == nil {
			// First result is in nxt (A). Next destination must be B.
			curTex, curView, curRef = nxtTex, nxtView, nxtRef
			nxtTex, nxtView, nxtRef = texB, viewB, refB
			fromExternal = false
			return
		}
		curTex, nxtTex = nxtTex, curTex
		curView, nxtView = nxtView, curView
		curRef, nxtRef = nxtRef, curRef
	}

	for i := range nodes {
		n := nodes[i]
		switch n.Kind {
		case render.ImageFilterGrayscale:
			if err := doPass(passArgs{mode: 1}); err != nil {
				discardOnErr()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
		case render.ImageFilterInvert:
			if err := doPass(passArgs{mode: 2}); err != nil {
				discardOnErr()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
		case render.ImageFilterErode:
			if err := doPass(passArgs{mode: 7, radius: uint32(n.MorphRadius)}); err != nil {
				discardOnErr()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
		case render.ImageFilterDilate:
			if err := doPass(passArgs{mode: 8, radius: uint32(n.MorphRadius)}); err != nil {
				discardOnErr()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
		case render.ImageFilterOffset:
			if err := doPass(passArgs{mode: 9, offX: float32(n.IntOffsetX), offY: float32(n.IntOffsetY)}); err != nil {
				discardOnErr()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
		case render.ImageFilterAlphaThreshold:
			if err := doPass(passArgs{mode: 10, colA: float32(n.AlphaThreshold) / 255.0}); err != nil {
				discardOnErr()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
		case render.ImageFilterLighting:
			if err := doPass(passArgs{mode: 11,
				lightX: n.LightPos[0], lightY: n.LightPos[1], lightZ: n.LightPos[2],
				lightKD: n.LightKD, lightKS: 0,
				lightR: float32(n.LightColor.R), lightG: float32(n.LightColor.G), lightB: float32(n.LightColor.B),
				surfScale: n.SurfaceScale,
			}); err != nil {
				discardOnErr()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
		case render.ImageFilterTile:
			if err := doPass(passArgs{mode: 12,
				tileX0: n.TileSrcRect[0], tileY0: n.TileSrcRect[1],
				tileX1: n.TileSrcRect[2], tileY1: n.TileSrcRect[3],
			}); err != nil {
				discardOnErr()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
		case render.ImageFilterBlur:
			sigma := math.Max(0.01, n.Radius)
			r := uint32(math.Max(1, math.Ceil(sigma*3)))
			if err := doPass(passArgs{mode: 0, radius: r, dirX: 1, dirY: 0, offX: float32(sigma)}); err != nil {
				discardOnErr()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
			if err := doPass(passArgs{mode: 0, radius: r, dirX: 0, dirY: 1, offX: float32(sigma)}); err != nil {
				discardOnErr()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
		case render.ImageFilterBlurXY:
			sx := math.Max(0, n.RadiusX)
			sy := math.Max(0, n.RadiusY)
			if sx > 0 {
				rx := uint32(math.Max(1, math.Ceil(sx*3)))
				if err := doPass(passArgs{mode: 0, radius: rx, dirX: 1, dirY: 0, offX: float32(sx)}); err != nil {
					return nil, gpucontext.TextureView{}, nil, err
				}
				swap()
			}
			if sy > 0 {
				ry := uint32(math.Max(1, math.Ceil(sy*3)))
				if err := doPass(passArgs{mode: 0, radius: ry, dirX: 0, dirY: 1, offX: float32(sy)}); err != nil {
					return nil, gpucontext.TextureView{}, nil, err
				}
				swap()
			}
		case render.ImageFilterColorMatrix:
			if err := doPass(passArgs{mode: 3, matrix: n.Matrix}); err != nil {
				discardOnErr()
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
				discardOnErr()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
			if n.ShadowBlur > 0 {
				sigma := math.Max(0.01, n.ShadowBlur)
				r := uint32(math.Max(1, math.Ceil(sigma*3)))
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
				discardOnErr()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			swap()
		default:
			discardOnErr()
			cleanupPasses()
			return nil, gpucontext.TextureView{}, nil, fmt.Errorf("filter gpu unsupported node %v", n.Kind)
		}
	}

	// Finish graph: the result's own pool ref travels into the publish slot —
	// zero-copy, zero-alloc in steady state (the compositor's release is what
	// hands it back). Only a result that is not one of the run's targets
	// (external-only source with no pass written into A/B) needs a second
	// pooled target and a GPU copy.
	if !wantPixels {
		if curTex == nil || curView == nil {
			err = fmt.Errorf("filter gpu publish: nil result tex")
		} else {
			slot = filterPublishSlot{tex: curTex, view: curView, ref: curRef}
			pubRef = curRef
		}
		if err != nil {
			// Fallback: acquire publish RT + GPU copy (keeps correctness).
			slot, err = cache.acquirePublish(device, w, h)
			if err != nil {
				pubRef = res.Ref{}
				discardOnErr()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, err
			}
			if curTex != nil {
				enc.CopyTextureToTexture(curTex, slot.tex, []hal.TextureCopy{{
					Source: hal.ImageCopyTexture{
						Texture: curTex, MipLevel: 0,
						Origin: hal.Origin3D{}, Aspect: types.TextureAspectAll,
					},
					Destination: hal.ImageCopyTexture{
						Texture: slot.tex, MipLevel: 0,
						Origin: hal.Origin3D{}, Aspect: types.TextureAspectAll,
					},
					Size: hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec
				}})
			} else if curView != nil {
				// External-only: sample-copy into publish via mode-6 pass.
				if err := doPass(passArgs{mode: 6, srcView: curView, dstView: slot.view}); err != nil {
					dropSlot()
					discardOnErr()
					cleanupPasses()
					return nil, gpucontext.TextureView{}, nil, err
				}
			} else {
				dropSlot()
				discardOnErr()
				cleanupPasses()
				return nil, gpucontext.TextureView{}, nil, fmt.Errorf("filter gpu publish: no source")
			}
		}
	}
	// one WriteBuffer for all pass uniforms before Finish/Submit.
	if err := flushPassUniforms(); err != nil {
		dropSlot()
		discardOnErr()
		cleanupPasses()
		return nil, gpucontext.TextureView{}, nil, err
	}
	if sharedEnc != nil && len(leading) > 0 {
		// Mesh seed should already live on sharedEnc; leading is unused.
		return nil, gpucontext.TextureView{}, nil, fmt.Errorf("filter gpu: sharedEnc with leading CBs")
	}
	cmd, err := enc.Finish()
	if err != nil {
		dropSlot()
		cleanupPasses()
		return nil, gpucontext.TextureView{}, nil, err
	}
	cache.mu.Lock()
	cache.lastGraphFinishes = 1
	cache.mu.Unlock()
	// Single Queue.Submit for optional leading seed CBs + filter CB.
	// Leading must run first so FromView samples a populated seed RT.
	nLead := len(leading)
	var all []hal.CommandBuffer
	if nLead == 0 {
		all = []hal.CommandBuffer{cmd}
	} else {
		all = make([]hal.CommandBuffer, 0, nLead+1)
		all = append(all, leading...)
		all = append(all, cmd)
	}
	// Queue.Submit takes hal.CommandBuffer: convert element-wise (same
	// elements, same order, behavior unchanged).
	halAll := make([]hal.CommandBuffer, 0, len(all))
	for _, cb := range all {
		halAll = append(halAll, cb)
	}
	if _, err := submitGated(gate, queue, rtRefs(refA, refB, refH, slot.ref), halAll...); err != nil {
		for _, c := range leading {
			if c != nil {
				device.FreeCommandBuffer(c)
			}
		}
		device.FreeCommandBuffer(cmd)
		dropSlot()
		cleanupPasses()
		return nil, gpucontext.TextureView{}, nil, err
	}
	for _, c := range leading {
		if c != nil {
			device.FreeCommandBuffer(c)
		}
	}
	device.FreeCommandBuffer(cmd)
	cleanupPasses()

	submitted = true
	if !wantPixels {
		view := packView(slot.view)
		release := func() { cache.releasePublish(slot, track) }
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

	enc2, err := device.CreateCommandEncoder(filterGPUReadEncoderDesc)
	if err != nil {
		return nil, gpucontext.TextureView{}, nil, err
	}
	enc2.CopyTextureToBuffer(curTex, staging, []hal.BufferTextureCopy{{
		BufferLayout: hal.ImageDataLayout{BytesPerRow: alignedRow, RowsPerImage: uint32(h)}, //nolint:gosec
		TextureBase:  hal.ImageCopyTexture{Texture: curTex, MipLevel: 0, Aspect: types.TextureAspectAll},
		Size:         hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, //nolint:gosec
	}})
	cmd2, err := enc2.Finish()
	if err != nil {
		return nil, gpucontext.TextureView{}, nil, err
	}
	if _, err := submitGated(gate, queue, rtRefs(curRef), cmd2); err != nil {
		device.FreeCommandBuffer(cmd2)
		return nil, gpucontext.TextureView{}, nil, err
	}
	device.FreeCommandBuffer(cmd2)
	device.Poll(hal.PollWait)

	mapping, mErr := device.MapBuffer(staging, 0, stagingSize)
	if mErr != nil {
		return nil, gpucontext.TextureView{}, nil, mErr
	}
	srcMapped := unsafe.Slice((*byte)(mapping.Ptr), stagingSize) //nolint:gosec // hal.BufferMapping opaque pointer
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
	_ = device.UnmapBuffer(staging)
	return out, gpucontext.TextureView{}, nil, nil
}
