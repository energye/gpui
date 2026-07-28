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

// DrawImageCircular draws img centered at local (cx,cy), clipped to a circle of
// the given radius and scaled to the circle diameter (FImg-CIRCLE).
// radius<=0 is a no-op. Origin-aware.
func DrawImageCircular(pc *PaintContext, img *render.ImageBuf, cx, cy, radius float64) {
	if pc == nil || pc.DC == nil || img == nil || img.Disposed() || radius <= 0 {
		return
	}
	acx, acy := pc.Abs(cx, cy)
	pc.DC.DrawImageCircular(img, acx, acy, radius)
}

// DrawImageRect draws a source rectangle of img into a destination rect in local
// coordinates (FImg-RECT / FC-DRAW-IMAGE-RECT). src is in image pixel space;
// if empty, the full image is used. dstW/dstH must be >0.
func DrawImageRect(pc *PaintContext, img *render.ImageBuf, src image.Rectangle, x, y, dstW, dstH float64) {
	if pc == nil || pc.DC == nil || img == nil || img.Disposed() || dstW <= 0 || dstH <= 0 {
		return
	}
	ax, ay := pc.Abs(x, y)
	opts := render.DrawImageOptions{
		X: ax, Y: ay, DstWidth: dstW, DstHeight: dstH,
		Opacity: 1,
	}
	if src.Dx() > 0 && src.Dy() > 0 {
		// Copy so caller can reuse/mutate their rect later safely.
		r := src
		opts.SrcRect = &r
	}
	pc.DC.DrawImageEx(img, opts)
}

// AtlasSprite is a thin alias of render.AtlasSprite for UI call sites
// (FImg-ATLAS / FC-DRAW-ATLAS). Src* are image pixels; Dst* are local logical.
type AtlasSprite = render.AtlasSprite

// DrawAtlas draws multiple sprites from one atlas image (FImg-ATLAS).
// Each sprite's DstX/DstY are origin-relative local coordinates (shifted by Abs).
// Src* stay in image pixel space. Empty sprites slice is a no-op.
func DrawAtlas(pc *PaintContext, img *render.ImageBuf, sprites []AtlasSprite) {
	if pc == nil || pc.DC == nil || img == nil || img.Disposed() || len(sprites) == 0 {
		return
	}
	ox, oy := pc.OriginX, pc.OriginY
	if ox == 0 && oy == 0 {
		pc.DC.DrawAtlas(img, sprites)
		return
	}
	// Shift destinations into absolute space without mutating caller's slice.
	abs := make([]render.AtlasSprite, len(sprites))
	copy(abs, sprites)
	for i := range abs {
		abs[i].DstX += ox
		abs[i].DstY += oy
	}
	pc.DC.DrawAtlas(img, abs)
}
