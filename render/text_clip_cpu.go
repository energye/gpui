package render

import (
	"image"
	"image/draw"
	"math"

	"github.com/energye/gpui/render/text"
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
// The scratch is normally limited to the run's glyph bounds instead of the
// full clip rectangle. If ink touches a scratch edge that is not a clip edge,
// the draw is repeated against the full clip so a glyph overhang can never be
// lost by the tighter allocation.
//
// Returns false when no clip is active; callers then keep the direct path.
// Returns true when it handled (or safely skipped) the draw.
func (c *Context) tryDrawTextWithClip(s string, face text.Face, x, y float64, drawFn func(dst draw.Image, x, y float64)) bool {
	if c == nil || c.pixmap == nil || face == nil || drawFn == nil || !c.isClipActive() {
		return false
	}
	clipBounds, ok := c.clipDeviceBounds()
	if !ok {
		return false
	}
	canvas := image.Rect(0, 0, c.pixmap.Width(), c.pixmap.Height())
	clipBounds = clipBounds.Intersect(canvas)
	if clipBounds.Empty() {
		return true
	}

	hint := textRunBounds(s, face, x, y).Intersect(clipBounds)
	if hint.Empty() {
		return true
	}
	if c.drawTextClipScratch(hint, clipBounds, x, y, drawFn) {
		return true
	}
	if hint != clipBounds {
		// A glyph crossed the run-bounds edge. Re-render with the full clip
		// instead of compositing the clipped first attempt twice.
		c.drawTextClipScratch(clipBounds, clipBounds, x, y, drawFn)
	}
	return true
}

// drawTextClipScratch renders one text run into a scratch image, checks that
// ink did not cross a free scratch edge, then composites it through the active
// clip coverage. It returns false only when the scratch was too tight.
func (c *Context) drawTextClipScratch(bounds, clipBounds image.Rectangle, x, y float64, drawFn func(dst draw.Image, x, y float64)) bool {
	if bounds.Empty() {
		return true
	}
	scratch := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	drawFn(scratch, x-float64(bounds.Min.X), y-float64(bounds.Min.Y))

	if hasInkOnFreeEdge(scratch, bounds, clipBounds) {
		return false
	}

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

// textRunBounds returns a conservative device-space rectangle for a text run
// at baseline (x, y). Glyph outline bounds and metrics are unioned, then
// padded for antialiasing, hinting drift, synthetic embolden, and bitmap emoji.
func textRunBounds(s string, face text.Face, x, y float64) image.Rectangle {
	if face == nil {
		return image.Rect(int(math.Floor(x)), int(math.Floor(y)), int(math.Ceil(x)), int(math.Ceil(y)))
	}
	metrics := face.Metrics()
	lineHeight := metrics.Ascent + metrics.Descent
	if lineHeight <= 0 {
		lineHeight = face.Size()
	}
	if lineHeight <= 0 {
		lineHeight = 16
	}
	pad := lineHeight*0.5 + 4

	minX := x - pad
	maxX := x + face.Advance(s) + pad
	minY := y - metrics.Ascent - pad
	maxY := y + metrics.Descent + pad

	glyphCount := 0
	for g := range face.Glyphs(s) {
		glyphCount++
		gx, gy := x+g.X, y+g.Y
		if gx+g.Advance > maxX {
			maxX = gx + g.Advance
		}
		if g.Bounds.Empty() {
			continue
		}
		if gx+g.Bounds.MinX < minX {
			minX = gx + g.Bounds.MinX
		}
		if gx+g.Bounds.MaxX > maxX {
			maxX = gx + g.Bounds.MaxX
		}
		if gy+g.Bounds.MinY < minY {
			minY = gy + g.Bounds.MinY
		}
		if gy+g.Bounds.MaxY > maxY {
			maxY = gy + g.Bounds.MaxY
		}
	}

	// Hinted advances may drift from the unhinted positions used above. One
	// pixel per glyph is a safe upper bound for that drift.
	drift := float64(glyphCount)
	minX -= drift
	maxX += drift
	if maxX <= minX {
		maxX = minX + 1
	}
	if maxY <= minY {
		maxY = minY + 1
	}

	return image.Rect(
		int(math.Floor(minX))-1,
		int(math.Floor(minY))-1,
		int(math.Ceil(maxX))+1,
		int(math.Ceil(maxY))+1,
	)
}

// hasInkOnFreeEdge reports whether visible text reached a scratch edge that is
// not also an edge of the active clip. Such ink means the run-bounds hint may
// have cut off a glyph; the caller then falls back to the full clip.
func hasInkOnFreeEdge(scratch *image.RGBA, bounds, clipBounds image.Rectangle) bool {
	if scratch == nil || scratch.Rect.Empty() {
		return false
	}
	checkLeft := bounds.Min.X > clipBounds.Min.X
	checkTop := bounds.Min.Y > clipBounds.Min.Y
	checkRight := bounds.Max.X < clipBounds.Max.X
	checkBottom := bounds.Max.Y < clipBounds.Max.Y
	if !checkLeft && !checkTop && !checkRight && !checkBottom {
		return false
	}
	w, h := scratch.Rect.Dx(), scratch.Rect.Dy()
	alphaAt := func(x, y int) uint8 {
		return scratch.Pix[y*scratch.Stride+x*4+3]
	}
	if checkLeft {
		for y := 0; y < h; y++ {
			if alphaAt(0, y) != 0 {
				return true
			}
		}
	}
	if checkRight {
		for y := 0; y < h; y++ {
			if alphaAt(w-1, y) != 0 {
				return true
			}
		}
	}
	if checkTop {
		for x := 0; x < w; x++ {
			if alphaAt(x, 0) != 0 {
				return true
			}
		}
	}
	if checkBottom {
		for x := 0; x < w; x++ {
			if alphaAt(x, h-1) != 0 {
				return true
			}
		}
	}
	return false
}
