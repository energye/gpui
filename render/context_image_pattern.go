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

import ()

// CreateImagePattern creates an image pattern from a rectangular region of an image.
// The pattern can be used with SetFillPattern or SetStrokePattern.
//
// Example:
//
//	img, _ := render.LoadImage("texture.png")
//	pattern := dc.CreateImagePattern(img, 0, 0, 100, 100)
//	dc.SetFillPattern(pattern)
//	dc.DrawRectangle(0, 0, 400, 300)
//	dc.Fill()
func (c *Context) CreateImagePattern(img *ImageBuf, x, y, w, h int) Pattern {
	return &ImagePattern{
		image:   img,
		x:       x,
		y:       y,
		w:       w,
		h:       h,
		inverse: Identity(), // identity transform: device coords = image coords (tiling from origin)
	}
}

// SetFillPattern sets the fill pattern.
// It also updates the Brush field for consistency with ColorAt precedence.
// For solid patterns, stores the color inline (zero allocations).
// SetFillPattern sets the fill pattern.
// It also updates the Brush field for consistency with ColorAt precedence.
// For solid patterns, stores the color inline (zero allocations).
func (c *Context) SetFillPattern(pattern Pattern) {
	if sp, ok := pattern.(*SolidPattern); ok {
		c.paint.solidColor = sp.Color
		c.paint.isSolid = true
		c.paint.Brush = nil
		c.paint.Pattern = nil
		return
	}
	c.paint.Pattern = pattern
	c.paint.Brush = BrushFromPattern(pattern)
	c.paint.isSolid = false
}

// SetStrokePattern sets the stroke pattern.
// It also updates the Brush field for consistency with ColorAt precedence.
// For solid patterns, stores the color inline (zero allocations).
// SetStrokePattern sets the stroke pattern.
// It also updates the Brush field for consistency with ColorAt precedence.
// For solid patterns, stores the color inline (zero allocations).
func (c *Context) SetStrokePattern(pattern Pattern) {
	if sp, ok := pattern.(*SolidPattern); ok {
		c.paint.solidColor = sp.Color
		c.paint.isSolid = true
		c.paint.Brush = nil
		c.paint.Pattern = nil
		return
	}
	c.paint.Pattern = pattern
	c.paint.Brush = BrushFromPattern(pattern)
	c.paint.isSolid = false
}

// ImagePattern represents an image-based pattern with full affine transform support.
// The pattern stores a pre-computed inverse matrix that maps device-space coordinates
// back to image-space for sampling. This enables correct rendering under rotation,
// scale, skew, and any combination of affine transforms.
//
// For backward compatibility, SetAnchor/SetScale convenience methods rebuild
// the inverse from anchor+scale parameters. For full affine control, the inverse
// is set directly by DrawImageEx or via SetTransform.
type ImagePattern struct {
	image   *ImageBuf
	x, y    int     // source region offset within the image
	w, h    int     // source region size (0 = full image dimension)
	inverse Matrix  // maps device-space -> image-space (pre-computed)
	opacity float64 // opacity multiplier (0=transparent, 1=opaque; 0 means default=1)
	clamp   bool    // when true, out-of-bounds returns transparent instead of tiling

	// Legacy fields for SetAnchor/SetScale backward compatibility.
	// When these are used, rebuildInverse() computes the inverse from them.
	anchorX float64
	anchorY float64
	scaleX  float64 // horizontal scale factor (0 means 1.0)
	scaleY  float64 // vertical scale factor (0 means 1.0)
}

// SetAnchor sets the canvas position where the pattern is anchored.
// This offsets all coordinate lookups so the image appears at (x, y)
// on the canvas rather than tiled from the origin.
// The inverse transform is rebuilt to reflect the new anchor.
// SetAnchor sets the canvas position where the pattern is anchored.
// This offsets all coordinate lookups so the image appears at (x, y)
// on the canvas rather than tiled from the origin.
// The inverse transform is rebuilt to reflect the new anchor.
func (p *ImagePattern) SetAnchor(x, y float64) {
	p.anchorX = x
	p.anchorY = y
	p.rebuildInverse()
}

// SetOpacity sets the opacity multiplier for the pattern (0.0 to 1.0).
// SetOpacity sets the opacity multiplier for the pattern (0.0 to 1.0).
func (p *ImagePattern) SetOpacity(opacity float64) {
	p.opacity = opacity
}

// SetClamp enables clamp mode. When true, coordinates outside the image region
// return transparent black instead of tiling/wrapping.
// SetClamp enables clamp mode. When true, coordinates outside the image region
// return transparent black instead of tiling/wrapping.
func (p *ImagePattern) SetClamp(clamp bool) {
	p.clamp = clamp
}

// SetScale sets the scale factors for the pattern. The source image is scaled
// by these factors before being sampled. For example, SetScale(2, 2) makes
// each source pixel cover 2x2 destination pixels.
// The inverse transform is rebuilt to reflect the new scale.
// SetScale sets the scale factors for the pattern. The source image is scaled
// by these factors before being sampled. For example, SetScale(2, 2) makes
// each source pixel cover 2x2 destination pixels.
// The inverse transform is rebuilt to reflect the new scale.
func (p *ImagePattern) SetScale(sx, sy float64) {
	p.scaleX = sx
	p.scaleY = sy
	p.rebuildInverse()
}

// SetTransform sets the full forward transform (image-space to device-space)
// for the pattern. The inverse is computed and cached for sampling.
// This overrides any anchor/scale settings.
// SetTransform sets the full forward transform (image-space to device-space)
// for the pattern. The inverse is computed and cached for sampling.
// This overrides any anchor/scale settings.
func (p *ImagePattern) SetTransform(m Matrix) {
	p.inverse = m.Invert()
}

// GPUPatternSource exposes image pattern fields for GPU-native fill (G.03).
// Returns nil image when the pattern is empty/invalid.
// GPUPatternSource exposes image pattern fields for GPU-native fill (G.03).
// Returns nil image when the pattern is empty/invalid.
func (p *ImagePattern) GPUPatternSource() (img *ImageBuf, srcX, srcY, srcW, srcH int, inverse Matrix, opacity float64, clamp bool) {
	if p == nil || p.image == nil {
		return nil, 0, 0, 0, 0, Identity(), 1, false
	}
	imgW, imgH := p.image.Bounds()
	srcW, srcH = p.w, p.h
	if srcW <= 0 {
		srcW = imgW
	}
	if srcH <= 0 {
		srcH = imgH
	}
	op := p.opacity
	if op <= 0 {
		op = 1
	}
	return p.image, p.x, p.y, srcW, srcH, p.inverse, op, p.clamp
}

// rebuildInverse computes the inverse transform from the legacy anchor+scale fields.
// The forward transform is: Translate(anchorX, anchorY) * Scale(scaleX, scaleY),
// mapping image coordinates to device coordinates.
// rebuildInverse computes the inverse transform from the legacy anchor+scale fields.
// The forward transform is: Translate(anchorX, anchorY) * Scale(scaleX, scaleY),
// mapping image coordinates to device coordinates.
func (p *ImagePattern) rebuildInverse() {
	sx := p.scaleX
	if sx == 0 {
		sx = 1
	}
	sy := p.scaleY
	if sy == 0 {
		sy = 1
	}
	// Forward: device = Translate(anchor) * Scale(s) * imageCoord
	// Inverse: imageCoord = Scale(1/s) * Translate(-anchor) * device
	//        = (device - anchor) / s
	forward := Translate(p.anchorX, p.anchorY).Multiply(Scale(sx, sy))
	p.inverse = forward.Invert()
}

// ColorAt implements the Pattern interface.
// It samples the image at the given device-space coordinates by applying the
// pre-computed inverse transform to map back to image-space. In clamp mode,
// out-of-bounds coordinates return transparent black; otherwise the pattern tiles.
// ColorAt implements the Pattern interface.
// It samples the image at the given device-space coordinates by applying the
// pre-computed inverse transform to map back to image-space. In clamp mode,
// out-of-bounds coordinates return transparent black; otherwise the pattern tiles.
func (p *ImagePattern) ColorAt(x, y float64) RGBA {
	// Apply inverse transform: device-space -> image-space.
	imgPt := p.inverse.TransformPoint(Pt(x, y))
	lx := imgPt.X
	ly := imgPt.Y

	// Get image bounds.
	imgW, imgH := p.image.Bounds()

	// Determine pattern region.
	patternW := p.w
	patternH := p.h
	if patternW == 0 {
		patternW = imgW
	}
	if patternH == 0 {
		patternH = imgH
	}

	if p.clamp {
		// Clamp mode: out-of-bounds returns transparent.
		ix := int(lx)
		iy := int(ly)
		if ix < 0 || ix >= patternW || iy < 0 || iy >= patternH {
			return RGBA{}
		}
		ix += p.x
		iy += p.y
		r, g, b, a := p.image.GetRGBA(ix, iy)
		col := RGBA{
			R: float64(r) / 255.0,
			G: float64(g) / 255.0,
			B: float64(b) / 255.0,
			A: float64(a) / 255.0,
		}
		if p.opacity > 0 && p.opacity < 1.0 {
			col.A *= p.opacity
		}
		return col
	}

	// Wrap coordinates to pattern region (tiling).
	px := int(lx) % patternW
	py := int(ly) % patternH
	if px < 0 {
		px += patternW
	}
	if py < 0 {
		py += patternH
	}

	// Add source region offset.
	px += p.x
	py += p.y

	// Sample the image.
	r, g, b, a := p.image.GetRGBA(px, py)
	col := RGBA{
		R: float64(r) / 255.0,
		G: float64(g) / 255.0,
		B: float64(b) / 255.0,
		A: float64(a) / 255.0,
	}
	if p.opacity > 0 && p.opacity < 1.0 {
		col.A *= p.opacity
	}
	return col
}

// DrawImageRounded draws an image at (x, y) clipped to a rounded rectangle.
// The image is drawn at its natural size, clipped by a rounded rectangle with
// the given corner radius. This is a convenience method equivalent to:
//
//	dc.Push()
//	dc.DrawRoundedRectangle(x, y, w, h, radius)
//	dc.Clip()
//	dc.DrawImage(img, x, y)
//	dc.Pop()
