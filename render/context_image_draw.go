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

import (
	"math"

	intImage "github.com/energye/gpui/render/internal/image"
)

// IsAdvancedBlendMode reports whether mode requires destination sampling
// (separable/non-separable advanced blends) rather than fixed-function factors.
// Used by layer Pop dual-tex (P0-3 / G.06).
func IsAdvancedBlendMode(mode BlendMode) bool {
	switch mode {
	case BlendMultiply, BlendScreen, BlendOverlay,
		BlendDarken, BlendLighten, BlendColorDodge, BlendColorBurn,
		BlendHardLight, BlendSoftLight, BlendDifference, BlendExclusion,
		BlendHue, BlendSaturation, BlendColor, BlendLuminosity:
		return true
	default:
		return false
	}
}

// DrawImage draws an image at the specified position.
// The current transformation matrix is applied to the position and size.
//
// Example:
//
//	img, _ := render.LoadImage("photo.png")
//	dc.DrawImage(img, 100, 100)
//
// DrawImage draws an image at the specified position.
// The current transformation matrix is applied to the position and size.
//
// Example:
//
//	img, _ := render.LoadImage("photo.png")
//	dc.DrawImage(img, 100, 100)
func (c *Context) DrawImage(img *ImageBuf, x, y float64) {
	c.syncPublishedFilterBeforeDraw()
	c.DrawImageEx(img, DrawImageOptions{
		X:             x,
		Y:             y,
		Interpolation: InterpBilinear,
		Opacity:       1.0,
		BlendMode:     BlendNormal,
	})
}

// DrawImageEx draws an image with advanced options.
// The current transformation matrix is applied to the position and size.
// The image is drawn through the Fill() pipeline, which means it respects
// the current clip region. This follows the enterprise pattern used by
// Skia, Cairo, tiny-skia, and Vello: image drawing = fillRect + image shader.
//
// Example:
//
//	dc.DrawImageEx(img, render.DrawImageOptions{
//	    X: 100,
//	    Y: 100,
//	    DstWidth: 200,
//	    DstHeight: 150,
//	    Interpolation: render.InterpBicubic,
//	    Opacity: 0.8,
//	    BlendMode: render.BlendNormal,
//	})
//
// DrawImageEx draws an image with advanced options.
// The current transformation matrix is applied to the position and size.
// The image is drawn through the Fill() pipeline, which means it respects
// the current clip region. This follows the enterprise pattern used by
// Skia, Cairo, tiny-skia, and Vello: image drawing = fillRect + image shader.
//
// Example:
//
//	dc.DrawImageEx(img, render.DrawImageOptions{
//	    X: 100,
//	    Y: 100,
//	    DstWidth: 200,
//	    DstHeight: 150,
//	    Interpolation: render.InterpBicubic,
//	    Opacity: 0.8,
//	    BlendMode: render.BlendNormal,
//	})
func (c *Context) DrawImageEx(img *ImageBuf, opts DrawImageOptions) {
	c.syncPublishedFilterBeforeDraw()
	if img == nil || img.Disposed() {
		return
	}
	// Default values. InterpNearest starts at 1 so zero means unspecified, not nearest.
	if opts.Interpolation == 0 {
		opts.Interpolation = InterpBilinear
	}
	if opts.Opacity == 0 {
		opts.Opacity = 1.0
	}

	// Get source dimensions
	srcWidth, srcHeight := img.Bounds()
	srcX, srcY := 0, 0
	srcW, srcH := srcWidth, srcHeight
	if opts.SrcRect != nil {
		srcX = opts.SrcRect.Min.X
		srcY = opts.SrcRect.Min.Y
		srcW = opts.SrcRect.Dx()
		srcH = opts.SrcRect.Dy()
	}

	// Determine destination size in user coordinates (before transform).
	dstWidth := opts.DstWidth
	dstHeight := opts.DstHeight
	if dstWidth == 0 {
		dstWidth = float64(srcW)
	}
	if dstHeight == 0 {
		dstHeight = float64(srcH)
	}

	// I.04 R1: Prefer GPU textured quads. UseMipmaps uses GPU bilinear (approx
	// vs true mip chain). Bicubic uses a GPU 4x4 convolution variant
	// (textured_quad_bicubic.wgsl, Catmull-Rom kernel matching the CPU path);
	// GPU failure falls back to the CPU bicubic samplers below.
	if c.tryGPUDrawImage(img, opts, srcX, srcY, srcW, srcH, dstWidth, dstHeight) {
		return
	}
	if opts.Interpolation == InterpBicubic {
		// Direct CPU DrawImage with bicubic sampling for correctness.
		dstImg := c.pixmapToImageBuf(c.pixmap)
		if dstImg != nil {
			srcRect := &intImage.Rect{X: srcX, Y: srcY, Width: srcW, Height: srcH}
			// Destination in device pixels via CTM translation+scale approx for axis-aligned.
			ctm := c.totalMatrix()
			tl := ctm.TransformPoint(Pt(opts.X, opts.Y))
			br := ctm.TransformPoint(Pt(opts.X+dstWidth, opts.Y+dstHeight))
			dx := int(math.Min(tl.X, br.X))
			dy := int(math.Min(tl.Y, br.Y))
			dw := int(math.Abs(br.X-tl.X) + 0.5)
			dh := int(math.Abs(br.Y-tl.Y) + 0.5)
			if dw > 0 && dh > 0 {
				intImage.DrawImage(dstImg, img, intImage.DrawParams{
					SrcRect:    srcRect,
					DstRect:    intImage.Rect{X: dx, Y: dy, Width: dw, Height: dh},
					Interp:     opts.Interpolation,
					Opacity:    opts.Opacity,
					BlendMode:  opts.BlendMode,
					UseMipmaps: opts.UseMipmaps,
				})
				c.recordCPUFallbackReason("image:bicubic")
				return
			}
		}
	}

	// Compute scale factors for the image pattern.
	// The pattern maps source pixels to destination pixels.
	scaleX := dstWidth / float64(srcW)
	scaleY := dstHeight / float64(srcH)

	// Compose the full forward transform: image-space -> device-space.
	//
	// The transform chain is:
	//   1. Scale source pixels to destination size: Scale(scaleX, scaleY)
	//   2. Position at destination: Translate(opts.X, opts.Y)
	//   3. Apply current CTM to device space: totalMatrix()
	//
	// Forward: device = totalMatrix * Translate(x,y) * Scale(sx,sy) * imageCoord
	// Inverse: imageCoord = inverse(totalMatrix * Translate(x,y) * Scale(sx,sy)) * device
	//
	// This follows the Cairo/Skia/tiny-skia pattern (see IMAGE-PATTERN-TRANSFORM-RESEARCH.md).
	patternTransform := c.totalMatrix().
		Multiply(Translate(opts.X, opts.Y)).
		Multiply(Scale(scaleX, scaleY))
	inverse := patternTransform.Invert()

	// Create image pattern with pre-computed inverse transform.
	pattern := &ImagePattern{
		image:   img,
		x:       srcX,
		y:       srcY,
		w:       srcW,
		h:       srcH,
		inverse: inverse,
		opacity: opts.Opacity,
		clamp:   true, // Don't tile — clamp to image bounds
	}

	// Save current state (paint, path, transform).
	c.Push()

	// Set image pattern as fill source.
	c.SetFillPattern(pattern)

	// Draw rectangle at destination using the current transform.
	// DrawRectangle applies the transform, which is what we want.
	c.DrawRectangle(opts.X, opts.Y, dstWidth, dstHeight)

	// Fill the rectangle — the Fill() pipeline handles clipping automatically.
	_ = c.Fill()

	c.Pop()
}

// tryGPUDrawImage attempts to render the image via GPU Tier 3 (textured quad).
// Returns true if the image was queued for GPU rendering, false if the caller
// should fall back to the CPU SetFillPattern→Fill() path.
//
// The destination is always a full CTM-transformed parallelogram (TL/TR/BR/BL).
// Rotation and skew are handled by emitting non-axis-aligned quad vertices;
// the previous axis-aligned-only gate incorrectly fell back to ImagePattern
// Fill, which GPU solid-path rendering cannot texture.
// DrawImageRounded draws an image at (x, y) clipped to a rounded rectangle.
// The image is drawn at its natural size, clipped by a rounded rectangle with
// the given corner radius. This is a convenience method equivalent to:
//
//	dc.Push()
//	dc.DrawRoundedRectangle(x, y, w, h, radius)
//	dc.Clip()
//	dc.DrawImage(img, x, y)
//	dc.Pop()
func (c *Context) DrawImageRounded(img *ImageBuf, x, y, radius float64) {
	w, h := img.Bounds()
	fw := float64(w)
	fh := float64(h)

	c.Push()
	c.DrawRoundedRectangle(x, y, fw, fh, radius)
	c.Clip()
	c.DrawImage(img, x, y)
	c.Pop()
}

// DrawImageCircular draws an image at the specified center, clipped to a circle.
// The image is drawn centered at (cx, cy) and clipped by a circle with the given
// radius. The image is scaled to fit within the circle's bounding box.
// DrawImageCircular draws an image at the specified center, clipped to a circle.
// The image is drawn centered at (cx, cy) and clipped by a circle with the given
// radius. The image is scaled to fit within the circle's bounding box.
func (c *Context) DrawImageCircular(img *ImageBuf, cx, cy, radius float64) {
	c.Push()
	c.DrawCircle(cx, cy, radius)
	c.Clip()

	// Draw image centered, scaled to fit the circle diameter.
	diameter := radius * 2
	c.DrawImageEx(img, DrawImageOptions{
		X:         cx - radius,
		Y:         cy - radius,
		DstWidth:  diameter,
		DstHeight: diameter,
		Opacity:   1.0,
		BlendMode: BlendNormal,
	})

	c.Pop()
}

// ExportImageBuf copies the current surface pixels into dst for DrawImage.
//
// Skia-class pattern for continuous effects: render+filter on a small offscreen
// Context, ExportImageBuf every frame, then DrawImage on the present path.
// Pixmap storage is premultiplied; dst is created/updated as FormatRGBAPremul
// so PremultipliedData does not double-multiply.
//
// Reuses *dst when size matches; always bumps generation id after copy.
