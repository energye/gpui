//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/text"
)

// These regressions pin the five misc APIs to the GPU path: every call must
// queue real GPU work and must not silently fall back to CPU. Pixel checks use
// the readback image, not just the path counters.
func TestMiscDrawGPUPath(t *testing.T) {
	requireNativeGPU(t)

	t.Run("DrawColor", func(t *testing.T) {
		dc := render.NewContext(160, 140)
		defer dc.Close()
		dc.ClearWithColor(render.RGBA{R: 1, A: 1})
		dc.ClipRect(10, 10, 100, 100)
		dc.DrawColor(render.RGBA{B: 1, A: 1}, render.BlendNormal)
		img := dc.Image()
		if r, g, b, a := img.At(20, 20).RGBA(); r>>8 != 0 || g>>8 != 0 || b>>8 != 255 || a>>8 != 255 {
			t.Fatalf("inside clip = (%d,%d,%d,%d), want opaque blue", r>>8, g>>8, b>>8, a>>8)
		}
		if r, g, b, a := img.At(5, 5).RGBA(); r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
			t.Fatalf("outside clip = (%d,%d,%d,%d), want opaque red", r>>8, g>>8, b>>8, a>>8)
		}
		st := dc.RenderPathStats()
		if st.GPUOps == 0 {
			t.Fatalf("DrawColor GPUOps==0: %s", st.LogLine())
		}
		if st.CPUFallbackOps != 0 {
			t.Fatalf("DrawColor CPU fallback: %s", st.LogLine())
		}
	})

	t.Run("DrawColorAdvancedBlend", func(t *testing.T) {
		dc := render.NewContext(40, 30)
		defer dc.Close()
		dc.ClearWithColor(render.RGBA{R: 1, A: 1})
		dc.ClipRect(5, 5, 20, 15)
		dc.DrawColor(render.RGBA{R: 0.2, G: 0.3, B: 0.8, A: 1}, render.BlendScreen)
		img := dc.Image()
		if r, g, b, a := img.At(10, 10).RGBA(); r>>8 != 255 || g>>8 < 73 || g>>8 > 79 || b>>8 < 201 || b>>8 > 207 || a>>8 != 255 {
			t.Fatalf("screen blend = (%d,%d,%d,%d), want approx (255,76,204,255)", r>>8, g>>8, b>>8, a>>8)
		}
		if r, g, b, a := img.At(2, 2).RGBA(); r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
			t.Fatalf("outside clip = (%d,%d,%d,%d), want red", r>>8, g>>8, b>>8, a>>8)
		}
		st := dc.RenderPathStats()
		if st.GPUOps == 0 || st.CPUFallbackOps != 0 {
			t.Fatalf("DrawColor advanced blend stats: %s", st.LogLine())
		}
	})

	// Two advanced DrawColor calls in a row: the first PopLayer must stay
	// queued for the base surface while the second layer self-flushes. Before
	// destination tracking, the first Multiply pop was resolved into the second
	// layer's RT and vanished from the final Screen composite.
	t.Run("DrawColorTwoAdvancedLayers", func(t *testing.T) {
		dc := render.NewContext(120, 90)
		defer dc.Close()
		dc.ClearWithColor(render.RGBA{R: 1, G: 0.12, B: 0.10, A: 1})
		dc.ClipRect(0, 5, 120, 20)
		dc.DrawColor(render.RGBA{R: 0.9, G: 0.9, B: 0.9, A: 1}, render.BlendMultiply)
		dc.ResetClip()
		dc.ClipRect(0, 60, 120, 20)
		dc.DrawColor(render.RGBA{R: 0.2, G: 0.3, B: 0.8, A: 1}, render.BlendScreen)
		dc.ResetClip()
		img := dc.Image()
		if r, g, b, a := img.At(40, 15).RGBA(); r>>8 < 225 || r>>8 > 233 || g>>8 < 24 || g>>8 > 32 || b>>8 < 18 || b>>8 > 26 || a>>8 != 255 {
			t.Fatalf("multiply overlay = (%d,%d,%d,%d), want approx (229,28,22,255)", r>>8, g>>8, b>>8, a>>8)
		}
		if r, g, b, a := img.At(40, 70).RGBA(); r>>8 != 255 || g>>8 < 95 || g>>8 > 101 || b>>8 < 206 || b>>8 > 212 || a>>8 != 255 {
			t.Fatalf("screen overlay = (%d,%d,%d,%d), want approx (255,98,209,255)", r>>8, g>>8, b>>8, a>>8)
		}
		if r, g, b, a := img.At(40, 45).RGBA(); r>>8 != 255 || g>>8 < 28 || g>>8 > 32 || b>>8 < 23 || b>>8 > 27 || a>>8 != 255 {
			t.Fatalf("untouched band = (%d,%d,%d,%d), want approx base red", r>>8, g>>8, b>>8, a>>8)
		}
		st := dc.RenderPathStats()
		if st.GPUOps == 0 || st.CPUFallbackOps != 0 {
			t.Fatalf("sequential advanced stats: %s", st.LogLine())
		}
	})

	// Nested isolation: a Multiply DrawColor inside PushLayerIsolated must
	// resolve against the enclosing layer RT, not the base surface. When the
	// parent has no pending draw commands, PopLayer must still flush the
	// advanced child before releasing that RT.
	t.Run("DrawColorInsideIsolatedLayer", func(t *testing.T) {
		dc := render.NewContext(120, 80)
		defer dc.Close()
		dc.ClearWithColor(render.RGBA{R: 1, G: 0, B: 0, A: 1})
		dc.PushLayerIsolated(1)
		dc.ClipRect(10, 10, 80, 40)
		dc.DrawColor(render.RGBA{R: 0.9, G: 0.9, B: 0.9, A: 1}, render.BlendMultiply)
		dc.ResetClip()
		dc.PopLayer()
		img := dc.Image()
		if r, g, b, a := img.At(20, 20).RGBA(); r>>8 < 225 || r>>8 > 233 || g>>8 < 225 || g>>8 > 233 || b>>8 < 225 || b>>8 > 233 || a>>8 != 255 {
			t.Fatalf("nested multiply = (%d,%d,%d,%d), want approx (229,229,229,255)", r>>8, g>>8, b>>8, a>>8)
		}
		if r, g, b, a := img.At(100, 60).RGBA(); r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
			t.Fatalf("outside isolated layer = (%d,%d,%d,%d), want red", r>>8, g>>8, b>>8, a>>8)
		}
		st := dc.RenderPathStats()
		if st.GPUOps == 0 || st.CPUFallbackOps != 0 {
			t.Fatalf("nested advanced stats: %s", st.LogLine())
		}
	})

	// Effect surface: when the only pending work is an advanced DrawColor,
	// FlushGPU must still publish the effect RT and resolve the base-destined
	// pop into it. Keeping the pop for the (never flushed) base view used to
	// leave GPUFilterTexture empty or stale.
	t.Run("DrawColorEffectSurfaceAdvancedBlend", func(t *testing.T) {
		dc := render.NewContext(64, 48)
		defer dc.Close()
		dc.SetEffectSurface(true)
		dc.ClearWithColor(render.RGBA{R: 1, A: 1})
		dc.ClipRect(8, 8, 32, 20)
		dc.DrawColor(render.RGBA{R: 0.9, G: 0.9, B: 0.9, A: 1}, render.BlendMultiply)
		dc.ResetClip()
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
		view, w, h, ok := dc.GPUFilterTexture()
		if !ok || view.IsNil() {
			t.Fatalf("effect texture missing after advanced DrawColor: ok=%v nil=%v", ok, view.IsNil())
		}
		dst := render.NewContext(64, 48)
		defer dst.Close()
		dst.DrawGPUTexture(view, 0, 0, w, h)
		img := dst.Image()
		if r, g, b, a := img.At(16, 16).RGBA(); r>>8 < 225 || r>>8 > 233 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
			t.Fatalf("effect multiply = (%d,%d,%d,%d), want approx (229,0,0,255)", r>>8, g>>8, b>>8, a>>8)
		}
		if r, g, b, a := img.At(2, 2).RGBA(); r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
			t.Fatalf("outside clip = (%d,%d,%d,%d), want red", r>>8, g>>8, b>>8, a>>8)
		}
		st := dc.RenderPathStats()
		if st.GPUOps == 0 || st.CPUFallbackOps != 0 {
			t.Fatalf("effect advanced stats: %s", st.LogLine())
		}
	})

	t.Run("DrawPaint", func(t *testing.T) {
		dc := render.NewContext(160, 140)
		defer dc.Close()
		dc.ClearWithColor(render.RGBA{R: 1, A: 1})
		brush := render.NewLinearGradientBrush(10, 0, 110, 0)
		brush.AddColorStop(0, render.RGBA{R: 1, A: 1})
		brush.AddColorStop(1, render.RGBA{B: 1, A: 1})
		dc.SetFillBrush(brush)
		dc.ClipRect(10, 10, 100, 100)
		dc.DrawPaint()
		img := dc.Image()
		lr, lg, lb, la := img.At(20, 20).RGBA()
		rr, rg, rb, ra := img.At(100, 20).RGBA()
		if la>>8 != 255 || lr <= rr || lb >= rb {
			t.Fatalf("gradient left/right not red->blue: left=(%d,%d,%d,%d) right=(%d,%d,%d,%d)",
				lr>>8, lg>>8, lb>>8, la>>8, rr>>8, rg>>8, rb>>8, ra>>8)
		}
		if r, g, b, a := img.At(5, 5).RGBA(); r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
			t.Fatalf("outside clip = (%d,%d,%d,%d), want opaque red", r>>8, g>>8, b>>8, a>>8)
		}
		st := dc.RenderPathStats()
		if st.GPUOps == 0 || st.CPUFallbackOps != 0 {
			t.Fatalf("DrawPaint stats: %s", st.LogLine())
		}
	})

	t.Run("DrawShadow", func(t *testing.T) {
		dc := render.NewContext(160, 140)
		defer dc.Close()
		dc.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})
		path := render.NewPath()
		path.MoveTo(30, 30)
		path.LineTo(100, 30)
		path.LineTo(100, 80)
		path.LineTo(30, 80)
		path.Close()
		dc.DrawShadow(path, render.RGBA{A: 1}, 10, false)
		img := dc.Image()
		if r, g, b, a := img.At(65, 12).RGBA(); r>>8 != 255 || g>>8 != 255 || b>>8 != 255 || a>>8 != 255 {
			t.Fatalf("above path = (%d,%d,%d,%d), want white", r>>8, g>>8, b>>8, a>>8)
		}
		r, g, b, _ := img.At(65, 90).RGBA()
		if r>>8 > 245 || g>>8 > 245 || b>>8 > 245 || r>>8 < 150 {
			t.Fatalf("below path = (%d,%d,%d), want visible soft shadow", r>>8, g>>8, b>>8)
		}
		st := dc.RenderPathStats()
		if st.GPUOps == 0 || st.CPUFallbackOps != 0 {
			t.Fatalf("DrawShadow stats: %s", st.LogLine())
		}
	})

	t.Run("DrawParagraph", func(t *testing.T) {
		dc := render.NewContext(240, 160)
		defer dc.Close()
		dc.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})
		dc.SetRGBA(0, 0, 0, 1)
		source, err := text.NewFontSourceFromFile("text/testdata/source-sans/TTF/SourceSans3-Regular.ttf")
		if err != nil {
			t.Skipf("test font unavailable: %v", err)
		}
		layout := text.LayoutText("HH\nHH", source.Face(28), text.LayoutOptions{LineSpacing: 2.0})
		dc.DrawParagraph(layout, 20, 20)
		img := dc.Image()
		ink := 0
		for y := 0; y < 160; y++ {
			for x := 0; x < 240; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
					ink++
				}
			}
		}
		if ink == 0 {
			t.Fatal("DrawParagraph produced no visible ink")
		}
		st := dc.RenderPathStats()
		if st.GPUOps == 0 || st.CPUFallbackOps != 0 {
			t.Fatalf("DrawParagraph stats: %s", st.LogLine())
		}
	})
}
