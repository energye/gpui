//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// TestP43MaskPlaneStats pins the P4.3 measurement ledger: every masked draw
// rebuilds its mask plane today, so the counters must move per call; the CPU
// leg reports zeros.
func TestP43MaskPlaneStats(t *testing.T) {
	requireFilterOpsGPU(t)
	dc := render.NewContext(128, 128)
	defer dc.Close()
	for i := 0; i < 2; i++ {
		dc.PushMaskLayer(render.NewMask(64, 64))
		dc.SetRGBA(1, 0, 0, 1)
		dc.DrawRectangle(16, 16, 64, 64)
		_ = dc.Fill()
		dc.PopLayer()
		_ = dc.Image()
	}
	calls, allocs, bytes, _, hits := dc.MaskPlaneStats()
	if calls < 2 || allocs < 1 || bytes == 0 {
		t.Fatalf("mask plane ledger = calls %d allocs %d bytes %d, want >=2/>=1/>0", calls, allocs, bytes)
	}
	if hits == 0 {
		t.Fatalf("mask plane pool never hit (calls %d allocs %d): pooling is not wired", calls, allocs)
	}

	cpu := render.NewContext(32, 32, render.WithCPUOnly())
	defer cpu.Close()
	cpu.PushMaskLayer(render.NewMask(16, 16))
	cpu.SetRGBA(1, 0, 0, 1)
	cpu.DrawRectangle(4, 4, 8, 8)
	_ = cpu.Fill()
	cpu.PopLayer()
	if c, p, b, rb, h := cpu.MaskPlaneStats(); c != 0 || p != 0 || b != 0 || rb != 0 || h != 0 {
		t.Fatalf("CPU leg ledger = %d/%d/%d/%d/%d, want zeros", c, p, b, rb, h)
	}
}
