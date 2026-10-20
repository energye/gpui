package main

import (
	"fmt"
	"image"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
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
	"rrect_image_center",
	"rrect_image_corner",
	"rrect_image_outside",
	"rrect_radius_zero_corner",
	"rrect_nested_inside",
	"rrect_nested_outer",
	"rrect_nested_outside",
	"rrect_pop_restores_image",
	"rrect_text_visible",
	"rrect_text_outside",
	"rrect_text_pop_restores",
}

func newProbeContext(w, h int) *render.Context {
	dc := render.NewContext(w, h)
	dc.ClearWithColor(colCardBG)
	return dc
}

func probeRGB(dc *render.Context, x, y int) [3]float64 {
	if dc == nil {
		return [3]float64{-1, -1, -1}
	}
	img := dc.Image()
	if img == nil {
		return [3]float64{-1, -1, -1}
	}
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		return [3]float64{-1, -1, -1}
	}
	r, g, bl, _ := img.At(x, y).RGBA()
	return [3]float64{float64(r>>8) / 255, float64(g>>8) / 255, float64(bl>>8) / 255}
}

func nearRGB(got, want [3]float64, tol float64) bool {
	for i := 0; i < 3; i++ {
		if math.Abs(got[i]-want[i]) > tol {
			return false
		}
	}
	return true
}

func rgbOf(c render.RGBA) [3]float64 {
	return [3]float64{c.R, c.G, c.B}
}

func inverted(c render.RGBA) [3]float64 {
	return [3]float64{1 - c.R, 1 - c.G, 1 - c.B}
}

func drawQuadImageCPU(dc *render.Context, img *render.ImageBuf, dst dstSpec) {
	if dc == nil || img == nil {
		return
	}
	dc.DrawImageEx(img, render.DrawImageOptions{
		X: dst.x, Y: dst.y,
		DstWidth: dst.w, DstHeight: dst.h,
		Interpolation: render.InterpNearest,
	})
}

func drawTextRows(dc *render.Context, ox, oy float64, face text.Face) {
	if dc == nil || face == nil {
		return
	}
	dc.SetFont(face)
	dc.SetRGBA(colText.R, colText.G, colText.B, colText.A)
	for i := 0; i < 3; i++ {
		dc.DrawString(textLine, ox+cClip.x-56, oy+cClip.y+42+float64(i*40))
	}
}

func runLogicProbes() []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}
	img, err := makeQuadImage(200, 160)
	if err != nil {
		add("rrect_image_center", false, "makeQuadImage: "+err.Error())
		for _, k := range probeKeys[1:] {
			add(k, false, "image unavailable")
		}
		return out
	}

	// A/B: image clipped by a rounded rectangle. The image is larger than the
	// clip, so an ignored RRect shows the image in the corner cut-outs.
	dc := newProbeContext(240, 200)
	dc.Push()
	dc.ClipRoundRect(bClip.x, bClip.y, bClip.w, bClip.h, bClip.r)
	drawQuadImageCPU(dc, img, dstSpec{x: bDst.x, y: bDst.y, w: bDst.w, h: bDst.h})
	dc.Pop()
	got := probeRGB(dc, 80, 70)
	add("rrect_image_center", nearRGB(got, rgbOf(colQuadTL), 0.02),
		fmt.Sprintf("center=%v want=%v", got, rgbOf(colQuadTL)))
	got = probeRGB(dc, int(bClip.x+2), int(bClip.y+2))
	add("rrect_image_corner", nearRGB(got, rgbOf(colCardBG), 0.02),
		fmt.Sprintf("corner=%v want cardBG=%v", got, rgbOf(colCardBG)))
	got = probeRGB(dc, 10, 70)
	add("rrect_image_outside", nearRGB(got, rgbOf(colCardBG), 0.02),
		fmt.Sprintf("outside=%v want cardBG=%v", got, rgbOf(colCardBG)))
	dc.Close()

	// radius 0 is a plain rectangular clip: the corner must show the image.
	zc := newProbeContext(220, 180)
	zc.Push()
	zc.ClipRoundRect(20, 20, 160, 120, 0)
	drawQuadImageCPU(zc, img, dstSpec{x: 0, y: 0, w: 200, h: 160})
	zc.Pop()
	got = probeRGB(zc, 22, 22)
	add("rrect_radius_zero_corner", nearRGB(got, rgbOf(colQuadTL), 0.02),
		fmt.Sprintf("corner=%v want image TL=%v", got, rgbOf(colQuadTL)))
	zc.Close()

	// Nested clip: a rect inside the RRect must intersect, not replace it.
	nc := newProbeContext(240, 200)
	nc.Push()
	nc.ClipRoundRect(40, 20, 160, 140, 28)
	nc.Push()
	nc.ClipRect(80, 50, 60, 60)
	drawQuadImageCPU(nc, img, dstSpec{x: 0, y: 0, w: 200, h: 160})
	nc.Pop()
	nc.Pop()
	got = probeRGB(nc, 90, 70)
	add("rrect_nested_inside", nearRGB(got, rgbOf(colQuadTL), 0.03),
		fmt.Sprintf("inside=%v want image TL=%v", got, rgbOf(colQuadTL)))
	got = probeRGB(nc, 60, 34)
	add("rrect_nested_outer", nearRGB(got, rgbOf(colCardBG), 0.03),
		fmt.Sprintf("outer-only=%v want cardBG=%v", got, rgbOf(colCardBG)))
	got = probeRGB(nc, 10, 10)
	add("rrect_nested_outside", nearRGB(got, rgbOf(colCardBG), 0.03),
		fmt.Sprintf("outside=%v want cardBG=%v", got, rgbOf(colCardBG)))
	nc.Close()

	// Push/Pop restore: after the clipped draw, an unclipped draw must keep
	// its sharp image corners on the right side of card D.
	pc := newProbeContext(430, 232)
	pc.Push()
	pc.ClipRoundRect(dClip.x, dClip.y, dClip.w, dClip.h, dClip.r)
	drawQuadImageCPU(pc, img, dDst)
	pc.Pop()
	drawQuadImageCPU(pc, img, dRight)
	got = probeRGB(pc, int(dClip.x+2), int(dClip.y+2))
	leftCorner := nearRGB(got, rgbOf(colCardBG), 0.03)
	got = probeRGB(pc, int(dRight.x+2), int(dRight.y+2))
	rightCorner := nearRGB(got, rgbOf(colQuadTL), 0.03)
	add("rrect_pop_restores_image", leftCorner && rightCorner,
		fmt.Sprintf("leftCorner=%v rightCorner=%v", got, rgbOf(colQuadTL)))
	pc.Close()

	// Text: glyph ink must survive inside the RRect and must not appear in the
	// corner cut-outs or outside the clip. The text origin is deliberately
	// left of the clip so a missing clip leaks a large, visible run.
	face, _, err := text.LoadDefaultFace(40)
	if err != nil {
		add("rrect_text_visible", false, "font: "+err.Error())
		add("rrect_text_outside", false, "font: "+err.Error())
		add("rrect_text_pop_restores", false, "font: "+err.Error())
		return out
	}
	tc := newProbeContext(430, 232)
	tc.Push()
	tc.ClipRoundRect(cClip.x, cClip.y, cClip.w, cClip.h, cClip.r)
	drawTextRows(tc, 0, 0, face)
	tc.Pop()
	inside, outside := countTextInk(tc.Image(), 430, 232, cClip, 2)
	add("rrect_text_visible", inside > 200, fmt.Sprintf("inside=%d", inside))
	add("rrect_text_outside", outside == 0, fmt.Sprintf("outside=%d", outside))
	// After Pop, text drawn left of the clip is visible again.
	tc.Push()
	tc.ClipRoundRect(cClip.x, cClip.y, cClip.w, cClip.h, cClip.r)
	drawTextRows(tc, 0, 0, face)
	tc.Pop()
	tc.SetFont(face)
	tc.SetRGBA(colText.R, colText.G, colText.B, colText.A)
	tc.DrawString("MMMM MMMM", 0, 64)
	_, outside = countTextInk(tc.Image(), 430, 232, cClip, 2)
	add("rrect_text_pop_restores", outside > 50, fmt.Sprintf("outsideAfterPop=%d", outside))
	tc.Close()

	return out
}

// countTextInk counts bright glyph pixels in the card content, split into
// inside the RRect and outside its 2px-expanded bounding box.
func countTextInk(img image.Image, w, h int, clip clipSpec, margin int) (inside, outside int) {
	if img == nil {
		return 0, 0
	}
	b := img.Bounds()
	x0, y0 := int(clip.x), int(clip.y)
	x1, y1 := int(clip.x+clip.w), int(clip.y+clip.h)
	// Do not count card titles above the clip as text leakage.
	for y := y0; y < h && y < b.Dy(); y++ {
		for x := 0; x < w && x < b.Dx(); x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			bright := (0.2126*float64(r>>8) + 0.7152*float64(g>>8) + 0.0722*float64(bl>>8)) > 150
			if !bright {
				continue
			}
			in := x >= x0 && x < x1 && y >= y0 && y < y1
			if in {
				inside++
				continue
			}
			if x < x0-margin || x >= x1+margin || y < y0-margin || y >= y1+margin {
				outside++
			}
		}
	}
	return inside, outside
}

// ---------------------------------------------------------------------------
// GPU snapshot pixel probes
// ---------------------------------------------------------------------------

func runPixelProbes(img image.Image, bx, by, dpr float64) []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}
	if img == nil {
		for _, k := range []string{
			"pixel_a_texture_center", "pixel_a_texture_corner", "pixel_a_texture_edge", "pixel_a_texture_outside",
			"pixel_b_image_center", "pixel_b_image_corner", "pixel_b_image_edge", "pixel_b_image_outside",
			"pixel_c_text_inside", "pixel_c_text_outside", "pixel_c_text_corner",
			"pixel_d_left_corner", "pixel_d_left_center", "pixel_d_right_corner", "pixel_d_right_center",
		} {
			add(k, false, "snapshot unavailable")
		}
		return out
	}
	read := func(card int, x, y float64) ([3]float64, bool) {
		ax := bx + cardAt[card][0] + x
		ay := by + cardAt[card][1] + y
		px, py := int(math.Round(ax*dpr)), int(math.Round(ay*dpr))
		b := img.Bounds()
		if px < b.Min.X || px >= b.Max.X || py < b.Min.Y || py >= b.Max.Y {
			return [3]float64{}, false
		}
		r, g, bl, _ := img.At(px, py).RGBA()
		return [3]float64{float64(r>>8) / 255, float64(g>>8) / 255, float64(bl>>8) / 255}, true
	}
	check := func(key string, card int, x, y float64, want [3]float64) {
		got, ok := read(card, x, y)
		if !ok {
			add(key, false, "sample outside snapshot")
			return
		}
		add(key, nearRGB(got, want, 0.03), fmt.Sprintf("got=%v want=%v", got, want))
	}

	check("pixel_a_texture_center", 0, 150, 62, inverted(colQuadTL))
	check("pixel_a_texture_corner", 0, aClip.x+2, aClip.y+2, rgbOf(colCardBG))
	check("pixel_a_texture_edge", 0, 160, 32, inverted(colQuadTL))
	check("pixel_a_texture_outside", 0, aClip.x-10, aClip.y+70, rgbOf(colCardBG))

	check("pixel_b_image_center", 1, 80, 70, rgbOf(colQuadTL))
	check("pixel_b_image_corner", 1, bClip.x+2, bClip.y+2, rgbOf(colCardBG))
	check("pixel_b_image_edge", 1, 80, 32, rgbOf(colQuadTL))
	check("pixel_b_image_outside", 1, 10, 70, rgbOf(colCardBG))

	check("pixel_c_text_corner", 2, cClip.x+3, cClip.y+3, rgbOf(colCardBG))
	inside, outside := countTextInkCard(img, bx, by, dpr, 2, cClip, 2)
	add("pixel_c_text_inside", inside > 200, fmt.Sprintf("inside=%d", inside))
	add("pixel_c_text_outside", outside == 0, fmt.Sprintf("outside=%d", outside))

	check("pixel_d_left_corner", 3, dClip.x+2, dClip.y+2, rgbOf(colCardBG))
	check("pixel_d_left_center", 3, 70, 70, rgbOf(colQuadTL))
	check("pixel_d_right_corner", 3, dRight.x+2, dRight.y+2, rgbOf(colQuadTL))
	check("pixel_d_right_center", 3, 300, 80, rgbOf(colQuadTL))

	return out
}

// countTextInkCard is countTextInk in window-snapshot coordinates.
func countTextInkCard(img image.Image, bx, by, dpr float64, card int, clip clipSpec, margin int) (inside, outside int) {
	if img == nil {
		return 0, 0
	}
	b := img.Bounds()
	x0 := int(math.Round((bx + cardAt[card][0] + clip.x) * dpr))
	y0 := int(math.Round((by + cardAt[card][1] + clip.y) * dpr))
	x1 := int(math.Round((bx + cardAt[card][0] + clip.x + clip.w) * dpr))
	y1 := int(math.Round((by + cardAt[card][1] + clip.y + clip.h) * dpr))
	cx0 := int(math.Round((bx + cardAt[card][0]) * dpr))
	// Start at the clip top: the card title above the clip is not content and
	// must not be mistaken for leaked glyph ink.
	cy0 := int(math.Round((by + cardAt[card][1] + clip.y) * dpr))
	cx1 := int(math.Round((bx + cardAt[card][0] + cardW) * dpr))
	cy1 := int(math.Round((by + cardAt[card][1] + cardH) * dpr))
	m := int(math.Round(float64(margin) * dpr))
	for y := cy0; y < cy1 && y < b.Max.Y; y++ {
		for x := cx0; x < cx1 && x < b.Max.X; x++ {
			if x < b.Min.X || y < b.Min.Y {
				continue
			}
			r, g, bl, _ := img.At(x, y).RGBA()
			bright := (0.2126*float64(r>>8) + 0.7152*float64(g>>8) + 0.0722*float64(bl>>8)) > 150
			if !bright {
				continue
			}
			in := x >= x0 && x < x1 && y >= y0 && y < y1
			if in {
				inside++
				continue
			}
			if x < x0-m || x >= x1+m || y < y0-m || y >= y1+m {
				outside++
			}
		}
	}
	return inside, outside
}
