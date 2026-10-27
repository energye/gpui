//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

// DrawShaderMask draws content into an isolated layer and modulates it by the
// alpha of mask, the Flutter ShaderMask form (Skia saveLayer + a dstIn rect).
// Content outside the bounds is clipped, matching saveLayer bounds. The canvas'
// brush, blend, AA, clip and CTM are restored before returning.
//
// mask may be nil, in which case content is drawn unmasked. A nil canvas, nil
// draw func or non-positive extent is a no-op.
func DrawShaderMask(c Canvas, x, y, w, h float64, mask Brush, draw func()) {
	if c == nil || draw == nil || w <= 0 || h <= 0 {
		return
	}
	if mask == nil {
		draw()
		return
	}
	// Capture the caller's paint before draw() can change it: the callback is
	// free to set its own brush/blend, and neither may leak past the mask.
	prevBrush, prevBlend := c.FillBrush(), c.BlendMode()

	c.Push()
	c.PushLayerIsolated(1)
	c.Push()
	c.ClipRect(x, y, w, h)
	draw()

	c.ClearPath()
	c.SetFillBrush(mask)
	c.SetBlendMode(BlendDestinationIn)
	// fillNoAA ignores the blend mode, so the mask fill must stay anti-aliased.
	if !c.AntiAlias() {
		c.SetAntiAlias(true)
	}
	c.DrawRectangle(x, y, w, h)
	_ = c.Fill()
	c.SetFillBrush(prevBrush)
	c.SetBlendMode(prevBlend)

	c.Pop()
	c.PopLayer()
	c.Pop()
}
