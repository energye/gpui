package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// TestP12_ClipRectDifferenceGPU verifies C.02/C.03: ClipOpDifference
// routes through GPU (GPUOps increases, cpu_fb=0) and pixels stay correct.
func TestP12_ClipRectDifferenceGPU(t *testing.T) {
	requireNativeGPU(t)

	const w, h = 64, 64
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush bg: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	// Full canvas clip then punch a hole — HasMaskClip, no gpuClipPath.
	dc.ClipRect(0, 0, w, h)
	dc.ClipRectOp(16, 16, 32, 32, render.ClipOpDifference)
	dc.SetRGB(0.9, 0.1, 0.1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush clipped: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("diff-clip %s base=%d", stats.LogLine(), base)
	if stats.GPUOps <= base {
		t.Fatalf("ClipOpDifference fill must use GPU: base=%d after=%d %s", base, stats.GPUOps, stats.LogLine())
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("clip-mask must not force CPU fallback: %s", stats.LogLine())
	}

	// Outside hole: red
	r, g, b, _ := sampleRGBA(dc, 4, 4)
	t.Logf("outside=%d,%d,%d", r, g, b)
	if r < 180 || g > 60 || b > 60 {
		t.Fatalf("outside hole expected red, got %d,%d,%d", r, g, b)
	}
	// Inside hole: white bg preserved
	r2, g2, b2, _ := sampleRGBA(dc, 32, 32)
	t.Logf("hole=%d,%d,%d", r2, g2, b2)
	if int(r2)+int(g2)+int(b2) < 700 {
		t.Fatalf("hole should stay white, got %d,%d,%d", r2, g2, b2)
	}
}

// TestP12_ClipPathDifferenceGPU punches a circular hole via path difference.
func TestP12_ClipPathDifferenceGPU(t *testing.T) {
	requireNativeGPU(t)

	const w, h = 48, 48
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	_ = dc.FlushGPU()
	base := dc.RenderPathStats().GPUOps

	dc.ClipRect(0, 0, w, h)
	dc.DrawCircle(24, 24, 10)
	dc.ClipPathOp(render.ClipOpDifference)
	dc.SetRGB(0, 0.2, 0.9)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	_ = dc.FlushGPU()
	stats := dc.RenderPathStats()
	t.Logf("path-diff %s base=%d", stats.LogLine(), base)
	if stats.GPUOps <= base {
		t.Fatalf("path difference must GPUOps: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("path difference cpu_fb: %s", stats.LogLine())
	}
	// Corner blue-ish
	r, g, b, _ := sampleRGBA(dc, 2, 2)
	t.Logf("corner=%d,%d,%d", r, g, b)
	if b < 150 {
		t.Fatalf("corner expected blue fill, got %d,%d,%d", r, g, b)
	}
	// Center hole white
	cr, cg, cb, _ := sampleRGBA(dc, 24, 24)
	t.Logf("center=%d,%d,%d", cr, cg, cb)
	if int(cr)+int(cg)+int(cb) < 650 {
		t.Fatalf("circle hole should stay light, got %d,%d,%d", cr, cg, cb)
	}
}
