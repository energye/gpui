//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
)

// BenchmarkFillSweepFieldAA measures the AA-rect sweep-gradient field path
// end to end (C1). After the memoization fix a stable brush + stable
// geometry re-uses the previous field, so steady-state allocations drop to
// ~0 B/op (the first build pays once). Run with -benchmem.
func BenchmarkFillSweepFieldAA(b *testing.B) {
	shared := NewGPUShared()
	shared.mu.Lock()
	err := shared.ensureGPU()
	shared.mu.Unlock()
	if err != nil || !shared.gpuReady {
		b.Skipf("no GPU available: %v", err)
	}
	rc := &GPURenderContext{shared: shared}
	// Readback target: Data-backed offscreen surface (the same shape the
	// sweepgradient window's card raster exercises through the live path).
	data := make([]byte, 512*320*4)
	target := render.GPURenderTarget{
		Width: 512, Height: 320,
		Data: data, Stride: 512 * 4,
	}

	brush := render.NewSweepGradientBrush(0, 0, 0).
		AddColorStop(0, render.RGBA{R: 1, A: 1}).
		AddColorStop(0.5, render.RGBA{R: 0.2, G: 0.5, B: 1, A: 1}).
		AddColorStop(1, render.RGBA{R: 1, A: 1})
	paint := &render.Paint{}
	path := render.NewPath()
	path.MoveTo(40, 40)
	path.LineTo(440, 40)
	path.LineTo(440, 240)
	path.LineTo(40, 240)
	path.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := rc.fillColorAtFieldMaskedGPU(target, path, paint, brush.ColorAt, brushFieldSeedSweep(brush)); err != nil {
			b.Fatalf("fill: %v", err)
		}
	}
}
