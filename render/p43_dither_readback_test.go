//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0 (January 2004)
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import "testing"

// TestP43DitherReadbackCounters is the P4.3 dither measurement probe. It answers
// "does enabling SetDither force an extra GPU flush / CPU readback?" on the real
// GPU path with objective counters (RenderPathStats), not with a mean pixel
// value. Dither is a pixmap postprocess applied AFTER FlushGPU
// (m4_dither.go applyDitherIfEnabled, called from context_gpu.go:187); it must
// not add a flush or a CPU fallback. If this env has no real GPU path the test
// SKIPs with the reason instead of going green on a CPU-only run.
//
// Scope note (dither_contract_test.go A12 / RENDER_FEATURE_COVERAGE §6): dither
// has NO effect on the FlushGPUWithView / PresentWith window-present path —
// render.Canvas has no SetDither and live present frames are recording canvases,
// so a real window cannot enable dither on its main DC. This probe therefore
// measures the only path where dither actually runs (offscreen Context.Image()).
func TestP43DitherReadbackCounters(t *testing.T) {
	const w, h = 96, 96
	read := func(dither bool) RenderPathStats {
		dc := ditherGradientContext(w, h, dither)
		defer dc.Close()
		_ = dc.Image() // FlushGPU + optional dither pixmap postprocess
		return dc.RenderPathStats()
	}
	off, on := read(false), read(true)

	if off.GPUOps == 0 || on.GPUOps == 0 {
		t.Skipf("no real GPU path in this env (off gpu_ops=%d on gpu_ops=%d); run with WGPU_NATIVE_PATH/lib",
			off.GPUOps, on.GPUOps)
	}
	if on.FrameFlushes != off.FrameFlushes {
		t.Fatalf("dither changed frame_flushes: off=%d on=%d", off.FrameFlushes, on.FrameFlushes)
	}
	if on.CPUFallbackOps != off.CPUFallbackOps {
		t.Fatalf("dither changed cpu_fallback_ops: off=%d on=%d", off.CPUFallbackOps, on.CPUFallbackOps)
	}
	t.Logf("P43 dither readback: gpu_ops off=%d on=%d | frame_flushes off=%d on=%d | cpu_fallback off=%d on=%d",
		off.GPUOps, on.GPUOps, off.FrameFlushes, on.FrameFlushes, off.CPUFallbackOps, on.CPUFallbackOps)
}
