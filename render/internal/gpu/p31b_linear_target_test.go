//go:build !nogpu

package gpu

import (
	"testing"
	"unsafe"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

// P3.1b-2a 能力钉：opt-in sRGB 目标上固定功能 13 模式走硬件线性混合，
// shader 输出经 encode_out 线性化；默认 Unorm 目标仍按 sRGB 域并保持降级计数。
func TestP31bLinearTargetFixedBlend(t *testing.T) {
	shared, rc := newP2Shared(t)

	const size = 64
	const rowBytes = size * 4
	fill := func(format types.TextureFormat) ([4]byte, int64) {
		view, release, err := rc.CreateOffscreenTextureFormat(size, size, format)
		if err != nil {
			t.Fatalf("CreateOffscreenTextureFormat(%v): %v", format, err)
		}
		defer release()
		target := render.GPURenderTarget{
			View: view, ViewWidth: size, ViewHeight: size,
			Width: size, Height: size, Stride: rowBytes,
		}
		path := render.NewPath()
		path.Rectangle(0, 0, size, size)
		base := render.NewPaint()
		base.SetBrush(render.Solid(render.RGBA{R: 0.5, G: 0.25, B: 0.1, A: 1}))
		if err := rc.FillPath(target, path, base, render.Identity()); err != nil {
			t.Fatalf("FillPath base (%v): %v", format, err)
		}
		before := shared.linearFixedFunctionFallbacks.Load()
		plus := render.NewPaint()
		plus.SetBrush(render.Solid(render.RGBA{R: 0.25, G: 0.25, B: 0.25, A: 1}))
		plus.BlendMode = render.BlendPlus
		plus.BlendSpace = render.BlendSpaceLinear
		if err := rc.FillPath(target, path, plus, render.Identity()); err != nil {
			t.Fatalf("FillPath plus (%v): %v", format, err)
		}
		if err := rc.Flush(target); err != nil {
			t.Fatalf("Flush (%v): %v", format, err)
		}
		delta := shared.linearFixedFunctionFallbacks.Load() - before

		tex := unpackView(view).Texture()
		device, queue := shared.Device(), shared.Queue()
		staging, err := device.CreateBuffer(&hal.BufferDescriptor{
			Label: "p31b_linear_target_staging", Size: rowBytes * size,
			Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
		})
		if err != nil {
			t.Fatalf("staging: %v", err)
		}
		defer staging.Destroy()
		enc, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "p31b_linear_target_copy"})
		if err != nil {
			t.Fatalf("encoder: %v", err)
		}
		enc.CopyTextureToBuffer(tex, staging, []hal.BufferTextureCopy{{
			BufferLayout: hal.ImageDataLayout{Offset: 0, BytesPerRow: rowBytes, RowsPerImage: size},
			TextureBase:  hal.ImageCopyTexture{Texture: tex, MipLevel: 0, Origin: hal.Origin3D{}, Aspect: types.TextureAspectAll},
			Size:         hal.Extent3D{Width: size, Height: size, DepthOrArrayLayers: 1},
		}})
		cmd, err := enc.Finish()
		if err != nil {
			t.Fatalf("finish copy: %v", err)
		}
		defer device.FreeCommandBuffer(cmd)
		if _, err := queue.Submit(cmd); err != nil {
			t.Fatalf("submit copy: %v", err)
		}
		mapping, err := device.MapBuffer(staging, 0, rowBytes*size)
		if err != nil {
			t.Fatalf("map: %v", err)
		}
		raw := make([]byte, rowBytes*size)
		copy(raw, unsafe.Slice((*byte)(mapping.Ptr), rowBytes*size))
		if err := device.UnmapBuffer(staging); err != nil {
			t.Fatalf("unmap: %v", err)
		}
		center := size/2*rowBytes + size/2*4
		var out [4]byte
		copy(out[:], raw[center:center+4])
		return out, delta
	}

	srgb, srgbFallback := fill(types.TextureFormatRGBA8UnormSrgb)
	unorm, unormFallback := fill(types.TextureFormatRGBA8Unorm)
	t.Logf("P3.1b-2a sRGB target=%v fallback=%d; Unorm target=%v fallback=%d", srgb, srgbFallback, unorm, unormFallback)

	// sRGB target: Plus runs in linear light, shader output linearized.
	// linear(0.5)+linear(0.25)=0.2649 -> sRGB 141; G 0.1018 -> 89; B 0.0609 -> 70.
	wantSrgb := [3]int{141, 89, 70}
	for i := 0; i < 3; i++ {
		if d := int(srgb[i]) - wantSrgb[i]; d < -3 || d > 3 {
			t.Fatalf("sRGB target channel %d = %d, want %d±3 (linear Plus); bytes=%v", i, srgb[i], wantSrgb[i], srgb)
		}
	}
	if srgbFallback != 0 {
		t.Fatalf("sRGB target fixed-function linear fallback = %d, want 0", srgbFallback)
	}
	// Unorm target: Plus stays in encoded domain; linear request is counted as
	// a deliberate degradation.
	wantUnorm := [3]int{192, 128, 90}
	for i := 0; i < 3; i++ {
		if d := int(unorm[i]) - wantUnorm[i]; d < -3 || d > 3 {
			t.Fatalf("Unorm target channel %d = %d, want %d±3 (encoded Plus); bytes=%v", i, unorm[i], wantUnorm[i], unorm)
		}
	}
	if unormFallback != 1 {
		t.Fatalf("Unorm target fixed-function linear fallback = %d, want 1", unormFallback)
	}
}
