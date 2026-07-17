//go:build !nogpu

package gpu

import (
	"os"
	"testing"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// TestOpt40_GPUTexUniformSlab_OneWriteForMultiSlot packs N gpu-tex uniforms
// into one slab WriteBuffer (class A opt40).
func TestOpt40_GPUTexUniformSlab_OneWriteForMultiSlot(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	device, queue, cleanup := createNativeTestDevice(t)
	t.Cleanup(cleanup)
	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	t.Cleanup(func() { s.Destroy() })
	if err := s.ensurePipelines(); err != nil {
		t.Fatal(err)
	}

	mkView := func(label string) *webgpu.TextureView {
		tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
			Label: label, Size: webgpu.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1},
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatBGRA8Unorm,
			Usage:  types.TextureUsageTextureBinding | types.TextureUsageRenderAttachment,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { tex.Release() })
		v, err := device.CreateTextureView(tex, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { v.Release() })
		return v
	}
	v1, v2 := mkView("opt40a"), mkView("opt40b")
	cmd := func(v *webgpu.TextureView, op float32, x float32) GPUTextureDrawCommand {
		return GPUTextureDrawCommand{
			View: gpucontext.NewTextureView(unsafe.Pointer(v)),
			DstX: x, DstY: 0, DstW: 10, DstH: 10, Opacity: op,
			ViewportWidth: 64, ViewportHeight: 64,
			U0: 0, V0: 0, U1: 1, V1: 1,
		}
	}
	// Different views → cannot merge; two uniform slots.
	cmds := []GPUTextureDrawCommand{cmd(v1, 0.5, 0), cmd(v2, 0.8, 12)}
	w0 := s.lastSubmitStats.WriteBuffers
	res, err := s.buildGPUTextureResources(cmds, 64, 64, false, nil)
	if err != nil || res == nil {
		t.Fatalf("build: res=%v err=%v", res, err)
	}
	if s.gpuTexUniformSlots < 2 {
		t.Fatalf("slots=%d want ≥2", s.gpuTexUniformSlots)
	}
	if s.gpuTexUniformSlab == nil {
		t.Fatal("expected gpuTexUniformSlab")
	}
	// Vertex WriteBuffer + at most one slab uniform WriteBuffer.
	dw := s.lastSubmitStats.WriteBuffers - w0
	if dw < 1 || dw > 2 {
		t.Fatalf("WriteBuffers delta=%d want 1–2 (verts ± one slab)", dw)
	}
	w1 := s.lastSubmitStats.WriteBuffers
	// Rebuild identical → vertex sticky skip; uniform sticky skip → 0 WB.
	if _, err := s.buildGPUTextureResources(cmds, 64, 64, false, nil); err != nil {
		t.Fatal(err)
	}
	if s.lastSubmitStats.WriteBuffers != w1 {
		t.Fatalf("identical rebuild should skip WB, %d→%d", w1, s.lastSubmitStats.WriteBuffers)
	}
	// Opacity change → one slab WriteBuffer (verts sticky).
	cmds[0].Opacity = 0.25
	w2 := s.lastSubmitStats.WriteBuffers
	if _, err := s.buildGPUTextureResources(cmds, 64, 64, false, nil); err != nil {
		t.Fatal(err)
	}
	d := s.lastSubmitStats.WriteBuffers - w2
	if d != 1 {
		t.Fatalf("opacity change: WriteBuffers delta=%d want 1 (slab only)", d)
	}
}
