//go:build !(js && wasm)

package webgpu

import (
	"testing"
	"unsafe"

	"github.com/energye/gpui/gpu/hal"
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
	"github.com/energye/gpui/gpu/types"
)

func TestStencilOperationValuesMatchNativeTypes(t *testing.T) {
	tests := []struct {
		name string
		got  types.StencilOperation
		want types.StencilOperation
	}{
		{"Keep", types.StencilOperationKeep, types.StencilOperationKeep},
		{"Zero", types.StencilOperationZero, types.StencilOperationZero},
		{"Replace", types.StencilOperationReplace, types.StencilOperationReplace},
		{"Invert", types.StencilOperationInvert, types.StencilOperationInvert},
		{"IncrementClamp", types.StencilOperationIncrementClamp, types.StencilOperationIncrementClamp},
		{"DecrementClamp", types.StencilOperationDecrementClamp, types.StencilOperationDecrementClamp},
		{"IncrementWrap", types.StencilOperationIncrementWrap, types.StencilOperationIncrementWrap},
		{"DecrementWrap", types.StencilOperationDecrementWrap, types.StencilOperationDecrementWrap},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("types.StencilOperation%s = %d, want %d", tt.name, tt.got, tt.want)
			}
			if rwgpu.StencilOperation(tt.got) != rwgpu.StencilOperation(tt.want) {
				t.Fatalf("types.StencilOperation%s native cast = %d, want %d", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestConvertDepthStencilStateUsesNativeStencilValues(t *testing.T) {
	ds := &DepthStencilState{
		Format:            types.TextureFormatDepth24Plus,
		DepthWriteEnabled: true,
		DepthCompare:      types.CompareFunctionLess,
		StencilFront: hal.StencilFaceState{
			Compare:     types.CompareFunctionAlways,
			FailOp:      types.StencilOperationKeep,
			DepthFailOp: types.StencilOperationZero,
			PassOp:      types.StencilOperationIncrementWrap,
		},
		StencilBack: hal.StencilFaceState{
			Compare:     types.CompareFunctionAlways,
			FailOp:      types.StencilOperationReplace,
			DepthFailOp: types.StencilOperationDecrementClamp,
			PassOp:      types.StencilOperationDecrementWrap,
		},
	}

	rd := convertDepthStencilState(ds)
	if rd.StencilFront.FailOp != rwgpu.StencilOperation(types.StencilOperationKeep) {
		t.Fatalf("front fail op = %d, want native Keep=%d", rd.StencilFront.FailOp, types.StencilOperationKeep)
	}
	if rd.StencilFront.DepthFailOp != rwgpu.StencilOperation(types.StencilOperationZero) {
		t.Fatalf("front depth fail op = %d, want native Zero=%d", rd.StencilFront.DepthFailOp, types.StencilOperationZero)
	}
	if rd.StencilFront.PassOp != rwgpu.StencilOperation(types.StencilOperationIncrementWrap) {
		t.Fatalf("front pass op = %d, want native IncrementWrap=%d", rd.StencilFront.PassOp, types.StencilOperationIncrementWrap)
	}
	if rd.StencilBack.FailOp != rwgpu.StencilOperation(types.StencilOperationReplace) {
		t.Fatalf("back fail op = %d, want native Replace=%d", rd.StencilBack.FailOp, types.StencilOperationReplace)
	}
	if rd.StencilBack.DepthFailOp != rwgpu.StencilOperation(types.StencilOperationDecrementClamp) {
		t.Fatalf("back depth fail op = %d, want native DecrementClamp=%d", rd.StencilBack.DepthFailOp, types.StencilOperationDecrementClamp)
	}
	if rd.StencilBack.PassOp != rwgpu.StencilOperation(types.StencilOperationDecrementWrap) {
		t.Fatalf("back pass op = %d, want native DecrementWrap=%d", rd.StencilBack.PassOp, types.StencilOperationDecrementWrap)
	}
}

func TestConvertRenderPipelineDescKeepsVertexAttributesAlive(t *testing.T) {
	desc := &RenderPipelineDescriptor{
		Vertex: VertexState{
			EntryPoint: "vs_main",
			Buffers: []VertexBufferLayout{
				{
					ArrayStride: 16,
					StepMode:    types.VertexStepModeVertex,
					Attributes: []types.VertexAttribute{
						{Format: types.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},
						{Format: types.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1},
					},
				},
			},
		},
		Primitive: types.PrimitiveState{
			Topology:  types.PrimitiveTopologyTriangleList,
			FrontFace: types.FrontFaceCCW,
			CullMode:  types.CullModeNone,
		},
	}

	rd, keepAlive := convertRenderPipelineDesc(desc)
	if len(rd.Vertex.Buffers) != 1 {
		t.Fatalf("converted buffers = %d, want 1", len(rd.Vertex.Buffers))
	}
	if len(keepAlive) != 1 {
		t.Fatalf("keepAlive outer len = %d, want 1", len(keepAlive))
	}
	if len(keepAlive[0]) != 2 {
		t.Fatalf("keepAlive[0] len = %d, want 2", len(keepAlive[0]))
	}
	if rd.Vertex.Buffers[0].AttributeCount != 2 {
		t.Fatalf("attribute count = %d, want 2", rd.Vertex.Buffers[0].AttributeCount)
	}
	gotPtr := uintptr(unsafe.Pointer(rd.Vertex.Buffers[0].Attributes))
	wantPtr := uintptr(unsafe.Pointer(&keepAlive[0][0]))
	if gotPtr != wantPtr {
		t.Fatalf("attribute pointer = %#x, want keep-alive backing pointer %#x", gotPtr, wantPtr)
	}
}
