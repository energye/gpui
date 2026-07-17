//go:build !nogpu

package gpu

import (
	"testing"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

func TestOpt27_GPUTexBGSlotCache_ReusesView(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	t.Cleanup(cleanup)

	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	t.Cleanup(func() { s.Destroy() })
	if err := s.ensurePipelines(); err != nil {
		t.Fatalf("ensurePipelines: %v", err)
	}

	mkView := func(label string) (*webgpu.Texture, *webgpu.TextureView) {
		tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
			Label: label, Size: webgpu.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1},
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatBGRA8Unorm,
			Usage:  types.TextureUsageTextureBinding | types.TextureUsageRenderAttachment,
		})
		if err != nil {
			t.Fatal(err)
		}
		view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
			Label: label + "_v", Format: types.TextureFormatBGRA8Unorm,
			Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		return tex, view
	}

	t1, v1 := mkView("g1")
	t2, v2 := mkView("g2")
	t3, v3 := mkView("g3")
	t.Cleanup(func() {
		v1.Release()
		t1.Release()
		v2.Release()
		t2.Release()
		v3.Release()
		t3.Release()
	})

	ubuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "opt27_u", Size: imageUniformSize,
		Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ubuf.Release() })

	var cache gpuTexBGSlotCache
	var pending []*webgpu.BindGroup
	t.Cleanup(func() {
		for _, bg := range pending {
			if bg != nil {
				bg.Release()
			}
		}
		cache.releaseAll()
	})

	layout := s.imagePipeline.uniformLayout
	samp := s.imagePipeline.SamplerFor(false)

	bg1a, err := cache.getOrCreate(device, layout, ubuf, 0, v1, samp, &pending)
	if err != nil || bg1a == nil {
		t.Fatalf("bg1a: %v", err)
	}
	bg1b, err := cache.getOrCreate(device, layout, ubuf, 0, v1, samp, &pending)
	if err != nil || bg1b != bg1a {
		t.Fatalf("same view must reuse BG, err=%v", err)
	}

	// Filter publish ping-pongs a small set of views — all must stay hot.
	bg2, err := cache.getOrCreate(device, layout, ubuf, 0, v2, samp, &pending)
	if err != nil || bg2 == nil || bg2 == bg1a {
		t.Fatalf("bg2: %v same=%v", err, bg2 == bg1a)
	}
	bg3, err := cache.getOrCreate(device, layout, ubuf, 0, v3, samp, &pending)
	if err != nil || bg3 == nil {
		t.Fatalf("bg3: %v", err)
	}

	// Revisit first views — must still hit without retiring (3 slots < 4).
	pendingBefore := len(pending)
	bg1c, err := cache.getOrCreate(device, layout, ubuf, 0, v1, samp, &pending)
	if err != nil || bg1c != bg1a {
		t.Fatalf("ping-pong reuse of v1 failed, err=%v", err)
	}
	if len(pending) != pendingBefore {
		t.Fatalf("reuse must not retire BG, pending %d→%d", pendingBefore, len(pending))
	}
	bg2b, err := cache.getOrCreate(device, layout, ubuf, 0, v2, samp, &pending)
	if err != nil || bg2b != bg2 {
		t.Fatalf("ping-pong reuse of v2 failed")
	}
}

func TestOpt27_BuildGPUTextureResources_MultiViewBGCache(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	t.Cleanup(cleanup)
	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	t.Cleanup(func() { s.Destroy() })

	mkView := func(label string) (*webgpu.Texture, *webgpu.TextureView) {
		tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
			Label: label, Size: webgpu.Extent3D{Width: 8, Height: 8, DepthOrArrayLayers: 1},
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatBGRA8Unorm,
			Usage:  types.TextureUsageTextureBinding | types.TextureUsageRenderAttachment,
		})
		if err != nil {
			t.Fatal(err)
		}
		view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
			Label: label + "_v", Format: types.TextureFormatBGRA8Unorm,
			Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		return tex, view
	}

	t1, v1 := mkView("ov1")
	t2, v2 := mkView("ov2")
	t.Cleanup(func() {
		v1.Release()
		t1.Release()
		v2.Release()
		t2.Release()
	})

	cmdA := GPUTextureDrawCommand{
		DstX: 0, DstY: 0, DstW: 8, DstH: 8,
		Opacity: 1, ViewportWidth: 64, ViewportHeight: 64,
		View: gpucontext.NewTextureView(unsafe.Pointer(v1)),
	}
	cmdB := GPUTextureDrawCommand{
		DstX: 8, DstY: 0, DstW: 8, DstH: 8,
		Opacity: 1, ViewportWidth: 64, ViewportHeight: 64,
		View: gpucontext.NewTextureView(unsafe.Pointer(v2)),
	}

	// Alternating single-cmd builds (glow publish ping-pong on poolIdx 0).
	for i := 0; i < 6; i++ {
		cmd := cmdA
		if i%2 == 1 {
			cmd = cmdB
		}
		res, err := s.buildGPUTextureResources([]GPUTextureDrawCommand{cmd}, 64, 64, false, nil)
		if err != nil {
			t.Fatalf("build %d: %v", i, err)
		}
		if res == nil || len(res.drawCalls) != 1 {
			t.Fatalf("build %d: bad resources", i)
		}
	}
	n := s.gpuTexBGCacheCount()
	if n < 2 {
		t.Fatalf("expected ≥2 cached BGs after ping-pong, got %d", n)
	}
	if n > 4 {
		t.Fatalf("ring cap 4, got %d", n)
	}

	for _, bg := range s.pendingBindGroupRelease {
		if bg != nil {
			bg.Release()
		}
	}
	s.pendingBindGroupRelease = s.pendingBindGroupRelease[:0]

	// Warm alternating builds should hit both slots — no further retire.
	pending0 := len(s.pendingBindGroupRelease)
	for i := 0; i < 4; i++ {
		cmd := cmdA
		if i%2 == 1 {
			cmd = cmdB
		}
		if _, err := s.buildGPUTextureResources([]GPUTextureDrawCommand{cmd}, 64, 64, false, nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.pendingBindGroupRelease) != pending0 {
		t.Fatalf("warm ping-pong should not retire BGs, pending %d→%d", pending0, len(s.pendingBindGroupRelease))
	}
}
