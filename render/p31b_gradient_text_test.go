//go:build !nogpu

package render_test

import (
	"testing"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"

	_ "github.com/energye/gpui/render/gpu"
)

// TestP31bSrgbTargetGradientText pins two more shader families on the opt-in
// sRGB target: the gradient ramp cover shader and the glyph text shader.
//
// Both must decode their sRGB-domain source (stops / text color) to linear
// light and let the hardware encode on write. A missing decode is not
// detectable with a single opaque fill (identity round-trip), so the test
// compares the sRGB target against the Unorm control: at full coverage the
// same source color must land on the same visible sRGB bytes.
func TestP31bSrgbTargetGradientText(t *testing.T) {
	const w, h = 256, 160

	src, err := text.NewFontSourceFromFile("text/testdata/ahem.ttf")
	if err != nil {
		t.Fatalf("Ahem font: %v", err)
	}
	face := src.Face(64)

	renderScene := func(format types.TextureFormat) ([]byte, render.RenderPathStats) {
		dc := render.NewContext(w, h)
		defer dc.Close()
		// Ahem 'A' is a solid em square, so (32,108) gives full coverage and
		// the interior color is the exact text color on either target.
		grad := render.NewLinearGradientBrush(0, 0, w, 0)
		grad.AddColorStop(0, render.RGBA{R: 1, A: 1})
		grad.AddColorStop(1, render.RGBA{B: 1, A: 1})
		dc.SetFillBrush(grad)
		dc.DrawRectangle(0, 0, w, 64)
		if err := dc.Fill(); err != nil {
			t.Fatalf("gradient fill (%v): %v", format, err)
		}
		dc.SetFont(face)
		dc.SetRGBA(0.5, 0.25, 0.1, 1)
		dc.DrawString("A", 8, 140)

		view, release, err := dc.CreateOffscreenTextureFormat(w, h, format)
		if err != nil {
			t.Fatalf("offscreen (%v): %v", format, err)
		}
		defer release()
		if err := dc.FlushGPUWithView(view, w, h); err != nil {
			t.Fatalf("flush (%v): %v", format, err)
		}
		rb := dc.GPURenderContext().(interface {
			ReadbackViewStraightRGBA(view gpucontext.TextureView, w, h int) ([]byte, error)
		})
		rgba, err := rb.ReadbackViewStraightRGBA(view, w, h)
		if err != nil {
			t.Fatalf("readback (%v): %v", format, err)
		}
		stats := dc.RenderPathStats()
		if stats.GPUOps == 0 || stats.CPUFallbackOps != 0 {
			t.Fatalf("format %v stats=%+v, want GPUOps>0 CPUFallbackOps=0", format, stats)
		}
		return rgba, stats
	}

	unorm, _ := renderScene(types.TextureFormatRGBA8Unorm)
	srgb, _ := renderScene(types.TextureFormatRGBA8UnormSrgb)

	pixel := func(buf []byte, x, y int) [4]byte {
		off := (y*w + x) * 4
		var p [4]byte
		copy(p[:], buf[off:off+4])
		return p
	}
	// Gradient midpoint and the Ahem glyph interior must be identical on both
	// targets (same visible sRGB bytes), within the ±3/255 P3.1b tolerance.
	for _, sample := range []struct {
		name string
		x, y int
	}{
		{"gradient-left", 32, 32},
		{"gradient-mid", 128, 32},
		{"gradient-right", 224, 32},
		{"ahem-interior", 32, 108},
	} {
		a := pixel(unorm, sample.x, sample.y)
		b := pixel(srgb, sample.x, sample.y)
		for i := 0; i < 3; i++ {
			if d := int(a[i]) - int(b[i]); d < -3 || d > 3 {
				t.Fatalf("%s unorm=%v srgb=%v channel %d delta %d > 3", sample.name, a, b, i, d)
			}
		}
	}
	// The text interior is the exact source color; a double decode on the
	// sRGB target would read near linear (55,13,3) instead of sRGB (128,64,26).
	p31bCheckColor(t, "ahem interior sRGB", pixel(srgb, 32, 108), [3]int{128, 64, 26})
}
