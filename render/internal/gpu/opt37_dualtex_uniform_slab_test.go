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

func TestOpt37_DualTexUniformSlotStride_Aligned(t *testing.T) {
	if dualTexUniformSlotStride < dualTexUniformPayloadSize {
		t.Fatal("stride < payload")
	}
	if dualTexUniformSlotStride%256 != 0 {
		t.Fatalf("stride %d not multiple of 256", dualTexUniformSlotStride)
	}
}

// TestOpt37_DualTexMultiUniformSlab_OneWrite packs N advanced-blend ops into
// one slab WriteBuffer (class A opt37).
func TestOpt37_DualTexMultiUniformSlab_OneWrite(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	shared, _ := r73Session(t)
	device, queue := shared.device, shared.queue
	cache := &shared.dualTexBlend
	if err := cache.ensure(device); err != nil {
		t.Fatal(err)
	}
	const w, h uint32 = 32, 32
	mk := func(label string) (*webgpu.Texture, *webgpu.TextureView) {
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
	dstTex, dstView := mk("opt37_dst")
	src0Tex, src0View := mk("opt37_src0")
	src1Tex, src1View := mk("opt37_src1")
	t.Cleanup(func() {
		dstView.Release(); dstTex.Release()
		src0View.Release(); src0Tex.Release()
		src1View.Release(); src1Tex.Release()
	})
	// Seed solid colors.
	px := make([]byte, w*h*4)
	for i := 0; i < len(px); i += 4 {
		px[i+0], px[i+1], px[i+2], px[i+3] = 0, 0, 255, 255
	}
	_ = queue.WriteTexture(&webgpu.ImageCopyTexture{Texture: src0Tex}, px,
		&webgpu.ImageDataLayout{BytesPerRow: w * 4, RowsPerImage: h},
		&webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1})
	for i := 0; i < len(px); i += 4 {
		px[i+0], px[i+1], px[i+2], px[i+3] = 255, 0, 0, 255
	}
	_ = queue.WriteTexture(&webgpu.ImageCopyTexture{Texture: src1Tex}, px,
		&webgpu.ImageDataLayout{BytesPerRow: w * 4, RowsPerImage: h},
		&webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1})
	_ = queue.WriteTexture(&webgpu.ImageCopyTexture{Texture: dstTex}, make([]byte, w*h*4),
		&webgpu.ImageDataLayout{BytesPerRow: w * 4, RowsPerImage: h},
		&webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1})

	ops := []dualTexViewBlendOp{
		{srcView: src0View, bounds: image.Rect(0, 0, int(w), int(h)), mode: render.BlendMultiply, opacity: 1},
		{srcView: src1View, bounds: image.Rect(0, 0, int(w)/2, int(h)/2), mode: render.BlendScreen, opacity: 0.8},
	}
	enc, err := device.CreateCommandEncoder(dualTexMultiEncoderDesc)
	if err != nil {
		t.Fatal(err)
	}
	outs, err := dualTexAdvancedBlendViewsMultiIntoEncoder(device, queue, cache, dstView, ops, int(w), int(h), enc)
	if err != nil {
		enc.DiscardEncoding()
		t.Fatalf("multi into: %v", err)
	}
	cmd, err := enc.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatal(err)
	}
	cmd.Release()
	for _, o := range outs {
		cache.putOutBGRA(o.tex, o.view, o.bounds.Dx(), o.bounds.Dy())
	}
	cache.mu.Lock()
	slots := cache.lastMultiUniformSlots
	wb := cache.lastMultiUniformWB
	slab := cache.uniformSlab
	cache.mu.Unlock()
	if slots != 2 {
		t.Fatalf("slots=%d want 2", slots)
	}
	if wb != 1 {
		t.Fatalf("uniform WriteBuffers=%d want 1 (slab)", wb)
	}
	if slab == nil {
		t.Fatal("uniformSlab nil")
	}
	// Warm re-run: still one WriteBuffer, slab reused.
	enc2, err := device.CreateCommandEncoder(dualTexMultiEncoderDesc)
	if err != nil {
		t.Fatal(err)
	}
	outs2, err := dualTexAdvancedBlendViewsMultiIntoEncoder(device, queue, cache, dstView, ops, int(w), int(h), enc2)
	if err != nil {
		enc2.DiscardEncoding()
		t.Fatal(err)
	}
	cmd2, err := enc2.Finish()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = queue.Submit(cmd2)
	cmd2.Release()
	for _, o := range outs2 {
		cache.putOutBGRA(o.tex, o.view, o.bounds.Dx(), o.bounds.Dy())
	}
	cache.mu.Lock()
	wb2 := cache.lastMultiUniformWB
	slab2 := cache.uniformSlab
	cache.mu.Unlock()
	if wb2 != 1 {
		t.Fatalf("warm WB=%d want 1", wb2)
	}
	if slab2 != slab {
		t.Fatal("slab reallocated on warm re-run")
	}
}
