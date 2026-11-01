//go:build !nogpu

package render_test

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

func strokeSceneDraw(t *testing.T, withEffect bool) *render.Context {
	t.Helper()
	dc := render.NewContext(240, 140)
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 0, 0)
	st := render.DefaultStroke().WithWidth(6).WithCap(render.LineCapRound)
	if withEffect {
		// Corner effect on a zigzag: expands to a visibly different outline.
		st = st.WithPathEffect(render.NewCornerPathEffect(14))
	}
	dc.SetStroke(st)
	p := render.NewPath()
	p.MoveTo(20, 110)
	p.LineTo(80, 30)
	p.LineTo(140, 110)
	p.LineTo(200, 30)
	dc.DrawPath(p)
	if err := dc.Stroke(); err != nil {
		t.Fatalf("Stroke: %v", err)
	}
	return dc
}

func setRenderMode(t *testing.T, mode string) func() {
	t.Helper()
	orig, had := os.LookupEnv("GOGPU_RENDER_MODE")
	if mode == "" {
		_ = os.Unsetenv("GOGPU_RENDER_MODE")
	} else {
		_ = os.Setenv("GOGPU_RENDER_MODE", mode)
	}
	return func() {
		if had {
			_ = os.Setenv("GOGPU_RENDER_MODE", orig)
		} else {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		}
	}
}

// TestPathEffectGPUStrokeParity proves the batch-4 mount really reaches the
// GPU geometry path (no silent CPU fallback) and lands within the shared
// rasterizer tolerance of the CPU twin.
func TestPathEffectGPUStrokeParity(t *testing.T) {
	requireNativeGPU(t)

	restore := setRenderMode(t, "")
	gpuDC := strokeSceneDraw(t, true)
	err := gpuDC.FlushGPU()
	if err != nil {
		gpuDC.Close()
		t.Skipf("GPU flush unavailable: %v", err)
	}
	stats := gpuDC.RenderPathStats()
	gpuImg := gpuDC.Image()
	gpuDC.Close()
	if stats.GPUOps == 0 {
		t.Fatalf("effect stroke took no GPU ops: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps != 0 {
		t.Fatalf("effect stroke fell back to CPU: %s", stats.LogLine())
	}
	restore()

	restore = setRenderMode(t, "cpu")
	cpuDC := strokeSceneDraw(t, true)
	_ = cpuDC.FlushGPU()
	cpuImg := cpuDC.Image()
	cpuDC.Close()
	restore()

	st, err := imagediff.Images(gpuImg, cpuImg)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	total := st.TotalPixels
	changedPct := 100 * float64(st.ChangedPixels) / float64(total)
	t.Logf("effect GPU vs CPU: changed %d/%d (%.3f%%) mean %.3f max %d %s",
		st.ChangedPixels, total, changedPct, st.MeanAbs, st.MaxDelta, stats.LogLine())

	// GPU and CPU are independent AA rasterizers: only fully settled pixels
	// (pure red or pure white on both sides) must classify identically; the
	// edge band is bounded separately. Same discipline as the conic parity.
	settledMismatch, unsettled := 0, 0
	gpuIn, cpuIn := 0, 0
	b := gpuImg.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gr, _, _, _ := gpuImg.At(x, y).RGBA()
			cr, _, _, _ := cpuImg.At(x, y).RGBA()
			gG, cG := int(gr>>8), int(cr>>8)
			if gG > 128 {
				gpuIn++
			}
			if cG > 128 {
				cpuIn++
			}
			gSettled := gG < 2 || gG > 253
			cSettled := cG < 2 || cG > 253
			if !gSettled || !cSettled {
				unsettled++
				continue
			}
			if (gG > 128) != (cG > 128) {
				settledMismatch++
			}
		}
	}
	if settledMismatch != 0 {
		t.Fatalf("settled GPU vs CPU classification mismatch = %d, want 0", settledMismatch)
	}
	if limit := total / 10; unsettled > limit {
		t.Fatalf("unsettled AA band = %d px, want <= %d (10%%)", unsettled, limit)
	}
	if delta := gpuIn - cpuIn; delta < -total/200 || delta > total/200 {
		t.Fatalf("inside-pixel delta GPU-CPU = %d, want within ±%d (0.5%%)", delta, total/200)
	}

	// Discrimination: the same scene without the effect must differ from the
	// effected GPU render (a silently ignored effect would draw the plain one).
	restore = setRenderMode(t, "")
	plainDC := strokeSceneDraw(t, false)
	if err := plainDC.FlushGPU(); err != nil {
		plainDC.Close()
		t.Skipf("GPU flush unavailable: %v", err)
	}
	plainStats := plainDC.RenderPathStats()
	plainImg := plainDC.Image()
	plainDC.Close()
	restore()
	if plainStats.GPUOps == 0 {
		t.Fatalf("plain stroke took no GPU ops: %s", plainStats.LogLine())
	}
	diff, err := imagediff.Images(gpuImg, plainImg)
	if err != nil {
		t.Fatalf("diff plain: %v", err)
	}
	// A corner effect only reshapes the outline near the polyline vertices,
	// so the expected ink difference is local — any real difference at all
	// rules out a silently ignored effect (an ignored one yields exactly 0).
	if diff.ChangedPixels < 100 {
		t.Fatalf("effect changed only %d/%d px — effect silently ignored?", diff.ChangedPixels, diff.TotalPixels)
	}
}
