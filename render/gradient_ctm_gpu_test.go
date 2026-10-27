//go:build !nogpu

package render_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
)

// GPU counterpart of gradient_ctm_test.go: the same brush-follows-CTM contract
// must hold when the fill is served by the GPU kernels (span/ramp/field), not
// only by the software rasterizer.

func gradientCTMGpuLinear() *render.LinearGradientBrush {
	g := render.NewLinearGradientBrush(0, 0, 100, 0)
	g.AddColorStop(0, render.RGBA{R: 0, G: 0, B: 0, A: 1})
	g.AddColorStop(1, render.RGBA{R: 1, G: 0, B: 0, A: 1})
	return g
}

func gradientCTMGpuFill(t *testing.T, dc *render.Context, x, y, w, h float64) {
	t.Helper()
	dc.DrawRectangle(x, y, w, h)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	if stats.GPUOps == 0 {
		t.Fatalf("gradient fill did not reach the GPU: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps != 0 {
		t.Fatalf("gradient fill fell back to CPU: %s", stats.LogLine())
	}
}

func gradientCTMGpuPixel(t *testing.T, dc *render.Context, x, y int) (r, g, b, a uint8) {
	t.Helper()
	return s3cSample(dc, x, y)
}

func gradientCTMGpuClose(t *testing.T, gotR, gotG, gotB, wantR, wantG, wantB uint8, tol int, what string) {
	t.Helper()
	if d := int(gotR) - int(wantR); d < -tol || d > tol {
		t.Errorf("%s R = %d, want %d ±%d", what, gotR, wantR, tol)
	}
	if d := int(gotG) - int(wantG); d < -tol || d > tol {
		t.Errorf("%s G = %d, want %d ±%d", what, gotG, wantG, tol)
	}
	if d := int(gotB) - int(wantB); d < -tol || d > tol {
		t.Errorf("%s B = %d, want %d ±%d", what, gotB, wantB, tol)
	}
}

func TestGradientFollowsCTM_GPU(t *testing.T) {
	s3cRequireGPU(t)
	const tol = 3

	t.Run("translate", func(t *testing.T) {
		base := render.NewContext(220, 100)
		defer base.Close()
		base.SetFillBrush(gradientCTMGpuLinear())
		gradientCTMGpuFill(t, base, 0, 0, 100, 100)

		moved := render.NewContext(220, 100)
		defer moved.Close()
		moved.Translate(100, 0)
		moved.SetFillBrush(gradientCTMGpuLinear())
		gradientCTMGpuFill(t, moved, 0, 0, 100, 100)

		for _, x := range []int{1, 25, 50, 75, 98} {
			br, bg, bb, _ := gradientCTMGpuPixel(t, base, x, 50)
			gr, gg, gb, _ := gradientCTMGpuPixel(t, moved, x+100, 50)
			gradientCTMGpuClose(t, gr, gg, gb, br, bg, bb, tol, "translated gradient")
		}
		if _, _, _, a := gradientCTMGpuPixel(t, moved, 50, 50); a != 0 {
			t.Errorf("left of translated rect alpha = %d, want 0", a)
		}
	})

	t.Run("rotate90", func(t *testing.T) {
		base := render.NewContext(100, 100)
		defer base.Close()
		base.SetFillBrush(gradientCTMGpuLinear())
		gradientCTMGpuFill(t, base, 0, 0, 100, 100)

		rot := render.NewContext(100, 100)
		defer rot.Close()
		rot.Translate(100, 0)
		rot.Rotate(math.Pi / 2)
		rot.SetFillBrush(gradientCTMGpuLinear())
		gradientCTMGpuFill(t, rot, 0, 0, 100, 100)

		for _, x := range []int{1, 25, 50, 75, 98} {
			br, bg, bb, _ := gradientCTMGpuPixel(t, base, x, 50)
			gr, gg, gb, _ := gradientCTMGpuPixel(t, rot, 50, x)
			gradientCTMGpuClose(t, gr, gg, gb, br, bg, bb, tol, "rotated gradient")
		}
	})

	t.Run("uniform_scale", func(t *testing.T) {
		base := render.NewContext(220, 220)
		defer base.Close()
		base.SetFillBrush(gradientCTMGpuLinear())
		gradientCTMGpuFill(t, base, 0, 0, 100, 100)

		scaled := render.NewContext(220, 220)
		defer scaled.Close()
		scaled.Scale(2, 2)
		scaled.SetFillBrush(gradientCTMGpuLinear())
		gradientCTMGpuFill(t, scaled, 0, 0, 100, 100)

		// Device pixel 2x has its center at 2x+0.5, not at 2(x+0.5); the
		// half-pixel offset is amplified by the sRGB encode near black, so the
		// near-zero probe (x=1) is not comparable this way.
		for _, x := range []int{25, 50, 75, 98} {
			br, bg, bb, _ := gradientCTMGpuPixel(t, base, x, 50)
			gr, gg, gb, _ := gradientCTMGpuPixel(t, scaled, 2*x, 100)
			gradientCTMGpuClose(t, gr, gg, gb, br, bg, bb, tol, "uniform-scaled gradient")
		}
	})
}

// TestGradientFollowsCTM_GPU_NonUniform pins the inverse-sampling fallback on
// the GPU: a non-uniform scale cannot be baked into gradient geometry, so the
// ColorAt field must sample the base brush through the inverse matrix.
func TestGradientFollowsCTM_GPU_NonUniform(t *testing.T) {
	s3cRequireGPU(t)
	dc := render.NewContext(200, 100)
	defer dc.Close()

	brush := render.NewLinearGradientBrush(0, 0, 100, 100)
	brush.AddColorStop(0, render.RGBA{R: 0, G: 0, B: 0, A: 1})
	brush.AddColorStop(1, render.RGBA{R: 0, G: 0, B: 1, A: 1})

	dc.Scale(2, 1)
	dc.SetFillBrush(brush)
	gradientCTMGpuFill(t, dc, 0, 0, 100, 100)

	inv := render.Scale(2, 1).Invert()
	for _, x := range []int{10, 60, 100, 150, 190} {
		u := inv.TransformPoint(render.Pt(float64(x)+0.5, 40.5))
		want := brush.ColorAt(u.X, u.Y)
		// The GPU field samples the same ColorAt in the same encoded space;
		// allow one quantization step plus rounding.
		_, _, gotB, _ := gradientCTMGpuPixel(t, dc, x, 40)
		if d := float64(gotB)/255 - want.B; d < -0.01 || d > 0.01 {
			t.Errorf("non-uniform GPU pixel %d B = %.4f, want %.4f", x, float64(gotB)/255, want.B)
		}
	}
}
