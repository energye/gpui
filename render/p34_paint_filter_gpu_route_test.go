//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
)

// TestP34PaintMaskFilterGPURoute pins the P3.4 paint-level MaskFilter on the
// real GPU route: with a device bound, a filtered rectangle fill must not be
// eaten by the SDF shape shortcut — the filtered-paint branch in FillPath
// (fillPaintFilteredAsImage) owns it and the edge is feathered, matching the
// CPU-tier pin TestPaintMaskFilterSoftensEdges. Regression context: the shape
// shortcut used to paint a hard edge and still count a GPU op (C-card root
// cause, evidence/P3.4).
func TestP34PaintMaskFilterGPURoute(t *testing.T) {
	if err := setupNativeDevice(t); err != nil {
		t.Skipf("native device unavailable: %v", err)
	}
	dc := render.NewContext(64, 64)
	t.Cleanup(func() { _ = dc.Close() })
	if dc.GPURenderContext() == nil {
		t.Fatal("per-context GPU not established — route precondition")
	}
	dc.ClearWithColor(render.Transparent)
	dc.SetRGB(1, 1, 1)
	dc.SetPaintMaskFilter(3)
	dc.DrawRectangle(16, 16, 32, 32)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	stats := dc.RenderPathStats()
	if stats.GPUOps == 0 {
		t.Fatalf("gpu_ops=0 — filtered fill did not ride the GPU route")
	}
	img := dc.Image()
	at := func(x, y int) (byte, byte, byte, byte) {
		r, g, b, a := img.At(x, y).RGBA()
		return byte(r >> 8), byte(g >> 8), byte(b >> 8), byte(a >> 8)
	}
	if _, _, _, a := at(32, 32); a < 200 {
		t.Fatalf("interior alpha=%d, want opaque", a)
	}
	if _, _, _, a := at(16, 32); a == 0 || a > 245 {
		t.Fatalf("boundary alpha=%d, want a feathered intermediate value", a)
	}
	if _, _, _, a := at(14, 32); a == 0 {
		t.Fatal("no outward halo at x=14 — MaskFilter blur did not apply on the GPU route")
	}
	if _, _, _, a := at(4, 32); a != 0 {
		t.Fatalf("far-field alpha=%d, want untouched transparent background", a)
	}
}
