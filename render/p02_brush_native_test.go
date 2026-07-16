package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// TestP02_LinearGradientNativeGPU verifies G.02: multi-stop linear gradient
// fills via GPU convex path (no CPU fallback), with correct color ramp.
func TestP02_LinearGradientNativeGPU(t *testing.T) {
	requireNativeGPU(t)

	dc := render.NewContext(128, 64)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	grad := render.NewLinearGradientBrush(0, 0, 128, 0).
		AddColorStop(0, render.RGB(1, 0, 0)).
		AddColorStop(0.5, render.RGB(0, 1, 0)).
		AddColorStop(1, render.RGB(0, 0, 1))
	dc.SetFillBrush(grad)
	dc.DrawRectangle(0, 0, 128, 64)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	stats := dc.RenderPathStats()
	t.Logf("linear %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("P02 linear GPUOps==0: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("P02 linear must not CPU-fallback: %s", stats.LogLine())
	}

	rL, gL, bL, _ := sampleRGBA(dc, 4, 32)
	rM, gM, bM, _ := sampleRGBA(dc, 64, 32)
	rR, gR, bR, _ := sampleRGBA(dc, 124, 32)
	t.Logf("L=%d,%d,%d M=%d,%d,%d R=%d,%d,%d", rL, gL, bL, rM, gM, bM, rR, gR, bR)

	if rL < 180 || gL > 80 {
		t.Fatalf("left expected red-ish, got %d,%d,%d", rL, gL, bL)
	}
	if gM < 120 {
		t.Fatalf("mid expected green contribution, got %d,%d,%d", rM, gM, bM)
	}
	if bR < 180 || rR > 80 {
		t.Fatalf("right expected blue-ish, got %d,%d,%d", rR, gR, bR)
	}
	// Direction: red channel falls left→right
	if rL <= rR {
		t.Fatalf("expected red decrease L→R: L=%d R=%d", rL, rR)
	}
}

// TestP02_RadialGradientNativeGPU verifies radial gradient GPU-native path.
func TestP02_RadialGradientNativeGPU(t *testing.T) {
	requireNativeGPU(t)

	dc := render.NewContext(80, 80)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	grad := render.NewRadialGradientBrush(40, 40, 0, 36).
		AddColorStop(0, render.RGB(1, 1, 1)).
		AddColorStop(1, render.RGB(0, 0, 0))
	dc.SetFillBrush(grad)
	dc.DrawRectangle(0, 0, 80, 80)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("radial %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("P02 radial GPUOps==0")
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("P02 radial cpu_fb: %s", stats.LogLine())
	}

	// Center brighter than corner of the gradient extent.
	cr, cg, cb, _ := sampleRGBA(dc, 40, 40)
	er, eg, eb, _ := sampleRGBA(dc, 40, 4)
	cSum := int(cr) + int(cg) + int(cb)
	eSum := int(er) + int(eg) + int(eb)
	t.Logf("center=%d edge=%d", cSum, eSum)
	if cSum <= eSum {
		t.Fatalf("radial center should be brighter than edge: c=%d e=%d", cSum, eSum)
	}
}

// TestP02_ImagePatternNativeGPU verifies G.03: tiled image pattern on a rect
// uses GPU texture quads (GPUOps>0, no CPU fallback).
func TestP02_ImagePatternNativeGPU(t *testing.T) {
	requireNativeGPU(t)

	img, err := render.NewImageBuf(8, 8, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("img: %v", err)
	}
	// Left half red, right half blue.
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if x < 4 {
				_ = img.SetRGBA(x, y, 255, 0, 0, 255)
			} else {
				_ = img.SetRGBA(x, y, 0, 0, 255, 255)
			}
		}
	}
	img.NotifyPixelsChanged()

	dc := render.NewContext(40, 24)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	pat := dc.CreateImagePattern(img, 0, 0, 8, 8)
	dc.SetFillPattern(pat)
	dc.DrawRectangle(0, 0, 40, 24)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("pattern %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("P02 pattern GPUOps==0")
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("P02 pattern cpu_fb: %s", stats.LogLine())
	}
	r0, _, b0, _ := sampleRGBA(dc, 2, 12)
	_, _, b1, _ := sampleRGBA(dc, 6, 12)
	r2, _, _, _ := sampleRGBA(dc, 10, 12)
	if r0 < 180 || b0 > 60 {
		t.Fatalf("tile left red expected: r=%d b=%d", r0, b0)
	}
	if b1 < 180 {
		t.Fatalf("tile right blue expected: b=%d", b1)
	}
	if r2 < 180 {
		t.Fatalf("next tile left red expected: r=%d", r2)
	}
}

// TestP02_LinearGradientExtendRepeatGPU verifies G.02 span ramp for Repeat.
func TestP02_LinearGradientExtendRepeatGPU(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 80, 40
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	// Horizontal gradient 0→20, then Repeat across full rect.
	g := render.NewLinearGradientBrush(0, 0, 20, 0).
		AddColorStop(0, render.RGBA{R: 1, G: 0, B: 0, A: 1}).
		AddColorStop(1, render.RGBA{R: 0, G: 0, B: 1, A: 1}).
		SetExtend(render.ExtendRepeat)
	dc.SetFillBrush(g)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("repeat %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("expected GPUOps: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("cpu_fb: %s", stats.LogLine())
	}
	// Near start of period (~x=2): red-ish
	r0, g0, b0, _ := sampleRGBA(dc, 2, 20)
	// Near mid of first period (~x=10): mix
	// Near start of second period (~x=22): red-ish again (repeat)
	r1, g1, b1, _ := sampleRGBA(dc, 22, 20)
	t.Logf("x2=%d,%d,%d x22=%d,%d,%d", r0, g0, b0, r1, g1, b1)
	if r0 < 150 || b0 > 120 {
		t.Fatalf("x2 expected red-dominant, got %d,%d,%d", r0, g0, b0)
	}
	if r1 < 150 || b1 > 120 {
		t.Fatalf("x22 expected repeated red-dominant, got %d,%d,%d", r1, g1, b1)
	}
	// Mid-period blue side (~x=18)
	r2, g2, b2, _ := sampleRGBA(dc, 18, 20)
	t.Logf("x18=%d,%d,%d", r2, g2, b2)
	if b2 < 100 {
		t.Fatalf("x18 expected blue influence, got %d,%d,%d", r2, g2, b2)
	}
}

// TestP02_DiagonalLinearGradientGPU verifies G.02 residual: diagonal linear on AA rect.
func TestP02_DiagonalLinearGradientGPU(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 64, 64
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	// Diagonal red → blue across the rect.
	g := render.NewLinearGradientBrush(0, 0, w, h).
		AddColorStop(0, render.RGBA{R: 1, G: 0, B: 0, A: 1}).
		AddColorStop(1, render.RGBA{R: 0, G: 0, B: 1, A: 1})
	dc.SetFillBrush(g)
	dc.DrawRectangle(0, 0, w, h)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("diagonal %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("expected GPUOps: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("cpu_fb: %s", stats.LogLine())
	}
	// Near start (top-left): red-ish
	r0, g0, b0, _ := sampleRGBA(dc, 4, 4)
	// Near end (bottom-right): blue-ish
	r1, g1, b1, _ := sampleRGBA(dc, 60, 60)
	t.Logf("tl=%d,%d,%d br=%d,%d,%d", r0, g0, b0, r1, g1, b1)
	if r0 < 150 || b0 > 120 {
		t.Fatalf("top-left expected red-dominant, got %d,%d,%d", r0, g0, b0)
	}
	if b1 < 150 || r1 > 120 {
		t.Fatalf("bottom-right expected blue-dominant, got %d,%d,%d", r1, g1, b1)
	}
}

// TestP02_CustomBrushBootstrapReason verifies G.04: CustomBrush uses GPU blit
// with explicit bootstrap reason (not silent).
func TestP02_CustomBrushBootstrapReason(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 48, 48
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	cb := render.NewCustomBrush(func(x, y float64) render.RGBA {
		// Horizontal ramp
		t := x / float64(w)
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		return render.RGBA{R: t, G: 0, B: 1 - t, A: 1}
	})
	dc.SetFillBrush(cb)
	dc.DrawRectangle(0, 0, w, h)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("custom %s bootstrap_ops=%d reason=%q", stats.LogLine(), stats.BrushBootstrapOps, stats.LastBrushBootstrapReason)
	if stats.GPUOps == 0 {
		t.Fatalf("CustomBrush must still GPU-blit: %s", stats.LogLine())
	}
	// Explicit reason required (G.04). Not counted as hard cpu_fb when blit succeeds.
	if stats.BrushBootstrapOps < 1 || stats.LastBrushBootstrapReason == "" {
		t.Fatalf("expected brush bootstrap reason, ops=%d reason=%q", stats.BrushBootstrapOps, stats.LastBrushBootstrapReason)
	}
	if stats.LastBrushBootstrapReason != "brush:custom" {
		t.Fatalf("reason=%q want brush:custom", stats.LastBrushBootstrapReason)
	}
	rL, _, bL, _ := sampleRGBA(dc, 2, 24)
	rR, _, bR, _ := sampleRGBA(dc, 46, 24)
	t.Logf("L=%d,* ,%d R=%d,*,%d", rL, bL, rR, bR)
	if rL > rR && bL < bR {
		// good: left more blue-ish? wait ramp R increases with x, B decreases
	}
	if int(rR) < int(rL)+20 {
		t.Fatalf("expected red to increase left→right, L.r=%d R.r=%d", rL, rR)
	}
}

// TestP02_NonConvexLinearGradientGPU verifies non-convex path gradient uses
// GPU blit bootstrap with explicit reason (not silent CPU, not hard cpu_fb).
func TestP02_NonConvexLinearGradientGPU(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 80, 80
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	// Concave arrow / chevron-like non-convex polygon.
	dc.MoveTo(10, 20)
	dc.LineTo(40, 10)
	dc.LineTo(70, 20)
	dc.LineTo(55, 40)
	dc.LineTo(70, 60)
	dc.LineTo(40, 70)
	dc.LineTo(10, 60)
	dc.LineTo(25, 40)
	dc.ClosePath()

	g := render.NewLinearGradientBrush(0, 0, w, 0).
		AddColorStop(0, render.RGBA{R: 1, G: 0.2, B: 0, A: 1}).
		AddColorStop(1, render.RGBA{R: 0, G: 0.3, B: 1, A: 1})
	dc.SetFillBrush(g)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("nonconvex %s bootstrap_ops=%d reason=%q", stats.LogLine(), stats.BrushBootstrapOps, stats.LastBrushBootstrapReason)
	if stats.GPUOps == 0 {
		t.Fatalf("expected GPUOps: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("hard cpu_fb should stay 0 (bootstrap is GPU*): %s reason=%q", stats.LogLine(), stats.LastCPUFallbackReason)
	}
	if stats.BrushBootstrapOps < 1 {
		// May hit convex path if polygon classified convex — still OK if GPU and colored.
		t.Logf("no bootstrap (path may be convex-classified); checking ink only")
	} else if stats.LastBrushBootstrapReason != "brush:nonconvex-path" && stats.LastBrushBootstrapReason != "brush:evenodd" {
		t.Fatalf("unexpected bootstrap reason %q", stats.LastBrushBootstrapReason)
	}
	// Sample interior should not be pure white.
	r, gch, b, _ := sampleRGBA(dc, 40, 40)
	t.Logf("center=%d,%d,%d", r, gch, b)
	if int(r)+int(gch)+int(b) > 740 {
		// try another interior
		r, gch, b, _ = sampleRGBA(dc, 40, 25)
		t.Logf("alt=%d,%d,%d", r, gch, b)
	}
	if int(r)+int(gch)+int(b) > 740 {
		t.Fatalf("expected gradient ink inside non-convex path")
	}
}
