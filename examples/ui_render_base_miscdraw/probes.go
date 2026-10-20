package main

import (
	"fmt"
	"image"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

type probeResult struct {
	Key    string
	OK     bool
	Detail string
}

var probeKeys = []string{
	"misc_nil_context",
	"draw_color_clip",
	"draw_color_blend_copy",
	"draw_paint_gradient_clip",
	"draw_paint_preserves_path",
	"clip_bounds_unclipped",
	"clip_bounds_nested",
	"clip_bounds_transform",
	"clip_bounds_path",
	"draw_paragraph_lines",
	"draw_paragraph_origin",
	"draw_shadow_soft_not_occluder",
	"draw_shadow_elevation_noop",
	"draw_shadow_spot_alpha",
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func rawMiscPixel(img image.Image, x, y int) [4]int {
	if img == nil {
		return [4]int{}
	}
	r, g, b, a := img.At(x, y).RGBA()
	return [4]int{int(r >> 8), int(g >> 8), int(b >> 8), int(a >> 8)}
}

func nearU8(got, want, tol int) bool { return absInt(got-want) <= tol }

func probeDC(w, h int) *render.Context { return render.NewContext(w, h) }

func runLogicProbes() []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}

	// 1. Nil receiver is a no-op for all five entry points.
	func() {
		defer func() {
			if r := recover(); r != nil {
				add("misc_nil_context", false, fmt.Sprintf("panic: %v", r))
			}
		}()
		var dc *render.Context
		dc.DrawColor(render.RGBA{A: 1}, render.BlendNormal)
		dc.DrawPaint()
		dc.DrawShadow(nil, render.RGBA{A: 1}, 1, false)
		dc.DrawParagraph(nil, 0, 0)
		b := dc.ClipBounds()
		add("misc_nil_context", b.Empty(), "nil methods no-op, ClipBounds empty")
	}()

	// 2. DrawColor fills only the clip and leaves path + paint state alone.
	{
		dc := probeDC(80, 60)
		dc.ClearWithColor(render.RGBA{R: 1, A: 1})
		dc.SetRGBA(0, 1, 0, 1)
		dc.DrawRectangle(12, 12, 6, 6)
		dc.ClipRect(10, 10, 30, 20)
		dc.DrawColor(render.RGBA{B: 1, A: 1}, render.BlendNormal)
		img := dc.Image()
		in := rawMiscPixel(img, 20, 20)
		outPx := rawMiscPixel(img, 5, 5)
		_ = dc.Fill()
		fill := rawMiscPixel(dc.Image(), 14, 14)
		dc.Close()
		ok := in == [4]int{0, 0, 255, 255} && outPx == [4]int{255, 0, 0, 255} &&
			fill == [4]int{0, 255, 0, 255}
		add("draw_color_clip", ok, fmt.Sprintf("in=%v out=%v preserved=%v", in, outPx, fill))
	}

	// 3. DrawColor honors an explicit replace blend.
	{
		dc := probeDC(40, 30)
		dc.ClearWithColor(render.RGBA{R: 1, A: 1})
		dc.ClipRect(5, 5, 20, 15)
		dc.DrawColor(render.RGBA{B: 1, A: 1}, render.BlendCopy)
		img := dc.Image()
		in := rawMiscPixel(img, 10, 10)
		outPx := rawMiscPixel(img, 2, 2)
		dc.Close()
		add("draw_color_blend_copy", in == [4]int{0, 0, 255, 255} && outPx == [4]int{255, 0, 0, 255},
			fmt.Sprintf("copy in=%v out=%v", in, outPx))
	}

	// 4. DrawPaint uses the current brush and clip, not the current path.
	{
		dc := probeDC(80, 60)
		dc.ClearWithColor(render.RGBA{R: 1, A: 1})
		b := render.NewLinearGradientBrush(10, 0, 50, 0)
		b.AddColorStop(0, render.RGBA{R: 1, A: 1})
		b.AddColorStop(1, render.RGBA{B: 1, A: 1})
		dc.SetFillBrush(b)
		dc.ClipRect(10, 10, 40, 30)
		dc.DrawPaint()
		img := dc.Image()
		l := rawMiscPixel(img, 12, 20)
		r := rawMiscPixel(img, 48, 20)
		outPx := rawMiscPixel(img, 60, 20)
		dc.Close()
		ok := l[0] > 200 && l[2] < 130 && r[2] > 200 && r[0] < 130 &&
			outPx == [4]int{255, 0, 0, 255}
		add("draw_paint_gradient_clip", ok, fmt.Sprintf("left=%v right=%v out=%v", l, r, outPx))
	}

	// 5. DrawPaint preserves the current path for a later Fill.
	{
		dc := probeDC(50, 40)
		dc.ClearWithColor(render.RGBA{R: 1, A: 1})
		dc.DrawRectangle(12, 12, 8, 8)
		dc.ClipRect(5, 5, 30, 25)
		dc.SetRGBA(0, 0, 1, 1)
		dc.DrawPaint()
		dc.SetRGBA(0, 1, 0, 1)
		_ = dc.Fill()
		px := rawMiscPixel(dc.Image(), 15, 15)
		dc.Close()
		add("draw_paint_preserves_path", px == [4]int{0, 255, 0, 255}, fmt.Sprintf("path px=%v", px))
	}

	// 6-9. ClipBounds: full canvas, nested intersection, CTM inverse, path AABB.
	{
		dc := probeDC(100, 80)
		full := dc.ClipBounds()
		dc.ClipRect(10, 20, 30, 40)
		rect := dc.ClipBounds()
		dc.ClipRect(15, 25, 10, 10)
		nested := dc.ClipBounds()
		dc.Close()
		wantFull := image.Rect(0, 0, 100, 80)
		wantRect := image.Rect(10, 20, 40, 60)
		wantNested := image.Rect(15, 25, 25, 35)
		add("clip_bounds_unclipped", full == wantFull, fmt.Sprintf("full=%v", full))
		add("clip_bounds_nested", rect == wantRect && nested == wantNested,
			fmt.Sprintf("rect=%v nested=%v", rect, nested))
	}
	{
		dc := probeDC(100, 80)
		dc.Translate(5, 7)
		dc.Scale(2, 2)
		dc.ClipRect(10, 20, 10, 10)
		got := dc.ClipBounds()
		dc.Close()
		add("clip_bounds_transform", got == image.Rect(10, 20, 20, 30), fmt.Sprintf("local=%v", got))
	}
	{
		dc := probeDC(100, 80)
		dc.Translate(50, 40)
		dc.Rotate(math.Pi / 4)
		p := render.NewPath()
		p.MoveTo(-10, -10)
		p.LineTo(10, -10)
		p.LineTo(10, 10)
		p.LineTo(-10, 10)
		p.Close()
		dc.DrawPath(p)
		dc.Clip()
		got := dc.ClipBounds()
		dc.Close()
		ok := !got.Empty()
		for _, pt := range []image.Point{{X: -11, Y: -11}, {X: 11, Y: 11}} {
			if !pt.In(got) {
				ok = false
			}
		}
		add("clip_bounds_path", ok, fmt.Sprintf("rotated AABB=%v", got))
	}

	// 10-11. DrawParagraph paints every line at its baseline, from the origin.
	{
		src, err := text.NewFontSourceFromFile("render/text/testdata/source-sans/TTF/SourceSans3-Regular.ttf")
		if err != nil {
			add("draw_paragraph_lines", false, fmt.Sprintf("font: %v", err))
			add("draw_paragraph_origin", false, "font unavailable")
		} else {
			dc := probeDC(240, 160)
			dc.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})
			dc.SetRGBA(0, 0, 0, 1)
			layout := text.LayoutText("HH\nHH", src.Face(28), text.LayoutOptions{LineSpacing: 2.0})
			dc.DrawParagraph(layout, 20, 20)
			img := dc.Image()
			dc.Close()
			count := func(y0, y1 int) int {
				n := 0
				for y := y0; y < y1; y++ {
					for x := 0; x < 240; x++ {
						p := rawMiscPixel(img, x, y)
						if p[3] != 0 && (p[0] < 240 || p[1] < 240 || p[2] < 240) {
							n++
						}
					}
				}
				return n
			}
			l0, l1 := layout.Lines[0], layout.Lines[1]
			n0 := count(20+int(l0.Y-l0.Ascent), 20+int(l0.Y+l0.Descent)+1)
			n1 := count(20+int(l1.Y-l1.Ascent), 20+int(l1.Y+l1.Descent)+1)
			leftInk := 0
			for y := 0; y < 160; y++ {
				for x := 0; x < 19; x++ {
					p := rawMiscPixel(img, x, y)
					if p[3] != 0 && (p[0] < 240 || p[1] < 240 || p[2] < 240) {
						leftInk++
					}
				}
			}
			add("draw_paragraph_lines", n0 > 0 && n1 > 0, fmt.Sprintf("line ink=%d/%d", n0, n1))
			add("draw_paragraph_origin", leftInk == 0, fmt.Sprintf("ink left of x=20: %d", leftInk))
		}
	}

	// 12. DrawShadow is a soft offset silhouette, not the occluder itself.
	{
		restore := render.DisableGPUFilterGraphForTest()
		dc := probeDC(160, 140)
		dc.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})
		p := render.NewPath()
		p.MoveTo(30, 30)
		p.LineTo(100, 30)
		p.LineTo(100, 80)
		p.LineTo(30, 80)
		p.Close()
		dc.DrawShadow(p, render.RGBA{A: 1}, 10, false)
		img := dc.Image()
		dc.Close()
		restore()
		above := rawMiscPixel(img, 65, 12)
		inside := rawMiscPixel(img, 65, 35)
		below := rawMiscPixel(img, 65, 90)
		ok := above[0] >= 245 && above[1] >= 245 && above[2] >= 245 &&
			inside[0] > 150 && inside[1] > 150 && inside[2] > 150 &&
			below[0] < 245 && below[0] > 150 &&
			below[0] == below[1] && below[1] == below[2]
		add("draw_shadow_soft_not_occluder", ok,
			fmt.Sprintf("above=%v inside=%v below=%v", above, inside, below))
	}

	// 13. Elevation <= 0 and nil paths are no-ops.
	{
		dc := probeDC(40, 30)
		dc.ClearWithColor(render.RGBA{R: 0.2, G: 0.4, B: 0.6, A: 1})
		before := rawMiscPixel(dc.Image(), 10, 10)
		p := render.NewPath()
		p.MoveTo(5, 5)
		p.LineTo(20, 5)
		p.LineTo(20, 20)
		p.Close()
		dc.DrawShadow(nil, render.RGBA{A: 1}, 5, false)
		dc.DrawShadow(p, render.RGBA{A: 1}, 0, false)
		after := rawMiscPixel(dc.Image(), 10, 10)
		dc.Close()
		add("draw_shadow_elevation_noop", before == after, fmt.Sprintf("before=%v after=%v", before, after))
	}

	// 14. The spot color is around 25% alpha before blur; on white it must be
	// visible but never an opaque black silhouette.
	{
		restore := render.DisableGPUFilterGraphForTest()
		dc := probeDC(100, 100)
		dc.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})
		p := render.NewPath()
		p.MoveTo(20, 20)
		p.LineTo(80, 20)
		p.LineTo(80, 60)
		p.LineTo(20, 60)
		p.Close()
		dc.DrawShadow(p, render.RGBA{A: 1}, 12, true)
		px := rawMiscPixel(dc.Image(), 50, 75)
		dc.Close()
		restore()
		add("draw_shadow_spot_alpha", px[0] < 250 && px[0] > 100 && px[0] == px[1] && px[1] == px[2],
			fmt.Sprintf("center=%v", px))
	}

	return out
}

// ---------------------------------------------------------------------------
// Pixel probes over the baked window snapshot
// ---------------------------------------------------------------------------

func sampleContent(img image.Image, bx, by, dpr float64, card int, lx, ly float64) [4]int {
	if img == nil {
		return [4]int{}
	}
	x := int(math.Round((bx + cardAt[card][0] + contentX0 + lx) * dpr))
	y := int(math.Round((by + cardAt[card][1] + contentY0 + ly) * dpr))
	return rawMiscPixel(img, x, y)
}

func countInk(img image.Image, bx, by, dpr float64, card int, lx0, ly0, lx1, ly1 float64) int {
	if img == nil {
		return 0
	}
	n := 0
	for ly := ly0; ly < ly1; ly += 1 {
		for lx := lx0; lx < lx1; lx += 1 {
			p := sampleContent(img, bx, by, dpr, card, lx, ly)
			if p[3] > 0 && (p[0] < 235 || p[1] < 235 || p[2] < 235) {
				n++
			}
		}
	}
	return n
}

func runPixelProbes(img image.Image, bx, by, dpr float64) []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}
	at := func(card int, x, y float64) [4]int { return sampleContent(img, bx, by, dpr, card, x, y) }

	// A: shape, soft shadow below it, clean background above.
	shapeA := at(0, 60, 40)
	shadowA := at(0, 60, 82)
	aboveA := at(0, 400, 8)
	add("pixel_shadow_shape", shapeA[2] > 120 && shapeA[2] > shapeA[0] && shapeA[1] > shapeA[0],
		fmt.Sprintf("shape=%v", shapeA))
	add("pixel_shadow_soft", shadowA[0] < 235 && shadowA[0] > 140 &&
		nearU8(shadowA[0], shadowA[1], 8) && nearU8(shadowA[1], shadowA[2], 8),
		fmt.Sprintf("below=%v", shadowA))
	add("pixel_shadow_above", aboveA[0] > 235 && aboveA[1] > 235 && aboveA[2] > 235,
		fmt.Sprintf("above=%v", aboveA))

	// B: Multiply band, untouched middle, Screen band, rounded Normal clip.
	bm := at(1, 50, 30)
	bmid := at(1, 50, 75)
	bs := at(1, 50, 120)
	bcirc := at(1, 360, 75)
	bout := at(1, 300, 75)
	add("pixel_color_multiply", nearU8(bm[0], 229, 8) && nearU8(bm[1], 28, 8) && nearU8(bm[2], 22, 8),
		fmt.Sprintf("multiply=%v", bm))
	add("pixel_color_middle", bmid[0] > 245 && nearU8(bmid[1], 31, 8) && nearU8(bmid[2], 26, 8),
		fmt.Sprintf("middle=%v", bmid))
	add("pixel_color_screen", bs[0] > 245 && bs[1] > 70 && bs[2] > 185,
		fmt.Sprintf("screen=%v", bs))
	add("pixel_color_roundclip", bcirc[0] > 245 && bcirc[1] > 245 && bcirc[2] > 245 &&
		bout[2] > 200 && bout[0] < 80,
		fmt.Sprintf("in=%v out=%v", bcirc, bout))

	// C: gradient edges, preserved path square, outside clip.
	cl := at(2, 24, 75)
	cr := at(2, 180, 75)
	cpath := at(2, 50, 64)
	cout := at(2, 5, 75)
	add("pixel_paint_gradient", cl[0] > cr[0] && cr[2] > cl[2] && cl[2] < 180 && cr[0] < 120,
		fmt.Sprintf("left=%v right=%v", cl, cr))
	add("pixel_paint_path", cpath[1] > 180 && cpath[0] < 80 && cpath[2] < 100,
		fmt.Sprintf("path=%v", cpath))
	add("pixel_paint_outside", nearU8(cout[0], 240, 6) && nearU8(cout[1], 244, 6) && nearU8(cout[2], 249, 6),
		fmt.Sprintf("outside=%v", cout))

	// D: flat rect, nested inner, frame, rotated panel.
	drect := at(3, 50, 50)
	dnest := at(3, 150, 50)
	dframe := at(3, 50, 21)
	drot := at(3, 282, 62)
	add("pixel_bounds_rect", drect[0] > 235 && drect[1] > 110 && drect[1] < 170 && drect[2] < 70,
		fmt.Sprintf("rect=%v", drect))
	add("pixel_bounds_nested", dnest[0] > 180 && dnest[2] > 100 && dnest[1] < 90,
		fmt.Sprintf("nested=%v", dnest))
	add("pixel_bounds_frame", dframe[2] > 140 && dframe[0] < 90,
		fmt.Sprintf("frame=%v", dframe))
	add("pixel_bounds_transform", drot[1] > 140 && drot[2] > 140 && drot[0] < 90,
		fmt.Sprintf("rotated=%v", drot))

	// E: ink in both columns, no ink in the page margin.
	leftInk := countInk(img, bx, by, dpr, 4, 18, 14, 400, 120)
	rightInk := countInk(img, bx, by, dpr, 4, 448, 14, 840, 120)
	blank := at(4, 5, 8)
	add("pixel_paragraph_left", leftInk > 80, fmt.Sprintf("left ink=%d", leftInk))
	add("pixel_paragraph_right", rightInk > 40, fmt.Sprintf("right ink=%d", rightInk))
	add("pixel_paragraph_blank", blank[0] > 235 && blank[1] > 235 && blank[2] > 235,
		fmt.Sprintf("blank=%v", blank))

	return out
}
