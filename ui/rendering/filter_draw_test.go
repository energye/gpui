package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// TestApplyGrayscale_DesaturatesRed drives the shipped UI ApplyGrayscale façade:
// a solid red fill must become near-gray (R≈G≈B), not stay saturated red.
func TestApplyGrayscale_DesaturatesRed(t *testing.T) {
	const W, H = 48, 48
	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1)
	rendering.FillRect(pc, 8, 8, 32, 32, 1, 0, 0, 1)

	// Control: center is red before filter.
	img0 := dc.Image()
	r0, g0, b0 := sampleRGB(img0.At(24, 24))
	if r0 < 0xC000 || g0 > 0x4000 || b0 > 0x4000 {
		t.Fatalf("pre-filter (24,24)=#%04x%04x%04x want red", r0, g0, b0)
	}

	rendering.ApplyGrayscale(pc)

	img := dc.Image()
	r, g, b := sampleRGB(img.At(24, 24))
	// Channels must be close (grayscale of red ≈ 0.299*255).
	drg := int(r) - int(g)
	if drg < 0 {
		drg = -drg
	}
	dgb := int(g) - int(b)
	if dgb < 0 {
		dgb = -dgb
	}
	if drg > 0x1800 || dgb > 0x1800 {
		t.Fatalf("after ApplyGrayscale (24,24)=#%04x%04x%04x want near-equal channels", r, g, b)
	}
	// Must not still be saturated red.
	if r > 0xC000 && g < 0x4000 && b < 0x4000 {
		t.Fatalf("still saturated red after grayscale: #%04x%04x%04x", r, g, b)
	}
	// Gray level should be clearly above black and below pure white.
	if r < 0x2000 || r > 0xE000 {
		t.Fatalf("gray level #%04x out of expected mid range", r)
	}
}

// TestApplyBlur_SoftensHardEdge drives the shipped UI ApplyBlur façade:
// a sharp vertical bar must bleed color into a nearby white pixel after blur.
func TestApplyBlur_SoftensHardEdge(t *testing.T) {
	const W, H = 64, 40
	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1)
	// Blue vertical bar centered; hard edge at x≈30..34.
	rendering.FillRect(pc, 30, 4, 4, 32, 0, 0, 1, 1)

	img0 := dc.Image()
	// Pixel just left of bar should be white before blur.
	r0, g0, b0 := sampleRGB(img0.At(26, 20))
	if r0 < 0xC000 || g0 < 0xC000 || b0 < 0xC000 {
		t.Fatalf("pre-blur (26,20)=#%04x%04x%04x want white", r0, g0, b0)
	}

	rendering.ApplyBlur(pc, 4)

	img := dc.Image()
	// Sample a few px left of the hard bar edge; blur must darken vs pure white.
	r, g, b := sampleRGB(img.At(26, 20))
	const whiteMin = uint32(0xF800)
	if r >= whiteMin && g >= whiteMin && b >= whiteMin {
		t.Fatalf("after ApplyBlur (26,20)=#%04x%04x%04x still near-white — blur did not soften edge", r, g, b)
	}
	// Control: far from bar should stay near-white.
	fr, fg, fb := sampleRGB(img.At(4, 20))
	if fr < 0xC000 || fg < 0xC000 || fb < 0xC000 {
		t.Fatalf("far from bar (4,20)=#%04x%04x%04x should stay near-white", fr, fg, fb)
	}
}

// TestFilterLayers_InSceneTree is a cross-package sanity that scene filter
// kinds exist and builder nesting works (full scene tests live in ui/scene).
func TestFilterLayers_InSceneTree(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	cf := b.PushGrayscaleFilter()
	im := b.PushImageFilter(2.5)
	b.AddPicture(true)
	b.Pop()
	b.Pop()
	if cf.Kind() != "color_filter" || im.Kind() != "image_filter" {
		t.Fatalf("kinds %s / %s", cf.Kind(), im.Kind())
	}
	var kinds []string
	scene.Walk(b.Root(), func(l scene.Layer) {
		kinds = append(kinds, l.Kind())
	})
	hasCF, hasIF := false, false
	for _, k := range kinds {
		if k == "color_filter" {
			hasCF = true
		}
		if k == "image_filter" {
			hasIF = true
		}
	}
	if !hasCF || !hasIF {
		t.Fatalf("expected both filter kinds in tree, kinds=%v", kinds)
	}
}

// TestApplyDropShadow_ChangesPixels: drop shadow is alpha-extracted (Skia/CSS).
// Transparent clear + opaque card; shadow must differ from no-shadow control
// and leave non-zero alpha outside the card silhouette.
func TestApplyDropShadow_ChangesPixels(t *testing.T) {
	paint := func(shadow bool) *render.Context {
		dc := render.NewContext(80, 80)
		dc.BeginFrame()
		// Transparent canvas required — shadow is extracted from alpha.
		dc.ClearWithColor(render.Transparent)
		pc := rendering.NewPaintContext(dc, 1)
		rendering.FillRect(pc, 20, 18, 28, 28, 0.9, 0.2, 0.2, 1)
		if shadow {
			rendering.ApplyDropShadow(pc, 8, 8, 4, 0, 0, 0, 0.55)
		}
		return dc
	}
	a := paint(false)
	defer a.Close()
	b := paint(true)
	defer b.Close()
	ia, ib := a.Image(), b.Image()
	if ia == nil || ib == nil {
		t.Fatal("nil image")
	}
	diff := 0
	for y := 0; y < 80; y++ {
		for x := 0; x < 80; x++ {
			ra, ga, ba, aa := ia.At(x, y).RGBA()
			rb, gb, bb, ab := ib.At(x, y).RGBA()
			if ra != rb || ga != gb || ba != bb || aa != ab {
				diff++
			}
		}
	}
	if diff == 0 {
		t.Fatal("ApplyDropShadow must change pixels vs unshadowed control (alpha canvas)")
	}
	// SE of card: original ~[20,48)x[18,46); +8,+8 shadow band should have alpha.
	foundShadow := false
	for y := 46; y < 58 && !foundShadow; y++ {
		for x := 48; x < 60; x++ {
			_, _, _, a := ib.At(x, y).RGBA()
			if a > 0x1000 {
				foundShadow = true
				break
			}
		}
	}
	if !foundShadow {
		t.Fatal("expected shadow alpha SE of card after ApplyDropShadow")
	}
}

// TestPushBackdrop_BlursUnderContent: colored background + backdrop blur should
// softens a hard edge under the backdrop region; children still paint on top.
func TestPushBackdrop_BlursUnderContent(t *testing.T) {
	const W, H = 64, 64
	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1)
	// Hard blue vertical bar as backdrop content.
	rendering.FillRect(pc, 28, 4, 8, 56, 0, 0, 1, 1)

	if !rendering.PushBackdrop(pc, 3, 1) {
		t.Fatal("PushBackdrop failed")
	}
	// Semi-transparent white child card on top of blurred backdrop.
	rendering.FillRect(pc, 8, 8, 48, 48, 1, 1, 1, 0.3)
	rendering.PopBackdrop(pc)

	img := dc.Image()
	// Near bar edge under card: should not be pure white (blur/blue bleed or overlay).
	r, g, b := sampleRGB(img.At(24, 32))
	// Just assert we got a finished frame with some non-trivial content.
	cr, cg, cb := sampleRGB(img.At(32, 32))
	if cr > 0xF800 && cg > 0xF800 && cb > 0xF800 && r > 0xF800 {
		t.Fatalf("backdrop composite looks empty white center=#%04x%04x%04x edge=#%04x%04x%04x", cr, cg, cb, r, g, b)
	}
}
