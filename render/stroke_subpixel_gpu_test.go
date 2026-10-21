//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
)

// TestStrokeSubpixelWidthGPU pins the GPU stroke chain to Skia/Flutter
// semantics: 0.5px keeps half coverage instead of being promoted to a
// full-opacity 1px line. The 0.5px line must be visibly lighter than the
// 1px reference at the same sub-pixel position.
func TestStrokeSubpixelWidthGPU(t *testing.T) {
	requireNativeGPU(t)

	draw := func(width, y float64) (uint8, string) {
		dc := render.NewContext(64, 32)
		defer dc.Close()
		dc.ResetRenderPathStats()
		dc.ClearWithColor(render.White)
		dc.SetRGB(0, 0, 0)
		dc.SetLineWidth(width)
		dc.MoveTo(4, y)
		dc.LineTo(60, y)
		if err := dc.Stroke(); err != nil {
			t.Fatalf("width %v stroke: %v", width, err)
		}
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("width %v flush: %v", width, err)
		}
		stats := dc.RenderPathStats()
		if stats.GPUOps == 0 {
			t.Skipf("GPU stroke unavailable: %s", stats.LogLine())
		}
		r, _, _, _ := sampleRGBA(dc, 32, 16)
		return r, stats.LogLine()
	}

	half, halfStats := draw(0.5, 16.5)
	full, fullStats := draw(1.0, 16.5)
	t.Logf("0.5px=%d (%s), 1px=%d (%s)", half, halfStats, full, fullStats)

	if full > 32 {
		t.Fatalf("1px reference too light: %d", full)
	}
	if half < 64 || half > 200 {
		t.Fatalf("0.5px stroke=%d, want ~128 (sub-pixel coverage)", half)
	}
	if int(half) < int(full)+64 {
		t.Fatalf("0.5px=%d not lighter than 1px=%d (clamped?)", half, full)
	}
}
