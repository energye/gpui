package main

import (
	"fmt"
	"image"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/svg"
)

type probeResult struct {
	Key    string
	OK     bool
	Detail string
}

var probeKeys = []string{
	"fill_none_inherited",
	"root_percent_dims",
	"filled_center",
	"stroke_center_transparent",
	"override_keeps_none",
	"nested_transform",
	"invalid_xml_rejected",
}

func rgbOf(c render.RGBA) [3]float64 {
	return [3]float64{c.R, c.G, c.B}
}

func nearRGB(got, want [3]float64, tol float64) bool {
	for i := 0; i < 3; i++ {
		if math.Abs(got[i]-want[i]) > tol {
			return false
		}
	}
	return true
}

func probePixel(img image.Image, x, y int) (rgb [3]float64, alpha float64, ok bool) {
	if img == nil {
		return rgb, 0, false
	}
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		return rgb, 0, false
	}
	r, g, bl, a := img.At(x, y).RGBA()
	return [3]float64{float64(r>>8) / 255, float64(g>>8) / 255, float64(bl>>8) / 255},
		float64(a>>8) / 255, true
}

func runLogicProbes() []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}

	// P2 regression: root fill="none" must survive the SVG default fill.
	img, err := svg.Render([]byte(`<svg viewBox="0 0 16 16" fill="none"><rect width="16" height="16"/></svg>`), 16, 16)
	inheritOK := err == nil
	inheritDetail := "render error"
	if inheritOK {
		for y := 0; y < 16 && inheritOK; y++ {
			for x := 0; x < 16; x++ {
				if _, a, _ := probePixel(img, x, y); a != 0 {
					inheritOK = false
					inheritDetail = fmt.Sprintf("alpha=%v at (%d,%d)", a, x, y)
					break
				}
			}
		}
		if inheritOK {
			inheritDetail = "all 16x16 pixels transparent"
		}
	}
	add("fill_none_inherited", inheritOK, inheritDetail)

	// P3 regression: percentages resolve against the viewBox, not stripped.
	doc, err := svg.Parse([]byte(`<svg width="50%" height="25%" viewBox="0 0 16 8"></svg>`))
	percentOK := err == nil && doc.Width == 8 && doc.Height == 2
	add("root_percent_dims", percentOK,
		fmt.Sprintf("width=%.3g height=%.3g want 8/2 err=%v", docWidth(doc), docHeight(doc), err))

	// Filled baseline.
	img, err = svg.Render([]byte(svgFilled), 64, 64)
	center, centerA, ok := probePixel(img, 32, 32)
	_, cornerA, _ := probePixel(img, 4, 4)
	add("filled_center", err == nil && ok && centerA > 0.99 && nearRGB(center, rgbOf(colFilled), 0.04) && cornerA == 0,
		fmt.Sprintf("center=%v a=%.3f cornerA=%.3f err=%v", center, centerA, cornerA, err))

	// Stroke-only: inherited fill=none must keep the center transparent.
	img, err = svg.Render([]byte(svgStroke), 64, 64)
	_, centerA, _ = probePixel(img, 32, 32)
	ring, ringA, _ := probePixel(img, 32, 8)
	add("stroke_center_transparent", err == nil && centerA == 0 && ringA > 0.99 && nearRGB(ring, rgbOf(colStroke), 0.06),
		fmt.Sprintf("centerA=%.3f ring=%v ringA=%.3f err=%v", centerA, ring, ringA, err))

	// Color override must recolor the stroke but leave the inherited none area.
	img, err = svg.RenderWithColor([]byte(svgStroke), 64, 64, imageColor(colOverride))
	_, centerA, _ = probePixel(img, 32, 32)
	ring, ringA, _ = probePixel(img, 32, 8)
	add("override_keeps_none", err == nil && centerA == 0 && ringA > 0.99 && nearRGB(ring, rgbOf(colOverride), 0.06),
		fmt.Sprintf("centerA=%.3f ring=%v ringA=%.3f err=%v", centerA, ring, ringA, err))

	// Nested <g transform="translate(2 2)">: both squares shift; the gap stays empty.
	img, err = svg.Render([]byte(svgNested), 64, 64)
	first, firstA, _ := probePixel(img, 20, 20)
	second, secondA, _ := probePixel(img, 44, 44)
	_, gapA, _ := probePixel(img, 32, 32)
	add("nested_transform", err == nil && firstA > 0.99 && secondA > 0.99 && gapA == 0 &&
		nearRGB(first, rgbOf(colNested), 0.04) && nearRGB(second, rgbOf(colNested), 0.04),
		fmt.Sprintf("first=%v a=%.3f second=%v a=%.3f gapA=%.3f err=%v",
			first, firstA, second, secondA, gapA, err))

	if _, err := svg.Parse([]byte("not xml at all")); err == nil {
		add("invalid_xml_rejected", false, "Parse accepted invalid XML")
	} else {
		add("invalid_xml_rejected", true, err.Error())
	}

	return out
}

func docWidth(d *svg.Document) float64 {
	if d == nil {
		return -1
	}
	return d.Width
}

func docHeight(d *svg.Document) float64 {
	if d == nil {
		return -1
	}
	return d.Height
}

// ---------------------------------------------------------------------------
// GPU snapshot pixel probes
// ---------------------------------------------------------------------------

func runPixelProbes(img image.Image, bx, by, dpr float64) []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}
	keys := []string{
		"pixel_a_center", "pixel_a_corner",
		"pixel_b_center", "pixel_b_ring",
		"pixel_c_first", "pixel_c_second", "pixel_c_gap",
		"pixel_d_center", "pixel_d_corner",
	}
	if img == nil {
		for _, k := range keys {
			add(k, false, "snapshot unavailable")
		}
		return out
	}

	read := func(card int, x, y float64) ([3]float64, bool) {
		ax := bx + cardAt[card][0] + x
		ay := by + cardAt[card][1] + y
		px, py := int(math.Round(ax*dpr)), int(math.Round(ay*dpr))
		rgb, _, ok := probePixel(img, px, py)
		return rgb, ok
	}
	check := func(key string, card int, x, y float64, want [3]float64) {
		got, ok := read(card, x, y)
		if !ok {
			add(key, false, "sample outside snapshot")
			return
		}
		add(key, nearRGB(got, want, 0.04), fmt.Sprintf("got=%v want=%v", got, want))
	}

	check("pixel_a_center", 0, iconX+iconSize/2, iconY+iconSize/2, rgbOf(colFilled))
	check("pixel_a_corner", 0, iconX+20, iconY+20, rgbOf(colCardBG))

	check("pixel_b_center", 1, iconX+iconSize/2, iconY+iconSize/2, rgbOf(colCardBG))
	check("pixel_b_ring", 1, iconX+iconSize/2, iconY+28, rgbOf(colStroke))

	check("pixel_c_first", 2, iconX+56, iconY+56, rgbOf(colNested))
	check("pixel_c_second", 2, iconX+124, iconY+124, rgbOf(colNested))
	check("pixel_c_gap", 2, iconX+90, iconY+90, rgbOf(colCardBG))

	check("pixel_d_center", 3, iconX+iconSize/2, iconY+iconSize/2, rgbOf(colOverride))
	check("pixel_d_corner", 3, iconX+20, iconY+20, rgbOf(colCardBG))

	return out
}
