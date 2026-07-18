//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// TestOpt44_FilterPassRenderPassDesc_NoAllocWarm ensures filter-pass RP desc
// reuses backing storage after first init (class A opt44 / R8.3).
func TestOpt44_FilterPassRenderPassDesc_NoAllocWarm(t *testing.T) {
	c := &filterGPUCache{}
	fakeView := &webgpu.TextureView{}

	desc := c.filterPassRenderPassDesc(fakeView)
	if desc == nil {
		t.Fatal("nil desc")
	}
	if !c.filterPassRPInited {
		t.Fatal("filterPassRPInited false after first call")
	}
	if len(desc.ColorAttachments) != 1 {
		t.Fatalf("ColorAttachments len=%d", len(desc.ColorAttachments))
	}
	if desc.ColorAttachments[0].View != fakeView {
		t.Fatal("color view not set")
	}
	if desc.ColorAttachments[0].LoadOp != types.LoadOpClear {
		t.Fatal("load op")
	}
	if desc.ColorAttachments[0].StoreOp != types.StoreOpStore {
		t.Fatal("store op")
	}

	fake2 := &webgpu.TextureView{}
	allocs := testing.AllocsPerRun(200, func() {
		d := c.filterPassRenderPassDesc(fake2)
		if d == nil || d.ColorAttachments[0].View != fake2 {
			t.Fatal("warm desc broken")
		}
		if len(d.ColorAttachments) != 1 {
			t.Fatal("len")
		}
	})
	if allocs != 0 {
		t.Fatalf("warm filterPassRenderPassDesc allocs=%v want 0", allocs)
	}
}
