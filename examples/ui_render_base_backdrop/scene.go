package main

import (
	"fmt"
	"math"
	"os"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH = 430.0, 232.0
	// Content region inside each card: the dark panel carries the title on
	// top, the backdrop demo lives in the inset area (ui_render_base_* house
	// style).
	contentX0, contentY0 = 12.0, 36.0
	contentW, contentH   = cardW - 2*contentX0, cardH - contentY0 - 12.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// Palette. Flat pixel probes read these same constants, so a palette edit can
// never desync the drawn pixels from the asserted ones.
var (
	colCardBG  = [3]float64{0.12, 0.135, 0.165}
	colTitle   = [3]float64{0.80, 0.86, 0.94}
	colStripeA = [3]float64{0.18, 0.78, 0.70} // teal stripe
	colStripeB = [3]float64{0.30, 0.55, 0.95} // blue stripe
	colChip    = [3]float64{1, 1, 1}          // sharp child drawn after Pop
)

// ---- card geometry (content-local) ----

const (
	stripeW = 24.0 // stripe period

	// A: full-surface backdrop straight on the base canvas, dim panel + chip
	// drawn inside the layer.
	aPanelX, aPanelY, aPanelW, aPanelH = 60.0, 24.0, 286.0, 136.0
	aChipX, aChipY, aChipW, aChipH     = 150.0, 120.0, 100.0, 30.0

	// B: isolated card (repaint boundary): stripes live inside the glass panel
	// only, so the full-surface blur has visible work only there.
	bPanelX, bPanelY, bPanelW, bPanelH = 60.0, 24.0, 286.0, 136.0
	bBarW, bBarPeriod                  = 10.0, 20.0
	bTintAlpha                         = 0.15
	bChipX, bChipY, bChipW, bChipH     = 150.0, 120.0, 100.0, 30.0

	// C: clipped dim panel straight on the base canvas.
	cPanelX, cPanelY, cPanelW, cPanelH = 80.0, 20.0, 246.0, 144.0

	// D: nested backdrop inside an outer isolated layer.
	dOuterW                        = 330.0 // outer bands width
	dStripX, dStripW               = 100.0, 206.0
	dChipX, dChipY, dChipW, dChipH = 336.0, 36.0, 64.0, 30.0
	dBandH                         = contentH / 2
	bDimAlpha                      = 0.5
)

// stripeColorAt resolves the stripe color at a content-local x (analytic
// oracle: the probes and the painter share this function).
func stripeColorAt(x float64) [3]float64 {
	if int(math.Floor(x/stripeW))%2 == 0 {
		return colStripeA
	}
	return colStripeB
}

// dimmed returns c after a black dim of alpha a (the panel fill is black).
func dimmed(c [3]float64, a float64) [3]float64 {
	return [3]float64{c[0] * (1 - a), c[1] * (1 - a), c[2] * (1 - a)}
}

// ---- pixel probe spec ----

// band is an independent channel bound on the sampled pixel (0..1 scale), on
// top of the oracle match.
type band struct {
	ch       int
	min, max float64
}

// b8 builds a band in byte units for readability.
func b8(ch int, lo, hi byte) band {
	return band{ch: ch, min: float64(lo) / 255, max: float64(hi) / 255}
}

// probeSpec is one end-of-run pixel probe: card index, content-local sample
// point, the expected-color oracle and channel bands.
type probeSpec struct {
	key, desc string
	card      int
	pt        render.Point
	want      func(ox, oy, dpr float64, devPX, devPY int) (r, g, b float64)
	bands     []band
}

const tolSolid = 8.0 / 255

var probeSpecs []probeSpec

func init() {
	solid := func(c [3]float64) func(float64, float64, float64, int, int) (float64, float64, float64) {
		return func(float64, float64, float64, int, int) (float64, float64, float64) {
			return c[0], c[1], c[2]
		}
	}
	add := func(key, desc string, card int, pt render.Point,
		want func(float64, float64, float64, int, int) (float64, float64, float64), bands ...band) {
		probeSpecs = append(probeSpecs, probeSpec{key, desc, card, pt, want, bands})
	}

	bgBands := []band{b8(0, 20, 42), b8(1, 24, 45), b8(2, 32, 52)}

	// A: base-level backdrop. Panel dims the stripes; the chip is drawn into
	// the layer after the dim fill and must keep its exact color.
	add("a_stripe_out", "A 面板外条纹原样 = teal", 0, render.Pt(14, 90),
		solid(colStripeA), b8(1, 190, 210), b8(0, 35, 60), b8(2, 165, 195))
	add("a_teal_in", "A 面板内 teal 被压暗 50%", 0, render.Pt(60, 90),
		solid(dimmed(colStripeA, bDimAlpha)), b8(1, 90, 115), b8(0, 12, 38), b8(2, 72, 96))
	add("a_blue_in", "A 面板内 blue 条纹被压暗 50%", 0, render.Pt(90, 90),
		solid(dimmed(colStripeB, bDimAlpha)), b8(2, 105, 140), b8(0, 25, 52), b8(1, 55, 85))
	add("a_above", "A 面板上方（y<24）条纹原样 = teal", 0, render.Pt(200, 10),
		solid(colStripeA), b8(1, 190, 210), b8(0, 35, 60), b8(2, 165, 195))
	add("a_chip", "A 层内后画 chip 不受压暗 = 白", 0, render.Pt(200, 135),
		solid(colChip), b8(0, 250, 255), b8(1, 250, 255), b8(2, 250, 255))

	// B: isolated card, backdrop + Blur(6) + 15% white tint. Two stripe centers
	// converge to the same value only when the blur actually ran.
	glassAvg := func() [3]float64 {
		blurred := 0.5 // black/white stripe mean inside the kernel
		return [3]float64{
			blurred*(1-bTintAlpha) + 1*bTintAlpha,
			blurred*(1-bTintAlpha) + 1*bTintAlpha,
			blurred*(1-bTintAlpha) + 1*bTintAlpha,
		}
	}
	add("b_black_center", "B 黑条纹中心被抹平 + 15% 白 ≈ 0.575", 1, render.Pt(110, 90),
		solid(glassAvg()), b8(0, 120, 175), b8(1, 120, 175), b8(2, 120, 175))
	add("b_white_center", "B 白条纹中心被抹平 + 15% 白 ≈ 0.575", 1, render.Pt(120, 90),
		solid(glassAvg()), b8(0, 120, 175), b8(1, 120, 175), b8(2, 120, 175))
	add("b_bg_out", "B 玻璃面板外卡片底色原样（平色模糊不改变）", 1, render.Pt(14, 90),
		solid(colCardBG), bgBands...)
	add("b_chip", "B Pop 之后画的子内容依旧锐利 = 白", 1, render.Pt(200, 135),
		solid(colChip), b8(0, 250, 255), b8(1, 250, 255), b8(2, 250, 255))

	// C: base-level backdrop under an active clip. Outside the clip the parent
	// must survive (that is the full-surface Pop contract).
	add("c_teal_in", "C 裁剪面板内 teal 压暗 50%", 2, render.Pt(100, 90),
		solid(dimmed(colStripeA, bDimAlpha)), b8(1, 90, 115), b8(0, 12, 38), b8(2, 72, 96))
	add("c_out_left", "C 面板左侧条纹原样 = teal", 2, render.Pt(14, 90),
		solid(colStripeA), b8(1, 190, 210), b8(0, 35, 60), b8(2, 165, 195))
	add("c_out_right", "C 面板右侧条纹原样 = blue", 2, render.Pt(380, 90),
		solid(colStripeB), b8(2, 225, 255), b8(0, 55, 90), b8(1, 115, 155))
	add("c_edge_out", "C 裁剪边界外 4px = blue 原样", 2, render.Pt(76, 90),
		solid(colStripeB), b8(2, 225, 255), b8(0, 55, 90), b8(1, 115, 155))
	add("c_edge_in", "C 裁剪边界内 4px = blue 压暗 50%", 2, render.Pt(84, 90),
		solid(dimmed(colStripeB, bDimAlpha)), b8(2, 105, 140), b8(0, 25, 52), b8(1, 55, 85))
	add("c_above", "C 面板上方（y<20）条纹原样 = teal", 2, render.Pt(200, 8),
		solid(colStripeA), b8(1, 190, 210), b8(0, 35, 60), b8(2, 165, 195))

	// D: nested backdrop inside an outer isolated layer. The nested snapshot
	// must see the outer layer's bands, not the card background.
	add("d_band_out", "D 外层 teal 带在暗条外 = teal 原样", 3, render.Pt(20, 40),
		solid(colStripeA), b8(1, 190, 210), b8(0, 35, 60), b8(2, 165, 195))
	add("d_band_in", "D 暗条内 teal 带压暗 50%（快照看到外层图层）", 3, render.Pt(140, 40),
		solid(dimmed(colStripeA, bDimAlpha)), b8(1, 90, 115), b8(0, 12, 38), b8(2, 72, 96))
	add("d_lower_in", "D 暗条内 blue 带压暗 50%", 3, render.Pt(140, 140),
		solid(dimmed(colStripeB, bDimAlpha)), b8(2, 105, 140), b8(0, 25, 52), b8(1, 55, 85))
	add("d_lower_out", "D 暗条右缘外 blue 带原样 = blue", 3, render.Pt(320, 140),
		solid(colStripeB), b8(2, 225, 255), b8(0, 55, 90), b8(1, 115, 155))
	add("d_chip", "D 外层 Pop 之后画的 chip = 白", 3, render.Pt(368, 51),
		solid(colChip), b8(0, 250, 255), b8(1, 250, 255), b8(2, 250, 255))
	add("d_card_bg", "D 外层带外（x>330）卡片底色原样", 3, render.Pt(390, 90),
		solid(colCardBG), bgBands...)
}

// ---- logic probes (value-level, run once at startup) ----

func probeCtxOpaque(w, h int) *render.Context {
	dc := render.NewContext(w, h)
	dc.ClearWithColor(render.White)
	return dc
}

func sampleRGBA(dc *render.Context, x, y int) (r, g, b, a float64) {
	img := dc.Image()
	r32, g32, b32, a32 := img.At(x, y).RGBA()
	return float64(r32>>8) / 255, float64(g32>>8) / 255, float64(b32>>8) / 255, float64(a32>>8) / 255
}

func near4(r, g, b, a, wr, wg, wb, wa, tol float64) bool {
	return math.Abs(r-wr) <= tol && math.Abs(g-wg) <= tol && math.Abs(b-wb) <= tol && math.Abs(a-wa) <= tol
}

func fillRect(dc *render.Context, x, y, w, h float64, c [3]float64) {
	dc.SetRGBA(c[0], c[1], c[2], 1)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}

// dimRect fills a black overlay at the given alpha (the layer dim verb).
func dimRect(dc *render.Context, x, y, w, h, alpha float64) {
	dc.SetRGBA(0, 0, 0, alpha)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}

var probeDetail string

func setDetail(format string, args ...any) { probeDetail = fmt.Sprintf(format, args...) }

func runLogicProbes() (probes, failed []string) {
	check := func(key string, pass bool) {
		probes = append(probes, key)
		if !pass {
			failed = append(failed, key)
			if probeDetail != "" {
				fmt.Fprintf(os.Stderr, "logic %s FAILED: %s\n", key, probeDetail)
			}
		}
		probeDetail = ""
	}
	const tol = 2.0 / 255

	// 1) An untouched backdrop layer is an identity: the snapshot composites
	// back over the parent unchanged.
	{
		dc := probeCtxOpaque(60, 60)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		dc.PushBackdropLayer(render.BlendNormal, 1)
		dc.PopLayer()
		r, g, b, a := sampleRGBA(dc, 30, 30)
		check("backdrop_empty_is_identity", near4(r, g, b, a, 1, 0, 0, 1, tol))
		_ = dc.Close()
	}

	// 2) A black 50% fill inside the layer dims the snapshot below it.
	{
		dc := probeCtxOpaque(60, 60)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		dc.PushBackdropLayer(render.BlendNormal, 1)
		dimRect(dc, 0, 0, 60, 60, 0.5)
		dc.PopLayer()
		r, g, b, a := sampleRGBA(dc, 30, 30)
		setDetail("got=(%.3f,%.3f,%.3f,%.3f) want=(0.5,0,0,1)", r, g, b, a)
		check("backdrop_dims_content", near4(r, g, b, a, 0.5, 0, 0, 1, tol))
		_ = dc.Close()
	}

	// 3) The dim only covers the fill rect; the rest of the snapshot is
	// composited back untouched.
	{
		dc := probeCtxOpaque(60, 60)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		dc.PushBackdropLayer(render.BlendNormal, 1)
		dc.SetRGBA(0, 0, 0, 0.5)
		dc.DrawRectangle(0, 0, 30, 60)
		_ = dc.Fill()
		dc.PopLayer()
		lr, lg, lb, _ := sampleRGBA(dc, 15, 30)
		rr, rg, rb, _ := sampleRGBA(dc, 45, 30)
		check("backdrop_outside_fill_untouched",
			near4(lr, lg, lb, 1, 0.5, 0, 0, 1, tol) && near4(rr, rg, rb, 1, 1, 0, 0, 1, tol))
		_ = dc.Close()
	}

	// 4) The layer opacity scales the composite: 0.5 over the parent.
	{
		dc := probeCtxOpaque(60, 60)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		dc.PushBackdropLayer(render.BlendNormal, 0.5)
		fillRect(dc, 0, 0, 60, 60, [3]float64{0, 0, 1})
		dc.PopLayer()
		r, g, b, a := sampleRGBA(dc, 30, 30)
		check("backdrop_pop_opacity_half", near4(r, g, b, a, 0.5, 0, 0.5, 1, tol))
		_ = dc.Close()
	}

	// 5) Content drawn into the layer after the dim fill keeps source alpha
	// (opaque chip stays opaque over the dimmed snapshot).
	{
		dc := probeCtxOpaque(60, 60)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		dc.PushBackdropLayer(render.BlendNormal, 1)
		dimRect(dc, 0, 0, 60, 60, 0.5)
		fillRect(dc, 10, 10, 20, 20, [3]float64{1, 1, 1})
		dc.PopLayer()
		dr, dg, db, _ := sampleRGBA(dc, 45, 30)
		cr, cg, cb, ca := sampleRGBA(dc, 20, 20)
		setDetail("dim=(%.3f,%.3f,%.3f) chip=(%.3f,%.3f,%.3f,%.3f)", dr, dg, db, cr, cg, cb, ca)
		check("backdrop_layer_child_stays_sharp",
			near4(dr, dg, db, 1, 0.5, 0, 0, 1, tol) && near4(cr, cg, cb, ca, 1, 1, 1, 1, tol))
		_ = dc.Close()
	}

	// 6) With a clip active, the layer only carries the clipped dim; the
	// parent outside the clip is restored by the Pop composite.
	{
		dc := probeCtxOpaque(60, 60)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		dc.ClipRect(0, 0, 30, 60)
		dc.PushBackdropLayer(render.BlendNormal, 1)
		dimRect(dc, 0, 0, 60, 60, 0.5)
		dc.PopLayer()
		dc.ResetClip()
		lr, lg, lb, _ := sampleRGBA(dc, 15, 30)
		rr, rg, rb, _ := sampleRGBA(dc, 45, 30)
		setDetail("in=(%.3f,%.3f,%.3f) out=(%.3f,%.3f,%.3f)", lr, lg, lb, rr, rg, rb)
		check("backdrop_clip_keeps_outside",
			near4(lr, lg, lb, 1, 0.5, 0, 0, 1, tol) && near4(rr, rg, rb, 1, 1, 0, 0, 1, tol))
		_ = dc.Close()
	}

	// 7) Blur softens a hard parent edge: the seam mixes both sides.
	{
		dc := probeCtxOpaque(60, 60)
		fillRect(dc, 0, 0, 30, 60, [3]float64{1, 0, 0})
		fillRect(dc, 30, 0, 30, 60, [3]float64{0, 0, 1})
		dc.PushBackdropLayer(render.BlendNormal, 1)
		dc.ApplyBlur(4)
		dc.PopLayer()
		sr, sg, sb, _ := sampleRGBA(dc, 30, 30)
		lr, lg, lb, _ := sampleRGBA(dc, 10, 30)
		check("backdrop_blur_softens_edge",
			near4(sr, sg, sb, 1, 0.5, 0, 0.5, 1, 0.2) && near4(lr, lg, lb, 1, 1, 0, 0, 1, 0.1))
		_ = dc.Close()
	}

	// 8) Blur only touches the existing snapshot: content drawn after it (the
	// sharp child) composites at full opacity.
	{
		dc := probeCtxOpaque(60, 60)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		dc.PushBackdropLayer(render.BlendNormal, 1)
		dc.ApplyBlur(4)
		fillRect(dc, 10, 10, 20, 20, [3]float64{1, 1, 1})
		dc.PopLayer()
		r, g, b, a := sampleRGBA(dc, 20, 20)
		check("backdrop_blur_keeps_child_sharp", near4(r, g, b, a, 1, 1, 1, 1, tol))
		_ = dc.Close()
	}

	// 9) Nested backdrop: the inner snapshot sees the enclosing isolation
	// layer's content, so dimming the band keeps its hue.
	{
		dc := probeCtxOpaque(60, 60)
		dc.PushLayerIsolated(1)
		fillRect(dc, 0, 0, 60, 12, [3]float64{0, 1, 0})
		dc.PushBackdropLayer(render.BlendNormal, 1)
		dc.SetRGBA(0, 0, 0, 0.5)
		dc.DrawRectangle(30, 0, 30, 12)
		_ = dc.Fill()
		dc.PopLayer()
		dc.PopLayer()
		r, g, b, a := sampleRGBA(dc, 45, 6)
		check("backdrop_nested_sees_outer_layer", near4(r, g, b, a, 0, 0.5, 0, 1, tol))
		_ = dc.Close()
	}

	// 10) The same nested setup: the band outside the dim keeps its full
	// color, and the enclosing layer's transparent area stays transparent.
	{
		dc := probeCtxOpaque(60, 60)
		dc.PushLayerIsolated(1)
		fillRect(dc, 0, 0, 60, 12, [3]float64{0, 1, 0})
		dc.PushBackdropLayer(render.BlendNormal, 1)
		dc.SetRGBA(0, 0, 0, 0.5)
		dc.DrawRectangle(30, 0, 30, 12)
		_ = dc.Fill()
		dc.PopLayer()
		dc.PopLayer()
		br, bgc, bb, _ := sampleRGBA(dc, 15, 6)
		wr, wg, wb, _ := sampleRGBA(dc, 45, 30)
		check("backdrop_nested_band_outside_dim",
			near4(br, bgc, bb, 1, 0, 1, 0, 1, tol) && near4(wr, wg, wb, 1, 1, 1, 1, 1, tol))
		_ = dc.Close()
	}

	// 11) The snapshot is taken at push time: content drawn before the push in
	// paint order is what the layer dims.
	{
		dc := probeCtxOpaque(60, 60)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		fillRect(dc, 0, 0, 60, 60, [3]float64{0, 1, 0})
		dc.PushBackdropLayer(render.BlendNormal, 1)
		dimRect(dc, 0, 0, 60, 60, 0.5)
		dc.PopLayer()
		r, g, b, a := sampleRGBA(dc, 30, 30)
		setDetail("got=(%.3f,%.3f,%.3f,%.3f) want=(0,0.5,0,1)", r, g, b, a)
		check("backdrop_sees_parent_draw_order", near4(r, g, b, a, 0, 0.5, 0, 1, tol))
		_ = dc.Close()
	}

	// 12) Pop returns to the parent: later draws land on top of the composited
	// backdrop.
	{
		dc := probeCtxOpaque(60, 60)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		dc.PushBackdropLayer(render.BlendNormal, 1)
		dimRect(dc, 0, 0, 60, 60, 0.5)
		dc.PopLayer()
		fillRect(dc, 10, 10, 20, 20, [3]float64{0, 0, 1})
		dr, dg, db, _ := sampleRGBA(dc, 45, 30)
		cr, cg, cb, _ := sampleRGBA(dc, 20, 20)
		setDetail("dim=(%.3f,%.3f,%.3f) later=(%.3f,%.3f,%.3f)", dr, dg, db, cr, cg, cb)
		check("backdrop_then_parent_draw_on_top",
			near4(dr, dg, db, 1, 0.5, 0, 0, 1, tol) && near4(cr, cg, cb, 1, 0, 0, 1, 1, tol))
		_ = dc.Close()
	}

	// 13) Two consecutive backdrops compose: the second snapshot contains the
	// first dim, so a second 50% dim leaves 25%.
	{
		dc := probeCtxOpaque(60, 60)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		dc.PushBackdropLayer(render.BlendNormal, 1)
		dc.SetRGBA(0, 0, 0, 0.5)
		dc.DrawRectangle(0, 0, 60, 60)
		_ = dc.Fill()
		dc.PopLayer()
		dc.PushBackdropLayer(render.BlendNormal, 1)
		dc.SetRGBA(0, 0, 0, 0.5)
		dc.DrawRectangle(0, 0, 60, 60)
		_ = dc.Fill()
		dc.PopLayer()
		r, g, b, a := sampleRGBA(dc, 30, 30)
		check("backdrop_twice_composes", near4(r, g, b, a, 0.25, 0, 0, 1, tol))
		_ = dc.Close()
	}

	// 14) Zero opacity is a no-op composite.
	{
		dc := probeCtxOpaque(60, 60)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		dc.PushBackdropLayer(render.BlendNormal, 0)
		fillRect(dc, 0, 0, 60, 60, [3]float64{0, 0, 1})
		dc.PopLayer()
		r, g, b, a := sampleRGBA(dc, 30, 30)
		check("backdrop_opacity_zero_noop", near4(r, g, b, a, 1, 0, 0, 1, tol))
		_ = dc.Close()
	}

	return probes, failed
}

// ---- scene ----

func buildBackdropScene(shell *wrkit.ShellChrome) {
	// isolated=true gives the card its own repaint-boundary layer (A/B cards
	// drawing a backdrop inside it exercise the isolated/nested path);
	// isolated=false paints straight on the base canvas, which is the path the
	// presented frame used to lose.
	card := func(i int, title string, isolated bool, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.SetRepaintBoundary(isolated)
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, colTitle[0], colTitle[1], colTitle[2]),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	// A and C stay at the base canvas on purpose (no isolation layer): they
	// pin the base-level backdrop path, which is where the presented frame went
	// black while the snapshot stayed correct. B and D run inside their card's
	// repaint-boundary layer and pin the isolated/nested paths.
	card(0, "A 背板 + 半透明压暗（基面直达，无隔离层）", false, paintDimCard)
	card(1, "B 毛玻璃：Blur(6) + 15% 白（隔离卡）", true, paintBlurCard)
	card(2, "C 背板 + 裁剪面板（Pop 整面贴回）", false, paintClipCard)
	card(3, "D 嵌套背板：外层隔离层内再 Push（隔离卡）", true, paintNestedCard)
}

func cardBG(dc render.Canvas, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

func fillBand(dc render.Canvas, x, y, w, h float64, c [3]float64) {
	dc.SetRGBA(c[0], c[1], c[2], 1)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}

// fillStripes paints the shared teal/blue stripe page the analytic oracle
// resolves through stripeColorAt.
func fillStripes(dc render.Canvas, x, y, w, h float64) {
	for i := 0; float64(i)*stripeW < w; i++ {
		bandX := x + float64(i)*stripeW
		bw := math.Min(stripeW, x+w-bandX)
		if bw <= 0 {
			break
		}
		fillBand(dc, bandX, y, bw, h, stripeColorAt(float64(i)*stripeW))
	}
}

func paintDimCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	fillStripes(dc, cx, cy, contentW, contentH)

	// Deliberately base-level: PushBackdropLayer snapshots the whole window,
	// the dim panel only covers part of this card, and the Pop blit is
	// full-surface (this is the presented-path regression this card pins).
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, bDimAlpha)
	dc.DrawRectangle(cx+aPanelX, cy+aPanelY, aPanelW, aPanelH)
	_ = dc.Fill()
	fillBand(dc, cx+aChipX, cy+aChipY, aChipW, aChipH, colChip)
	dc.PopLayer()
}

func paintBlurCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	// High-frequency black/white bars inside the glass panel: the blur is a
	// full-surface filter, so confining the detailed texture to the panel keeps
	// the card outside it visually stable.
	fillBand(dc, cx+bPanelX, cy+bPanelY, bPanelW, bPanelH, colChip)
	for x := 0.0; x < bPanelW; x += bBarPeriod {
		bw := math.Min(bBarW, bPanelW-x)
		fillBand(dc, cx+bPanelX+x, cy+bPanelY, bw, bPanelH, [3]float64{0, 0, 0})
	}
	// Flutter BackdropFilter pattern: clip to the panel first so the blur and
	// tint only land inside it (BackdropFilterLayer paints "everywhere in cull
	// rect"; with no clip the filter would hit the whole window).
	dc.ClipRect(cx+bPanelX, cy+bPanelY, bPanelW, bPanelH)
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.ApplyBlur(6)
	dc.SetRGBA(1, 1, 1, bTintAlpha)
	dc.DrawRectangle(cx+bPanelX, cy+bPanelY, bPanelW, bPanelH)
	_ = dc.Fill()
	dc.PopLayer()
	dc.ResetClip()

	// Sharp child after the Pop: the frosted panel must not blur it.
	fillBand(dc, cx+bChipX, cy+bChipY, bChipW, bChipH, colChip)
}

func paintClipCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	fillStripes(dc, cx, cy, contentW, contentH)

	// Flutter-idiomatic saveLayer bounds: clip first, then push the backdrop.
	// The clip becomes the layer bounds, so the filtered snapshot only lands
	// inside the panel while the parent's stripes outside survive.
	dc.ClipRect(cx+cPanelX, cy+cPanelY, cPanelW, cPanelH)
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, bDimAlpha)
	dc.DrawRectangle(cx+cPanelX, cy+cPanelY, cPanelW, cPanelH)
	_ = dc.Fill()
	dc.PopLayer()
	dc.ResetClip()
}

func paintNestedCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	// Outer isolation layer with two bands, then a nested backdrop inside it:
	// the inner snapshot must be the outer layer's content, and returning to
	// the outer RT after the inner Pop must not clear it.
	dc.PushLayerIsolated(1)
	fillBand(dc, cx, cy, dOuterW, dBandH, colStripeA)
	fillBand(dc, cx, cy+dBandH, dOuterW, dBandH, colStripeB)
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, bDimAlpha)
	dc.DrawRectangle(cx+dStripX, cy, dStripW, contentH)
	_ = dc.Fill()
	dc.PopLayer()
	dc.PopLayer()

	fillBand(dc, cx+dChipX, cy+dChipY, dChipW, dChipH, colChip)
}
