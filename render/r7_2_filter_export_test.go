//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

func requireR72GPU(t *testing.T) {
	t.Helper()
	if render.Accelerator() == nil {
		// Force init via context.
		dc := render.NewContext(4, 4)
		_ = dc.Close()
	}
	if render.Accelerator() == nil {
		t.Skip("no GPU accelerator")
	}
	if !render.FiltersRegistered() {
		t.Fatal("filters not registered")
	}
}

// TestR72_ApplyBlur_GPUFilterTextureNoExportPrefer ensures ApplyBlur publishes a
// GPU filter texture so present can DrawGPUTexture without ExportImageBuf.
func TestR72_ApplyBlur_GPUFilterTextureNoExportPrefer(t *testing.T) {
	requireR72GPU(t)
	if !render.GPUFilterGraphRegistered() {
		dc0 := render.NewContext(8, 8)
		_ = dc0.Close()
	}
	if !render.GPUFilterGraphRegistered() {
		t.Skip("GPU filter graph not registered")
	}
	dc := render.NewContext(64, 48)
	defer dc.Close()
	dc.SetEffectSurface(true)
	dc.ClearWithColor(render.RGBA{R: 0.1, G: 0.1, B: 0.2, A: 1})
	dc.SetRGBA(1, 0.4, 0.1, 0.95)
	dc.DrawCircle(32, 24, 14)
	_ = dc.Fill()
	base := dc.RenderPathStats().GPUOps
	dc.ApplyBlur(2)
	stats := dc.RenderPathStats()
	if stats.GPUOps <= base {
		t.Fatalf("ApplyBlur should record GPUOps: base=%d after=%d %s", base, stats.GPUOps, stats.LogLine())
	}
	if stats.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fb: %s", stats.LogLine())
	}
	view, w, h, ok := dc.GPUFilterTexture()
	if !ok || view.IsNil() || w != 64 || h != 48 {
		t.Fatalf("expected GPUFilterTexture after ApplyBlur ok=%v %dx%d nil=%v", ok, w, h, view.IsNil())
	}
	var img *render.ImageBuf
	if !dc.ExportImageBuf(&img) || img == nil {
		t.Fatal("ExportImageBuf after GPU filter failed")
	}
	iw, ih := img.Bounds()
	if iw != 64 || ih != 48 {
		t.Fatalf("export size %dx%d", iw, ih)
	}
	if !img.IsGPUDirty() {
		t.Fatal("export should mark GPU dirty")
	}
}

// TestR72_ExportImageBuf_AfterGPUFilter_NoPendingSucceeds: after GPU filter
// publish, Export with no pending draws must succeed (R7.2 single materialize).
func TestR72_ExportImageBuf_AfterGPUFilter_NoPendingSucceeds(t *testing.T) {
	requireR72GPU(t)
	if !render.GPUFilterGraphRegistered() {
		dc0 := render.NewContext(8, 8)
		_ = dc0.Close()
	}
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.SetEffectSurface(true)
	dc.SetRGBA(0.2, 0.8, 1, 1)
	dc.DrawRectangle(4, 4, 24, 24)
	_ = dc.Fill()
	dc.ApplyBlur(1.5)
	if _, _, _, ok := dc.GPUFilterTexture(); !ok {
		t.Skip("no GPU filter publish on this path")
	}
	var img *render.ImageBuf
	if !dc.ExportImageBuf(&img) || img == nil {
		t.Fatal("export failed")
	}
	gen := img.GenerationID()
	_ = img.TakeGPUDirty()
	if !dc.ExportImageBuf(&img) {
		t.Fatal("second export failed")
	}
	if img.GenerationID() != gen {
		t.Fatalf("gen changed on second export")
	}
}
