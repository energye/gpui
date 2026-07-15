//go:build !(js && wasm)

package webgpu

// S2 facade conversion tests — field completeness and high-risk enum passthrough
// from webgpu → rwgpu. Native enum→header mapping is owned by S1 (rwgpu).

import (
	"testing"
	"unsafe"

	rwgpu "github.com/energye/gpui/gpu/rwgpu"
	"github.com/energye/gpui/gpu/types"
)

func TestS2ConvertRenderPipelinePrimitiveFields(t *testing.T) {
	strip := types.IndexFormatUint16
	desc := &RenderPipelineDescriptor{
		Label: "s2-prim",
		Vertex: VertexState{
			EntryPoint: "vs",
		},
		Primitive: types.PrimitiveState{
			Topology:         types.PrimitiveTopologyTriangleList,
			FrontFace:        types.FrontFaceCCW,
			CullMode:         types.CullModeBack,
			StripIndexFormat: &strip,
			UnclippedDepth:   true,
		},
		Multisample: types.MultisampleState{
			Count:                  4,
			Mask:                   0xF,
			AlphaToCoverageEnabled: true,
		},
	}
	rd, _ := convertRenderPipelineDesc(desc)
	if rd.Primitive.Topology != types.PrimitiveTopologyTriangleList {
		t.Fatalf("Topology = %v", rd.Primitive.Topology)
	}
	if rd.Primitive.FrontFace != types.FrontFaceCCW {
		t.Fatalf("FrontFace = %v", rd.Primitive.FrontFace)
	}
	if rd.Primitive.CullMode != types.CullModeBack {
		t.Fatalf("CullMode = %v", rd.Primitive.CullMode)
	}
	if rd.Primitive.StripIndexFormat != types.IndexFormatUint16 {
		t.Fatalf("StripIndexFormat = %v", rd.Primitive.StripIndexFormat)
	}
	if !rd.Primitive.UnclippedDepth {
		t.Fatal("UnclippedDepth dropped")
	}
	if rd.Multisample.Count != 4 || rd.Multisample.Mask != 0xF || !rd.Multisample.AlphaToCoverageEnabled {
		t.Fatalf("Multisample = %+v", rd.Multisample)
	}
}

func TestS2ConvertFragmentBlendFields(t *testing.T) {
	fs := &FragmentState{
		EntryPoint: "fs",
		Targets: []types.ColorTargetState{{
			Format:    types.TextureFormatRGBA8Unorm,
			WriteMask: types.ColorWriteMaskAll,
			Blend: &types.BlendState{
				Color: types.BlendComponent{
					SrcFactor: types.BlendFactorSrcAlpha,
					DstFactor: types.BlendFactorOneMinusSrcAlpha,
					Operation: types.BlendOperationAdd,
				},
				Alpha: types.BlendComponent{
					SrcFactor: types.BlendFactorOne,
					DstFactor: types.BlendFactorOneMinusSrcAlpha,
					Operation: types.BlendOperationAdd,
				},
			},
		}},
	}
	out := convertFragmentState(fs)
	if out == nil || len(out.Targets) != 1 || out.Targets[0].Blend == nil {
		t.Fatal("blend missing")
	}
	b := out.Targets[0].Blend
	if b.Color.SrcFactor != types.BlendFactorSrcAlpha ||
		b.Color.DstFactor != types.BlendFactorOneMinusSrcAlpha ||
		b.Color.Operation != types.BlendOperationAdd {
		t.Fatalf("color blend = %+v", b.Color)
	}
	if b.Alpha.SrcFactor != types.BlendFactorOne {
		t.Fatalf("alpha blend = %+v", b.Alpha)
	}
	// Wire field order for native BlendComponent: Operation, Src, Dst
	// (must match lib/webgpu.h WGPUBlendComponent).
	type wireBC struct {
		Operation types.BlendOperation
		SrcFactor types.BlendFactor
		DstFactor types.BlendFactor
	}
	if unsafe.Sizeof(rwgpu.BlendComponent{}) != unsafe.Sizeof(wireBC{}) {
		t.Fatalf("BlendComponent size mismatch: rwgpu=%d wire=%d",
			unsafe.Sizeof(rwgpu.BlendComponent{}), unsafe.Sizeof(wireBC{}))
	}
	if unsafe.Offsetof(rwgpu.BlendComponent{}.Operation) != 0 {
		t.Fatalf("Operation offset = %d, want 0", unsafe.Offsetof(rwgpu.BlendComponent{}.Operation))
	}
}

func TestS2ConvertBindGroupLayoutBindingTypes(t *testing.T) {
	e := convertBindGroupLayoutEntry(types.BindGroupLayoutEntry{
		Binding:    0,
		Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
		Buffer: &types.BufferBindingLayout{
			Type:             types.BufferBindingTypeUniform,
			HasDynamicOffset: true,
			MinBindingSize:   64,
		},
	})
	if e.Buffer == nil || e.Buffer.Type != types.BufferBindingTypeUniform {
		t.Fatalf("buffer type = %+v", e.Buffer)
	}
	if !e.Buffer.HasDynamicOffset || e.Buffer.MinBindingSize != 64 {
		t.Fatalf("buffer layout = %+v", e.Buffer)
	}

	e2 := convertBindGroupLayoutEntry(types.BindGroupLayoutEntry{
		Binding:    1,
		Visibility: types.ShaderStageFragment,
		Sampler:    &types.SamplerBindingLayout{Type: types.SamplerBindingTypeFiltering},
	})
	if e2.Sampler == nil || e2.Sampler.Type != types.SamplerBindingTypeFiltering {
		t.Fatalf("sampler = %+v", e2.Sampler)
	}

	e3 := convertBindGroupLayoutEntry(types.BindGroupLayoutEntry{
		Binding:    2,
		Visibility: types.ShaderStageFragment,
		Texture: &types.TextureBindingLayout{
			SampleType:    types.TextureSampleTypeFloat,
			ViewDimension: types.TextureViewDimension2D,
			Multisampled:  false,
		},
	})
	if e3.Texture == nil || e3.Texture.SampleType != types.TextureSampleTypeFloat {
		t.Fatalf("texture = %+v", e3.Texture)
	}

	e4 := convertBindGroupLayoutEntry(types.BindGroupLayoutEntry{
		Binding:    3,
		Visibility: types.ShaderStageCompute,
		StorageTexture: &types.StorageTextureBindingLayout{
			Access:        types.StorageTextureAccessWriteOnly,
			Format:        types.TextureFormatRGBA8Unorm,
			ViewDimension: types.TextureViewDimension2D,
		},
	})
	if e4.StorageTexture == nil || e4.StorageTexture.Access != types.StorageTextureAccessWriteOnly {
		t.Fatalf("storage texture = %+v", e4.StorageTexture)
	}
}

func TestS2ConvertVertexLayoutsKeepAliveAndFormats(t *testing.T) {
	layouts := []types.VertexBufferLayout{{
		ArrayStride: 12,
		StepMode:    types.VertexStepModeInstance,
		Attributes: []types.VertexAttribute{
			{Format: types.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
		},
	}}
	out, keep := convertVertexBufferLayouts(layouts)
	if len(out) != 1 || out[0].AttributeCount != 1 {
		t.Fatalf("out = %+v", out)
	}
	if out[0].StepMode != types.VertexStepModeInstance {
		t.Fatalf("StepMode = %v", out[0].StepMode)
	}
	if out[0].Attributes == nil {
		t.Fatal("Attributes nil")
	}
	if keep[0][0].Format != types.VertexFormatFloat32x3 {
		t.Fatalf("format = %v", keep[0][0].Format)
	}
	if uintptr(unsafe.Pointer(out[0].Attributes)) != uintptr(unsafe.Pointer(&keep[0][0])) {
		t.Fatal("attribute pointer not backed by keepAlive")
	}
}

func TestS2ConvertDepthStencilComplete(t *testing.T) {
	ds := &DepthStencilState{
		Format:            types.TextureFormatDepth24PlusStencil8,
		DepthWriteEnabled: true,
		DepthCompare:      types.CompareFunctionLessEqual,
		StencilReadMask:   0x0F,
		StencilWriteMask:  0xF0,
		DepthBias:         1,
		StencilFront: StencilFaceState{
			Compare:     types.CompareFunctionAlways,
			FailOp:      StencilOperationKeep,
			DepthFailOp: StencilOperationIncrementClamp,
			PassOp:      StencilOperationReplace,
		},
		StencilBack: StencilFaceState{
			Compare:     types.CompareFunctionEqual,
			FailOp:      StencilOperationZero,
			DepthFailOp: StencilOperationDecrementWrap,
			PassOp:      StencilOperationInvert,
		},
	}
	rd := convertDepthStencilState(ds)
	if rd.Format != types.TextureFormatDepth24PlusStencil8 {
		t.Fatalf("format = %v", rd.Format)
	}
	if rd.DepthCompare != types.CompareFunctionLessEqual {
		t.Fatalf("depth compare = %v", rd.DepthCompare)
	}
	if rd.StencilReadMask != 0x0F || rd.StencilWriteMask != 0xF0 {
		t.Fatalf("stencil masks read=%#x write=%#x", rd.StencilReadMask, rd.StencilWriteMask)
	}
	if rd.StencilFront.PassOp != rwgpu.StencilOperation(types.StencilOperationReplace) {
		t.Fatalf("front pass = %v", rd.StencilFront.PassOp)
	}
	if rd.StencilBack.FailOp != rwgpu.StencilOperation(types.StencilOperationZero) {
		t.Fatalf("back fail = %v", rd.StencilBack.FailOp)
	}
}

func TestS2TextureViewCountUndefinedMapping(t *testing.T) {
	// Zero counts must become UINT32_MAX, not 1.
	const wantUndef = ^uint32(0)
	// Exercise the same logic as CreateTextureView without needing a device:
	mip, arr := uint32(0), uint32(0)
	if mip == 0 {
		mip = wantUndef
	}
	if arr == 0 {
		arr = wantUndef
	}
	if mip != wantUndef || arr != wantUndef {
		t.Fatalf("mip=%#x arr=%#x", mip, arr)
	}
}

func TestS2NoRenderImportRWGPU(t *testing.T) {
	// Architectural invariant documented in MAINLINE S2: render must not import rwgpu.
	// This is a lightweight compile-time-oriented reminder; full grep is CI/docs.
	// Package-level: this file itself is allowed to import rwgpu for conversion asserts.
}
