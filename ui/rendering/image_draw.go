package rendering

import (
	"image"

	"github.com/energye/gpui/render"
)

// Image draw façades on PaintContext (Flutter Canvas drawImage* subset).
// Coordinates are logical, origin-relative (Y-down). render executes raster.

// DrawImageBuf draws img at local (x,y). If dstW and dstH are both >0 the image
// is scaled into that destination (DrawImageEx); otherwise it is drawn 1:1
// (DrawImage). Skips disposed or nil buffers (FImg-DRAW / FC-DRAW-IMAGE).
func DrawImageBuf(pc *PaintContext, img *render.ImageBuf, x, y, dstW, dstH float64) {
	if pc == nil || pc.DC == nil || img == nil || img.Disposed() {
		return
	}
	ax, ay := pc.Abs(x, y)
	if dstW <= 0 || dstH <= 0 {
		pc.DC.DrawImage(img, ax, ay)
		return
	}
	pc.DC.DrawImageEx(img, render.DrawImageOptions{
		X: ax, Y: ay, DstWidth: dstW, DstHeight: dstH,
	})
}

// DrawImageRounded draws img at its natural size with a uniform rounded-rect
// clip (FImg-ROUND / FC-DRAW-IMAGE path). radius<=0 draws without rounding
// (hard rect). Origin-aware like DrawImageBuf.
func DrawImageRounded(pc *PaintContext, img *render.ImageBuf, x, y, radius float64) {
	if pc == nil || pc.DC == nil || img == nil || img.Disposed() {
		return
	}
	ax, ay := pc.Abs(x, y)
	if radius <= 0 {
		pc.DC.DrawImage(img, ax, ay)
		return
	}
	pc.DC.DrawImageRounded(img, ax, ay, radius)
}

// DrawImageNine draws a nine-patch (lattice) image into a destination rect
// (FImg-NINE / FC-DRAW-IMAGE-NINE). center is the stretchable center rectangle
// in source image pixel coordinates. Corners stay unscaled; edges stretch on
// one axis; center stretches on both. Origin-aware.
func DrawImageNine(pc *PaintContext, img *render.ImageBuf, center image.Rectangle, x, y, dstW, dstH float64) {
	if pc == nil || pc.DC == nil || img == nil || img.Disposed() || dstW <= 0 || dstH <= 0 {
		return
	}
	ax, ay := pc.Abs(x, y)
	pc.DC.DrawImageNine(img, center, ax, ay, dstW, dstH)
}
