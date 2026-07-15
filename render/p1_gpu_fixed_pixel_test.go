//go:build !nogpu

package render_test

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/internal/blend"
)

// P1.2: fixed-pixel tests on the real render Context + GPU accelerator path
// (webgpu -> rwgpu -> libwgpu_native), with CPU readback via FlushGPU/SavePNG path.

func requireNativeGPU(t *testing.T) {
	t.Helper()
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		// Allow default discovery but prefer explicit path in CI.
		t.Log("WGPU_NATIVE_PATH unset; relying on default lib discovery")
	}
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	// Force a trivial GPU op to ensure device init works.
	dc := render.NewContext(8, 8)
	defer dc.Close()
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, 8, 8)
	dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Skipf("GPU flush unavailable: %v", err)
	}
	stats := dc.RenderPathStats()
	if stats.GPUOps == 0 && stats.CPUFallbackOps == 0 {
		// Accelerator registered but no path attempted — unexpected.
		t.Logf("path stats after probe: %+v", stats)
	}
}

func sampleRGBA(dc *render.Context, x, y int) (r, g, b, a uint8) {
	img := dc.Image()
	rr, gg, bb, aa := img.At(x, y).RGBA()
	return uint8(rr >> 8), uint8(gg >> 8), uint8(bb >> 8), uint8(aa >> 8)
}

func almostEq(t *testing.T, name string, got, want uint8, tol uint8) {
	t.Helper()
	d := int(got) - int(want)
	if d < 0 {
		d = -d
	}
	if d > int(tol) {
		t.Fatalf("%s=%d want %d±%d", name, got, want, tol)
	}
}

func TestP12GPUFixedPixel_SourceOverPremul(t *testing.T) {
	requireNativeGPU(t)

	const w, h = 64, 64
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()

	dc.ClearWithColor(render.White)

	// Opaque blue destination
	dc.SetRGBA(0, 0, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	dc.Fill()

	// Premul-style 50% red source over blue: SetRGBA uses straight alpha in API;
	// renderer converts for compositing. Expect roughly (128,0,128)-ish or
	// documented premul SO result after flush.
	dc.SetRGBA(1, 0, 0, 0.5)
	dc.DrawRectangle(16, 16, 32, 32)
	dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("expected GPU ops on accelerator path, got %s", stats.LogLine())
	}

	// Reference: straight red@50 over opaque blue using package blend after premul.
	// premul src = (128,0,0,128) approx; dst = (0,0,255,255)
	sr := blend.GetBlendFunc(blend.BlendSourceOver)
	// mulDiv style: 255*0.5 = 127.5 -> use 128
	pr, pg, pb, pa := byte(128), byte(0), byte(0), byte(128)
	dr, dg, db, da := byte(0), byte(0), byte(255), byte(255)
	wr, wg, wb, wa := sr(pr, pg, pb, pa, dr, dg, db, da)

	gr, gg, gb, ga := sampleRGBA(dc, 32, 32)
	t.Logf("gpu center rgba=%d,%d,%d,%d ref=%d,%d,%d,%d", gr, gg, gb, ga, wr, wg, wb, wa)
	// AA/blend path tolerance: UI gate, not bit-exact shader match.
	almostEq(t, "r", gr, wr, 40)
	almostEq(t, "g", gg, wg, 40)
	almostEq(t, "b", gb, wb, 40)
	almostEq(t, "a", ga, wa, 5)
}

func TestP12GPUFixedPixel_OpaqueReplace(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ClearWithColor(render.Blue)
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, 32, 32)
	dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Fatalf("expected GPU ops, got %s", dc.RenderPathStats().LogLine())
	}
	r, g, b, a := sampleRGBA(dc, 16, 16)
	almostEq(t, "r", r, 255, 2)
	almostEq(t, "g", g, 0, 2)
	almostEq(t, "b", b, 0, 2)
	almostEq(t, "a", a, 255, 2)
}

func TestP12GPUFixedPixel_ClipOutsideClear(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(64, 64)
	defer dc.Close()
	// Paint white background on GPU path so flush does not leave transparent black.
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 64, 64)
	dc.Fill()

	dc.DrawCircle(32, 32, 12)
	dc.Clip()

	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, 64, 64)
	dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	// Outside clip should remain white-ish
	r, g, b, a := sampleRGBA(dc, 2, 2)
	almostEq(t, "outside-r", r, 255, 5)
	almostEq(t, "outside-g", g, 255, 5)
	almostEq(t, "outside-b", b, 255, 5)
	almostEq(t, "outside-a", a, 255, 5)
	// Inside clip should be red-dominant
	r, g, b, a = sampleRGBA(dc, 32, 32)
	if r < 200 || g > 40 || b > 40 {
		t.Fatalf("inside clip rgba=%d,%d,%d,%d want red", r, g, b, a)
	}
	if dc.RenderPathStats().GPUOps == 0 && dc.RenderPathStats().CPUFallbackOps == 0 {
		t.Fatalf("expected routing stats, got %s", dc.RenderPathStats().LogLine())
	}
}

func TestP12GPUFixedPixel_HairlineStroke(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 64, 64)
	dc.Fill()
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(1)
	dc.DrawLine(8, 32, 56, 32)
	dc.Stroke()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	// 1px AA hairline may be ~50% coverage at the geometric center; require
	// darker than background (not pure white) along the segment.
	foundDark := false
	for _, y := range []int{31, 32, 33} {
		r, g, b, _ := sampleRGBA(dc, 32, y)
		if int(r)+int(g)+int(b) < 600 { // darker than light gray
			foundDark = true
			t.Logf("hairline sample y=%d rgba=%d,%d,%d", y, r, g, b)
			break
		}
	}
	if !foundDark {
		r, g, b, _ := sampleRGBA(dc, 32, 32)
		t.Fatalf("hairline not visible near center rgba=%d,%d,%d", r, g, b)
	}
	// Far from line remains white
	r, g, b, _ := sampleRGBA(dc, 32, 8)
	almostEq(t, "bg-r", r, 255, 5)
	almostEq(t, "bg-g", g, 255, 5)
	almostEq(t, "bg-b", b, 255, 5)
}
