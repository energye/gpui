//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
)

// TestBackdropLayer4xNestedFilterBlursSeed pins the 4x MSAA path for a bounded
// backdrop layer nested inside an isolated layer: the filter must consume the
// seeded parent content instead of folding pending work into a pixmap that the
// active layer RT never materialized (mid-frame fold returns false).
func TestBackdropLayer4xNestedFilterBlursSeed(t *testing.T) {
	requireNativeGPU(t)
	render.SetMSAASampleCount(render.MSAASampleCount4)
	defer render.SetMSAASampleCount(0)

	const w, h = 64, 32
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.BeginFrame()
	dc.PushLayerIsolated(1)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.1, 0.1)
	dc.DrawRectangle(20, 0, 24, h)
	_ = dc.Fill()
	dc.SetRGB(0, 0, 0)
	dc.DrawRectangle(30, 12, 4, 8)
	_ = dc.Fill()

	dc.ClipRect(24, 0, 16, h)
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.ApplyBlur(3)
	dc.PopLayer()
	dc.ResetClip()
	dc.PopLayer()

	rgba := backdropPresentRGBA(t, dc, w, h)
	// Inside the bounds the blur must smear the black square into red.
	if got := backdropPixel(rgba, w, 32, 16); got[0] < 20 {
		t.Fatalf("inside 4x backdrop bounds = %v want blurred mix (blur did not run)", got)
	}
	// Outside the bounded layer the parent snapshot must stay untouched.
	if got := backdropPixel(rgba, w, 10, 16); got[0] < 250 || got[1] < 250 || got[2] < 250 {
		t.Fatalf("outside 4x backdrop bounds = %v want parent white", got)
	}
}

// TestBackdropLayer4xPostFilterTintKeepsSeed pins the B-card regression: after
// a backdrop blur seeds the layer RT, the following tint fill must load that
// seed instead of clearing it through the base 4x MSAA attachment. A single 4x
// context clears the 1x seed on the tint pass and leaves the sharp source bars
// (black + 15% white ≈ 38), while the 1x layer target keeps the blurred mix
// (≈ 147).
func TestBackdropLayer4xPostFilterTintKeepsSeed(t *testing.T) {
	requireNativeGPU(t)
	render.SetMSAASampleCount(render.MSAASampleCount4)
	defer render.SetMSAASampleCount(0)

	const w, h = 64, 32
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.BeginFrame()
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0, 0, 0)
	for x := 20.0; x < 44; x += 10 {
		dc.DrawRectangle(x, 0, 5, h)
		_ = dc.Fill()
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush base: %v", err)
	}

	dc.ClipRect(20, 0, 24, h)
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.ApplyBlur(6)
	dc.SetRGBA(1, 1, 1, 0.15)
	dc.DrawRectangle(20, 0, 24, h)
	_ = dc.Fill()
	dc.PopLayer()
	dc.ResetClip()

	rgba := backdropPresentRGBA(t, dc, w, h)
	// At the black bar center the blur must leave a gray mix, then the 15% tint
	// lifts it to ~147. The seed-lost path keeps black and lands near 38.
	for _, x := range []int{32, 37} {
		// Both the black-bar center and the white-gap center must land in the
		// blurred gray band. 90..210 rejects the sharp black (≈38) and sharp
		// white (≈255) bars on either side.
		if got := backdropPixel(rgba, w, x, 16); got[0] < 90 || got[1] < 90 || got[2] < 90 || got[0] > 210 || got[1] > 210 || got[2] > 210 {
			t.Fatalf("inside tinted 4x backdrop @x=%d = %v want blurred seed (~147), not sharp bar+tint", x, got)
		}
	}
	// Outside the bounded layer the parent snapshot must stay untouched.
	if got := backdropPixel(rgba, w, 10, 16); got[0] < 250 || got[1] < 250 || got[2] < 250 {
		t.Fatalf("outside tinted 4x backdrop = %v want parent white", got)
	}
}
