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

import "bytes"

// SetMask sets an alpha mask for subsequent drawing operations.
// The mask modulates the alpha of all Fill and Stroke operations —
// each pixel's coverage is multiplied by the mask value at that pixel.
// A mask value of 255 means fully visible, 0 means fully transparent.
//
// When both a mask and a clip are active, they compose multiplicatively:
// finalCoverage = shapeCoverage * clipCoverage * maskCoverage / (255*255).
//
// Pass nil to clear the mask.
func (c *Context) SetMask(mask *Mask) {
	c.mask = mask
	if mask == nil {
		c.clearGPUMaskTexture()
		return
	}
	// The Mask may have been mutated in place since the last upload; drop the
	// retained R8 texture before re-uploading so the cache cannot reuse stale
	// pixels.
	c.clearGPUMaskTexture()
	c.syncGPUMaskTexture()
}

// GetMask returns the current mask, or nil if no mask is set.
func (c *Context) GetMask() *Mask {
	return c.mask
}

// InvertMask inverts the current mask.
// Has no effect if no mask is set.
func (c *Context) InvertMask() {
	if c.mask != nil {
		c.mask.Invert()
		c.clearGPUMaskTexture()
		c.syncGPUMaskTexture()
	}
}

// ClearMask removes the current mask.
func (c *Context) ClearMask() {
	c.mask = nil
	c.clearGPUMaskTexture()
}

// clearGPUMaskTexture releases the MaskAware R8 plane and retained CPU copy.
func (c *Context) clearGPUMaskTexture() {
	if a := Accelerator(); a != nil {
		if ma, ok := a.(MaskAware); ok {
			ma.ClearMaskTexture()
		}
	}
}

// syncGPUMaskTexture uploads the MaskAware R8 plane, or deactivates it while
// retaining its texture for a cheap reactivation (temporary clip masks).
func (c *Context) syncGPUMaskTexture() {
	a := Accelerator()
	if a == nil {
		return
	}
	ma, ok := a.(MaskAware)
	if !ok {
		return
	}
	if c.mask == nil {
		ma.SetMaskTexture(nil, 0, 0)
		return
	}
	ma.SetMaskTexture(c.mask.Data(), c.mask.Width(), c.mask.Height())
}

// ApplyMask applies a mask to already-drawn content using DestinationIn blending.
// For each pixel covered by the mask rectangle the premultiplied channels are
// scaled by the mask value:
//
//	pixel = pixel * mask.At(x,y) / 255
//
// Pixels outside the mask rectangle, and pixels outside the active clip, are
// left untouched. This is a post-processing operation — it modifies existing
// pixel content, not future draws. Pass nil to no-op.
//
// Implementation follows Skia's drawImage(skMask, kDstIn): the mask becomes a
// premultiplied white image whose alpha channel carries the mask, drawn in
// device space with the DestinationIn blend operator. It stays an ordinary draw
// operation — on GPU it rides the image-quad blend pipeline, with no read-back
// and no frame materialization.
func (c *Context) ApplyMask(mask *Mask) {
	if c == nil || c.pixmap == nil || mask == nil {
		return
	}
	mw, mh := mask.Width(), mask.Height()
	if mw <= 0 || mh <= 0 {
		return
	}
	img := c.maskCompositeImage(mask)
	if img == nil {
		return
	}
	// Device-space quad: the mask maps 1:1 onto the surface origin. The active
	// clip still applies (a clipped pixel is not composited, matching Skia).
	// Nearest sampling keeps one mask texel = one device pixel at any dpr.
	c.Push()
	c.SetTransform(Identity())
	c.DrawImageEx(img, DrawImageOptions{
		X:             0,
		Y:             0,
		DstWidth:      float64(mw),
		DstHeight:     float64(mh),
		BlendMode:     BlendDestinationIn,
		Interpolation: InterpNearest,
	})
	c.Pop()
}

// maskCompositeImage returns the cached premultiplied white RGBA image whose
// alpha channel is the mask. The buffer identity is stable across calls so the
// GPU image cache reuses one texture; content is refreshed (and the image
// marked dirty) only when the mask bytes actually changed.
func (c *Context) maskCompositeImage(mask *Mask) *ImageBuf {
	if c == nil || mask == nil {
		return nil
	}
	mw, mh := mask.Width(), mask.Height()
	n := mw * mh
	src := mask.Data()
	if n <= 0 || len(src) < n {
		return nil
	}
	if c.maskComposite == nil || c.maskCompositeW != mw || c.maskCompositeH != mh {
		if c.maskComposite != nil {
			c.maskComposite.Dispose()
			c.maskComposite = nil
		}
		img, err := NewImageBuf(mw, mh, FormatRGBAPremul)
		if err != nil {
			return nil
		}
		c.maskComposite = img
		c.maskCompositeW, c.maskCompositeH = mw, mh
		c.maskCompositeCopy = nil
	}
	if !bytes.Equal(c.maskCompositeCopy, src[:n]) {
		data := c.maskComposite.Data()
		if len(data) < n*4 {
			return nil
		}
		for i := 0; i < n; i++ {
			v := src[i]
			data[i*4+0] = v
			data[i*4+1] = v
			data[i*4+2] = v
			data[i*4+3] = v
		}
		if cap(c.maskCompositeCopy) < n {
			c.maskCompositeCopy = make([]uint8, n)
		} else {
			c.maskCompositeCopy = c.maskCompositeCopy[:n]
		}
		copy(c.maskCompositeCopy, src[:n])
		c.maskComposite.NotifyPixelsChanged()
	}
	return c.maskComposite
}

// AsMask creates a mask from the current unfilled path.
// The current path is rasterized (filled with white on a transparent background)
// and the alpha channel is extracted into a Mask. The fill rule from the context
// is used for rasterization. The path is NOT cleared after this operation.
//
// IMPORTANT: AsMask works with the current path, which is cleared by [Context.Fill].
// Call AsMask BEFORE Fill, or use [Context.FillPreserve] to keep the path.
//
// Correct usage patterns:
//
//	// Pattern 1: AsMask before Fill
//	dc.DrawCircle(50, 50, 30)
//	mask := dc.AsMask() // captures the circle path as a mask
//	dc.Fill() // fills the circle (clears the path)
//
//	// Pattern 2: FillPreserve + AsMask
//	dc.DrawCircle(50, 50, 30)
//	dc.FillPreserve() // fills but keeps the path
//	mask := dc.AsMask() // still has the circle path
//	dc.ClearPath()
//
//	// Pattern 3: Capture rendered output as mask
//	dc.DrawCircle(50, 50, 30)
//	dc.Fill() // path is now cleared
//	mask := NewMaskFromAlpha(dc.Image()) // capture from rendered pixels
//
// Common mistake (returns empty mask):
//
//	dc.DrawCircle(50, 50, 30)
//	dc.Fill() // clears the path!
//	mask := dc.AsMask() // path is empty → mask is all zeros
func (c *Context) AsMask() *Mask {
	mask := NewMask(c.Width(), c.Height())

	// Create a temporary context for rasterizing the path
	temp := NewContext(c.Width(), c.Height())
	temp.path = c.path.Clone()
	temp.SetRGBA(1, 1, 1, 1)
	_ = temp.Fill() // Software renderer never fails

	// Extract alpha channel from the rendered path
	img := temp.Image()
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			// a is 0-65535, shift by 8 to get 0-255
			// #nosec G115 -- safe: a>>8 is always in range [0, 255]
			mask.Set(x-bounds.Min.X, y-bounds.Min.Y, uint8(a>>8))
		}
	}

	return mask
}
