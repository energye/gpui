//go:build !nogpu

package gpu

// SDF 覆盖线前置（evidence/SDF/coverage-line.md「驱动拒绝可见性」验收项）：
// 批 4f（evidence/P3.8/batch4f-assessment.md §2.3）两版新 shader（解析行积分 /
// 32×32 子采样）naga 编译通过但 NVIDIA 驱动静默拒绝——画面全空、零错误输出，
// 拒绝点在 CreateShaderModule/CreateRenderPipeline 成功之后的驱动内部。
//
// 本探针把「shader 构造 → module → pipeline → 绘制 → 读回」逐级显形：
// 每个变体用最小 WGSL（全屏三角形 + 圆覆盖）携带一种「复杂度签名」，
// 画到 64×64 离屏目标并逐像素读回。产出每条腿的
// 「module err / pipeline err / 中心像素」——圈出本机驱动可接受的构造包络，
// 为覆盖 LUT 路线（coverage-line.md 候选 1）放行。
//
// 变体签名（全部活代码，防 naga 死码消除掩盖）：
//   control  平滑覆盖（现行 sdf_render.wgsl 的核心子集，探针自证可用）
//   trans    asin/atan2 超越函数（解析行积分版的签名）
//   loop     32×32 嵌套循环（子采样版的签名）
//   array    数组 + 动态索引（分段排序数组的签名）
//   lut      256×1 覆盖 LUT 纹理 + 线性采样（候选 1 的构造本身）

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

const sdfProbeVS = `
@vertex fn vs(@builtin(vertex_index) vi: u32) -> @builtin(position) vec4<f32> {
	var pos = array<vec2<f32>, 3>(vec2<f32>(-1.0, -1.0), vec2<f32>(3.0, -1.0), vec2<f32>(-1.0, 3.0));
	return vec4<f32>(pos[vi], 0.0, 1.0);
}`

// sdfProbeFS returns a fragment shader carrying the named complexity
// signature. All variants share the same circle-coverage core so the center
// pixel is comparable across variants.
func sdfProbeFS(variant string) string {
	core := `
	let p = pos.xy - vec2<f32>(32.0, 32.0);
	let d = length(p);
	let cov = 1.0 - smoothstep(19.25, 20.75, d);
	return vec4<f32>(cov, 0.0, 0.0, 1.0);`
	switch variant {
	case "control":
		return sdfProbeVS + "\n@fragment fn fs(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {" + core + "\n}"
	case "trans":
		return sdfProbeVS + `
@fragment fn fs(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
	let p = pos.xy - vec2<f32>(32.0, 32.0);
	let d = length(p);
	let t = asin(clamp(p.x * 0.03, -1.0, 1.0)) * 1e-6 + atan2(p.y, p.x) * 1e-6;
	let cov = clamp(1.0 - smoothstep(19.25, 20.75, d) - t, 0.0, 1.0);
	return vec4<f32>(cov, 0.0, 0.0, 1.0);
}`
	case "loop":
		return sdfProbeVS + `
@fragment fn fs(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
	let p = pos.xy - vec2<f32>(32.0, 32.0);
	let d = length(p);
	var s: f32 = 0.0;
	for (var i: u32 = 0u; i < 32u; i++) {
		for (var j: u32 = 0u; j < 32u; j++) {
			s = s + f32((i * j) % 7u) * 1e-6;
		}
	}
	let cov = clamp(1.0 - smoothstep(19.25, 20.75, d) - s * 1e-3, 0.0, 1.0);
	return vec4<f32>(cov, 0.0, 0.0, 1.0);
}`
	case "array":
		return sdfProbeVS + `
@fragment fn fs(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
	let p = pos.xy - vec2<f32>(32.0, 32.0);
	let d = length(p);
	var a = array<i32, 8>(8, 7, 6, 5, 4, 3, 2, 1);
	let idx = i32(p.x) & 7;
	let cov = clamp(1.0 - smoothstep(19.25, 20.75, d) - f32(a[idx]) * 1e-6, 0.0, 1.0);
	return vec4<f32>(cov, 0.0, 0.0, 1.0);
}`
	case "lut":
		return sdfProbeVS + `
@group(0) @binding(0) var lutTex: texture_2d<f32>;
@group(0) @binding(1) var lutSamp: sampler;
@fragment fn fs(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
	let p = pos.xy - vec2<f32>(32.0, 32.0);
	let d = length(p);
	let cov = textureSampleLevel(lutTex, lutSamp, vec2<f32>(d / 24.0, 0.5), 0.0).r;
	return vec4<f32>(cov, 0.0, 0.0, 1.0);
}`
	default:
		return ""
	}
}

// sdfProbeDraw creates module+pipeline for the variant, draws one fullscreen
// triangle onto a 64×64 BGRA8 offscreen target, and readbacks via the P0
// calibration machinery. Every stage's error is reported separately so a
// silent driver rejection becomes visible at the exact stage it occurs.
func sdfProbeDraw(t *testing.T, device hal.Device, queue hal.Queue, variant string) (pix []byte, moduleErr, pipeErr error) {
	t.Helper()
	const size = 64

	shader, moduleErr := device.CreateShaderModule(&hal.ShaderModuleDescriptor{
		Label: "sdf_probe_" + variant,
		WGSL:  sdfProbeFS(variant),
	})
	if moduleErr != nil {
		return nil, fmt.Errorf("create shader module: %w", moduleErr), nil
	}
	defer shader.Destroy()

	// Pipeline layout: the lut variant binds a texture+sampler; the others
	// are bind-group-free.
	var bgl hal.BindGroupLayout
	if variant == "lut" {
		l, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
			Label: "sdf_probe_lut_bgl",
			Entries: []types.BindGroupLayoutEntry{
				{
					Binding:    0,
					Visibility: types.ShaderStageFragment,
					Texture:    &types.TextureBindingLayout{SampleType: types.TextureSampleTypeFloat, ViewDimension: types.TextureViewDimension2D},
				},
				{
					Binding:    1,
					Visibility: types.ShaderStageFragment,
					Sampler:    &types.SamplerBindingLayout{Type: types.SamplerBindingTypeFiltering},
				},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("create lut bgl: %w", err), nil
		}
		defer l.Destroy()
		bgl = l
	}
	var pipeLayout hal.PipelineLayout
	{
		layouts := []hal.BindGroupLayout{}
		if bgl != nil {
			layouts = append(layouts, bgl)
		}
		pl, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
			Label:            "sdf_probe_layout_" + variant,
			BindGroupLayouts: layouts,
		})
		if err != nil {
			return nil, fmt.Errorf("create pipeline layout: %w", err), nil
		}
		defer pl.Destroy()
		pipeLayout = pl
	}

	target, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "sdf_probe_target_" + variant,
		Size:          hal.Extent3D{Width: size, Height: size, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatBGRA8Unorm,
		Usage:         types.TextureUsageRenderAttachment | types.TextureUsageCopySrc,
	})
	if err != nil {
		return nil, fmt.Errorf("create target: %w", err), nil
	}
	defer target.Destroy()
	targetView, err := device.CreateTextureView(target, &hal.TextureViewDescriptor{
		Label:  "sdf_probe_target_view_" + variant,
		Format: types.TextureFormatBGRA8Unorm, Dimension: types.TextureViewDimension2D,
		Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("create target view: %w", err), nil
	}
	defer targetView.Destroy()

	pipeline, pipeErr := device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "sdf_probe_pipeline_" + variant,
		Layout: pipeLayout,
		Vertex: hal.VertexState{Module: shader, EntryPoint: "vs"},
		Fragment: &hal.FragmentState{
			Module:     shader,
			EntryPoint: "fs",
			Targets: []types.ColorTargetState{{
				Format:    types.TextureFormatBGRA8Unorm,
				WriteMask: types.ColorWriteMaskAll,
			}},
		},
		Primitive:   triangleListPrimitive(),
		Multisample: multisampleState(1),
	})
	if pipeErr != nil {
		return nil, moduleErr, fmt.Errorf("create render pipeline: %w", pipeErr)
	}
	defer pipeline.Destroy()

	var lutBG hal.BindGroup
	if variant == "lut" {
		// 256×1 coverage ramp: opaque up to 19.25/24, falling to 0 at 20.75/24.
		lutTex, err := device.CreateTexture(&hal.TextureDescriptor{
			Label:         "sdf_probe_lut_tex",
			Size:          hal.Extent3D{Width: 256, Height: 1, DepthOrArrayLayers: 1},
			MipLevelCount: 1,
			SampleCount:   1,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatRGBA8Unorm,
			Usage:         types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
		})
		if err != nil {
			return nil, moduleErr, fmt.Errorf("create lut texture: %w", err)
		}
		defer lutTex.Destroy()
		data := make([]byte, 256*4)
		for i := 0; i < 256; i++ {
			x := float64(i) / 24.0
			var cov float64
			switch {
			case x <= 19.25/24.0:
				cov = 1
			case x >= 20.75/24.0:
				cov = 0
			default:
				cov = (20.75/24.0 - x) / 1.5 / 24.0
			}
			data[i*4] = uint8(cov * 255)
			data[i*4+3] = 255
		}
		if err := queue.WriteTexture(&hal.ImageCopyTexture{Texture: lutTex, MipLevel: 0, Origin: hal.Origin3D{}, Aspect: types.TextureAspectAll},
			data, &hal.ImageDataLayout{Offset: 0, BytesPerRow: 256 * 4, RowsPerImage: 1},
			&hal.Extent3D{Width: 256, Height: 1, DepthOrArrayLayers: 1}); err != nil {
			return nil, moduleErr, fmt.Errorf("upload lut: %w", err)
		}
		lutView, err := device.CreateTextureView(lutTex, &hal.TextureViewDescriptor{
			Label: "sdf_probe_lut_view", Format: types.TextureFormatRGBA8Unorm,
			Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
		})
		if err != nil {
			return nil, moduleErr, fmt.Errorf("create lut view: %w", err)
		}
		defer lutView.Destroy()
		samp, err := device.CreateSampler(&hal.SamplerDescriptor{
			Label: "sdf_probe_lut_sampler", MagFilter: types.FilterModeLinear, MinFilter: types.FilterModeLinear,
		})
		if err != nil {
			return nil, moduleErr, fmt.Errorf("create lut sampler: %w", err)
		}
		defer samp.Destroy()
		bg, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
			Label: "sdf_probe_lut_bg", Layout: bgl,
			Entries: []hal.BindGroupEntry{
				{Binding: 0, TextureView: lutView},
				{Binding: 1, Sampler: samp},
			},
		})
		if err != nil {
			return nil, moduleErr, fmt.Errorf("create lut bind group: %w", err)
		}
		defer bg.Destroy()
		lutBG = bg
	}

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "sdf_probe_encoder"})
	if err != nil {
		return nil, moduleErr, fmt.Errorf("create encoder: %w", err)
	}
	rp, rpErr := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "sdf_probe_pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View: targetView, LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
			ClearValue: types.Color{R: 0, G: 0, B: 0, A: 0},
		}},
	})
	if rpErr != nil {
		return nil, moduleErr, fmt.Errorf("begin render pass: %w", rpErr)
	}
	rp.SetPipeline(pipeline)
	if lutBG != nil {
		rp.SetBindGroup(0, lutBG, nil)
	}
	rp.Draw(3, 1, 0, 0)
	_ = rp.End()
	// VK-LAYOUT-001: explicit transition to CopySrc (same as sdf_render.go).
	encoder.TransitionTextures([]hal.TextureBarrier{{
		Texture: target,
		Usage: hal.TextureUsageTransition{
			OldUsage: types.TextureUsageRenderAttachment,
			NewUsage: types.TextureUsageCopySrc,
		},
	}})
	cmdBuf, err := encoder.Finish()
	if err != nil {
		return nil, moduleErr, fmt.Errorf("finish encoder: %w", err)
	}
	defer device.FreeCommandBuffer(cmdBuf)
	if _, err := queue.Submit(cmdBuf); err != nil {
		return nil, moduleErr, fmt.Errorf("submit: %w", err)
	}

	pm, err := downloadTestPixmap(device, queue, target, testFmtBGRA8, size, size)
	if err != nil {
		return nil, moduleErr, fmt.Errorf("readback: %w", err)
	}
	return pm.Data(), nil, nil
}

// TestSDFShaderDriverProbe runs every complexity-signature variant through
// the staged probe and reports where — if anywhere — the driver rejects.
// The control leg self-certifies the probe (current-shader core must draw).
func TestSDFShaderDriverProbe(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	// Center pixel of a 64×64 BGRA8 target, premul red coverage.
	type leg struct {
		variant string
		wantR   byte // expected center-pixel R after BGRA→RGBA conversion
	}
	legs := []leg{
		{"control", 255},
		{"trans", 255},
		{"loop", 255},
		{"array", 255},
		{"lut", 255},
	}
	rejected := []string{}
	for _, lg := range legs {
		pix, moduleErr, pipeErr := sdfProbeDraw(t, device, queue, lg.variant)
		if moduleErr != nil {
			t.Errorf("%s: module stage FAILED: %v", lg.variant, moduleErr)
			continue
		}
		if pipeErr != nil {
			t.Errorf("%s: pipeline stage FAILED: %v", lg.variant, pipeErr)
			rejected = append(rejected, lg.variant)
			continue
		}
		if len(pix) < 64*64*4 {
			t.Errorf("%s: readback short: %d bytes", lg.variant, len(pix))
			continue
		}
		// downloadTestPixmap already converted BGRA→RGBA: center pixel (32,32).
		i := (32*64 + 32) * 4
		r, g, b, a := pix[i], pix[i+1], pix[i+2], pix[i+3]
		if r != lg.wantR {
			t.Errorf("%s: draw stage — center pixel (%d,%d,%d,%d), want R=%d (silently rejected or wrong output)",
				lg.variant, r, g, b, a, lg.wantR)
			rejected = append(rejected, lg.variant)
			continue
		}
		t.Logf("%s: OK (module+pipeline+draw+readback, center R=%d)", lg.variant, r)
	}
	if len(rejected) > 0 {
		t.Logf("PROBE VERDICT: rejected/blank variants = %v (构造包络收窄，见 coverage-line.md)", rejected)
	} else {
		t.Logf("PROBE VERDICT: all variants accepted — LUT construction (候选 1) 放行")
	}
}
