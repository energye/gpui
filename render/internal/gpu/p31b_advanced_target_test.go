//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

// TestP31bAdvancedBlendSrgbTarget pins the dual-texture advanced blend path on
// an opt-in sRGB target. The advanced path snapshots the destination, decodes
// the sRGB bytes to linear light, applies the W3C blend, and the target's
// sRGB attachment encodes the result once on write.
func TestP31bAdvancedBlendSrgbTarget(t *testing.T) {
	shared, rc := newP2Shared(t)
	const size = 64
	data := make([]byte, size*size*4)

	run := func(format types.TextureFormat) [4]byte {
		view, release, err := rc.CreateOffscreenTextureFormat(size, size, format)
		if err != nil {
			t.Fatalf("CreateOffscreenTextureFormat(%v): %v", format, err)
		}
		defer release()
		target := render.GPURenderTarget{
			View: view, ViewWidth: size, ViewHeight: size,
			Data: data, Width: size, Height: size, Stride: size * 4,
		}
		path := render.NewPath()
		path.Rectangle(0, 0, size, size)
		base := render.NewPaint()
		base.SetBrush(render.Solid(render.RGBA{R: 0.5, G: 0.5, B: 0.5, A: 1}))
		if err := rc.FillPath(target, path, base, render.Identity()); err != nil {
			t.Fatalf("base fill (%v): %v", format, err)
		}
		screen := render.NewPaint()
		screen.SetBrush(render.Solid(render.RGBA{R: 0.25, G: 0.25, B: 0.25, A: 1}))
		screen.BlendMode = render.BlendScreen
		screen.BlendSpace = render.BlendSpaceLinear
		if err := rc.FillPath(target, path, screen, render.Identity()); err != nil {
			t.Fatalf("screen fill (%v): %v", format, err)
		}
		if err := rc.Flush(target); err != nil {
			t.Fatalf("flush (%v): %v", format, err)
		}
		return p31bReadCenter(t, shared, view, size)
	}

	// Both targets receive the same linear-light advanced blend; the Unorm
	// leg re-encodes in the shader (P3.1), the sRGB leg lets the hardware
	// encode once. Screen(linear 0.5, linear 0.25) = 0.254 -> sRGB 138.
	unorm := run(types.TextureFormatRGBA8Unorm)
	for i := 0; i < 3; i++ {
		if d := int(unorm[i]) - 138; d < -3 || d > 3 {
			t.Fatalf("Unorm linear screen center=%v, want [138 138 138]±3", unorm)
		}
	}
	srgb := run(types.TextureFormatRGBA8UnormSrgb)
	for i := 0; i < 3; i++ {
		if d := int(srgb[i]) - 138; d < -3 || d > 3 {
			t.Fatalf("sRGB linear screen center=%v, want [138 138 138]±3", srgb)
		}
	}
}
