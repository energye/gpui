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

// withPaintFrozen runs fn and restores the ambient paint top-level state
// afterwards (color/brush/pattern/shader/blend mode & space). The repo's
// Push/Pop deliberately does NOT save paint (catalog §8 「Context.Pop 不还原
// 笔刷」), so one-shot compositing states inside convenience entries must be
// frozen explicitly — 深审 D6 (L1-P1-3/L2-P3-9): DrawImageEx/DrawShaderMask
// used to leak BlendMode/BlendSpace/Shader to the caller.
func (c *Context) withPaintFrozen(fn func()) {
	if c == nil || c.paint == nil {
		fn()
		return
	}
	saved := *c.paint
	defer func() { *c.paint = saved }()
	fn()
}

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
	// Live Context: freeze the whole ambient paint across the mask sublayer
	// and detach any ambient Shader — otherwise the callback's fills and the
	// dstIn mask fill itself get hijacked by the shader (深审 D6 / L2-P3-9).
	if ctx, ok := c.(*Context); ok {
		ctx.withPaintFrozen(func() {
			if ctx.paint != nil {
				ctx.paint.Shader = nil
			}
			drawShaderMaskBody(c, x, y, w, h, mask, draw)
		})
		return
	}
	drawShaderMaskBody(c, x, y, w, h, mask, draw)
}

// drawShaderMaskBody is the Flutter ShaderMask saveLayer+dstIn form. For the
// live Context the caller additionally froze the paint; the explicit
// brush/blend/space restore below is what the recording Canvas (Recorder)
// replays as real state commands.
func drawShaderMaskBody(c Canvas, x, y, w, h float64, mask Brush, draw func()) {
	// Capture the caller's paint before draw() can change it: the callback is
	// free to set its own brush/blend, and neither may leak past the mask.
	prevBrush, prevBlend, prevSpace := c.FillBrush(), c.BlendMode(), c.BlendSpace()

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
	c.SetBlendSpace(prevSpace)

	c.Pop()
	c.PopLayer()
	c.Pop()
}
