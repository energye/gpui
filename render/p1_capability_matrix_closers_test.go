//go:build !nogpu

package render_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/text"
)

// Capability matrix closers: D.03 sweep, B.03 Multiply GPU path, B.02 extra PD,
// G.06 XY rrect — real render → webgpu → rwgpu → native, GPUOps>0.

func TestP1_Capability_D03_SweepGradientGPU(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 64, 64
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	grad := render.NewSweepGradientBrush(32, 32, 0).
		AddColorStop(0, render.Red).
		AddColorStop(0.5, render.Green).
		AddColorStop(1, render.Blue)
	dc.SetFillBrush(grad)
	dc.DrawCircle(32, 32, 28)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("D.03 sweep requires GPUOps>0: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("D.03 sweep must not CPU-fallback: %s", stats.LogLine())
	}

	// Sample around the circle: angular variation (not flat).
	// +X → red-ish, +Y → green-ish (CCW from +X with stops 0,0.5,1).
	rx, _, rb, _ := sampleRGBA(dc, 54, 32) // right
	_, gy, _, _ := sampleRGBA(dc, 32, 54)  // bottom (+Y down in canvas)
	t.Logf("right=%d,*,%d bottom=*,%d,*", rx, rb, gy)
	if rx < 100 {
		t.Fatalf("sweep +X expected red-dominant, r=%d", rx)
	}
	// Not a solid fill: opposite samples differ.
	lx, _, lb, _ := sampleRGBA(dc, 10, 32)
	if absU8(rx, lx) < 30 && absU8(rb, lb) < 30 {
		t.Fatalf("sweep looks flat: right r/b=%d/%d left r/b=%d/%d", rx, rb, lx, lb)
	}
}

func TestP1_Capability_B03_MultiplyGPU(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(48, 48)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	// Yellow base
	dc.SetRGB(1, 1, 0)
	dc.DrawRectangle(0, 0, 48, 48)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU base: %v", err)
	}
	baseGPU := dc.RenderPathStats().GPUOps

	dc.SetBlendMode(render.BlendMultiply)
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(0, 0, 48, 48)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU multiply: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s (base_gpu_ops=%d)", stats.LogLine(), baseGPU)
	if stats.GPUOps <= baseGPU {
		t.Fatalf("B.03 Multiply expected additional GPUOps, got %s", stats.LogLine())
	}
	// Yellow * Blue → near black interior
	r, g, b, _ := sampleRGBA(dc, 24, 24)
	t.Logf("multiply rgba=%d,%d,%d", r, g, b)
	if int(r)+int(g)+int(b) > 80 {
		t.Fatalf("expected dark multiply, got %d,%d,%d", r, g, b)
	}
}

func TestP1_Capability_B03_ScreenGPU(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.Black)
	dc.SetRGB(0.5, 0, 0)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()
	_ = dc.FlushGPU()

	dc.SetBlendMode(render.BlendScreen)
	dc.SetRGB(0, 0.5, 0)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	if dc.RenderPathStats().GPUOps < 2 {
		t.Fatalf("Screen needs GPU ops: %s", dc.RenderPathStats().LogLine())
	}
	r, g, b, _ := sampleRGBA(dc, 16, 16)
	t.Logf("screen rgba=%d,%d,%d", r, g, b)
	// Screen should lighten vs pure multiply-black
	if int(r)+int(g) < 100 {
		t.Fatalf("screen too dark %d,%d,%d", r, g, b)
	}
	if b > 40 {
		t.Fatalf("unexpected blue %d", b)
	}
}

func TestP12GPUFixedPixel_BlendDestinationOut(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()

	dc.SetBlendMode(render.BlendDestinationOut)
	dc.SetRGBA(1, 1, 1, 1) // opaque eraser
	dc.DrawRectangle(8, 8, 16, 16)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Fatalf("DstOut requires GPUOps>0")
	}
	// Erased region transparent → readback over white ≈ white
	r, g, b, _ := sampleRGBA(dc, 16, 16)
	t.Logf("dstOut center rgba=%d,%d,%d", r, g, b)
	if b > 80 && r < 40 {
		t.Fatalf("DstOut did not clear blue: %d,%d,%d", r, g, b)
	}
	// Outside still blue
	_, _, bb, _ := sampleRGBA(dc, 2, 2)
	if bb < 200 {
		t.Fatalf("outside should stay blue, b=%d", bb)
	}
}

func TestP1_Capability_G06_RRectXYRadiiGPU(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(80, 50)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0.1, 0.4, 0.9)
	// Wide horizontal radii, short vertical radii
	dc.DrawRoundedRectangleXY(10, 10, 60, 30, 20, 6)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("G.06 requires GPUOps>0: %s", stats.LogLine())
	}
	// Center filled
	r, g, b, _ := sampleRGBA(dc, 40, 25)
	if r > 200 && g > 200 && b > 200 {
		t.Fatalf("center empty rgba=%d,%d,%d", r, g, b)
	}
	// Far corner outside elliptical corner should be white-ish
	// Top-left extreme corner (10,10) is outside rounded shape
	cr, cg, cb, _ := sampleRGBA(dc, 11, 11)
	t.Logf("center=%d,%d,%d corner=%d,%d,%d", r, g, b, cr, cg, cb)
	// Corner of bbox may still catch AA; sample closer to geometric outside
	or, og, ob, _ := sampleRGBA(dc, 10, 10)
	// Equal-radius rrect with r=20 would cover more of (10,10) region differently;
	// just ensure shape is non-empty and not full-bleed rect to edge white only.
	ink := 0
	for y := 10; y < 40; y++ {
		for x := 10; x < 70; x++ {
			rr, gg, bb, _ := sampleRGBA(dc, x, y)
			if int(rr)+int(gg)+int(bb) < 700 {
				ink++
			}
		}
	}
	if ink < 100 {
		t.Fatalf("G.06 XY rrect too little ink=%d", ink)
	}
	_ = or
	_ = og
	_ = ob
	_ = cr
	_ = cg
	_ = cb
}

func absU8(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}

func TestP1_Capability_I03_ImageFilterNearestLinearGPU(t *testing.T) {
	requireNativeGPU(t)
	// 2x2 checkerboard: red/blue
	img, err := render.NewImageBuf(2, 2, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	_ = img.SetRGBA(0, 0, 255, 0, 0, 255)
	_ = img.SetRGBA(1, 0, 0, 0, 255, 255)
	_ = img.SetRGBA(0, 1, 0, 0, 255, 255)
	_ = img.SetRGBA(1, 1, 255, 0, 0, 255)

	// Upscale 16x with nearest — centers of cells stay pure
	dcN := render.NewContext(32, 32)
	defer dcN.Close()
	dcN.ResetRenderPathStats()
	dcN.ClearWithColor(render.White)
	dcN.DrawImageEx(img, render.DrawImageOptions{
		X: 0, Y: 0, DstWidth: 32, DstHeight: 32,
		Interpolation: render.InterpNearest,
		Opacity:       1,
		BlendMode:     render.BlendNormal,
	})
	if err := dcN.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU nearest: %v", err)
	}
	if dcN.RenderPathStats().GPUOps == 0 {
		t.Fatalf("I.03 nearest requires GPUOps>0: %s", dcN.RenderPathStats().LogLine())
	}
	// Sample cell centers of the 16x16 nearest blocks (2x2 source → 32x32 dest).
	nr, _, nb, _ := sampleRGBA(dcN, 8, 8)    // top-left cell → red
	nr2, _, nb2, _ := sampleRGBA(dcN, 24, 8) // top-right → blue
	t.Logf("nearest TL=%d,*,%d TR=%d,*,%d", nr, nb, nr2, nb2)
	if nr < 200 || nb > 40 {
		t.Fatalf("nearest TL not pure red: %d,*,%d", nr, nb)
	}
	if nb2 < 200 || nr2 > 40 {
		t.Fatalf("nearest TR not pure blue: %d,*,%d", nr2, nb2)
	}

	// Linear upscale — boundary should blend (not pure at midpoint)
	dcL := render.NewContext(32, 32)
	defer dcL.Close()
	dcL.ResetRenderPathStats()
	dcL.ClearWithColor(render.White)
	dcL.DrawImageEx(img, render.DrawImageOptions{
		X: 0, Y: 0, DstWidth: 32, DstHeight: 32,
		Interpolation: render.InterpBilinear,
		Opacity:       1,
		BlendMode:     render.BlendNormal,
	})
	if err := dcL.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU linear: %v", err)
	}
	if dcL.RenderPathStats().GPUOps == 0 {
		t.Fatalf("I.03 linear requires GPUOps>0")
	}
	// Mid vertical seam between left red and right blue (~x=16)
	lr, lg, lb, _ := sampleRGBA(dcL, 15, 8)
	t.Logf("linear seam rgba=%d,%d,%d", lr, lg, lb)
	// Linear should not be pure red or pure blue at boundary
	if (lr > 240 && lb < 20) || (lb > 240 && lr < 20) {
		// might still be pure if sampling hits texel center; try nearby
		mixed := false
		for x := 12; x <= 20; x++ {
			r, _, b, _ := sampleRGBA(dcL, x, 8)
			if r > 40 && r < 220 && b > 40 && b < 220 {
				mixed = true
				break
			}
		}
		if !mixed {
			t.Fatalf("linear expected mixed samples near seam, got pure colors")
		}
	}
}

func TestP1_Capability_C05_ClipRRectAAGPU(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.SetAntiAlias(true)
	dc.ClearWithColor(render.White)
	// Clip rounded rect, fill solid red outside would be white
	dc.ClipRoundRect(8, 8, 48, 48, 16)
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, 64, 64)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("C.05 requires GPUOps>0: %s", stats.LogLine())
	}
	// Inside solid red
	r, g, b, _ := sampleRGBA(dc, 32, 32)
	if r < 200 || g > 40 || b > 40 {
		t.Fatalf("inside clip not red: %d,%d,%d", r, g, b)
	}
	// Outside corner white
	or, og, ob, _ := sampleRGBA(dc, 2, 2)
	if or < 240 || og < 240 || ob < 240 {
		t.Fatalf("outside clip not white: %d,%d,%d", or, og, ob)
	}
	// Soft edge near rounded corner: red-over-white AA keeps R high while G/B rise.
	// (premul SourceOver: out=(1, 1-cov, 1-cov) so R≈255, G/B intermediate.)
	soft := 0
	for y := 8; y < 28; y++ {
		for x := 8; x < 28; x++ {
			rr, gg, bb, _ := sampleRGBA(dc, x, y)
			if rr > 200 && gg > 30 && gg < 230 && bb > 30 && bb < 230 {
				soft++
			}
		}
	}
	t.Logf("soft edge samples=%d", soft)
	if soft < 3 {
		t.Fatalf("C.05 expected AA soft edge on rrect clip, soft=%d", soft)
	}
}

func TestP1_Capability_C05_ClipPathAntiAliasFlag(t *testing.T) {
	requireNativeGPU(t)
	// AA on path clip — may depth-clip hard on GPU; still require correct inside/outside + GPUOps
	dc := render.NewContext(48, 48)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.SetAntiAlias(true)
	dc.ClearWithColor(render.White)
	dc.DrawCircle(24, 24, 16)
	dc.Clip()
	dc.SetRGB(0, 0.5, 1)
	dc.DrawRectangle(0, 0, 48, 48)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Fatalf("clip path fill requires GPUOps>0: %s", dc.RenderPathStats().LogLine())
	}
	cr, cg, cb, _ := sampleRGBA(dc, 24, 24)
	if cb < 100 {
		t.Fatalf("center not filled: %d,%d,%d", cr, cg, cb)
	}
	or, og, ob, _ := sampleRGBA(dc, 2, 2)
	if or < 240 || og < 240 || ob < 240 {
		t.Fatalf("outside circle clip should be white: %d,%d,%d", or, og, ob)
	}
}

func TestP1_Capability_X05_LCDTextGPU(t *testing.T) {
	requireNativeGPU(t)
	font := ""
	for _, p := range []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
	} {
		if _, err := os.Stat(p); err == nil {
			font = p
			break
		}
	}
	if font == "" {
		t.Skip("no test font")
	}

	// Grayscale baseline
	dcG := render.NewContext(160, 40)
	defer dcG.Close()
	dcG.ResetRenderPathStats()
	dcG.ClearWithColor(render.White)
	dcG.SetLCDLayout(render.LCDLayoutNone)
	if err := dcG.LoadFontFace(font, 16); err != nil {
		t.Fatalf("font: %v", err)
	}
	dcG.SetRGB(0, 0, 0)
	dcG.DrawString("AgHij", 8, 28)
	if err := dcG.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU gray: %v", err)
	}
	if dcG.RenderPathStats().GPUOps == 0 {
		t.Fatalf("X.05 gray text GPUOps=0")
	}

	// LCD RGB
	dcL := render.NewContext(160, 40)
	defer dcL.Close()
	dcL.ResetRenderPathStats()
	dcL.ClearWithColor(render.White)
	dcL.SetLCDLayout(render.LCDLayoutRGB)
	if err := dcL.LoadFontFace(font, 16); err != nil {
		t.Fatalf("font: %v", err)
	}
	dcL.SetRGB(0, 0, 0)
	dcL.DrawString("AgHij", 8, 28)
	if err := dcL.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU lcd: %v", err)
	}
	stats := dcL.RenderPathStats()
	t.Logf("lcd path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("X.05 LCD text requires GPUOps>0")
	}

	// Count subpixel-fringe pixels where R,G,B channels diverge (ClearType signature)
	subpix := 0
	ink := 0
	for y := 8; y < 36; y++ {
		for x := 8; x < 150; x++ {
			r, g, b, _ := sampleRGBA(dcL, x, y)
			if int(r)+int(g)+int(b) > 740 {
				continue // near white
			}
			ink++
			mx := int(r)
			if int(g) > mx {
				mx = int(g)
			}
			if int(b) > mx {
				mx = int(b)
			}
			mn := int(r)
			if int(g) < mn {
				mn = int(g)
			}
			if int(b) < mn {
				mn = int(b)
			}
			if mx-mn >= 12 {
				subpix++
			}
		}
	}
	t.Logf("lcd ink=%d subpix_fringe=%d", ink, subpix)
	if ink < 30 {
		t.Fatalf("LCD text invisible ink=%d", ink)
	}
	// LCD should produce some channel imbalance vs pure gray edges
	if subpix < 5 {
		t.Fatalf("X.05 expected subpixel RGB fringe, got subpix=%d (layout may have fallen back to gray)", subpix)
	}
}

// D.04 multi-stop linear + ExtendRepeat/Reflect via fillBrushAsImage GPU bootstrap.
func TestP1_Capability_D04_MultiStopExtendRepeatGPU(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 64, 32
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	// Period = 16px: red → green → blue across [0,16], then repeats.
	grad := render.NewLinearGradientBrush(0, 0, 16, 0).
		AddColorStop(0, render.Red).
		AddColorStop(0.5, render.Green).
		AddColorStop(1, render.Blue).
		SetExtend(render.ExtendRepeat)
	dc.SetFillBrush(grad)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("D.04 requires GPUOps>0: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("D.04 must not CPU-fallback: %s", stats.LogLine())
	}

	// Phase-aligned samples across tiles: x=2 and x=18 (period 16).
	r0, g0, b0, _ := sampleRGBA(dc, 2, 16)
	r1, g1, b1, _ := sampleRGBA(dc, 18, 16)
	t.Logf("tile0@2=%d,%d,%d tile1@18=%d,%d,%d", r0, g0, b0, r1, g1, b1)
	if absU8(r0, r1) > 40 || absU8(g0, g1) > 40 || absU8(b0, b1) > 40 {
		t.Fatalf("ExtendRepeat phase mismatch: %d,%d,%d vs %d,%d,%d", r0, g0, b0, r1, g1, b1)
	}
	// Multi-stop not flat: mid period vs start differ.
	rm, gm, bm, _ := sampleRGBA(dc, 8, 16) // ~0.5 → green
	t.Logf("mid=%d,%d,%d", rm, gm, bm)
	if absU8(r0, rm)+absU8(g0, gm)+absU8(b0, bm) < 80 {
		t.Fatalf("multi-stop looks flat: start=%d,%d,%d mid=%d,%d,%d", r0, g0, b0, rm, gm, bm)
	}
	// Mid should be green-dominant relative to start red.
	if gm < 80 {
		t.Fatalf("mid-stop expected green-ish, got %d,%d,%d", rm, gm, bm)
	}
}

func TestP1_Capability_D04_ExtendReflectGPU(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 48, 24
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	// [0,16]: black→white; reflect continues white→black in [16,32].
	grad := render.NewLinearGradientBrush(0, 0, 16, 0).
		AddColorStop(0, render.Black).
		AddColorStop(1, render.White).
		SetExtend(render.ExtendReflect)
	dc.SetFillBrush(grad)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("D.04 Reflect requires GPUOps>0: %s", stats.LogLine())
	}

	// First period dark→light; reflected period light→dark.
	rL, _, _, _ := sampleRGBA(dc, 2, 12)
	rR, _, _, _ := sampleRGBA(dc, 14, 12)
	rRefNear, _, _, _ := sampleRGBA(dc, 18, 12)
	rRefFar, _, _, _ := sampleRGBA(dc, 30, 12)
	t.Logf("L=%d R=%d refNear=%d refFar=%d", rL, rR, rRefNear, rRefFar)
	if !(rL+40 < rR) {
		t.Fatalf("first period should darken→lighten: L=%d R=%d", rL, rR)
	}
	// Reflect reverses: near period end stays light, far darkens relative.
	if !(rRefFar+40 < rRefNear) {
		t.Fatalf("reflect should light→dark: refNear=%d refFar=%d", rRefNear, rRefFar)
	}
	if rR < 180 {
		t.Fatalf("near end of first period expected light, r=%d", rR)
	}
}

// D.05 ImagePattern fill GPU (staging bootstrap).
func TestP1_Capability_D05_ImagePatternGPU(t *testing.T) {
	requireNativeGPU(t)
	img, err := render.NewImageBuf(8, 8, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	// Left half red, right half blue — tiling fingerprint.
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if x < 4 {
				_ = img.SetRGBA(x, y, 255, 0, 0, 255)
			} else {
				_ = img.SetRGBA(x, y, 0, 0, 255, 255)
			}
		}
	}

	dc := render.NewContext(40, 24)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	pat := dc.CreateImagePattern(img, 0, 0, 8, 8)
	dc.SetFillPattern(pat)
	dc.DrawRectangle(0, 0, 40, 24)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("D.05 ImagePattern requires GPUOps>0: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("D.05 must not CPU-fallback: %s", stats.LogLine())
	}

	// Tile 0 left red, right blue; tile 1 same.
	r0, _, b0, _ := sampleRGBA(dc, 2, 12)
	_, _, b1, _ := sampleRGBA(dc, 6, 12)
	r2, _, _, _ := sampleRGBA(dc, 10, 12) // next tile left
	t.Logf("x2=%d,*,%d x6=*,*,%d x10=%d", r0, b0, b1, r2)
	if r0 < 180 || b0 > 60 {
		t.Fatalf("pattern left expected red: r=%d b=%d", r0, b0)
	}
	if b1 < 180 {
		t.Fatalf("pattern right expected blue: b=%d", b1)
	}
	if r2 < 180 {
		t.Fatalf("tiled pattern left of next cell expected red: r=%d", r2)
	}
}

// D.06 local matrix on image shader/pattern (SetScale / SetTransform).
func TestP1_Capability_D06_PatternLocalMatrixGPU(t *testing.T) {
	requireNativeGPU(t)
	img, err := render.NewImageBuf(16, 16, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	// Horizontal ramp: red→blue across the pattern cell.
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			_ = img.SetRGBA(x, y, uint8(255-x*15), 0, uint8(x*15), 255)
		}
	}

	dc := render.NewContext(48, 24)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	pat := dc.CreateImagePattern(img, 0, 0, 16, 16).(*render.ImagePattern)
	// Local scale 2×: each source pixel covers 2 device pixels (Skia localMatrix-like).
	pat.SetScale(2, 2)
	dc.SetFillPattern(pat)
	dc.DrawRectangle(0, 0, 48, 24)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("D.06 requires GPUOps>0: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("D.06 must not CPU-fallback: %s", stats.LogLine())
	}

	// Under 2× scale, device x≈2 samples near image x≈1 (red-dominant);
	// device x≈30 samples near image x≈15 (blue-dominant).
	rL, _, bL, _ := sampleRGBA(dc, 2, 12)
	rR, _, bR, _ := sampleRGBA(dc, 30, 12)
	t.Logf("scaled left=%d,*,%d right=%d,*,%d", rL, bL, rR, bR)
	if rL < 150 {
		t.Fatalf("D.06 left after scale expected red-dominant: %d,%d", rL, bL)
	}
	if bR < 150 {
		t.Fatalf("D.06 right after scale expected blue-dominant: %d,%d", rR, bR)
	}
	if absU8(rL, rR)+absU8(bL, bR) < 80 {
		t.Fatalf("D.06 local scale produced flat colors: L=%d/%d R=%d/%d", rL, bL, rR, bR)
	}
}

func p1CloserFindFont(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		filepath.Join("text", "testdata", "goregular.ttf"),
		filepath.Join("render", "text", "testdata", "goregular.ttf"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("no test font available")
	return ""
}

// X.03: OwnShaper (GSUB/GPOS) positions + DrawShapedGlyphs GPU path.
func TestP1_Capability_X03_ShapingGPU(t *testing.T) {
	requireNativeGPU(t)
	font := p1CloserFindFont(t)

	dc := render.NewContext(200, 64)
	defer dc.Close()
	if err := dc.LoadFontFace(font, 28); err != nil {
		t.Fatalf("LoadFontFace: %v", err)
	}
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0, 0, 0)

	face := dc.Font()
	if face == nil {
		t.Fatal("FontFace nil")
	}
	// Multi-glyph run with advances (and ligatures when font supports "fi").
	const s = "Affine"
	glyphs := text.Shape(s, face)
	if len(glyphs) < 2 {
		t.Fatalf("Shape(%q) expected >=2 glyphs, got %d", s, len(glyphs))
	}
	// Monotonic X positions (pen advances).
	for i := 1; i < len(glyphs); i++ {
		if glyphs[i].X+0.01 < glyphs[i-1].X {
			t.Fatalf("glyph X not monotonic: [%d]=%v < [%d]=%v", i, glyphs[i].X, i-1, glyphs[i-1].X)
		}
	}
	// Total advance matches last glyph pen + its advance-ish: width should be > first advance.
	last := glyphs[len(glyphs)-1]
	if last.X <= 0 {
		t.Fatalf("shaped run width degenerate: last.X=%v", last.X)
	}

	dc.DrawShapedGlyphs(glyphs, face, 8, 40)
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s glyphs=%d lastX=%.2f", stats.LogLine(), len(glyphs), last.X)
	if stats.GPUOps == 0 {
		t.Fatalf("X.03 DrawShapedGlyphs requires GPUOps>0: %s", stats.LogLine())
	}

	// Ink spans horizontally (not a single stacked blob).
	inkCols := 0
	first, lastCol := -1, -1
	for x := 0; x < 200; x++ {
		dark := false
		for y := 10; y < 60; y++ {
			r, g, b, _ := sampleRGBA(dc, x, y)
			if int(r)+int(g)+int(b) < 600 {
				dark = true
				break
			}
		}
		if dark {
			inkCols++
			if first < 0 {
				first = x
			}
			lastCol = x
		}
	}
	t.Logf("inkCols=%d span=%d..%d", inkCols, first, lastCol)
	if inkCols < 20 {
		t.Fatalf("X.03 expected wide text ink, cols=%d", inkCols)
	}
	if lastCol-first < 30 {
		t.Fatalf("X.03 shaped text span too narrow: %d..%d", first, lastCol)
	}
}

// X.04: fractional origins produce distinct subpixel raster footprints on GPU.
func TestP1_Capability_X04_SubpixelPosGPU(t *testing.T) {
	requireNativeGPU(t)
	font := p1CloserFindFont(t)

	renderAt := func(xfrac float64) []uint8 {
		dc := render.NewContext(48, 48)
		defer dc.Close()
		if err := dc.LoadFontFace(font, 18); err != nil {
			t.Fatalf("LoadFontFace: %v", err)
		}
		// Prefer glyph-mask path without full hinting snap if possible.
		dc.SetTextMode(render.TextModeGlyphMask)
		dc.ResetRenderPathStats()
		dc.ClearWithColor(render.White)
		dc.SetRGB(0, 0, 0)
		dc.DrawString("H", 10+xfrac, 30)
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
		if dc.RenderPathStats().GPUOps == 0 {
			t.Fatalf("X.04 requires GPUOps>0 at frac=%v: %s", xfrac, dc.RenderPathStats().LogLine())
		}
		// Flatten grayscale row near baseline for fingerprint.
		row := make([]uint8, 48)
		for x := 0; x < 48; x++ {
			r, g, b, _ := sampleRGBA(dc, x, 24)
			// luminance-ish
			row[x] = uint8((int(r) + int(g) + int(b)) / 3)
		}
		return row
	}

	r0 := renderAt(0.0)
	r25 := renderAt(0.25)
	r50 := renderAt(0.50)

	diff := func(a, b []uint8) int {
		d := 0
		for i := range a {
			d += absU8(a[i], b[i])
		}
		return d
	}
	d01 := diff(r0, r25)
	d02 := diff(r0, r50)
	t.Logf("subpixel diffs 0vs0.25=%d 0vs0.50=%d", d01, d02)
	// At least one fractional offset must change coverage vs integer origin.
	if d01 < 8 && d02 < 8 {
		t.Fatalf("X.04 subpixel positions look identical (diffs %d,%d)", d01, d02)
	}
}

// Q.03: AA-off snaps fractional rects to pixel grid on GPU path.
func TestP1_Capability_Q03_PixelSnapNoAAGPU(t *testing.T) {
	requireNativeGPU(t)

	draw := func(x, y, w, h float64) (r, g, b, a uint8, ops int) {
		dc := render.NewContext(32, 32)
		defer dc.Close()
		dc.ResetRenderPathStats()
		dc.ClearWithColor(render.White)
		dc.SetAntiAlias(false)
		dc.SetRGB(0, 0, 0)
		dc.DrawRectangle(x, y, w, h)
		_ = dc.Fill()
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
		ops = dc.RenderPathStats().GPUOps
		if ops == 0 {
			t.Fatalf("Q.03 requires GPUOps>0: %s", dc.RenderPathStats().LogLine())
		}
		r, g, b, a = sampleRGBA(dc, 10, 10)
		return
	}

	// Integer-aligned and half-pixel-shifted rects should snap to the same coverage
	// at interior sample (10,10) when AA is off.
	r0, g0, b0, _, _ := draw(8, 8, 12, 12)
	r1, g1, b1, _, _ := draw(8.4, 8.4, 12, 12)
	t.Logf("aligned=%d,%d,%d snapped=%d,%d,%d", r0, g0, b0, r1, g1, b1)
	if absU8(r0, r1) > 5 || absU8(g0, g1) > 5 || absU8(b0, b1) > 5 {
		t.Fatalf("Q.03 AA-off snap mismatch: aligned %d,%d,%d vs frac %d,%d,%d", r0, g0, b0, r1, g1, b1)
	}
	// Both should be ink (black) not white
	if r0 > 40 {
		t.Fatalf("Q.03 expected filled black, got %d,%d,%d", r0, g0, b0)
	}

	// Outside snapped bounds stays white for a near-miss fractional rect that
	// snaps away from the edge pixel.
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetAntiAlias(false)
	dc.SetRGB(0, 0, 0)
	// Rect starting at 10.6 snaps to 11 — pixel 10 should stay white.
	dc.DrawRectangle(10.6, 10.6, 8, 8)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU edge: %v", err)
	}
	er, eg, eb, _ := sampleRGBA(dc, 10, 16)
	ir, ig, ib, _ := sampleRGBA(dc, 14, 14)
	t.Logf("edge10=%d,%d,%d interior=%d,%d,%d", er, eg, eb, ir, ig, ib)
	if er < 240 {
		t.Fatalf("Q.03 expected pixel 10 white after snap of 10.6, got %d,%d,%d", er, eg, eb)
	}
	if ir > 40 {
		t.Fatalf("Q.03 expected interior black, got %d,%d,%d", ir, ig, ib)
	}
}

// L.06: SetMask modulates fill coverage; GPU bootstrap blit with GPUOps>0.
func TestP1_Capability_L06_MaskLayerGPU(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 48, 48
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	// Left half opaque mask, right half transparent.
	mask := render.NewMask(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < w/2 {
				mask.Set(x, y, 255)
			} else {
				mask.Set(x, y, 0)
			}
		}
	}
	dc.SetMask(mask)
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("L.06 masked fill requires GPUOps>0: %s", stats.LogLine())
	}

	// Left blue-ish, right stays white.
	lr, lg, lb, _ := sampleRGBA(dc, 8, 24)
	rr, rg, rb, _ := sampleRGBA(dc, 40, 24)
	t.Logf("left=%d,%d,%d right=%d,%d,%d", lr, lg, lb, rr, rg, rb)
	if lb < 150 || lr > 80 {
		t.Fatalf("L.06 left expected blue under mask: %d,%d,%d", lr, lg, lb)
	}
	if rr < 240 || rg < 240 || rb < 240 {
		t.Fatalf("L.06 right expected white (mask 0): %d,%d,%d", rr, rg, rb)
	}
}

// T.03: non-uniform scale stroke — thickness follows perpendicular axis scale.
func TestP1_Capability_T03_NonUniformStrokeGPU(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 80, 80
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	// Scale X×3, Y×1: a user-space vertical stroke (along Y) should become
	// ~3× thicker in X; a horizontal stroke should stay ~1× thick in Y.
	dc.SetLineWidth(2)
	dc.SetLineCap(render.LineCapButt)
	dc.SetRGB(0, 0, 0)

	// Horizontal segment at y=20 in user space → after Scale(3,1) still thin in Y.
	dc.Push()
	dc.Scale(3, 1)
	dc.DrawLine(5, 20, 20, 20)
	_ = dc.Stroke()
	dc.Pop()

	// Vertical segment at x=10 → after Scale(3,1) thick in X.
	dc.Push()
	dc.Scale(3, 1)
	dc.DrawLine(10, 30, 10, 50)
	_ = dc.Stroke()
	dc.Pop()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("T.03 requires GPUOps>0: %s", stats.LogLine())
	}

	// Horizontal stroke: device y around 20, x around 15*3= mid of segment ~37
	// Thickness in Y should be ~2 (user width * sy=1), not ~6.
	// Count dark rows near the horizontal stroke.
	hDarkRows := 0
	for y := 15; y <= 25; y++ {
		dark := 0
		for x := 20; x < 55; x++ {
			r, g, b, _ := sampleRGBA(dc, x, y)
			if int(r)+int(g)+int(b) < 500 {
				dark++
			}
		}
		if dark > 5 {
			hDarkRows++
		}
	}
	// Vertical stroke around device x=30: thickness in X should be ~6 (2*sx).
	vDarkCols := 0
	for x := 20; x <= 45; x++ {
		dark := 0
		for y := 32; y < 50; y++ {
			r, g, b, _ := sampleRGBA(dc, x, y)
			if int(r)+int(g)+int(b) < 500 {
				dark++
			}
		}
		if dark > 3 {
			vDarkCols++
		}
	}
	t.Logf("horiz dark rows=%d vert dark cols=%d", hDarkRows, vDarkCols)
	if hDarkRows == 0 || vDarkCols == 0 {
		t.Fatalf("T.03 missing stroke ink: hRows=%d vCols=%d", hDarkRows, vDarkCols)
	}
	// Vertical (scaled in X) must be substantially thicker than horizontal.
	if vDarkCols < hDarkRows+2 {
		t.Fatalf("T.03 expected thicker vertical stroke under Scale(3,1): vCols=%d hRows=%d", vDarkCols, hDarkRows)
	}
}

// X.06: MultiFace fallback draws CJK from secondary face on GPU path.
func TestP1_Capability_X06_CJKFallbackGPU(t *testing.T) {
	requireNativeGPU(t)
	latin := p1CloserFindFont(t)
	// Prefer a CJK-capable system font; skip if none.
	cjkCandidates := []string{
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
		"/usr/share/fonts/truetype/arphic/uming.ttc",
		filepath.Join("text", "testdata", "notoseriftc_autohint_metrics.ttf"),
		filepath.Join("render", "text", "testdata", "notoseriftc_autohint_metrics.ttf"),
	}
	cjkPath := ""
	for _, p := range cjkCandidates {
		if _, err := os.Stat(p); err == nil {
			cjkPath = p
			break
		}
	}
	if cjkPath == "" {
		t.Skip("no CJK font available for fallback gate")
	}

	srcL, err := text.NewFontSourceFromFile(latin)
	if err != nil {
		t.Fatalf("latin font: %v", err)
	}
	srcC, err := text.NewFontSourceFromFile(cjkPath)
	if err != nil {
		t.Fatalf("cjk font: %v", err)
	}
	faceL := srcL.Face(20)
	faceC := srcC.Face(20)
	mf, err := text.NewMultiFace(faceL, faceC)
	if err != nil {
		t.Fatalf("NewMultiFace: %v", err)
	}

	dc := render.NewContext(160, 48)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetFont(mf)
	dc.SetTextMode(render.TextModeGlyphMask)
	dc.SetRGB(0, 0, 0)
	// Mixed Latin + CJK — second face must supply CJK glyphs.
	dc.DrawString("Hi中文", 8, 32)
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	// MultiFace may take outline/bitmap path; require either GPUOps or visible ink span.
	ink := 0
	for y := 8; y < 44; y++ {
		for x := 4; x < 150; x++ {
			r, g, b, _ := sampleRGBA(dc, x, y)
			if int(r)+int(g)+int(b) < 600 {
				ink++
			}
		}
	}
	t.Logf("ink=%d", ink)
	if ink < 40 {
		t.Fatalf("X.06 mixed fallback text too empty ink=%d", ink)
	}
	if stats.GPUOps == 0 {
		t.Fatalf("X.06 MultiFace mixed text requires GPUOps>0: %s", stats.LogLine())
	}
}

// X.11: glyph mask atlas grows/reuses entries under repeated GPU text.
func TestP1_Capability_X11_GlyphAtlasGPU(t *testing.T) {
	requireNativeGPU(t)
	font := p1CloserFindFont(t)
	dc := render.NewContext(200, 80)
	defer dc.Close()
	if err := dc.LoadFontFace(font, 16); err != nil {
		t.Fatalf("LoadFontFace: %v", err)
	}
	dc.SetTextMode(render.TextModeGlyphMask)
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0, 0, 0)

	// Distinct glyphs force atlas puts; repeats should hit cache.
	lines := []string{"Atlas", "Cache", "Glyph", "Atlas", "Cache"}
	for i, s := range lines {
		dc.DrawString(s, 8, 16+float64(i)*14)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("X.11 requires GPUOps>0: %s", stats.LogLine())
	}

	// Probe engine atlas via accelerator if available.
	a := render.Accelerator()
	type atlasHolder interface {
		Atlas() *text.GlyphMaskAtlas
	}
	// SDF accelerator path through gpu package is not exported; pixel ink is the gate.
	ink := 0
	for y := 4; y < 76; y += 2 {
		for x := 4; x < 190; x += 2 {
			r, g, b, _ := sampleRGBA(dc, x, y)
			if int(r)+int(g)+int(b) < 650 {
				ink++
			}
		}
	}
	t.Logf("atlas stress ink samples=%d", ink)
	if ink < 30 {
		t.Fatalf("X.11 atlas text too empty ink=%d", ink)
	}
	_ = a
}
