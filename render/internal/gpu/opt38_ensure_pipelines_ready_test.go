//go:build !nogpu

package gpu

import (
	"os"
	"testing"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

// TestOpt38_EnsurePipelines_ReadyFastPath skips full ensure on warm frames
// (class A opt38). Second ensurePipelines call must report lastEnsurePipelines=0.
func TestOpt38_EnsurePipelines_ReadyFastPath(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	shared := NewGPUShared()
	t.Cleanup(func() { shared.Close() })
	if err := shared.ensureGPU(); err != nil || !shared.gpuReady {
		t.Skipf("GPU not ready: %v", err)
	}
	shared.ensurePipelines()
	s := NewGPURenderSession(shared.device, shared.queue, 1)
	t.Cleanup(func() { s.Destroy() })
	s.SetConvexRenderer(NewConvexRenderer(shared.device, shared.queue, 1))
	s.SetSDFPipeline(NewSDFRenderPipeline(shared.device, shared.queue, 1))
	s.SetStencilRenderer(NewStencilRenderer(shared.device, shared.queue, 1))

	if err := s.ensureClipBindLayout(); err != nil {
		t.Fatal(err)
	}
	if err := s.ensurePipelines(); err != nil {
		t.Fatal(err)
	}
	if s.lastEnsurePipelines != 1 {
		t.Fatalf("first ensure last=%d want 1 (full)", s.lastEnsurePipelines)
	}
	if !s.pipelinesReady {
		t.Fatal("pipelinesReady false after full ensure")
	}
	// Warm: identical layouts → fast skip.
	if err := s.ensurePipelines(); err != nil {
		t.Fatal(err)
	}
	if s.lastEnsurePipelines != 0 {
		t.Fatalf("warm ensure last=%d want 0 (fast)", s.lastEnsurePipelines)
	}
	if s.ensurePipelinesFastN < 1 {
		t.Fatalf("ensurePipelinesFastN=%d want ≥1", s.ensurePipelinesFastN)
	}
	if s.ensurePipelinesFullN < 1 {
		t.Fatalf("ensurePipelinesFullN=%d want ≥1", s.ensurePipelinesFullN)
	}
	// Inject new convex renderer → must re-enter full ensure.
	s.SetConvexRenderer(NewConvexRenderer(shared.device, shared.queue, 1))
	if s.pipelinesReady {
		t.Fatal("SetConvexRenderer should clear pipelinesReady")
	}
	if err := s.ensurePipelines(); err != nil {
		t.Fatal(err)
	}
	if s.lastEnsurePipelines != 1 {
		t.Fatalf("after inject last=%d want 1", s.lastEnsurePipelines)
	}
}

// TestOpt38_EnsurePipelines_WarmRenderFrame uses RenderFrameGrouped twice and
// expects the second frame's ensure path to be warm (fast).
func TestOpt38_EnsurePipelines_WarmRenderFrame(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	shared := NewGPUShared()
	t.Cleanup(func() { shared.Close() })
	if err := shared.ensureGPU(); err != nil || !shared.gpuReady {
		t.Skipf("GPU not ready: %v", err)
	}
	s := NewGPURenderSession(shared.device, shared.queue, 1)
	t.Cleanup(func() { s.Destroy() })
	s.SetConvexRenderer(NewConvexRenderer(shared.device, shared.queue, 1))
	s.SetSDFPipeline(NewSDFRenderPipeline(shared.device, shared.queue, 1))
	s.SetStencilRenderer(NewStencilRenderer(shared.device, shared.queue, 1))

	const w, h uint32 = 64, 64
	mk := func(label string) (*webgpu.Texture, *webgpu.TextureView) {
		tex, err := shared.device.CreateTexture(&webgpu.TextureDescriptor{
			Label: label, Size: webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatBGRA8Unorm,
			Usage:  types.TextureUsageRenderAttachment | types.TextureUsageTextureBinding | types.TextureUsageCopySrc,
		})
		if err != nil {
			t.Fatal(err)
		}
		view, err := shared.device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
			Format: types.TextureFormatBGRA8Unorm, Dimension: types.TextureViewDimension2D,
			Aspect: types.TextureAspectAll, MipLevelCount: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		return tex, view
	}
	cmd := ConvexDrawCommand{
		Points: []render.Point{{X: 8, Y: 8}, {X: 40, Y: 8}, {X: 24, Y: 40}},
		Color:  [4]float32{1, 0, 0, 1},
	}
	groups := []ScissorGroup{{ConvexCommands: []ConvexDrawCommand{cmd}}}
	for i, label := range []string{"opt38_a", "opt38_b"} {
		tex, view := mk(label)
		t.Cleanup(func() { view.Release(); tex.Release() })
		target := render.GPURenderTarget{
			View: gpucontext.NewTextureView(unsafe.Pointer(view)),
			ViewWidth: w, ViewHeight: h, Width: int(w), Height: int(h),
		}
		if err := s.RenderFrameGrouped(target, groups, nil, nil); err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if i == 0 && s.lastEnsurePipelines != 1 {
			t.Fatalf("frame0 ensure last=%d want 1", s.lastEnsurePipelines)
		}
		if i == 1 && s.lastEnsurePipelines != 0 {
			t.Fatalf("frame1 ensure last=%d want 0 (warm)", s.lastEnsurePipelines)
		}
	}
}
