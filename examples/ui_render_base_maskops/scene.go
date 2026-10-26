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
	// top, the mask demo lives in the inset area (ui_render_base_* house style).
	contentX0, contentY0 = 12.0, 36.0
	contentW, contentH   = cardW - 2*contentX0, cardH - contentY0 - 12.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// Palette. Flat pixel probes read these same constants, so a palette edit can
// never desync the drawn pixels from the asserted ones.
var (
	colCardBG = [3]float64{0.12, 0.135, 0.165}
	colTitle  = [3]float64{0.80, 0.86, 0.94}
	colFillA  = [3]float64{0.18, 0.78, 0.70} // teal
	colFillB  = [3]float64{0.85, 0.45, 0.20} // orange
	colFillC1 = [3]float64{0.30, 0.55, 0.95} // blue
	colFillC2 = [3]float64{0.80, 0.35, 0.85} // purple
)

// ---- card geometry (content-local) ----

const (
	// A: ApplyMask. The mask quad maps 1:1 onto the surface origin, so card A
	// is the top-left card and the quad ends inside the content: everything
	// right of x=320 / below y=115 is outside the mask rect and must stay
	// untouched (the semantic revision: outside = untouched, not cleared).
	aQuadW, aQuadH = 320.0, 115.0
	// A ramp bands (content-local x): 255 / 170 / 85 / 0.
	aRampX1, aRampX2, aRampX3 = 80.0, 160.0, 240.0
	aBand1Y, aBand1H          = 10.0, 50.0  // teal,   y 10..60
	aBand2Y, aBand2H          = 66.0, 50.0  // orange, y 66..116
	aBand3Y, aBand3H          = 122.0, 56.0 // purple, y 122..178

	// B / C: masked band (half-width mask) + cleared band (full width).
	bBand1Y, bBand1H = 20.0, 60.0
	bBand2Y, bBand2H = 100.0, 60.0

	// D: circle mask r=62 centered in the content.
	dCircleR  = 62.0
	dCircleCX = 203.0
	dCircleCY = 92.0
)

// ---- mask builders + per-card cache ----

// maskSlot caches one card's mask and rebuilds it only when the geometry it
// was laid out for changes (window size / card origin on resize).
type maskSlot struct {
	key  string
	mask *render.Mask
}

func (s *maskSlot) get(key string, build func() *render.Mask) *render.Mask {
	if s.mask == nil || s.key != key {
		s.mask = build()
		s.key = key
	}
	return s.mask
}

var (
	aQuadMaskSlot maskSlot
	bMaskSlot     maskSlot
	cMaskSlot     maskSlot
	dMaskSlot     maskSlot
)

// buildHalfMask makes a window-sized mask: 255 left of splitX, 0 right of it.
func buildHalfMask(w, h, splitX int) *render.Mask {
	m := render.NewMask(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < splitX && x < w; x++ {
			m.Set(x, y, 255)
		}
	}
	return m
}

// buildApplyMaskQuad builds card A's partial-quad ramp mask. The quad spans
// [0,qw)x[0,qh) of the surface (ApplyMask draws at the surface origin); the
// ramp bands start at the content origin (cx, cy). Texels left of / above the
// content stay 0 — they are outside the clip, so they can never be composited.
func buildApplyMaskQuad(qw, qh, cx, cy int) *render.Mask {
	m := render.NewMask(qw, qh)
	for y := cy; y < qh; y++ {
		for x := cx; x < qw; x++ {
			lx := x - cx
			var v uint8
			switch {
			case lx < int(aRampX1):
				v = 255
			case lx < int(aRampX2):
				v = 170
			case lx < int(aRampX3):
				v = 85
			}
			m.Set(x, y, v)
		}
	}
	return m
}

// buildCircleMask makes a window-sized hard circle mask centered at (cx, cy).
func buildCircleMask(w, h, cx, cy int, r float64) *render.Mask {
	m := render.NewMask(w, h)
	r2 := r * r
	x0, x1 := cx-int(r)-1, cx+int(r)+1
	y0, y1 := cy-int(r)-1, cy+int(r)+1
	for y := y0; y <= y1; y++ {
		if y < 0 || y >= h {
			continue
		}
		for x := x0; x <= x1; x++ {
			if x < 0 || x >= w {
				continue
			}
			dx, dy := float64(x-cx), float64(y-cy)
			if dx*dx+dy*dy <= r2 {
				m.Set(x, y, 255)
			}
		}
	}
	return m
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

// blend returns fill over the card background with mask coverage cov (0..1):
// a masked layer composites as SourceOver of fill*cov onto the card panel.
func blend(bg, fill [3]float64, cov float64) (r, g, b float64) {
	f := func(i int) float64 { return bg[i]*(1-cov) + fill[i]*cov }
	return f(0), f(1), f(2)
}

const tolSolid = 8.0 / 255

var probeSpecs []probeSpec

func init() {
	solid := func(c [3]float64) func(float64, float64, float64, int, int) (float64, float64, float64) {
		return func(float64, float64, float64, int, int) (float64, float64, float64) {
			return c[0], c[1], c[2]
		}
	}
	ramp := func(fill [3]float64, m float64) func(float64, float64, float64, int, int) (float64, float64, float64) {
		return func(float64, float64, float64, int, int) (float64, float64, float64) {
			return blend(colCardBG, fill, m)
		}
	}
	add := func(key, desc string, card int, pt render.Point,
		want func(float64, float64, float64, int, int) (float64, float64, float64), bands ...band) {
		probeSpecs = append(probeSpecs, probeSpec{key, desc, card, pt, want, bands})
	}

	bgBands := []band{b8(0, 20, 42), b8(1, 24, 45), b8(2, 32, 52)}

	// A: ApplyMask ramp 255/170/85/0 over the isolated content bands, plus the
	// two "outside the mask rect untouched" pins (x > 320, y > 115).
	add("a_mask255", "A ramp 255 = teal band full", 0, render.Pt(40, 35),
		solid(colFillA), b8(1, 190, 210), b8(0, 35, 60), b8(2, 165, 195))
	add("a_mask170", "A ramp 170 = teal x 170/255 over card BG", 0, render.Pt(120, 35),
		ramp(colFillA, 170.0/255), b8(1, 130, 160), b8(2, 118, 148))
	add("a_mask85", "A ramp 85 on the orange band = orange x 85/255", 0, render.Pt(200, 90),
		ramp(colFillB, 85.0/255), b8(0, 80, 115), b8(1, 50, 78))
	add("a_mask0", "A ramp 0 = content cleared -> card BG", 0, render.Pt(280, 35),
		solid(colCardBG), bgBands...)
	add("a_out_x", "A right of the mask quad (x>320) untouched = teal", 0, render.Pt(370, 35),
		solid(colFillA), b8(1, 190, 210), b8(0, 35, 60))
	add("a_out_y", "A below the mask quad (y>115) untouched = purple", 0, render.Pt(40, 150),
		solid(colFillC2), b8(0, 185, 215), b8(2, 200, 230))

	// B: SetMask half coverage, ClearMask restores the full-width fill.
	add("b1_left", "B SetMask 255-left -> teal painted left half", 1, render.Pt(80, 50),
		solid(colFillA), b8(1, 190, 210), b8(0, 35, 60), b8(2, 165, 195))
	add("b1_right", "B SetMask 0-right -> right half unpainted = card BG", 1, render.Pt(320, 50),
		solid(colCardBG), bgBands...)
	add("b2_left", "B ClearMask -> orange painted left half", 1, render.Pt(80, 130),
		solid(colFillB), b8(0, 205, 230), b8(1, 100, 130), b8(2, 40, 65))
	add("b2_right", "B ClearMask -> orange painted right half", 1, render.Pt(320, 130),
		solid(colFillB), b8(0, 205, 230), b8(1, 100, 130), b8(2, 40, 65))

	// C: InvertMask flips the same Mask object in place.
	add("c1_left", "C band 1 under the canonical mask = blue", 2, render.Pt(80, 50),
		solid(colFillC1), b8(2, 225, 255), b8(0, 55, 90), b8(1, 115, 155))
	add("c1_right", "C band 1 right half unpainted = card BG", 2, render.Pt(320, 50),
		solid(colCardBG), bgBands...)
	add("c2_left", "C band 2 after InvertMask: left excluded = card BG", 2, render.Pt(80, 130),
		solid(colCardBG), bgBands...)
	add("c2_right", "C band 2 after InvertMask: right painted = purple", 2, render.Pt(320, 130),
		solid(colFillC2), b8(0, 185, 215), b8(2, 200, 230), b8(1, 70, 105))

	// D: PushMaskLayer composites the layer through a circle mask.
	add("d_center", "D circle mask center = purple", 3, render.Pt(203, 92),
		solid(colFillC2), b8(0, 185, 215), b8(2, 200, 230), b8(1, 70, 105))
	add("d_inside", "D 50px above center (r=62) = purple", 3, render.Pt(203, 42),
		solid(colFillC2), b8(0, 185, 215), b8(2, 200, 230), b8(1, 70, 105))
	add("d_out", "D 80px above center (outside r=62) = card BG", 3, render.Pt(203, 172),
		solid(colCardBG), bgBands...)
	add("d_corner", "D content corner outside the circle = card BG", 3, render.Pt(20, 20),
		solid(colCardBG), bgBands...)
}

// ---- logic probes (value-level, run once at startup) ----

// Logic probed on an offscreen CPU context: the same public mask contract the
// window draws with, checked at value level (pixels + Mask values).

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

// halfMask builds a 60x60 mask: 255 left of 30, 0 right of it.
func halfMask() *render.Mask {
	m := render.NewMask(60, 60)
	for y := 0; y < 60; y++ {
		for x := 0; x < 30; x++ {
			m.Set(x, y, 255)
		}
	}
	return m
}

// probeDetail carries the sampled values of the probe being checked so a
// failure prints why it failed, not just which key failed.
var probeDetail string

func setDetail(format string, args ...any) { probeDetail = fmt.Sprintf(format, args...) }

// runLogicProbes checks the five mask write-path APIs (SetMask / ClearMask /
// InvertMask / ApplyMask / PushMaskLayer+PopLayer) value-level, including the
// two semantics pinned by this window: ApplyMask scales the destination inside
// the mask rect only, and soft mask layers keep their true coverage.
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

	// 1) SetMask modulates Fill coverage (mask 0 = pixel untouched).
	{
		dc := probeCtxOpaque(60, 60)
		m := halfMask()
		dc.SetMask(m)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		lr, lg, lb, la := sampleRGBA(dc, 15, 30)
		rr, rg, rb, ra := sampleRGBA(dc, 45, 30)
		check("setmask_modulates_fill",
			near4(lr, lg, lb, la, 1, 0, 0, 1, tol) && near4(rr, rg, rb, ra, 1, 1, 1, 1, tol))
		_ = dc.Close()
	}

	// 2) ClearMask removes the modulation: the next fill covers both halves.
	{
		dc := probeCtxOpaque(60, 60)
		dc.SetMask(halfMask())
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		dc.ClearMask()
		fillRect(dc, 0, 0, 60, 60, [3]float64{0, 0, 1})
		lr, lg, lb, _ := sampleRGBA(dc, 15, 30)
		rr, rg, rb, _ := sampleRGBA(dc, 45, 30)
		check("clearmask_restores_fill",
			near4(lr, lg, lb, 1, 0, 0, 1, 1, tol) && near4(rr, rg, rb, 1, 0, 0, 1, 1, tol))
		_ = dc.Close()
	}

	// 3) SetMask modulates Strokes the same way (documented contract).
	{
		dc := probeCtxOpaque(60, 60)
		dc.SetMask(halfMask())
		dc.SetRGBA(1, 0, 0, 1)
		dc.SetLineWidth(6)
		dc.DrawLine(4, 30, 56, 30)
		if err := dc.Stroke(); err != nil {
			check("setmask_modulates_stroke", false)
		} else {
			lr, lg, lb, la := sampleRGBA(dc, 12, 30)
			rr, rg, rb, ra := sampleRGBA(dc, 48, 30)
			check("setmask_modulates_stroke",
				near4(lr, lg, lb, la, 1, 0, 0, 1, tol) && near4(rr, rg, rb, ra, 1, 1, 1, 1, tol))
		}
		_ = dc.Close()
	}

	// 4) Push/Pop restores an equivalent mask (Push clones the mask object, so
	// this pins value equality, not pointer identity).
	{
		dc := probeCtxOpaque(60, 60)
		m := halfMask()
		dc.SetMask(m)
		dc.Push()
		dc.ClearMask()
		cleared := dc.GetMask() == nil
		dc.Pop()
		gm := dc.GetMask()
		restored := gm != nil && gm.Width() == 60 && gm.Height() == 60 &&
			gm.At(15, 30) == 255 && gm.At(45, 30) == 0
		setDetail("cleared=%v restored_nil=%v restored=%v", cleared, gm == nil, restored)
		check("mask_push_pop_restores", cleared && restored)
		_ = dc.Close()
	}

	// 5) InvertMask flips the same Mask object in place (255 - v).
	{
		m := render.NewMask(60, 60)
		m.Set(3, 3, 200)
		m.Set(7, 7, 55)
		dc := probeCtxOpaque(60, 60)
		dc.SetMask(m)
		dc.InvertMask()
		check("invertmask_flips_values",
			m.At(3, 3) == 55 && m.At(7, 7) == 200 && dc.GetMask() == m)
		_ = dc.Close()
	}

	// 6) InvertMask changes which half the next fill covers.
	{
		dc := probeCtxOpaque(60, 60)
		dc.SetMask(halfMask())
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		dc.InvertMask()
		fillRect(dc, 0, 0, 60, 60, [3]float64{0, 1, 0})
		lr, lg, lb, _ := sampleRGBA(dc, 15, 30)
		rr, rg, rb, _ := sampleRGBA(dc, 45, 30)
		check("invertmask_fill_flips",
			near4(lr, lg, lb, 1, 1, 0, 0, 1, tol) && near4(rr, rg, rb, 1, 0, 1, 0, 1, tol))
		_ = dc.Close()
	}

	// 7) ApplyMask scales premultiplied channels by mask/255.
	{
		dc := render.NewContext(60, 60)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		m := render.NewMask(60, 60)
		m.Fill(128)
		dc.ApplyMask(m)
		r, g, b, a := sampleRGBA(dc, 30, 30)
		setDetail("got=(%.3f,%.3f,%.3f,%.3f) want=(0.502,0,0,0.502) stats=%s", r, g, b, a, dc.RenderPathStats().LogLine())
		check("applymask_scales_channels", near4(r, g, b, a, 128.0/255, 0, 0, 128.0/255, tol))
		_ = dc.Close()
	}

	// 8) ApplyMask mask 0 clears the covered destination.
	{
		dc := render.NewContext(60, 60)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		m := render.NewMask(60, 60)
		m.Fill(0)
		dc.ApplyMask(m)
		r, g, b, a := sampleRGBA(dc, 30, 30)
		setDetail("got=(%.3f,%.3f,%.3f,%.3f) want=(0,0,0,0)", r, g, b, a)
		check("applymask_zero_clears", near4(r, g, b, a, 0, 0, 0, 0, tol))
		_ = dc.Close()
	}

	// 9) Pixels beyond the mask rect are untouched (the semantic revision:
	// outside the mask rectangle the destination keeps its content).
	{
		dc := render.NewContext(60, 60)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		m := render.NewMask(30, 30) // quad [0,30)x[0,30), all zero
		m.Fill(0)
		dc.ApplyMask(m)
		ir, ig, ib, ia := sampleRGBA(dc, 15, 15)
		or, og, ob, oa := sampleRGBA(dc, 45, 45)
		setDetail("in15,15=(%.3f,%.3f,%.3f,%.3f) want=(0,0,0,0); out45,45=(%.3f,%.3f,%.3f,%.3f) want=(1,0,0,1)",
			ir, ig, ib, ia, or, og, ob, oa)
		check("applymask_outside_rect_untouched",
			near4(ir, ig, ib, ia, 0, 0, 0, 0, tol) && near4(or, og, ob, oa, 1, 0, 0, 1, tol))
		_ = dc.Close()
	}

	// 10) ApplyMask respects the active clip.
	{
		dc := render.NewContext(60, 60)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		dc.ClipRect(0, 0, 30, 60)
		m := render.NewMask(60, 60)
		m.Fill(0)
		dc.ApplyMask(m)
		ir, ig, ib, ia := sampleRGBA(dc, 15, 30)
		or, og, ob, oa := sampleRGBA(dc, 45, 30)
		setDetail("in-clip15,30=(%.3f,%.3f,%.3f,%.3f) want=(0,0,0,0); out-clip45,30=(%.3f,%.3f,%.3f,%.3f) want=(1,0,0,1)",
			ir, ig, ib, ia, or, og, ob, oa)
		check("applymask_respects_clip",
			near4(ir, ig, ib, ia, 0, 0, 0, 0, tol) && near4(or, og, ob, oa, 1, 0, 0, 1, tol))
		_ = dc.Close()
	}

	// 11) The scene recipe: isolate in a saveLayer, ApplyMask inside, pop — the
	// ramp fades the layer content toward the background while the area outside
	// the mask rect stays at full strength.
	{
		dc := probeCtxOpaque(60, 60)
		dc.PushLayerIsolated(1.0)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		m := render.NewMask(60, 30) // quad [0,60)x[0,30)
		m.Fill(128)
		dc.ApplyMask(m)
		dc.PopLayer()
		ir, ig, ib, ia := sampleRGBA(dc, 30, 15) // inside: red x 128/255 over white
		or, og, ob, oa := sampleRGBA(dc, 30, 45) // below the quad: untouched
		setDetail("in30,15=(%.3f,%.3f,%.3f,%.3f) want=(1,0.498,0.498,1); out30,45=(%.3f,%.3f,%.3f,%.3f) want=(1,0,0,1)",
			ir, ig, ib, ia, or, og, ob, oa)
		check("applymask_in_isolation_layer",
			near4(ir, ig, ib, ia, 1, 1-128.0/255, 1-128.0/255, 1, 3.0/255) &&
				near4(or, og, ob, oa, 1, 0, 0, 1, tol))
		_ = dc.Close()
	}

	// 12) PushMaskLayer composites the layer through the mask (hard circle).
	{
		dc := probeCtxOpaque(60, 60)
		m := buildCircleMask(60, 60, 30, 30, 15)
		dc.PushMaskLayer(m)
		fillRect(dc, 0, 0, 60, 60, [3]float64{0, 0, 0})
		dc.PopLayer()
		cr, cg, cb, ca := sampleRGBA(dc, 30, 30)
		or, og, ob, oa := sampleRGBA(dc, 5, 5)
		check("pushmasklayer_circle_composite",
			near4(cr, cg, cb, ca, 0, 0, 0, 1, tol) && near4(or, og, ob, oa, 1, 1, 1, 1, tol))
		_ = dc.Close()
	}

	// 13) Soft mask values keep their true coverage through the layer
	// composite (the premultiplied SourceOver pin: 128/255 black over white).
	{
		dc := probeCtxOpaque(60, 60)
		m := render.NewMask(60, 60)
		for y := 0; y < 60; y++ {
			for x := 0; x < 30; x++ {
				m.Set(x, y, 128)
			}
		}
		dc.PushMaskLayer(m)
		fillRect(dc, 0, 0, 60, 60, [3]float64{0, 0, 0})
		dc.PopLayer()
		lr, lg, lb, la := sampleRGBA(dc, 15, 30)
		rr, rg, rb, ra := sampleRGBA(dc, 45, 30)
		soft := 1.0 - 128.0/255
		check("pushmasklayer_soft_mask_composite",
			near4(lr, lg, lb, la, soft, soft, soft, 1, 3.0/255) &&
				near4(rr, rg, rb, ra, 1, 1, 1, 1, tol))
		_ = dc.Close()
	}

	// 14) PushMaskLayer(nil) degrades to a plain (unmasked) layer.
	{
		dc := probeCtxOpaque(60, 60)
		dc.PushMaskLayer(nil)
		fillRect(dc, 0, 0, 60, 60, [3]float64{1, 0, 0})
		dc.PopLayer()
		r, g, b, a := sampleRGBA(dc, 30, 30)
		check("pushmasklayer_nil_plain", near4(r, g, b, a, 1, 0, 0, 1, tol))
		_ = dc.Close()
	}

	return probes, failed
}

// ---- scene ----

// buildMaskOpsScene fills the shell body with the four mask write-path cards.
func buildMaskOpsScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.SetRepaintBoundary(true)
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, colTitle[0], colTitle[1], colTitle[2]),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A  ApplyMask — ramp 255/170/85/0, quad-bounded (x>320/y>115 untouched)", paintApplyMaskCard)
	card(1, "B  SetMask left-half 255 -> ClearMask restores the full fill", paintSetClearMaskCard)
	card(2, "C  InvertMask in place — same Mask flips the next band", paintInvertMaskCard)
	card(3, "D  PushMaskLayer r=62 circle -> PopLayer composite", paintMaskLayerCard)
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

// paintApplyMaskCard: ApplyMask is a destructive DstIn post-process, so the
// content is first isolated in a layer (Normal PushLayer is an F1 opacity
// group and would let the DstIn hit the card background). The mask quad maps
// 1:1 onto the surface origin — this is the top-left card, and the quad ends
// inside the content so the untouched region (x>320 / y>115) is visible.
func paintApplyMaskCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	// Masks are device-space post-processes: a recorder carries them as
	// commands, the live context executes them directly. A walk whose target
	// has no mask capability refuses and counts.
	dc, ok := pc.MaskCanvas()
	if !ok {
		return
	}
	ox, oy := pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	cxi, cyi := int(math.Round(cx)), int(math.Round(cy))

	quad := aQuadMaskSlot.get(fmt.Sprintf("quad:%d,%d,%d,%d", cxi+int(aQuadW), cyi+int(aQuadH), cxi, cyi),
		func() *render.Mask {
			return buildApplyMaskQuad(cxi+int(aQuadW), cyi+int(aQuadH), cxi, cyi)
		})

	// saveLayer isolation (Flutter Canvas.saveLayer semantics): ApplyMask is a
	// destructive DstIn post-process, so it must scale the layer content — a
	// plain PushLayer(Normal) is an F1 opacity group whose draws land straight
	// on the parent, which would scale the card panel underneath too.
	dc.PushLayerIsolated(1.0)
	fillBand(dc, cx, cy+aBand1Y, contentW, aBand1H, colFillA)
	fillBand(dc, cx, cy+aBand2Y, contentW, aBand2H, colFillB)
	fillBand(dc, cx, cy+aBand3Y, contentW, aBand3H, colFillC2)
	// Skia-aligned: the mask only touches the mask rectangle ∩ the active clip.
	dc.ClipRect(cx, cy, contentW, contentH)
	dc.ApplyMask(quad)
	dc.ResetClip()
	dc.PopLayer()
}

// paintSetClearMaskCard: SetMask modulates every following Fill, ClearMask
// removes the modulation. The mask is surface-sized because the accelerator
// samples the R8 plane against the whole surface.
func paintSetClearMaskCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	// Masks are device-space post-processes: a recorder carries them as
	// commands, the live context executes them directly. A walk whose target
	// has no mask capability refuses and counts.
	dc, ok := pc.MaskCanvas()
	if !ok {
		return
	}
	ox, oy := pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	w, h := dc.Width(), dc.Height()
	splitX := int(math.Round(ox + contentX0 + contentW/2))

	m := bMaskSlot.get(fmt.Sprintf("half:%d:%dx%d", splitX, w, h), func() *render.Mask {
		return buildHalfMask(w, h, splitX)
	})
	dc.SetMask(m)
	fillBand(dc, cx, cy+bBand1Y, contentW, bBand1H, colFillA)
	dc.ClearMask()
	fillBand(dc, cx, cy+bBand2Y, contentW, bBand2H, colFillB)
}

// paintInvertMaskCard: InvertMask flips the stored Mask in place (255 - v),
// which flips which half the next fill covers. The mask object survives
// between frames, so the paint normalises it back to the canonical state
// first (inverting twice is the identity).
func paintInvertMaskCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	// Masks are device-space post-processes: a recorder carries them as
	// commands, the live context executes them directly. A walk whose target
	// has no mask capability refuses and counts.
	dc, ok := pc.MaskCanvas()
	if !ok {
		return
	}
	ox, oy := pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	w, h := dc.Width(), dc.Height()
	splitX := int(math.Round(ox + contentX0 + contentW/2))

	m := cMaskSlot.get(fmt.Sprintf("half:%d:%dx%d", splitX, w, h), func() *render.Mask {
		return buildHalfMask(w, h, splitX)
	})
	dc.SetMask(m)
	if m.At(0, 0) == 0 {
		dc.InvertMask() // restore the canonical 255-left state after last frame
	}
	fillBand(dc, cx, cy+bBand1Y, contentW, bBand1H, colFillC1)
	dc.InvertMask()
	fillBand(dc, cx, cy+bBand2Y, contentW, bBand2H, colFillC2)
	dc.ClearMask()
}

// paintMaskLayerCard: PushMaskLayer composites the whole layer through the
// mask at PopLayer time — masked-out layer pixels drop out, the card panel
// underneath shows through.
func paintMaskLayerCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	// Masks are device-space post-processes: a recorder carries them as
	// commands, the live context executes them directly. A walk whose target
	// has no mask capability refuses and counts.
	dc, ok := pc.MaskCanvas()
	if !ok {
		return
	}
	ox, oy := pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	w, h := dc.Width(), dc.Height()
	ccx := int(math.Round(ox + contentX0 + dCircleCX))
	ccy := int(math.Round(oy + contentY0 + dCircleCY))

	m := dMaskSlot.get(fmt.Sprintf("circle:%d,%d:%dx%d", ccx, ccy, w, h), func() *render.Mask {
		return buildCircleMask(w, h, ccx, ccy, dCircleR)
	})
	dc.PushMaskLayer(m)
	fillBand(dc, cx, cy, contentW, contentH, colFillC2)
	dc.PopLayer()
}
