package main

import (
	"bytes"
	"fmt"
	"image"
	"math"

	"github.com/energye/gpui/render"
)

// ---------------------------------------------------------------------------
// Logic probes (value-level, CPU offscreen child)
// ---------------------------------------------------------------------------

type probeResult struct {
	Key    string
	OK     bool
	Detail string
}

var probeKeys = []string{
	"filter_nil_context",
	"filter_invert_exact",
	"filter_invert_twice",
	"filter_invert_translucent_premul",
	"filter_grayscale_luma709",
	"filter_blurxy_zero_noop",
	"filter_blurxy_y_only",
	"filter_blurxy_x_only",
	"filter_blurxy_alpha_uniform",
	"filter_graph_matches_sequential",
	"filter_graph_color_coalesce",
	"filter_graph_empty_noop",
	"filter_graph_skips_noop",
	"filter_graph_deterministic",
	"filter_graph_blur_matches_direct",
	"filter_texture_cpu_unavailable",
}

func drawTestContent(dc *render.Context, w, h int) {
	if dc == nil {
		return
	}
	dc.ClearWithColor(render.RGBA{R: 0.10, G: 0.20, B: 0.30, A: 1})
	dc.SetRGBA(0.90, 0.20, 0.10, 1)
	dc.DrawRectangle(4, 4, float64(w/2-6), float64(h-8))
	_ = dc.Fill()
	dc.SetRGBA(0.10, 0.80, 0.70, 1)
	dc.DrawRectangle(float64(w/2+2), 8, float64(w/2-10), float64(h-16))
	_ = dc.Fill()
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

func imageMaxDiff(a, b image.Image) int {
	if a == nil || b == nil {
		return 1 << 30
	}
	ra, rb := a.Bounds(), b.Bounds()
	w, h := ra.Dx(), ra.Dy()
	if rb.Dx() < w {
		w = rb.Dx()
	}
	if rb.Dy() < h {
		h = rb.Dy()
	}
	maxd := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r0, g0, b0, a0 := a.At(ra.Min.X+x, ra.Min.Y+y).RGBA()
			r1, g1, b1, a1 := b.At(rb.Min.X+x, rb.Min.Y+y).RGBA()
			for _, d := range []int{
				int(r0>>8) - int(r1>>8), int(g0>>8) - int(g1>>8),
				int(b0>>8) - int(b1>>8), int(a0>>8) - int(a1>>8),
			} {
				if d < 0 {
					d = -d
				}
				if d > maxd {
					maxd = d
				}
			}
		}
	}
	return maxd
}

func rawPixel(img image.Image, x, y int) [4]int {
	if img == nil {
		return [4]int{}
	}
	r, g, b, a := img.At(x, y).RGBA()
	return [4]int{int(r >> 8), int(g >> 8), int(b >> 8), int(a >> 8)}
}

func luma709(p [4]int) float64 {
	return 0.2126*float64(p[0]) + 0.7152*float64(p[1]) + 0.0722*float64(p[2])
}

func runLogicProbes() []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}
	const w, h = 40, 28

	// 1. Nil receiver: every entry point must be a no-op, and no texture.
	func() {
		defer func() {
			if r := recover(); r != nil {
				add("filter_nil_context", false, fmt.Sprintf("panic: %v", r))
			}
		}()
		var dc *render.Context
		dc.ApplyInvert()
		dc.ApplyBlurXY(1, 1)
		dc.ApplyImageFilterGraph(render.ImageFilterNode{Kind: render.ImageFilterInvert})
		_, _, _, ok := dc.GPUFilterTexture()
		add("filter_nil_context", !ok, "nil Apply* no-op, GPUFilterTexture ok=false")
	}()

	// 2. Invert is exactly 255-v on straight color, alpha untouched.
	{
		src := render.NewContext(w, h)
		drawTestContent(src, w, h)
		before := imageBytes(src.Image())
		src.ApplyInvert()
		after := imageBytes(src.Image())
		src.Close()
		bad := 0
		for i := 0; i+3 < len(before); i += 4 {
			for c := 0; c < 3; c++ {
				want := 255 - int(before[i+c])
				d := int(after[i+c]) - want
				if d < 0 {
					d = -d
				}
				if d > 1 {
					bad++
				}
			}
			if before[i+3] != after[i+3] {
				bad++
			}
		}
		add("filter_invert_exact", len(before) == len(after) && bad == 0, fmt.Sprintf("bad channels=%d", bad))
	}

	// 3. Invert twice is identity within rounding.
	{
		src := render.NewContext(w, h)
		drawTestContent(src, w, h)
		first := src.Image()
		src.ApplyInvert()
		src.ApplyInvert()
		second := src.Image()
		src.Close()
		d := imageMaxDiff(first, second)
		add("filter_invert_twice", d <= 1, fmt.Sprintf("max|Δ|=%d", d))
	}

	// 4. Invert on premultiplied storage: premul' = alpha - premul (Skia
	// color-filter semantics). The old GPU shader used 1-premul and produced
	// channels greater than alpha on translucent pixels.
	{
		dc := render.NewContext(8, 8)
		dc.ClearWithColor(render.RGBA{R: 1, G: 0.5, B: 0.25, A: 0.5})
		before := rawPixel(dc.Image(), 4, 4)
		dc.ApplyInvert()
		after := rawPixel(dc.Image(), 4, 4)
		dc.Close()
		ok := after[3] == before[3]
		for c := 0; c < 3; c++ {
			if d := absInt(after[c] - (before[3] - before[c])); d > 1 {
				ok = false
			}
		}
		if after[0] > after[3] || after[1] > after[3] || after[2] > after[3] {
			ok = false
		}
		add("filter_invert_translucent_premul", ok,
			fmt.Sprintf("src=%v -> %v want premul=alpha-src", before, after))
	}

	// 5. Grayscale uses Rec.709 luma on straight color.
	{
		src := render.NewContext(w, h)
		drawTestContent(src, w, h)
		orig := imageBytes(src.Image())
		src.ApplyGrayscale()
		gray := src.Image()
		src.Close()
		bad := 0
		maxd := 0
		for i := 0; i+3 < len(orig); i += 4 {
			want := luma709([4]int{int(orig[i]), int(orig[i+1]), int(orig[i+2]), int(orig[i+3])})
			got := rawPixel(gray, (i/4)%w, (i/4)/w)
			if got[0] != got[1] || got[1] != got[2] {
				bad++
			}
			d := absInt(got[0] - int(want+0.5))
			if d > maxd {
				maxd = d
			}
			if d > 1 {
				bad++
			}
			if got[3] != int(orig[i+3]) {
				bad++
			}
		}
		add("filter_grayscale_luma709", bad == 0, fmt.Sprintf("bad=%d max|Δ|=%d", bad, maxd))
	}

	// 6. Zero-radius blur is an identity, not a kernel pass.
	{
		dc := render.NewContext(w, h)
		drawTestContent(dc, w, h)
		before := imageBytes(dc.Image())
		dc.ApplyBlurXY(0, 0)
		after := imageBytes(dc.Image())
		dc.Close()
		add("filter_blurxy_zero_noop", bytes.Equal(before, after), "bytes identical")
	}

	// 7. ApplyBlurXY(0,ry): vertical edges soften, vertical (left/right)
	// edges stay sharp.
	{
		dc := render.NewContext(64, 64)
		dc.ClearWithColor(render.RGBA{R: 0.05, G: 0.08, B: 0.14, A: 1})
		dc.SetRGBA(0.92, 0.76, 0.18, 1)
		dc.DrawRectangle(0, 20, 64, 24)
		_ = dc.Fill()
		dc.SetRGBA(0.16, 0.82, 0.62, 1)
		dc.DrawRectangle(44, 0, 12, 64)
		_ = dc.Fill()
		src := dc.Image()
		dc.ApplyBlurXY(0, 3)
		res := dc.Image()
		dc.Close()

		center := rawPixel(res, 32, 32)
		centerSrc := rawPixel(src, 32, 32)
		above := rawPixel(res, 32, 8)
		aboveSrc := rawPixel(src, 32, 8)
		edge := rawPixel(res, 32, 18)
		bg := rawPixel(src, 32, 8)
		band := rawPixel(src, 32, 32)
		edgeMixed := edge[0] > bg[0]+8 && edge[0] < band[0]-8
		xSharp := maxDiff3(rawPixel(res, 46, 60), rawPixel(res, 42, 60)) > 80
		ok := maxDiff3(center, centerSrc) <= 1 &&
			maxDiff3(above, aboveSrc) <= 1 && edgeMixed && xSharp
		add("filter_blurxy_y_only", ok,
			fmt.Sprintf("center=%v src=%v edge=%v bg=%v band=%v xsharp=%v", center, centerSrc, edge, bg, band, xSharp))
	}

	// 8. ApplyBlurXY(rx,0): mirror image of the previous probe.
	{
		dc := render.NewContext(64, 64)
		dc.ClearWithColor(render.RGBA{R: 0.05, G: 0.08, B: 0.14, A: 1})
		dc.SetRGBA(0.16, 0.82, 0.62, 1)
		dc.DrawRectangle(20, 0, 24, 64)
		_ = dc.Fill()
		dc.SetRGBA(0.92, 0.76, 0.18, 1)
		dc.DrawRectangle(0, 44, 64, 12)
		_ = dc.Fill()
		src := dc.Image()
		dc.ApplyBlurXY(3, 0)
		res := dc.Image()
		dc.Close()

		center := rawPixel(res, 32, 32)
		centerSrc := rawPixel(src, 32, 32)
		left := rawPixel(res, 8, 32)
		leftSrc := rawPixel(src, 8, 32)
		edge := rawPixel(res, 18, 32)
		bg := rawPixel(src, 8, 32)
		bar := rawPixel(src, 32, 32)
		edgeMixed := edge[0] > bg[0]+8 && edge[0] < bar[0]-8
		// Below the horizontal bar (y>56) only the vertical bar exists; its
		// top edge at y=44 must stay sharp when ry=0.
		ySharp := maxDiff3(rawPixel(res, 60, 46), rawPixel(res, 60, 42)) > 80
		ok := maxDiff3(center, centerSrc) <= 1 && maxDiff3(left, leftSrc) <= 1 && edgeMixed && ySharp
		add("filter_blurxy_x_only", ok,
			fmt.Sprintf("center=%v src=%v edge=%v bg=%v bar=%v ysharp=%v", center, centerSrc, edge, bg, bar, ySharp))
	}

	// 9. Uniform alpha survives a blur unchanged.
	{
		dc := render.NewContext(32, 32)
		dc.ClearWithColor(render.RGBA{R: 0.2, G: 0.4, B: 0.6, A: 0.5})
		before := rawPixel(dc.Image(), 16, 16)
		dc.ApplyBlurXY(4, 4)
		after := rawPixel(dc.Image(), 16, 16)
		dc.Close()
		ok := absInt(after[3]-before[3]) <= 1 && maxDiff3(after, before) <= 1
		add("filter_blurxy_alpha_uniform", ok, fmt.Sprintf("before=%v after=%v", before, after))
	}

	// 10. The multi-node graph equals the same ops applied one by one.
	{
		mk := func() *render.Context {
			dc := render.NewContext(w, h)
			drawTestContent(dc, w, h)
			return dc
		}
		seq := mk()
		seq.ApplyBlurXY(0, 2)
		seq.ApplyGrayscale()
		seq.ApplyInvert()
		seqImg := seq.Image()
		seq.Close()

		graph := mk()
		graph.ApplyImageFilterGraph(
			render.ImageFilterNode{Kind: render.ImageFilterBlurXY, RadiusX: 0, RadiusY: 2},
			render.ImageFilterNode{Kind: render.ImageFilterGrayscale},
			render.ImageFilterNode{Kind: render.ImageFilterInvert},
		)
		graphImg := graph.Image()
		graph.Close()
		d := imageMaxDiff(seqImg, graphImg)
		add("filter_graph_matches_sequential", d <= 1, fmt.Sprintf("max|Δ|=%d", d))
	}

	// 11. Consecutive color-matrix nodes are coalesced without changing pixels.
	{
		mk := func() *render.Context {
			dc := render.NewContext(w, h)
			drawTestContent(dc, w, h)
			return dc
		}
		seq := mk()
		seq.ApplyGrayscale()
		seq.ApplyInvert()
		seqImg := seq.Image()
		seq.Close()

		graph := mk()
		graph.ApplyImageFilterGraph(
			render.ImageFilterNode{Kind: render.ImageFilterGrayscale},
			render.ImageFilterNode{Kind: render.ImageFilterInvert},
		)
		graphImg := graph.Image()
		graph.Close()
		d := imageMaxDiff(seqImg, graphImg)
		add("filter_graph_color_coalesce", d <= 1, fmt.Sprintf("max|Δ|=%d", d))
	}

	// 12. Empty graph is a no-op.
	{
		dc := render.NewContext(w, h)
		drawTestContent(dc, w, h)
		before := imageBytes(dc.Image())
		dc.ApplyImageFilterGraph()
		after := imageBytes(dc.Image())
		dc.Close()
		add("filter_graph_empty_noop", bytes.Equal(before, after), "bytes identical")
	}

	// 13. A no-op node is dropped, but the following node still runs.
	{
		mk := func() *render.Context {
			dc := render.NewContext(w, h)
			drawTestContent(dc, w, h)
			return dc
		}
		direct := mk()
		direct.ApplyInvert()
		directImg := direct.Image()
		direct.Close()

		graph := mk()
		graph.ApplyImageFilterGraph(
			render.ImageFilterNode{Kind: render.ImageFilterBlurXY, RadiusX: 0, RadiusY: 0},
			render.ImageFilterNode{Kind: render.ImageFilterInvert},
		)
		graphImg := graph.Image()
		graph.Close()
		d := imageMaxDiff(directImg, graphImg)
		add("filter_graph_skips_noop", d <= 1, fmt.Sprintf("max|Δ|=%d", d))
	}

	// 14. Same input + same graph is deterministic.
	{
		mk := func() *render.Context {
			dc := render.NewContext(w, h)
			drawTestContent(dc, w, h)
			dc.ApplyImageFilterGraph(
				render.ImageFilterNode{Kind: render.ImageFilterBlurXY, RadiusX: 1, RadiusY: 2},
				render.ImageFilterNode{Kind: render.ImageFilterGrayscale},
				render.ImageFilterNode{Kind: render.ImageFilterInvert},
			)
			return dc
		}
		a := mk()
		b := mk()
		eq := bytes.Equal(imageBytes(a.Image()), imageBytes(b.Image()))
		a.Close()
		b.Close()
		add("filter_graph_deterministic", eq, "two identical graphs byte-identical")
	}

	// 15. A one-node blur graph matches ApplyBlurXY directly.
	{
		mk := func() *render.Context {
			dc := render.NewContext(w, h)
			drawTestContent(dc, w, h)
			return dc
		}
		direct := mk()
		direct.ApplyBlurXY(1, 2)
		directImg := direct.Image()
		direct.Close()

		graph := mk()
		graph.ApplyImageFilterGraph(
			render.ImageFilterNode{Kind: render.ImageFilterBlurXY, RadiusX: 1, RadiusY: 2},
		)
		graphImg := graph.Image()
		graph.Close()
		d := imageMaxDiff(directImg, graphImg)
		add("filter_graph_blur_matches_direct", d <= 1, fmt.Sprintf("max|Δ|=%d", d))
	}

	// 16. The CPU path publishes no GPU texture (the zero-copy view is a GPU
	// capability only; the GPU child of this window pins the other side).
	{
		dc := render.NewContext(w, h)
		drawTestContent(dc, w, h)
		dc.ApplyInvert()
		_, vw, vh, ok := dc.GPUFilterTexture()
		dc.Close()
		add("filter_texture_cpu_unavailable", !ok, fmt.Sprintf("ok=%v %dx%d", ok, vw, vh))
	}

	return out
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func maxDiff3(a, b [4]int) int {
	d := 0
	for i := 0; i < 3; i++ {
		if v := absInt(a[i] - b[i]); v > d {
			d = v
		}
	}
	return d
}

// ---------------------------------------------------------------------------
// Pixel probes (snapshot)
// ---------------------------------------------------------------------------

func snapshotRect(bodyX, bodyY, dpr float64, card int, lx, ly, w, h float64) image.Rectangle {
	x0 := (bodyX + cardAt[card][0] + contentX0 + lx) * dpr
	y0 := (bodyY + cardAt[card][1] + contentY0 + ly) * dpr
	x1 := x0 + w*dpr
	y1 := y0 + h*dpr
	return image.Rect(int(math.Round(x0)), int(math.Round(y0)), int(math.Round(x1)), int(math.Round(y1)))
}

func readPixel(img image.Image, r image.Rectangle) ([3]float64, bool) {
	var out [3]float64
	if img == nil || r.Empty() {
		return out, false
	}
	p := r.Min
	if p.X < img.Bounds().Min.X || p.Y < img.Bounds().Min.Y ||
		p.X >= img.Bounds().Max.X || p.Y >= img.Bounds().Max.Y {
		return out, false
	}
	cr, cg, cb, _ := img.At(p.X, p.Y).RGBA()
	out = [3]float64{float64(cr>>8) / 255, float64(cg>>8) / 255, float64(cb>>8) / 255}
	return out, true
}

func near3(a, b [3]float64, tol float64) bool {
	return math.Abs(a[0]-b[0]) <= tol && math.Abs(a[1]-b[1]) <= tol && math.Abs(a[2]-b[2]) <= tol
}

func rgb3(p [4]int) [3]float64 {
	return [3]float64{float64(p[0]) / 255, float64(p[1]) / 255, float64(p[2]) / 255}
}

func regionSpread(img image.Image, r image.Rectangle) (int, int) {
	if img == nil {
		return 1 << 30, 0
	}
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return 1 << 30, 0
	}
	var mn [3]int = [3]int{255, 255, 255}
	var mx [3]int
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			v := [3]int{int(cr >> 8), int(cg >> 8), int(cb >> 8)}
			for c := 0; c < 3; c++ {
				if v[c] < mn[c] {
					mn[c] = v[c]
				}
				if v[c] > mx[c] {
					mx[c] = v[c]
				}
			}
		}
	}
	spread := 0
	for c := 0; c < 3; c++ {
		if d := mx[c] - mn[c]; d > spread {
			spread = d
		}
	}
	return spread, r.Dx() * r.Dy()
}

func regionMean(img image.Image, r image.Rectangle) ([3]float64, int) {
	var mean [3]float64
	if img == nil {
		return mean, 0
	}
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return mean, 0
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			mean[0] += float64(cr >> 8)
			mean[1] += float64(cg >> 8)
			mean[2] += float64(cb >> 8)
		}
	}
	n := float64(r.Dx() * r.Dy())
	mean[0] /= n
	mean[1] /= n
	mean[2] /= n
	return mean, r.Dx() * r.Dy()
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

	point := func(card int, lx, ly float64) ([3]float64, bool) {
		return readPixel(snap, snapshotRect(bodyX, bodyY, dpr, card, lx, ly, 1, 1))
	}

	// A: source and inverted result. Every expectation is derived from the
	// source panel of the same snapshot, so palette/rounding drift shows up.
	for _, ly := range []float64{16, 48, 81, 113} {
		src, okS := point(0, 40, ly)
		res, okR := point(0, halfX+40, ly)
		want := [3]float64{1 - src[0], 1 - src[1], 1 - src[2]}
		add(fmt.Sprintf("px_a_invert_y%d", int(ly)), okS && okR && near3(res, want, 3.0/255),
			fmt.Sprintf("src=(%.3f,%.3f,%.3f) res=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f)",
				src[0], src[1], src[2], res[0], res[1], res[2], want[0], want[1], want[2]))
	}
	{
		srcR := snapshotRect(bodyX, bodyY, dpr, 0, 0, 0, halfW, topH)
		resR := snapshotRect(bodyX, bodyY, dpr, 0, halfX, 0, halfW, topH)
		maxd, n := 0, 0
		badX, badY := -1, -1
		var badSrc, badRes [3]int
		for dy := 0; dy < resR.Dy() && dy < srcR.Dy(); dy++ {
			for dx := 0; dx < resR.Dx() && dx < srcR.Dx(); dx++ {
				sr, sg, sb, _ := snap.At(srcR.Min.X+dx, srcR.Min.Y+dy).RGBA()
				rr, rg, rb, _ := snap.At(resR.Min.X+dx, resR.Min.Y+dy).RGBA()
				for c, d := range []int{
					255 - int(sr>>8) - int(rr>>8),
					255 - int(sg>>8) - int(rg>>8),
					255 - int(sb>>8) - int(rb>>8),
				} {
					if d < 0 {
						d = -d
					}
					if d > maxd {
						maxd = d
						badX, badY = dx, dy
						badSrc = [3]int{int(sr >> 8), int(sg >> 8), int(sb >> 8)}
						badRes = [3]int{int(rr >> 8), int(rg >> 8), int(rb >> 8)}
					}
					_ = c
				}
				n++
			}
		}
		add("rg_a_invert_all", n > 0 && maxd <= 2,
			fmt.Sprintf("max|res-(255-src)|=%d at local(%d,%d) src=%v res=%v over %d px",
				maxd, badX, badY, badSrc, badRes, n))
	}

	// B: vertical blur only.
	{
		srcBand, _ := point(1, 60, 65)
		resBand, _ := point(1, halfX+60, 65)
		srcBG, _ := point(1, 60, 10)
		resBG, _ := point(1, halfX+60, 10)
		edge, _ := point(1, halfX+60, 40)
		srcBar, _ := point(1, 155, 110)
		resBar, _ := point(1, halfX+155, 110)
		srcLeft, _ := point(1, 135, 110)
		resLeft, _ := point(1, halfX+135, 110)
		add("px_b_band_center", near3(resBand, srcBand, 2.0/255),
			fmt.Sprintf("src=%.3f res=%.3f", srcBand[0], resBand[0]))
		add("px_b_far_above", near3(resBG, srcBG, 2.0/255),
			fmt.Sprintf("src=%.3f res=%.3f", srcBG[0], resBG[0]))
		lo, hi := math.Min(srcBG[0], srcBand[0]), math.Max(srcBG[0], srcBand[0])
		add("px_b_edge_soft", edge[0] > lo+8.0/255 && edge[0] < hi-8.0/255,
			fmt.Sprintf("edge=%.3f between bg=%.3f band=%.3f", edge[0], lo, hi))
		add("px_b_bar_kept", near3(resBar, srcBar, 2.0/255) && near3(resLeft, srcLeft, 2.0/255),
			fmt.Sprintf("bar=%.3f src=%.3f left=%.3f", resBar[0], srcBar[0], resLeft[0]))

		bandRegion := snapshotRect(bodyX, bodyY, dpr, 1, 10, 60, 120, 10)
		spread, n := regionSpread(snap, bandRegion)
		add("rg_b_band_flat", spread <= 2 && n > 0, fmt.Sprintf("spread=%d over %d px", spread, n))

		leftR := snapshotRect(bodyX, bodyY, dpr, 1, 132, 100, 6, 20)
		barR := snapshotRect(bodyX, bodyY, dpr, 1, 144, 100, 6, 20)
		lm, _ := regionMean(snap, leftR)
		bm, _ := regionMean(snap, barR)
		ls, _ := regionSpread(snap, leftR)
		bs, _ := regionSpread(snap, barR)
		barDiff := math.Max(math.Abs(bm[0]-lm[0]), math.Max(math.Abs(bm[1]-lm[1]), math.Abs(bm[2]-lm[2])))
		add("rg_b_x_sharp", barDiff > 80 && ls <= 2 && bs <= 2,
			fmt.Sprintf("left=(%.1f,%.1f,%.1f) bar=(%.1f,%.1f,%.1f) maxΔ=%.1f spreads=%d/%d",
				lm[0], lm[1], lm[2], bm[0], bm[1], bm[2], barDiff, ls, bs))

		farR := snapshotRect(bodyX, bodyY, dpr, 1, halfX+10, 10, 120, 3)
		haloR := snapshotRect(bodyX, bodyY, dpr, 1, halfX+10, 40, 120, 3)
		fm, _ := regionMean(snap, farR)
		hm, _ := regionMean(snap, haloR)
		add("rg_b_edge_halo", hm[0] > fm[0]+8,
			fmt.Sprintf("far=%.1f halo=%.1f", fm[0], hm[0]))
	}

	// C: graph result is 255-luma, with the blur transition preserved and the
	// vertical boundary sharp (radiusX=0).
	{
		srcL, _ := point(2, 50, 60)
		resL, _ := point(2, halfX+50, 60)
		srcR, _ := point(2, 150, 60)
		resR, _ := point(2, halfX+150, 60)
		srcT, _ := point(2, 50, 10)
		resT, _ := point(2, halfX+50, 10)
		gray := func(p [3]float64) [3]float64 {
			v := 1 - (0.2126*p[0] + 0.7152*p[1] + 0.0722*p[2])
			return [3]float64{v, v, v}
		}
		for _, tc := range []struct {
			key string
			src [3]float64
			res [3]float64
		}{
			{"px_c_left", srcL, resL},
			{"px_c_right", srcR, resR},
			{"px_c_top", srcT, resT},
		} {
			want := gray(tc.src)
			spread := math.Max(math.Abs(tc.res[0]-tc.res[1]), math.Max(math.Abs(tc.res[1]-tc.res[2]), math.Abs(tc.res[0]-tc.res[2])))
			add(tc.key, spread <= 1.0/255 && near3(tc.res, want, 4.0/255),
				fmt.Sprintf("res=(%.3f,%.3f,%.3f) want=%.3f spread=%.3f", tc.res[0], tc.res[1], tc.res[2], want[0], spread))
		}

		grayRegion := snapshotRect(bodyX, bodyY, dpr, 2, halfX, 38, halfW, 80)
		spread, n := regionGraySpread(snap, grayRegion)
		add("rg_c_gray_region", spread <= 2 && n > 0, fmt.Sprintf("max channel spread=%d over %d px", spread, n))

		col := snapshotRect(bodyX, bodyY, dpr, 2, halfX+50, 14, 1, 24)
		uniq := columnUnique(snap, col)
		add("rg_c_blur_transition", uniq >= 6, fmt.Sprintf("distinct gray levels=%d", uniq))

		vl := snapshotRect(bodyX, bodyY, dpr, 2, halfX+94, 40, 4, 40)
		vr := snapshotRect(bodyX, bodyY, dpr, 2, halfX+102, 40, 4, 40)
		ml, _ := regionMean(snap, vl)
		mr, _ := regionMean(snap, vr)
		add("rg_c_vertical_sharp", math.Abs(ml[0]-mr[0]) > 40,
			fmt.Sprintf("left=%.1f right=%.1f", ml[0], mr[0]))
	}

	// D: the published GPU view drawn live into the snapshot.
	{
		srcBase, _ := point(3, 5, 5)
		resBase, _ := point(3, halfX+5, 5)
		srcBand, _ := point(3, 40, 65)
		resBand, _ := point(3, halfX+40, 65)
		srcStrip, _ := point(3, 110, 110)
		resStrip, _ := point(3, halfX+110, 110)
		resBelow, _ := point(3, halfX+90, 110)
		edge, _ := point(3, halfX+40, 40)
		add("px_d_corner_base", near3(resBase, srcBase, 2.0/255),
			fmt.Sprintf("src=(%.3f,%.3f,%.3f) res=(%.3f,%.3f,%.3f)", srcBase[0], srcBase[1], srcBase[2], resBase[0], resBase[1], resBase[2]))
		add("px_d_band_center", near3(resBand, srcBand, 2.0/255),
			fmt.Sprintf("src=%.3f res=%.3f", srcBand[0], resBand[0]))
		add("px_d_stripe_kept", near3(resStrip, srcStrip, 2.0/255) && near3(resBelow, srcBase, 3.0/255),
			fmt.Sprintf("stripe=%.3f src=%.3f below=%.3f", resStrip[0], srcStrip[0], resBelow[0]))
		lo, hi := math.Min(srcBase[0], srcBand[0]), math.Max(srcBase[0], srcBand[0])
		add("px_d_edge_soft", edge[0] > lo+8.0/255 && edge[0] < hi-8.0/255,
			fmt.Sprintf("edge=%.3f between %.3f..%.3f", edge[0], lo, hi))

		cornerRegion := snapshotRect(bodyX, bodyY, dpr, 3, halfX, 0, 40, 20)
		spread, n := regionSpread(snap, cornerRegion)
		add("rg_d_corner_uniform", spread <= 2 && n > 0, fmt.Sprintf("spread=%d over %d px", spread, n))

		resRegion := snapshotRect(bodyX, bodyY, dpr, 3, halfX, 0, halfW, topH)
		spread, n = regionSpread(snap, resRegion)
		add("rg_d_not_blank", spread > 40 && n > 0, fmt.Sprintf("spread=%d over %d px", spread, n))

		if filterDRefImg == nil {
			add("rg_d_matches_cpu_ref", false, "CPU reference image missing")
		} else {
			refB := filterDRefImg.Bounds()
			maxd, sum, count := 0, 0.0, 0
			for dy := 0; dy < resRegion.Dy(); dy++ {
				for dx := 0; dx < resRegion.Dx(); dx++ {
					rx := refB.Min.X + dx*refB.Dx()/resRegion.Dx()
					ry := refB.Min.Y + dy*refB.Dy()/resRegion.Dy()
					sr, sg, sb, _ := snap.At(resRegion.Min.X+dx, resRegion.Min.Y+dy).RGBA()
					rr, rg, rb, _ := filterDRefImg.At(rx, ry).RGBA()
					for _, d := range []int{int(sr>>8) - int(rr>>8), int(sg>>8) - int(rg>>8), int(sb>>8) - int(rb>>8)} {
						if d < 0 {
							d = -d
						}
						sum += float64(d)
						count++
						if d > maxd {
							maxd = d
						}
					}
				}
			}
			mean := sum / math.Max(1, float64(count))
			add("rg_d_matches_cpu_ref", maxd <= 6 && mean <= 1.5,
				fmt.Sprintf("max|Δ|=%d mean=%.3f over %d samples", maxd, mean, count))
		}
	}

	// Layout sanity: the gap between the two panels is the card background.
	{
		gap, ok := point(0, 203, 92)
		want := [3]float64{colCardBG[0], colCardBG[1], colCardBG[2]}
		add("px_card_gap", ok && near3(gap, want, 3.0/255),
			fmt.Sprintf("got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f)", gap[0], gap[1], gap[2], want[0], want[1], want[2]))
	}

	return out
}

func regionGraySpread(img image.Image, r image.Rectangle) (int, int) {
	if img == nil {
		return 1 << 30, 0
	}
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return 1 << 30, 0
	}
	maxSpread := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			v := [3]int{int(cr >> 8), int(cg >> 8), int(cb >> 8)}
			s := v[0]
			if v[1] > s {
				s = v[1]
			}
			if v[2] > s {
				s = v[2]
			}
			m := v[0]
			if v[1] < m {
				m = v[1]
			}
			if v[2] < m {
				m = v[2]
			}
			if s-m > maxSpread {
				maxSpread = s - m
			}
		}
	}
	return maxSpread, r.Dx() * r.Dy()
}

func columnUnique(img image.Image, r image.Rectangle) int {
	if img == nil {
		return 0
	}
	r = r.Intersect(img.Bounds())
	seen := map[int]struct{}{}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		cr, _, _, _ := img.At(r.Min.X, y).RGBA()
		seen[int(cr>>8)] = struct{}{}
	}
	return len(seen)
}
