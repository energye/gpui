// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

package raster

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// tinySkiaColor holds premultiplied RGBA color matching tiny-skia's
// set_color_rgba8(50, 127, 150, 200) after premultiplication.
//
// Straight alpha: R=50, G=127, B=150, A=200
// Premultiplied:  R=50*200/255≈39, G=127*200/255≈99, B=150*200/255≈117, A=200
type tinySkiaColor struct {
	R, G, B, A uint8
}

// premultipliedColor returns the premultiplied RGBA for the tiny-skia test paint.
// tiny-skia set_color_rgba8(50, 127, 150, 200) stores straight alpha internally
// and premultiplies when rasterizing. The golden PNG stores premultiplied values.
// Uses tiny-skia's div255: (a*b + 128) / 255 (NOT truncation).
func premultipliedColor() tinySkiaColor {
	return tinySkiaColor{
		R: div255(50, 200),  // 39
		G: div255(127, 200), // 100
		B: div255(150, 200), // 118
		A: 200,
	}
}

func div255(a, b uint16) uint8 {
	return uint8((a*b + 128) / 255)
}

// renderWithAnalyticFillerOnWhite rasterizes a path using AnalyticFiller and
// composites the result onto a WHITE background using source-over blending.
// All output pixels have A=255, making the image lossless through PNG round-trip
// (no un-premultiply/re-premultiply precision loss).
//
// This matches Skia Fiddle golden generation with canvas->clear(SK_ColorWHITE).
//
// Source-over compositing uses Skia's exact formula (SkAlphaMulQ):
//
//	scale = cov + 1                          (SkAlpha255To256)
//	srcR = (paintR * scale) >> 8             (SkAlphaMulQ)
//	srcA = (paintA * scale) >> 8
//	invScale = (255 - srcA) + 1              (SkAlpha255To256)
//	dstR = srcR + (255 * invScale) >> 8      (source-over on white)
//	dstA = 255
func renderWithAnalyticFillerOnWhite(
	width, height int,
	path PathLike,
	fillRule FillRule,
	paint tinySkiaColor,
	aaShift int,
) *image.RGBA {
	eb := NewEdgeBuilder(aaShift)
	eb.SetFlattenCurves(true)
	eb.BuildFromPath(path, IdentityTransform{})

	coverageBuf := make([]uint8, width*height)
	FillToBuffer(eb, width, height, fillRule, coverageBuf)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cov := uint32(coverageBuf[y*width+x])
			if cov == 0 {
				img.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
				continue
			}
			// Skia's SkAlphaMulQ: scale = cov + 1, result = (ch * scale) >> 8
			scale := cov + 1
			srcR := (uint32(paint.R) * scale) >> 8
			srcG := (uint32(paint.G) * scale) >> 8
			srcB := (uint32(paint.B) * scale) >> 8
			srcA := (uint32(paint.A) * scale) >> 8

			invScale := (255 - srcA) + 1
			r := uint8(srcR + (255*invScale)>>8)
			g := uint8(srcG + (255*invScale)>>8)
			b := uint8(srcB + (255*invScale)>>8)
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

// loadGoldenPNG loads a golden reference PNG from testdata/golden/.
// Returns nil and calls t.Fatal if the file cannot be loaded.
func loadGoldenPNG(t *testing.T, name string) *image.RGBA {
	t.Helper()

	path := filepath.Join(testdataGoldenDir(), name)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open golden image %s: %v", path, err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("failed to decode golden image %s: %v", path, err)
	}

	// If already RGBA, use directly (preserves raw premultiplied bytes).
	// Do NOT use rgba.Set(x,y, img.At(x,y)) — the color.Color interface
	// round-trips through un-premultiply/re-premultiply, losing precision.
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	if nrgba, ok := img.(*image.NRGBA); ok {
		bounds := nrgba.Bounds()
		rgba := image.NewRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := nrgba.NRGBAAt(x, y)
				a16 := uint16(c.A)
				rgba.SetRGBA(x, y, color.RGBA{
					R: div255(uint16(c.R), a16),
					G: div255(uint16(c.G), a16),
					B: div255(uint16(c.B), a16),
					A: c.A,
				})
			}
		}
		return rgba
	}
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}
	return rgba
}

// goldenCompareResult holds the results of a pixel-by-pixel image comparison.
type goldenCompareResult struct {
	TotalPixels int         // Total number of pixels compared
	DiffCount   int         // Number of pixels that differ
	MaxDiff     int         // Maximum per-channel difference across all pixels
	DiffPct     float64     // Percentage of differing pixels
	DiffMap     *image.RGBA // Visual diff map (green=match, red=mismatch)
}

// compareImages performs pixel-by-pixel comparison of two RGBA images.
// Returns the comparison result including a visual diff map.
//
// Diff map encoding:
//   - Green channel: match confidence (255 = exact match)
//   - Red channel: mismatch magnitude (brighter = bigger difference)
//   - Alpha: 255 for any pixel where either image has content
func compareImages(got, want *image.RGBA) goldenCompareResult {
	bounds := got.Bounds()
	wantBounds := want.Bounds()

	// Use intersection of bounds for comparison
	w := bounds.Dx()
	h := bounds.Dy()
	if wantBounds.Dx() < w {
		w = wantBounds.Dx()
	}
	if wantBounds.Dy() < h {
		h = wantBounds.Dy()
	}

	result := goldenCompareResult{
		TotalPixels: w * h,
		DiffMap:     image.NewRGBA(image.Rect(0, 0, w, h)),
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			gc := got.RGBAAt(x+bounds.Min.X, y+bounds.Min.Y)
			wc := want.RGBAAt(x+wantBounds.Min.X, y+wantBounds.Min.Y)

			gr8 := gc.R
			gg8 := gc.G
			gb8 := gc.B
			ga8 := gc.A
			wr8 := wc.R
			wg8 := wc.G
			wb8 := wc.B
			wa8 := wc.A

			// Per-channel absolute differences
			dR := absDiffU8(gr8, wr8)
			dG := absDiffU8(gg8, wg8)
			dB := absDiffU8(gb8, wb8)
			dA := absDiffU8(ga8, wa8)

			maxChanDiff := maxU8(maxU8(dR, dG), maxU8(dB, dA))

			if maxChanDiff == 0 {
				// Exact match — show green if either image has content
				if ga8 > 0 || wa8 > 0 {
					result.DiffMap.SetRGBA(x, y, color.RGBA{R: 0, G: 128, B: 0, A: 255})
				}
				continue
			}

			result.DiffCount++
			if int(maxChanDiff) > result.MaxDiff {
				result.MaxDiff = int(maxChanDiff)
			}
			// Red channel = mismatch magnitude (scaled to be visible)
			diffVis := maxChanDiff
			if diffVis < 32 {
				diffVis = 32 // minimum visibility for small diffs
			}
			result.DiffMap.SetRGBA(x, y, color.RGBA{R: diffVis, G: 0, B: 0, A: 255})
		}
	}

	if result.TotalPixels > 0 {
		result.DiffPct = float64(result.DiffCount) / float64(result.TotalPixels) * 100.0
	}
	return result
}

// saveDiffMap writes a diff map image to the tmp/ directory for visual inspection.
func saveDiffMap(t *testing.T, img *image.RGBA, name string) {
	t.Helper()

	dir := filepath.Join(projectRoot(), "tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("warning: cannot create tmp dir: %v", err)
		return
	}

	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Logf("warning: cannot create diff image %s: %v", path, err)
		return
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Logf("warning: cannot encode diff image: %v", err)
		return
	}
	t.Logf("diff map saved: %s", path)
}

// saveRendered writes a rendered image to the tmp/ directory for visual inspection.
func saveRendered(t *testing.T, img *image.RGBA, name string) {
	t.Helper()

	dir := filepath.Join(projectRoot(), "tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("warning: cannot create tmp dir: %v", err)
		return
	}

	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Logf("warning: cannot create rendered image %s: %v", path, err)
		return
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Logf("warning: cannot encode rendered image: %v", err)
		return
	}
	t.Logf("rendered image saved: %s", path)
}

// logCompareResult reports comparison statistics using t.Logf (diagnostic, not assertions).
func logCompareResult(t *testing.T, testName string, result goldenCompareResult) {
	t.Helper()
	t.Logf("=== Golden comparison: %s ===", testName)
	t.Logf("  total pixels:    %d", result.TotalPixels)
	t.Logf("  differing pixels: %d (%.2f%%)", result.DiffCount, result.DiffPct)
	t.Logf("  max channel diff: %d", result.MaxDiff)
	if result.DiffCount == 0 {
		t.Logf("  result: EXACT MATCH")
	}
}

// --- Skia AAA Golden Comparison Tests ---
// These compare against golden images generated by Skia's AAA algorithm
// (fiddle.skia.org), which is the target algorithm we ported.

// TestAnalyticFiller_SkiaAAAPolygonGolden compares against Skia AAA output
// LEVEL 2: COMPOSITING — RGB image comparison (coverage + compositing combined).
// Primary rasterizer validation is in Level 1 coverage tests (CoverageVsCpp).
//
// Polygon compositing test (no AA, Winding fill, white background).
func TestCompositing_PolygonRGB(t *testing.T) {
	path := &testPath{
		verbs: []PathVerb{
			MoveTo,
			LineTo,
			LineTo,
			LineTo,
			LineTo,
		},
		points: []float32{
			75.160671, 88.756136,
			24.797274, 88.734053,
			9.255130, 40.828792,
			50.012955, 11.243795,
			90.744819, 40.864522,
		},
	}

	paint := premultipliedColor()
	got := renderWithAnalyticFillerOnWhite(100, 100, path, FillRuleNonZero, paint, 0)

	golden := loadGoldenPNG(t, "skia-aaa-polygon-white.png")

	result := compareImages(got, golden)
	logCompareResult(t, "skia-aaa-polygon (no AA, Winding, white bg)", result)

	saveRendered(t, got, "golden_rendered_skia_aaa_polygon.png")
	saveDiffMap(t, result.DiffMap, "golden_diff_skia_aaa_polygon.png")

	if result.DiffCount > 0 {
		t.Errorf("FAIL: polygon RGB diff=%d pixels (max=%d), want diff=0", result.DiffCount, result.MaxDiff)
	}
}

// LEVEL 2: COMPOSITING — float rect RGB image comparison.
func TestCompositing_FloatRectRGB(t *testing.T) {
	path := &testPath{
		verbs: []PathVerb{
			MoveTo,
			LineTo,
			LineTo,
			LineTo,
			Close,
		},
		points: []float32{
			10.3, 15.4,
			90.8, 15.4,
			90.8, 86.0,
			10.3, 86.0,
		},
	}

	paint := premultipliedColor()
	got := renderWithAnalyticFillerOnWhite(100, 100, path, FillRuleNonZero, paint, 2)

	golden := loadGoldenPNG(t, "skia-aaa-float-rect-aa-white.png")

	result := compareImages(got, golden)
	logCompareResult(t, "skia-aaa-float-rect-aa (AA, Winding, white bg)", result)

	saveRendered(t, got, "golden_rendered_skia_aaa_float_rect.png")
	saveDiffMap(t, result.DiffMap, "golden_diff_skia_aaa_float_rect.png")

	// Golden generated from C++ Skia-exact tool (verbatim Skia walker + same compositing).
	if result.DiffCount > 0 {
		t.Errorf("REGRESSION: float rect diff=%d pixels (max=%d), want diff=0", result.DiffCount, result.MaxDiff)
	}
}

// LEVEL 2: COMPOSITING — star RGB image comparison.
func TestCompositing_StarRGB(t *testing.T) {
	path := &testPath{
		verbs: []PathVerb{
			MoveTo,
			LineTo,
			LineTo,
			LineTo,
			LineTo,
			Close,
		},
		points: []float32{
			50.0, 7.5,
			75.0, 87.5,
			10.0, 37.5,
			90.0, 37.5,
			25.0, 87.5,
		},
	}

	paint := premultipliedColor()
	// Skia AAA star golden uses Winding fill, not EvenOdd
	got := renderWithAnalyticFillerOnWhite(100, 100, path, FillRuleNonZero, paint, 2)

	golden := loadGoldenPNG(t, "skia-aaa-star-aa-white.png")

	result := compareImages(got, golden)
	logCompareResult(t, "skia-aaa-star-aa (AA, Winding, white bg)", result)

	saveRendered(t, got, "golden_rendered_skia_aaa_star.png")
	saveDiffMap(t, result.DiffMap, "golden_diff_skia_aaa_star.png")

	// Golden generated from C++ Skia-exact tool (verbatim Skia walker + same compositing).
	// Coverage and compositing both verified: must be pixel-perfect.
	if result.DiffCount > 0 {
		t.Errorf("REGRESSION: star diff=%d pixels (max=%d), want diff=0", result.DiffCount, result.MaxDiff)
	}
}

// --- Utility functions ---

func absDiffU8(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

func maxU8(a, b uint8) uint8 {
	if a > b {
		return a
	}
	return b
}

// thisFileDir returns the directory containing this test file via runtime.Caller.
func thisFileDir() string {
	//nolint:dogsled // runtime.Caller returns 4 values; we only need the filename
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filename)
}

// testdataGoldenDir returns the path to testdata/golden/ relative to this test file.
func testdataGoldenDir() string {
	return filepath.Join(thisFileDir(), "testdata", "golden")
}

// projectRoot returns the project root (gg/) by walking up from this test file.
func projectRoot() string {
	// This file is at internal/raster/analytic_filler_golden_test.go
	// Project root is 2 levels up.
	return filepath.Join(thisFileDir(), "..", "..")
}
func TestAnalyticFiller_TraceBlitAaaY39(t *testing.T) {
	// Exact SkFixed values from C++ full_walk trace at y=39:
	// blit_aaa: ul=783154 ur=917504 ll=868350 lr=917504
	ul := int32(783154)
	ur := int32(917504)
	ll := int32(868350)
	lr := int32(917504)

	af := NewAnalyticFiller(100, 100)
	for i := range af.coverage {
		af.coverage[i] = 0
	}

	af.blitAaaTrapezoidRow(ul, ur, ll, lr, 50412, 0x7FFFFFFF, 255)

	t.Logf("pixel 11: alpha=%d (C++=0)", af.coverage[11])
	t.Logf("pixel 12: alpha=%d (C++=108)", af.coverage[12])
	t.Logf("pixel 13: alpha=%d (C++=249)", af.coverage[13])

	if af.coverage[12] != 108 {
		t.Errorf("pixel 12: got=%d want=108 (C++ reference)", af.coverage[12])
	}
}

// LEVEL 1: RASTERIZER — coverage buffer comparison vs C++ Skia-exact.
// These are the PRIMARY validation tests. Coverage is the rasterizer output.
// C++ tool uses verbatim Skia source (SkScan_AAAPath.cpp aaa_walk_edges).
// Must be diff=0 (pixel-perfect).

// TestCoverage_Star compares star coverage byte-for-byte vs C++ Skia-exact.
func TestCoverage_Star(t *testing.T) {
	cppFile := filepath.Join(projectRoot(), "tmp", "skia_coverage", "star_coverage_skia_exact.bin")
	cppData, err := os.ReadFile(cppFile)
	if err != nil {
		t.Skipf("C++ coverage not found: %v (run full_walk.exe first)", err)
	}
	if len(cppData) != 10000 {
		t.Fatalf("unexpected C++ coverage size: %d, want 10000", len(cppData))
	}

	path := &testPath{
		verbs:  []PathVerb{MoveTo, LineTo, LineTo, LineTo, LineTo, Close},
		points: []float32{50.0, 7.5, 75.0, 87.5, 10.0, 37.5, 90.0, 37.5, 25.0, 87.5},
	}
	eb := NewEdgeBuilder(2)
	eb.SetFlattenCurves(true)
	eb.BuildFromPath(path, IdentityTransform{})
	goBuf := make([]uint8, 100*100)
	FillToBuffer(eb, 100, 100, FillRuleNonZero, goBuf)

	diffCount := 0
	maxDiff := 0
	type covDiff struct {
		x, y    int
		goCov   int
		cppCov  int
		absDiff int
	}
	var diffs []covDiff

	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			g := int(goBuf[y*100+x])
			c := int(cppData[y*100+x])
			d := g - c
			if d < 0 {
				d = -d
			}
			if d > 0 {
				diffCount++
				if d > maxDiff {
					maxDiff = d
				}
				diffs = append(diffs, covDiff{x, y, g, c, d})
			}
		}
	}

	t.Logf("Coverage comparison Go vs C++: %d diff pixels, max diff=%d", diffCount, maxDiff)

	shown := 0
	for _, d := range diffs {
		if shown >= 50 {
			break
		}
		t.Logf("  (%2d,%2d): go=%3d cpp=%3d diff=%+d", d.x, d.y, d.goCov, d.cppCov, d.goCov-d.cppCov)
		shown++
	}
	if len(diffs) > 50 {
		t.Logf("  ... and %d more", len(diffs)-50)
	}

	// Coverage must be pixel-perfect vs Skia-exact C++ walker.
	// C++ tool uses verbatim Skia source (SkScan_AAAPath.cpp aaa_walk_edges).
	if diffCount > 0 {
		t.Errorf("REGRESSION: coverage diff=%d (max=%d), want diff=0 (pixel-perfect)", diffCount, maxDiff)
	}
}

// TestCoverage_FloatRect compares float rect coverage byte-for-byte vs C++ Skia-exact.
func TestCoverage_FloatRect(t *testing.T) {
	cppFile := filepath.Join(projectRoot(), "tmp", "skia_coverage", "rect_coverage_skia_exact.bin")
	cppData, err := os.ReadFile(cppFile)
	if err != nil {
		t.Skipf("C++ rect coverage not found: %v (run skia_exact.exe first)", err)
	}
	if len(cppData) != 10000 {
		t.Fatalf("unexpected C++ coverage size: %d, want 10000", len(cppData))
	}

	path := &testPath{
		verbs:  []PathVerb{MoveTo, LineTo, LineTo, LineTo, Close},
		points: []float32{10.3, 15.4, 90.8, 15.4, 90.8, 86.0, 10.3, 86.0},
	}
	eb := NewEdgeBuilder(2)
	eb.SetFlattenCurves(true)
	eb.BuildFromPath(path, IdentityTransform{})
	goBuf := make([]uint8, 100*100)
	FillToBuffer(eb, 100, 100, FillRuleNonZero, goBuf)

	diffCount := 0
	maxDiff := 0
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			g := int(goBuf[y*100+x])
			c := int(cppData[y*100+x])
			d := g - c
			if d < 0 {
				d = -d
			}
			if d > 0 {
				diffCount++
				if d > maxDiff {
					maxDiff = d
				}
				if diffCount <= 10 {
					t.Logf("  (%2d,%2d): go=%3d cpp=%3d diff=%+d", x, y, g, c, g-c)
				}
			}
		}
	}

	t.Logf("Rect coverage comparison Go vs C++: %d diff pixels, max diff=%d", diffCount, maxDiff)

	if diffCount > 0 {
		t.Errorf("REGRESSION: rect coverage diff=%d (max=%d), want diff=0", diffCount, maxDiff)
	}
}

// TestCoverage_Polygon compares polygon coverage byte-for-byte vs C++ Skia-exact.
func TestCoverage_Polygon(t *testing.T) {
	cppFile := filepath.Join(projectRoot(), "tmp", "skia_coverage", "polygon_coverage_skia_exact.bin")
	cppData, err := os.ReadFile(cppFile)
	if err != nil {
		t.Skipf("C++ polygon coverage not found: %v (run skia_exact.exe first)", err)
	}
	if len(cppData) != 10000 {
		t.Fatalf("unexpected C++ coverage size: %d, want 10000", len(cppData))
	}

	path := &testPath{
		verbs:  []PathVerb{MoveTo, LineTo, LineTo, LineTo, LineTo},
		points: []float32{75.160671, 88.756136, 24.797274, 88.734053, 9.255130, 40.828792, 50.012955, 11.243795, 90.744819, 40.864522},
	}
	eb := NewEdgeBuilder(0) // aaShift=0, no AA
	eb.SetFlattenCurves(true)
	eb.BuildFromPath(path, IdentityTransform{})
	goBuf := make([]uint8, 100*100)
	FillToBuffer(eb, 100, 100, FillRuleNonZero, goBuf)

	diffCount := 0
	maxDiff := 0
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			g := int(goBuf[y*100+x])
			c := int(cppData[y*100+x])
			d := g - c
			if d < 0 {
				d = -d
			}
			if d > 0 {
				diffCount++
				if d > maxDiff {
					maxDiff = d
				}
				if diffCount <= 10 {
					t.Logf("  (%2d,%2d): go=%3d cpp=%3d diff=%+d", x, y, g, c, g-c)
				}
			}
		}
	}

	t.Logf("Polygon coverage comparison Go vs C++: %d diff pixels, max diff=%d", diffCount, maxDiff)

	if diffCount > 0 {
		for y := 0; y < 100; y++ {
			cnt := 0
			for x := 0; x < 100; x++ {
				if int(goBuf[y*100+x]) != int(cppData[y*100+x]) {
					cnt++
				}
			}
			if cnt > 0 {
				t.Logf("  y=%d: %d diff pixels", y, cnt)
			}
		}
		t.Errorf("REGRESSION: polygon coverage diff=%d (max=%d), want diff=0 (pixel-perfect)", diffCount, maxDiff)
	}
}
