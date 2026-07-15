//go:build !(js && wasm)

package webgpu

// S2 A–E native smoke through the webgpu facade (not raw rwgpu).
// Proves: CreateInstance → … → Draw → readback works with descriptor conversion.

import (
	"context"
	"testing"
	"time"

	"github.com/energye/gpui/gpu/types"
)

func s2Device(t *testing.T) (*Instance, *Adapter, *Device, *Queue) {
	t.Helper()
	inst, err := CreateInstance(nil)
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	adapter, err := inst.RequestAdapter(nil)
	if err != nil {
		inst.Release()
		t.Fatalf("RequestAdapter: %v", err)
	}
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		adapter.Release()
		inst.Release()
		t.Fatalf("RequestDevice: %v", err)
	}
	q := device.Queue()
	if q == nil {
		device.Release()
		adapter.Release()
		inst.Release()
		t.Fatal("Queue nil")
	}
	return inst, adapter, device, q
}

func s2Release(inst *Instance, adapter *Adapter, device *Device) {
	if device != nil {
		device.Release()
	}
	if adapter != nil {
		adapter.Release()
	}
	if inst != nil {
		inst.Release()
	}
}

func TestS2AE_BufferWriteCopyMap(t *testing.T) {
	inst, adapter, device, queue := s2Device(t)
	defer s2Release(inst, adapter, device)

	const size = uint64(128)
	want := make([]byte, size)
	for i := range want {
		want[i] = byte(i + 1)
	}
	src, err := device.CreateBuffer(&BufferDescriptor{
		Size:  size,
		Usage: BufferUsageCopyDst | BufferUsageCopySrc,
	})
	if err != nil {
		t.Fatalf("src: %v", err)
	}
	defer src.Release()
	dst, err := device.CreateBuffer(&BufferDescriptor{
		Size:  size,
		Usage: BufferUsageCopyDst | BufferUsageMapRead,
	})
	if err != nil {
		t.Fatalf("dst: %v", err)
	}
	defer dst.Release()

	if err := queue.WriteBuffer(src, 0, want); err != nil {
		t.Fatalf("WriteBuffer: %v", err)
	}
	enc, err := device.CreateCommandEncoder(nil)
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	enc.CopyBufferToBuffer(src, 0, dst, 0, size)
	cmd, err := enc.Finish()
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	cmd.Release()
	device.Poll(PollWait)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := dst.Map(ctx, MapModeRead, 0, size); err != nil {
		t.Fatalf("Map: %v", err)
	}
	mr, err := dst.MappedRange(0, size)
	if err != nil {
		t.Fatalf("MappedRange: %v", err)
	}
	got := mr.Bytes()
	for i := 0; i < 32; i++ {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%#x want %#x", i, got[i], want[i])
		}
	}
	_ = dst.Unmap()
}

func TestS2AE_TextureWriteCopyMap(t *testing.T) {
	inst, adapter, device, queue := s2Device(t)
	defer s2Release(inst, adapter, device)

	const w, h = uint32(4), uint32(4)
	const bpp = 4
	const bytesPerRow = 256
	upload := make([]byte, bytesPerRow*h)
	for y := uint32(0); y < h; y++ {
		for x := uint32(0); x < w; x++ {
			o := y*bytesPerRow + x*bpp
			upload[o+0], upload[o+1], upload[o+2], upload[o+3] = 0xAA, 0xBB, 0xCC, 0xFF
		}
	}

	tex, err := device.CreateTexture(&TextureDescriptor{
		Size:          Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     TextureDimension2D,
		Format:        TextureFormatRGBA8Unorm,
		Usage:         TextureUsageCopyDst | TextureUsageCopySrc | TextureUsageTextureBinding,
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	defer tex.Release()

	view, err := device.CreateTextureView(tex, &TextureViewDescriptor{
		Format:    TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D,
		Aspect:    types.TextureAspectAll,
		// MipLevelCount/ArrayLayerCount 0 → UNDEFINED inside facade
	})
	if err != nil {
		t.Fatalf("CreateTextureView: %v", err)
	}
	defer view.Release()

	sampler, err := device.CreateSampler(&SamplerDescriptor{
		AddressModeU: types.AddressModeClampToEdge,
		AddressModeV: types.AddressModeClampToEdge,
		MagFilter:    types.FilterModeNearest,
		MinFilter:    types.FilterModeNearest,
		MipmapFilter: MipmapFilterModeNearest,
		Anisotropy:   1,
	})
	if err != nil {
		t.Fatalf("CreateSampler: %v", err)
	}
	defer sampler.Release()

	if err := queue.WriteTexture(
		&ImageCopyTexture{Texture: tex, Aspect: types.TextureAspectAll},
		upload,
		&ImageDataLayout{BytesPerRow: bytesPerRow, RowsPerImage: h},
		&Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
	); err != nil {
		t.Fatalf("WriteTexture: %v", err)
	}

	stagingSize := uint64(bytesPerRow * h)
	staging, err := device.CreateBuffer(&BufferDescriptor{
		Size:  stagingSize,
		Usage: BufferUsageCopyDst | BufferUsageMapRead,
	})
	if err != nil {
		t.Fatalf("staging: %v", err)
	}
	defer staging.Release()

	enc, err := device.CreateCommandEncoder(nil)
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	enc.CopyTextureToBuffer(tex, staging, []BufferTextureCopy{{
		BufferLayout: ImageDataLayout{BytesPerRow: bytesPerRow, RowsPerImage: h},
		TextureBase:  ImageCopyTexture{Texture: tex, Aspect: types.TextureAspectAll},
		Size:         Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
	}})
	cmd, err := enc.Finish()
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	cmd.Release()
	device.Poll(PollWait)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := staging.Map(ctx, MapModeRead, 0, stagingSize); err != nil {
		t.Fatalf("Map: %v", err)
	}
	mr, err := staging.MappedRange(0, stagingSize)
	if err != nil {
		t.Fatalf("MappedRange: %v", err)
	}
	got := mr.Bytes()
	if got[0] != 0xAA || got[1] != 0xBB || got[2] != 0xCC || got[3] != 0xFF {
		t.Fatalf("texel = %02x %02x %02x %02x", got[0], got[1], got[2], got[3])
	}
	_ = staging.Unmap()
}

func TestS2AE_DrawReadback(t *testing.T) {
	inst, adapter, device, queue := s2Device(t)
	defer s2Release(inst, adapter, device)

	const w, h = uint32(8), uint32(8)
	const bpp = 4
	const bytesPerRow = 256

	shader, err := device.CreateShaderModule(&ShaderModuleDescriptor{
		WGSL: `
@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    var pos = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>( 3.0, -1.0),
        vec2<f32>(-1.0,  3.0)
    );
    return vec4<f32>(pos[idx], 0.0, 1.0);
}
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 1.0, 1.0); // blue
}
`,
	})
	if err != nil {
		t.Fatalf("shader: %v", err)
	}
	defer shader.Release()

	pipeline, err := device.CreateRenderPipeline(&RenderPipelineDescriptor{
		Vertex: VertexState{Module: shader, EntryPoint: "vs_main"},
		Fragment: &FragmentState{
			Module:     shader,
			EntryPoint: "fs_main",
			Targets: []types.ColorTargetState{{
				Format:    TextureFormatRGBA8Unorm,
				WriteMask: types.ColorWriteMaskAll,
				Blend: &types.BlendState{
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
				},
			}},
		},
		Primitive: types.PrimitiveState{
			Topology:  types.PrimitiveTopologyTriangleList,
			FrontFace: types.FrontFaceCCW,
			CullMode:  types.CullModeNone,
		},
		Multisample: types.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
	})
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	defer pipeline.Release()

	rt, err := device.CreateTexture(&TextureDescriptor{
		Size:          Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     TextureDimension2D,
		Format:        TextureFormatRGBA8Unorm,
		Usage:         TextureUsageRenderAttachment | TextureUsageCopySrc,
	})
	if err != nil {
		t.Fatalf("rt: %v", err)
	}
	defer rt.Release()
	view, err := device.CreateTextureView(rt, &TextureViewDescriptor{
		Format:          TextureFormatRGBA8Unorm,
		Dimension:       types.TextureViewDimension2D,
		Aspect:          types.TextureAspectAll,
		MipLevelCount:   1,
		ArrayLayerCount: 1,
	})
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	defer view.Release()

	enc, err := device.CreateCommandEncoder(nil)
	if err != nil {
		t.Fatalf("enc: %v", err)
	}
	pass, err := enc.BeginRenderPass(&RenderPassDescriptor{
		ColorAttachments: []RenderPassColorAttachment{{
			View:       view,
			LoadOp:     types.LoadOpClear,
			StoreOp:    types.StoreOpStore,
			ClearValue: types.Color{R: 1, G: 0, B: 0, A: 1}, // red clear
		}},
	})
	if err != nil {
		t.Fatalf("BeginRenderPass: %v", err)
	}
	pass.SetPipeline(pipeline)
	pass.SetViewport(0, 0, float32(w), float32(h), 0, 1)
	pass.SetScissorRect(0, 0, w, h)
	pass.SetBlendConstant(&types.Color{})
	pass.SetStencilReference(0)
	pass.Draw(3, 1, 0, 0)
	if err := pass.End(); err != nil {
		t.Fatalf("End: %v", err)
	}

	stagingSize := uint64(bytesPerRow * h)
	staging, err := device.CreateBuffer(&BufferDescriptor{
		Size:  stagingSize,
		Usage: BufferUsageCopyDst | BufferUsageMapRead,
	})
	if err != nil {
		t.Fatalf("staging: %v", err)
	}
	defer staging.Release()
	enc.CopyTextureToBuffer(rt, staging, []BufferTextureCopy{{
		BufferLayout: ImageDataLayout{BytesPerRow: bytesPerRow, RowsPerImage: h},
		TextureBase:  ImageCopyTexture{Texture: rt, Aspect: types.TextureAspectAll},
		Size:         Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
	}})
	cmd, err := enc.Finish()
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	cmd.Release()
	device.Poll(PollWait)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := staging.Map(ctx, MapModeRead, 0, stagingSize); err != nil {
		t.Fatalf("Map: %v", err)
	}
	mr, err := staging.MappedRange(0, stagingSize)
	if err != nil {
		t.Fatalf("MappedRange: %v", err)
	}
	got := mr.Bytes()
	o := (h/2)*bytesPerRow + (w/2)*bpp
	r, g, b := got[o], got[o+1], got[o+2]
	// Expect blue draw over red clear
	if b < 200 || r > 30 || g > 30 {
		t.Fatalf("center rgba=%d,%d,%d want blue (facade pipeline/draw broken?)", r, g, b)
	}
	_ = staging.Unmap()
}
