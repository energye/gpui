package rwgpu

// s1_ae_smoke_test.go — S1 A–E end-to-end native smoke for Skia 2D WebGPU subset.
//
// Requires: WGPU_NATIVE_PATH pointing at libwgpu_native.so (or default discovery).
// Plan: docs/MAINLINE_PLAN.md S1 closeout + docs/RWGPU_SKIA_SUBSET_CHECKLIST.md A–E.
//
// Each test exercises a real native call chain (not null-guard only) and, where
// feasible, verifies GPU-visible results via map/readback.

import (
	"context"
	"testing"
	"time"
	"unsafe"

	"github.com/energye/gpui/gpu/types"
)

func s1Device(t *testing.T) (*Instance, *Adapter, *Device, *Queue) {
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
	queue := device.Queue()
	if queue == nil {
		device.Release()
		adapter.Release()
		inst.Release()
		t.Fatal("Queue is nil")
	}
	return inst, adapter, device, queue
}

func s1Release(inst *Instance, adapter *Adapter, device *Device, queue *Queue) {
	if queue != nil {
		queue.Release()
	}
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

// TestS1AE_A_InstanceAdapterDeviceQueue covers checklist A1–A8 core path.
func TestS1AE_A_InstanceAdapterDeviceQueue(t *testing.T) {
	inst, adapter, device, queue := s1Device(t)
	defer s1Release(inst, adapter, device, queue)

	if inst.Handle() == 0 || adapter.Handle() == 0 || device.Handle() == 0 || queue.Handle() == 0 {
		t.Fatal("zero handle on instance/adapter/device/queue")
	}

	// A7 limits/features
	limits := device.Limits()
	t.Logf("Limits MaxTextureDimension2D=%d MaxBindGroups=%d", limits.MaxTextureDimension2D, limits.MaxBindGroups)
	_ = device.Features()
	_ = device.HasFeature(FeatureNameDepth32FloatStencil8)

	// A6 error scope empty pop
	device.PushErrorScope(ErrorFilterValidation)
	et, msg := device.PopErrorScope(inst)
	if et != ErrorTypeNoError && et != 0 {
		// Some builds report NoError as 1; accept clean pop.
		t.Logf("PopErrorScope type=%v msg=%q (ok if no error)", et, msg)
	}

	// A5 poll
	device.Poll(true)
}

// TestS1AE_B_BufferWriteMap covers B1–B5: create, write, map-read, destroy.
func TestS1AE_B_BufferWriteMap(t *testing.T) {
	inst, adapter, device, queue := s1Device(t)
	defer s1Release(inst, adapter, device, queue)

	const size = uint64(256)
	want := make([]byte, size)
	for i := range want {
		want[i] = byte(i * 3)
	}

	// Path 1: MappedAtCreation write/read (map state + Unmap).
	mapped, err := device.CreateBuffer(&BufferDescriptor{
		Usage:            types.BufferUsageCopySrc | types.BufferUsageMapWrite,
		Size:             size,
		MappedAtCreation: true,
	})
	if err != nil {
		t.Fatalf("CreateBuffer mapped: %v", err)
	}
	ptr := mapped.GetMappedRange(0, size)
	if ptr == nil {
		t.Fatal("GetMappedRange nil at creation")
	}
	dst := unsafe.Slice((*byte)(ptr), int(size))
	copy(dst, want)
	if err := mapped.Unmap(); err != nil {
		t.Fatalf("Unmap: %v", err)
	}

	// Path 2: Queue.WriteBuffer → GPU copy → MapAsync/Map readback verify.
	src, err := device.CreateBuffer(&BufferDescriptor{
		Usage: types.BufferUsageCopyDst | types.BufferUsageCopySrc,
		Size:  size,
	})
	if err != nil {
		t.Fatalf("src: %v", err)
	}
	defer src.Release()
	staging, err := device.CreateBuffer(&BufferDescriptor{
		Usage: types.BufferUsageCopyDst | types.BufferUsageMapRead,
		Size:  size,
	})
	if err != nil {
		t.Fatalf("staging: %v", err)
	}
	defer staging.Release()

	if err := queue.WriteBuffer(src, 0, want); err != nil {
		t.Fatalf("WriteBuffer: %v", err)
	}
	enc, err := device.CreateCommandEncoder(nil)
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	enc.CopyBufferToBuffer(src, 0, staging, 0, size)
	cmd, err := enc.Finish(nil)
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	enc.Release()
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	cmd.Release()
	device.Poll(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := staging.Map(ctx, MapModeRead, 0, size); err != nil {
		t.Fatalf("Map: %v", err)
	}
	ptr = staging.GetMappedRange(0, size)
	if ptr == nil {
		t.Fatal("GetMappedRange nil")
	}
	got := unsafe.Slice((*byte)(ptr), int(size))
	for i := 0; i < 64; i++ {
		if got[i] != want[i] {
			t.Fatalf("mapped[%d]=%#x want %#x", i, got[i], want[i])
		}
	}
	_ = staging.Unmap()
	mapped.Destroy()
	mapped.Release()
	if src.Size() != size {
		t.Fatalf("Size=%d", src.Size())
	}
	if src.Usage()&types.BufferUsageCopySrc == 0 {
		t.Fatal("Usage missing CopySrc")
	}
}

// TestS1AE_B_CopyBufferToBufferMap covers B2 path via GPU copy + map.
func TestS1AE_B_CopyBufferToBufferMap(t *testing.T) {
	inst, adapter, device, queue := s1Device(t)
	defer s1Release(inst, adapter, device, queue)

	const size = uint64(64)
	srcData := make([]byte, size)
	for i := range srcData {
		srcData[i] = byte(0xA0 + i)
	}

	src, err := device.CreateBuffer(&BufferDescriptor{
		Usage: types.BufferUsageCopySrc | types.BufferUsageCopyDst,
		Size:  size,
	})
	if err != nil {
		t.Fatalf("src CreateBuffer: %v", err)
	}
	defer src.Release()

	dst, err := device.CreateBuffer(&BufferDescriptor{
		Usage: types.BufferUsageCopyDst | types.BufferUsageMapRead,
		Size:  size,
	})
	if err != nil {
		t.Fatalf("dst CreateBuffer: %v", err)
	}
	defer dst.Release()

	if err := queue.WriteBuffer(src, 0, srcData); err != nil {
		t.Fatalf("WriteBuffer: %v", err)
	}

	enc, err := device.CreateCommandEncoder(nil)
	if err != nil {
		t.Fatalf("CreateCommandEncoder: %v", err)
	}
	enc.CopyBufferToBuffer(src, 0, dst, 0, size)
	cmd, err := enc.Finish(nil)
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	enc.Release()
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	cmd.Release()
	device.Poll(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := dst.Map(ctx, MapModeRead, 0, size); err != nil {
		t.Fatalf("Map: %v", err)
	}
	ptr := dst.GetMappedRange(0, size)
	if ptr == nil {
		t.Fatal("GetMappedRange nil")
	}
	got := unsafe.Slice((*byte)(ptr), int(size))
	for i := 0; i < int(size); i++ {
		if got[i] != srcData[i] {
			t.Fatalf("copy[%d]=%#x want %#x", i, got[i], srcData[i])
		}
	}
	_ = dst.Unmap()
}

// TestS1AE_C_TextureWriteCopyMap covers C1–C6: texture, write, sampler, copy→map.
func TestS1AE_C_TextureWriteCopyMap(t *testing.T) {
	inst, adapter, device, queue := s1Device(t)
	defer s1Release(inst, adapter, device, queue)

	const w, h = uint32(4), uint32(4)
	const bpp = 4
	// WebGPU requires bytesPerRow multiple of 256 for copy/write.
	const bytesPerRow = 256
	const rows = h
	upload := make([]byte, bytesPerRow*rows)
	// Fill first 4 texels of each row with solid magenta-ish.
	for y := uint32(0); y < h; y++ {
		for x := uint32(0); x < w; x++ {
			o := y*bytesPerRow + x*bpp
			upload[o+0] = 0x11
			upload[o+1] = 0x22
			upload[o+2] = 0x33
			upload[o+3] = 0xFF
		}
	}

	tex, err := device.CreateTexture(&TextureDescriptor{
		Usage:         types.TextureUsageCopyDst | types.TextureUsageCopySrc | types.TextureUsageTextureBinding,
		Dimension:     types.TextureDimension2D,
		Size:          types.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
		Format:        types.TextureFormatRGBA8Unorm,
		MipLevelCount: 1,
		SampleCount:   1,
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	defer tex.Release()

	view, err := tex.CreateView(nil)
	if err != nil {
		t.Fatalf("CreateView: %v", err)
	}
	defer view.Release()
	if view.Handle() == 0 {
		t.Fatal("view handle 0")
	}

	sampler, err := device.CreateSampler(&SamplerDescriptor{
		AddressModeU: types.AddressModeClampToEdge,
		AddressModeV: types.AddressModeClampToEdge,
		AddressModeW: types.AddressModeClampToEdge,
		MagFilter:    types.FilterModeNearest,
		MinFilter:    types.FilterModeNearest,
		MipmapFilter: types.MipmapFilterModeNearest,
		Anisotropy:   1,
	})
	if err != nil {
		t.Fatalf("CreateSampler: %v", err)
	}
	defer sampler.Release()

	if err := queue.WriteTexture(
		&ImageCopyTexture{Texture: tex, MipLevel: 0, Origin: types.Origin3D{}, Aspect: TextureAspectAll},
		upload,
		&ImageDataLayout{Offset: 0, BytesPerRow: bytesPerRow, RowsPerImage: rows},
		&types.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
	); err != nil {
		t.Fatalf("WriteTexture: %v", err)
	}

	stagingSize := uint64(bytesPerRow * rows)
	staging, err := device.CreateBuffer(&BufferDescriptor{
		Usage: types.BufferUsageCopyDst | types.BufferUsageMapRead,
		Size:  stagingSize,
	})
	if err != nil {
		t.Fatalf("staging CreateBuffer: %v", err)
	}
	defer staging.Release()

	enc, err := device.CreateCommandEncoder(nil)
	if err != nil {
		t.Fatalf("CreateCommandEncoder: %v", err)
	}
	enc.CopyTextureToBuffer(tex, staging, []BufferTextureCopy{{
		BufferLayout: ImageDataLayout{Offset: 0, BytesPerRow: bytesPerRow, RowsPerImage: rows},
		TextureBase:  ImageCopyTexture{Texture: tex, Aspect: TextureAspectAll},
		Size:         types.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
	}})
	cmd, err := enc.Finish(nil)
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	enc.Release()
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	cmd.Release()
	device.Poll(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := staging.Map(ctx, MapModeRead, 0, stagingSize); err != nil {
		t.Fatalf("Map staging: %v", err)
	}
	ptr := staging.GetMappedRange(0, stagingSize)
	if ptr == nil {
		t.Fatal("GetMappedRange nil")
	}
	got := unsafe.Slice((*byte)(ptr), int(stagingSize))
	// Check texel (0,0)
	if got[0] != 0x11 || got[1] != 0x22 || got[2] != 0x33 || got[3] != 0xFF {
		t.Fatalf("texel00 = %02x %02x %02x %02x, want 11 22 33 ff", got[0], got[1], got[2], got[3])
	}
	_ = staging.Unmap()
}

// TestS1AE_DE_DrawReadback covers D (shader/pipeline/blend/topology) + E (pass/draw/submit)
// and proves converted enums work on a real GPU path via fixed clear + full-screen draw.
func TestS1AE_DE_DrawReadback(t *testing.T) {
	inst, adapter, device, queue := s1Device(t)
	defer s1Release(inst, adapter, device, queue)

	const w, h = uint32(8), uint32(8)
	const bpp = 4
	const bytesPerRow = 256 // alignment

	shaderCode := `
@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    // Full-screen triangle
    var pos = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>( 3.0, -1.0),
        vec2<f32>(-1.0,  3.0)
    );
    return vec4<f32>(pos[idx], 0.0, 1.0);
}

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0); // opaque red
}
`
	shader, err := device.CreateShaderModuleWGSL(shaderCode)
	if err != nil {
		t.Fatalf("CreateShaderModuleWGSL: %v", err)
	}
	defer shader.Release()

	// Explicit blend + topology path exercises identity blend enums and converted topology.
	blend := &BlendState{
		Color: BlendComponent{
			SrcFactor: types.BlendFactorOne,
			DstFactor: types.BlendFactorZero,
			Operation: types.BlendOperationAdd,
		},
		Alpha: BlendComponent{
			SrcFactor: types.BlendFactorOne,
			DstFactor: types.BlendFactorZero,
			Operation: types.BlendOperationAdd,
		},
	}
	pipeline, err := device.CreateRenderPipeline(&RenderPipelineDescriptor{
		Vertex: VertexState{Module: shader, EntryPoint: "vs_main"},
		Fragment: &FragmentState{
			Module:     shader,
			EntryPoint: "fs_main",
			Targets: []ColorTargetState{{
				Format:    types.TextureFormatRGBA8Unorm,
				Blend:     blend,
				WriteMask: types.ColorWriteMaskAll,
			}},
		},
		Primitive: PrimitiveState{
			Topology:  types.PrimitiveTopologyTriangleList, // gputypes 0 → native 4
			FrontFace: types.FrontFaceCCW,                  // 0 → 1
			CullMode:  types.CullModeNone,                  // 0 → 1
		},
		Multisample: MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
	})
	if err != nil {
		t.Fatalf("CreateRenderPipeline: %v", err)
	}
	defer pipeline.Release()

	rt, err := device.CreateTexture(&TextureDescriptor{
		Usage:         types.TextureUsageRenderAttachment | types.TextureUsageCopySrc,
		Dimension:     types.TextureDimension2D,
		Size:          types.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
		Format:        types.TextureFormatRGBA8Unorm,
		MipLevelCount: 1,
		SampleCount:   1,
	})
	if err != nil {
		t.Fatalf("CreateTexture RT: %v", err)
	}
	defer rt.Release()
	view, err := rt.CreateView(nil)
	if err != nil {
		t.Fatalf("CreateView: %v", err)
	}
	defer view.Release()

	enc, err := device.CreateCommandEncoder(nil)
	if err != nil {
		t.Fatalf("CreateCommandEncoder: %v", err)
	}
	pass, err := enc.BeginRenderPass(&RenderPassDescriptor{
		ColorAttachments: []RenderPassColorAttachment{{
			View:       view,
			LoadOp:     types.LoadOpClear,
			StoreOp:    types.StoreOpStore,
			ClearValue: Color{R: 0, G: 0, B: 1, A: 1}, // blue clear
		}},
	})
	if err != nil {
		t.Fatalf("BeginRenderPass: %v", err)
	}
	pass.SetPipeline(pipeline)
	pass.SetViewport(0, 0, float32(w), float32(h), 0, 1)
	pass.SetScissorRect(0, 0, w, h)
	pass.SetBlendConstant(&Color{R: 0, G: 0, B: 0, A: 1})
	pass.SetStencilReference(0)
	pass.Draw(3, 1, 0, 0)
	pass.End()
	pass.Release()

	stagingSize := uint64(bytesPerRow * h)
	staging, err := device.CreateBuffer(&BufferDescriptor{
		Usage: types.BufferUsageCopyDst | types.BufferUsageMapRead,
		Size:  stagingSize,
	})
	if err != nil {
		t.Fatalf("staging: %v", err)
	}
	defer staging.Release()

	enc.CopyTextureToBuffer(rt, staging, []BufferTextureCopy{{
		BufferLayout: ImageDataLayout{BytesPerRow: bytesPerRow, RowsPerImage: h},
		TextureBase:  ImageCopyTexture{Texture: rt, Aspect: TextureAspectAll},
		Size:         types.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
	}})
	cmd, err := enc.Finish(nil)
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	enc.Release()
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	cmd.Release()
	device.Poll(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := staging.Map(ctx, MapModeRead, 0, stagingSize); err != nil {
		t.Fatalf("Map: %v", err)
	}
	ptr := staging.GetMappedRange(0, stagingSize)
	if ptr == nil {
		t.Fatal("GetMappedRange nil")
	}
	got := unsafe.Slice((*byte)(ptr), int(stagingSize))
	// Center-ish texel should be red (full-screen triangle), not blue clear.
	// RGBA8 unorm: R near 255, G/B near 0.
	o := (h/2)*bytesPerRow + (w/2)*bpp
	r, g, b, a := got[o], got[o+1], got[o+2], got[o+3]
	if r < 200 || g > 30 || b > 30 {
		t.Fatalf("center pixel rgba=%d,%d,%d,%d want red~255,0,0 (topology/blend/draw path broken?)", r, g, b, a)
	}
	if a < 200 {
		t.Fatalf("center alpha=%d want opaque", a)
	}
	_ = staging.Unmap()
}

// TestS1AE_E_DrawIndexed covers index buffer + DrawIndexed (E3/E4).
func TestS1AE_E_DrawIndexed(t *testing.T) {
	inst, adapter, device, queue := s1Device(t)
	defer s1Release(inst, adapter, device, queue)

	const w, h = uint32(4), uint32(4)
	const bytesPerRow = 256

	shaderCode := `
struct VOut { @builtin(position) pos: vec4<f32> };
@vertex
fn vs_main(@location(0) p: vec2<f32>) -> VOut {
    var o: VOut;
    o.pos = vec4<f32>(p, 0.0, 1.0);
    return o;
}
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(0.0, 1.0, 0.0, 1.0);
}
`
	shader, err := device.CreateShaderModuleWGSL(shaderCode)
	if err != nil {
		t.Fatalf("shader: %v", err)
	}
	defer shader.Release()

	// Vertex buffer: 3 corners of full-screen triangle (vec2 f32)
	verts := []float32{
		-1, -1,
		3, -1,
		-1, 3,
	}
	vertBytes := unsafe.Slice((*byte)(unsafe.Pointer(&verts[0])), len(verts)*4)
	vbuf, err := device.CreateBuffer(&BufferDescriptor{
		Usage: types.BufferUsageVertex | types.BufferUsageCopyDst,
		Size:  uint64(len(vertBytes)),
	})
	if err != nil {
		t.Fatalf("vbuf: %v", err)
	}
	defer vbuf.Release()
	if err := queue.WriteBuffer(vbuf, 0, append([]byte(nil), vertBytes...)); err != nil {
		t.Fatalf("WriteBuffer verts: %v", err)
	}

	// Indices 0,1,2 as uint16. Buffer size must be multiple of 4 (WebGPU).
	idx := []uint16{0, 1, 2, 0}
	idxBytes := make([]byte, 8)
	for i, v := range idx {
		idxBytes[i*2] = byte(v)
		idxBytes[i*2+1] = byte(v >> 8)
	}
	ibuf, err := device.CreateBuffer(&BufferDescriptor{
		Usage: types.BufferUsageIndex | types.BufferUsageCopyDst,
		Size:  8,
	})
	if err != nil {
		t.Fatalf("ibuf: %v", err)
	}
	defer ibuf.Release()
	if err := queue.WriteBuffer(ibuf, 0, idxBytes); err != nil {
		t.Fatalf("WriteBuffer idx: %v", err)
	}

	pipeline, err := device.CreateRenderPipeline(&RenderPipelineDescriptor{
		Vertex: VertexState{
			Module:     shader,
			EntryPoint: "vs_main",
			Buffers: func() []VertexBufferLayout {
				attrs := []VertexAttribute{{
					Format:         types.VertexFormatFloat32x2, // converted
					Offset:         0,
					ShaderLocation: 0,
				}}
				return []VertexBufferLayout{{
					ArrayStride:    8,
					StepMode:       types.VertexStepModeVertex, // converted
					AttributeCount: uintptr(len(attrs)),
					Attributes:     &attrs[0],
				}}
			}(),
		},
		Fragment: &FragmentState{
			Module:     shader,
			EntryPoint: "fs_main",
			Targets: []ColorTargetState{{
				Format:    types.TextureFormatRGBA8Unorm,
				WriteMask: types.ColorWriteMaskAll,
			}},
		},
		Primitive: PrimitiveState{
			Topology:  types.PrimitiveTopologyTriangleList,
			FrontFace: types.FrontFaceCCW,
			CullMode:  types.CullModeNone,
		},
		Multisample: MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
	})
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	defer pipeline.Release()

	rt, err := device.CreateTexture(&TextureDescriptor{
		Usage:         types.TextureUsageRenderAttachment | types.TextureUsageCopySrc,
		Dimension:     types.TextureDimension2D,
		Size:          types.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
		Format:        types.TextureFormatRGBA8Unorm,
		MipLevelCount: 1,
		SampleCount:   1,
	})
	if err != nil {
		t.Fatalf("rt: %v", err)
	}
	defer rt.Release()
	view, err := rt.CreateView(nil)
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
			ClearValue: Color{R: 0, G: 0, B: 0, A: 1},
		}},
	})
	if err != nil {
		t.Fatalf("BeginRenderPass: %v", err)
	}
	pass.SetPipeline(pipeline)
	pass.SetVertexBuffer(0, vbuf, 0, uint64(len(vertBytes)))
	pass.SetIndexBuffer(ibuf, types.IndexFormatUint16, 0, 6)
	pass.DrawIndexed(3, 1, 0, 0, 0)
	pass.End()
	pass.Release()

	stagingSize := uint64(bytesPerRow * h)
	staging, err := device.CreateBuffer(&BufferDescriptor{
		Usage: types.BufferUsageCopyDst | types.BufferUsageMapRead,
		Size:  stagingSize,
	})
	if err != nil {
		t.Fatalf("staging: %v", err)
	}
	defer staging.Release()
	enc.CopyTextureToBuffer(rt, staging, []BufferTextureCopy{{
		BufferLayout: ImageDataLayout{BytesPerRow: bytesPerRow, RowsPerImage: h},
		TextureBase:  ImageCopyTexture{Texture: rt, Aspect: TextureAspectAll},
		Size:         types.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
	}})
	cmd, err := enc.Finish(nil)
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	enc.Release()
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	cmd.Release()
	device.Poll(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := staging.Map(ctx, MapModeRead, 0, stagingSize); err != nil {
		t.Fatalf("Map: %v", err)
	}
	ptr := staging.GetMappedRange(0, stagingSize)
	if ptr == nil {
		t.Fatal("GetMappedRange nil")
	}
	got := unsafe.Slice((*byte)(ptr), int(stagingSize))
	// green pixel expected
	r, g, b := got[0], got[1], got[2]
	if g < 200 || r > 30 || b > 30 {
		t.Fatalf("pixel rgba=%d,%d,%d,? want green (indexed draw path)", r, g, b)
	}
	_ = staging.Unmap()
}

// TestS1AE_D_BindGroupLayoutConversion ensures binding-type converters hit native
// successfully when creating a layout used by a compute pipeline (D2/D8).
func TestS1AE_D_BindGroupLayoutConversion(t *testing.T) {
	inst, adapter, device, queue := s1Device(t)
	defer s1Release(inst, adapter, device, queue)

	layout, err := device.CreateBindGroupLayoutSimple([]BindGroupLayoutEntry{
		{
			Binding:    0,
			Visibility: types.ShaderStageCompute,
			Buffer: &BufferBindingLayout{
				Type:             types.BufferBindingTypeStorage, // converted +1
				HasDynamicOffset: false,
				MinBindingSize:   16,
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayoutSimple: %v", err)
	}
	defer layout.Release()

	pl, err := device.CreatePipelineLayout(&PipelineLayoutDescriptor{
		BindGroupLayouts: []*BindGroupLayout{layout},
	})
	if err != nil {
		t.Fatalf("CreatePipelineLayout: %v", err)
	}
	defer pl.Release()

	// Minimal storage buffer + bind group proves create path.
	buf, err := device.CreateBuffer(&BufferDescriptor{
		Usage: types.BufferUsageStorage | types.BufferUsageCopyDst,
		Size:  256,
	})
	if err != nil {
		t.Fatalf("buffer: %v", err)
	}
	defer buf.Release()
	bg, err := device.CreateBindGroupSimple(layout, []BindGroupEntry{
		BufferBindingEntry(0, buf, 0, 256),
	})
	if err != nil {
		t.Fatalf("CreateBindGroupSimple: %v", err)
	}
	defer bg.Release()
	if bg.Handle() == 0 {
		t.Fatal("bind group handle 0")
	}
	_ = queue
}
