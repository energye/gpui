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

func TestOpt28_ImageVertSticky_SkipsRepeatWrite(t *testing.T) {
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

	px := []byte{255, 0, 0, 255, 0, 255, 0, 255, 0, 0, 255, 255, 255, 255, 0, 255}
	cmd := ImageDrawCommand{
		PixelData: px, GenerationID: 7, ImgWidth: 2, ImgHeight: 2, ImgStride: 8,
		DstX: 1, DstY: 2, DstW: 10, DstH: 12, Opacity: 1,
		TLX: 1, TLY: 2, TRX: 11, TRY: 2, BRX: 11, BRY: 14, BLX: 1, BLY: 14,
		ViewportWidth: 64, ViewportHeight: 64,
		U0: 0, V0: 0, U1: 1, V1: 1,
	}
	if _, err := s.buildImageResources([]ImageDrawCommand{cmd}, 64, 64, nil); err != nil {
		t.Fatal(err)
	}
	if !s.imageVertLastValid {
		t.Fatal("imageVertLastValid not armed")
	}
	w1 := s.lastSubmitStats.WriteBuffers
	if _, err := s.buildImageResources([]ImageDrawCommand{cmd}, 64, 64, nil); err != nil {
		t.Fatal(err)
	}
	w2 := s.lastSubmitStats.WriteBuffers
	if w2 != w1 {
		t.Fatalf("expected sticky skip on identical image verts+uniform, WriteBuffers %d→%d", w1, w2)
	}

	cmd.DstX = 3
	cmd.TLX, cmd.TRX, cmd.BRX, cmd.BLX = 3, 13, 13, 3
	if _, err := s.buildImageResources([]ImageDrawCommand{cmd}, 64, 64, nil); err != nil {
		t.Fatal(err)
	}
	w3 := s.lastSubmitStats.WriteBuffers
	if w3 <= w2 {
		t.Fatalf("geometry change must WriteBuffer verts, %d→%d", w2, w3)
	}
}

func TestOpt28_GPUTexVertSticky_SkipsRepeatWrite(t *testing.T) {
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

	tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label: "opt28", Size: webgpu.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatBGRA8Unorm,
		Usage:  types.TextureUsageTextureBinding | types.TextureUsageRenderAttachment,
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
		Label: "opt28v", Format: types.TextureFormatBGRA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { view.Release(); tex.Release() })

	cmd := GPUTextureDrawCommand{
		DstX: 10, DstY: 20, DstW: 30, DstH: 40,
		Opacity: 1, ViewportWidth: 100, ViewportHeight: 80,
		View: gpucontext.NewTextureView(unsafe.Pointer(view)),
	}
	if _, err := s.buildGPUTextureResources([]GPUTextureDrawCommand{cmd}, 100, 80, false, nil); err != nil {
		t.Fatal(err)
	}
	if !s.gpuTexVertLastValid {
		t.Fatal("gpuTexVertLastValid not armed")
	}
	w1 := s.lastSubmitStats.WriteBuffers
	if _, err := s.buildGPUTextureResources([]GPUTextureDrawCommand{cmd}, 100, 80, false, nil); err != nil {
		t.Fatal(err)
	}
	w2 := s.lastSubmitStats.WriteBuffers
	if w2 != w1 {
		t.Fatalf("expected gpu-tex vertex+uniform sticky, WriteBuffers %d→%d", w1, w2)
	}
	cmd.DstY = 21
	if _, err := s.buildGPUTextureResources([]GPUTextureDrawCommand{cmd}, 100, 80, false, nil); err != nil {
		t.Fatal(err)
	}
	if s.lastSubmitStats.WriteBuffers <= w2 {
		t.Fatal("dst change must re-upload verts")
	}
}
