//go:build !(js && wasm)

package webgpu

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
)

func TestR76_ConvertVertexLayouts_CommonShape(t *testing.T) {
	layouts := []VertexBufferLayout{{
		ArrayStride: 16,
		StepMode:    types.VertexStepModeVertex,
		Attributes: []types.VertexAttribute{
			{Format: types.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},
			{Format: types.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1},
		},
	}}
	sc := acquireRPLConvertScratch()
	defer releaseRPLConvertScratch(sc)
	bufs, keep := convertVertexBufferLayoutsInto(sc, layouts)
	if len(bufs) != 1 || len(keep) != 1 {
		t.Fatalf("bufs=%d keep=%d", len(bufs), len(keep))
	}
	if bufs[0].AttributeCount != 2 {
		t.Fatalf("attr count %d", bufs[0].AttributeCount)
	}
	if keep[0][0].ShaderLocation != 0 || keep[0][1].ShaderLocation != 1 {
		t.Fatalf("attrs %+v", keep[0])
	}
}

func TestR76_ConvertFragment_CommonShape(t *testing.T) {
	blend := &types.BlendState{
		Color: types.BlendComponent{SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorOneMinusSrcAlpha, Operation: types.BlendOperationAdd},
		Alpha: types.BlendComponent{SrcFactor: types.BlendFactorOne, DstFactor: types.BlendFactorOneMinusSrcAlpha, Operation: types.BlendOperationAdd},
	}
	fs := &FragmentState{
		EntryPoint: "fs_main",
		Targets: []ColorTargetState{{
			Format:    types.TextureFormatBGRA8Unorm,
			WriteMask: types.ColorWriteMaskAll,
			Blend:     blend,
		}},
	}
	sc := acquireRPLConvertScratch()
	defer releaseRPLConvertScratch(sc)
	convertFragmentStateInto(sc, fs)
	if len(sc.fragment.Targets) != 1 {
		t.Fatalf("targets=%d", len(sc.fragment.Targets))
	}
	if sc.fragment.Targets[0].Blend == nil {
		t.Fatal("blend nil")
	}
	if sc.fragment.Targets[0].Blend.Color.SrcFactor != types.BlendFactorOne {
		t.Fatalf("blend src=%v", sc.fragment.Targets[0].Blend.Color.SrcFactor)
	}
}

func TestR76_ConvertPipelineDescInto_NoNilCrash(t *testing.T) {
	sc := acquireRPLConvertScratch()
	defer releaseRPLConvertScratch(sc)
	desc := &RenderPipelineDescriptor{
		Vertex: VertexState{
			EntryPoint: "vs_main",
			Buffers: []VertexBufferLayout{{
				ArrayStride: 8,
				Attributes:  []types.VertexAttribute{{Format: types.VertexFormatFloat32x2, ShaderLocation: 0}},
			}},
		},
		Fragment: &FragmentState{
			EntryPoint: "fs_main",
			Targets: []ColorTargetState{{
				Format:    types.TextureFormatBGRA8Unorm,
				WriteMask: types.ColorWriteMaskAll,
			}},
		},
		Multisample: MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
	}
	rDesc, keep := convertRenderPipelineDescInto(sc, desc)
	if rDesc == nil || rDesc.Fragment == nil {
		t.Fatal("nil rDesc/fragment")
	}
	if len(keep) != 1 || keep[0][0].ShaderLocation != 0 {
		t.Fatalf("keep=%+v", keep)
	}
	if len(rDesc.Vertex.Buffers) != 1 {
		t.Fatalf("buffers=%d", len(rDesc.Vertex.Buffers))
	}
}
