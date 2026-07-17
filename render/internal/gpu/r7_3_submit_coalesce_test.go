//go:build !nogpu

package gpu

import (
	"image"
	"os"
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

func r73Session(t *testing.T) (*GPUShared, *GPURenderSession) {
	t.Helper()
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
	if s == nil {
		t.Fatal("nil session")
	}
	t.Cleanup(func() { s.Destroy() })
	return shared, s
}

// TestR73_DualTexMultiBundle_DeferredSubmit records multi dual-tex without
// immediate Submit and verifies CoalescedCBs when leading-only submit runs.
func TestR73_DualTexMultiBundle_DeferredSubmit(t *testing.T) {
	shared, session := r73Session(t)
	device, queue := shared.device, shared.queue
	cache := &shared.dualTexBlend
	if err := cache.ensure(device); err != nil {
		t.Fatalf("dual-tex ensure: %v", err)
	}

	const w, h = 32, 32
	mkRT := func(label string) (*webgpu.Texture, *webgpu.TextureView) {
		tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
			Label: label, Size: webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatBGRA8Unorm,
			Usage:  types.TextureUsageRenderAttachment | types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
		})
		if err != nil {
			t.Fatal(err)
		}
		view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
			Format: types.TextureFormatBGRA8Unorm, Dimension: types.TextureViewDimension2D,
			Aspect: types.TextureAspectAll, MipLevelCount: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		return tex, view
	}
	dstTex, dstView := mkRT("r73_dst")
	defer dstTex.Release()
	defer dstView.Release()
	srcTex, srcView := mkRT("r73_src")
	defer srcTex.Release()
	defer srcView.Release()

	px := make([]byte, w*h*4)
	for i := 0; i < len(px); i += 4 {
		px[i+0], px[i+1], px[i+2], px[i+3] = 0, 0, 255, 255
	}
	_ = queue.WriteTexture(&webgpu.ImageCopyTexture{Texture: srcTex}, px,
		&webgpu.ImageDataLayout{BytesPerRow: w * 4, RowsPerImage: h},
		&webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1})
	_ = queue.WriteTexture(&webgpu.ImageCopyTexture{Texture: dstTex}, make([]byte, w*h*4),
		&webgpu.ImageDataLayout{BytesPerRow: w * 4, RowsPerImage: h},
		&webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1})

	ops := []dualTexViewBlendOp{{
		srcView: srcView,
		bounds:  image.Rect(0, 0, w, h),
		mode:    render.BlendMultiply,
		opacity: 1,
	}}
	bundle, err := dualTexAdvancedBlendViewsMultiBundle(device, queue, cache, dstView, ops, w, h, false)
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	if bundle.Cmd == nil {
		t.Fatal("expected deferred Cmd")
	}
	if len(bundle.Outs) != 1 {
		t.Fatalf("outs=%d", len(bundle.Outs))
	}
	session.EnqueueLeadingSubmit(bundle.Cmd, bundle.Cleanup)
	if err := session.FlushLeadingSubmitsOnly(); err != nil {
		t.Fatalf("flush leading: %v", err)
	}
	st := session.LastSubmitPathStats()
	if st.Submits != 1 {
		t.Fatalf("Submits=%d want 1", st.Submits)
	}
	if st.CoalescedCBs != 1 {
		t.Fatalf("CoalescedCBs=%d want 1", st.CoalescedCBs)
	}
	for _, o := range bundle.Outs {
		cache.putOutBGRA(o.tex, o.view, o.bounds.Dx(), o.bounds.Dy())
	}
}

// TestR73_SubmitWithLeading_TwoCBs checks two-CB coalesce counter.
func TestR73_SubmitWithLeading_TwoCBs(t *testing.T) {
	_, s := r73Session(t)
	mk := func(label string) *webgpu.CommandBuffer {
		enc, err := s.device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{Label: label})
		if err != nil {
			t.Fatal(err)
		}
		cmd, err := enc.Finish()
		if err != nil {
			t.Fatal(err)
		}
		return cmd
	}
	lead := mk("r73_lead")
	main := mk("r73_main")
	s.EnqueueLeadingSubmit(lead, nil)
	if err := s.submitWithLeading(main); err != nil {
		t.Fatalf("submit: %v", err)
	}
	st := s.LastSubmitPathStats()
	if st.CoalescedCBs != 2 {
		t.Fatalf("CoalescedCBs=%d want 2", st.CoalescedCBs)
	}
	if st.Submits < 1 {
		t.Fatalf("Submits=%d", st.Submits)
	}
}
