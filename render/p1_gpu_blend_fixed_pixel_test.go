//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// B.02: Porter-Duff fixed-function GPU blend modes (Copy / Plus / Clear)
// on the real Context → webgpu → rwgpu → libwgpu_native path.

func TestP12GPUFixedPixel_BlendCopy(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 64, 64
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()

	dc.ClearWithColor(render.White)
	// Opaque blue base (GPU SourceOver)
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Copy red over center — must replace, not SourceOver
	dc.SetBlendMode(render.BlendCopy)
	dc.SetRGBA(1, 0, 0, 0.5)
	dc.DrawRectangle(16, 16, 32, 32)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("B.02 Copy requires GPUOps>0: %s", stats.LogLine())
	}

	// Center: Copy 50% red replaces blue on the GPU RT. After readback
	// composites over white, expect ~ (255,127,127) pink — not purple
	// SourceOver red@50 over blue ~(128,0,128).
	r, g, b, a := sampleRGBA(dc, 32, 32)
	t.Logf("copy center rgba=%d,%d,%d,%d", r, g, b, a)
	if r < 200 {
		t.Fatalf("Copy red too low: rgba=%d,%d,%d,%d", r, g, b, a)
	}
	// White contribution lifts G/B; SourceOver-over-blue would keep G near 0.
	if g < 80 {
		t.Fatalf("Copy looks like SourceOver over blue (g too low): rgba=%d,%d,%d,%d", r, g, b, a)
	}
	// G and B should track (white under partial alpha), not B>>G (blue residual).
	if int(b)-int(g) > 40 {
		t.Fatalf("Copy residual blue too high vs green: rgba=%d,%d,%d,%d", r, g, b, a)
	}
	// Outside center remains blue
	br, bg, bb, _ := sampleRGBA(dc, 4, 4)
	if bb < 200 || br > 40 {
		t.Fatalf("base blue corrupted: rgba=%d,%d,%d", br, bg, bb)
	}
}

func TestP12GPUFixedPixel_BlendPlus(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ResetRenderPathStats()

	dc.ClearWithColor(render.Black)
	// 50% red
	dc.SetRGBA(1, 0, 0, 0.5)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()

	dc.SetBlendMode(render.BlendPlus)
	dc.SetRGBA(0, 1, 0, 0.5)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("B.02 Plus requires GPUOps>0: %s", stats.LogLine())
	}

	r, g, b, a := sampleRGBA(dc, 16, 16)
	t.Logf("plus center rgba=%d,%d,%d,%d", r, g, b, a)
	// Premul Plus: both channels present
	if r < 80 || g < 80 {
		t.Fatalf("Plus expected both R and G significant, got %d,%d,%d", r, g, b)
	}
	if b > 40 {
		t.Fatalf("Plus unexpected blue %d", b)
	}
}

func TestP12GPUFixedPixel_BlendClear(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ResetRenderPathStats()

	dc.ClearWithColor(render.White)
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()

	dc.SetBlendMode(render.BlendClear)
	dc.SetRGB(1, 0, 0) // color ignored by Clear
	dc.DrawRectangle(8, 8, 16, 16)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("B.02 Clear requires GPUOps>0: %s", stats.LogLine())
	}

	// Cleared region should be near transparent; after composite-over-white
	// readback of a=0 leaves white, or near-white if residual.
	// GPU clear writes zeros; compositeBGRAOverRGBA leaves dst (white).
	r, g, b, a := sampleRGBA(dc, 16, 16)
	t.Logf("clear center rgba=%d,%d,%d,%d", r, g, b, a)
	// Must not remain solid blue
	if b > 200 && r < 40 && g < 40 {
		t.Fatalf("Clear had no effect (still blue)")
	}
	// Outside remains blue
	br, _, bb, _ := sampleRGBA(dc, 2, 2)
	if bb < 200 {
		t.Fatalf("outside clear should stay blue, got r=%d b=%d", br, bb)
	}
}

// --- Residual Porter-Duff fixed-function modes (B.02) ---

func TestP12GPUFixedPixel_BlendSourceOut(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ResetRenderPathStats()

	dc.ClearWithColor(render.White)
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()

	// SrcOut over opaque dest → transparent (src*(1-dstA)=0)
	dc.SetBlendMode(render.BlendSourceOut)
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(8, 8, 16, 16)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("B.02 SourceOut requires GPUOps>0: %s", stats.LogLine())
	}
	r, g, b, _ := sampleRGBA(dc, 16, 16)
	t.Logf("srcOut center rgba=%d,%d,%d", r, g, b)
	// Must not remain solid blue; transparent composites over white ≈ white
	if b > 80 && r < 40 {
		t.Fatalf("SourceOut had no effect (still blue): %d,%d,%d", r, g, b)
	}
	_, _, bb, _ := sampleRGBA(dc, 2, 2)
	if bb < 200 {
		t.Fatalf("outside should stay blue, b=%d", bb)
	}
}

func TestP12GPUFixedPixel_BlendDestinationIn(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ResetRenderPathStats()

	dc.ClearWithColor(render.White)
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()

	// DstIn: keep dst scaled by srcA. srcA=0.5 → half-alpha blue → pale blue over white
	dc.SetBlendMode(render.BlendDestinationIn)
	dc.SetRGBA(1, 0, 0, 0.5) // color ignored; alpha matters
	dc.DrawRectangle(8, 8, 16, 16)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("B.02 DestinationIn requires GPUOps>0: %s", stats.LogLine())
	}
	r, g, b, a := sampleRGBA(dc, 16, 16)
	t.Logf("dstIn center rgba=%d,%d,%d,%d", r, g, b, a)
	// Pale blue: B significant but less than full 255 and R/G lifted by white under
	if b < 80 {
		t.Fatalf("DestinationIn expected residual blue, got %d,%d,%d", r, g, b)
	}
	if b > 240 && r < 20 && g < 20 {
		t.Fatalf("DestinationIn looks fully opaque blue (srcA ignored): %d,%d,%d", r, g, b)
	}
	// Outside remains solid blue
	_, _, bb, _ := sampleRGBA(dc, 2, 2)
	if bb < 200 {
		t.Fatalf("outside should stay blue, b=%d", bb)
	}
}

func TestP12GPUFixedPixel_BlendSourceIn(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ResetRenderPathStats()

	// Semi-transparent blue dest so SrcIn scales source by dstA (≠ Copy).
	dc.ClearWithColor(render.Transparent)
	dc.SetRGBA(0, 0, 1, 0.5)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()

	dc.SetBlendMode(render.BlendSourceIn)
	dc.SetRGB(1, 0, 0) // opaque red * dstA0.5 → half red
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("B.02 SourceIn requires GPUOps>0: %s", stats.LogLine())
	}
	r, g, b, a := sampleRGBA(dc, 16, 16)
	t.Logf("srcIn center rgba=%d,%d,%d,%d", r, g, b, a)
	// SrcIn with dstA=0.5 and opaque red → premul/straight half-red ≈ (128,0,0,128).
	// Opaque Copy would be (255,0,0,255). Transparent miss would leave blue-ish or a≈0.
	if r < 90 || r > 170 {
		t.Fatalf("SourceIn red out of half-alpha band: %d,%d,%d,%d", r, g, b, a)
	}
	if a < 90 || a > 170 {
		t.Fatalf("SourceIn alpha out of half band: %d,%d,%d,%d", r, g, b, a)
	}
	if b > 40 {
		t.Fatalf("SourceIn residual blue dominates: %d,%d,%d", r, g, b)
	}
	if g > 40 {
		t.Fatalf("SourceIn unexpected green: %d,%d,%d", r, g, b)
	}
}

func TestP12GPUFixedPixel_BlendDestinationOver(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ResetRenderPathStats()

	// Transparent base; red center first, then DstOver blue full.
	// Center: red stays on top (dst); outside: blue fills.
	dc.ClearWithColor(render.Transparent)
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(8, 8, 16, 16)
	_ = dc.Fill()

	dc.SetBlendMode(render.BlendDestinationOver)
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("B.02 DestinationOver requires GPUOps>0: %s", stats.LogLine())
	}
	cr, cg, cb, _ := sampleRGBA(dc, 16, 16)
	t.Logf("dstOver center rgba=%d,%d,%d", cr, cg, cb)
	if cr < 200 || cb > 80 {
		t.Fatalf("center should stay red under DstOver: %d,%d,%d", cr, cg, cb)
	}
	or, og, ob, _ := sampleRGBA(dc, 2, 2)
	t.Logf("dstOver outside rgba=%d,%d,%d", or, og, ob)
	if ob < 200 || or > 80 {
		t.Fatalf("outside should be blue under DstOver: %d,%d,%d", or, og, ob)
	}
}

func TestP12GPUFixedPixel_BlendDestinationAtop(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ResetRenderPathStats()

	dc.ClearWithColor(render.White)
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()

	// Opaque dest: DstAtop → dst * srcA. srcA=0.5 → pale blue (like DstIn on opaque).
	dc.SetBlendMode(render.BlendDestinationAtop)
	dc.SetRGBA(1, 0, 0, 0.5)
	dc.DrawRectangle(8, 8, 16, 16)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("B.02 DestinationAtop requires GPUOps>0: %s", stats.LogLine())
	}
	r, g, b, _ := sampleRGBA(dc, 16, 16)
	t.Logf("dstAtop center rgba=%d,%d,%d", r, g, b)
	if b < 80 {
		t.Fatalf("DestinationAtop expected residual blue: %d,%d,%d", r, g, b)
	}
	if b > 240 && r < 20 && g < 20 {
		t.Fatalf("DestinationAtop looks fully opaque blue: %d,%d,%d", r, g, b)
	}
	_, _, bb, _ := sampleRGBA(dc, 2, 2)
	if bb < 200 {
		t.Fatalf("outside should stay blue, b=%d", bb)
	}
}
