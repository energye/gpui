package main

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"os"

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

func webpImageBytes(img image.Image) []byte {
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

func webpChannelDiff(a, b image.Image) (maxDelta, diffSamples int) {
	if a == nil || b == nil {
		return 255, 1 << 30
	}
	ab, bb := a.Bounds(), b.Bounds()
	w, h := ab.Dx(), bb.Dx()
	if ab.Dy() < h {
		h = ab.Dy()
	}
	if bb.Dy() < h {
		h = bb.Dy()
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ar, ag, abl, aa := a.At(ab.Min.X+x, ab.Min.Y+y).RGBA()
			br, bg, bbl, ba := b.At(bb.Min.X+x, bb.Min.Y+y).RGBA()
			for _, d := range []int{
				int(ar>>8) - int(br>>8),
				int(ag>>8) - int(bg>>8),
				int(abl>>8) - int(bbl>>8),
				int(aa>>8) - int(ba>>8),
			} {
				if d < 0 {
					d = -d
				}
				if d > maxDelta {
					maxDelta = d
				}
				if d != 0 {
					diffSamples++
				}
			}
		}
	}
	return maxDelta, diffSamples
}

func webpAlphaHistogram(img image.Image) (zero, partial, opaque int) {
	if img == nil {
		return 0, 0, 0
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			switch a >> 8 {
			case 0:
				zero++
			case 255:
				opaque++
			default:
				partial++
			}
		}
	}
	return zero, partial, opaque
}

func webpMeanAbsRGB(a, b image.Image) float64 {
	if a == nil || b == nil {
		return math.Inf(1)
	}
	ab, bb := a.Bounds(), b.Bounds()
	w, h := ab.Dx(), bb.Dx()
	if ab.Dy() < h {
		h = ab.Dy()
	}
	if bb.Dy() < h {
		h = bb.Dy()
	}
	if w <= 0 || h <= 0 {
		return math.Inf(1)
	}
	var sum float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ar, ag, abl, _ := a.At(ab.Min.X+x, ab.Min.Y+y).RGBA()
			br, bg, bbl, _ := b.At(bb.Min.X+x, bb.Min.Y+y).RGBA()
			sum += math.Abs(float64(ar>>8)-float64(br>>8)) / 255
			sum += math.Abs(float64(ag>>8)-float64(bg>>8)) / 255
			sum += math.Abs(float64(abl>>8)-float64(bbl>>8)) / 255
		}
	}
	return sum / float64(w*h*3)
}

func webpDistinctColors(img image.Image) int {
	if img == nil {
		return 0
	}
	seen := map[[3]uint8]struct{}{}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			seen[[3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8)}] = struct{}{}
		}
	}
	return len(seen)
}

func runLogicProbes() []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}

	lossless, err := render.LoadWebP(assetDir + "/webp_lossless.webp")
	if err != nil {
		add("webp_load_lossless_ok", false, err.Error())
		return out
	}
	losslessStd := lossless.ToStdImage()
	add("webp_load_lossless_ok", losslessStd != nil, fmt.Sprintf("%dx%d", lossless.Width(), lossless.Height()))
	add("webp_lossless_dimensions", lossless.Width() == 96 && lossless.Height() == 64,
		fmt.Sprintf("got %dx%d want 96x64", lossless.Width(), lossless.Height()))
	add("webp_lossless_format", lossless.Format() == render.FormatRGBA8,
		fmt.Sprintf("format=%v want RGBA8", lossless.Format()))

	ref, err := render.LoadImage(assetDir + "/webp_lossless_ref.png")
	if err != nil {
		add("webp_lossless_exact_vs_png", false, "PNG reference: "+err.Error())
	} else {
		maxDelta, diffSamples := webpChannelDiff(losslessStd, ref.ToStdImage())
		add("webp_lossless_exact_vs_png", maxDelta == 0 && diffSamples == 0,
			fmt.Sprintf("max channel Δ=%d differing samples=%d (want 0/0)", maxDelta, diffSamples))
	}

	zero, partial, opaque := webpAlphaHistogram(losslessStd)
	add("webp_alpha_zero_preserved", zero == 379, fmt.Sprintf("alpha=0 pixels=%d want 379", zero))
	add("webp_alpha_partial_preserved", partial == 64, fmt.Sprintf("alpha 1..254 pixels=%d want 64", partial))
	add("webp_alpha_opaque_preserved", opaque == 5701, fmt.Sprintf("alpha=255 pixels=%d want 5701", opaque))

	byExt, errExt := render.LoadImage(assetDir + "/webp_lossless.webp")
	byBody, errBody := render.LoadImage(assetDir + "/webp_lossless.bin")
	add("webp_loadimage_ext", errExt == nil && byExt != nil, fmt.Sprintf("err=%v", errExt))
	if byExt != nil {
		maxDelta, diffSamples := webpChannelDiff(losslessStd, byExt.ToStdImage())
		add("webp_loadimage_ext_pixels", maxDelta == 0 && diffSamples == 0,
			fmt.Sprintf("max channel Δ=%d differing samples=%d", maxDelta, diffSamples))
	} else {
		add("webp_loadimage_ext_pixels", false, "nil buffer")
	}
	add("webp_loadimage_content_sniff", errBody == nil && byBody != nil,
		fmt.Sprintf("err=%v", errBody))
	if byBody != nil {
		maxDelta, diffSamples := webpChannelDiff(losslessStd, byBody.ToStdImage())
		add("webp_paths_pixel_equal", maxDelta == 0 && diffSamples == 0,
			fmt.Sprintf("LoadImage(.bin) vs LoadWebP max channel Δ=%d samples=%d", maxDelta, diffSamples))
	} else {
		add("webp_paths_pixel_equal", false, "nil buffer")
	}

	lossy, err := render.LoadWebP(assetDir + "/webp_lossy.webp")
	if err != nil {
		add("webp_lossy_ok", false, err.Error())
	} else {
		std := lossy.ToStdImage()
		add("webp_lossy_ok", lossy.Width() == 160 && lossy.Height() == 120,
			fmt.Sprintf("%dx%d", lossy.Width(), lossy.Height()))
		_, _, opaque := webpAlphaHistogram(std)
		add("webp_lossy_opaque", opaque == 160*120, fmt.Sprintf("opaque=%d want 19200", opaque))
		colors := webpDistinctColors(std)
		add("webp_lossy_diverse", colors >= 1000, fmt.Sprintf("distinct colors=%d want >=1000", colors))
		lossyRef, refErr := render.LoadImage(assetDir + "/webp_lossy_ref.png")
		if refErr != nil {
			add("webp_lossy_close_to_ref", false, refErr.Error())
		} else {
			mean := webpMeanAbsRGB(std, lossyRef.ToStdImage())
			add("webp_lossy_close_to_ref", mean <= 0.05, fmt.Sprintf("mean |Δ|/255=%.5f want <=0.05", mean))
		}
	}

	second, err := render.LoadWebP(assetDir + "/webp_lossless.webp")
	if err != nil || second == nil {
		add("webp_deterministic", false, fmt.Sprintf("second load: %v", err))
	} else {
		add("webp_deterministic", bytes.Equal(webpImageBytes(losslessStd), webpImageBytes(second.ToStdImage())),
			"two decodes of the same file are byte-identical")
	}

	if buf, err := render.LoadWebP(assetDir + "/does-not-exist.webp"); err == nil || buf != nil {
		add("webp_missing_error", false, "expected (nil, error)")
	} else {
		add("webp_missing_error", true, err.Error())
	}
	if buf, err := render.LoadWebP(assetDir + "/webp_lossless_ref.png"); err == nil || buf != nil {
		add("webp_wrong_format_error", false, "PNG bytes unexpectedly decoded as WebP")
	} else {
		add("webp_wrong_format_error", true, err.Error())
	}
	empty, err := os.CreateTemp("", "gpui_webp_empty_*.webp")
	if err != nil {
		add("webp_empty_error", false, "temp: "+err.Error())
	} else {
		path := empty.Name()
		_ = empty.Close()
		buf, loadErr := render.LoadWebP(path)
		_ = os.Remove(path)
		add("webp_empty_error", loadErr != nil && buf == nil, fmt.Sprintf("err=%v", loadErr))
	}

	lossless.Dispose()
	if ref != nil {
		ref.Dispose()
	}
	if byExt != nil {
		byExt.Dispose()
	}
	if byBody != nil {
		byBody.Dispose()
	}
	if lossy != nil {
		lossy.Dispose()
	}
	if second != nil {
		second.Dispose()
	}
	return out
}

// ---------------------------------------------------------------------------
// Pixel probes (snapshot + decoded oracle)
// ---------------------------------------------------------------------------

func snapshotRect(bodyX, bodyY, dpr float64, card int, lx, ly, w, h float64) image.Rectangle {
	x0 := (bodyX + cardAt[card][0] + lx) * dpr
	y0 := (bodyY + cardAt[card][1] + ly) * dpr
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

func pixelRGB(img image.Image, x, y int) [3]float64 {
	r, g, b, _ := img.At(x, y).RGBA()
	return [3]float64{float64(r>>8) / 255, float64(g>>8) / 255, float64(b>>8) / 255}
}

func sourceRGB(img image.Image, x, y int) [3]float64 {
	b := img.Bounds()
	return pixelRGB(img, b.Min.X+x, b.Min.Y+y)
}

func maxChannelDelta(a, b [3]float64) float64 {
	return math.Max(math.Abs(a[0]-b[0]), math.Max(math.Abs(a[1]-b[1]), math.Abs(a[2]-b[2])))
}

func blendOver(src, bg [3]float64, alpha float64) [3]float64 {
	return [3]float64{
		src[0]*alpha + bg[0]*(1-alpha),
		src[1]*alpha + bg[1]*(1-alpha),
		src[2]*alpha + bg[2]*(1-alpha),
	}
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
	ensureAssets()
	if assetErr != nil {
		add("pixel_assets", false, assetErr.Error())
		return out
	}

	// A: 2x lossless over the checker. Point probes map a source pixel to the
	// exact 2x destination; transparent/partial pixels compare with the
	// checker background model.
	type point struct {
		key    string
		card   int
		lx, ly float64
		want   [3]float64
		tol    float64
		detail string
	}
	points := []point{
		{"pt_a_lossless_red", 0, losslessX + 2*20, losslessY + 2*10, sourceRGB(losslessWebP.std, 20, 10), 2.0 / 255, "A 2x source(20,10) red"},
		{"pt_a_lossless_green", 0, losslessX + 2*70, losslessY + 2*10, sourceRGB(losslessWebP.std, 70, 10), 2.0 / 255, "A 2x source(70,10) green"},
		{"pt_a_lossless_blue", 0, losslessX + 2*20, losslessY + 2*52, sourceRGB(losslessWebP.std, 20, 52), 2.0 / 255, "A 2x source(20,52) blue"},
		{"pt_a_partial_alpha", 0, losslessX + 2*10, losslessY + 2*44,
			blendOver([3]float64{1, 1, 1}, checkerColor(losslessX+2*10-contentX0, losslessY+2*44-contentY0), 128.0/255.0), 4.0 / 255,
			"A 2x alpha=128 over checker"},
		{"pt_a_alpha_hole", 0, losslessX + 2*80, losslessY + 2*52,
			checkerColor(losslessX+2*80-contentX0, losslessY+2*52-contentY0), 2.0 / 255, "A alpha=0 shows checker"},
		{"pt_a_ref_png", 0, refX + 20, refY + 10, sourceRGB(refPNG.std, 20, 10), 2.0 / 255, "A 1x PNG reference red"},
		{"pt_a_zoom", 0, zoomX + 4*4, zoomY + 4*4, sourceRGB(losslessWebP.std, zoomCropX+4, zoomCropY+4), 2.0 / 255, "A 4x crop"},
		{"pt_b_lossy_left", 1, lossyX + 20, lossyY + 20, sourceRGB(lossyWebP.std, 20, 20), 3.0 / 255, "B lossy gradient"},
		{"pt_b_lossy_light", 1, lossyX + 122, lossyY + 44, sourceRGB(lossyWebP.std, 122, 44), 3.0 / 255, "B lossy light circle"},
		{"pt_b_lossy_dark", 1, lossyX + 40, lossyY + 90, sourceRGB(lossyWebP.std, 40, 90), 3.0 / 255, "B lossy dark panel"},
		{"pt_b_zoom", 1, lossyZoomX + 2*10, lossyZoomY + 2*10, sourceRGB(lossyWebP.std, lossyZoomCropX+10, lossyZoomCropY+10), 3.0 / 255, "B 2x crop"},
	}
	// C: four thumbnails, same source pixel in each path.
	for i := 0; i < 4; i++ {
		cx := contentX0 + float64(i%2)*halfX + (cellW-thumbW)/2
		cy := contentY0 + float64(i/2)*cellH + 22
		points = append(points, point{
			key: fmt.Sprintf("pt_c_path_%d", i), card: 2, lx: cx + 20, ly: cy + 10,
			want: sourceRGB(refPNG.std, 20, 10), tol: 2.0 / 255,
			detail: fmt.Sprintf("C thumb %d red", i),
		})
	}
	// D: red/green badges and the card footer.
	points = append(points,
		point{"pt_d_red_badge", 3, contentX0 + 6, contentY0 + 24 + 3, [3]float64{0.86, 0.20, 0.18}, 4.0 / 255, "D red error badge"},
		point{"pt_d_green_badge", 3, contentX0 + 6, contentY0 + 162 + 3, [3]float64{0.25, 0.72, 0.40}, 4.0 / 255, "D green control badge"},
		point{"pt_card_bg", 0, contentX0 + halfW + 4, contentY0 + 100, [3]float64{colCardBG[0], colCardBG[1], colCardBG[2]}, 2.0 / 255, "A/B card gap"},
	)
	for _, p := range points {
		r := snapshotRect(bodyX, bodyY, dpr, p.card, p.lx, p.ly, 1, 1)
		got := pixelRGB(snap, r.Min.X, r.Min.Y)
		delta := maxChannelDelta(got, p.want)
		add(p.key, delta <= p.tol,
			fmt.Sprintf("%s got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) Δ=%.4f tol=%.4f",
				p.detail, got[0], got[1], got[2], p.want[0], p.want[1], p.want[2], delta, p.tol))
	}

	// A: 4x nearest means every source pixel is a uniform 4x4 block in the
	// destination (card-local, before DPR scaling).
	aBlock := snapshotRect(bodyX, bodyY, dpr, 0, zoomX+4*4, zoomY+4*4, 4, 4)
	uA, _, _ := regionStats(snap, aBlock)
	add("rg_a_zoom_4x_block", uA == 1, fmt.Sprintf("A 4x block unique=%d want 1", uA))

	// B: 2x nearest block. Chroma may vary per source pixel, but the two
	// destination pixels from one source pixel must be identical.
	bBlock := snapshotRect(bodyX, bodyY, dpr, 1, lossyZoomX+2*10, lossyZoomY+2*10, 2, 2)
	uB, _, _ := regionStats(snap, bBlock)
	add("rg_b_zoom_2x_block", uB == 1, fmt.Sprintf("B 2x block unique=%d want 1", uB))

	// C: all four loading paths must produce the same thumbnail pixels.
	thumbs := make([]image.Rectangle, 4)
	for i := range thumbs {
		cx := contentX0 + float64(i%2)*halfX + (cellW-thumbW)/2
		cy := contentY0 + float64(i/2)*cellH + 22
		thumbs[i] = snapshotRect(bodyX, bodyY, dpr, 2, cx, cy, thumbW, thumbH)
	}
	maxDiff, total := 0, 0
	for i := 1; i < 4; i++ {
		d, t := regionDiffCount(snap, snap, thumbs[0], thumbs[i])
		if d > maxDiff {
			maxDiff = d
		}
		total += t
	}
	add("rg_c_paths_identical", maxDiff == 0, fmt.Sprintf("C thumbnails max diff=%d over %d px (want 0)", maxDiff, total))

	// C: a transparent source pixel must show the card background, not black.
	cx0 := contentX0 + (cellW-thumbW)/2
	cy0 := contentY0 + 22
	cHole := snapshotRect(bodyX, bodyY, dpr, 2, cx0+80, cy0+52, 1, 1)
	gotHole := pixelRGB(snap, cHole.Min.X, cHole.Min.Y)
	wantHole := [3]float64{colCardBG[0], colCardBG[1], colCardBG[2]}
	add("rg_c_transparent_bg", maxChannelDelta(gotHole, wantHole) <= 2.0/255,
		fmt.Sprintf("C alpha hole got=(%.3f,%.3f,%.3f) want card bg", gotHole[0], gotHole[1], gotHole[2]))

	// D: the error badges are visibly red and the control badge green.
	dCard := snapshotRect(bodyX, bodyY, dpr, 3, contentX0, contentY0, contentW, contentH)
	red, green, totalD := 0, 0, 0
	dCard = dCard.Intersect(snap.Bounds())
	for y := dCard.Min.Y; y < dCard.Max.Y; y++ {
		for x := dCard.Min.X; x < dCard.Max.X; x++ {
			r, g, b, _ := snap.At(x, y).RGBA()
			totalD++
			if r>>8 > 160 && g>>8 < 100 && b>>8 < 100 {
				red++
			}
			if g>>8 > 140 && r>>8 < 120 && b>>8 < 140 {
				green++
			}
		}
	}
	add("rg_d_red_badges", red >= 200, fmt.Sprintf("D red px=%d/%d want >=200", red, totalD))
	add("rg_d_green_badge", green >= 40, fmt.Sprintf("D green px=%d/%d want >=40", green, totalD))

	return out
}
