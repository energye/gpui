//go:build !nogpu

package gpu

import (
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"math"
	"sync"

	gpucontext "github.com/energye/gpui/gpu/context"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	"unsafe"
)

// dual-tex advanced blend shader: sample dest + src, write composited premul RGBA.
// Mode codes: 1=Mul 2=Screen 3=Overlay 4=Hue 5=Sat 6=Color 7=Lum
// 8=Darken 9=Lighten 10=ColorDodge 11=ColorBurn 12=HardLight 13=SoftLight 14=Diff 15=Exclusion.
const dualTexBlendWGSL = `
struct VSOut {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

struct Params {
    mode: u32,
    // non-zero => dst_tex is tight damage snap; sample with in.uv (0-1).
    // zero => sample dst with same UV rect as src (full-texture path).
    dst_tight: u32,
    // Sample rect for src (and dst when dst_tight==0) in 0-1 texture space.
    uv_min: vec2<f32>,
    uv_max: vec2<f32>,
    opacity: f32,
    _pad1: f32,
    _pad2: f32,
    _pad3: f32,
}

@group(0) @binding(0) var dst_tex: texture_2d<f32>;
@group(0) @binding(1) var src_tex: texture_2d<f32>;
@group(0) @binding(2) var samp: sampler;
@group(0) @binding(3) var<uniform> params: Params;

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

fn luminosity(c: vec3<f32>) -> f32 {
    return 0.2126 * c.r + 0.7152 * c.g + 0.0722 * c.b;
}
fn saturation(c: vec3<f32>) -> f32 {
    return max(c.r, max(c.g, c.b)) - min(c.r, min(c.g, c.b));
}
fn clip_color(c: vec3<f32>) -> vec3<f32> {
    let lum = luminosity(c);
    let n = min(c.r, min(c.g, c.b));
    let x = max(c.r, max(c.g, c.b));
    var result = c;
    if (n < 0.0) {
        result = lum + (result - lum) * lum / (lum - n);
    }
    if (x > 1.0) {
        result = lum + (result - lum) * (1.0 - lum) / (x - lum);
    }
    return result;
}
fn set_lum(c: vec3<f32>, lum: f32) -> vec3<f32> {
    let d = lum - luminosity(c);
    return clip_color(c + d);
}
fn set_sat(c: vec3<f32>, sat: f32) -> vec3<f32> {
    let min_c = min(c.r, min(c.g, c.b));
    let max_c = max(c.r, max(c.g, c.b));
    if (max_c > min_c) {
        let scale = sat / (max_c - min_c);
        return (c - min_c) * scale;
    }
    return vec3<f32>(0.0);
}
fn blend_hue(cs: vec3<f32>, cb: vec3<f32>) -> vec3<f32> {
    return set_lum(set_sat(cs, saturation(cb)), luminosity(cb));
}
fn blend_sat(cs: vec3<f32>, cb: vec3<f32>) -> vec3<f32> {
    return set_lum(set_sat(cb, saturation(cs)), luminosity(cb));
}
fn blend_color(cs: vec3<f32>, cb: vec3<f32>) -> vec3<f32> {
    return set_lum(cs, luminosity(cb));
}
fn blend_lum(cs: vec3<f32>, cb: vec3<f32>) -> vec3<f32> {
    return set_lum(cb, luminosity(cs));
}

// blend_fn(mode, backdrop cb, source cs)
fn blend_channel_hardlight(cb: f32, cs: f32) -> f32 {
    // decision on source
    if (cs <= 0.5) {
        return 2.0 * cb * cs;
    }
    return 1.0 - 2.0 * (1.0 - cb) * (1.0 - cs);
}
fn blend_channel_softlight(cb: f32, cs: f32) -> f32 {
    if (cs <= 0.5) {
        return cb - (1.0 - 2.0 * cs) * cb * (1.0 - cb);
    }
    var d: f32;
    if (cb <= 0.25) {
        d = ((16.0 * cb - 12.0) * cb + 4.0) * cb;
    } else {
        d = sqrt(cb);
    }
    return cb + (2.0 * cs - 1.0) * (d - cb);
}
fn blend_channel_dodge(cb: f32, cs: f32) -> f32 {
    if (cs >= 1.0) { return 1.0; }
    return min(1.0, cb / (1.0 - cs));
}
fn blend_channel_burn(cb: f32, cs: f32) -> f32 {
    if (cs <= 0.0) { return 0.0; }
    return max(0.0, 1.0 - (1.0 - cb) / cs);
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
    if (mode == 4u) { return blend_hue(cs, cb); }
    if (mode == 5u) { return blend_sat(cs, cb); }
    if (mode == 6u) { return blend_color(cs, cb); }
    if (mode == 7u) { return blend_lum(cs, cb); }
    if (mode == 8u) {
        return min(cb, cs);
    }
    if (mode == 9u) {
        return max(cb, cs);
    }
    if (mode == 10u) {
        return vec3<f32>(blend_channel_dodge(cb.r, cs.r), blend_channel_dodge(cb.g, cs.g), blend_channel_dodge(cb.b, cs.b));
    }
    if (mode == 11u) {
        return vec3<f32>(blend_channel_burn(cb.r, cs.r), blend_channel_burn(cb.g, cs.g), blend_channel_burn(cb.b, cs.b));
    }
    if (mode == 12u) {
        return vec3<f32>(blend_channel_hardlight(cb.r, cs.r), blend_channel_hardlight(cb.g, cs.g), blend_channel_hardlight(cb.b, cs.b));
    }
    if (mode == 13u) {
        return vec3<f32>(blend_channel_softlight(cb.r, cs.r), blend_channel_softlight(cb.g, cs.g), blend_channel_softlight(cb.b, cs.b));
    }
    if (mode == 14u) {
        return abs(cs - cb);
    }
    if (mode == 15u) {
        return cs + cb - 2.0 * cs * cb;
    }
    // Multiply (1) default
    return vec3<f32>(
        blend_channel_multiply(cb.r, cs.r),
        blend_channel_multiply(cb.g, cs.g),
        blend_channel_multiply(cb.b, cs.b),
    );
}

// W3C Compositing: backdrop b, source s; advanced color B(Cb,Cs)
@fragment
fn fs_main(in: VSOut) -> @location(0) vec4<f32> {
    let suv = mix(params.uv_min, params.uv_max, in.uv);
    var duv = suv;
    if (params.dst_tight != 0u) {
        duv = in.uv;
    }
    let dp = textureSample(dst_tex, samp, duv);
    let sp = textureSample(src_tex, samp, suv);
    let d = unpremul(dp);
    let s = unpremul(sp);
    let ab = d.a;
    let as_ = s.a;
    let bcol = blend_fn(params.mode, d.rgb, s.rgb);
    let co = (1.0 - ab) * s.rgb + (1.0 - as_) * d.rgb + ab * as_ * bcol;
    let ao = as_ + ab * (1.0 - as_);
    let op = params.opacity;
    return vec4<f32>(co * ao * op, ao * op);
}
`

// R7.3: static encoder descriptors (no per-call heap for Label string header).
var (
	dualTexMultiEncoderDesc = &webgpu.CommandEncoderDescriptor{Label: "dual_tex_multi_enc"}
	dualTexViewsEncoderDesc = &webgpu.CommandEncoderDescriptor{Label: "dual_tex_views_bgra_enc"}
)

// dualTexBlendCache holds reusable GPU objects for dual-texture advanced blend.
type dualTexBlendCache struct {
	mu           sync.Mutex
	device       *webgpu.Device
	shader       *webgpu.ShaderModule
	bgl          *webgpu.BindGroupLayout
	pipeLay      *webgpu.PipelineLayout
	pipeline     *webgpu.RenderPipeline // RGBA8 target
	pipelineBGRA *webgpu.RenderPipeline // BGRA8 target (layers/swapchain)
	sampler      *webgpu.Sampler
	uniform      *webgpu.Buffer
	// Multi-layer single-submit uniform ring (WriteBuffer before Submit).
	uniformRing []*webgpu.Buffer
	// F1: pool bounds-sized BGRA temps (out / dest snaps).
	outPool map[[2]int][]dualTexPooledTex
	// cool holds temps for 2 frames before reusing (async GPU safety).
	cool []dualTexCoolItem
	// fullSnaps: double-buffered window-sized dest snaps (into-dest path).
	fullSnaps [2]dualTexFullSnap
	fullSnapI int
	// deferredViews: src views recorded into an external encoder; released after
	// a short cool-down so Finish/Submit can still sample them (F1 single-submit).
	deferredViews []*webgpu.TextureView
	// Full BGRA texture views cached by texture pointer (stable layer RTs).
	fullViewCache map[uintptr]*webgpu.TextureView
	// Bind groups keyed by dst/src/uniform view pointers (opt26 multi-slot reuse).
	bgCache map[dualTexBGKey]*webgpu.BindGroup
	// multiBG: per-op slot reuse for dualTexAdvancedBlendViewsMultiBundle.
	// After Queue.Submit of the previous frame, slot BGs are safe to replace.
	multiBG       []dualTexMultiBGSlot
	paramsScratch []byte
}

type dualTexBGKey struct {
	dst, src, ubuf uintptr
}

type dualTexMultiBGSlot struct {
	key dualTexBGKey
	bg  *webgpu.BindGroup
}

type dualTexPooledTex struct {
	tex  *webgpu.Texture
	view *webgpu.TextureView
}

type dualTexCoolItem struct {
	tex  *webgpu.Texture
	view *webgpu.TextureView
	w, h int
	age  int
}

// dualTexFullSnap is a reusable full-surface dest snapshot for into-dest blend.
type dualTexFullSnap struct {
	tex  *webgpu.Texture
	view *webgpu.TextureView
	w, h int
}

func (c *dualTexBlendCache) release() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pipeline != nil {
		c.pipeline.Release()
		c.pipeline = nil
	}
	if c.pipelineBGRA != nil {
		c.pipelineBGRA.Release()
		c.pipelineBGRA = nil
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
	for _, u := range c.uniformRing {
		if u != nil {
			u.Release()
		}
	}
	c.uniformRing = nil
	for _, bucket := range c.outPool {
		for _, it := range bucket {
			if it.view != nil {
				it.view.Release()
			}
			if it.tex != nil {
				it.tex.Release()
			}
		}
	}
	c.outPool = nil
	for _, it := range c.cool {
		if it.view != nil {
			it.view.Release()
		}
		if it.tex != nil {
			it.tex.Release()
		}
	}
	c.cool = nil
	for i := range c.fullSnaps {
		if c.fullSnaps[i].view != nil {
			c.fullSnaps[i].view.Release()
			c.fullSnaps[i].view = nil
		}
		if c.fullSnaps[i].tex != nil {
			c.fullSnaps[i].tex.Release()
			c.fullSnaps[i].tex = nil
		}
		c.fullSnaps[i].w, c.fullSnaps[i].h = 0, 0
	}
	for _, sv := range c.deferredViews {
		if sv != nil {
			sv.Release()
		}
	}
	c.deferredViews = nil
	for _, v := range c.fullViewCache {
		if v != nil {
			v.Release()
		}
	}
	c.fullViewCache = nil
	for _, bg := range c.bgCache {
		if bg != nil {
			bg.Release()
		}
	}
	c.bgCache = nil
	for i := range c.multiBG {
		if c.multiBG[i].bg != nil {
			c.multiBG[i].bg.Release()
			c.multiBG[i].bg = nil
		}
	}
	c.multiBG = nil
	c.paramsScratch = nil
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
		c.pipeline, c.pipelineBGRA, c.pipeLay, c.bgl, c.shader, c.sampler, c.uniform = nil, nil, nil, nil, nil, nil, nil
	}
	c.device = device
	if c.pipeline != nil && c.pipelineBGRA != nil {
		return nil
	}
	// Partial cache (e.g. pre-BGRA builds): drop and rebuild both pipelines.
	if c.pipeline != nil || c.pipelineBGRA != nil || c.shader != nil {
		if c.pipeline != nil {
			c.pipeline.Release()
			c.pipeline = nil
		}
		if c.pipelineBGRA != nil {
			c.pipelineBGRA.Release()
			c.pipelineBGRA = nil
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
				Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: 48},
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
	pipeBGRA, err := device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "dual_tex_advanced_blend_pipe_bgra",
		Layout: pipeLay,
		Vertex: webgpu.VertexState{
			Module:     shader,
			EntryPoint: "vs_main",
		},
		Fragment: &webgpu.FragmentState{
			Module:     shader,
			EntryPoint: "fs_main",
			Targets: []types.ColorTargetState{{
				Format:    types.TextureFormatBGRA8Unorm,
				Blend:     &replace,
				WriteMask: types.ColorWriteMaskAll,
			}},
		},
		Primitive:   triangleListPrimitive(),
		Multisample: types.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
	})
	if err != nil {
		pipe.Release()
		pipeLay.Release()
		bgl.Release()
		shader.Release()
		return fmt.Errorf("dual-tex blend pipeline BGRA: %w", err)
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
		Size:  48,
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
	c.pipelineBGRA = pipeBGRA
	c.sampler = samp
	c.uniform = uni
	return nil
}

// dualTexAdvancedBlend composites src over dst using GPU dual-texture sampling.
// dstRGBA/srcRGBA are tight premul RGBA8 (bw*bh*4). mode is Multiply/Screen/Overlay/HSL.
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
	case render.BlendMultiply:
		modeU = 1
	case render.BlendScreen:
		modeU = 2
	case render.BlendOverlay:
		modeU = 3
	case render.BlendHue:
		modeU = 4
	case render.BlendSaturation:
		modeU = 5
	case render.BlendColor:
		modeU = 6
	case render.BlendLuminosity:
		modeU = 7
	case render.BlendDarken:
		modeU = 8
	case render.BlendLighten:
		modeU = 9
	case render.BlendColorDodge:
		modeU = 10
	case render.BlendColorBurn:
		modeU = 11
	case render.BlendHardLight:
		modeU = 12
	case render.BlendSoftLight:
		modeU = 13
	case render.BlendDifference:
		modeU = 14
	case render.BlendExclusion:
		modeU = 15
	default:
		modeU = 1
	}
	if err := dualTexWriteParams(queue, cache.uniform, modeU, 0, 0, 1, 1, 1, false); err != nil {
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
			{Binding: 3, Buffer: uniform, Offset: 0, Size: 48},
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

// dualTexModeU maps BlendMode to shader mode code.
func dualTexModeU(mode render.BlendMode) uint32 {
	switch mode {
	case render.BlendMultiply:
		return 1
	case render.BlendScreen:
		return 2
	case render.BlendOverlay:
		return 3
	case render.BlendHue:
		return 4
	case render.BlendSaturation:
		return 5
	case render.BlendColor:
		return 6
	case render.BlendLuminosity:
		return 7
	case render.BlendDarken:
		return 8
	case render.BlendLighten:
		return 9
	case render.BlendColorDodge:
		return 10
	case render.BlendColorBurn:
		return 11
	case render.BlendHardLight:
		return 12
	case render.BlendSoftLight:
		return 13
	case render.BlendDifference:
		return 14
	case render.BlendExclusion:
		return 15
	default:
		return 1
	}
}

// dualTexCreateTex creates an RGBA8 2D texture (+view). optional upload of tight RGBA.
func dualTexCreateTex(device *webgpu.Device, queue *webgpu.Queue, label string, bw, bh int, data []byte, usage types.TextureUsage) (*webgpu.Texture, *webgpu.TextureView, error) {
	return dualTexCreateTexFmt(device, queue, label, bw, bh, data, usage, types.TextureFormatRGBA8Unorm)
}

func dualTexCreateTexFmt(device *webgpu.Device, queue *webgpu.Queue, label string, bw, bh int, data []byte, usage types.TextureUsage, format types.TextureFormat) (*webgpu.Texture, *webgpu.TextureView, error) {
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
		Format:        format,
		Usage:         usage,
	})
	if err != nil {
		return nil, nil, err
	}
	view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
		Label:         label + "_view",
		Format:        format,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		tex.Release()
		return nil, nil, err
	}
	if data != nil && queue != nil {
		tight := uint32(bw * 4) //nolint:gosec
		aligned := alignTextureBytesPerRow(tight)
		upload := data
		var padScratch *[]byte
		if aligned != tight && bh > 1 {
			// R7.1: reuse image staging pool for row-pitch padding (WriteTexture copies immediately).
			need := int(aligned) * bh
			padScratch = acquireImageStaging(need)
			padded := *padScratch
			for y := 0; y < bh; y++ {
				copy(padded[y*int(aligned):y*int(aligned)+bw*4], data[y*bw*4:(y+1)*bw*4])
			}
			upload = padded
		}
		err := queue.WriteTexture(
			&webgpu.ImageCopyTexture{Texture: tex, MipLevel: 0},
			upload,
			&webgpu.ImageDataLayout{
				Offset:       0,
				BytesPerRow:  aligned,
				RowsPerImage: uint32(bh), //nolint:gosec
			},
			&webgpu.Extent3D{Width: uint32(bw), Height: uint32(bh), DepthOrArrayLayers: 1}, //nolint:gosec
		)
		releaseImageStaging(padScratch)
		if err != nil {
			view.Release()
			tex.Release()
			return nil, nil, err
		}
	}
	return tex, view, nil
}

// dualTexAdvancedBlendNoReadback composites src over dst on GPU and returns the
// result texture/view WITHOUT CPU map/Poll. Caller must keep tex alive until
// after the frame Flush (see retainBrushCoverResult).

// dualTexParamsPool reuses the 48-byte CPU uniform staging for dual-tex (R7.1).
// WriteBuffer copies into the queue before return, so recycling after Write is safe.
var dualTexParamsPool = sync.Pool{
	New: func() any {
		b := make([]byte, 48)
		return &b
	},
}

// dualTexWriteParams writes blend mode + UV sample rect into the dual-tex uniform.
// uv_min/uv_max are in 0-1 texture space; full texture uses (0,0)-(1,1).
func dualTexWriteParams(queue *webgpu.Queue, uniform *webgpu.Buffer, modeU uint32, u0, v0, u1, v1, opacity float32, dstTight bool) error {
	if queue == nil || uniform == nil {
		return fmt.Errorf("dual-tex params: nil queue/uniform")
	}
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}
	p := dualTexParamsPool.Get().(*[]byte)
	data := (*p)[:48]
	clear(data)
	binary.LittleEndian.PutUint32(data[0:4], modeU)
	if dstTight {
		binary.LittleEndian.PutUint32(data[4:8], 1)
	}
	binary.LittleEndian.PutUint32(data[8:12], math.Float32bits(u0))
	binary.LittleEndian.PutUint32(data[12:16], math.Float32bits(v0))
	binary.LittleEndian.PutUint32(data[16:20], math.Float32bits(u1))
	binary.LittleEndian.PutUint32(data[20:24], math.Float32bits(v1))
	binary.LittleEndian.PutUint32(data[24:28], math.Float32bits(opacity))
	err := queue.WriteBuffer(uniform, 0, data)
	dualTexParamsPool.Put(p)
	return err
}

func dualTexAdvancedBlendNoReadback(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *dualTexBlendCache,
	dstRGBA, srcRGBA []byte,
	bw, bh int,
	mode render.BlendMode,
) (*webgpu.Texture, *webgpu.TextureView, error) {
	if device == nil || queue == nil || cache == nil {
		return nil, nil, fmt.Errorf("dual-tex: nil device/queue/cache")
	}
	if bw <= 0 || bh <= 0 {
		return nil, nil, fmt.Errorf("dual-tex: empty size")
	}
	need := bw * bh * 4
	if len(dstRGBA) < need || len(srcRGBA) < need {
		return nil, nil, fmt.Errorf("dual-tex: short buffers")
	}
	if err := cache.ensure(device); err != nil {
		return nil, nil, err
	}
	modeU := dualTexModeU(mode)
	if err := dualTexWriteParams(queue, cache.uniform, modeU, 0, 0, 1, 1, 1, false); err != nil {
		return nil, nil, err
	}

	dstTex, dstView, err := dualTexCreateTex(device, queue, "dual_tex_dst", bw, bh, dstRGBA,
		types.TextureUsageTextureBinding|types.TextureUsageCopyDst)
	if err != nil {
		return nil, nil, err
	}
	defer dstView.Release()
	defer dstTex.Release()

	srcTex, srcView, err := dualTexCreateTex(device, queue, "dual_tex_src", bw, bh, srcRGBA,
		types.TextureUsageTextureBinding|types.TextureUsageCopyDst)
	if err != nil {
		return nil, nil, err
	}
	defer srcView.Release()
	defer srcTex.Release()

	outTex, outView, err := dualTexCreateTex(device, queue, "dual_tex_out", bw, bh, nil,
		types.TextureUsageRenderAttachment|types.TextureUsageCopySrc|types.TextureUsageTextureBinding)
	if err != nil {
		return nil, nil, err
	}

	cache.mu.Lock()
	bgl := cache.bgl
	pipeline := cache.pipeline
	sampler := cache.sampler
	uniform := cache.uniform
	cache.mu.Unlock()

	bg, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label:  "dual_tex_blend_bg_nr",
		Layout: bgl,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, TextureView: dstView},
			{Binding: 1, TextureView: srcView},
			{Binding: 2, Sampler: sampler},
			{Binding: 3, Buffer: uniform, Offset: 0, Size: 48},
		},
	})
	if err != nil {
		outView.Release()
		outTex.Release()
		return nil, nil, fmt.Errorf("dual-tex bind: %w", err)
	}
	defer bg.Release()

	enc, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: "dual_tex_nr_enc"})
	if err != nil {
		outView.Release()
		outTex.Release()
		return nil, nil, err
	}
	rp, err := enc.BeginRenderPass(&webgpu.RenderPassDescriptor{
		Label: "dual_tex_nr_pass",
		ColorAttachments: []webgpu.RenderPassColorAttachment{{
			View:       outView,
			LoadOp:     types.LoadOpClear,
			StoreOp:    types.StoreOpStore,
			ClearValue: types.Color{R: 0, G: 0, B: 0, A: 0},
		}},
	})
	if err != nil {
		outView.Release()
		outTex.Release()
		return nil, nil, err
	}
	rp.SetPipeline(pipeline)
	rp.SetBindGroup(0, bg, nil)
	rp.Draw(3, 1, 0, 0)
	rp.End()
	cmd, err := enc.Finish()
	if err != nil {
		outView.Release()
		outTex.Release()
		return nil, nil, err
	}
	defer cmd.Release()
	if _, err := queue.Submit(cmd); err != nil {
		outView.Release()
		outTex.Release()
		return nil, nil, err
	}
	// No Poll/Map — result stays on GPU for QueueGPUTextureDraw.
	return outTex, outView, nil
}

// dualTexAdvancedBlendViewsRegion dual-tex blends bounds of dst/src GPU textures.
// Prefer dualTexAdvancedBlendViewsRegionSized when full dimensions are known.

func dualTexQuantizeWH(w, h int) (int, int) {
	// Bucket sizes to limit pool fragmentation (scattered damage rects).
	const step = 64
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	w = ((w + step - 1) / step) * step
	h = ((h + step - 1) / step) * step
	if w > 4096 {
		w = 4096
	}
	if h > 4096 {
		h = 4096
	}
	return w, h
}

func (c *dualTexBlendCache) getOutBGRA(device *webgpu.Device, queue *webgpu.Queue, w, h int) (*webgpu.Texture, *webgpu.TextureView, error) {
	if c == nil || device == nil || w <= 0 || h <= 0 {
		return nil, nil, fmt.Errorf("dual-tex out pool: bad args")
	}
	// Requested (w,h) is the logical damage size; allocate quantized bucket.
	// Callers that copy/sample only use the top-left w×h of the bucket must
	// pass exact sizes into copy; into-dest uses exact bw/bh for copy size and
	// only samples 0-1 of the tight content via viewport-mapped draw — so keep
	// allocation exact for correctness, but still quantize pool keys by
	// allocating quantized and requiring callers to use exact copy extents.
	qw, qh := dualTexQuantizeWH(w, h)
	key := [2]int{qw, qh}
	c.mu.Lock()
	if c.outPool != nil {
		if b := c.outPool[key]; len(b) > 0 {
			it := b[len(b)-1]
			c.outPool[key] = b[:len(b)-1]
			c.mu.Unlock()
			return it.tex, it.view, nil
		}
	}
	c.mu.Unlock()
	// COPY_DST required for into-dest snap path (CopyTextureToTexture from frameScratch).
	usage := types.TextureUsageRenderAttachment | types.TextureUsageCopySrc | types.TextureUsageCopyDst | types.TextureUsageTextureBinding
	return dualTexCreateTexFmt(device, queue, "dual_tex_snap_bgra", qw, qh, nil, usage, types.TextureFormatBGRA8Unorm)
}

// putOutBGRA returns an out texture to the pool after the frame no longer needs it.
// Currently retainBrushCoverResult owns lifetime until flush; pooling is hooked via
// release path when hold list is drained — callers may put after QueueGPUTextureDraw flush.
func (c *dualTexBlendCache) putOutBGRA(tex *webgpu.Texture, view *webgpu.TextureView, w, h int) {
	if c == nil || tex == nil || view == nil || w <= 0 || h <= 0 {
		if view != nil {
			view.Release()
		}
		if tex != nil {
			tex.Release()
		}
		return
	}
	w, h = dualTexQuantizeWH(w, h)
	key := [2]int{w, h}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.outPool == nil {
		c.outPool = make(map[[2]int][]dualTexPooledTex)
	}
	b := c.outPool[key]
	if len(b) >= 8 {
		view.Release()
		tex.Release()
		return
	}
	c.outPool[key] = append(b, dualTexPooledTex{tex: tex, view: view})
}

// dualTexAgeCool advances cooling temps; items age>=2 return to outPool.
func (c *dualTexBlendCache) dualTexAgeCool() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// Release src views deferred from external-encoder into-dest encodes.
	// Age>=1 means at least one subsequent dual-tex call (or frame) happened.
	if len(c.deferredViews) > 0 {
		// Drop all deferred views that have cooled one cycle: move to release.
		// We keep them one dualTexAgeCool later by swapping through cool via a
		// simple two-phase: first call marks, second releases — use half split.
		n := len(c.deferredViews)
		keepFrom := n / 2
		if n <= 2 {
			// small batch: release all (submit already completed last frame).
			keepFrom = 0
		}
		for i := 0; i < keepFrom; i++ {
			// shift: actually release older half
		}
		// Simpler policy: always release all deferred views here. Callers only
		// defer when externalEnc is used, and dualTexAgeCool runs at the *start*
		// of the *next* into-dest encode (next frame). That is enough lifetime.
		for _, sv := range c.deferredViews {
			if sv != nil {
				sv.Release()
			}
		}
		c.deferredViews = c.deferredViews[:0]
	}
	if len(c.cool) == 0 {
		return
	}
	next := c.cool[:0]
	for _, it := range c.cool {
		it.age++
		if it.age >= 1 {
			if c.outPool == nil {
				c.outPool = make(map[[2]int][]dualTexPooledTex)
			}
			key := [2]int{it.w, it.h}
			b := c.outPool[key]
			if len(b) < 8 {
				c.outPool[key] = append(b, dualTexPooledTex{tex: it.tex, view: it.view})
				continue
			}
			if it.view != nil {
				it.view.Release()
			}
			if it.tex != nil {
				it.tex.Release()
			}
			continue
		}
		next = append(next, it)
	}
	c.cool = next
}

func (c *dualTexBlendCache) coolTex(tex *webgpu.Texture, view *webgpu.TextureView, w, h int) {
	if c == nil || tex == nil || view == nil {
		if view != nil {
			view.Release()
		}
		if tex != nil {
			tex.Release()
		}
		return
	}
	w, h = dualTexQuantizeWH(w, h)
	c.mu.Lock()
	// Hard cap cooling set to avoid VRAM OOM when damage rects thrash.
	const maxCool = 8
	for len(c.cool) >= maxCool {
		old := c.cool[0]
		c.cool = c.cool[1:]
		if old.view != nil {
			old.view.Release()
		}
		if old.tex != nil {
			old.tex.Release()
		}
	}
	c.cool = append(c.cool, dualTexCoolItem{tex: tex, view: view, w: w, h: h, age: 0})
	c.mu.Unlock()
}

// dualTexLayerIntoDestOp describes one deferred advanced layer to blend into dst.
type dualTexLayerIntoDestOp struct {
	srcTex  *webgpu.Texture
	srcView *webgpu.TextureView
	bounds  image.Rectangle
	mode    render.BlendMode
	opacity float32
}

// dualTexBlendLayersIntoDest composites advanced layers into dstTex (frameScratch)
// with a single command buffer: per layer, snap dest region → dual-tex write back
// with LoadOpLoad + scissor. Eliminates per-layer Submit and out+blit.

func (c *dualTexBlendCache) ensureFullSnap(device *webgpu.Device, queue *webgpu.Queue, w, h int) (*webgpu.Texture, *webgpu.TextureView, error) {
	if c == nil || device == nil || w <= 0 || h <= 0 {
		return nil, nil, fmt.Errorf("full snap: bad args")
	}
	c.mu.Lock()
	idx := c.fullSnapI % 2
	c.fullSnapI++
	fs := c.fullSnaps[idx]
	c.mu.Unlock()
	if fs.tex != nil && fs.w == w && fs.h == h && fs.view != nil {
		return fs.tex, fs.view, nil
	}
	if fs.view != nil {
		fs.view.Release()
	}
	if fs.tex != nil {
		fs.tex.Release()
	}
	usage := types.TextureUsageCopyDst | types.TextureUsageTextureBinding | types.TextureUsageCopySrc
	tex, view, err := dualTexCreateTexFmt(device, queue, "dual_tex_full_snap", w, h, nil, usage, types.TextureFormatBGRA8Unorm)
	if err != nil {
		return nil, nil, err
	}
	c.mu.Lock()
	c.fullSnaps[idx] = dualTexFullSnap{tex: tex, view: view, w: w, h: h}
	c.mu.Unlock()
	return tex, view, nil
}

// dualTexBlendLayersIntoDest is DEAD for F1 present path (kept for future
// single-submit work). Verified: it does not modify dest under current
// barriers. Live path is dualTexAdvancedBlendViewsRegionSized → out RT → blit
// in resolvePendingAdvancedLayersEnc. Do not call from Flush until a green
// pixel test proves into-dest writes.
//
// externalEnc, when non-nil, records into that encoder and does NOT submit
// (caller owns Finish/Submit). When nil, creates a private encoder and submits.
func dualTexBlendLayersIntoDest(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *dualTexBlendCache,
	dstTex *webgpu.Texture,
	dstW, dstH int,
	ops []dualTexLayerIntoDestOp,
	externalEnc *webgpu.CommandEncoder,
) error {
	if device == nil || queue == nil || cache == nil || dstTex == nil || len(ops) == 0 {
		return fmt.Errorf("dual-tex into-dest: bad args")
	}
	if err := cache.ensure(device); err != nil {
		return err
	}
	if cache.pipelineBGRA == nil {
		return fmt.Errorf("dual-tex into-dest: no BGRA pipeline")
	}
	cache.dualTexAgeCool()

	// R7.1: cache full dest view (stable RT).
	dstView, err := cache.viewForBGRATexture(device, dstTex, "dual_tex_into_dst")
	if err != nil {
		return err
	}

	// Filter valid ops first.
	type prepared struct {
		op     dualTexLayerIntoDestOp
		bounds image.Rectangle
		bw, bh int
	}
	full := image.Rect(0, 0, dstW, dstH)
	prep := make([]prepared, 0, len(ops))
	for i := range ops {
		op := ops[i]
		if op.srcTex == nil {
			continue
		}
		b := op.bounds.Intersect(full)
		bw, bh := b.Dx(), b.Dy()
		if bw <= 0 || bh <= 0 {
			continue
		}
		prep = append(prep, prepared{op: op, bounds: b, bw: bw, bh: bh})
	}
	if len(prep) == 0 {
		return nil
	}

	// Uniform slots: 256-byte stride (WebGPU min dynamic/fixed offset alignment).
	const uniStride = 256
	uniSize := uint64(uniStride * len(prep))
	uniBuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "dual_tex_into_uni_slots",
		Size:  uniSize,
		Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
	})
	if err != nil {
		return err
	}
	defer uniBuf.Release()

	ownEnc := false
	var enc *webgpu.CommandEncoder
	if externalEnc != nil {
		enc = externalEnc
	} else {
		var err error
		enc, err = device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: "dual_tex_into_dest_enc"})
		if err != nil {
			return err
		}
		ownEnc = true
	}

	cache.mu.Lock()
	bgl := cache.bgl
	pipeline := cache.pipelineBGRA
	sampler := cache.sampler
	cache.mu.Unlock()

	// R7.0/R7.1: src views are cache-owned (viewForBGRATexture); do not Release.
	// Reuse one 256-byte CPU slot for all WriteBuffer uploads in this run.
	slot := make([]byte, uniStride)
	for i := range prep {
		p := &prep[i]
		// Full-surface dest snap (double-buffered). Avoids per-damage VRAM explosion.
		snapTex, snapView, err := cache.ensureFullSnap(device, queue, dstW, dstH)
		if err != nil {
			return err
		}
		// Copy only damage region into the same origin on the snap so UV rects match.
		enc.CopyTextureToTexture(dstTex, snapTex, []webgpu.TextureCopy{{
			Source: webgpu.ImageCopyTexture{
				Texture: dstTex, MipLevel: 0,
				Origin: webgpu.Origin3D{X: uint32(p.bounds.Min.X), Y: uint32(p.bounds.Min.Y), Z: 0}, //nolint:gosec
				Aspect: types.TextureAspectAll,
			},
			Destination: webgpu.ImageCopyTexture{
				Texture: snapTex, MipLevel: 0,
				Origin: webgpu.Origin3D{X: uint32(p.bounds.Min.X), Y: uint32(p.bounds.Min.Y), Z: 0}, //nolint:gosec
				Aspect: types.TextureAspectAll,
			},
			Size: webgpu.Extent3D{Width: uint32(p.bw), Height: uint32(p.bh), DepthOrArrayLayers: 1}, //nolint:gosec
		}})

		srcView, err := cache.viewForBGRATexture(device, p.op.srcTex, "dual_tex_into_src")
		if err != nil {
			return err
		}

		u0 := float32(p.bounds.Min.X) / float32(dstW)
		v0 := float32(p.bounds.Min.Y) / float32(dstH)
		u1 := float32(p.bounds.Max.X) / float32(dstW)
		v1 := float32(p.bounds.Max.Y) / float32(dstH)
		opac := p.op.opacity
		if opac < 0 {
			opac = 0
		}
		if opac > 1 {
			opac = 1
		}
		modeU := dualTexModeU(p.op.mode)
		clear(slot)
		binary.LittleEndian.PutUint32(slot[0:4], modeU)
		binary.LittleEndian.PutUint32(slot[4:8], 0) // full snap: same UV rect for dst+src
		binary.LittleEndian.PutUint32(slot[8:12], math.Float32bits(u0))
		binary.LittleEndian.PutUint32(slot[12:16], math.Float32bits(v0))
		binary.LittleEndian.PutUint32(slot[16:20], math.Float32bits(u1))
		binary.LittleEndian.PutUint32(slot[20:24], math.Float32bits(v1))
		binary.LittleEndian.PutUint32(slot[24:28], math.Float32bits(opac))
		if err := queue.WriteBuffer(uniBuf, uint64(i*uniStride), slot); err != nil { //nolint:gosec
			return err
		}

		bg, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
			Label:  "dual_tex_into_bg",
			Layout: bgl,
			Entries: []webgpu.BindGroupEntry{
				{Binding: 0, TextureView: snapView},
				{Binding: 1, TextureView: srcView},
				{Binding: 2, Sampler: sampler},
				{Binding: 3, Buffer: uniBuf, Offset: uint64(i * uniStride), Size: 48}, //nolint:gosec
			},
		})
		if err != nil {
			return err
		}

		rp, err := enc.BeginRenderPass(&webgpu.RenderPassDescriptor{
			Label: "dual_tex_into_pass",
			ColorAttachments: []webgpu.RenderPassColorAttachment{{
				View:    dstView,
				LoadOp:  types.LoadOpLoad,
				StoreOp: types.StoreOpStore,
			}},
		})
		if err != nil {
			bg.Release()
			return err
		}
		rp.SetPipeline(pipeline)
		rp.SetBindGroup(0, bg, nil)
		rp.SetViewport(float32(p.bounds.Min.X), float32(p.bounds.Min.Y), float32(p.bw), float32(p.bh), 0, 1)
		rp.SetScissorRect(uint32(p.bounds.Min.X), uint32(p.bounds.Min.Y), uint32(p.bw), uint32(p.bh)) //nolint:gosec
		rp.Draw(3, 1, 0, 0)
		_ = rp.End()
		bg.Release()
	}

	if ownEnc {
		cmd, err := enc.Finish()
		if err != nil {
			return err
		}
		defer cmd.Release()
		if _, err := queue.Submit(cmd); err != nil {
			return err
		}
		// R7.1: src/dst views are fullViewCache-owned; nothing to free after submit.
		return nil
	}
	// External encoder (F1 single-submit): views are cache-owned — no deferred Release.
	return nil
}

func dualTexAdvancedBlendViewsRegion(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *dualTexBlendCache,
	dstTex, srcTex *webgpu.Texture,
	bounds image.Rectangle,
	mode render.BlendMode,
) (*webgpu.Texture, *webgpu.TextureView, error) {
	// Infer full size lower-bound from bounds (correct when damage includes bottom-right,
	// or when bounds is the full surface). Interior-only damage should use Sized.
	return dualTexAdvancedBlendTexturesRegionSized(device, queue, cache, dstTex, srcTex, bounds, mode, bounds.Max.X, bounds.Max.Y)
}

// dualTexAdvancedBlendTexturesRegionSized creates temporary views for texture handles.
func dualTexAdvancedBlendTexturesRegionSized(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *dualTexBlendCache,
	dstTex, srcTex *webgpu.Texture,
	bounds image.Rectangle,
	mode render.BlendMode,
	dstW, dstH int,
) (*webgpu.Texture, *webgpu.TextureView, error) {
	if device == nil || cache == nil || dstTex == nil || srcTex == nil {
		return nil, nil, fmt.Errorf("dual-tex textures: nil args")
	}
	// R7.1: full-view cache for stable BGRA layer/swapchain RTs (no per-call Create+Release).
	dstView, err := cache.viewForBGRATexture(device, dstTex, "dual_tex_dst_full")
	if err != nil {
		return nil, nil, err
	}
	srcView, err := cache.viewForBGRATexture(device, srcTex, "dual_tex_src_full")
	if err != nil {
		return nil, nil, err
	}
	return dualTexAdvancedBlendViewsRegionSized(device, queue, cache, dstView, srcView, bounds, mode, dstW, dstH)
}

// viewForBGRATexture returns a cached full texture view for layer/swapchain RTs.
func (c *dualTexBlendCache) viewForBGRATexture(device *webgpu.Device, tex *webgpu.Texture, label string) (*webgpu.TextureView, error) {
	if c == nil || device == nil || tex == nil {
		return nil, fmt.Errorf("dual-tex view cache: nil args")
	}
	key := uintptr(unsafe.Pointer(tex))
	c.mu.Lock()
	if c.fullViewCache != nil {
		if v := c.fullViewCache[key]; v != nil {
			c.mu.Unlock()
			return v, nil
		}
	}
	c.mu.Unlock()
	v, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
		Label: label, Format: types.TextureFormatBGRA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.fullViewCache == nil {
		c.fullViewCache = make(map[uintptr]*webgpu.TextureView)
	}
	if prev := c.fullViewCache[key]; prev != nil {
		c.mu.Unlock()
		v.Release()
		return prev, nil
	}
	c.fullViewCache[key] = v
	c.mu.Unlock()
	return v, nil
}

// dualTexViewBlendOp is one advanced-blend layer for multi-pass single Submit.
type dualTexViewBlendOp struct {
	srcView *webgpu.TextureView
	bounds  image.Rectangle
	mode    render.BlendMode
	opacity float32
}

type dualTexViewBlendOut struct {
	tex     *webgpu.Texture
	view    *webgpu.TextureView
	bounds  image.Rectangle
	opacity float32
}

func (c *dualTexBlendCache) acquireUniformRing(device *webgpu.Device, n int) error {
	if c == nil || device == nil || n <= 0 {
		return fmt.Errorf("dual-tex uniform ring: bad args")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for len(c.uniformRing) < n {
		b, err := device.CreateBuffer(&webgpu.BufferDescriptor{
			Label: fmt.Sprintf("dual_tex_uniform_ring_%d", len(c.uniformRing)),
			Size:  48,
			Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
		})
		if err != nil {
			return err
		}
		c.uniformRing = append(c.uniformRing, b)
	}
	return nil
}

// dualTexMultiBundle is a finished dual-tex multi pass ready for submit (R7.3).
// When Cmd is non-nil, caller must Submit then call Cleanup (bind groups).
// When Cmd is nil, work was already submitted and Cleanup is a no-op.
type dualTexMultiBundle struct {
	Outs    []dualTexViewBlendOut
	Cmd     *webgpu.CommandBuffer
	Cleanup func()
}

// dualTexAdvancedBlendViewsMulti encodes multiple dual-tex advanced blends into
// one command buffer and submits immediately. Prefer dualTexAdvancedBlendViewsMultiBundle
// when the next blit Flush can coalesce Submit (R7.3).
func dualTexAdvancedBlendViewsMulti(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *dualTexBlendCache,
	dstView *webgpu.TextureView,
	ops []dualTexViewBlendOp,
	dstW, dstH int,
) ([]dualTexViewBlendOut, error) {
	b, err := dualTexAdvancedBlendViewsMultiBundle(device, queue, cache, dstView, ops, dstW, dstH, true)
	if err != nil {
		return nil, err
	}
	return b.Outs, nil
}

// multiBindGroup returns a cached bind group for multi-bundle op slot i (opt26).
// Same dst/src/uniform pointers reuse the native BG — avoids per-frame CreateBindGroup.
// Safe to replace a slot after the previous frame's dual-tex CB has been Submitted
// (Cleanup no longer Releases BGs; ownership stays on the cache).
func (c *dualTexBlendCache) multiBindGroup(
	device *webgpu.Device,
	bgl *webgpu.BindGroupLayout,
	sampler *webgpu.Sampler,
	dst, src *webgpu.TextureView,
	ubuf *webgpu.Buffer,
	slot int,
) (*webgpu.BindGroup, error) {
	if device == nil || bgl == nil || sampler == nil || dst == nil || src == nil || ubuf == nil {
		return nil, fmt.Errorf("dual-tex multi bg: nil arg")
	}
	key := dualTexBGKey{
		dst:  uintptr(unsafe.Pointer(dst)),
		src:  uintptr(unsafe.Pointer(src)),
		ubuf: uintptr(unsafe.Pointer(ubuf)),
	}
	c.mu.Lock()
	if slot >= 0 && slot < len(c.multiBG) && c.multiBG[slot].bg != nil && c.multiBG[slot].key == key {
		bg := c.multiBG[slot].bg
		c.mu.Unlock()
		return bg, nil
	}
	c.mu.Unlock()

	bg, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label:  "dual_tex_multi_bg_cached",
		Layout: bgl,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, TextureView: dst},
			{Binding: 1, TextureView: src},
			{Binding: 2, Sampler: sampler},
			{Binding: 3, Buffer: ubuf, Offset: 0, Size: 48},
		},
	})
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	for len(c.multiBG) <= slot {
		c.multiBG = append(c.multiBG, dualTexMultiBGSlot{})
	}
	// Another caller may have filled the slot; keep existing if key matches.
	if c.multiBG[slot].bg != nil && c.multiBG[slot].key == key {
		prev := c.multiBG[slot].bg
		c.mu.Unlock()
		bg.Release()
		return prev, nil
	}
	if c.multiBG[slot].bg != nil {
		// Previous frame already submitted — safe to release replaced BG.
		c.multiBG[slot].bg.Release()
	}
	c.multiBG[slot] = dualTexMultiBGSlot{key: key, bg: bg}
	c.mu.Unlock()
	return bg, nil
}

// dualTexAdvancedBlendViewsMultiBundle encodes multi dual-tex advanced blends.
// submitNow=true: Submit immediately (legacy). submitNow=false: return Cmd for
// coalesced Queue.Submit with a following blit CB (R7.3).
func dualTexAdvancedBlendViewsMultiBundle(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *dualTexBlendCache,
	dstView *webgpu.TextureView,
	ops []dualTexViewBlendOp,
	dstW, dstH int,
	submitNow bool,
) (dualTexMultiBundle, error) {
	if device == nil || queue == nil || cache == nil || dstView == nil || len(ops) == 0 {
		return dualTexMultiBundle{}, fmt.Errorf("dual-tex multi: bad args")
	}
	if err := cache.ensure(device); err != nil {
		return dualTexMultiBundle{}, err
	}
	if cache.pipelineBGRA == nil {
		return dualTexMultiBundle{}, fmt.Errorf("dual-tex multi: no BGRA pipeline")
	}
	if err := cache.acquireUniformRing(device, len(ops)); err != nil {
		return dualTexMultiBundle{}, err
	}

	cache.mu.Lock()
	bgl := cache.bgl
	pipeline := cache.pipelineBGRA
	sampler := cache.sampler
	uniforms := append([]*webgpu.Buffer(nil), cache.uniformRing...)
	cache.mu.Unlock()

	enc, err := device.CreateCommandEncoder(dualTexMultiEncoderDesc)
	if err != nil {
		return dualTexMultiBundle{}, err
	}
	outs := make([]dualTexViewBlendOut, 0, len(ops))
	cleanupOuts := func() {
		for _, o := range outs {
			if o.view != nil {
				o.view.Release()
			}
			if o.tex != nil {
				o.tex.Release()
			}
		}
	}
	fail := func(err error) (dualTexMultiBundle, error) {
		enc.DiscardEncoding()
		cleanupOuts()
		return dualTexMultiBundle{}, err
	}

	full := image.Rect(0, 0, dstW, dstH)
	for i := range ops {
		op := ops[i]
		if op.srcView == nil {
			continue
		}
		bounds := op.bounds.Intersect(full)
		bw, bh := bounds.Dx(), bounds.Dy()
		if bw <= 0 || bh <= 0 {
			continue
		}
		outTex, outView, oerr := cache.getOutBGRA(device, queue, bw, bh)
		if oerr != nil {
			return fail(oerr)
		}
		u0 := float32(bounds.Min.X) / float32(dstW)
		v0 := float32(bounds.Min.Y) / float32(dstH)
		u1 := float32(bounds.Max.X) / float32(dstW)
		v1 := float32(bounds.Max.Y) / float32(dstH)
		modeU := dualTexModeU(op.mode)
		if err := dualTexWriteParams(queue, uniforms[i], modeU, u0, v0, u1, v1, 1, false); err != nil {
			outView.Release()
			outTex.Release()
			return fail(err)
		}
		// opt26: slot-cached BG (dst/src/uniform). Do not Release after Submit —
		// multiBindGroup owns the handle for reuse on warm frames.
		bg, berr := cache.multiBindGroup(device, bgl, sampler, dstView, op.srcView, uniforms[i], len(outs))
		if berr != nil {
			outView.Release()
			outTex.Release()
			return fail(berr)
		}
		rp, rerr := enc.BeginRenderPass(&webgpu.RenderPassDescriptor{
			Label: fmt.Sprintf("dual_tex_multi_pass_%d", i),
			ColorAttachments: []webgpu.RenderPassColorAttachment{{
				View:       outView,
				LoadOp:     types.LoadOpClear,
				StoreOp:    types.StoreOpStore,
				ClearValue: types.Color{R: 0, G: 0, B: 0, A: 0},
			}},
		})
		if rerr != nil {
			outView.Release()
			outTex.Release()
			return fail(rerr)
		}
		rp.SetPipeline(pipeline)
		rp.SetBindGroup(0, bg, nil)
		rp.Draw(3, 1, 0, 0)
		rp.End()
		outs = append(outs, dualTexViewBlendOut{tex: outTex, view: outView, bounds: bounds, opacity: op.opacity})
	}

	if len(outs) == 0 {
		enc.DiscardEncoding()
		return dualTexMultiBundle{}, fmt.Errorf("dual-tex multi: no valid ops")
	}
	cmd, err := enc.Finish()
	if err != nil {
		cleanupOuts()
		return dualTexMultiBundle{}, err
	}

	if submitNow {
		defer cmd.Release()
		if _, err := queue.Submit(cmd); err != nil {
			cleanupOuts()
			return dualTexMultiBundle{}, err
		}
		// BGs owned by cache (opt26); outs released by caller via putOutBGRA/cool.
		return dualTexMultiBundle{Outs: outs, Cleanup: func() {}}, nil
	}

	// Deferred submit: BGs stay on dualTexBlendCache.multiBG for reuse (opt26).
	return dualTexMultiBundle{
		Outs:    outs,
		Cmd:     cmd,
		Cleanup: func() {},
	}, nil
}

// dualTexAdvancedBlendViewsRegionSized is the UV-rect dual-tex path with explicit
// full texture dimensions (required for correct partial-damage UVs).
// dstView/srcView must remain alive until Submit returns.
func dualTexAdvancedBlendViewsRegionSized(
	device *webgpu.Device,
	queue *webgpu.Queue,
	cache *dualTexBlendCache,
	dstView, srcView *webgpu.TextureView,
	bounds image.Rectangle,
	mode render.BlendMode,
	dstW, dstH int,
) (*webgpu.Texture, *webgpu.TextureView, error) {
	if device == nil || queue == nil || cache == nil || dstView == nil || srcView == nil {
		return nil, nil, fmt.Errorf("dual-tex views: nil args")
	}
	bw, bh := bounds.Dx(), bounds.Dy()
	if bw <= 0 || bh <= 0 {
		return nil, nil, fmt.Errorf("dual-tex views: empty bounds")
	}
	if err := cache.ensure(device); err != nil {
		return nil, nil, err
	}
	if cache.pipelineBGRA == nil {
		return nil, nil, fmt.Errorf("dual-tex views: no BGRA pipeline")
	}
	if dstW <= 0 || dstH <= 0 {
		// Infer a lower bound; correct for full-surface damage.
		dstW = bounds.Max.X
		dstH = bounds.Max.Y
		if dstW < bw {
			dstW = bw
		}
		if dstH < bh {
			dstH = bh
		}
	}
	full := image.Rect(0, 0, dstW, dstH)
	bounds = bounds.Intersect(full)
	bw, bh = bounds.Dx(), bounds.Dy()
	if bw <= 0 || bh <= 0 {
		return nil, nil, fmt.Errorf("dual-tex views: empty bounds after intersect")
	}

	outTex, outView, err := cache.getOutBGRA(device, queue, bw, bh)
	if err != nil {
		return nil, nil, err
	}

	u0 := float32(bounds.Min.X) / float32(dstW)
	v0 := float32(bounds.Min.Y) / float32(dstH)
	u1 := float32(bounds.Max.X) / float32(dstW)
	v1 := float32(bounds.Max.Y) / float32(dstH)
	// WGSL fullscreen triangle uses v flipped relative to texture origin in some
	// paths; keep top-left origin matching CopyTextureToTexture previous behavior
	// (bounds in pixel space, UV y down). textureSample uses top-left 0,0.
	modeU := dualTexModeU(mode)
	if err := dualTexWriteParams(queue, cache.uniform, modeU, u0, v0, u1, v1, 1, false); err != nil {
		outView.Release()
		outTex.Release()
		return nil, nil, err
	}

	cache.mu.Lock()
	bgl := cache.bgl
	pipeline := cache.pipelineBGRA
	sampler := cache.sampler
	uniform := cache.uniform
	cache.mu.Unlock()

	bg, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label:  "dual_tex_views_bgra_bg",
		Layout: bgl,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, TextureView: dstView},
			{Binding: 1, TextureView: srcView},
			{Binding: 2, Sampler: sampler},
			{Binding: 3, Buffer: uniform, Offset: 0, Size: 48},
		},
	})
	if err != nil {
		outView.Release()
		outTex.Release()
		return nil, nil, err
	}
	defer bg.Release()

	enc, err := device.CreateCommandEncoder(dualTexViewsEncoderDesc)
	if err != nil {
		outView.Release()
		outTex.Release()
		return nil, nil, err
	}
	rp, err := enc.BeginRenderPass(&webgpu.RenderPassDescriptor{
		Label: "dual_tex_views_bgra_pass",
		ColorAttachments: []webgpu.RenderPassColorAttachment{{
			View:       outView,
			LoadOp:     types.LoadOpClear,
			StoreOp:    types.StoreOpStore,
			ClearValue: types.Color{R: 0, G: 0, B: 0, A: 0},
		}},
	})
	if err != nil {
		outView.Release()
		outTex.Release()
		return nil, nil, err
	}
	rp.SetPipeline(pipeline)
	rp.SetBindGroup(0, bg, nil)
	rp.Draw(3, 1, 0, 0)
	rp.End()
	cmd, err := enc.Finish()
	if err != nil {
		outView.Release()
		outTex.Release()
		return nil, nil, err
	}
	defer cmd.Release()
	if _, err := queue.Submit(cmd); err != nil {
		outView.Release()
		outTex.Release()
		return nil, nil, err
	}
	return outTex, outView, nil
}

// readTextureViewRegionRGBA copies a rectangle from a TextureView into tight
// premul RGBA8. Handles BGRA8Unorm offscreen RTs (swizzle) and RGBA8 sources.
// bounds is in texture pixel space; texW/texH are full texture dimensions.
func readTextureViewRegionRGBA(
	device *webgpu.Device,
	queue *webgpu.Queue,
	view gpucontext.TextureView,
	bounds image.Rectangle,
	texW, texH int,
) ([]byte, error) {
	if device == nil || queue == nil || view.IsNil() {
		return nil, fmt.Errorf("readTextureViewRegionRGBA: nil args")
	}
	full := image.Rect(0, 0, texW, texH)
	bounds = bounds.Intersect(full)
	if bounds.Empty() {
		return nil, fmt.Errorf("readTextureViewRegionRGBA: empty bounds")
	}
	bw, bh := bounds.Dx(), bounds.Dy()
	wgpuView := (*webgpu.TextureView)(view.Pointer())
	if wgpuView == nil {
		return nil, fmt.Errorf("readTextureViewRegionRGBA: nil view ptr")
	}
	tex := wgpuView.Texture()
	if tex == nil {
		return nil, fmt.Errorf("readTextureViewRegionRGBA: nil texture")
	}

	tightRow := uint32(bw * 4) //nolint:gosec
	alignedRow := alignTextureBytesPerRow(tightRow)
	stagingSize := uint64(alignedRow) * uint64(bh)
	staging, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "layer_view_readback",
		Size:  stagingSize,
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("readback staging: %w", err)
	}
	defer staging.Release()

	enc, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: "layer_view_read_enc"})
	if err != nil {
		return nil, err
	}
	enc.CopyTextureToBuffer(tex, staging, []webgpu.BufferTextureCopy{{
		BufferLayout: webgpu.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  alignedRow,
			RowsPerImage: uint32(bh), //nolint:gosec
		},
		TextureBase: webgpu.ImageCopyTexture{
			Texture:  tex,
			MipLevel: 0,
			Origin: webgpu.Origin3D{
				X: uint32(bounds.Min.X), //nolint:gosec
				Y: uint32(bounds.Min.Y), //nolint:gosec
				Z: 0,
			},
			Aspect: types.TextureAspectAll,
		},
		Size: webgpu.Extent3D{
			Width:              uint32(bw), //nolint:gosec
			Height:             uint32(bh), //nolint:gosec
			DepthOrArrayLayers: 1,
		},
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
		return nil, fmt.Errorf("readback map: %w", err)
	}
	mapped, err := staging.MappedRange(0, stagingSize)
	if err != nil {
		_ = staging.Unmap()
		return nil, err
	}
	src := mapped.Bytes()
	out := make([]byte, bw*bh*4)
	// Offscreen cache textures are BGRA8Unorm; convert to RGBA for dual-tex/CPU.
	// If source was already RGBA the swizzle is wrong — CreateOffscreenTexture
	// documents BGRA; dual-tex temps are separate and not read via this helper.
	if alignedRow == tightRow {
		for i := 0; i < bw*bh; i++ {
			b := src[i*4+0]
			g := src[i*4+1]
			r := src[i*4+2]
			a := src[i*4+3]
			out[i*4+0] = r
			out[i*4+1] = g
			out[i*4+2] = b
			out[i*4+3] = a
		}
	} else {
		for y := 0; y < bh; y++ {
			row := src[y*int(alignedRow) : y*int(alignedRow)+bw*4]
			dst := out[y*bw*4 : (y+1)*bw*4]
			for i := 0; i < bw; i++ {
				b := row[i*4+0]
				g := row[i*4+1]
				r := row[i*4+2]
				a := row[i*4+3]
				dst[i*4+0] = r
				dst[i*4+1] = g
				dst[i*4+2] = b
				dst[i*4+3] = a
			}
		}
	}
	mapped.Release()
	_ = staging.Unmap()
	return out, nil
}

func readTextureViewRegionStraightRGBA(
	device *webgpu.Device,
	queue *webgpu.Queue,
	view gpucontext.TextureView,
	bounds image.Rectangle,
	texW, texH int,
) ([]byte, error) {
	if device == nil || queue == nil || view.IsNil() {
		return nil, fmt.Errorf("readTextureViewRegionRGBA: nil args")
	}
	full := image.Rect(0, 0, texW, texH)
	bounds = bounds.Intersect(full)
	if bounds.Empty() {
		return nil, fmt.Errorf("readTextureViewRegionRGBA: empty bounds")
	}
	bw, bh := bounds.Dx(), bounds.Dy()
	wgpuView := (*webgpu.TextureView)(view.Pointer())
	if wgpuView == nil {
		return nil, fmt.Errorf("readTextureViewRegionRGBA: nil view ptr")
	}
	tex := wgpuView.Texture()
	if tex == nil {
		return nil, fmt.Errorf("readTextureViewRegionRGBA: nil texture")
	}

	tightRow := uint32(bw * 4) //nolint:gosec
	alignedRow := alignTextureBytesPerRow(tightRow)
	stagingSize := uint64(alignedRow) * uint64(bh)
	staging, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "filter_rgba_readback",
		Size:  stagingSize,
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("readback staging: %w", err)
	}
	defer staging.Release()

	enc, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: "filter_rgba_read_enc"})
	if err != nil {
		return nil, err
	}
	enc.CopyTextureToBuffer(tex, staging, []webgpu.BufferTextureCopy{{
		BufferLayout: webgpu.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  alignedRow,
			RowsPerImage: uint32(bh), //nolint:gosec
		},
		TextureBase: webgpu.ImageCopyTexture{
			Texture:  tex,
			MipLevel: 0,
			Origin: webgpu.Origin3D{
				X: uint32(bounds.Min.X), //nolint:gosec
				Y: uint32(bounds.Min.Y), //nolint:gosec
				Z: 0,
			},
			Aspect: types.TextureAspectAll,
		},
		Size: webgpu.Extent3D{
			Width:              uint32(bw), //nolint:gosec
			Height:             uint32(bh), //nolint:gosec
			DepthOrArrayLayers: 1,
		},
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
		return nil, fmt.Errorf("readback map: %w", err)
	}
	mapped, err := staging.MappedRange(0, stagingSize)
	if err != nil {
		_ = staging.Unmap()
		return nil, err
	}
	src := mapped.Bytes()
	out := make([]byte, bw*bh*4)
	// RGBA8Unorm source — no channel swizzle.
	if alignedRow == tightRow {
		copy(out, src[:bw*bh*4])
	} else {
		for y := 0; y < bh; y++ {
			copy(out[y*bw*4:(y+1)*bw*4], src[y*int(alignedRow):y*int(alignedRow)+bw*4])
		}
	}
	mapped.Release()
	_ = staging.Unmap()
	return out, nil
}
