package render

import "image"

// DrawImageNine draws a nine-patch (lattice) image into a destination rect (I.07).
// center is the stretchable center rectangle in source image pixel coordinates.
// Corners are drawn unscaled; edges stretch along one axis; center stretches both.
func (c *Context) DrawImageNine(img *ImageBuf, center image.Rectangle, dstX, dstY, dstW, dstH float64) {
	if c == nil || img == nil || dstW <= 0 || dstH <= 0 {
		return
	}
	imgW, imgH := img.Bounds()
	if imgW <= 0 || imgH <= 0 {
		return
	}
	// Clamp center to image.
	if center.Min.X < 0 {
		center.Min.X = 0
	}
	if center.Min.Y < 0 {
		center.Min.Y = 0
	}
	if center.Max.X > imgW {
		center.Max.X = imgW
	}
	if center.Max.Y > imgH {
		center.Max.Y = imgH
	}
	if center.Dx() <= 0 || center.Dy() <= 0 {
		c.DrawImageEx(img, DrawImageOptions{X: dstX, Y: dstY, DstWidth: dstW, DstHeight: dstH, Opacity: 1})
		return
	}

	left := float64(center.Min.X)
	top := float64(center.Min.Y)
	right := float64(imgW - center.Max.X)
	bottom := float64(imgH - center.Max.Y)
	centerW := float64(center.Dx())
	centerH := float64(center.Dy())

	// Destination edge sizes (corners keep source size; center takes remainder).
	dstLeft := left
	dstTop := top
	dstRight := right
	dstBottom := bottom
	if dstW < dstLeft+dstRight {
		scale := dstW / (dstLeft + dstRight)
		dstLeft *= scale
		dstRight *= scale
	}
	if dstH < dstTop+dstBottom {
		scale := dstH / (dstTop + dstBottom)
		dstTop *= scale
		dstBottom *= scale
	}
	dstCenterW := dstW - dstLeft - dstRight
	dstCenterH := dstH - dstTop - dstBottom
	if dstCenterW < 0 {
		dstCenterW = 0
	}
	if dstCenterH < 0 {
		dstCenterH = 0
	}

	type cell struct {
		sx, sy, sw, sh float64
		dx, dy, dw, dh float64
	}
	// Source x bands: [0,left), [left,left+centerW), [left+centerW, imgW)
	sx0, sx1, sx2 := 0.0, left, left+centerW
	sw0, sw1, sw2 := left, centerW, right
	sy0, sy1, sy2 := 0.0, top, top+centerH
	sh0, sh1, sh2 := top, centerH, bottom

	dx0, dx1, dx2 := dstX, dstX+dstLeft, dstX+dstLeft+dstCenterW
	dw0, dw1, dw2 := dstLeft, dstCenterW, dstRight
	dy0, dy1, dy2 := dstY, dstY+dstTop, dstY+dstTop+dstCenterH
	dh0, dh1, dh2 := dstTop, dstCenterH, dstBottom

	cells := []cell{
		// top row
		{sx0, sy0, sw0, sh0, dx0, dy0, dw0, dh0},
		{sx1, sy0, sw1, sh0, dx1, dy0, dw1, dh0},
		{sx2, sy0, sw2, sh0, dx2, dy0, dw2, dh0},
		// middle
		{sx0, sy1, sw0, sh1, dx0, dy1, dw0, dh1},
		{sx1, sy1, sw1, sh1, dx1, dy1, dw1, dh1},
		{sx2, sy1, sw2, sh1, dx2, dy1, dw2, dh1},
		// bottom
		{sx0, sy2, sw0, sh2, dx0, dy2, dw0, dh2},
		{sx1, sy2, sw1, sh2, dx1, dy2, dw1, dh2},
		{sx2, sy2, sw2, sh2, dx2, dy2, dw2, dh2},
	}

	sprites := make([]AtlasSprite, 0, 9)
	for _, ce := range cells {
		if ce.sw <= 0 || ce.sh <= 0 || ce.dw <= 0 || ce.dh <= 0 {
			continue
		}
		sprites = append(sprites, AtlasSprite{
			SrcX: ce.sx, SrcY: ce.sy, SrcW: ce.sw, SrcH: ce.sh,
			DstX: ce.dx, DstY: ce.dy, DstW: ce.dw, DstH: ce.dh,
			Opacity: 1,
		})
	}
	if len(sprites) == 0 {
		return
	}
	c.DrawAtlas(img, sprites)
}
