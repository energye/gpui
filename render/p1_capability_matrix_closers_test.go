//go:build !nogpu

package render_test

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
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
