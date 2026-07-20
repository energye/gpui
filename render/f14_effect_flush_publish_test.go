//go:build !nogpu

package render

import "testing"

// TestF14_EffectSurface_FlushGPU_PublishesGPUFilterTexture ensures SetEffectSurface
// offscreens publish a zero-readback GPU texture on FlushGPU (no Map required for
// present via GPUFilterTexture / DrawGPUTexture).
func TestF14_EffectSurface_FlushGPU_PublishesGPUFilterTexture(t *testing.T) {
	if Accelerator() == nil {
		t.Skip("no GPU accelerator")
	}
	dc := NewContext(64, 48)
	defer func() { _ = dc.Close() }()
	dc.SetEffectSurface(true)
	dc.ClearWithColor(RGBA{R: 0.1, G: 0.2, B: 0.3, A: 1})
	dc.SetRGBA(1, 0.4, 0.1, 1)
	dc.DrawCircle(32, 24, 16)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	type pcounter interface{ PendingCount() int }
	pending := 0
	if rc := dc.gpuCtxOps(); rc != nil {
		if pc, ok := rc.(pcounter); ok {
			pending = pc.PendingCount()
		}
	}
	if pending <= 0 {
		t.Fatalf("expected pending GPU draws after Fill, got %d (ensureGPU/FillShape path)", pending)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	view, w, h, ok := dc.GPUFilterTexture()
	if !ok || view.IsNil() || w != 64 || h != 48 {
		t.Fatalf("expected GPUFilterTexture after effect FlushGPU ok=%v %dx%d nil=%v stale=%v",
			ok, w, h, view.IsNil(), dc.pixmapFilterStale)
	}
	// Export still works via lazy materialize (may Map once).
	var img *ImageBuf
	if !dc.ExportImageBuf(&img) || img == nil {
		t.Fatal("ExportImageBuf should materialize published effect RT")
	}
}

// TestF14_EffectSurface_Off_StillCPUReadback keeps non-effect contexts on the
// classic nil-view Flush path (no spurious GPUFilterTexture publish).
func TestF14_EffectSurface_Off_StillCPUReadback(t *testing.T) {
	if Accelerator() == nil {
		t.Skip("no GPU accelerator")
	}
	dc := NewContext(32, 32)
	defer func() { _ = dc.Close() }()
	dc.SetRGBA(0, 1, 0, 1)
	dc.DrawRectangle(4, 4, 24, 24)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	if _, _, _, ok := dc.GPUFilterTexture(); ok {
		t.Fatal("non-effect FlushGPU must not publish GPUFilterTexture")
	}
}
