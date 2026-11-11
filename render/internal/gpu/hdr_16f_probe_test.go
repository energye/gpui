//go:build !nogpu

package gpu

import (
	"testing"
	"unsafe"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// M5 HDR 全链 P0 底座探针（evidence/M5/assessment-design.md §5 P0）：
// 真 GPU 上证明 RGBA16Float 离屏面「可建 + 可渲 + 可混合 + >1 语义完整」。
// 这不是 API，只是把设计里「驱动地基已在」从纸面结论变成可复现红绿：
//  1. 建 32×32 RGBA16Float 面（RENDER_ATTACHMENT|COPY_SRC），清黑后画一笔
//     (3.0, 0.5, 0.25, 1.0)——R>1 必须原样落在面上（8bit 面会钳 1.0）；
//  2. 第二 pass LoadOpLoad + SrcAlpha/OneMinusSrcAlpha 混合一笔
//     (2.0, 0, 0, 0.5)——验证 16F 面可混合且结果 = 2.5/0.25/0.125/0.75；
//  3. 半浮点位型逐通道断言（half 值在 3.0/2.5/0.5/0.25/0.125/0.75 上精确）。
//
// GLES 腿由 gles 半浮点能力位（capabilities.go:800）覆盖，本探针跑 native；
// GLES 真窗验收归 P4。
func TestM5HDR16FRenderTargetProbe(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	const size = 32 // 32*8 = 256 B/row，正好满足 CopyTextureToBuffer 的 256 对齐
	target, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "m5_hdr16f_target",
		Size:          hal.Extent3D{Width: size, Height: size, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatRGBA16Float,
		Usage:         types.TextureUsageRenderAttachment | types.TextureUsageCopySrc,
	})
	if err != nil {
		t.Fatalf("create RGBA16Float render target: %v", err)
	}
	defer target.Destroy()
	view, err := device.CreateTextureView(target, &hal.TextureViewDescriptor{
		Label: "m5_hdr16f_view", Format: types.TextureFormatRGBA16Float,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		t.Fatalf("create view: %v", err)
	}
	defer view.Destroy()

	const vs = `
@vertex fn vs(@builtin(vertex_index) vi: u32) -> @builtin(position) vec4<f32> {
	var pos = array<vec2<f32>, 3>(vec2<f32>(-1.0, -1.0), vec2<f32>(3.0, -1.0), vec2<f32>(-1.0, 3.0));
	return vec4<f32>(pos[vi], 0.0, 1.0);
}`
	shader, err := device.CreateShaderModule(&hal.ShaderModuleDescriptor{
		Label: "m5_hdr16f_shader",
		WGSL: vs + `
@fragment fn fsA() -> @location(0) vec4<f32> { return vec4<f32>(3.0, 0.5, 0.25, 1.0); }
@fragment fn fsB() -> @location(0) vec4<f32> { return vec4<f32>(2.0, 0.0, 0.0, 0.5); }`,
	})
	if err != nil {
		t.Fatalf("create shader module: %v", err)
	}
	defer shader.Destroy()

	layout, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{Label: "m5_hdr16f_layout"})
	if err != nil {
		t.Fatalf("create pipeline layout: %v", err)
	}
	defer layout.Destroy()

	newPipe := func(label, entry string, blend *types.BlendState) hal.RenderPipeline {
		p, err := device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
			Label:  label,
			Layout: layout,
			Vertex: hal.VertexState{Module: shader, EntryPoint: "vs"},
			Fragment: &hal.FragmentState{
				Module: shader, EntryPoint: entry,
				Targets: []types.ColorTargetState{{
					Format:    types.TextureFormatRGBA16Float,
					Blend:     blend,
					WriteMask: types.ColorWriteMaskAll,
				}},
			},
			Primitive:   triangleListPrimitive(),
			Multisample: multisampleState(1),
		})
		if err != nil {
			t.Fatalf("create render pipeline %s (RGBA16Float target): %v", label, err)
		}
		return p
	}
	alphaBlend := types.BlendState{
		Color: types.BlendComponent{SrcFactor: types.BlendFactorSrcAlpha, DstFactor: types.BlendFactorOneMinusSrcAlpha, Operation: types.BlendOperationAdd},
		Alpha: types.BlendComponent{SrcFactor: types.BlendFactorSrcAlpha, DstFactor: types.BlendFactorOneMinusSrcAlpha, Operation: types.BlendOperationAdd},
	}
	pipeA := newPipe("m5_hdr16f_pipe_a", "fsA", nil)
	defer pipeA.Destroy()
	pipeB := newPipe("m5_hdr16f_pipe_b", "fsB", &alphaBlend)
	defer pipeB.Destroy()

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "m5_hdr16f_encoder"})
	if err != nil {
		t.Fatalf("create encoder: %v", err)
	}
	rp, err := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "m5_hdr16f_pass_a",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View: view, LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
			ClearValue: types.Color{R: 0, G: 0, B: 0, A: 0},
		}},
	})
	if err != nil {
		t.Fatalf("begin pass A: %v", err)
	}
	rp.SetPipeline(pipeA)
	rp.Draw(3, 1, 0, 0)
	if err := rp.End(); err != nil {
		t.Fatalf("end pass A: %v", err)
	}
	rp2, err := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "m5_hdr16f_pass_b",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View: view, LoadOp: types.LoadOpLoad, StoreOp: types.StoreOpStore,
		}},
	})
	if err != nil {
		t.Fatalf("begin pass B: %v", err)
	}
	rp2.SetPipeline(pipeB)
	rp2.Draw(3, 1, 0, 0)
	if err := rp2.End(); err != nil {
		t.Fatalf("end pass B: %v", err)
	}
	encoder.TransitionTextures([]hal.TextureBarrier{{
		Texture: target,
		Usage: hal.TextureUsageTransition{
			OldUsage: types.TextureUsageRenderAttachment,
			NewUsage: types.TextureUsageCopySrc,
		},
	}})
	cmd, err := encoder.Finish()
	if err != nil {
		t.Fatalf("finish encoder: %v", err)
	}
	defer device.FreeCommandBuffer(cmd)

	const rowBytes = size * 8
	staging, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label: "m5_hdr16f_staging", Size: rowBytes * size,
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("create staging buffer: %v", err)
	}
	defer staging.Destroy()
	encoder2, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "m5_hdr16f_copy"})
	if err != nil {
		t.Fatalf("create copy encoder: %v", err)
	}
	encoder2.CopyTextureToBuffer(target, staging, []hal.BufferTextureCopy{{
		BufferLayout: hal.ImageDataLayout{Offset: 0, BytesPerRow: rowBytes, RowsPerImage: size},
		TextureBase:  hal.ImageCopyTexture{Texture: target, MipLevel: 0, Origin: hal.Origin3D{}, Aspect: types.TextureAspectAll},
		Size:         hal.Extent3D{Width: size, Height: size, DepthOrArrayLayers: 1},
	}})
	copyCmd, err := encoder2.Finish()
	if err != nil {
		t.Fatalf("finish copy encoder: %v", err)
	}
	defer device.FreeCommandBuffer(copyCmd)
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("submit draw: %v", err)
	}
	if _, err := queue.Submit(copyCmd); err != nil {
		t.Fatalf("submit copy: %v", err)
	}
	mapping, err := device.MapBuffer(staging, 0, rowBytes*size)
	if err != nil {
		t.Fatalf("map staging: %v", err)
	}
	raw := make([]byte, rowBytes*size)
	copy(raw, unsafe.Slice((*byte)(mapping.Ptr), rowBytes*size))
	if err := device.UnmapBuffer(staging); err != nil {
		t.Fatalf("unmap staging: %v", err)
	}

	center := size/2*rowBytes + size/2*8
	halfAt := func(off int) uint16 {
		return uint16(raw[off]) | uint16(raw[off+1])<<8
	}
	got := [4]uint16{halfAt(center), halfAt(center + 2), halfAt(center + 4), halfAt(center + 6)}
	// Half bit patterns: 3.0=0x4200, 2.5=0x4100, 0.75=0x3A00, 0.25=0x3400, 0.125=0x3000.
	want := [4]uint16{0x4100, 0x3400, 0x3000, 0x3A00}
	t.Logf("M5 P0 RGBA16Float center half bits: got=%04x/%04x/%04x/%04x want=%04x/%04x/%04x/%04x",
		got[0], got[1], got[2], got[3], want[0], want[1], want[2], want[3])
	if got != want {
		t.Fatalf("RGBA16Float draw+blend mismatch: got %04x/%04x/%04x/%04x want %04x/%04x/%04x/%04x", got[0], got[1], got[2], got[3], want[0], want[1], want[2], want[3])
	}
}
