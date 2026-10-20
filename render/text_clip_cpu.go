package render

import (
	"image"
	"image/draw"
)

// tryDrawTextWithClip applies the active clip to CPU bitmap/aliased text.
//
// The glyph-mask and MSDF GPU paths already receive the clip through
// setGPUClipRect and the shader. The CPU bitmap paths historically called
// text.Draw directly on c.pixmap, which bypassed both the scissor and the
// analytic RRect/path clip. Render the run into a transparent scratch image
// and composite it through the same ClipStack coverage used by the CPU shape
// fillers, so text and vector content share one clip semantic.
//
// Returns false when no clip is active; callers then keep the direct path.
// Returns true when it handled (or safely skipped) the draw.
func (c *Context) tryDrawTextWithClip(x, y float64, drawFn func(dst draw.Image, x, y float64)) bool {
	if c == nil || c.pixmap == nil || drawFn == nil || !c.isClipActive() {
		return false
	}
	bounds, ok := c.clipDeviceBounds()
	if !ok {
		return false
	}
	canvas := image.Rect(0, 0, c.pixmap.Width(), c.pixmap.Height())
	bounds = bounds.Intersect(canvas)
	if bounds.Empty() {
		return true
	}

	scratch := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	drawFn(scratch, x-float64(bounds.Min.X), y-float64(bounds.Min.Y))

	mask := image.NewAlpha(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for py := 0; py < bounds.Dy(); py++ {
		for px := 0; px < bounds.Dx(); px++ {
			cov := c.clipStack.Coverage(float64(bounds.Min.X+px)+0.5, float64(bounds.Min.Y+py)+0.5)
			mask.Pix[py*mask.Stride+px] = cov
		}
	}
	draw.DrawMask(c.pixmap, bounds, scratch, image.Point{}, mask, image.Point{}, draw.Over)
	return true
}
