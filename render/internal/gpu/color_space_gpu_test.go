//go:build !nogpu

package gpu

import (
	"math"
	"testing"
	"unsafe"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// P3.1b-1：GPU WGSL 的 colorSpaceWGSL 与 CPU Go 函数逐值对拍。
// 用 256×1 RGBA8Unorm 目标把 shader 输出原样读回（不做 sRGB 编码），
// x=0..255 覆盖整个 [0,1] 区间；容差 1/255。
func TestP31bColorSpaceGPUMatchesCPU(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	const size = 256
	const rowBytes = size * 4
	vs := `
@vertex fn vs(@builtin(vertex_index) vi: u32) -> @builtin(position) vec4<f32> {
	var pos = array<vec2<f32>, 3>(vec2<f32>(-1.0, -1.0), vec2<f32>(3.0, -1.0), vec2<f32>(-1.0, 3.0));
	return vec4<f32>(pos[vi], 0.0, 1.0);
}`
	fs := `
@fragment fn fs_main(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    let v = (floor(pos.x) + 0.5) / 256.0;
    return vec4<f32>(srgb_to_linear(v), linear_to_srgb(v), 0.0, 1.0);
}`
	shader, err := device.CreateShaderModule(&hal.ShaderModuleDescriptor{Label: "p31b_colorspace", WGSL: vs + colorSpaceWGSL + fs})
	if err != nil {
		t.Fatalf("create shader: %v", err)
	}
	defer shader.Destroy()
	layout, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{Label: "p31b_colorspace_layout"})
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	defer layout.Destroy()
	target, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "p31b_colorspace_target",
		Size:          hal.Extent3D{Width: size, Height: 1, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA8Unorm,
		Usage:  types.TextureUsageRenderAttachment | types.TextureUsageCopySrc,
	})
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	defer target.Destroy()
	view, err := device.CreateTextureView(target, &hal.TextureViewDescriptor{
		Label: "p31b_colorspace_view", Format: types.TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	defer view.Destroy()
	pipe, err := device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label: "p31b_colorspace_pipe", Layout: layout,
		Vertex: hal.VertexState{Module: shader, EntryPoint: "vs"},
		Fragment: &hal.FragmentState{
			Module: shader, EntryPoint: "fs_main",
			Targets: []types.ColorTargetState{{Format: types.TextureFormatRGBA8Unorm, WriteMask: types.ColorWriteMaskAll}},
		},
		Primitive:   triangleListPrimitive(),
		Multisample: multisampleState(1),
	})
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	defer pipe.Destroy()
	enc, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "p31b_colorspace_enc"})
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	rp, err := enc.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "p31b_colorspace_pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View: view, LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
			ClearValue: types.Color{R: 0, G: 0, B: 0, A: 1},
		}},
	})
	if err != nil {
		t.Fatalf("render pass: %v", err)
	}
	rp.SetPipeline(pipe)
	rp.Draw(3, 1, 0, 0)
	if err := rp.End(); err != nil {
		t.Fatalf("end pass: %v", err)
	}
	enc.TransitionTextures([]hal.TextureBarrier{{
		Texture: target,
		Usage:   hal.TextureUsageTransition{OldUsage: types.TextureUsageRenderAttachment, NewUsage: types.TextureUsageCopySrc},
	}})
	cmd, err := enc.Finish()
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	defer device.FreeCommandBuffer(cmd)
	staging, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label: "p31b_colorspace_staging", Size: rowBytes,
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("staging: %v", err)
	}
	defer staging.Destroy()
	ce, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "p31b_colorspace_copy"})
	if err != nil {
		t.Fatalf("copy encoder: %v", err)
	}
	ce.CopyTextureToBuffer(target, staging, []hal.BufferTextureCopy{{
		BufferLayout: hal.ImageDataLayout{Offset: 0, BytesPerRow: rowBytes, RowsPerImage: 1},
		TextureBase:  hal.ImageCopyTexture{Texture: target, MipLevel: 0, Origin: hal.Origin3D{}, Aspect: types.TextureAspectAll},
		Size:         hal.Extent3D{Width: size, Height: 1, DepthOrArrayLayers: 1},
	}})
	copyCmd, err := ce.Finish()
	if err != nil {
		t.Fatalf("finish copy: %v", err)
	}
	defer device.FreeCommandBuffer(copyCmd)
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("submit draw: %v", err)
	}
	if _, err := queue.Submit(copyCmd); err != nil {
		t.Fatalf("submit copy: %v", err)
	}
	mapping, err := device.MapBuffer(staging, 0, rowBytes)
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	raw := make([]byte, rowBytes)
	copy(raw, unsafe.Slice((*byte)(mapping.Ptr), rowBytes))
	if err := device.UnmapBuffer(staging); err != nil {
		t.Fatalf("unmap: %v", err)
	}
	for i := 0; i < size; i++ {
		v := float32((float64(i) + 0.5) / size)
		wantLinear := srgbToLinear(v) * 255
		wantSRGB := linearToSRGB(v) * 255
		gotLinear := float32(raw[i*4])
		gotSRGB := float32(raw[i*4+1])
		if math.Abs(float64(gotLinear-wantLinear)) > 1.5 {
			t.Fatalf("x=%d srgb_to_linear: gpu=%v cpu=%v", i, gotLinear, wantLinear)
		}
		if math.Abs(float64(gotSRGB-wantSRGB)) > 1.5 {
			t.Fatalf("x=%d linear_to_srgb: gpu=%v cpu=%v", i, gotSRGB, wantSRGB)
		}
	}
}
