//go:build !nogpu

package render_test

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// Verifies GPU path places y=0 content at top of readback image (Y-down UI).
func TestGPUYOrientation_TopIsTop(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Log("WGPU_NATIVE_PATH unset")
	}
	if render.Accelerator() == nil {
		t.Skip("no GPU accelerator")
	}
	const w, h = 64, 128
	dc := render.NewContext(w, h)
	defer dc.Close()

	// White clear
	dc.SetRGBA(1, 1, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	dc.Fill()

	// Red strip at TOP (y=0..16)
	dc.SetRGBA(1, 0, 0, 1)
	dc.DrawRectangle(0, 0, w, 16)
	dc.Fill()

	// Green strip at BOTTOM (y=h-16..h)
	dc.SetRGBA(0, 1, 0, 1)
	dc.DrawRectangle(0, h-16, w, 16)
	dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Skipf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Skip("no GPU ops — CPU only")
	}

	img := dc.Image()
	// Sample top and bottom centers
	tr, tg, tb, _ := img.At(w/2, 8).RGBA()
	br, bg, bb, _ := img.At(w/2, h-8).RGBA()
	t.Logf("top(y=8) rgb=%d,%d,%d bottom(y=%d) rgb=%d,%d,%d",
		tr>>8, tg>>8, tb>>8, h-8, br>>8, bg>>8, bb>>8)

	// Top must be red-dominant
	if tr>>8 < 200 || tg>>8 > 50 {
		t.Errorf("TOP should be red, got %d,%d,%d — GPU Y may be inverted", tr>>8, tg>>8, tb>>8)
	}
	// Bottom must be green-dominant
	if bg>>8 < 200 || br>>8 > 50 {
		t.Errorf("BOTTOM should be green, got %d,%d,%d — GPU Y may be inverted", br>>8, bg>>8, bb>>8)
	}
}
