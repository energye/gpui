//go:build !nogpu

package render_test

import (
	"image"
	"math"
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// conicQuarterDiskPath builds a quarter disk whose arc is a single conic with
// weight sqrt(2)/2 (Skia's exact quarter-circle weight).
func conicQuarterDiskPath(cx, cy, r float64) *render.Path {
	p := render.NewPath()
	p.MoveTo(cx+r, cy)
	p.ConicTo(cx+r, cy-r, cx, cy-r, math.Sqrt2/2)
	p.LineTo(cx, cy)
	p.Close()
	return p
}

func renderConicQuarterDisk(t *testing.T, size int) image.Image {
	t.Helper()
	dc := render.NewContext(size, size)
	defer func() { _ = dc.Close() }()

	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 0, 0)
	dc.DrawPath(conicQuarterDiskPath(10, float64(size)-10, 100))
	dc.Fill()
	return dc.Image()
}

// TestConicGPUMatchesCPU proves the batch-1 boundary expansion is identical on
// both rasterization paths: the GPU never sees a conic verb (deviceSpacePath
// expands it up front), and its geometry must match the CPU path that expands
// at SoftwareRenderer.Fill. Channel equality is asserted on fully settled
// pixels; the AA edge band is measured and bounded separately because the two
// rasterizers own edge coverage independently.
func TestConicGPUMatchesCPU(t *testing.T) {
	requireNativeGPU(t)

	restore := func() {}
	if old, ok := os.LookupEnv("GOGPU_RENDER_MODE"); ok {
		restore = func() { _ = os.Setenv("GOGPU_RENDER_MODE", old) }
	} else {
		restore = func() { _ = os.Unsetenv("GOGPU_RENDER_MODE") }
	}
	defer restore()
	_ = os.Unsetenv("GOGPU_RENDER_MODE")

	const size = 160
	// GPU pass: assert the accelerator really handled it, not a silent CPU fallback.
	dc := render.NewContext(size, size)
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 0, 0)
	dc.DrawPath(conicQuarterDiskPath(10, float64(size)-10, 100))
	dc.Fill()
	gpuImg := dc.Image()
	st := dc.RenderPathStats()
	_ = dc.Close()
	if st.GPUOps == 0 {
		t.Fatalf("conic GPU pass took no GPU ops: %s", st.LogLine())
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("conic GPU pass fell back to CPU: %s", st.LogLine())
	}

	// CPU pass.
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	cpuImg := renderConicQuarterDisk(t, size)

	b := gpuImg.Bounds()
	settledWorst, unsettled, mismatch := 0, 0, 0
	gpuInside, cpuInside := 0, 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gr, gg, gb, ga := gpuImg.At(x, y).RGBA()
			cr, cg, cb, ca := cpuImg.At(x, y).RGBA()
			gG, cG := int(gg>>8), int(cg>>8)
			if gG < 128 {
				gpuInside++
			}
			if cG < 128 {
				cpuInside++
			}
			// GPU and CPU are independent AA rasterizers: pixels partially
			// covered by the arc edge legitimately differ in coverage. Compare
			// only fully settled interior/exterior pixels (G near 0 or 255);
			// shape agreement is pinned separately by the inside-area count.
			gSettled := gG < 2 || gG > 253
			cSettled := cG < 2 || cG > 253
			if !gSettled || !cSettled {
				unsettled++
				continue
			}
			if (gG < 2) != (cG < 2) {
				mismatch++
				continue
			}
			for _, d := range []int{
				conicChannelDiff(gr, cr), conicChannelDiff(gg, cg),
				conicChannelDiff(gb, cb), conicChannelDiff(ga, ca),
			} {
				if d > settledWorst {
					settledWorst = d
				}
			}
		}
	}
	total := b.Dx() * b.Dy()
	// Declared tolerances (measured 2026-11-01 on the native backend):
	// settled channel diff 1, edge band 174/25600 = 0.68%, inside-count delta
	// 2/25600 = 0.0078%. Gates below leave modest headroom for driver AA drift.
	if settledWorst > 1 {
		t.Errorf("settled GPU vs CPU max channel diff = %d, want <=1", settledWorst)
	}
	if mismatch != 0 {
		t.Errorf("settled red/white classification mismatch = %d, want 0", mismatch)
	}
	if limit := total / 100; unsettled > limit {
		t.Errorf("unsettled edge band = %d px, want <= %d (1%%)", unsettled, limit)
	}
	if delta := gpuInside - cpuInside; delta < -total/1000 || delta > total/1000 {
		t.Errorf("inside-pixel delta GPU-CPU = %d, want within ±%d (0.1%%)", delta, total/1000)
	}
}

func conicChannelDiff(a, b uint32) int {
	if a > b {
		return int((a - b) >> 8)
	}
	return int((b - a) >> 8)
}
