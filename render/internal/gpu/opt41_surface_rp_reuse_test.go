package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// TestOpt41_SurfaceRenderPassDesc_NoAllocWarm ensures surfaceRenderPassDesc
// reuses backing storage after first init (class A opt41).
func TestOpt41_SurfaceRenderPassDesc_NoAllocWarm(t *testing.T) {
	s := &GPURenderSession{}
	// stencil view pointer only used as identity; no native calls.
	fakeView := &webgpu.TextureView{}
	s.textures.stencilView = fakeView
	s.sampleCount = 1

	// Cold: may allocate slice header wiring once.
	_ = s.surfaceRenderPassDesc("t", fakeView, types.LoadOpClear, types.LoadOpClear, types.LoadOpClear)
	if !s.surfaceRPInited {
		t.Fatal("surfaceRPInited false after first call")
	}
	if len(s.surfaceRPDesc.ColorAttachments) != 1 {
		t.Fatalf("ColorAttachments len=%d", len(s.surfaceRPDesc.ColorAttachments))
	}
	if s.surfaceRPDesc.DepthStencilAttachment != &s.surfaceDSAtt {
		t.Fatal("DepthStencilAttachment not pointing at surfaceDSAtt")
	}

	allocs := testing.AllocsPerRun(200, func() {
		desc := s.surfaceRenderPassDesc("t", fakeView, types.LoadOpLoad, types.LoadOpLoad, types.LoadOpClear)
		if desc == nil || desc.DepthStencilAttachment == nil {
			t.Fatal("nil desc")
		}
		if desc.ColorAttachments[0].View != fakeView {
			t.Fatal("color view not set")
		}
		if desc.DepthStencilAttachment.StencilLoadOp != types.LoadOpLoad {
			t.Fatal("stencil load not applied")
		}
	})
	if allocs != 0 {
		t.Fatalf("warm surfaceRenderPassDesc allocs=%v want 0", allocs)
	}
}
