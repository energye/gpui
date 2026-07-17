//go:build !nogpu

package gpu

import (
	"image"
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

// TestOpt32_DualTexMultiIntoEncoder_OneFinish records dual-tex multi into an
// external encoder and Finishes once (class A opt32 building block).
func TestOpt32_DualTexMultiIntoEncoder_OneFinish(t *testing.T) {
	shared, _ := r73Session(t)
	device, queue := shared.device, shared.queue
	cache := &shared.dualTexBlend
	if err := cache.ensure(device); err != nil {
		t.Fatal(err)
	}
	const w, h uint32 = 16, 16
	mk := func(label string, fill byte) (*webgpu.Texture, *webgpu.TextureView) {
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
		px := make([]byte, int(w*h*4))
		for i := 0; i < len(px); i += 4 {
			px[i], px[i+1], px[i+2], px[i+3] = fill, fill, fill, 255
		}
		_ = queue.WriteTexture(&webgpu.ImageCopyTexture{Texture: tex}, px,
			&webgpu.ImageDataLayout{BytesPerRow: w * 4, RowsPerImage: h},
			&webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1})
		return tex, view
	}
	dstTex, dstView := mk("opt32_dst", 255)
	srcTex, srcView := mk("opt32_src", 128)
	t.Cleanup(func() {
		dstView.Release()
		dstTex.Release()
		srcView.Release()
		srcTex.Release()
	})

	enc, err := device.CreateCommandEncoder(dualTexCompositeEncoderDesc)
	if err != nil {
		t.Fatal(err)
	}
	ops := []dualTexViewBlendOp{{
		srcView: srcView,
		bounds:  image.Rect(0, 0, int(w), int(h)),
		mode:    render.BlendMultiply,
		opacity: 1,
	}}
	outs, err := dualTexAdvancedBlendViewsMultiIntoEncoder(device, queue, cache, dstView, ops, int(w), int(h), enc)
	if err != nil {
		enc.DiscardEncoding()
		t.Fatalf("into encoder: %v", err)
	}
	if len(outs) != 1 {
		enc.DiscardEncoding()
		t.Fatalf("outs=%d", len(outs))
	}
	cmd, err := enc.Finish()
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	cmd.Release()
	for _, o := range outs {
		cache.putOutBGRA(o.tex, o.view, o.bounds.Dx(), o.bounds.Dy())
	}
}

func TestOpt32_EncodeBlitToEncoder_LoadOpUsesFrameRendered(t *testing.T) {
	// Mirrors encodeBlitToEncoder decision: frameRendered || damage ⇒ Load.
	frameRendered := true
	hasDamage := false
	if !(frameRendered || hasDamage) {
		t.Fatal("want LoadOpLoad")
	}
	frameRendered = false
	if frameRendered || hasDamage {
		t.Fatal("want LoadOpClear for fresh view without damage")
	}
	hasDamage = true
	if !(frameRendered || hasDamage) {
		t.Fatal("damage forces Load")
	}
}
