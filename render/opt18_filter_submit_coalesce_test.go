//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// TestOpt18_ApplyBlur_MeshSeedGPUFilter keeps GPU filter publish + no CPU
// fallback after mesh seed coalesce (opt18 single Queue.Submit path).
func TestOpt18_ApplyBlur_MeshSeedGPUFilter(t *testing.T) {
	requireR72GPU(t)
	if !render.GPUFilterGraphRegistered() {
		dc0 := render.NewContext(8, 8)
		dc0.SetRGB(1, 0, 0)
		dc0.DrawRectangle(0, 0, 8, 8)
		_ = dc0.Fill()
		_ = dc0.FlushGPU()
		_ = dc0.Close()
	}
	if !render.GPUFilterGraphRegistered() {
		t.Skip("GPU filter graph not registered")
	}

	dc := render.NewContext(96, 72)
	defer dc.Close()
	dc.SetEffectSurface(true)
	dc.ClearWithColor(render.RGBA{R: 0.05, G: 0.05, B: 0.08, A: 1})
	dc.SetRGB(0.1, 0.2, 0.9)
	// Pending draws before blur — exercises FlushAndFilterFromView coalesce.
	dc.DrawRectangle(8, 8, 40, 30)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.4, 0.1)
	dc.DrawCircle(60, 40, 18)
	_ = dc.Fill()
	base := dc.RenderPathStats().GPUOps
	dc.ApplyBlur(2)

	view, w, h, ok := dc.GPUFilterTexture()
	if !ok || view.IsNil() || w <= 0 || h <= 0 {
		t.Fatalf("expected GPUFilterTexture after ApplyBlur, ok=%v %dx%d nil=%v stats=%s",
			ok, w, h, view.IsNil(), dc.RenderPathStats().LogLine())
	}
	st := dc.RenderPathStats()
	if st.GPUOps <= base {
		t.Fatalf("expected GPUOps increase, base=%d after=%s", base, st.LogLine())
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fallback must be 0: %s", st.LogLine())
	}
	t.Logf("opt18 blur %s gpu_tex=%dx%d", st.LogLine(), w, h)
}
