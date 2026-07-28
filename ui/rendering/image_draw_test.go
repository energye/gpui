package rendering_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// solidImage builds an ImageBuf filled with a single RGBA color.
func solidImage(t *testing.T, w, h int, r, g, b, a uint8) *render.ImageBuf {
	t.Helper()
	img, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			_ = img.SetRGBA(x, y, r, g, b, a)
		}
	}
	return img
}

// TestDrawImageRounded_ClipsCorners drives the shipped UI DrawImageRounded
// façade (not raw pc.DC): solid red image under rounded clip must leave the
// hard-rect corner void near-background, while the interior stays red.
func TestDrawImageRounded_ClipsCorners(t *testing.T) {
	const W, H = 100, 100
	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	img := solidImage(t, 80, 80, 255, 0, 0, 255)
	defer img.Dispose()

	// Origin offset so Abs() is exercised.
	const ox, oy = 10.0, 10.0
	pc := rendering.NewPaintContext(dc, 1).WithOrigin(ox, oy)
	const rad = 20.0
	// Local (0,0) → absolute (10,10); image 80×80, radius 20.
	rendering.DrawImageRounded(pc, img, 0, 0, rad)

	out := dc.Image()
	if out == nil {
		t.Fatal("nil image")
	}

	// Interior of rrect (absolute ~50,50) must be red.
	cr, cg, cb := sampleRGB(out.At(50, 50))
	if cr < 0xC000 || cg > 0x4000 || cb > 0x4000 {
		t.Fatalf("interior (50,50)=#%04x%04x%04x want red", cr, cg, cb)
	}

	// Hard-rect top-left corner of the image bounds is outside radius-20 arc.
	// Absolute image origin (10,10); pixel (11,11) sits in the rounded void.
	// If DrawImageRounded were a plain DrawImage, this would be red.
	tr, tg, tb := sampleRGB(out.At(11, 11))
	if tg < 0xA000 || tb < 0xA000 {
		t.Fatalf("rrect corner void (11,11)=#%04x%04x%04x want near-white (not red)", tr, tg, tb)
	}

	// Outside image bounds stay white.
	or, og, ob := sampleRGB(out.At(2, 2))
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("outside (2,2)=#%04x%04x%04x want white", or, og, ob)
	}
}

// TestDrawImageNine_CornersUnscaledCenterStretches drives the shipped UI
// DrawImageNine façade with a high-contrast 3×3 color tile source.
//
// Source 30×30: corners red, edges green, center blue; center rect (10,10)-(20,20).
// Dest 90×90: nine-patch keeps top-left 10×10 red unscaled; dest (15,15) falls in
// the stretched center → blue. A naive full-image stretch would still sample the
// corner band at (15,15) → red. That distinction proves nine-patch semantics.
func TestDrawImageNine_CornersUnscaledCenterStretches(t *testing.T) {
	const srcN = 30
	img, err := render.NewImageBuf(srcN, srcN, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	defer img.Dispose()

	// Paint 3×3 color lattice: 10px bands.
	for y := 0; y < srcN; y++ {
		for x := 0; x < srcN; x++ {
			var r, g, b uint8
			col := x / 10 // 0 left, 1 mid, 2 right
			row := y / 10
			switch {
			case col == 1 && row == 1:
				b = 255 // center blue
			case col == 1 || row == 1:
				g = 255 // edges green
			default:
				r = 255 // corners red
			}
			_ = img.SetRGBA(x, y, r, g, b, 255)
		}
	}
	center := image.Rect(10, 10, 20, 20)

	const dstN = 90
	dc := render.NewContext(dstN+20, dstN+20)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	const ox, oy = 5.0, 5.0
	pc := rendering.NewPaintContext(dc, 1).WithOrigin(ox, oy)
	rendering.DrawImageNine(pc, img, center, 0, 0, dstN, dstN)

	out := dc.Image()
	if out == nil {
		t.Fatal("nil image")
	}

	// Absolute dest origin (5,5). Corner cell is unscaled 10×10 → still red near (8,8).
	cr, cg, cb := sampleRGB(out.At(8, 8))
	if cr < 0xC000 || cg > 0x4000 || cb > 0x4000 {
		t.Fatalf("nine corner (8,8)=#%04x%04x%04x want red (unscaled corner)", cr, cg, cb)
	}

	// Dest local (15,15) → absolute (20,20): past the 10px corner into stretched
	// center band → must be blue under nine-patch. Naive stretch would still be
	// in the scaled corner (src≈5,5) → red.
	br, bg, bb := sampleRGB(out.At(20, 20))
	if bb < 0xC000 || br > 0x4000 || bg > 0x4000 {
		t.Fatalf("nine center-ish (20,20)=#%04x%04x%04x want blue (stretched center, not red corner)", br, bg, bb)
	}

	// Far center of dest (absolute 5+45,5+45)=(50,50) also blue.
	mr, mg, mb := sampleRGB(out.At(50, 50))
	if mb < 0xC000 || mr > 0x4000 || mg > 0x4000 {
		t.Fatalf("nine mid (50,50)=#%04x%04x%04x want blue", mr, mg, mb)
	}

	// Contrast: plain stretch of the same source to the same dest — (20,20) red.
	dc2 := render.NewContext(dstN+20, dstN+20)
	defer dc2.Close()
	dc2.BeginFrame()
	dc2.ClearWithColor(render.White)
	pc2 := rendering.NewPaintContext(dc2, 1).WithOrigin(ox, oy)
	rendering.DrawImageBuf(pc2, img, 0, 0, dstN, dstN)
	out2 := dc2.Image()
	sr, sg, sb := sampleRGB(out2.At(20, 20))
	if sr < 0xA000 {
		t.Fatalf("control stretch (20,20)=#%04x%04x%04x expected red-ish corner sample (proves nine≠stretch)", sr, sg, sb)
	}
	// nine at same pixel was blue; stretch is red — distinct.
	if sb > 0x8000 && sr < 0x4000 {
		t.Fatal("control stretch unexpectedly blue; fixture invalid")
	}
}

// TestDrawImageBuf_Basic keeps the public 1:1 / scaled path green (used by RO).
func TestDrawImageBuf_Basic(t *testing.T) {
	dc := render.NewContext(60, 60)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	img := solidImage(t, 20, 20, 0, 200, 0, 255)
	defer img.Dispose()

	pc := rendering.NewPaintContext(dc, 1).WithOrigin(5, 5)
	rendering.DrawImageBuf(pc, img, 10, 10, 0, 0) // 1:1 at abs (15,15)

	out := dc.Image()
	gr, gg, gb := sampleRGB(out.At(16, 16))
	if gg < 0xA000 {
		t.Fatalf("DrawImageBuf 1:1 (16,16)=#%04x%04x%04x want green", gr, gg, gb)
	}
}

// TestDrawImageCircular_ClipsOutsideCircle: red image under circular clip leaves
// hard-rect corner void near-white (not red).
func TestDrawImageCircular_ClipsOutsideCircle(t *testing.T) {
	dc := render.NewContext(100, 100)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	img := solidImage(t, 60, 60, 255, 0, 0, 255)
	defer img.Dispose()

	pc := rendering.NewPaintContext(dc, 1).WithOrigin(10, 10)
	// Local center (30,30) → abs (40,40), radius 25.
	rendering.DrawImageCircular(pc, img, 30, 30, 25)

	out := dc.Image()
	cr, cg, cb := sampleRGB(out.At(40, 40))
	if cr < 0xC000 || cg > 0x4000 {
		t.Fatalf("circle center (40,40)=#%04x%04x%04x want red", cr, cg, cb)
	}
	// Corner of bounding box of circle (abs 40-25=15) is outside the disk.
	tr, tg, tb := sampleRGB(out.At(16, 16))
	if tg < 0xA000 || tb < 0xA000 {
		t.Fatalf("outside circle (16,16)=#%04x%04x%04x want near-white", tr, tg, tb)
	}
}

// TestDrawImageRect_SrcCrop: left half of a red|blue split image stretched to
// dest must sample red only (not blue from the right half).
func TestDrawImageRect_SrcCrop(t *testing.T) {
	img, err := render.NewImageBuf(40, 20, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Dispose()
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			if x < 20 {
				_ = img.SetRGBA(x, y, 255, 0, 0, 255)
			} else {
				_ = img.SetRGBA(x, y, 0, 0, 255, 255)
			}
		}
	}

	dc := render.NewContext(80, 60)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1).WithOrigin(5, 5)
	// Crop left half (red) into a 40×40 dest at local (10,10) → abs (15,15).
	src := image.Rect(0, 0, 20, 20)
	rendering.DrawImageRect(pc, img, src, 10, 10, 40, 40)

	out := dc.Image()
	// Center of dest abs (15+20, 15+20)=(35,35) must be red, not blue.
	rr, rg, rb := sampleRGB(out.At(35, 35))
	if rr < 0xC000 || rb > 0x4000 {
		t.Fatalf("SrcRect crop center (35,35)=#%04x%04x%04x want red (not blue right half)", rr, rg, rb)
	}
}

// TestDrawAtlas_TwoSprites: atlas with red left / green right tiles drawn to
// two destinations via DrawAtlas.
func TestDrawAtlas_TwoSprites(t *testing.T) {
	img, err := render.NewImageBuf(20, 10, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Dispose()
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			if x < 10 {
				_ = img.SetRGBA(x, y, 255, 0, 0, 255)
			} else {
				_ = img.SetRGBA(x, y, 0, 255, 0, 255)
			}
		}
	}

	dc := render.NewContext(80, 40)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1).WithOrigin(4, 4)
	sprites := []rendering.AtlasSprite{
		{SrcX: 0, SrcY: 0, SrcW: 10, SrcH: 10, DstX: 0, DstY: 0, DstW: 20, DstH: 20, Opacity: 1},
		{SrcX: 10, SrcY: 0, SrcW: 10, SrcH: 10, DstX: 30, DstY: 0, DstW: 20, DstH: 20, Opacity: 1},
	}
	rendering.DrawAtlas(pc, img, sprites)

	out := dc.Image()
	// First sprite abs center ~ (4+10, 4+10)=(14,14) red.
	rr, rg, rb := sampleRGB(out.At(14, 14))
	if rr < 0xC000 || rg > 0x4000 {
		t.Fatalf("atlas sprite0 (14,14)=#%04x%04x%04x want red", rr, rg, rb)
	}
	// Second sprite abs center ~ (4+30+10, 4+10)=(44,14) green.
	gr, gg, gb := sampleRGB(out.At(44, 14))
	if gg < 0xA000 || gr > 0x4000 {
		t.Fatalf("atlas sprite1 (44,14)=#%04x%04x%04x want green", gr, gg, gb)
	}
}
