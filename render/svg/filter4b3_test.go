//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"image"
	"image/color"
	"testing"

	_ "github.com/energye/gpui/render/filters"
)

// P3.8 批 4b-3（feTurbulence + feConvolveMatrix 新算法）钉测。resvg
// turbulence.rs / convolve_matrix.rs 逐字直译的验收：语料样本入
// testdata/p38/filter4b/（feTurbulence- / feConvolveMatrix- 目录前缀），
// 感知分级 ≤ close + 关键像素逻辑探针 + 负用例（usvg dummy 语义 → 透明
// 输出：divisor=0 / target 越界 / kernel 数量不配 / numOctaves<0）。

// anyNonZeroAlpha reports whether any sampled pixel in rect has alpha > 0.
func anyNonZeroAlpha(t *testing.T, img image.Image, x0, y0, x1, y1 int) bool {
	t.Helper()
	for y := y0; y < y1; y += 3 {
		for x := x0; x < x1; x += 3 {
			if _, _, _, a := img.At(x, y).RGBA(); a>>8 > 0 {
				return true
			}
		}
	}
	return false
}

// flatColor reports whether the sampled pixels all share one color
// (baseFrequency=0 makes the noise constant — resvg noise2 at the origin).
func flatColor(t *testing.T, img image.Image, pts [][2]int) color.RGBA {
	t.Helper()
	first, ok := img.At(pts[0][0], pts[0][1]).(color.RGBA)
	if !ok {
		r, g, b, a := img.At(pts[0][0], pts[0][1]).RGBA()
		first = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
	}
	for _, p := range pts[1:] {
		r, g, b, a := img.At(p[0], p[1]).RGBA()
		if uint8(r>>8) != first.R || uint8(g>>8) != first.G || uint8(b>>8) != first.B || uint8(a>>8) != first.A {
			t.Fatalf("image not flat: %v -> %v vs %v", p, color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}, first)
		}
	}
	return first
}

func TestP384b3TurbulenceParse(t *testing.T) {
	doc := mustParseFile(t, "filter4b/feTurbulence-no-attributes.svg")
	p := filterOf(t, doc, "filter1").Primitives[0]
	if p.Kind != "feTurbulence" {
		t.Fatalf("kind = %s", p.Kind)
	}
	// Defaults (usvg): baseFrequency 0/0, numOctaves 1, seed 0, turbulence.
	tp := turbulenceParams(p, 0, 0, true)
	if tp.BaseFrequencyX != 0 || tp.BaseFrequencyY != 0 || tp.NumOctaves != 1 || tp.Seed != 0 || tp.FractalNoise {
		t.Fatalf("defaults = %+v", tp)
	}

	doc2 := mustParseFile(t, "filter4b/feTurbulence-baseFrequency=0.05-0.01.svg")
	p2 := filterOf(t, doc2, "filter1").Primitives[0]
	if len(p2.BaseFrequency) != 2 || p2.BaseFrequency[0] != 0.05 || p2.BaseFrequency[1] != 0.01 {
		t.Fatalf("baseFrequency list = %v", p2.BaseFrequency)
	}
	tp2 := turbulenceParams(p2, 0, 0, true)
	if tp2.BaseFrequencyX == 0 || tp2.BaseFrequencyY == 0 {
		t.Fatalf("two-value baseFrequency not applied: %+v", tp2)
	}

	// Negative baseFrequency → both 0 (usvg sign rule).
	doc3 := mustParseFile(t, "filter4b/feTurbulence-baseFrequency=0.05--0.01.svg")
	p3 := filterOf(t, doc3, "filter1").Primitives[0]
	tp3 := turbulenceParams(p3, 0, 0, true)
	if tp3.BaseFrequencyX != 0 || tp3.BaseFrequencyY != 0 {
		t.Fatalf("negative component should zero both: %+v", tp3)
	}

	// seed="1.5" truncates to 1 (usvg f32 trunc).
	doc4 := mustParseFile(t, "filter4b/feTurbulence-seed=1.5.svg")
	p4 := filterOf(t, doc4, "filter1").Primitives[0]
	if tp4 := turbulenceParams(p4, 0, 0, true); tp4.Seed != 1 {
		t.Fatalf("seed 1.5 → %d, want 1", tp4.Seed)
	}

	// type="invalid" → turbulence (exact-match semantics).
	doc5 := mustParseFile(t, "filter4b/feTurbulence-type=invalid.svg")
	p5 := filterOf(t, doc5, "filter1").Primitives[0]
	if tp5 := turbulenceParams(p5, 0, 0, true); tp5.FractalNoise {
		t.Fatal("type=invalid must stay turbulence")
	}
}

func TestP384b3TurbulenceNoAttributes(t *testing.T) {
	// All defaults: baseFrequency=0 → noise constant per channel; turbulence
	// mode → |n|·255 flat fill.
	doc := mustParseFile(t, "filter4b/feTurbulence-no-attributes.svg")
	img := doc.Render(200, 200)
	flatColor(t, img, [][2]int{{50, 50}, {150, 60}, {80, 140}, {170, 170}})
	gradeAgainstPNG4b2(t, doc, "feTurbulence-no-attributes")
}

func TestP384b3TurbulenceFractalNoise(t *testing.T) {
	// fractalNoise alpha = (n·255+255)/2 ∈ [127,255] — never transparent.
	doc := mustParseFile(t, "filter4b/feTurbulence-type=fractalNoise.svg")
	img := doc.Render(200, 200)
	for _, p := range [][2]int{{60, 60}, {100, 100}, {140, 140}, {50, 150}} {
		if a := img.RGBAAt(p[0], p[1]).A; a < 127 {
			t.Fatalf("fractalNoise alpha at %v = %d, want ≥127", p, a)
		}
	}
	gradeAgainstPNG4b2(t, doc, "feTurbulence-type=fractalNoise")
}

func TestP384b3TurbulenceSeeds(t *testing.T) {
	// Different seeds → different lattices → different patterns.
	d20 := mustParseFile(t, "filter4b/feTurbulence-seed=20.svg")
	dn20 := mustParseFile(t, "filter4b/feTurbulence-seed=-20.svg")
	i20, in20 := d20.Render(200, 200), dn20.Render(200, 200)
	same := true
	for y := 10; y < 190 && same; y += 7 {
		for x := 10; x < 190 && same; x += 7 {
			if i20.RGBAAt(x, y) != in20.RGBAAt(x, y) {
				same = false
			}
		}
	}
	if same {
		t.Fatal("seed=20 and seed=-20 produced identical noise")
	}
	gradeAgainstPNG4b2(t, d20, "feTurbulence-seed=20")
	gradeAgainstPNG4b2(t, dn20, "feTurbulence-seed=-20")
	gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/feTurbulence-seed=1.5.svg"), "feTurbulence-seed=1.5")
}

func TestP384b3TurbulenceNumOctaves(t *testing.T) {
	// numOctaves=0 → sum 0 → turbulence mode output all-zero (transparent);
	// numOctaves=-1 clamps to 0 the same way (usvg sign rule).
	for _, name := range []string{"feTurbulence-numOctaves=0", "feTurbulence-numOctaves=-1"} {
		doc := mustParseFile(t, "filter4b/"+name+".svg")
		img := doc.Render(200, 200)
		if a := img.RGBAAt(100, 100).A; a != 0 {
			t.Errorf("%s: center alpha = %d, want 0", name, a)
		}
		gradeAgainstPNG4b2(t, doc, name)
	}
	gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/feTurbulence-numOctaves=5.svg"), "feTurbulence-numOctaves=5")
}

func TestP384b3TurbulenceBaseFrequency(t *testing.T) {
	for _, name := range []string{
		"feTurbulence-baseFrequency=0.01",
		"feTurbulence-baseFrequency=0.05-0.05",
		"feTurbulence-baseFrequency=0.05-0",
		"feTurbulence-baseFrequency=0.05--0.01",
		"feTurbulence-baseFrequency=-0.05",
	} {
		gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/"+name+".svg"), name)
	}
}

func TestP384b3TurbulenceStitchAndCIF(t *testing.T) {
	// stitchTiles frequency adjustment + color-interpolation-filters=sRGB.
	gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/feTurbulence-stitchTiles=stitch.svg"), "feTurbulence-stitchTiles=stitch")
	gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/feTurbulence-color-interpolation-filters=sRGB.svg"), "feTurbulence-color-interpolation-filters=sRGB")
}

func TestP384b3TurbulenceComplexTransform(t *testing.T) {
	// skewX(30) on the filtered rect: noise coordinates are post-transform
	// (resvg get_scale ignores the skew off-diagonal → same sampling).
	doc := mustParseFile(t, "filter4b/feTurbulence-complex-transform.svg")
	img := doc.Render(200, 200)
	if !anyNonZeroAlpha(t, img, 40, 60, 160, 130) {
		t.Fatal("skewed turbulence output empty")
	}
	gradeAgainstPNG4b2(t, doc, "feTurbulence-complex-transform")
}

func TestP384b3TurbulencePrimitiveUnitsOBB(t *testing.T) {
	gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/feTurbulence-primitiveUnits=objectBoundingBox.svg"), "feTurbulence-primitiveUnits=objectBoundingBox")
}

func TestP384b3ConvolveParse(t *testing.T) {
	// unset-order: 3×3 default order, kernel sum 0.9 → default divisor.
	doc := mustParseFile(t, "filter4b/feConvolveMatrix-unset-order.svg")
	p := filterOf(t, doc, "filter1").Primitives[0]
	if p.Kind != "feConvolveMatrix" || len(p.KernelMatrix) != 9 {
		t.Fatalf("primitive = %s kernel len = %d", p.Kind, len(p.KernelMatrix))
	}
	cp, ok := convolveMatrixParams(p, true)
	if !ok {
		t.Fatal("unset-order must be valid")
	}
	if cp.Columns != 3 || cp.Rows != 3 || cp.TargetX != 1 || cp.TargetY != 1 {
		t.Fatalf("order/target = %+v", cp)
	}
	if cp.Divisor <= 0.89 || cp.Divisor >= 0.91 {
		t.Fatalf("auto divisor = %v, want ~0.9", cp.Divisor)
	}
	if cp.EdgeMode != 1 /* duplicate */ || cp.PreserveAlpha {
		t.Fatalf("edge/preserveAlpha = %+v", cp)
	}

	// order="4 2" → columns=4 rows=2, kernel 8 values.
	doc2 := mustParseFile(t, "filter4b/feConvolveMatrix-order=4-2.svg")
	cp2, ok2 := convolveMatrixParams(filterOf(t, doc2, "filter1").Primitives[0], true)
	if !ok2 || cp2.Columns != 4 || cp2.Rows != 2 || len(cp2.Kernel) != 8 {
		t.Fatalf("order 4-2 = %+v ok=%v", cp2, ok2)
	}

	// targetX="2" explicit; edgeMode="wrap".
	doc3 := mustParseFile(t, "filter4b/feConvolveMatrix-targetX=2.svg")
	cp3, _ := convolveMatrixParams(filterOf(t, doc3, "filter1").Primitives[0], true)
	if cp3.TargetX != 2 || cp3.TargetY != 1 {
		t.Fatalf("targetX=2 → %+v", cp3)
	}
	doc4 := mustParseFile(t, "filter4b/feConvolveMatrix-edgeMode=wrap.svg")
	cp4, _ := convolveMatrixParams(filterOf(t, doc4, "filter1").Primitives[0], true)
	if cp4.EdgeMode != 2 {
		t.Fatalf("edgeMode=wrap → %d", cp4.EdgeMode)
	}
	doc5 := mustParseFile(t, "filter4b/feConvolveMatrix-preserveAlpha=true.svg")
	cp5, _ := convolveMatrixParams(filterOf(t, doc5, "filter1").Primitives[0], true)
	if !cp5.PreserveAlpha {
		t.Fatal("preserveAlpha=true not parsed")
	}
}

func TestP384b3ConvolveBasics(t *testing.T) {
	// 0.1×9 box-ish blur at the default and explicit orders.
	for _, name := range []string{
		"feConvolveMatrix-unset-order",
		"feConvolveMatrix-order=4",
		"feConvolveMatrix-order=4-2",
		"feConvolveMatrix-order=4-4",
	} {
		gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/"+name+".svg"), name)
	}
}

func TestP384b3ConvolveBias(t *testing.T) {
	// bias=9999 saturates to opaque white inside the pattern rect; the
	// extreme biases must produce visibly different results.
	dWhite := mustParseFile(t, "filter4b/feConvolveMatrix-bias=9999.svg")
	img := dWhite.Render(200, 200)
	if c := img.RGBAAt(110, 110); c.A != 255 || c.R != 255 || c.G != 255 || c.B != 255 {
		t.Fatalf("bias=9999 center = %v, want opaque white", c)
	}
	dDark := mustParseFile(t, "filter4b/feConvolveMatrix-bias=-0.5.svg")
	if dDark.Render(200, 200).RGBAAt(110, 110) == img.RGBAAt(110, 110) {
		t.Fatal("bias=-0.5 and bias=9999 rendered identically")
	}
	gradeAgainstPNG4b2(t, dWhite, "feConvolveMatrix-bias=9999")
	gradeAgainstPNG4b2(t, dDark, "feConvolveMatrix-bias=-0.5")
	gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/feConvolveMatrix-bias=0.5.svg"), "feConvolveMatrix-bias=0.5")
}

func TestP384b3ConvolveDivisor(t *testing.T) {
	// divisor=0 → usvg dummy primitive → transparent output.
	d0 := mustParseFile(t, "filter4b/feConvolveMatrix-divisor=0.svg")
	if a := d0.Render(200, 200).RGBAAt(110, 110).A; a != 0 {
		t.Fatalf("divisor=0 center alpha = %d, want 0", a)
	}
	gradeAgainstPNG4b2(t, d0, "feConvolveMatrix-divisor=0")
	gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/feConvolveMatrix-custom-divisor.svg"), "feConvolveMatrix-custom-divisor")
	// Kernel sums to zero → divisor falls back to 1.0 (valid convolve).
	gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/feConvolveMatrix-kernelMatrix-with-zero-sum-and-no-divisor.svg"), "feConvolveMatrix-kernelMatrix-with-zero-sum-and-no-divisor")
}

func TestP384b3ConvolveEdgeModes(t *testing.T) {
	for _, name := range []string{
		"feConvolveMatrix-edgeMode=none",
		"feConvolveMatrix-edgeMode=wrap",
		"feConvolveMatrix-edgeMode=wrap-with-matrix-larger-than-target",
	} {
		gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/"+name+".svg"), name)
	}
}

func TestP384b3ConvolveTargetX(t *testing.T) {
	gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/feConvolveMatrix-targetX=0.svg"), "feConvolveMatrix-targetX=0")
	gradeAgainstPNG4b2(t, mustParseFile(t, "filter4b/feConvolveMatrix-targetX=2.svg"), "feConvolveMatrix-targetX=2")
	// Out-of-range targets → dummy → transparent.
	for _, name := range []string{"feConvolveMatrix-targetX=-1", "feConvolveMatrix-targetX=3"} {
		doc := mustParseFile(t, "filter4b/"+name+".svg")
		if a := doc.Render(200, 200).RGBAAt(110, 110).A; a != 0 {
			t.Errorf("%s: center alpha = %d, want 0", name, a)
		}
		gradeAgainstPNG4b2(t, doc, name)
	}
}

func TestP384b3ConvolveInvalidKernel(t *testing.T) {
	// Empty / missing / miscounted kernelMatrix and order fallbacks that
	// break the count — all usvg dummies → transparent output.
	for _, name := range []string{
		"feConvolveMatrix-empty-kernelMatrix",
		"feConvolveMatrix-no-kernelMatrix",
		"feConvolveMatrix-kernelMatrix-with-not-enough-values",
		"feConvolveMatrix-kernelMatrix-with-too-many-values",
		"feConvolveMatrix-order=0",
		"feConvolveMatrix-order-with-a-negative-value-1",
		"feConvolveMatrix-order-with-a-negative-value-2",
	} {
		doc := mustParseFile(t, "filter4b/"+name+".svg")
		if a := doc.Render(200, 200).RGBAAt(110, 110).A; a != 0 {
			t.Errorf("%s: center alpha = %d, want 0 (dummy primitive)", name, a)
		}
		gradeAgainstPNG4b2(t, doc, name)
	}
}

func TestP384b3ConvolvePreserveAlpha(t *testing.T) {
	// preserveAlpha: kernel runs on straight alpha, source alpha passes
	// through — the convolved alpha at pattern gaps stays 0 while the
	// default path bleeds neighbour alpha into gaps.
	dP := mustParseFile(t, "filter4b/feConvolveMatrix-preserveAlpha=true.svg")
	gradeAgainstPNG4b2(t, dP, "feConvolveMatrix-preserveAlpha=true")
}
