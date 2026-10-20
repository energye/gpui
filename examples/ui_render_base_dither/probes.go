package main

import (
	"bytes"
	"fmt"
	"image"
	"math"

	"github.com/energye/gpui/render"
)

// ---------------------------------------------------------------------------
// Logic probes (value-level, offscreen)
// ---------------------------------------------------------------------------

type probeResult struct {
	Key    string
	OK     bool
	Detail string
}

func newShapeContext(kind string, w, h int, dither bool) *render.Context {
	dc := render.NewContext(w, h)
	drawBakedShape(dc, kind, w, h)
	if dither {
		dc.SetDither(true)
	}
	return dc
}

func imageBytes(img image.Image) []byte {
	if img == nil {
		return nil
	}
	b := img.Bounds()
	out := make([]byte, 0, b.Dx()*b.Dy()*4)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			out = append(out, byte(r>>8), byte(g>>8), byte(bl>>8), byte(a>>8))
		}
	}
	return out
}

func imageUnique(img image.Image) int {
	if img == nil {
		return 0
	}
	return countUnique(img, img.Bounds())
}

func imageAlphaUniform(img image.Image) bool {
	if img == nil {
		return false
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a>>8 != 255 {
				return false
			}
		}
	}
	return true
}

func imageMeanAbsDiff(a, b image.Image) float64 {
	if a == nil || b == nil {
		return math.Inf(1)
	}
	ra, rb := a.Bounds(), b.Bounds()
	w, h := ra.Dx(), ra.Dy()
	if rb.Dx() < w {
		w = rb.Dx()
	}
	if rb.Dy() < h {
		h = rb.Dy()
	}
	if w <= 0 || h <= 0 {
		return math.Inf(1)
	}
	var sum float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r0, g0, b0, _ := a.At(ra.Min.X+x, ra.Min.Y+y).RGBA()
			r1, g1, b1, _ := b.At(rb.Min.X+x, rb.Min.Y+y).RGBA()
			sum += math.Abs(float64(r0>>8)-float64(r1>>8)) / 255
			sum += math.Abs(float64(g0>>8)-float64(g1>>8)) / 255
			sum += math.Abs(float64(b0>>8)-float64(b1>>8)) / 255
		}
	}
	return sum / float64(w*h*3)
}

func imageDiffCount(a, b image.Image) (int, int) {
	if a == nil || b == nil {
		return 0, 0
	}
	ra, rb := a.Bounds(), b.Bounds()
	w, h := ra.Dx(), ra.Dy()
	if rb.Dx() < w {
		w = rb.Dx()
	}
	if rb.Dy() < h {
		h = rb.Dy()
	}
	total, diff := 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r0, g0, b0, _ := a.At(ra.Min.X+x, ra.Min.Y+y).RGBA()
			r1, g1, b1, _ := b.At(rb.Min.X+x, rb.Min.Y+y).RGBA()
			total++
			if r0 != r1 || g0 != g1 || b0 != b1 {
				diff++
			}
		}
	}
	return diff, total
}

func runLogicProbes() []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}

	// 1. nil receiver must be a no-op.
	func() {
		defer func() {
			if r := recover(); r != nil {
				add("dither_nil_context", false, fmt.Sprintf("panic: %v", r))
			}
		}()
		var dc *render.Context
		dc.SetDither(true)
		add("dither_nil_context", !dc.Dither(), "nil SetDither no-op, Dither=false")
	}()

	// 2-3. Default off + explicit toggle.
	{
		dc := render.NewContext(8, 8)
		def := dc.Dither()
		dc.SetDither(true)
		on := dc.Dither()
		dc.SetDither(false)
		off := dc.Dither()
		dc.Close()
		add("dither_default_off", !def, "new Context Dither()=false")
		add("dither_toggle_state", on && !off, "SetDither(true/false) flips Dither()")
	}

	// 4-7. Linear gradient: pixels change, diversity rises, mean stays bounded,
	// and alpha is untouched.
	{
		off := newShapeContext("linear", 64, 16, false)
		on := newShapeContext("linear", 64, 16, true)
		io, in := off.Image(), on.Image()
		uOff, uOn := imageUnique(io), imageUnique(in)
		diff, total := imageDiffCount(io, in)
		mean := imageMeanAbsDiff(io, in)
		alpha := imageAlphaUniform(in)
		off.Close()
		on.Close()
		add("dither_linear_changes", diff > 0 && total > 0, fmt.Sprintf("diff=%d/%d", diff, total))
		add("dither_linear_diversity", uOn > uOff, fmt.Sprintf("unique off=%d on=%d", uOff, uOn))
		add("dither_mean_bounded", mean <= 3.0/255.0, fmt.Sprintf("mean|Δ|/255=%.4f", mean))
		add("dither_alpha_preserved", alpha, "all alpha=255")
	}

	// 8-9. Radial and sweep also receive the postprocess.
	{
		for _, kind := range []string{"radial", "sweep"} {
			off := newShapeContext(kind, 48, 32, false)
			on := newShapeContext(kind, 48, 32, true)
			io, in := off.Image(), on.Image()
			diff, total := imageDiffCount(io, in)
			off.Close()
			on.Close()
			add("dither_"+kind+"_changes", diff > 0 && total > 0, fmt.Sprintf("diff=%d/%d", diff, total))
		}
	}

	// 10. Same input + same flag is deterministic.
	{
		a := newShapeContext("linear", 40, 12, true)
		b := newShapeContext("linear", 40, 12, true)
		eq := bytes.Equal(imageBytes(a.Image()), imageBytes(b.Image()))
		a.Close()
		b.Close()
		add("dither_deterministic", eq, "two identical contexts byte-identical")
	}

	// 11. SetDither after the draw still applies on the next Image().
	{
		off := newShapeContext("linear", 32, 8, false)
		after := render.NewContext(32, 8)
		drawBakedShape(after, "linear", 32, 8)
		after.SetDither(true)
		io, ia := off.Image(), after.Image()
		diff, total := imageDiffCount(io, ia)
		off.Close()
		after.Close()
		add("dither_after_draw", diff > 0 && total > 0, fmt.Sprintf("diff=%d/%d (flag read at Image)", diff, total))
	}

	// 12. Explicit false is the same as never enabling it.
	{
		control := newShapeContext("linear", 24, 8, false)
		explicit := render.NewContext(24, 8)
		drawBakedShape(explicit, "linear", 24, 8)
		explicit.SetDither(false)
		eq := bytes.Equal(imageBytes(control.Image()), imageBytes(explicit.Image()))
		control.Close()
		explicit.Close()
		add("dither_off_control", eq, "SetDither(false) == default off")
	}

	// 13. Ordered Bayer 4x4 signature on a flat fill (interior only; GPU solid
	// edges may carry fractional coverage).
	{
		dc := render.NewContext(16, 16)
		drawBakedShape(dc, "solid", 16, 16)
		dc.SetDither(true)
		img := dc.Image()
		ok := true
		for y := 4; y < 12 && ok; y++ {
			for x := 4; x < 8; x++ {
				if !sameRGB(img, x, y, x+4, y) {
					ok = false
					break
				}
			}
		}
		for y := 4; y < 8 && ok; y++ {
			for x := 4; x < 12; x++ {
				if !sameRGB(img, x, y, x, y+4) {
					ok = false
					break
				}
			}
		}
		dc.Close()
		add("dither_bayer4_period", ok, "flat interior RGB repeats every 4 px")
	}

	// 14. Known limit: the postprocess touches flat fills too.
	{
		off := newShapeContext("solid", 16, 16, false)
		on := newShapeContext("solid", 16, 16, true)
		diff, total := imageDiffCount(off.Image(), on.Image())
		off.Close()
		on.Close()
		add("dither_whole_pixmap_limit", diff > 0 && total > 0,
			fmt.Sprintf("KNOWN LIMIT: solid fill changed at %d/%d px", diff, total))
	}

	// 15. Known limit: repeated readback without new drawing compounds.
	{
		dc := render.NewContext(32, 8)
		drawBakedShape(dc, "linear", 32, 8)
		dc.SetDither(true)
		first := imageBytes(dc.Image())
		second := imageBytes(dc.Image())
		dc.Close()
		add("dither_repeat_readback_limit", !bytes.Equal(first, second),
			"KNOWN LIMIT: second Image() keeps changing pixels")
	}

	// 16. Disabling stops further passes; already-dithered pixels are stable.
	{
		dc := render.NewContext(24, 8)
		drawBakedShape(dc, "linear", 24, 8)
		dc.SetDither(true)
		_ = dc.Image()
		dc.SetDither(false)
		a := imageBytes(dc.Image())
		b := imageBytes(dc.Image())
		dc.Close()
		add("dither_disable_stops", bytes.Equal(a, b), "after SetDither(false), repeated Image() stable")
	}

	return out
}

func sameRGB(img image.Image, x0, y0, x1, y1 int) bool {
	r0, g0, b0, _ := img.At(x0, y0).RGBA()
	r1, g1, b1, _ := img.At(x1, y1).RGBA()
	return r0 == r1 && g0 == g1 && b0 == b1
}

// ---------------------------------------------------------------------------
// Pixel probes (snapshot + baked image)
// ---------------------------------------------------------------------------

func snapshotRect(bodyX, bodyY, dpr float64, card int, lx, ly, w, h float64) image.Rectangle {
	x0 := (bodyX + cardAt[card][0] + contentX0 + lx) * dpr
	y0 := (bodyY + cardAt[card][1] + contentY0 + ly) * dpr
	x1 := x0 + w*dpr
	y1 := y0 + h*dpr
	return image.Rect(int(math.Round(x0)), int(math.Round(y0)), int(math.Round(x1)), int(math.Round(y1)))
}

func regionStats(img image.Image, r image.Rectangle) (unique int, mean [3]float64, n int) {
	if img == nil {
		return 0, mean, 0
	}
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return 0, mean, 0
	}
	seen := map[[3]uint8]struct{}{}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			seen[[3]uint8{uint8(cr >> 8), uint8(cg >> 8), uint8(cb >> 8)}] = struct{}{}
			mean[0] += float64(cr>>8) / 255
			mean[1] += float64(cg>>8) / 255
			mean[2] += float64(cb>>8) / 255
		}
	}
	n = r.Dx() * r.Dy()
	mean[0] /= float64(n)
	mean[1] /= float64(n)
	mean[2] /= float64(n)
	return len(seen), mean, n
}

// regionMeanDiff compares two same-sized regions that may sit at different
// origins (the off/on halves of a card), walking them in local coordinates.
func regionMeanDiff(a, b image.Image, ra, rb image.Rectangle) float64 {
	if a == nil || b == nil {
		return math.Inf(1)
	}
	ra = ra.Intersect(a.Bounds())
	rb = rb.Intersect(b.Bounds())
	w, h := ra.Dx(), rb.Dx()
	if ra.Dy() < h {
		h = ra.Dy()
	}
	if rb.Dy() < h {
		h = rb.Dy()
	}
	if w <= 0 || h <= 0 {
		return math.Inf(1)
	}
	var sum float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r0, g0, b0, _ := a.At(ra.Min.X+x, ra.Min.Y+y).RGBA()
			r1, g1, b1, _ := b.At(rb.Min.X+x, rb.Min.Y+y).RGBA()
			sum += math.Abs(float64(r0>>8)-float64(r1>>8)) / 255
			sum += math.Abs(float64(g0>>8)-float64(g1>>8)) / 255
			sum += math.Abs(float64(b0>>8)-float64(b1>>8)) / 255
		}
	}
	return sum / float64(w*h*3)
}

func regionDiffCount(a, b image.Image, ra, rb image.Rectangle) (int, int) {
	if a == nil || b == nil {
		return 0, 0
	}
	ra = ra.Intersect(a.Bounds())
	rb = rb.Intersect(b.Bounds())
	w, h := ra.Dx(), rb.Dx()
	if ra.Dy() < h {
		h = ra.Dy()
	}
	if rb.Dy() < h {
		h = rb.Dy()
	}
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	diff := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r0, g0, b0, _ := a.At(ra.Min.X+x, ra.Min.Y+y).RGBA()
			r1, g1, b1, _ := b.At(rb.Min.X+x, rb.Min.Y+y).RGBA()
			if r0 != r1 || g0 != g1 || b0 != b1 {
				diff++
			}
		}
	}
	return diff, w * h
}

func uniformBlock(img image.Image, r image.Rectangle) bool {
	if img == nil {
		return false
	}
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return false
	}
	r0, g0, b0, _ := img.At(r.Min.X, r.Min.Y).RGBA()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			r1, g1, b1, _ := img.At(x, y).RGBA()
			if r0 != r1 || g0 != g1 || b0 != b1 {
				return false
			}
		}
	}
	return true
}

func runPixelProbes(snap image.Image, bodyX, bodyY, dpr float64) []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}
	if snap == nil {
		add("pixel_snapshot", false, "snapshot missing or undecodable")
		return out
	}

	// Point probes against the analytic base model. On regions allow the
	// documented dither delta; off regions are tighter.
	type point struct {
		key        string
		card       int
		lx, ly     float64
		kind       string
		srcW, srcH int
		sx, sy     float64
		tol        float64
		desc       string
	}
	points := []point{
		{"pt_a_off", 0, 40, 20, "linear", int(halfW), int(topH), 40.5, 20.5, 3.0 / 255, "A off gradient"},
		{"pt_a_on", 0, halfX + 40, 20, "linear", int(halfW), int(topH), 40.5, 20.5, 5.0 / 255, "A on gradient"},
		{"pt_a_zoom_off", 0, 48, zoomY + 16, "linear", 24, 16, 12.5, 4.5, 5.0 / 255, "A 4x zoom off"},
		{"pt_a_zoom_on", 0, halfX + 48, zoomY + 16, "linear", 24, 16, 12.5, 4.5, 6.0 / 255, "A 4x zoom on"},
		{"pt_b_off", 1, 99, 92, "radial", int(halfW), int(contentH), 99.5, 92.5, 4.0 / 255, "B radial center off"},
		{"pt_b_on", 1, halfX + 99, 92, "radial", int(halfW), int(contentH), 99.5, 92.5, 5.0 / 255, "B radial center on"},
		{"pt_c_off", 2, 40, 20, "sweep", int(halfW), int(contentH), 40.5, 20.5, 4.0 / 255, "C sweep off"},
		{"pt_c_on", 2, halfX + 40, 20, "sweep", int(halfW), int(contentH), 40.5, 20.5, 5.0 / 255, "C sweep on"},
		{"pt_d_off", 3, 40, 92, "solid", int(halfW), int(contentH), 40.5, 92.5, 3.0 / 255, "D solid off"},
		{"pt_d_on", 3, halfX + 40, 92, "solid", int(halfW), int(contentH), 40.5, 92.5, 4.0 / 255, "D solid on"},
		{"pt_card_bg", 0, 203, 92, "", 0, 0, 0, 0, 2.0 / 255, "card background gap"},
	}
	for _, p := range points {
		r := snapshotRect(bodyX, bodyY, dpr, p.card, p.lx, p.ly, 1, 1)
		cr, cg, cb, _ := snap.At(r.Min.X, r.Min.Y).RGBA()
		got := [3]float64{float64(cr>>8) / 255, float64(cg>>8) / 255, float64(cb>>8) / 255}
		want := [3]float64{colCardBG[0], colCardBG[1], colCardBG[2]}
		if p.kind != "" {
			c := oracleColor(p.kind, p.sx, p.sy, p.srcW, p.srcH)
			want = [3]float64{c.R, c.G, c.B}
		}
		delta := math.Max(math.Abs(got[0]-want[0]), math.Max(math.Abs(got[1]-want[1]), math.Abs(got[2]-want[2])))
		add(p.key, delta <= p.tol,
			fmt.Sprintf("%s got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) Δ=%.4f tol=%.4f",
				p.desc, got[0], got[1], got[2], want[0], want[1], want[2], delta, p.tol))
	}

	// Region probes on the actual snapshot.
	aOff := snapshotRect(bodyX, bodyY, dpr, 0, 0, 0, halfW, topH)
	aOn := snapshotRect(bodyX, bodyY, dpr, 0, halfX, 0, halfW, topH)
	bOff := snapshotRect(bodyX, bodyY, dpr, 1, 0, 0, halfW, contentH)
	bOn := snapshotRect(bodyX, bodyY, dpr, 1, halfX, 0, halfW, contentH)
	cOff := snapshotRect(bodyX, bodyY, dpr, 2, 0, 0, halfW, contentH)
	cOn := snapshotRect(bodyX, bodyY, dpr, 2, halfX, 0, halfW, contentH)
	dOff := snapshotRect(bodyX, bodyY, dpr, 3, 0, 0, halfW, topH)
	dOn := snapshotRect(bodyX, bodyY, dpr, 3, halfX, 0, halfW, topH)

	// A: vertical gradient span on the off half.
	topA := image.Rect(aOff.Min.X+20, aOff.Min.Y+12, aOff.Max.X-20, aOff.Min.Y+22)
	botA := image.Rect(aOff.Min.X+20, aOff.Max.Y-22, aOff.Max.X-20, aOff.Max.Y-12)
	_, topMean, _ := regionStats(snap, topA)
	_, botMean, _ := regionStats(snap, botA)
	spanA := math.Abs(topMean[0]-botMean[0]) + math.Abs(topMean[1]-botMean[1]) + math.Abs(topMean[2]-botMean[2])
	add("rg_a_gradient_span", spanA > 8.0/255.0, fmt.Sprintf("A off top/bottom mean span=%.4f", spanA))

	// A on: same base, more colors, bounded delta.
	uAOff, _, _ := regionStats(snap, aOff)
	uAOn, _, _ := regionStats(snap, aOn)
	add("rg_a_on_more_colors", uAOn > uAOff, fmt.Sprintf("A unique off=%d on=%d", uAOff, uAOn))
	dA, tA := regionDiffCount(snap, snap, aOff, aOn)
	add("rg_a_on_differs", dA*100 >= tA*60, fmt.Sprintf("A off/on differ %d/%d (%.1f%%)", dA, tA, 100*float64(dA)/float64(tA)))
	add("rg_a_mean_bounded", regionMeanDiff(snap, snap, aOff, aOn) <= 3.0/255.0,
		fmt.Sprintf("A mean|Δ|/255=%.4f", regionMeanDiff(snap, snap, aOff, aOn)))

	// A 4x nearest zoom: one source pixel must become a uniform 4x4 block.
	zoomBlock := snapshotRect(bodyX, bodyY, dpr, 0, 3*zoomS, zoomY+3*zoomS, zoomS, zoomS)
	uBlock, _, _ := regionStats(snap, zoomBlock)
	add("rg_a_zoom_nearest_block", uBlock == 1, fmt.Sprintf("A 4x block unique=%d (want 1)", uBlock))

	// B: radial center brighter than the corner.
	bc := image.Rect(bOff.Min.X+80, bOff.Min.Y+72, bOff.Min.X+120, bOff.Min.Y+112)
	bk := image.Rect(bOff.Min.X+8, bOff.Min.Y+8, bOff.Min.X+36, bOff.Min.Y+36)
	_, bcm, _ := regionStats(snap, bc)
	_, bkm, _ := regionStats(snap, bk)
	add("rg_b_radial_center", bcm[1] > bkm[1]+0.20, fmt.Sprintf("B center G=%.3f corner G=%.3f", bcm[1], bkm[1]))
	uBOff, _, _ := regionStats(snap, bOff)
	uBOn, _, _ := regionStats(snap, bOn)
	add("rg_b_on_more_colors", uBOn > uBOff, fmt.Sprintf("B unique off=%d on=%d", uBOff, uBOn))
	dB, tB := regionDiffCount(snap, snap, bOff, bOn)
	add("rg_b_on_differs", dB*100 >= tB*60, fmt.Sprintf("B off/on differ %d/%d (%.1f%%)", dB, tB, 100*float64(dB)/float64(tB)))

	// C: sweep hue must span multiple quadrants.
	q1 := image.Rect(cOff.Min.X+18, cOff.Min.Y+18, cOff.Min.X+48, cOff.Min.Y+48)
	q2 := image.Rect(cOff.Max.X-48, cOff.Min.Y+18, cOff.Max.X-18, cOff.Min.Y+48)
	q3 := image.Rect(cOff.Min.X+18, cOff.Max.Y-48, cOff.Min.X+48, cOff.Max.Y-18)
	q4 := image.Rect(cOff.Max.X-48, cOff.Max.Y-48, cOff.Max.X-18, cOff.Max.Y-18)
	_, m1, _ := regionStats(snap, q1)
	_, m2, _ := regionStats(snap, q2)
	_, m3, _ := regionStats(snap, q3)
	_, m4, _ := regionStats(snap, q4)
	hueSpan := 0.0
	for ch := 0; ch < 3; ch++ {
		mn := math.Min(math.Min(m1[ch], m2[ch]), math.Min(m3[ch], m4[ch]))
		mx := math.Max(math.Max(m1[ch], m2[ch]), math.Max(m3[ch], m4[ch]))
		if mx-mn > hueSpan {
			hueSpan = mx - mn
		}
	}
	add("rg_c_sweep_hue_span", hueSpan > 0.30, fmt.Sprintf("C quadrant channel span=%.3f", hueSpan))
	uCOff, _, _ := regionStats(snap, cOff)
	uCOn, _, _ := regionStats(snap, cOn)
	add("rg_c_on_more_colors", uCOn > uCOff, fmt.Sprintf("C unique off=%d on=%d", uCOff, uCOn))
	dC, tC := regionDiffCount(snap, snap, cOff, cOn)
	add("rg_c_on_differs", dC*100 >= tC*60, fmt.Sprintf("C off/on differ %d/%d (%.1f%%)", dC, tC, 100*float64(dC)/float64(tC)))

	// D: flat fill, the known whole-pixmap limit. Stay off the 1px frame /
	// AA edge: the interior of the off fill must be a single RGB value.
	dOffIn := dOff.Inset(8)
	dOnIn := dOn.Inset(8)
	uDOff, _, _ := regionStats(snap, dOffIn)
	add("rg_d_solid_off_uniform", uDOff == 1, fmt.Sprintf("D off interior unique=%d (want 1)", uDOff))
	dD, tD := regionDiffCount(snap, snap, dOffIn, dOnIn)
	add("rg_d_solid_on_differs", dD*100 >= tD*80, fmt.Sprintf("D off/on differ %d/%d (%.1f%%)", dD, tD, 100*float64(dD)/float64(tD)))
	add("rg_d_mean_bounded", regionMeanDiff(snap, snap, dOffIn, dOnIn) <= 3.0/255.0,
		fmt.Sprintf("D mean|Δ|/255=%.4f", regionMeanDiff(snap, snap, dOffIn, dOnIn)))
	dZoomBlock := snapshotRect(bodyX, bodyY, dpr, 3, 3*zoomS, zoomY+3*zoomS, zoomS, zoomS)
	uDZoom, _, _ := regionStats(snap, dZoomBlock)
	add("rg_d_zoom_nearest_block", uDZoom == 1, fmt.Sprintf("D 4x block unique=%d (want 1)", uDZoom))

	// Baked checks: exact Bayer period on the flat on image, and visible
	// amplified diff panel.
	if imgDZoomOn != nil && imgDZoomOn.std != nil {
		r := imgDZoomOn.std.Bounds().Inset(2)
		ok := true
		for y := r.Min.Y; y < r.Max.Y-4 && ok; y++ {
			for x := r.Min.X; x < r.Max.X-8; x += 4 {
				if !sameRGB(imgDZoomOn.std, x, y, x+4, y) {
					ok = false
					break
				}
			}
		}
		add("baked_d_bayer4_period", ok, "D on baked zoom image RGB period=4 in interior")
	}
	for _, d := range []struct {
		key string
		img *bakedImage
	}{
		{"baked_a_diff_visible", imgADiff},
		{"baked_d_diff_visible", imgDDiff},
	} {
		if d.img == nil || d.img.std == nil {
			continue
		}
		b := d.img.std.Bounds()
		nonDark, total := 0, 0
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, _ := d.img.std.At(x, y).RGBA()
				total++
				if r>>8 > 8 || g>>8 > 8 || bl>>8 > 8 {
					nonDark++
				}
			}
		}
		add(d.key, nonDark*100 >= total*5,
			fmt.Sprintf("%s non-dark %d/%d (%.1f%%)", d.key, nonDark, total, 100*float64(nonDark)/float64(total)))
	}

	return out
}
