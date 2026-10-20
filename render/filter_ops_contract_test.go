package render_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
)

func filterPixel(img image.Image, x, y int) (r, g, b, a uint8) {
	cr, cg, cb, ca := img.At(x, y).RGBA()
	return uint8(cr >> 8), uint8(cg >> 8), uint8(cb >> 8), uint8(ca >> 8)
}

// filterRawPixel reads the live pixmap bytes. Pixmap storage is premultiplied
// RGBA in every backend, so 0/partial/255 alpha semantics are asserted here
// instead of through image.RGBA's premultiplied At() values.
func filterRawPixel(ctx *render.Context, x, y int) (r, g, b, a uint8) {
	pm := ctx.PixmapForTest()
	data := pm.Data()
	i := (y*pm.Width() + x) * 4
	return data[i], data[i+1], data[i+2], data[i+3]
}

func filterMaxDiff(a, b image.Image) int {
	ab, bb := a.Bounds(), b.Bounds()
	w, h := ab.Dx(), bb.Dx()
	if ab.Dy() < h {
		h = ab.Dy()
	}
	if bb.Dy() < h {
		h = bb.Dy()
	}
	max := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ar, ag, abl, aa := a.At(ab.Min.X+x, ab.Min.Y+y).RGBA()
			br, bg, bbl, ba := b.At(bb.Min.X+x, bb.Min.Y+y).RGBA()
			for _, d := range []int{
				int(ar>>8) - int(br>>8), int(ag>>8) - int(bg>>8),
				int(abl>>8) - int(bbl>>8), int(aa>>8) - int(ba>>8),
			} {
				if d < 0 {
					d = -d
				}
				if d > max {
					max = d
				}
			}
		}
	}
	return max
}

func filterAbsDiff(a, b int) int {
	if a < b {
		return b - a
	}
	return a - b
}

// TestFilterOpsInvertContract pins ApplyInvert to the color-matrix contract:
// the matrix runs on straight-alpha colors (Skia/Flutter color-filter
// semantics), results are stored premultiplied, and alpha is preserved.
// The GPU filter graph is disabled here so this test always exercises the CPU
// implementation; the GPU graph is pinned separately in filter_ops_gpu_test.go.
// warmFilterOpsGPU forces the lazy GPU/filter-graph registration before a test
// disables the hooks. Disabling first would let the first Image/Flush install
// the hooks again, and the restore function would then put back nil, leaving
// the package's later GPU filter tests unable to register.
func warmFilterOpsGPU(t *testing.T) {
	t.Helper()
	dc := render.NewContext(8, 8)
	dc.ClearWithColor(render.RGBA{R: 1, A: 1})
	dc.SetRGB(0, 1, 0)
	dc.DrawRectangle(0, 0, 8, 8)
	_ = dc.Fill()
	_ = dc.Image()
	_ = dc.Close()
}

func TestFilterOpsInvertContract(t *testing.T) {
	warmFilterOpsGPU(t)
	restore := render.DisableGPUFilterGraphForTest()
	defer restore()
	if !render.FiltersRegistered() {
		t.Fatal("filters not registered")
	}

	// Opaque pixels: premultiplied invert is exactly 255-v.
	dc := render.NewContext(16, 16)
	defer dc.Close()
	dc.ClearWithColor(render.RGBA{R: 1, G: 0.2, B: 0.1, A: 1})
	sr, sg, sb, sa := filterRawPixel(dc, 8, 8)
	if sa != 255 {
		t.Fatalf("source alpha = %d, want 255", sa)
	}
	dc.ApplyInvert()
	r, g, b, a := filterPixel(dc.Image(), 8, 8)
	if filterAbsDiff(int(r), 255-int(sr)) > 1 ||
		filterAbsDiff(int(g), 255-int(sg)) > 1 ||
		filterAbsDiff(int(b), 255-int(sb)) > 1 ||
		a != 255 {
		t.Fatalf("invert opaque src=%d,%d,%d -> %d,%d,%d,%d; want 255-v with alpha 255",
			sr, sg, sb, r, g, b, a)
	}

	// Semi-transparent pixels keep alpha and invert in straight-alpha space:
	//   straight' = 255 - straight, premul' = alpha - premul.
	alpha := render.NewContext(8, 8)
	defer alpha.Close()
	alpha.ClearWithColor(render.RGBA{R: 1, A: 0.5})
	sr, sg, sb, sa = filterRawPixel(alpha, 4, 4)
	if sa < 126 || sa > 129 {
		t.Fatalf("semi-transparent source alpha = %d, want ~128", sa)
	}
	if filterAbsDiff(int(sr), int(sa)) > 1 || sg > 1 || sb > 1 {
		t.Fatalf("semi-transparent source not premultiplied red: %d,%d,%d,%d", sr, sg, sb, sa)
	}
	alpha.ApplyInvert()
	ar, ag, ab, aa := filterPixel(alpha.Image(), 4, 4)
	wantR, wantG, wantB := int(sa)-int(sr), int(sa)-int(sg), int(sa)-int(sb)
	if filterAbsDiff(int(aa), int(sa)) > 1 {
		t.Fatalf("invert changed alpha: src=%d out=%d", sa, aa)
	}
	if filterAbsDiff(int(ar), wantR) > 1 || filterAbsDiff(int(ag), wantG) > 1 ||
		filterAbsDiff(int(ab), wantB) > 1 {
		t.Fatalf("invert semi-transparent src=%d,%d,%d,%d -> %d,%d,%d,%d; want premul %d,%d,%d,%d",
			sr, sg, sb, sa, ar, ag, ab, aa, wantR, wantG, wantB, sa)
	}
	// Premultiplied storage invariant: no channel may exceed alpha.
	if ar > aa || ag > aa || ab > aa {
		t.Fatalf("invert produced invalid premultiplied pixel: %d,%d,%d,%d", ar, ag, ab, aa)
	}

	// Fully transparent pixels stay transparent black (no hidden RGB).
	clear := render.NewContext(4, 4)
	defer clear.Close()
	clear.Clear()
	clear.ApplyInvert()
	cr, cg, cb, ca := filterPixel(clear.Image(), 2, 2)
	if cr != 0 || cg != 0 || cb != 0 || ca != 0 {
		t.Fatalf("invert transparent -> %d,%d,%d,%d, want 0,0,0,0", cr, cg, cb, ca)
	}
}

// TestFilterOpsBlurXYAnisotropicContract pins radiusX=0 to no horizontal
// spread while radiusY=3 spreads vertically. The CPU path is forced so the
// result is independent of the GPU filter backend.
//
// Geometry: an 80x40 canvas with a white 20x12 bar at (30,14)-(50,26).
// radiusY=3 is a normalized Gaussian with sigma=3 and half-kernel 9, so the
// bar is visible ~9px above/below and completely absent beyond that; radiusX=0
// leaves the columns next to the bar untouched.
func TestFilterOpsBlurXYAnisotropicContract(t *testing.T) {
	warmFilterOpsGPU(t)
	restore := render.DisableGPUFilterGraphForTest()
	defer restore()
	if !render.FiltersRegistered() {
		t.Fatal("filters not registered")
	}
	const w, h = 80, 40
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ClearWithColor(render.RGBA{A: 1})
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(30, 14, 20, 12)
	_ = dc.Fill()
	// Capture the source before blur: the rectangle edge has AA coverage in
	// column 29, so "no horizontal spread" is asserted against these values
	// rather than against zero.
	before := dc.Image()
	dc.ApplyBlurXY(0, 3)
	img := dc.Image()

	if r, _, _, _ := filterPixel(img, 40, 20); r < 200 {
		t.Fatalf("blur center r=%d, want bright bar", r)
	}
	// radiusX=0: columns two or more pixels outside the bar stay background,
	// and the AA edge column keeps its source value (vertical blur only).
	for _, x := range []int{20, 28} {
		if r, _, _, _ := filterPixel(img, x, 20); r != 0 {
			t.Fatalf("blur spread horizontally with radiusX=0 at x=%d: r=%d", x, r)
		}
	}
	preEdge, _, _, _ := filterPixel(before, 29, 20)
	postEdge, _, _, _ := filterPixel(img, 29, 20)
	if filterAbsDiff(int(postEdge), int(preEdge)) > 2 {
		t.Fatalf("radiusX=0 changed AA edge column: pre=%d post=%d", preEdge, postEdge)
	}
	// radiusY=3: 4px from the bar edge the Gaussian tail is clearly visible.
	if r, _, _, _ := filterPixel(img, 40, 10); r < 15 {
		t.Fatalf("blur did not spread vertically above bar: r=%d", r)
	}
	if r, _, _, _ := filterPixel(img, 40, 29); r < 15 {
		t.Fatalf("blur did not spread vertically below bar: r=%d", r)
	}
	// Beyond the 3-sigma support (9px) the result is exactly background.
	if r, _, _, _ := filterPixel(img, 40, 2); r != 0 {
		t.Fatalf("blur leaked beyond kernel support above: r=%d", r)
	}
	if r, _, _, _ := filterPixel(img, 40, 38); r != 0 {
		t.Fatalf("blur leaked beyond kernel support below: r=%d", r)
	}
	// The vertical tail must decay monotonically away from the bar.
	above4, _, _, _ := filterPixel(img, 40, 10)
	above8, _, _, _ := filterPixel(img, 40, 6)
	if above4 <= above8 {
		t.Fatalf("vertical blur tail not decaying: y=10 r=%d <= y=6 r=%d", above4, above8)
	}

	// Zero radius is identity.
	base := render.NewContext(w, h)
	defer base.Close()
	base.ClearWithColor(render.RGBA{A: 1})
	base.SetRGB(1, 1, 1)
	base.DrawRectangle(30, 14, 20, 12)
	_ = base.Fill()
	baseBefore := base.Image()
	base.ApplyBlurXY(0, 0)
	if d := filterMaxDiff(baseBefore, base.Image()); d != 0 {
		t.Fatalf("ApplyBlurXY(0,0) changed pixels: max diff %d", d)
	}
}

// TestFilterOpsGraphOrderAndSequential pins node order and the graph's
// ping-pong equivalence to sequential Apply* calls on the CPU path.
func TestFilterOpsGraphOrderAndSequential(t *testing.T) {
	warmFilterOpsGPU(t)
	restore := render.DisableGPUFilterGraphForTest()
	defer restore()
	if !render.FiltersRegistered() {
		t.Fatal("filters not registered")
	}
	scaleR := [20]float32{
		0.5, 0, 0, 0, 0,
		0, 1, 0, 0, 0,
		0, 0, 1, 0, 0,
		0, 0, 0, 1, 0,
	}
	mk := func(nodes ...render.ImageFilterNode) image.Image {
		dc := render.NewContext(24, 24)
		defer dc.Close()
		dc.ClearWithColor(render.RGBA{R: 1, A: 1})
		dc.ApplyImageFilterGraph(nodes...)
		return dc.Image()
	}
	// scaleR then invert: R=0.5 -> inverted 0.5 (128). Reverse order: invert
	// first gives R=0, then scale keeps 0.
	a := mk(
		render.ImageFilterNode{Kind: render.ImageFilterColorMatrix, Matrix: scaleR},
		render.ImageFilterNode{Kind: render.ImageFilterInvert},
	)
	b := mk(
		render.ImageFilterNode{Kind: render.ImageFilterInvert},
		render.ImageFilterNode{Kind: render.ImageFilterColorMatrix, Matrix: scaleR},
	)
	ar, ag, ab, _ := filterPixel(a, 12, 12)
	br, bg, bb, _ := filterPixel(b, 12, 12)
	if ar < 124 || ar > 130 || ag < 250 || ab < 250 {
		t.Fatalf("scaleR->invert = %d,%d,%d, want ~128,255,255", ar, ag, ab)
	}
	if br > 2 || bg < 250 || bb < 250 {
		t.Fatalf("invert->scaleR = %d,%d,%d, want 0,255,255", br, bg, bb)
	}

	// A multi-node graph must equal the same operations applied sequentially.
	graph := render.NewContext(64, 48)
	defer graph.Close()
	graph.ClearWithColor(render.RGBA{R: 0.1, G: 0.2, B: 0.4, A: 1})
	graph.SetRGB(1, 0.3, 0.1)
	graph.DrawRectangle(8, 8, 30, 24)
	_ = graph.Fill()
	graph.ApplyImageFilterGraph(
		render.ImageFilterNode{Kind: render.ImageFilterBlurXY, RadiusX: 1.5, RadiusY: 1.5},
		render.ImageFilterNode{Kind: render.ImageFilterGrayscale},
		render.ImageFilterNode{Kind: render.ImageFilterInvert},
	)

	seq := render.NewContext(64, 48)
	defer seq.Close()
	seq.ClearWithColor(render.RGBA{R: 0.1, G: 0.2, B: 0.4, A: 1})
	seq.SetRGB(1, 0.3, 0.1)
	seq.DrawRectangle(8, 8, 30, 24)
	_ = seq.Fill()
	seq.ApplyBlurXY(1.5, 1.5)
	seq.ApplyGrayscale()
	seq.ApplyInvert()

	if d := filterMaxDiff(graph.Image(), seq.Image()); d > 2 {
		t.Fatalf("graph vs sequential max channel diff = %d, want <=2", d)
	}
}
