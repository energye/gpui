//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// requireFilterOpsGPU skips when the native GPU filter path is unavailable so
// these regressions run on the real path in CI and skip cleanly elsewhere.
func requireFilterOpsGPU(t *testing.T) {
	t.Helper()
	if !render.GPUFilterGraphRegistered() {
		// A real GPU draw + flush initializes GPUShared, which wires the
		// multi-RT filter graph hooks. Do it even when Accelerator() is already
		// non-nil: another test may have initialized the device before the
		// graph hooks were installed.
		warmFilterOpsGPU(t)
	}
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	if !render.FiltersRegistered() {
		t.Fatal("filters not registered")
	}
	if !render.GPUFilterGraphRegistered() {
		t.Skip("GPU filter graph not registered")
	}
}

// GPU invert must run in straight-alpha space like the CPU color matrix, then
// store premultiplied: premul' = alpha - premul. The old shader did 1-premul,
// which could produce channels greater than alpha on translucent pixels.
func TestFilterOpsGPUInvertKeepsPremultipliedAlpha(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 16, 16
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ClearWithColor(render.RGBA{R: 1, A: 0.5})
	sr, sg, sb, sa := filterRawPixel(dc, 8, 8)
	if sa < 126 || sa > 129 {
		t.Fatalf("source alpha = %d, want ~128", sa)
	}

	dc.ResetRenderPathStats()
	dc.ApplyInvert()
	stats := dc.RenderPathStats()
	if stats.GPUOps == 0 {
		t.Fatalf("ApplyInvert did not use the GPU graph: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps != 0 {
		t.Fatalf("ApplyInvert fell back to CPU: %s", stats.LogLine())
	}
	view, vw, vh, ok := dc.GPUFilterTexture()
	if !ok || view.IsNil() || vw != w || vh != h {
		t.Fatalf("GPUFilterTexture = ok=%v %dx%d nil=%v, want %dx%d", ok, vw, vh, view.IsNil(), w, h)
	}

	// Composite the published view into a fresh context before touching the
	// source pixmap: this is the zero-readback present path.
	dst := render.NewContext(w, h)
	defer dst.Close()
	dst.Clear()
	dst.DrawGPUTexture(view, 0, 0, w, h)
	img := dst.Image()
	r, g, b, a := filterPixel(img, 8, 8)
	wantR, wantG, wantB := int(sa)-int(sr), int(sa)-int(sg), int(sa)-int(sb)
	if filterAbsDiff(int(a), int(sa)) > 1 {
		t.Fatalf("composited invert alpha = %d, want %d", a, sa)
	}
	if filterAbsDiff(int(r), wantR) > 1 || filterAbsDiff(int(g), wantG) > 1 ||
		filterAbsDiff(int(b), wantB) > 1 {
		t.Fatalf("composited invert = %d,%d,%d,%d, want premul %d,%d,%d,%d",
			r, g, b, a, wantR, wantG, wantB, sa)
	}
	if r > a || g > a || b > a {
		t.Fatalf("GPU invert produced invalid premultiplied pixel: %d,%d,%d,%d", r, g, b, a)
	}
}

// ApplyBlurXY must publish a zero-copy texture that another context can draw
// with DrawGPUTexture, and the result must match a CPU reference (tolerance
// 3/255 for kernel/rounding differences).
func TestFilterOpsGPUBlurXYTextureCrossContext(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 80, 60
	src := render.NewContext(w, h)
	defer src.Close()
	src.SetEffectSurface(true)
	src.ClearWithColor(render.RGBA{R: 0.05, G: 0.06, B: 0.1, A: 1})
	src.SetRGBA(1, 0.35, 0.1, 1)
	src.DrawRectangle(20, 15, 40, 30)
	_ = src.Fill()

	src.ResetRenderPathStats()
	src.ApplyBlurXY(0, 3)
	stats := src.RenderPathStats()
	if stats.GPUOps == 0 {
		t.Fatalf("ApplyBlurXY did not use the GPU graph: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps != 0 {
		t.Fatalf("ApplyBlurXY fell back to CPU: %s", stats.LogLine())
	}
	view, vw, vh, ok := src.GPUFilterTexture()
	if !ok || view.IsNil() || vw != w || vh != h {
		t.Fatalf("GPUFilterTexture = ok=%v %dx%d nil=%v, want %dx%d", ok, vw, vh, view.IsNil(), w, h)
	}

	// Independent CPU reference: identical content, filter applied without the
	// GPU hooks. Disable before Apply* so the CPU branch is selected.
	ref := render.NewContext(w, h)
	defer ref.Close()
	ref.ClearWithColor(render.RGBA{R: 0.05, G: 0.06, B: 0.1, A: 1})
	ref.SetRGBA(1, 0.35, 0.1, 1)
	ref.DrawRectangle(20, 15, 40, 30)
	_ = ref.Fill()
	restore := render.DisableGPUFilterGraphForTest()
	ref.ApplyBlurXY(0, 3)
	restore()
	refImg := ref.Image()

	dst := render.NewContext(w, h)
	defer dst.Close()
	dst.Clear()
	dst.DrawGPUTexture(view, 0, 0, w, h)
	gpuImg := dst.Image()

	if d := filterMaxDiff(gpuImg, refImg); d > 3 {
		t.Fatalf("GPU texture vs CPU reference max channel diff = %d, want <=3", d)
	}
}

// The effect publish path (FlushGPU → GPUFilterTexture) must include CPU base
// content, not just pending GPU draws. Regression: an opaque effect surface
// published transparent corners over its ClearWithColor background.
func TestFilterOpsGPUEffectPublishKeepsClearBase(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 32, 32
	src := render.NewContext(w, h)
	defer src.Close()
	src.SetEffectSurface(true)
	src.ClearWithColor(render.RGBA{R: 0.05, G: 0.06, B: 0.1, A: 1})
	src.SetRGB(1, 0, 0)
	src.DrawRectangle(8, 8, 16, 16)
	_ = src.Fill()
	if err := src.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	view, vw, vh, ok := src.GPUFilterTexture()
	if !ok || view.IsNil() || vw != w || vh != h {
		t.Fatalf("GPUFilterTexture = ok=%v %dx%d nil=%v, want %dx%d", ok, vw, vh, view.IsNil(), w, h)
	}

	dst := render.NewContext(w, h)
	defer dst.Close()
	dst.Clear()
	dst.DrawGPUTexture(view, 0, 0, w, h)
	img := dst.Image()

	// Background outside the rect: premultiplied (0.05,0.06,0.1) ≈ 12,15,25.
	r, g, b, a := filterPixel(img, 2, 2)
	if a < 250 || filterAbsDiff(int(r), 12) > 2 || filterAbsDiff(int(g), 15) > 2 || filterAbsDiff(int(b), 25) > 2 {
		t.Fatalf("published background = %d,%d,%d,%d, want ~12,15,25,255", r, g, b, a)
	}
	r, g, b, a = filterPixel(img, 16, 16)
	if a < 250 || r < 250 || g > 2 || b > 2 {
		t.Fatalf("published rect = %d,%d,%d,%d, want 255,0,0,255", r, g, b, a)
	}
}

// A CPU-rasterized block (FillRectCPU) is base content too: a later GPU draw
// plus ApplyBlurXY must filter the block together with the GPU content, not
// drop it when seeding the filter RT from pending commands only.
func TestFilterOpsGPUBlurXYKeepsCPUFallbackBase(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 64, 48
	draw := func(dc *render.Context) {
		dc.Clear()
		dc.FillRectCPU(6, 6, 18, 16, render.RGBA{G: 1, A: 1})
		dc.SetRGB(1, 0.2, 0.1)
		dc.DrawRectangle(34, 18, 22, 18)
		_ = dc.Fill()
	}
	gpu := render.NewContext(w, h)
	defer gpu.Close()
	draw(gpu)
	gpu.ApplyBlurXY(0, 3)
	view, vw, vh, ok := gpu.GPUFilterTexture()
	if !ok || view.IsNil() || vw != w || vh != h {
		t.Fatalf("GPUFilterTexture = ok=%v %dx%d nil=%v, want %dx%d", ok, vw, vh, view.IsNil(), w, h)
	}

	cpu := render.NewContext(w, h)
	defer cpu.Close()
	draw(cpu)
	restore := render.DisableGPUFilterGraphForTest()
	cpu.ApplyBlurXY(0, 3)
	restore()

	dst := render.NewContext(w, h)
	defer dst.Close()
	dst.Clear()
	dst.DrawGPUTexture(view, 0, 0, w, h)
	if d := filterMaxDiff(dst.Image(), cpu.Image()); d > 3 {
		t.Fatalf("GPU texture vs CPU reference max channel diff = %d, want <=3", d)
	}
}

// The multi-node GPU graph (BlurXY -> Grayscale -> Invert) must match the same
// operations applied sequentially on the CPU, including translucent content.
func TestFilterOpsGPUGraphMatchesCPUSequential(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 64, 48
	draw := func(dc *render.Context) {
		dc.ClearWithColor(render.RGBA{R: 0.08, G: 0.1, B: 0.16, A: 1})
		dc.SetRGBA(1, 0.25, 0.05, 1)
		dc.DrawRectangle(8, 8, 28, 20)
		_ = dc.Fill()
		dc.SetRGBA(0.1, 0.9, 0.8, 0.6)
		dc.DrawRectangle(30, 22, 26, 18)
		_ = dc.Fill()
	}

	gpu := render.NewContext(w, h)
	defer gpu.Close()
	draw(gpu)
	gpu.ResetRenderPathStats()
	gpu.ApplyImageFilterGraph(
		render.ImageFilterNode{Kind: render.ImageFilterBlurXY, RadiusX: 0, RadiusY: 2},
		render.ImageFilterNode{Kind: render.ImageFilterGrayscale},
		render.ImageFilterNode{Kind: render.ImageFilterInvert},
	)
	stats := gpu.RenderPathStats()
	if stats.GPUOps == 0 {
		t.Fatalf("filter graph did not use the GPU path: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps != 0 {
		t.Fatalf("filter graph fell back to CPU: %s", stats.LogLine())
	}
	gpuImg := gpu.Image()

	cpu := render.NewContext(w, h)
	defer cpu.Close()
	draw(cpu)
	restore := render.DisableGPUFilterGraphForTest()
	cpu.ApplyBlurXY(0, 2)
	cpu.ApplyGrayscale()
	cpu.ApplyInvert()
	restore()
	cpuImg := cpu.Image()

	if d := filterMaxDiff(gpuImg, cpuImg); d > 3 {
		t.Fatalf("GPU graph vs CPU sequential max channel diff = %d, want <=3", d)
	}
}
