package rendering_test

import (
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

func sampleRGB(c color.Color) (r, g, b uint32) {
	rr, gg, bb, _ := c.RGBA()
	return rr, gg, bb
}

// TestPaintContext_PushClipRRect_ClipsOutsideCorners drives the shipped
// PaintContext.PushClipRRect / PopClip path: content drawn under a rounded
// clip must not paint the hard-rect corner (outside the rrect arc), and Pop
// must restore so later draws are unclipped.
func TestPaintContext_PushClipRRect_ClipsOutsideCorners(t *testing.T) {
	const W, H = 120, 120
	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	// Origin offset so Abs() is exercised (not only 0,0).
	const ox, oy = 10.0, 10.0
	pc := rendering.NewPaintContext(dc, 1).WithOrigin(ox, oy)

	// Local rrect: (10,10)-(90,90) → absolute (20,20)-(100,100), radius 20.
	const lx, ly, lw, lh, rad = 10.0, 10.0, 80.0, 80.0, 20.0
	pc.PushClipRRect(lx, ly, lw, lh, rad)

	// Paint a solid red full local canvas; only the rrect interior may show.
	dc.SetRGBA(1, 0, 0, 1)
	ax0, ay0 := pc.Abs(0, 0)
	dc.DrawRectangle(ax0, ay0, 200, 200)
	_ = dc.Fill()

	pc.PopClip()

	img := dc.Image()
	if img == nil {
		t.Fatal("nil image")
	}

	// Center of rrect (absolute 60,60) must be red.
	cr, cg, cb := sampleRGB(img.At(60, 60))
	if cr < 0xC000 || cg > 0x4000 || cb > 0x4000 {
		t.Fatalf("center (60,60)=#%04x%04x%04x want red (clip interior)", cr, cg, cb)
	}

	// Hard-rect top-left corner of the clip bounds is outside a radius-20 rrect.
	// Absolute clip origin is (20,20); pixel (21,21) sits in the rounded corner void.
	// If PushClipRRect were a no-op or only a hard rect, this would be red (high R, low G/B).
	tr, tg, tb := sampleRGB(img.At(21, 21))
	if tg < 0xA000 || tb < 0xA000 {
		t.Fatalf("rrect corner void (21,21)=#%04x%04x%04x want near-white (not red fill)", tr, tg, tb)
	}

	// Far outside absolute clip bounds must stay white.
	or, og, ob := sampleRGB(img.At(5, 5))
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("outside (5,5)=#%04x%04x%04x want white", or, og, ob)
	}

	// After PopClip, drawing at the former corner void must paint (stack restored).
	dc.SetRGBA(0, 0, 1, 1)
	dc.DrawRectangle(20, 20, 4, 4)
	_ = dc.Fill()
	img = dc.Image()
	br, bg, bb := sampleRGB(img.At(21, 21))
	if bb < 0xC000 || br > 0x4000 || bg > 0x4000 {
		t.Fatalf("after PopClip (21,21)=#%04x%04x%04x want blue (stack restored)", br, bg, bb)
	}
}

// TestPaintContext_PushClipRRect_ZeroRadiusFallsBackToRect ensures radius<=0
// still clips to the hard rect via the same UI entry point.
func TestPaintContext_PushClipRRect_ZeroRadiusFallsBackToRect(t *testing.T) {
	dc := render.NewContext(80, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1)
	pc.PushClipRRect(20, 20, 40, 40, 0)

	dc.SetRGBA(0, 0.8, 0, 1)
	dc.DrawRectangle(0, 0, 80, 80)
	_ = dc.Fill()
	pc.PopClip()

	img := dc.Image()
	gr, gg, gb := sampleRGB(img.At(40, 40))
	if gg < 0xA000 {
		t.Fatalf("inside rect clip (40,40)=#%04x%04x%04x want green", gr, gg, gb)
	}
	or, og, ob := sampleRGB(img.At(5, 5))
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("outside (5,5)=#%04x%04x%04x want white", or, og, ob)
	}
}

// TestPaintContext_PushClipRect_StillWorks keeps rect clip exported path green.
func TestPaintContext_PushClipRect_StillWorks(t *testing.T) {
	dc := render.NewContext(60, 60)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1).WithOrigin(5, 5)
	pc.PushClipRect(5, 5, 30, 30) // abs 10..40
	dc.SetRGBA(1, 0, 1, 1)
	ax, ay := pc.Abs(0, 0)
	dc.DrawRectangle(ax, ay, 100, 100)
	_ = dc.Fill()
	pc.PopClip()

	img := dc.Image()
	pr, pg, pb := sampleRGB(img.At(20, 20))
	if pr < 0xA000 || pb < 0xA000 {
		t.Fatalf("inside (20,20)=#%04x%04x%04x want magenta", pr, pg, pb)
	}
	or, og, ob := sampleRGB(img.At(2, 2))
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("outside (2,2)=#%04x%04x%04x want white", or, og, ob)
	}
}
