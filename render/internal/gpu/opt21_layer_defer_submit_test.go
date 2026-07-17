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

// TestOpt21_DeferSurfaceSubmit_CoalescesLayerFills encodes two offscreen
// layer fills with deferSurfaceSubmit and drains them in one Queue.Submit
// (class A opt21 — mid-frame PopLayer submit coalesce).
func TestOpt21_DeferSurfaceSubmit_CoalescesLayerFills(t *testing.T) {
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

	const w, h uint32 = 64, 64
	mkView := func(label string) (*webgpu.Texture, *webgpu.TextureView) {
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

	for i, label := range []string{"opt21_layer0", "opt21_layer1"} {
		tex, view := mkView(label)
		t.Cleanup(func() {
			view.Release()
			tex.Release()
		})
		cmd := ConvexDrawCommand{
			Points: []render.Point{{X: 8, Y: 8}, {X: 40, Y: 8}, {X: 24, Y: 40}},
			Color:  [4]float32{1, 0, float32(i) * 0.5, 1},
		}
		groups := []ScissorGroup{{
			ConvexCommands: []ConvexDrawCommand{cmd},
		}}
		target := render.GPURenderTarget{
			View:       gpucontext.NewTextureView(unsafe.Pointer(view)),
			ViewWidth:  w,
			ViewHeight: h,
			Width:      int(w),
			Height:     int(h),
		}
		s.SetDeferSurfaceSubmit(true)
		if err := s.RenderFrameGrouped(target, groups, nil, nil); err != nil {
			s.SetDeferSurfaceSubmit(false)
			t.Fatalf("layer %d encode: %v", i, err)
		}
		s.SetDeferSurfaceSubmit(false)
		if got := len(s.leadSubmitCBs); got != i+1 {
			t.Fatalf("after layer %d lead CBs=%d want %d", i, got, i+1)
		}
	}

	// Present-equivalent drain: one Submit with both layer CBs.
	if err := s.FlushLeadingSubmitsOnly(); err != nil {
		t.Fatalf("flush leading: %v", err)
	}
	if len(s.leadSubmitCBs) != 0 {
		t.Fatalf("lead CBs leftover=%d", len(s.leadSubmitCBs))
	}
	st := s.LastSubmitPathStats()
	if st.Submits != 1 {
		t.Fatalf("Submits=%d want 1", st.Submits)
	}
	if st.CoalescedCBs != 2 {
		t.Fatalf("CoalescedCBs=%d want 2 (two deferred layer fills)", st.CoalescedCBs)
	}

	// A following present encode must not see stranded leadings and must succeed.
	tex, view := mkView("opt21_present")
	t.Cleanup(func() {
		view.Release()
		tex.Release()
	})
	cmd := ConvexDrawCommand{
		Points: []render.Point{{X: 4, Y: 4}, {X: 20, Y: 4}, {X: 12, Y: 20}},
		Color:  [4]float32{0, 1, 0, 1},
	}
	target := render.GPURenderTarget{
		View:       gpucontext.NewTextureView(unsafe.Pointer(view)),
		ViewWidth:  w,
		ViewHeight: h,
		Width:      int(w),
		Height:     int(h),
	}
	if err := s.RenderFrameGrouped(target, []ScissorGroup{{ConvexCommands: []ConvexDrawCommand{cmd}}}, nil, nil); err != nil {
		t.Fatalf("present encode: %v", err)
	}
}
