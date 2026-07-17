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

// TestOpt36_FilterSeedSharedEncoder_OneFinish encodes a seed clear pass then
// continues the blur graph on the same encoder (class A opt36).
func TestOpt36_FilterSeedSharedEncoder_OneFinish(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	shared := NewGPUShared()
	t.Cleanup(func() { shared.Close() })
	if err := shared.ensureGPU(); err != nil || !shared.gpuReady {
		t.Skipf("GPU not ready: %v", err)
	}
	device, queue := shared.device, shared.queue
	cache := &shared.filterGPU

	const w, h uint32 = 48, 48
	tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label: "opt36_seed", Size: webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatBGRA8Unorm,
		Usage:  types.TextureUsageRenderAttachment | types.TextureUsageTextureBinding | types.TextureUsageCopySrc | types.TextureUsageCopyDst,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tex.Release() })
	view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
		Format: types.TextureFormatBGRA8Unorm, Dimension: types.TextureViewDimension2D,
		Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { view.Release() })

	// Seed: solid fill via session sharedEncoder (mesh half of FlushAndFilter).
	enc, err := device.CreateCommandEncoder(filterSeedMeshEncoderDesc)
	if err != nil {
		t.Fatal(err)
	}
	s := NewGPURenderSession(device, queue, 1)
	t.Cleanup(func() { s.Destroy() })
	s.SetConvexRenderer(NewConvexRenderer(device, queue, 1))
	s.SetSDFPipeline(NewSDFRenderPipeline(device, queue, 1))
	s.SetStencilRenderer(NewStencilRenderer(device, queue, 1))

	srcView := gpucontext.NewTextureView(unsafe.Pointer(view))
	target := render.GPURenderTarget{
		Width: int(w), Height: int(h), View: srcView, ViewWidth: w, ViewHeight: h,
	}
	cmd := ConvexDrawCommand{
		Points: []render.Point{{X: 8, Y: 8}, {X: 40, Y: 8}, {X: 24, Y: 40}},
		Color:  [4]float32{0.1, 0.2, 0.9, 1},
	}
	if err := s.RenderFrameGrouped(target, []ScissorGroup{{ConvexCommands: []ConvexDrawCommand{cmd}}}, nil, enc); err != nil {
		enc.DiscardEncoding()
		t.Fatalf("seed encode: %v", err)
	}

	nodes := []render.ImageFilterNode{{Kind: render.ImageFilterBlur, Radius: 1.5}}
	outView, release, err := runGPUFilterGraphFromViewIntoEncoder(device, queue, cache, srcView, int(w), int(h), nodes, enc)
	if err != nil {
		t.Fatalf("into encoder: %v", err)
	}
	if release != nil {
		t.Cleanup(release)
	}
	if outView.IsNil() {
		t.Fatal("nil filter out view")
	}
	cache.mu.Lock()
	finishes := cache.lastGraphFinishes
	sharedUsed := cache.lastUsedSharedEnc
	wb := cache.lastPassUniformWB
	slots := cache.lastPassUniformSlots
	cache.mu.Unlock()
	if !sharedUsed {
		t.Fatal("expected lastUsedSharedEnc")
	}
	if finishes != 1 {
		t.Fatalf("graph Finishes=%d want 1 (seed+filter one Finish)", finishes)
	}
	if wb != 1 {
		t.Fatalf("pass uniform WB=%d want 1", wb)
	}
	if slots < 2 {
		t.Fatalf("blur slots=%d want >=2", slots)
	}
}

// TestOpt36_FilterFromView_StillOneFinish keeps standalone FromView at one Finish.
func TestOpt36_FilterFromView_StillOneFinish(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	shared := NewGPUShared()
	t.Cleanup(func() { shared.Close() })
	if err := shared.ensureGPU(); err != nil || !shared.gpuReady {
		t.Skipf("GPU not ready: %v", err)
	}
	device, queue := shared.device, shared.queue
	cache := &shared.filterGPU
	const w, h = 32, 32
	src := make([]byte, w*h*4)
	for i := 0; i < len(src); i += 4 {
		src[i+0], src[i+1], src[i+2], src[i+3] = 30, 60, 200, 255
	}
	// CPU upload path (no shared enc).
	_, err := runGPUFilterGraph(device, queue, cache, src, w, h, []render.ImageFilterNode{
		{Kind: render.ImageFilterBlur, Radius: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	cache.mu.Lock()
	finishes := cache.lastGraphFinishes
	sharedUsed := cache.lastUsedSharedEnc
	cache.mu.Unlock()
	if sharedUsed {
		t.Fatal("standalone graph should not mark sharedEnc")
	}
	if finishes != 1 {
		t.Fatalf("standalone Finishes=%d want 1", finishes)
	}
}

func TestOpt36_FilterSeedSharedEncoder_StaticDescs(t *testing.T) {
	if filterSeedMeshEncoderDesc == nil || filterGPUBatchEncoderDesc == nil || filterGPUReadEncoderDesc == nil {
		t.Fatal("static filter encoder descs missing")
	}
}
